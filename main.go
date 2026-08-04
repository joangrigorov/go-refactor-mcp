// Package main is the entrypoint for go-refactor-mcp server.
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/joangrigorov/go-refactor-mcp/internal/server"
	mcpServer "github.com/mark3labs/mcp-go/server"
)

const version = "1.0.0"

func main() {
	helpFlag := flag.Bool("help", false, "Show help message")
	flag.BoolVar(helpFlag, "h", false, "Show help message (shorthand)")
	versionFlag := flag.Bool("version", false, "Show version information")
	flag.BoolVar(versionFlag, "v", false, "Show version information (shorthand)")

	flag.Usage = printHelp

	flag.Parse()

	if *helpFlag || isHelpArg() {
		printHelp()
		os.Exit(0)
	}

	if *versionFlag || isVersionArg() {
		fmt.Printf("go-refactor-mcp version %s\n", version)
		os.Exit(0)
	}

	s := server.NewServer()

	if err := mcpServer.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}

func isHelpArg() bool {
	if len(os.Args) > 1 {
		arg := os.Args[1]
		return arg == "help" || arg == "-help" || arg == "--help" || arg == "-h"
	}
	return false
}

func isVersionArg() bool {
	if len(os.Args) > 1 {
		arg := os.Args[1]
		return arg == "version" || arg == "-version" || arg == "--version" || arg == "-v"
	}
	return false
}

func printHelp() {
	helpText := fmt.Sprintf(`go-refactor-mcp - Model Context Protocol (MCP) Go Refactoring Server v%s

Usage:
  go-refactor-mcp [flags]
  go-refactor-mcp [command]

Description:
  go-refactor-mcp is a production-grade, stateless Model Context Protocol server.
  It provides LLMs with deterministic Go refactoring tools via JSON-RPC over STDIO.

Flags:
  -h, --help     Show this help message and exit
  -v, --version  Show version information and exit

Available MCP Tools:
  - rename_symbol       Renames a symbol across the entire module.
  - move_file           Moves a .go file (and _test.go), updating package/imports.
  - move_directory      Moves a package directory and updates module import paths.
  - implement_interface Generates missing interface method stubs for a struct.
  - analyze_shadowing   Scans for shadowed variables in files or packages.

For MCP Host Configuration (Claude Desktop, Cursor, agy, Grok), see README.md.
`, version)
	fmt.Print(helpText)
}
