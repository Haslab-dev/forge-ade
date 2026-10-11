package agent

import (
	"strings"
	"testing"
)

func TestParseSkillInvocationLeading(t *testing.T) {
	name, args, ok := parseSkillInvocation("/skill:semantic-compression compress this file")
	if !ok {
		t.Fatal("expected match")
	}
	if name != "semantic-compression" {
		t.Errorf("name = %q, want semantic-compression", name)
	}
	if args != "compress this file" {
		t.Errorf("args = %q, want 'compress this file'", args)
	}
}

func TestParseSkillInvocationMidPrompt(t *testing.T) {
	name, args, ok := parseSkillInvocation("fix the bug /skill:review focus on auth")
	if !ok {
		t.Fatal("expected match")
	}
	if name != "review" {
		t.Errorf("name = %q, want review", name)
	}
	if args != "fix the bug focus on auth" {
		t.Errorf("args = %q, want 'fix the bug focus on auth'", args)
	}
}

func TestParseSkillInvocationNone(t *testing.T) {
	if _, _, ok := parseSkillInvocation("just a normal message"); ok {
		t.Fatal("expected no match")
	}
}

func TestXMLDialectScanner(t *testing.T) {
	s := newXMLDialectScanner()
	var all []dialectEvent
	for _, chunk := range []string{
		"I'll check the file.\n<invoke name=\"read_file\"><parameter name=\"path\">/tmp/x.go</parameter>",
		"<parameter name=\"start_line\">1</parameter></invoke>",
	} {
		all = append(all, s.feed(chunk)...)
	}
	all = append(all, s.flush()...)

	var toolStart, toolEnd int
	var text string
	for _, ev := range all {
		switch ev.kind {
		case "text":
			text += ev.text
		case "tool_start":
			toolStart++
			if ev.toolName != "read_file" {
				t.Errorf("tool_start name = %q, want read_file", ev.toolName)
			}
		case "tool_end":
			toolEnd++
			if ev.toolName != "read_file" {
				t.Errorf("tool_end name = %q, want read_file", ev.toolName)
			}
			if ev.args["path"] != "/tmp/x.go" {
				t.Errorf("tool_end path = %v, want /tmp/x.go", ev.args["path"])
			}
			if ev.args["start_line"] != float64(1) {
				t.Errorf("tool_end start_line = %v, want 1", ev.args["start_line"])
			}
		}
	}
	if toolStart != 1 || toolEnd != 1 {
		t.Fatalf("expected 1 tool start+end, got %d/%d", toolStart, toolEnd)
	}
	if text == "" {
		t.Error("expected leading text to be captured")
	}
}

func TestXMLDialectScannerThinking(t *testing.T) {
	s := newXMLDialectScanner()
	events := s.feed("<thinking>Let me reason about this carefully</thinking>Then I'll answer.")
	events = append(events, s.flush()...)
	var thinking string
	var text string
	for _, ev := range events {
		switch ev.kind {
		case "thinking":
			thinking += ev.text
		case "text":
			text += ev.text
		}
	}
	if thinking != "Let me reason about this carefully" {
		t.Errorf("thinking = %q", thinking)
	}
	if text != "Then I'll answer." {
		t.Errorf("text = %q", text)
	}
}

func TestXMLDialectScannerDeepSeekDSML(t *testing.T) {
	s := newXMLDialectScanner()
	chunks := []string{
		"Saya mulai eksekusi.\n\n<｜｜DSML｜｜ calls>\n<｜｜DSML｜｜ invoke name=\"write\">\n",
		"<｜｜DSML｜｜ parameter name=\"content\" string=\"true\">import React from 'react'\n\nexport function Component() {\n  return <div>Hello</div>\n}\n</｜｜DSML｜｜ parameter>\n",
		"<｜｜DSML｜｜ parameter name=\"path\" string=\"true\">/src/Component.tsx</｜｜DSML｜｜ parameter>\n</｜｜DSML｜｜ invoke>\n</｜｜DSML｜｜ calls>",
	}
	var events []dialectEvent
	for _, chunk := range chunks {
		events = append(events, s.feed(chunk)...)
	}
	events = append(events, s.flush()...)

	var text string
	var toolStart, toolEnd int
	var args map[string]any

	for _, ev := range events {
		switch ev.kind {
		case "text":
			text += ev.text
		case "tool_start":
			toolStart++
			if ev.toolName != "write" {
				t.Errorf("expected tool_start write, got %q", ev.toolName)
			}
		case "tool_end":
			toolEnd++
			args = ev.args
		}
	}

	if toolStart != 1 || toolEnd != 1 {
		t.Fatalf("expected 1 tool_start and 1 tool_end, got %d/%d", toolStart, toolEnd)
	}
	if strings.Contains(text, "import React") || strings.Contains(text, "DSML") {
		t.Errorf("code or DSML tags leaked into text: %q", text)
	}
	if !strings.Contains(text, "Saya mulai eksekusi.") {
		t.Errorf("conversational text lost: %q", text)
	}
	if args["path"] != "/src/Component.tsx" {
		t.Errorf("expected path /src/Component.tsx, got %v", args["path"])
	}
	contentStr, ok := args["content"].(string)
	if !ok || !strings.Contains(contentStr, "<div>Hello</div>") {
		t.Errorf("expected content to contain <div>Hello</div>, got %v", args["content"])
	}
}

func TestXMLDialectScannerPreservesHTMLAndGenerics(t *testing.T) {
	s := newXMLDialectScanner()
	input := "Here is a React component with `<div className=\"box\">` and TypeScript generic `useState<Theme>()`."
	events := s.feed(input)
	events = append(events, s.flush()...)

	var text string
	for _, ev := range events {
		if ev.kind == "text" {
			text += ev.text
		}
	}

	if text != input {
		t.Errorf("expected text %q, got %q", input, text)
	}
}
