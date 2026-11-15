package git

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// TestCheckpoint_NonGitSnapshotRestoreRoundTrip covers the file-level
// fallback in a plain (non-git) directory: snapshot copies workspace files,
// restore copies them back after later mutations.
func TestCheckpoint_NonGitSnapshotRestoreRoundTrip(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	write := func(name, content string) {
		t.Helper()
		full := filepath.Join(dir, name)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
	}
	read := func(name string) string {
		t.Helper()
		data, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil {
			t.Fatalf("read %s: %v", name, err)
		}
		return string(data)
	}

	write("notes/meeting 1.md", "original notes\n")
	write("src/nested file.py", "print('v1')\n")
	write("node_modules/pkg/index.js", "junk\n")
	write(".git/HEAD", "ref: fake\n") // must be excluded from the snapshot

	rec, err := CreateCheckpoint(ctx, dir, "sess-rt", "turn-rt", "before edits")
	if err != nil {
		t.Fatalf("CreateCheckpoint: %v", err)
	}
	if rec.CommitHash != "local_snapshot" {
		t.Fatalf("expected local_snapshot commit hash, got %q", rec.CommitHash)
	}
	if len(rec.Files) == 0 {
		t.Fatal("expected snapshot to record files")
	}

	snapDir := filepath.Join(dir, ".forge", "snapshots", "sess-rt", "turn-rt")
	for _, f := range []string{"notes/meeting 1.md", "src/nested file.py"} {
		if _, err := os.Stat(filepath.Join(snapDir, filepath.FromSlash(f))); err != nil {
			t.Errorf("snapshot missing %s: %v", f, err)
		}
	}
	if _, err := os.Stat(filepath.Join(snapDir, "node_modules")); !os.IsNotExist(err) {
		t.Error("snapshot must skip node_modules")
	}
	if _, err := os.Stat(filepath.Join(snapDir, ".git")); !os.IsNotExist(err) {
		t.Error("snapshot must skip .git")
	}

	// Mutate and add files after the checkpoint.
	write("notes/meeting 1.md", "rewritten notes\n")
	write("src/nested file.py", "print('v2')\n")
	write("post_checkpoint.txt", "created later\n")

	if err := RestoreCheckpoint(ctx, dir, "sess-rt", "turn-rt"); err != nil {
		t.Fatalf("RestoreCheckpoint: %v", err)
	}

	if got := read("notes/meeting 1.md"); got != "original notes\n" {
		t.Errorf("restore did not revert notes/meeting 1.md, got %q", got)
	}
	if got := read("src/nested file.py"); got != "print('v1')\n" {
		t.Errorf("restore did not revert src/nested file.py, got %q", got)
	}
}

// TestCheckpoint_IndexPrunedToMostRecent verifies checkpoints.json is pruned
// to the most recent maxCheckpoints entries after each snapshot.
func TestCheckpoint_IndexPrunedToMostRecent(t *testing.T) {
	dir := t.TempDir()
	ctx := context.Background()

	for i := 0; i < maxCheckpoints+10; i++ {
		turn := "turn"
		desc := "checkpoint"
		if _, err := CreateCheckpoint(ctx, dir, "s", turn, desc); err != nil {
			t.Fatalf("CreateCheckpoint %d: %v", i, err)
		}
	}

	records, err := ListCheckpoints(dir, "")
	if err != nil {
		t.Fatalf("ListCheckpoints: %v", err)
	}
	if len(records) > maxCheckpoints {
		t.Fatalf("expected index pruned to %d entries, got %d", maxCheckpoints, len(records))
	}
}
