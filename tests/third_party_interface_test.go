package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joangrigorov/go-refactor-mcp/internal/server"
)

func TestMCPIntegration_ThirdPartyInterface_StdlibShorthand_HTTPHandler(t *testing.T) {
	s := server.NewServer()
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module example.com/httptest\n\ngo 1.22.0\n"), 0600)

	// File does NOT import "net/http" initially
	code := `package main

type MyServer struct{}

func main() {}
`
	targetFile := filepath.Join(tempDir, "server.go")
	_ = os.WriteFile(targetFile, []byte(code), 0600)

	res := callMCPTool(t, s, "implement_interface", map[string]any{
		"file_path":      targetFile,
		"struct_name":    "MyServer",
		"interface_name": "http.Handler",
	})

	if res.IsError {
		t.Fatalf("expected implement_interface to succeed for http.Handler, got: %s", getResultText(t, res))
	}

	content, err := os.ReadFile(targetFile) // #nosec G304
	if err != nil {
		t.Fatalf("failed reading server.go: %v", err)
	}
	contentStr := string(content)

	if !strings.Contains(contentStr, "ServeHTTP(") {
		t.Errorf("expected server.go to contain 'ServeHTTP(', got:\n%s", contentStr)
	}
	if !strings.Contains(contentStr, "http.ResponseWriter") || !strings.Contains(contentStr, "*http.Request") {
		t.Errorf("expected server.go to contain ResponseWriter and Request parameters, got:\n%s", contentStr)
	}
	if !strings.Contains(contentStr, `"net/http"`) {
		t.Errorf("expected server.go to automatically import 'net/http', got:\n%s", contentStr)
	}

	// Verify post-refactor compilation succeeds cleanly
	verifyCompilation(t, tempDir)
}

func TestMCPIntegration_ThirdPartyInterface_StdlibShorthand_SQLScanner(t *testing.T) {
	s := server.NewServer()
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module example.com/sqltest\n\ngo 1.22.0\n"), 0600)

	code := `package main

type CustomColumn struct {
	Data string
}

func main() {}
`
	targetFile := filepath.Join(tempDir, "column.go")
	_ = os.WriteFile(targetFile, []byte(code), 0600)

	res := callMCPTool(t, s, "implement_interface", map[string]any{
		"file_path":      targetFile,
		"struct_name":    "CustomColumn",
		"interface_name": "sql.Scanner",
	})

	if res.IsError {
		t.Fatalf("expected implement_interface to succeed for sql.Scanner, got: %s", getResultText(t, res))
	}

	content, err := os.ReadFile(targetFile) // #nosec G304
	if err != nil {
		t.Fatalf("failed reading column.go: %v", err)
	}
	contentStr := string(content)

	if !strings.Contains(contentStr, "Scan(") || !strings.Contains(contentStr, "error") {
		t.Errorf("expected column.go to contain 'Scan(...) error', got:\n%s", contentStr)
	}

	verifyCompilation(t, tempDir)
}

func TestMCPIntegration_ThirdPartyInterface_UnresolvablePackagePath(t *testing.T) {
	s := server.NewServer()
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module example.com/unresolvable\n\ngo 1.22.0\n"), 0600)

	code := `package main

type Storage struct{}

func main() {}
`
	targetFile := filepath.Join(tempDir, "storage.go")
	_ = os.WriteFile(targetFile, []byte(code), 0600)

	res := callMCPTool(t, s, "implement_interface", map[string]any{
		"file_path":      targetFile,
		"struct_name":    "Storage",
		"interface_name": "invalid/pkg.MyIface",
	})

	if !res.IsError {
		t.Fatalf("expected error when resolving non-existent package path, but got success")
	}

	errMsg := getResultText(t, res)
	if !strings.Contains(errMsg, "could not be resolved from standard library, current file, or workspace packages") {
		t.Errorf("expected actionable error explaining interface could not be resolved, got: %s", errMsg)
	}
}

func TestMCPIntegration_ThirdPartyInterface_NonInterfaceTypeInPackage(t *testing.T) {
	s := server.NewServer()
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module example.com/notifacepkg\n\ngo 1.22.0\n"), 0600)

	code := `package main

type Client struct{}

func main() {}
`
	targetFile := filepath.Join(tempDir, "client.go")
	_ = os.WriteFile(targetFile, []byte(code), 0600)

	res := callMCPTool(t, s, "implement_interface", map[string]any{
		"file_path":      targetFile,
		"struct_name":    "Client",
		"interface_name": "http.Request",
	})

	if !res.IsError {
		t.Fatalf("expected error when target is a struct in net/http, but got success")
	}

	errMsg := getResultText(t, res)
	if !strings.Contains(errMsg, "is a struct, not an interface") {
		t.Errorf("expected actionable error indicating target is a struct, not an interface, got: %s", errMsg)
	}
}
