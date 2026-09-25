package refactor

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/tools/go/packages"
)

// MoveFileOptions specifies arguments for moving or renaming a file.
type MoveFileOptions struct {
	SourceFile string
	DestDir    string
	NewName    string
	BuildTags  string
	Dir        string // Optional root directory of the module or workspace
}

// MoveFile moves SourceFile (and associated _test.go file if present) to DestDir and/or renames it to NewName.
func MoveFile(opts MoveFileOptions) error {
	absSource, err := filepath.Abs(opts.SourceFile)
	if err != nil {
		return fmt.Errorf("invalid source file path: %w", err)
	}

	info, statErr := os.Stat(absSource)
	if statErr != nil {
		return fmt.Errorf("source file does not exist: %w", statErr)
	}
	if info.IsDir() {
		return fmt.Errorf("source path is a directory, use MoveDirectory instead: %s", absSource)
	}

	sourceDir := filepath.Dir(absSource)

	destTrimmed := strings.TrimSpace(opts.DestDir)
	nameTrimmed := strings.TrimSpace(opts.NewName)
	if destTrimmed == "" && nameTrimmed == "" {
		return fmt.Errorf("at least one of 'dest_dir' or 'new_name' is required to move or rename a file")
	}

	absDestDir := sourceDir
	if destTrimmed != "" {
		d, err := filepath.Abs(opts.DestDir)
		if err != nil {
			return fmt.Errorf("invalid destination directory path: %w", err)
		}
		absDestDir = d
	}

	newName := nameTrimmed
	if newName != "" {
		newName = filepath.Base(newName)
		if !strings.HasSuffix(newName, ".go") {
			newName += ".go"
		}
	}

	origBaseName := filepath.Base(absSource)
	targetBaseName := origBaseName
	if newName != "" {
		targetBaseName = newName
	}

	if sourceDir == absDestDir && origBaseName == targetBaseName {
		return fmt.Errorf("destination file path is identical to source file path: %s", absSource)
	}

	workspaceStart := sourceDir
	if strings.TrimSpace(opts.Dir) != "" {
		workspaceStart = strings.TrimSpace(opts.Dir)
	}

	ws, err := FindWorkspace(workspaceStart)
	if err != nil {
		return fmt.Errorf("failed finding workspace: %w", err)
	}

	filesToMove := collectCompanionFiles(sourceDir, absSource, origBaseName)

	// Guard against overwriting existing destination files
	for _, srcPath := range filesToMove {
		destFileName := computeDestFileName(srcPath, absSource, newName)
		destPath := filepath.Join(absDestDir, destFileName)
		if destPath != srcPath {
			if _, statErr := os.Stat(destPath); statErr == nil {
				return fmt.Errorf("destination file %q already exists; specify a different 'new_name' or remove the existing file before moving", destPath)
			}
		}
	}

	oldPkgName := DeterminePackageName(sourceDir, "")
	oldImportPath, _ := ws.CalculateImportPath(sourceDir)

	movedSymbols := collectExportedSymbols(filesToMove)
	remainingFilesInOldDir := countRemainingGoFiles(sourceDir, filesToMove)

	if cycleErr := checkCyclicDependency(ws, filesToMove, absDestDir, opts.BuildTags); cycleErr != nil {
		return cycleErr
	}

	if mkdirErr := os.MkdirAll(absDestDir, 0750); mkdirErr != nil {
		return fmt.Errorf("failed to create destination directory: %w", mkdirErr)
	}

	newPkgName := DeterminePackageName(absDestDir, "")
	newImportPath, _ := ws.CalculateImportPath(absDestDir)

	if err := moveFilesAndAdjustClauses(filesToMove, absSource, absDestDir, newName, newPkgName); err != nil {
		return err
	}

	if oldImportPath != "" && newImportPath != "" && oldImportPath != newImportPath {
		_ = updateWorkspaceMovedFileImports(ws, sourceDir, oldImportPath, newImportPath, oldPkgName, newPkgName, movedSymbols, remainingFilesInOldDir)
	}

	return nil
}

func collectCompanionFiles(sourceDir, absSource, baseName string) []string {
	var filesToMove []string
	filesToMove = append(filesToMove, absSource)

	if strings.HasSuffix(baseName, "_test.go") {
		nonTestName := strings.TrimSuffix(baseName, "_test.go") + ".go"
		candidate := filepath.Join(sourceDir, nonTestName)
		if _, candidateErr := os.Stat(candidate); candidateErr == nil {
			filesToMove = append(filesToMove, candidate)
		}
	} else {
		testName := strings.TrimSuffix(baseName, ".go") + "_test.go"
		candidate := filepath.Join(sourceDir, testName)
		if _, candidateErr := os.Stat(candidate); candidateErr == nil {
			filesToMove = append(filesToMove, candidate)
		}
	}
	return filesToMove
}

func moveFilesAndAdjustClauses(filesToMove []string, absSource, absDestDir, newName, newPkgName string) error {
	for _, srcPath := range filesToMove {
		destFileName := computeDestFileName(srcPath, absSource, newName)
		destPath := filepath.Join(absDestDir, destFileName)

		contentBytes, readErr := os.ReadFile(srcPath) // #nosec G304
		if readErr != nil {
			return fmt.Errorf("failed to read %s: %w", srcPath, readErr)
		}

		fset := token.NewFileSet()
		fileAST, parseErr := parser.ParseFile(fset, srcPath, contentBytes, parser.ParseComments)
		if parseErr != nil {
			return fmt.Errorf("failed to parse %s: %w", srcPath, parseErr)
		}

		targetPkgName := newPkgName
		if strings.HasSuffix(srcPath, "_test.go") && strings.HasSuffix(fileAST.Name.Name, "_test") {
			targetPkgName = newPkgName + "_test"
		}

		fileAST.Name.Name = targetPkgName

		if writeErr := WriteASTWithBuildTags(fset, fileAST, string(contentBytes), destPath); writeErr != nil {
			return fmt.Errorf("failed to write moved file %s: %w", destPath, writeErr)
		}

		if destPath != srcPath {
			if removeErr := os.Remove(srcPath); removeErr != nil {
				return fmt.Errorf("failed to remove old file %s: %w", srcPath, removeErr)
			}
		}
	}
	return nil
}

func computeDestFileName(srcPath, absSource, newName string) string {
	if newName == "" {
		return filepath.Base(srcPath)
	}

	origBase := filepath.Base(absSource)
	if srcPath == absSource {
		return newName
	}

	if strings.HasSuffix(origBase, "_test.go") {
		baseNoExt := strings.TrimSuffix(newName, "_test.go")
		baseNoExt = strings.TrimSuffix(baseNoExt, ".go")
		return baseNoExt + ".go"
	}

	baseNoExt := strings.TrimSuffix(newName, ".go")
	return baseNoExt + "_test.go"
}

func collectExportedSymbols(filePaths []string) map[string]bool {
	symbols := make(map[string]bool)
	for _, fp := range filePaths {
		fset := token.NewFileSet()
		fileAST, parseErr := parser.ParseFile(fset, fp, nil, parser.ParseComments)
		if parseErr != nil {
			continue
		}
		for _, decl := range fileAST.Decls {
			switch d := decl.(type) {
			case *ast.GenDecl:
				for _, spec := range d.Specs {
					switch s := spec.(type) {
					case *ast.TypeSpec:
						if unicode.IsUpper(rune(s.Name.Name[0])) {
							symbols[s.Name.Name] = true
						}
					case *ast.ValueSpec:
						for _, name := range s.Names {
							if unicode.IsUpper(rune(name.Name[0])) {
								symbols[name.Name] = true
							}
						}
					}
				}
			case *ast.FuncDecl:
				if d.Recv == nil && d.Name != nil && unicode.IsUpper(rune(d.Name.Name[0])) {
					symbols[d.Name.Name] = true
				}
			}
		}
	}
	return symbols
}

func countRemainingGoFiles(dir string, movingFiles []string) int {
	movingMap := make(map[string]bool)
	for _, mf := range movingFiles {
		abs, absErr := filepath.Abs(mf)
		if absErr == nil {
			movingMap[abs] = true
		}
	}

	count := 0
	entries, readErr := os.ReadDir(dir)
	if readErr != nil {
		return 0
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") {
			abs := filepath.Join(dir, entry.Name())
			if !movingMap[abs] {
				count++
			}
		}
	}
	return count
}

func updateWorkspaceMovedFileImports(
	ws *Workspace, sourceDir, oldImportPath, newImportPath, oldPkgName, newPkgName string,
	movedSymbols map[string]bool,
	remainingFilesInOldDir int,
) error {
	return ws.WalkGoFiles(func(path string, _ *ModuleInfo) error {
		return processFileMovedImports(path, sourceDir, oldImportPath, newImportPath, oldPkgName, newPkgName, movedSymbols, remainingFilesInOldDir)
	})
}

func processFileMovedImports(
	path, sourceDir, oldImportPath, newImportPath, oldPkgName, newPkgName string,
	movedSymbols map[string]bool,
	remainingFilesInOldDir int,
) error {
	fset := token.NewFileSet()
	astFile, parseErr := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if parseErr != nil {
		return nil
	}

	var oldImpSpec *ast.ImportSpec
	var oldImpAlias string

	for _, imp := range astFile.Imports {
		impPath, _ := strconv.Unquote(imp.Path.Value)
		if impPath == oldImportPath {
			oldImpSpec = imp
			if imp.Name != nil {
				oldImpAlias = imp.Name.Name
			} else {
				oldImpAlias = oldPkgName
			}
			break
		}
	}

	if oldImpSpec == nil {
		if filepath.Dir(path) == sourceDir {
			return updateSourcePackageFile(path, newImportPath, newPkgName, movedSymbols)
		}
		return nil
	}

	usesMovedSymbols, usesRemainingSymbols := analyzeSymbolUsages(astFile, oldImpAlias, movedSymbols, remainingFilesInOldDir)
	modified := updateFileImportSpecsAndSelectors(astFile, oldImpSpec, oldImpAlias, oldPkgName, newPkgName, newImportPath, movedSymbols, usesMovedSymbols, usesRemainingSymbols)

	if modified {
		_ = WriteASTFileWithImports(fset, astFile, path)
	}

	return nil
}

func updateSourcePackageFile(path, newImportPath, newPkgName string, movedSymbols map[string]bool) error {
	fset := token.NewFileSet()
	astFile, parseErr := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if parseErr != nil {
		return nil
	}

	usesMovedSymbols := false
	ast.Inspect(astFile, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && movedSymbols[id.Name] {
			usesMovedSymbols = true
		}
		return true
	})

	if !usesMovedSymbols {
		return nil
	}

	targetAlias := ResolveUniqueImportAlias(astFile, newImportPath, newPkgName, nil)

	if !HasImport(astFile, newImportPath) {
		newSpec := &ast.ImportSpec{
			Path: &ast.BasicLit{
				Kind:  token.STRING,
				Value: strconv.Quote(newImportPath),
			},
		}
		if targetAlias != newPkgName {
			newSpec.Name = ast.NewIdent(targetAlias)
		}
		AddImportSpec(astFile, newSpec)
	} else {
		existingSpec := FindImportSpec(astFile, newImportPath)
		targetAlias = GetImportAlias(existingSpec, targetAlias)
	}

	rewriteUnprefixedSymbols(astFile, targetAlias, movedSymbols)

	return WriteASTFileWithImports(fset, astFile, path)
}

func rewriteUnprefixedSymbols(fileAST *ast.File, newPkgName string, movedSymbols map[string]bool) {
	ast.Inspect(fileAST, func(n ast.Node) bool {
		rewriteUnprefixedNode(n, newPkgName, movedSymbols)
		return true
	})
}

func qualifyIdent(expr ast.Expr, newPkgName string, movedSymbols map[string]bool) ast.Expr {
	if id, ok := expr.(*ast.Ident); ok && movedSymbols[id.Name] {
		return &ast.SelectorExpr{X: ast.NewIdent(newPkgName), Sel: id}
	}
	return expr
}

func rewriteExprSlice(list []ast.Expr, newPkgName string, movedSymbols map[string]bool) {
	for i, expr := range list {
		list[i] = qualifyIdent(expr, newPkgName, movedSymbols)
	}
}

func rewriteUnprefixedNode(n ast.Node, newPkgName string, movedSymbols map[string]bool) {
	switch parent := n.(type) {
	case *ast.ReturnStmt:
		rewriteExprSlice(parent.Results, newPkgName, movedSymbols)
	case *ast.CallExpr:
		parent.Fun = qualifyIdent(parent.Fun, newPkgName, movedSymbols)
		rewriteExprSlice(parent.Args, newPkgName, movedSymbols)
	case *ast.AssignStmt:
		rewriteExprSlice(parent.Rhs, newPkgName, movedSymbols)
	case *ast.ValueSpec:
		parent.Type = qualifyIdent(parent.Type, newPkgName, movedSymbols)
		rewriteExprSlice(parent.Values, newPkgName, movedSymbols)
	case *ast.Field:
		parent.Type = qualifyIdent(parent.Type, newPkgName, movedSymbols)
	case *ast.StarExpr:
		parent.X = qualifyIdent(parent.X, newPkgName, movedSymbols)
	case *ast.ArrayType:
		parent.Elt = qualifyIdent(parent.Elt, newPkgName, movedSymbols)
	case *ast.CompositeLit:
		parent.Type = qualifyIdent(parent.Type, newPkgName, movedSymbols)
		rewriteExprSlice(parent.Elts, newPkgName, movedSymbols)
	case *ast.KeyValueExpr:
		parent.Value = qualifyIdent(parent.Value, newPkgName, movedSymbols)
	case *ast.BinaryExpr:
		parent.X = qualifyIdent(parent.X, newPkgName, movedSymbols)
		parent.Y = qualifyIdent(parent.Y, newPkgName, movedSymbols)
	}
}

func analyzeSymbolUsages(astFile *ast.File, oldImpAlias string, movedSymbols map[string]bool, remainingFilesInOldDir int) (bool, bool) {
	usesMovedSymbols := false
	usesRemainingSymbols := false

	ast.Inspect(astFile, func(n ast.Node) bool {
		sel, ok := n.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		id, ok := sel.X.(*ast.Ident)
		if !ok || id.Name != oldImpAlias {
			return true
		}

		if movedSymbols[sel.Sel.Name] {
			usesMovedSymbols = true
		} else {
			usesRemainingSymbols = true
		}
		return true
	})

	if !usesMovedSymbols && !usesRemainingSymbols {
		if remainingFilesInOldDir == 0 {
			usesMovedSymbols = true
		} else {
			usesRemainingSymbols = true
		}
	}

	return usesMovedSymbols, usesRemainingSymbols
}

func updateFileImportSpecsAndSelectors(
	astFile *ast.File,
	oldImpSpec *ast.ImportSpec,
	oldImpAlias, oldPkgName, newPkgName, newImportPath string,
	movedSymbols map[string]bool,
	usesMovedSymbols, usesRemainingSymbols bool,
) bool {
	if !usesMovedSymbols {
		return false
	}

	modified := false
	targetAlias := newPkgName

	existingSpec := FindImportSpec(astFile, newImportPath)
	if existingSpec != nil {
		targetAlias = GetImportAlias(existingSpec, newPkgName)
		if !usesRemainingSymbols && oldImpSpec != existingSpec {
			RemoveImportSpec(astFile, oldImpSpec)
			modified = true
		}
	} else {
		if !usesRemainingSymbols {
			targetAlias = ResolveUniqueImportAlias(astFile, newImportPath, newPkgName, oldImpSpec)
			oldImpSpec.Path.Value = strconv.Quote(newImportPath)
			if targetAlias != newPkgName {
				oldImpSpec.Name = ast.NewIdent(targetAlias)
			} else if oldImpSpec.Name != nil && (oldImpSpec.Name.Name == oldPkgName || oldImpSpec.Name.Name == newPkgName) {
				oldImpSpec.Name = nil
			}
			modified = true
		} else {
			targetAlias = ResolveUniqueImportAlias(astFile, newImportPath, newPkgName, nil)
			newSpec := &ast.ImportSpec{
				Path: &ast.BasicLit{
					Kind:  token.STRING,
					Value: strconv.Quote(newImportPath),
				},
			}
			if targetAlias != newPkgName {
				newSpec.Name = ast.NewIdent(targetAlias)
			}
			AddImportSpec(astFile, newSpec)
			modified = true
		}
	}

	if oldPkgName != targetAlias || oldImpAlias != targetAlias {
		ast.Inspect(astFile, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			id, ok := sel.X.(*ast.Ident)
			if !ok || id.Name != oldImpAlias {
				return true
			}
			if movedSymbols[sel.Sel.Name] {
				id.Name = targetAlias
				modified = true
			}
			return true
		})
	}

	return modified
}

func checkCyclicDependency(ws *Workspace, sourceFiles []string, absDestDir string, buildTags string) error {
	userTags := ParseBuildTags(buildTags)
	pkgs, pkgErr := ws.LoadPackages(userTags)
	if pkgErr != nil {
		return nil
	}

	destPkgPath, pathErr := ws.CalculateImportPath(absDestDir)
	if pathErr != nil {
		return nil
	}

	sourceImports := make(map[string]bool)
	for _, srcPath := range sourceFiles {
		fset := token.NewFileSet()
		astFile, parseErr := parser.ParseFile(fset, srcPath, nil, parser.ImportsOnly)
		if parseErr == nil {
			for _, imp := range astFile.Imports {
				path := strings.Trim(imp.Path.Value, `"`)
				sourceImports[path] = true
			}
		}
	}

	for impPath := range sourceImports {
		if impPath == destPkgPath {
			continue
		}
		if hasDependency(pkgs, impPath, destPkgPath) {
			return fmt.Errorf("cyclic dependency detected: moving file to package %q creates a cyclic dependency chain with %q", destPkgPath, impPath)
		}
	}

	return nil
}

func hasDependency(pkgs []*packages.Package, startPkgPath, targetPkgPath string) bool {
	visited := make(map[string]bool)
	var queue []string

	queue = append(queue, startPkgPath)
	visited[startPkgPath] = true

	for len(queue) > 0 {
		curr := queue[0]
		queue = queue[1:]

		if curr == targetPkgPath {
			return true
		}

		for _, pkg := range pkgs {
			if pkg.PkgPath == curr {
				for impPath := range pkg.Imports {
					if !visited[impPath] {
						visited[impPath] = true
						queue = append(queue, impPath)
					}
				}
			}
		}
	}
	return false
}
