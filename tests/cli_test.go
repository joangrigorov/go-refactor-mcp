package tests

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestCLIHelpAndVersion(t *testing.T) {
	tempDir := t.TempDir()
	binaryName := "go-refactor-mcp"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	binaryPath := filepath.Join(tempDir, binaryName)

	// Build binary for testing CLI flags
	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../main.go") //nolint:gosec
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed building test binary: %v\nOutput: %s", err, string(out))
	}

	testCases := []struct {
		name           string
		args           []string
		expectedOutput string
	}{
		{
			name:           "dash-h",
			args:           []string{"-h"},
			expectedOutput: "go-refactor-mcp - Model Context Protocol",
		},
		{
			name:           "double-dash-help",
			args:           []string{"--help"},
			expectedOutput: "Available MCP Tools:",
		},
		{
			name:           "help-command",
			args:           []string{"help"},
			expectedOutput: "rename_symbol",
		},
		{
			name:           "dash-v",
			args:           []string{"-v"},
			expectedOutput: "go-refactor-mcp version 1.0.0",
		},
		{
			name:           "double-dash-version",
			args:           []string{"--version"},
			expectedOutput: "go-refactor-mcp version 1.0.0",
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			cmd := exec.Command(binaryPath, tc.args...) //nolint:gosec
			outBytes, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("command failed with args %v: %v\nOutput: %s", tc.args, err, string(outBytes))
			}
			outStr := string(outBytes)
			if !strings.Contains(outStr, tc.expectedOutput) {
				t.Errorf("expected output to contain %q, got: %q", tc.expectedOutput, outStr)
			}
		})
	}
}
