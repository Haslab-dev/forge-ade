package git

import (
	"context"
	"os"
	"testing"
)

func TestGitCommitGraph(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get cwd: %v", err)
	}

	engine := NewEngine()
	res, err := engine.GetCommitGraph(context.Background(), cwd, 0, 10, "")
	if err != nil {
		t.Fatalf("unexpected error fetching commit graph: %v", err)
	}

	if res == nil {
		t.Fatalf("expected non-nil result")
	}

	if res.Limit != 10 {
		t.Errorf("expected limit 10, got %d", res.Limit)
	}
}
