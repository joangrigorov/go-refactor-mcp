# Walkthrough: Mandatory Directory Parameter & Workspace Ceiling Safeguards

Enforced `directory` (workspace root) as a mandatory parameter across all MCP refactoring tools (`rename_symbol`, `move_file`, `move_directory`) and implemented strict boundary and ceiling validation safeguards to prevent refactoring operations from climbing outside or modifying files outside the designated workspace.

## Breaking Changes Summary
- **All MCP Tools Require `directory`**: `move_file` and `move_directory` now require `directory` alongside `rename_symbol`.
- **Options Structs Require `Dir`**: `refactor.MoveFileOptions.Dir`, `refactor.MoveDirOptions.Dir`, and `refactor.RenameOptions.Dir` are now required fields.
- **Strict Boundary Enforcement**: Passing source files, destination directories, or target files that resolve outside the specified `directory` causes operations to abort immediately before package loading or disk mutations.
- **Ceiling Safeguard**: Workspace discovery now respects an impenetrable ceiling at `ceilingDir`, preventing upward traversal past the declared workspace root even if parent `go.work` or `go.mod` files exist.

---

## Key Changes

### 1. Workspace Ceiling Safeguard (`internal/refactor/workspace.go`)
- Introduced `FindWorkspaceWithCeiling(startDir, ceilingDir string) (*Workspace, error)`:
  - Verifies that `startDir` is contained within `ceilingDir`.
  - Halts upward directory climbing in `searchWorkspaceRoots(startDir, ceilingDir)` the moment `curr == ceilingDir`, ensuring parent directories are never inspected.
  - Filters `go.work` module entries in `loadGoWorkWorkspace` to exclude any module paths located outside `ceilingDir`.

### 2. Mandatory Directory & Boundary Checks in Refactoring Engines
- **`internal/refactor/move_file.go`**:
  - Requires `opts.Dir` and validates that it exists and is a directory.
  - Resolves relative `SourceFile` and `DestDir` paths against `absDir`.
  - Validates that both `absSource` and `absDestDir` reside within `absDir`.
  - Discovers the workspace using `FindWorkspaceWithCeiling(absDir, absDir)`.
  - Extracted `validateMoveFilePaths` helper to keep cyclomatic complexity (`gocyclo`) well below thresholds.
- **`internal/refactor/move_dir.go`**:
  - Requires `opts.Dir` and validates directory existence.
  - Resolves relative `SourceDir` and `DestDir` against `absDir`.
  - Validates that both `absSource` and `absDest` reside within `absDir`.
  - Discovers the workspace using `FindWorkspaceWithCeiling(absDir, absDir)`.
- **`internal/refactor/rename.go`**:
  - Requires `opts.Dir` and validates directory existence.
  - Resolves relative `File` against `absDir` and verifies it resides within `absDir`.
  - Discovers the workspace using `FindWorkspaceWithCeiling(absDir, absDir)`.
  - Added defense-in-depth in `applyASTRename` to guarantee that no file outside `opts.Dir` can ever be modified.

### 3. Server Tool Registration & Validation (`internal/server/server.go`)
- Configured `directory` as `mcp.Required()` in `move_file` and `move_directory` MCP tool declarations.
- Updated `handleMoveFile` and `handleMoveDirectory` to validate `directory != ""` and return clear, actionable error messages if missing.

### 4. Documentation & Test Suite
- Updated `README.md` tool table to show `directory` as a required parameter across all 3 tools.
- Updated MCP server unit tests in `internal/server/server_test.go` and integration tests in `tests/mcp_integration_test.go` to verify required `directory` validation and boundary error surfacing.
- Updated all existing integration and unit tests in `tests/` to provide `Dir: tempDir`.
- Added `TestCeilingBoundaryEnforcement` and `TestBoundaryValidationErrors` in `tests/workspace_isolation_test.go`.

---

## Verification Results

### Unit Tests
```bash
go test -count=1 ./...
```
All unit and integration tests across all packages passed cleanly in ~8.4s.

### Linter (`golangci-lint`)
```bash
golangci-lint run
```
0 issues reported. Cyclomatic complexity (`gocyclo`), variable shadowing (`govet`), and error checking (`errcheck`) passed with 0 warnings.

### Security Analysis (`gosec`)
```bash
gosec ./...
```
0 security issues found across all packages.
