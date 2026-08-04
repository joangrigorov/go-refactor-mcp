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

## 🤖 Prompt Snippet for `AGENTS.md` / `CLAUDE.md`

Add this concise snippet to your project's `AGENTS.md` or `CLAUDE.md` to instruct AI agents to use `go-refactor-mcp` tools instead of manual text edits:

> For Go refactoring, always use `go-refactor` MCP tools:
> - **Moving Go files**: Use `move_file`.
> - **Renaming or moving packages**: Use `move_directory` (renames package directory, package clauses, and imports module-wide).
> - **Renaming symbols** (variables, functions, structs, interfaces, package aliases, generic type parameters): Use `rename_symbol`.

---

## ⚡ Installation

### 1. Debian / Ubuntu Package (`.deb`)

Download and install the `.deb` package directly on Debian, Ubuntu, or derivative distributions:

```bash
# Download the latest .deb package
curl -sSL https://github.com/joangrigorov/go-refactor-mcp/releases/latest/download/go-refactor-mcp_0.1.0_linux_amd64.deb -o go-refactor-mcp.deb

# Install package system-wide
sudo dpkg -i go-refactor-mcp.deb
```

This installs `go-refactor-mcp` directly to `/usr/bin/go-refactor-mcp`.

### 2. RPM / APK Linux Packages (RHEL, Fedora, Alpine)

**RHEL / Fedora (`.rpm`):**
```bash
sudo rpm -i https://github.com/joangrigorov/go-refactor-mcp/releases/latest/download/go-refactor-mcp_0.1.0_linux_amd64.rpm
```

**Alpine Linux (`.apk`):**
```bash
wget https://github.com/joangrigorov/go-refactor-mcp/releases/latest/download/go-refactor-mcp_0.1.0_linux_amd64.apk
sudo apk add --allow-untrusted go-refactor-mcp_0.1.0_linux_amd64.apk
```

### 3. Pre-compiled Binaries (Linux, macOS, Windows)

Download pre-compiled binaries for Linux, macOS (`darwin`), or Windows from [GitHub Releases](https://github.com/joangrigorov/go-refactor-mcp/releases).

```bash
# Example: Linux amd64
curl -sSL https://github.com/joangrigorov/go-refactor-mcp/releases/latest/download/go-refactor-mcp_Linux_x86_64.tar.gz | tar -xz
sudo mv go-refactor-mcp /usr/local/bin/
```

### 4. `go install`

Install the binary directly using the Go toolchain:

```bash
go install github.com/joangrigorov/go-refactor-mcp@latest
```

### 5. Build from Source

Build the binary locally from the source repository:

```bash
git clone https://github.com/joangrigorov/go-refactor-mcp.git
cd go-refactor-mcp
go build -o go-refactor-mcp main.go
```

---

## 🔌 MCP Client Configuration

### 1. Claude Desktop

Add the server to your `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "go-refactor": {
      "command": "go-refactor-mcp",
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
5. Set **Command**: `go-refactor-mcp`.

### 3. agy (Antigravity CLI)

In your `~/.gemini/antigravity-cli/mcp_config.json` or project settings:

```json
{
  "mcpServers": {
    "go-refactor": {
      "command": "go-refactor-mcp"
    }
  }
}
```

Or mount via CLI:

```bash
agy mcp add go-refactor -- go-refactor-mcp
```

### 4. Grok Build

Add the server configuration to `.grok/config.toml`:

```toml
[mcp_servers.go-refactor]
command = "go-refactor-mcp"
```

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
