package agent

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestAgentCLIManager(t *testing.T) {
	// Isolate the real ~/.forge-ade store: CreateSession persists to
	// $HOME/.forge-ade/agentsessions/index.json, and an unisolated run
	// polluted the user's store with one "Test Agent Task" debris session
	// per test run (the resurrecting cli-test-* sidebar projects).
	t.Setenv("HOME", t.TempDir())
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

	// The persisted store must live under the isolated HOME, never the
	// user's real one.
	if _, err := os.Stat(filepath.Join(cliStoreDirForTest(t), "index.json")); err != nil {
		t.Fatalf("expected isolated index.json in fake HOME: %v", err)
	}
}

func TestAgentCLIDeleteSessionPersistsAcrossReload(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	tempDir := t.TempDir()
	cliMgr := NewAgentCLIManager(tempDir, nil)

	sess := cliMgr.CreateSession("test-agent", "Test Agent", "sh", []string{"-c", "true"}, tempDir)
	if sess == nil {
		t.Fatalf("expected non-nil CLI session")
	}

	cliMgr.DeleteSession(sess.ID)
	if _, ok := cliMgr.Sessions[sess.ID]; ok {
		t.Fatalf("deleted session still in memory")
	}

	// A fresh manager (as on app restart) must not resurrect the record.
	reloaded := NewAgentCLIManager(tempDir, nil)
	if _, ok := reloaded.Sessions[sess.ID]; ok {
		t.Fatalf("deleted session resurrected from disk after reload")
	}
}

func TestAgentCLILoaderPurgesTempWorkspaceDebris(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	tempDir := t.TempDir()

	// Seed the store with one debris record (temp workspace) and one real
	// record, exactly like the polluted index.json found in the wild.
	dir := cliStoreDirForTest(t)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir store: %v", err)
	}
	seed := []map[string]any{
		{
			"id":               "cli-test-agent-111",
			"agentId":          "test-agent",
			"agentName":        "Test Agent",
			"title":            "Test Agent Task",
			"executable":       "sh",
			"workingDirectory": filepath.Join(t.TempDir(), "cli-test-111"),
			"status":           "completed",
			"startedAt":        1700000000000,
		},
		{
			"id":         "cli-real-222",
			"agentId":    "gemini",
			"agentName":  "Gemini",
			"title":      "Real Work",
			"executable": "sh",
			// A literal non-temp path: the fake HOME above is itself a temp
			// dir, so a real-workspace record must live outside it.
			"workingDirectory": "/Users/fakeuser/projects/real-app",
			"status":           "completed",
			"startedAt":        1700000001000,
		},
	}
	data, err := json.Marshal(seed)
	if err != nil {
		t.Fatalf("marshal seed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "index.json"), data, 0o644); err != nil {
		t.Fatalf("write seed index: %v", err)
	}

	mgr := NewAgentCLIManager(tempDir, nil)
	if _, ok := mgr.Sessions["cli-test-agent-111"]; ok {
		t.Fatalf("temp-workspace debris must not load")
	}
	if _, ok := mgr.Sessions["cli-real-222"]; !ok {
		t.Fatalf("real session must survive the debris purge")
	}

	// The purge must be persisted so the debris stops resurrecting.
	raw, err := os.ReadFile(filepath.Join(dir, "index.json"))
	if err != nil {
		t.Fatalf("read purged index: %v", err)
	}
	var got []map[string]any
	if err := json.Unmarshal(raw, &got); err != nil {
		t.Fatalf("unmarshal purged index: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("expected purged index to hold 1 record, got %d", len(got))
	}
	if got[0]["id"] != "cli-real-222" {
		t.Fatalf("purged index kept the wrong record: %v", got[0]["id"])
	}
}

// cliStoreDirForTest returns the agent CLI session store directory inside the
// current (HOME-isolated) fake home.
func cliStoreDirForTest(t *testing.T) string {
	t.Helper()
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("user home: %v", err)
	}
	return filepath.Join(home, ".forge-ade", "agentsessions")
}
