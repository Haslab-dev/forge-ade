package ui

import (
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/app/native/pkg/agent"
	"github.com/hasdev/forge-ade/app/native/pkg/engine"
)

func TestBotTaskModalStepClickAndInspect(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "bot-modal-*")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	eng := engine.NewEngine(tempDir)
	app := NewAppUI(eng, tempDir)
	defer app.Close()

	app.Bots = agent.NewBotStore(tempDir)
	bot := app.Bots.Create(agent.BotProfile{
		ID:   "bot-test-1",
		Name: "Bewustein",
		Role: "Assistant",
	})

	now := time.Now()
	sess := &agent.AgentSession{
		ID:            "task-sess-1",
		BotID:         bot.ID,
		Title:         "Prepare three Social pairs",
		Status:        "completed",
		AgentID:       "agent-internal",
		AgentType:     "internal",
		WorkspacePath: tempDir,
		Tasks:         agent.NewTaskManager(),
		CreatedAt:     now,
		UpdatedAt:     now,
		Messages: []*agent.ChatMessage{
			{ID: 1, Role: "user", Text: "Prepare three Social pairs", Done: true, Timestamp: now},
			{
				ID:        2,
				Role:      "assistant",
				Text:      "Prepared all three social pairs.",
				Done:      true,
				Timestamp: now,
				Tools: []*agent.ToolExecution{
					{
						ID:         "tool-1",
						ToolName:   "read_file",
						Command:    `{"path":"~/workspace/blog-drafts/social-1.md"}`,
						Output:     "Read completed successfully (pi_score ~0.006)",
						Status:     "completed",
						DurationMs: 120,
					},
					{
						ID:         "tool-2",
						ToolName:   "bash",
						Command:    "ls -la ~/workspace/blog-drafts",
						Output:     "social-1.md\nsocial-2.md",
						Status:     "completed",
						DurationMs: 45,
					},
				},
			},
		},
	}

	app.Agent.Lock()
	app.Agent.Sessions = []*agent.AgentSession{sess}
	app.Agent.ActiveSessionID = sess.ID
	app.Agent.Unlock()

	app.SelectedBotTaskID = sess.ID
	app.ShowBotTaskModal = true

	tw := ui.NewTesterWith(func(c *ui.Context) { app.View(c) }, 1280, 800, ui.TesterOptions{Dark: true})

	// Initially Summary is shown
	if _, ok := tw.Find("Summary"); !ok {
		t.Fatalf("expected Summary in BotTaskModal initially")
	}

	// Click first step
	if err := tw.Click("read_file"); err != nil {
		t.Fatalf("failed to click step read_file: %v", err)
	}

	if app.SelectedBotTaskStepKey == "" {
		t.Fatalf("expected SelectedBotTaskStepKey to be set after clicking step, got empty")
	}
	if _, ok := tw.Find("Step 1 of 2"); !ok {
		t.Fatalf("expected Step 1 of 2 in step detail pane after click")
	}
	if _, ok := tw.Find("Read completed successfully (pi_score ~0.006)"); !ok {
		t.Fatalf("expected step output in step detail pane after click")
	}

	// Click second step directly to inspect it
	if err := tw.Click("bash"); err != nil {
		t.Fatalf("failed to click step bash: %v", err)
	}
	if _, ok := tw.Find("Step 2 of 2"); !ok {
		t.Fatalf("expected Step 2 of 2 after clicking second step")
	}
	if _, ok := tw.Find("ls -la ~/workspace/blog-drafts"); !ok {
		t.Fatalf("expected bash command in step detail pane")
	}

	// Click "Started" timeline node to return to task summary
	if err := tw.Click("Started"); err != nil {
		t.Fatalf("failed to click Started overview row: %v", err)
	}
	if app.SelectedBotTaskStepKey != "" {
		t.Fatalf("expected SelectedBotTaskStepKey to clear after clicking Started, got %q", app.SelectedBotTaskStepKey)
	}
	if _, ok := tw.Find("Prepared all three social pairs."); !ok {
		t.Fatalf("expected task summary after returning to overview")
	}
}
