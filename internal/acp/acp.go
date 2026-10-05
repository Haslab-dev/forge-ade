// Package acp implements a client for the Agent Client Protocol (ACP) v1 —
// JSON-RPC 2.0 over newline-delimited stdio — letting ForgeADE spawn and talk
// to external ACP agent subprocesses (initialize → session/new →
// session/prompt with streamed session/update notifications and
// session/request_permission callbacks).
package acp

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/hasdev/forge-ade/internal/agent"
	"github.com/hasdev/forge-ade/internal/events"
	"gopkg.in/yaml.v3"
)

const protocolVersion = 1

// AgentConfig describes one external ACP agent (a subprocess speaking ACP).
type AgentConfig struct {
	ID               string            `json:"id"`
	Name             string            `json:"name"`
	Command          string            `json:"command"`
	Args             []string          `json:"args,omitempty"`
	Env              map[string]string `json:"env,omitempty"`
	Enabled          bool              `json:"enabled"`
	SupportedModels  []string          `json:"supported_models,omitempty"`
	DefaultModel     string            `json:"default_model,omitempty"`
	SupportedEfforts []string          `json:"supported_efforts,omitempty"`
	DefaultEffort    string            `json:"default_effort,omitempty"`
}

// PermissionOption is one choice offered by a provider permission request.
type PermissionOption struct {
	OptionID string `json:"optionId"`
	Name     string `json:"name"`
	Kind     string `json:"kind"` // allow_once | allow_always | reject_once | reject_always
}

// ACPSession is one conversation with an external agent. One live provider
// process backs one session (the kybern-drivers shape): driver protocol per
// provider, not shared ACP. The JSON shape mirrors internal/agent.Session
// fields used by the chat UI (id, name, state, messages with the same
// ContentBlock tags) so the same view renders both.
type ACPSession struct {
	ID                string               `json:"id"`
	AgentID           string               `json:"agent_id"`
	AgentName         string               `json:"agent_name"`
	Name              string               `json:"name"`
	State             string               `json:"state"` // idle | thinking | executing | awaiting_approval
	Folder            string               `json:"folder"`
	Messages          []agent.AgentMessage `json:"messages"`
	AutoApprove       bool                 `json:"auto_approve"`
	PendingPermission *PendingPermission   `json:"pending_permission,omitempty"`
	CreatedAt         time.Time            `json:"created_at"`
	UpdatedAt         time.Time            `json:"updated_at"`

	ProviderID string `json:"provider_id,omitempty"` // provider-side session/thread id
	// live connection; never serialized.
	sess         AgentSession
	sink         *sessionSink
	promptCancel context.CancelFunc
}

// PendingPermission carries the provider request id as a string (every
// kybern-shaped driver keys approvals by string request id).
type PendingPermission struct {
	RequestIDStr string             `json:"-"`
	ToolCall     map[string]any     `json:"toolCall,omitempty"`
	Options      []PermissionOption `json:"options"`
}

// Manager owns agent configs and running sessions. Each session owns its own
// provider process connection.
type Manager struct {
	mu          sync.RWMutex
	configs     map[string]*AgentConfig
	sessions    map[string]*ACPSession
	drivers     *Registry
	dataDir     string
	bus         *events.Bus
	mcpMgr      MCPLister // optional: powers /mcp:* slash entries
	nextMsgID   uint64
	nextSessNum int
}

func NewManager(dataDir string, bus *events.Bus) *Manager {
	m := &Manager{
		configs:  map[string]*AgentConfig{},
		sessions: map[string]*ACPSession{},
		drivers:  NewRegistry(),
		dataDir:  dataDir,
		bus:      bus,
	}
	m.loadConfigs()
	m.loadSessions()
	return m
}

// ---------------------------------------------------------------------------
// Config persistence
// ---------------------------------------------------------------------------

func (m *Manager) configsPath() string {
	return filepath.Join(m.dataDir, "acp_agents.json")
}

func (m *Manager) seedDefaultsIfMissing() {
	defaults := []*AgentConfig{
		{
			ID:               "agent-claude",
			Name:             "Claude Code",
			Command:          "claude",
			Args:             []string{},
			Enabled:          true,
			SupportedModels:  []string{"deepseek-v4-1-flash", "claude-3-7-sonnet-20250219", "claude-3-5-sonnet-20241022", "claude-3-5-haiku-20241022", "claude-3-opus-20240229"},
			DefaultModel:     "deepseek-v4-1-flash",
			SupportedEfforts: []string{"low", "medium", "high", "xhigh"},
			DefaultEffort:    "high",
		},
		{
			ID:               "agent-codex",
			Name:             "Codex",
			Command:          "codex",
			Args:             []string{"app-server"},
			Enabled:          true,
			SupportedModels:  []string{"glm-5-3-flash", "gpt-5.6-sol", "gpt-5.5", "o3", "o3-mini", "gpt-4o"},
			DefaultModel:     "glm-5-3-flash",
			SupportedEfforts: []string{"low", "medium", "high", "max"},
			DefaultEffort:    "medium",
		},
		{
			ID:               "agent-opencode",
			Name:             "OpenCode",
			Command:          "opencode",
			Args:             []string{},
			Enabled:          true,
			SupportedModels:  []string{"myairouter_cloud/kenari/glm-5-3-flash", "deepseek/deepseek-flash", "qwen2.5-coder:latest", "deepseek-r1:latest", "claude-3-7-sonnet", "gpt-4o"},
			DefaultModel:     "myairouter_cloud/kenari/glm-5-3-flash",
			SupportedEfforts: []string{"low", "medium", "high"},
			DefaultEffort:    "medium",
		},
		{
			ID:               "agent-pi",
			Name:             "Pi Agent",
			Command:          "pi-acp",
			Args:             []string{},
			Enabled:          true,
			SupportedModels:  []string{"myairouter_cloud/kenari/glm-5-3-flash", "sumopod/gpt-5.6-luna", "cerebras/qwen-3.8-27b", "kenari/glm-5-3-flash"},
			DefaultModel:     "myairouter_cloud/kenari/glm-5-3-flash",
			SupportedEfforts: []string{"off", "minimal", "low", "medium", "high"},
			DefaultEffort:    "medium",
		},
		{
			ID:               "agent-ohmypi",
			Name:             "Oh My Pi (omp)",
			Command:          "omp",
			Args:             []string{},
			Enabled:          true,
			SupportedModels:  []string{"glm-coding/glm-5.3-flash", "sumopod/gpt-5.6-luna", "ina/inadigital-flash", "ina/inadigital-pro"},
			DefaultModel:     "glm-coding/glm-5.3-flash",
			SupportedEfforts: []string{"off", "minimal", "low", "medium", "high"},
			DefaultEffort:    "medium",
		},
		{
			ID:               "agent-cursor",
			Name:             "Cursor CLI",
			Command:          "agent",
			Args:             []string{"acp"},
			Enabled:          true,
			SupportedModels:  []string{"auto", "gpt-5.4-high"},
			SupportedEfforts: []string{"low", "medium", "high"},
			DefaultEffort:    "medium",
		},
	}

	changed := false
	for _, def := range defaults {
		if existing, ok := m.configs[def.ID]; !ok {
			m.configs[def.ID] = def
			changed = true
		} else {
			// Native drivers build their own command line; clear legacy ACP
			// flags and fix wrong binaries so every harness spawns correctly.
			if def.ID == "agent-pi" {
				if existing.Command == "npx" || existing.Command == "pi-acp" ||
					(existing.Command == "pi" && len(existing.Args) > 0 && existing.Args[0] == "--mode") {
					existing.Command = "pi-acp" // driver maps the wrapper to the pi RPC binary
					existing.Args = []string{}
					changed = true
				}
			}
			if def.ID == "agent-ohmypi" && (existing.Command != "omp" || len(existing.Args) > 0) {
				existing.Command = "omp"
				existing.Args = []string{}
				changed = true
			}
			if def.ID == "agent-opencode" && len(existing.Args) > 0 {
				existing.Command = "opencode"
				existing.Args = []string{}
				changed = true
			}
		}
	}
	if changed {
		m.saveConfigsLocked()
	}
}

func (m *Manager) findSessionLocked(sessionID string) *ACPSession {
	if s, ok := m.sessions[sessionID]; ok {
		return s
	}
	for _, s := range m.sessions {
		if s.ProviderID == sessionID {
			return s
		}
	}
	return nil
}

func (m *Manager) loadConfigs() {
	data, err := os.ReadFile(m.configsPath())
	if err != nil {
		m.seedDefaultsIfMissing()
		return
	}
	var cfgs []*AgentConfig
	if err := json.Unmarshal(data, &cfgs); err != nil {
		m.seedDefaultsIfMissing()
		return
	}
	for _, c := range cfgs {
		m.configs[c.ID] = c
	}
	m.seedDefaultsIfMissing()
}

func (m *Manager) saveConfigsLocked() {
	out := make([]*AgentConfig, 0, len(m.configs))
	for _, c := range m.configs {
		out = append(out, c)
	}
	data, err := json.MarshalIndent(out, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(m.configsPath(), data, 0644)
}

// CheckBinary tests whether a command binary exists in $PATH or user local paths.
func (m *Manager) CheckBinary(command string) (bool, string) {
	if command == "" {
		return false, "Command is empty"
	}
	if command == "pi" || command == "pi-acp" || command == "agent-pi" {
		if resolved := resolveBinary("pi-acp"); resolved != "pi-acp" {
			if _, err := os.Stat(resolved); err == nil {
				return true, resolved
			}
		}
		if resolved := resolveBinary("pi"); resolved != "pi" {
			if _, err := os.Stat(resolved); err == nil {
				return true, resolved
			}
		}
		if resolved := resolveBinary("npx"); resolved != "npx" {
			if _, err := os.Stat(resolved); err == nil {
				return true, resolved
			}
		}
	}
	resolved := resolveBinary(command)
	if _, err := os.Stat(resolved); err == nil {
		return true, resolved
	}
	path, err := exec.LookPath(command)
	if err != nil {
		return false, fmt.Sprintf("%q not found in PATH", command)
	}
	return true, path
}

// ToggleAgent toggles an agent's enabled state.
func (m *Manager) ToggleAgent(id string, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	cfg, ok := m.configs[id]
	if !ok {
		return fmt.Errorf("agent %q not found", id)
	}
	cfg.Enabled = enabled
	m.saveConfigsLocked()
	return nil
}

// ListAgents returns all configured ACP agents.
func (m *Manager) ListAgents() []AgentConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]AgentConfig, 0, len(m.configs))
	for _, c := range m.configs {
		out = append(out, *c)
	}
	return out
}

// SaveAgent creates or updates an agent config.
func (m *Manager) SaveAgent(cfg AgentConfig) (AgentConfig, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cfg.Name == "" || cfg.Command == "" {
		return cfg, fmt.Errorf("name and command are required")
	}
	if cfg.ID == "" {
		cfg.ID = fmt.Sprintf("acp-%d", time.Now().UnixNano())
	}
	m.configs[cfg.ID] = &cfg
	m.saveConfigsLocked()
	return cfg, nil
}

// DeleteAgent removes a config, stopping any live sessions for it.
func (m *Manager) DeleteAgent(id string) error {
	m.mu.Lock()
	var toClose []AgentSession
	for _, s := range m.sessions {
		if s.AgentID == id && s.sess != nil {
			toClose = append(toClose, s.sess)
		}
	}
	delete(m.configs, id)
	m.saveConfigsLocked()
	m.mu.Unlock()
	for _, sess := range toClose {
		_ = sess.Close()
	}
	return nil
}

// SlashCommandItem represents one available slash command or skill.
type SlashCommandItem struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"` // "command" | "skill"
}

// GetAgentModels returns the available models for a given ACP agent by inspecting local config files
// (~/.omp/agent/models.yml, ~/.pi/agent/models.json, ~/.codex/config.toml, ~/.codex/models_cache.json,
// ~/.claude/settings.json, ~/.config/opencode/opencode.jsonc).
func (m *Manager) GetAgentModels(agentID string) []string {
	m.mu.RLock()
	cfg, ok := m.configs[agentID]
	var cmd string
	if ok {
		cmd = cfg.Command
	}
	m.mu.RUnlock()

	detected := DetectLocalAgentModels(agentID, cmd)
	if len(detected) > 0 && ok {
		m.mu.Lock()
		cfg.SupportedModels = detected
		m.saveConfigsLocked()
		m.mu.Unlock()
		return detected
	}

	m.mu.RLock()
	if ok && len(cfg.SupportedModels) > 0 {
		cached := cfg.SupportedModels
		m.mu.RUnlock()
		return cached
	}
	m.mu.RUnlock()

	return detected
}

// stripJsonComments removes single-line // comments from JSONC text while preserving string literals.
func stripJsonComments(input string) string {
	var sb strings.Builder
	lines := strings.Split(input, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "//") {
			continue
		}
		// Also clean trailing comma before closing brace/bracket if any
		sb.WriteString(line)
		sb.WriteString("\n")
	}
	res := sb.String()
	// Replace trailing commas (whitespace/newline tolerant) before } or ]
	trailingComma := regexp.MustCompile(`,\s*([}\]])`)
	return trailingComma.ReplaceAllString(res, "$1")
}

// DetectLocalAgentModels returns the models each harness can really run,
// read from that harness's own configuration (verified live against each
// CLI): pi ~/.pi/agent/models.json, omp ~/.omp/agent/models.yml, claude
// ~/.claude/settings.json env aliases, codex ~/.codex/{config.toml,
// models_cache.json}, opencode ~/.config/opencode/opencode.json(c).
func DetectLocalAgentModels(agentID, command string) []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	var models []string
	seen := map[string]bool{}
	add := func(id string) {
		id = strings.TrimSpace(id)
		if id != "" && !seen[id] {
			seen[id] = true
			models = append(models, id)
		}
	}

	isPi := strings.Contains(agentID, "pi") && !strings.Contains(agentID, "ohmypi") || command == "pi"
	isOmp := strings.Contains(agentID, "ohmypi") || strings.Contains(agentID, "omp") || command == "omp"

	// 1. Pi (~/.pi/agent/models.json): providers → models[].id
	if isPi {
		if data, err := os.ReadFile(filepath.Join(home, ".pi", "agent", "models.json")); err == nil {
			var cfg struct {
				Providers map[string]struct {
					Models []struct {
						ID string `json:"id"`
					} `json:"models"`
				} `json:"providers"`
			}
			if json.Unmarshal(data, &cfg) == nil {
				for prov, pv := range cfg.Providers {
					for _, m := range pv.Models {
						if m.ID != "" {
							add(prov + "/" + m.ID)
						}
					}
				}
			}
		}
	}

	// 2. Oh My Pi (~/.omp/agent/models.yml): same shape, YAML.
	if isOmp {
		if data, err := os.ReadFile(filepath.Join(home, ".omp", "agent", "models.yml")); err == nil {
			var cfg struct {
				Providers map[string]struct {
					Models []struct {
						ID string `yaml:"id"`
					} `yaml:"models"`
				} `yaml:"providers"`
			}
			if yaml.Unmarshal(data, &cfg) == nil {
				for prov, pv := range cfg.Providers {
					for _, m := range pv.Models {
						if m.ID != "" {
							add(prov + "/" + m.ID)
						}
					}
				}
			}
		}
	}

	// 3. Claude Code (~/.claude/settings.json env aliases are the real
	// routing: everything maps through ANTHROPIC_BASE_URL).
	if strings.Contains(agentID, "claude") || command == "claude" {
		if data, err := os.ReadFile(filepath.Join(home, ".claude", "settings.json")); err == nil {
			var st struct {
				Env map[string]string `json:"env"`
			}
			if json.Unmarshal(data, &st) == nil {
				for _, key := range []string{"ANTHROPIC_MODEL", "ANTHROPIC_DEFAULT_SONNET_MODEL", "ANTHROPIC_DEFAULT_OPUS_MODEL", "ANTHROPIC_DEFAULT_HAIKU_MODEL"} {
					add(st.Env[key])
				}
			}
		}
	}

	// 4. Codex (~/.codex/config.toml model + models_cache.json slugs; bare ids).
	if strings.Contains(agentID, "codex") || command == "codex" {
		if data, err := os.ReadFile(filepath.Join(home, ".codex", "config.toml")); err == nil {
			for _, l := range strings.Split(string(data), "\n") {
				trimmed := strings.TrimSpace(l)
				if strings.HasPrefix(trimmed, "model ") || strings.HasPrefix(trimmed, "model =") {
					if _, val, ok := strings.Cut(trimmed, "="); ok {
						add(strings.Trim(strings.TrimSpace(val), `"'`))
					}
				}
			}
		}
		if data, err := os.ReadFile(filepath.Join(home, ".codex", "models_cache.json")); err == nil {
			var c struct {
				Models []struct {
					Slug string `json:"slug"`
				} `json:"models"`
			}
			if json.Unmarshal(data, &c) == nil {
				for _, m := range c.Models {
					if m.Slug != "" && m.Slug != "codex-auto-review" {
						add(m.Slug)
					}
				}
			}
		}
	}

	// 5. OpenCode (~/.config/opencode/opencode.jsonc|.json): provider.models.
	if strings.Contains(agentID, "opencode") || command == "opencode" {
		for _, name := range []string{"opencode.jsonc", "opencode.json"} {
			data, err := os.ReadFile(filepath.Join(home, ".config", "opencode", name))
			if err != nil {
				continue
			}
			cleaned := stripJsonComments(string(data))
			var cfg struct {
				Provider map[string]struct {
					Models map[string]struct {
						Name string `json:"name"`
					} `json:"models"`
				} `json:"provider"`
			}
			if json.Unmarshal([]byte(cleaned), &cfg) == nil {
				for prov, pv := range cfg.Provider {
					for mid := range pv.Models {
						add(prov + "/" + mid)
					}
				}
			}
		}
	}

	return models
}

func (m *Manager) GetSlashCommandsAndSkills(agentID string) []SlashCommandItem {
	var items []SlashCommandItem
	seen := make(map[string]bool)

	addItem := func(name, desc, category string) {
		if !seen[name] {
			seen[name] = true
			items = append(items, SlashCommandItem{Name: name, Description: desc, Category: category})
		}
	}

	// Standard agent slash commands
	addItem("/plan", "Create an implementation plan before writing code", "command")
	addItem("/review", "Review code changes, recent edits, or staged diffs", "command")
	addItem("/commit", "Generate conventional commit message and commit changes", "command")
	addItem("/compact", "Compress and summarize current conversation context", "command")
	addItem("/skill", "Invoke a specialized agent skill (/skill <name>)", "command")
	addItem("/help", "Show all available agent capabilities and commands", "command")
	addItem("/reset", "Clear conversation history and reset agent session", "command")
	addItem("/gsd:plan", "Run GSD planning workflow", "command")
	addItem("/gsd:execute", "Run GSD autonomous task execution", "command")
	addItem("/gsd:status", "Inspect active GSD task status", "command")

	// Discovered skills from ~/.omp, ~/.pi, ~/.forge-ade
	home, err := os.UserHomeDir()
	if err == nil {
		skillDirs := []string{
			filepath.Join(home, ".omp", "plugins", "node_modules", "mattpocock-skills", "skills"),
			filepath.Join(home, ".omp", "agent", "skills"),
			filepath.Join(home, ".pi", "agent", "skills"),
			filepath.Join(home, ".forge-ade", "skills"),
		}
		for _, base := range skillDirs {
			_ = filepath.Walk(base, func(path string, info os.FileInfo, err error) error {
				if err != nil || info == nil {
					return nil
				}
				if info.Name() == "SKILL.md" {
					dir := filepath.Dir(path)
					skillName := filepath.Base(dir)
					desc := fmt.Sprintf("Run %s skill", skillName)
					if data, err := os.ReadFile(path); err == nil {
						lines := strings.Split(string(data), "\n")
						for _, l := range lines {
							trimmed := strings.TrimSpace(l)
							if strings.HasPrefix(trimmed, "description:") {
								desc = strings.TrimSpace(strings.TrimPrefix(trimmed, "description:"))
								break
							}
						}
					}
					addItem(fmt.Sprintf("/skill:%s", skillName), desc, "skill")
				}
				return nil
			})
		}
	}

	// MCP servers (mention to direct the agent at a server) and their tools.
	if m.mcpMgr != nil {
		for _, srv := range m.mcpMgr.ListServers() {
			addItem(fmt.Sprintf("/mcp:%s", srv.Name), fmt.Sprintf("MCP server: %s", srv.Name), "mcp")
		}
		for _, tool := range m.mcpMgr.ListTools() {
			desc := tool.Description
			if desc == "" {
				desc = fmt.Sprintf("MCP tool on server %s", tool.ServerName)
			}
			addItem(fmt.Sprintf("/mcp:%s:%s", tool.ServerName, tool.Name), desc, "mcp-tool")
		}
	}

	return items
}

// ---------------------------------------------------------------------------
// Sessions
// ---------------------------------------------------------------------------

// ListSessions returns all live ACP sessions.
func (m *Manager) ListSessions() []ACPSession {
	m.mu.RLock()
	defer m.mu.RUnlock()
	out := make([]ACPSession, 0, len(m.sessions))
	for _, s := range m.sessions {
		out = append(out, *s)
	}
	return out
}

// GetSession returns one session (copy).
func (m *Manager) GetSession(id string) (ACPSession, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	s, ok := m.sessions[id]
	if !ok {
		return ACPSession{}, false
	}
	return *s, true
}

// spawnSession spawns one provider process for a new session and starts its
// event drain. This replaces the shared-connection ensureConn: one process per
// session, speaking the provider's own protocol (kybern-drivers shape).
func (m *Manager) spawnSession(ctx context.Context, cfg *AgentConfig, folder string) (AgentSession, <-chan DriverEvent, <-chan struct{}, error) {
	driver := m.drivers.DriverFor(cfg)
	mode := ModeAcceptEdits
	if cfg.ID == "agent-codex" || cfg.Command == "codex" {
		mode = ModeAuto
	}
	sessionCfg := SessionConfig{
		CWD:            folder,
		Binary:         cfg.Command,
		Env:            cfg.Env,
		Model:          cfg.DefaultModel,
		Effort:         cfg.DefaultEffort,
		PermissionMode: mode,
	}
	if driver.ID() == "codex" && sessionCfg.Model != "" {
		// Codex wants bare slugs; provider routing lives in config.toml.
		if i := strings.LastIndexByte(sessionCfg.Model, '/'); i >= 0 {
			sessionCfg.Model = sessionCfg.Model[i+1:]
		}
	}
	sess, evs, err := driver.Spawn(ctx, sessionCfg)
	if err != nil {
		return nil, nil, nil, err
	}
	return sess, evs, sess.Done(), nil
}

// CreateSession spawns the agent process and opens a new session with its driver.
func (m *Manager) CreateSession(ctx context.Context, agentID, name, folder string) (ACPSession, error) {
	m.mu.RLock()
	cfg := m.configs[agentID]
	var cfgCopy AgentConfig
	if cfg != nil {
		cfgCopy = *cfg
	}
	m.mu.RUnlock()
	if cfg == nil {
		return ACPSession{}, fmt.Errorf("unknown acp agent %q", agentID)
	}
	if folder == "" {
		if f, err := os.Getwd(); err == nil {
			folder = f
		}
	}

	sess, evs, done, err := m.spawnSession(ctx, &cfgCopy, folder)
	if err != nil {
		return ACPSession{}, err
	}

	m.mu.Lock()
	m.nextSessNum++
	s := &ACPSession{
		ID:          fmt.Sprintf("acps-%d-%d", time.Now().UnixNano(), m.nextSessNum),
		AgentID:     agentID,
		AgentName:   cfgCopy.Name,
		Name:        name,
		State:       "idle",
		Folder:      folder,
		AutoApprove: true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		sess:        sess,
	}
	if s.Name == "" {
		s.Name = cfgCopy.Name
	}
	m.sessions[s.ID] = s
	m.mu.Unlock()

	sink := &sessionSink{m: m, sessionID: s.ID}
	m.mu.Lock()
	s.sink = sink
	m.saveSessionsLocked()
	m.mu.Unlock()

	go sink.drain(evs, done)
	m.emitUpdate(s.ID)
	return *s, nil
}

// SetSessionModel sets the active model on the session's provider connection.
func (m *Manager) SetSessionModel(ctx context.Context, sessionID, model string) error {
	if model == "" {
		return nil
	}
	m.mu.RLock()
	s := m.findSessionLocked(sessionID)
	if s == nil || s.sess == nil {
		m.mu.RUnlock()
		return fmt.Errorf("unknown session %q", sessionID)
	}
	sess := s.sess
	m.mu.RUnlock()
	return sess.SetModel(ctx, model)
}

// sessionRecord is the persisted projection of an ACPSession: enough to
// recreate the provider session (with its transcript) after an app restart.
type sessionRecord struct {
	ID         string    `json:"id"`
	AgentID    string    `json:"agent_id"`
	AgentName  string    `json:"agent_name"`
	Name       string    `json:"name"`
	Folder     string    `json:"folder"`
	ProviderID string    `json:"provider_id"`
	CreatedAt  time.Time `json:"created_at"`
}

func (m *Manager) sessionsPath() string {
	return filepath.Join(m.dataDir, "acp_sessions.json")
}

func (m *Manager) saveSessionsLocked() {
	records := make([]sessionRecord, 0, len(m.sessions))
	for _, s := range m.sessions {
		records = append(records, sessionRecord{
			ID: s.ID, AgentID: s.AgentID, AgentName: s.AgentName,
			Name: s.Name, Folder: s.Folder, ProviderID: s.ProviderID,
			CreatedAt: s.CreatedAt,
		})
	}
	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(m.sessionsPath(), data, 0644)
}

// loadSessions restores persisted session shells. Provider connections are
// re-spawned lazily by ensureSessionLive on the next Send.
func (m *Manager) loadSessions() {
	data, err := os.ReadFile(m.sessionsPath())
	if err != nil {
		return
	}
	var records []sessionRecord
	if json.Unmarshal(data, &records) != nil {
		return
	}
	for _, r := range records {
		if _, exists := m.sessions[r.ID]; exists {
			continue
		}
		m.sessions[r.ID] = &ACPSession{
			ID: r.ID, AgentID: r.AgentID, AgentName: r.AgentName,
			Name: r.Name, Folder: r.Folder, ProviderID: r.ProviderID,
			CreatedAt: r.CreatedAt, UpdatedAt: time.Now(),
		}
	}
}

// ensureSessionLive re-spawns the provider connection for a persisted session,
// resuming the same provider-side conversation where the driver supports it.
func (m *Manager) ensureSessionLive(ctx context.Context, s *ACPSession) error {
	m.mu.Lock()
	if s.sess != nil {
		m.mu.Unlock()
		return nil
	}
	cfg, ok := m.configs[s.AgentID]
	var cfgCopy AgentConfig
	if ok {
		cfgCopy = *cfg
	}
	m.mu.Unlock()
	if !ok {
		return fmt.Errorf("unknown agent %q for session %s", s.AgentID, s.ID)
	}

	driver := m.drivers.DriverFor(&cfgCopy)
	mode := ModeAcceptEdits
	if cfgCopy.ID == "agent-codex" || cfgCopy.Command == "codex" {
		mode = ModeAuto
	}
	sessionCfg := SessionConfig{
		CWD:             s.Folder,
		Binary:          cfgCopy.Command,
		Env:             cfgCopy.Env,
		Model:           cfgCopy.DefaultModel,
		Effort:          cfgCopy.DefaultEffort,
		PermissionMode:  mode,
		ResumeSessionID: s.ProviderID, // continue the same provider conversation
	}
	sess, evs, err := driver.Spawn(ctx, sessionCfg)
	if err != nil && sessionCfg.ResumeSessionID != "" {
		// Provider no longer knows this session (e.g. it never completed a
		// turn, or its transcript was pruned). Fall back to a fresh one.
		sessionCfg.ResumeSessionID = ""
		sess, evs, err = driver.Spawn(ctx, sessionCfg)
	}
	if err != nil {
		return err
	}
	sink := &sessionSink{m: m, sessionID: s.ID}
	m.mu.Lock()
	s.sess = sess
	s.sink = sink
	m.mu.Unlock()
	go sink.drain(evs, sess.Done())
	return nil
}

// Send appends a user message and runs one prompt turn. It returns when the
// agent finishes the turn (turn-completed event observed by the sink) or the
// context is cancelled.
func (m *Manager) Send(ctx context.Context, sessionID, message string, mentionedFiles []string) error {
	m.mu.RLock()
	s := m.findSessionLocked(sessionID)
	m.mu.RUnlock()
	if s == nil {
		return fmt.Errorf("unknown acp session %q", sessionID)
	}
	sessionID = s.ID

	// A persisted session from a previous app run has no live process yet.
	if s.sess == nil {
		if err := m.ensureSessionLive(ctx, s); err != nil {
			return fmt.Errorf("resume session: %w", err)
		}
	}

	// Compose prompt text with mentioned file contents appended; folders are
	// listed so the agent can request specific files from them.
	text := message
	for _, f := range mentionedFiles {
		info, err := os.Stat(f)
		if err != nil {
			text += fmt.Sprintf("\n\n[File: %s (not found)]", f)
			continue
		}
		if info.IsDir() {
			entries, err := os.ReadDir(f)
			if err != nil {
				text += fmt.Sprintf("\n\n[Folder: %s (unreadable: %v)]", f, err)
				continue
			}
			text += fmt.Sprintf("\n\n[Folder: %s]", f)
			for i, e := range entries {
				if i >= 100 {
					text += "\n  … (truncated)"
					break
				}
				kind := "file"
				if e.IsDir() {
					kind = "dir"
				}
				text += fmt.Sprintf("\n  %s [%s]", e.Name(), kind)
			}
			continue
		}
		if data, err := os.ReadFile(f); err == nil {
			text += fmt.Sprintf("\n\n[File: %s]\n%s", f, string(data))
		} else {
			text += fmt.Sprintf("\n\n[File: %s (unreadable)]", f)
		}
	}

	m.mu.Lock()
	if s.sess == nil {
		m.mu.Unlock()
		return fmt.Errorf("session %s has no live provider connection", sessionID)
	}
	s.Messages = append(s.Messages, agent.AgentMessage{
		ID:        fmt.Sprintf("m-%d", time.Now().UnixNano()),
		Role:      "user",
		Content:   []agent.ContentBlock{{Type: "text", Text: message}},
		Timestamp: time.Now(),
	})
	s.State = "thinking"
	s.UpdatedAt = time.Now()
	assistant := &agent.AgentMessage{
		ID:        fmt.Sprintf("m-%d", time.Now().UnixNano()),
		Role:      "assistant",
		Content:   []agent.ContentBlock{},
		Timestamp: time.Now(),
	}
	s.Messages = append(s.Messages, *assistant)
	sess := s.sess
	m.mu.Unlock()

	turnCtx, cancel := context.WithCancel(ctx)
	m.mu.Lock()
	s.promptCancel = cancel
	m.mu.Unlock()
	defer cancel()

	m.emitUpdate(sessionID)
	m.emitEvent(events.AgentTurnStart, sessionID, nil)

	if s.sink != nil {
		s.sink.beginTurn(assistant.ID)
	}
	if err := sess.SendMessage(turnCtx, assistant.ID, text); err != nil && turnCtx.Err() == nil {
		m.finishTurn(sessionID, assistant.ID, "error: "+err.Error(), true)
		return err
	}

	// Contract: Send returns when the turn ends (UI unsubscribes its event
	// listeners right after). Drivers ack `prompt` before the run finishes,
	// so wait for the sink to observe TurnCompleted/TurnFailed/Exited.
	if s.sink != nil {
		if err := s.sink.waitTurnDone(turnCtx); err != nil && turnCtx.Err() == nil {
			return err
		}
	}
	return nil
}

// Cancel interrupts the running prompt turn.
func (m *Manager) Cancel(sessionID string) {
	m.mu.RLock()
	s := m.findSessionLocked(sessionID)
	if s == nil {
		m.mu.RUnlock()
		return
	}
	sessionID = s.ID
	if s.promptCancel != nil {
		s.promptCancel()
	}
	sess := s.sess
	m.mu.RUnlock()

	if sess != nil {
		_ = sess.Interrupt(context.Background())
	}
	m.mu.Lock()
	if s := m.findSessionLocked(sessionID); s != nil && s.State != "idle" {
		s.State = "idle"
		s.UpdatedAt = time.Now()
	}
	m.mu.Unlock()
	m.emitUpdate(sessionID)
}

// SetAutoApprove toggles auto-approval of permission requests.
func (m *Manager) SetAutoApprove(sessionID string, enabled bool) {
	m.mu.Lock()
	if s := m.findSessionLocked(sessionID); s != nil {
		sessionID = s.ID
		s.AutoApprove = enabled
		s.UpdatedAt = time.Now()
	}
	m.mu.Unlock()
	m.emitUpdate(sessionID)
}

// RespondPermission answers a pending provider permission request.
// With optionID empty and cancel=false, the first allow option is chosen.
func (m *Manager) RespondPermission(sessionID, optionID string, cancel bool) error {
	m.mu.Lock()
	s := m.findSessionLocked(sessionID)
	if s == nil || s.PendingPermission == nil {
		m.mu.Unlock()
		return fmt.Errorf("no pending permission for session %q", sessionID)
	}
	sessionID = s.ID
	reqID := s.PendingPermission.RequestIDStr
	decision := ""
	if cancel {
		decision = "cancel"
	} else if optionID != "" {
		switch optionID {
		case "allow":
			decision = "allow"
		case "deny", "reject":
			decision = "deny"
		default:
			decision = optionID
		}
	} else {
		for _, o := range s.PendingPermission.Options {
			if strings.HasPrefix(o.Kind, "allow") {
				decision = "allow"
				break
			}
		}
		if decision == "" {
			decision = "deny"
		}
	}
	sess := s.sess
	s.PendingPermission = nil
	if s.State == "awaiting_approval" {
		s.State = "executing"
	}
	s.UpdatedAt = time.Now()
	m.mu.Unlock()

	var err error
	if sess != nil {
		err = sess.RespondPermission(context.Background(), reqID, decision)
	}
	m.emitUpdate(sessionID)
	return err
}

// StopAll closes every live session's provider process.
func (m *Manager) StopAll() {
	m.mu.Lock()
	var toClose []AgentSession
	for _, s := range m.sessions {
		if s.sess != nil {
			toClose = append(toClose, s.sess)
		}
	}
	m.mu.Unlock()
	for _, sess := range toClose {
		_ = sess.Close()
	}
}

// finishTurn marks the turn complete and appends a trailing error block when
// the turn ended abnormally.
func (m *Manager) finishTurn(sessionID, assistantMsgID, errMsg string, isErr bool) {
	m.mu.Lock()
	if s, ok := m.sessions[sessionID]; ok {
		for i := range s.Messages {
			if s.Messages[i].ID == assistantMsgID && isErr && errMsg != "" {
				s.Messages[i].Content = append(s.Messages[i].Content, agent.ContentBlock{
					Type: "text", Text: "\n\n" + errMsg,
				})
			}
		}
		s.State = "idle"
		s.UpdatedAt = time.Now()
	}
	m.mu.Unlock()
	m.emitEvent(events.AgentTurnEnd, sessionID, map[string]interface{}{"error": isErr})
	m.emitUpdate(sessionID)
}

func (m *Manager) emitUpdate(sessionID string) {
	if m.bus != nil {
		m.bus.Publish(events.Event{Type: "agent:updated", Data: map[string]interface{}{"session_id": sessionID}})
	}
}

func (m *Manager) emitEvent(evType events.EventType, sessionID string, extra map[string]interface{}) {
	if m.bus == nil {
		return
	}
	data := map[string]interface{}{"session_id": sessionID}
	for k, v := range extra {
		data[k] = v
	}
	m.bus.Publish(events.Event{Type: evType, Data: data})
}

// MCPLister is the subset of the MCP manager the slash catalog needs.
type MCPLister interface {
	ListServers() []MCPServerInfo
	ListTools() []MCPToolInfo
}

// MCPServerInfo mirrors mcp.ServerConfig fields used here.
type MCPServerInfo struct {
	Name string
}

// MCPToolInfo mirrors mcp.Tool fields used here.
type MCPToolInfo struct {
	ServerName  string
	Name        string
	Description string
}

// SetMCPManager attaches the MCP manager (called from app wiring).
func (m *Manager) SetMCPManager(l MCPLister) { m.mcpMgr = l }
