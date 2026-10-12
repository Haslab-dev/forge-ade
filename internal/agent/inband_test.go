package agent

import (
	"encoding/json"
	"testing"
)

func TestParseInbandToolCallsMimoSkill(t *testing.T) {
	input := `<toolcall><function=skill><parameter=skill>impeccable</parameter></function></toolcall>`
	calls, cleaned := parseInbandToolCalls(input)
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Function.Name != "skill" {
		t.Errorf("expected function name 'skill', got %q", calls[0].Function.Name)
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(calls[0].Function.Arguments), &args); err != nil {
		t.Fatalf("failed to unmarshal args: %v", err)
	}
	if args["skill"] != "impeccable" || args["name"] != "impeccable" {
		t.Errorf("expected skill and name = 'impeccable', got %v", args)
	}
	if cleaned != "" {
		t.Errorf("expected empty cleaned text, got %q", cleaned)
	}
}

func TestParseInbandToolCallsMixedText(t *testing.T) {
	input := `I will load the design skill for you.
<toolcall><function=load_skill><parameter=name>impeccable</parameter></function></toolcall>
Please wait a moment.`

	calls, cleaned := parseInbandToolCalls(input)
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Function.Name != "load_skill" {
		t.Errorf("expected function name 'load_skill', got %q", calls[0].Function.Name)
	}
	if !testing.Short() {
		expectedCleaned := "I will load the design skill for you.\n\nPlease wait a moment."
		if cleaned != expectedCleaned {
			t.Errorf("cleaned text mismatch: got %q, want %q", cleaned, expectedCleaned)
		}
	}
}

func TestParseInbandToolCallsJSON(t *testing.T) {
	input := `<tool_call>
{"name": "read", "arguments": {"path": "src/App.tsx"}}
</tool_call>`
	calls, cleaned := parseInbandToolCalls(input)
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Function.Name != "read" {
		t.Errorf("expected 'read', got %q", calls[0].Function.Name)
	}
	if cleaned != "" {
		t.Errorf("expected empty cleaned text, got %q", cleaned)
	}
}

func TestParseInbandToolCallsInvoke(t *testing.T) {
	input := `<invoke name="python_eval">
<parameter name="code">import math; math.sqrt(256)</parameter>
</invoke>`
	calls, _ := parseInbandToolCalls(input)
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Function.Name != "python_eval" {
		t.Errorf("expected 'python_eval', got %q", calls[0].Function.Name)
	}
}

func TestParseInbandToolCallsDeepSeekDSML(t *testing.T) {
	input := `< | | DSML | | calls>
< | | DSML | | invoke name="read_multiple">
< | | DSML | | parameter name="paths" string="false">
[
  "/tmp/skills/impeccable/SKILL.md",
  "/tmp/skills/impeccable/reference.md"
]
<| | DSML | | parameter>
</ | | DSML | | invoke>
</ | | DSML | | calls>`

	calls, cleaned := parseInbandToolCalls(input)
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].Function.Name != "read_multiple" {
		t.Fatalf("expected function 'read_multiple', got %q", calls[0].Function.Name)
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(calls[0].Function.Arguments), &args); err != nil {
		t.Fatalf("failed to unmarshal args: %v", err)
	}
	paths, ok := args["paths"].([]any)
	if !ok || len(paths) != 2 {
		t.Fatalf("expected 2 paths in args, got %v", args["paths"])
	}
	if cleaned != "" {
		t.Fatalf("expected empty cleaned text, got %q", cleaned)
	}
}
