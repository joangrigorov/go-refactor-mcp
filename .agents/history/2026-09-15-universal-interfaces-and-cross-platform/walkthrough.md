# Walkthrough: Phase 2 - Universal Interfaces & Cross-Platform Refactoring

Phase 2 introduces two core capabilities to `go-refactor-mcp`:
1. **Dynamic Universal & Embedded Interface Resolution** in `implement_interface`.
2. **Cross-Platform / Multi-OS Symbol Refactoring** in `rename_symbol`.

---

## Changes Implemented

### 1. Universal & Embedded Interface Resolution (`internal/refactor/impl_iface.go`)
- **Stdlib Shorthand Registry**: Added `stdlibShorthands` mapping common standard library package shorthands without directory paths (`"http"` -> `"net/http"`, `"sql"` -> `"database/sql"`, `"driver"` -> `"database/sql/driver"`, `"json"` -> `"encoding/json"`, etc.).
- **Dynamic Interface Loading**: Updated `resolveInterfaceDynamic` to resolve shorthand standard library paths and arbitrary module packages via `packages.Load`. Added validation to return an actionable error if the target type is not an interface (e.g. attempting to implement a struct or function).
- **Embedded Interface Traversal**: Updated `resolveLocalInterface` to recursively flatten embedded interfaces:
  - `*ast.Ident`: Traverses local embedded interfaces within the file and across workspace packages.
  - `*ast.SelectorExpr`: Traverses imported embedded interfaces (e.g. `io.Reader`, `io.ReadCloser`, `http.Handler`).
- **Method Deduplication**: Added `deduplicateStubs` to guarantee clean method sets when multiple embedded interfaces share methods.
- **Auto-Imports via `ximports`**: Ensured `WriteASTFileWithImports` automatically inserts missing imports for external types in generated method signatures.

### 2. Multi-OS Cross-Platform Symbol Refactoring (`internal/refactor/workspace.go` & `internal/refactor/rename.go`)
- **Platform Discovery (`DiscoverWorkspacePlatforms`)**: Walks workspace Go files to detect platform-specific file naming suffixes (`*_windows.go`, `*_darwin.go`, `*_linux.go`, etc.) and `//go:build` / `// +build` directives. Discovers foreign operating systems differing from `runtime.GOOS`.
- **Multi-Pass Platform Package Loading (`LoadAllPlatformPackages`)**:
  - Pass 1: Loads packages for host `runtime.GOOS`.
  - Pass 2+: For each detected foreign platform, executes `packages.Load` with `cfg.Env = append(os.Environ(), "GOOS="+targetOS)`.
  - Checks for syntax errors in platform-specific files and returns actionable diagnostics.
- **Cross-Platform Symbol Matching (`sameSymbolAcrossPlatforms`)**:
  - Matches declarations and usages across OS passes for package-level symbols (`obj.Parent() == obj.Pkg().Scope()`) and methods on identical named receiver types.
  - Prevents matching local variables across different files or platforms.
- **Deduplicated Disk Writing**: Tracks `writtenFiles` so shared files evaluated across multiple platform passes are only written once.

---

## Verification Results

### 1. Automated Integration Tests
Added 3 dedicated integration test suites exercising the MCP server interface:
- **`tests/cross_platform_rename_test.go`**:
  - `TestMCPIntegration_CrossPlatformRename_MultiOSHappyPath`: Renames symbols across `runner.go`, `runner_linux.go`, `runner_windows.go`, and `runner_darwin.go`.
  - **Tri-Platform Compiler Verification**: Asserts `GOOS=linux go build ./...`, `GOOS=windows go build ./...`, and `GOOS=darwin go build ./...` all compile cleanly post-refactor.
  - `TestMCPIntegration_CrossPlatformRename_WindowsOnlySymbol`: Verifies renaming a symbol declared and used only in a foreign OS file.
  - `TestMCPIntegration_CrossPlatformRename_SyntaxErrorInPlatformFile`: Verifies actionable error diagnostics when a foreign platform file contains syntax errors.
- **`tests/embedded_interface_test.go`**:
  - `TestMCPIntegration_EmbeddedInterface_LocalHierarchy`: Verifies stub generation for nested local embedded interfaces (`ReadCloser` -> `Reader`, `Closer`).
  - `TestMCPIntegration_EmbeddedInterface_StdlibEmbedding`: Verifies stub generation for interface embedding `io.ReadCloser` and adding `Flush()`.
  - `TestMCPIntegration_EmbeddedInterface_UndefinedEmbeddedType`: Verifies actionable error when an interface embeds an undefined type.
  - `TestMCPIntegration_EmbeddedInterface_TargetNotAnInterface`: Verifies actionable error when attempting to implement a struct.
- **`tests/third_party_interface_test.go`**:
  - `TestMCPIntegration_ThirdPartyInterface_StdlibShorthand_HTTPHandler`: Verifies shorthand `http.Handler` generates `ServeHTTP` and auto-imports `net/http`.
  - `TestMCPIntegration_ThirdPartyInterface_StdlibShorthand_SQLScanner`: Verifies shorthand `sql.Scanner` generates `Scan(...) error`.
  - `TestMCPIntegration_ThirdPartyInterface_UnresolvablePackagePath`: Verifies actionable error for invalid package paths.
  - `TestMCPIntegration_ThirdPartyInterface_NonInterfaceTypeInPackage`: Verifies actionable error for non-interface types (e.g. `http.Request`).

Full test suite execution:
```bash
go test -v ./...
# 45 passed, 0 failed across all packages
```

### 2. Quality Gates
- **Linter Verification**:
  ```bash
  golangci-lint run
  # 0 issues.
  ```
- **Security Audit**:
  ```bash
  gosec ./...
  # 0 issues.
  ```
- **Vulnerability Check**:
  ```bash
  govulncheck ./...
  # 0 module vulnerabilities.
  ```
