package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joangrigorov/go-refactor-mcp/internal/refactor"
	"github.com/joangrigorov/go-refactor-mcp/internal/server"
)

func TestMoveDirectory_DetectsTransitiveCyclicDependency(t *testing.T) {
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module example.com/dircycle\n\ngo 1.22.0\n"), 0600)

	// pkgB defines DoB
	pkgBDir := filepath.Join(tempDir, "pkgB")
	_ = os.MkdirAll(pkgBDir, 0750)
	_ = os.WriteFile(filepath.Join(pkgBDir, "b.go"), []byte("package pkgB\n\nfunc DoB() {}\n"), 0600)

	// pkgC imports pkgB
	pkgCDir := filepath.Join(tempDir, "pkgC")
	_ = os.MkdirAll(pkgCDir, 0750)
	codeC := `package pkgC

import "example.com/dircycle/pkgB"

func CallB() {
	pkgB.DoB()
}
`
	_ = os.WriteFile(filepath.Join(pkgCDir, "c.go"), []byte(codeC), 0600)

	// pkgA imports pkgC
	pkgADir := filepath.Join(tempDir, "pkgA")
	_ = os.MkdirAll(pkgADir, 0750)
	codeA := `package pkgA

import "example.com/dircycle/pkgC"

func CallC() {
	pkgC.CallB()
}
`
	_ = os.WriteFile(filepath.Join(pkgADir, "a.go"), []byte(codeA), 0600)

	verifyCompilation(t, tempDir)

	// Moving pkgA into pkgB: pkgB would import pkgC, while pkgC imports pkgB -> cycle!
	opts := refactor.MoveDirOptions{
		Dir:       tempDir,
		SourceDir: pkgADir,
		DestDir:   pkgBDir,
	}

	err := refactor.MoveDirectory(opts)
	if err == nil {
		t.Fatalf("expected MoveDirectory to fail with cyclic dependency error, got nil")
	}

	if !strings.Contains(err.Error(), "cyclic dependency detected") {
		t.Errorf("expected error to contain 'cyclic dependency detected', got: %v", err)
	}

	// Verify source files are untouched and compilation remains intact
	if _, statErr := os.Stat(filepath.Join(pkgADir, "a.go")); statErr != nil {
		t.Errorf("pkgA/a.go should not have been moved or removed: %v", statErr)
	}
	verifyCompilation(t, tempDir)
}

func TestMoveDirectory_DetectsDirectSelfDependency(t *testing.T) {
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module example.com/dirselfcycle\n\ngo 1.22.0\n"), 0600)

	// target package
	targetDir := filepath.Join(tempDir, "targetpkg")
	_ = os.MkdirAll(targetDir, 0750)
	_ = os.WriteFile(filepath.Join(targetDir, "target.go"), []byte("package targetpkg\n\nfunc TargetFunc() {}\n"), 0600)

	// source package directly imports targetpkg
	sourceDir := filepath.Join(tempDir, "sourcepkg")
	_ = os.MkdirAll(sourceDir, 0750)
	codeSource := `package sourcepkg

import "example.com/dirselfcycle/targetpkg"

func UseTarget() {
	targetpkg.TargetFunc()
}
`
	_ = os.WriteFile(filepath.Join(sourceDir, "source.go"), []byte(codeSource), 0600)

	verifyCompilation(t, tempDir)

	opts := refactor.MoveDirOptions{
		Dir:       tempDir,
		SourceDir: sourceDir,
		DestDir:   targetDir,
	}

	err := refactor.MoveDirectory(opts)
	if err == nil {
		t.Fatalf("expected MoveDirectory to fail with cyclic dependency error, got nil")
	}

	if !strings.Contains(err.Error(), "cyclic dependency detected") {
		t.Errorf("expected cyclic dependency error, got: %v", err)
	}

	verifyCompilation(t, tempDir)
}

func TestMoveDirectory_DetectsDestinationAlreadyImportsSource(t *testing.T) {
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module example.com/dirdestdep\n\ngo 1.22.0\n"), 0600)

	// source package
	sourceDir := filepath.Join(tempDir, "service")
	_ = os.MkdirAll(sourceDir, 0750)
	_ = os.WriteFile(filepath.Join(sourceDir, "service.go"), []byte("package service\n\nfunc Run() {}\n"), 0600)

	// dest package imports source package
	destDir := filepath.Join(tempDir, "consumer")
	_ = os.MkdirAll(destDir, 0750)
	codeDest := `package consumer

import "example.com/dirdestdep/service"

func Consume() {
	service.Run()
}
`
	_ = os.WriteFile(filepath.Join(destDir, "consumer.go"), []byte(codeDest), 0600)

	verifyCompilation(t, tempDir)

	opts := refactor.MoveDirOptions{
		Dir:       tempDir,
		SourceDir: sourceDir,
		DestDir:   destDir,
	}

	err := refactor.MoveDirectory(opts)
	if err == nil {
		t.Fatalf("expected MoveDirectory to fail with cyclic dependency error, got nil")
	}

	if !strings.Contains(err.Error(), "cyclic dependency detected") {
		t.Errorf("expected cyclic dependency error, got: %v", err)
	}

	verifyCompilation(t, tempDir)
}

func TestMoveDirectory_MCPTool_CyclicDependencyError(t *testing.T) {
	s := server.NewServer()
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module example.com/mcpcyelemove\n\ngo 1.22.0\n"), 0600)

	pkgBDir := filepath.Join(tempDir, "pkgB")
	_ = os.MkdirAll(pkgBDir, 0750)
	_ = os.WriteFile(filepath.Join(pkgBDir, "b.go"), []byte("package pkgB\n\nfunc DoB() {}\n"), 0600)

	pkgCDir := filepath.Join(tempDir, "pkgC")
	_ = os.MkdirAll(pkgCDir, 0750)
	_ = os.WriteFile(filepath.Join(pkgCDir, "c.go"), []byte("package pkgC\n\nimport \"example.com/mcpcyelemove/pkgB\"\n\nfunc CallB() { pkgB.DoB() }\n"), 0600)

	pkgADir := filepath.Join(tempDir, "pkgA")
	_ = os.MkdirAll(pkgADir, 0750)
	_ = os.WriteFile(filepath.Join(pkgADir, "a.go"), []byte("package pkgA\n\nimport \"example.com/mcpcyelemove/pkgC\"\n\nfunc CallC() { pkgC.CallB() }\n"), 0600)

	res := callMCPTool(t, s, "move_directory", map[string]any{
		"directory":  tempDir,
		"source_dir": pkgADir,
		"dest_dir":   pkgBDir,
	})

	if !res.IsError {
		t.Fatalf("expected MCP call to return error result, got success: %s", getResultText(t, res))
	}

	text := getResultText(t, res)
	if !strings.Contains(text, "cyclic dependency detected") {
		t.Errorf("expected cyclic dependency error in MCP result, got: %s", text)
	}

	verifyCompilation(t, tempDir)
}
