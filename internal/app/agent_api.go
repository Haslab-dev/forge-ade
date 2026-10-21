package app

import (
	"fmt"
	"strings"

	"github.com/Haslab-dev/RakitKode/agent"
	"github.com/Haslab-dev/RakitKode/decisions"
)

// ---------------------------------------------------------------------------
// Agent & LLM & Git API
// ---------------------------------------------------------------------------

// CreateAgentSession creates a new agent session with specified role filter.
func (a *App) CreateAgentSession(name string, role string, folder string) (*agent.Session, error) {
	if folder == "" || strings.EqualFold(folder, "general") {
		folder = a.generalFolder()
	}
	sess, err := a.agentMgr.CreateSession(name, agent.RoleFilter(role), folder)
	if err == nil && sess != nil {
		a.ensureSessionBoardTools(sess)
	}
	return sess, err
}

// ListAgentSessions returns all active agent sessions.
func (a *App) ListAgentSessions() []*agent.Session {
	sessions := a.agentMgr.ListSessions()
	for _, sess := range sessions {
		a.ensureSessionBoardTools(sess)
	}
	return sessions
}

// ListAgentSessionsForFolder returns the agent sessions linked to the given
// project folder (and its subfolders) — the session history for the current
// project only, hiding sessions from other projects.
func (a *App) ListAgentSessionsForFolder(folder string) []*agent.Session {
	sessions := a.agentMgr.ListSessionsForFolder(folder)
	for _, sess := range sessions {
		a.ensureSessionBoardTools(sess)
	}
	return sessions
}

// GetAgentSession returns a single agent session by ID.
func (a *App) GetAgentSession(id string) (*agent.Session, error) {
	sess, ok := a.agentMgr.GetSession(id)
	if !ok {
		return nil, fmt.Errorf("session not found")
	}
	a.ensureSessionBoardTools(sess)
	return sess, nil
}

// UpdateAgentSession updates editable agent session fields (name, role, custom prompt, custom rules).
func (a *App) UpdateAgentSession(id string, name string, role string, customPrompt string, customRules string) (*agent.Session, error) {
	sess, err := a.agentMgr.UpdateSession(id, name, agent.RoleFilter(role), customPrompt, customRules)
	if err == nil && sess != nil {
		a.ensureSessionBoardTools(sess)
	}
	return sess, err
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
	sess, err := a.agentMgr.CreateSessionFromDefinition(defID, folder)
	if err == nil && sess != nil {
		a.ensureSessionBoardTools(sess)
	}
	return sess, err
}

// SendAgentMessage sends a message to an agent session with optional @ file mentions.
func (a *App) SendAgentMessage(sessionID string, content string, mentionedPaths []string) error {
	if sess, ok := a.agentMgr.GetSession(sessionID); ok && sess != nil {
		a.ensureSessionBoardTools(sess)
	}
	return a.agentMgr.SendMessage(a.ctx, sessionID, content, mentionedPaths)
}

// SteerAgentMessage delivers a mid-turn instruction directly into the running agent loop
// between tool iterations without aborting the turn.
func (a *App) SteerAgentMessage(sessionID string, content string) error {
	if sess, ok := a.agentMgr.GetSession(sessionID); ok && sess != nil {
		a.ensureSessionBoardTools(sess)
	}
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

// GetJevStatus returns the operational status of TypeSafe AI Jev tool router and local small model.
func (a *App) GetJevStatus() map[string]interface{} {
	if a.agentMgr == nil {
		return map[string]interface{}{
			"enabled":        false,
			"active":         false,
			"has_key":        false,
			"local_fallback": true,
			"model":          "jev-latest",
			"local_model":    decisions.SmallModelVersion,
		}
	}
	r := a.agentMgr.GetJevRouter()
	if r == nil {
		return map[string]interface{}{
			"enabled":        false,
			"active":         false,
			"has_key":        false,
			"local_fallback": true,
			"model":          "jev-latest",
			"local_model":    decisions.SmallModelVersion,
		}
	}
	return map[string]interface{}{
		"enabled":        r.GetEnabled(),
		"active":         r.IsEnabled(),
		"has_key":        r.GetAPIKey() != "",
		"local_fallback": !r.IsEnabled(),
		"model":          "jev-latest",
		"local_model":    decisions.SmallModelVersion,
	}
}

// SetJevAPIKey updates the API key for TypeSafe AI Jev router and persists it.
func (a *App) SetJevAPIKey(key string) error {
	cfg := a.appSettings.Get()
	cfg.Jev.APIKey = key
	_, err := a.SaveAppSettings(cfg)
	return err
}

// SetJevEnabled toggles TypeSafe AI Jev router on or off and persists it.
func (a *App) SetJevEnabled(enabled bool) error {
	cfg := a.appSettings.Get()
	cfg.Jev.Enabled = enabled
	_, err := a.SaveAppSettings(cfg)
	return err
}

// GetUsageSummary returns aggregated usage statistics across models, sessions, and providers,
// combining standard RakitKode agent sessions and external ACP orchestrator sessions.
func (a *App) GetUsageSummary() map[string]interface{} {
	var summary map[string]interface{}
	if a.agentMgr != nil {
		summary = a.agentMgr.GetUsageSummary()
	} else {
		summary = map[string]interface{}{
			"totals": map[string]interface{}{
				"total_tokens":      0,
				"prompt_tokens":     0,
				"completion_tokens": 0,
				"cached_tokens":     0,
				"session_count":     0,
				"turn_count":        0,
			},
			"by_model":    []map[string]interface{}{},
			"by_provider": []map[string]interface{}{},
			"by_session":  []map[string]interface{}{},
		}
	}
	if a.acpMgr != nil {
		summary = a.mergeACPUsage(summary)
	}
	return summary
}

func toUsageInt(v interface{}) int {
	switch n := v.(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	case float32:
		return int(n)
	}
	return 0
}

func (a *App) mergeACPUsage(base map[string]interface{}) map[string]interface{} {
	if a.acpMgr == nil {
		return base
	}
	acpSummary := a.acpMgr.GetUsageSummary()
	if acpSummary == nil {
		return base
	}

	// Merge totals
	baseTotals, _ := base["totals"].(map[string]interface{})
	acpTotals, _ := acpSummary["totals"].(map[string]interface{})
	if baseTotals == nil {
		baseTotals = make(map[string]interface{})
		base["totals"] = baseTotals
	}
	if acpTotals != nil {
		baseTotals["total_tokens"] = toUsageInt(baseTotals["total_tokens"]) + toUsageInt(acpTotals["total_tokens"])
		baseTotals["prompt_tokens"] = toUsageInt(baseTotals["prompt_tokens"]) + toUsageInt(acpTotals["prompt_tokens"])
		baseTotals["completion_tokens"] = toUsageInt(baseTotals["completion_tokens"]) + toUsageInt(acpTotals["completion_tokens"])
		baseTotals["cached_tokens"] = toUsageInt(baseTotals["cached_tokens"]) + toUsageInt(acpTotals["cached_tokens"])
		baseTotals["session_count"] = toUsageInt(baseTotals["session_count"]) + toUsageInt(acpTotals["session_count"])
		baseTotals["turn_count"] = toUsageInt(baseTotals["turn_count"]) + toUsageInt(acpTotals["turn_count"])
	}

	// Merge by_model
	var baseByModel []map[string]interface{}
	if raw, ok := base["by_model"].([]map[string]interface{}); ok {
		baseByModel = raw
	} else if rawAny, ok := base["by_model"].([]interface{}); ok {
		for _, item := range rawAny {
			if m, ok := item.(map[string]interface{}); ok {
				baseByModel = append(baseByModel, m)
			}
		}
	}

	acpByModel, _ := acpSummary["by_model"].([]map[string]interface{})
	for _, acpM := range acpByModel {
		modelName, _ := acpM["model"].(string)
		found := false
		for _, bM := range baseByModel {
			if bM["model"] == modelName {
				bM["prompt_tokens"] = toUsageInt(bM["prompt_tokens"]) + toUsageInt(acpM["prompt_tokens"])
				bM["completion_tokens"] = toUsageInt(bM["completion_tokens"]) + toUsageInt(acpM["completion_tokens"])
				bM["cached_tokens"] = toUsageInt(bM["cached_tokens"]) + toUsageInt(acpM["cached_tokens"])
				bM["total_tokens"] = toUsageInt(bM["total_tokens"]) + toUsageInt(acpM["total_tokens"])
				bM["turns"] = toUsageInt(bM["turns"]) + toUsageInt(acpM["turns"])
				bM["sessions"] = toUsageInt(bM["sessions"]) + toUsageInt(acpM["sessions"])
				found = true
				break
			}
		}
		if !found {
			baseByModel = append(baseByModel, acpM)
		}
	}
	base["by_model"] = baseByModel

	// Merge by_provider
	var baseByProvider []map[string]interface{}
	if raw, ok := base["by_provider"].([]map[string]interface{}); ok {
		baseByProvider = raw
	} else if rawAny, ok := base["by_provider"].([]interface{}); ok {
		for _, item := range rawAny {
			if p, ok := item.(map[string]interface{}); ok {
				baseByProvider = append(baseByProvider, p)
			}
		}
	}

	acpByProvider, _ := acpSummary["by_provider"].([]map[string]interface{})
	for _, acpP := range acpByProvider {
		providerName, _ := acpP["provider"].(string)
		found := false
		for _, bP := range baseByProvider {
			if bP["provider"] == providerName {
				bP["prompt_tokens"] = toUsageInt(bP["prompt_tokens"]) + toUsageInt(acpP["prompt_tokens"])
				bP["completion_tokens"] = toUsageInt(bP["completion_tokens"]) + toUsageInt(acpP["completion_tokens"])
				bP["cached_tokens"] = toUsageInt(bP["cached_tokens"]) + toUsageInt(acpP["cached_tokens"])
				bP["total_tokens"] = toUsageInt(bP["total_tokens"]) + toUsageInt(acpP["total_tokens"])
				bP["sessions"] = toUsageInt(bP["sessions"]) + toUsageInt(acpP["sessions"])
				found = true
				break
			}
		}
		if !found {
			baseByProvider = append(baseByProvider, acpP)
		}
	}
	base["by_provider"] = baseByProvider

	// Merge by_session
	var baseBySession []map[string]interface{}
	if raw, ok := base["by_session"].([]map[string]interface{}); ok {
		baseBySession = raw
	} else if rawAny, ok := base["by_session"].([]interface{}); ok {
		for _, item := range rawAny {
			if s, ok := item.(map[string]interface{}); ok {
				baseBySession = append(baseBySession, s)
			}
		}
	}
	acpBySession, _ := acpSummary["by_session"].([]map[string]interface{})
	baseBySession = append(baseBySession, acpBySession...)
	base["by_session"] = baseBySession

	return base
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
