//go:build darwin && forge_iabtest

package browseruse

// Reproduces the "ref e1 not found" click failure against a real loaded page
// (the test-app Vite dev server). Run:
//
//	FORGE_ADE_IAB_TEST=1 FORGE_ADE_IAB_URL=http://localhost:5174 \
//	  go test -tags forge_iabtest ./internal/browseruse -run TestIABClickStale -v

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
	"unsafe"
)

func TestIABClickStale(t *testing.T) {
	url := os.Getenv("FORGE_ADE_IAB_URL")
	if url == "" {
		t.Skip("set FORGE_ADE_IAB_URL to a reachable page")
	}
	win := makeTestWindow()
	if win == nil {
		t.Skip("no window")
	}
	e := newIABEngine(t.TempDir(), func() unsafe.Pointer { return win }, nil)
	defer e.Close()
	e.SetRect(0, 0, 1200, 800, true)

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	res, err := e.Navigate(ctx, url)
	if err != nil {
		t.Fatalf("navigate: %v", err)
	}
	snap, _ := res["snapshot"].(string)
	t.Logf("snapshot after navigate (first 300 chars):\n%.300s", snap)

	// Reproduce the exact agent sequence: click a ref from the navigate
	// snapshot immediately.
	ref := findRef(snap, "button", "")
	if ref == "" {
		ref = firstRef(snap)
	}
	if ref == "" {
		t.Fatalf("no refs in snapshot")
	}
	if _, err := e.Click(ctx, ref); err != nil {
		t.Errorf("click right after navigate: %v", err)
	} else {
		t.Logf("click right after navigate OK (ref=%s)", ref)
	}

	// Again: fresh snapshot then click.
	snap2, err := e.Snapshot(ctx)
	if err != nil {
		t.Fatalf("snapshot: %v", err)
	}
	ref2 := firstRef(snap2)
	if ref2 == "" {
		t.Fatalf("no refs in second snapshot:\n%s", snap2)
	}
	if _, err := e.Click(ctx, ref2); err != nil {
		t.Errorf("click after fresh snapshot: %v", err)
	} else {
		t.Logf("click after fresh snapshot OK (ref=%s)", ref2)
	}

	// Check whether the page reloads between calls (HMR etc.): set a marker,
	// wait, see if it survives.
	if _, err := e.evalString(ctx, "window.__forgeMarker = 'alive'; 'ok'"); err != nil {
		t.Fatalf("marker set: %v", err)
	}
	time.Sleep(2500 * time.Millisecond)
	marker, _ := e.evalString(ctx, "window.__forgeMarker || 'gone'")
	t.Logf("marker after 2.5s: %s (page %s)", marker, map[bool]string{true: "stable", false: "RELOADED"}[marker == "alive"])

	// And verify attributes actually persist after a snapshot.
	if _, err := e.Snapshot(ctx); err != nil {
		t.Fatalf("snapshot3: %v", err)
	}
	attr, _ := e.evalString(ctx, fmt.Sprintf("document.querySelector('[data-fref=%q]') ? 'present' : 'missing'", ref2))
	t.Logf("data-fref %s after fresh snapshot: %s", ref2, attr)
	if attr == "missing" {
		// Dump how many refs exist at all.
		n, _ := e.evalString(ctx, "String(document.querySelectorAll('[data-fref]').length)")
		t.Errorf("attribute missing right after snapshot; total attrs present: %s", n)
	}
}

func firstRef(snapshot string) string {
	i := strings.Index(snapshot, "ref=e")
	if i < 0 {
		return ""
	}
	r := snapshot[i+len("ref="):]
	if j := strings.IndexAny(r, " :"); j >= 0 {
		r = r[:j]
	}
	return r
}
