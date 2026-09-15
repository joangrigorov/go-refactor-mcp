package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joangrigorov/go-refactor-mcp/internal/refactor"
)

func TestMonorepoMultiModuleDiscoveryAndRefactoring(t *testing.T) {
	tempDir := t.TempDir()

	// Monorepo root without go.work, containing services/auth and services/gateway
	authDir := filepath.Join(tempDir, "services", "auth")
	_ = os.MkdirAll(filepath.Join(authDir, "token"), 0750)
	_ = os.WriteFile(filepath.Join(authDir, "go.mod"), []byte("module monorepo.com/auth\n\ngo 1.22.0\n"), 0600)

	tokenFile := filepath.Join(authDir, "token", "token.go")
	tokenCode := `package token

type TokenClaims struct {
	Subject string
}
`
	_ = os.WriteFile(tokenFile, []byte(tokenCode), 0600)

	gatewayDir := filepath.Join(tempDir, "services", "gateway")
	_ = os.MkdirAll(filepath.Join(gatewayDir, "middleware"), 0750)
	_ = os.WriteFile(filepath.Join(gatewayDir, "go.mod"), []byte("module monorepo.com/gateway\n\ngo 1.22.0\n"), 0600)

	gwFile := filepath.Join(gatewayDir, "middleware", "auth.go")
	gwCode := `package middleware

import "monorepo.com/auth/token"

func Authenticate(t token.TokenClaims) string {
	return t.Subject
}
`
	_ = os.WriteFile(gwFile, []byte(gwCode), 0600)

	// 1. Verify FindWorkspace discovers both modules starting from monorepo root
	ws, err := refactor.FindWorkspace(tempDir)
	if err != nil {
		t.Fatalf("FindWorkspace failed for monorepo: %v", err)
	}

	if len(ws.Modules) != 2 {
		t.Fatalf("expected 2 discovered modules in monorepo, got %d", len(ws.Modules))
	}

	// 2. Move directory services/auth/token to services/auth/jwt_tokens
	destDir := filepath.Join(authDir, "jwt_tokens")
	opts := refactor.MoveDirOptions{
		SourceDir: filepath.Join(authDir, "token"),
		DestDir:   destDir,
	}

	if err := refactor.MoveDirectory(opts); err != nil {
		t.Fatalf("MoveDirectory failed in monorepo: %v", err)
	}

	// 3. Verify gateway middleware was updated across modules
	gwBytes, err := os.ReadFile(gwFile) //nolint:gosec
	if err != nil {
		t.Fatalf("failed reading gateway file: %v", err)
	}
	gwStr := string(gwBytes)

	if !strings.Contains(gwStr, `"monorepo.com/auth/jwt_tokens"`) {
		t.Errorf("gateway middleware missing updated import 'monorepo.com/auth/jwt_tokens':\n%s", gwStr)
	}
}
