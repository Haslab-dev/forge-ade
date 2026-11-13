package ui

// editor_search.go upgrades the editor Search activity to the webview's
// ProjectSearchModal parity: filename and content modes with regex /
// case-sensitivity / whole-word toggles, include-glob filter, and a
// replace-all row — powered by the shared RakitKode search manager.

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/Haslab-dev/RakitKode/search"
	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/app/native/pkg/icons"
	"github.com/hasdev/forge-ade/app/native/pkg/theme"
)

// renderFileSearch draws the Search side pane of the editor activity bar.
func (a *AppUI) renderFileSearch(c *ui.Context) {
	tc := a.ThemeMgr.Colors()

	ui.Row(c).Height(35).Padding(0, 12).Gap(6).AlignItems(ui.Center).
		BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
		ui.Text(c, "SEARCH").FontSize(12).FontWeight(600).LetterSpacing(0.4)
		ui.Box(c).Grow(1)
		count := len(a.FileSearchHits)
		if a.FileSearchMode == "content" {
			count = 0
			for _, r := range a.ContentHits {
				count++
				_ = r
			}
		}
		if count > 0 {
			ui.Text(c, fmt.Sprintf("%d results", count)).FontSize(10).TextColor(tc.TextMuted)
		}
		if a.FileSearchBusy {
			ui.Spinner(c).Size(12, 12).Label("Searching")
		}
	})

	// Mode segmented control: Files | Content. Empty/legacy state acts as
	// Files so the pane always has an active mode.
	if a.FileSearchMode == "" {
		a.FileSearchMode = "files"
	}
	ui.Row(c).Padding(6, 10, 2, 10).Gap(4).AlignItems(ui.Center).Children(func() {
		for _, m := range []struct{ id, label string }{{"files", "Files"}, {"content", "Content"}} {
			m := m
			active := a.FileSearchMode == m.id
			b := ui.Row(c.Key("fsmode-"+m.id)).Height(22).Radius(6).Padding(0, 10).AlignItems(ui.Center).Justify(ui.Center)
			b.Children(func() {
				ui.Text(c, m.label).FontSize(11)
			})
			if active {
				b.Background(tc.Accent.Alpha(0.14)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.3))
			} else if b.Hovered() {
				b.Background(tc.Text.Alpha(0.05))
			}
			if b.Clicked() {
				a.FileSearchMode = m.id
				a.ContentHitsAt = "" // re-run under the new mode
				a.runEditorSearch()
			}
		}
	})

	// Query row.
	ui.Row(c).Padding(4, 10).Gap(6).AlignItems(ui.Center).Children(func() {
		ui.Icon(c, icons.TablerSearch).Width(13).Height(13).TextColor(tc.IconMuted)
		in := ui.TextInput(c, &a.FileSearchQuery).Placeholder("Search in files... (⏎)").Grow(1)
		if in.Changed() || in.Submitted() {
			a.runEditorSearch()
		}
	})

	// Options row (content mode): Aa | .* | [w] toggles + include glob.
	if a.FileSearchMode == "content" {
		ui.Row(c).Padding(2, 10).Gap(4).AlignItems(ui.Center).Children(func() {
			a.renderSearchToggle(c, tc, "Aa", "Match case", &a.FileSearchCase)
			a.renderSearchToggle(c, tc, ".*", "Regular expression", &a.FileSearchRegex)
			a.renderSearchToggle(c, tc, "ab", "Match whole word", &a.FileSearchWholeWord)
			ui.Box(c).Grow(1)
			globIn := ui.TextInput(c, &a.FileSearchGlob).Placeholder("*.go, src/**").Grow(1).
				FontSize(11).Height(22)
			if globIn.Changed() || globIn.Submitted() {
				a.runEditorSearch()
			}
		})

		// Replace row.
		chev := icons.TablerChevronRight
		if a.FileSearchReplaceOpen {
			chev = icons.TablerChevronDown
		}
		ui.Row(c).Padding(2, 10).Gap(4).AlignItems(ui.Center).Children(func() {
			chevBtn := IconButton(c, tc, chev, 12)
			if chevBtn.Clicked() {
				a.FileSearchReplaceOpen = !a.FileSearchReplaceOpen
			}
			if a.FileSearchReplaceOpen {
				repIn := ui.TextInput(c, &a.FileSearchReplace).Placeholder("Replace").Grow(1).FontSize(11).Height(22)
				_ = repIn
				repBtn := ui.Row(c.Key("replace-all")).Height(22).Radius(6).Padding(0, 8).AlignItems(ui.Center).
					Background(tc.Warning.Alpha(0.14)).BorderWidth(1).BorderColor(tc.Warning.Alpha(0.4))
				repBtn.Children(func() {
					ui.Text(c, "Replace All").FontSize(10).FontWeight(600).TextColor(tc.Warning)
				})
				if repBtn.Clicked() && !a.FileSearchBusy && strings.TrimSpace(a.FileSearchQuery) != "" {
					a.runEditorReplaceAll()
				}
			}
		})
	}

	// Content mode results: grouped by file (RankedResult has Line+Content).
	if a.FileSearchMode == "content" {
		a.renderContentSearchResults(c, tc)
		return
	}

	if len(a.FileSearchHits) == 0 {
		ui.Column(c).Grow(1).Center().Gap(6).Padding(20).Children(func() {
			ui.Icon(c, icons.TablerSearch).Width(28).Height(28).TextColor(tc.IconMuted)
			q := strings.TrimSpace(a.FileSearchQuery)
			if q != "" && len(q) >= 2 {
				ui.Text(c, "No results for \""+q+"\"").FontSize(12).TextColor(tc.TextMuted).MaxWidth(200)
			} else {
				ui.Text(c, "Type at least 2 characters").FontSize(12).TextColor(tc.TextMuted)
				ui.Text(c, "Searches across all workspace files").FontSize(11).TextColor(tc.TextMuted)
			}
		})
		return
	}

	// Filename results grouped by file.
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).Padding(2, 6).Gap(2).Children(func() {
			lastPath := ""
			for i, hit := range a.FileSearchHits {
				i, hit := i, hit
				if hit.Path != lastPath {
					lastPath = hit.Path
					ui.Row(c.Key("fsh-h-"+hit.Path)).Height(24).Radius(4).Padding(0, 6).Gap(6).
						AlignItems(ui.Center).Children(func() {
						ui.Icon(c, icons.FileIconSVG(hit.Path)).Width(12).Height(12).TextColor(tc.IconMuted)
						ui.Text(c, filepath.Base(hit.Path)).FontSize(11).FontWeight(600).Grow(1).SingleLine()
						ui.Text(c, filepath.Dir(hit.Path)).FontSize(10).TextColor(tc.TextMuted).SingleLine()
					})
				}
				row := ui.Row(c.Key(fmt.Sprintf("fsh-%d", i))).Height(24).Radius(4).
					Padding(0, 6, 0, 20).Gap(6).AlignItems(ui.Center)
				row.Children(func() {
					ui.Text(c, fmt.Sprintf("%d", hit.Line)).FontSize(10).TextColor(tc.TextMuted)
					line := hit.Text
					if len(line) > 60 {
						line = line[:60] + "…"
					}
					ui.Text(c, line).FontSize(11).Grow(1).SingleLine()
				})
				if a.SelectedFile == hit.Path {
					row.Background(tc.Accent.Alpha(0.08))
				} else if row.Hovered() {
					row.Background(tc.Text.Alpha(0.04))
				}
				if row.Clicked() {
					full := a.Workspace.ResolveFullPath(hit.Path)
					if doc, err := a.Editor.OpenFile(full, hit.Path); err == nil && doc != nil {
						if hit.ByteStart >= 0 && hit.ByteStart < doc.Editor.Buffer.Len() {
							doc.Editor.Select(hit.ByteStart, hit.ByteStart+len(strings.ToLower(a.FileSearchQuery)))
							doc.Editor.GoTo(doc.Editor.CursorPos())
						}
					}
					a.SelectedFile = hit.Path
				}
			}
		})
	})
}

// renderSearchToggle draws one Aa / .* / ab option toggle.
func (a *AppUI) renderSearchToggle(c *ui.Context, tc theme.ThemeColors, label, tip string, flag *bool) {
	b := ui.Row(c.Key("stog-"+label+tip)).Size(22, 22).Radius(5).AlignItems(ui.Center).Justify(ui.Center)
	b.Children(func() {
		ui.Text(c, label).FontSize(10).FontWeight(600)
	})
	if *flag {
		b.Background(tc.Accent.Alpha(0.16)).BorderWidth(1).BorderColor(tc.Accent.Alpha(0.4))
	} else {
		b.Background(tc.Surface).BorderWidth(1).BorderColor(tc.Border)
	}
	b.Tooltip(tip)
	if b.Clicked() {
		*flag = !*flag
		a.runEditorSearch()
	}
}

// runEditorSearch executes the active mode's search off the render path.
func (a *AppUI) runEditorSearch() {
	query := strings.TrimSpace(a.FileSearchQuery)
	if a.FileSearchMode != "content" || len(query) < 2 || a.Engine == nil || a.Engine.SearchMgr == nil {
		if len(query) < 2 {
			a.ContentHits = nil
		}
		return
	}
	if a.Workspace != nil {
		a.Engine.SearchMgr.SetDirectories([]string{a.Workspace.RootDir})
	}
	signature := fmt.Sprintf("%s|%v|%v|%v|%s", query, a.FileSearchCase, a.FileSearchRegex, a.FileSearchWholeWord, a.FileSearchGlob)
	if signature == a.ContentHitsAt {
		return
	}
	a.ContentHitsAt = signature
	a.FileSearchBusy = true
	if a.Window != nil {
		a.Window.Invalidate()
	}
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		_ = ctx
		opts := search.SearchOptions{
			Query:          query,
			MatchCase:      a.FileSearchCase,
			UseRegex:       a.FileSearchRegex,
			MatchWholeWord: a.FileSearchWholeWord,
			Glob:           strings.TrimSpace(a.FileSearchGlob),
			Limit:          200,
		}
		hits, err := a.Engine.SearchMgr.SearchContentWithOptions(opts)
		a.FileSearchBusy = false
		if err == nil {
			a.ContentHits = hits
		}
		if a.Window != nil {
			a.Window.Invalidate()
		}
	}()
}

// runEditorReplaceAll applies replace-all across the workspace.
func (a *AppUI) runEditorReplaceAll() {
	if a.Engine == nil || a.Engine.SearchMgr == nil || a.Workspace == nil {
		return
	}
	a.FileSearchBusy = true
	if a.Window != nil {
		a.Window.Invalidate()
	}
	go func() {
		opts := search.ReplaceOptions{
			SearchOptions: search.SearchOptions{
				Query:          a.FileSearchQuery,
				MatchCase:      a.FileSearchCase,
				UseRegex:       a.FileSearchRegex,
				MatchWholeWord: a.FileSearchWholeWord,
				Glob:           strings.TrimSpace(a.FileSearchGlob),
			},
			Replacement: a.FileSearchReplace,
		}
		res, err := a.Engine.SearchMgr.ReplaceAll(opts)
		a.FileSearchBusy = false
		if a.Window != nil {
			msg := fmt.Sprintf("Replaced %d occurrences in %d files", res.TotalReplacements, res.FilesChanged)
			if err != nil {
				msg = err.Error()
			}
			// Toasts must be added on the main thread via Invalidate; carry
			// the message through the status surface.
			a.ContentHitsAt = ""
			a.SearchStatusMsg = msg
			a.Window.Invalidate()
		}
	}()
}

// renderContentSearchResults draws content-mode hits grouped by file.
func (a *AppUI) renderContentSearchResults(c *ui.Context, tc theme.ThemeColors) {
	if len(a.ContentHits) == 0 {
		ui.Column(c).Grow(1).Center().Gap(6).Padding(20).Children(func() {
			ui.Icon(c, icons.TablerSearch).Width(28).Height(28).TextColor(tc.IconMuted)
			q := strings.TrimSpace(a.FileSearchQuery)
			if q != "" && len(q) >= 2 {
				ui.Text(c, "No results for \""+q+"\"").FontSize(12).TextColor(tc.TextMuted).MaxWidth(200)
			} else {
				ui.Text(c, "Search file contents").FontSize(12).TextColor(tc.TextMuted)
				ui.Text(c, "ripgrep-backed, regex and glob aware").FontSize(11).TextColor(tc.TextMuted)
			}
		})
		return
	}

	// Group by path, keep order by score.
	grouped := map[string][]search.RankedResult{}
	var paths []string
	for _, h := range a.ContentHits {
		if _, ok := grouped[h.Path]; !ok {
			paths = append(paths, h.Path)
		}
		grouped[h.Path] = append(grouped[h.Path], h)
	}
	sort.Strings(paths)

	if a.SearchStatusMsg != "" {
		ui.Row(c).Padding(4, 10).Children(func() {
			ui.Text(c, a.SearchStatusMsg).FontSize(10).TextColor(tc.Success)
		})
	}

	ui.Scroll(c).Grow(1).Children(func() {
		col := ui.Column(c).Padding(2, 6).Gap(2)
		col.Children(func() {
			for _, p := range paths {
				p := p
				hits := grouped[p]
				ui.Row(c.Key("cs-h-"+p)).Height(24).Radius(4).Padding(0, 6).Gap(6).
					AlignItems(ui.Center).Children(func() {
					ui.Icon(c, icons.FileIconSVG(p)).Width(12).Height(12).TextColor(tc.IconMuted)
					ui.Text(c, filepath.Base(p)).FontSize(11).FontWeight(600).Grow(1).SingleLine()
					ui.Text(c, fmt.Sprintf("%d", len(hits))).FontSize(10).TextColor(tc.TextMuted)
				})
				for j, h := range hits {
					j, h := j, h
					row := ui.Row(c.Key(fmt.Sprintf("cs-%s-%d", p, j))).Height(24).Radius(4).
						Padding(0, 6, 0, 20).Gap(6).AlignItems(ui.Center)
					row.Children(func() {
						ui.Text(c, fmt.Sprintf("%d", h.Line)).FontSize(10).TextColor(tc.TextMuted)
						line := h.Content
						if len(line) > 60 {
							line = line[:60] + "…"
						}
						ui.Text(c, line).FontSize(11).Grow(1).SingleLine()
					})
					if row.Hovered() {
						row.Background(tc.Text.Alpha(0.04))
					}
					if row.Clicked() {
						a.openSearchHitAtLine(p, h.Line, a.FileSearchQuery)
					}
				}
			}
		})
	}).Grow(1)
}

// openSearchHitAtLine opens a file and jumps to the given line (1-based).
func (a *AppUI) openSearchHitAtLine(relPath string, line int, query string) {
	full := a.Workspace.ResolveFullPath(relPath)
	doc, err := a.Editor.OpenFile(full, relPath)
	if err != nil || doc == nil {
		return
	}
	buf := doc.Editor.Buffer
	text := buf.String()
	offset := 0
	if line > 1 {
		lines := strings.Split(text, "\n")
		if line-1 < len(lines) {
			for i := 0; i < line-1; i++ {
				offset += len(lines[i]) + 1
			}
		}
	}
	length := len(query)
	if length <= 0 {
		length = 0
	}
	if length > 0 && offset+length <= buf.Len() {
		doc.Editor.Select(offset, offset+length)
		doc.Editor.GoTo(doc.Editor.CursorPos())
	}
	a.SelectedFile = relPath
}
