//go:build darwin && forge_iabtest

package browseruse

// Long-horizon reload watch: the agent sessions saw reload storms 20-60s
// after navigation; earlier harness tests only observed the first ~13s.
// Navigate, then sample the load counter every 2s for 90s.
//
//	FORGE_ADE_IAB_TEST=1 FORGE_ADE_IAB_URL=http://localhost:5176 \
//	  go test -tags forge_iabtest ./internal/browseruse -run TestIABLongWatch -v

import (
	"context"
	"os"
	"testing"
	"time"
	"unsafe"
)

func TestIABLongWatch(t *testing.T) {
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

	ctx, cancel := context.WithTimeout(context.Background(), 180*time.Second)
	defer cancel()

	if _, err := e.Navigate(ctx, url); err != nil {
		t.Fatalf("navigate: %v", err)
	}
	_, _ = e.evalString(ctx, "window.__fm = 'alive'")

	for i := 1; i <= 45; i++ {
		time.Sleep(2 * time.Second)
		n, _ := e.evalString(ctx, "String(window.__forgeLoads || '?')")
		marker, _ := e.evalString(ctx, "window.__fm || 'unset'")
		count, _ := e.evalString(ctx, "(document.body.innerText.match(/Count is \\d+/)||['?'])[0]")
		if i%3 == 0 || n != "1" || marker != "alive" {
			t.Logf("t+%03ds loads=%s count=%s marker=%s", i*2, n, count, marker)
		}
	}
}
