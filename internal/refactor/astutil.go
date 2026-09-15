package refactor

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	ximports "golang.org/x/tools/imports"
)

// DeterminePackageName determines the package clause name for a directory.
// If existing non-test .go files exist in dirPath, their package name (e.g. "main" or custom) is preserved.
func DeterminePackageName(dirPath string, fallbackName string) string {
	entries, readErr := os.ReadDir(dirPath)
	if readErr == nil {
		for _, entry := range entries {
			if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") {
				fset := token.NewFileSet()
				node, parseErr := parser.ParseFile(fset, filepath.Join(dirPath, entry.Name()), nil, parser.PackageClauseOnly)
				if parseErr == nil && node.Name != nil && !strings.HasSuffix(node.Name.Name, "_test") {
					return node.Name.Name
				}
			}
		}
	}

	target := fallbackName
	if target == "" {
		target = filepath.Base(dirPath)
	}

	return CleanPackageIdentifier(target)
}

// CleanPackageIdentifier normalizes a directory or package name to a valid Go identifier.
func CleanPackageIdentifier(name string) string {
	clean := strings.ReplaceAll(name, "-", "_")
	clean = strings.ReplaceAll(clean, ".", "_")
	if clean == "" || clean == "." {
		return "main"
	}
	return clean
}

// WriteASTFile formats an AST node and writes it to filename preserving exact AST structure.
func WriteASTFile(fset *token.FileSet, fileAST *ast.File, filename string) error {
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, fileAST); err != nil {
		return fmt.Errorf("failed formatting ast for %s: %w", filename, err)
	}

	return os.WriteFile(filename, buf.Bytes(), 0600) // #nosec G304 G703
}

// WriteASTFileWithImports formats an AST node, tidies imports via ximports, and writes it to filename.
func WriteASTFileWithImports(fset *token.FileSet, fileAST *ast.File, filename string) error {
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, fileAST); err != nil {
		return fmt.Errorf("failed formatting ast for %s: %w", filename, err)
	}

	formatted, impErr := ximports.Process(filename, buf.Bytes(), nil)
	content := buf.Bytes()
	if impErr == nil {
		content = formatted
	}

	return os.WriteFile(filename, content, 0600) // #nosec G304 G703
}

// WriteASTWithBuildTags writes an AST to destPath while preserving build tags from origContent.
func WriteASTWithBuildTags(fset *token.FileSet, fileAST *ast.File, origContent string, destPath string) error {
	buildTags := ExtractBuildTags(origContent)
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, fileAST); err != nil {
		return fmt.Errorf("failed formatting ast for %s: %w", destPath, err)
	}

	var combined strings.Builder
	if len(buildTags) > 0 {
		for _, tag := range buildTags {
			combined.WriteString(tag)
			combined.WriteString("\n")
		}
		combined.WriteString("\n")
	}
	combined.Write(buf.Bytes())

	return os.WriteFile(destPath, []byte(combined.String()), 0600) // #nosec G304 G703
}

// FindImportSpec finds an import spec by matching its unquoted import path.
func FindImportSpec(fileAST *ast.File, importPath string) *ast.ImportSpec {
	for _, imp := range fileAST.Imports {
		if imp.Path != nil {
			path, unquoteErr := strconv.Unquote(imp.Path.Value)
			if unquoteErr == nil && path == importPath {
				return imp
			}
		}
	}
	return nil
}

// HasImport returns true if the AST imports the given path.
func HasImport(fileAST *ast.File, importPath string) bool {
	return FindImportSpec(fileAST, importPath) != nil
}

// GetImportAlias extracts the package identifier bound by an import spec.
func GetImportAlias(spec *ast.ImportSpec, defaultPkgName string) string {
	if spec != nil && spec.Name != nil && spec.Name.Name != "" && spec.Name.Name != "_" && spec.Name.Name != "." {
		return spec.Name.Name
	}
	return defaultPkgName
}

// RemoveImportSpec removes an import spec from the AST file declarations.
func RemoveImportSpec(fileAST *ast.File, targetSpec *ast.ImportSpec) {
	for _, decl := range fileAST.Decls {
		g, ok := decl.(*ast.GenDecl)
		if ok && g.Tok == token.IMPORT {
			var newSpecs []ast.Spec
			for _, s := range g.Specs {
				if is, ok := s.(*ast.ImportSpec); ok && is == targetSpec {
					continue
				}
				newSpecs = append(newSpecs, s)
			}
			g.Specs = newSpecs
			return
		}
	}
}

// AddImportSpec inserts an import spec into the file AST.
func AddImportSpec(fileAST *ast.File, spec *ast.ImportSpec) {
	if spec == nil || spec.Path == nil {
		return
	}
	pathVal, unquoteErr := strconv.Unquote(spec.Path.Value)
	if unquoteErr == nil && HasImport(fileAST, pathVal) {
		return
	}

	for _, decl := range fileAST.Decls {
		g, ok := decl.(*ast.GenDecl)
		if ok && g.Tok == token.IMPORT {
			for _, s := range g.Specs {
				if is, ok := s.(*ast.ImportSpec); ok && is.Path != nil {
					p, _ := strconv.Unquote(is.Path.Value)
					if p == pathVal {
						return
					}
				}
			}
			g.Specs = append(g.Specs, spec)
			return
		}
	}

	importDecl := &ast.GenDecl{
		Tok:   token.IMPORT,
		Specs: []ast.Spec{spec},
	}
	fileAST.Decls = append([]ast.Decl{importDecl}, fileAST.Decls...)
}
