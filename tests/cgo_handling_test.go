package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joangrigorov/go-refactor-mcp/internal/refactor"
)

func TestCGOHandling(t *testing.T) {
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module example.com/cgomod\n\ngo 1.22.0\n"), 0600)

	cgoCode := `package cgoapp

/*
#include <stdio.h>
#include <stdlib.h>

void printHello() {
    printf("Hello CGO\n");
}
*/
import "C"

type CgoRunner struct{}

func (c *CgoRunner) Execute() {
	C.printHello()
}
`
	filePath := filepath.Join(tempDir, "cgo.go")
	if err := os.WriteFile(filePath, []byte(cgoCode), 0600); err != nil {
		t.Fatalf("failed to write cgo file: %v", err)
	}

	opts := refactor.RenameOptions{
		Dir:  tempDir,
		File: filePath,
		From: "CgoRunner",
		To:   "NativeRunner",
	}

	if err := refactor.RenameSymbol(opts); err != nil {
		t.Fatalf("RenameSymbol failed on CGO file: %v", err)
	}

	resBytes, err := os.ReadFile(filePath) //nolint:gosec
	if err != nil {
		t.Fatalf("failed reading cgo file: %v", err)
	}

	resStr := string(resBytes)
	if !strings.Contains(resStr, "NativeRunner") {
		t.Errorf("CgoRunner was not renamed to NativeRunner: %s", resStr)
	}

	if !strings.Contains(resStr, `#include <stdio.h>`) {
		t.Errorf("cgo preamble comments were lost during refactoring: %s", resStr)
	}
}
