package agent

import (
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/hasdev/forge-ade/app/native/pkg/engine"
)

func newSteerTestState(t *testing.T) (*AgentState, *engine.Engine) {
	t.Helper()
	tempDir := t.TempDir()
	eng := engine.NewEngine(tempDir)
	// Hermetic: no live ACP manager, so a mis-routed steer can never spawn a
	// real CLI or touch the network.
	eng.ACPMgr = nil
	as := NewAgentState(eng, tempDir, func() {})
	as.ACPMgr = nil
	return as, eng
}

func newSteerSession(t *testing.T, as *AgentState, agentID, agentType string) *AgentSession {
	t.Helper()
	sess := as.NewSession("Steer " + agentID)
	as.mu.Lock()
	sess.AgentID = agentID
	sess.AgentType = agentType
	as.mu.Unlock()
	return sess
}

// SteerInternal must route ACP sessions to the ACP steer path, not the
// RakitKode internal loop (the user-reported bug: ACP steer always failed).
func TestSteerInternalRoutesACPToSteerACP(t *testing.T) {
	as, _ := newSteerTestState(t)
	sess := newSteerSession(t, as, "agent-agy", "acp")
	as.SelectSession(sess.ID)

	// Mid-turn on the Agy stream transport, with no ACP manager wired: the
	// injection path is attempted and its (hermetic) error surfaces — proof
	// the call left the internal branch (which would say "no active internal
	// agent session").
	as.acpTransportProbe = func(sessionID string) (string, bool) {
		if sessionID != sess.ID {
			t.Errorf("probe called with unexpected session %q", sessionID)
		}
		return "agy-stream", true
	}

	err := as.SteerInternal(sess.ID, "focus on the parser")
	if err == nil {
		t.Fatalf("expected the ACP injection attempt to error without an ACP manager")
	}
	if !strings.Contains(err.Error(), "acp") {
		t.Fatalf("expected an ACP-path error, got %q", err.Error())
	}
}

// Mid-turn on a transport without mid-turn prompts keeps the item queued and
// reports the queued-for-next-turn outcome instead of failing.
func TestSteerACPQueuesForNextTurnOnUnsupportedTransport(t *testing.T) {
	as, _ := newSteerTestState(t)
	sess := newSteerSession(t, as, "agent-opencode", "acp")
	as.SelectSession(sess.ID)

	// The item is queued the way SubmitMessage does mid-turn.
	as.mu.Lock()
	sess.Status = "running"
	sess.SteerQueue = []string{"check the diffs first"}
	as.SteerQueue = append([]string(nil), sess.SteerQueue...)
	as.mu.Unlock()

	as.acpTransportProbe = func(string) (string, bool) {
		return "acp", true // standard JSON-RPC ACP: no mid-turn prompt
	}

	err := as.SteerInternal(sess.ID, "check the diffs first")
	if !errors.Is(err, ErrSteerQueuedNextTurn) {
		t.Fatalf("expected ErrSteerQueuedNextTurn, got %v", err)
	}

	as.mu.Lock()
	stillQueued := len(sess.SteerQueue) == 1 && sess.SteerQueue[0] == "check the diffs first"
	mirrorQueued := len(as.SteerQueue) == 1 && as.SteerQueue[0] == "check the diffs first"
	as.mu.Unlock()
	if !stillQueued || !mirrorQueued {
		t.Fatalf("expected item to stay queued (session=%v mirror=%v)", sess.SteerQueue, as.SteerQueue)
	}

	// The turn-completion drain consumes it and hands it to the next turn.
	next := as.HandleTurnDone(sess.ID, TurnEndInfo{Status: "completed"})
	if next != "check the diffs first" {
		t.Fatalf("expected HandleTurnDone to drain the queued steer, got %q", next)
	}
}

// With no turn in flight, steering runs the queued prompt right away as the
// session's next turn, appending the user bubble like the internal path.
func TestSteerACPRunsImmediatelyWhenIdle(t *testing.T) {
	as, _ := newSteerTestState(t)
	sess := newSteerSession(t, as, "agent-agy", "acp")
	as.SelectSession(sess.ID)

	as.mu.Lock()
	sess.Status = "idle"
	sess.SteerQueue = []string{"now add tests"}
	as.SteerQueue = append([]string(nil), sess.SteerQueue...)
	as.mu.Unlock()

	as.acpTransportProbe = func(string) (string, bool) {
		return "agy-stream", false // no live turn
	}

	err := as.SteerInternal(sess.ID, "now add tests")
	if err != nil {
		t.Fatalf("expected idle steer to succeed, got %v", err)
	}

	as.mu.Lock()
	queueEmpty := len(sess.SteerQueue) == 0 && len(as.SteerQueue) == 0
	bubble := false
	for _, m := range sess.Messages {
		if m.Role == "user" && m.Text == "now add tests" && m.Done {
			bubble = true
		}
	}
	as.mu.Unlock()
	if !queueEmpty {
		t.Fatalf("expected queue drained after immediate run, session=%v mirror=%v", sess.SteerQueue, as.SteerQueue)
	}
	if !bubble {
		t.Fatalf("expected a user transcript bubble for the steer text")
	}

	// The detached turn consumes the queue entry exactly once: a concurrent
	// completion drain must not find it again.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && sess.Status != "error" && sess.Status != "idle" {
		time.Sleep(5 * time.Millisecond)
	}
	if next := as.HandleTurnDone(sess.ID, TurnEndInfo{Status: "completed"}); next != "" {
		t.Fatalf("expected no second drain of the steer item, got %q", next)
	}
}

// Real mid-turn injection: the stream transports attempt the prompt on the
// running session.
func TestSteerACPInjectsMidTurnOnStreamTransport(t *testing.T) {
	as, eng := newSteerTestState(t)
	sess := newSteerSession(t, as, "agent-agy", "acp")
	as.SelectSession(sess.ID)

	as.mu.Lock()
	sess.Status = "running"
	sess.SteerQueue = []string{"also update the changelog"}
	as.SteerQueue = append([]string(nil), sess.SteerQueue...)
	as.mu.Unlock()

	as.acpTransportProbe = func(string) (string, bool) {
		return "agy-stream", true
	}

	err := as.SteerACP(sess.ID, "also update the changelog")
	if err == nil {
		t.Fatalf("expected the injection attempt to be made (error expected with no ACP manager wired)")
	}

	// A failed injection keeps the queued text (never silently lost) and does
	// not append a transcript bubble for an undelivered message.
	as.mu.Lock()
	stillQueued := len(sess.SteerQueue) == 1 && sess.SteerQueue[0] == "also update the changelog"
	as.mu.Unlock()
	if !stillQueued {
		t.Fatalf("expected queued text preserved after failed injection, got %v", sess.SteerQueue)
	}
	as.mu.Lock()
	for _, m := range sess.Messages {
		if m.Role == "user" && m.Text == "also update the changelog" {
			t.Fatalf("transcript bubble must not be appended for an undelivered steer")
		}
	}
	as.mu.Unlock()
	_ = eng
}

// Steering an unknown session is a descriptive error, never a crash.
func TestSteerInternalUnknownSession(t *testing.T) {
	as, _ := newSteerTestState(t)
	err := as.SteerInternal("sess-does-not-exist", "hello")
	if err == nil {
		t.Fatalf("expected an error for an unknown session")
	}
	if !strings.Contains(err.Error(), "no active agent session") {
		t.Fatalf("expected 'no active agent session', got %q", err.Error())
	}
}

// Internal sessions keep using the RakitKode steer path.
func TestSteerInternalKeepsInternalRouting(t *testing.T) {
	as, _ := newSteerTestState(t)
	sess := newSteerSession(t, as, "agent-internal", "internal")
	as.SelectSession(sess.ID)
	// No remote manager id: the internal branch reports its specific error —
	// the ACP probe must never be consulted.
	as.acpTransportProbe = func(string) (string, bool) {
		t.Fatalf("ACP transport probe must not be consulted for internal sessions")
		return "", false
	}
	err := as.SteerInternal(sess.ID, "focus up")
	if err == nil {
		t.Fatalf("expected the internal-path error without a connected RK session")
	}
	if !strings.Contains(err.Error(), "internal") {
		t.Fatalf("expected an internal-path error, got %q", err.Error())
	}
}

// Mid-turn stream transports treat a permission gate as turn-in-flight.
func TestACPMidTurnSteerProtocolSupport(t *testing.T) {
	supported := []string{"agy-stream", "claude-stream", "stream-json", "AGY-STREAM"}
	for _, p := range supported {
		if !acpMidTurnSteerSupported(p) {
			t.Errorf("expected protocol %q to support mid-turn steer", p)
		}
	}
	unsupported := []string{"acp", "codex-appserver", "", "jsonrpc"}
	for _, p := range unsupported {
		if acpMidTurnSteerSupported(p) {
			t.Errorf("expected protocol %q to fall back to next-turn queue", p)
		}
	}
}
