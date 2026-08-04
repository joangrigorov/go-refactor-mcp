package refactor

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"golang.org/x/tools/go/packages"
	ximports "golang.org/x/tools/imports"
)

// MoveFileOptions specifies arguments for moving a file.
type MoveFileOptions struct {
	SourceFile string
	DestDir    string
}

// MoveFile moves a file (and its linked _test.go if present) to DestDir.
func MoveFile(opts MoveFileOptions) error {
	absSource, err := filepath.Abs(opts.SourceFile)
	if err != nil {
		return fmt.Errorf("invalid source file path: %w", err)
	}

	info, err := os.Stat(absSource)
	if err != nil || info.IsDir() {
		return fmt.Errorf("source file %s does not exist or is a directory", absSource)
	}

	sourceDir := filepath.Dir(absSource)
	moduleRoot, err := FindModuleRoot(sourceDir)
	if err != nil {
		moduleRoot = sourceDir
	}

	absDestDir, err := filepath.Abs(opts.DestDir)
	if err != nil {
		return fmt.Errorf("invalid dest dir: %w", err)
	}

	// Idempotency check: if source file is already in destDir, return success
	if sourceDir == absDestDir {
		return nil
	}

	// Check for associated _test.go file or main file
	var filesToMove []string
	filesToMove = append(filesToMove, absSource)

	baseName := filepath.Base(absSource)
	if strings.HasSuffix(baseName, "_test.go") {
		nonTestName := strings.TrimSuffix(baseName, "_test.go") + ".go"
		candidate := filepath.Join(sourceDir, nonTestName)
		if _, err := os.Stat(candidate); err == nil {
			filesToMove = append(filesToMove, candidate)
		}
	} else {
		testName := strings.TrimSuffix(baseName, ".go") + "_test.go"
		candidate := filepath.Join(sourceDir, testName)
		if _, err := os.Stat(candidate); err == nil {
			filesToMove = append(filesToMove, candidate)
		}
	}

	// Calculate old import path and old package name before moving
	oldPkgName := determinePackageName(sourceDir)
	oldImportPath, err := calculateImportPath(moduleRoot, sourceDir)
	if err != nil {
		oldImportPath = ""
	}

	// Collect exported top-level symbols defined in source files being moved
	movedSymbols := collectExportedSymbols(filesToMove)

	// Check if old package directory has remaining .go files after moving filesToMove
	remainingFilesInOldDir := countRemainingGoFiles(sourceDir, filesToMove)

	// 1. Cycle detection check before making changes
	if err := checkCyclicDependency(moduleRoot, filesToMove, absDestDir); err != nil {
		return err
	}

	// Make destination directory if it doesn't exist
	if err := os.MkdirAll(absDestDir, 0750); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Determine new package name & import path for destDir
	newPkgName := determinePackageName(absDestDir)
	newImportPath, err := calculateImportPath(moduleRoot, absDestDir)
	if err != nil {
		newImportPath = ""
	}

	// Physically move files and update package clauses
	for _, srcPath := range filesToMove {
		destPath := filepath.Join(absDestDir, filepath.Base(srcPath))

		contentBytes, err := os.ReadFile(srcPath) //nolint:gosec
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", srcPath, err)
		}

		fset := token.NewFileSet()
		fileAST, err := parser.ParseFile(fset, srcPath, contentBytes, parser.ParseComments)
		if err != nil {
			return fmt.Errorf("failed to parse %s: %w", srcPath, err)
		}

		targetPkgName := newPkgName
		if strings.HasSuffix(srcPath, "_test.go") && strings.HasSuffix(fileAST.Name.Name, "_test") {
			targetPkgName = newPkgName + "_test"
		}

		fileAST.Name.Name = targetPkgName

		if err := writeASTWithBuildTags(fset, fileAST, string(contentBytes), destPath); err != nil {
			return fmt.Errorf("failed to write moved file %s: %w", destPath, err)
		}

		if err := os.Remove(srcPath); err != nil {
			return fmt.Errorf("failed to remove old file %s: %w", srcPath, err)
		}
	}

	// Update import paths and symbol selectors across workspace files
	if oldImportPath != "" && newImportPath != "" && oldImportPath != newImportPath {
		_ = updateWorkspaceMovedFileImports(moduleRoot, sourceDir, oldImportPath, newImportPath, oldPkgName, newPkgName, movedSymbols, remainingFilesInOldDir)
	}

	return nil
}

func calculateImportPath(moduleRoot, dir string) (string, error) {
	rel, err := filepath.Rel(moduleRoot, dir)
	if err != nil {
		return "", err
	}
	modName, err := GetModuleName(moduleRoot)
	if err != nil {
		return "", err
	}
	if rel == "." || rel == "" {
		return modName, nil
	}
	return filepath.ToSlash(filepath.Join(modName, rel)), nil
}

func collectExportedSymbols(filePaths []string) map[string]bool {
	symbols := make(map[string]bool)
	for _, fp := range filePaths {
		fset := token.NewFileSet()
		fileAST, err := parser.ParseFile(fset, fp, nil, parser.ParseComments)
		if err != nil {
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
		abs, err := filepath.Abs(mf)
		if err == nil {
			movingMap[abs] = true
		}
	}

	count := 0
	entries, err := os.ReadDir(dir)
	if err != nil {
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
	moduleRoot, sourceDir, oldImportPath, newImportPath, oldPkgName, newPkgName string,
	movedSymbols map[string]bool,
	remainingFilesInOldDir int,
) error {
	return filepath.Walk(moduleRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || IsVendorPath(path) {
			return err
		}
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

	// Handle remaining files in source package directory that referenced moved symbols un-prefixed
	if oldImpSpec == nil {
		if filepath.Dir(path) == sourceDir {
			return updateSourcePackageFile(path, newImportPath, newPkgName, movedSymbols)
		}
		return nil
	}

	usesMovedSymbols, usesRemainingSymbols := analyzeSymbolUsages(astFile, oldImpAlias, movedSymbols, remainingFilesInOldDir)

	modified := updateFileImportSpecsAndSelectors(astFile, oldImpSpec, oldImpAlias, oldPkgName, newPkgName, newImportPath, movedSymbols, usesMovedSymbols, usesRemainingSymbols)

	if modified {
		var buf bytes.Buffer
		if err := format.Node(&buf, fset, astFile); err == nil {
			formatted, err := ximports.Process(path, buf.Bytes(), nil)
			if err == nil {
				_ = os.WriteFile(path, formatted, 0600)
			} else {
				_ = os.WriteFile(path, buf.Bytes(), 0600)
			}
		}
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

	if !hasImport(astFile, newImportPath) {
		newSpec := &ast.ImportSpec{
			Path: &ast.BasicLit{
				Kind:  token.STRING,
				Value: strconv.Quote(newImportPath),
			},
		}
		addImportSpec(astFile, newSpec)
	}

	rewriteUnprefixedSymbols(astFile, newPkgName, movedSymbols)

	var buf bytes.Buffer
	if err := format.Node(&buf, fset, astFile); err == nil {
		formatted, err := ximports.Process(path, buf.Bytes(), nil)
		if err == nil {
			_ = os.WriteFile(path, formatted, 0600)
		} else {
			_ = os.WriteFile(path, buf.Bytes(), 0600)
		}
	}

	return nil
}

func rewriteUnprefixedSymbols(fileAST *ast.File, newPkgName string, movedSymbols map[string]bool) {
	ast.Inspect(fileAST, func(n ast.Node) bool {
		rewriteUnprefixedNode(n, newPkgName, movedSymbols)
		return true
	})
}

func rewriteUnprefixedNode(n ast.Node, newPkgName string, movedSymbols map[string]bool) {
	if !rewriteUnprefixedNodePart1(n, newPkgName, movedSymbols) {
		rewriteUnprefixedNodePart2(n, newPkgName, movedSymbols)
	}
}

func rewriteUnprefixedNodePart1(n ast.Node, newPkgName string, movedSymbols map[string]bool) bool {
	switch parent := n.(type) {
	case *ast.ReturnStmt:
		for i, expr := range parent.Results {
			if id, ok := expr.(*ast.Ident); ok && movedSymbols[id.Name] {
				parent.Results[i] = &ast.SelectorExpr{X: ast.NewIdent(newPkgName), Sel: id}
			}
		}
		return true
	case *ast.CallExpr:
		if id, ok := parent.Fun.(*ast.Ident); ok && movedSymbols[id.Name] {
			parent.Fun = &ast.SelectorExpr{X: ast.NewIdent(newPkgName), Sel: id}
		}
		for i, arg := range parent.Args {
			if id, ok := arg.(*ast.Ident); ok && movedSymbols[id.Name] {
				parent.Args[i] = &ast.SelectorExpr{X: ast.NewIdent(newPkgName), Sel: id}
			}
		}
		return true
	case *ast.AssignStmt:
		for i, rhs := range parent.Rhs {
			if id, ok := rhs.(*ast.Ident); ok && movedSymbols[id.Name] {
				parent.Rhs[i] = &ast.SelectorExpr{X: ast.NewIdent(newPkgName), Sel: id}
			}
		}
		return true
	case *ast.ValueSpec:
		if id, ok := parent.Type.(*ast.Ident); ok && movedSymbols[id.Name] {
			parent.Type = &ast.SelectorExpr{X: ast.NewIdent(newPkgName), Sel: id}
		}
		for i, val := range parent.Values {
			if id, ok := val.(*ast.Ident); ok && movedSymbols[id.Name] {
				parent.Values[i] = &ast.SelectorExpr{X: ast.NewIdent(newPkgName), Sel: id}
			}
		}
		return true
	case *ast.Field:
		if id, ok := parent.Type.(*ast.Ident); ok && movedSymbols[id.Name] {
			parent.Type = &ast.SelectorExpr{X: ast.NewIdent(newPkgName), Sel: id}
		}
		return true
	}
	return false
}

func rewriteUnprefixedNodePart2(n ast.Node, newPkgName string, movedSymbols map[string]bool) {
	switch parent := n.(type) {
	case *ast.StarExpr:
		if id, ok := parent.X.(*ast.Ident); ok && movedSymbols[id.Name] {
			parent.X = &ast.SelectorExpr{X: ast.NewIdent(newPkgName), Sel: id}
		}
	case *ast.ArrayType:
		if id, ok := parent.Elt.(*ast.Ident); ok && movedSymbols[id.Name] {
			parent.Elt = &ast.SelectorExpr{X: ast.NewIdent(newPkgName), Sel: id}
		}
	case *ast.CompositeLit:
		if id, ok := parent.Type.(*ast.Ident); ok && movedSymbols[id.Name] {
			parent.Type = &ast.SelectorExpr{X: ast.NewIdent(newPkgName), Sel: id}
		}
		for i, elt := range parent.Elts {
			if id, ok := elt.(*ast.Ident); ok && movedSymbols[id.Name] {
				parent.Elts[i] = &ast.SelectorExpr{X: ast.NewIdent(newPkgName), Sel: id}
			}
		}
	case *ast.KeyValueExpr:
		if id, ok := parent.Value.(*ast.Ident); ok && movedSymbols[id.Name] {
			parent.Value = &ast.SelectorExpr{X: ast.NewIdent(newPkgName), Sel: id}
		}
	case *ast.BinaryExpr:
		if id, ok := parent.X.(*ast.Ident); ok && movedSymbols[id.Name] {
			parent.X = &ast.SelectorExpr{X: ast.NewIdent(newPkgName), Sel: id}
		}
		if id, ok := parent.Y.(*ast.Ident); ok && movedSymbols[id.Name] {
			parent.Y = &ast.SelectorExpr{X: ast.NewIdent(newPkgName), Sel: id}
		}
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

	existingSpec := findImportSpec(astFile, newImportPath)
	if existingSpec != nil {
		targetAlias = getImportAlias(existingSpec, newPkgName)
		if !usesRemainingSymbols && oldImpSpec != existingSpec {
			removeImportSpec(astFile, oldImpSpec)
			modified = true
		}
	} else {
		if !usesRemainingSymbols {
			oldImpSpec.Path.Value = strconv.Quote(newImportPath)
			if oldImpSpec.Name != nil && (oldImpSpec.Name.Name == oldPkgName || oldImpSpec.Name.Name == newPkgName) {
				oldImpSpec.Name = nil
			}
			modified = true
		} else {
			newSpec := &ast.ImportSpec{
				Path: &ast.BasicLit{
					Kind:  token.STRING,
					Value: strconv.Quote(newImportPath),
				},
			}
			addImportSpec(astFile, newSpec)
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

func findImportSpec(fileAST *ast.File, importPath string) *ast.ImportSpec {
	for _, imp := range fileAST.Imports {
		if imp.Path != nil {
			path, err := strconv.Unquote(imp.Path.Value)
			if err == nil && path == importPath {
				return imp
			}
		}
	}
	return nil
}

func getImportAlias(spec *ast.ImportSpec, defaultPkgName string) string {
	if spec != nil && spec.Name != nil && spec.Name.Name != "" && spec.Name.Name != "_" && spec.Name.Name != "." {
		return spec.Name.Name
	}
	return defaultPkgName
}

func removeImportSpec(fileAST *ast.File, targetSpec *ast.ImportSpec) {
	for _, decl := range fileAST.Decls {
		g, ok := decl.(*ast.GenDecl)
		if ok && g.Tok == token.IMPORT {
			var newSpecs []ast.Spec
			for _, s := range g.Specs {
				if is, ok := s.(*ast.ImportSpec); ok && is == targetSpec {
					continue
				}
				newSpecs = append(newSpecs, s)
			}
			g.Specs = newSpecs
			return
		}
	}
}

func hasImport(fileAST *ast.File, importPath string) bool {
	return findImportSpec(fileAST, importPath) != nil
}

func addImportSpec(fileAST *ast.File, spec *ast.ImportSpec) {
	if spec == nil || spec.Path == nil {
		return
	}
	pathVal, err := strconv.Unquote(spec.Path.Value)
	if err == nil && hasImport(fileAST, pathVal) {
		return // Avoid duplicate imports
	}

	for _, decl := range fileAST.Decls {
		g, ok := decl.(*ast.GenDecl)
		if ok && g.Tok == token.IMPORT {
			for _, s := range g.Specs {
				if is, ok := s.(*ast.ImportSpec); ok && is.Path != nil {
					p, _ := strconv.Unquote(is.Path.Value)
					if p == pathVal {
						return
					}
				}
			}
			g.Specs = append(g.Specs, spec)
			return
		}
	}

	importDecl := &ast.GenDecl{
		Tok:   token.IMPORT,
		Specs: []ast.Spec{spec},
	}
	fileAST.Decls = append([]ast.Decl{importDecl}, fileAST.Decls...)
}

func determinePackageName(dirPath string) string {
	entries, err := os.ReadDir(dirPath)
	if err == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") {
				fset := token.NewFileSet()
				node, err := parser.ParseFile(fset, filepath.Join(dirPath, entry.Name()), nil, parser.PackageClauseOnly)
				if err == nil && node.Name != nil && !strings.HasSuffix(node.Name.Name, "_test") {
					return node.Name.Name
				}
			}
		}
	}
	base := filepath.Base(dirPath)
	clean := strings.ReplaceAll(base, "-", "_")
	clean = strings.ReplaceAll(clean, ".", "_")
	if clean == "" || clean == "." {
		return "main"
	}
	return clean
}

func checkCyclicDependency(moduleRoot string, sourceFiles []string, absDestDir string) error {
	pkgs, err := LoadModulePackages(moduleRoot)
	if err != nil {
		return nil
	}

	relDest, err := filepath.Rel(moduleRoot, absDestDir)
	if err != nil {
		return nil
	}
	modName, err := GetModuleName(moduleRoot)
	if err != nil {
		return nil
	}

	destPkgPath := filepath.ToSlash(filepath.Join(modName, relDest))

	sourceImports := make(map[string]bool)
	for _, srcPath := range sourceFiles {
		fset := token.NewFileSet()
		astFile, err := parser.ParseFile(fset, srcPath, nil, parser.ImportsOnly)
		if err == nil {
			for _, imp := range astFile.Imports {
				path := strings.Trim(imp.Path.Value, `"`)
				sourceImports[path] = true
			}
		}
	}

	for impPath := range sourceImports {
		if impPath == destPkgPath {
			return fmt.Errorf("cyclic dependency detected: moving file to package %q creates an import cycle with %q", destPkgPath, impPath)
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

func writeASTWithBuildTags(fset *token.FileSet, fileAST *ast.File, origContent string, destPath string) error {
	buildTags := ExtractBuildTags(origContent)
	var buf strings.Builder
	if len(buildTags) > 0 {
		for _, tag := range buildTags {
			buf.WriteString(tag)
			buf.WriteString("\n")
		}
		buf.WriteString("\n")
	}

	if err := writeASTToFile(fset, fileAST, destPath); err != nil {
		return err
	}

	formattedBytes, err := os.ReadFile(destPath) //nolint:gosec
	if err != nil {
		return err
	}

	if len(buildTags) > 0 {
		finalContent := buf.String() + string(formattedBytes)
		return os.WriteFile(destPath, []byte(finalContent), 0600)
	}
	return nil
}
