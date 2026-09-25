package tests

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joangrigorov/go-refactor-mcp/internal/server"
	"github.com/mark3labs/mcp-go/mcp"
	mcpServer "github.com/mark3labs/mcp-go/server"
)

func callMCPTool(t *testing.T, s *mcpServer.MCPServer, toolName string, args map[string]any) *mcp.CallToolResult {
	t.Helper()
	st := s.GetTool(toolName)
	if st == nil {
		t.Fatalf("tool %q not registered on server", toolName)
	}
	req := mcp.CallToolRequest{
		Params: mcp.CallToolParams{
			Name:      toolName,
			Arguments: args,
		},
	}
	res, err := st.Handler(context.Background(), req)
	if err != nil {
		t.Fatalf("unexpected handler error: %v", err)
	}
	return res
}

func getResultText(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	if res == nil || len(res.Content) == 0 {
		t.Fatalf("expected non-empty CallToolResult content")
	}
	if textContent, ok := res.Content[0].(mcp.TextContent); ok {
		return textContent.Text
	}
	t.Fatalf("first content block is not TextContent: %T", res.Content[0])
	return ""
}

func verifyCompilation(t *testing.T, dir string) {
	t.Helper()
	cmd := exec.Command("go", "build", "./...") // #nosec G204
	cmd.Dir = dir
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("post-refactor compilation failed in %s: %v\nOutput:\n%s", dir, err, string(out))
	}
}

func TestMCPIntegration_RenameSymbol_HappyPath(t *testing.T) {
	s := server.NewServer()
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module example.com/mcprenametest\n\ngo 1.22.0\n"), 0600)

	pkgA := filepath.Join(tempDir, "pkgA")
	_ = os.MkdirAll(pkgA, 0750)
	codeA := `package pkgA

func CalculateTotal(a, b int) int {
	return a + b
}
`
	_ = os.WriteFile(filepath.Join(pkgA, "calc.go"), []byte(codeA), 0600)

	pkgB := filepath.Join(tempDir, "pkgB")
	_ = os.MkdirAll(pkgB, 0750)
	codeB := `package pkgB

import (
	"fmt"
	"example.com/mcprenametest/pkgA"
)

func Run() {
	res := pkgA.CalculateTotal(1, 2)
	fmt.Println(res)
}
`
	_ = os.WriteFile(filepath.Join(pkgB, "runner.go"), []byte(codeB), 0600)

	// Call rename_symbol through MCP server
	res := callMCPTool(t, s, "rename_symbol", map[string]any{
		"directory": tempDir,
		"from":      "CalculateTotal",
		"to":        "ComputeTotalSum",
	})

	if res.IsError {
		t.Fatalf("expected success, got error: %s", getResultText(t, res))
	}
	text := getResultText(t, res)
	if !strings.Contains(text, "Successfully renamed 'CalculateTotal' to 'ComputeTotalSum'") {
		t.Errorf("unexpected success text: %s", text)
	}

	// Verify both packages updated
	bBytes, _ := os.ReadFile(filepath.Join(pkgB, "runner.go")) // #nosec G304
	if !strings.Contains(string(bBytes), "pkgA.ComputeTotalSum(1, 2)") {
		t.Errorf("runner.go was not updated with new symbol name: %s", string(bBytes))
	}

	// Verify workspace compiles cleanly
	verifyCompilation(t, tempDir)
}

func TestMCPIntegration_RenameSymbol_OffsetAndFile(t *testing.T) {
	s := server.NewServer()
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module example.com/mcpoffsettest\n\ngo 1.22.0\n"), 0600)

	code := `package main

type Config struct {
	Timeout int
}

func main() {
	c := Config{Timeout: 30}
	_ = c.Timeout
}
`
	mainFile := filepath.Join(tempDir, "main.go")
	_ = os.WriteFile(mainFile, []byte(code), 0600)

	offset := strings.Index(code, "Timeout")
	if offset == -1 {
		t.Fatalf("failed to locate Timeout in test code")
	}

	res := callMCPTool(t, s, "rename_symbol", map[string]any{
		"directory": tempDir,
		"file":      mainFile,
		"offset":    offset,
		"from":      "Timeout",
		"to":        "TimeoutDuration",
	})

	if res.IsError {
		t.Fatalf("expected success, got error: %s", getResultText(t, res))
	}

	updatedBytes, _ := os.ReadFile(mainFile) // #nosec G304
	if !strings.Contains(string(updatedBytes), "TimeoutDuration int") {
		t.Errorf("expected struct field renamed to TimeoutDuration, got: %s", string(updatedBytes))
	}

	verifyCompilation(t, tempDir)
}

func TestMCPIntegration_RenameSymbol_ActionableErrors(t *testing.T) {
	s := server.NewServer()

	// Missing directory
	res := callMCPTool(t, s, "rename_symbol", map[string]any{
		"directory": "",
		"from":      "A",
		"to":        "B",
	})
	if !res.IsError || !strings.Contains(getResultText(t, res), "argument 'directory' is required") {
		t.Errorf("expected missing directory error, got: %s", getResultText(t, res))
	}

	// Missing from
	res = callMCPTool(t, s, "rename_symbol", map[string]any{
		"directory": ".",
		"from":      "",
		"to":        "B",
	})
	if !res.IsError || !strings.Contains(getResultText(t, res), "argument 'from' is required") {
		t.Errorf("expected missing from error, got: %s", getResultText(t, res))
	}

	// Invalid Go keyword target
	res = callMCPTool(t, s, "rename_symbol", map[string]any{
		"directory": ".",
		"from":      "Foo",
		"to":        "var",
	})
	if !res.IsError || !strings.Contains(getResultText(t, res), "not a valid Go identifier") {
		t.Errorf("expected invalid identifier error for keyword 'var', got: %s", getResultText(t, res))
	}

	// Invalid token target
	res = callMCPTool(t, s, "rename_symbol", map[string]any{
		"directory": ".",
		"from":      "Foo",
		"to":        "123invalid",
	})
	if !res.IsError || !strings.Contains(getResultText(t, res), "not a valid Go identifier") {
		t.Errorf("expected invalid identifier error for '123invalid', got: %s", getResultText(t, res))
	}
}

func TestMCPIntegration_MoveFile_HappyPath(t *testing.T) {
	s := server.NewServer()
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module example.com/mcpmovetest\n\ngo 1.22.0\n"), 0600)

	srcDir := filepath.Join(tempDir, "sourcepkg")
	_ = os.MkdirAll(srcDir, 0750)
	destDir := filepath.Join(tempDir, "destpkg")

	serviceFile := filepath.Join(srcDir, "service.go")
	testFile := filepath.Join(srcDir, "service_test.go")

	_ = os.WriteFile(serviceFile, []byte("package sourcepkg\n\nfunc DoService() string { return \"ok\" }\n"), 0600)
	_ = os.WriteFile(testFile, []byte("package sourcepkg_test\n\nimport \"testing\"\n\nfunc TestDoService(t *testing.T) {}\n"), 0600)

	// Call move_file
	res := callMCPTool(t, s, "move_file", map[string]any{
		"directory":   tempDir,
		"source_file": serviceFile,
		"dest_dir":    destDir,
	})

	if res.IsError {
		t.Fatalf("expected success, got error: %s", getResultText(t, res))
	}

	// Verify both main file and companion test file moved
	movedService := filepath.Join(destDir, "service.go")
	movedTest := filepath.Join(destDir, "service_test.go")

	if _, err := os.Stat(movedService); err != nil {
		t.Errorf("moved service file not found at %s", movedService)
	}
	if _, err := os.Stat(movedTest); err != nil {
		t.Errorf("moved test companion file not found at %s", movedTest)
	}

	content, _ := os.ReadFile(movedService) // #nosec G304
	if !strings.Contains(string(content), "package destpkg") {
		t.Errorf("expected package destpkg, got: %s", string(content))
	}

	verifyCompilation(t, tempDir)
}

func TestMCPIntegration_MoveFile_RenameInPlace(t *testing.T) {
	s := server.NewServer()
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module example.com/mcprenametest\n\ngo 1.22.0\n"), 0600)

	oldFile := filepath.Join(tempDir, "legacy.go")
	_ = os.WriteFile(oldFile, []byte("package main\n\nfunc main() {}\n"), 0600)

	res := callMCPTool(t, s, "move_file", map[string]any{
		"directory":   tempDir,
		"source_file": oldFile,
		"new_name":    "app.go",
	})

	if res.IsError {
		t.Fatalf("expected success, got error: %s", getResultText(t, res))
	}

	if _, err := os.Stat(filepath.Join(tempDir, "app.go")); err != nil {
		t.Errorf("renamed file app.go does not exist: %v", err)
	}
	if _, err := os.Stat(oldFile); err == nil {
		t.Errorf("old file legacy.go still exists")
	}

	verifyCompilation(t, tempDir)
}

func TestMCPIntegration_MoveFile_ActionableErrors(t *testing.T) {
	s := server.NewServer()
	tempDir := t.TempDir()

	// 0. Missing directory
	res := callMCPTool(t, s, "move_file", map[string]any{
		"source_file": "sample.go",
		"dest_dir":    tempDir,
	})
	if !res.IsError || !strings.Contains(getResultText(t, res), "argument 'directory' is required") {
		t.Errorf("expected directory required error, got: %s", getResultText(t, res))
	}

	// 1. Missing source_file
	res = callMCPTool(t, s, "move_file", map[string]any{
		"directory":   tempDir,
		"source_file": "",
		"dest_dir":    tempDir,
	})
	if !res.IsError || !strings.Contains(getResultText(t, res), "argument 'source_file' is required") {
		t.Errorf("expected source_file required error, got: %s", getResultText(t, res))
	}

	// 2. Non-Go file
	readmeFile := filepath.Join(tempDir, "README.md")
	_ = os.WriteFile(readmeFile, []byte("# Docs"), 0600)
	res = callMCPTool(t, s, "move_file", map[string]any{
		"directory":   tempDir,
		"source_file": readmeFile,
		"dest_dir":    tempDir,
	})
	if !res.IsError || !strings.Contains(getResultText(t, res), "must be a .go file") {
		t.Errorf("expected must be a .go file error, got: %s", getResultText(t, res))
	}

	// 3. Neither dest_dir nor new_name
	sampleFile := filepath.Join(tempDir, "sample.go")
	_ = os.WriteFile(sampleFile, []byte("package main\n"), 0600)
	res = callMCPTool(t, s, "move_file", map[string]any{
		"directory":   tempDir,
		"source_file": sampleFile,
	})
	if !res.IsError || !strings.Contains(getResultText(t, res), "at least one of 'dest_dir' (to move file) or 'new_name'") {
		t.Errorf("expected at least one required error, got: %s", getResultText(t, res))
	}

	// 4. Destination file collision
	destDir := filepath.Join(tempDir, "target")
	_ = os.MkdirAll(destDir, 0750)
	existingDestFile := filepath.Join(destDir, "sample.go")
	_ = os.WriteFile(existingDestFile, []byte("package target\n"), 0600)

	_ = os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module example.com/colltest\n\ngo 1.22.0\n"), 0600)

	res = callMCPTool(t, s, "move_file", map[string]any{
		"directory":   tempDir,
		"source_file": sampleFile,
		"dest_dir":    destDir,
	})
	if !res.IsError || !strings.Contains(getResultText(t, res), "already exists") {
		t.Errorf("expected destination file already exists error, got: %s", getResultText(t, res))
	}

	// 5. Source file outside workspace directory
	outsideFile := filepath.Join(t.TempDir(), "outside.go")
	_ = os.WriteFile(outsideFile, []byte("package outside\n"), 0600)
	res = callMCPTool(t, s, "move_file", map[string]any{
		"directory":   tempDir,
		"source_file": outsideFile,
		"dest_dir":    destDir,
	})
	if !res.IsError || !strings.Contains(getResultText(t, res), "is outside workspace directory") {
		t.Errorf("expected outside workspace error, got: %s", getResultText(t, res))
	}
}

func TestMCPIntegration_MoveDirectory_HappyPath(t *testing.T) {
	s := server.NewServer()
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module example.com/mcpdirmovetest\n\ngo 1.22.0\n"), 0600)

	sourceDir := filepath.Join(tempDir, "mathops")
	_ = os.MkdirAll(sourceDir, 0750)
	_ = os.WriteFile(filepath.Join(sourceDir, "add.go"), []byte("package mathops\n\nfunc Add(a, b int) int { return a + b }\n"), 0600)

	consumerDir := filepath.Join(tempDir, "consumer")
	_ = os.MkdirAll(consumerDir, 0750)
	consumerCode := `package consumer

import (
	"fmt"
	"example.com/mcpdirmovetest/mathops"
)

func Compute() {
	fmt.Println(mathops.Add(1, 2))
}
`
	_ = os.WriteFile(filepath.Join(consumerDir, "compute.go"), []byte(consumerCode), 0600)

	destDir := filepath.Join(tempDir, "pkg", "mathops")

	res := callMCPTool(t, s, "move_directory", map[string]any{
		"directory":  tempDir,
		"source_dir": sourceDir,
		"dest_dir":   destDir,
	})

	if res.IsError {
		t.Fatalf("expected success, got error: %s", getResultText(t, res))
	}

	consumerBytes, _ := os.ReadFile(filepath.Join(consumerDir, "compute.go")) // #nosec G304
	if !strings.Contains(string(consumerBytes), `"example.com/mcpdirmovetest/pkg/mathops"`) {
		t.Errorf("expected updated import path in consumer: %s", string(consumerBytes))
	}

	verifyCompilation(t, tempDir)
}

func TestMCPIntegration_MoveDirectory_ActionableErrors(t *testing.T) {
	s := server.NewServer()
	tempDir := t.TempDir()

	// 0. Missing directory
	res := callMCPTool(t, s, "move_directory", map[string]any{
		"source_dir": "mathops",
		"dest_dir":   tempDir,
	})
	if !res.IsError || !strings.Contains(getResultText(t, res), "argument 'directory' is required") {
		t.Errorf("expected directory required error, got: %s", getResultText(t, res))
	}

	// 1. Missing source_dir
	res = callMCPTool(t, s, "move_directory", map[string]any{
		"directory":  tempDir,
		"source_dir": "",
		"dest_dir":   tempDir,
	})
	if !res.IsError || !strings.Contains(getResultText(t, res), "argument 'source_dir' is required") {
		t.Errorf("expected missing source_dir error, got: %s", getResultText(t, res))
	}

	// 2. Source is a file, not a directory -> should suggest move_file
	filePath := filepath.Join(tempDir, "app.go")
	_ = os.WriteFile(filePath, []byte("package main\n"), 0600)
	res = callMCPTool(t, s, "move_directory", map[string]any{
		"directory":  tempDir,
		"source_dir": filePath,
		"dest_dir":   filepath.Join(tempDir, "dest"),
	})
	if !res.IsError || !strings.Contains(getResultText(t, res), "is a file, not a directory; use 'move_file' instead") {
		t.Errorf("expected hint to use move_file, got: %s", getResultText(t, res))
	}

	// 3. Source directory outside workspace
	outsideDir := t.TempDir()
	res = callMCPTool(t, s, "move_directory", map[string]any{
		"directory":  tempDir,
		"source_dir": outsideDir,
		"dest_dir":   filepath.Join(tempDir, "dest"),
	})
	if !res.IsError || !strings.Contains(getResultText(t, res), "is outside workspace directory") {
		t.Errorf("expected outside workspace directory error, got: %s", getResultText(t, res))
	}
}

