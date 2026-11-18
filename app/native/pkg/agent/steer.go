package agent

import (
	"errors"
	"strings"
	"time"
)

// ErrSteerQueuedNextTurn is the honest ACP fallback outcome: the instruction
// could not be injected into the running turn (the transport cannot accept
// mid-turn prompts), so it stays in the session's steer queue and the
// turn-completion path (HandleTurnDone) delivers it as the next turn. The
// steer tray surfaces this verbatim instead of reporting a failure.
var ErrSteerQueuedNextTurn = errors.New("Queued for next turn")

// acpMidTurnSteerSupported reports whether the transport's agent reads user
// messages from its stdin loop continuously, so a new user message written
// mid-turn is delivered to the running conversation (Zed-style steering):
// the Agy (Antigravity) and Claude Code stream-json harnesses both do — the
// same raw user-event line Prompt writes for a normal turn. Plain ACP
// JSON-RPC and the Codex app-server have no defined mid-turn prompt, so they
// take the queue-for-next-turn fallback instead.
func acpMidTurnSteerSupported(protocol string) bool {
	switch strings.ToLower(strings.TrimSpace(protocol)) {
	case "agy-stream", "claude-stream", "stream-json":
		return true
	}
	return false
}

// SteerACP injects text into a running ACP session. Outcomes:
//
//   - Turn in flight on a stream-json transport: the text is written straight
//     into the running CLI's stdin (the session/prompt-equivalent injection
//     path) and a user bubble is appended to the transcript immediately.
//   - No turn in flight: the queued text is popped and run right away as the
//     session's next turn (the Steer button only renders while the UI
//     believes the session is streaming, so this covers flag drift and turns
//     that completed between render and click).
//   - Turn in flight on a transport without mid-turn prompts: ErrSteerQueuedNextTurn
//     — the item stays queued and auto-runs on turn completion.
func (as *AgentState) SteerACP(sessionID, text string) error {
	text = strings.TrimSpace(text)
	if text == "" {
		return errors.New("nothing to steer")
	}

	protocol, running := as.acpSessionTransport(sessionID)

	if running && acpMidTurnSteerSupported(protocol) {
		// Pop BEFORE sending: if turn_end lands while the stdin write is in
		// flight, HandleTurnDone's queue drain would otherwise double-deliver
		// the same instruction (once from the buffered stdin, once as the
		// relaunched turn).
		// Pop first when present so a concurrent turn_end cannot double-
		// deliver the same instruction; a miss (completion drain already
		// took it) still injects — the user explicitly asked to steer.
		popped := as.takeSessionSteerItem(sessionID, text)
		if err := as.promptACPSession(sessionID, text); err != nil {
			if popped {
				as.ensureSessionSteerItem(sessionID, text) // restore for retry
			}
			return err
		}
		as.appendSteerTranscriptBubble(sessionID, text)
		return nil
	}

	if !running {
		// Pop first so the turn-completion drain cannot double-send the same
		// prompt if a turn_end lands concurrently; a false return means the
		// completion path already consumed it (and relaunched the turn).
		if !as.takeSessionSteerItem(sessionID, text) {
			return nil
		}
		as.appendSteerTranscriptBubble(sessionID, text)
		go as.triggerAgentTurnForSession(sessionID, text)
		return nil
	}

	// Mid-turn on a transport that cannot accept prompts: keep the text
	// queued — HandleTurnDone pops it and the caller relaunches the turn
	// with it (app.go OnTurnDone → TriggerTurnForSession).
	as.ensureSessionSteerItem(sessionID, text)
	return ErrSteerQueuedNextTurn
}

// acpSessionTransport resolves the live ACP session's transport protocol and
// whether a turn is currently in flight. "waiting_permission" still counts as
// in flight: the turn resumes once the permission is answered.
func (as *AgentState) acpSessionTransport(sessionID string) (protocol string, running bool) {
	if as.acpTransportProbe != nil {
		return as.acpTransportProbe(sessionID)
	}

	var live *acpSessionLike
	if as.ACPMgr != nil {
		for _, s := range as.ACPMgr.ListSessions() {
			if s != nil && s.ID == sessionID {
				live = &acpSessionLike{Protocol: s.Protocol, Status: s.Status}
				break
			}
		}
	} else if as.engine != nil && as.engine.ACPMgr != nil {
		if s, ok := as.engine.ACPMgr.GetSession(sessionID); ok && s != nil {
			live = &acpSessionLike{Protocol: s.Protocol, Status: s.Status}
		}
	}
	if live == nil {
		return "", false
	}
	return live.Protocol, live.Status == "running" || live.Status == "waiting_permission"
}

// acpSessionLike is the minimal view of a core acp.Session the transport
// probe needs, so tests can fake the lookup without the core package.
type acpSessionLike struct {
	Protocol string
	Status   string
}

// promptACPSession sends the steer text over the live ACP connection. For
// stream transports this writes the user event line into the running CLI's
// stdin — the same write path a normal turn uses.
func (as *AgentState) promptACPSession(sessionID, text string) error {
	if as.ACPMgr != nil {
		return as.ACPMgr.Prompt(sessionID, text)
	}
	if as.engine != nil && as.engine.ACPMgr != nil {
		return as.engine.ACPMgr.Prompt(sessionID, text)
	}
	return errors.New("acp manager unavailable")
}

// appendSteerTranscriptBubble appends the steer text as a completed user
// message on the session transcript (parity with the internal-agent steer
// path and the turn-completion drain) and persists it.
func (as *AgentState) appendSteerTranscriptBubble(sessionID, text string) {
	as.mu.Lock()
	var sess *AgentSession
	for _, s := range as.Sessions {
		if s.ID == sessionID {
			sess = s
			break
		}
	}
	if sess == nil {
		as.mu.Unlock()
		return
	}
	// Finalize an open streaming assistant message before appending the user
	// bubble: ensureStreamingMessage only treats a trailing assistant !Done
	// message as "the streaming one", so leaving it open under a new user
	// bubble would orphan it as a perpetually-"in progress" bubble while
	// the original turn's remaining deltas open a fresh bubble below.
	if n := len(sess.Messages); n > 0 {
		if last := sess.Messages[n-1]; last != nil && last.Role == "assistant" && !last.Done {
			last.Done = true
			last.IsThinking = false
			last.ShowThinking = false
			last.ShowTools = false
		}
	}
	sess.Messages = append(sess.Messages, &ChatMessage{
		ID:        nextMessageIDLocked(sess),
		Role:      "user",
		Text:      text,
		Done:      true,
		Timestamp: time.Now(),
	})
	sess.UpdatedAt = time.Now()
	if sess.ID == as.ActiveSessionID {
		as.Messages = sess.Messages
	}
	as.mu.Unlock()

	goSave("SaveSessionToDisk", func() { as.SaveSessionToDisk(sess) })
	if as.Invalidate != nil {
		as.Invalidate()
	}
}

// takeSessionSteerItem removes the first queue entry equal to text from the
// session's queue (and its foreground mirror when the session is active),
// returning whether it was removed.
func (as *AgentState) takeSessionSteerItem(sessionID, text string) bool {
	as.mu.Lock()
	removed := false
	for _, s := range as.Sessions {
		if s.ID != sessionID {
			continue
		}
		for i, it := range s.SteerQueue {
			if it == text {
				s.SteerQueue = append(s.SteerQueue[:i], s.SteerQueue[i+1:]...)
				removed = true
				break
			}
		}
		if removed && s.ID == as.ActiveSessionID {
			as.SteerQueue = append([]string(nil), s.SteerQueue...)
		}
		break
	}
	var snapshot []string
	if removed {
		for _, s := range as.Sessions {
			if s.ID == sessionID {
				snapshot = append([]string(nil), s.SteerQueue...)
				break
			}
		}
	}
	as.mu.Unlock()

	if removed {
		as.saveSteerQueuesSlice(snapshot)
	}
	return removed
}

// ensureSessionSteerItem keeps the text present on the session's steer queue
// (and the foreground mirror) so the turn-completion drain delivers it.
func (as *AgentState) ensureSessionSteerItem(sessionID, text string) {
	as.mu.Lock()
	present := false
	for _, s := range as.Sessions {
		if s.ID != sessionID {
			continue
		}
		for _, it := range s.SteerQueue {
			if it == text {
				present = true
				break
			}
		}
		if !present {
			s.SteerQueue = append(s.SteerQueue, text)
		}
		if s.ID == as.ActiveSessionID {
			as.SteerQueue = append([]string(nil), s.SteerQueue...)
		}
		snapshot := append([]string(nil), s.SteerQueue...)
		as.mu.Unlock()
		as.saveSteerQueuesSlice(snapshot)
		return
	}
	as.mu.Unlock()
}
