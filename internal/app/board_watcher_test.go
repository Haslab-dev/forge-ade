package app

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestBoardWatcher_Normalization(t *testing.T) {
	rawTasks := []BoardTask{
		{
			Title: "Test task without key or ID",
		},
		{
			Key:      "ADE-5",
			Title:    "Existing task with custom key",
			Status:   "INVALID_STATUS",
			Priority: "INVALID_PRIORITY",
		},
	}

	normalized, modified := normalizeBoardTasks(rawTasks)
	if !modified {
		t.Fatal("expected normalizeBoardTasks to report modified=true")
	}

	if len(normalized) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(normalized))
	}

	// Task 0 checks:
	// Since Task 1 has ADE-5, next key assigned to Task 0 should be ADE-6
	t0 := normalized[0]
	if t0.Key != "ADE-6" {
		t.Errorf("expected Key ADE-6, got %s", t0.Key)
	}
	if t0.ID == "" {
		t.Error("expected non-empty ID")
	}
	if t0.Status != "todo" {
		t.Errorf("expected default status 'todo', got %s", t0.Status)
	}
	if t0.Priority != "medium" {
		t.Errorf("expected default priority 'medium', got %s", t0.Priority)
	}
	if t0.Assignee != "Pandu-Micro" {
		t.Errorf("expected default assignee 'Pandu-Micro', got %s", t0.Assignee)
	}
	if t0.CreatedAt == 0 || t0.UpdatedAt == 0 {
		t.Error("expected non-zero timestamps")
	}

	// Task 1 checks:
	t1 := normalized[1]
	if t1.Key != "ADE-5" {
		t.Errorf("expected Key ADE-5 to be preserved, got %s", t1.Key)
	}
	if t1.Status != "todo" {
		t.Errorf("expected normalized status 'todo', got %s", t1.Status)
	}
	if t1.Priority != "medium" {
		t.Errorf("expected normalized priority 'medium', got %s", t1.Priority)
	}

	// Normalizing already clean tasks should not modify anything
	cleanTasks, cleanModified := normalizeBoardTasks(normalized)
	if cleanModified {
		t.Error("expected clean tasks to not be modified")
	}
	if len(cleanTasks) != 2 {
		t.Errorf("expected 2 tasks, got %d", len(cleanTasks))
	}
}

func TestBoardWatcher_WatchWorkspace_InitializesFiles(t *testing.T) {
	tempDir := t.TempDir()
	app := &App{}

	bw, err := newBoardWatcher(app)
	if err != nil {
		t.Fatalf("newBoardWatcher failed: %v", err)
	}
	defer bw.Stop()

	bw.WatchWorkspace([]string{tempDir})

	// Verify .forge/board_tasks.json and .forge/board_instructions.md exist
	forgeDir := filepath.Join(tempDir, ".forge")
	tasksPath := filepath.Join(forgeDir, "board_tasks.json")
	instructionsPath := filepath.Join(forgeDir, "board_instructions.md")

	if _, err := os.Stat(tasksPath); os.IsNotExist(err) {
		t.Errorf("expected %s to be created", tasksPath)
	}
	if _, err := os.Stat(instructionsPath); os.IsNotExist(err) {
		t.Errorf("expected %s to be created", instructionsPath)
	}

	// Verify tasks file contains valid JSON
	data, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatalf("failed to read tasks file: %v", err)
	}
	var tasks []BoardTask
	if err := json.Unmarshal(data, &tasks); err != nil {
		t.Fatalf("expected valid JSON array in %s: %v", tasksPath, err)
	}
}

func TestBoardWatcher_ExternalFileChange(t *testing.T) {
	tempDir := t.TempDir()
	app := &App{}

	bw, err := newBoardWatcher(app)
	if err != nil {
		t.Fatalf("newBoardWatcher failed: %v", err)
	}
	defer bw.Stop()

	bw.WatchWorkspace([]string{tempDir})

	tasksPath := filepath.Join(tempDir, ".forge", "board_tasks.json")

	// External agent writes a new task
	externalData := []map[string]interface{}{
		{
			"title":    "Added by External Agent",
			"priority": "high",
			"status":   "in_progress",
		},
	}
	bytes, _ := json.MarshalIndent(externalData, "", "  ")
	if err := os.WriteFile(tasksPath, bytes, 0644); err != nil {
		t.Fatalf("failed to write external task: %v", err)
	}

	// Wait for debounce and processing
	time.Sleep(350 * time.Millisecond)

	// Read normalized file
	data, err := os.ReadFile(tasksPath)
	if err != nil {
		t.Fatalf("failed to read tasks file after watcher: %v", err)
	}

	var updatedTasks []BoardTask
	if err := json.Unmarshal(data, &updatedTasks); err != nil {
		t.Fatalf("failed to unmarshal updated tasks: %v", err)
	}

	if len(updatedTasks) != 1 {
		t.Fatalf("expected 1 task, got %d", len(updatedTasks))
	}

	task := updatedTasks[0]
	if task.Title != "Added by External Agent" {
		t.Errorf("expected title 'Added by External Agent', got %s", task.Title)
	}
	if task.Key != "ADE-1" {
		t.Errorf("expected auto-generated Key ADE-1, got %s", task.Key)
	}
	if task.ID == "" {
		t.Error("expected auto-generated ID")
	}
	if task.Status != "in_progress" {
		t.Errorf("expected status in_progress, got %s", task.Status)
	}
	if task.Priority != "high" {
		t.Errorf("expected priority high, got %s", task.Priority)
	}
}
