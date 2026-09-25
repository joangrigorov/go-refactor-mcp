package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joangrigorov/go-refactor-mcp/internal/refactor"
)

func TestMoveDirectoryPreservesPackageMain(t *testing.T) {
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module example.com/cmdmod\n\ngo 1.22.0\n"), 0600)

	cmdDir := filepath.Join(tempDir, "cmd", "server")
	_ = os.MkdirAll(cmdDir, 0750)

	mainCode := `package main

import "fmt"

func main() {
	fmt.Println("server running")
}
`
	mainFile := filepath.Join(cmdDir, "main.go")
	_ = os.WriteFile(mainFile, []byte(mainCode), 0600)

	destDir := filepath.Join(tempDir, "cmd", "api_gateway")

	opts := refactor.MoveDirOptions{
		Dir:       tempDir,
		SourceDir: cmdDir,
		DestDir:   destDir,
	}

	if err := refactor.MoveDirectory(opts); err != nil {
		t.Fatalf("MoveDirectory failed: %v", err)
	}

	movedContent, err := os.ReadFile(filepath.Join(destDir, "main.go")) //nolint:gosec
	if err != nil {
		t.Fatalf("failed reading moved main.go: %v", err)
	}

	if !strings.HasPrefix(string(movedContent), "package main") {
		t.Errorf("expected moved command file to preserve 'package main', got:\n%s", string(movedContent))
	}
}
