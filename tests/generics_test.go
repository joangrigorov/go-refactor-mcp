package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joangrigorov/go-refactor-mcp/internal/refactor"
)

func TestGenericsRenaming(t *testing.T) {
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module example.com/genmod\n\ngo 1.22.0\n"), 0600)

	pkg1Dir := filepath.Join(tempDir, "genpkg")
	_ = os.MkdirAll(pkg1Dir, 0750)

	genCode := `package genpkg

type Container[T any] struct {
	Value T
}

func Identity[T any](item T) T {
	return item
}
`
	genFile := filepath.Join(pkg1Dir, "container.go")
	_ = os.WriteFile(genFile, []byte(genCode), 0600)

	// App package instantiating Container[T]
	appDir := filepath.Join(tempDir, "app")
	_ = os.MkdirAll(appDir, 0750)
	appCode := `package app

import "example.com/genmod/genpkg"

func Run() {
	c := genpkg.Container[string]{Value: "hello"}
	_ = c
}
`
	appFile := filepath.Join(appDir, "main.go")
	_ = os.WriteFile(appFile, []byte(appCode), 0600)

	// 1. Rename generic struct Container -> Box
	optsStruct := refactor.RenameOptions{
		Dir:  tempDir,
		File: genFile,
		From: "Container",
		To:   "Box",
	}

	if err := refactor.RenameSymbol(optsStruct); err != nil {
		t.Fatalf("RenameSymbol for generic struct failed: %v", err)
	}

	resGen, _ := os.ReadFile(genFile) //nolint:gosec
	if !strings.Contains(string(resGen), "type Box[T any] struct") {
		t.Errorf("Container struct not renamed to Box in genFile: %s", string(resGen))
	}

	resApp, _ := os.ReadFile(appFile) //nolint:gosec
	if !strings.Contains(string(resApp), "genpkg.Box[string]") {
		t.Errorf("Container instantiation not updated to Box in appFile: %s", string(resApp))
	}

	// 2. Rename type parameter T -> Element
	optsTypeParam := refactor.RenameOptions{
		Dir:  tempDir,
		File: genFile,
		From: "T",
		To:   "Element",
	}

	if err := refactor.RenameSymbol(optsTypeParam); err != nil {
		t.Fatalf("RenameSymbol for type parameter T failed: %v", err)
	}

	resGenAfter, _ := os.ReadFile(genFile) //nolint:gosec
	if !strings.Contains(string(resGenAfter), "type Box[Element any] struct") {
		t.Errorf("Type parameter T not renamed to Element: %s", string(resGenAfter))
	}
}
