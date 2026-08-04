package refactor

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// MoveDirOptions specifies arguments for moving a directory.
type MoveDirOptions struct {
	SourceDir string
	DestDir   string
}

// MoveDirectory moves a directory and updates all import paths referencing it across the workspace.
func MoveDirectory(opts MoveDirOptions) error {
	absSource, err := filepath.Abs(opts.SourceDir)
	if err != nil {
		return fmt.Errorf("invalid source dir: %w", err)
	}
	absDest, err := filepath.Abs(opts.DestDir)
	if err != nil {
		return fmt.Errorf("invalid dest dir: %w", err)
	}

	if absSource == absDest {
		return nil // Idempotent
	}

	info, err := os.Stat(absSource)
	if err != nil || !info.IsDir() {
		return fmt.Errorf("source directory %s does not exist or is not a directory", absSource)
	}

	moduleRoot, err := FindModuleRoot(absSource)
	if err != nil {
		moduleRoot = filepath.Dir(absSource)
	}

	modName, err := GetModuleName(moduleRoot)
	if err != nil {
		return fmt.Errorf("could not determine module name: %w", err)
	}

	relOld, err := filepath.Rel(moduleRoot, absSource)
	if err != nil {
		return fmt.Errorf("failed to get relative path for source dir: %w", err)
	}
	relNew, err := filepath.Rel(moduleRoot, absDest)
	if err != nil {
		return fmt.Errorf("failed to get relative path for dest dir: %w", err)
	}

	oldImportPath := filepath.ToSlash(filepath.Join(modName, relOld))
	newImportPath := filepath.ToSlash(filepath.Join(modName, relNew))

	// Move directory on disk
	if mkdirErr := os.MkdirAll(filepath.Dir(absDest), 0750); mkdirErr != nil {
		return fmt.Errorf("failed to create parent directory for dest: %w", mkdirErr)
	}

	if renameErr := os.Rename(absSource, absDest); renameErr != nil {
		return fmt.Errorf("failed to move directory from %s to %s: %w", absSource, absDest, renameErr)
	}

	// Update package declarations inside moved directory files according to their specific directory level
	err = filepath.Walk(absDest, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
			return walkErr
		}
		fset := token.NewFileSet()
		astFile, parseErr := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if parseErr == nil && astFile.Name != nil {
			dirPkgName := cleanDirPackageName(filepath.Dir(path))
			targetPkgName := dirPkgName
			if strings.HasSuffix(astFile.Name.Name, "_test") {
				targetPkgName = dirPkgName + "_test"
			}
			if astFile.Name.Name != targetPkgName {
				astFile.Name.Name = targetPkgName
				_ = writeASTToFile(fset, astFile, path)
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to update moved directory package declarations: %w", err)
	}

	// Walk workspace and update import specs across all .go files
	err = filepath.Walk(moduleRoot, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || IsVendorPath(path) {
			return walkErr
		}

		fset := token.NewFileSet()
		astFile, parseErr := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if parseErr != nil {
			return nil
		}

		modified := false
		for _, imp := range astFile.Imports {
			importPath, _ := strconv.Unquote(imp.Path.Value)
			if importPath == oldImportPath || strings.HasPrefix(importPath, oldImportPath+"/") {
				updatedPath := strings.Replace(importPath, oldImportPath, newImportPath, 1)
				imp.Path.Value = strconv.Quote(updatedPath)
				modified = true
			}
		}

		if modified {
			_ = writeASTToFile(fset, astFile, path)
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed updating module import paths: %w", err)
	}

	return nil
}

func cleanDirPackageName(dirPath string) string {
	base := filepath.Base(dirPath)
	clean := strings.ReplaceAll(base, "-", "_")
	clean = strings.ReplaceAll(clean, ".", "_")
	if clean == "" || clean == "." {
		return "main"
	}
	return clean
}
