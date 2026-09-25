package server

import (
	"context"
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

	// Missing directory argument
	reqMissingDir := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "move_file",
			Arguments: map[string]any{
				"source_file": "nonexistent.go",
				"dest_dir":    "somedir",
			},
		},
	}
	resMissingDir, err := handleMoveFile(ctx, reqMissingDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resMissingDir.IsError {
		t.Errorf("expected error result when directory is missing")
	}

	// With directory argument, nonexistent file
	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "move_file",
			Arguments: map[string]any{
				"directory":   t.TempDir(),
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

	// Missing directory argument
	reqMissingDir := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "move_directory",
			Arguments: map[string]any{
				"source_dir": "nonexistent_dir",
				"dest_dir":   "somedir",
			},
		},
	}
	resMissingDir, err := handleMoveDirectory(ctx, reqMissingDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !resMissingDir.IsError {
		t.Errorf("expected error result when directory is missing")
	}

	// With directory argument, nonexistent directory
	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name: "move_directory",
			Arguments: map[string]any{
				"directory":  t.TempDir(),
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
