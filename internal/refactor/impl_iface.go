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

// stdlibShorthands maps common standard library package shorthands without directory paths to canonical import paths.
var stdlibShorthands = map[string]string{
	"http":         "net/http",
	"url":          "net/url",
	"mail":         "net/mail",
	"rpc":          "net/rpc",
	"smtp":         "net/smtp",
	"sql":          "database/sql",
	"driver":       "database/sql/driver",
	"json":         "encoding/json",
	"xml":          "encoding/xml",
	"csv":          "encoding/csv",
	"base64":       "encoding/base64",
	"hex":          "encoding/hex",
	"binary":       "encoding/binary",
	"tar":          "archive/tar",
	"zip":          "archive/zip",
	"atomic":       "sync/atomic",
	"slog":         "log/slog",
	"syslog":       "log/syslog",
	"tabwriter":    "text/tabwriter",
	"scanner":      "text/scanner",
	"template":     "text/template",
	"htmltemplate": "html/template",
	"fstest":       "testing/fstest",
	"iotest":       "testing/iotest",
	"quick":        "testing/quick",
	"pprof":        "runtime/pprof",
	"trace":        "runtime/trace",
	"tls":          "crypto/tls",
	"x509":         "crypto/x509",
	"rsa":          "crypto/rsa",
	"ecdsa":        "crypto/ecdsa",
	"ed25519":      "crypto/ed25519",
	"rand":         "crypto/rand",
	"sha256":       "crypto/sha256",
	"sha512":       "crypto/sha512",
	"sha1":         "crypto/sha1",
	"md5":          "crypto/md5",
	"hmac":         "crypto/hmac",
	"cipher":       "crypto/cipher",
	"subtle":       "crypto/subtle",
	"color":        "image/color",
	"draw":         "image/draw",
	"gif":          "image/gif",
	"jpeg":         "image/jpeg",
	"png":          "image/png",
	"heap":         "container/heap",
	"list":         "container/list",
	"ring":         "container/ring",
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
	return resolveStubsInternal(absPath, fset, astFile, interfaceName, make(map[string]bool))
}

func resolveStubsInternal(absPath string, fset *token.FileSet, astFile *ast.File, interfaceName string, visited map[string]bool) ([]MethodStub, error) {
	if stubs, ok := standardInterfaces[interfaceName]; ok {
		return stubs, nil
	}

	localStubs, err := resolveLocalInterface(absPath, fset, astFile, interfaceName, visited)
	if err == nil && len(localStubs) > 0 {
		return localStubs, nil
	}
	if err != nil && (strings.Contains(err.Error(), "failed to resolve embedded interface") || strings.Contains(err.Error(), "not an interface")) {
		return nil, err
	}

	dynStubs, dynErr := resolveInterfaceDynamic(absPath, astFile, interfaceName)
	if dynErr != nil {
		return nil, dynErr
	}
	if len(dynStubs) > 0 {
		return dynStubs, nil
	}

	if wsStubs, wsErr := resolveInterfaceInWorkspace(filepath.Dir(absPath), interfaceName); wsErr == nil && len(wsStubs) > 0 {
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

func resolveInterfaceDynamic(absPath string, astFile *ast.File, interfaceName string) ([]MethodStub, error) {
	lastDot := strings.LastIndex(interfaceName, ".")
	if lastDot == -1 {
		return nil, nil
	}

	pkgPart := interfaceName[:lastDot]
	ifaceName := interfaceName[lastDot+1:]

	pkgPath := pkgPart
	if !strings.Contains(pkgPart, "/") {
		matchedImport := false
		if astFile != nil {
			for _, imp := range astFile.Imports {
				importPath, _ := strconv.Unquote(imp.Path.Value)
				alias := filepath.Base(importPath)
				if imp.Name != nil && imp.Name.Name != "" {
					alias = imp.Name.Name
				}
				if alias == pkgPart {
					pkgPath = importPath
					matchedImport = true
					break
				}
			}
		}
		if !matchedImport {
			if stdlibPath, ok := stdlibShorthands[pkgPart]; ok {
				pkgPath = stdlibPath
			}
		}
	}

	dir := "."
	if absPath != "" {
		dir = filepath.Dir(absPath)
	}

	cfg := &packages.Config{
		Mode: packages.NeedTypes | packages.NeedImports,
		Dir:  dir,
	}

	pkgs, err := packages.Load(cfg, pkgPath)
	if err != nil || len(pkgs) == 0 || pkgs[0].Types == nil {
		return nil, nil
	}

	obj := pkgs[0].Types.Scope().Lookup(ifaceName)
	if obj == nil {
		return nil, nil
	}

	itype, ok := obj.Type().Underlying().(*types.Interface)
	if !ok {
		return nil, fmt.Errorf("type %q in package %q is a %s, not an interface", ifaceName, pkgPath, typeKind(obj.Type().Underlying()))
	}

	return extractInterfaceStubsFromType(itype, astFile), nil
}

func extractInterfaceStubsFromType(itype *types.Interface, astFile *ast.File) []MethodStub {
	var stubs []MethodStub

	targetPkgName := ""
	aliasMap := make(map[string]string)
	if astFile != nil {
		if astFile.Name != nil {
			targetPkgName = astFile.Name.Name
		}
		for _, imp := range astFile.Imports {
			if imp.Path != nil {
				p, err := strconv.Unquote(imp.Path.Value)
				if err == nil && imp.Name != nil && imp.Name.Name != "" {
					aliasMap[p] = imp.Name.Name
				}
			}
		}
	}

	qualifier := func(pkg *types.Package) string {
		if pkg == nil || pkg.Name() == targetPkgName {
			return ""
		}
		if alias, ok := aliasMap[pkg.Path()]; ok {
			return alias
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
	return deduplicateStubs(stubs)
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

func resolveLocalInterface(absPath string, fset *token.FileSet, astFile *ast.File, interfaceName string, visited map[string]bool) ([]MethodStub, error) {
	cleanName := interfaceName
	if idx := strings.LastIndex(cleanName, "."); idx != -1 {
		cleanName = cleanName[idx+1:]
	}

	if visited[cleanName] {
		return nil, nil
	}
	visited[cleanName] = true

	var foundTypeSpec *ast.TypeSpec
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
			foundTypeSpec = ts
			break
		}
		if foundTypeSpec != nil {
			break
		}
	}

	if foundTypeSpec == nil {
		return nil, fmt.Errorf("interface %q not found in file", cleanName)
	}

	itype, ok := foundTypeSpec.Type.(*ast.InterfaceType)
	if !ok {
		return nil, fmt.Errorf("type %q is a %s, not an interface", cleanName, astTypeKind(foundTypeSpec.Type))
	}

	var stubs []MethodStub
	if itype.Methods != nil {
		for _, m := range itype.Methods.List {
			// Case 1: Standard method declaration
			if ft, ok := m.Type.(*ast.FuncType); ok {
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
				continue
			}

			// Case 2: Embedded local interface (*ast.Ident)
			if id, ok := m.Type.(*ast.Ident); ok {
				embeddedStubs, err := resolveLocalInterface(absPath, fset, astFile, id.Name, visited)
				if err == nil && len(embeddedStubs) > 0 {
					stubs = append(stubs, embeddedStubs...)
					continue
				}
				if absPath != "" {
					wsStubs, wsErr := resolveInterfaceInWorkspace(filepath.Dir(absPath), id.Name)
					if wsErr == nil && len(wsStubs) > 0 {
						stubs = append(stubs, wsStubs...)
						continue
					}
				}
				return nil, fmt.Errorf("failed to resolve embedded interface %q in %q: interface could not be found", id.Name, cleanName)
			}

			// Case 3: Embedded imported interface (*ast.SelectorExpr, e.g. io.Reader or io.ReadCloser)
			if sel, ok := m.Type.(*ast.SelectorExpr); ok {
				if xIdent, ok := sel.X.(*ast.Ident); ok {
					embeddedIface := xIdent.Name + "." + sel.Sel.Name
					embeddedStubs, err := resolveStubsInternal(absPath, fset, astFile, embeddedIface, visited)
					if err != nil || len(embeddedStubs) == 0 {
						return nil, fmt.Errorf("failed to resolve embedded interface %q in %q: %w", embeddedIface, cleanName, err)
					}
					stubs = append(stubs, embeddedStubs...)
				}
			}
		}
	}

	return deduplicateStubs(stubs), nil
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
		stubs, resolveErr := resolveLocalInterface(path, fset, fileAST, cleanName, make(map[string]bool))
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

func deduplicateStubs(stubs []MethodStub) []MethodStub {
	seen := make(map[string]bool)
	var deduped []MethodStub
	for _, s := range stubs {
		if !seen[s.Name] {
			seen[s.Name] = true
			deduped = append(deduped, s)
		}
	}
	return deduped
}

func typeKind(t types.Type) string {
	switch t.(type) {
	case *types.Struct:
		return "struct"
	case *types.Signature:
		return "function"
	case *types.Basic:
		return "basic type"
	case *types.Pointer:
		return "pointer"
	default:
		return "non-interface type"
	}
}

func astTypeKind(expr ast.Expr) string {
	switch expr.(type) {
	case *ast.StructType:
		return "struct"
	case *ast.FuncType:
		return "function"
	case *ast.ArrayType:
		return "slice/array"
	case *ast.MapType:
		return "map"
	default:
		return "non-interface type"
	}
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
