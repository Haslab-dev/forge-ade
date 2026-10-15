package tools

import (
	"context"
	"testing"
)

func TestPythonEvalTool(t *testing.T) {
	if findPythonBinary() == "" {
		t.Skip("skipping python test: python3 not found on system")
	}

	tool := pythonEvalTool()
	ctx := context.Background()

	// 1. Math expression REPL evaluation
	res, err := tool.Handler(ctx, map[string]any{
		"code": "import math\nmath.sqrt(144)",
	})
	if err != nil {
		t.Fatalf("python eval failed: %v", err)
	}
	m, ok := res.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", res)
	}
	if m["result"] != "12.0" {
		t.Errorf("expected result '12.0', got %q", m["result"])
	}

	// 2. Multiline execution with print
	resPrint, err := tool.Handler(ctx, map[string]any{
		"code": "a = [1, 2, 3]\nprint(sum(a))",
	})
	if err != nil {
		t.Fatalf("python print failed: %v", err)
	}
	mPrint, _ := resPrint.(map[string]any)
	if mPrint["stdout"] != "6" {
		t.Errorf("expected stdout '6', got %q", mPrint["stdout"])
	}
}

func TestBunEvalTool(t *testing.T) {
	if findBunBinary() == "" {
		t.Skip("skipping bun test: bun not found on system")
	}

	tool := evalTool()
	ctx := context.Background()

	// 1. JS expression with auto-detection (exact user case from prompt)
	res, err := tool.Handler(ctx, map[string]any{
		"code": "const items = ['apple', 'banana', 'cherry']; items.map(x => x.toUpperCase()).join(' - ')",
	})
	if err != nil {
		t.Fatalf("bun eval failed: %v", err)
	}
	m, ok := res.(map[string]any)
	if !ok {
		t.Fatalf("expected map[string]any, got %T", res)
	}
	if m["result"] != "APPLE - BANANA - CHERRY" {
		t.Errorf("expected result 'APPLE - BANANA - CHERRY', got %q", m["result"])
	}
}

func TestDetectCodeLanguage(t *testing.T) {
	cases := []struct {
		code     string
		expected string
	}{
		{"const x = [1, 2, 3];", "javascript"},
		{"let y = 'hello';", "javascript"},
		{"console.log('hi');", "javascript"},
		{"items.map(x => x * 2)", "javascript"},
		{"import math\nmath.sqrt(16)", "python"},
		{"def foo(): return 42", "python"},
		{"echo 'hello world'", "bash"},
	}

	for _, c := range cases {
		got := detectCodeLanguage(c.code)
		if got != c.expected {
			t.Errorf("code %q: expected %q, got %q", c.code, c.expected, got)
		}
	}
}
