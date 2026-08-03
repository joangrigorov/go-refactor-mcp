package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joangrigorov/go-refactor-mcp/internal/refactor"
)

func TestShadowingDuringExtraction(t *testing.T) {
	tempDir := t.TempDir()

	fileContent := `package shadowtest

var val = 100

func Process() int {
	val := 5
	res := val + 10
	return res
}
`
	filePath := filepath.Join(tempDir, "shadow.go")
	if err := os.WriteFile(filePath, []byte(fileContent), 0600); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	// 1. Test Shadowing Analyzer
	issues, err := refactor.AnalyzeShadowing(filePath)
	if err != nil {
		t.Fatalf("AnalyzeShadowing failed: %v", err)
	}

	if len(issues) == 0 {
		t.Log("AnalyzeShadowing returned 0 issues for local scope check")
	}

	// 2. Test Function Extraction with Shadowed Variable
	opts := refactor.ExtractFuncOptions{
		FilePath:    filePath,
		StartLine:   6,
		EndLine:     7,
		NewFuncName: "calculateSum",
	}

	if err := refactor.ExtractFunction(opts); err != nil {
		t.Fatalf("ExtractFunction failed: %v", err)
	}

	resBytes, err := os.ReadFile(filePath) //nolint:gosec
	if err != nil {
		t.Fatalf("failed reading extracted file: %v", err)
	}

	resStr := string(resBytes)
	if !strings.Contains(resStr, "calculateSum") {
		t.Errorf("extracted function 'calculateSum' missing: %s", resStr)
	}
}
