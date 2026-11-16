package agent

import (
	"testing"
)

func TestPromptHistoryStepSessionBased(t *testing.T) {
	as := &AgentState{
		PromptHistoryIdx: -1,
	}

	// 1. With no active session, PromptHistoryStep returns false.
	text, ok := as.PromptHistoryStep(-1, "my draft")
	if ok || text != "" {
		t.Fatalf("expected false on no active session, got ok=%v text=%q", ok, text)
	}

	// 2. Setup session with user and assistant messages.
	sess1 := &AgentSession{
		ID: "sess-1",
		Messages: []*ChatMessage{
			{Role: "user", Text: "first prompt"},
			{Role: "assistant", Text: "response 1"},
			{Role: "user", Text: "second prompt"},
			{Role: "assistant", Text: "response 2"},
			{Role: "user", Text: "third prompt"},
		},
	}
	as.Sessions = []*AgentSession{sess1}
	as.ActiveSessionID = "sess-1"

	// Pressing Down (delta +1) while at live draft returns false.
	text, ok = as.PromptHistoryStep(1, "my draft")
	if ok {
		t.Fatalf("expected false for Down arrow when not browsing, got ok=%v text=%q", ok, text)
	}

	// Pressing Up (delta -1) recalls newest prompt ("third prompt") and stashes "my draft".
	text, ok = as.PromptHistoryStep(-1, "my draft")
	if !ok || text != "third prompt" {
		t.Fatalf("expected 'third prompt', got ok=%v text=%q", ok, text)
	}
	if as.PromptHistoryDraft != "my draft" {
		t.Fatalf("expected draft 'my draft', got %q", as.PromptHistoryDraft)
	}

	// Pressing Up again recalls "second prompt".
	text, ok = as.PromptHistoryStep(-1, text)
	if !ok || text != "second prompt" {
		t.Fatalf("expected 'second prompt', got ok=%v text=%q", ok, text)
	}

	// Pressing Up again recalls "first prompt".
	text, ok = as.PromptHistoryStep(-1, text)
	if !ok || text != "first prompt" {
		t.Fatalf("expected 'first prompt', got ok=%v text=%q", ok, text)
	}

	// Pressing Up again stays at oldest ("first prompt").
	text, ok = as.PromptHistoryStep(-1, text)
	if !ok || text != "first prompt" {
		t.Fatalf("expected 'first prompt' clamped, got ok=%v text=%q", ok, text)
	}

	// Pressing Down (delta +1) moves back forward to "second prompt".
	text, ok = as.PromptHistoryStep(1, text)
	if !ok || text != "second prompt" {
		t.Fatalf("expected 'second prompt', got ok=%v text=%q", ok, text)
	}

	// Pressing Down again moves to "third prompt".
	text, ok = as.PromptHistoryStep(1, text)
	if !ok || text != "third prompt" {
		t.Fatalf("expected 'third prompt', got ok=%v text=%q", ok, text)
	}

	// Pressing Down again moves past newest and restores stashed draft.
	text, ok = as.PromptHistoryStep(1, text)
	if !ok || text != "my draft" {
		t.Fatalf("expected restored 'my draft', got ok=%v text=%q", ok, text)
	}
	if as.PromptHistoryIdx != -1 {
		t.Fatalf("expected PromptHistoryIdx == -1 after returning to draft, got %d", as.PromptHistoryIdx)
	}

	// 3. Switch to another session with different messages.
	sess2 := &AgentSession{
		ID: "sess-2",
		Messages: []*ChatMessage{
			{Role: "user", Text: "other session prompt"},
		},
	}
	as.Sessions = append(as.Sessions, sess2)
	as.SelectSession("sess-2")

	// Up arrow in sess-2 should only yield sess-2 prompts.
	text, ok = as.PromptHistoryStep(-1, "")
	if !ok || text != "other session prompt" {
		t.Fatalf("expected 'other session prompt' in sess-2, got ok=%v text=%q", ok, text)
	}

	// Further Up stays at "other session prompt".
	text, ok = as.PromptHistoryStep(-1, text)
	if !ok || text != "other session prompt" {
		t.Fatalf("expected 'other session prompt', got ok=%v text=%q", ok, text)
	}
}
