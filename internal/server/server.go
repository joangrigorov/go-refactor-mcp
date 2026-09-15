// Package server implements the MCP STDIO server and refactoring tool handlers.
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/joangrigorov/go-refactor-mcp/internal/refactor"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

// NewServer creates a new MCP server with all refactoring tools registered.
func NewServer() *server.MCPServer {
	s := server.NewMCPServer("go-refactor-mcp", "1.0.0")

	registerTools(s)

	return s
}

func registerTools(s *server.MCPServer) {
	// 1. rename_symbol
	renameTool := mcp.NewTool("rename_symbol",
		mcp.WithDescription("Renames a symbol (variable, function, struct, interface, package, generic type parameter) across the entire Go module."),
		mcp.WithString("directory", mcp.Required(), mcp.Description("Root directory of the module or workspace")),
		mcp.WithString("from", mcp.Required(), mcp.Description("Current symbol name")),
		mcp.WithString("to", mcp.Required(), mcp.Description("New symbol name")),
		mcp.WithString("file", mcp.Description("Target file path (optional)")),
		mcp.WithNumber("offset", mcp.Description("Byte offset of symbol position (optional)")),
		mcp.WithString("build_tags", mcp.Description("Optional build tags override (e.g. 'integration,e2e')")),
	)
	s.AddTool(renameTool, handleRenameSymbol)

	// 2. move_file
	moveFileTool := mcp.NewTool("move_file",
		mcp.WithDescription("Moves or renames a .go file (and associated _test.go) to a new directory or filename, updating package clauses, build tags, and imports without creating cyclic dependencies."),
		mcp.WithString("source_file", mcp.Required(), mcp.Description("Source .go file path")),
		mcp.WithString("dest_dir", mcp.Description("Destination directory path (optional if new_name is provided)")),
		mcp.WithString("new_name", mcp.Description("Optional new filename (e.g. 'saga.go') for renaming in-place or upon move")),
		mcp.WithString("build_tags", mcp.Description("Optional build tags override (e.g. 'integration,e2e')")),
	)
	s.AddTool(moveFileTool, handleMoveFile)

	// 3. move_directory
	moveDirTool := mcp.NewTool("move_directory",
		mcp.WithDescription("Moves an entire package directory and updates all import paths referencing this package across the module."),
		mcp.WithString("source_dir", mcp.Required(), mcp.Description("Source directory path")),
		mcp.WithString("dest_dir", mcp.Required(), mcp.Description("Destination directory path")),
		mcp.WithString("build_tags", mcp.Description("Optional build tags override (e.g. 'integration,e2e')")),
	)
	s.AddTool(moveDirTool, handleMoveDirectory)

	// 4. implement_interface
	implIfaceTool := mcp.NewTool("implement_interface",
		mcp.WithDescription("Generates missing method stubs on a struct for a specified interface."),
		mcp.WithString("file_path", mcp.Required(), mcp.Description("Path to file containing struct")),
		mcp.WithString("struct_name", mcp.Required(), mcp.Description("Name of target struct")),
		mcp.WithString("interface_name", mcp.Required(), mcp.Description("Interface identifier (e.g., io.Reader or local interface)")),
		mcp.WithString("build_tags", mcp.Description("Optional build tags override (e.g. 'integration,e2e')")),
	)
	s.AddTool(implIfaceTool, handleImplementInterface)

	// 5. analyze_shadowing
	shadowTool := mcp.NewTool("analyze_shadowing",
		mcp.WithDescription("Scans a file or package for shadowed variables to avoid logic bugs."),
		mcp.WithString("target_path", mcp.Required(), mcp.Description("Target file or package directory path")),
		mcp.WithString("build_tags", mcp.Description("Optional build tags override (e.g. 'integration,e2e')")),
	)
	s.AddTool(shadowTool, handleAnalyzeShadowing)
}

func handleRenameSymbol(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	dir := strings.TrimSpace(req.GetString("directory", ""))
	from := strings.TrimSpace(req.GetString("from", ""))
	to := strings.TrimSpace(req.GetString("to", ""))
	file := strings.TrimSpace(req.GetString("file", ""))
	offset := req.GetInt("offset", 0)
	buildTags := strings.TrimSpace(req.GetString("build_tags", ""))

	if dir == "" {
		return mcp.NewToolResultError("rename_symbol: argument 'directory' is required. Specify the root directory of the Go module or workspace (e.g. '.')"), nil
	}
	if from == "" {
		return mcp.NewToolResultError("rename_symbol: argument 'from' is required (current symbol name)"), nil
	}
	if to == "" {
		return mcp.NewToolResultError("rename_symbol: argument 'to' is required (new symbol name)"), nil
	}

	opts := refactor.RenameOptions{
		Dir:       dir,
		File:      file,
		From:      from,
		To:        to,
		Offset:    offset,
		BuildTags: buildTags,
	}

	if err := refactor.RenameSymbol(opts); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("rename_symbol failed: %v", err)), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("Successfully renamed '%s' to '%s'", from, to)), nil
}

func handleMoveFile(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	src := strings.TrimSpace(req.GetString("source_file", ""))
	dest := strings.TrimSpace(req.GetString("dest_dir", ""))
	newName := strings.TrimSpace(req.GetString("new_name", ""))
	buildTags := strings.TrimSpace(req.GetString("build_tags", ""))

	if src == "" {
		return mcp.NewToolResultError("move_file: argument 'source_file' is required. Specify the relative or absolute path to a .go file"), nil
	}
	if !strings.HasSuffix(src, ".go") {
		return mcp.NewToolResultError(fmt.Sprintf("move_file: argument 'source_file' (%q) must be a .go file. Use 'move_directory' for directory moves", src)), nil
	}
	if dest == "" && newName == "" {
		return mcp.NewToolResultError("move_file: at least one of 'dest_dir' (to move file) or 'new_name' (to rename file) is required"), nil
	}

	opts := refactor.MoveFileOptions{
		SourceFile: src,
		DestDir:    dest,
		NewName:    newName,
		BuildTags:  buildTags,
	}

	if err := refactor.MoveFile(opts); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("move_file failed: %v", err)), nil
	}
	if dest != "" && newName != "" {
		return mcp.NewToolResultText(fmt.Sprintf("Successfully moved and renamed file '%s' to '%s/%s'", src, dest, newName)), nil
	} else if newName != "" {
		return mcp.NewToolResultText(fmt.Sprintf("Successfully renamed file '%s' to '%s'", src, newName)), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("Successfully moved file '%s' to '%s'", src, dest)), nil
}

func handleMoveDirectory(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	src := strings.TrimSpace(req.GetString("source_dir", ""))
	dest := strings.TrimSpace(req.GetString("dest_dir", ""))
	buildTags := strings.TrimSpace(req.GetString("build_tags", ""))

	if src == "" {
		return mcp.NewToolResultError("move_directory: argument 'source_dir' is required. Specify the source directory path"), nil
	}
	if dest == "" {
		return mcp.NewToolResultError("move_directory: argument 'dest_dir' is required. Specify the destination directory path"), nil
	}

	opts := refactor.MoveDirOptions{
		SourceDir: src,
		DestDir:   dest,
		BuildTags: buildTags,
	}

	if err := refactor.MoveDirectory(opts); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("move_directory failed: %v", err)), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("Successfully moved directory '%s' to '%s'", src, dest)), nil
}

func handleImplementInterface(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	filePath := strings.TrimSpace(req.GetString("file_path", ""))
	structName := strings.TrimSpace(req.GetString("struct_name", ""))
	interfaceName := strings.TrimSpace(req.GetString("interface_name", ""))
	buildTags := strings.TrimSpace(req.GetString("build_tags", ""))

	if filePath == "" {
		return mcp.NewToolResultError("implement_interface: argument 'file_path' is required. Specify the path to the Go file containing the struct"), nil
	}
	if structName == "" {
		return mcp.NewToolResultError("implement_interface: argument 'struct_name' is required. Specify the name of the target struct"), nil
	}
	if interfaceName == "" {
		return mcp.NewToolResultError("implement_interface: argument 'interface_name' is required (e.g. 'io.Reader' or a local interface name)"), nil
	}

	opts := refactor.ImplIfaceOptions{
		FilePath:      filePath,
		StructName:    structName,
		InterfaceName: interfaceName,
		BuildTags:     buildTags,
	}

	if err := refactor.ImplementInterface(opts); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("implement_interface failed: %v", err)), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("Successfully generated method stubs for interface '%s' on struct '%s'", interfaceName, structName)), nil
}

func handleAnalyzeShadowing(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	targetPath := strings.TrimSpace(req.GetString("target_path", ""))
	buildTags := strings.TrimSpace(req.GetString("build_tags", ""))

	if targetPath == "" {
		return mcp.NewToolResultError("analyze_shadowing: argument 'target_path' is required. Specify the path to a Go file or package directory"), nil
	}

	issues, err := refactor.AnalyzeShadowing(targetPath, buildTags)
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("analyze_shadowing failed: %v", err)), nil
	}

	if len(issues) == 0 {
		return mcp.NewToolResultText("No shadowed variables detected."), nil
	}

	jsonBytes, err := json.MarshalIndent(issues, "", "  ")
	if err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("failed encoding shadowing report: %v", err)), nil
	}

	return mcp.NewToolResultText(string(jsonBytes)), nil
}
