package agentclirc

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// builder accumulates one CLI's RuntimeConfig.
type builder struct {
	rc RuntimeConfig
}

// Runtime returns the accumulated config with nil slices replaced by empty
// ones, so JSON marshalling always emits arrays (never null) for consumers
// that read .length/.map directly.
func (b *builder) Runtime() RuntimeConfig {
	if b.rc.Paths == nil {
		b.rc.Paths = []ConfigPath{}
	}
	if b.rc.Providers == nil {
		b.rc.Providers = []ProviderInfo{}
	}
	if b.rc.MCPServers == nil {
		b.rc.MCPServers = []MCPServer{}
	}
	if b.rc.Skills == nil {
		b.rc.Skills = []Skill{}
	}
	return b.rc
}

func (b *builder) addError(err error) {
	b.rc.addError(err)
}

func (b *builder) addPath(purpose, path string) {
	_, err := os.Stat(path)
	b.rc.Paths = append(b.rc.Paths, ConfigPath{Purpose: purpose, Path: path, Exists: err == nil})
}

// parseSkillsDir scans a directory of Agent-Skills-style skill folders, each
// holding a SKILL.md with YAML frontmatter (name/description). Missing dirs
// are simply skipped.
func parseSkillsDir(dir string) []Skill {
	if !dirExists(dir) {
		return nil
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []Skill
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		skill := parseSkillDir(filepath.Join(dir, e.Name()))
		if skill != nil {
			out = append(out, *skill)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

func parseSkillDir(dir string) *Skill {
	data, err := os.ReadFile(filepath.Join(dir, "SKILL.md"))
	if err != nil {
		return nil
	}
	content := string(data)
	skill := &Skill{Name: filepath.Base(dir), Path: dir}
	if strings.HasPrefix(content, "---") {
		if end := strings.Index(content[3:], "\n---"); end != -1 {
			frontmatter := content[3 : 3+end]
			// Minimal frontmatter keys — avoid a full YAML dependency here.
			for _, line := range strings.Split(frontmatter, "\n") {
				idx := strings.Index(line, ":")
				if idx == -1 {
					continue
				}
				key := strings.TrimSpace(line[:idx])
				val := strings.Trim(strings.TrimSpace(line[idx+1:]), `"'`)
				switch key {
				case "name":
					if val != "" {
						skill.Name = val
					}
				case "description":
					skill.Description = val
				}
			}
		}
	}
	return skill
}

// addDirSkills appends the skills of one directory to the config.
func (b *builder) addDirSkills(dir string) {
	if skills := parseSkillsDir(dir); len(skills) > 0 {
		b.rc.Skills = append(b.rc.Skills, skills...)
	}
}

// ---------------------------------------------------------------------------
// Per-CLI definitions
// ---------------------------------------------------------------------------

// claude: ~/.claude/settings.json (user settings, `model`),
// ~/.claude.json (global MCP + state), ~/.claude/skills/.
func buildClaude(home string) *builder {
	b := &builder{rc: RuntimeConfig{ID: "claude-code", Name: "Claude Code", Executable: "claude", ConfigRoot: filepath.Join(home, ".claude")}}
	settingsPath := filepath.Join(home, ".claude", "settings.json")
	globalPath := filepath.Join(home, ".claude.json")
	b.addPath("config", settingsPath)
	b.addPath("mcp", globalPath)
	b.addPath("skills", filepath.Join(home, ".claude", "skills"))

	if settings, err := readJSON(settingsPath); err == nil {
		b.rc.DefaultModel = asString(settings["model"])
	} else if fileExists(settingsPath) {
		b.addError(err)
	}

	if global, err := readJSON(globalPath); err == nil {
		b.rc.MCPServers = append(b.rc.MCPServers, mcpServersFromMap(asMap(global["mcpServers"]))...)
	} else if fileExists(globalPath) {
		b.addError(err)
	}

	b.addDirSkills(filepath.Join(home, ".claude", "skills"))
	return b
}

// codex: $CODEX_HOME (default ~/.codex) config.toml — `model`,
// `[model_providers.*]`, `[mcp_servers.*]`; skills under ~/.codex/skills.
func buildCodex(home string) *builder {
	b := &builder{rc: RuntimeConfig{ID: "codex", Name: "Codex", Executable: "codex", ConfigRoot: codexConfigDir(home)}}
	dir := codexConfigDir(home)
	configPath := filepath.Join(dir, "config.toml")
	b.addPath("config", configPath)
	b.addPath("skills", filepath.Join(dir, "skills"))

	if cfg, err := readTOML(configPath); err == nil {
		b.rc.DefaultModel = asString(cfg["model"])
		for _, id := range sortedKeys(asMap(cfg["model_providers"])) {
			p := asMap(asMap(cfg["model_providers"])[id])
			b.rc.Providers = append(b.rc.Providers, ProviderInfo{
				ID:      id,
				Name:    asString(p["name"]),
				BaseURL: asString(p["base_url"]),
			})
		}
		b.rc.MCPServers = append(b.rc.MCPServers, mcpServersFromTomlMap(asMap(cfg["mcp_servers"]))...)
	} else if fileExists(configPath) {
		b.addError(err)
	}

	b.addDirSkills(filepath.Join(dir, "skills"))
	return b
}

// opencode: opencode/opencode.json(.jsonc) under XDG_CONFIG_HOME (defaulting
// to ~/.config — the same layout on macOS and Windows per the docs) — `model`,
// `provider` map (models per provider), `mcp` map; skills/agents dirs beside it.
func buildOpencode(home string) *builder {
	b := &builder{rc: RuntimeConfig{ID: "opencode", Name: "OpenCode", Executable: "opencode", ConfigRoot: opencodeConfigDir(home)}}
	dir := opencodeConfigDir(home)
	var configPath string
	for _, name := range []string{"opencode.json", "opencode.jsonc"} {
		if fileExists(filepath.Join(dir, name)) {
			configPath = filepath.Join(dir, name)
			break
		}
	}
	b.addPath("config", configPath)
	b.addPath("skills", filepath.Join(dir, "skill"))
	b.addPath("skills", filepath.Join(dir, "skills"))
	b.addPath("agents", filepath.Join(dir, "agent"))
	b.addPath("auth", filepath.Join(home, ".local", "share", "opencode", "auth.json"))

	if configPath != "" {
		if cfg, err := readJSON(configPath); err == nil {
			b.rc.DefaultModel = asString(cfg["model"])
			providers := asMap(cfg["provider"])
			for _, id := range sortedKeys(providers) {
				p := asMap(providers[id])
				info := ProviderInfo{ID: id, Name: asString(p["name"])}
				if opts := asMap(p["options"]); opts != nil {
					info.BaseURL = asString(opts["baseURL"])
				}
				if models := asMap(p["models"]); models != nil {
					info.Models = sortedKeys(models)
				}
				b.rc.Providers = append(b.rc.Providers, info)
			}
			b.rc.MCPServers = append(b.rc.MCPServers, mcpServersFromOpencode(asMap(cfg["mcp"]))...)
		} else if fileExists(configPath) {
			b.addError(err)
		}
	}

	b.addDirSkills(filepath.Join(dir, "skill"))
	b.addDirSkills(filepath.Join(dir, "skills"))
	return b
}

// gemini: ~/.gemini/settings.json (`mcpServers`, model), extensions dir.
func buildGemini(home string) *builder {
	b := &builder{rc: RuntimeConfig{ID: "gemini", Name: "Gemini CLI", Executable: "gemini", ConfigRoot: filepath.Join(home, ".gemini")}}
	settingsPath := filepath.Join(home, ".gemini", "settings.json")
	b.addPath("config", settingsPath)
	b.addPath("extensions", filepath.Join(home, ".gemini", "extensions"))
	b.addPath("skills", filepath.Join(home, ".gemini", "skills"))

	if settings, err := readJSON(settingsPath); err == nil {
		b.rc.DefaultModel = asString(settings["model"])
		b.rc.MCPServers = append(b.rc.MCPServers, mcpServersFromMap(asMap(settings["mcpServers"]))...)
	} else if fileExists(settingsPath) {
		b.addError(err)
	}

	b.addDirSkills(filepath.Join(home, ".gemini", "skills"))
	return b
}

// antigravity: ~/.gemini/antigravity-cli/settings.json (+ mcp_config.json),
// global skills in ~/.gemini/config/skills (documented for macOS & Windows).
func buildAntigravity(home string) *builder {
	b := &builder{rc: RuntimeConfig{ID: "antigravity", Name: "Antigravity CLI", Executable: "antigravity", ConfigRoot: filepath.Join(home, ".gemini", "antigravity-cli")}}
	dir := filepath.Join(home, ".gemini", "antigravity-cli")
	settingsPath := filepath.Join(dir, "settings.json")
	mcpPath := filepath.Join(dir, "mcp_config.json")
	b.addPath("config", settingsPath)
	b.addPath("mcp", mcpPath)
	b.addPath("skills", filepath.Join(home, ".gemini", "config", "skills"))
	b.addPath("skills", filepath.Join(dir, "builtin", "skills"))

	if settings, err := readJSON(settingsPath); err == nil {
		b.rc.DefaultModel = asString(settings["model"])
		b.rc.MCPServers = append(b.rc.MCPServers, mcpServersFromMap(asMap(settings["mcpServers"]))...)
	} else if fileExists(settingsPath) {
		b.addError(err)
	}
	if mcpCfg, err := readJSON(mcpPath); err == nil {
		b.rc.MCPServers = append(b.rc.MCPServers, mcpServersFromMap(asMap(mcpCfg["mcpServers"]))...)
	} else if fileExists(mcpPath) {
		b.addError(err)
	}

	b.addDirSkills(filepath.Join(home, ".gemini", "config", "skills"))
	b.addDirSkills(filepath.Join(dir, "builtin", "skills"))
	return b
}

// pi: ~/.pi/agent — skills/ + agents/, models via models.json or models.toml.
func buildPi(home string) *builder {
	b := &builder{rc: RuntimeConfig{ID: "pi", Name: "Pi", Executable: "pi", ConfigRoot: filepath.Join(home, ".pi", "agent")}}
	dir := filepath.Join(home, ".pi", "agent")
	b.addPath("config", filepath.Join(dir, "settings.json"))
	b.addPath("config", filepath.Join(dir, "models.json"))
	b.addPath("config", filepath.Join(dir, "models.toml"))
	b.addPath("skills", filepath.Join(dir, "skills"))
	b.addPath("agents", filepath.Join(dir, "agents"))

	if models, err := readJSON(filepath.Join(dir, "models.json")); err == nil {
		b.collectPiModels(models)
	} else if fileExists(filepath.Join(dir, "models.json")) {
		b.addError(err)
	}
	if models, err := readTOML(filepath.Join(dir, "models.toml")); err == nil {
		b.collectPiModels(models)
	} else if fileExists(filepath.Join(dir, "models.toml")) {
		b.addError(err)
	}
	if settings, err := readJSON(filepath.Join(dir, "settings.json")); err == nil {
		if b.rc.DefaultModel == "" {
			b.rc.DefaultModel = asString(settings["model"])
		}
		b.rc.MCPServers = append(b.rc.MCPServers, mcpServersFromMap(asMap(settings["mcpServers"]))...)
	} else if fileExists(filepath.Join(dir, "settings.json")) {
		b.addError(err)
	}

	b.addDirSkills(filepath.Join(dir, "skills"))
	return b
}

// collectPiModels reads pi's models file defensively: either a list of
// provider objects or a map of provider id → {models|baseUrl|name}.
func (b *builder) collectPiModels(v map[string]interface{}) {
	if v == nil {
		return
	}
	// Map form: { "<providerId>": { "models": [...], "baseUrl": "...", "name": "..." } }
	for _, id := range sortedKeys(v) {
		p := asMap(v[id])
		if p == nil {
			continue
		}
		info := ProviderInfo{
			ID:      id,
			Name:    asString(p["name"]),
			BaseURL: asString(p["baseUrl"]),
			Models:  asStringSlice(p["models"]),
		}
		if info.Name == "" && info.BaseURL == "" && len(info.Models) == 0 {
			continue
		}
		b.rc.Providers = append(b.rc.Providers, info)
	}
	if len(b.rc.Providers) > 0 {
		return
	}
	// Nested form: { "providers": [...] }
	if list, ok := v["providers"].([]interface{}); ok {
		for _, item := range list {
			p := asMap(item)
			if p == nil {
				continue
			}
			id := asString(p["id"])
			if id == "" {
				id = asString(p["name"])
			}
			if id == "" {
				continue
			}
			b.rc.Providers = append(b.rc.Providers, ProviderInfo{
				ID:      id,
				Name:    asString(p["name"]),
				BaseURL: asString(p["baseUrl"]),
				Models:  asStringSlice(p["models"]),
			})
		}
	}
}

// omp (OhMyPi): ~/.omp/agent — config.yml (settings, `model`), models.yml
// (providers → models[].id), skills/<name>/SKILL.md, agents/. Global MCP
// servers live in ~/.omp/mcp.json (`mcpServers` — project-level .omp/mcp.json
// is the documented per-repo location); profiles in ~/.omp/profiles.
func buildOmp(home string) *builder {
	b := &builder{rc: RuntimeConfig{ID: "ohmypi", Name: "OhMyPi", Executable: "omp", ConfigRoot: filepath.Join(home, ".omp", "agent")}}
	dir := filepath.Join(home, ".omp", "agent")
	var configPath string
	for _, name := range []string{"config.yml", "config.yaml"} {
		if fileExists(filepath.Join(dir, name)) {
			configPath = filepath.Join(dir, name)
			break
		}
	}
	var modelsPath string
	for _, name := range []string{"models.yml", "models.yaml"} {
		if fileExists(filepath.Join(dir, name)) {
			modelsPath = filepath.Join(dir, name)
			break
		}
	}
	mcpPath := filepath.Join(home, ".omp", "mcp.json")
	b.addPath("config", configPath)
	b.addPath("config", modelsPath)
	b.addPath("mcp", mcpPath)
	b.addPath("skills", filepath.Join(dir, "skills"))
	b.addPath("agents", filepath.Join(dir, "agents"))
	b.addPath("data", filepath.Join(home, ".omp", "profiles"))

	if configPath != "" {
		if cfg, err := readYAML(configPath); err == nil {
			b.rc.DefaultModel = asString(cfg["model"])
			b.rc.MCPServers = append(b.rc.MCPServers, mcpServersFromMap(asMap(cfg["mcpServers"]))...)
		} else if fileExists(configPath) {
			b.addError(err)
		}
	}

	// models.yml: { providers: { <id>: { models: [ { id: ... } ] } } }
	if modelsPath != "" {
		if models, err := readYAML(modelsPath); err == nil {
			b.collectOmpModels(models)
		} else if fileExists(modelsPath) {
			b.addError(err)
		}
	}

	// Global mcp.json (same mcpServers shape as the documented project file).
	if mcpCfg, err := readJSON(mcpPath); err == nil {
		b.rc.MCPServers = append(b.rc.MCPServers, mcpServersFromMap(asMap(mcpCfg["mcpServers"]))...)
	} else if fileExists(mcpPath) {
		b.addError(err)
	}

	b.addDirSkills(filepath.Join(dir, "skills"))
	return b
}

// collectOmpModels reads models.yml: a providers map whose entries carry a
// models list of { id } objects (the verified omp shape), with defensive
// fallbacks for a flat models list or provider-level string lists.
func (b *builder) collectOmpModels(v map[string]interface{}) {
	if v == nil {
		return
	}
	providers := asMap(v["providers"])
	for _, id := range sortedKeys(providers) {
		p := asMap(providers[id])
		if p == nil {
			continue
		}
		info := ProviderInfo{
			ID:      id,
			Name:    asString(p["name"]),
			BaseURL: asString(p["baseUrl"]),
		}
		// models: [ { id: "..." } ] — the documented omp shape.
		if list, ok := p["models"].([]interface{}); ok {
			for _, item := range list {
				switch m := item.(type) {
				case map[string]interface{}:
					if mid := asString(m["id"]); mid != "" {
						info.Models = append(info.Models, mid)
					}
				case string:
					info.Models = append(info.Models, m)
				}
			}
		}
		if info.Name == "" && info.BaseURL == "" && len(info.Models) == 0 {
			continue
		}
		b.rc.Providers = append(b.rc.Providers, info)
	}
}

// collectOmpProviders handles omp's provider config shapes: a map of
// provider id → {name/baseUrl/models/apiKey} or a `providers:` list.
func (b *builder) collectOmpProviders(cfg map[string]interface{}) {
	if m := asMap(cfg["providers"]); m != nil {
		for _, id := range sortedKeys(m) {
			p := asMap(m[id])
			info := ProviderInfo{ID: id}
			if p != nil {
				info.Name = asString(p["name"])
				info.BaseURL = asString(p["baseUrl"])
				if info.BaseURL == "" {
					info.BaseURL = asString(p["base_url"])
				}
				info.Models = asStringSlice(p["models"])
			}
			b.rc.Providers = append(b.rc.Providers, info)
		}
		return
	}
	if list, ok := cfg["providers"].([]interface{}); ok {
		for _, item := range list {
			p := asMap(item)
			if p == nil {
				continue
			}
			id := asString(p["id"])
			if id == "" {
				id = asString(p["name"])
			}
			if id == "" {
				continue
			}
			b.rc.Providers = append(b.rc.Providers, ProviderInfo{
				ID:      id,
				Name:    asString(p["name"]),
				BaseURL: asString(p["baseUrl"]),
				Models:  asStringSlice(p["models"]),
			})
		}
	}
}

// ---------------------------------------------------------------------------
// MCP extraction helpers
// ---------------------------------------------------------------------------

// mcpServersFromMap reads the common { name: { command|args|env|url } } shape
// used by Claude, Gemini, Antigravity and pi settings.
func mcpServersFromMap(m map[string]interface{}) []MCPServer {
	if m == nil {
		return nil
	}
	var out []MCPServer
	for _, name := range sortedKeys(m) {
		s := asMap(m[name])
		if s == nil {
			continue
		}
		out = append(out, MCPServer{
			Name:    name,
			Type:    mcpType(asString(s["type"]), asString(s["url"])),
			Command: mcpCommand(s),
			URL:     asString(s["url"]),
		})
	}
	return out
}

// mcpServersFromTomlMap reads codex's [mcp_servers.<name>] tables.
func mcpServersFromTomlMap(m map[string]interface{}) []MCPServer {
	if m == nil {
		return nil
	}
	var out []MCPServer
	for _, name := range sortedKeys(m) {
		s := asMap(m[name])
		if s == nil {
			continue
		}
		out = append(out, MCPServer{
			Name:    name,
			Type:    mcpType(asString(s["url_type"]), asString(s["url"])),
			Command: mcpCommand(s),
			URL:     asString(s["url"]),
		})
	}
	return out
}

// mcpServersFromOpencode reads opencode's `mcp` map: { name: { type:
// "local"|"remote", command:[...], url, enabled } }.
func mcpServersFromOpencode(m map[string]interface{}) []MCPServer {
	if m == nil {
		return nil
	}
	var out []MCPServer
	for _, name := range sortedKeys(m) {
		s := asMap(m[name])
		if s == nil {
			continue
		}
		if enabled, ok := s["enabled"].(bool); ok && !enabled {
			continue
		}
		t := asString(s["type"])
		out = append(out, MCPServer{
			Name:    name,
			Type:    mcpType(t, asString(s["url"])),
			Command: strings.Join(asStringSlice(s["command"]), " "),
			URL:     asString(s["url"]),
		})
	}
	return out
}

func mcpCommand(s map[string]interface{}) string {
	if c := asString(s["command"]); c != "" {
		if args := asStringSlice(s["args"]); len(args) > 0 {
			return strings.Join(append([]string{c}, args...), " ")
		}
		return c
	}
	return ""
}

func mcpType(t, url string) string {
	if strings.EqualFold(t, "remote") || (t == "" && url != "") {
		return "remote"
	}
	return "local"
}
