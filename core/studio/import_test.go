package studio

import (
	"os"
	"path/filepath"
	"testing"
)

func TestImportAgentArtifactMarkdownDeck(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)
	deck := filepath.Join(dir, "simple_slides.md")
	src := "# Title Slide\n- point one\n- point two\n---\n# Second Slide\n- alpha\n"
	if err := os.WriteFile(deck, []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	imported, err := m.ImportAgentArtifact(deck)
	if err != nil || !imported {
		t.Fatalf("import failed: imported=%v err=%v", imported, err)
	}

	proj := m.ActiveProject()
	ver := proj.ActiveVersion()
	if ver == nil || ver.Prompt != "Agent: simple_slides.md" {
		t.Fatalf("unexpected version prompt %+v", ver)
	}
	if len(ver.Data.Slides) != 2 {
		t.Fatalf("expected 2 slides, got %d", len(ver.Data.Slides))
	}
	if ver.Data.Slides[0].Title != "Title Slide" {
		t.Fatalf("slide title = %q", ver.Data.Slides[0].Title)
	}
	if ver.Data.RawMarkdown != src {
		t.Fatal("expected raw markdown preserved")
	}
}

func TestImportAgentArtifactHTMLPage(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)
	page := filepath.Join(dir, "deck.html")
	if err := os.WriteFile(page, []byte("<html><body><h1>Deck</h1></body></html>"), 0o644); err != nil {
		t.Fatal(err)
	}

	imported, err := m.ImportAgentArtifact(page)
	if err != nil || !imported {
		t.Fatalf("import failed: imported=%v err=%v", imported, err)
	}
	if got := m.ActiveProject().ActiveVersion().Data.EntryHTML; got != page {
		t.Fatalf("EntryHTML = %q, want %q", got, page)
	}
}

func TestImportAgentArtifactPDFAndPPTX(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)

	pdf := filepath.Join(dir, "presentation.pdf")
	if err := os.WriteFile(pdf, []byte("%PDF-1.4 test"), 0o644); err != nil {
		t.Fatal(err)
	}
	if imported, err := m.ImportAgentArtifact(pdf); !imported || err != nil {
		t.Fatalf("expected PDF imported, got imported=%v err=%v", imported, err)
	}

	pptx := filepath.Join(dir, "slides.pptx")
	if err := os.WriteFile(pptx, []byte("PK test pptx"), 0o644); err != nil {
		t.Fatal(err)
	}
	if imported, err := m.ImportAgentArtifact(pptx); !imported || err != nil {
		t.Fatalf("expected PPTX imported, got imported=%v err=%v", imported, err)
	}
}

func TestImportAgentArtifactSceneJS(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)
	// Switch to the 3D project (seeded).
	for _, p := range m.Projects {
		if p.Type == ArtifactTypeCodeJS {
			m.SelectProject(p.ID)
			break
		}
	}
	scene := filepath.Join(dir, "scene.js")
	if err := os.WriteFile(scene, []byte("import * as THREE from 'three';"), 0o644); err != nil {
		t.Fatal(err)
	}

	imported, err := m.ImportAgentArtifact(scene)
	if err != nil || !imported {
		t.Fatalf("import failed: imported=%v err=%v", imported, err)
	}
	data := m.ActiveProject().ActiveVersion().Data
	if data.JSEngine != "threejs" || data.Code == "" {
		t.Fatalf("expected 3D code imported, got engine=%q code=%d bytes", data.JSEngine, len(data.Code))
	}
}

func TestImportAgentArtifactRejectsMismatch(t *testing.T) {
	dir := t.TempDir()
	m := NewManager(dir)
	txt := filepath.Join(dir, "notes.txt")
	if err := os.WriteFile(txt, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}
	if imported, err := m.ImportAgentArtifact(txt); imported || err == nil {
		t.Fatalf("expected rejection of .txt, got imported=%v err=%v", imported, err)
	}
}
