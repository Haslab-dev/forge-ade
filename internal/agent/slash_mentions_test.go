package agent

import (
	"strings"
	"testing"

	"github.com/hasdev/forge-ade/internal/mcp"
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

func TestBuildMCPDirective(t *testing.T) {
	m := mcp.NewManager(t.TempDir())
	_, _ = m.SaveServer(mcp.ServerConfig{Name: "exa", Command: "exa"})
	directive := buildMCPDirective(m, slashMention{Kind: "mcp", Name: "exa"})
	if !strings.Contains(directive, "no matching tools") && !strings.Contains(directive, "exa") {
		t.Fatalf("unexpected directive: %q", directive)
	}
}
