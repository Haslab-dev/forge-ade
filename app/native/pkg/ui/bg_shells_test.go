package ui

import (
	"os"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/app/native/pkg/agent"
	"github.com/hasdev/forge-ade/app/native/pkg/engine"
)

func TestFormatMiniShellPreview(t *testing.T) {
	raw := "\x1b[32mStarting\x1b[0m\r\n  Local: http://localhost:3000\r\n"
	got := formatMiniShellPreview(raw)
	if strings.Contains(got, "\x1b") {
		t.Fatalf("expected ANSI codes stripped, got %q", got)
	}
	if !strings.Contains(got, "Local: http://localhost:3000") {
		t.Fatalf("expected output preserved, got %q", got)
	}
}

func TestBgShellPillAndPopoverRender(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "bg-shell-*")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	eng := engine.NewEngine(tempDir)
	app := NewAppUI(eng, tempDir)
	app.Agent.Lock()
	app.Agent.Sessions = []*agent.AgentSession{
		{ID: "s1", Title: "Dev", WorkspacePath: tempDir},
	}
	app.Agent.ActiveSessionID = "s1"
	app.Agent.Unlock()

	// Spawn a background command in Engine.ToolReg
	task, err := eng.ToolReg.StartBackgroundCommand("s1", "echo 'Local: http://localhost:3000' && sleep 5", tempDir)
	if err != nil {
		t.Fatalf("StartBackgroundCommand: %v", err)
	}
	defer func() { eng.ToolReg.StopTask(task.ID) }()

	shells := app.activeBackgroundShells("s1")
	if len(shells) != 1 {
		t.Fatalf("expected 1 active background shell, got %d", len(shells))
	}

	app.ShowBgShellPopover = true
	tw := ui.NewTesterWith(func(c *ui.Context) { app.View(c) }, 1280, 800, ui.TesterOptions{Dark: true})
	if _, ok := tw.Find("Background Shells"); !ok {
		t.Fatalf("expected Background Shells popover header to render")
	}
	if _, ok := tw.Find("echo 'Local: http://localhost:3000' && sleep 5"); !ok {
		t.Fatalf("expected session command name in popover")
	}

	app.stopBackgroundShell(shells[0])
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if len(app.activeBackgroundShells("s1")) == 0 {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if got := len(app.activeBackgroundShells("s1")); got != 0 {
		t.Fatalf("expected 0 active background shells after stop, got %d", got)
	}
}
