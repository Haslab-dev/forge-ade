package agentsession

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/hasdev/forge-ade/internal/events"
)

func newTestManager(t *testing.T) (*Manager, *ConfigStore, string) {
	t.Helper()
	dataDir := t.TempDir()
	bus := events.NewBus()
	configs := NewConfigStore(dataDir)
	m := NewManager(bus, dataDir, configs)
	return m, configs, dataDir
}

func TestConfigStoreDefaultsAndSave(t *testing.T) {
	m, configs, dir := newTestManager(t)
	_ = m

	got := configs.List()
	if len(got) != len(DefaultAgentCLIConfigs()) {
		t.Fatalf("expected %d default configs, got %d", len(DefaultAgentCLIConfigs()), len(got))
	}
	var claude *AgentCLIConfig
	for i := range got {
		if got[i].ID == "claude-code" {
			claude = &got[i]
		}
	}
	if claude == nil || claude.Executable != "claude" || !claude.Enabled {
		t.Fatalf("claude-code default missing or wrong: %+v", claude)
	}

	// User edit persists across reload.
	claude.Executable = "/opt/tools/claude"
	if _, err := configs.Save(*claude); err != nil {
		t.Fatalf("save: %v", err)
	}
	reloaded := NewConfigStore(dir)
	after, err := reloaded.Get("claude-code")
	if err != nil {
		t.Fatalf("get after reload: %v", err)
	}
	if after.Executable != "/opt/tools/claude" {
		t.Fatalf("user edit lost after reload: %+v", after)
	}

	// Reset restores the built-in executable.
	if _, err := reloaded.Reset("claude-code"); err != nil {
		t.Fatalf("reset: %v", err)
	}
	after, _ = reloaded.Get("claude-code")
	if after.Executable != "claude" {
		t.Fatalf("reset did not restore default: %+v", after)
	}
}

func TestSettingsStoreDefaults(t *testing.T) {
	_, _, dir := newTestManager(t)
	s := NewSettingsStore(dir).Get()
	if s.DefaultMode != "terminal" {
		t.Fatalf("default mode must be terminal, got %q", s.DefaultMode)
	}
	if s.Terminal.FontSize != 13 || s.Terminal.CursorStyle != "block" {
		t.Fatalf("unexpected terminal defaults: %+v", s.Terminal)
	}
}

func TestRenamedDefaultMigrates(t *testing.T) {
	dataDir := t.TempDir()
	path := dataDir + "/agent-clis.json"
	// A config file saved by an older build: antigravity still holds the old
	// default executable "antigravity" (renamed to "agy").
	old := DefaultAgentCLIConfigs()
	for i := range old {
		if old[i].ID == "antigravity" {
			old[i].Executable = "antigravity"
		}
	}
	raw, _ := json.Marshal(old)
	if err := os.WriteFile(path, raw, 0644); err != nil {
		t.Fatalf("seed: %v", err)
	}

	configs := NewConfigStore(dataDir)
	got, err := configs.Get("antigravity")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Executable != "agy" {
		t.Fatalf("stale default should migrate to agy, got %q", got.Executable)
	}

	// A user-edited executable is preserved.
	edited, _ := configs.Get("antigravity")
	edited.Executable = "/usr/local/bin/my-agy"
	_, _ = configs.Save(edited)
	after := NewConfigStore(dataDir)
	got, _ = after.Get("antigravity")
	if got.Executable != "/usr/local/bin/my-agy" {
		t.Fatalf("user edit should survive reload, got %q", got.Executable)
	}
}

func TestSessionLifecycle(t *testing.T) {
	m, _, dir := newTestManager(t)

	// Use the shell as a stand-in "agent CLI" via a custom config so the test
	// never depends on an agent binary being installed.
	cfg := AgentCLIConfig{
		ID: "test-cli", Name: "Test CLI", Executable: "/bin/sh", Args: []string{"-i"}, Enabled: true,
	}
	if _, err := m.configs.Save(cfg); err != nil {
		t.Fatalf("save test config: %v", err)
	}

	rec, err := m.Create("test-cli", t.TempDir(), "diagnostic")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if rec.Status != StatusRunning {
		t.Fatalf("expected running, got %s", rec.Status)
	}

	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(m.OutputTail(rec.ID), "$") || strings.Contains(m.OutputTail(rec.ID), "#") {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	if _, err := m.Write(rec.ID, []byte("echo forged_session_marker\n")); err != nil {
		t.Fatalf("write: %v", err)
	}
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(m.OutputTail(rec.ID), "forged_session_marker") {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !strings.Contains(m.OutputTail(rec.ID), "forged_session_marker") {
		t.Fatalf("output tail never received echoed marker: %q", m.OutputTail(rec.ID))
	}

	if err := m.Stop(rec.ID); err != nil {
		t.Fatalf("stop: %v", err)
	}
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		rec, _ = m.Get(rec.ID)
		if !rec.Running() {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if rec.Running() {
		t.Fatalf("session still running after stop: %+v", rec)
	}
	if rec.Status != StatusTerminated {
		t.Fatalf("user-initiated stop should end terminated, got %s", rec.Status)
	}

	// History survives manager reload (app restart semantics). Stop flips the
	// record to terminated immediately; the reaper flushes the output log
	// moments later, so poll briefly for the durable tail.
	m2 := NewManager(events.NewBus(), dir, m.configs)
	list := m2.List()
	if len(list) != 1 {
		t.Fatalf("expected 1 persisted session, got %d", len(list))
	}
	if list[0].Status != StatusTerminated {
		t.Fatalf("persisted session should be terminated after reload, got %s", list[0].Status)
	}
	tailDeadline := time.Now().Add(5 * time.Second)
	for {
		if strings.Contains(m2.OutputTail(rec.ID), "forged_session_marker") {
			break
		}
		if time.Now().After(tailDeadline) {
			t.Fatalf("persisted output tail missing marker: %q", m2.OutputTail(rec.ID))
		}
		time.Sleep(50 * time.Millisecond)
	}

	if err := m2.Delete(rec.ID); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, ok := m2.Get(rec.ID); ok {
		t.Fatal("session record should be gone after delete")
	}
	if m2.OutputTail(rec.ID) != "" {
		t.Fatal("output log should be gone after delete")
	}
}

func TestCreateFailsForUnknownAgent(t *testing.T) {
	m, _, _ := newTestManager(t)
	if _, err := m.Create("does-not-exist", "", ""); err == nil {
		t.Fatal("expected error for unknown agent id")
	}
}

func TestDeriveTitle(t *testing.T) {
	cases := []struct {
		in     string
		want   string
		wantOK bool
	}{
		{"analyze this repository", "Analyze this repository", true},
		{"  fix   the   login bug  ", "Fix the login bug", true},
		{"/switch", "", false},                     // CLI command
		{"!ls -la", "", false},                     // bash escape (omp)
		{"$print('x')", "", false},                 // python escape (omp)
		{"ok", "", false},                          // too short
		{"\x1b[A\x1b[B", "", false},                // arrow keys only
		{"testing\x1b[2K~", "Testing~", true},      // ANSI stripped, letters kept
	}
	for _, tc := range cases {
		got, ok := deriveTitle(tc.in, "")
		if ok != tc.wantOK || (ok && got != tc.want) {
			t.Fatalf("deriveTitle(%q) = %q,%v; want %q,%v", tc.in, got, ok, tc.want, tc.wantOK)
		}
	}
	// Secret hygiene: typing while a password prompt is on screen never titles.
	if _, ok := deriveTitle("hunter2 secretpw", "user@host: sudo [sudo] password for user:"); ok {
		t.Fatal("password input must not become the title")
	}
}

func TestAutoTitleFromFirstInput(t *testing.T) {
	m, _, _ := newTestManager(t)
	cfg := AgentCLIConfig{ID: "title-cli", Name: "Title CLI", Executable: "/bin/sh", Args: []string{"-i"}, Enabled: true}
	if _, err := m.configs.Save(cfg); err != nil {
		t.Fatalf("save: %v", err)
	}
	rec, err := m.Create("title-cli", t.TempDir(), "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if !rec.TitleAuto {
		t.Fatalf("session created without a title must be auto-titled")
	}

	// Keystrokes arrive in fragments (as from a real terminal) then Enter.
	for _, frag := range []string{"analy", "ze ", "this re", "po\r"} {
		if _, err := m.Write(rec.ID, []byte(frag)); err != nil {
			t.Fatalf("write %q: %v", frag, err)
		}
	}

	deadline := time.Now().Add(3 * time.Second)
	for {
		rec, _ = m.Get(rec.ID)
		if rec.Title == "Analyze this repo" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("title never applied; got %q", rec.Title)
		}
		time.Sleep(40 * time.Millisecond)
	}
	if rec.TitleAuto {
		t.Fatal("auto title should clear titleAuto")
	}
	// Subsequent input no longer renames.
	_, _ = m.Write(rec.ID, []byte("something else entirely\r"))
	time.Sleep(150 * time.Millisecond)
	if rec, _ = m.Get(rec.ID); rec.Title != "Analyze this repo" {
		t.Fatalf("title changed after being set: %q", rec.Title)
	}
	_ = m.Stop(rec.ID)
	time.Sleep(200 * time.Millisecond)
	m.Shutdown()
}

func TestResumeHintCapturedAndUsed(t *testing.T) {
	if got := resumeHintFromOutput("Resume with -c (or command below):\n  agy --conversation=ab904070-0a5c-451e-9a8d-16adddfe87b4"); got == nil || got[0] != "--conversation=ab904070-0a5c-451e-9a8d-16adddfe87b4" {
		t.Fatalf("conversation hint not captured: %v", got)
	}
	if got := resumeHintFromOutput("resuming… omp --resume=ab12cd34"); got == nil || got[0] != "--resume=ab12cd34" {
		t.Fatalf("resume id hint not captured: %v", got)
	}
	if got := resumeHintFromOutput("no hints in this output"); got != nil {
		t.Fatalf("false positive: %v", got)
	}

	// Manager flow: a fake CLI prints a resume hint at startup; after a
	// restart the hint must be on the record (and would drive the relaunch).
	m, _, _ := newTestManager(t)
	cfg := AgentCLIConfig{
		ID:         "hint-cli",
		Name:       "Hint CLI",
		Executable: "/bin/sh",
		Args:       []string{"-c", "echo 'resume with: hintcli --conversation=abcd1234-0000-9999'; exec /bin/sh -i"},
		Enabled:    true,
	}
	if _, err := m.configs.Save(cfg); err != nil {
		t.Fatalf("save: %v", err)
	}
	rec, err := m.Create("hint-cli", t.TempDir(), "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		rec, _ = m.Get(rec.ID)
		if len(rec.ResumeHint) == 1 && rec.ResumeHint[0] == "--conversation=abcd1234-0000-9999" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("hint never captured; got %v", rec.ResumeHint)
		}
		time.Sleep(40 * time.Millisecond)
	}

	after, err := m.Restart(rec.ID)
	if err != nil {
		t.Fatalf("restart: %v", err)
	}
	if !after.Running() {
		t.Fatalf("restarted session should be running, got %s", after.Status)
	}
	// The restarted CLI prints the hint again (same script); the captured
	// hint must survive and still name the conversation.
	deadline = time.Now().Add(5 * time.Second)
	for {
		if after, _ = m.Get(rec.ID); len(after.ResumeHint) == 1 && after.ResumeHint[0] == "--conversation=abcd1234-0000-9999" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("hint lost after restart: %v", after.ResumeHint)
		}
		time.Sleep(40 * time.Millisecond)
	}
	_ = m.Stop(rec.ID)
	time.Sleep(200 * time.Millisecond)
	m.Shutdown()
}

func TestRestartReusesRecord(t *testing.T) {
	m, _, _ := newTestManager(t)
	cfg := AgentCLIConfig{ID: "restart-cli", Name: "Restart CLI", Executable: "/bin/sh", Args: []string{"-i"}, Enabled: true}
	if _, err := m.configs.Save(cfg); err != nil {
		t.Fatalf("save: %v", err)
	}
	rec, err := m.Create("restart-cli", t.TempDir(), "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	time.Sleep(100 * time.Millisecond)

	after, err := m.Restart(rec.ID)
	if err != nil {
		t.Fatalf("restart: %v", err)
	}
	if after.ID != rec.ID {
		t.Fatalf("restart must reuse the session id: %s != %s", after.ID, rec.ID)
	}
	if !after.Running() {
		t.Fatalf("restarted session should be running, got %s", after.Status)
	}

	// The old generation's reaper must not clobber the new process: the
	// session stays running, input reaches the new PTY, and output streams.
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		if cur, _ := m.Get(rec.ID); !cur.Running() {
			t.Fatalf("old reaper finalized the record after restart: %+v", cur)
		}
		if strings.Contains(m.OutputTail(rec.ID), "restart_alive_marker") {
			break
		}
		if _, err := m.Write(rec.ID, []byte("echo restart_alive_marker\n")); err != nil {
			t.Fatalf("write to restarted session failed (handle clobbered?): %v", err)
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !strings.Contains(m.OutputTail(rec.ID), "restart_alive_marker") {
		t.Fatalf("restarted session never echoed marker; tail: %q", m.OutputTail(rec.ID))
	}

	// Drain the restarted process before temp-dir cleanup.
	_ = m.Stop(rec.ID)
	deadline = time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if cur, ok := m.Get(rec.ID); ok && !cur.Running() {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	m.Shutdown()
}

func TestRestartCarriesHistory(t *testing.T) {
	m, _, _ := newTestManager(t)
	cfg := AgentCLIConfig{ID: "hist-cli", Name: "Hist CLI", Executable: "/bin/sh", Args: []string{"-i"}, Enabled: true}
	if _, err := m.configs.Save(cfg); err != nil {
		t.Fatalf("save: %v", err)
	}
	rec, err := m.Create("hist-cli", t.TempDir(), "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	// Write a marker in the first generation and wait for its echo.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		if strings.Contains(m.OutputTail(rec.ID), "first_generation_history") {
			break
		}
		_, _ = m.Write(rec.ID, []byte("echo first_generation_history\n"))
		time.Sleep(50 * time.Millisecond)
	}

	if _, err := m.Restart(rec.ID); err != nil {
		t.Fatalf("restart: %v", err)
	}
	// The new generation's ring is seeded with the previous history.
	if tail := m.OutputTail(rec.ID); !strings.Contains(tail, "first_generation_history") {
		t.Fatalf("restart lost previous history; tail: %q", tail)
	}
	_ = m.Stop(rec.ID)
	time.Sleep(200 * time.Millisecond)
	m.Shutdown()
}
