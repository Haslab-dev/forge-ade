package app

import (
	"context"
	"testing"

	"github.com/Haslab-dev/RakitKode/search"
	"github.com/Haslab-dev/RakitKode/tools"
)

func TestBoardAPI_CRUD(t *testing.T) {
	tempDir := t.TempDir()
	app := &App{}

	// 1. Initial list should be empty
	tasks, err := app.ListBoardTasks(tempDir)
	if err != nil {
		t.Fatalf("ListBoardTasks failed: %v", err)
	}
	if len(tasks) != 0 {
		t.Fatalf("expected 0 tasks, got %d", len(tasks))
	}

	// 2. Create task
	t1 := BoardTask{
		Title:       "Setup database schema for analytics",
		Description: "Create tables for session events",
		Status:      "todo",
		Priority:    "high",
		Assignee:    "Pandu-Micro",
		Labels:      []string{"db", "analytics"},
	}
	created, err := app.SaveBoardTask(tempDir, t1)
	if err != nil {
		t.Fatalf("SaveBoardTask failed: %v", err)
	}
	if created.Key != "ADE-1" {
		t.Errorf("expected Key ADE-1, got %s", created.Key)
	}
	if created.ID == "" {
		t.Error("expected non-empty ID")
	}

	// 3. Create second task -> should have key ADE-2
	t2 := BoardTask{
		Title:    "Build frontend dashboard",
		Status:   "todo",
		Priority: "urgent",
	}
	created2, err := app.SaveBoardTask(tempDir, t2)
	if err != nil {
		t.Fatalf("SaveBoardTask 2 failed: %v", err)
	}
	if created2.Key != "ADE-2" {
		t.Errorf("expected Key ADE-2, got %s", created2.Key)
	}

	// 4. List should return 2 tasks
	list, err := app.ListBoardTasks(tempDir)
	if err != nil {
		t.Fatalf("ListBoardTasks failed: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("expected 2 tasks, got %d", len(list))
	}

	// 5. GetBoardTask by Key
	fetched, err := app.GetBoardTask(tempDir, "ADE-1")
	if err != nil {
		t.Fatalf("GetBoardTask by Key failed: %v", err)
	}
	if fetched.Title != t1.Title {
		t.Errorf("expected title %q, got %q", t1.Title, fetched.Title)
	}

	// 6. Move status
	moved, err := app.MoveBoardTaskStatus(tempDir, "ADE-1", "in_progress")
	if err != nil {
		t.Fatalf("MoveBoardTaskStatus failed: %v", err)
	}
	if moved.Status != "in_progress" {
		t.Errorf("expected in_progress, got %s", moved.Status)
	}

	// 7. Complete task
	done, err := app.MoveBoardTaskStatus(tempDir, "ADE-1", "done")
	if err != nil {
		t.Fatalf("MoveBoardTaskStatus to done failed: %v", err)
	}
	if done.Status != "done" || done.CompletedAt == 0 {
		t.Errorf("expected done with non-zero CompletedAt, got status=%s, completedAt=%d", done.Status, done.CompletedAt)
	}

	// 8. Delete task ADE-2
	if err := app.DeleteBoardTask(tempDir, "ADE-2"); err != nil {
		t.Fatalf("DeleteBoardTask failed: %v", err)
	}
	afterDel, err := app.ListBoardTasks(tempDir)
	if err != nil {
		t.Fatalf("ListBoardTasks failed: %v", err)
	}
	if len(afterDel) != 1 {
		t.Fatalf("expected 1 task remaining, got %d", len(afterDel))
	}
	if afterDel[0].Key != "ADE-1" {
		t.Errorf("expected ADE-1 remaining, got %s", afterDel[0].Key)
	}
}

func TestBoardAgentTools(t *testing.T) {
	tempDir := t.TempDir()
	app := &App{}
	t.Setenv("HOME", tempDir)

	si := search.NewSearchManager()
	reg := tools.NewRegistry(si)
	RegisterBoardTools(reg, app)

	createTool, ok := reg.Lookup("board_create_task")
	if !ok {
		t.Fatal("board_create_task tool not registered")
	}
	listTool, ok := reg.Lookup("board_list_tasks")
	if !ok {
		t.Fatal("board_list_tasks tool not registered")
	}
	getTool, ok := reg.Lookup("board_get_task")
	if !ok {
		t.Fatal("board_get_task tool not registered")
	}
	updateTool, ok := reg.Lookup("board_update_task")
	if !ok {
		t.Fatal("board_update_task tool not registered")
	}
	deleteTool, ok := reg.Lookup("board_delete_task")
	if !ok {
		t.Fatal("board_delete_task tool not registered")
	}

	ctx := context.Background()

	// 1. Agent creates task
	resCreate, err := createTool.Handler(ctx, map[string]any{
		"title":       "Fix race condition in diff parser",
		"description": "Ensure concurrent diffs don't panic",
		"priority":    "urgent",
		"status":      "todo",
		"labels":      []any{"bug", "concurrency"},
	})
	if err != nil {
		t.Fatalf("createTool failed: %v", err)
	}
	cMap, ok := resCreate.(map[string]any)
	if !ok || cMap["success"] != true {
		t.Fatalf("expected success true in result, got: %v", resCreate)
	}
	taskObj, ok := cMap["task"].(BoardTask)
	if !ok || taskObj.Key != "ADE-1" {
		t.Fatalf("expected created task with Key ADE-1, got %v", cMap["task"])
	}

	// 2. Agent lists tasks
	resList, err := listTool.Handler(ctx, map[string]any{
		"status": "todo",
	})
	if err != nil {
		t.Fatalf("listTool failed: %v", err)
	}
	lMap, ok := resList.(map[string]any)
	if !ok || lMap["total"] != 1 {
		t.Fatalf("expected total 1, got %v", resList)
	}

	// 3. Agent gets task by key
	resGet, err := getTool.Handler(ctx, map[string]any{
		"task_id_or_key": "ADE-1",
	})
	if err != nil {
		t.Fatalf("getTool failed: %v", err)
	}
	gTask, ok := resGet.(BoardTask)
	if !ok || gTask.Title != "Fix race condition in diff parser" {
		t.Fatalf("expected matching task, got %v", resGet)
	}

	// 4. Agent updates task status to in_progress
	resUpdate, err := updateTool.Handler(ctx, map[string]any{
		"task_id_or_key": "ADE-1",
		"status":         "in_progress",
		"assignee":       "Forge Agent",
	})
	if err != nil {
		t.Fatalf("updateTool failed: %v", err)
	}
	uMap, ok := resUpdate.(map[string]any)
	if !ok || uMap["success"] != true {
		t.Fatalf("expected update success, got %v", resUpdate)
	}
	uTask, ok := uMap["task"].(BoardTask)
	if !ok || uTask.Status != "in_progress" || uTask.Assignee != "Forge Agent" {
		t.Fatalf("expected in_progress status and Forge Agent assignee, got %v", uMap["task"])
	}

	// 5. Agent deletes task
	resDel, err := deleteTool.Handler(ctx, map[string]any{
		"task_id_or_key": "ADE-1",
	})
	if err != nil {
		t.Fatalf("deleteTool failed: %v", err)
	}
	dMap, ok := resDel.(map[string]any)
	if !ok || dMap["success"] != true {
		t.Fatalf("expected delete success, got %v", resDel)
	}
}
