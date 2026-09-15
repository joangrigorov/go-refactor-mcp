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
func AnalyzeShadowing(targetPath string, _ ...string) ([]ShadowIssue, error) {
	cleanTarget := strings.TrimSpace(targetPath)
	if cleanTarget == "" {
		return nil, fmt.Errorf("argument 'target_path' cannot be empty")
	}

	absPath, err := filepath.Abs(cleanTarget)
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

	// Package-level scope
	pushScope()
	for _, decl := range astFile.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if ok && (gen.Tok == token.VAR || gen.Tok == token.CONST) {
			for _, spec := range gen.Specs {
				valSpec, ok := spec.(*ast.ValueSpec)
				if ok {
					for _, name := range valSpec.Names {
						addVar(name.Name, fset.Position(name.Pos()).Line)
					}
				}
			}
		}
	}

	// Inspect functions
	for _, decl := range astFile.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}

		pushScope() // Function level scope

		// Receiver
		if fn.Recv != nil {
			for _, field := range fn.Recv.List {
				for _, name := range field.Names {
					addVar(name.Name, fset.Position(name.Pos()).Line)
				}
			}
		}

		// Parameters
		if fn.Type.Params != nil {
			for _, param := range fn.Type.Params.List {
				for _, name := range param.Names {
					addVar(name.Name, fset.Position(name.Pos()).Line)
				}
			}
		}

		// Function Body
		walkScopedNode(fn.Body, fset, pushScope, popScope, addVar)

		popScope()
	}

	popScope() // Pop package-level scope

	return issues
}

func walkScopedNode(n ast.Node, fset *token.FileSet, pushScope func(), popScope func(), addVar func(string, int)) {
	if n == nil {
		return
	}

	switch node := n.(type) {
	case *ast.BlockStmt:
		pushScope()
		for _, stmt := range node.List {
			walkScopedNode(stmt, fset, pushScope, popScope, addVar)
		}
		popScope()

	case *ast.IfStmt:
		pushScope()
		if node.Init != nil {
			walkScopedNode(node.Init, fset, pushScope, popScope, addVar)
		}
		walkScopedNode(node.Cond, fset, pushScope, popScope, addVar)
		walkScopedNode(node.Body, fset, pushScope, popScope, addVar)
		if node.Else != nil {
			walkScopedNode(node.Else, fset, pushScope, popScope, addVar)
		}
		popScope()

	case *ast.ForStmt:
		pushScope()
		if node.Init != nil {
			walkScopedNode(node.Init, fset, pushScope, popScope, addVar)
		}
		if node.Cond != nil {
			walkScopedNode(node.Cond, fset, pushScope, popScope, addVar)
		}
		if node.Post != nil {
			walkScopedNode(node.Post, fset, pushScope, popScope, addVar)
		}
		walkScopedNode(node.Body, fset, pushScope, popScope, addVar)
		popScope()

	case *ast.RangeStmt:
		pushScope()
		if node.Tok == token.DEFINE {
			if key, ok := node.Key.(*ast.Ident); ok {
				addVar(key.Name, fset.Position(key.Pos()).Line)
			}
			if val, ok := node.Value.(*ast.Ident); ok {
				addVar(val.Name, fset.Position(val.Pos()).Line)
			}
		}
		walkScopedNode(node.X, fset, pushScope, popScope, addVar)
		walkScopedNode(node.Body, fset, pushScope, popScope, addVar)
		popScope()

	case *ast.AssignStmt:
		if node.Tok == token.DEFINE {
			for _, expr := range node.Lhs {
				if id, ok := expr.(*ast.Ident); ok {
					addVar(id.Name, fset.Position(id.Pos()).Line)
				}
			}
		}
		for _, expr := range node.Rhs {
			walkScopedNode(expr, fset, pushScope, popScope, addVar)
		}

	case *ast.DeclStmt:
		if gen, ok := node.Decl.(*ast.GenDecl); ok && gen.Tok == token.VAR {
			for _, spec := range gen.Specs {
				if valSpec, ok := spec.(*ast.ValueSpec); ok {
					for _, name := range valSpec.Names {
						addVar(name.Name, fset.Position(name.Pos()).Line)
					}
				}
			}
		}

	default:
		ast.Inspect(n, func(child ast.Node) bool {
			if child == nil || child == n {
				return true
			}
			switch child.(type) {
			case *ast.BlockStmt, *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt, *ast.AssignStmt, *ast.DeclStmt:
				walkScopedNode(child, fset, pushScope, popScope, addVar)
				return false
			}
			return true
		})
	}
}
