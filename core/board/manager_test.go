package board

import (
	"os"
	"path/filepath"
	"testing"
)

func TestBoardManager_CRUD(t *testing.T) {
	tempDir := t.TempDir()
	mgr := NewManager(tempDir)

	var lastUpdatedTasks []Task
	var lastUpdatedBytes []byte
	mgr.SetOnUpdated(func(tasks []Task, rawBytes []byte) {
		lastUpdatedTasks = tasks
		lastUpdatedBytes = rawBytes
	})

	// 1. Initially empty
	tasks, err := mgr.ListTasks(tempDir)
	if err != nil {
		t.Fatalf("ListTasks failed: %v", err)
	}
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks, got %d", len(tasks))
	}

	// 2. Save new task
	saved, err := mgr.SaveTask(tempDir, Task{
		Title:       "Test task",
		Description: "Testing board manager",
		Status:      "todo",
		Priority:    "high",
	})
	if err != nil {
		t.Fatalf("SaveTask failed: %v", err)
	}
	if saved.ID == "" || saved.Key != "ADE-1" {
		t.Errorf("expected generated Key ADE-1 and non-empty ID, got Key=%s ID=%s", saved.Key, saved.ID)
	}
	if len(lastUpdatedTasks) != 1 {
		t.Errorf("expected update callback with 1 task, got %d", len(lastUpdatedTasks))
	}
	if len(lastUpdatedBytes) == 0 {
		t.Error("expected non-empty rawBytes in update callback")
	}

	// 3. Get task by ID and Key
	byId, err := mgr.GetTask(tempDir, saved.ID)
	if err != nil {
		t.Fatalf("GetTask by ID failed: %v", err)
	}
	if byId.Title != "Test task" {
		t.Errorf("expected title 'Test task', got %s", byId.Title)
	}

	byKey, err := mgr.GetTask(tempDir, "ade-1")
	if err != nil {
		t.Fatalf("GetTask by Key failed: %v", err)
	}
	if byKey.ID != saved.ID {
		t.Errorf("expected ID %s, got %s", saved.ID, byKey.ID)
	}

	// 4. Move status
	moved, err := mgr.MoveTaskStatus(tempDir, saved.Key, "in_progress")
	if err != nil {
		t.Fatalf("MoveTaskStatus failed: %v", err)
	}
	if moved.Status != "in_progress" {
		t.Errorf("expected status 'in_progress', got %s", moved.Status)
	}

	// 5. Invalid status should fail
	_, err = mgr.MoveTaskStatus(tempDir, saved.Key, "invalid_status")
	if err == nil {
		t.Error("expected error on invalid status, got nil")
	}

	// 6. Delete task
	if err := mgr.DeleteTask(tempDir, saved.Key); err != nil {
		t.Fatalf("DeleteTask failed: %v", err)
	}

	remaining, err := mgr.ListTasks(tempDir)
	if err != nil {
		t.Fatalf("ListTasks after delete failed: %v", err)
	}
	if len(remaining) != 0 {
		t.Errorf("expected 0 tasks after delete, got %d", len(remaining))
	}
}

func TestBoardManager_LegacyMigration(t *testing.T) {
	tempDir := t.TempDir()
	forgeDir := filepath.Join(tempDir, ".forge")
	if err := os.MkdirAll(forgeDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	legacyJSON := `[{"id":"legacy-1","key":"ADE-10","title":"Legacy Task","status":"todo","priority":"low"}]`
	legacyPath := filepath.Join(forgeDir, "jira_tasks.json")
	if err := os.WriteFile(legacyPath, []byte(legacyJSON), 0644); err != nil {
		t.Fatalf("write legacy file failed: %v", err)
	}

	mgr := NewManager(tempDir)
	tasks, err := mgr.ListTasks(tempDir)
	if err != nil {
		t.Fatalf("ListTasks failed: %v", err)
	}
	if len(tasks) != 1 || tasks[0].Key != "ADE-10" {
		t.Errorf("expected migrated task ADE-10, got %+v", tasks)
	}

	// Legacy file should have been removed and replaced by board_tasks.json
	if _, err := os.Stat(legacyPath); !os.IsNotExist(err) {
		t.Error("expected legacy jira_tasks.json to be deleted after migration")
	}
	newPath := filepath.Join(forgeDir, "board_tasks.json")
	if _, err := os.Stat(newPath); err != nil {
		t.Errorf("expected board_tasks.json to exist: %v", err)
	}
}
