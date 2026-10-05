//go:build darwin && forge_iabtest

package browseruse

// Diagnoses the "counter resets after every browser action" report: navigate
// to the Vite app, click the counter, and watch the count + a reload marker
// across several actions and delays.
//
//	FORGE_ADE_IAB_TEST=1 FORGE_ADE_IAB_URL=http://localhost:5176 \
//	  go test -tags forge_iabtest ./internal/browseruse -run TestIABCounterReset -v

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"
	"unsafe"
)

func TestIABCounterReset(t *testing.T) {
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
	e.SetRect(0, 0, 1200, 800, true)

	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	count := func(label string) {
		s, _ := e.evalString(ctx, "(document.body.innerText.match(/Count is \\d+/)||['?'])[0]")
		marker, _ := e.evalString(ctx, "window.__fm || 'unset'")
		t.Logf("%-28s %s (reload-marker: %s)", label, s, marker)
	}

	if _, err := e.Navigate(ctx, url); err != nil {
		t.Fatalf("navigate: %v", err)
	}
	_, _ = e.evalString(ctx, "window.__fm = 'alive'")
	count("after navigate")

	// The exact agent flow: snapshot → click (twice), with the same delays.
	for i := 1; i <= 3; i++ {
		if _, err := e.Snapshot(ctx); err != nil {
			t.Fatalf("snapshot %d: %v", i, err)
		}
		if _, err := e.Click(ctx, "e1"); err != nil {
			t.Errorf("click %d: %v", i, err)
			continue
		}
		count("after click " + string(rune('0'+i)))
		time.Sleep(1200 * time.Millisecond) // agent-like gap; watch for vite reload
		count("1.2s after click " + string(rune('0'+i)))
	}

	// Extended stability watch: does the page reload on its own over time?
	for i := 1; i <= 6; i++ {
		time.Sleep(2500 * time.Millisecond)
		count(fmt.Sprintf("t+%d.%ds", i*2, 5))
	}
}

// findRef is reused from iab_darwin_test.go; strings import used above.
var _ = strings.TrimSpace
