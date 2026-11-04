package app

import (
	"context"
	"testing"

	"github.com/Haslab-dev/RakitKode/agent"
	"github.com/Haslab-dev/RakitKode/events"
	"github.com/Haslab-dev/RakitKode/llm"
	"github.com/Haslab-dev/RakitKode/mcp"
	"github.com/Haslab-dev/RakitKode/plugins"
	"github.com/Haslab-dev/RakitKode/search"
	"github.com/Haslab-dev/RakitKode/skills"
	"github.com/Haslab-dev/RakitKode/tools"
)

func TestSetAgentSessionModel(t *testing.T) {
	appNil := &App{}
	if err := appNil.SetAgentSessionModel("sess-1", "gpt-4o"); err == nil {
		t.Fatalf("expected error when agentMgr is nil, got nil")
	}

	tempHome := t.TempDir()
	bus := events.NewBus()
	llmClient := llm.NewLLMClient(tempHome)
	si := search.NewSearchManager()
	toolReg := tools.NewRegistry(si)
	skillMgr := skills.NewManager(tempHome)
	pluginMgr := plugins.NewManager(tempHome, bus)
	mcpMgr := mcp.NewManager(tempHome)

	agentMgr := agent.NewManager(llmClient, toolReg, skillMgr, pluginMgr, mcpMgr, nil, bus, tempHome)

	app := &App{
		ctx:       context.Background(),
		agentMgr:  agentMgr,
		llmClient: llmClient,
	}

	sess, err := app.CreateAgentSession("Test Session", "code", tempHome)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	// Change model
	if err := app.SetAgentSessionModel(sess.ID, "deepseek-chat"); err != nil {
		t.Fatalf("SetAgentSessionModel failed: %v", err)
	}

	updated, ok := agentMgr.GetSession(sess.ID)
	if !ok {
		t.Fatalf("session not found")
	}
	if updated.Model != "deepseek-chat" {
		t.Errorf("expected model 'deepseek-chat', got %q", updated.Model)
	}

	// Empty model returns error
	if err := app.SetAgentSessionModel(sess.ID, ""); err == nil {
		t.Errorf("expected error for empty model, got nil")
	}
}
