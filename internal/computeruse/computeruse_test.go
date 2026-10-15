package computeruse

import (
	"bytes"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/hasdev/forge-ade/internal/sandbox"
)

func TestValidateClick(t *testing.T) {
	// Empty button defaults to left.
	spec, err := ValidateClick(10, 20, "", false)
	if err != nil {
		t.Fatalf("ValidateClick: %v", err)
	}
	if spec.Button != "left" || spec.X != 10 || spec.Y != 20 {
		t.Fatalf("unexpected spec %+v", spec)
	}

	if _, err := ValidateClick(1, 2, "sideways", false); err == nil {
		t.Error("invalid button accepted")
	}
	if _, err := ValidateClick(-1, 2, "left", false); err == nil {
		t.Error("negative x accepted")
	}
	// Case-insensitive.
	if spec, _ := ValidateClick(0, 0, "RIGHT", true); spec.Button != "right" || !spec.Double {
		t.Error("button normalization failed")
	}
}

func TestValidateScroll(t *testing.T) {
	spec, err := ValidateScroll(5, 6, "", 0)
	if err != nil {
		t.Fatalf("ValidateScroll: %v", err)
	}
	if spec.Direction != "down" || spec.Amount != 3 {
		t.Fatalf("defaults not applied: %+v", spec)
	}

	if spec, _ := ValidateScroll(0, 0, "UP", 99); spec.Direction != "up" || spec.Amount != 50 {
		t.Fatalf("normalize/clamp failed: %+v", spec)
	}
	if spec, _ := ValidateScroll(0, 0, "down", -7); spec.Amount != 7 {
		t.Fatalf("negative amount not abs(): %+v", spec)
	}
	if _, err := ValidateScroll(0, 0, "sideways", 1); err == nil {
		t.Error("invalid direction accepted")
	}
	if _, err := ValidateScroll(-3, 0, "down", 1); err == nil {
		t.Error("negative x accepted")
	}
}

func TestValidateRegion(t *testing.T) {
	region, err := ValidateRegion(0, 0, 0, 0)
	if err != nil || region.W != 0 {
		t.Fatalf("full-screen region: %+v err=%v", region, err)
	}
	if region, err = ValidateRegion(10, 20, 300, 200); err != nil || region.W != 300 {
		t.Fatalf("region: %+v err=%v", region, err)
	}
	for _, bad := range [][4]int{{0, 0, -5, 100}, {0, 0, 100, 0}, {-1, 0, 100, 100}, {0, 0, 100, -1}} {
		if _, err := ValidateRegion(bad[0], bad[1], bad[2], bad[3]); err == nil {
			t.Errorf("invalid region %v accepted", bad)
		}
	}
}

func TestParseKeyChord(t *testing.T) {
	mods, base, err := ParseKeyChord("cmd+shift+t")
	if err != nil {
		t.Fatalf("ParseKeyChord: %v", err)
	}
	if len(mods) != 2 || mods[0] != "cmd" || mods[1] != "shift" || base != "t" {
		t.Fatalf("mods=%v base=%q", mods, base)
	}

	// Aliases normalize.
	if _, base, _ = ParseKeyChord("Control+ALT+Delete"); base != "delete" {
		t.Fatalf("alias parsing failed: base=%q", base)
	}

	// Single key, no modifiers.
	mods, base, err = ParseKeyChord("Return")
	if err != nil || len(mods) != 0 || base != "return" {
		t.Fatalf("plain key: mods=%v base=%q err=%v", mods, base, err)
	}

	for _, bad := range []string{"", "  ", "cmd+", "meta+ctrl+bogus+*"} {
		if _, _, err := ParseKeyChord(bad); err == nil {
			t.Errorf("invalid chord %q accepted", bad)
		}
	}
}

func TestKeyCodeFor(t *testing.T) {
	for _, name := range []string{"a", "return", "tab", "space", "escape", "up", "f5", "1", "-"} {
		if _, ok := keyCodeFor(name); !ok {
			t.Errorf("known key %q missing from table", name)
		}
	}
	if vk, _ := keyCodeFor("return"); vk != 0x24 {
		t.Errorf("return keycode = %#x, want 0x24", vk)
	}
	if _, ok := keyCodeFor("notakey"); ok {
		t.Error("unknown key found in table")
	}
}

func TestSplitKeepNewlines(t *testing.T) {
	// NOTE: never call TypeText/Click/Key in tests — with Accessibility
	// granted to the test binary they would act on the real desktop.
	lines := splitKeepNewlines("a\n\nb")
	if len(lines) != 3 || lines[0].text != "a" || !lines[0].newline || lines[1].text != "" || !lines[1].newline || lines[2].text != "b" || lines[2].newline {
		t.Fatalf("splitKeepNewlines: %+v", lines)
	}
	last := splitKeepNewlines("abc")
	if len(last) != 1 || last[0].newline {
		t.Fatalf("trailing text split wrong: %+v", last)
	}
}

func TestPngSizeAndUnder(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	dir, err := sandbox.Sub(home, sandbox.DirTmp)
	if err != nil {
		t.Fatalf("Sub: %v", err)
	}
	// Real 3x2 PNG generated in-process.
	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 3, 2))
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, "shot.png")
	if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
		t.Fatal(err)
	}
	w, h, err := pngSize(p)
	if err != nil || w != 3 || h != 2 {
		t.Fatalf("pngSize = %d,%d err=%v", w, h, err)
	}
	if _, _, err := pngSize(filepath.Join(dir, "missing.png")); err == nil {
		t.Error("pngSize succeeded on missing file")
	}
}

func TestSettingsShape(t *testing.T) {
	m := NewManager()
	info := m.Settings()
	// Shape only — permission values are machine-dependent.
	if info.ScreenW < 0 || info.ScreenH < 0 {
		t.Fatalf("negative screen size: %+v", info)
	}
	if info.ScreenW > 0 && !info.Supported {
		t.Fatal("Supported=false despite a known display size")
	}
}
