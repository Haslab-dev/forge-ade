package ui

import (
	"testing"
)

func TestTaskManager(t *testing.T) {
	tm := NewTaskManager()

	completed, total := tm.Progress()
	if total != 0 || completed != 0 {
		t.Fatalf("expected 0/0 completed initially, got %d/%d", completed, total)
	}

	tm.SetGoal("Initial Goal", []TaskItem{
		{ID: "1", Title: "Task 1", Status: TaskCompleted},
		{ID: "2", Title: "Task 2", Status: TaskCompleted},
		{ID: "3", Title: "Task 3", Status: TaskPending},
	})

	completed, total = tm.Progress()
	if total != 3 || completed != 2 {
		t.Fatalf("expected 2/3 completed, got %d/%d", completed, total)
	}

	// Toggle task 1 (was completed -> pending)
	tm.ToggleTask("1")
	completed, _ = tm.Progress()
	if completed != 1 {
		t.Fatalf("expected 1 completed after toggle, got %d", completed)
	}

	// Toggle back
	tm.ToggleTask("1")
	completed, _ = tm.Progress()
	if completed != 2 {
		t.Fatalf("expected 2 completed, got %d", completed)
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
