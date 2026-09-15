# Implementation Plan: Streamline MCP Toolset & Directory Cyclic Dependency Checking

Focus the MCP server exclusively on its three core, high-leverage refactoring tools (`rename_symbol`, `move_file`, and `move_directory`), drop unused tools (`implement_interface`, `analyze_shadowing`), and add robust cyclic dependency checking to `move_directory` (matching `move_file`).

---

## User Direction
- Drop tools 4 (`implement_interface`) and 5 (`analyze_shadowing`).
- Implement Option B: Add cyclic dependency checking to `move_directory` to match `move_file`, and call it done.
- Maintain strict quality gates: 100% test pass, 0 linter issues (`golangci-lint`), 0 security issues (`gosec`), `govulncheck`.

---

## Proposed Changes

### 1. Drop Unused MCP Tools
#### [`internal/server/server.go`](internal/server/server.go)
- Remove `implement_interface` and `analyze_shadowing` tool definitions and handlers from `NewServer()`.
- Expose only `rename_symbol`, `move_file`, and `move_directory`.

#### [`internal/refactor/types.go`](internal/refactor/types.go)
- Remove `ShadowIssue` struct.

#### Remove Obsolete Files
- Delete `internal/refactor/impl_iface.go` and `internal/refactor/shadow.go`.
- Delete obsolete test suites:
  - `tests/embedded_interface_test.go`
  - `tests/implement_interface_params_test.go`
  - `tests/implement_interface_stdlib_test.go`
  - `tests/shadow_analysis_test.go`
  - `tests/third_party_interface_test.go`

#### [`tests/mcp_integration_test.go`](tests/mcp_integration_test.go) & [`internal/server/server_test.go`](internal/server/server_test.go)
- Remove integration and server unit tests for dropped tools.
- Clean unused imports.

---

### 2. Cyclic Dependency Checking in `move_directory`
#### [`internal/refactor/move_dir.go`](internal/refactor/move_dir.go)
- Implement `checkDirectoryCyclicDependency(ws *Workspace, absSource, oldImportPath, newImportPath, buildTags string) error`:
  - Loads workspace packages via `ws.LoadAllPlatformPackages` (with fallback to `ws.LoadPackages`).
  - Scans all Go files in `absSource` (and subdirectories) to collect external package imports.
  - Checks if moving to `newImportPath` creates:
    1. A direct self-dependency (`impPath == newImportPath`).
    2. A transitive cycle (`hasDependency(pkgs, impPath, newImportPath)`).
    3. An inverted cycle where `newImportPath` already depends on `oldImportPath` (`hasDependency(pkgs, newImportPath, oldImportPath)`).
  - Aborts before any filesystem modification occurs and returns actionable error message.

---

### 3. Verification & Tests
#### [`tests/move_directory_cycle_test.go`](tests/move_directory_cycle_test.go)
- `TestMoveDirectory_DetectsTransitiveCyclicDependency`: Validates detection and rollback when a transitive cycle is formed.
- `TestMoveDirectory_DetectsDirectSelfDependency`: Validates detection when source imports target directly.
- `TestMoveDirectory_DetectsDestinationAlreadyImportsSource`: Validates detection when destination package already imports source.
- `TestMoveDirectory_MCPTool_CyclicDependencyError`: Validates error result propagation over MCP JSON-RPC protocol.

#### [`README.md`](README.md)
- Update MCP toolset table to feature the 3 core tools with parameters and capabilities.

---

## Verification Plan

1. **Unit & Integration Tests**:
   ```bash
   go test -v ./...
   ```
2. **Linter Verification**:
   ```bash
   golangci-lint run
   ```
3. **Security Audit**:
   ```bash
   gosec ./...
   ```
4. **Vulnerability Audit**:
   ```bash
   govulncheck ./...
   ```
