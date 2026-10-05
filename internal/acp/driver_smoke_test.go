package acp

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// withFakesInPATH prepends dir to PATH for the duration of the test.
func withFakesInPATH(t *testing.T, dir string) {
	t.Helper()
	t.Setenv("PATH", dir+":"+os.Getenv("PATH"))
}

func collectEvents(t *testing.T, evs <-chan DriverEvent, done <-chan struct{}, wantStop bool) []DriverEvent {
	t.Helper()
	var got []DriverEvent
	deadline := time.After(10 * time.Second)
	for {
		select {
		case ev, ok := <-evs:
			if !ok {
				return got
			}
			got = append(got, ev)
			if wantStop && ev.Kind == EvTurnCompleted {
				return got
			}
		case <-done:
			return got
		case <-deadline:
			t.Fatalf("timed out collecting events; got %d", len(got))
		}
	}
}

func textOfSession(t *testing.T, m *Manager, id string) string {
	t.Helper()
	sess, ok := m.GetSession(id)
	if !ok {
		t.Fatalf("session %s missing", id)
	}
	var sb strings.Builder
	for _, msg := range sess.Messages {
		if msg.Role != "assistant" {
			continue
		}
		for _, b := range msg.Content {
			if b.Type == "text" {
				sb.WriteString(b.Text)
			}
		}
	}
	return sb.String()
}

func TestClaudeDriverSmoke(t *testing.T) {
	fake := filepath.Join(t.TempDir(), "claude")
	script := "#!/usr/bin/env python3\nimport runpy; runpy.run_path(%q, run_name=\"__main__\")\n"
	if err := os.WriteFile(fake, []byte(strings.Replace(script, "%q", "'/tmp/forgeade-smoke/fake_claude.py'", 1)), 0o755); err != nil {
		t.Fatal(err)
	}
	// Simpler: write a wrapper that execs python3 with the script.
	if err := os.WriteFile(fake, []byte("#!/bin/sh\nexec python3 /tmp/forgeade-smoke/fake_claude.py\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	withFakesInPATH(t, filepath.Dir(fake))

	d := NewClaudeDriver()
	sess, evs, err := d.Spawn(context.Background(), SessionConfig{CWD: t.TempDir()})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer sess.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sess.SendMessage(ctx, "user-1", "hi"); err != nil {
		t.Fatalf("send: %v", err)
	}
	got := collectEvents(t, evs, sess.Done(), true)
	var text, deltas string
	sawInitAdoption := false
	for _, ev := range got {
		switch ev.Kind {
		case EvTextDelta:
			deltas += ev.Delta
		case EvMessageCompleted:
			text = ev.Text
		case EvSessionBound:
			// Spawn-time bind uses a generated UUID; the fake's init frame
			// later reports fake-claude-1, which the driver must adopt.
			if ev.SessionID == "fake-claude-1" {
				sawInitAdoption = true
			}
		}
	}
	if !sawInitAdoption {
		t.Errorf("driver never adopted provider session id from system/init")
	}
	if !strings.Contains(deltas, "hello ") || !strings.Contains(text, "fake claude: hi") {
		t.Errorf("missing stream/final text: deltas=%q text=%q", deltas, text)
	}
}

func TestCodexDriverSmoke(t *testing.T) {
	fake := filepath.Join(t.TempDir(), "codex")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\nexec python3 /tmp/forgeade-smoke/fake_codex.py\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	withFakesInPATH(t, filepath.Dir(fake))

	d := NewCodexDriver()
	sess, evs, err := d.Spawn(context.Background(), SessionConfig{CWD: t.TempDir()})
	if err != nil {
		t.Fatalf("spawn: %v", err)
	}
	defer sess.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := sess.SendMessage(ctx, "user-1", "hi"); err != nil {
		t.Fatalf("send: %v", err)
	}
	got := collectEvents(t, evs, sess.Done(), true)
	var text, deltas, threadID string
	for _, ev := range got {
		switch ev.Kind {
		case EvSessionBound:
			threadID = ev.SessionID
		case EvTextDelta:
			deltas += ev.Delta
		case EvMessageCompleted:
			text = ev.Text
		}
	}
	if threadID != "fake-thread-1" {
		t.Errorf("thread id %q", threadID)
	}
	if deltas != "hello from fake codex" || text != "hello from fake codex" {
		t.Errorf("deltas=%q text=%q", deltas, text)
	}
}

func TestPiDriverSmoke(t *testing.T) {
	for _, flavor := range []string{"pi", "omp"} {
		t.Run(flavor, func(t *testing.T) {
			bin := "pi"
			if flavor == "omp" {
				bin = "omp"
			}
			wrapper := "#!/bin/sh\nexec python3 /tmp/forgeade-smoke/fake_pi.py\n"
			if flavor == "omp" {
				wrapper = "#!/bin/sh\nFAKE_FLAVOR=omp exec python3 /tmp/forgeade-smoke/fake_pi.py\n"
			}
			fake := filepath.Join(t.TempDir(), bin)
			if err := os.WriteFile(fake, []byte(wrapper), 0o755); err != nil {
				t.Fatal(err)
			}
			withFakesInPATH(t, filepath.Dir(fake))

			var d AgentDriver
			if flavor == "omp" {
				d = NewOmpDriver()
			} else {
				d = NewPiDriver()
			}
			sess, evs, err := d.Spawn(context.Background(), SessionConfig{CWD: t.TempDir()})
			if err != nil {
				t.Fatalf("spawn: %v", err)
			}
			defer sess.Close()

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := sess.SendMessage(ctx, "user-1", "hi"); err != nil {
				t.Fatalf("send: %v", err)
			}
			got := collectEvents(t, evs, sess.Done(), true)
			var deltas, text, sessionID string
			for _, ev := range got {
				switch ev.Kind {
				case EvSessionBound:
					sessionID = ev.SessionID
				case EvTextDelta:
					deltas += ev.Delta
				case EvMessageCompleted:
					text = ev.Text
				}
			}
			if sessionID != "fake-sess-1" {
				t.Errorf("session id %q", sessionID)
			}
			if deltas != "hello from fake pi" || text != "hello from fake pi" {
				t.Errorf("deltas=%q text=%q", deltas, text)
			}
		})
	}
}

func TestManagerSendViaFakeOMP(t *testing.T) {
	fake := filepath.Join(t.TempDir(), "omp")
	if err := os.WriteFile(fake, []byte("#!/bin/sh\nFAKE_FLAVOR=omp exec python3 /tmp/forgeade-smoke/fake_pi.py\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	withFakesInPATH(t, filepath.Dir(fake))

	m := NewManager(t.TempDir(), nil)
	sess, err := m.CreateSession(context.Background(), "agent-ohmypi", "smoke", t.TempDir())
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	defer m.StopAll()

	// Wait for session-bound from initialize, then send.
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		s, _ := m.GetSession(sess.ID)
		if s.ProviderID != "" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		_ = m.Send(context.Background(), sess.ID, "hello", nil)
	}()

	deadline = time.Now().Add(8 * time.Second)
	for time.Now().Before(deadline) {
		s, _ := m.GetSession(sess.ID)
		if textOfSession(t, m, sess.ID) != "" && s.State == "idle" {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	wg.Wait()

	got := textOfSession(t, m, sess.ID)
	if !strings.Contains(got, "hello from fake pi") {
		t.Errorf("manager send produced %q", got)
	}
	s2, _ := m.GetSession(sess.ID)
	if s2.State != "idle" {
		t.Errorf("state %q after turn", s2.State)
	}
}

func TestOmpProfileValidation(t *testing.T) {
	if _, err := resolveOmpProfile(map[string]string{"OMP_PROFILE": "work-2.dev"}); err != nil {
		t.Errorf("valid profile rejected: %v", err)
	}
	if _, err := resolveOmpProfile(map[string]string{"OMP_PROFILE": "Work"}); err == nil {
		t.Errorf("uppercase profile accepted")
	}
	if _, err := resolveOmpProfile(map[string]string{"OMP_PROFILE": "default"}); err != nil {
		t.Errorf("default profile rejected: %v", err)
	}
}

func TestSummarizeToolCall(t *testing.T) {
	in := json.RawMessage(`{"command":"go test ./...","path":"/x"}`)
	got := summarizeToolCall("bash", in)
	if got != "bash: go test ./..." {
		t.Errorf("summarize: %q", got)
	}
	_ = exec.Command // keep exec import when tests change
}
