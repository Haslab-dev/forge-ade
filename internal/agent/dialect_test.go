package agent

import "testing"

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
