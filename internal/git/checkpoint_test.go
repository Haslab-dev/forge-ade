package git

import (
	"context"
	"os"
	"testing"
)

func TestCheckpoint_NonGitFallback(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "checkpoint_test_*")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tempDir)

	ctx := context.Background()

	// Create checkpoint
	rec, err := CreateCheckpoint(ctx, tempDir, "sess-1", "turn-1", "before test edit")
	if err != nil {
		t.Fatalf("CreateCheckpoint failed: %v", err)
	}
	if rec == nil || rec.ID == "" {
		t.Fatalf("expected valid checkpoint record, got %+v", rec)
	}

	// List checkpoints
	list, err := ListCheckpoints(tempDir, "sess-1")
	if err != nil {
		t.Fatalf("ListCheckpoints failed: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("expected 1 checkpoint, got %d", len(list))
	}
	if list[0].TurnID != "turn-1" {
		t.Errorf("expected turn-1, got %s", list[0].TurnID)
	}
}
