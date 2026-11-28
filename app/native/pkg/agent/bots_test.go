package agent

import (
	"strings"
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

func TestBotTemplatesTasksAndTurnStatus(t *testing.T) {
	if tpls := DefaultBotTemplates(); len(tpls) < 3 {
		t.Fatalf("expected at least 3 built-in bot templates, got %d", len(tpls))
	}

	dir := t.TempDir()
	store := NewBotStore(dir)
	bot := store.Create(BotProfile{
		Name:      "Bewustein",
		Role:      "Coordinator",
		AgentID:   "agent-internal",
		AgentType: "internal",
	})

	now := time.Now()
	as := &AgentState{
		workspacePath:   dir,
		ActiveAgentID:   "agent-internal",
		ActiveAgentType: "internal",
	}
	home := as.OpenBotHomeSession(store, bot)
	taskSess, err := as.SpawnBotTask(store, bot, "Fix login bug", "Investigate auth token refresh", "agent-claude", "acp", false)
	if err != nil {
		t.Fatalf("SpawnBotTask failed: %v", err)
	}
	if taskSess.BotID != bot.ID || taskSess.ParentSessionID != home.ID {
		t.Fatalf("expected task linked to bot %s and parent %s, got bot=%s parent=%s", bot.ID, home.ID, taskSess.BotID, taskSess.ParentSessionID)
	}
	taskSess.Status = "running"
	home.Status = "running"
	home.Messages = []*ChatMessage{
		{
			ID:            1,
			Role:          "assistant",
			IsThinking:    false,
			TotalSteps:    2,
			TurnStartedAt: now.Add(-4 * time.Second),
			Tools: []*ToolExecution{
				{ID: "t1", ToolName: "read_file", Command: strings.Repeat("a", 150), Output: "package main\nfunc main() {}", Status: "completed"},
				{ID: "t2", ToolName: "grep_search", Command: "AuthToken", Status: "running"},
			},
		},
	}

	tasks := as.BotTasks(store, bot.ID)
	if len(tasks) != 2 || tasks[0].Title != "Fix login bug" || tasks[0].Status != "running" {
		t.Fatalf("unexpected BotTasks: %+v", tasks)
	}
	if len(tasks[1].Steps) != 2 || tasks[1].CurrentStep != "grep_search: AuthToken" {
		t.Fatalf("unexpected Home Chat turn task: %+v", tasks[1])
	}
	if len(tasks[1].Steps[0].FullDetail) != 150 || len(tasks[1].Steps[0].Detail) >= 150 || tasks[1].Steps[0].Output == "" {
		t.Fatalf("expected FullDetail and Output preserved on BotTaskStep: %+v", tasks[1].Steps[0])
	}

	st := as.ActiveTurnStatus(store, home.ID)
	if !st.Active || st.CurrentTool != "grep_search" || st.CompletedTools != 1 || st.BackgroundRunning != 1 {
		t.Fatalf("unexpected ActiveTurnStatus: %+v", st)
	}

	// Process isolation: switching to another bot while the first bot is running
	// must NOT leak StreamingSnapshot() / Stop button into the second bot.
	bot2 := store.Create(BotProfile{Name: "Reviewer", AgentID: "agent-internal", AgentType: "internal"})
	as.mu.Lock()
	as.IsStreaming = true
	as.mu.Unlock()
	home2 := as.OpenBotHomeSession(store, bot2)
	if home2 == nil || home2.ID == home.ID {
		t.Fatalf("expected distinct home session for bot2")
	}
	if as.StreamingSnapshot() {
		t.Fatalf("expected StreamingSnapshot() == false for idle bot2 while bot1 is running")
	}
	// Switching back to bot1 restores streaming=true.
	as.OpenBotHomeSession(store, bot)
	if !as.StreamingSnapshot() {
		t.Fatalf("expected StreamingSnapshot() == true when switching back to running bot1")
	}
}

func TestBotPersonaInheritsParentHarnessAndLimitations(t *testing.T) {
	dir := t.TempDir()
	store := NewBotStore(dir)

	bot := store.Create(BotProfile{
		Name:         "Auditor",
		Role:         "Security Reviewer",
		SystemPrompt: "Check auth boundaries first.",
		Limitations:  "Read-only; never edit files.",
	})
	if bot.AgentID != "" || bot.AgentType != "" {
		t.Fatalf("expected persona-only bot to leave AgentID/AgentType empty, got %q/%q", bot.AgentID, bot.AgentType)
	}

	as := &AgentState{
		workspacePath:   dir,
		ActiveAgentID:   "agent-internal",
		ActiveAgentType: "internal",
		Sessions: []*AgentSession{
			{
				ID:            "parent-1",
				Title:         "Parent Chat",
				Status:        "idle",
				AgentID:       "agent-claude",
				AgentType:     "acp",
				Model:         "claude-sonnet-4-5",
				WorkspacePath: dir,
				Tasks:         NewTaskManager(),
			},
		},
		ActiveSessionID: "parent-1",
	}

	home := as.OpenBotHomeSession(store, bot)
	if home.AgentID != "agent-claude" || home.AgentType != "acp" || home.Model != "claude-sonnet-4-5" {
		t.Fatalf("expected bot home session to inherit parent harness, got %s/%s/%s", home.AgentID, home.AgentType, home.Model)
	}

	handed, err := as.HandToBot(bot, "parent-1", "Audit login handler", false)
	if err != nil {
		t.Fatalf("HandToBot failed: %v", err)
	}
	if handed.AgentID != "agent-claude" || handed.AgentType != "acp" {
		t.Fatalf("expected handed session to inherit parent harness, got %s/%s", handed.AgentID, handed.AgentType)
	}
	if instr := bot.FormatPersonaInstructions(); !strings.Contains(instr, "Limitations & Guardrails:\nRead-only; never edit files.") {
		t.Fatalf("expected Limitations in FormatPersonaInstructions, got %q", instr)
	}

	updated := store.UpdatePersona(bot.ID, "Auditor v2", "AppSec", "Focus on OWASP top 10.", "No network calls.", "/tmp/ws")
	if updated == nil || updated.Name != "Auditor v2" || updated.Limitations != "No network calls." || updated.WorkspacePath != "/tmp/ws" || updated.AgentID != "" {
		t.Fatalf("unexpected UpdatePersona result: %+v", updated)
	}
}

func TestBuildBotTaskStepSummarizesJSONCommand(t *testing.T) {
	step := buildBotTaskStep(&ToolExecution{
		ID:         "t-json",
		ToolName:   "read_file",
		Command:    `{"file_path":"/tmp/blog-drafts/social.md","limit":50}`,
		Output:     "ok",
		Status:     "completed",
		DurationMs: 42,
	})
	if step.Detail != "/tmp/blog-drafts/social.md" {
		t.Fatalf("expected concise Detail from JSON file_path, got %q", step.Detail)
	}
	if step.FullDetail != `{"file_path":"/tmp/blog-drafts/social.md","limit":50}` {
		t.Fatalf("expected FullDetail to preserve full JSON, got %q", step.FullDetail)
	}
}
