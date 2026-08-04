// Package refactor provides stateless Go AST, package, and symbol refactoring utilities.
package refactor

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/tools/go/packages"
)

// ShadowIssue represents a detected shadowed variable.
type ShadowIssue struct {
	VarName      string `json:"var_name"`
	Line         int    `json:"line"`
	ShadowedLine int    `json:"shadowed_line"`
	Scope        string `json:"scope"`
	Message      string `json:"message"`
}

// CamelToSnake converts a CamelCase string to snake_case.
func CamelToSnake(s string) string {
	var res strings.Builder
	runes := []rune(s)
	for i, r := range runes {
		if i > 0 && unicode.IsUpper(r) {
			if unicode.IsLower(runes[i-1]) || (i+1 < len(runes) && unicode.IsLower(runes[i+1])) {
				res.WriteRune('_')
			}
		}
		res.WriteRune(unicode.ToLower(r))
	}
	return res.String()
}

// LoadModulePackages loads packages starting from dir/module path.
func LoadModulePackages(dir string) ([]*packages.Package, error) {
	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedImports | packages.NeedTypes | packages.NeedTypesInfo |
			packages.NeedSyntax | packages.NeedDeps,
		Dir: dir,
		Tests: true,
	}
	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, fmt.Errorf("failed to load packages in %s: %w", dir, err)
	}
	return pkgs, nil
}

// FindModuleRoot finds the directory containing go.mod starting from startDir.
func FindModuleRoot(startDir string) (string, error) {
	curr := startDir
	for {
		if _, err := os.Stat(filepath.Join(curr, "go.mod")); err == nil {
			return curr, nil
		}
		parent := filepath.Dir(curr)
		if parent == curr {
			break
		}
		curr = parent
	}
	return "", fmt.Errorf("go.mod not found starting from %s", startDir)
}

// GetModuleName reads module path from go.mod in moduleRoot.
func GetModuleName(moduleRoot string) (string, error) {
	content, err := os.ReadFile(filepath.Join(moduleRoot, "go.mod")) // #nosec G304
	if err != nil {
		return "", err
	}
	re := regexp.MustCompile(`(?m)^module\s+([^\s]+)`)
	matches := re.FindStringSubmatch(string(content))
	if len(matches) < 2 {
		return "", fmt.Errorf("could not parse module name from go.mod")
	}
	return matches[1], nil
}

// IsVendorPath checks if a path resides inside vendor folder.
func IsVendorPath(path string) bool {
	clean := filepath.ToSlash(path)
	return strings.Contains(clean, "/vendor/") || strings.HasPrefix(clean, "vendor/")
}

// ExtractBuildTags extracts //go:build or // +build comments from file content.
func ExtractBuildTags(content string) []string {
	var tags []string
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//go:build") || strings.HasPrefix(trimmed, "// +build") {
			tags = append(tags, line)
		} else if strings.HasPrefix(trimmed, "package ") {
			break
		}
	}
	return tags
}

// ParseAST parses a single file preserving comments.
func ParseAST(filePath string) (*token.FileSet, *ast.File, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filePath, nil, parser.ParseComments)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to parse %s: %w", filePath, err)
	}
	return fset, file, nil
}
