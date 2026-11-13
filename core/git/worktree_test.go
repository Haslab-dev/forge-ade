package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestWorktreeOperations(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "git-worktree-test-*")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	repoDir := filepath.Join(tmpDir, "repo")
	if err := os.MkdirAll(repoDir, 0755); err != nil {
		t.Fatalf("MkdirAll failed: %v", err)
	}

	// Init git repo
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = repoDir
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=Test",
			"GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=Test",
			"GIT_COMMITTER_EMAIL=test@example.com",
		)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v failed: %v\nOutput: %s", args, err, out)
		}
	}

	run("init")
	run("config", "user.name", "Test")
	run("config", "user.email", "test@example.com")

	// Commit initial file so HEAD exists
	readme := filepath.Join(repoDir, "README.md")
	if err := os.WriteFile(readme, []byte("# Test Repo\n"), 0644); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}
	run("add", "README.md")
	run("commit", "-m", "initial commit")

	ctx := context.Background()

	// 1. List worktrees initially
	trees, err := ListWorktrees(ctx, repoDir)
	if err != nil {
		t.Fatalf("ListWorktrees failed: %v", err)
	}
	if len(trees) != 1 {
		t.Fatalf("expected 1 main worktree, got %d", len(trees))
	}

	// 2. Create linked worktree
	subDir := filepath.Join(tmpDir, "subagent-wt")
	err = CreateWorktree(ctx, repoDir, subDir, "subagent-feature", true)
	if err != nil {
		t.Fatalf("CreateWorktree failed: %v", err)
	}

	// Verify worktrees list now has 2
	trees, err = ListWorktrees(ctx, repoDir)
	if err != nil {
		t.Fatalf("ListWorktrees failed: %v", err)
	}
	if len(trees) != 2 {
		t.Fatalf("expected 2 worktrees, got %d", len(trees))
	}

	foundSub := false
	for _, tree := range trees {
		if tree.Branch == "subagent-feature" {
			foundSub = true
			break
		}
	}
	if !foundSub {
		t.Errorf("expected to find branch 'subagent-feature' in worktree list")
	}

	// 3. Remove linked worktree
	err = RemoveWorktree(ctx, repoDir, subDir, true)
	if err != nil {
		t.Fatalf("RemoveWorktree failed: %v", err)
	}

	// Verify worktrees list is back to 1
	trees, err = ListWorktrees(ctx, repoDir)
	if err != nil {
		t.Fatalf("ListWorktrees failed: %v", err)
	}
	if len(trees) != 1 {
		t.Fatalf("expected 1 worktree after removal, got %d", len(trees))
	}
}
