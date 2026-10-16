package app

import (
	"fmt"
	"strings"

	"github.com/Haslab-dev/RakitKode/agent"
)

// ---------------------------------------------------------------------------
// Agent & LLM & Git API
// ---------------------------------------------------------------------------

// CreateAgentSession creates a new agent session with specified role filter.
func (a *App) CreateAgentSession(name string, role string, folder string) (*agent.Session, error) {
	if folder == "" || strings.EqualFold(folder, "general") {
		folder = a.generalFolder()
	}
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
	if folder == "" || strings.EqualFold(folder, "general") {
		folder = a.generalFolder()
	}
	return a.agentMgr.CreateSessionFromDefinition(defID, folder)
}

// SendAgentMessage sends a message to an agent session with optional @ file mentions.
func (a *App) SendAgentMessage(sessionID string, content string, mentionedPaths []string) error {
	return a.agentMgr.SendMessage(a.ctx, sessionID, content, mentionedPaths)
}

// SteerAgentMessage delivers a mid-turn instruction directly into the running agent loop
// between tool iterations without aborting the turn.
func (a *App) SteerAgentMessage(sessionID string, content string) error {
	return a.agentMgr.SteerMessage(sessionID, content)
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

// CompactAgentSession forces compaction of earlier conversation turns for a session.
func (a *App) CompactAgentSession(sessionID string) (*agent.Session, error) {
	if a.agentMgr == nil {
		return nil, fmt.Errorf("agent manager unavailable")
	}
	return a.agentMgr.CompactSession(sessionID)
}

func (a *App) ToggleAgentTask(sessionID string, taskID string, completed bool) error {
	a.agentMgr.ToggleTask(sessionID, taskID, completed)
	return nil
}

func (a *App) ClearAgentTasks(sessionID string) error {
	a.agentMgr.ClearTasks(sessionID)
	return nil
}

// GetSubagents returns child sessions for a parent agent session.
func (a *App) GetSubagents(parentID string) []*agent.Session {
	return a.agentMgr.GetSubagents(parentID)
}

// GetJevStatus returns the operational status of TypeSafe AI Jev tool router.
func (a *App) GetJevStatus() map[string]interface{} {
	r := a.agentMgr.GetJevRouter()
	if r == nil {
		return map[string]interface{}{"enabled": false, "has_key": false}
	}
	return map[string]interface{}{
		"enabled": r.IsEnabled(),
		"has_key": r.GetAPIKey() != "",
	}
}

// SetJevAPIKey updates the API key for TypeSafe AI Jev router.
func (a *App) SetJevAPIKey(key string) error {
	r := a.agentMgr.GetJevRouter()
	if r != nil {
		r.SetAPIKey(key)
	}
	return nil
}

// GetUsageSummary returns aggregated usage statistics across models, sessions, and providers.
func (a *App) GetUsageSummary() map[string]interface{} {
	if a.agentMgr == nil {
		return map[string]interface{}{}
	}
	return a.agentMgr.GetUsageSummary()
}

// GetJevStats returns operational and token-saving metrics from the Jev router.
func (a *App) DecideAgentRole(task string) (map[string]interface{}, error) {
	if a.agentMgr == nil {
		return map[string]interface{}{
			"role":       "coding",
			"reason":     "Agent manager unavailable",
			"confidence": 0.5,
			"source":     "fallback",
		}, nil
	}
	dec := a.agentMgr.DecideSubagentRole(task)
	if dec == nil {
		return map[string]interface{}{
			"role":       "coding",
			"reason":     "Default coding role",
			"confidence": 0.8,
			"source":     "fallback",
		}, nil
	}
	return map[string]interface{}{
		"role":       dec.Role,
		"reason":     dec.Reason,
		"confidence": dec.Confidence,
		"source":     dec.Source,
	}, nil
}

func (a *App) GetJevStats() map[string]interface{} {
	if a.agentMgr == nil {
		return map[string]interface{}{}
	}
	r := a.agentMgr.GetJevRouter()
	if r == nil {
		return map[string]interface{}{}
	}
	return r.GetStats()
}

// ListBackgroundTasks returns all active and recent background tasks.
func (a *App) ListBackgroundTasks(sessionID string) []map[string]any {
	tasks := a.agentMgr.ToolRegistry().ListTasks(sessionID)
	out := make([]map[string]any, 0, len(tasks))
	for _, t := range tasks {
		out = append(out, t.TaskSnapshot())
	}
	return out
}

// StopBackgroundTask stops a running background task.
func (a *App) StopBackgroundTask(taskID string) bool {
	return a.agentMgr.ToolRegistry().StopTask(taskID)
}

// GetBackgroundTaskOutput returns current output snapshot of a task.
func (a *App) GetBackgroundTaskOutput(taskID string) (map[string]any, error) {
	task, ok := a.agentMgr.ToolRegistry().GetTask(taskID)
	if !ok {
		return nil, fmt.Errorf("task %s not found", taskID)
	}
	return task.TaskSnapshot(), nil
}

// ListAgentSchedules returns active schedules/timers.
func (a *App) ListAgentSchedules(sessionID string) []map[string]any {
	if a.agentMgr == nil {
		return nil
	}
	schedules := a.agentMgr.ListSchedules(sessionID)
	out := make([]map[string]any, 0, len(schedules))
	for _, s := range schedules {
		out = append(out, s.ToSnapshot())
	}
	return out
}

// CancelAgentSchedule cancels an active schedule.
func (a *App) CancelAgentSchedule(scheduleID string) bool {
	if a.agentMgr == nil {
		return false
	}
	return a.agentMgr.CancelSchedule(scheduleID)
}
