# Fix STDIO Handshake Hang on Modern MCP Protocol & Help Text Cleanup

## Summary

In `v0.4.0`, the `go-refactor-mcp` server became stuck at `'initializing'` when launched by modern MCP clients such as Antigravity (`agy -c`). In addition, `--help` still listed the two deprecated tools (`implement_interface`, `analyze_shadowing`).

This fix resolves the root cause in the STDIO transport layer, cleans up the CLI help text, supports dynamic build version injection via GoReleaser, and adds automated test coverage for the modern MCP handshake protocol.

---

## Root Cause Analysis

### 1. The STDIO Initialization Hang
1. **Modern MCP Protocol Handshake (SEP-2575)**:
   In commit `880332d`, `github.com/mark3labs/mcp-go` was upgraded from `v0.58.0` to `v1.0.0`. `v1.0.0` added support for protocol version `2026-07-28` (`server/discover` and `subscriptions/listen`).
2. **Default Tool Capabilities**:
   When tools are registered on `server.NewMCPServer(...)` without explicit capability configuration, `mcp-go` defaults `capabilities.tools.listChanged` to `true`.
3. **Subscriptions Request Fan-out**:
   Modern clients detecting `listChanged: true` immediately dispatch a `subscriptions/listen` request for `toolsListChanged`, followed by `tools/list`.
4. **Blocking STDIO Loop**:
   In `mcp-go@v1.0.0/server/subscriptions.go`:
   ```go
   if !allowed.IsEmpty() {
       <-ctx.Done()
   }
   ```
   When `toolsListChanged` is allowed, `handleSubscriptionsListen` blocks indefinitely on `<-ctx.Done()`. In `server/stdio.go`, all non-tool calls are processed synchronously within the main input stream loop:
   ```go
   response := s.server.HandleMessage(ctx, rawMessage)
   ```
   Because `handleSubscriptionsListen` blocked the reader loop, `tools/list` was never read from `stdin`, leaving the client waiting forever for tool discovery.

### 2. Help Text & Release Versioning
- `printHelp()` in `main.go` was still displaying descriptions for `implement_interface` and `analyze_shadowing`.
- `const version = "1.0.0"` in `main.go` could not be overridden by GoReleaser's `-ldflags "-X main.version={{.Version}}"`.

---

## Key Changes

### 1. Disable `tools.listChanged` on Server ([`internal/server/server.go`](internal/server/server.go))
- Initialized server with `server.WithToolCapabilities(false)`:
  ```go
  func NewServer(version ...string) *server.MCPServer {
      v := "1.0.0"
      if len(version) > 0 && version[0] != "" {
          v = version[0]
      }
      s := server.NewMCPServer(
          "go-refactor-mcp",
          v,
          server.WithToolCapabilities(false),
      )
      registerTools(s)
      return s
  }
  ```
- Because `go-refactor-mcp` is a stateless refactoring server with a fixed toolset, `listChanged=false` accurately reflects its semantics.
- When `listChanged=false`, `allowed.IsEmpty()` is `true`, allowing `subscriptions/listen` to respond immediately with a success acknowledgement instead of blocking the STDIO loop.

### 2. CLI Help & Version Metadata ([`main.go`](main.go))
- Removed `implement_interface` and `analyze_shadowing` from `printHelp()`.
- Changed `const version` to package-level `var (version, commit, date)` so GoReleaser can inject git tag, commit hash, and build timestamp via `-ldflags`.
- Updated version flag to display full build metadata: `go-refactor-mcp version <version> (commit: <commit>, built at: <date>)`.

### 3. Automated Modern Handshake Tests ([`tests/cli_test.go`](tests/cli_test.go))
- Updated `TestCLIHelpAndVersion` with negative assertions (`unexpectedOutput`) verifying that deprecated tools do not appear in help text.
- Added `TestMCPModernProtocolHandshake` which executes the full protocol sequence (`server/discover` -> `subscriptions/listen` -> `tools/list`) over STDIO pipes with timeout guards to guarantee that the server never blocks or deadlocks during client initialization.

---

## Verification

### 1. Full Local Verification Suite
- **Unit & Integration Tests**:
  ```bash
  go test -v ./...
  ```
  Result: All 42 tests passed across `internal/server` and `tests/`.
- **Linter**:
  ```bash
  golangci-lint run
  ```
  Result: `0 issues`.
- **Security Audit**:
  ```bash
  gosec ./...
  ```
  Result: `Files: 9, Lines: 2259, Issues: 0`.

### 2. Live Client Verification with `agy`
- Compiled and verified the fixed binary with `agy -c`.
- Result: `go-refactor` connected instantly with zero timeout warnings in `~/.gemini/antigravity-cli/cli.log`.
- Inspected `~/.gemini/antigravity-cli/mcp/go-refactor/`:
  - `rename_symbol.json` (created)
  - `move_file.json` (created)
  - `move_directory.json` (created)
  - Deprecated tool schemas completely cleared.
