package tools

import (
	"context"
	"testing"
	"time"
)

func TestBackgroundTaskLifecycle(t *testing.T) {
	task, err := StartBackgroundCommand("", "echo hello-bg && sleep 0.2", t.TempDir())
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if task.Status != TaskRunning {
		t.Fatalf("expected running, got %s", task.Status)
	}

	final, ok := WaitTask(context.Background(), task.ID, 5*time.Second)
	if !ok || final.Status != TaskCompleted {
		t.Fatalf("expected completed, got %s (ok=%v)", final.Status, ok)
	}
	if want := "hello-bg"; !contains(final.OutputTail, want) {
		t.Fatalf("tail %q missing %q", final.OutputTail, want)
	}
	if final.ExitCode != 0 {
		t.Fatalf("exit code %d", final.ExitCode)
	}

	// Notification drains exactly once.
	if notes := DrainTaskNotifications(task.SessionID); len(notes) != 0 {
		t.Fatalf("notifications for other session leaked: %+v", notes)
	}
	// The task had no session; drain with empty session returns nothing and
	// per-session drain after lookup-by-list should still work.
	tasks := ListTasks("")
	if len(tasks) == 0 {
		t.Fatal("task not listed")
	}
}

func TestBackgroundTaskStop(t *testing.T) {
	task, err := StartBackgroundCommand("sess-1", "sleep 30", t.TempDir())
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if !StopTask(task.ID) {
		t.Fatal("stop returned false for running task")
	}
	final, _ := WaitTask(context.Background(), task.ID, 5*time.Second)
	if final.Status != TaskStopped {
		t.Fatalf("expected stopped, got %s", final.Status)
	}
	if notes := DrainTaskNotifications("sess-1"); len(notes) != 1 {
		t.Fatalf("expected 1 notification, got %d", len(notes))
	}
	if notes := DrainTaskNotifications("sess-1"); len(notes) != 0 {
		t.Fatalf("notification not consumed: %d", len(notes))
	}
}

func TestTaskOutputTool(t *testing.T) {
	r := &Registry{tools: map[string]ToolSpec{}}
	r.Register(taskOutputTool())

	task, err := StartBackgroundCommand("s", "printf out-xyz", t.TempDir())
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	WaitTask(context.Background(), task.ID, 5*time.Second)

	res, err := r.Execute(context.Background(), "task_output", `{"task_id":"`+task.ID+`","block":true,"timeout":5}`)
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	m := res.(map[string]any)
	if m["status"] != string(TaskCompleted) {
		t.Fatalf("status %v", m["status"])
	}
	if _, ok := m["output_tail"]; !ok {
		t.Fatalf("missing output tail in %v", m)
	}
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || indexOf(s, sub) >= 0)
}

func indexOf(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
