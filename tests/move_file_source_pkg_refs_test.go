package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joangrigorov/go-refactor-mcp/internal/refactor"
)

func TestMoveFileSourcePackageUnprefixedReferences(t *testing.T) {
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module example.com/sourcepkgmod\n\ngo 1.22.0\n"), 0600)

	authDir := filepath.Join(tempDir, "pkg", "auth")
	_ = os.MkdirAll(authDir, 0750)

	errorsFile := filepath.Join(authDir, "errors.go")
	errorsCode := `package auth

import "errors"

var ErrInvalidToken = errors.New("invalid token")
var ErrExpiredToken = errors.New("expired token")
`
	_ = os.WriteFile(errorsFile, []byte(errorsCode), 0600)

	authFile := filepath.Join(authDir, "auth.go")
	authCode := `package auth

func Login() error {
	return ErrInvalidToken
}

func Validate() error {
	return ErrExpiredToken
}
`
	_ = os.WriteFile(authFile, []byte(authCode), 0600)

	// Move errors.go from pkg/auth to pkg/autherrs
	autherrsDir := filepath.Join(tempDir, "pkg", "autherrs")
	opts := refactor.MoveFileOptions{
		SourceFile: errorsFile,
		DestDir:    autherrsDir,
	}

	if err := refactor.MoveFile(opts); err != nil {
		t.Fatalf("MoveFile failed: %v", err)
	}

	// Verify auth.go now imports autherrs and references autherrs.ErrInvalidToken / autherrs.ErrExpiredToken
	authRes, err := os.ReadFile(authFile) //nolint:gosec
	if err != nil {
		t.Fatalf("failed reading auth.go: %v", err)
	}

	authStr := string(authRes)

	if !strings.Contains(authStr, `"example.com/sourcepkgmod/pkg/autherrs"`) {
		t.Errorf("auth.go missing import for autherrs package:\n%s", authStr)
	}

	if !strings.Contains(authStr, "autherrs.ErrInvalidToken") {
		t.Errorf("auth.go missing 'autherrs.ErrInvalidToken' reference:\n%s", authStr)
	}

	if !strings.Contains(authStr, "autherrs.ErrExpiredToken") {
		t.Errorf("auth.go missing 'autherrs.ErrExpiredToken' reference:\n%s", authStr)
	}
}
