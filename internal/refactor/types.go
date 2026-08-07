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

// Standard OS/Arch build tags to ignore during custom build tag auto-discovery
var standardGoBuildTags = map[string]bool{
	"android": true, "darwin": true, "dragonfly": true, "freebsd": true, "illumos": true,
	"ios": true, "js": true, "linux": true, "netbsd": true, "openbsd": true, "plan9": true,
	"solaris": true, "windows": true, "aix": true, "wasip1": true, "wasm": true,
	"386": true, "amd64": true, "arm": true, "arm64": true, "loong64": true,
	"mips": true, "mipsle": true, "mips64": true, "mips64le": true, "ppc64": true,
	"ppc64le": true, "riscv64": true, "s390x": true, "cgo": true, "unix": true,
	"gc": true, "gccgo": true, "ignore": true,
}

var tagIdentRegex = regexp.MustCompile(`[a-zA-Z0-9_\.]+`)

// ParseBuildTags splits a comma, space, or semicolon separated tag string into a clean slice.
func ParseBuildTags(input string) []string {
	if strings.TrimSpace(input) == "" {
		return nil
	}
	raw := strings.FieldsFunc(input, func(r rune) bool {
		return r == ',' || r == ' ' || r == ';'
	})
	var tags []string
	seen := make(map[string]bool)
	for _, t := range raw {
		trimmed := strings.TrimSpace(t)
		if trimmed != "" && !seen[trimmed] {
			seen[trimmed] = true
			tags = append(tags, trimmed)
		}
	}
	return tags
}

// DiscoverWorkspaceBuildTags scans all non-vendor Go source files under dir for custom //go:build or // +build tags.
func DiscoverWorkspaceBuildTags(dir string) ([]string, error) {
	tagSet := make(map[string]bool)
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}
		if info.IsDir() {
			name := info.Name()
			if name == "vendor" || name == ".git" || name == ".github" || name == "node_modules" || (strings.HasPrefix(name, ".") && name != ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || IsVendorPath(path) {
			return nil
		}

		content, readErr := os.ReadFile(path) // #nosec G304 G122
		if readErr != nil {
			return nil
		}

		lines := strings.Split(string(content), "\n")
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "package ") {
				break
			}
			if strings.HasPrefix(trimmed, "//go:build") || strings.HasPrefix(trimmed, "// +build") {
				matches := tagIdentRegex.FindAllString(trimmed, -1)
				for _, match := range matches {
					if match == "go" || match == "build" {
						continue
					}
					if !standardGoBuildTags[match] {
						tagSet[match] = true
					}
				}
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	var tags []string
	for tag := range tagSet {
		tags = append(tags, tag)
	}
	return tags, nil
}

// LoadModulePackages loads packages starting from dir/module path, auto-discovering build tags.
func LoadModulePackages(dir string) ([]*packages.Package, error) {
	return LoadModulePackagesWithTags(dir, nil)
}

// LoadModulePackagesWithTags loads packages starting from dir/module path using specified or auto-discovered build tags.
func LoadModulePackagesWithTags(dir string, userTags []string) ([]*packages.Package, error) {
	moduleRoot, err := FindModuleRoot(dir)
	if err != nil {
		moduleRoot = dir
	}

	tags := userTags
	if len(tags) == 0 {
		discovered, discErr := DiscoverWorkspaceBuildTags(moduleRoot)
		if discErr == nil {
			tags = discovered
		}
	}

	cfg := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedImports | packages.NeedTypes | packages.NeedTypesInfo |
			packages.NeedSyntax | packages.NeedDeps,
		Dir:   moduleRoot,
		Tests: true,
	}

	if len(tags) > 0 {
		cfg.BuildFlags = []string{"-tags=" + strings.Join(tags, ",")}
	}

	pkgs, err := packages.Load(cfg, "./...")
	if err != nil {
		return nil, fmt.Errorf("failed to load packages in %s: %w", moduleRoot, err)
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
