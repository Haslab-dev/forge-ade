package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func setupHunkRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	run("config", "user.email", "t@t")
	run("config", "user.name", "T")
	os.WriteFile(filepath.Join(dir, "f.txt"), []byte("line1\nline2\nline3\nline4\nline5\n"), 0644)
	run("add", ".")
	run("commit", "-q", "-m", "init")
	return dir
}

func TestGetFileDiffHunks(t *testing.T) {
	dir := setupHunkRepo(t)
	e := NewEngine()
	ctx := context.Background()

	os.WriteFile(filepath.Join(dir, "f.txt"), []byte("line1\nCHANGED\nline3\nline4\nNEW\n"), 0644)

	hunks, err := e.GetFileDiffHunks(ctx, dir, "f.txt")
	if err != nil {
		t.Fatalf("GetFileDiffHunks: %v", err)
	}
	if len(hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %d", len(hunks))
	}
	h := hunks[0]
	if h.NewStart != 1 || h.NewLines != 5 {
		t.Errorf("unexpected new range: start=%d lines=%d", h.NewStart, h.NewLines)
	}
	if h.OldStart != 1 || h.OldLines != 5 {
		t.Errorf("unexpected old range: start=%d lines=%d", h.OldStart, h.OldLines)
	}
	foundAdded, foundRemoved := false, false
	for _, l := range h.Body {
		if l == "+CHANGED" {
			foundAdded = true
		}
		if l == "-line2" {
			foundRemoved = true
		}
	}
	if !foundAdded || !foundRemoved {
		t.Errorf("hunk body missing +/- lines: %v", h.Body)
	}

	clean, err := e.GetFileDiffHunks(ctx, dir, "untracked.txt")
	if err != nil || len(clean) != 0 {
		t.Fatalf("untracked file should yield no hunks, got %v err=%v", clean, err)
	}
}

func TestRevertDiffHunk(t *testing.T) {
	dir := setupHunkRepo(t)
	e := NewEngine()
	ctx := context.Background()

	os.WriteFile(filepath.Join(dir, "f.txt"), []byte("line1\nline2\nCHANGED\nline4\nNEW LINE\n"), 0644)

	hunks, err := e.GetFileDiffHunks(ctx, dir, "f.txt")
	if err != nil || len(hunks) != 1 {
		t.Fatalf("expected 1 hunk, got %v err=%v", hunks, err)
	}

	if err := e.RevertDiffHunk(ctx, dir, "f.txt", 0); err != nil {
		t.Fatalf("RevertDiffHunk: %v", err)
	}

	b, _ := os.ReadFile(filepath.Join(dir, "f.txt"))
	if string(b) != "line1\nline2\nline3\nline4\nline5\n" {
		t.Fatalf("file not restored to HEAD: %q", b)
	}

	if err := e.RevertDiffHunk(ctx, dir, "f.txt", 5); err == nil {
		t.Fatalf("expected out-of-range hunk index error")
	}
}

func TestStatusByPath(t *testing.T) {
	dir := setupHunkRepo(t)
	e := NewEngine()
	ctx := context.Background()

	os.WriteFile(filepath.Join(dir, "f.txt"), []byte("modified\n"), 0644)
	os.WriteFile(filepath.Join(dir, "new.txt"), []byte("new\n"), 0644)

	m, err := e.StatusByPath(ctx, dir)
	if err != nil {
		t.Fatalf("StatusByPath: %v", err)
	}
	if m["f.txt"] != "M" {
		t.Errorf("f.txt should be M, got %q", m["f.txt"])
	}
	if m["new.txt"] != "U" {
		t.Errorf("new.txt should be U, got %q", m["new.txt"])
	}

	if err := os.Remove(filepath.Join(dir, "f.txt")); err != nil {
		t.Fatal(err)
	}
	m2, _ := e.StatusByPath(ctx, dir)
	if m2["f.txt"] != "D" {
		t.Errorf("deleted f.txt should be D, got %q", m2["f.txt"])
	}
}

func TestFindRepoRoot(t *testing.T) {
	dir := setupHunkRepo(t)
	sub := filepath.Join(dir, "a", "b")
	if err := os.MkdirAll(sub, 0755); err != nil {
		t.Fatal(err)
	}
	root, ok := FindRepoRoot(sub)
	if !ok || root != dir {
		t.Fatalf("expected root %s, got %s ok=%v", dir, root, ok)
	}
	tmp := t.TempDir()
	if _, ok := FindRepoRoot(tmp); ok {
		t.Fatalf("expected no repo root for %s", tmp)
	}
}
