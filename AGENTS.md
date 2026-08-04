# Agent Developer Guidelines

This repository enforces strict quality control standards for AI coding agents and human contributors.

## Verification Requirements Before Submitting Changes

Before committing or opening a pull request for any changes to this codebase, you **MUST** run and pass all of the following local verification steps:

1. **Unit Test Suite**:
   ```bash
   go test ./...
   ```
   All unit tests across all packages must pass cleanly.

2. **Linter Verification (`golangci-lint`)**:
   ```bash
   golangci-lint run
   ```
   Must complete with `0 issues`. Ensure cyclomatic complexity (`gocyclo`), variable shadowing (`govet`), error checking (`errcheck`), and all enabled linters pass without errors.

3. **Security Analysis (`gosec`)**:
   ```bash
   gosec ./...
   ```
   Must complete with 0 security issues. Use `// #nosec Gxxx` annotations only when explicitly justified.

4. **Vulnerability Audit (`govulncheck`)**:
   ```bash
   govulncheck ./...
   ```
   Audit for known vulnerabilities across module dependencies and standard library usage.

## Git & PR Conventions

- **Branch Naming**: Always create a conventional semantic branch (e.g., `fix/*`, `feat/*`, `refactor/*`, `test/*`) before making changes.
- **Commit Messages**: Follow Conventional Commits specification with descriptive extended bodies (using multiple `-m` flags).
- **No Command Chaining**: Execute git commands individually without chaining (`&&`, `;`).
- **Pull Requests**: Include a clear summary of changes and test coverage in PR descriptions.
