# Streamline MCP Toolset & Add Directory Cyclic Dependency Checking

## Summary

This change streamlines the `go-refactor-mcp` server to focus strictly on its three core, high-leverage AST refactoring tools:
1. `rename_symbol`: Semantic symbol renaming across modules, aliased imports, and foreign build targets.
2. `move_file`: Single-file/companion-test refactoring with AST import rewriting, deduplication, and cycle detection.
3. `move_directory`: Full package directory relocation with cross-workspace import rewriting and cyclic dependency prevention.

Tools 4 (`implement_interface`) and 5 (`analyze_shadowing`) have been completely removed from the server, refactor package, and test suite. In addition, `move_directory` now includes comprehensive cyclic dependency checking prior to executing file operations on disk, bringing its safety guardrails in line with `move_file`.

---

## Key Changes

### 1. Streamline MCP Server to Core 3 Tools
- **MCP Server Registration ([`internal/server/server.go`](internal/server/server.go))**:
  - Removed tool registrations and handlers for `implement_interface` and `analyze_shadowing`.
  - Exposes only `rename_symbol`, `move_file`, and `move_directory`.
- **Refactor Package Cleanup ([`internal/refactor/types.go`](internal/refactor/types.go))**:
  - Removed `ShadowIssue` struct.
  - Deleted `internal/refactor/impl_iface.go` and `internal/refactor/shadow.go`.
- **Test Suite Cleanup**:
  - Removed obsolete test suites: `embedded_interface_test.go`, `implement_interface_params_test.go`, `implement_interface_stdlib_test.go`, `shadow_analysis_test.go`, and `third_party_interface_test.go`.
  - Updated [`internal/server/server_test.go`](internal/server/server_test.go) and [`tests/mcp_integration_test.go`](tests/mcp_integration_test.go) to test only the 3 core tools.
- **Documentation ([`README.md`](README.md))**:
  - Updated the MCP toolset table to feature the 3 core tools, their parameters, and their capabilities.

### 2. Cyclic Dependency Prevention in `move_directory`
- **Dependency Graph Inspection ([`internal/refactor/move_dir.go`](internal/refactor/move_dir.go))**:
  - Implemented `checkDirectoryCyclicDependency(ws, absSource, oldImportPath, newImportPath, buildTags)` called before any disk modifications.
  - Uses `ws.LoadAllPlatformPackages` (with fallback to `ws.LoadPackages`) to inspect the entire workspace package graph across target platforms.
  - Detects:
    1. Direct self-dependencies where files in the source directory already import the destination package path.
    2. Transitive dependency cycles where an imported package (or its downstream dependencies) depends on the destination package path (`hasDependency(pkgs, impPath, newImportPath)`).
    3. Inverted cycles where an existing destination package already depends on the source package path (`hasDependency(pkgs, newImportPath, oldImportPath)`).
  - Rejects invalid operations before any files are moved or imports rewritten, preserving workspace integrity.

### 3. Comprehensive Test Coverage
- **New Test Suite ([`tests/move_directory_cycle_test.go`](tests/move_directory_cycle_test.go))**:
  - `TestMoveDirectory_DetectsTransitiveCyclicDependency`: Verifies transitive cycle detection (`pkgA -> pkgC -> pkgB`) when attempting to move `pkgA` into `pkgB`.
  - `TestMoveDirectory_DetectsDirectSelfDependency`: Verifies direct cycle detection when source files already import the target package path.
  - `TestMoveDirectory_DetectsDestinationAlreadyImportsSource`: Verifies cycle detection when destination package already depends on the source package.
  - `TestMoveDirectory_MCPTool_CyclicDependencyError`: Verifies that cyclic dependency errors are properly surfaced over the MCP JSON-RPC protocol as actionable error results.

---

## Verification Results

### 1. Unit & Integration Test Suite
```bash
go test -v ./...
```
Output:
```
ok  	github.com/joangrigorov/go-refactor-mcp/internal/server	0.009s
ok  	github.com/joangrigorov/go-refactor-mcp/tests	6.562s
```
All tests passed across all packages.

### 2. Linter Verification
```bash
golangci-lint run
```
Output: `0 issues.`

### 3. Security Analysis
```bash
gosec ./...
```
Output: `Files: 9, Lines: 2244, Issues: 0`

### 4. Vulnerability Audit
```bash
govulncheck ./...
```
Audit completed cleanly with no vulnerable project code or third-party dependencies called.
