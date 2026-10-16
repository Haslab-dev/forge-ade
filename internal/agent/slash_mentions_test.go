package agent

import (
	"strings"
	"testing"

	"github.com/Haslab-dev/RakitKode/mcp"
)

func TestExtractSlashMentions(t *testing.T) {
	text := "use /skill:code-review and /mcp:exa then /mcp:exa:web_search for docs"
	ms := extractSlashMentions(text)
	if len(ms) != 3 {
		t.Fatalf("got %d mentions: %+v", len(ms), ms)
	}
	if ms[0].Kind != "skill" || ms[0].Name != "code-review" {
		t.Fatalf("m0 = %+v", ms[0])
	}
	if ms[1].Kind != "mcp" || ms[1].Name != "exa" {
		t.Fatalf("m1 = %+v", ms[1])
	}
	if ms[2].Kind != "mcp" || ms[2].Name != "exa:web_search" {
		t.Fatalf("m2 = %+v", ms[2])
	}
	if !strings.Contains(text, "/skill:code-review") {
		t.Fatal("sanity")
	}
}

func TestExtractSlashCommands(t *testing.T) {
	tests := []struct {
		input    string
		wantKind string
		wantName string
		wantArgs string
	}{
		{"/goal Build awesome feature", "goal", "Build awesome feature", ""},
		{"/plan Refactor architecture", "plan", "Refactor architecture", ""},
		{"/plan-review", "plan-review", "", ""},
		{"/security", "security", "", ""},
		{"/vibe", "vibe", "", ""},
		{"/model", "model", "", ""},
		{"/resume", "resume", "", ""},
		{"/settings", "settings", "", ""},
		{"/setup", "setup", "", ""},
		{"/clear", "clear", "", ""},
		{"/skill impeccable polish the buttons", "skill", "impeccable", "polish the buttons"},
		{"/skill:impeccable polish the buttons", "skill", "impeccable", "polish the buttons"},
		{"/skill:\"Native SDK Discovery & Authoring\" test this", "skill", "Native SDK Discovery & Authoring", "test this"},
		{"/skill:'Native SDK Discovery & Authoring'", "skill", "Native SDK Discovery & Authoring", ""},
	}

	for _, tt := range tests {
		ms := extractSlashMentions(tt.input)
		if len(ms) == 0 {
			t.Fatalf("expected mention for %q, got none", tt.input)
		}
		if ms[0].Kind != tt.wantKind {
			t.Errorf("input %q: got kind %q, want %q", tt.input, ms[0].Kind, tt.wantKind)
		}
		if ms[0].Name != tt.wantName {
			t.Errorf("input %q: got name %q, want %q", tt.input, ms[0].Name, tt.wantName)
		}
		if tt.wantArgs != "" && ms[0].Args != tt.wantArgs {
			t.Errorf("input %q: got args %q, want %q", tt.input, ms[0].Args, tt.wantArgs)
		}
	}
}

func TestBuildMCPDirective(t *testing.T) {
	m := mcp.NewManager(t.TempDir())
	_, _ = m.SaveServer(mcp.ServerConfig{Name: "exa", Command: "exa"})
	directive := buildMCPDirective(m, slashMention{Kind: "mcp", Name: "exa"})
	if !strings.Contains(directive, "no matching tools") && !strings.Contains(directive, "exa") {
		t.Fatalf("unexpected directive: %q", directive)
	}
}
