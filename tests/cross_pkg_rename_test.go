package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joangrigorov/go-refactor-mcp/internal/refactor"
)

func TestCrossPackageRenaming(t *testing.T) {
	tempDir := t.TempDir()

	// Create go.mod
	goModContent := "module example.com/testmod\n\ngo 1.22.0\n"
	if err := os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte(goModContent), 0600); err != nil {
		t.Fatalf("failed creating go.mod: %v", err)
	}

	// Package pkg1: defines User struct
	pkg1Dir := filepath.Join(tempDir, "pkg1")
	_ = os.MkdirAll(pkg1Dir, 0750)
	pkg1Code := `package pkg1

type User struct {
	Name string
}
`
	if err := os.WriteFile(filepath.Join(pkg1Dir, "user.go"), []byte(pkg1Code), 0600); err != nil {
		t.Fatalf("failed creating user.go: %v", err)
	}

	// Package pkg2: normal import of User
	pkg2Dir := filepath.Join(tempDir, "pkg2")
	_ = os.MkdirAll(pkg2Dir, 0750)
	pkg2Code := `package pkg2

import "example.com/testmod/pkg1"

func GetName(u pkg1.User) string {
	return u.Name
}
`
	if err := os.WriteFile(filepath.Join(pkg2Dir, "service.go"), []byte(pkg2Code), 0600); err != nil {
		t.Fatalf("failed creating service.go: %v", err)
	}

	// Package pkg3: aliased import of pkg1
	pkg3Dir := filepath.Join(tempDir, "pkg3")
	_ = os.MkdirAll(pkg3Dir, 0750)
	pkg3Code := `package pkg3

import p1 "example.com/testmod/pkg1"

func MakeUser() p1.User {
	return p1.User{Name: "Alice"}
}
`
	if err := os.WriteFile(filepath.Join(pkg3Dir, "alias.go"), []byte(pkg3Code), 0600); err != nil {
		t.Fatalf("failed creating alias.go: %v", err)
	}

	// Package pkg4: dot-import of pkg1
	pkg4Dir := filepath.Join(tempDir, "pkg4")
	_ = os.MkdirAll(pkg4Dir, 0750)
	pkg4Code := `package pkg4

import . "example.com/testmod/pkg1"

func ProcessUser(u User) string {
	return u.Name
}
`
	if err := os.WriteFile(filepath.Join(pkg4Dir, "dot.go"), []byte(pkg4Code), 0600); err != nil {
		t.Fatalf("failed creating dot.go: %v", err)
	}

	// Perform Rename of User -> Account
	opts := refactor.RenameOptions{
		Dir:  tempDir,
		File: filepath.Join(pkg1Dir, "user.go"),
		From: "User",
		To:   "Account",
	}

	if err := refactor.RenameSymbol(opts); err != nil {
		t.Fatalf("RenameSymbol failed: %v", err)
	}

	// Assert updates across all packages
	pkg1Res, _ := os.ReadFile(filepath.Join(pkg1Dir, "user.go")) //nolint:gosec
	if !strings.Contains(string(pkg1Res), "type Account struct") {
		t.Errorf("pkg1 user.go missing 'type Account struct': %s", string(pkg1Res))
	}

	pkg2Res, _ := os.ReadFile(filepath.Join(pkg2Dir, "service.go")) //nolint:gosec
	if !strings.Contains(string(pkg2Res), "pkg1.Account") {
		t.Errorf("pkg2 service.go missing 'pkg1.Account': %s", string(pkg2Res))
	}

	pkg3Res, _ := os.ReadFile(filepath.Join(pkg3Dir, "alias.go")) //nolint:gosec
	if !strings.Contains(string(pkg3Res), "p1.Account") {
		t.Errorf("pkg3 alias.go missing 'p1.Account': %s", string(pkg3Res))
	}

	pkg4Res, _ := os.ReadFile(filepath.Join(pkg4Dir, "dot.go")) //nolint:gosec
	if !strings.Contains(string(pkg4Res), "ProcessUser(u Account)") {
		t.Errorf("pkg4 dot.go missing 'ProcessUser(u Account)': %s", string(pkg4Res))
	}
}
