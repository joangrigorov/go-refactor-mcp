package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joangrigorov/go-refactor-mcp/internal/refactor"
)

func TestImplementInterfaceDynamicStdlibResolution(t *testing.T) {
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module example.com/ifacemod\n\ngo 1.22.0\n"), 0600)

	serverFile := filepath.Join(tempDir, "server.go")
	code := `package ifacemod

import "net/http"

type MyHandler struct{}
`
	_ = os.WriteFile(serverFile, []byte(code), 0600)

	opts := refactor.ImplIfaceOptions{
		FilePath:      serverFile,
		StructName:    "MyHandler",
		InterfaceName: "http.Handler",
	}

	if err := refactor.ImplementInterface(opts); err != nil {
		t.Fatalf("ImplementInterface failed: %v", err)
	}

	resBytes, err := os.ReadFile(serverFile) //nolint:gosec
	if err != nil {
		t.Fatalf("failed reading serverFile: %v", err)
	}

	resultCode := string(resBytes)
	if !strings.Contains(resultCode, "ServeHTTP(") {
		t.Errorf("expected ServeHTTP method stub in serverFile, got:\n%s", resultCode)
	}
}
