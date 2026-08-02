package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/hasdev/forge-ade/internal/events"
)

// AgentDefinition is a pre-configured agent the user defines once (in global
// settings) and selects in chat.
type AgentDefinition struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Description string   `json:"description,omitempty"`
	Role        RoleFilter `json:"role_filter,omitempty"`
	Model       string   `json:"model,omitempty"`
	Prompt      string   `json:"prompt,omitempty"`
	Rules       string   `json:"rules,omitempty"`
	Color       string   `json:"color,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// definitionsPath returns where ForgeADE stores pre-configured agent
// definitions (~/.forge-ade/forgeade.agents.json).
func (m *Manager) definitionsPath() string {
	if m.storePath != "" {
		return filepath.Join(filepath.Dir(m.storePath), "forgeade.agents.json")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".forge-ade", "forgeade.agents.json")
}

// ListAgentDefinitions returns all pre-configured agent definitions: built-in
// defaults plus ForgeADE-managed ones.
func (m *Manager) ListAgentDefinitions() []AgentDefinition {
	m.mu.RLock()

	defs := make([]AgentDefinition, 0, len(m.definitions)+4)
	for _, d := range m.definitions {
		defs = append(defs, d)
	}
	managedIDs := make(map[string]bool, len(defs))
	for _, d := range defs {
		managedIDs[d.ID] = true
	}
	m.mu.RUnlock()

	// Built-in defaults always available, unless the user overrode them.
	for _, b := range builtinAgentDefs() {
		if !managedIDs[b.ID] {
			defs = append(defs, b)
		}
	}
	return defs
}

// builtinAgentDefs are the always-available default agents.
func builtinAgentDefs() []AgentDefinition {
	return []AgentDefinition{
		{
			ID:          "coding",
			Name:        "Coding Agent",
			Description: "Write clean, production-ready code, run tests, and check git status.",
			Role:        RoleCoding,
			CreatedAt:   time.Unix(0, 0),
			UpdatedAt:   time.Unix(0, 0),
		},
		{
			ID:          "planning",
			Name:        "Planning Agent",
			Description: "Break tasks into structured steps and produce plans without mutating code.",
			Role:        RolePlanning,
			CreatedAt:   time.Unix(0, 0),
			UpdatedAt:   time.Unix(0, 0),
		},
		{
			ID:          "research",
			Name:        "Research Agent",
			Description: "Explore the codebase, trace symbols and dependencies, and synthesize insights.",
			Role:        RoleResearch,
			CreatedAt:   time.Unix(0, 0),
			UpdatedAt:   time.Unix(0, 0),
		},
		{
			ID:          "custom",
			Name:        "Custom Agent",
			Description: "General assistant for ad-hoc tasks.",
			Role:        RoleCustom,
			CreatedAt:   time.Unix(0, 0),
			UpdatedAt:   time.Unix(0, 0),
		},
	}
}

// SaveAgentDefinition creates or updates a pre-configured agent definition.
func (m *Manager) SaveAgentDefinition(def AgentDefinition) (AgentDefinition, error) {
	m.mu.Lock()
	if strings.TrimSpace(def.Name) == "" {
		m.mu.Unlock()
		return def, fmt.Errorf("agent name is required")
	}
	now := time.Now()
	if def.ID == "" {
		def.ID = uuid.New().String()
		def.CreatedAt = now
	}
	def.UpdatedAt = now
	if def.Role == "" {
		def.Role = RoleCoding
	}
	m.definitions[def.ID] = def
	m.saveDefinitionsLocked()
	m.mu.Unlock()

	m.ConfigChanged()
	return def, nil
}

// DeleteAgentDefinition removes a pre-configured agent definition.
func (m *Manager) DeleteAgentDefinition(id string) error {
	m.mu.Lock()
	delete(m.definitions, id)
	m.saveDefinitionsLocked()
	m.mu.Unlock()

	m.ConfigChanged()
	return nil
}

// CreateSessionFromDefinition creates a chat session from a pre-configured
// agent definition, scoped to the given project folder.
func (m *Manager) CreateSessionFromDefinition(defID string, folder string) (*Session, error) {
	def, ok := m.getDefinition(defID)
	if !ok {
		return nil, fmt.Errorf("agent definition %s not found", defID)
	}

	projFolder := normalizeFolder(folder)
	sysPrompt := buildSystemPrompt(def.Role, def.Prompt, def.Rules)

	m.mu.Lock()
	now := time.Now()
	sess := &Session{
		ID:           uuid.New().String(),
		Name:         def.Name,
		RoleFilter:   def.Role,
		State:        StateIdle,
		Folder:       projFolder,
		ProjectName:  filepath.Base(projFolder),
		Messages:     make([]AgentMessage, 0),
		Tasks:        make([]TaskItem, 0),
		SystemPrompt: sysPrompt,
		CustomPrompt: def.Prompt,
		CustomRules:  def.Rules,
		CreatedAt:    now,
		UpdatedAt:    now,
	}
	m.sessions[sess.ID] = sess
	m.saveSessionsLocked()
	m.mu.Unlock()

	// Apply the definition's model if set.
	if def.Model != "" && m.llmClient != nil {
		m.llmClient.SetActiveModelByID(def.Model)
	}

	// Emit so the frontend (ShellScreen, sidebar) refreshes the session list.
	m.emitSessionUpdate(sess.ID)

	return sess, nil
}

// ApplyDefinitionToSession re-configures an EXISTING session to use a
// pre-configured agent definition — changing its context (role, system prompt,
// custom rules, model) without creating a new session. Re-selecting an agent
// mode in the UI should switch the current session's behavior, not spawn a
// new tab.
func (m *Manager) ApplyDefinitionToSession(sessionID string, defID string) error {
	def, ok := m.getDefinition(defID)
	if !ok {
		return fmt.Errorf("agent definition %s not found", defID)
	}

	m.mu.Lock()
	sess, ok := m.sessions[sessionID]
	if !ok {
		m.mu.Unlock()
		return fmt.Errorf("session %s not found", sessionID)
	}
	// Changing agent mode re-configures the session's CONTEXT only — the
	// session title must not be overwritten by the definition's name.
	sess.RoleFilter = def.Role
	sess.CustomPrompt = def.Prompt
	sess.CustomRules = def.Rules
	sess.SystemPrompt = buildSystemPrompt(def.Role, def.Prompt, def.Rules)
	sess.UpdatedAt = time.Now()
	m.saveSessionsLocked()
	m.mu.Unlock()

	if def.Model != "" && m.llmClient != nil {
		m.llmClient.SetActiveModelByID(def.Model)
	}

	m.emitSessionUpdate(sessionID)
	return nil
}

func (m *Manager) getDefinition(id string) (AgentDefinition, bool) {
	m.mu.RLock()
	def, ok := m.definitions[id]
	m.mu.RUnlock()
	if ok {
		return def, true
	}
	// Fall back to built-in agents.
	for _, b := range builtinAgentDefs() {
		if b.ID == id {
			return b, true
		}
	}
	return AgentDefinition{}, false
}

func (m *Manager) saveDefinitionsLocked() {
	path := m.definitionsPath()
	_ = os.MkdirAll(filepath.Dir(path), 0755)
	data, err := json.MarshalIndent(m.definitions, "", "  ")
	if err == nil {
		_ = os.WriteFile(path, data, 0644)
	}
}

func (m *Manager) loadDefinitions() {
	m.mu.Lock()
	defer m.mu.Unlock()
	path := m.definitionsPath()
	data, err := os.ReadFile(path)
	if err != nil {
		m.definitions = make(map[string]AgentDefinition)
		return
	}
	var list []AgentDefinition
	if json.Unmarshal(data, &list) != nil {
		m.definitions = make(map[string]AgentDefinition)
		return
	}
	m.definitions = make(map[string]AgentDefinition, len(list))
	for _, d := range list {
		if d.ID != "" {
			m.definitions[d.ID] = d
		}
	}
}

func normalizeFolder(folder string) string {
	if folder == "" {
		if cwd, err := os.Getwd(); err == nil {
			return cwd
		}
		return ""
	}
	if abs, err := filepath.Abs(folder); err == nil {
		return abs
	}
	return folder
}

// ConfigChanged publishes an event so the frontend can refresh provider/model
// lists after settings change.
func (m *Manager) ConfigChanged() {
	if m.bus != nil {
		m.bus.Publish(events.Event{
			Type: "agent:config:changed",
			Data: map[string]interface{}{},
		})
	}
}
