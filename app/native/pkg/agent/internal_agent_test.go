package agent

import (
	"os"
	"testing"
	"time"

	rakitagent "github.com/Haslab-dev/RakitKode/agent"
	"github.com/hasdev/forge-ade/app/native/pkg/engine"
)

func TestInternalAgentNoDeadlock(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "internal-agent-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	eng := engine.NewEngine(tempDir)
	as := NewAgentState(eng, tempDir, func() {})

	// Create internal session
	sess := as.NewSession("Internal Test")
	sess.AgentID = "agent-internal"
	sess.AgentType = "internal"
	sess.Model = "mimo-v2-6-flash"

	as.mu.Lock()
	as.ActiveAgentID = "agent-internal"
	as.ActiveAgentType = "internal"
	as.ActiveModel = "mimo-v2-6-flash"
	as.mu.Unlock()

	// 1. ensureInternalManagerSession must create a valid backing manager session
	mgrID, err := as.ensureInternalManagerSession(sess)
	if err != nil {
		t.Fatalf("ensureInternalManagerSession failed: %v", err)
	}
	if mgrID == "" {
		t.Fatalf("expected non-empty manager ID")
	}

	// 2. syncInternalSessionState must not deadlock (previously deadlocked on as.mu.Lock)
	done := make(chan bool, 1)
	go func() {
		as.mu.Lock()
		as.syncInternalSessionState(mgrID, sess)
		as.mu.Unlock()
		done <- true
	}()

	select {
	case <-done:
		// Passed, no deadlock!
	case <-time.After(2 * time.Second):
		t.Fatalf("syncInternalSessionState deadlocked!")
	}

	// 3. MapInternalSession must map correctly
	mapped := as.MapInternalSession(mgrID)
	if mapped != sess.ID {
		t.Errorf("expected mapped session ID %s, got %s", sess.ID, mapped)
	}

	// 4. IsInternalSession must return true
	if !as.IsInternalSession(sess.ID) {
		t.Errorf("expected IsInternalSession(%s) to be true", sess.ID)
	}

	// 5. internalManagerID must return mgrID
	if id := as.internalManagerID(sess.ID); id != mgrID {
		t.Errorf("expected internalManagerID %s, got %s", mgrID, id)
	}

	// 6. CancelStreaming must not deadlock
	as.mu.Lock()
	as.IsStreaming = true
	as.ActiveSessionID = sess.ID
	as.mu.Unlock()

	cancelDone := make(chan bool, 1)
	go func() {
		as.CancelStreaming()
		cancelDone <- true
	}()

	select {
	case <-cancelDone:
		// Passed
	case <-time.After(2 * time.Second):
		t.Fatalf("CancelStreaming deadlocked!")
	}

	// 7. HandleTurnDone must not deadlock
	turnDone := make(chan bool, 1)
	go func() {
		as.HandleTurnDone(sess.ID, TurnEndInfo{
			Status: "completed",
		})
		turnDone <- true
	}()

	select {
	case <-turnDone:
		// Passed
	case <-time.After(2 * time.Second):
		t.Fatalf("HandleTurnDone deadlocked!")
	}
}

func TestInternalAgentOrchestratorAndPersona(t *testing.T) {
	tempDir := t.TempDir()
	t.Setenv("HOME", tempDir)

	eng := engine.NewEngine(tempDir)
	as := NewAgentState(eng, tempDir, func() {})
	sess := as.NewSession("Orchestrator Test")
	sess.AgentID = "agent-internal"
	sess.AgentType = "internal"

	as.mu.Lock()
	as.ActiveAgentID = "agent-internal"
	as.ActiveAgentType = "internal"
	as.mu.Unlock()

	if pid, _ := as.EffectivePersona(sess); pid != "orchestrator" {
		t.Fatalf("expected default EffectivePersona orchestrator, got %q", pid)
	}

	mgrID, err := as.ensureInternalManagerSession(sess)
	if err != nil {
		t.Fatalf("ensureInternalManagerSession failed: %v", err)
	}
	mgrSess, ok := eng.AgentMgr.GetSession(mgrID)
	if !ok {
		t.Fatalf("expected backing session %s to exist", mgrID)
	}
	if mgrSess.RoleFilter != rakitagent.RoleOrchestrator {
		t.Fatalf("expected manager session role %q, got %q", rakitagent.RoleOrchestrator, mgrSess.RoleFilter)
	}

	// Switching to ModePlan resolves to planning without error.
	as.mu.Lock()
	as.ExecutionMode = ModePlan
	as.mu.Unlock()
	if pid, _ := as.EffectivePersona(sess); pid != "planning" {
		t.Fatalf("expected ModePlan EffectivePersona planning, got %q", pid)
	}

	// Returning to ModeBypass and selecting reviewer persona applies reviewer definition.
	as.mu.Lock()
	as.ExecutionMode = ModeBypass
	as.mu.Unlock()
	as.SetSessionPersona(sess.ID, "reviewer")
	as.mu.Lock()
	as.syncInternalSessionState(mgrID, sess)
	as.mu.Unlock()

	mgrSess, _ = eng.AgentMgr.GetSession(mgrID)
	if mgrSess.RoleFilter != rakitagent.RoleReviewer {
		t.Fatalf("expected manager session role %q after SetSessionPersona, got %q", rakitagent.RoleReviewer, mgrSess.RoleFilter)
	}
	if !mgrSess.ReadOnly {
		t.Fatalf("expected reviewer persona to set ReadOnly=true on manager session")
	}
}
