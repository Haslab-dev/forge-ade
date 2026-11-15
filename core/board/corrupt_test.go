package board

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func corruptBoardPath(ws string) string {
	return filepath.Join(ws, ".forge", "board_tasks.json")
}

// TestSaveTask_PreservesCorruptFile verifies the corruption guard: a garbage
// board_tasks.json must be preserved as a .corrupt-* sibling (not destroyed
// by the write), and the incoming task must still land on a fresh board.
func TestSaveTask_PreservesCorruptFile(t *testing.T) {
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, ".forge"), 0755); err != nil {
		t.Fatal(err)
	}
	garbage := `{"tasks": [ {"id": "half-wri`
	if err := os.WriteFile(corruptBoardPath(ws), []byte(garbage), 0644); err != nil {
		t.Fatal(err)
	}

	mgr := NewManager(ws)
	_, err := mgr.SaveTask(ws, Task{Title: "Fresh task", Status: "todo", Priority: "high"})
	if err == nil {
		t.Fatal("expected descriptive error about the corrupt file, got nil")
	}

	// The corrupt bytes must survive under a .corrupt-<timestamp> name.
	entries, _ := os.ReadDir(filepath.Join(ws, ".forge"))
	corruptBackups := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "board_tasks.json.corrupt-") {
			corruptBackups++
			data, rerr := os.ReadFile(filepath.Join(ws, ".forge", e.Name()))
			if rerr != nil || string(data) != garbage {
				t.Errorf("corrupt backup %s missing or altered: %v", e.Name(), rerr)
			}
		}
	}
	if corruptBackups == 0 {
		t.Fatal("no board_tasks.json.corrupt-* backup created")
	}

	// The new task exists on a freshly started board.
	tasks, lerr := mgr.ListTasks(ws)
	if lerr != nil {
		t.Fatalf("ListTasks after recovery: %v", lerr)
	}
	if len(tasks) != 1 || tasks[0].Title != "Fresh task" {
		t.Fatalf("expected the incoming task on a fresh board, got %+v", tasks)
	}
}

// TestListTasks_QuarantinesCorruptFile verifies ListTasks preserves the
// corrupt bytes and reports the failure instead of silently returning empty.
func TestListTasks_QuarantinesCorruptFile(t *testing.T) {
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, ".forge"), 0755); err != nil {
		t.Fatal(err)
	}
	garbage := []byte(`[not json`)
	if err := os.WriteFile(corruptBoardPath(ws), garbage, 0644); err != nil {
		t.Fatal(err)
	}

	mgr := NewManager(ws)
	_, err := mgr.ListTasks(ws)
	if err == nil {
		t.Fatal("expected error for corrupt board file, got nil")
	}
	if _, statErr := os.Stat(corruptBoardPath(ws)); !os.IsNotExist(statErr) {
		t.Error("corrupt board_tasks.json should have been renamed away")
	}
	matches, _ := filepath.Glob(corruptBoardPath(ws) + ".corrupt-*")
	if len(matches) == 0 {
		t.Fatal("expected a .corrupt-* backup of the corrupt file")
	}
}

// TestMoveTaskStatusAndDelete_PreserveCorruptFile covers the remaining
// read-modify-write paths: both must quarantine and error without writing.
func TestMoveTaskStatusAndDelete_PreserveCorruptFile(t *testing.T) {
	ws := t.TempDir()
	if err := os.MkdirAll(filepath.Join(ws, ".forge"), 0755); err != nil {
		t.Fatal(err)
	}
	garbage := []byte(`{"corrupt": tru`)
	writeGarbage := func() {
		if err := os.WriteFile(corruptBoardPath(ws), garbage, 0644); err != nil {
			t.Fatal(err)
		}
	}

	mgr := NewManager(ws)

	writeGarbage()
	if _, err := mgr.MoveTaskStatus(ws, "ADE-1", "done"); err == nil {
		t.Error("MoveTaskStatus: expected corrupt-file error")
	}

	writeGarbage()
	if err := mgr.DeleteTask(ws, "ADE-1"); err == nil {
		t.Error("DeleteTask: expected corrupt-file error")
	}

	matches, _ := filepath.Glob(corruptBoardPath(ws) + ".corrupt-*")
	if len(matches) != 2 {
		t.Fatalf("expected 2 corrupt backups (one per operation), got %d", len(matches))
	}
}

// TestNextKeyScansExistingNumbers verifies new keys never collide with
// existing ones regardless of formatting (case, whitespace).
func TestNextKeyScansExistingNumbers(t *testing.T) {
	existing := []Task{
		{Key: "ade-4"},
		{Key: " ADE-99 "},
		{Key: "ADE- 120"},
		{Key: "unrelated"},
	}
	if got := maxADEKeyNumber(existing); got != 120 {
		t.Fatalf("maxADEKeyNumber = %d, want 120", got)
	}

	// A new task key must not reuse a number that exists in messy formats.
	normalized, _ := NormalizeTasks(append(existing, Task{Title: "new"}))
	seen := map[string]bool{}
	for _, task := range normalized {
		if task.Key == "" {
			t.Fatal("NormalizeTasks left a task without a key")
		}
		if seen[task.Key] {
			t.Fatalf("key collision on %s", task.Key)
		}
		seen[task.Key] = true
	}
	if !seen["ADE-121"] {
		t.Errorf("expected new key ADE-121, got keys %v", seen)
	}
}
