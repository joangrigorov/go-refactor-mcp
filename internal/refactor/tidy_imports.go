package refactor

import (
	"fmt"
	"os"
	"path/filepath"

	"golang.org/x/tools/imports"
)

// TidyImports organizes, formats, and cleans unused imports in a file.
func TidyImports(filePath string) error {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("invalid file path: %w", err)
	}

	src, err := os.ReadFile(absPath) //nolint:gosec
	if err != nil {
		return fmt.Errorf("failed reading file %s: %w", absPath, err)
	}

	formatted, err := imports.Process(absPath, src, nil)
	if err != nil {
		return fmt.Errorf("failed processing imports for %s: %w", absPath, err)
	}

	if string(src) == string(formatted) {
		return nil // Idempotent
	}

	return os.WriteFile(absPath, formatted, 0600)
}
