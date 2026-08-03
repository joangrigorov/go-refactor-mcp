package refactor

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// AnalyzeShadowing scans a file or package directory and returns detected shadowed variables.
func AnalyzeShadowing(targetPath string) ([]ShadowIssue, error) {
	absPath, err := filepath.Abs(targetPath)
	if err != nil {
		return nil, fmt.Errorf("invalid target path: %w", err)
	}

	info, err := os.Stat(absPath)
	if err != nil {
		return nil, fmt.Errorf("path does not exist: %w", err)
	}

	var issues []ShadowIssue
	var files []string

	if info.IsDir() {
		err = filepath.Walk(absPath, func(p string, f os.FileInfo, e error) error {
			if e == nil && !f.IsDir() && strings.HasSuffix(p, ".go") && !IsVendorPath(p) {
				files = append(files, p)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	} else {
		files = append(files, absPath)
	}

	for _, file := range files {
		fset := token.NewFileSet()
		astFile, err := parser.ParseFile(fset, file, nil, parser.ParseComments)
		if err != nil {
			continue
		}

		fileIssues := inspectFileShadowing(fset, astFile)
		issues = append(issues, fileIssues...)
	}

	return issues, nil
}

func inspectFileShadowing(fset *token.FileSet, astFile *ast.File) []ShadowIssue {
	var issues []ShadowIssue

	// Scope tracking stack
	type scopeVar struct {
		name string
		line int
	}

	var scopeStack [][]scopeVar

	pushScope := func() {
		scopeStack = append(scopeStack, []scopeVar{})
	}

	popScope := func() {
		if len(scopeStack) > 0 {
			scopeStack = scopeStack[:len(scopeStack)-1]
		}
	}

	addVar := func(name string, line int) {
		if name == "" || name == "_" {
			return
		}
		if len(scopeStack) == 0 {
			return
		}

		// Check outer scopes for shadowing
		for i := len(scopeStack) - 2; i >= 0; i-- {
			for _, outer := range scopeStack[i] {
				if outer.name == name {
					issues = append(issues, ShadowIssue{
						VarName:      name,
						Line:         line,
						ShadowedLine: outer.line,
						Scope:        "local",
						Message:      fmt.Sprintf("variable '%s' at line %d shadows variable declared at line %d", name, line, outer.line),
					})
					break
				}
			}
		}

		currIdx := len(scopeStack) - 1
		scopeStack[currIdx] = append(scopeStack[currIdx], scopeVar{name: name, line: line})
	}

	// Traversal
	pushScope() // Top-level package scope

	for _, decl := range astFile.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}

		pushScope() // Func scope

		// Parameters
		if fn.Type.Params != nil {
			for _, param := range fn.Type.Params.List {
				for _, name := range param.Names {
					addVar(name.Name, fset.Position(name.Pos()).Line)
				}
			}
		}

		// Function Body
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			if n == nil {
				return true
			}

			switch stmt := n.(type) {
			case *ast.BlockStmt:
				pushScope()
			case *ast.AssignStmt:
				if stmt.Tok == token.DEFINE { // := definition
					for _, expr := range stmt.Lhs {
						if id, ok := expr.(*ast.Ident); ok {
							addVar(id.Name, fset.Position(id.Pos()).Line)
						}
					}
				}
			}
			return true
		})

		popScope()
	}

	popScope()

	return issues
}
