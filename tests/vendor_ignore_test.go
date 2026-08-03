package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joangrigorov/go-refactor-mcp/internal/refactor"
)

func TestVendorFolderIgnoring(t *testing.T) {
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module example.com/vendormod\n\ngo 1.22.0\n"), 0600)

	// Vendor directory code
	vendorDir := filepath.Join(tempDir, "vendor", "dep")
	_ = os.MkdirAll(vendorDir, 0750)
	vendorCode := `package dep

type Config struct {
	Timeout int
}
`
	vendorFile := filepath.Join(vendorDir, "config.go")
	_ = os.WriteFile(vendorFile, []byte(vendorCode), 0600)

	// Main code
	mainDir := filepath.Join(tempDir, "app")
	_ = os.MkdirAll(mainDir, 0750)
	mainCode := `package app

type Config struct {
	Timeout int
}
`
	mainFile := filepath.Join(mainDir, "config.go")
	_ = os.WriteFile(mainFile, []byte(mainCode), 0600)

	// Perform Rename of Config -> Settings
	opts := refactor.RenameOptions{
		Dir:  tempDir,
		File: mainFile,
		From: "Config",
		To:   "Settings",
	}

	if err := refactor.RenameSymbol(opts); err != nil {
		t.Fatalf("RenameSymbol failed: %v", err)
	}

	// Verify main code modified
	mainRes, _ := os.ReadFile(mainFile) //nolint:gosec
	if !strings.Contains(string(mainRes), "Settings") {
		t.Errorf("main code Config was not renamed to Settings: %s", string(mainRes))
	}

	// Verify vendor code was ignored and NOT modified
	vendorRes, _ := os.ReadFile(vendorFile) //nolint:gosec
	if strings.Contains(string(vendorRes), "Settings") {
		t.Errorf("vendor code should NOT have been modified: %s", string(vendorRes))
	}
}
