package ui

// turn_status_card_test.go smoke-renders the ZCode-style floating turn-status
// card headlessly: with tasks, changes, and a toggled card flag, View must
// lay out without panicking and the card sections must be present.

import (
	"os"
	"strings"
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
		ID: "sess-status", Title: "Status", Status: "idle",
		WorkspacePath: tempDir, Tasks: tm,
		Messages: []*agent.ChatMessage{
			{ID: 1, Role: "user", Text: "go", Done: true, Timestamp: time.Now()},
			// The completed turn carries its file changes: the status panel's
			// Changes section reads the last assistant message's Changes once
			// no session is streaming (no live git polling in the test).
			{ID: 2, Role: "assistant", Text: "done", Done: true, Timestamp: time.Now(),
				Changes: []*agent.FileChange{{Path: "main.go", Additions: 3, Deletions: 1}}},
		},
	}
	as.Lock()
	as.Sessions = []*agent.AgentSession{sess}
	as.ActiveSessionID = "sess-status"
	as.Messages = sess.Messages
	as.TaskManager = tm
	as.Unlock()

	// Panel expanded: Changes and Progress sections must lay out.
	app.ShowTurnStatusCard = true
	tw := ui.NewTesterWith(func(c *ui.Context) { app.View(c) }, 1280, 800, ui.TesterOptions{Dark: true})
	if _, ok := tw.Find("Progress"); !ok {
		t.Fatalf("Progress header not found in open status panel")
	}
	if _, ok := tw.Find("Changes"); !ok {
		t.Fatalf("Changes header not found in open status panel")
	}

	// Collapsed capsule: sections must not render.
	app.ShowTurnStatusCard = false
	tw2 := ui.NewTesterWith(func(c *ui.Context) { app.View(c) }, 1280, 800, ui.TesterOptions{Dark: true})
	if _, ok := tw2.Find("Progress"); ok {
		t.Fatalf("collapsed capsule must not render Progress")
	}
}

func TestExtractDiffFromToolOutput(t *testing.T) {
	validJSON := `{"additions":2,"deletions":1,"diff":"@@ -1,3 +1,4 @@\n line1\n-old\n+new\n+added\n"}`
	got := extractDiffFromToolOutput(validJSON, "src/App.tsx")
	if !strings.Contains(got, "--- a/src/App.tsx") || !strings.Contains(got, "+added") {
		t.Fatalf("unexpected extracted diff from valid JSON: %q", got)
	}

	truncatedJSON := `{"additions":45,"deletions":2,"diff":"@@ -22,5 +22,5 @@\n modelUrl?: string | null;\n+ onModelLoaded?: (meshes: Mesh[]) \u003d\u003e void;\n- onModelLoaded?: () \u003d\u003e void;\n ... [truncated]`
	gotTrunc := extractDiffFromToolOutput(truncatedJSON, "src/components/BabylonViewer.tsx")
	if !strings.Contains(gotTrunc, "--- a/src/components/BabylonViewer.tsx") || !strings.Contains(gotTrunc, "+ onModelLoaded?: (meshes: Mesh[]) => void;") {
		t.Fatalf("unexpected extracted diff from truncated JSON: %q", gotTrunc)
	}
}

func TestParseToolArgsEdits(t *testing.T) {
	raw := `{"file_path":"src/App.tsx","edits":[{"old_string":"a","new_string":"b"},{"old_string":"c","new_string":"d"}]}`
	parsed := parseToolArgs(raw, "edit")
	if parsed.Path != "src/App.tsx" {
		t.Fatalf("Path = %q, want src/App.tsx", parsed.Path)
	}
	if len(parsed.Edits) != 2 || parsed.Edits[0].OldString != "a" || parsed.Edits[1].NewString != "d" {
		t.Fatalf("unexpected edits: %+v", parsed.Edits)
	}
}
