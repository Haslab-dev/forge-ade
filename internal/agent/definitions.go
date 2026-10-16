package agent

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/Haslab-dev/RakitKode/events"
	"github.com/google/uuid"
)

// AgentDefinition is a pre-configured agent the user defines once (in global
// settings) and selects in chat.
type AgentDefinition struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	Role        RoleFilter `json:"role_filter,omitempty"`
	Model       string     `json:"model,omitempty"`
	Prompt      string     `json:"prompt,omitempty"`
	Rules       string     `json:"rules,omitempty"`
	Color       string     `json:"color,omitempty"`
	AllowBash   bool       `json:"allow_bash"`
	ReadOnly    bool       `json:"read_only"`
	Tools       []string   `json:"tools,omitempty"`       // tool categories or tool names (e.g. "filesystem", "terminal", "git", "web", "browser")
	MCPServers  []string   `json:"mcp_servers,omitempty"` // enabled MCP server names (e.g. "exa", "figma")
	Skills      []string   `json:"skills,omitempty"`      // associated skill IDs
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
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
			AllowBash:   true,
			ReadOnly:    false,
			Tools:       []string{"filesystem", "terminal", "git", "web", "browser"},
			Skills:      []string{"implement", "grill-with-docs", "improve-codebase-architecture", "to-spec", "prototype"},
			CreatedAt:   time.Unix(0, 0),
			UpdatedAt:   time.Unix(0, 0),
		},
		{
			ID:          "planning",
			Name:        "Planning Agent",
			Description: "Break tasks into structured steps and produce plans without mutating code.",
			Role:        RolePlanning,
			AllowBash:   false,
			ReadOnly:    true,
			Tools:       []string{"filesystem", "git"},
			Skills:      []string{"wayfinder", "to-tickets", "triage", "grill-me", "to-spec"},
			CreatedAt:   time.Unix(0, 0),
			UpdatedAt:   time.Unix(0, 0),
		},
		{
			ID:          "research",
			Name:        "Research Agent",
			Description: "Explore the codebase, trace symbols and dependencies, synthesize insights, and research documentation.",
			Role:        RoleResearch,
			AllowBash:   true,
			ReadOnly:    true,
			Tools:       []string{"filesystem", "terminal", "web"},
			Skills:      []string{"grill-me", "to-spec", "prototype", "source-verification", "academic-literature-review", "citation-tracking", "evidence-synthesis"},
			CreatedAt:   time.Unix(0, 0),
			UpdatedAt:   time.Unix(0, 0),
		},
		{
			ID:          "custom",
			Name:        "Custom Agent",
			Description: "General assistant for ad-hoc tasks.",
			Role:        RoleCustom,
			AllowBash:   true,
			ReadOnly:    false,
			Tools:       []string{"filesystem", "terminal", "git", "web", "browser"},
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

	projFolder := m.normalizeFolder(folder)
	sysPrompt := buildSystemPrompt(def.Role, def.Prompt, def.Rules)

	m.mu.Lock()
	now := time.Now()
	sess := &Session{
		ID:           uuid.New().String(),
		Name:         def.Name,
		RoleFilter:   def.Role,
		Model:        def.Model,
		AllowBash:    def.AllowBash,
		ReadOnly:     def.ReadOnly,
		Tools:        def.Tools,
		MCPServers:   def.MCPServers,
		Skills:       def.Skills,
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
	if def.Model != "" {
		sess.Model = def.Model
	}
	sess.AllowBash = def.AllowBash
	sess.ReadOnly = def.ReadOnly
	sess.Tools = def.Tools
	sess.MCPServers = def.MCPServers
	sess.Skills = def.Skills
	sess.UpdatedAt = time.Now()
	m.saveSessionsLocked()
	m.mu.Unlock()

	if def.Model != "" && m.llmClient != nil {
		m.llmClient.SetActiveModelByID(def.Model)
	}

	m.emitSessionUpdate(sessionID)
	return nil
}

// FindDefinition returns a configured agent definition by ID, role, or name.
func (m *Manager) FindDefinition(roleOrID string) (AgentDefinition, bool) {
	if roleOrID == "" {
		return AgentDefinition{}, false
	}
	trimmed := strings.TrimSpace(roleOrID)
	lower := strings.ToLower(trimmed)

	// 1. Direct ID match
	if def, ok := m.getDefinition(trimmed); ok {
		return def, true
	}
	if def, ok := m.getDefinition(lower); ok {
		return def, true
	}

	// 2. Search user-managed definitions by Role or Name
	m.mu.RLock()
	for _, d := range m.definitions {
		if strings.ToLower(string(d.Role)) == lower || strings.EqualFold(d.Name, trimmed) {
			m.mu.RUnlock()
			return d, true
		}
	}
	m.mu.RUnlock()

	// 3. Search built-in definitions by Role or Name
	for _, b := range builtinAgentDefs() {
		if strings.ToLower(string(b.Role)) == lower || strings.EqualFold(b.Name, trimmed) {
			// Check if user has an override in m.definitions for this built-in ID
			if userDef, ok := m.getDefinition(b.ID); ok {
				return userDef, true
			}
			return b, true
		}
	}

	return AgentDefinition{}, false
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
	m.definitions = make(map[string]AgentDefinition)
	path := m.definitionsPath()
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}

	// 1. Try unmarshaling as map[string]AgentDefinition (how saveDefinitionsLocked writes it)
	var asMap map[string]AgentDefinition
	if err := json.Unmarshal(data, &asMap); err == nil {
		for id, d := range asMap {
			if d.ID == "" {
				d.ID = id
			}
			m.definitions[d.ID] = d
		}
	} else {
		// 2. Fall back to unmarshaling as []AgentDefinition
		var asList []AgentDefinition
		if err := json.Unmarshal(data, &asList); err == nil {
			for _, d := range asList {
				if d.ID != "" {
					m.definitions[d.ID] = d
				}
			}
		}
	}

	// 3. Hydrate missing tools and skills for built-in archetypes
	for _, b := range builtinAgentDefs() {
		if cur, ok := m.definitions[b.ID]; ok {
			changed := false
			if len(cur.Tools) == 0 && len(b.Tools) > 0 {
				cur.Tools = b.Tools
				changed = true
			}
			if len(cur.Skills) == 0 && len(b.Skills) > 0 {
				cur.Skills = b.Skills
				changed = true
			}
			if changed {
				m.definitions[b.ID] = cur
			}
		}
	}
}

func (m *Manager) normalizeFolder(folder string) string {
	if folder != "" {
		if abs, err := filepath.Abs(folder); err == nil {
			return abs
		}
		return folder
	}
	// Fall back to the current workspace, then home — never the process
	// working directory (which is "/" for a Finder-launched app).
	if def, ok := m.defaultFolder.Load().(string); ok && def != "" {
		return def
	}
	if home, err := os.UserHomeDir(); err == nil {
		return home
	}
	return ""
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
