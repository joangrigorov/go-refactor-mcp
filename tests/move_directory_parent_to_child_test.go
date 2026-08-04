package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joangrigorov/go-refactor-mcp/internal/refactor"
)

func TestMoveDirectoryParentToChild(t *testing.T) {
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module example.com/parentchild\n\ngo 1.22.0\n"), 0600)

	sagasDir := filepath.Join(tempDir, "sagas")
	_ = os.MkdirAll(sagasDir, 0750)

	file1Content := `package sagas

func ProcessSaga() string {
	return "saga_done"
}
`
	_ = os.WriteFile(filepath.Join(sagasDir, "saga.go"), []byte(file1Content), 0600)

	mainDir := filepath.Join(tempDir, "mainpkg")
	_ = os.MkdirAll(mainDir, 0750)

	mainContent := `package main

import (
	"fmt"
	"example.com/parentchild/sagas"
)

func main() {
	fmt.Println(sagas.ProcessSaga())
}
`
	_ = os.WriteFile(filepath.Join(mainDir, "main.go"), []byte(mainContent), 0600)

	destDir := filepath.Join(sagasDir, "order_payment")

	opts := refactor.MoveDirOptions{
		SourceDir: sagasDir,
		DestDir:   destDir,
	}

	err := refactor.MoveDirectory(opts)
	if err != nil {
		t.Fatalf("expected MoveDirectory to succeed, got error: %v", err)
	}

	// Verify moved file exists in destination
	movedFilePath := filepath.Join(destDir, "saga.go")
	contentBytes, readErr := os.ReadFile(movedFilePath)
	if readErr != nil {
		t.Fatalf("failed to read moved file at %s: %v", movedFilePath, readErr)
	}

	if !strings.Contains(string(contentBytes), "package order_payment") {
		t.Errorf("expected package clause 'package order_payment', got content:\n%s", string(contentBytes))
	}

	// Verify workspace import update in main.go
	mainBytes, readErr := os.ReadFile(filepath.Join(mainDir, "main.go"))
	if readErr != nil {
		t.Fatalf("failed to read main.go: %v", readErr)
	}

	if !strings.Contains(string(mainBytes), `"example.com/parentchild/sagas/order_payment"`) {
		t.Errorf("expected main.go to import updated path 'example.com/parentchild/sagas/order_payment', got:\n%s", string(mainBytes))
	}
}
