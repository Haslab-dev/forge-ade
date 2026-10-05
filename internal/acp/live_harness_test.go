package acp

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// liveHarness defines one provider's live end-to-end check.
type liveHarness struct {
	agentID string // Manager agent config id
	binary  string // binary that must exist to run the test
	model   string // model requested for this harness
	timeout time.Duration
}

var liveHarnesses = []liveHarness{
	{agentID: "agent-codex", binary: "codex", model: "glm-5-3-flash", timeout: 4 * time.Minute},
	{agentID: "agent-claude", binary: "claude", model: "deepseek-v4-1-flash", timeout: 4 * time.Minute},
	{agentID: "agent-ohmypi", binary: "omp", model: "glm-coding/glm-5.3-flash", timeout: 4 * time.Minute},
	{agentID: "agent-opencode", binary: "opencode", model: "myairouter_cloud/kenari/glm-5-3-flash", timeout: 4 * time.Minute},
	{agentID: "agent-pi", binary: "pi", model: "myairouter_cloud/kenari/glm-5-3-flash", timeout: 4 * time.Minute},
}

// assistantText collects all assistant text blocks of a session.
func assistantText(t *testing.T, m *Manager, sessionID string) string {
	t.Helper()
	s, ok := m.GetSession(sessionID)
	if !ok {
		return ""
	}
	var sb strings.Builder
	for _, msg := range s.Messages {
		if msg.Role != "assistant" {
			continue
		}
		for _, b := range msg.Content {
			if b.Type == "text" {
				sb.WriteString(b.Text)
			}
		}
	}
	return strings.TrimSpace(sb.String())
}

// waitForIdle polls until the session is idle with assistant output.
func waitForIdle(t *testing.T, m *Manager, sessionID string, timeout time.Duration) (string, string) {
	t.Helper()
	dl := time.Now().Add(timeout)
	for time.Now().Before(dl) {
		s, _ := m.GetSession(sessionID)
		if s.State == "idle" && s.PendingPermission == nil {
			return assistantText(t, m, sessionID), s.State
		}
		time.Sleep(150 * time.Millisecond)
	}
	s, _ := m.GetSession(sessionID)
	return assistantText(t, m, sessionID), s.State
}

// TestLiveAllHarnesses drives one real prompt turn on every installed agent
// and asserts a non-empty assistant response through the Manager pipeline.
func TestLiveAllHarnesses(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	cwd = strings.TrimSuffix(cwd, "/internal/acp")

	for _, h := range liveHarnesses {
		h := h
		t.Run(h.agentID, func(t *testing.T) {
			if _, err := exec.LookPath(h.binary); err != nil {
				t.Skipf("%s not installed", h.binary)
			}
			m := NewManager(t.TempDir(), nil)
			defer m.StopAll()

			sess, err := m.CreateSession(context.Background(), h.agentID, "live-"+h.agentID, cwd)
			if err != nil {
				t.Fatalf("create session: %v", err)
			}

			// Model selection (best-effort; drivers fall back to configured model).
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
			_ = m.SetSessionModel(ctx, sess.ID, h.model)
			cancel()

			if err := m.Send(context.Background(), sess.ID, "Reply with exactly the single word: pong", nil); err != nil {
				t.Fatalf("send: %v", err)
			}
			text, state := waitForIdle(t, m, sess.ID, h.timeout)
			if state != "idle" {
				t.Fatalf("turn did not go idle within %s (state=%q, text=%q)", h.timeout, state, text)
			}
			if text == "" {
				t.Fatalf("turn finished with empty assistant text")
			}
			t.Logf("[%s] model=%s reply=%q", h.agentID, h.model, text)
		})
	}
}

// TestLiveHarnessSessionBinding verifies each installed harness reaches the
// SessionBound handshake without a model call (fast prerequisite check).
func TestLiveHarnessSessionBinding(t *testing.T) {
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	cwd = strings.TrimSuffix(cwd, "/internal/acp")

	for _, h := range liveHarnesses {
		h := h
		t.Run(h.agentID, func(t *testing.T) {
			if _, err := exec.LookPath(h.binary); err != nil {
				t.Skipf("%s not installed", h.binary)
			}
			m := NewManager(t.TempDir(), nil)
			defer m.StopAll()

			sess, err := m.CreateSession(context.Background(), h.agentID, "bind-"+h.agentID, cwd)
			if err != nil {
				t.Fatalf("create session: %v", err)
			}
			dl := time.Now().Add(90 * time.Second)
			for time.Now().Before(dl) {
				s, _ := m.GetSession(sess.ID)
				if s.ProviderID != "" {
					t.Logf("[%s] bound to provider session %s", h.agentID, s.ProviderID)
					return
				}
				time.Sleep(100 * time.Millisecond)
			}
			t.Fatalf("session never bound to a provider id")
		})
	}
}
