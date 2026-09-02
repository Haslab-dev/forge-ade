// Package discovery finds MCP servers and skills configured by OTHER agent
// tools on this machine (Claude, Codex, Cursor, Windsurf, Gemini, opencode,
// Antigravity) plus the app's own config, so they can be imported with one
// click. Discovery never mutates anything; imports are explicit.
package discovery

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/hasdev/forge-ade/internal/mcp"
	"github.com/hasdev/forge-ade/internal/skills"
)

// DiscoveredMCPServer is an MCP server entry found in any config source.
type DiscoveredMCPServer struct {
	Name        string            `json:"name"`
	Command     string            `json:"command,omitempty"`
	Args        []string          `json:"args,omitempty"`
	Env         map[string]string `json:"env,omitempty"`
	URL         string            `json:"url,omitempty"`
	Type        string            `json:"type"` // local | remote
	Origin      string            `json:"origin"`
	OriginLabel string            `json:"originLabel"`
	ConfigPath  string            `json:"configPath,omitempty"`
	Imported    bool              `json:"imported"` // already present in our own config
}

// DiscoveredSkill is a skill directory (SKILL.md) found in any source.
type DiscoveredSkill struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Path        string `json:"path,omitempty"`
	Origin      string `json:"origin"`
	OriginLabel string `json:"originLabel"`
	Imported    bool   `json:"imported"`
}

type mcpSource struct {
	origin  string
	label   string
	paths   []string // candidate config files (first existing one wins per entry)
	kind    string   // "claude" | "mcpServers" | "codex" | "opencode" | "gemini"
	loader  func(path string) []DiscoveredMCPServer
}

// homeDir returns the user home directory ("" when unavailable).
func homeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}

func expand(p string) string {
	return os.ExpandEnv(p)
}

// ---------------------------------------------------------------------------
// JSON / JSONC helpers
// ---------------------------------------------------------------------------

// stripJSONComments removes // and /* */ comments outside of strings so
// jsonc files (opencode etc.) parse as plain JSON.
func stripJSONComments(data string) string {
	var b strings.Builder
	b.Grow(len(data))
	inString := false
	escaped := false
	for i := 0; i < len(data); i++ {
		c := data[i]
		if inString {
			b.WriteByte(c)
			if escaped {
				escaped = false
			} else if c == '\\' {
				escaped = true
			} else if c == '"' {
				inString = false
			}
			continue
		}
		switch {
		case c == '"':
			inString = true
			b.WriteByte(c)
		case c == '/' && i+1 < len(data) && data[i+1] == '/':
			for i < len(data) && data[i] != '\n' {
				i++
			}
			if i < len(data) {
				b.WriteByte('\n')
			}
		case c == '/' && i+1 < len(data) && data[i+1] == '*':
			i += 2
			for i+1 < len(data) && !(data[i] == '*' && data[i+1] == '/') {
				i++
			}
			i++ // skip trailing '/'
		default:
			b.WriteByte(c)
		}
	}
	return b.String()
}

func readJSONFile(path string, v interface{}) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(stripJSONComments(string(data))), v)
}

// genericMCPServers reads a JSON file with a top-level `mcpServers` map —
// the shape used by Claude, Cursor, Windsurf, Gemini, Antigravity.
func genericMCPServers(path string) []DiscoveredMCPServer {
	var doc struct {
		MCPServers map[string]struct {
			Command string            `json:"command"`
			Args    []string          `json:"args"`
			Env     map[string]string `json:"env"`
			URL     string            `json:"url"`
			Type    string            `json:"type"`
		} `json:"mcpServers"`
	}
	if err := readJSONFile(path, &doc); err != nil {
		return nil
	}
	var out []DiscoveredMCPServer
	for name, s := range doc.MCPServers {
		if name == "" || (s.Command == "" && s.URL == "") {
			continue
		}
		out = append(out, DiscoveredMCPServer{
			Name:    name,
			Command: expand(s.Command),
			Args:    expandArgs(s.Args),
			Env:     expandEnv(s.Env),
			URL:     expand(s.URL),
			Type:    serverType(s.Type, s.URL),
		})
	}
	return out
}

// claudeJSON reads ~/.claude.json which keeps servers under the top-level
// `mcpServers` map and optionally per-project `projects.<path>.mcpServers`.
func claudeJSON(path string) []DiscoveredMCPServer {
	var doc struct {
		MCPServers map[string]struct {
			Command string            `json:"command"`
			Args    []string          `json:"args"`
			Env     map[string]string `json:"env"`
			URL     string            `json:"url"`
		} `json:"mcpServers"`
		Projects map[string]struct {
			MCPServers map[string]struct {
				Command string            `json:"command"`
				Args    []string          `json:"args"`
				Env     map[string]string `json:"env"`
				URL     string            `json:"url"`
			} `json:"mcpServers"`
		} `json:"projects"`
	}
	if err := readJSONFile(path, &doc); err != nil {
		return nil
	}
	var out []DiscoveredMCPServer
	add := func(name string, s struct {
		Command string            `json:"command"`
		Args    []string          `json:"args"`
		Env     map[string]string `json:"env"`
		URL     string            `json:"url"`
	}) {
		if name == "" || (s.Command == "" && s.URL == "") {
			return
		}
		out = append(out, DiscoveredMCPServer{
			Name:    name,
			Command: expand(s.Command),
			Args:    expandArgs(s.Args),
			Env:     expandEnv(s.Env),
			URL:     expand(s.URL),
			Type:    serverType("", s.URL),
		})
	}
	for name, s := range doc.MCPServers {
		add(name, s)
	}
	for _, p := range doc.Projects {
		for name, s := range p.MCPServers {
			add(name, s)
		}
	}
	return out
}

// codexTOML reads ~/.codex/config.toml with `[mcp_servers.<name>]` tables.
func codexTOML(path string) []DiscoveredMCPServer {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	type serverTable struct {
		Command string            `toml:"command"`
		Args    []string          `toml:"args"`
		Env     map[string]string `toml:"env"`
		URL     string            `toml:"url"`
	}
	var doc struct {
		MCPServers map[string]serverTable `toml:"mcp_servers"`
	}
	if err := toml.Unmarshal(data, &doc); err != nil {
		return nil
	}
	var out []DiscoveredMCPServer
	for name, s := range doc.MCPServers {
		if name == "" || (s.Command == "" && s.URL == "") {
			continue
		}
		out = append(out, DiscoveredMCPServer{
			Name:    name,
			Command: expand(s.Command),
			Args:    expandArgs(s.Args),
			Env:     expandEnv(s.Env),
			URL:     expand(s.URL),
			Type:    serverType("", s.URL),
		})
	}
	return out
}

// opencodeConfig reads opencode.json / opencode.jsonc. Servers use the
// shape `mcp.<name> = {type: "local", command: [..], environment: {..}}`
// or `{type: "remote", url: "..."}`. When both files exist, jsonc wins.
func opencodeConfig(dir string) []DiscoveredMCPServer {
	entries := map[string]DiscoveredMCPServer{}
	for _, name := range []string{"opencode.json", "opencode.jsonc"} {
		path := filepath.Join(dir, name)
		if _, err := os.Stat(path); err != nil {
			continue
		}
		for _, s := range opencodeServers(path) {
			entries[s.Name] = s // jsonc processed last, overwriting json
		}
	}
	var out []DiscoveredMCPServer
	for _, s := range entries {
		out = append(out, s)
	}
	return out
}

func opencodeServers(path string) []DiscoveredMCPServer {
	var doc struct {
		MCP map[string]struct {
			Type        string            `json:"type"`
			Command     []string          `json:"command"`
			Environment map[string]string `json:"environment"`
			URL         string            `json:"url"`
			Enabled     *bool             `json:"enabled"`
		} `json:"mcp"`
	}
	if err := readJSONFile(path, &doc); err != nil {
		return nil
	}
	var out []DiscoveredMCPServer
	for name, s := range doc.MCP {
		if name == "" {
			continue
		}
		if s.Enabled != nil && !*s.Enabled {
			continue
		}
		if len(s.Command) > 0 {
			out = append(out, DiscoveredMCPServer{
				Name:    name,
				Command: expand(s.Command[0]),
				Args:    expandArgs(s.Command[1:]),
				Env:     expandEnv(s.Environment),
				Type:    "local",
			})
		} else if s.URL != "" {
			out = append(out, DiscoveredMCPServer{
				Name: name,
				URL:  expand(s.URL),
				Type: "remote",
			})
		}
	}
	return out
}

func serverType(t, url string) string {
	if url != "" {
		return "remote"
	}
	if t == "remote" {
		return "remote"
	}
	return "local"
}

func expandArgs(args []string) []string {
	if len(args) == 0 {
		return nil
	}
	out := make([]string, 0, len(args))
	for _, a := range args {
		out = append(out, expand(a))
	}
	return out
}

func expandEnv(env map[string]string) map[string]string {
	if len(env) == 0 {
		return nil
	}
	out := make(map[string]string, len(env))
	for k, v := range env {
		out[k] = expand(v)
	}
	return out
}

// ---------------------------------------------------------------------------
// MCP discovery
// ---------------------------------------------------------------------------

func mcpSources() []mcpSource {
	home := homeDir()
	if home == "" {
		return nil
	}
	appData := filepath.Join(home, "Library", "Application Support")
	configDir := os.Getenv("XDG_CONFIG_HOME")
	if configDir == "" {
		configDir = filepath.Join(home, ".config")
	}
	return []mcpSource{
		{
			origin: "claude", label: "Claude Code", kind: "claude",
			paths: []string{filepath.Join(home, ".claude.json")},
			loader: claudeJSON,
		},
		{
			origin: "claude", label: "Claude Code", kind: "mcpServers",
			paths:  []string{filepath.Join(home, ".claude", "mcp.json")},
			loader: genericMCPServers,
		},
		{
			origin: "claude-desktop", label: "Claude Desktop", kind: "mcpServers",
			paths:  []string{filepath.Join(appData, "Claude", "claude_desktop_config.json")},
			loader: genericMCPServers,
		},
		{
			origin: "codex", label: "Codex", kind: "codex",
			paths:  []string{filepath.Join(home, ".codex", "config.toml")},
			loader: codexTOML,
		},
		{
			origin: "cursor", label: "Cursor", kind: "mcpServers",
			paths:  []string{filepath.Join(home, ".cursor", "mcp.json")},
			loader: genericMCPServers,
		},
		{
			origin: "windsurf", label: "Windsurf", kind: "mcpServers",
			paths:  []string{filepath.Join(home, ".codeium", "windsurf", "mcp_config.json")},
			loader: genericMCPServers,
		},
		{
			origin: "gemini", label: "Gemini CLI", kind: "mcpServers",
			paths:  []string{filepath.Join(home, ".gemini", "settings.json")},
			loader: genericMCPServers,
		},
		{
			origin: "opencode", label: "opencode", kind: "opencode",
			paths:  []string{configDir},
			loader: func(path string) []DiscoveredMCPServer { return opencodeConfig(path) },
		},
		{
			origin: "antigravity", label: "Google Antigravity", kind: "mcpServers",
			paths:  []string{filepath.Join(home, ".gemini", "antigravity-cli", "mcp_config.json")},
			loader: genericMCPServers,
		},
	}
}

// DiscoverMCPServers aggregates MCP server entries from every known agent
// config location. `own` is the app's current config, used to flag entries
// that are already imported. Results are deduped by name (first source wins;
// the app's own entries always win).
func DiscoverMCPServers(own []mcp.ServerConfig) []DiscoveredMCPServer {
	ownNames := map[string]bool{}
	for _, s := range own {
		ownNames[strings.ToLower(s.Name)] = true
	}

	seen := map[string]bool{}
	var out []DiscoveredMCPServer
	for _, src := range mcpSources() {
		for _, path := range src.paths {
			for _, s := range src.loader(path) {
				key := strings.ToLower(s.Name)
				if key == "" || seen[key] {
					continue
				}
				seen[key] = true
				s.Origin = src.origin
				s.OriginLabel = src.label
				s.ConfigPath = path
				s.Imported = ownNames[key]
				out = append(out, s)
			}
		}
	}
	return out
}

// ToServerConfig converts a discovered entry into the app's own config shape.
func (d *DiscoveredMCPServer) ToServerConfig() mcp.ServerConfig {
	return mcp.ServerConfig{
		Name:    d.Name,
		Command: d.Command,
		Args:    d.Args,
		Env:     d.Env,
		Type:    d.Type,
		URL:     d.URL,
		Enabled: true,
	}
}

// ---------------------------------------------------------------------------
// Skill discovery
// ---------------------------------------------------------------------------

// skillDirs returns every directory that may contain `<name>/SKILL.md`
// skills from other tools (and our own, which is filtered by the caller).
func skillDirs() []DiscoveredSkill {
	home := homeDir()
	if home == "" {
		return nil
	}
	type dirSource struct {
		dir   string
		origin string
		label  string
	}
	cwd, _ := os.Getwd()
	sources := []dirSource{
		{filepath.Join(home, ".claude", "skills"), "claude", "Claude Code"},
		{filepath.Join(home, ".agents", "skills"), "agents-md", "AGENTS.md skills"},
		{filepath.Join(home, ".gemini", "antigravity-cli", "builtin", "skills"), "antigravity", "Google Antigravity"},
	}
	if cwd != "" {
		sources = append(sources,
			dirSource{filepath.Join(cwd, ".claude", "skills"), "claude-workspace", "Claude Code (workspace)"},
			dirSource{filepath.Join(cwd, ".agents", "skills"), "agents-workspace", "AGENTS.md skills (workspace)"},
		)
	}

	var out []DiscoveredSkill
	seen := map[string]bool{}
	for _, src := range sources {
		entries, err := os.ReadDir(src.dir)
		if err != nil {
			continue
		}
		for _, entry := range entries {
			if !entry.IsDir() {
				continue
			}
			skillFile := filepath.Join(src.dir, entry.Name(), "SKILL.md")
			data, err := os.ReadFile(skillFile)
			if err != nil {
				continue
			}
			sk := parseSkillFrontmatter(entry.Name(), string(data))
			key := strings.ToLower(sk.Name)
			if key == "" || seen[key] {
				continue
			}
			seen[key] = true
			sk.Origin = src.origin
			sk.OriginLabel = src.label
			sk.Path = filepath.Dir(skillFile)
			out = append(out, sk)
		}
	}
	return out
}

// parseSkillFrontmatter extracts name/description from SKILL.md frontmatter,
// mirroring internal/skills parsing.
func parseSkillFrontmatter(fallbackName, content string) DiscoveredSkill {
	sk := DiscoveredSkill{Name: fallbackName}
	lines := strings.Split(content, "\n")
	inFrontmatter := false
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if i == 0 && trimmed == "---" {
			inFrontmatter = true
			continue
		}
		if inFrontmatter {
			if trimmed == "---" {
				break
			}
			if strings.HasPrefix(trimmed, "name:") {
				sk.Name = strings.TrimSpace(strings.TrimPrefix(trimmed, "name:"))
			} else if strings.HasPrefix(trimmed, "description:") {
				sk.Description = strings.TrimSpace(strings.TrimPrefix(trimmed, "description:"))
			}
		}
	}
	return sk
}

// DiscoverSkills aggregates skills from all known locations. `ownNames` holds
// the skill names already available to the app (its own global + workspace
// dirs); matching entries are flagged Imported and deduped.
func DiscoverSkills(own []skills.Skill) []DiscoveredSkill {
	ownNames := map[string]bool{}
	for _, s := range own {
		ownNames[strings.ToLower(s.Name)] = true
	}

	seen := map[string]bool{}
	var out []DiscoveredSkill
	for _, s := range skillDirs() {
		key := strings.ToLower(s.Name)
		if key == "" || seen[key] {
			continue
		}
		seen[key] = true
		s.Imported = ownNames[key]
		out = append(out, s)
	}
	return out
}

// CopySkill imports a discovered skill by copying its directory into the
// app's global skills dir. Returns the destination path.
func CopySkill(srcDir string) (string, error) {
	home := homeDir()
	if home == "" {
		return "", fmt.Errorf("cannot resolve home directory")
	}
	name := filepath.Base(srcDir)
	if name == "" || name == "." || strings.Contains(name, "/") {
		return "", fmt.Errorf("invalid skill directory")
	}
	dest := filepath.Join(home, ".forge-ade", "skills", name)
	if err := os.MkdirAll(dest, 0755); err != nil {
		return "", err
	}
	return dest, copyDir(srcDir, dest)
}

func copyDir(src, dest string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, e := range entries {
		s := filepath.Join(src, e.Name())
		d := filepath.Join(dest, e.Name())
		if e.IsDir() {
			if err := os.MkdirAll(d, 0755); err != nil {
				return err
			}
			if err := copyDir(s, d); err != nil {
				return err
			}
		} else {
			data, err := os.ReadFile(s)
			if err != nil {
				return err
			}
			if err := os.WriteFile(d, data, 0644); err != nil {
				return err
			}
		}
	}
	return nil
}
