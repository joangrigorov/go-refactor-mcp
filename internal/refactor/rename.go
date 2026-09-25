package refactor

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"
)

// RenameOptions specifies arguments for symbol renaming.
type RenameOptions struct {
	Dir       string // Root directory of module/workspace (required)
	File      string // Target file path (optional if position is given)
	Offset    int    // Byte offset or position in file (optional)
	From      string // Old symbol name
	To        string // New symbol name
	Package   string // Target package path/name (optional)
	BuildTags string // Optional build tags override (comma-separated)
}

// RenameSymbol renames a symbol (var, func, struct, interface, field, type param) across the module.
func RenameSymbol(opts RenameOptions) error {
	dirTrimmed := strings.TrimSpace(opts.Dir)
	if dirTrimmed == "" {
		return fmt.Errorf("directory parameter is required")
	}

	absDir, err := filepath.Abs(dirTrimmed)
	if err != nil {
		return fmt.Errorf("invalid directory path: %w", err)
	}

	dirInfo, err := os.Stat(absDir)
	if err != nil {
		return fmt.Errorf("workspace directory does not exist: %w", err)
	}
	if !dirInfo.IsDir() {
		return fmt.Errorf("workspace directory is not a directory: %s", absDir)
	}
	opts.Dir = absDir

	cleanFrom := strings.TrimSpace(opts.From)
	cleanTo := strings.TrimSpace(opts.To)
	if cleanFrom == "" || cleanTo == "" {
		return fmt.Errorf("both 'from' and 'to' symbol names are required")
	}
	if !token.IsIdentifier(cleanTo) || token.Lookup(cleanTo).IsKeyword() {
		return fmt.Errorf("target symbol name %q is not a valid Go identifier (must start with a letter/underscore and cannot be a Go keyword)", cleanTo)
	}
	if cleanFrom == cleanTo {
		return nil // Idempotent: already has desired name
	}
	opts.From = cleanFrom
	opts.To = cleanTo

	if strings.TrimSpace(opts.File) != "" {
		cleanFile := strings.TrimSpace(opts.File)
		if !filepath.IsAbs(cleanFile) {
			cleanFile = filepath.Join(absDir, cleanFile)
		}
		cleanFile = filepath.Clean(cleanFile)
		relFile, relErr := filepath.Rel(absDir, cleanFile)
		if relErr != nil || relFile == ".." || strings.HasPrefix(relFile, ".."+string(filepath.Separator)) {
			return fmt.Errorf("file %q is outside workspace directory %q", cleanFile, absDir)
		}
		opts.File = cleanFile
	}

	ws, err := FindWorkspaceWithCeiling(absDir, absDir)
	if err != nil {
		ws = &Workspace{
			Root:    absDir,
			Modules: []*ModuleInfo{{Root: absDir, Path: ""}},
		}
	}

	userTags := ParseBuildTags(opts.BuildTags)
	pkgs, err := ws.LoadAllPlatformPackages(userTags)
	if err != nil {
		return fmt.Errorf("failed to load workspace packages: %w", err)
	}

	targetObj := findTargetSymbol(pkgs, opts)
	if targetObj == nil {
		if opts.File != "" {
			return fmt.Errorf("symbol %q not found in file %q (%d packages scanned). Check spelling, offset, or verify the symbol declaration", opts.From, opts.File, len(pkgs))
		}
		return fmt.Errorf("symbol %q not found in workspace (%d packages scanned). If the symbol is unexported or located in a specific file, specify the 'file' parameter", opts.From, len(pkgs))
	}

	return applyASTRename(pkgs, targetObj, opts)
}

func findTargetSymbol(pkgs []*packages.Package, opts RenameOptions) types.Object {
	for _, pkg := range pkgs {
		if pkg.TypesInfo == nil {
			continue
		}
		for id, obj := range pkg.TypesInfo.Defs {
			if id != nil && obj != nil && obj.Name() == opts.From {
				if matchFileAndOffset(pkg.Fset, id.Pos(), opts.File, opts.Offset) {
					return obj
				}
			}
		}
	}

	for _, pkg := range pkgs {
		if pkg.TypesInfo == nil {
			continue
		}
		for id, obj := range pkg.TypesInfo.Uses {
			if id != nil && obj != nil && obj.Name() == opts.From {
				if matchFileAndOffset(pkg.Fset, id.Pos(), opts.File, opts.Offset) {
					return obj
				}
			}
		}
	}

	return nil
}

func matchFileAndOffset(fset *token.FileSet, pos token.Pos, fileFilter string, offsetFilter int) bool {
	if fileFilter == "" {
		return true
	}
	position := fset.Position(pos)
	absDefFile, _ := filepath.Abs(position.Filename)
	absTargetFile, _ := filepath.Abs(fileFilter)
	if absDefFile != absTargetFile {
		return false
	}
	if offsetFilter > 0 && position.Offset != offsetFilter {
		return false
	}
	return true
}

func applyASTRename(pkgs []*packages.Package, targetObj types.Object, opts RenameOptions) error {
	writtenFiles := make(map[string]bool)
	for _, pkg := range pkgs {
		for _, syntaxFile := range pkg.Syntax {
			pos := pkg.Fset.Position(syntaxFile.Pos())
			if pos.Filename == "" || IsVendorPath(pos.Filename) {
				continue
			}
			absFile, absErr := filepath.Abs(pos.Filename)
			if absErr == nil {
				rel, relErr := filepath.Rel(opts.Dir, absFile)
				if relErr != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
					continue
				}
			}
			if writtenFiles[pos.Filename] {
				continue
			}

			modified := false
			ast.Inspect(syntaxFile, func(n ast.Node) bool {
				id, ok := n.(*ast.Ident)
				if !ok || id.Name != opts.From {
					return true
				}

				if pkg.TypesInfo != nil {
					objDef := pkg.TypesInfo.Defs[id]
					objUse := pkg.TypesInfo.Uses[id]
					if sameObject(objDef, targetObj) || sameObject(objUse, targetObj) ||
						sameSymbolAcrossPlatforms(objDef, targetObj) || sameSymbolAcrossPlatforms(objUse, targetObj) {
						id.Name = opts.To
						modified = true
					} else if opts.File != "" && isSameFile(pkg.Fset.Position(id.Pos()).Filename, opts.File) && isTypeParamObject(targetObj) {
						id.Name = opts.To
						modified = true
					}
				}
				return true
			})

			if modified {
				if err := WriteASTFile(pkg.Fset, syntaxFile, pos.Filename); err != nil {
					return fmt.Errorf("failed writing refactored file %s: %w", pos.Filename, err)
				}
				writtenFiles[pos.Filename] = true
			}
		}
	}
	return nil
}

func sameObject(a, b types.Object) bool {
	if a == nil || b == nil {
		return false
	}
	if a == b {
		return true
	}
	return a.Name() == b.Name() && a.Pos() == b.Pos()
}

func sameSymbolAcrossPlatforms(a, b types.Object) bool {
	if a == nil || b == nil {
		return false
	}
	if sameObject(a, b) {
		return true
	}
	if a.Name() != b.Name() {
		return false
	}
	if a.Pkg() == nil || b.Pkg() == nil {
		return false
	}
	if a.Pkg().Path() != b.Pkg().Path() {
		return false
	}

	// Case 1: Package-level declarations (functions, types, vars, consts)
	if a.Parent() != nil && a.Parent() == a.Pkg().Scope() &&
		b.Parent() != nil && b.Parent() == b.Pkg().Scope() {
		return true
	}

	// Case 2: Methods on the same named receiver type
	sigA, okA := a.Type().(*types.Signature)
	sigB, okB := b.Type().(*types.Signature)
	if okA && okB && sigA.Recv() != nil && sigB.Recv() != nil {
		recvA := receiverTypeName(sigA.Recv())
		recvB := receiverTypeName(sigB.Recv())
		if recvA != "" && recvA == recvB {
			return true
		}
	}

	return false
}

func receiverTypeName(recv *types.Var) string {
	if recv == nil {
		return ""
	}
	t := recv.Type()
	if ptr, ok := t.(*types.Pointer); ok {
		t = ptr.Elem()
	}
	if named, ok := t.(*types.Named); ok {
		return named.Obj().Name()
	}
	return ""
}

func isTypeParamObject(obj types.Object) bool {
	if obj == nil {
		return false
	}
	_, ok := obj.Type().(*types.TypeParam)
	return ok
}

func isSameFile(f1, f2 string) bool {
	a1, err1 := filepath.Abs(f1)
	a2, err2 := filepath.Abs(f2)
	if err1 != nil || err2 != nil {
		return f1 == f2
	}
	return a1 == a2
}
