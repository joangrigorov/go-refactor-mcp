package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joangrigorov/go-refactor-mcp/internal/refactor"
)

func TestGoWorkCrossModuleMoveFile(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Create moduleA with math package
	modADir := filepath.Join(tempDir, "moduleA")
	_ = os.MkdirAll(filepath.Join(modADir, "math"), 0750)
	_ = os.WriteFile(filepath.Join(modADir, "go.mod"), []byte("module example.com/modA\n\ngo 1.22.0\n"), 0600)

	calcFile := filepath.Join(modADir, "math", "calc.go")
	calcCode := `package math

type Calculator struct {
	Precision int
}
`
	_ = os.WriteFile(calcFile, []byte(calcCode), 0600)

	// 2. Create moduleB with app importing moduleA/math
	modBDir := filepath.Join(tempDir, "moduleB")
	_ = os.MkdirAll(filepath.Join(modBDir, "app"), 0750)
	_ = os.WriteFile(filepath.Join(modBDir, "go.mod"), []byte("module example.com/modB\n\ngo 1.22.0\n"), 0600)

	appFile := filepath.Join(modBDir, "app", "main.go")
	appCode := `package app

import "example.com/modA/math"

func Run() math.Calculator {
	return math.Calculator{Precision: 2}
}
`
	_ = os.WriteFile(appFile, []byte(appCode), 0600)

	// 3. Create go.work
	goWork := `go 1.22.0

use (
	./moduleA
	./moduleB
)
`
	_ = os.WriteFile(filepath.Join(tempDir, "go.work"), []byte(goWork), 0600)

	// 4. Move calc.go from moduleA/math to moduleB/engine
	destDir := filepath.Join(modBDir, "engine")
	opts := refactor.MoveFileOptions{
		Dir:        tempDir,
		SourceFile: calcFile,
		DestDir:    destDir,
	}

	if err := refactor.MoveFile(opts); err != nil {
		t.Fatalf("MoveFile across modules failed: %v", err)
	}

	// 5. Verify moved file exists in moduleB/engine with package engine
	movedFile := filepath.Join(destDir, "calc.go")
	movedBytes, err := os.ReadFile(movedFile) //nolint:gosec
	if err != nil {
		t.Fatalf("failed reading moved file in moduleB: %v", err)
	}
	if !strings.HasPrefix(string(movedBytes), "package engine") {
		t.Errorf("expected moved file package 'package engine', got:\n%s", string(movedBytes))
	}

	// 6. Verify moduleB/app/main.go updated import to point to example.com/modB/engine
	appBytes, err := os.ReadFile(appFile) //nolint:gosec
	if err != nil {
		t.Fatalf("failed reading app file: %v", err)
	}
	appStr := string(appBytes)
	if !strings.Contains(appStr, `"example.com/modB/engine"`) {
		t.Errorf("expected app to import 'example.com/modB/engine', got:\n%s", appStr)
	}
	if !strings.Contains(appStr, "engine.Calculator") {
		t.Errorf("expected app selector to be 'engine.Calculator', got:\n%s", appStr)
	}
}
