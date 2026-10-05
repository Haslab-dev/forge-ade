//go:build darwin && forge_iabtest

package browseruse

// Tests the "hidden navigation" hypothesis for the reload storm: the real app
// creates the WKWebView hidden and only unhides it when the viewer panel
// auto-opens, so the first navigate happens on a hidden view. Verify whether
// a hidden-load + later unhide triggers repeated document loads on a Vite page.
//
//	FORGE_ADE_IAB_TEST=1 FORGE_ADE_IAB_URL=http://localhost:5176 \
//	  go test -tags forge_iabtest ./internal/browseruse -run TestIABHiddenNavigate -v

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"
	"unsafe"
)

func loads(t *testing.T, e *iabEngine, ctx context.Context, label string) {
	n, _ := e.evalString(ctx, "String(window.__forgeLoads || '?')")
	marker, _ := e.evalString(ctx, "window.__fm || 'unset'")
	t.Logf("%-32s loads=%s (marker: %s)", label, n, marker)
}

func TestIABHiddenNavigate(t *testing.T) {
	url := os.Getenv("FORGE_ADE_IAB_URL")
	if url == "" {
		t.Skip("set FORGE_ADE_IAB_URL")
	}
	win := makeTestWindow()
	if win == nil {
		t.Skip("no window")
	}
	e := newIABEngine(t.TempDir(), func() unsafe.Pointer { return win }, nil)
	defer e.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	// Phase 1: navigate while the view is hidden and 0-sized — exactly what
	// the real app does (browser_navigate arrives before the panel auto-opens).
	loads(t, e, ctx, "after start (hidden)")

	if _, err := e.Navigate(ctx, url); err != nil {
		t.Fatalf("navigate: %v", err)
	}
	_, _ = e.evalString(ctx, "window.__fm = 'alive'")
	loads(t, e, ctx, "after navigate (hidden)")

	for i := 1; i <= 4; i++ {
		time.Sleep(1500 * time.Millisecond)
		loads(t, e, ctx, fmt.Sprintf("hidden since navigate +%d.5s", i))
	}

	// Phase 2: unhide (the panel auto-opens) — does the unhide itself cause
	// more reloads?
	e.SetRect(0, 0, 1200, 800, true)
	loads(t, e, ctx, "after unhide")
	for i := 1; i <= 6; i++ {
		time.Sleep(1500 * time.Millisecond)
		loads(t, e, ctx, fmt.Sprintf("visible since unhide +%d.5s", i))
	}
}
