package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/joangrigorov/go-refactor-mcp/internal/refactor"
)

func TestMoveFileAliasedImportDeduplication(t *testing.T) {
	tempDir := t.TempDir()

	_ = os.WriteFile(filepath.Join(tempDir, "go.mod"), []byte("module example.com/aliasmod\n\ngo 1.22.0\n"), 0600)

	folDir := filepath.Join(tempDir, "pkg", "followed_pkg")
	_ = os.MkdirAll(folDir, 0750)
	folFile := filepath.Join(folDir, "club_followed.go")
	_ = os.WriteFile(folFile, []byte("package followed_pkg\n\ntype ClubFollowed struct{}\n"), 0600)

	unfolDir := filepath.Join(tempDir, "pkg", "unfollowed_pkg")
	_ = os.MkdirAll(unfolDir, 0750)
	unfolFile := filepath.Join(unfolDir, "club_unfollowed.go")
	_ = os.WriteFile(unfolFile, []byte("package unfollowed_pkg\n\ntype ClubUnfollowed struct{}\n"), 0600)

	applyDir := filepath.Join(tempDir, "pkg", "apply")
	_ = os.MkdirAll(applyDir, 0750)
	applyFile := filepath.Join(applyDir, "apply.go")
	applyCode := `package apply

import (
	"example.com/aliasmod/pkg/followed_pkg"
	"example.com/aliasmod/pkg/unfollowed_pkg"
)

func Process(f followed_pkg.ClubFollowed, u unfollowed_pkg.ClubUnfollowed) {}
`
	_ = os.WriteFile(applyFile, []byte(applyCode), 0600)

	destDir := filepath.Join(tempDir, "pkg", "following_events")

	// Move 5: Move club_followed.go to following_events
	opts5 := refactor.MoveFileOptions{
		Dir:        tempDir,
		SourceFile: folFile,
		DestDir:    destDir,
	}
	if err := refactor.MoveFile(opts5); err != nil {
		t.Fatalf("MoveFile 5 failed: %v", err)
	}

	// Move 6: Move club_unfollowed.go to following_events
	opts6 := refactor.MoveFileOptions{
		Dir:        tempDir,
		SourceFile: unfolFile,
		DestDir:    destDir,
	}
	if err := refactor.MoveFile(opts6); err != nil {
		t.Fatalf("MoveFile 6 failed: %v", err)
	}

	// Read apply.go
	res, err := os.ReadFile(applyFile) //nolint:gosec
	if err != nil {
		t.Fatalf("failed reading apply.go: %v", err)
	}

	str := string(res)
	targetImport := `"example.com/aliasmod/pkg/following_events"`

	count := strings.Count(str, targetImport)
	if count != 1 {
		t.Errorf("expected import %s to appear exactly once, but appeared %d times in:\n%s", targetImport, count, str)
	}

	if strings.Contains(str, `"example.com/aliasmod/pkg/followed_pkg"`) || strings.Contains(str, `"example.com/aliasmod/pkg/unfollowed_pkg"`) {
		t.Errorf("apply.go should not contain old imports:\n%s", str)
	}
}
