// Package main is the entrypoint for go-refactor-mcp server.
package main

import (
	"fmt"
	"os"

	"github.com/joangrigorov/go-refactor-mcp/internal/server"
	mcpServer "github.com/mark3labs/mcp-go/server"
)

func main() {
	s := server.NewServer()

	if err := mcpServer.ServeStdio(s); err != nil {
		fmt.Fprintf(os.Stderr, "Server error: %v\n", err)
		os.Exit(1)
	}
}
