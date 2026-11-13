package agent

import (
	"os"
	"testing"
)

func TestAgentCLIManager(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "cli-test-*")
	if err != nil {
		t.Fatalf("temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	cliMgr := NewAgentCLIManager(tempDir, nil)

	// Verify configs discovery works without crash
	configs := cliMgr.Configs
	if len(configs) == 0 {
		t.Logf("no agent CLI configs found on system (normal in clean sandbox)")
	}

	// Create CLI task session
	sess := cliMgr.CreateSession("test-agent", "Test Agent", "sh", []string{"-c", "echo 'hello from agent cli'"}, tempDir)
	if sess == nil {
		t.Fatalf("expected non-nil CLI session")
	}
	if sess.AgentID != "test-agent" || sess.Status != "running" {
		t.Fatalf("expected running test-agent session, got %v", sess)
	}

	if sess.Terminal == nil {
		t.Fatalf("expected embedded terminal in CLI session")
	}

	// Verify ActiveSession lookup
	active := cliMgr.ActiveSession()
	if active == nil || active.ID != sess.ID {
		t.Fatalf("expected active session to match created session")
	}

	// Stop session
	cliMgr.StopSession(sess.ID)
	if sess.Status != "completed" {
		t.Fatalf("expected session status 'completed' after StopSession, got '%s'", sess.Status)
	}
}
