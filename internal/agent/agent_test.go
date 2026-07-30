package agent

import (
	"os"
	"testing"

	"github.com/hasdev/forge-ade/internal/events"
	"github.com/hasdev/forge-ade/internal/llm"
	"github.com/hasdev/forge-ade/internal/mcp"
	"github.com/hasdev/forge-ade/internal/search"
	"github.com/hasdev/forge-ade/internal/skills"
	"github.com/hasdev/forge-ade/internal/tools"
)

func TestAgentManagerSession(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "forge-agent-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	bus := events.NewBus()
	llmClient := llm.NewLLMClient(tempDir)
	searchMgr := search.NewSearchManager()
	toolReg := tools.NewRegistry(searchMgr)
	skillMgr := skills.NewManager()
	mcpMgr := mcp.NewManager()

	mgr := NewManager(llmClient, toolReg, skillMgr, mcpMgr, bus)

	sess, err := mgr.CreateSession("Test Agent", RoleCoding, tempDir)
	if err != nil {
		t.Fatalf("failed to create session: %v", err)
	}

	if sess.Name != "Test Agent" {
		t.Errorf("expected session name 'Test Agent', got '%s'", sess.Name)
	}

	if sess.RoleFilter != RoleCoding {
		t.Errorf("expected role coding, got %s", sess.RoleFilter)
	}

	list := mgr.ListSessions()
	if len(list) != 1 {
		t.Errorf("expected 1 session, got %d", len(list))
	}

	found, ok := mgr.GetSession(sess.ID)
	if !ok || found.ID != sess.ID {
		t.Errorf("failed to get session by ID")
	}

	mgr.DeleteSession(sess.ID)
	listAfter := mgr.ListSessions()
	if len(listAfter) != 0 {
		t.Errorf("expected 0 sessions after delete, got %d", len(listAfter))
	}
}
