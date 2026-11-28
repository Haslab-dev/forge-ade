package ui

import (
	"os"
	"testing"

	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/app/native/pkg/agent"
	"github.com/hasdev/forge-ade/app/native/pkg/engine"
)

func TestHandoffPersonaAutoAppliesHarnessAndModel(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "handoff-persona-*")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	eng := engine.NewEngine(tempDir)
	app := NewAppUI(eng, tempDir)
	defer app.Close()
	app.Bots = agent.NewBotStore(tempDir)

	if def, ok := eng.AgentMgr.FindDefinition("research"); ok {
		def.Model = "gpt-6-luna"
		if _, err := eng.AgentMgr.SaveAgentDefinition(def); err != nil {
			t.Fatalf("SaveAgentDefinition: %v", err)
		}
	} else {
		t.Fatalf("expected builtin research definition")
	}
	app.Settings.SetPersonaHarness("research", "agent-agy", "acp")

	app.OpenHandoffModal()
	tw := ui.NewTesterWith(func(c *ui.Context) { app.View(c) }, 1280, 800, ui.TesterOptions{Dark: true})

	if err := tw.Click("Research"); err != nil {
		t.Fatalf("failed to click Research persona: %v", err)
	}
	if app.HandoffTargetAgentID != "agent-agy" || app.HandoffTargetModel != "gpt-6-luna" {
		t.Fatalf("expected Research persona to auto-apply agent-agy / gpt-6-luna, got %s / %s",
			app.HandoffTargetAgentID, app.HandoffTargetModel)
	}
}
