package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/joangrigorov/go-refactor-mcp/internal/refactor"
)

func TestWorkspaceModfileParsingRobustness(t *testing.T) {
	tempDir := t.TempDir()

	// go.mod with leading comments, quotes, and whitespace
	goMod := `// Leading license comment
// Another comment line
module "example.com/robust/mod"

go 1.22.0
`
	_ = os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte(goMod), 0600)

	ws, err := refactor.FindWorkspace(tempDir)
	if err != nil {
		t.Fatalf("FindWorkspace failed: %v", err)
	}

	if ws.IsWork {
		t.Errorf("expected single module workspace, got go.work")
	}

	if len(ws.Modules) != 1 {
		t.Fatalf("expected 1 module, got %d", len(ws.Modules))
	}

	if ws.Modules[0].Path != "example.com/robust/mod" {
		t.Errorf("expected module path 'example.com/robust/mod', got %q", ws.Modules[0].Path)
	}

	importPath, err := ws.CalculateImportPath(filepath.Join(tempDir, "pkg", "service"))
	if err != nil {
		t.Fatalf("CalculateImportPath failed: %v", err)
	}

	expectedImport := "example.com/robust/mod/pkg/service"
	if importPath != expectedImport {
		t.Errorf("expected import path %q, got %q", expectedImport, importPath)
	}
}

func TestWorkspaceGoWorkDiscovery(t *testing.T) {
	tempDir := t.TempDir()

	// Create module A
	modADir := filepath.Join(tempDir, "moduleA")
	_ = os.MkdirAll(modADir, 0750)
	_ = os.WriteFile(filepath.Join(modADir, "go.mod"), []byte("module example.com/modA\n\ngo 1.22.0\n"), 0600)

	// Create module B
	modBDir := filepath.Join(tempDir, "moduleB")
	_ = os.MkdirAll(modBDir, 0750)
	_ = os.WriteFile(filepath.Join(modBDir, "go.mod"), []byte("module example.com/modB\n\ngo 1.22.0\n"), 0600)

	// Create go.work
	goWork := `go 1.22.0

use (
	./moduleA
	./moduleB
)
`
	_ = os.WriteFile(filepath.Join(tempDir, "go.work"), []byte(goWork), 0600)

	ws, err := refactor.FindWorkspace(filepath.Join(modADir, "subpkg"))
	if err != nil {
		t.Fatalf("FindWorkspace failed in workspace subdir: %v", err)
	}

	if !ws.IsWork {
		t.Errorf("expected ws.IsWork == true")
	}

	if len(ws.Modules) != 2 {
		t.Fatalf("expected 2 workspace modules, got %d", len(ws.Modules))
	}

	importA, err := ws.CalculateImportPath(filepath.Join(modADir, "core"))
	if err != nil {
		t.Fatalf("CalculateImportPath failed for moduleA: %v", err)
	}
	if importA != "example.com/modA/core" {
		t.Errorf("expected 'example.com/modA/core', got %q", importA)
	}

	importB, err := ws.CalculateImportPath(filepath.Join(modBDir, "worker"))
	if err != nil {
		t.Fatalf("CalculateImportPath failed for moduleB: %v", err)
	}
	if importB != "example.com/modB/worker" {
		t.Errorf("expected 'example.com/modB/worker', got %q", importB)
	}
}
