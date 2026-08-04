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
		// Try resolving locally in astFile
		localStubs, err := resolveLocalInterface(fset, astFile, opts.InterfaceName)
		if err == nil && len(localStubs) > 0 {
			stubs = localStubs
		} else {
			// Try resolving in workspace module
			wsStubs, err := resolveInterfaceInWorkspace(filepath.Dir(absPath), opts.InterfaceName)
			if err == nil && len(wsStubs) > 0 {
				stubs = wsStubs
			} else {
				// Fallback stub if interface cannot be found
				cleanName := opts.InterfaceName
				if idx := strings.LastIndex(cleanName, "."); idx != -1 {
					cleanName = cleanName[idx+1:]
				}
				stubs = []MethodStub{
					{Name: "Handle" + cleanName, Params: "", Results: "error"},
				}
			}
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

func resolveLocalInterface(fset *token.FileSet, astFile *ast.File, interfaceName string) ([]MethodStub, error) {
	cleanName := interfaceName
	if idx := strings.LastIndex(cleanName, "."); idx != -1 {
		cleanName = cleanName[idx+1:]
	}

	var stubs []MethodStub
	for _, decl := range astFile.Decls {
		g, ok := decl.(*ast.GenDecl)
		if !ok || g.Tok != token.TYPE {
			continue
		}
		for _, spec := range g.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok || ts.Name.Name != cleanName {
				continue
			}
			itype, ok := ts.Type.(*ast.InterfaceType)
			if !ok || itype.Methods == nil {
				continue
			}

			for _, m := range itype.Methods.List {
				ft, ok := m.Type.(*ast.FuncType)
				if !ok {
					continue
				}
				paramsStr := formatFieldList(fset, ft.Params)
				resultsStr := formatResultsFieldList(fset, ft.Results)

				if len(m.Names) > 0 {
					for _, name := range m.Names {
						stubs = append(stubs, MethodStub{
							Name:    name.Name,
							Params:  paramsStr,
							Results: resultsStr,
						})
					}
				}
			}
		}
	}
	return stubs, nil
}

func resolveInterfaceInWorkspace(dirPath string, interfaceName string) ([]MethodStub, error) {
	moduleRoot, err := FindModuleRoot(dirPath)
	if err != nil {
		moduleRoot = dirPath
	}

	cleanName := interfaceName
	if idx := strings.LastIndex(cleanName, "."); idx != -1 {
		cleanName = cleanName[idx+1:]
	}

	var foundStubs []MethodStub
	_ = filepath.Walk(moduleRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || IsVendorPath(path) {
			return err
		}
		fset := token.NewFileSet()
		fileAST, parseErr := parser.ParseFile(fset, path, nil, parser.ParseComments)
		if parseErr != nil {
			return nil
		}
		stubs, resolveErr := resolveLocalInterface(fset, fileAST, cleanName)
		if resolveErr == nil && len(stubs) > 0 {
			foundStubs = stubs
			return filepath.SkipAll
		}
		return nil
	})

	if len(foundStubs) > 0 {
		return foundStubs, nil
	}
	return nil, fmt.Errorf("interface %s not found", interfaceName)
}

func formatFieldList(fset *token.FileSet, fields *ast.FieldList) string {
	if fields == nil || len(fields.List) == 0 {
		return ""
	}
	var parts []string
	for _, field := range fields.List {
		var typeBuf bytes.Buffer
		_ = format.Node(&typeBuf, fset, field.Type)
		typeStr := typeBuf.String()

		if len(field.Names) > 0 {
			var names []string
			for _, name := range field.Names {
				names = append(names, name.Name)
			}
			parts = append(parts, strings.Join(names, ", ")+" "+typeStr)
		} else {
			parts = append(parts, typeStr)
		}
	}
	return strings.Join(parts, ", ")
}

func formatResultsFieldList(fset *token.FileSet, fields *ast.FieldList) string {
	if fields == nil || len(fields.List) == 0 {
		return ""
	}
	resStr := formatFieldList(fset, fields)
	if len(fields.List) > 1 || (len(fields.List) == 1 && len(fields.List[0].Names) > 0) {
		return "(" + resStr + ")"
	}
	return resStr
}
