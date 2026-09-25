# Implementation Plan: Mandatory Directory Parameter & Workspace Ceiling Safeguards

## Overview
Enforce `directory` (workspace root) as a mandatory parameter across all MCP refactoring tools (`rename_symbol`, `move_file`, and `move_directory`) and introduce strict boundary and ceiling safeguards. This prevents refactoring operations from climbing above the workspace root or mutating files outside the designated workspace.

## Breaking Changes
1. `move_file`: `directory` argument is now required (was previously optional).
2. `move_directory`: `directory` argument is now required (was previously optional).
3. `refactor.MoveFileOptions.Dir`: required.
4. `refactor.MoveDirOptions.Dir`: required.
5. `refactor.RenameOptions.Dir`: required.
6. Strict boundary validation: Passing `source_file`, `dest_dir`, `source_dir`, or `file` that resolves outside `directory` returns an immediate boundary error.
7. Upward ceiling safeguard: `FindWorkspaceWithCeiling` terminates upward discovery at `ceilingDir`, preventing discovery of parent `go.work` or `go.mod` files.

## Proposed Changes

### `internal/refactor/workspace.go`
- Add `FindWorkspaceWithCeiling(startDir, ceilingDir string) (*Workspace, error)`.
- Update `searchWorkspaceRoots(startDir, ceilingDir string)` to immediately break upward climbing when reaching `ceilingDir`.
- Filter modules in `loadGoWorkWorkspace` to exclude any module paths located outside `ceilingDir`.

### `internal/refactor/move_file.go`
- Require `opts.Dir`. Validate directory existence.
- Resolve relative `SourceFile` and `DestDir` against `opts.Dir`.
- Validate that `absSource` and `absDestDir` reside within `opts.Dir`.
- Use `FindWorkspaceWithCeiling(absDir, absDir)`.
- Extract `validateMoveFilePaths` helper to maintain low cyclomatic complexity (`gocyclo`).

### `internal/refactor/move_dir.go`
- Require `opts.Dir`. Validate directory existence.
- Resolve relative `SourceDir` and `DestDir` against `opts.Dir`.
- Validate that `absSource` and `absDest` reside within `opts.Dir`.
- Use `FindWorkspaceWithCeiling(absDir, absDir)`.

### `internal/refactor/rename.go`
- Require `opts.Dir`. Validate directory existence.
- Resolve relative `File` against `opts.Dir` and validate it resides within `opts.Dir`.
- Use `FindWorkspaceWithCeiling(absDir, absDir)`.
- Defense-in-depth: Ensure `applyASTRename` never writes to any file outside `opts.Dir`.

### `internal/server/server.go`
- Set `mcp.Required()` for `directory` on `move_file` and `move_directory`.
- Validate `dir != ""` in `handleMoveFile` and `handleMoveDirectory`, returning actionable error messages.

### Documentation & Tests
- Update `README.md` tool table to show `directory` as required for all tools.
- Update `internal/server/server_test.go` and `tests/mcp_integration_test.go` to verify required directory validation and boundary errors.
- Update all existing test suites to pass `Dir: tempDir`.
- Add `TestCeilingBoundaryEnforcement` and `TestBoundaryValidationErrors` in `tests/workspace_isolation_test.go`.

## Verification Plan
1. Unit tests: `go test -count=1 ./...`
2. Linter: `golangci-lint run` (0 issues)
3. Security analysis: `gosec ./...` (0 issues)
4. Vulnerability check: `govulncheck ./...`
5. Privacy audit: Ensure no private project names or local home paths exist in repository.
