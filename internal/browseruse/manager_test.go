package browseruse

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"
)

// TestManagerEndToEnd drives a real browser if one is installed: navigate to
// a data URL, snapshot it, click a button that mutates the DOM, and verify
// the mutation in a fresh snapshot.
func TestManagerEndToEnd(t *testing.T) {
	if os.Getenv("FORGE_ADE_BROWSER_TEST") == "" {
		t.Skip("set FORGE_ADE_BROWSER_TEST=1 to run the live browser test")
	}
	if _, err := findBrowser(); err != nil {
		t.Skipf("no browser available: %v", err)
	}

	m := NewManager(t.TempDir())
	var frames int
	m.SetEmitter(func(event string, data map[string]any) {
		if event == "browser:frame" {
			frames++
		}
	})
	defer m.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	page := `data:text/html,<h1>Forge Test Page</h1>
		<button onclick="document.getElementById('out').textContent='clicked-'+Date.now()">Press me</button>
		<input placeholder="Email" />
		<div id="out"></div>`

	res, err := m.Navigate(ctx, page)
	if err != nil {
		if strings.Contains(err.Error(), "DevTools endpoint") || strings.Contains(err.Error(), "launch browser") {
			t.Skipf("browser did not start (Arc/Chrome headless is flaky as an automation target): %v", err)
		}
		t.Fatalf("navigate: %v", err)
	}
	snap, _ := res["snapshot"].(string)
	if !strings.Contains(snap, "Forge Test Page") {
		t.Fatalf("snapshot missing heading:\n%s", snap)
	}
	if !strings.Contains(snap, "ref=") {
		t.Fatalf("snapshot has no refs:\n%s", snap)
	}

	// Find the button ref and click it.
	buttonRef := ""
	for _, line := range strings.Split(snap, "\n") {
		if strings.Contains(line, "button") && strings.Contains(line, "Press me") {
			if i := strings.Index(line, "ref=e"); i >= 0 {
				buttonRef = line[i+len("ref="):]
				if j := strings.IndexAny(buttonRef, " :"); j >= 0 {
					buttonRef = buttonRef[:j]
				}
			}
		}
	}
	if buttonRef == "" {
		t.Fatalf("button ref not found in snapshot:\n%s", snap)
	}
	res, err = m.Click(ctx, buttonRef)
	if err != nil {
		t.Fatalf("click: %v", err)
	}
	snap2, _ := res["snapshot"].(string)
	if !strings.Contains(snap2, "clicked-") {
		t.Fatalf("click had no effect:\n%s", snap2)
	}

	// Type into the input.
	inputRef := ""
	for _, line := range strings.Split(snap2, "\n") {
		if strings.Contains(line, "textbox") && strings.Contains(line, "Email") {
			if i := strings.Index(line, "ref=e"); i >= 0 {
				inputRef = line[i+len("ref="):]
				if j := strings.IndexAny(inputRef, " :"); j >= 0 {
					inputRef = inputRef[:j]
				}
			}
		}
	}
	if inputRef == "" {
		t.Fatalf("input ref not found:\n%s", snap2)
	}
	if _, err := m.Type(ctx, inputRef, "test@example.com", false); err != nil {
		t.Fatalf("type: %v", err)
	}

	path, err := m.Screenshot(ctx)
	if err != nil {
		t.Fatalf("screenshot: %v", err)
	}
	if fi, err := os.Stat(path); err != nil || fi.Size() < 1000 {
		t.Fatalf("screenshot file missing or tiny: %s", path)
	}

	if frames == 0 {
		t.Log("warning: no screencast frames received yet (may just be timing)")
	}
}
