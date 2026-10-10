package app

import (
	"fmt"

	"github.com/hasdev/forge-ade/internal/agent"
)

// ---------------------------------------------------------------------------
// Agent & LLM & Git API
// ---------------------------------------------------------------------------

// CreateAgentSession creates a new agent session with specified role filter.
func (a *App) CreateAgentSession(name string, role string, folder string) (*agent.Session, error) {
	return a.agentMgr.CreateSession(name, agent.RoleFilter(role), folder)
}

// ListAgentSessions returns all active agent sessions.
func (a *App) ListAgentSessions() []*agent.Session {
	return a.agentMgr.ListSessions()
}

// ListAgentSessionsForFolder returns the agent sessions linked to the given
// project folder (and its subfolders) — the session history for the current
// project only, hiding sessions from other projects.
func (a *App) ListAgentSessionsForFolder(folder string) []*agent.Session {
	return a.agentMgr.ListSessionsForFolder(folder)
}

// GetAgentSession returns a single agent session by ID.
func (a *App) GetAgentSession(id string) (*agent.Session, error) {
	sess, ok := a.agentMgr.GetSession(id)
	if !ok {
		return nil, fmt.Errorf("session not found")
	}
	return sess, nil
}

// UpdateAgentSession updates editable agent session fields (name, role, custom prompt, custom rules).
func (a *App) UpdateAgentSession(id string, name string, role string, customPrompt string, customRules string) (*agent.Session, error) {
	return a.agentMgr.UpdateSession(id, name, agent.RoleFilter(role), customPrompt, customRules)
}

// ListAgentDefinitions returns all pre-configured agent definitions.
func (a *App) ListAgentDefinitions() []agent.AgentDefinition {
	return a.agentMgr.ListAgentDefinitions()
}

// SaveAgentDefinition creates or updates a pre-configured agent definition.
func (a *App) SaveAgentDefinition(def agent.AgentDefinition) (agent.AgentDefinition, error) {
	return a.agentMgr.SaveAgentDefinition(def)
}

// DeleteAgentDefinition removes a pre-configured agent definition.
func (a *App) DeleteAgentDefinition(id string) error {
	return a.agentMgr.DeleteAgentDefinition(id)
}

// CreateAgentSessionFromDefinition creates a chat session from a pre-configured
// agent definition, scoped to the given project folder.
func (a *App) CreateAgentSessionFromDefinition(defID string, folder string) (*agent.Session, error) {
	return a.agentMgr.CreateSessionFromDefinition(defID, folder)
}

// SendAgentMessage sends a message to an agent session with optional @ file mentions.
func (a *App) SendAgentMessage(sessionID string, content string, mentionedPaths []string) error {
	return a.agentMgr.SendMessage(a.ctx, sessionID, content, mentionedPaths)
}

// RespondAgentApproval responds to a pending tool execution approval.
func (a *App) RespondAgentApproval(sessionID string, approve bool, autoApproveAll bool) error {
	return a.agentMgr.RespondApproval(a.ctx, sessionID, approve, autoApproveAll)
}

// StopAgentTurn cancels the currently running agent turn for a session.
func (a *App) StopAgentTurn(sessionID string) {
	a.agentMgr.StopTurn(sessionID)
}

// SetAgentDialect switches a session's tool-calling dialect ("" = native, "xml" = in-band).
func (a *App) SetAgentDialect(sessionID string, dialect string) error {
	return a.agentMgr.SetDialect(sessionID, dialect)
}

// RespondAgentAsk answers pending `ask` questions and resumes the agent turn.
func (a *App) RespondAgentAsk(sessionID string, answers map[string]any) error {
	return a.agentMgr.RespondAsk(sessionID, answers)
}

// SetAgentAutoApprove toggles yolo mode (always approve tool calls) for a session.
func (a *App) SetAgentAutoApprove(sessionID string, enabled bool) error {
	return a.agentMgr.SetAutoApprove(sessionID, enabled)
}

// ApplyAgentDefinitionToSession re-configures an existing session to use a
// pre-configured agent definition (role, prompt, rules, model) without
// creating a new session.
func (a *App) ApplyAgentDefinitionToSession(sessionID string, defID string) error {
	return a.agentMgr.ApplyDefinitionToSession(sessionID, defID)
}

// DeleteAgentSession deletes an agent session.
func (a *App) DeleteAgentSession(id string) error {
	a.agentMgr.DeleteSession(id)
	return nil
}

func (a *App) ToggleAgentTask(sessionID string, taskID string, completed bool) error {
	a.agentMgr.ToggleTask(sessionID, taskID, completed)
	return nil
}

// GetProviderProfiles gets all configured provider profiles.
