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
	)
	s.AddTool(renameTool, handleRenameSymbol)

	// 2. move_file
	moveFileTool := mcp.NewTool("move_file",
		mcp.WithDescription("Moves a .go file (and associated _test.go) to a new directory, updating package clauses, build tags, and imports without creating cyclic dependencies."),
		mcp.WithString("source_file", mcp.Required(), mcp.Description("Source .go file path")),
		mcp.WithString("dest_dir", mcp.Required(), mcp.Description("Destination directory path")),
	)
	s.AddTool(moveFileTool, handleMoveFile)

	// 3. move_directory
	moveDirTool := mcp.NewTool("move_directory",
		mcp.WithDescription("Moves an entire package directory and updates all import paths referencing this package across the module."),
		mcp.WithString("source_dir", mcp.Required(), mcp.Description("Source directory path")),
		mcp.WithString("dest_dir", mcp.Required(), mcp.Description("Destination directory path")),
	)
	s.AddTool(moveDirTool, handleMoveDirectory)

	// 4. extract_function
	extractFuncTool := mcp.NewTool("extract_function",
		mcp.WithDescription("Extracts a range of code lines into a new function with input/output variable resolution."),
		mcp.WithString("file_path", mcp.Required(), mcp.Description("Path to .go file")),
		mcp.WithNumber("start_line", mcp.Required(), mcp.Description("Start line number")),
		mcp.WithNumber("end_line", mcp.Required(), mcp.Description("End line number")),
		mcp.WithString("new_func_name", mcp.Required(), mcp.Description("Name for the new extracted function")),
	)
	s.AddTool(extractFuncTool, handleExtractFunction)

	// 5. extract_interface
	extractIfaceTool := mcp.NewTool("extract_interface",
		mcp.WithDescription("Generates an interface definition from the exported methods of a struct."),
		mcp.WithString("file_path", mcp.Required(), mcp.Description("Path to file containing struct")),
		mcp.WithString("struct_name", mcp.Required(), mcp.Description("Name of target struct")),
		mcp.WithString("interface_name", mcp.Required(), mcp.Description("Name of interface to generate")),
		mcp.WithString("dest_file_path", mcp.Description("Destination file path for interface (defaults to same file)")),
	)
	s.AddTool(extractIfaceTool, handleExtractInterface)

	// 6. implement_interface
	implIfaceTool := mcp.NewTool("implement_interface",
		mcp.WithDescription("Generates missing method stubs on a struct for a specified interface."),
		mcp.WithString("file_path", mcp.Required(), mcp.Description("Path to file containing struct")),
		mcp.WithString("struct_name", mcp.Required(), mcp.Description("Name of target struct")),
		mcp.WithString("interface_name", mcp.Required(), mcp.Description("Interface identifier (e.g., io.Reader or local interface)")),
	)
	s.AddTool(implIfaceTool, handleImplementInterface)

	// 7. add_struct_tags
	addTagsTool := mcp.NewTool("add_struct_tags",
		mcp.WithDescription("Generates or appends struct field tags (convert CamelCase to snake_case)."),
		mcp.WithString("file_path", mcp.Required(), mcp.Description("Path to file containing struct")),
		mcp.WithString("struct_name", mcp.Required(), mcp.Description("Name of target struct")),
		mcp.WithString("tags", mcp.Description("Comma-separated tag keys to append, e.g. 'json,yaml,db'")),
	)
	s.AddTool(addTagsTool, handleAddStructTags)

	// 8. tidy_imports
	tidyTool := mcp.NewTool("tidy_imports",
		mcp.WithDescription("Organizes, groups, and cleans unused imports in a file (goimports)."),
		mcp.WithString("file_path", mcp.Required(), mcp.Description("Path to target .go file")),
	)
	s.AddTool(tidyTool, handleTidyImports)

	// 9. analyze_shadowing
	shadowTool := mcp.NewTool("analyze_shadowing",
		mcp.WithDescription("Scans a file or package for shadowed variables to avoid logic bugs."),
		mcp.WithString("target_path", mcp.Required(), mcp.Description("Target file or package directory path")),
	)
	s.AddTool(shadowTool, handleAnalyzeShadowing)
}

func handleRenameSymbol(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	dir, _ := req.Params.Arguments["directory"].(string)
	from, _ := req.Params.Arguments["from"].(string)
	to, _ := req.Params.Arguments["to"].(string)
	file, _ := req.Params.Arguments["file"].(string)
	offsetFloat, _ := req.Params.Arguments["offset"].(float64)

	opts := refactor.RenameOptions{
		Dir:    dir,
		File:   file,
		From:   from,
		To:     to,
		Offset: int(offsetFloat),
	}

	if err := refactor.RenameSymbol(opts); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("rename_symbol failed: %v", err)), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("Successfully renamed '%s' to '%s'", from, to)), nil
}

func handleMoveFile(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	src, _ := req.Params.Arguments["source_file"].(string)
	dest, _ := req.Params.Arguments["dest_dir"].(string)

	opts := refactor.MoveFileOptions{
		SourceFile: src,
		DestDir:    dest,
	}

	if err := refactor.MoveFile(opts); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("move_file failed: %v", err)), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("Successfully moved file '%s' to '%s'", src, dest)), nil
}

func handleMoveDirectory(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	src, _ := req.Params.Arguments["source_dir"].(string)
	dest, _ := req.Params.Arguments["dest_dir"].(string)

	opts := refactor.MoveDirOptions{
		SourceDir: src,
		DestDir:   dest,
	}

	if err := refactor.MoveDirectory(opts); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("move_directory failed: %v", err)), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("Successfully moved directory '%s' to '%s'", src, dest)), nil
}

func handleExtractFunction(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	filePath, _ := req.Params.Arguments["file_path"].(string)
	startLine, _ := req.Params.Arguments["start_line"].(float64)
	endLine, _ := req.Params.Arguments["end_line"].(float64)
	newName, _ := req.Params.Arguments["new_func_name"].(string)

	opts := refactor.ExtractFuncOptions{
		FilePath:    filePath,
		StartLine:   int(startLine),
		EndLine:     int(endLine),
		NewFuncName: newName,
	}

	if err := refactor.ExtractFunction(opts); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("extract_function failed: %v", err)), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("Successfully extracted lines %d-%d into function '%s'", int(startLine), int(endLine), newName)), nil
}

func handleExtractInterface(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	filePath, _ := req.Params.Arguments["file_path"].(string)
	structName, _ := req.Params.Arguments["struct_name"].(string)
	destPath, _ := req.Params.Arguments["dest_file_path"].(string)
	interfaceName, _ := req.Params.Arguments["interface_name"].(string)

	opts := refactor.ExtractIfaceOptions{
		FilePath:      filePath,
		StructName:    structName,
		DestFilePath:  destPath,
		InterfaceName: interfaceName,
	}

	if err := refactor.ExtractInterface(opts); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("extract_interface failed: %v", err)), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("Successfully generated interface '%s' for struct '%s'", interfaceName, structName)), nil
}

func handleImplementInterface(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	filePath, _ := req.Params.Arguments["file_path"].(string)
	structName, _ := req.Params.Arguments["struct_name"].(string)
	interfaceName, _ := req.Params.Arguments["interface_name"].(string)

	opts := refactor.ImplIfaceOptions{
		FilePath:      filePath,
		StructName:    structName,
		InterfaceName: interfaceName,
	}

	if err := refactor.ImplementInterface(opts); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("implement_interface failed: %v", err)), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("Successfully generated method stubs for interface '%s' on struct '%s'", interfaceName, structName)), nil
}

func handleAddStructTags(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	filePath, _ := req.Params.Arguments["file_path"].(string)
	structName, _ := req.Params.Arguments["struct_name"].(string)

	var tags []string
	if rawTags, ok := req.Params.Arguments["tags"].(string); ok && rawTags != "" {
		for _, t := range strings.Split(rawTags, ",") {
			trimmed := strings.TrimSpace(t)
			if trimmed != "" {
				tags = append(tags, trimmed)
			}
		}
	}

	opts := refactor.AddStructTagsOptions{
		FilePath:   filePath,
		StructName: structName,
		Tags:       tags,
	}

	if err := refactor.AddStructTags(opts); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("add_struct_tags failed: %v", err)), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("Successfully added/updated struct tags on '%s'", structName)), nil
}

func handleTidyImports(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	filePath, _ := req.Params.Arguments["file_path"].(string)

	if err := refactor.TidyImports(filePath); err != nil {
		return mcp.NewToolResultError(fmt.Sprintf("tidy_imports failed: %v", err)), nil
	}
	return mcp.NewToolResultText(fmt.Sprintf("Successfully formatted and tidied imports in '%s'", filePath)), nil
}

func handleAnalyzeShadowing(_ context.Context, req mcp.CallToolRequest) (*mcp.CallToolResult, error) {
	targetPath, _ := req.Params.Arguments["target_path"].(string)

	issues, err := refactor.AnalyzeShadowing(targetPath)
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
