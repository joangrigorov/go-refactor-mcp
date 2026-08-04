package refactor

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/token"
	"go/types"
	"os"
	"path/filepath"

	"golang.org/x/tools/go/packages"
)

// RenameOptions specifies arguments for symbol renaming.
type RenameOptions struct {
	Dir     string // Root directory of module/workspace
	File    string // Target file path (optional if position is given)
	Offset  int    // Byte offset or position in file (optional)
	From    string // Old symbol name
	To      string // New symbol name
	Package string // Target package path/name (optional)
}

// RenameSymbol renames a symbol (var, func, struct, interface, field, type param) across the module.
func RenameSymbol(opts RenameOptions) error {
	if opts.From == "" || opts.To == "" {
		return fmt.Errorf("both 'from' and 'to' symbol names are required")
	}
	if opts.From == opts.To {
		return nil // Idempotent: already has desired name
	}

	moduleRoot, err := FindModuleRoot(opts.Dir)
	if err != nil {
		moduleRoot = opts.Dir
	}

	pkgs, err := LoadModulePackages(moduleRoot)
	if err != nil {
		return fmt.Errorf("failed to load workspace packages: %w", err)
	}

	targetObj := findTargetSymbol(pkgs, opts)

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
	for _, pkg := range pkgs {
		for _, syntaxFile := range pkg.Syntax {
			pos := pkg.Fset.Position(syntaxFile.Pos())
			if pos.Filename == "" || IsVendorPath(pos.Filename) {
				continue
			}

			modified := false
			ast.Inspect(syntaxFile, func(n ast.Node) bool {
				id, ok := n.(*ast.Ident)
				if !ok || id.Name != opts.From {
					return true
				}

				if targetObj != nil && pkg.TypesInfo != nil {
					objDef := pkg.TypesInfo.Defs[id]
					objUse := pkg.TypesInfo.Uses[id]
					if sameObject(objDef, targetObj) || sameObject(objUse, targetObj) {
						id.Name = opts.To
						modified = true
					} else if opts.File != "" && isSameFile(pkg.Fset.Position(id.Pos()).Filename, opts.File) && isTypeParamObject(targetObj) {
						id.Name = opts.To
						modified = true
					}
				} else if opts.File == "" || isSameFile(pkg.Fset.Position(id.Pos()).Filename, opts.File) {
					id.Name = opts.To
					modified = true
				}
				return true
			})

			if modified {
				if err := writeASTToFile(pkg.Fset, syntaxFile, pos.Filename); err != nil {
					return fmt.Errorf("failed writing refactored file %s: %w", pos.Filename, err)
				}
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

func writeASTToFile(fset *token.FileSet, file *ast.File, filename string) error {
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, file); err != nil {
		return err
	}
	return os.WriteFile(filename, buf.Bytes(), 0600)
}
