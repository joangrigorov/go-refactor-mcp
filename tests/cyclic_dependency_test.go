package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joangrigorov/go-refactor-mcp/internal/refactor"
)

func TestCyclicDependencyPrevention(t *testing.T) {
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module example.com/cyclemod\n\ngo 1.22.0\n"), 0600)

	// pkgC imports pkgB
	pkgCDir := filepath.Join(tempDir, "pkgC")
	_ = os.MkdirAll(pkgCDir, 0750)
	codeC := `package pkgC

import "example.com/cyclemod/pkgB"

func CallBFromC() {
	pkgB.DoB()
}
`
	_ = os.WriteFile(filepath.Join(pkgCDir, "c.go"), []byte(codeC), 0600)

	// pkgA imports pkgC
	pkgADir := filepath.Join(tempDir, "pkgA")
	_ = os.MkdirAll(pkgADir, 0750)
	codeA := `package pkgA

import "example.com/cyclemod/pkgC"

func CallC() {
	pkgC.CallBFromC()
}
`
	fileA := filepath.Join(pkgADir, "a.go")
	_ = os.WriteFile(fileA, []byte(codeA), 0600)

	// pkgB defines DoB
	pkgBDir := filepath.Join(tempDir, "pkgB")
	_ = os.MkdirAll(pkgBDir, 0750)
	codeB := `package pkgB

func DoB() {}
`
	fileB := filepath.Join(pkgBDir, "b.go")
	_ = os.WriteFile(fileB, []byte(codeB), 0600)

	// Moving a.go (which imports pkgC) into pkgB would make pkgB import pkgC (which imports pkgB), creating a cycle
	opts := refactor.MoveFileOptions{
		Dir:        tempDir,
		SourceFile: fileA,
		DestDir:    pkgBDir,
	}

	err := refactor.MoveFile(opts)
	if err == nil {
		t.Fatalf("expected MoveFile to fail with cyclic dependency error, but got nil")
	}

	if !strings.Contains(err.Error(), "cyclic dependency detected") {
		t.Errorf("expected cyclic dependency error message, got: %v", err)
	}

	// Verify fileA was NOT deleted or corrupted
	if _, statErr := os.Stat(fileA); statErr != nil {
		t.Errorf("fileA should not have been moved or deleted after cycle detection: %v", statErr)
	}
}
