//go:build darwin && forge_iabtest

package browseruse

// CONFIRMATION test for the reload-storm root cause: the app mirrors agent
// session JSON into <workspace>/.forge/sessions/ on every update, and the
// Vite dev server watching the workspace reloads the page. Simulate those
// writes while the page is open and watch the load counter.
//
//	FORGE_ADE_IAB_TEST=1 FORGE_ADE_IAB_URL=http://localhost:5174 \
//	  go test -tags forge_iabtest ./internal/browseruse -run TestIABStormWrites -v

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
	"unsafe"
)

func TestIABStormWrites(t *testing.T) {
	url := os.Getenv("FORGE_ADE_IAB_URL")
	if url == "" {
		t.Skip("set FORGE_ADE_IAB_URL")
	}
	ws := os.Getenv("FORGE_ADE_IAB_WS")
	if ws == "" {
		ws = "/Users/lutfiikbalmajid/hasdev/test-app"
	}
	win := makeTestWindow()
	if win == nil {
		t.Skip("no window")
	}
	e := newIABEngine(t.TempDir(), func() unsafe.Pointer { return win }, nil)
	defer e.Close()
	e.SetRect(0, 0, 1200, 800, true)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	if _, err := e.Navigate(ctx, url); err != nil {
		t.Fatalf("navigate: %v", err)
	}

	dir := filepath.Join(ws, ".forge", "sessions")
	_ = os.MkdirAll(dir, 0755)
	probe := filepath.Join(dir, "probe-storm-write.json")
	defer os.Remove(probe)
	payload, _ := os.ReadFile(filepath.Join(dir, "session-1791033157297.json"))
	if len(payload) == 0 {
		payload = []byte(`{"id":"probe"}`)
	}

	stop := make(chan struct{})
	go func() {
		for {
			select {
			case <-stop:
				return
			case <-time.After(2 * time.Second):
				_ = os.WriteFile(probe, payload, 0644) // same as SaveAgentSessionDisk
			}
		}
	}()

	for i := 1; i <= 8; i++ {
		time.Sleep(2500 * time.Millisecond)
		n, _ := e.evalString(ctx, "String(window.__forgeLoads || '?')")
		marker, _ := e.evalString(ctx, "window.__fm || 'unset'")
		t.Logf("writes t+%02ds loads=%s marker=%s", i*25/10, n, marker)
	}
	close(stop)
}
