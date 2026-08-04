# go-refactor-mcp

A production-grade, stateless Model Context Protocol (MCP) server written in Go that provides LLMs with deterministic, stateless, and comprehensive Go refactoring tools directly manipulating Go AST and package graphs.

[![CI](https://github.com/joangrigorov/go-refactor-mcp/actions/workflows/ci.yml/badge.svg)](https://github.com/joangrigorov/go-refactor-mcp/actions/workflows/ci.yml)
[![Go Version](https://img.shields.io/github/go-mod/go-version/joangrigorov/go-refactor-mcp)](go.mod)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](LICENSE)

---

## 💡 Token Efficiency & Reduced Debug Loops

When AI agents perform refactoring manually via search-and-replace, they often miss package references, imports, or aliased types across large codebases. This triggers tedious `build -> fail -> fix` error loops that burn through tokens and context window.

`go-refactor-mcp` performs deterministic, AST-level refactoring in a single step—updating all references across your entire module cleanly without extra diagnostic loops.

---

## 🛠️ Comprehensive MCP Toolset

`go-refactor-mcp` exposes 5 production-grade refactoring tools over standard STDIO (JSON-RPC):

| Tool Name | Parameters | Description |
|---|---|---|
| `rename_symbol` | `directory` (string, required)<br>`from` (string, required)<br>`to` (string, required)<br>`file` (string, optional)<br>`offset` (number, optional) | Renames a variable, function, struct, interface, package, or generic type parameter across the entire module. Handles cross-package, aliased, and dot-imported symbols. |
| `move_file` | `source_file` (string, required)<br>`dest_dir` (string, required) | Moves a `.go` file (and associated `_test.go`) to a new directory. Preserves `//go:build` tags, updates package clauses, updates import paths, and aborts if an import cycle is detected. |
| `move_directory` | `source_dir` (string, required)<br>`dest_dir` (string, required) | Moves an entire package directory and updates all import paths referencing this package and its sub-packages workspace-wide. |
| `implement_interface` | `file_path` (string, required)<br>`struct_name` (string, required)<br>`interface_name` (string, required) | Generates missing method stubs for a struct to satisfy an interface (e.g. `io.Reader`, `fmt.Stringer`, or local interface). |
| `analyze_shadowing` | `target_path` (string, required) | Scans a file or package directory for shadowed variables, returning structured reports to prevent logic bugs. |

---

## ⚡ Installation & MCP Client Configuration

Build the binary locally:

```bash
git clone https://github.com/joangrigorov/go-refactor-mcp.git
cd go-refactor-mcp
go build -o go-refactor-mcp main.go
```

### 1. Claude Desktop

Add the server to your `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "go-refactor": {
      "command": "/path/to/go-refactor-mcp/go-refactor-mcp",
      "args": []
    }
  }
}
```

### 2. Cursor

1. Open **Cursor Settings** -> **Features** -> **MCP Servers**.
2. Click **+ Add New MCP Server**.
3. Set **Name**: `go-refactor`.
4. Set **Type**: `stdio`.
5. Set **Command**: `/path/to/go-refactor-mcp/go-refactor-mcp`.

### 3. agy (Antigravity CLI)

In your `~/.gemini/antigravity-cli/mcp_config.json` or project settings:

```json
{
  "mcpServers": {
    "go-refactor": {
      "command": "/path/to/go-refactor-mcp/go-refactor-mcp"
    }
  }
}
```

Or mount via CLI:

```bash
agy mcp add go-refactor -- /path/to/go-refactor-mcp/go-refactor-mcp
```

### 4. Grok Build

Add the server configuration to `.grok/config.toml`:

```toml
[mcp_servers.go-refactor]
command = "/path/to/go-refactor-mcp/go-refactor-mcp"
```

For standard MCP clients operating over STDIO, point your client configuration directly to the built `/path/to/go-refactor-mcp/go-refactor-mcp` binary.

---

## 🧪 Testing & Code Quality

Run the comprehensive integration test suite:

```bash
go test -v ./...
```

Run static analysis & vulnerability scanning:

```bash
golangci-lint run
govulncheck ./...
```

---

## 📄 License

MIT License. See [LICENSE](LICENSE) for details.
