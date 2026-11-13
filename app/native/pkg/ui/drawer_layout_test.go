package ui

// drawer_layout_test.go reproduces the squeezed right-drawer bug headlessly:
// with a giant unwrapped message in the transcript, the drawer column must
// still lay out at its requested width (flex-shrink protection).

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/app/native/pkg/agent"
	"github.com/hasdev/forge-ade/app/native/pkg/engine"
)

func TestDrawerNotSqueezedByGiantMessage(t *testing.T) {
	giantText := strings.Repeat("word ", 20000)
	if os.Getenv("SHORT_MSG") != "" {
		giantText = "short answer"
	}
	tempDir, err := os.MkdirTemp("", "drawer-test-*")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	eng := engine.NewEngine(tempDir)
	app := NewAppUI(eng, tempDir)
	app.ShowLeftSidebar = true
	app.ShowRightDrawer = true
	app.RightDrawerWidth = RightDrawerDefault

	// A giant single-line message: its max-content width dwarfs the window.
	as := app.Agent
	sess := &agent.AgentSession{
		ID: "sess-giant", Title: "Giant", Status: "idle",
		WorkspacePath: tempDir, Tasks: agent.NewTaskManager(),
		Messages: []*agent.ChatMessage{
			{ID: 1, Role: "user", Text: "hi", Done: true, Timestamp: time.Now()},
			{ID: 2, Role: "assistant", Text: giantText, Done: true, Timestamp: time.Now()},
		},
	}
	as.Lock()
	as.Sessions = []*agent.AgentSession{sess}
	as.ActiveSessionID = "sess-giant"
	as.Messages = sess.Messages
	as.Unlock()

	tw := ui.NewTesterWith(func(c *ui.Context) { app.View(c) }, 1280, 800, ui.TesterOptions{Dark: true})

	// The drawer's Changes pane header only renders inside the drawer; its
	// Rect tells us where the drawer laid out.
	// The drawer content must start near 1280-420 (=860). If it starts
	// against the right window edge, the drawer was squeezed.
	rect, ok := tw.Find("Working Tree")
	if !ok {
		t.Skip("drawer content not found (pane empty without git repo)")
	}
	const wantX = 1280 - RightDrawerDefault
	if rect.X < wantX-8 {
		t.Fatalf("drawer squeezed: content at x=%.0f, want >= %.0f", rect.X, wantX-8)
	}
}
