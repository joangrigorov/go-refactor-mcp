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
	"unicode"
)

// ExtractIfaceOptions specifies arguments for interface extraction.
type ExtractIfaceOptions struct {
	FilePath      string
	StructName    string
	DestFilePath  string
	InterfaceName string
}

// ExtractInterface scans exported methods on StructName and generates an interface in DestFilePath.
func ExtractInterface(opts ExtractIfaceOptions) error {
	absSource, err := filepath.Abs(opts.FilePath)
	if err != nil {
		return fmt.Errorf("invalid source file path: %w", err)
	}

	absDest := absSource
	if opts.DestFilePath != "" {
		absDest, err = filepath.Abs(opts.DestFilePath)
		if err != nil {
			return fmt.Errorf("invalid dest file path: %w", err)
		}
	}

	// Parse source file AST
	fset := token.NewFileSet()
	fileAST, err := parser.ParseFile(fset, absSource, nil, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("failed parsing source file: %w", err)
	}

	// Collect exported methods on StructName receiver
	var interfaceMethods []*ast.Field

	for _, decl := range fileAST.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || fn.Name == nil {
			continue
		}

		// Check if method is exported
		if !unicode.IsUpper(rune(fn.Name.Name[0])) {
			continue
		}

		// Check receiver type
		if isReceiverForStruct(fn.Recv, opts.StructName) {
			interfaceMethods = append(interfaceMethods, &ast.Field{
				Names: []*ast.Ident{ast.NewIdent(fn.Name.Name)},
				Type:  fn.Type,
			})
		}
	}

	if len(interfaceMethods) == 0 {
		return fmt.Errorf("no exported methods found for struct %s in %s", opts.StructName, opts.FilePath)
	}

	// Build interface spec node
	interfaceSpec := &ast.TypeSpec{
		Name: ast.NewIdent(opts.InterfaceName),
		Type: &ast.InterfaceType{
			Methods: &ast.FieldList{
				List: interfaceMethods,
			},
		},
	}

	genDecl := &ast.GenDecl{
		Tok:   token.TYPE,
		Specs: []ast.Spec{interfaceSpec},
	}

	// Load or parse dest file
	var destFset *token.FileSet
	var destAST *ast.File

	if _, err := os.Stat(absDest); err == nil {
		destFset = token.NewFileSet()
		destAST, err = parser.ParseFile(destFset, absDest, nil, parser.ParseComments)
		if err != nil {
			return fmt.Errorf("failed to parse dest file: %w", err)
		}
	} else {
		destFset = token.NewFileSet()
		pkgName := fileAST.Name.Name
		destAST = &ast.File{
			Name:  ast.NewIdent(pkgName),
			Decls: []ast.Decl{},
		}
	}

	// Check if interface already exists (Idempotent)
	for _, decl := range destAST.Decls {
		if g, ok := decl.(*ast.GenDecl); ok {
			for _, spec := range g.Specs {
				if ts, ok := spec.(*ast.TypeSpec); ok && ts.Name.Name == opts.InterfaceName {
					return nil // Already exists
				}
			}
		}
	}

	destAST.Decls = append(destAST.Decls, genDecl)

	var buf bytes.Buffer
	if err := format.Node(&buf, destFset, destAST); err != nil {
		return fmt.Errorf("failed formatting dest ast: %w", err)
	}

	if err := os.WriteFile(absDest, buf.Bytes(), 0600); err != nil {
		return fmt.Errorf("failed writing interface file %s: %w", absDest, err)
	}

	return nil
}

func isReceiverForStruct(recv *ast.FieldList, structName string) bool {
	if recv == nil || len(recv.List) == 0 {
		return false
	}
	t := recv.List[0].Type
	if star, ok := t.(*ast.StarExpr); ok {
		t = star.X
	}
	if ident, ok := t.(*ast.Ident); ok {
		return ident.Name == structName
	}
	return false
}
