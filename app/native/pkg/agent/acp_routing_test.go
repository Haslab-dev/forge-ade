package agent

import (
	"os"
	"testing"
	"time"

	"github.com/Haslab-dev/RakitKode/events"
	"github.com/hasdev/forge-ade/app/native/pkg/engine"
)

func TestACPRoutingWithoutInternalAPIKey(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "acp-routing-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	eng := engine.NewEngine(tempDir)

	as := NewAgentState(eng, tempDir, func() {})
	// Sessions shared with ForgeADE under ~/.forge/sessions/General may
	// preload into any workspace; the default-harness assertions only hold
	// on a clean disk.
	preloaded := len(as.Sessions) > 0
	if !preloaded {
		if as.ActiveAgentID != "agent-agy" {
			t.Errorf("expected default ActiveAgentID 'agent-agy', got '%s'", as.ActiveAgentID)
		}
		if as.ActiveAgentType != "acp" {
			t.Errorf("expected default ActiveAgentType 'acp', got '%s'", as.ActiveAgentType)
		}
		if as.ActiveModel != "gemini-3.8-flash-high" {
			t.Errorf("expected default ActiveModel 'gemini-3.8-flash-high', got '%s'", as.ActiveModel)
		}
	}

	// Create new session
	sess := as.NewSession("Pair Programming")
	if sess.AgentID != "agent-agy" || sess.AgentType != "acp" {
		t.Errorf("new session has wrong agent/type: %+v", sess)
	}

	// Simulate existing legacy session with agent-monolith and gpt-4o
	legacySess := &AgentSession{
		ID:            "sess-legacy-1",
		Title:         "Legacy Session",
		Status:        "idle",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		Model:         "gpt-4o",
		AgentID:       "agent-monolith",
		AgentType:     "internal",
		WorkspacePath: tempDir,
		Messages:      []*ChatMessage{},
		Tasks:         NewTaskManager(),
	}
	as.Sessions = append([]*AgentSession{legacySess}, as.Sessions...)
	as.ActiveSessionID = legacySess.ID
	as.syncActiveSession()

	// Verify Submit on legacy session routes to ACP agent and doesn't crash or return LLM Request Notice
	as.InputText = "test hello"
	as.SubmitMessage()

	// Give a moment for turn setup
	time.Sleep(20 * time.Millisecond)

	as.Lock()
	defer as.Unlock()
	if legacySess.AgentID != "agent-agy" || legacySess.AgentType != "acp" {
		t.Errorf("legacy session was not auto-routed to agent-agy: %+v", legacySess)
	}
	if legacySess.Model != "gemini-3.8-flash-high" {
		t.Errorf("legacy session model was not upgraded to gemini-3.8-flash-high, got '%s'", legacySess.Model)
	}
}

func TestACPStreamTokenAccumulation(t *testing.T) {
	tempDir := t.TempDir()
	eng := engine.NewEngine(tempDir)
	bus := eng.Bus

	acpMgr := NewACPManager(eng, func() {})
	as := NewAgentState(eng, tempDir, func() {})
	as.ACPMgr = acpMgr

	sess := as.NewSession("Stream Test")
	msg := &ChatMessage{
		ID:        len(sess.Messages) + 1,
		Role:      "assistant",
		Text:      "",
		Done:      false,
		Timestamp: time.Now(),
	}
	sess.Messages = append(sess.Messages, msg)

	// Wire token chunk callback similar to AppUI
	acpMgr.OnTokenChunk = func(sessionID, text string) {
		as.Lock()
		for _, s := range as.Sessions {
			if s.ID == sessionID && len(s.Messages) > 0 {
				last := s.Messages[len(s.Messages)-1]
				if !last.Done {
					last.Text += text
				}
			}
		}
		as.Unlock()
	}

	acpMgr.OnTurnDone = func(sessionID string, info TurnEndInfo) {
		as.Lock()
		for _, s := range as.Sessions {
			if s.ID == sessionID && len(s.Messages) > 0 {
				last := s.Messages[len(s.Messages)-1]
				last.Done = true
			}
		}
		as.Unlock()
	}

	// Publish streaming tokens via event bus
	bus.Publish(events.Event{
		Type: events.AgentMessageDelta,
		Data: map[string]interface{}{
			"sessionId": sess.ID,
			"delta":     "Hello ",
		},
	})
	bus.Publish(events.Event{
		Type: events.AgentMessageDelta,
		Data: map[string]interface{}{
			"sessionId": sess.ID,
			"delta":     "world!",
		},
	})

	time.Sleep(20 * time.Millisecond)

	as.Lock()
	if msg.Text != "Hello world!" {
		t.Errorf("expected accumulated text 'Hello world!', got '%s'", msg.Text)
	}
	if msg.Done {
		t.Errorf("expected message to not be done yet")
	}
	as.Unlock()

	// Turn completion
	bus.Publish(events.Event{
		Type: "agent:turn_end",
		Data: map[string]interface{}{
			"sessionId": sess.ID,
		},
	})

	time.Sleep(20 * time.Millisecond)

	as.Lock()
	if !msg.Done {
		t.Errorf("expected message to be marked done after turn_end")
	}
	as.Unlock()
}

func TestSelectACPAgentAutoSelectsModel(t *testing.T) {
	tempDir := t.TempDir()
	eng := engine.NewEngine(tempDir)
	acpMgr := NewACPManager(eng, func() {})
	as := NewAgentState(eng, tempDir, func() {})
	as.ACPMgr = acpMgr

	cases := []struct {
		agentID    string
		wantModel  string
		wantEffort ReasoningEffort
	}{
		{"agent-claude", "sonnet", EffortHigh},
		{"agent-codex", "deepseek-v4-1-flash", EffortMedium},
		{"agent-opencode", "Kenari/glm-5-3-flash", EffortMedium},
		{"agent-gemini", "gemini-2.5-pro", EffortHigh},
	}

	for _, tc := range cases {
		gotModel := as.SelectACPAgent(tc.agentID)
		if gotModel != tc.wantModel {
			t.Fatalf("%s: SelectACPAgent returned model %q, want %q", tc.agentID, gotModel, tc.wantModel)
		}
		if as.ActiveAgentID != tc.agentID {
			t.Fatalf("%s: ActiveAgentID = %q, want %q", tc.agentID, as.ActiveAgentID, tc.agentID)
		}
		if as.ActiveAgentType != "acp" {
			t.Fatalf("%s: ActiveAgentType = %q, want \"acp\"", tc.agentID, as.ActiveAgentType)
		}
		if as.ActiveModel != tc.wantModel {
			t.Fatalf("%s: ActiveModel = %q, want %q", tc.agentID, as.ActiveModel, tc.wantModel)
		}
		if as.ReasoningEffort != tc.wantEffort {
			t.Fatalf("%s: ReasoningEffort = %q, want %q", tc.agentID, as.ReasoningEffort, tc.wantEffort)
		}

		// Force a stale model from another harness to ensure NewSession auto-corrects it.
		as.ActiveModel = "some-other-agent-model"
		sess := as.NewSession("Task for " + tc.agentID)
		if sess.Model != tc.wantModel {
			t.Fatalf("%s: NewSession.Model = %q, want %q", tc.agentID, sess.Model, tc.wantModel)
		}
		as.DeleteSession(sess.ID)
	}
}

func TestParallelSessionsRunWithoutInterruption(t *testing.T) {
	tempDir := t.TempDir()
	eng := engine.NewEngine(tempDir)
	eng.ACPMgr = nil
	acpMgr := NewACPManager(eng, func() {})
	as := NewAgentState(eng, tempDir, func() {})
	as.ACPMgr = acpMgr

	acpMgr.OnTokenChunk = as.HandleTokenChunk
	acpMgr.OnTurnStart = as.HandleTurnStart

	// Start Session 1 and mark it running.
	sess1 := as.NewSession("Parallel One")
	defer as.DeleteSession(sess1.ID)
	as.HandleTurnStart(sess1.ID, "prompt 1")
	as.HandleTokenChunk(sess1.ID, "A1 ")

	// Queue a steer prompt on running Session 1.
	as.InputText = "queued-for-s1"
	as.SubmitMessage()

	if len(as.SteerQueue) != 1 || as.SteerQueue[0] != "queued-for-s1" {
		t.Fatalf("expected sess1 steer queue [queued-for-s1], got %v", as.SteerQueue)
	}

	// User clicks + New task while Session 1 keeps running in background.
	as.ClearActiveSession()
	if sess1.Status != "running" {
		t.Fatalf("expected sess1 status 'running' after ClearActiveSession, got %q", sess1.Status)
	}
	if !as.HasRunningSession() {
		t.Fatalf("expected HasRunningSession() == true while sess1 runs in background")
	}
	if as.IsStreaming {
		t.Fatalf("expected global foreground IsStreaming == false on New Task view")
	}
	if len(as.SteerQueue) != 0 {
		t.Fatalf("expected foreground SteerQueue empty on New Task view, got %v", as.SteerQueue)
	}

	// Start Session 2 in parallel.
	sess2 := as.NewSession("Parallel Two")
	defer as.DeleteSession(sess2.ID)
	if sess2.ID == sess1.ID {
		t.Fatalf("expected distinct session ID for parallel session, got duplicate %s", sess2.ID)
	}
	as.HandleTurnStart(sess2.ID, "prompt 2")

	// Concurrent streaming into Session 1 and Session 2 via event bus.
	eng.Bus.Publish(events.Event{
		Type: events.AgentMessageDelta,
		Data: map[string]interface{}{"sessionId": sess1.ID, "delta": "A2"},
	})
	eng.Bus.Publish(events.Event{
		Type: events.AgentMessageDelta,
		Data: map[string]interface{}{"sessionId": sess2.ID, "delta": "B1 "},
	})

	// Wait deterministically for async bus dispatch.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		var s1Ready, s2Ready bool
		as.Lock()
		for _, m := range sess1.Messages {
			if m.Role == "assistant" && m.Text == "A1 A2" {
				s1Ready = true
			}
		}
		for _, m := range sess2.Messages {
			if m.Role == "assistant" && m.Text == "B1 " {
				s2Ready = true
			}
		}
		as.Unlock()
		if s1Ready && s2Ready {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Switching to sess1 and back to sess2 preserves each session's queue and streaming state.
	as.SelectSession(sess1.ID)
	if !as.IsStreaming || len(as.SteerQueue) != 1 || as.SteerQueue[0] != "queued-for-s1" {
		t.Fatalf("expected sess1 streaming=true and steerQueue=[queued-for-s1], got streaming=%v queue=%v", as.IsStreaming, as.SteerQueue)
	}
	as.SelectSession(sess2.ID)
	if !as.IsStreaming || len(as.SteerQueue) != 0 {
		t.Fatalf("expected sess2 streaming=true and empty steerQueue, got streaming=%v queue=%v", as.IsStreaming, as.SteerQueue)
	}

	// Background Session 1 completes while Session 2 is in foreground.
	nextSteer := as.HandleTurnDone(sess1.ID, TurnEndInfo{Status: "completed"})
	if nextSteer != "queued-for-s1" {
		t.Fatalf("expected background sess1 to drain 'queued-for-s1', got %q", nextSteer)
	}
	if sess2.Status != "running" || !as.IsStreaming {
		t.Fatalf("expected foreground sess2 to keep running untouched, got status=%q isStreaming=%v", sess2.Status, as.IsStreaming)
	}

	// Verify messages remained isolated across sessions.
	var s1Assistant, s2Assistant string
	as.Lock()
	for _, m := range sess1.Messages {
		if m.Role == "assistant" {
			s1Assistant += m.Text
		}
	}
	for _, m := range sess2.Messages {
		if m.Role == "assistant" {
			s2Assistant += m.Text
		}
	}
	as.Unlock()
	if s1Assistant != "A1 A2" {
		t.Fatalf("expected sess1 assistant text 'A1 A2', got %q", s1Assistant)
	}
	if s2Assistant != "B1 " {
		t.Fatalf("expected sess2 assistant text 'B1 ', got %q", s2Assistant)
	}

	// Verify CancelStreaming only stops active session (sess2) without affecting another session.
	as.SelectSession(sess1.ID)
	as.HandleTurnStart(sess1.ID, nextSteer)
	as.SelectSession(sess2.ID)
	as.CancelStreaming()
	if sess2.Status == "running" {
		t.Fatalf("expected active sess2 to stop after CancelStreaming")
	}
	if sess1.Status != "running" {
		t.Fatalf("expected background sess1 to remain running when sess2 is cancelled, got %q", sess1.Status)
	}
}

func TestSubmitFromNewTaskStartsParallelSessionWhileBackgroundRuns(t *testing.T) {
	tempDir := t.TempDir()
	eng := engine.NewEngine(tempDir)
	eng.ACPMgr = nil
	acpMgr := NewACPManager(eng, func() {})
	as := NewAgentState(eng, tempDir, func() {})
	as.ACPMgr = acpMgr

	sess1 := as.NewSession("Background Session")
	defer as.DeleteSession(sess1.ID)
	as.HandleTurnStart(sess1.ID, "prompt 1")

	as.ClearActiveSession()
	as.InputText = "new parallel task prompt"
	as.SubmitMessage()

	sess2 := as.ActiveSession()
	if sess2 == nil || sess2.ID == sess1.ID {
		t.Fatalf("expected SubmitMessage from New Task to create a new parallel session, got %v", sess2)
	}
	defer as.DeleteSession(sess2.ID)

	if len(sess1.SteerQueue) != 0 {
		t.Fatalf("expected background sess1 steer queue untouched, got %v", sess1.SteerQueue)
	}
	if sess1.Status != "running" {
		t.Fatalf("expected background sess1 to remain running, got %q", sess1.Status)
	}
	// Give the goroutine (which exits immediately because ACP manager is nil) a moment to settle cleanly.
	time.Sleep(30 * time.Millisecond)
}
