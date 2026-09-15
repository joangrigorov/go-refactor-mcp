package tests

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joangrigorov/go-refactor-mcp/internal/server"
)

func verifyCrossPlatformCompilation(t *testing.T, dir string, targetOS string) {
	t.Helper()
	cmd := exec.Command("go", "build", "./...") // #nosec G204
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GOOS="+targetOS)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("cross-platform compilation failed for GOOS=%s in %s: %v\nOutput:\n%s", targetOS, dir, err, string(out))
	}
}

func TestMCPIntegration_CrossPlatformRename_MultiOSHappyPath(t *testing.T) {
	s := server.NewServer()
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module example.com/crosstest\n\ngo 1.22.0\n"), 0600)

	runnerGo := `package main

import "fmt"

func main() {
	fmt.Println(InitPlatform())
}
`
	runnerLinux := `package main

func InitPlatform() string {
	return "linux"
}
`
	runnerWindows := `package main

func InitPlatform() string {
	return "windows"
}
`
	runnerDarwin := `package main

func InitPlatform() string {
	return "darwin"
}
`

	_ = os.WriteFile(filepath.Join(tempDir, "runner.go"), []byte(runnerGo), 0600)
	_ = os.WriteFile(filepath.Join(tempDir, "runner_linux.go"), []byte(runnerLinux), 0600)
	_ = os.WriteFile(filepath.Join(tempDir, "runner_windows.go"), []byte(runnerWindows), 0600)
	_ = os.WriteFile(filepath.Join(tempDir, "runner_darwin.go"), []byte(runnerDarwin), 0600)

	res := callMCPTool(t, s, "rename_symbol", map[string]any{
		"directory": tempDir,
		"from":      "InitPlatform",
		"to":        "InitializePlatform",
	})

	if res.IsError {
		t.Fatalf("expected rename_symbol to succeed, got error: %s", getResultText(t, res))
	}

	for _, fname := range []string{"runner.go", "runner_linux.go", "runner_windows.go", "runner_darwin.go"} {
		content, err := os.ReadFile(filepath.Join(tempDir, fname)) // #nosec G304
		if err != nil {
			t.Fatalf("failed reading %s: %v", fname, err)
		}
		cStr := string(content)
		if !strings.Contains(cStr, "InitializePlatform") {
			t.Errorf("expected %s to contain 'InitializePlatform', got:\n%s", fname, cStr)
		}
		if strings.Contains(cStr, "InitPlatform(") || strings.Contains(cStr, "InitPlatform()") {
			t.Errorf("expected %s to NOT contain 'InitPlatform', got:\n%s", fname, cStr)
		}
	}

	// Tri-platform compiler verification across Linux, Windows, Darwin
	verifyCrossPlatformCompilation(t, tempDir, "linux")
	verifyCrossPlatformCompilation(t, tempDir, "windows")
	verifyCrossPlatformCompilation(t, tempDir, "darwin")
}

func TestMCPIntegration_CrossPlatformRename_WindowsOnlySymbol(t *testing.T) {
	s := server.NewServer()
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module example.com/crosstestwin\n\ngo 1.22.0\n"), 0600)

	runnerGo := `package main

func main() {}
`
	runnerWindows := `package main

func WindowsSpecificHelper() string {
	return "win_ok"
}

func CallWin() string {
	return WindowsSpecificHelper()
}
`

	_ = os.WriteFile(filepath.Join(tempDir, "runner.go"), []byte(runnerGo), 0600)
	_ = os.WriteFile(filepath.Join(tempDir, "runner_windows.go"), []byte(runnerWindows), 0600)

	res := callMCPTool(t, s, "rename_symbol", map[string]any{
		"directory": tempDir,
		"from":      "WindowsSpecificHelper",
		"to":        "WindowsRenamedHelper",
		"file":      filepath.Join(tempDir, "runner_windows.go"),
	})

	if res.IsError {
		t.Fatalf("expected rename_symbol to succeed for Windows-only symbol, got: %s", getResultText(t, res))
	}

	winContent, err := os.ReadFile(filepath.Join(tempDir, "runner_windows.go")) // #nosec G304
	if err != nil {
		t.Fatalf("failed reading runner_windows.go: %v", err)
	}
	if !strings.Contains(string(winContent), "WindowsRenamedHelper") {
		t.Errorf("expected runner_windows.go to contain 'WindowsRenamedHelper', got:\n%s", string(winContent))
	}

	verifyCrossPlatformCompilation(t, tempDir, "windows")
}

func TestMCPIntegration_CrossPlatformRename_SyntaxErrorInPlatformFile(t *testing.T) {
	s := server.NewServer()
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module example.com/brokenwin\n\ngo 1.22.0\n"), 0600)

	runnerGo := `package main

func main() {}
`
	runnerWindowsBroken := `package main

func InvalidSyntaxFunction(((( {
	return "error"
}
`

	_ = os.WriteFile(filepath.Join(tempDir, "runner.go"), []byte(runnerGo), 0600)
	_ = os.WriteFile(filepath.Join(tempDir, "runner_windows.go"), []byte(runnerWindowsBroken), 0600)

	res := callMCPTool(t, s, "rename_symbol", map[string]any{
		"directory": tempDir,
		"from":      "main",
		"to":        "mainRenamed",
	})

	if !res.IsError {
		t.Fatalf("expected error due to syntax error in runner_windows.go, but got success")
	}

	errMsg := getResultText(t, res)
	if !strings.Contains(errMsg, "syntax error") && !strings.Contains(errMsg, "windows platform") {
		t.Errorf("expected actionable error mentioning syntax error and windows platform, got: %s", errMsg)
	}
}
