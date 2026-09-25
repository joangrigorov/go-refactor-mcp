package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joangrigorov/go-refactor-mcp/internal/refactor"
)

func TestStandaloneModuleDoesNotClimbToParentOrSiblings(t *testing.T) {
	parentDir := t.TempDir()

	projectA := filepath.Join(parentDir, "project-a")
	_ = os.MkdirAll(projectA, 0750)
	_ = os.WriteFile(filepath.Join(projectA, "go.mod"), []byte("module example.com/projectA\n\ngo 1.22.0\n"), 0600)

	projectB := filepath.Join(parentDir, "project-b")
	subPkg := filepath.Join(projectB, "pkg", "foo")
	_ = os.MkdirAll(subPkg, 0750)
	_ = os.WriteFile(filepath.Join(projectB, "go.mod"), []byte("module example.com/projectB\n\ngo 1.22.0\n"), 0600)

	// 1. Pointing directly at standalone module project-b
	ws, err := refactor.FindWorkspace(projectB)
	if err != nil {
		t.Fatalf("FindWorkspace failed for project-b: %v", err)
	}
	if len(ws.Modules) != 1 {
		t.Fatalf("expected exactly 1 module in standalone workspace, got %d", len(ws.Modules))
	}
	if ws.Modules[0].Root != projectB {
		t.Errorf("expected module root %s, got %s", projectB, ws.Modules[0].Root)
	}

	// 2. Pointing at a subpackage inside project-b
	wsSub, err := refactor.FindWorkspace(subPkg)
	if err != nil {
		t.Fatalf("FindWorkspace failed for subpackage in project-b: %v", err)
	}
	if len(wsSub.Modules) != 1 {
		t.Fatalf("expected exactly 1 module for subpackage workspace, got %d", len(wsSub.Modules))
	}
	if wsSub.Modules[0].Root != projectB {
		t.Errorf("expected module root %s, got %s", projectB, wsSub.Modules[0].Root)
	}
}

func TestDiscoverSubModulesNeverCrossesGitBoundaries(t *testing.T) {
	parentDir := t.TempDir()

	// Siblings with .git directories or git submodule files
	repoA := filepath.Join(parentDir, "repo-a")
	_ = os.MkdirAll(filepath.Join(repoA, ".git"), 0750)
	_ = os.WriteFile(filepath.Join(repoA, "go.mod"), []byte("module example.com/repoA\n\ngo 1.22.0\n"), 0600)

	repoB := filepath.Join(parentDir, "repo-b")
	_ = os.MkdirAll(repoB, 0750)
	// Git worktree or submodule .git file
	_ = os.WriteFile(filepath.Join(repoB, ".git"), []byte("gitdir: /path/to/gitdir\n"), 0600)
	_ = os.WriteFile(filepath.Join(repoB, "go.mod"), []byte("module example.com/repoB\n\ngo 1.22.0\n"), 0600)

	// Invoking FindWorkspace on the parent directory containing sibling git repos
	ws, err := refactor.FindWorkspace(parentDir)
	if err == nil {
		t.Fatalf("expected error from FindWorkspace on parentDir of sibling git repos, but got workspace with %d modules", len(ws.Modules))
	}
}

func TestMonorepoWithGitRootAndSubmodules(t *testing.T) {
	monoDir := t.TempDir()

	// Monorepo git root
	_ = os.MkdirAll(filepath.Join(monoDir, ".git"), 0750)

	// Legitimate monorepo sub-modules
	authDir := filepath.Join(monoDir, "services", "auth")
	_ = os.MkdirAll(authDir, 0750)
	_ = os.WriteFile(filepath.Join(authDir, "go.mod"), []byte("module mono.com/auth\n\ngo 1.22.0\n"), 0600)

	gwDir := filepath.Join(monoDir, "services", "gateway")
	_ = os.MkdirAll(gwDir, 0750)
	_ = os.WriteFile(filepath.Join(gwDir, "go.mod"), []byte("module mono.com/gateway\n\ngo 1.22.0\n"), 0600)

	// Third-party git submodule inside monorepo tree
	submoduleDir := filepath.Join(monoDir, "vendor_repo")
	_ = os.MkdirAll(filepath.Join(submoduleDir, ".git"), 0750)
	_ = os.WriteFile(filepath.Join(submoduleDir, "go.mod"), []byte("module vendor.com/ext\n\ngo 1.22.0\n"), 0600)

	// 1. Invoking on monorepo root should discover auth and gateway, but skip submodule
	ws, err := refactor.FindWorkspace(monoDir)
	if err != nil {
		t.Fatalf("FindWorkspace failed on monorepo root: %v", err)
	}
	if len(ws.Modules) != 2 {
		t.Fatalf("expected 2 discovered modules in monorepo, got %d", len(ws.Modules))
	}
	for _, mod := range ws.Modules {
		if mod.Root == submoduleDir {
			t.Errorf("discoverSubModules should have skipped git submodule %s", submoduleDir)
		}
	}

	// 2. Invoking on authDir directly must NOT climb out to monorepo root
	wsAuth, err := refactor.FindWorkspace(authDir)
	if err != nil {
		t.Fatalf("FindWorkspace failed on authDir: %v", err)
	}
	if len(wsAuth.Modules) != 1 {
		t.Fatalf("expected 1 module for standalone module query in monorepo subfolder, got %d", len(wsAuth.Modules))
	}
	if wsAuth.Modules[0].Root != authDir {
		t.Errorf("expected module root %s, got %s", authDir, wsAuth.Modules[0].Root)
	}
}

func TestRenameSymbolIsolationBetweenSiblingRepos(t *testing.T) {
	parentDir := t.TempDir()

	projectA := filepath.Join(parentDir, "project-a")
	_ = os.MkdirAll(filepath.Join(projectA, ".git"), 0750)
	_ = os.MkdirAll(filepath.Join(projectA, "pkg"), 0750)
	_ = os.WriteFile(filepath.Join(projectA, "go.mod"), []byte("module example.com/projectA\n\ngo 1.22.0\n"), 0600)
	paFile := filepath.Join(projectA, "pkg", "server.go")
	paContent := `package pkg

type ServerConfig struct {
	Port int
}
`
	_ = os.WriteFile(paFile, []byte(paContent), 0600)

	projectB := filepath.Join(parentDir, "project-b")
	_ = os.MkdirAll(filepath.Join(projectB, ".git"), 0750)
	_ = os.MkdirAll(filepath.Join(projectB, "pkg"), 0750)
	_ = os.WriteFile(filepath.Join(projectB, "go.mod"), []byte("module example.com/projectB\n\ngo 1.22.0\n"), 0600)
	pbFile := filepath.Join(projectB, "pkg", "server.go")
	pbContent := `package pkg

type ServerConfig struct {
	Port int
}
`
	_ = os.WriteFile(pbFile, []byte(pbContent), 0600)

	// Rename ServerConfig in projectB to CustomServerConfig
	opts := refactor.RenameOptions{
		Dir:  projectB,
		From: "ServerConfig",
		To:   "CustomServerConfig",
	}

	if err := refactor.RenameSymbol(opts); err != nil {
		t.Fatalf("RenameSymbol in project-b failed: %v", err)
	}

	// Verify project-b was updated
	pbUpdated, err := os.ReadFile(pbFile) //nolint:gosec
	if err != nil {
		t.Fatalf("failed reading project-b file: %v", err)
	}
	if !strings.Contains(string(pbUpdated), "type CustomServerConfig struct") {
		t.Errorf("project-b was not updated with CustomServerConfig:\n%s", string(pbUpdated))
	}

	// Verify project-a was NOT modified
	paAfter, err := os.ReadFile(paFile) //nolint:gosec
	if err != nil {
		t.Fatalf("failed reading project-a file: %v", err)
	}
	if string(paAfter) != paContent {
		t.Errorf("project-a was modified unexpectedly! Content:\n%s", string(paAfter))
	}
}
