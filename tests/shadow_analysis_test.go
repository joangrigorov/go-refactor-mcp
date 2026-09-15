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
	val := 5 // shadows package-level val
	res := val + 10

	if true {
		res := 20 // shadows local res
		_ = res
	}

	// Sibling block: declaring 's' here should not shadow 's' in another sibling block
	if true {
		s := "block1"
		_ = s
	}
	if true {
		s := "block2"
		_ = s
	}

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

	if len(issues) != 2 {
		t.Fatalf("expected exactly 2 shadow issues (val and res), got %d: %+v", len(issues), issues)
	}

	// Verify val shadowing
	if issues[0].VarName != "val" {
		t.Errorf("expected first issue to be 'val', got %q", issues[0].VarName)
	}
	if issues[1].VarName != "res" {
		t.Errorf("expected second issue to be 'res', got %q", issues[1].VarName)
	}
}
