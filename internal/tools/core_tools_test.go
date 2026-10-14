package tools

import (
	"context"
	"testing"
	"time"
)

type mockSessionBridge struct {
	SessionBridge
	questions []AskQuestion
	todos     []TodoItem
}

func (m *mockSessionBridge) Ask(questions []AskQuestion) error {
	m.questions = questions
	return nil
}

func TestAskToolFlexibility(t *testing.T) {
	bridge := &mockSessionBridge{}
	ctx := WithSessionBridge(context.Background(), bridge)
	tool := askTool()

	// 1. Free-text question (no options / null options)
	_, err := tool.Handler(ctx, map[string]any{
		"questions": []any{
			map[string]any{
				"id":       "product",
				"question": "What is this product?",
				"options":  nil,
			},
		},
	})
	if err != nil {
		t.Fatalf("free-text question failed: %v", err)
	}
	if len(bridge.questions) != 1 || bridge.questions[0].ID != "product" {
		t.Fatalf("unexpected questions: %+v", bridge.questions)
	}
	if len(bridge.questions[0].Options) != 0 {
		t.Fatalf("expected 0 options for free-text question, got %v", bridge.questions[0].Options)
	}

	// 2. Object options
	_, err = tool.Handler(ctx, map[string]any{
		"questions": []any{
			map[string]any{
				"id":       "theme",
				"question": "Choose a theme",
				"options": []any{
					map[string]any{"label": "Dark", "description": "Sleek dark theme"},
					map[string]any{"title": "Light"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("object options failed: %v", err)
	}
	if len(bridge.questions[0].Options) != 2 {
		t.Fatalf("expected 2 options, got %v", bridge.questions[0].Options)
	}
	if bridge.questions[0].Options[0] != "Dark — Sleek dark theme" {
		t.Errorf("expected Dark — Sleek dark theme, got %q", bridge.questions[0].Options[0])
	}
	if bridge.questions[0].Options[1] != "Light" {
		t.Errorf("expected Light, got %q", bridge.questions[0].Options[1])
	}

	// 3. Single question passed at top level
	_, err = tool.Handler(ctx, map[string]any{
		"id":       "single",
		"question": "Single question?",
	})
	if err != nil {
		t.Fatalf("single question failed: %v", err)
	}
	if len(bridge.questions) != 1 || bridge.questions[0].ID != "single" {
		t.Fatalf("unexpected single question: %+v", bridge.questions)
	}
}

func TestCleanToolPathAndArgAliases(t *testing.T) {
	// Test cleanToolPath
	cases := []struct {
		input    string
		expected string
	}{
		{`index.css"`, "index.css"},
		{`"index.css"`, "index.css"},
		{`index.css"}`, "index.css"},
		{`index.css"}]`, "index.css"},
		{"  /foo/bar.go  ", "/foo/bar.go"},
		{"`path/to/file`", "path/to/file"},
	}
	for _, tc := range cases {
		got := cleanToolPath(tc.input)
		if got != tc.expected {
			t.Errorf("cleanToolPath(%q) = %q; want %q", tc.input, got, tc.expected)
		}
	}

	// Test argPath aliases
	if p := argPath(map[string]any{"filepath": "test.go"}); p != "test.go" {
		t.Errorf("argPath filepath alias failed: %q", p)
	}
	if p := argPath(map[string]any{"filePath": `main.rs"`}); p != "main.rs" {
		t.Errorf("argPath filePath alias failed: %q", p)
	}
	if p := argPath(map[string]any{"target_file": `index.html"}`}); p != "index.html" {
		t.Errorf("argPath target_file alias failed: %q", p)
	}
	if p := argPath(map[string]any{"path": map[string]any{"file": "nested.ts"}}); p != "nested.ts" {
		t.Errorf("argPath nested object failed: %q", p)
	}

	// Test argContent aliases
	if c := argContent(map[string]any{"text": "hello"}); c != "hello" {
		t.Errorf("argContent text failed: %q", c)
	}
	if c := argContent(map[string]any{"code": "fmt.Println()"}); c != "fmt.Println()" {
		t.Errorf("argContent code failed: %q", c)
	}

	// Test argCommand aliases
	if cmd := argCommand(map[string]any{"cmd": "ls -la"}); cmd != "ls -la" {
		t.Errorf("argCommand cmd failed: %q", cmd)
	}
	if cmd := argCommand(map[string]any{"script": "echo 1"}); cmd != "echo 1" {
		t.Errorf("argCommand script failed: %q", cmd)
	}
}

func (m *mockSessionBridge) GetTodos() []TodoItem {
	if m.todos != nil {
		return m.todos
	}
	return []TodoItem{
		{ID: "1", Title: "Step 1", Status: "completed"},
		{ID: "2", Title: "Step 2", Status: "in_progress"},
	}
}

func (m *mockSessionBridge) SetTodos(items []TodoItem) {
	m.todos = items
}

func TestUpdateTasksTool(t *testing.T) {
	bridge := &mockSessionBridge{}
	ctx := WithSessionBridge(context.Background(), bridge)
	tool := updateTasksTool()

	// 1. Standard task update
	res, err := tool.Handler(ctx, map[string]any{
		"tasks": []any{
			map[string]any{
				"id":          "1",
				"title":       "Step 1",
				"status":      "completed",
				"active_form": "",
			},
			map[string]any{
				"id":          "2",
				"title":       "Step 2",
				"status":      "completed",
				"active_form": "",
			},
			map[string]any{
				"id":          "3",
				"title":       "Step 3",
				"status":      "in_progress",
				"active_form": "Executing Step 3...",
			},
			map[string]any{
				"id":     "4",
				"title":  "Step 4",
				"status": "pending",
			},
		},
	})
	if err != nil {
		t.Fatalf("update_tasks handler returned error: %v", err)
	}

	m, ok := res.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", res)
	}
	if m["completed"] != 2 {
		t.Errorf("expected completed=2, got %v", m["completed"])
	}
	if m["in_progress"] != 1 {
		t.Errorf("expected in_progress=1, got %v", m["in_progress"])
	}
	if m["pending"] != 1 {
		t.Errorf("expected pending=1, got %v", m["pending"])
	}
	if m["total"] != 4 {
		t.Errorf("expected total=4, got %v", m["total"])
	}
	if m["current_focus"] != "Executing Step 3..." {
		t.Errorf("expected current_focus='Executing Step 3...', got %v", m["current_focus"])
	}
	justCompleted, ok := m["just_completed"].([]string)
	if !ok || len(justCompleted) != 1 || justCompleted[0] != "Step 2" {
		t.Errorf("expected just_completed=['Step 2'], got %v", m["just_completed"])
	}

	// 2. Clear checklist with empty array []
	resClear, err := tool.Handler(ctx, map[string]any{
		"tasks": []any{},
	})
	if err != nil {
		t.Fatalf("clear tasks failed: %v", err)
	}
	mClear, ok := resClear.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", resClear)
	}
	if mClear["total"] != 0 {
		t.Errorf("expected total=0 on clear, got %v", mClear["total"])
	}
	if len(bridge.todos) != 0 {
		t.Errorf("expected bridge.todos to be empty after clear, got %d items", len(bridge.todos))
	}

	// 3. Stringified JSON with status synonyms ("done", "in-progress")
	jsonStr := `[{"title":"Task A","status":"done"},{"title":"Task B","status":"in-progress","active_form":"Building Task B"}]`
	resSynonym, err := tool.Handler(ctx, map[string]any{
		"tasks": jsonStr,
	})
	if err != nil {
		t.Fatalf("stringified json tasks failed: %v", err)
	}
	mSyn, ok := resSynonym.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", resSynonym)
	}
	if mSyn["completed"] != 1 || mSyn["in_progress"] != 1 || mSyn["total"] != 2 {
		t.Errorf("unexpected synonym counts: %+v", mSyn)
	}
	if len(bridge.todos) != 2 {
		t.Fatalf("expected bridge.todos to have 2 items, got %d", len(bridge.todos))
	}
	if bridge.todos[0].Status != "completed" || bridge.todos[1].Status != "in_progress" {
		t.Errorf("synonym status mapping failed: %+v", bridge.todos)
	}
}

func (m *mockSessionBridge) SpawnSubagent(task string, role string) (string, error) {
	return "mock-subagent-123", nil
}

func (m *mockSessionBridge) SpawnSubagentWithModel(task string, role string, model string) (string, error) {
	return "mock-subagent-123", nil
}

func (m *mockSessionBridge) GetSubagents() []SubagentSummary {
	return []SubagentSummary{
		{ID: "mock-subagent-123", Name: "Mock Researcher", Role: "research", State: "idle", Output: "Analysis completed successfully."},
	}
}

func (m *mockSessionBridge) WaitSubagent(ctx context.Context, subID string, timeout time.Duration) (map[string]any, error) {
	return map[string]any{
		"subagent_id": subID,
		"status":      "completed",
		"state":       "idle",
		"output":      "Mock subagent completed review: all checks pass.",
	}, nil
}

func TestInvokeSubagentTool(t *testing.T) {
	bridge := &mockSessionBridge{}
	ctx := WithSessionBridge(context.Background(), bridge)
	tool := invokeSubagentTool()

	// 1. Asynchronous spawn
	res, err := tool.Handler(ctx, map[string]any{
		"role": "research",
		"task": "Investigate unit tests",
	})
	if err != nil {
		t.Fatalf("invoke_subagent returned error: %v", err)
	}
	m, ok := res.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", res)
	}
	if m["subagent_id"] != "mock-subagent-123" {
		t.Errorf("expected subagent_id=mock-subagent-123, got %v", m["subagent_id"])
	}
	if m["status"] != "spawned" {
		t.Errorf("expected status=spawned, got %v", m["status"])
	}

	// 2. Blocking spawn
	resBlock, err := tool.Handler(ctx, map[string]any{
		"role":  "coding",
		"task":  "Review StatusBadge.tsx",
		"block": true,
	})
	if err != nil {
		t.Fatalf("blocking invoke_subagent returned error: %v", err)
	}
	mBlock, ok := resBlock.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", resBlock)
	}
	if mBlock["status"] != "completed" {
		t.Errorf("expected status=completed, got %v", mBlock["status"])
	}
	if mBlock["output"] == "" {
		t.Errorf("expected non-empty output")
	}
}

func TestSubagentOutputAndListTools(t *testing.T) {
	bridge := &mockSessionBridge{}
	ctx := WithSessionBridge(context.Background(), bridge)

	// get_subagent_output
	outTool := getSubagentOutputTool()
	res, err := outTool.Handler(ctx, map[string]any{
		"subagent_id": "mock-subagent-123",
		"block":       false,
	})
	if err != nil {
		t.Fatalf("get_subagent_output returned error: %v", err)
	}
	m, ok := res.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", res)
	}
	if m["subagent_id"] != "mock-subagent-123" {
		t.Errorf("expected subagent_id=mock-subagent-123, got %v", m["subagent_id"])
	}

	// list_subagents
	listTool := listSubagentsTool()
	resList, err := listTool.Handler(ctx, map[string]any{})
	if err != nil {
		t.Fatalf("list_subagents returned error: %v", err)
	}
	mList, ok := resList.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", resList)
	}
	if mList["count"] != 1 {
		t.Errorf("expected count=1, got %v", mList["count"])
	}
}
