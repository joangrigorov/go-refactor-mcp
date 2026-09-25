package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joangrigorov/go-refactor-mcp/internal/refactor"
)

func TestMoveFileRenameInPlace(t *testing.T) {
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module example.com/renametest\n\ngo 1.22.0\n"), 0600)

	sagasDir := filepath.Join(tempDir, "sagas")
	_ = os.MkdirAll(sagasDir, 0750)

	fileSaga := filepath.Join(sagasDir, "order_payment_saga.go")
	_ = os.WriteFile(fileSaga, []byte(`package sagas

func RunSaga() string {
	return "ok"
}
`), 0600)

	fileSagaTest := filepath.Join(sagasDir, "order_payment_saga_test.go")
	_ = os.WriteFile(fileSagaTest, []byte(`package sagas

import "testing"

func TestRunSaga(t *testing.T) {
	if RunSaga() != "ok" {
		t.Fail()
	}
}
`), 0600)

	// In-place rename order_payment_saga.go to saga.go
	err := refactor.MoveFile(refactor.MoveFileOptions{
		Dir:        tempDir,
		SourceFile: fileSaga,
		NewName:    "saga.go",
	})
	if err != nil {
		t.Fatalf("expected MoveFile (in-place rename) to succeed, got error: %v", err)
	}

	// Verify original files no longer exist
	if _, err := os.Stat(fileSaga); err == nil {
		t.Errorf("expected original file %s to be removed", fileSaga)
	}
	if _, err := os.Stat(fileSagaTest); err == nil {
		t.Errorf("expected original test file %s to be removed", fileSagaTest)
	}

	// Verify renamed files exist
	renamedFile := filepath.Join(sagasDir, "saga.go")
	contentBytes, readErr := os.ReadFile(renamedFile) // #nosec G304
	if readErr != nil {
		t.Fatalf("failed reading renamed file %s: %v", renamedFile, readErr)
	}
	if !strings.Contains(string(contentBytes), "package sagas") {
		t.Errorf("expected renamed file to keep package sagas, got:\n%s", string(contentBytes))
	}

	renamedTestFile := filepath.Join(sagasDir, "saga_test.go")
	testBytes, readErr := os.ReadFile(renamedTestFile) // #nosec G304
	if readErr != nil {
		t.Fatalf("failed reading renamed test file %s: %v", renamedTestFile, readErr)
	}
	if !strings.Contains(string(testBytes), "TestRunSaga") {
		t.Errorf("expected test file content preserved, got:\n%s", string(testBytes))
	}
}

func TestMoveFileMoveAndRenameSimultaneously(t *testing.T) {
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module example.com/moverenametest\n\ngo 1.22.0\n"), 0600)

	srcDir := filepath.Join(tempDir, "pkgA")
	_ = os.MkdirAll(srcDir, 0750)

	srcFile := filepath.Join(srcDir, "old_name.go")
	_ = os.WriteFile(srcFile, []byte(`package pkgA

func Hello() string { return "world" }
`), 0600)

	srcTestFile := filepath.Join(srcDir, "old_name_test.go")
	_ = os.WriteFile(srcTestFile, []byte(`package pkgA

import "testing"

func TestHello(t *testing.T) {}
`), 0600)

	destDir := filepath.Join(tempDir, "pkgB")

	// Move AND rename simultaneously
	err := refactor.MoveFile(refactor.MoveFileOptions{
		Dir:        tempDir,
		SourceFile: srcFile,
		DestDir:    destDir,
		NewName:    "new_name.go",
	})
	if err != nil {
		t.Fatalf("expected MoveFile (move+rename) to succeed, got error: %v", err)
	}

	// Verify moved & renamed files exist in pkgB
	destFile := filepath.Join(destDir, "new_name.go")
	contentBytes, readErr := os.ReadFile(destFile) // #nosec G304
	if readErr != nil {
		t.Fatalf("failed reading moved & renamed file %s: %v", destFile, readErr)
	}
	if !strings.Contains(string(contentBytes), "package pkgB") {
		t.Errorf("expected package clause 'package pkgB', got:\n%s", string(contentBytes))
	}

	destTestFile := filepath.Join(destDir, "new_name_test.go")
	testBytes, readErr := os.ReadFile(destTestFile) // #nosec G304
	if readErr != nil {
		t.Fatalf("failed reading moved & renamed test file %s: %v", destTestFile, readErr)
	}
	if !strings.Contains(string(testBytes), "package pkgB") {
		t.Errorf("expected package clause 'package pkgB' in test file, got:\n%s", string(testBytes))
	}
}
