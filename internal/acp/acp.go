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
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	Command         string            `json:"command"`
	Args            []string          `json:"args,omitempty"`
	Env             map[string]string `json:"env,omitempty"`
	Enabled         bool              `json:"enabled"`
	SupportedModels []string          `json:"supported_models,omitempty"`
	DefaultModel    string            `json:"default_model,omitempty"`
}

// PermissionOption is one choice offered by session/request_permission.
type PermissionOption struct {
	OptionID string `json:"optionId"`
	Name     string `json:"name"`
	Kind     string `json:"kind"` // allow_once | allow_always | reject_once | reject_always
}

// PendingPermission is an unanswered session/request_permission request.
type PendingPermission struct {
	RequestID uint64             `json:"-"`
	ToolCall  map[string]any     `json:"toolCall,omitempty"`
	Options   []PermissionOption `json:"options"`
}

// ACPSession is one conversation with an external ACP agent. The JSON shape
// mirrors internal/agent.Session fields used by the chat UI (id, name, state,
// messages with the same ContentBlock tags) so the same view renders both.
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

	acpSessionID string
	promptCancel context.CancelFunc
}

// Manager owns ACP agent configs and running sessions.
type Manager struct {
	mu          sync.RWMutex
	configs     map[string]*AgentConfig
	sessions    map[string]*ACPSession
	conns       map[string]*conn // agent id → live process connection
	dataDir     string
	bus         *events.Bus
	nextMsgID   uint64
	nextSessNum int
}

func NewManager(dataDir string, bus *events.Bus) *Manager {
	m := &Manager{
		configs:  map[string]*AgentConfig{},
		sessions: map[string]*ACPSession{},
		conns:    map[string]*conn{},
		dataDir:  dataDir,
		bus:      bus,
	}
	m.loadConfigs()
	return m
}

// ---------------------------------------------------------------------------
// Config persistence
// ---------------------------------------------------------------------------

func (m *Manager) configsPath() string {
	return filepath.Join(m.dataDir, "acp_agents.json")
}

func (m *Manager) loadConfigs() {
	data, err := os.ReadFile(m.configsPath())
	if err != nil {
		// Seed default ACP agents: Pi and OhMyPi (OMP)
		m.configs["agent-pi"] = &AgentConfig{
			ID:      "agent-pi",
			Name:    "Pi Agent",
			Command: "npx",
			Args:    []string{"-y", "pi-acp@0.0.33"},
			Enabled: true,
		}
		m.configs["agent-ohmypi"] = &AgentConfig{
			ID:      "agent-ohmypi",
			Name:    "OhMyPi (omp)",
			Command: "omp",
			Args:    []string{"acp"},
			Enabled: true,
		}
		m.saveConfigsLocked()
		return
	}
	var cfgs []*AgentConfig
	if err := json.Unmarshal(data, &cfgs); err != nil {
		return
	}
	for _, c := range cfgs {
		// Automatically migrate legacy flags like --acp to standard ACP server args
		if c.ID == "agent-ohmypi" && len(c.Args) == 1 && c.Args[0] == "--acp" {
			c.Args = []string{"acp"}
		}
		if c.ID == "agent-pi" && (c.Command == "pi" && len(c.Args) == 1 && c.Args[0] == "--acp") {
			c.Command = "npx"
			c.Args = []string{"-y", "pi-acp@0.0.33"}
		}
		m.configs[c.ID] = c
	}
	// Ensure defaults exist if file was empty
	if len(m.configs) == 0 {
		m.configs["agent-pi"] = &AgentConfig{
			ID:      "agent-pi",
			Name:    "Pi Agent",
			Command: "npx",
			Args:    []string{"-y", "pi-acp@0.0.33"},
			Enabled: true,
		}
		m.configs["agent-ohmypi"] = &AgentConfig{
			ID:      "agent-ohmypi",
			Name:    "OhMyPi (omp)",
			Command: "omp",
			Args:    []string{"acp"},
			Enabled: true,
		}
		m.saveConfigsLocked()
	}
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

// CheckBinary tests whether a command binary exists in $PATH.
func (m *Manager) CheckBinary(command string) (bool, string) {
	if command == "" {
		return false, "Command is empty"
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

// DeleteAgent removes a config, killing any live connection for it.
func (m *Manager) DeleteAgent(id string) error {
	m.mu.Lock()
	if c, ok := m.conns[id]; ok {
		c.close()
		delete(m.conns, id)
	}
	delete(m.configs, id)
	m.saveConfigsLocked()
	m.mu.Unlock()
	return nil
}

// SlashCommandItem represents one available slash command or skill.
type SlashCommandItem struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"` // "command" | "skill"
}

// GetAgentModels returns the available models for a given ACP agent by inspecting local config files
// (~/.omp/agent/models.yml, ~/.pi/agent/models.json) or cached SupportedModels.
func (m *Manager) GetAgentModels(agentID string) []string {
	m.mu.RLock()
	cfg, ok := m.configs[agentID]
	var cmd string
	var cached []string
	if ok {
		cmd = cfg.Command
		cached = cfg.SupportedModels
	}
	m.mu.RUnlock()

	if len(cached) > 0 {
		return cached
	}

	detected := DetectLocalAgentModels(agentID, cmd)
	if len(detected) > 0 && ok {
		m.mu.Lock()
		cfg.SupportedModels = detected
		m.saveConfigsLocked()
		m.mu.Unlock()
	}
	return detected
}

// DetectLocalAgentModels inspects ~/.omp/agent/models.yml and ~/.pi/agent/models.json.
func DetectLocalAgentModels(agentID, command string) []string {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	var models []string
	seen := make(map[string]bool)

	addModel := func(id string) {
		id = strings.TrimSpace(id)
		if id != "" && !seen[id] {
			seen[id] = true
			models = append(models, id)
		}
	}

	// 1. Check Pi models (~/.pi/agent/models.json)
	if strings.Contains(agentID, "pi") || command == "pi" || agentID == "" {
		piPath := filepath.Join(home, ".pi", "agent", "models.json")
		if data, err := os.ReadFile(piPath); err == nil {
			var piData struct {
				Providers map[string]struct {
					Models []struct {
						ID   string `json:"id"`
						Name string `json:"name"`
					} `json:"models"`
				} `json:"providers"`
			}
			if err := json.Unmarshal(data, &piData); err == nil {
				for _, prov := range piData.Providers {
					for _, m := range prov.Models {
						addModel(m.ID)
					}
				}
			}
		}
	}

	// 2. Check OMP models (~/.omp/agent/models.yml)
	if strings.Contains(agentID, "omp") || command == "omp" || len(models) == 0 {
		ompPath := filepath.Join(home, ".omp", "agent", "models.yml")
		if data, err := os.ReadFile(ompPath); err == nil {
			var ompData struct {
				Providers map[string]struct {
					Models []struct {
						ID   string `yaml:"id"`
						Name string `yaml:"name"`
					} `yaml:"models"`
				} `yaml:"providers"`
			}
			if err := yaml.Unmarshal(data, &ompData); err == nil {
				for _, prov := range ompData.Providers {
					for _, m := range prov.Models {
						addModel(m.ID)
					}
				}
			}
		}
	}

	return models
}

// GetSlashCommandsAndSkills returns slash commands and discovered skills for Pi / OMP / workspace.
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

// ensureConn spawns the agent process and performs the ACP initialize
// handshake, reusing a live connection when present.
func (m *Manager) ensureConn(ctx context.Context, agentID string) (*conn, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c, ok := m.conns[agentID]; ok && !c.dead {
		return c, nil
	}
	cfg, ok := m.configs[agentID]
	if !ok {
		return nil, fmt.Errorf("unknown acp agent %q", agentID)
	}
	c, err := spawn(cfg)
	if err != nil {
		return nil, err
	}
	c.manager = m
	m.conns[agentID] = c
	go c.readLoop()

	if _, err := c.call(ctx, "initialize", map[string]any{
		"protocolVersion": protocolVersion,
		"clientInfo":      map[string]any{"name": "ForgeADE", "version": "0.8.5"},
		"clientCapabilities": map[string]any{
			"fs": map[string]any{"readTextFile": true, "writeTextFile": true},
		},
	}, nil); err != nil {
		c.close()
		delete(m.conns, agentID)
		return nil, fmt.Errorf("acp initialize: %w", err)
	}
	return c, nil
}

// CreateSession connects to the agent and opens a new ACP session.
func (m *Manager) CreateSession(ctx context.Context, agentID, name, folder string) (ACPSession, error) {
	c, err := m.ensureConn(ctx, agentID)
	if err != nil {
		return ACPSession{}, err
	}
	if folder == "" {
		if f, err := os.Getwd(); err == nil {
			folder = f
		}
	}
	var res struct {
		SessionID string `json:"sessionId"`
	}
	if _, err := c.call(ctx, "session/new", map[string]any{
		"cwd":        folder,
		"mcpServers": []any{},
	}, &res); err != nil {
		return ACPSession{}, fmt.Errorf("session/new: %w", err)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	cfg := m.configs[agentID]
	m.nextSessNum++
	s := &ACPSession{
		ID:           fmt.Sprintf("acps-%d-%d", time.Now().UnixNano(), m.nextSessNum),
		AgentID:      agentID,
		AgentName:    cfg.Name,
		Name:         name,
		State:        "idle",
		Folder:       folder,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
		acpSessionID: res.SessionID,
	}
	if s.Name == "" {
		s.Name = cfg.Name
	}
	m.sessions[s.ID] = s
	m.emitUpdate(s.ID)
	return *s, nil
}

// Send appends a user message and runs one prompt turn. It returns when the
// agent finishes the turn (stop reason received) or the context is cancelled.
func (m *Manager) Send(ctx context.Context, sessionID, message string, mentionedFiles []string) error {
	m.mu.RLock()
	s, ok := m.sessions[sessionID]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("unknown acp session %q", sessionID)
	}
	// Compose prompt text with mentioned file contents appended.
	text := message
	for _, f := range mentionedFiles {
		if data, err := os.ReadFile(f); err == nil {
			text += fmt.Sprintf("\n\n[File: %s]\n%s", f, string(data))
		} else {
			text += fmt.Sprintf("\n\n[File: %s (unreadable)]", f)
		}
	}

	m.mu.Lock()
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
	acpID := s.acpSessionID
	agentID := s.AgentID
	m.mu.Unlock()

	turnCtx, cancel := context.WithCancel(ctx)
	m.mu.Lock()
	s.promptCancel = cancel
	m.mu.Unlock()
	defer cancel()

	m.emitUpdate(sessionID)
	m.emitEvent(events.AgentTurnStart, sessionID, nil)

	c, err := m.ensureConn(turnCtx, agentID)
	if err != nil {
		m.finishTurn(sessionID, assistant.ID, "connection error: "+err.Error(), true)
		return err
	}

	c.beginStream(sessionID, assistant.ID)
	var res struct {
		StopReason string `json:"stopReason"`
	}
	_, callErr := c.call(turnCtx, "session/prompt", map[string]any{
		"sessionId": acpID,
		"prompt":    []map[string]any{{"type": "text", "text": text}},
	}, &res)
	c.endStream()

	if callErr != nil && turnCtx.Err() == nil {
		m.finishTurn(sessionID, assistant.ID, "error: "+callErr.Error(), true)
		return callErr
	}
	m.finishTurn(sessionID, assistant.ID, res.StopReason, false)
	return nil
}

// Cancel interrupts the running prompt turn with session/cancel.
func (m *Manager) Cancel(sessionID string) {
	m.mu.RLock()
	s, ok := m.sessions[sessionID]
	if !ok {
		m.mu.RUnlock()
		return
	}
	agentID := s.AgentID
	if s.promptCancel != nil {
		s.promptCancel()
	}
	acpID := s.acpSessionID
	m.mu.RUnlock()

	m.mu.Lock()
	if c, ok := m.conns[agentID]; ok {
		c.notify("session/cancel", map[string]any{"sessionId": acpID})
	}
	if s, ok := m.sessions[sessionID]; ok && s.State != "idle" {
		s.State = "idle"
		s.UpdatedAt = time.Now()
	}
	m.mu.Unlock()
	m.emitUpdate(sessionID)
}

// SetAutoApprove toggles auto-approval of permission requests.
func (m *Manager) SetAutoApprove(sessionID string, enabled bool) {
	m.mu.Lock()
	if s, ok := m.sessions[sessionID]; ok {
		s.AutoApprove = enabled
		s.UpdatedAt = time.Now()
	}
	m.mu.Unlock()
	m.emitUpdate(sessionID)
}

// RespondPermission answers a pending session/request_permission request.
// With optionID empty and cancel=false, the first allow option is chosen.
func (m *Manager) RespondPermission(sessionID, optionID string, cancel bool) error {
	m.mu.Lock()
	s, ok := m.sessions[sessionID]
	if !ok || s.PendingPermission == nil {
		m.mu.Unlock()
		return fmt.Errorf("no pending permission for session %q", sessionID)
	}
	reqID := s.PendingPermission.RequestID
	if optionID == "" && !cancel {
		for _, o := range s.PendingPermission.Options {
			if len(o.Kind) >= 5 && o.Kind[:5] == "allow" {
				optionID = o.OptionID
				break
			}
		}
		if optionID == "" && len(s.PendingPermission.Options) > 0 {
			optionID = s.PendingPermission.Options[0].OptionID
		}
	}
	c, connOK := m.conns[s.AgentID]
	acpID := s.acpSessionID
	s.PendingPermission = nil
	if s.State == "awaiting_approval" {
		s.State = "executing"
	}
	s.UpdatedAt = time.Now()
	m.mu.Unlock()

	if connOK && !cancel {
		c.respond(reqID, map[string]any{
			"outcome": map[string]any{"optionId": optionID, "outcome": "selected"},
		})
	} else if connOK {
		c.respond(reqID, map[string]any{
			"outcome": map[string]any{"outcome": "cancelled"},
		})
	}
	_ = acpID
	m.emitUpdate(sessionID)
	return nil
}

// StopAll kills every live agent process.
func (m *Manager) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for id, c := range m.conns {
		c.close()
		delete(m.conns, id)
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
