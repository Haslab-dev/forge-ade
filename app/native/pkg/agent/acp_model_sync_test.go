package agent

import (
	"testing"
	"time"

	"github.com/hasdev/forge-ade/app/native/pkg/engine"
)

// waitForAsyncSaves lets syncACPSessionState's background persistence (goSave)
// finish before t.TempDir cleanup races it.
func waitForAsyncSaves() {
	time.Sleep(100 * time.Millisecond)
}

// The effective ACP model (what the agent process was actually configured
// with after normalization) must be written back to the session and surfaced
// as ActiveModel — the UI shows the real current model, never a stale or
// guessed selection.
func TestSyncACPSessionStateWritesBackEffectiveModel(t *testing.T) {
	tempDir := t.TempDir()
	eng := engine.NewEngine(tempDir)
	eng.ACPMgr = nil
	as := NewAgentState(eng, tempDir, func() {})
	as.ACPMgr = nil

	sess := as.NewSession("Model Sync")
	as.SelectSession(sess.ID)
	as.mu.Lock()
	sess.AgentID = "agent-agy"
	sess.AgentType = "acp"
	sess.Model = "stale-selection"
	as.mu.Unlock()
	as.mu.Lock()
	as.ActiveModel = "stale-selection"
	as.mu.Unlock()

	as.syncACPSessionState(sess, "gemini-3.8-flash-high", "conv-123")

	as.mu.Lock()
	sessModel := sess.Model
	sessRemote := sess.RemoteSessionID
	activeModel := as.ActiveModel
	as.mu.Unlock()

	if sessModel != "gemini-3.8-flash-high" {
		t.Fatalf("expected session model written back, got %q", sessModel)
	}
	if sessRemote != "conv-123" {
		t.Fatalf("expected remote session id written back, got %q", sessRemote)
	}
	if activeModel != "gemini-3.8-flash-high" {
		t.Fatalf("expected ActiveModel to track the real model, got %q", activeModel)
	}
	if got := as.SessionModel(sess.ID); got != "gemini-3.8-flash-high" {
		t.Fatalf("SessionModel accessor: got %q", got)
	}
	if got := as.SessionModel("sess-missing"); got != "" {
		t.Fatalf("SessionModel for unknown session should be empty, got %q", got)
	}
	waitForAsyncSaves()
}

// Internal-handle session ids must never be stored as remote conversation
// ids (they are local handles, not agent conversations).
func TestSyncACPSessionStateIgnoresLocalHandleIDs(t *testing.T) {
	tempDir := t.TempDir()
	eng := engine.NewEngine(tempDir)
	eng.ACPMgr = nil
	as := NewAgentState(eng, tempDir, func() {})
	as.ACPMgr = nil

	sess := as.NewSession("Local Handle")
	as.SelectSession(sess.ID)
	as.mu.Lock()
	sess.AgentID = "agent-agy"
	sess.AgentType = "acp"
	as.mu.Unlock()

	as.syncACPSessionState(sess, "", "session-1234")
	as.mu.Lock()
	remote := sess.RemoteSessionID
	as.mu.Unlock()
	if remote != "" {
		t.Fatalf("local session- handle must not become RemoteSessionID, got %q", remote)
	}

	as.syncACPSessionState(sess, "", "acp-5678")
	as.mu.Lock()
	remote = sess.RemoteSessionID
	as.mu.Unlock()
	if remote != "" {
		t.Fatalf("local acp- handle must not become RemoteSessionID, got %q", remote)
	}
	waitForAsyncSaves()
}
