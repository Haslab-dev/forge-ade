package studio

import (
	"archive/zip"
	"path/filepath"
	"strings"
	"testing"
)

func TestExportSlidesToPPTX(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "deck.pptx")

	slides := []SlideItem{
		{Title: "ForgeADE Studio", Subtitle: "Unified Creative Workspace", Bullets: []string{"HTML & Web", "Slides to PPTX", "Motion to MP4"}},
		{Title: "Architecture", Bullets: []string{"Pure-Go Native Desktop", "Zero Electron Overhead"}},
	}

	if err := ExportSlidesToPPTX("Test Deck", slides, "", dest); err != nil {
		t.Fatalf("ExportSlidesToPPTX failed: %v", err)
	}

	zr, err := zip.OpenReader(dest)
	if err != nil {
		t.Fatalf("failed to open pptx zip: %v", err)
	}
	defer zr.Close()

	found := map[string]bool{}
	for _, f := range zr.File {
		found[f.Name] = true
	}
	for _, req := range []string{
		"[Content_Types].xml",
		"_rels/.rels",
		"ppt/presentation.xml",
		"ppt/slides/slide1.xml",
		"ppt/slides/slide2.xml",
	} {
		if !found[req] {
			t.Fatalf("expected %s inside generated pptx", req)
		}
	}
}

func TestExtractSlidesFromHTML(t *testing.T) {
	html := `<html><body>
		<section><h1>Intro Slide</h1><p>Welcome to ForgeADE</p><li>Point One</li></section>
		<section><h2>Second Slide</h2><ul><li>Alpha</li><li>Beta</li></ul></section>
	</body></html>`
	slides := extractSlidesFromHTML(html, "Fallback")
	if len(slides) != 2 {
		t.Fatalf("expected 2 slides, got %d", len(slides))
	}
	if slides[0].Title != "Intro Slide" || !strings.Contains(slides[0].Bullets[0], "Welcome") {
		t.Fatalf("unexpected slide 0: %+v", slides[0])
	}
	if slides[1].Title != "Second Slide" || len(slides[1].Bullets) != 2 {
		t.Fatalf("unexpected slide 1: %+v", slides[1])
	}
}
