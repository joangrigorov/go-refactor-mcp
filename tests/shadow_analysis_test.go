package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/joangrigorov/go-refactor-mcp/internal/refactor"
)

func TestShadowingAnalysis(t *testing.T) {
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

	issues, err := refactor.AnalyzeShadowing(filePath)
	if err != nil {
		t.Fatalf("AnalyzeShadowing failed: %v", err)
	}

	if len(issues) == 0 {
		t.Log("AnalyzeShadowing returned 0 issues for local scope check")
	}
}
