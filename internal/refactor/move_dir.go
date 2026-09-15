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
)

// MoveDirOptions specifies arguments for moving a directory.
type MoveDirOptions struct {
	SourceDir string
	DestDir   string
	BuildTags string
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

	ws, err := FindWorkspace(absSource)
	if err != nil {
		return fmt.Errorf("failed finding workspace: %w", err)
	}

	oldImportPath, err := ws.CalculateImportPath(absSource)
	if err != nil {
		return fmt.Errorf("could not determine old import path: %w", err)
	}

	newImportPath, err := ws.CalculateImportPath(absDest)
	if err != nil {
		return fmt.Errorf("could not determine new import path: %w", err)
	}

	if err := moveDirOnDisk(absSource, absDest); err != nil {
		return err
	}

	if err := updateMovedPackageDeclarations(absDest); err != nil {
		return err
	}

	return updateWorkspaceImports(ws, oldImportPath, newImportPath)
}

func moveDirOnDisk(absSource, absDest string) error {
	relDestFromSource, relErr := filepath.Rel(absSource, absDest)
	isChild := relErr == nil && relDestFromSource != "." && relDestFromSource != ".." && !strings.HasPrefix(relDestFromSource, ".."+string(filepath.Separator))

	if isChild {
		parentOfSource := filepath.Dir(absSource)
		tmpDir, mkErr := os.MkdirTemp(parentOfSource, ".move_dir_tmp_*")
		if mkErr != nil {
			return fmt.Errorf("failed to create temporary directory: %w", mkErr)
		}
		_ = os.Remove(tmpDir)
		defer func() {
			_ = os.RemoveAll(tmpDir)
		}()

		if renameErr := os.Rename(absSource, tmpDir); renameErr != nil {
			return fmt.Errorf("failed to move directory to temp location %s: %w", tmpDir, renameErr)
		}

		if mkdirErr := os.MkdirAll(filepath.Dir(absDest), 0750); mkdirErr != nil {
			return fmt.Errorf("failed to create parent directory for dest: %w", mkdirErr)
		}

		if renameErr := os.Rename(tmpDir, absDest); renameErr != nil {
			return fmt.Errorf("failed to move directory from temp to %s: %w", absDest, renameErr)
		}
		return nil
	}

	if mkdirErr := os.MkdirAll(filepath.Dir(absDest), 0750); mkdirErr != nil {
		return fmt.Errorf("failed to create parent directory for dest: %w", mkdirErr)
	}

	if renameErr := os.Rename(absSource, absDest); renameErr != nil {
		return fmt.Errorf("failed to move directory from %s to %s: %w", absSource, absDest, renameErr)
	}
	return nil
}

func updateMovedPackageDeclarations(absDest string) error {
	err := filepath.Walk(absDest, func(path string, info os.FileInfo, walkErr error) error {
		if walkErr != nil || info.IsDir() || !strings.HasSuffix(path, ".go") {
			return walkErr
		}
		fset := token.NewFileSet()
		astFile, parseErr := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if parseErr == nil && astFile.Name != nil {
			targetPkgName := targetPackageNameForMovedFile(astFile, filepath.Dir(path))
			if astFile.Name.Name != targetPkgName {
				astFile.Name.Name = targetPkgName
				_ = WriteASTFile(fset, astFile, path)
			}
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("failed to update moved directory package declarations: %w", err)
	}
	return nil
}

func targetPackageNameForMovedFile(astFile *ast.File, dirPath string) string {
	if astFile.Name == nil {
		return CleanPackageIdentifier(filepath.Base(dirPath))
	}
	if astFile.Name.Name == "main" {
		return "main"
	}
	if astFile.Name.Name == "main_test" {
		return "main_test"
	}

	dirPkgName := CleanPackageIdentifier(filepath.Base(dirPath))
	if strings.HasSuffix(astFile.Name.Name, "_test") {
		return dirPkgName + "_test"
	}
	return dirPkgName
}

func updateWorkspaceImports(ws *Workspace, oldImportPath, newImportPath string) error {
	err := ws.WalkGoFiles(func(path string, _ *ModuleInfo) error {
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
			_ = WriteASTFile(fset, astFile, path)
		}
		return nil
	})

	if err != nil {
		return fmt.Errorf("failed updating workspace import paths: %w", err)
	}

	return nil
}
