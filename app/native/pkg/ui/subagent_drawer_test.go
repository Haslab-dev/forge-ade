package ui

import (
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/app/native/pkg/agent"
	"github.com/hasdev/forge-ade/app/native/pkg/engine"
)

func TestChildAgentsDrawerShowsACPSubagentsAndHybridWorkers(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "child-agents-*")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	eng := engine.NewEngine(tempDir)
	app := NewAppUI(eng, tempDir)
	defer app.Close()
	app.Bots = agent.NewBotStore(tempDir)

	now := time.Now()
	parent := &agent.AgentSession{
		ID:            "sess-acp-parent",
		Title:         "Research Automations",
		Status:        "running",
		AgentID:       "agent-agy",
		AgentType:     "acp",
		WorkspacePath: tempDir,
		Tasks:         agent.NewTaskManager(),
		CreatedAt:     now,
		UpdatedAt:     now,
		Messages: []*agent.ChatMessage{
			{
				ID:        1,
				Role:      "assistant",
				Timestamp: now,
				Tools: []*agent.ToolExecution{
					{
						ID:       "sub-1",
						ToolName: "Subagent",
						Command:  `{"Role":"Core Automations Researcher","Prompt":"Inspect core/automations/"}`,
						Output:   "Found Manager and StartScheduler in core/automations/automations.go",
						Status:   "completed",
					},
				},
			},
		},
	}
	worker := &agent.AgentSession{
		ID:              "sess-worker-1",
		ParentSessionID: parent.ID,
		WorkerName:      "ui-auditor",
		Title:           "ui-auditor",
		Status:          "running",
		AgentID:         "agent-agy",
		AgentType:       "acp",
		WorkspacePath:   tempDir,
		Tasks:           agent.NewTaskManager(),
		CreatedAt:       now,
		UpdatedAt:       now,
		Messages: []*agent.ChatMessage{
			{ID: 1, Role: "user", Text: "Audit UI automations", Timestamp: now},
		},
	}

	app.Agent.Lock()
	app.Agent.Sessions = []*agent.AgentSession{worker, parent}
	app.Agent.ActiveSessionID = parent.ID
	app.Agent.Unlock()

	app.ShowLeftSidebar = false
	app.ShowRightDrawer = true
	app.RightDrawerTab = RightTabSubagent

	tw := ui.NewTesterWith(func(c *ui.Context) { app.View(c) }, 1280, 800, ui.TesterOptions{Dark: true})

	subs := app.snapshotSubagents(parent.ID)
	if len(subs) != 2 {
		t.Fatalf("expected 2 subagents (1 hybrid worker + 1 ACP subagent), got %d", len(subs))
	}
	if _, ok := tw.Find("Child Agents"); !ok {
		t.Fatalf("expected Child Agents drawer header")
	}
	if err := tw.Click("ui-auditor"); err != nil {
		t.Fatalf("failed to click hybrid worker row: %v", err)
	}
	if app.SelectedSubagentID != "sess-worker-1" {
		t.Fatalf("expected SelectedSubagentID == sess-worker-1, got %q", app.SelectedSubagentID)
	}
	if _, ok := tw.Find("All agents"); !ok {
		t.Fatalf("expected All agents back button in subagent detail view")
	}
	if _, ok := tw.Find("Audit UI automations"); !ok {
		t.Fatalf("expected worker prompt when inspecting clicked worker")
	}
}
