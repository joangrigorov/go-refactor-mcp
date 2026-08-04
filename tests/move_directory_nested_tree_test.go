package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joangrigorov/go-refactor-mcp/internal/refactor"
)

func TestMoveDirectoryNestedTreePackageBoundaries(t *testing.T) {
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module example.com/nestedmod\n\ngo 1.22.0\n"), 0600)

	// Top-level src/component/club
	clubDir := filepath.Join(tempDir, "src", "component", "club")
	_ = os.MkdirAll(clubDir, 0750)
	_ = os.WriteFile(filepath.Join(clubDir, "club.go"), []byte("package club\n\ntype Club struct{}\n"), 0600)

	// Sub-directory src/component/club/policies
	policiesDir := filepath.Join(clubDir, "policies")
	_ = os.MkdirAll(policiesDir, 0750)
	_ = os.WriteFile(filepath.Join(policiesDir, "policy.go"), []byte("package policies\n\ntype Policy struct{}\n"), 0600)

	// Nested sub-directory src/component/club/domain/member
	memberDir := filepath.Join(clubDir, "domain", "member")
	_ = os.MkdirAll(memberDir, 0750)
	_ = os.WriteFile(filepath.Join(memberDir, "member.go"), []byte("package member\n\ntype Member struct{}\n"), 0600)

	// Dependent app
	appDir := filepath.Join(tempDir, "src", "app")
	_ = os.MkdirAll(appDir, 0750)
	appFile := filepath.Join(appDir, "app.go")
	appCode := `package app

import (
	"example.com/nestedmod/src/component/club/policies"
	"example.com/nestedmod/src/component/club/domain/member"
)

func Handle(p policies.Policy, m member.Member) {}
`
	_ = os.WriteFile(appFile, []byte(appCode), 0600)

	// Move src/component/club to src/club_context
	destDir := filepath.Join(tempDir, "src", "club_context")
	opts := refactor.MoveDirOptions{
		SourceDir: clubDir,
		DestDir:   destDir,
	}

	if err := refactor.MoveDirectory(opts); err != nil {
		t.Fatalf("MoveDirectory failed: %v", err)
	}

	// 1. Check top-level file package name updated to club_context
	topBytes, err := os.ReadFile(filepath.Join(destDir, "club.go")) //nolint:gosec
	if err != nil {
		t.Fatalf("failed reading top-level file: %v", err)
	}
	if !strings.HasPrefix(string(topBytes), "package club_context") {
		t.Errorf("top-level file package should be 'package club_context', got:\n%s", string(topBytes))
	}

	// 2. Check child sub-directory package name preserved as policies
	childBytes, err := os.ReadFile(filepath.Join(destDir, "policies", "policy.go")) //nolint:gosec
	if err != nil {
		t.Fatalf("failed reading child policy file: %v", err)
	}
	if !strings.HasPrefix(string(childBytes), "package policies") {
		t.Errorf("child policy file package should remain 'package policies', got:\n%s", string(childBytes))
	}

	// 3. Check nested member sub-directory package name preserved as member
	nestedBytes, err := os.ReadFile(filepath.Join(destDir, "domain", "member", "member.go")) //nolint:gosec
	if err != nil {
		t.Fatalf("failed reading nested member file: %v", err)
	}
	if !strings.HasPrefix(string(nestedBytes), "package member") {
		t.Errorf("nested member file package should remain 'package member', got:\n%s", string(nestedBytes))
	}

	// 4. Check app.go import paths updated
	appRes, err := os.ReadFile(appFile) //nolint:gosec
	if err != nil {
		t.Fatalf("failed reading app.go: %v", err)
	}
	appStr := string(appRes)

	if !strings.Contains(appStr, `"example.com/nestedmod/src/club_context/policies"`) {
		t.Errorf("app.go missing updated policies import:\n%s", appStr)
	}
	if !strings.Contains(appStr, `"example.com/nestedmod/src/club_context/domain/member"`) {
		t.Errorf("app.go missing updated member import:\n%s", appStr)
	}
}
