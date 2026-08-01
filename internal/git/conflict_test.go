package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func setupConflictRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(name string, args ...string) string {
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("%s %v: %v\n%s", name, args, err, out)
		}
		return string(out)
	}
	runAllow := func(name string, args ...string) string {
		cmd := exec.Command(name, args...)
		cmd.Dir = dir
		out, _ := cmd.CombinedOutput()
		return string(out)
	}
	run("git", "init", "-q")
	run("git", "config", "user.email", "test@example.com")
	run("git", "config", "user.name", "Test")
	run("git", "config", "commit.gpgsign", "false")

	// base commit on main
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("line1\nline2\nline3\n"), 0644)
	run("git", "add", ".")
	run("git", "commit", "-q", "-m", "base")

	// feature branch changes the file
	run("git", "checkout", "-q", "-b", "feature")
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("line1\nFEATURE\nline3\n"), 0644)
	run("git", "add", ".")
	run("git", "commit", "-q", "-m", "feature change")

	// main changes the file differently → conflict on merge
	run("git", "checkout", "-q", "main")
	os.WriteFile(filepath.Join(dir, "file.txt"), []byte("line1\nMAIN\nline3\n"), 0644)
	run("git", "add", ".")
	run("git", "commit", "-q", "-m", "main change")

	runAllow("git", "merge", "feature")
	return dir
}

func TestConflictStageContentAndResolve(t *testing.T) {
	dir := setupConflictRepo(t)
	e := NewEngine()
	ctx := context.Background()

	// Status should report the conflict
	st, err := e.GetStatus(ctx, dir)
	if err != nil {
		t.Fatalf("GetStatus: %v", err)
	}
	if len(st.Conflicts) != 1 || st.Conflicts[0].Path != "file.txt" {
		t.Fatalf("expected 1 conflict for file.txt, got %+v", st.Conflicts)
	}
	if st.Conflicts[0].Status != "UU" {
		t.Fatalf("expected UU status, got %s", st.Conflicts[0].Status)
	}

	// Stage contents
	base, err := e.GetConflictStageContent(ctx, dir, "file.txt", 1)
	if err != nil {
		t.Fatalf("stage 1: %v", err)
	}
	ours, err := e.GetConflictStageContent(ctx, dir, "file.txt", 2)
	if err != nil {
		t.Fatalf("stage 2: %v", err)
	}
	theirs, err := e.GetConflictStageContent(ctx, dir, "file.txt", 3)
	if err != nil {
		t.Fatalf("stage 3: %v", err)
	}
	if !strings.Contains(base, "line2") || strings.Contains(base, "MAIN") || strings.Contains(base, "FEATURE") {
		t.Fatalf("ancestor stage unexpected: %q", base)
	}
	if !strings.Contains(ours, "MAIN") {
		t.Fatalf("ours stage unexpected: %q", ours)
	}
	if !strings.Contains(theirs, "FEATURE") {
		t.Fatalf("theirs stage unexpected: %q", theirs)
	}

	// Invalid stage
	if _, err := e.GetConflictStageContent(ctx, dir, "file.txt", 9); err == nil {
		t.Fatal("expected error for invalid stage")
	}

	// Resolve with "ours"
	if err := e.ResolveConflict(ctx, dir, "file.txt", "ours"); err != nil {
		t.Fatalf("ResolveConflict ours: %v", err)
	}
	content, _ := os.ReadFile(filepath.Join(dir, "file.txt"))
	if !strings.Contains(string(content), "MAIN") {
		t.Fatalf("after ours resolve, file should contain MAIN: %q", content)
	}
	st, _ = e.GetStatus(ctx, dir)
	if len(st.Conflicts) != 0 {
		t.Fatalf("expected no conflicts after resolve, got %+v", st.Conflicts)
	}
	// Accepting "ours" yields the HEAD content, so the file is clean (not staged).
}

func TestConflictResolveTheirsAndMark(t *testing.T) {
	dir := setupConflictRepo(t)
	e := NewEngine()
	ctx := context.Background()

	// theirs
	if err := e.ResolveConflict(ctx, dir, "file.txt", "theirs"); err != nil {
		t.Fatalf("ResolveConflict theirs: %v", err)
	}
	content, _ := os.ReadFile(filepath.Join(dir, "file.txt"))
	if !strings.Contains(string(content), "FEATURE") {
		t.Fatalf("after theirs resolve, file should contain FEATURE: %q", content)
	}

	// Recreate a conflict in a fresh repo for the "mark" flow.
	dir2 := setupConflictRepo(t)
	e2 := NewEngine()
	st, _ := e2.GetStatus(ctx, dir2)
	if len(st.Conflicts) != 1 {
		t.Fatalf("expected 1 conflict before mark, got %+v", st.Conflicts)
	}

	if err := e2.ResolveConflict(ctx, dir2, "file.txt", "mark"); err != nil {
		t.Fatalf("ResolveConflict mark: %v", err)
	}
	content, _ = os.ReadFile(filepath.Join(dir2, "file.txt"))
	if !strings.Contains(string(content), "<<<<<<<") {
		t.Fatalf("after mark resolve file keeps the (unedited) conflicted content: %q", content)
	}
	st, _ = e2.GetStatus(ctx, dir2)
	if len(st.Conflicts) != 0 {
		t.Fatalf("expected no conflicts after mark, got %+v", st.Conflicts)
	}
	if len(st.Staged) != 1 || st.Staged[0].Status != "M" {
		t.Fatalf("expected file staged as M after mark, got staged=%+v", st.Staged)
	}

	// unknown action
	if err := e2.ResolveConflict(ctx, dir2, "file.txt", "bogus"); err == nil {
		t.Fatal("expected error for unknown action")
	}
}
