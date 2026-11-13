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

// AgentQuestion is one structured question of an agent:ask payload
// (tools.AskQuestion parity): single- or multi-select chips with an optional
// recommended option and a free-form custom answer.
type AgentQuestion struct {
	ID          string   `json:"id"`
	Question    string   `json:"question"`
	Header      string   `json:"header,omitempty"`
	Options     []string `json:"options"`
	Multi       bool     `json:"multi,omitempty"`
	Recommended int      `json:"recommended,omitempty"` // index into Options, -1 = none

	// UI selection state (not part of the wire payload).
	Selected []string `json:"selected,omitempty"`
	Custom   string   `json:"custom,omitempty"`
}

// AnswersMap builds the RespondAgentAsk answers payload: question id →
// selection(s), including the custom free-text answer when present.
func (q *AgentQuestion) AnswersMap() map[string]any {
	var answer any
	switch {
	case len(q.Selected) > 0 && q.Custom != "":
		picks := append([]string(nil), q.Selected...)
		answer = append(picks, q.Custom)
	case len(q.Selected) > 0:
		if q.Multi {
			answer = append([]string(nil), q.Selected...)
		} else {
			answer = q.Selected[0]
		}
	case q.Custom != "":
		answer = q.Custom
	default:
		return nil
	}
	return map[string]any{q.ID: answer}
}

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

// HandleQuestions surfaces a structured agent:ask (ask tool payload) on the
// streaming message and flips the session to awaiting-input.
func (as *AgentState) HandleQuestions(sessionID string, questions []AgentQuestion) {
	if len(questions) == 0 {
		return
	}
	as.mu.Lock()
	for _, s := range as.Sessions {
		if s.ID != sessionID {
			continue
		}
		s.Status = "awaiting-approval"
		msg := as.ensureStreamingMessage(s)
		msg.PendingQuestions = questions
		as.Messages = s.Messages
		break
	}
	as.mu.Unlock()
	if as.Invalidate != nil {
		as.Invalidate()
	}
}

// PendingQuestions returns the live questions of a session's streaming
// message, if any.
func (as *AgentState) PendingQuestions(sessionID string) []AgentQuestion {
	as.mu.Lock()
	defer as.mu.Unlock()
	for _, s := range as.Sessions {
		if s.ID != sessionID {
			continue
		}
		for i := len(s.Messages) - 1; i >= 0; i-- {
			if len(s.Messages[i].PendingQuestions) > 0 {
				return s.Messages[i].PendingQuestions
			}
		}
		break
	}
	return nil
}

// UpdateQuestion mutates one question's UI selection state, addressed by the
// message-local index.
func (as *AgentState) UpdateQuestion(sessionID string, qIndex int, fn func(q *AgentQuestion)) {
	as.mu.Lock()
	defer as.mu.Unlock()
	for _, s := range as.Sessions {
		if s.ID != sessionID {
			continue
		}
		for i := len(s.Messages) - 1; i >= 0; i-- {
			msg := s.Messages[i]
			if len(msg.PendingQuestions) == 0 {
				continue
			}
			if qIndex >= 0 && qIndex < len(msg.PendingQuestions) {
				fn(&msg.PendingQuestions[qIndex])
			}
			break
		}
		break
	}
	if as.Invalidate != nil {
		as.Invalidate()
	}
}

// SubmitQuestionAnswers collects every question's selection into the
// RespondAgentAsk payload, clears the cards, and resumes the turn.
func (as *AgentState) SubmitQuestionAnswers(sessionID string) error {
	qs := as.PendingQuestions(sessionID)
	if len(qs) == 0 {
		return nil
	}
	answers := map[string]any{}
	for _, q := range qs {
		if m := q.AnswersMap(); m != nil {
			for k, v := range m {
				answers[k] = v
			}
		}
	}
	// Clear the pending cards first so the UI returns to streaming state.
	as.mu.Lock()
	for _, s := range as.Sessions {
		if s.ID != sessionID {
			continue
		}
		for i := len(s.Messages) - 1; i >= 0; i-- {
			if len(s.Messages[i].PendingQuestions) > 0 {
				s.Messages[i].PendingQuestions = nil
				break
			}
		}
		if s.Status == "awaiting-approval" {
			s.Status = "running"
		}
		break
	}
	as.mu.Unlock()
	if as.Invalidate != nil {
		as.Invalidate()
	}
	return as.RespondInternalAsk(sessionID, answers)
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
