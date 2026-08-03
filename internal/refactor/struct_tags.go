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
	"reflect"
	"strconv"
	"strings"
)

// AddStructTagsOptions specifies arguments for adding struct field tags.
type AddStructTagsOptions struct {
	FilePath   string
	StructName string
	Tags       []string // e.g. ["json", "yaml", "db"]
}

// AddStructTags appends or updates struct field tags converting CamelCase names to snake_case.
func AddStructTags(opts AddStructTagsOptions) error {
	absPath, err := filepath.Abs(opts.FilePath)
	if err != nil {
		return fmt.Errorf("invalid file path: %w", err)
	}

	if len(opts.Tags) == 0 {
		opts.Tags = []string{"json", "yaml"}
	}

	fset := token.NewFileSet()
	astFile, err := parser.ParseFile(fset, absPath, nil, parser.ParseComments)
	if err != nil {
		return fmt.Errorf("failed parsing file: %w", err)
	}

	var targetStruct *ast.StructType
	for _, decl := range astFile.Decls {
		g, ok := decl.(*ast.GenDecl)
		if !ok {
			continue
		}
		for _, spec := range g.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if ok && ts.Name.Name == opts.StructName {
				if st, ok := ts.Type.(*ast.StructType); ok {
					targetStruct = st
					break
				}
			}
		}
		if targetStruct != nil {
			break
		}
	}

	if targetStruct == nil {
		return fmt.Errorf("struct %s not found in %s", opts.StructName, opts.FilePath)
	}

	modified := false
	for _, field := range targetStruct.Fields.List {
		if len(field.Names) == 0 {
			continue // Embedded field
		}
		fieldName := field.Names[0].Name
		snakeName := CamelToSnake(fieldName)

		existingTagVal := ""
		if field.Tag != nil {
			unquoted, err := strconv.Unquote(field.Tag.Value)
			if err == nil {
				existingTagVal = unquoted
			}
		}

		tagStruct := reflect.StructTag(existingTagVal)
		tagParts := parseStructTag(existingTagVal)

		for _, t := range opts.Tags {
			cleanTag := strings.TrimSpace(t)
			if cleanTag == "" {
				continue
			}
			if _, exists := tagStruct.Lookup(cleanTag); !exists {
				tagParts = append(tagParts, fmt.Sprintf(`%s:"%s"`, cleanTag, snakeName))
				modified = true
			}
		}

		if modified {
			newTagStr := strings.Join(tagParts, " ")
			field.Tag = &ast.BasicLit{
				Kind:  token.STRING,
				Value: "`" + newTagStr + "`",
			}
		}
	}

	if !modified {
		return nil // Idempotent success
	}

	var buf bytes.Buffer
	if err := format.Node(&buf, fset, astFile); err != nil {
		return fmt.Errorf("failed formatting ast: %w", err)
	}

	return os.WriteFile(absPath, buf.Bytes(), 0600)
}

func parseStructTag(tagStr string) []string {
	if tagStr == "" {
		return nil
	}
	return strings.Fields(tagStr)
}
