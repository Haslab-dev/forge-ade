package studio

import (
	"testing"
)

func TestParseMarkdownDeckWithSeparators(t *testing.T) {
	src := `# Launch Keynote
## Why we build

- First bullet
- Second bullet

---

# Architecture
## Under the hood

1. Ingest
2. Transform

---

# Outro
`

	slides := ParseMarkdownDeck(src)
	if len(slides) != 3 {
		t.Fatalf("expected 3 slides, got %d", len(slides))
	}
	first := slides[0]
	if first.Title != "Launch Keynote" {
		t.Fatalf("title = %q", first.Title)
	}
	if first.Subtitle != "Why we build" {
		t.Fatalf("subtitle = %q", first.Subtitle)
	}
	if len(first.Bullets) != 2 || first.Bullets[0] != "First bullet" {
		t.Fatalf("bullets = %v", first.Bullets)
	}
	if first.Layout != "hero" {
		t.Fatalf("first slide layout = %q, want hero", first.Layout)
	}
	second := slides[1]
	if second.Title != "Architecture" || second.Subtitle != "Under the hood" {
		t.Fatalf("slide 2 = %q / %q", second.Title, second.Subtitle)
	}
	if len(second.Bullets) != 2 || second.Bullets[1] != "Transform" {
		t.Fatalf("numbered bullets = %v", second.Bullets)
	}
	if slides[2].Title != "Outro" {
		t.Fatalf("slide 3 title = %q", slides[2].Title)
	}
}

func TestParseMarkdownDeckHeadingsOnly(t *testing.T) {
	src := "Intro line\n# One\ntext one\n# Two\n- alpha\n# Three"
	slides := ParseMarkdownDeck(src)
	// The leading intro line becomes its own (title-only) slide.
	if len(slides) != 4 {
		t.Fatalf("expected 4 slides, got %d", len(slides))
	}
	if slides[0].Title != "Intro line" {
		t.Fatalf("preamble title = %q", slides[0].Title)
	}
	if slides[1].Title != "One" || slides[1].Subtitle != "text one" {
		t.Fatalf("slide 1 = %q / %q", slides[1].Title, slides[1].Subtitle)
	}
	if slides[2].Bullets[0] != "alpha" {
		t.Fatalf("bullets = %v", slides[2].Bullets)
	}
}

func TestParseMarkdownDeckIgnoresEmpty(t *testing.T) {
	if slides := ParseMarkdownDeck("#\n---\n---\n   \n"); len(slides) != 0 {
		t.Fatalf("expected no slides, got %d", len(slides))
	}
	if slides := ParseMarkdownDeck(""); len(slides) != 0 {
		t.Fatalf("expected no slides for empty input, got %d", len(slides))
	}
}
