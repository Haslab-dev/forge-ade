package board

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBoardWatcher_DetectExternalChanges(t *testing.T) {
	tempDir := t.TempDir()
	forgeDir := filepath.Join(tempDir, ".forge")
	if err := os.MkdirAll(forgeDir, 0755); err != nil {
		t.Fatalf("failed to create .forge dir: %v", err)
	}

	changeCh := make(chan []Task, 5)
	w, err := NewWatcher(WatcherOptions{
		ResolveDir: func(ws string) string {
			return filepath.Join(ws, ".forge")
		},
		OnChanged: func(tasks []Task) {
			changeCh <- tasks
		},
	})
	if err != nil {
		t.Fatalf("failed to create watcher: %v", err)
	}
	defer w.Stop()

	w.WatchWorkspace([]string{tempDir}, "")

	// Give fsnotify a brief moment to arm
	time.Sleep(50 * time.Millisecond)

	// Write external tasks file
	taskJSON := `[
		{
			"title": "External Task",
			"status": "todo",
			"priority": "high"
		}
	]`
	tasksFile := filepath.Join(forgeDir, "board_tasks.json")
	if err := os.WriteFile(tasksFile, []byte(taskJSON), 0644); err != nil {
		t.Fatalf("failed to write tasks file: %v", err)
	}

	select {
	case tasks := <-changeCh:
		if len(tasks) != 1 {
			t.Fatalf("expected 1 task, got %d", len(tasks))
		}
		if tasks[0].Title != "External Task" {
			t.Errorf("expected title 'External Task', got %q", tasks[0].Title)
		}
		if tasks[0].Key != "ADE-1" {
			t.Errorf("expected auto-assigned key ADE-1, got %q", tasks[0].Key)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for watcher callback")
	}

	// Verify UpdateLastHash ignores internal writes
	data, _ := os.ReadFile(tasksFile)
	w.UpdateLastHash(data)

	// Trigger event with same content
	_ = os.WriteFile(tasksFile, data, 0644)

	select {
	case <-changeCh:
		t.Fatal("unexpected change event for identical content matching lastHash")
	case <-time.After(300 * time.Millisecond):
		// Expected - no event fired
	}
}
