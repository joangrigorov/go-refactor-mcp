package refactor

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"go/types"
	"path/filepath"
	"strconv"
	"strings"

	"golang.org/x/tools/go/packages"
)

// ImplIfaceOptions specifies arguments for interface method stub generation.
type ImplIfaceOptions struct {
	FilePath      string
	StructName    string
	InterfaceName string
	BuildTags     string
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
	cleanStruct := strings.TrimSpace(opts.StructName)
	if cleanStruct == "" {
		return fmt.Errorf("argument 'struct_name' cannot be empty")
	}

	cleanIface := strings.TrimSpace(opts.InterfaceName)
	if cleanIface == "" {
		return fmt.Errorf("argument 'interface_name' cannot be empty")
	}

	absPath, err := filepath.Abs(opts.FilePath)
	if err != nil {
		return fmt.Errorf("invalid file path: %w", err)
	}

	fset := token.NewFileSet()
	astFile, err := parser.ParseFile(fset, absPath, nil, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("failed to parse file: %w", err)
	}

	availableStructs := findAvailableStructs(astFile)
	foundStruct := false
	for _, s := range availableStructs {
		if s == cleanStruct {
			foundStruct = true
			break
		}
	}
	if !foundStruct {
		if len(availableStructs) > 0 {
			return fmt.Errorf("struct %q not found in %s; available structs: [%s]", cleanStruct, filepath.Base(absPath), strings.Join(availableStructs, ", "))
		}
		return fmt.Errorf("struct %q not found in %s; no structs declared in this file", cleanStruct, filepath.Base(absPath))
	}

	stubs, err := resolveStubs(absPath, fset, astFile, cleanIface)
	if err != nil {
		return err
	}

	existingMethods := findExistingMethods(astFile, cleanStruct)

	modified := false
	receiverVar := strings.ToLower(string(cleanStruct[0]))

	for _, stub := range stubs {
		if existingMethods[stub.Name] {
			continue // Already implemented (Idempotent)
		}

		returnTypeStr := stub.Results
		if returnTypeStr != "" && !strings.HasPrefix(returnTypeStr, " ") {
			returnTypeStr = " " + returnTypeStr
		}

		stubCode := fmt.Sprintf("\nfunc (%s *%s) %s(%s)%s {\n\tpanic(\"unimplemented\")\n}\n",
			receiverVar, cleanStruct, stub.Name, stub.Params, returnTypeStr)

		stubFset := token.NewFileSet()
		stubAST, parseErr := parser.ParseFile(stubFset, "", "package p\n"+stubCode, parser.ParseComments)
		if parseErr == nil && len(stubAST.Decls) > 0 {
			astFile.Decls = append(astFile.Decls, stubAST.Decls[0])
			modified = true
		}
	}

	if !modified {
		return nil // Nothing to add, idempotent success
	}

	return WriteASTFileWithImports(fset, astFile, absPath)
}

func findAvailableStructs(astFile *ast.File) []string {
	var structs []string
	for _, decl := range astFile.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}
		for _, spec := range genDecl.Specs {
			typeSpec, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			if _, isStruct := typeSpec.Type.(*ast.StructType); isStruct {
				structs = append(structs, typeSpec.Name.Name)
			}
		}
	}
	return structs
}

func resolveStubs(absPath string, fset *token.FileSet, astFile *ast.File, interfaceName string) ([]MethodStub, error) {
	if stubs, ok := standardInterfaces[interfaceName]; ok {
		return stubs, nil
	}

	if localStubs, err := resolveLocalInterface(fset, astFile, interfaceName); err == nil && len(localStubs) > 0 {
		return localStubs, nil
	}

	if dynStubs := resolveInterfaceDynamic(absPath, astFile, interfaceName); len(dynStubs) > 0 {
		return dynStubs, nil
	}

	if wsStubs, err := resolveInterfaceInWorkspace(filepath.Dir(absPath), interfaceName); err == nil && len(wsStubs) > 0 {
		return wsStubs, nil
	}

	return nil, fmt.Errorf("interface %q could not be resolved from standard library, current file, or workspace packages. Check spelling or package imports", interfaceName)
}

func findExistingMethods(astFile *ast.File, structName string) map[string]bool {
	existingMethods := make(map[string]bool)
	for _, decl := range astFile.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Recv == nil || fn.Name == nil {
			continue
		}
		if isReceiverForStruct(fn.Recv, structName) {
			existingMethods[fn.Name.Name] = true
		}
	}
	return existingMethods
}

func resolveInterfaceDynamic(absPath string, astFile *ast.File, interfaceName string) []MethodStub {
	lastDot := strings.LastIndex(interfaceName, ".")
	if lastDot == -1 {
		return nil
	}

	pkgPart := interfaceName[:lastDot]
	ifaceName := interfaceName[lastDot+1:]

	pkgPath := pkgPart
	if !strings.Contains(pkgPart, "/") {
		for _, imp := range astFile.Imports {
			importPath, _ := strconv.Unquote(imp.Path.Value)
			alias := filepath.Base(importPath)
			if imp.Name != nil && imp.Name.Name != "" {
				alias = imp.Name.Name
			}
			if alias == pkgPart {
				pkgPath = importPath
				break
			}
		}
	}

	cfg := &packages.Config{
		Mode: packages.NeedTypes | packages.NeedImports,
		Dir:  filepath.Dir(absPath),
	}

	pkgs, err := packages.Load(cfg, pkgPath)
	if err != nil || len(pkgs) == 0 || pkgs[0].Types == nil {
		return nil
	}

	obj := pkgs[0].Types.Scope().Lookup(ifaceName)
	if obj == nil {
		return nil
	}

	itype, ok := obj.Type().Underlying().(*types.Interface)
	if !ok {
		return nil
	}

	return extractInterfaceStubsFromType(itype)
}

func extractInterfaceStubsFromType(itype *types.Interface) []MethodStub {
	var stubs []MethodStub
	qualifier := func(pkg *types.Package) string {
		if pkg == nil {
			return ""
		}
		return pkg.Name()
	}

	for i := 0; i < itype.NumMethods(); i++ {
		m := itype.Method(i)
		sig, ok := m.Type().(*types.Signature)
		if !ok {
			continue
		}

		params := formatTuple(sig.Params(), sig.Variadic(), qualifier)
		results := formatResultsTuple(sig.Results(), qualifier)

		stubs = append(stubs, MethodStub{
			Name:    m.Name(),
			Params:  params,
			Results: results,
		})
	}
	return stubs
}

func formatTuple(tuple *types.Tuple, variadic bool, qf types.Qualifier) string {
	if tuple == nil || tuple.Len() == 0 {
		return ""
	}
	var parts []string
	for i := 0; i < tuple.Len(); i++ {
		v := tuple.At(i)
		typeStr := types.TypeString(v.Type(), qf)
		if variadic && i == tuple.Len()-1 {
			if slice, ok := v.Type().(*types.Slice); ok {
				typeStr = "..." + types.TypeString(slice.Elem(), qf)
			}
		}
		if v.Name() != "" {
			parts = append(parts, v.Name()+" "+typeStr)
		} else {
			parts = append(parts, typeStr)
		}
	}
	return strings.Join(parts, ", ")
}

func formatResultsTuple(tuple *types.Tuple, qf types.Qualifier) string {
	if tuple == nil || tuple.Len() == 0 {
		return ""
	}
	joined := formatTuple(tuple, false, qf)
	if tuple.Len() > 1 || (tuple.Len() == 1 && tuple.At(0).Name() != "") {
		return "(" + joined + ")"
	}
	return joined
}

func isReceiverForStruct(recv *ast.FieldList, structName string) bool {
	if recv == nil || len(recv.List) == 0 {
		return false
	}
	t := recv.List[0].Type
	if star, ok := t.(*ast.StarExpr); ok {
		t = star.X
	}
	if id, ok := t.(*ast.Ident); ok {
		return id.Name == structName
	}
	return false
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
	ws, err := FindWorkspace(dirPath)
	if err != nil {
		ws = &Workspace{
			Root:    dirPath,
			Modules: []*ModuleInfo{{Root: dirPath, Path: ""}},
		}
	}

	cleanName := interfaceName
	if idx := strings.LastIndex(cleanName, "."); idx != -1 {
		cleanName = cleanName[idx+1:]
	}

	var foundStubs []MethodStub
	_ = ws.WalkGoFiles(func(path string, _ *ModuleInfo) error {
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
