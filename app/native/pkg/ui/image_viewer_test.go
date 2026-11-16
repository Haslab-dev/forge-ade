package ui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestIsImageOrSVG(t *testing.T) {
	valid := []string{
		"logo.png", "photo.JPG", "banner.jpeg", "anim.gif",
		"asset.webp", "icon.svg", "favicon.ico", "graphic.bmp",
		"nested/path/to/image.PNG", "dir/sub/vector.SVG",
	}
	for _, v := range valid {
		if !isImageOrSVG(v) {
			t.Errorf("isImageOrSVG(%q) = false; want true", v)
		}
	}

	invalid := []string{
		"main.go", "app.ts", "style.css", "readme.md",
		"package.json", "script.sh", "data.sqlite",
	}
	for _, inv := range invalid {
		if isImageOrSVG(inv) {
			t.Errorf("isImageOrSVG(%q) = true; want false", inv)
		}
	}
}

func TestIsSVG(t *testing.T) {
	if !isSVG("icon.svg") || !isSVG("LOGO.SVG") {
		t.Errorf("isSVG failed for .svg files")
	}
	if isSVG("icon.png") || isSVG("svg_file.go") {
		t.Errorf("isSVG returned true for non-svg files")
	}
}

func TestFormatByteSize(t *testing.T) {
	tests := []struct {
		in   int64
		want string
	}{
		{500, "500 B"},
		{1024, "1.0 KB"},
		{2048, "2.0 KB"},
		{1024 * 1024, "1.00 MB"},
		{5 * 1024 * 1024, "5.00 MB"},
	}
	for _, tt := range tests {
		got := formatByteSize(tt.in)
		if got != tt.want {
			t.Errorf("formatByteSize(%d) = %q; want %q", tt.in, got, tt.want)
		}
	}
}

func TestDecodeImageFileSVG(t *testing.T) {
	tmpDir := t.TempDir()
	svgPath := filepath.Join(tmpDir, "test.svg")
	svgContent := `<svg xmlns="http://www.w3.org/2000/svg" width="100" height="100" viewBox="0 0 100 100"><circle cx="50" cy="50" r="40" fill="red"/></svg>`
	if err := os.WriteFile(svgPath, []byte(svgContent), 0o644); err != nil {
		t.Fatalf("failed to write test svg: %v", err)
	}

	ci := decodeImageFile(svgPath)
	if ci == nil {
		t.Fatalf("decodeImageFile returned nil")
	}
	if ci.err != nil {
		t.Fatalf("decodeImageFile error: %v", ci.err)
	}
	if ci.format != "SVG" {
		t.Errorf("ci.format = %q; want SVG", ci.format)
	}
	if ci.width != 100 || ci.height != 100 {
		t.Errorf("ci.width, ci.height = (%d, %d); want (100, 100)", ci.width, ci.height)
	}
}

// getImage must not decode synchronously on the render path: a cache miss
// returns a loading placeholder and kicks exactly one bounded background
// decode.
func TestGetImageAsyncPlaceholder(t *testing.T) {
	tmpDir := t.TempDir()
	svgPath := filepath.Join(tmpDir, "test.svg")
	svgContent := `<svg xmlns="http://www.w3.org/2000/svg" width="64" height="64" viewBox="0 0 64 64"><rect width="64" height="64" fill="blue"/></svg>`
	if err := os.WriteFile(svgPath, []byte(svgContent), 0o644); err != nil {
		t.Fatalf("failed to write test svg: %v", err)
	}

	app := &AppUI{
		imageCache: make(map[string]*cachedImage),
	}
	ci := app.getImage(svgPath)
	if ci == nil {
		t.Fatalf("getImage returned nil")
	}
	if !ci.loading {
		t.Fatalf("getImage on cache miss should return a loading placeholder, got loading=false")
	}
	if ci.src != nil || ci.err != nil {
		t.Fatalf("placeholder should carry no image/error, got src=%v err=%v", ci.src, ci.err)
	}

	// The decode failure path is cached too: a missing file reports its stat
	// error without kicking a decode or blocking.
	ci = app.getImage(filepath.Join(tmpDir, "does-not-exist.svg"))
	if ci == nil || ci.err == nil || ci.loading {
		t.Fatalf("missing file should return a stat-error entry, got %+v", ci)
	}
}
