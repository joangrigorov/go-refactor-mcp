package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joangrigorov/go-refactor-mcp/internal/server"
)

func TestMCPIntegration_EmbeddedInterface_LocalHierarchy(t *testing.T) {
	s := server.NewServer()
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module example.com/embeddedtest\n\ngo 1.22.0\n"), 0600)

	code := `package main

type Reader interface {
	Read(p []byte) (n int, err error)
}

type Closer interface {
	Close() error
}

type ReadCloser interface {
	Reader
	Closer
	Reset() error
}

type MyBuffer struct{}

func main() {}
`
	targetFile := filepath.Join(tempDir, "buffer.go")
	_ = os.WriteFile(targetFile, []byte(code), 0600)

	res := callMCPTool(t, s, "implement_interface", map[string]any{
		"file_path":      targetFile,
		"struct_name":    "MyBuffer",
		"interface_name": "ReadCloser",
	})

	if res.IsError {
		t.Fatalf("expected implement_interface to succeed, got error: %s", getResultText(t, res))
	}

	content, err := os.ReadFile(targetFile) // #nosec G304
	if err != nil {
		t.Fatalf("failed reading target file: %v", err)
	}
	contentStr := string(content)

	for _, expectedMethod := range []string{
		"func (m *MyBuffer) Read(p []byte) (n int, err error)",
		"func (m *MyBuffer) Close() error",
		"func (m *MyBuffer) Reset() error",
	} {
		if !strings.Contains(contentStr, expectedMethod) {
			t.Errorf("expected buffer.go to contain method stub %q, got:\n%s", expectedMethod, contentStr)
		}
	}

	// Verify post-refactor compilation succeeds
	verifyCompilation(t, tempDir)

	// Verify idempotency
	res2 := callMCPTool(t, s, "implement_interface", map[string]any{
		"file_path":      targetFile,
		"struct_name":    "MyBuffer",
		"interface_name": "ReadCloser",
	})
	if res2.IsError {
		t.Fatalf("expected idempotent implement_interface call to succeed, got: %s", getResultText(t, res2))
	}

	verifyCompilation(t, tempDir)
}

func TestMCPIntegration_EmbeddedInterface_StdlibEmbedding(t *testing.T) {
	s := server.NewServer()
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module example.com/stdstream\n\ngo 1.22.0\n"), 0600)

	code := `package main

import "io"

type CustomStream interface {
	io.ReadCloser
	Flush() error
}

type StreamHandler struct{}

func main() {}
`
	targetFile := filepath.Join(tempDir, "stream.go")
	_ = os.WriteFile(targetFile, []byte(code), 0600)

	res := callMCPTool(t, s, "implement_interface", map[string]any{
		"file_path":      targetFile,
		"struct_name":    "StreamHandler",
		"interface_name": "CustomStream",
	})

	if res.IsError {
		t.Fatalf("expected implement_interface to succeed, got: %s", getResultText(t, res))
	}

	content, err := os.ReadFile(targetFile) // #nosec G304
	if err != nil {
		t.Fatalf("failed reading stream.go: %v", err)
	}
	contentStr := string(content)

	for _, expectedMethod := range []string{
		"Read(p []byte)",
		"err error)",
		"func (s *StreamHandler) Close() error",
		"func (s *StreamHandler) Flush() error",
	} {
		if !strings.Contains(contentStr, expectedMethod) {
			t.Errorf("expected stream.go to contain %q, got:\n%s", expectedMethod, contentStr)
		}
	}

	verifyCompilation(t, tempDir)
}

func TestMCPIntegration_EmbeddedInterface_UndefinedEmbeddedType(t *testing.T) {
	s := server.NewServer()
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module example.com/brokeniface\n\ngo 1.22.0\n"), 0600)

	code := `package main

type BrokenInterface interface {
	NonExistentInterface
	DoWork()
}

type Worker struct{}

func main() {}
`
	targetFile := filepath.Join(tempDir, "worker.go")
	_ = os.WriteFile(targetFile, []byte(code), 0600)

	res := callMCPTool(t, s, "implement_interface", map[string]any{
		"file_path":      targetFile,
		"struct_name":    "Worker",
		"interface_name": "BrokenInterface",
	})

	if !res.IsError {
		t.Fatalf("expected error implementing interface embedding undefined type, but succeeded")
	}

	errMsg := getResultText(t, res)
	if !strings.Contains(errMsg, "failed to resolve embedded interface") || !strings.Contains(errMsg, "NonExistentInterface") {
		t.Errorf("expected actionable error indicating which embedded interface failed to resolve, got: %s", errMsg)
	}
}

func TestMCPIntegration_EmbeddedInterface_TargetNotAnInterface(t *testing.T) {
	s := server.NewServer()
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module example.com/notiface\n\ngo 1.22.0\n"), 0600)

	code := `package main

type ConcreteConfig struct {
	Timeout int
}

type MyService struct{}

func main() {}
`
	targetFile := filepath.Join(tempDir, "service.go")
	_ = os.WriteFile(targetFile, []byte(code), 0600)

	res := callMCPTool(t, s, "implement_interface", map[string]any{
		"file_path":      targetFile,
		"struct_name":    "MyService",
		"interface_name": "ConcreteConfig",
	})

	if !res.IsError {
		t.Fatalf("expected error when target is a struct, but got success")
	}

	errMsg := getResultText(t, res)
	if !strings.Contains(errMsg, "is a struct, not an interface") {
		t.Errorf("expected error explaining target is a struct and not an interface, got: %s", errMsg)
	}
}
