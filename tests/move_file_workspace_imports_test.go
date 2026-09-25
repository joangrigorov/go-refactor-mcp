package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joangrigorov/go-refactor-mcp/internal/refactor"
)

func TestMoveFileWorkspaceImportUpdates(t *testing.T) {
	tempDir := t.TempDir()

	// Write go.mod
	goMod := "module example.com/movefilemod\n\ngo 1.22.0\n"
	_ = os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte(goMod), 0600)

	// pkg/foo/foo.go
	fooDir := filepath.Join(tempDir, "pkg", "foo")
	_ = os.MkdirAll(fooDir, 0750)
	fooFile := filepath.Join(fooDir, "foo.go")
	fooCode := `package foo

type Foo struct {
	ID string
}
`
	_ = os.WriteFile(fooFile, []byte(fooCode), 0600)

	// pkg/bar/bar.go (imports pkg/foo and uses foo.Foo)
	barDir := filepath.Join(tempDir, "pkg", "bar")
	_ = os.MkdirAll(barDir, 0750)
	barFile := filepath.Join(barDir, "bar.go")
	barCode := `package bar

import "example.com/movefilemod/pkg/foo"

func Process(f foo.Foo) string {
	return f.ID
}
`
	_ = os.WriteFile(barFile, []byte(barCode), 0600)

	// Move foo.go to pkg/baz
	bazDir := filepath.Join(tempDir, "pkg", "baz")
	opts := refactor.MoveFileOptions{
		Dir:        tempDir,
		SourceFile: fooFile,
		DestDir:    bazDir,
	}

	if err := refactor.MoveFile(opts); err != nil {
		t.Fatalf("MoveFile failed: %v", err)
	}

	// Verify bar.go import updated from pkg/foo to pkg/baz, and selector updated to baz.Foo
	barRes, err := os.ReadFile(barFile) //nolint:gosec
	if err != nil {
		t.Fatalf("failed reading bar.go: %v", err)
	}

	barStr := string(barRes)

	if !strings.Contains(barStr, `"example.com/movefilemod/pkg/baz"`) {
		t.Errorf("bar.go missing updated import 'example.com/movefilemod/pkg/baz':\n%s", barStr)
	}

	if strings.Contains(barStr, `"example.com/movefilemod/pkg/foo"`) {
		t.Errorf("bar.go should no longer import old package 'example.com/movefilemod/pkg/foo':\n%s", barStr)
	}

	if !strings.Contains(barStr, "baz.Foo") {
		t.Errorf("bar.go selector for Foo was not updated to 'baz.Foo':\n%s", barStr)
	}
}
