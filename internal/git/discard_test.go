package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func setupDiscardRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(name string, args ...string) {
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%s %v: %v\n%s", name, args, err, out)
		}
	}
	run("git", "init", "-q")
	run("git", "config", "user.email", "t@t")
	run("git", "config", "user.name", "T")
	os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("v1\n"), 0644)
	os.MkdirAll(filepath.Join(dir, "sub"), 0755)
	os.WriteFile(filepath.Join(dir, "sub", "tracked2.txt"), []byte("t2\n"), 0644)
	run("git", "add", ".")
	run("git", "commit", "-q", "-m", "init")
	return dir
}

func TestDiscardMixed(t *testing.T) {
	dir := setupDiscardRepo(t)
	e := NewEngine()
	ctx := context.Background()

	// Modify a tracked file (worktree change)
	os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("MODIFIED\n"), 0644)
	// Create an untracked file
	os.WriteFile(filepath.Join(dir, "new.txt"), []byte("new\n"), 0644)
	// Create an untracked nested file
	os.WriteFile(filepath.Join(dir, "sub", "nested.txt"), []byte("n\n"), 0644)

	if err := e.Discard(ctx, dir, []string{"tracked.txt"}); err != nil {
		t.Fatalf("discard tracked: %v", err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "tracked.txt")); string(b) != "v1\n" {
		t.Fatalf("tracked.txt not restored: %q", b)
	}

	if err := e.Discard(ctx, dir, []string{"new.txt"}); err != nil {
		t.Fatalf("discard untracked: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "new.txt")); !os.IsNotExist(err) {
		t.Fatalf("new.txt should be removed, stat err=%v", err)
	}

	if err := e.Discard(ctx, dir, []string{"sub/nested.txt"}); err != nil {
		t.Fatalf("discard nested untracked: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "sub", "nested.txt")); !os.IsNotExist(err) {
		t.Fatalf("nested.txt should be removed, stat err=%v", err)
	}

	// Mixed batch: one tracked modified + one untracked in a single call.
	os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("MODIFIED2\n"), 0644)
	os.WriteFile(filepath.Join(dir, "batch.txt"), []byte("b\n"), 0644)
	if err := e.Discard(ctx, dir, []string{"tracked.txt", "batch.txt"}); err != nil {
		t.Fatalf("discard batch: %v", err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "tracked.txt")); string(b) != "v1\n" {
		t.Fatalf("batch: tracked.txt not restored: %q", b)
	}
	if _, err := os.Stat(filepath.Join(dir, "batch.txt")); !os.IsNotExist(err) {
		t.Fatalf("batch: batch.txt should be removed")
	}

	// Status should now be clean.
	st, err := e.GetStatus(ctx, dir)
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if len(st.Staged) != 0 || len(st.Unstaged) != 0 || len(st.Untracked) != 0 {
		t.Fatalf("expected clean status, got staged=%v unstaged=%v untracked=%v", st.Staged, st.Unstaged, st.Untracked)
	}
}

func TestDiscardStagedFile(t *testing.T) {
	dir := setupDiscardRepo(t)
	e := NewEngine()
	ctx := context.Background()

	// Stage a modification to tracked.txt
	os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("STAGED\n"), 0644)
	if err := e.Stage(ctx, dir, []string{"tracked.txt"}); err != nil {
		t.Fatalf("stage: %v", err)
	}
	// Then add a further worktree modification (file is now MM)
	os.WriteFile(filepath.Join(dir, "tracked.txt"), []byte("WORKTREE\n"), 0644)

	// Discard fully reverts both the staged index and the worktree back to HEAD
	// (VS Code "Discard Changes" semantics), so the file becomes clean.
	if err := e.Discard(ctx, dir, []string{"tracked.txt"}); err != nil {
		t.Fatalf("discard staged-file: %v", err)
	}
	if b, _ := os.ReadFile(filepath.Join(dir, "tracked.txt")); string(b) != "v1\n" {
		t.Fatalf("expected full revert to HEAD, got %q", b)
	}
	st, _ := e.GetStatus(ctx, dir)
	if len(st.Staged) != 0 || len(st.Unstaged) != 0 {
		t.Fatalf("expected clean status, got staged=%v unstaged=%v", st.Staged, st.Unstaged)
	}

	// A staged-new file (in index, not in HEAD) is also removed entirely.
	os.WriteFile(filepath.Join(dir, "brandnew.txt"), []byte("x\n"), 0644)
	if err := e.Stage(ctx, dir, []string{"brandnew.txt"}); err != nil {
		t.Fatalf("stage brandnew: %v", err)
	}
	if err := e.Discard(ctx, dir, []string{"brandnew.txt"}); err != nil {
		t.Fatalf("discard staged-new: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "brandnew.txt")); !os.IsNotExist(err) {
		t.Fatalf("staged-new file should be removed after discard, stat err=%v", err)
	}
	st, _ = e.GetStatus(ctx, dir)
	if len(st.Staged) != 0 {
		t.Fatalf("expected staged list empty, got %v", st.Staged)
	}
}

func TestDiscardNoChangesReturnsNoError(t *testing.T) {
	dir := setupDiscardRepo(t)
	e := NewEngine()
	ctx := context.Background()

	// Discarding a clean tracked file: restore succeeds (no-op), no error.
	if err := e.Discard(ctx, dir, []string{"tracked.txt"}); err != nil {
		t.Fatalf("discard clean tracked file should not error: %v", err)
	}
}
