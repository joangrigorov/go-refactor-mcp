package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joangrigorov/go-refactor-mcp/internal/refactor"
)

func TestMoveFileMultiFileBatchDeduplication(t *testing.T) {
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module example.com/batchmod\n\ngo 1.22.0\n"), 0600)

	oldDir := filepath.Join(tempDir, "pkg", "events_old")
	_ = os.MkdirAll(oldDir, 0750)

	file1 := filepath.Join(oldDir, "event1.go")
	_ = os.WriteFile(file1, []byte("package events_old\n\ntype Event1 struct{}\n"), 0600)

	file2 := filepath.Join(oldDir, "event2.go")
	_ = os.WriteFile(file2, []byte("package events_old\n\ntype Event2 struct{}\n"), 0600)

	file3 := filepath.Join(oldDir, "event3.go")
	_ = os.WriteFile(file3, []byte("package events_old\n\ntype Event3 struct{}\n"), 0600)

	appDir := filepath.Join(tempDir, "pkg", "app")
	_ = os.MkdirAll(appDir, 0750)
	appFile := filepath.Join(appDir, "app.go")
	appCode := `package app

import "example.com/batchmod/pkg/events_old"

func Handle(e1 events_old.Event1, e2 events_old.Event2, e3 events_old.Event3) {}
`
	_ = os.WriteFile(appFile, []byte(appCode), 0600)

	destDir := filepath.Join(tempDir, "pkg", "events_v1")

	// Move file1, file2, file3 sequentially to events_v1
	for _, f := range []string{file1, file2, file3} {
		opts := refactor.MoveFileOptions{
			SourceFile: f,
			DestDir:    destDir,
		}
		if err := refactor.MoveFile(opts); err != nil {
			t.Fatalf("MoveFile failed for %s: %v", f, err)
		}
	}

	// Verify app.go content
	appRes, err := os.ReadFile(appFile) //nolint:gosec
	if err != nil {
		t.Fatalf("failed reading app.go: %v", err)
	}

	appStr := string(appRes)
	targetImport := `"example.com/batchmod/pkg/events_v1"`

	count := strings.Count(appStr, targetImport)
	if count != 1 {
		t.Errorf("expected target import %s to appear exactly once, but appeared %d times in:\n%s", targetImport, count, appStr)
	}

	if strings.Contains(appStr, `"example.com/batchmod/pkg/events_old"`) {
		t.Errorf("old import should be removed after all files moved:\n%s", appStr)
	}
}
