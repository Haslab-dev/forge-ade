package tools

import (
	"context"
	"testing"
)

type mockSessionBridge struct {
	SessionBridge
	questions []AskQuestion
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
