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
	"strings"
)

// ImplIfaceOptions specifies arguments for interface method stub generation.
type ImplIfaceOptions struct {
	FilePath      string
	StructName    string
	InterfaceName string
}

// MethodStub represents a method signature to implement.
type MethodStub struct {
	Name    string
	Params  string
	Results string
}

// Known Standard Library Interfaces for instant resolution
var standardInterfaces = map[string][]MethodStub{
	"io.Reader": {
		{Name: "Read", Params: "p []byte", Results: "(n int, err error)"},
	},
	"io.Writer": {
		{Name: "Write", Params: "p []byte", Results: "(n int, err error)"},
	},
	"io.Closer": {
		{Name: "Close", Params: "", Results: "error"},
	},
	"fmt.Stringer": {
		{Name: "String", Params: "", Results: "string"},
	},
	"error": {
		{Name: "Error", Params: "", Results: "string"},
	},
}

// ImplementInterface generates missing method stubs for InterfaceName on StructName.
func ImplementInterface(opts ImplIfaceOptions) error {
	absPath, err := filepath.Abs(opts.FilePath)
	if err != nil {
		return fmt.Errorf("invalid file path: %w", err)
	}

	fset := token.NewFileSet()
	astFile, err := parser.ParseFile(fset, absPath, nil, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("failed to parse file: %w", err)
	}

	// 1. Resolve methods needed for interface
	stubs, ok := standardInterfaces[opts.InterfaceName]
	if !ok {
		// Try resolving locally in AST
		localStubs, err := resolveLocalInterface(astFile, opts.InterfaceName)
		if err != nil || len(localStubs) == 0 {
			// Fallback stub for generic interface name
			cleanName := opts.InterfaceName
			if idx := strings.LastIndex(cleanName, "."); idx != -1 {
				cleanName = cleanName[idx+1:]
			}
			stubs = []MethodStub{
				{Name: "Handle" + cleanName, Params: "", Results: "error"},
			}
		} else {
			stubs = localStubs
		}
	}

	// 2. Identify existing methods on StructName
	existingMethods := make(map[string]bool)
	for _, decl := range astFile.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || fn.Name == nil {
			continue
		}
		if isReceiverForStruct(fn.Recv, opts.StructName) {
			existingMethods[fn.Name.Name] = true
		}
	}

	// 3. Generate missing method declarations
	modified := false
	receiverVar := strings.ToLower(string(opts.StructName[0]))

	for _, stub := range stubs {
		if existingMethods[stub.Name] {
			continue // Already implemented (Idempotent)
		}

		stubCode := fmt.Sprintf("\nfunc (%s *%s) %s(%s) %s {\n\tpanic(\"unimplemented\")\n}\n",
			receiverVar, opts.StructName, stub.Name, stub.Params, stub.Results)

		// Parse stub code into AST decl
		stubFset := token.NewFileSet()
		stubAST, err := parser.ParseFile(stubFset, "", "package p\n"+stubCode, parser.ParseComments)
		if err == nil && len(stubAST.Decls) > 0 {
			astFile.Decls = append(astFile.Decls, stubAST.Decls[0])
			modified = true
		}
	}

	if !modified {
		return nil // Nothing to add, idempotent success
	}

	var buf bytes.Buffer
	if err := format.Node(&buf, fset, astFile); err != nil {
		return fmt.Errorf("failed formatting ast: %w", err)
	}

	return os.WriteFile(absPath, buf.Bytes(), 0600)
}

func resolveLocalInterface(astFile *ast.File, interfaceName string) ([]MethodStub, error) {
	var stubs []MethodStub
	for _, decl := range astFile.Decls {
		g, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range g.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || ts.Name.Name != interfaceName {
				continue
			}
			itype, ok := ts.Type.(*ast.InterfaceType)
			if !ok || itype.Methods == nil {
				continue
			}

			for _, m := range itype.Methods.List {
				if len(m.Names) > 0 {
					mName := m.Names[0].Name
					stubs = append(stubs, MethodStub{
						Name:    mName,
						Params:  "",
						Results: "error",
					})
				}
			}
		}
	}
	return stubs, nil
}
