package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joangrigorov/go-refactor-mcp/internal/refactor"
)

func TestMoveFileWithBuildTags(t *testing.T) {
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module example.com/tagsmod\n\ngo 1.22.0\n"), 0600)

	sourceDir := filepath.Join(tempDir, "src")
	destDir := filepath.Join(tempDir, "dest")
	_ = os.MkdirAll(sourceDir, 0750)

	srcFile := filepath.Join(sourceDir, "sys_linux.go")
	codeWithTags := `//go:build linux && amd64
// +build linux,amd64

package src

func SysCall() int {
	return 42
}
`
	if err := os.WriteFile(srcFile, []byte(codeWithTags), 0600); err != nil {
		t.Fatalf("failed to write source file: %v", err)
	}

	opts := refactor.MoveFileOptions{
		SourceFile: srcFile,
		DestDir:    destDir,
	}

	if err := refactor.MoveFile(opts); err != nil {
		t.Fatalf("MoveFile failed: %v", err)
	}

	movedFile := filepath.Join(destDir, "sys_linux.go")
	resBytes, err := os.ReadFile(movedFile) //nolint:gosec
	if err != nil {
		t.Fatalf("failed to read moved file: %v", err)
	}

	resStr := string(resBytes)

	if !strings.Contains(resStr, "//go:build linux && amd64") {
		t.Errorf("build tag //go:build linux && amd64 missing: %s", resStr)
	}

	if !strings.Contains(resStr, "package dest") {
		t.Errorf("package declaration not updated to 'package dest': %s", resStr)
	}

	// Verify build tags are located before package declaration
	pkgIdx := strings.Index(resStr, "package dest")
	tagIdx := strings.Index(resStr, "//go:build linux && amd64")

	if tagIdx >= pkgIdx {
		t.Errorf("build tags must be positioned above package declaration: %s", resStr)
	}
}
