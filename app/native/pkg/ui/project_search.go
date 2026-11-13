package ui

// project_search.go ports ProjectSearchModal: the ⇧⌘F centered search-and-
// replace panel with regex/case/whole-word toggles, include globs, per-file
// grouped results, click navigation, and Replace All — sharing the
// editor_search.go machinery (ContentHits + runEditorSearch).

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Haslab-dev/RakitKode/search"
	"github.com/egoist/mygo/ui"
	"github.com/hasdev/forge-ade/app/native/pkg/icons"
)

// openProjectSearch opens the modal, prefilling the query from the active
// editor selection (≤100 chars) as forge-ade's Mod-Shift-f does.
func (a *AppUI) openProjectSearch() {
	a.ShowProjectSearch = true
	if a.FileSearchQuery == "" {
		if doc := a.Editor.ActiveDoc(); doc != nil {
			sel := doc.Editor.Selection()
			start, end := sel.Anchor, sel.Head
			if start > end {
				start, end = end, start
			}
			if end > start && end-start <= 100 {
				text := doc.Editor.Buffer.String()
				if end <= len(text) {
					a.FileSearchQuery = strings.TrimSpace(text[start:end])
				}
			}
		}
	}
	if a.FileSearchQuery != "" {
		a.FileSearchMode = "content"
		a.ContentHitsAt = "" // force a fresh search
		a.runEditorSearch()
	}
	if a.Window != nil {
		a.Window.Invalidate()
	}
}

// renderProjectSearchModal draws the Zed-style centered search modal.
func (a *AppUI) renderProjectSearchModal(c *ui.Context) {
	if !a.ShowProjectSearch {
		return
	}
	tc := a.ThemeMgr.Colors()

	ui.Modal(c, &a.ShowProjectSearch, func() {
		panel := ui.Column(c).Width(720).MaxHeight(560).Radius(16).Background(tc.SurfaceOverlay).
			BorderWidth(1).BorderColor(tc.Border).Padding(14).Gap(10).Clip().
			Shadow(0, 24, 72, -20, ui.RGBA(0, 0, 0, 0.55))
		panel.Children(func() {
			// Header
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, icons.TablerSearch).Width(15).Height(15).TextColor(tc.Accent)
				ui.Text(c, "Search in Project").FontSize(14).FontWeight(600)
				ui.Text(c, "⇧⌘F").FontSize(10).TextColor(tc.SecondaryLabel)
				ui.Box(c).Grow(1)
				if IconButton(c, tc, icons.TablerX, 13).Clicked() {
					a.ShowProjectSearch = false
				}
			})

			// Query row + option toggles.
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				qbox := ui.Row(c).Height(30).Radius(8).Padding(0, 10).Gap(6).AlignItems(ui.Center).
					Background(tc.Surface).BorderWidth(1).BorderColor(tc.Focus.Alpha(0.4)).Grow(1)
				qbox.Children(func() {
					ui.Icon(c, icons.TablerSearch).Width(13).Height(13).TextColor(tc.IconMuted)
					in := ui.TextInput(c, &a.FileSearchQuery).Placeholder("Search content… (⏎)").Grow(1)
					if in.Changed() || in.Submitted() {
						a.runEditorSearch()
					}
				})
				a.renderSearchToggle(c, tc, "Aa", "Match case", &a.FileSearchCase)
				a.renderSearchToggle(c, tc, ".*", "Regular expression", &a.FileSearchRegex)
				a.renderSearchToggle(c, tc, "ab", "Match whole word", &a.FileSearchWholeWord)
			})

			// Include glob + replace row.
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "include").FontSize(11).TextColor(tc.TextMuted)
				globIn := ui.TextInput(c, &a.FileSearchGlob).Placeholder("*.go, src/**").Grow(1).FontSize(11).Height(24)
				if globIn.Changed() || globIn.Submitted() {
					a.runEditorSearch()
				}
				repIn := ui.TextInput(c, &a.FileSearchReplace).Placeholder("replace with…").Grow(1).FontSize(11).Height(24)
				_ = repIn
				repBtn := ui.Row(c.Key("ps-replace-all")).Height(24).Radius(6).Padding(0, 10).AlignItems(ui.Center).
					Background(tc.Warning.Alpha(0.14)).BorderWidth(1).BorderColor(tc.Warning.Alpha(0.4))
				repBtn.Children(func() {
					ui.Text(c, "Replace All").FontSize(11).FontWeight(600).TextColor(tc.Warning)
				})
				if repBtn.Clicked() && !a.FileSearchBusy && strings.TrimSpace(a.FileSearchQuery) != "" {
					a.runEditorReplaceAll()
				}
			})

			// Status line.
			status := ui.Row(c).Gap(8).AlignItems(ui.Center)
			status.Children(func() {
				if a.FileSearchBusy {
					ui.Spinner(c).Size(12, 12).Label("Searching")
				}
				if len(a.ContentHits) > 0 {
					ui.Text(c, fmt.Sprintf("%d matches", len(a.ContentHits))).FontSize(11).TextColor(tc.TextMuted)
				} else if strings.TrimSpace(a.FileSearchQuery) != "" && !a.FileSearchBusy {
					ui.Text(c, "No matches").FontSize(11).TextColor(tc.TextMuted)
				}
				if a.SearchStatusMsg != "" {
					ui.Text(c, a.SearchStatusMsg).FontSize(11).TextColor(tc.Success)
				}
				ui.Box(c).Grow(1)
				ui.Text(c, "esc close").FontSize(10).TextColor(tc.SecondaryLabel)
			})

			// Results grouped by file.
			results := ui.Scroll(c)
			results.Children(func() {
				col := ui.Column(c).Gap(2).Padding(2, 2)
				col.Children(func() {
					grouped := map[string][]search.RankedResult{}
					var paths []string
					for _, h := range a.ContentHits {
						if _, ok := grouped[h.Path]; !ok {
							paths = append(paths, h.Path)
						}
						grouped[h.Path] = append(grouped[h.Path], h)
					}
					sort.Strings(paths)
					for _, p := range paths {
						p := p
						hits := grouped[p]
						ui.Row(c.Key("ps-h-"+p)).Height(26).Radius(6).Padding(0, 6).Gap(6).
							AlignItems(ui.Center).Children(func() {
							ui.Icon(c, icons.FileIconSVG(p)).Width(13).Height(13).TextColor(tc.IconMuted)
							ui.Text(c, filepath.Base(p)).FontSize(12).FontWeight(600).Grow(1).SingleLine()
							ui.Text(c, filepath.Dir(p)).FontSize(10).TextColor(tc.TextMuted).SingleLine()
							ui.Text(c, fmt.Sprintf("%d", len(hits))).FontSize(10).TextColor(tc.TextMuted)
						})
						for j, h := range hits {
							j, h := j, h
							row := ui.Row(c.Key(fmt.Sprintf("ps-%s-%d", p, j))).Height(24).Radius(4).
								Padding(0, 6, 0, 24).Gap(6).AlignItems(ui.Center)
							row.Children(func() {
								ui.Text(c, fmt.Sprintf("%d", h.Line)).FontSize(10).TextColor(tc.TextMuted)
								line := h.Content
								if len(line) > 84 {
									line = line[:84] + "…"
								}
								ui.Text(c, line).FontSize(11).Grow(1).SingleLine()
							})
							if row.Hovered() {
								row.Background(tc.Text.Alpha(0.04))
							}
							if row.Clicked() {
								a.ShowProjectSearch = false
								a.openSearchHitAtLine(p, h.Line, a.FileSearchQuery)
								a.Mode = ModeEditor
								if a.Window != nil {
									a.Window.Invalidate()
								}
							}
						}
					}
				})
			}).Grow(1).MaxHeight(360)
		})
	})
}
