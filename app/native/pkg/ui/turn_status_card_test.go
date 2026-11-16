package ui

// turn_status_card_test.go smoke-renders the ZCode-style floating turn-status
// card headlessly: with tasks, changes, and a toggled card flag, View must
// lay out without panicking and the card sections must be present.

import (
	"os"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/app/native/pkg/agent"
	"github.com/hasdev/forge-ade/app/native/pkg/engine"
)

func TestTurnStatusCardRenders(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "turn-status-*")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	eng := engine.NewEngine(tempDir)
	app := NewAppUI(eng, tempDir)

	as := app.Agent
	tm := agent.NewTaskManager()
	tm.SetTasks([]agent.TaskItem{
		{ID: "t1", Title: "First task", Status: agent.TaskCompleted},
		{ID: "t2", Title: "Second task", ActiveForm: "Running the second task", Status: agent.TaskInProgress},
		{ID: "t3", Title: "Third task", Status: agent.TaskPending},
	})
	sess := &agent.AgentSession{
		ID: "sess-status", Title: "Status", Status: "running",
		WorkspacePath: tempDir, Tasks: tm,
		Messages: []*agent.ChatMessage{
			{ID: 1, Role: "user", Text: "go", Done: true, Timestamp: time.Now()},
			{ID: 2, Role: "assistant", Text: "working", Done: false, Timestamp: time.Now(), TurnStartedAt: time.Now()},
		},
	}
	as.Lock()
	as.Sessions = []*agent.AgentSession{sess}
	as.ActiveSessionID = "sess-status"
	as.Messages = sess.Messages
	as.TaskManager = tm
	as.Unlock()

	// Card open: the Progress sections must lay out.
	app.ShowTurnStatusCard = true
	tw := ui.NewTesterWith(func(c *ui.Context) { app.View(c) }, 1280, 800, ui.TesterOptions{Dark: true})
	if _, ok := tw.Find("Progress"); !ok {
		t.Fatalf("Progress header not found in open turn-status card")
	}
	if _, ok := tw.Find("Changes"); !ok {
		t.Fatalf("Changes header not found in open turn-status card")
	}

	// Card closed + toggle strip: strip appears when there is work to show.
	app.ShowTurnStatusCard = false
	tw2 := ui.NewTesterWith(func(c *ui.Context) { app.View(c) }, 1280, 800, ui.TesterOptions{Dark: true})
	if _, ok := tw2.Find("Progress"); ok {
		t.Fatalf("closed card must not render Progress")
	}
}
