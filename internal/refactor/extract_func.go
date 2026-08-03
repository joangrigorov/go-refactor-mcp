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
	"sort"
)

// ExtractFuncOptions specifies arguments for function extraction.
type ExtractFuncOptions struct {
	FilePath    string
	StartLine   int
	EndLine     int
	NewFuncName string
}

// ExtractFunction extracts a line-range of statements into a new function.
func ExtractFunction(opts ExtractFuncOptions) error {
	absPath, err := filepathAbs(opts.FilePath)
	if err != nil {
		return fmt.Errorf("invalid file path: %w", err)
	}

	fset := token.NewFileSet()
	astFile, err := parser.ParseFile(fset, absPath, nil, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("failed to parse file: %w", err)
	}

	// 1. Locate enclosing function and statements within startLine and endLine
	var enclosingFunc *ast.FuncDecl
	var parentBlock *ast.BlockStmt
	var startStmtIdx, endStmtIdx = -1, -1

	ast.Inspect(astFile, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			return true
		}

		for i, stmt := range fn.Body.List {
			stmtStart := fset.Position(stmt.Pos()).Line
			stmtEnd := fset.Position(stmt.End()).Line

			if stmtStart >= opts.StartLine && stmtEnd <= opts.EndLine {
				if enclosingFunc == nil {
					enclosingFunc = fn
					parentBlock = fn.Body
					startStmtIdx = i
					endStmtIdx = i
				} else if fn == enclosingFunc {
					endStmtIdx = i
				}
			}
		}
		return true
	})

	if enclosingFunc == nil || startStmtIdx == -1 {
		return fmt.Errorf("no statements found between line %d and %d", opts.StartLine, opts.EndLine)
	}

	extractedStmts := parentBlock.List[startStmtIdx : endStmtIdx+1]

	// 2. Identify variables referenced inside extractedStmts (inputs)
	// and variables modified inside extractedStmts that are referenced after endLine (outputs)
	varsInside := make(map[string]bool)
	assignedInside := make(map[string]bool)
	varsBefore := make(map[string]bool)
	varsAfter := make(map[string]bool)

	// Collect vars defined/used before target block in enclosing function
	for i := 0; i < startStmtIdx; i++ {
		collectIdents(parentBlock.List[i], varsBefore, nil)
	}

	// Collect vars inside extracted block
	for _, stmt := range extractedStmts {
		collectIdents(stmt, varsInside, assignedInside)
	}

	// Collect vars after extracted block
	for i := endStmtIdx + 1; i < len(parentBlock.List); i++ {
		collectIdents(parentBlock.List[i], varsAfter, nil)
	}

	// Inputs: vars defined before and referenced inside
	var inputs []string
	for v := range varsInside {
		if varsBefore[v] {
			inputs = append(inputs, v)
		}
	}
	sort.Strings(inputs)

	// Outputs: vars assigned inside and referenced after
	var outputs []string
	for v := range assignedInside {
		if varsAfter[v] {
			outputs = append(outputs, v)
		}
	}
	sort.Strings(outputs)

	// 3. Construct new function declaration
	paramFields := &ast.FieldList{}
	for _, in := range inputs {
		paramFields.List = append(paramFields.List, &ast.Field{
			Names: []*ast.Ident{ast.NewIdent(in)},
			Type:  ast.NewIdent("interface{}"), // generic placeholder type
		})
	}

	resultFields := &ast.FieldList{}
	for _, out := range outputs {
		resultFields.List = append(resultFields.List, &ast.Field{
			Names: []*ast.Ident{ast.NewIdent(out)},
			Type:  ast.NewIdent("interface{}"),
		})
	}

	newFunc := &ast.FuncDecl{
		Name: ast.NewIdent(opts.NewFuncName),
		Type: &ast.FuncType{
			Params:  paramFields,
			Results: resultFields,
		},
		Body: &ast.BlockStmt{
			List: extractedStmts,
		},
	}

	if len(outputs) > 0 {
		var retExprs []ast.Expr
		for _, out := range outputs {
			retExprs = append(retExprs, ast.NewIdent(out))
		}
		newFunc.Body.List = append(newFunc.Body.List, &ast.ReturnStmt{Results: retExprs})
	}

	// 4. Construct call expression statement to replace extracted block
	var callArgs []ast.Expr
	for _, in := range inputs {
		callArgs = append(callArgs, ast.NewIdent(in))
	}

	callExpr := &ast.CallExpr{
		Fun:  ast.NewIdent(opts.NewFuncName),
		Args: callArgs,
	}

	var replacementStmt ast.Stmt
	if len(outputs) == 0 {
		replacementStmt = &ast.ExprStmt{X: callExpr}
	} else {
		var lhs []ast.Expr
		for _, out := range outputs {
			lhs = append(lhs, ast.NewIdent(out))
		}
		replacementStmt = &ast.AssignStmt{
			Lhs: lhs,
			Tok: token.ASSIGN,
			Rhs: []ast.Expr{callExpr},
		}
	}

	// Replace statements in parent block
	newStmtList := make([]ast.Stmt, 0, len(parentBlock.List)-(endStmtIdx-startStmtIdx))
	newStmtList = append(newStmtList, parentBlock.List[:startStmtIdx]...)
	newStmtList = append(newStmtList, replacementStmt)
	newStmtList = append(newStmtList, parentBlock.List[endStmtIdx+1:]...)
	parentBlock.List = newStmtList

	// Append new function declaration to file
	astFile.Decls = append(astFile.Decls, newFunc)

	// Format and write back file
	var buf bytes.Buffer
	if err := format.Node(&buf, fset, astFile); err != nil {
		return fmt.Errorf("failed formatting ast: %w", err)
	}

	return os.WriteFile(absPath, buf.Bytes(), 0600)
}

func collectIdents(node ast.Node, idents map[string]bool, assigned map[string]bool) {
	if node == nil {
		return
	}
	ast.Inspect(node, func(n ast.Node) bool {
		switch stmt := n.(type) {
		case *ast.Ident:
			if stmt.Name != "" && stmt.Name != "_" {
				idents[stmt.Name] = true
			}
		case *ast.AssignStmt:
			if assigned != nil {
				for _, expr := range stmt.Lhs {
					if id, ok := expr.(*ast.Ident); ok {
						assigned[id.Name] = true
					}
				}
			}
		}
		return true
	})
}

func filepathAbs(path string) (string, error) {
	return filepath.Abs(path)
}
