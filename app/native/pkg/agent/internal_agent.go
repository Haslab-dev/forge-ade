package agent

// internal_agent.go routes ForgeADE Internal sessions through the RakitKode
// agent.Manager — the same full engine (tool loop, approvals, asks, subagents,
// schedules, background tasks, checkpoints) the webview app uses. The manager
// session UUID is stored on AgentSession.RemoteSessionID; bus events carry the
// manager ID, which MapInternalSession translates back to the native session.

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/Haslab-dev/RakitKode/agent"
	internalgit "github.com/hasdev/forge-ade/core/git"
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

// resolvePersonaKey resolves the persona key + role from plain inputs the
// caller captured under as.mu. It takes NO locks and consults the RakitKode
// AgentMgr freely — callers must invoke it with as.mu RELEASED (P0-1 layer 3:
// the RK streaming callback holds the manager lock while publishing bus
// events whose handlers take as.mu, so AgentMgr under as.mu can deadlock).
func (as *AgentState) resolvePersonaKey(personaID string, planMode bool) (string, agent.RoleFilter) {
	role := agent.RoleCoding
	var orchCfg agent.OrchestrationConfig
	if as.engine != nil && as.engine.AgentMgr != nil {
		orchCfg = as.engine.AgentMgr.GetOrchestrationConfig()
	}
	if planMode {
		return string(agent.RolePlanning), agent.RolePlanning
	}
	if personaID != "" {
		if as.engine != nil && as.engine.AgentMgr != nil {
			if def, ok := as.engine.AgentMgr.FindDefinition(personaID); ok && def.Role != "" {
				return personaID, def.Role
			}
		}
		return personaID, role
	}
	if orchCfg.Enabled {
		defRole := orchCfg.DefaultRole
		if defRole == "" {
			defRole = agent.RoleOrchestrator
		}
		return string(defRole), defRole
	}
	return string(agent.RoleOrchestrator), agent.RoleOrchestrator
}

// SetSessionPersona sets the persona ID for a session and clears applied state so
// the next turn applies the updated persona definition cleanly.
func (as *AgentState) SetSessionPersona(sessionID, personaID string) {
	if as == nil {
		return
	}
	var updated *AgentSession
	as.mu.Lock()
	as.ActivePersonaID = personaID
	for _, s := range as.Sessions {
		if s.ID == sessionID {
			s.PersonaID = personaID
			s.appliedPersonaKey = ""
			updated = s
			break
		}
	}
	as.mu.Unlock()
	if updated != nil {
		as.SaveSessionToDisk(updated)
	}
}

// EffectivePersona returns the resolved persona ID and display title for sess
// (falling back to the global orchestration config default role when enabled).
// Called from render — lock-light: persona inputs are snapshotted under
// as.mu, then AgentMgr is consulted with the lock released (P0-1 layer 3).
func (as *AgentState) EffectivePersona(sess *AgentSession) (string, string) {
	if as == nil {
		return "coding", "Coding Agent"
	}
	as.mu.Lock()
	personaID := ""
	if sess != nil && sess.PersonaID != "" {
		personaID = sess.PersonaID
	} else if as.ActivePersonaID != "" {
		personaID = as.ActivePersonaID
	}
	planMode := as.ExecutionMode == ModePlan
	as.mu.Unlock()

	key, _ := as.resolvePersonaKey(personaID, planMode)
	if as.engine != nil && as.engine.AgentMgr != nil && key != "" {
		if def, ok := as.engine.AgentMgr.FindDefinition(key); ok {
			name := def.Name
			if name == "" {
				name = string(def.Role)
			}
			return def.ID, name
		}
	}
	if key == "" {
		key = "coding"
	}
	return key, strings.ToUpper(key[:1]) + key[1:]
}

func (as *AgentState) ensureInternalManagerSession(sess *AgentSession) (string, error) {
	if as.engine == nil || as.engine.AgentMgr == nil {
		return "", fmt.Errorf("agent engine unavailable")
	}
	// Snapshot persona/workspace inputs under as.mu; every AgentMgr call
	// below happens with the lock released and only the final apply re-takes
	// it briefly (P0-1 layer 3).
	as.mu.Lock()
	remoteID := sess.RemoteSessionID
	title := sess.Title
	ws := sess.WorkspacePath
	if ws == "" {
		ws = as.workspacePath
	}
	personaID := ""
	if sess.PersonaID != "" {
		personaID = sess.PersonaID
	} else if as.ActivePersonaID != "" {
		personaID = as.ActivePersonaID
	}
	planMode := as.ExecutionMode == ModePlan
	as.mu.Unlock()

	targetKey, role := as.resolvePersonaKey(personaID, planMode)

	if remoteID != "" {
		if _, ok := as.engine.AgentMgr.GetSession(remoteID); ok {
			return remoteID, nil
		}
	}
	if ws == "" && as.engine != nil {
		ws = as.engine.DataDir
	}
	mgrSess, err := as.engine.AgentMgr.CreateSession(title, role, ws)
	if err != nil {
		return "", err
	}
	appliedKey := ""
	if targetKey != "" {
		if def, ok := as.engine.AgentMgr.FindDefinition(targetKey); ok {
			_ = as.engine.AgentMgr.ApplyDefinitionToSession(mgrSess.ID, def.ID)
			appliedKey = targetKey
		}
	}
	as.mu.Lock()
	sess.RemoteSessionID = mgrSess.ID
	if appliedKey != "" {
		sess.appliedPersonaKey = appliedKey
	}
	as.internalSessionMap.Store(mgrSess.ID, sess.ID)
	as.internalSessionMap.Store(sess.ID, sess.ID)
	as.mu.Unlock()
	return mgrSess.ID, nil
}

// internalTurnSnapshot carries everything a turn needs from AgentState,
// captured under as.mu in one pass so no AgentMgr/LLMClient call ever runs
// while the state lock is held (P0-1 layer 3).
type internalTurnSnapshot struct {
	appliedPersonaKey string
	personaID         string
	planMode          bool
	sessModel         string
	sessProvider      string
	activeModel       string
	activeProvider    string
	permissionMode    string
	autoApprove       bool
	mentions          []string
	wsPath            string
}

// syncInternalSessionState pushes the current model / execution mode onto the
// backing manager session before each turn. It takes NO locks: all AgentState
// inputs arrive via snap, and every AgentMgr/LLMClient call happens with
// as.mu released (P0-1 layer 3). Returns the values the caller must apply
// back to the session under as.mu.
func (as *AgentState) syncInternalSessionState(mgrID string, snap *internalTurnSnapshot) (appliedKey, activeModel string) {
	if as.engine == nil || as.engine.AgentMgr == nil {
		return "", ""
	}
	targetKey, targetRole := as.resolvePersonaKey(snap.personaID, snap.planMode)
	personaModel := ""
	if targetKey != "" && targetKey != snap.appliedPersonaKey {
		if def, ok := as.engine.AgentMgr.FindDefinition(targetKey); ok {
			_ = as.engine.AgentMgr.ApplyDefinitionToSession(mgrID, def.ID)
			personaModel = def.Model
		} else {
			_, _ = as.engine.AgentMgr.UpdateSession(mgrID, "", targetRole, "", "")
		}
		appliedKey = targetKey
	}

	activeModel = snap.sessModel
	if activeModel == "" && personaModel != "" {
		activeModel = personaModel
	}
	if activeModel == "" && snap.activeModel != "" {
		activeModel = snap.activeModel
	}
	if as.engine.LLMClient != nil {
		cfg := as.engine.LLMClient.GetConfig()
		if activeModel == "" {
			activeModel = cfg.Model
		} else {
			prov := snap.sessProvider
			if prov == "" {
				prov = snap.activeProvider
			}
			if prov == "" {
				prov = as.engine.LLMClient.FindProviderForModel(activeModel)
			}
			if prov != "" {
				as.engine.LLMClient.SetActiveModel(prov, activeModel)
			} else {
				as.engine.LLMClient.SetActiveModelByID(activeModel)
			}
		}
	}
	if activeModel != "" {
		_ = as.engine.AgentMgr.SetSessionModel(mgrID, activeModel)
	}
	// Always use native function calling by default for standard OpenAI/Claude/DeepSeek compatibility.
	_ = as.engine.AgentMgr.SetDialect(mgrID, "")
	if snap.permissionMode != "" {
		_ = as.engine.AgentMgr.SetPermissionMode(mgrID, snap.permissionMode)
	}
	_ = as.engine.AgentMgr.SetAutoApprove(mgrID, snap.autoApprove)
	return appliedKey, activeModel
}

// TriggerInternalTurn sends a user prompt through the RakitKode agent engine.
// Streaming events (agent:message_delta, agent:tool_*, agent:turn_end, ...)
// arrive on the shared bus and are bridged into Handle* by ACPManager.
func (as *AgentState) TriggerInternalTurn(sess *AgentSession, prompt string, attached ...string) error {
	mgrID, err := as.ensureInternalManagerSession(sess)
	if err != nil {
		return err
	}

	// One lock pass to snapshot everything, then AgentMgr/LLMClient run
	// strictly outside as.mu; only the short apply-back re-takes it (P0-1
	// layer 3: TriggerInternalTurn runs on a turn goroutine while the
	// streaming callback publishes events whose handlers need as.mu).
	as.mu.Lock()
	snap := &internalTurnSnapshot{
		appliedPersonaKey: sess.appliedPersonaKey,
		personaID:         sess.PersonaID,
		planMode:          as.ExecutionMode == ModePlan,
		sessModel:         sess.Model,
		sessProvider:      sess.Provider,
		activeModel:       as.ActiveModel,
		activeProvider:    as.ActiveProvider,
		permissionMode:    as.ExecutionMode.InternalPermissionMode(),
		autoApprove:       as.ExecutionMode == ModeBypass,
		wsPath:            sess.WorkspacePath,
	}
	mentions := append([]string(nil), attached...)
	if len(mentions) == 0 && len(as.AttachedFiles) > 0 {
		mentions = append([]string(nil), as.AttachedFiles...)
	}
	snap.mentions = mentions
	turnID := fmt.Sprintf("turn-%d", time.Now().UnixMilli())
	sess.LastCheckpointTurnID = turnID
	as.mu.Unlock()

	if as.engine == nil || as.engine.AgentMgr == nil {
		return fmt.Errorf("agent engine unavailable")
	}

	appliedKey, activeModel := as.syncInternalSessionState(mgrID, snap)
	as.mu.Lock()
	if appliedKey != "" {
		sess.appliedPersonaKey = appliedKey
	}
	if activeModel != "" {
		sess.Model = activeModel
	}
	as.mu.Unlock()

	wsPath := snap.wsPath
	if as.engine.GitEngine != nil && wsPath != "" && !strings.Contains(wsPath, ".forge") {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		_, _ = internalgit.CreateCheckpoint(ctx, wsPath, sess.ID, turnID, prompt)
		cancel()
	}

	// The manager owns turn lifetime (interrupt via StopTurn); the app
	// context stays alive like the webview's app-level ctx.
	return as.engine.AgentMgr.SendMessage(context.Background(), mgrID, prompt, snap.mentions)
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
	if v, ok := as.internalSessionMap.Load(sessionID); ok {
		return v.(string)
	}
	as.mu.Lock()
	defer as.mu.Unlock()
	for _, s := range as.Sessions {
		if s.ID == sessionID {
			as.internalSessionMap.Store(sessionID, sessionID)
			return sessionID
		}
	}
	for _, s := range as.Sessions {
		if s.RemoteSessionID == sessionID {
			as.internalSessionMap.Store(sessionID, s.ID)
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
// session (RespondAgentApproval binding parity). A missing engine session is
// an error, not a silent success: the approval bar must stay up so the user
// can retry.
func (as *AgentState) RespondInternalApproval(sessionID string, approve, autoApproveAll bool) error {
	if as.engine == nil || as.engine.AgentMgr == nil {
		return fmt.Errorf("agent engine unavailable: approval for session %s was not delivered", sessionID)
	}
	mgrID := as.internalManagerID(sessionID)
	if mgrID == "" {
		return fmt.Errorf("internal session %s has no connected agent session: approval was not delivered", sessionID)
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
		if s.ID == as.ActiveSessionID {
			as.Messages = s.Messages
		}
		break
	}
	as.mu.Unlock()
	if as.Invalidate != nil {
		as.Invalidate()
	}
}

// PendingQuestions returns a copy of the questions of a session's streaming
// message, if any. Render reads must not alias the live slice: the UI mutates
// selections through UpdateQuestion under the lock.
func (as *AgentState) PendingQuestions(sessionID string) []AgentQuestion {
	as.mu.Lock()
	defer as.mu.Unlock()
	for _, s := range as.Sessions {
		if s.ID != sessionID {
			continue
		}
		for i := len(s.Messages) - 1; i >= 0; i-- {
			if qs := s.Messages[i].PendingQuestions; len(qs) > 0 {
				out := make([]AgentQuestion, len(qs))
				for j, q := range qs {
					qc := q
					qc.Options = append([]string(nil), q.Options...)
					qc.Selected = append([]string(nil), q.Selected...)
					out[j] = qc
				}
				return out
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
	as.mu.Unlock()
	if as.Invalidate != nil {
		as.Invalidate()
	}
}

// SubmitQuestionAnswers collects every question's selection into the
// RespondAgentAsk payload, clears the cards, and resumes the turn. On a
// failed respond it restores the questions and the awaiting status: the
// manager is still blocked on the ask, and leaving the cards cleared would
// hang the turn forever.
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
	// Clear the pending cards first so the UI returns to streaming state,
	// remembering what was cleared for the failure path below.
	clearedMsgID := 0
	prevStatus := ""
	as.mu.Lock()
	for _, s := range as.Sessions {
		if s.ID != sessionID {
			continue
		}
		prevStatus = s.Status
		for i := len(s.Messages) - 1; i >= 0; i-- {
			if len(s.Messages[i].PendingQuestions) > 0 {
				clearedMsgID = s.Messages[i].ID
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

	if err := as.RespondInternalAsk(sessionID, answers); err != nil {
		// Restore the questions and the previous status so the user can fix
		// and retry (the agent keeps waiting for this ask).
		as.mu.Lock()
		for _, s := range as.Sessions {
			if s.ID != sessionID {
				continue
			}
			for i := len(s.Messages) - 1; i >= 0; i-- {
				if s.Messages[i].ID == clearedMsgID {
					s.Messages[i].PendingQuestions = qs
					break
				}
			}
			if prevStatus != "" {
				s.Status = prevStatus
			}
			break
		}
		as.mu.Unlock()
		if as.Invalidate != nil {
			as.Invalidate()
		}
		return err
	}
	return nil
}

// RespondInternalAsk answers a structured agent question with option answers
// (RespondAgentAsk binding parity). It returns a descriptive error instead of
// a silent nil when the backing engine session is missing: the caller must
// know the answers were NOT delivered, or the turn hangs forever.
func (as *AgentState) RespondInternalAsk(sessionID string, answers map[string]any) error {
	if as.engine == nil || as.engine.AgentMgr == nil {
		return fmt.Errorf("agent engine unavailable: answers for session %s were not delivered", sessionID)
	}
	mgrID := as.internalManagerID(sessionID)
	if mgrID == "" {
		// The ask may belong to a skill sub-agent without a native session
		// mapping — deliver the answers to whichever engine session is
		// actually blocked on questions, or the turn hangs forever.
		mgrID = as.findAwaitingAskSession()
	}
	if mgrID == "" {
		return fmt.Errorf("internal session %s has no connected agent session: answers were not delivered", sessionID)
	}
	return as.engine.AgentMgr.RespondAsk(mgrID, answers)
}

// findAwaitingAskSession finds the engine session currently blocked on ask
// questions (any session, including skill sub-agents whose native mapping is
// absent).
func (as *AgentState) findAwaitingAskSession() string {
	if as.engine == nil || as.engine.AgentMgr == nil {
		return ""
	}
	for _, sess := range as.engine.AgentMgr.ListSessions() {
		if sess != nil && sess.State == agent.StateAwaitingInput {
			return sess.ID
		}
	}
	return ""
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

// SteerInternal delivers a mid-turn instruction into the running session,
// routing by session kind (SteerAgentMessage binding parity): internal
// sessions inject into the running RakitKode loop (AgentMgr.SteerMessage);
// ACP sessions go through the ACP steer path (SteerACP), which either
// injects into the running harness or honestly reports the
// queued-for-next-turn fallback. The steer tray calls this for every session
// kind, so routing must live here.
func (as *AgentState) SteerInternal(sessionID, instruction string) error {
	agentID, agentType, known := as.sessionAgentKind(sessionID)
	if !known {
		return fmt.Errorf("no active agent session")
	}
	if agentType != "internal" && isACPAgentID(agentID, agentType) {
		return as.SteerACP(sessionID, instruction)
	}
	if as.engine == nil || as.engine.AgentMgr == nil {
		return fmt.Errorf("agent engine unavailable")
	}
	mgrID := as.internalManagerID(sessionID)
	if mgrID == "" {
		return fmt.Errorf("no active internal agent session")
	}
	return as.engine.AgentMgr.SteerMessage(mgrID, instruction)
}

// sessionAgentKind reports a session's agent id/type and whether the session
// exists at all.
func (as *AgentState) sessionAgentKind(sessionID string) (agentID, agentType string, known bool) {
	as.mu.Lock()
	defer as.mu.Unlock()
	for _, s := range as.Sessions {
		if s.ID == sessionID {
			return s.AgentID, s.AgentType, true
		}
	}
	return "", "", false
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

// RestoreLastTurnCheckpoint rolls the workspace back to the snapshot taken
// before the session's last agent turn (RestoreSessionCheckpoint parity,
// powering the chat review bar's Reject all).
func (as *AgentState) RestoreLastTurnCheckpoint(sessionID string) error {
	if as.engine == nil || as.engine.GitEngine == nil {
		return fmt.Errorf("git engine unavailable")
	}
	as.mu.Lock()
	var wsPath, turnID string
	for _, s := range as.Sessions {
		if s.ID == sessionID {
			wsPath = s.WorkspacePath
			turnID = s.LastCheckpointTurnID
			break
		}
	}
	as.mu.Unlock()
	if wsPath == "" || turnID == "" {
		return fmt.Errorf("no checkpoint for this turn")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	err := internalgit.RestoreCheckpoint(ctx, wsPath, sessionID, turnID)
	if err == nil && as.engine != nil {
		as.engine.GitEngine.Invalidate(wsPath)
	}
	return err
}
