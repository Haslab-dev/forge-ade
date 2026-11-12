package agent

import (
	"os"
	"testing"

	"github.com/hasdev/forge-ade/forge-ade-native/pkg/engine"
)

func TestACPAgentRegistry(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "acp-test-*")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	eng := engine.NewEngine(tempDir)
	acpMgr := NewACPManager(eng, nil)

	if len(acpMgr.Agents) < 8 {
		t.Fatalf("expected at least 8 registered ACP agents, got %d", len(acpMgr.Agents))
	}

	expectedAgents := map[string]string{
		"agent-internal":    "internal",
		"agent-agy":         "acp",
		"agent-claude":      "claude",
		"agent-codex":       "codex",
		"agent-opencode":    "opencode",
		"agent-commandcode": "commandcode",
		"agent-pi":          "pi",
		"agent-ohmypi":      "ohmypi",
		"agent-gemini":      "gemini",
	}

	for id, expectedType := range expectedAgents {
		found := false
		for _, a := range acpMgr.Agents {
			if a.ID == id {
				found = true
				if a.Type != expectedType {
					t.Fatalf("agent %s expected type %s, got %s", id, expectedType, a.Type)
				}
				break
			}
		}
		if !found {
			t.Fatalf("expected agent %s not found in registry", id)
		}
	}

	// Verify Internal Agent session launch
	sess, err := acpMgr.StartSession("agent-internal", tempDir, "Claude 3.7 Sonnet")
	if err != nil {
		t.Fatalf("failed to start internal ACP session: %v", err)
	}
	if sess == nil || sess.AgentID != "agent-internal" {
		t.Fatalf("expected valid internal session, got %v", sess)
	}

	// Test Permission request handling
	acpMgr.ActiveReq = &PermissionRequest{
		SessionID: sess.ID,
		RequestID: "req-1",
		ToolName:  "bash",
		Command:   "go build ./...",
	}

	_ = acpMgr.RespondPermission(sess.ID, "req-1", true)
	if acpMgr.ActiveReq != nil {
		t.Fatalf("expected active request to be cleared after response")
	}
}
