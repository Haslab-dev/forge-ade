package agent

import (
	"testing"
	"time"
)

func TestBotStoreAndHandToBot(t *testing.T) {
	dir := t.TempDir()
	store := NewBotStore(dir)

	bot := store.Create(BotProfile{
		Name:         "Scout",
		Role:         "Bug Triage & PR Fixer",
		SystemPrompt: "Reproduce bugs with a failing test before fixing.",
		AgentID:      "agent-claude",
		AgentType:    "acp",
		Model:        "claude-opus-4-6",
	})
	if bot == nil || bot.ID == "" {
		t.Fatalf("expected created bot with ID")
	}

	// Reload from disk and verify persistence.
	reloaded := NewBotStore(dir)
	list := reloaded.List()
	if len(list) != 1 || list[0].Name != "Scout" {
		t.Fatalf("expected persisted bot Scout, got %+v", list)
	}

	as := &AgentState{
		workspacePath:   dir,
		ActiveAgentID:   "agent-internal",
		ActiveAgentType: "internal",
		Sessions: []*AgentSession{
			{
				ID:            "sess-1",
				Title:         "Safari pay button bug",
				Status:        "idle",
				CreatedAt:     time.Now(),
				UpdatedAt:     time.Now(),
				AgentID:       "agent-internal",
				AgentType:     "internal",
				WorkspacePath: dir,
				Tasks:         NewTaskManager(),
			},
		},
		ActiveSessionID: "sess-1",
	}

	home := as.OpenBotHomeSession(reloaded, list[0])
	if home == nil || home.BotID != bot.ID {
		t.Fatalf("expected home session linked to bot %s, got %+v", bot.ID, home)
	}

	handed, err := as.HandToBot(list[0], "sess-1", "Fix Safari tap drop and add WebKit test", false)
	if err != nil {
		t.Fatalf("HandToBot failed: %v", err)
	}
	if handed.BotID != bot.ID || handed.HandoffFromID != "sess-1" {
		t.Fatalf("expected handed session linked to bot %s and sess-1, got %+v", bot.ID, handed)
	}

	reloaded.TogglePause(bot.ID)
	if !reloaded.Get(bot.ID).Paused {
		t.Fatalf("expected bot to be paused")
	}
	reloaded.Delete(bot.ID)
	if len(reloaded.List()) != 0 {
		t.Fatalf("expected empty bot list after delete")
	}
}
