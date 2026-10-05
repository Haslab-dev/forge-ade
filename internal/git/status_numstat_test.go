package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestNumstatRenamePath(t *testing.T) {
	cases := []struct{ in, want string }{
		{"app.go", "app.go"},
		{"dir/{old.go => new.go}", "dir/new.go"},
		{"{a => b}.go", "b.go"},
		{"old.go => new.go", "new.go"},
		{"dir/{a/b => c/d}.go", "dir/c/d.go"},
	}
	for _, c := range cases {
		if got := numstatRenamePath(c.in); got != c.want {
			t.Errorf("numstatRenamePath(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestCountUntrackedLines(t *testing.T) {
	repo := t.TempDir()
	if err := os.WriteFile(filepath.Join(repo, "new.txt"), []byte("a\nb\nc"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := countUntrackedLines(repo, "new.txt"); got != 3 {
		t.Errorf("countUntrackedLines = %d, want 3", got)
	}
	if got := countUntrackedLines(repo, "missing.txt"); got != 0 {
		t.Errorf("missing file should report 0, got %d", got)
	}
}

func TestAnnotateNumstatCountsModifiedFile(t *testing.T) {
	repo := t.TempDir()
	runGitCmd := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", args...)
		cmd.Dir = repo
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
	}
	runGitCmd("init", "-q")
	runGitCmd("config", "user.email", "test@test")
	runGitCmd("config", "user.name", "test")
	if err := os.WriteFile(filepath.Join(repo, "tracked.txt"), []byte("one\ntwo\nthree\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	runGitCmd("add", "tracked.txt")
	runGitCmd("commit", "-m", "init")
	if err := os.WriteFile(filepath.Join(repo, "tracked.txt"), []byte("one\nTWO\nthree\nfour\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	e := NewEngine()
	res, err := e.GetStatus(context.Background(), repo)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Unstaged) != 1 {
		t.Fatalf("expected 1 unstaged file, got %d", len(res.Unstaged))
	}
	f := res.Unstaged[0]
	if f.Additions != 2 || f.Deletions != 1 {
		t.Errorf("tracked.txt counts = +%d/-%d, want +2/-1", f.Additions, f.Deletions)
	}
}
