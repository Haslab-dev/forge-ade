package agentclirc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeAll materializes a fake home directory from path→content pairs
// (directories end with '/').
func writeAll(t *testing.T, files map[string]string) string {
	t.Helper()
	home := t.TempDir()
	for p, content := range files {
		full := filepath.Join(home, p)
		if strings.HasSuffix(content, "/") {
			if err := os.MkdirAll(full, 0o755); err != nil {
				t.Fatal(err)
			}
			continue
		}
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	return home
}

func TestBuildCodex(t *testing.T) {
	home := writeAll(t, map[string]string{
		".codex/config.toml": `
model = "gpt-5.3-codex"

[model_providers.openrouter]
name = "OpenRouter"
base_url = "https://openrouter.ai/api/v1"

[mcp_servers.context7]
command = "npx"
args = ["-y", "@context7/mcp"]

[mcp_servers.docs]
url = "https://docs.example.com/mcp"
`,
	})
	rc := buildCodex(home).rc
	if rc.DefaultModel != "gpt-5.3-codex" {
		t.Errorf("model = %q", rc.DefaultModel)
	}
	if len(rc.Providers) != 1 || rc.Providers[0].ID != "openrouter" || rc.Providers[0].BaseURL != "https://openrouter.ai/api/v1" {
		t.Errorf("providers = %+v", rc.Providers)
	}
	if len(rc.MCPServers) != 2 {
		t.Fatalf("mcp servers = %+v", rc.MCPServers)
	}
	byName := map[string]MCPServer{}
	for _, s := range rc.MCPServers {
		byName[s.Name] = s
	}
	if got := byName["context7"]; got.Type != "local" || !strings.Contains(got.Command, "context7/mcp") {
		t.Errorf("context7 = %+v", got)
	}
	if got := byName["docs"]; got.Type != "remote" {
		t.Errorf("docs = %+v", got)
	}
}

func TestBuildOpencode(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", "")
	home := writeAll(t, map[string]string{
		".config/opencode/opencode.json": `{
  "$schema": "https://opencode.ai/config.json",
  "model": "anthropic/claude-sonnet-4-5",
  // jsonc comment
  "provider": {
    "anthropic": {
      "name": "Anthropic",
      "models": { "claude-sonnet-4-5": {}, "claude-haiku-4-5": {} }
    },
    "ollama": { "options": { "baseURL": "http://localhost:11434/v1" } }
  },
  "mcp": {
    "context7": { "type": "local", "command": ["npx", "-y", "@context7/mcp"] },
    "web": { "type": "remote", "url": "https://mcp.example.com/sse", "enabled": true },
    "off": { "type": "local", "command": ["noop"], "enabled": false }
  }
}`,
		".config/opencode/skill/review/SKILL.md": "---\nname: review\ndescription: Review code\n---\nbody",
	})
	rc := buildOpencode(home).rc
	if rc.DefaultModel != "anthropic/claude-sonnet-4-5" {
		t.Errorf("model = %q", rc.DefaultModel)
	}
	if len(rc.Providers) != 2 {
		t.Fatalf("providers = %+v", rc.Providers)
	}
	byID := map[string]ProviderInfo{}
	for _, p := range rc.Providers {
		byID[p.ID] = p
	}
	if got := byID["anthropic"]; len(got.Models) != 2 {
		t.Errorf("anthropic models = %+v", got)
	}
	if got := byID["ollama"]; got.BaseURL != "http://localhost:11434/v1" {
		t.Errorf("ollama = %+v", got)
	}
	if len(rc.MCPServers) != 2 {
		t.Fatalf("mcp = %+v (disabled entry must be skipped)", rc.MCPServers)
	}
	if len(rc.Skills) != 1 || rc.Skills[0].Name != "review" {
		t.Errorf("skills = %+v", rc.Skills)
	}
}

func TestBuildClaude(t *testing.T) {
	home := writeAll(t, map[string]string{
		".claude/settings.json":        `{"model": "claude-opus-4-6", "env": {"FOO": "bar"}}`,
		".claude.json":                 `{"mcpServers": {"github": {"command": "npx", "args": ["-y", "@modelcontextprotocol/server-github"]}}}`,
		".claude/skills/deep/SKILL.md": "---\nname: deep\ndescription: Deep dive\n---\n",
	})
	rc := buildClaude(home).rc
	if rc.DefaultModel != "claude-opus-4-6" {
		t.Errorf("model = %q", rc.DefaultModel)
	}
	if len(rc.MCPServers) != 1 || rc.MCPServers[0].Name != "github" {
		t.Errorf("mcp = %+v", rc.MCPServers)
	}
	if len(rc.Skills) != 1 || rc.Skills[0].Description != "Deep dive" {
		t.Errorf("skills = %+v", rc.Skills)
	}
}

func TestBuildOmp(t *testing.T) {
	home := writeAll(t, map[string]string{
		".omp/agent/config.yml": "model: anthropic/claude-opus-4-6\n",
		// Verified omp shape: providers → models[].id
		".omp/agent/models.yml": `
providers:
  z-ai:
    models:
      - id: glm-5.3
      - id: glm-5.3-flash
  openrouter:
    baseUrl: https://openrouter.ai/api/v1
    models:
      - id: deepseek/deepseek-v4
`,
		// Global mcp.json — same mcpServers shape as the project file.
		".omp/mcp.json": `{
  "mcpServers": {
    "filesystem": { "command": "npx", "args": ["-y", "@modelcontextprotocol/server-filesystem", "/tmp"] }
  }
}`,
		// Skills: one level deep — skills/<name>/SKILL.md.
		".omp/agent/skills/ship/SKILL.md": "---\nname: ship\ndescription: Ship it\n---\n",
		// Nested skills are NOT discovered by omp — and neither by us.
		".omp/agent/skills/databases/postgres/SKILL.md": "---\nname: postgres\n---\n",
	})
	rc := buildOmp(home).rc
	if rc.DefaultModel != "anthropic/claude-opus-4-6" {
		t.Errorf("model = %q", rc.DefaultModel)
	}
	if len(rc.Providers) != 2 {
		t.Fatalf("providers = %+v", rc.Providers)
	}
	byID := map[string]ProviderInfo{}
	for _, p := range rc.Providers {
		byID[p.ID] = p
	}
	if got := byID["z-ai"]; len(got.Models) != 2 || got.Models[0] != "glm-5.3" {
		t.Errorf("z-ai = %+v", got)
	}
	if got := byID["openrouter"]; got.BaseURL != "https://openrouter.ai/api/v1" || len(got.Models) != 1 {
		t.Errorf("openrouter = %+v", got)
	}
	if len(rc.MCPServers) != 1 || rc.MCPServers[0].Name != "filesystem" {
		t.Errorf("mcp = %+v", rc.MCPServers)
	}
	if len(rc.Skills) != 1 || rc.Skills[0].Name != "ship" {
		t.Errorf("skills = %+v (nested dirs must not be discovered)", rc.Skills)
	}
}

func TestBuildPiAndAntigravityAndGemini(t *testing.T) {
	home := writeAll(t, map[string]string{
		".pi/agent/models.json":                   `{"providers": [{"id": "anthropic", "name": "Anthropic", "baseUrl": "https://api.anthropic.com", "models": ["claude-opus-4-6"]}]}`,
		".pi/agent/skills/pair/SKILL.md":          "---\nname: pair\ndescription: Pair program\n---\n",
		".gemini/antigravity-cli/settings.json":   `{"model": "gemini-3-pro"}`,
		".gemini/antigravity-cli/mcp_config.json": `{"mcpServers": {"remote1": {"url": "https://mcp.example.com"}}}`,
		".gemini/config/skills/gskills/SKILL.md":  "---\nname: gskills\ndescription: Antigravity global skill\n---\n",
		".gemini/settings.json":                   `{"mcpServers": {"local1": {"command": "uvx", "args": ["mcp-server-time"]}}}`,
	})
	pi := buildPi(home).rc
	if len(pi.Providers) != 1 || len(pi.Providers[0].Models) != 1 {
		t.Errorf("pi providers = %+v", pi.Providers)
	}
	if len(pi.Skills) != 1 || pi.Skills[0].Name != "pair" {
		t.Errorf("pi skills = %+v", pi.Skills)
	}

	ag := buildAntigravity(home).rc
	if ag.DefaultModel != "gemini-3-pro" {
		t.Errorf("antigravity model = %q", ag.DefaultModel)
	}
	if len(ag.MCPServers) != 1 || ag.MCPServers[0].Type != "remote" {
		t.Errorf("antigravity mcp = %+v", ag.MCPServers)
	}
	if len(ag.Skills) != 1 || ag.Skills[0].Name != "gskills" {
		t.Errorf("antigravity skills = %+v", ag.Skills)
	}

	gem := buildGemini(home).rc
	if len(gem.MCPServers) != 1 || gem.MCPServers[0].Name != "local1" {
		t.Errorf("gemini mcp = %+v", gem.MCPServers)
	}
}

func TestListCoversAllClis(t *testing.T) {
	t.Setenv("CODEX_HOME", "")
	// Point HOME at an empty temp dir: List must still return every CLI with
	// paths filled in and no panic.
	t.Setenv("HOME", t.TempDir())
	t.Setenv("USERPROFILE", os.Getenv("HOME"))
	list := List()
	ids := map[string]bool{}
	for _, rc := range list {
		ids[rc.ID] = true
	}
	for _, want := range []string{"pi", "ohmypi", "opencode", "codex", "claude-code", "antigravity", "gemini"} {
		if !ids[want] {
			t.Errorf("missing CLI %q in list: %v", want, ids)
		}
	}
}
