package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joangrigorov/go-refactor-mcp/internal/refactor"
)

func TestMoveTestFile(t *testing.T) {
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module example.com/testfilemod\n\ngo 1.22.0\n"), 0600)

	sourceDir := filepath.Join(tempDir, "pkgold")
	destDir := filepath.Join(tempDir, "pkgnew")
	_ = os.MkdirAll(sourceDir, 0750)

	mainFile := filepath.Join(sourceDir, "calc.go")
	testFile := filepath.Join(sourceDir, "calc_test.go")

	_ = os.WriteFile(mainFile, []byte("package pkgold\n\nfunc Add(a, b int) int { return a + b }\n"), 0600)
	_ = os.WriteFile(testFile, []byte("package pkgold_test\n\nimport (\n\t\"testing\"\n)\n\nfunc TestAdd(t *testing.T) {}\n"), 0600)

	opts := refactor.MoveFileOptions{
		Dir:        tempDir,
		SourceFile: mainFile,
		DestDir:    destDir,
	}

	if err := refactor.MoveFile(opts); err != nil {
		t.Fatalf("MoveFile failed: %v", err)
	}

	// Verify both calc.go and calc_test.go moved
	movedMain := filepath.Join(destDir, "calc.go")
	movedTest := filepath.Join(destDir, "calc_test.go")

	mainBytes, err := os.ReadFile(movedMain) //nolint:gosec
	if err != nil {
		t.Fatalf("moved main file missing: %v", err)
	}
	testBytes, err := os.ReadFile(movedTest) //nolint:gosec
	if err != nil {
		t.Fatalf("moved test file missing: %v", err)
	}

	if !strings.Contains(string(mainBytes), "package pkgnew") {
		t.Errorf("main file package should be 'pkgnew': %s", string(mainBytes))
	}

	if !strings.Contains(string(testBytes), "package pkgnew_test") {
		t.Errorf("test file package should be 'pkgnew_test': %s", string(testBytes))
	}
}
