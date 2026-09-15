package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joangrigorov/go-refactor-mcp/internal/refactor"
)

func TestGoWorkCrossModuleRenameSymbol(t *testing.T) {
	tempDir := t.TempDir()

	// 1. Create moduleA
	modADir := filepath.Join(tempDir, "moduleA")
	_ = os.MkdirAll(filepath.Join(modADir, "user"), 0750)
	_ = os.WriteFile(filepath.Join(modADir, "go.mod"), []byte("module example.com/modA\n\ngo 1.22.0\n"), 0600)

	userFile := filepath.Join(modADir, "user", "user.go")
	userCode := `package user

type UserProfile struct {
	ID string
}
`
	_ = os.WriteFile(userFile, []byte(userCode), 0600)

	// 2. Create moduleB (dependent on moduleA)
	modBDir := filepath.Join(tempDir, "moduleB")
	_ = os.MkdirAll(filepath.Join(modBDir, "service"), 0750)
	_ = os.WriteFile(filepath.Join(modBDir, "go.mod"), []byte("module example.com/modB\n\ngo 1.22.0\n"), 0600)

	serviceFile := filepath.Join(modBDir, "service", "service.go")
	serviceCode := `package service

import "example.com/modA/user"

func GetProfile() user.UserProfile {
	return user.UserProfile{ID: "usr_1"}
}
`
	_ = os.WriteFile(serviceFile, []byte(serviceCode), 0600)

	// 3. Create go.work tying both modules together
	goWork := `go 1.22.0

use (
	./moduleA
	./moduleB
)
`
	_ = os.WriteFile(filepath.Join(tempDir, "go.work"), []byte(goWork), 0600)

	// 4. Rename UserProfile -> AccountProfile targeting workspace root
	opts := refactor.RenameOptions{
		Dir:  tempDir,
		File: userFile,
		From: "UserProfile",
		To:   "AccountProfile",
	}

	if err := refactor.RenameSymbol(opts); err != nil {
		t.Fatalf("RenameSymbol in go.work workspace failed: %v", err)
	}

	// 5. Verify moduleA definition was renamed
	modABytes, err := os.ReadFile(userFile) //nolint:gosec
	if err != nil {
		t.Fatalf("failed reading userFile: %v", err)
	}
	if !strings.Contains(string(modABytes), "type AccountProfile struct") {
		t.Errorf("expected moduleA to contain 'type AccountProfile struct', got:\n%s", string(modABytes))
	}

	// 6. Verify moduleB cross-module reference was updated
	modBBytes, err := os.ReadFile(serviceFile) //nolint:gosec
	if err != nil {
		t.Fatalf("failed reading serviceFile: %v", err)
	}
	if !strings.Contains(string(modBBytes), "user.AccountProfile") {
		t.Errorf("expected moduleB to contain 'user.AccountProfile', got:\n%s", string(modBBytes))
	}
}
