# Implementation Plan: Workspace Boundary Isolation and Sibling Repositories Guard

## Problem Statement

When refactoring tools (`rename_symbol`, `move_file`, `move_directory`) are executed within a standalone Go module (such as a session in a project workspace), `searchWorkspaceRoots` in `internal/refactor/workspace.go` previously climbed out of the module to its parent directory `filepath.Dir(modRoots[0])` when no `go.work` file was present.

Additionally, `discoverSubModules` walked up to 3 levels deep from the search base and entered sibling git repositories, grouping them into a phantom multi-module monorepo and applying AST modifications across independent repositories.

## Requirements

1. **Do not climb out of standalone Go module**:
   - If `startDir` already has an enclosing `go.mod` (`len(modRoots) == 1`), stop. Do not climb to `filepath.Dir(modRoots[0])` unless an explicit `go.work` is present.
   - Sibling discovery should only run if `startDir` had no `go.mod` (`len(modRoots) == 0`, e.g. caller pointed tool directly at monorepo root without a root `go.mod`).
2. **Never cross `.git` boundaries**:
   - When walking directories during `discoverSubModules`, `WalkGoFiles`, and `DiscoverWorkspaceBuildTags`, detect `.git` boundaries (both directory `.git` and file `.git` for git submodules / worktrees) and return `filepath.SkipDir`.
   - Never enter or treat sibling git repositories as sub-modules of another workspace.
   - Halt the upward directory walk in `searchWorkspaceRoots` if a `.git` repository boundary is encountered without a `go.work`.
3. **Explicit Monorepo Root for Refactoring Tools**:
   - Provide an optional `directory` parameter on `move_file` and `move_directory` (matching `rename_symbol`) so callers operating in monorepos can explicitly designate the workspace root if needed.

## Proposed Changes

### `internal/refactor/workspace.go`
- Introduce `hasGitBoundary(dir string) bool` to detect if `dir/.git` exists.
- In `searchWorkspaceRoots`:
  - Stop recording parent `go.mod` files beyond the innermost enclosing module (`len(modRoots) == 0`).
  - Halt upward climbing upon encountering `hasGitBoundary(curr)` (unless `go.work` is present at that root).
  - Restrict sub-module discovery strictly to `workRoot == "" && len(modRoots) == 0`.
- In `discoverSubModules`:
  - When `path != baseDir && hasGitBoundary(path)`, immediately `return filepath.SkipDir`.
- In `WalkGoFiles`:
  - When `path != mod.Root && hasGitBoundary(path)`, immediately `return filepath.SkipDir`.

### `internal/refactor/types.go`
- In `DiscoverWorkspaceBuildTags`:
  - When `path != dir && hasGitBoundary(path)`, immediately `return filepath.SkipDir`.

### `internal/refactor/move_dir.go` and `internal/refactor/move_file.go`
- Add `Dir string` to `MoveDirOptions` and `MoveFileOptions`.
- Use `opts.Dir` if specified as the workspace search base.

### `internal/server/server.go`
- Expose optional `directory` argument in `move_file` and `move_directory` MCP tools.

### `tests/monorepo_multi_mod_test.go`
- Pass `Dir: tempDir` in `opts` when invoking `MoveDirectory` across monorepo modules.

### `tests/workspace_isolation_test.go`
- Comprehensive unit test suite covering:
  - Standalone module does not climb to parent or siblings.
  - Submodule discovery never crosses `.git` boundaries.
  - Monorepo with git root discovers submodules and skips embedded git submodules.
  - `rename_symbol` isolation between sibling repositories sharing symbol names.

## Verification Plan

1. `go test -v ./...` - all tests pass.
2. `golangci-lint run` - 0 issues.
3. `gosec ./...` - 0 issues.
4. `govulncheck ./...` - security check.
