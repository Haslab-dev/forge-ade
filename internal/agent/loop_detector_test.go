package agent

import (
	"testing"
)

func TestLoopDetectorNoLoop(t *testing.T) {
	detector := NewLoopDetector(10, 4)

	// Execute different tools
	tools := []string{"read", "edit", "bash", "grep", "search"}
	for _, tool := range tools {
		isLoop, count, loopTool := detector.Record(tool, "arg1", "res1")
		if isLoop {
			t.Fatalf("unexpected loop detected for tool %s: count=%d", loopTool, count)
		}
	}
}

func TestLoopDetectorSameToolDifferentInputs(t *testing.T) {
	detector := NewLoopDetector(10, 4)

	// Same tool, different inputs should not trigger loop
	for i := 0; i < 8; i++ {
		isLoop, _, _ := detector.Record("read", string(rune('a'+i)), "file contents")
		if isLoop {
			t.Fatalf("unexpected loop detected on step %d with varying input", i)
		}
	}
}

func TestLoopDetectorTriggersCircuitBreaker(t *testing.T) {
	detector := NewLoopDetector(10, 3) // triggers on 4th repeat

	for i := 1; i <= 3; i++ {
		isLoop, count, _ := detector.Record("bash", "go test ./...", "FAIL: exit status 1")
		if isLoop {
			t.Fatalf("loop triggered prematurely at count %d", i)
		}
		if count != i {
			t.Fatalf("expected count %d, got %d", i, count)
		}
	}

	// 4th repeat should trigger
	isLoop, count, tool := detector.Record("bash", "go test ./...", "FAIL: exit status 1")
	if !isLoop {
		t.Fatalf("expected loop to be detected on 4th repeated execution")
	}
	if count != 4 {
		t.Fatalf("expected repeat count 4, got %d", count)
	}
	if tool != "bash" {
		t.Fatalf("expected loop tool 'bash', got '%s'", tool)
	}
}

func TestLoopDetectorSlidingWindowEviction(t *testing.T) {
	detector := NewLoopDetector(5, 3)

	// Add 3 repeats
	detector.Record("read", "foo.go", "content")
	detector.Record("read", "foo.go", "content")
	detector.Record("read", "foo.go", "content")

	// Add 5 distinct calls to evict the old ones
	for i := 0; i < 5; i++ {
		detector.Record("diff", string(rune('0'+i)), "ok")
	}

	// Now add another repeat of foo.go, should not trigger since old ones were evicted from window of 5
	isLoop, count, _ := detector.Record("read", "foo.go", "content")
	if isLoop {
		t.Fatalf("loop should not trigger after window eviction")
	}
	if count != 1 {
		t.Fatalf("expected count 1 after eviction, got %d", count)
	}
}

func TestLoopDetectorReset(t *testing.T) {
	detector := NewLoopDetector(5, 2)
	detector.Record("edit", "a", "b")
	detector.Record("edit", "a", "b")
	detector.Reset()

	isLoop, count, _ := detector.Record("edit", "a", "b")
	if isLoop || count != 1 {
		t.Fatalf("expected clean state after reset, got isLoop=%v, count=%d", isLoop, count)
	}
}
