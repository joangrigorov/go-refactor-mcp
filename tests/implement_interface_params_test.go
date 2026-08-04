package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joangrigorov/go-refactor-mcp/internal/refactor"
)

func TestImplementInterfaceWithParametersAndMultiReturn(t *testing.T) {
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module example.com/implmod\n\ngo 1.22.0\n"), 0600)

	domainDir := filepath.Join(tempDir, "domain")
	_ = os.MkdirAll(domainDir, 0750)

	repoFile := filepath.Join(domainDir, "repo.go")
	repoCode := `package domain

import "context"

type Club struct {
	ID string
}

type ClubID string

type Repository interface {
	Save(ctx context.Context, club *Club) error
	LoadByID(ctx context.Context, id ClubID) (*Club, error)
}

type MockRepository struct{}

var _ Repository = (*MockRepository)(nil)
`
	_ = os.WriteFile(repoFile, []byte(repoCode), 0600)

	opts := refactor.ImplIfaceOptions{
		FilePath:      repoFile,
		StructName:    "MockRepository",
		InterfaceName: "Repository",
	}

	if err := refactor.ImplementInterface(opts); err != nil {
		t.Fatalf("ImplementInterface failed: %v", err)
	}

	resBytes, err := os.ReadFile(repoFile) //nolint:gosec
	if err != nil {
		t.Fatalf("failed reading repoFile: %v", err)
	}

	code := string(resBytes)

	if !strings.Contains(code, "Save(ctx context.Context, club *Club) error") {
		t.Errorf("Save method stub missing parameters (ctx context.Context, club *Club):\n%s", code)
	}

	if !strings.Contains(code, "LoadByID(ctx context.Context, id ClubID) (*Club, error)") {
		t.Errorf("LoadByID method stub missing parameters or multi-return type (*Club, error):\n%s", code)
	}
}
