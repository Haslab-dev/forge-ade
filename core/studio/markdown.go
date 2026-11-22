package studio

import (
	"fmt"
	"strings"
)

// ParseMarkdownDeck converts an agent-authored markdown deck into slides.
//
// Slides split on '---' horizontal rules when the document uses them,
// otherwise on each top-level '# ' heading. Within a slide block the first
// heading becomes the title, the next heading (or first plain paragraph
// line, when no title was found) becomes the subtitle, and list items —
// bulleted or numbered — become the bullets.
func ParseMarkdownDeck(src string) []SlideItem {
	src = strings.ReplaceAll(src, "\r\n", "\n")
	lines := strings.Split(src, "\n")

	var blocks [][]string
	if hasSlideSeparator(lines) {
		blocks = splitOn(lines, "---")
	} else {
		blocks = splitOn(lines, "#")
	}

	var slides []SlideItem
	for _, block := range blocks {
		var title, subtitle, body strings.Builder
		var bullets []string
		plain := 0
		for _, raw := range block {
			line := strings.TrimSpace(raw)
			if line == "" || line == "---" {
				continue
			}
			switch {
			case isHeading(line):
				text := strings.TrimLeft(line, "# ")
				if title.Len() == 0 {
					title.WriteString(text)
				} else if subtitle.Len() == 0 {
					subtitle.WriteString(text)
				}
			case isListItem(line):
				bullets = append(bullets, trimListMarker(line))
			default:
				plain++
				switch {
				case title.Len() == 0:
					title.WriteString(line)
				case subtitle.Len() == 0 && plain == 1 && len(bullets) == 0:
					subtitle.WriteString(line)
				default:
					if body.Len() > 0 {
						body.WriteString(" ")
					}
					body.WriteString(line)
				}
			}
		}
		if title.Len() == 0 && len(bullets) == 0 {
			continue
		}
		slide := SlideItem{
			ID:       fmt.Sprintf("slide-%d", len(slides)+1),
			Index:    len(slides),
			Title:    title.String(),
			Subtitle: subtitle.String(),
			Body:     body.String(),
			Layout:   "split",
			Theme:    "dark",
			Bullets:  bullets,
		}
		if len(slides) == 0 {
			slide.Layout = "hero"
		}
		slides = append(slides, slide)
	}
	return slides
}

func hasSlideSeparator(lines []string) bool {
	count := 0
	for _, l := range lines {
		if strings.TrimSpace(l) == "---" {
			count++
		}
	}
	return count >= 2
}

// splitOn cuts lines into blocks starting at each marker line ('---') or
// heading line ('#'), dropping anything before the first marker.
func splitOn(lines []string, marker string) [][]string {
	var blocks [][]string
	var cur []string
	for _, l := range lines {
		trimmed := strings.TrimSpace(l)
		isSplit := (marker == "---" && trimmed == "---") ||
			(marker == "#" && strings.HasPrefix(trimmed, "#") && !strings.HasPrefix(trimmed, "##"))
		if isSplit {
			if len(cur) > 0 {
				blocks = append(blocks, cur)
			}
			cur = []string{l}
			continue
		}
		cur = append(cur, l)
	}
	if len(cur) > 0 {
		blocks = append(blocks, cur)
	}
	return blocks
}

func isHeading(line string) bool {
	return strings.HasPrefix(line, "#")
}

func isListItem(line string) bool {
	if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") || strings.HasPrefix(line, "+ ") {
		return true
	}
	// Numbered list: "1. ", "12) ".
	dot := strings.IndexAny(line, ".)")
	if dot <= 0 || dot > 4 {
		return false
	}
	for _, r := range line[:dot] {
		if r < '0' || r > '9' {
			return false
		}
	}
	return len(line) > dot && line[dot+1] == ' '
}

func trimListMarker(line string) string {
	if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") || strings.HasPrefix(line, "+ ") {
		return strings.TrimSpace(line[2:])
	}
	if dot := strings.IndexAny(line, ".)"); dot > 0 && dot <= 4 {
		return strings.TrimSpace(line[dot+1:])
	}
	return line
}
