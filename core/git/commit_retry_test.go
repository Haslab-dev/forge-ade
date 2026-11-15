package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// gitTestRun runs git in dir with test identity, failing the test on error.
func gitTestRun(t *testing.T, dir string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@t", "GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@t",
		"GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v: %s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

// TestCommitRetryDoesNotSweepUntracked proves the retry stages ONLY the
// previously staged paths: an untracked file present during a failed first
// attempt must never be committed (the old `git add -A` behavior).
func TestCommitRetryDoesNotSweepUntracked(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := newRealRepo(t)
	ctx := context.Background()

	// Pre-commit hook that fails the first invocation, passes afterwards.
	hook := filepath.Join(dir, ".git", "hooks", "pre-commit")
	hookBody := "#!/bin/sh\nif [ ! -f .git/hook-failed ]; then\n  touch .git/hook-failed\n  exit 1\nfi\n"
	if err := os.WriteFile(hook, []byte(hookBody), 0o755); err != nil {
		t.Fatal(err)
	}

	// One staged file; one untracked file that must stay out of the commit.
	if err := os.WriteFile(filepath.Join(dir, "staged.txt"), []byte("staged\n"), 0644); err != nil {
		t.Fatal(err)
	}
	gitTestRun(t, dir, "add", "--", "staged.txt")
	if err := os.WriteFile(filepath.Join(dir, "untracked.txt"), []byte("untracked\n"), 0644); err != nil {
		t.Fatal(err)
	}

	e := NewEngine()
	if err := e.Commit(ctx, dir, "retry: staged only"); err != nil {
		t.Fatalf("Commit with retry: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, ".git", "hook-failed")); err != nil {
		t.Fatal("precondition: first commit attempt did not fail via hook")
	}

	names := gitTestRun(t, dir, "log", "-1", "--name-only", "--format=")
	if !strings.Contains(names, "staged.txt") {
		t.Fatalf("expected staged.txt in commit, got files %q", names)
	}
	if strings.Contains(names, "untracked.txt") {
		t.Fatalf("untracked.txt was swept into the commit (add -A regression): %q", names)
	}
}

// TestCommitWithoutStagedReturnsError pins the other half: with nothing
// staged the commit fails honestly instead of staging the whole tree.
func TestCommitWithoutStagedReturnsError(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := newRealRepo(t)
	ctx := context.Background()

	// Worktree-only modification plus an untracked file; nothing staged.
	if err := os.WriteFile(filepath.Join(dir, "README.md"), []byte("# changed\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "loose.txt"), []byte("loose\n"), 0644); err != nil {
		t.Fatal(err)
	}

	e := NewEngine()
	if err := e.Commit(ctx, dir, "should not auto-stage"); err == nil {
		t.Fatal("Commit with nothing staged must return an error (never add -A)")
	}

	status := gitTestRun(t, dir, "status", "--porcelain")
	if !strings.Contains(status, "M README.md") {
		t.Fatalf("worktree modification vanished — something staged/committed it: %q", status)
	}
	if strings.Contains(status, "A ") {
		t.Fatalf("files were staged by the failed commit path: %q", status)
	}
}
