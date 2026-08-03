package refactor

import (
	"fmt"
	"go/ast"
	"go/parser"

	"go/token"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"
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

	moduleRoot, err := FindModuleRoot(filepath.Dir(absSource))
	if err != nil {
		moduleRoot = filepath.Dir(absSource)
	}

	absDestDir, err := filepath.Abs(opts.DestDir)
	if err != nil {
		return fmt.Errorf("invalid dest dir: %w", err)
	}

	// Idempotency check: if source file is already in destDir, return success
	if filepath.Dir(absSource) == absDestDir {
		return nil
	}

	// Check for associated _test.go file or main file
	var filesToMove []string
	filesToMove = append(filesToMove, absSource)

	baseName := filepath.Base(absSource)
	if strings.HasSuffix(baseName, "_test.go") {
		nonTestName := strings.TrimSuffix(baseName, "_test.go") + ".go"
		candidate := filepath.Join(filepath.Dir(absSource), nonTestName)
		if _, err := os.Stat(candidate); err == nil {
			filesToMove = append(filesToMove, candidate)
		}
	} else {
		testName := strings.TrimSuffix(baseName, ".go") + "_test.go"
		candidate := filepath.Join(filepath.Dir(absSource), testName)
		if _, err := os.Stat(candidate); err == nil {
			filesToMove = append(filesToMove, candidate)
		}
	}

	// 1. Cycle detection check before making changes
	if err := checkCyclicDependency(moduleRoot, filesToMove, absDestDir); err != nil {
		return err
	}

	// Make destination directory if it doesn't exist
	if err := os.MkdirAll(absDestDir, 0750); err != nil {
		return fmt.Errorf("failed to create destination directory: %w", err)
	}

	// Determine new package name for destDir
	newPkgName := determinePackageName(absDestDir)

	for _, srcPath := range filesToMove {
		destPath := filepath.Join(absDestDir, filepath.Base(srcPath))

		// Read content & comments
		contentBytes, err := os.ReadFile(srcPath) //nolint:gosec
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", srcPath, err)
		}

		fset := token.NewFileSet()
		fileAST, err := parser.ParseFile(fset, srcPath, contentBytes, parser.ParseComments)
		if err != nil {
			return fmt.Errorf("failed to parse %s: %w", srcPath, err)
		}

		// Handle package name for test files vs main files
		targetPkgName := newPkgName
		if strings.HasSuffix(srcPath, "_test.go") && strings.HasSuffix(fileAST.Name.Name, "_test") {
			targetPkgName = newPkgName + "_test"
		}

		fileAST.Name.Name = targetPkgName

		// Write modified file AST to destPath while preserving build tags
		if err := writeASTWithBuildTags(fset, fileAST, string(contentBytes), destPath); err != nil {
			return fmt.Errorf("failed to write moved file %s: %w", destPath, err)
		}

		// Remove old source file
		if err := os.Remove(srcPath); err != nil {
			return fmt.Errorf("failed to remove old file %s: %w", srcPath, err)
		}
	}

	// Update module imports across the project
	_ = updateModuleImports(moduleRoot)

	return nil
}

func determinePackageName(dirPath string) string {
	// Look for existing .go files in dirPath
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
		return nil // skip if package graph is incomplete
	}

	// Determine dest package path
	relDest, err := filepath.Rel(moduleRoot, absDestDir)
	if err != nil {
		return nil
	}
	modName, err := GetModuleName(moduleRoot)
	if err != nil {
		return nil
	}

	destPkgPath := filepath.ToSlash(filepath.Join(modName, relDest))

	// Collect imports of the source file
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

	// Check if any package in sourceImports imports destPkgPath directly or transitively
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

	// Read AST formatted file
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

func updateModuleImports(moduleRoot string) error {
	pkgs, err := LoadModulePackages(moduleRoot)
	if err != nil {
		return nil
	}
	for _, pkg := range pkgs {
		for _, file := range pkg.GoFiles {
			if IsVendorPath(file) {
				continue
			}
			fset := token.NewFileSet()
			astFile, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
			if err == nil {
				_ = writeASTToFile(fset, astFile, file)
			}
		}
	}
	return nil
}
