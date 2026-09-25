# Changelog

## [0.5.0](https://github.com/joangrigorov/go-refactor-mcp/compare/v0.4.1...v0.5.0) (2026-09-25)


### ⚠ BREAKING CHANGES

* **server:** directory is now required for move_file and move_directory tools, and Dir is required in MoveFileOptions and MoveDirOptions.

### Features

* **server:** require directory parameter and enforce workspace ceiling safeguards ([#30](https://github.com/joangrigorov/go-refactor-mcp/issues/30)) ([c73a018](https://github.com/joangrigorov/go-refactor-mcp/commit/c73a0186c55575d1471f6c0709be04e63277f290))


### Bug Fixes

* **deps:** bump github.com/mark3labs/mcp-go in the go-dependencies group ([#27](https://github.com/joangrigorov/go-refactor-mcp/issues/27)) ([3929de5](https://github.com/joangrigorov/go-refactor-mcp/commit/3929de599e7661412ab72ad74f772f35bf7dc2ab))
* **workspace:** prevent climbing out of standalone modules and across git boundaries ([#28](https://github.com/joangrigorov/go-refactor-mcp/issues/28)) ([eb15b80](https://github.com/joangrigorov/go-refactor-mcp/commit/eb15b8052416dd3da97174f0789a26a454bec75e))

## [0.4.1](https://github.com/joangrigorov/go-refactor-mcp/compare/v0.4.0...v0.4.1) (2026-09-15)


### Bug Fixes

* **server:** resolve stdio handshake hang on modern protocol and cleanup help text ([#25](https://github.com/joangrigorov/go-refactor-mcp/issues/25)) ([0a8c488](https://github.com/joangrigorov/go-refactor-mcp/commit/0a8c488ca816917cb3828f617add5a3912c5930e))

## [0.4.0](https://github.com/joangrigorov/go-refactor-mcp/compare/v0.3.0...v0.4.0) (2026-09-15)


### Features

* **refactor:** add universal interface resolution and cross-platform symbol renaming ([#22](https://github.com/joangrigorov/go-refactor-mcp/issues/22)) ([5db8f82](https://github.com/joangrigorov/go-refactor-mcp/commit/5db8f829a1baa0a17c0f659958b1f708878c0c02))
* **workspace:** support go.work, monorepos, and import collision auto-aliasing ([#20](https://github.com/joangrigorov/go-refactor-mcp/issues/20)) ([061873a](https://github.com/joangrigorov/go-refactor-mcp/commit/061873a5e7f72e61f75359e145e46e4438dad7ea))


### Bug Fixes

* **deps:** bump github.com/mark3labs/mcp-go in the go-dependencies group ([#18](https://github.com/joangrigorov/go-refactor-mcp/issues/18)) ([880332d](https://github.com/joangrigorov/go-refactor-mcp/commit/880332d1012f1cbf242ae4a127b1ad9caf49723b))
* **deps:** bump golang.org/x/tools from 0.49.0 to 0.50.0 in the go-dependencies group ([#23](https://github.com/joangrigorov/go-refactor-mcp/issues/23)) ([8e71371](https://github.com/joangrigorov/go-refactor-mcp/commit/8e71371a929c1bb0555f6cc4fb952ef561833fee))
* **deps:** bump the go-dependencies group across 1 directory with 2 updates ([#16](https://github.com/joangrigorov/go-refactor-mcp/issues/16)) ([9f09b06](https://github.com/joangrigorov/go-refactor-mcp/commit/9f09b06dd6ef572056e66c27f7d59bdac616f09b))
* **server:** eliminate silent failures, improve error actionability, and add MCP integration tests ([#21](https://github.com/joangrigorov/go-refactor-mcp/issues/21)) ([703335c](https://github.com/joangrigorov/go-refactor-mcp/commit/703335c222aed0e84a83ae3afb996752f3c3e920))

## [0.3.0](https://github.com/joangrigorov/go-refactor-mcp/compare/v0.2.1...v0.3.0) (2026-08-07)


### Features

* **refactor:** auto-discover build tags and add optional build_tags tool flag ([#14](https://github.com/joangrigorov/go-refactor-mcp/issues/14)) ([03702a2](https://github.com/joangrigorov/go-refactor-mcp/commit/03702a2867bc83cc839b309e4b242654c50000a4))


### Bug Fixes

* **deps:** bump the github-actions-dependencies group with 5 updates ([#13](https://github.com/joangrigorov/go-refactor-mcp/issues/13)) ([3189ee7](https://github.com/joangrigorov/go-refactor-mcp/commit/3189ee7bebabc058b8a5ad6126339ede5fb48e87))

## [0.2.1](https://github.com/joangrigorov/go-refactor-mcp/compare/v0.2.0...v0.2.1) (2026-08-04)


### Bug Fixes

* **refactor:** handle parent-to-child directory moves, cycle checks, and file renaming ([#10](https://github.com/joangrigorov/go-refactor-mcp/issues/10)) ([45fc045](https://github.com/joangrigorov/go-refactor-mcp/commit/45fc045bd2655d52ff75b793697c24896d7c545c))

## [0.2.0](https://github.com/joangrigorov/go-refactor-mcp/compare/v0.1.0...v0.2.0) (2026-08-04)


### ⚠ BREAKING CHANGES

* completely deprecate and remove tidy_imports, add_struct_tags, extract_function, extract_interface

### Features

* add CLI help and version flags (-h, --help, -v, --version) ([302f1ed](https://github.com/joangrigorov/go-refactor-mcp/commit/302f1eda012d52b7aff211af875738957c174aff))
* **ci:** initialize release-please, goreleaser v2, and semantic PR validation ([#7](https://github.com/joangrigorov/go-refactor-mcp/issues/7)) ([e196567](https://github.com/joangrigorov/go-refactor-mcp/commit/e1965676db674df194623fd6fa6b87d462b4af6f))
* **go:** upgrade go.mod to Go 1.26 and align toolchain ([#5](https://github.com/joangrigorov/go-refactor-mcp/issues/5)) ([132cde2](https://github.com/joangrigorov/go-refactor-mcp/commit/132cde2fca222079830d9f21586f1b860e8f4968))
* initial implementation of go-refactor-mcp server and integration tests ([e47fb7e](https://github.com/joangrigorov/go-refactor-mcp/commit/e47fb7e9011ad5630292316ea1b5b835d4a63a51))


### Bug Fixes

* **ci:** set bump-minor-pre-major true in release-please-config.json ([0dc6b50](https://github.com/joangrigorov/go-refactor-mcp/commit/0dc6b509c834e61f1694571e11dc510e446a226b))
* **implement_interface:** preserve method parameters and return types for custom interface stubs ([3b11c13](https://github.com/joangrigorov/go-refactor-mcp/commit/3b11c139f30e7796ed053874f337a1c64f3c44fb))
* **move_directory:** preserve child sub-directory package clauses when moving nested directory trees ([31408f3](https://github.com/joangrigorov/go-refactor-mcp/commit/31408f3d4204b1472f1e306eda5ab8c69fe5ad98))
* **move_file:** normalize unquoted import paths and reuse existing import specs regardless of alias ([8d8d0d3](https://github.com/joangrigorov/go-refactor-mcp/commit/8d8d0d3e0716414e960f71a5b4ac892eb2a99fd9))
* **move_file:** prevent duplicate import statements when moving multiple files to the same target package ([5f93c62](https://github.com/joangrigorov/go-refactor-mcp/commit/5f93c62083c431b2ec41b0d115433fc37aa3ba71))
* **move_file:** update un-prefixed symbol references in remaining source package files ([f960d36](https://github.com/joangrigorov/go-refactor-mcp/commit/f960d36701a594373e62d0910f31b633dcf240b7))
* **move_file:** update workspace import paths for moved symbols ([f771c5e](https://github.com/joangrigorov/go-refactor-mcp/commit/f771c5edc047ab614692181e7f58ad440167077c))


### Code Refactoring

* completely deprecate and remove tidy_imports, add_struct_tags, extract_function, extract_interface ([3591147](https://github.com/joangrigorov/go-refactor-mcp/commit/3591147c0d46a337b1a622d09a90bb30a62739ea))
