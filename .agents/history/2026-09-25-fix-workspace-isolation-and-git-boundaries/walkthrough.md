# Walkthrough: Workspace Boundary Isolation & Git Boundary Guards

## Summary of Changes

Fixed a critical workspace discovery bug where refactoring tools (`rename_symbol`, `move_file`, `move_directory`) climbed out of standalone Go modules into parent directories and scanned across sibling git repositories, erroneously grouping independent projects into a single phantom multi-module monorepo.

### Root Cause & Resolutions

1. **Standalone Go Module Climbing Prevention (`internal/refactor/workspace.go`)**:
   - `searchWorkspaceRoots` previously checked `if workRoot == "" && len(modRoots) <= 1` and, when `len(modRoots) == 1`, set `searchBase = filepath.Dir(modRoots[0])`, scanning the parent directory.
   - Fixed by removing parent climbing. Sibling module discovery is now strictly restricted to `workRoot == "" && len(modRoots) == 0` (i.e. caller pointed tool directly at a monorepo root without a root `go.mod`).
   - The upward walk records only the innermost enclosing `go.mod` and halts if it hits a `.git` boundary without a `go.work`.

2. **Git Repository Boundary Guards (`internal/refactor/workspace.go` & `internal/refactor/types.go`)**:
   - Implemented `hasGitBoundary(dir string) bool` checking for `.git` (handling directory repositories, git submodules, and worktrees).
   - In `discoverSubModules`: Any directory where `path != baseDir && hasGitBoundary(path)` is immediately skipped via `filepath.SkipDir`. Sibling git repositories are never traversed or added as sub-modules.
   - In `WalkGoFiles` and `DiscoverWorkspaceBuildTags`: Subdirectories containing `.git` boundaries are skipped to avoid indexing foreign git repositories or submodules.

3. **Explicit Workspace Root Configuration (`internal/refactor/move_dir.go`, `internal/refactor/move_file.go`, `internal/server/server.go`)**:
   - Added optional `Dir` field to `MoveDirOptions` and `MoveFileOptions`.
   - Exposed optional `directory` parameter on `move_file` and `move_directory` MCP tools matching `rename_symbol`.
   - When specified, `FindWorkspace` uses this explicit root; otherwise it defaults to the file/directory location.

4. **Integration & Regression Tests (`tests/workspace_isolation_test.go`, `tests/monorepo_multi_mod_test.go`)**:
   - Added `tests/workspace_isolation_test.go` covering:
     - `TestStandaloneModuleDoesNotClimbToParentOrSiblings`
     - `TestDiscoverSubModulesNeverCrossesGitBoundaries`
     - `TestMonorepoWithGitRootAndSubmodules`
     - `TestRenameSymbolIsolationBetweenSiblingRepos`
   - Updated `tests/monorepo_multi_mod_test.go` to provide `Dir: tempDir` for explicit monorepo directory moves.

## Verification Results

### Unit Test Suite
```bash
go test ./...
```
Output:
```
ok  	github.com/joangrigorov/go-refactor-mcp/internal/server	0.082s
ok  	github.com/joangrigorov/go-refactor-mcp/tests	28.323s
```
All unit tests and integration tests pass cleanly.

### Linter Verification
```bash
golangci-lint run
```
Output:
```
0 issues.
```

### Security Audit
```bash
gosec ./...
```
Output:
```
Summary:
  Files  : 9
  Lines  : 2298
  Nosec  : 8
  Issues : 0
```
