package ui

import (
	"testing"
)

func TestTaskManager(t *testing.T) {
	tm := NewTaskManager()

	completed, total := tm.Progress()
	if total != 6 || completed != 4 {
		t.Fatalf("expected 4/6 completed initially, got %d/%d", completed, total)
	}

	// Toggle task 1 (was completed -> pending)
	tm.ToggleTask("1")
	completed, _ = tm.Progress()
	if completed != 3 {
		t.Fatalf("expected 3 completed after toggle, got %d", completed)
	}

	// Toggle back
	tm.ToggleTask("1")
	completed, _ = tm.Progress()
	if completed != 4 {
		t.Fatalf("expected 4 completed, got %d", completed)
	}

	// SetGoal
	tm.SetGoal("New Objective", []TaskItem{
		{ID: "a", Title: "Subtask A", Status: TaskPending},
		{ID: "b", Title: "Subtask B", Status: TaskCompleted},
	})

	completed, total = tm.Progress()
	if total != 2 || completed != 1 {
		t.Fatalf("expected 1/2 completed for new goal, got %d/%d", completed, total)
	}

	// Clear
	tm.Clear()
	_, total = tm.Progress()
	if total != 0 {
		t.Fatalf("expected 0 tasks after Clear, got %d", total)
	}
}
