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
