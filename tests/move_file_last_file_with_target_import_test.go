package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joangrigorov/go-refactor-mcp/internal/refactor"
)

func TestMoveFileLastFileImportingTargetPackage(t *testing.T) {
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module example.com/seqmove\n\ngo 1.22.0\n"), 0600)

	sagasDir := filepath.Join(tempDir, "sagas")
	_ = os.MkdirAll(sagasDir, 0750)

	fileSaga := filepath.Join(sagasDir, "order_payment_saga.go")
	_ = os.WriteFile(fileSaga, []byte(`package sagas

type OrderSaga struct{}
`), 0600)

	fileOutput := filepath.Join(sagasDir, "output.go")
	_ = os.WriteFile(fileOutput, []byte(`package sagas

type SagaOutput struct{}
`), 0600)

	filePorts := filepath.Join(sagasDir, "ports.go")
	_ = os.WriteFile(filePorts, []byte(`package sagas

type OrderPort interface {
	Exec(s OrderSaga, o SagaOutput)
}
`), 0600)

	destDir := filepath.Join(sagasDir, "order_payment")

	// Step 1: Move order_payment_saga.go to sagas/order_payment
	err := refactor.MoveFile(refactor.MoveFileOptions{
		SourceFile: fileSaga,
		DestDir:    destDir,
	})
	if err != nil {
		t.Fatalf("Step 1 failed: %v", err)
	}

	// Step 2: Move output.go to sagas/order_payment
	err = refactor.MoveFile(refactor.MoveFileOptions{
		SourceFile: fileOutput,
		DestDir:    destDir,
	})
	if err != nil {
		t.Fatalf("Step 2 failed: %v", err)
	}

	// Verify ports.go in sagas now imports example.com/seqmove/sagas/order_payment
	portsBytes, readErr := os.ReadFile(filePorts)
	if readErr != nil {
		t.Fatalf("failed reading ports.go: %v", readErr)
	}
	if !strings.Contains(string(portsBytes), `"example.com/seqmove/sagas/order_payment"`) {
		t.Fatalf("expected ports.go to import target package after steps 1 & 2, got:\n%s", string(portsBytes))
	}

	// Step 3: Move ports.go (last remaining file in sagas, which imports order_payment) to sagas/order_payment
	err = refactor.MoveFile(refactor.MoveFileOptions{
		SourceFile: filePorts,
		DestDir:    destDir,
	})
	if err != nil {
		t.Fatalf("Step 3 failed with unexpected error: %v", err)
	}

	// Verify ports.go was successfully moved and updated to package order_payment
	movedPortsPath := filepath.Join(destDir, "ports.go")
	movedBytes, readErr := os.ReadFile(movedPortsPath)
	if readErr != nil {
		t.Fatalf("failed reading moved ports.go at %s: %v", movedPortsPath, readErr)
	}
	if !strings.Contains(string(movedBytes), "package order_payment") {
		t.Errorf("expected moved ports.go to have 'package order_payment', got:\n%s", string(movedBytes))
	}
}
