package tests

import (
	"bufio"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
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
		name             string
		args             []string
		expectedOutput   string
		unexpectedOutput string
	}{
		{
			name:             "dash-h",
			args:             []string{"-h"},
			expectedOutput:   "go-refactor-mcp - Model Context Protocol",
			unexpectedOutput: "implement_interface",
		},
		{
			name:             "double-dash-help",
			args:             []string{"--help"},
			expectedOutput:   "Available MCP Tools:",
			unexpectedOutput: "analyze_shadowing",
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
			if tc.unexpectedOutput != "" && strings.Contains(outStr, tc.unexpectedOutput) {
				t.Errorf("expected output NOT to contain %q, got: %q", tc.unexpectedOutput, outStr)
			}
		})
	}
}

func TestMCPModernProtocolHandshake(t *testing.T) {
	tempDir := t.TempDir()
	binaryName := "go-refactor-mcp"
	if runtime.GOOS == "windows" {
		binaryName += ".exe"
	}
	binaryPath := filepath.Join(tempDir, binaryName)

	buildCmd := exec.Command("go", "build", "-o", binaryPath, "../main.go") //nolint:gosec
	if out, err := buildCmd.CombinedOutput(); err != nil {
		t.Fatalf("failed building test binary: %v\nOutput: %s", err, string(out))
	}

	cmd := exec.Command(binaryPath) //nolint:gosec
	stdin, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("failed opening stdin: %v", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		t.Fatalf("failed opening stdout: %v", err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed starting binary: %v", err)
	}
	defer func() {
		_ = cmd.Process.Kill()
	}()

	// 1. Send modern protocol server/discover
	discoverReq := `{"jsonrpc":"2.0","id":1,"method":"server/discover","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{}}}}` + "\n"
	if _, err := stdin.Write([]byte(discoverReq)); err != nil {
		t.Fatalf("failed writing discover: %v", err)
	}

	// Read discover response
	r := bufio.NewReader(stdout)
	line, err := r.ReadString('\n')
	if err != nil {
		t.Fatalf("failed reading discover response: %v", err)
	}
	t.Logf("discover response: %s", line)
	if !strings.Contains(line, `"id":1`) {
		t.Fatalf("unexpected discover response: %s", line)
	}

	// 2. Send subscriptions/listen (this must not hang stdio)
	listenReq := `{"jsonrpc":"2.0","id":2,"method":"subscriptions/listen","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{}},"notifications":{"toolsListChanged":true}}}` + "\n"
	if _, err := stdin.Write([]byte(listenReq)); err != nil {
		t.Fatalf("failed writing listen: %v", err)
	}

	// 3. Send tools/list
	toolsListReq := `{"jsonrpc":"2.0","id":3,"method":"tools/list","params":{"_meta":{"io.modelcontextprotocol/protocolVersion":"2026-07-28","io.modelcontextprotocol/clientCapabilities":{}}}}` + "\n"
	if _, err := stdin.Write([]byte(toolsListReq)); err != nil {
		t.Fatalf("failed writing tools/list: %v", err)
	}

	// Verify we can read responses without hanging (with channel timeout)
	readCh := make(chan string, 10)
	errCh := make(chan error, 1)
	go func() {
		for {
			l, err := r.ReadString('\n')
			if err != nil {
				errCh <- err
				return
			}
			t.Logf("received line: %s", l)
			readCh <- l
		}
	}()

	foundToolsList := false
	timeout := time.After(3 * time.Second)
	for !foundToolsList {
		select {
		case l := <-readCh:
			if strings.Contains(l, `"id":3`) && strings.Contains(l, "rename_symbol") {
				foundToolsList = true
			}
		case err := <-errCh:
			t.Fatalf("read error waiting for tools/list response: %v", err)
		case <-timeout:
			t.Fatal("timed out waiting for tools/list response: server hung on modern protocol handshake")
		}
	}
}

