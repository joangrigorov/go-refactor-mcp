package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joangrigorov/go-refactor-mcp/internal/refactor"
)

func TestMoveFileImportCollisionAutoAliasing(t *testing.T) {
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module example.com/collisionmod\n\ngo 1.22.0\n"), 0600)

	// 1. Existing types package: existing/types/types.go
	existingTypesDir := filepath.Join(tempDir, "existing", "types")
	_ = os.MkdirAll(existingTypesDir, 0750)
	existingCode := `package types

type Role string
`
	_ = os.WriteFile(filepath.Join(existingTypesDir, "role.go"), []byte(existingCode), 0600)

	// 2. Source errors package: src/club/errors.go
	sourceDir := filepath.Join(tempDir, "src", "club")
	_ = os.MkdirAll(sourceDir, 0750)
	errorsFile := filepath.Join(sourceDir, "errors.go")
	errorsCode := `package club

import "errors"

var ErrClubNotFound = errors.New("club not found")
`
	_ = os.WriteFile(errorsFile, []byte(errorsCode), 0600)

	// 3. Consumer file: app/consumer.go
	// Already imports example.com/collisionmod/existing/types as "types"
	// And imports example.com/collisionmod/src/club
	consumerDir := filepath.Join(tempDir, "app")
	_ = os.MkdirAll(consumerDir, 0750)
	consumerFile := filepath.Join(consumerDir, "consumer.go")
	consumerCode := `package app

import (
	"fmt"
	"example.com/collisionmod/existing/types"
	"example.com/collisionmod/src/club"
)

func Check(r types.Role) {
	fmt.Println(r, club.ErrClubNotFound)
}
`
	_ = os.WriteFile(consumerFile, []byte(consumerCode), 0600)

	// 4. Move errors.go to domain/types (whose package name is "types", causing an import collision with existing/types)
	destDir := filepath.Join(tempDir, "domain", "types")
	opts := refactor.MoveFileOptions{
		SourceFile: errorsFile,
		DestDir:    destDir,
	}

	if err := refactor.MoveFile(opts); err != nil {
		t.Fatalf("MoveFile failed: %v", err)
	}

	// 5. Read consumer.go and verify collision was safely resolved via alias
	consumerBytes, err := os.ReadFile(consumerFile) //nolint:gosec
	if err != nil {
		t.Fatalf("failed reading consumer.go: %v", err)
	}
	consumerStr := string(consumerBytes)

	// Should still have existing/types
	if !strings.Contains(consumerStr, `"example.com/collisionmod/existing/types"`) {
		t.Errorf("consumer.go should still import existing/types:\n%s", consumerStr)
	}

	// Should import domain/types with an alias (e.g. domainTypes)
	if !strings.Contains(consumerStr, `"example.com/collisionmod/domain/types"`) {
		t.Errorf("consumer.go missing import for domain/types:\n%s", consumerStr)
	}

	// Ensure there is an alias before the domain/types import path to avoid redeclaration
	hasAlias := strings.Contains(consumerStr, `domainTypes "example.com/collisionmod/domain/types"`) ||
		strings.Contains(consumerStr, `types2 "example.com/collisionmod/domain/types"`) ||
		strings.Contains(consumerStr, `domain_types "example.com/collisionmod/domain/types"`)

	if !hasAlias {
		t.Errorf("expected domain/types import to be aliased to avoid collision, got:\n%s", consumerStr)
	}

	// Ensure selector for ErrClubNotFound is updated to the aliased package name
	hasUpdatedSelector := strings.Contains(consumerStr, `domainTypes.ErrClubNotFound`) ||
		strings.Contains(consumerStr, `types2.ErrClubNotFound`) ||
		strings.Contains(consumerStr, `domain_types.ErrClubNotFound`)

	if !hasUpdatedSelector {
		t.Errorf("expected consumer selector to use aliased package name for ErrClubNotFound, got:\n%s", consumerStr)
	}
}
