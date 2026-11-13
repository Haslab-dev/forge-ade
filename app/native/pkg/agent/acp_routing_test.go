package agent

import (
	"os"
	"testing"
	"time"

	"github.com/Haslab-dev/RakitKode/events"
	"github.com/hasdev/forge-ade/forge-ade-native/pkg/engine"
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
