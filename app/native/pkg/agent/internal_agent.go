package agent

// internal_agent.go routes ForgeADE Internal sessions through the RakitKode
// agent.Manager — the same full engine (tool loop, approvals, asks, subagents,
// schedules, background tasks, checkpoints) the webview app uses. The manager
// session UUID is stored on AgentSession.RemoteSessionID; bus events carry the
// manager ID, which MapInternalSession translates back to the native session.

import (
	"context"
	"fmt"

	"github.com/Haslab-dev/RakitKode/agent"
)

// ensureInternalManagerSession returns the RakitKode agent session backing a
// native internal session, creating it on the first turn (and re-linking
// restored sessions whose manager session no longer exists).
func (as *AgentState) ensureInternalManagerSession(sess *AgentSession) (string, error) {
	if as.engine == nil || as.engine.AgentMgr == nil {
		return "", fmt.Errorf("agent engine unavailable")
	}
	if sess.RemoteSessionID != "" {
		if _, ok := as.engine.AgentMgr.GetSession(sess.RemoteSessionID); ok {
			return sess.RemoteSessionID, nil
		}
	}
	role := agent.RoleCoding
	mgrSess, err := as.engine.AgentMgr.CreateSession(sess.Title, role, sess.WorkspacePath)
	if err != nil {
		return "", err
	}
	sess.RemoteSessionID = mgrSess.ID
	return mgrSess.ID, nil
}

// syncInternalSessionState pushes the current model / execution mode onto the
// backing manager session before each turn, as SetAgentSessionModel and
// agentExecutionMode do in forge-ade.
func (as *AgentState) syncInternalSessionState(mgrID string, sess *AgentSession) {
	if as.engine == nil || as.engine.AgentMgr == nil {
		return
	}
	if sess.Model != "" {
		_ = as.engine.AgentMgr.SetSessionModel(mgrID, sess.Model)
	}
	mode := as.ExecutionMode.PermissionMode()
	if mode != "" {
		_ = as.engine.AgentMgr.SetPermissionMode(mgrID, mode)
	}
	_ = as.engine.AgentMgr.SetAutoApprove(mgrID, as.ExecutionMode == ModeBypass)
}

// TriggerInternalTurn sends a user prompt through the RakitKode agent engine.
// Streaming events (agent:message_delta, agent:tool_*, agent:turn_end, ...)
// arrive on the shared bus and are bridged into Handle* by ACPManager.
func (as *AgentState) TriggerInternalTurn(sess *AgentSession, prompt string) error {
	mgrID, err := as.ensureInternalManagerSession(sess)
	if err != nil {
		return err
	}
	as.mu.Lock()
	as.syncInternalSessionState(mgrID, sess)
	mentions := append([]string(nil), as.AttachedFiles...)
	as.mu.Unlock()

	if as.engine == nil || as.engine.AgentMgr == nil {
		return fmt.Errorf("agent engine unavailable")
	}
	// The manager owns turn lifetime (interrupt via StopTurn); the app
	// context stays alive like the webview's app-level ctx.
	return as.engine.AgentMgr.SendMessage(context.Background(), mgrID, prompt, mentions)
}

// IsInternalSession reports whether the session runs on the RakitKode agent
// engine (as opposed to an ACP harness or a CLI PTY session).
func (as *AgentState) IsInternalSession(sessionID string) bool {
	as.mu.Lock()
	defer as.mu.Unlock()
	for _, s := range as.Sessions {
		if s.ID == sessionID {
			return s.AgentType == "internal" && s.RemoteSessionID != ""
		}
	}
	return false
}

// MapInternalSession translates a bus session ID (the manager UUID) to the
// native session ID; direct native IDs pass through untouched.
func (as *AgentState) MapInternalSession(sessionID string) string {
	if sessionID == "" {
		return sessionID
	}
	as.mu.Lock()
	defer as.mu.Unlock()
	for _, s := range as.Sessions {
		if s.ID == sessionID {
			return sessionID
		}
	}
	for _, s := range as.Sessions {
		if s.RemoteSessionID == sessionID {
			return s.ID
		}
	}
	return sessionID
}

// internalManagerID resolves the backing manager session for a native ID.
func (as *AgentState) internalManagerID(sessionID string) string {
	as.mu.Lock()
	defer as.mu.Unlock()
	for _, s := range as.Sessions {
		if s.ID == sessionID && s.RemoteSessionID != "" {
			return s.RemoteSessionID
		}
	}
	return ""
}

// RespondInternalApproval answers a pending tool approval of an internal
// session (RespondAgentApproval binding parity).
func (as *AgentState) RespondInternalApproval(sessionID string, approve, autoApproveAll bool) error {
	if as.engine == nil || as.engine.AgentMgr == nil {
		return nil
	}
	mgrID := as.internalManagerID(sessionID)
	if mgrID == "" {
		return nil
	}
	return as.engine.AgentMgr.RespondApproval(context.Background(), mgrID, approve, autoApproveAll)
}

// RespondInternalAsk answers a structured agent question with option answers
// (RespondAgentAsk binding parity).
func (as *AgentState) RespondInternalAsk(sessionID string, answers map[string]any) error {
	if as.engine == nil || as.engine.AgentMgr == nil {
		return nil
	}
	mgrID := as.internalManagerID(sessionID)
	if mgrID == "" {
		return nil
	}
	return as.engine.AgentMgr.RespondAsk(mgrID, answers)
}

// StopInternalTurn cancels the running turn of an internal session
// (StopAgentTurn binding parity).
func (as *AgentState) StopInternalTurn(sessionID string) {
	if as.engine == nil || as.engine.AgentMgr == nil {
		return
	}
	if mgrID := as.internalManagerID(sessionID); mgrID != "" {
		as.engine.AgentMgr.StopTurn(mgrID)
	}
}

// SteerInternal delivers a mid-turn instruction into the running loop
// (SteerAgentMessage binding parity).
func (as *AgentState) SteerInternal(sessionID, instruction string) error {
	if as.engine == nil || as.engine.AgentMgr == nil {
		return fmt.Errorf("agent engine unavailable")
	}
	mgrID := as.internalManagerID(sessionID)
	if mgrID == "" {
		return fmt.Errorf("no active internal agent session")
	}
	return as.engine.AgentMgr.SteerMessage(mgrID, instruction)
}

// Subagents returns the child agents of an internal session, if any.
func (as *AgentState) Subagents(sessionID string) []*agent.Session {
	if as.engine == nil || as.engine.AgentMgr == nil {
		return nil
	}
	mgrID := as.internalManagerID(sessionID)
	if mgrID == "" {
		return nil
	}
	return as.engine.AgentMgr.GetSubagents(mgrID)
}
