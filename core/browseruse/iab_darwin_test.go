//go:build darwin && forge_iabtest

package browseruse

// Live end-to-end test for the in-app WKWebView engine: creates a real
// NSWindow (not the Wails window), hosts the engine's WKWebView, and drives
// navigate → snapshot → click → type → screenshot. The main goroutine pumps
// the Cocoa runloop so WKWebView completion handlers can fire.
//
// Run: FORGE_ADE_IAB_TEST=1 go test -tags forge_iabtest ./internal/browseruse -run TestIABEngineEndToEnd -v

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
	"unsafe"
)

func TestMain(m *testing.M) {
	// Pump the Cocoa main runloop on the locked main thread; the test suite
	// runs on a separate goroutine.
	go func() {
		os.Exit(m.Run())
	}()
	runLoopForever()
}

// TestIABEngineEndToEnd exercises the real embedded WKWebView.
func TestIABEngineEndToEnd(t *testing.T) {
	if os.Getenv("FORGE_ADE_IAB_TEST") == "" {
		t.Skip("set FORGE_ADE_IAB_TEST=1 to run the live in-app browser test")
	}
	win := makeTestWindow()
	if win == nil {
		t.Skip("could not create a test window (no WindowServer session?)")
	}

	page := `data:text/html,<h1>Forge IAB Test</h1>
		<button onclick="document.getElementById('out').textContent='clicked-'+Date.now()">Press me</button>
		<input placeholder="Email" />
		<div id="out"></div>`

	e := newIABEngine(t.TempDir(), func() unsafe.Pointer { return win }, nil)
	defer e.Close()
	e.SetRect(0, 0, 1200, 800, true) // position over the whole test window

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	res, err := e.Navigate(ctx, page)
	if err != nil {
		t.Fatalf("navigate: %v", err)
	}
	t.Logf("navigate result keys: %v (url=%q title=%q)", mapKeys(res), res["url"], res["title"])
	snap, _ := res["snapshot"].(string)
	if snap == "" {
		if s, err := e.Snapshot(ctx); err != nil {
			t.Fatalf("direct snapshot error: %v", err)
		} else {
			snap = s
		}
	}
	if !strings.Contains(snap, "Forge IAB Test") || !strings.Contains(snap, "ref=") {
		t.Fatalf("bad snapshot:\n%s", snap)
	}

	ref := findRef(snap, "button", "Press me")
	if ref == "" {
		t.Fatalf("button ref missing:\n%s", snap)
	}
	res, err = e.Click(ctx, ref)
	if err != nil {
		t.Fatalf("click: %v", err)
	}
	if snap2, _ := res["snapshot"].(string); !strings.Contains(snap2, "clicked-") {
		t.Fatalf("click had no effect:\n%s", snap2)
	}

	inputRef := findRef(res["snapshot"].(string), "textbox", "Email")
	if inputRef == "" {
		t.Fatalf("input ref missing")
	}
	if _, err := e.Type(ctx, inputRef, "test@example.com", false); err != nil {
		t.Fatalf("type: %v", err)
	}

	path, err := e.Screenshot(ctx)
	if err != nil {
		t.Fatalf("screenshot: %v", err)
	}
	if fi, err := os.Stat(path); err != nil || fi.Size() < 2000 {
		t.Fatalf("screenshot suspiciously small: %s (%v)", path, err)
	}
	t.Logf("screenshot saved: %s", path)
}

func findRef(snapshot, role, label string) string {
	for _, line := range strings.Split(snapshot, "\n") {
		if strings.Contains(line, role) && strings.Contains(line, label) {
			if i := strings.Index(line, "ref=e"); i >= 0 {
				r := line[i+len("ref="):]
				if j := strings.IndexAny(r, " :"); j >= 0 {
					r = r[:j]
				}
				return r
			}
		}
	}
	return ""
}

func mapKeys(m map[string]any) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
