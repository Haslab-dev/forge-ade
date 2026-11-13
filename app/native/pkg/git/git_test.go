package git

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/hasdev/forge-ade/app/native/pkg/engine"
)

func TestGitManager(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "git-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Init git repo
	cmd := exec.Command("git", "init")
	cmd.Dir = tempDir
	if err := cmd.Run(); err != nil {
		t.Skip("git not available")
	}

	cmd = exec.Command("git", "config", "user.email", "test@test.com")
	cmd.Dir = tempDir
	_ = cmd.Run()
	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tempDir
	_ = cmd.Run()

	testFile := filepath.Join(tempDir, "test.txt")
	_ = os.WriteFile(testFile, []byte("hello"), 0644)

	eng := engine.NewEngine("")
	gm := NewGitManager(eng, tempDir, nil)

	if len(gm.Untracked) != 1 {
		t.Fatalf("expected 1 untracked file, got %d", len(gm.Untracked))
	}

	gm.Stage("test.txt")
	if len(gm.Staged) != 1 {
		t.Fatalf("expected 1 staged file, got %d", len(gm.Staged))
	}

	if err := gm.Commit("Initial commit"); err != nil {
		t.Fatalf("failed to commit: %v", err)
	}

	if len(gm.Staged) != 0 {
		t.Fatalf("expected 0 staged files after commit, got %d", len(gm.Staged))
	}
}
