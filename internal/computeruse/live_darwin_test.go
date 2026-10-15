package computeruse

import (
	"os"
	"testing"
)

// TestScreenshotLive exercises the real screencapture path. It skips when
// Screen Recording is not granted to the test binary (grants are per-bundle,
// so CI typically skips while a locally-built-and-granted app runs it).
func TestScreenshotLive(t *testing.T) {
	m := NewManager()
	if !m.ScreenCaptureTrusted() {
		t.Skip("Screen Recording not granted to the test binary")
	}
	home := t.TempDir()
	path, w, h, err := m.Screenshot(home, Region{})
	if err != nil {
		t.Fatalf("Screenshot: %v", err)
	}
	defer os.Remove(path)
	if w <= 0 || h <= 0 {
		t.Fatalf("bad dimensions %dx%d", w, h)
	}
	if st, err := os.Stat(path); err != nil || st.Size() == 0 {
		t.Fatalf("screenshot missing/empty: %v", err)
	}
}
