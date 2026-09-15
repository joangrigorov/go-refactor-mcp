# Go Compatibility & Refactoring Architectural Assessment

**Date**: September 15, 2026  
**Target Project**: `go-refactor-mcp`  
**Purpose**: Comprehensive investigation of Go setup flavors, compatibility limits, and architectural assessment for Phase 0 refactoring.

---

## Part 1: Go Ecosystem Compatibility Analysis

### Category 1: High Value & Implementable in this MCP

#### 1. `go.work` Workspaces & Multi-Module Monorepos
* **Current State**: 
  - `FindModuleRoot` stops at the first `go.mod` found walking upwards from the target file or directory.
  - `LoadModulePackagesWithTags` loads packages via `./...` strictly from that single module root.
  - If a workspace root with `go.work` is passed without a `go.mod`, it errors with `"go.mod not found"`.
  - `updateWorkspaceImports` and `updateWorkspaceMovedFileImports` walk only `moduleRoot`, leaving sibling modules broken.
  - Moving files/directories between modules produces illegal import paths (e.g. `moduleA/../moduleB/pkg`).
  - Cycle detection only checks within the source module.
* **Compatibility Target**:
  - Detect `go.work` workspace roots using standard `golang.org/x/mod/modfile.ParseWork`.
  - Map every file and package path to its owning module root.
  - Enable workspace-wide package loading via `packages.Load` with `GOWORK` support.
  - Allow cross-module moves with correct new module import paths.
  - Traverse all member modules when rewriting imports and references.

#### 2. Any Standard Library & Third-Party Interface in `implement_interface`
* **Current State**:
  - Only 5 standard library interfaces (`io.Reader`, `io.Writer`, `io.Closer`, `fmt.Stringer`, `error`) are hardcoded.
  - Local interfaces defined in the same module are resolved via manual AST parsing.
  - Any external third-party interface (e.g., `net/http.Handler`, `database/sql/driver.Valuer`, AWS SDK, Gin, gRPC, etc.) fails to resolve and generates a dummy fallback stub: `func (s *S) Handle<Name>() error { panic("unimplemented") }`.
  - Cannot resolve embedded interfaces (`type ReadCloser interface { Reader; Closer }`) or interface type aliases via AST parsing.
* **Compatibility Target**:
  - Replace AST string scraping with `golang.org/x/tools/go/packages` and `go/types`.
  - Load the interface's package and inspect `types.Interface` method sets dynamically.
  - Automatically resolve method names, parameters, and return types for any standard or third-party package without hardcoded lists.

#### 3. Preserving `package main` & Custom Package Names in `move_directory`
* **Current State**:
  - `move_dir.go` unconditionally renames all moved Go files to match the destination folder base name (`cleanDirPackageName`), unless ending in `_test`.
  - Moving executable command packages (e.g., `cmd/server`) forcefully rewrites `package main` to `package server`, breaking `go build`.
* **Compatibility Target**:
  - Preserve `package main` declarations when moving directories.
  - Respect existing intentional package naming conventions across files in the directory.

#### 4. Package Import Collisions & Auto-Aliasing in `move_file`
* **Current State**:
  - If a file is moved to a directory named `types`, and files referencing the moved symbols already import another package named `types`, a redeclaration collision occurs (`types redeclared in this block`).
* **Compatibility Target**:
  - Detect imported package name collisions in target files.
  - Automatically generate a unique alias (e.g. `domainTypes "..."`) and rewrite symbol selectors to use the generated alias.

---

### Category 2: Implementable via Multi-Pass Architecture

#### Cross-Platform / Multi-OS Setups (`_windows.go`, `_darwin.go`, OS Build Tags)
* **Current State**:
  - `standardGoBuildTags` in `types.go` filters out all OS/Arch tags (`windows`, `darwin`, `linux`, etc.) during auto-discovery.
  - `go/packages` evaluates packages matching the current host operating system.
  - When running on Linux/macOS, `rename_symbol` does not load platform-specific files for other OSes (`*_windows.go`), leaving their symbol references untouched and broken.
* **Compatibility Target**:
  - Detect platform file suffixes in the workspace.
  - Run multi-pass `packages.Load` with alternate `GOOS`/`GOARCH` environment configurations (`cfg.Env`).
  - Merge the AST modifications across platform passes before writing to disk.

---

### Category 3: Blocked by Language Rules or Not Recommended

1. **Receiver Methods on Foreign Types (`move_file`)**:
   - **Language Constraint**: Go forbids defining methods on a struct outside the package where the struct is declared (`cannot define new methods on non-local type`).
   - **Verdict**: Cannot generate valid Go code. The refactoring engine must validate this pre-condition and abort early with a descriptive error.

2. **Legacy `GOPATH` Mode (No `go.mod`)**:
   - **Status**: Deprecated since Go 1.16+.
   - **Verdict**: Not recommended. Modern Go tooling assumes Go modules.

3. **Alternative Build Systems without `go.mod` (Bazel, Buck)**:
   - **Constraint**: Pure Bazel repositories without `go.mod` or standard Go CLI tools require custom BSP/LSP drivers.
   - **Verdict**: Only supported if using `gazelle` alongside standard `go.mod`.

---

## Part 2: Architectural Assessment of `go-refactor-mcp`

### Technical Debt & Code Smells in Prior Implementation

| Smell / Debt | Files Affected | Description & Impact |
|---|---|---|
| **No Workspace / Module Domain Model** | `types.go`, `move_file.go`, `move_dir.go`, `rename.go` | Procedural functions (`FindModuleRoot`, `GetModuleName`) operate in isolation. No shared concept of workspace boundaries. |
| **Regex Parsing of `go.mod`** | `types.go` (`GetModuleName`) | Uses regex `(?m)^module\s+([^\s]+)` instead of official `golang.org/x/mod/modfile`. Vulnerable to comments, whitespace variations, and quotes. |
| **Duplicated / Inconsistent Package Name Logic** | `move_file.go` vs `move_dir.go` | `move_file` inspects existing files in the dir; `move_dir` blindly computes snake_case of dir base name, obliterating `package main`. |
| **Duplicated Workspace Import Walkers** | `move_file.go` vs `move_dir.go` | `updateWorkspaceMovedFileImports` and `updateWorkspaceImports` both implement custom `filepath.Walk` traversals with separate AST parsing logic. |
| **Brittle Interface Scraping** | `impl_iface.go` | Manually traverses AST declarations looking for interface keywords rather than using `go/types` type-checking, preventing third-party and embedded interface resolution. |
| **Linter Evasion Hacks** | `move_file.go` | Split into `rewriteUnprefixedNodePart1` and `rewriteUnprefixedNodePart2` solely to evade cyclomatic complexity checks. |
| **Dangerous Blind Renaming Fallback** | `rename.go` | In `applyASTRename`, if `targetObj == nil`, it fell back to blindly string-matching and replacing identifiers module-wide. |
| **Dead Code** | `types.go` | `CamelToSnake` was exported and completely unused across the repository. |
| **Inconsistent File Formatting / Writing** | `rename.go`, `move_file.go`, `move_dir.go`, `impl_iface.go` | Inconsistent formatting and build tag handling. |

---

## Part 3: Phase 0 Refactoring Strategy

### Guiding Principles
1. **Zero Regressions**: 100% of existing tests must continue to pass without changing MCP tool interfaces or tool names.
2. **Unified Core Primitives**: Introduce `Workspace` and `astutil` to isolate filesystem/Go-module knowledge and AST rewriting from individual tool handlers.
3. **Idiomatic Go Tooling**: Use `golang.org/x/mod/modfile` and `go/types` instead of regex and manual AST scraping.
4. **Clean Code & Strict Linter Compliance**: Pass all local quality gates: `go test ./...`, `golangci-lint run` (0 issues), `gosec ./...` (0 issues), `govulncheck ./...`.
