package server

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/mark3labs/mcp-go/mcp"
)

func TestNewServer_ToolsRegistered(t *testing.T) {
	s := NewServer()
	if s == nil {
		t.Fatal("expected non-nil server")
	}

	expectedTools := []string{
		"rename_symbol",
		"move_file",
		"move_directory",
		"implement_interface",
		"analyze_shadowing",
	}

	tools := s.ListTools()
	for _, toolName := range expectedTools {
		tool, exists := tools[toolName]
		if !exists || tool == nil {
			t.Errorf("expected tool %q to be registered", toolName)
			continue
		}
		if tool.Tool.Description == "" {
			t.Errorf("expected tool %q to have a description", toolName)
		}
	}
}

func TestHandleRenameSymbol(t *testing.T) {
	ctx := context.Background()

	// Missing required params should return an error result
	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "rename_symbol",
			Arguments: map[string]any{
				"directory": "",
				"from":      "",
				"to":        "",
			},
		},
	}

	res, err := handleRenameSymbol(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Errorf("expected error result when from/to are empty")
	}
}

func TestHandleMoveFile(t *testing.T) {
	ctx := context.Background()

	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "move_file",
			Arguments: map[string]any{
				"source_file": "nonexistent.go",
				"dest_dir":    "somedir",
			},
		},
	}

	res, err := handleMoveFile(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Errorf("expected error result for nonexistent source file")
	}
}

func TestHandleMoveDirectory(t *testing.T) {
	ctx := context.Background()

	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "move_directory",
			Arguments: map[string]any{
				"source_dir": "nonexistent_dir",
				"dest_dir":   "somedir",
			},
		},
	}

	res, err := handleMoveDirectory(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Errorf("expected error result for nonexistent source directory")
	}
}

func TestHandleImplementInterface(t *testing.T) {
	ctx := context.Background()

	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "implement_interface",
			Arguments: map[string]any{
				"file_path":      "nonexistent.go",
				"struct_name":    "MyStruct",
				"interface_name": "io.Reader",
			},
		},
	}

	res, err := handleImplementInterface(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !res.IsError {
		t.Errorf("expected error result for nonexistent file")
	}
}

func TestHandleAnalyzeShadowing(t *testing.T) {
	ctx := context.Background()
	tempDir := t.TempDir()

	code := `package dummy

func Foo() int {
	x := 1
	return x
}
`
	filePath := filepath.Join(tempDir, "dummy.go")
	if err := os.WriteFile(filePath, []byte(code), 0600); err != nil {
		t.Fatalf("failed writing test file: %v", err)
	}

	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "analyze_shadowing",
			Arguments: map[string]any{
				"target_path": filePath,
			},
		},
	}

	res, err := handleAnalyzeShadowing(ctx, req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if res.IsError {
		t.Errorf("expected success result, got error: %v", res)
	}
}
