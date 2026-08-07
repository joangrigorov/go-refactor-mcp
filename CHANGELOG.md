# Changelog

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
