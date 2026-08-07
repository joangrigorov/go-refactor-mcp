package tests

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/joangrigorov/go-refactor-mcp/internal/refactor"
)

func TestParseBuildTags(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{input: "", expected: nil},
		{input: "   ", expected: nil},
		{input: "integration", expected: []string{"integration"}},
		{input: "integration,e2e", expected: []string{"integration", "e2e"}},
		{input: "integration, e2e; wireinject", expected: []string{"integration", "e2e", "wireinject"}},
		{input: "integration e2e integration", expected: []string{"integration", "e2e"}},
	}

	for _, tt := range tests {
		got := refactor.ParseBuildTags(tt.input)
		if len(got) != len(tt.expected) {
			t.Errorf("ParseBuildTags(%q) = %v; want %v", tt.input, got, tt.expected)
			continue
		}
		for i := range got {
			if got[i] != tt.expected[i] {
				t.Errorf("ParseBuildTags(%q)[%d] = %q; want %q", tt.input, i, got[i], tt.expected[i])
			}
		}
	}
}

func TestDiscoverWorkspaceBuildTags(t *testing.T) {
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module example.com/tagdisc\n\ngo 1.22.0\n"), 0600)

	file1 := filepath.Join(tempDir, "main.go")
	_ = os.WriteFile(file1, []byte("//go:build integration\npackage main\n"), 0600)

	file2 := filepath.Join(tempDir, "e2e_test.go")
	_ = os.WriteFile(file2, []byte("//go:build e2e && !unit\n// +build e2e,!unit\npackage main_test\n"), 0600)

	file3 := filepath.Join(tempDir, "sys_linux.go")
	_ = os.WriteFile(file3, []byte("//go:build linux && amd64\npackage main\n"), 0600)

	vendorDir := filepath.Join(tempDir, "vendor", "lib")
	_ = os.MkdirAll(vendorDir, 0750)
	_ = os.WriteFile(filepath.Join(vendorDir, "vendor.go"), []byte("//go:build vendor_tag\npackage lib\n"), 0600)

	tags, err := refactor.DiscoverWorkspaceBuildTags(tempDir)
	if err != nil {
		t.Fatalf("DiscoverWorkspaceBuildTags failed: %v", err)
	}

	if !slices.Contains(tags, "integration") {
		t.Errorf("expected 'integration' tag in discovered tags: %v", tags)
	}
	if !slices.Contains(tags, "e2e") {
		t.Errorf("expected 'e2e' tag in discovered tags: %v", tags)
	}
	if !slices.Contains(tags, "unit") {
		t.Errorf("expected 'unit' tag in discovered tags: %v", tags)
	}
	if slices.Contains(tags, "linux") || slices.Contains(tags, "amd64") {
		t.Errorf("standard OS/Arch tags should be excluded, got: %v", tags)
	}
	if slices.Contains(tags, "vendor_tag") {
		t.Errorf("vendor tags should be ignored, got: %v", tags)
	}
}

func TestRenameSymbolWithAutoDiscoveredBuildTags(t *testing.T) {
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module example.com/autotagmod\n\ngo 1.22.0\n"), 0600)

	appDir := filepath.Join(tempDir, "app")
	_ = os.MkdirAll(appDir, 0750)

	appCode := `package app

type QueryHandler struct {
	Name string
}
`
	if err := os.WriteFile(filepath.Join(appDir, "handler.go"), []byte(appCode), 0600); err != nil {
		t.Fatalf("failed creating handler.go: %v", err)
	}

	unitTestCode := `package app

import "testing"

func TestHandler(t *testing.T) {
	_ = QueryHandler{Name: "unit"}
}
`
	if err := os.WriteFile(filepath.Join(appDir, "handler_test.go"), []byte(unitTestCode), 0600); err != nil {
		t.Fatalf("failed creating handler_test.go: %v", err)
	}

	intDir := filepath.Join(tempDir, "integration_tests")
	_ = os.MkdirAll(intDir, 0750)

	intTestCode := `//go:build integration
// +build integration

package integration_tests

import (
	"testing"
	"example.com/autotagmod/app"
)

func TestIntegrationQuery(t *testing.T) {
	h := app.QueryHandler{Name: "integration"}
	_ = h.Name
}
`
	if err := os.WriteFile(filepath.Join(intDir, "query_test.go"), []byte(intTestCode), 0600); err != nil {
		t.Fatalf("failed creating query_test.go: %v", err)
	}

	// Rename QueryHandler -> RequestProcessor without passing BuildTags (relying on auto-discovery)
	opts := refactor.RenameOptions{
		Dir:  tempDir,
		File: filepath.Join(appDir, "handler.go"),
		From: "QueryHandler",
		To:   "RequestProcessor",
	}

	if err := refactor.RenameSymbol(opts); err != nil {
		t.Fatalf("RenameSymbol failed: %v", err)
	}

	appRes, _ := os.ReadFile(filepath.Join(appDir, "handler.go")) //nolint:gosec
	if !strings.Contains(string(appRes), "type RequestProcessor struct") {
		t.Errorf("app handler.go missing 'type RequestProcessor struct': %s", string(appRes))
	}

	unitRes, _ := os.ReadFile(filepath.Join(appDir, "handler_test.go")) //nolint:gosec
	if !strings.Contains(string(unitRes), "RequestProcessor{Name: \"unit\"}") {
		t.Errorf("app handler_test.go missing RequestProcessor reference: %s", string(unitRes))
	}

	intRes, _ := os.ReadFile(filepath.Join(intDir, "query_test.go")) //nolint:gosec
	if !strings.Contains(string(intRes), "app.RequestProcessor{Name: \"integration\"}") {
		t.Errorf("integration_tests query_test.go (with //go:build integration) missing 'app.RequestProcessor': %s", string(intRes))
	}
}

func TestRenameSymbolWithExplicitBuildTags(t *testing.T) {
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module example.com/explicittagmod\n\ngo 1.22.0\n"), 0600)

	appDir := filepath.Join(tempDir, "domain")
	_ = os.MkdirAll(appDir, 0750)

	appCode := `package domain

type CustomService struct {
	ID string
}
`
	if err := os.WriteFile(filepath.Join(appDir, "service.go"), []byte(appCode), 0600); err != nil {
		t.Fatalf("failed creating service.go: %v", err)
	}

	e2eDir := filepath.Join(tempDir, "e2e")
	_ = os.MkdirAll(e2eDir, 0750)

	e2eCode := `//go:build e2e

package e2e

import (
	"testing"
	"example.com/explicittagmod/domain"
)

func TestE2E(t *testing.T) {
	_ = domain.CustomService{ID: "e2e-1"}
}
`
	if err := os.WriteFile(filepath.Join(e2eDir, "e2e_test.go"), []byte(e2eCode), 0600); err != nil {
		t.Fatalf("failed creating e2e_test.go: %v", err)
	}

	opts := refactor.RenameOptions{
		Dir:       tempDir,
		File:      filepath.Join(appDir, "service.go"),
		From:      "CustomService",
		To:        "CoreService",
		BuildTags: "e2e",
	}

	if err := refactor.RenameSymbol(opts); err != nil {
		t.Fatalf("RenameSymbol failed: %v", err)
	}

	e2eRes, _ := os.ReadFile(filepath.Join(e2eDir, "e2e_test.go")) //nolint:gosec
	if !strings.Contains(string(e2eRes), "domain.CoreService") {
		t.Errorf("e2e_test.go missing 'domain.CoreService': %s", string(e2eRes))
	}
}
