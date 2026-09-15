package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joangrigorov/go-refactor-mcp/internal/refactor"
)

func TestRenameSymbolSafetyWhenSymbolNotFound(t *testing.T) {
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module example.com/safemod\n\ngo 1.22.0\n"), 0600)

	code := `package safemod

func Hello() string {
	err := "ok"
	return err
}
`
	filePath := filepath.Join(tempDir, "main.go")
	_ = os.WriteFile(filePath, []byte(code), 0600)

	// Attempt to rename a nonexistent symbol "NonExistentFoo"
	opts := refactor.RenameOptions{
		Dir:  tempDir,
		From: "NonExistentFoo",
		To:   "Bar",
	}

	err := refactor.RenameSymbol(opts)
	if err == nil {
		t.Fatalf("expected error when renaming non-existent symbol, got nil")
	}

	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected error to mention 'not found', got: %v", err)
	}

	// Verify file content was not modified
	content, readErr := os.ReadFile(filePath) //nolint:gosec
	if readErr != nil {
		t.Fatalf("failed reading file: %v", readErr)
	}

	if string(content) != code {
		t.Errorf("file should not have been modified, got:\n%s", string(content))
	}
}
