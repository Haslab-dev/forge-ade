package fuzzy

import "testing"

func TestFuzzyCamelCaseSeparatorAndBasename(t *testing.T) {
	if m := ScoreText("   ", "anything"); m == nil || m.Score != 0 {
		t.Fatalf("expected neutral non-nil match for empty query, got %+v", m)
	}
	if m := ScoreText("xyz", "ScoreFilePath"); m != nil {
		t.Fatalf("expected nil for non-matching query, got %+v", m)
	}

	// CamelCase boundary bonus: "sfp" matches S-F-P boundaries cleanly vs scattered lowercase letters.
	camel := ScoreText("sfp", "ScoreFilePath")
	scattered := ScoreText("sfp", "safep")
	if camel == nil || scattered == nil {
		t.Fatalf("expected both camel (%v) and scattered (%v) to match", camel, scattered)
	}
	if camel.Score <= scattered.Score {
		t.Errorf("expected camelCase match Score (%v) > non-boundary match (%v)", camel.Score, scattered.Score)
	}

	// Separator boundary bonus: "fb" at start/'_' beats mid-word occurrence.
	sep := ScoreText("fb", "foo_bar")
	plain := ScoreText("fb", "fxobary")
	if sep == nil || plain == nil {
		t.Fatalf("expected separator (%v) and plain (%v) to match", sep, plain)
	}
	if sep.Score <= plain.Score {
		t.Errorf("expected separator match Score (%v) > mid-word match (%v)", sep.Score, plain.Score)
	}

	// Basename priority in ScoreFilePath: "app.go" matches basename directly.
	base := ScoreFilePath("app.go", "app/native/pkg/ui/app.go")
	nested := ScoreFilePath("app.go", "app/other_pkg/v1.0/go.txt")
	if base == nil || nested == nil {
		t.Fatalf("expected both file paths to match (base=%v nested=%v)", base, nested)
	}
	if base.Score <= nested.Score {
		t.Errorf("expected basename match Score (%v) > full-path span Score (%v)", base.Score, nested.Score)
	}

	if m := ScoreFilePath("nonexistent", "app/native/pkg/ui/app.go"); m != nil {
		t.Errorf("expected nil for non-matching file path, got %+v", m)
	}
}
