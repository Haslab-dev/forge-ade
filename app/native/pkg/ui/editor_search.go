package ui

// editor_search.go upgrades the editor Search activity to the webview's
// ProjectSearchModal parity: filename and content modes with regex /
// case-sensitivity / whole-word toggles, include-glob filter, and a
// replace-all row — powered by the shared RakitKode search manager.

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/Haslab-dev/RakitKode/gitignore"
	rkignore "github.com/Haslab-dev/RakitKode/ignore"
	"github.com/Haslab-dev/RakitKode/search"
	"github.com/egoist/mygo/ui"
	mygoeditor "github.com/egoist/mygo/ui/editor"
	"github.com/hasdev/forge-ade/app/native/pkg/icons"
	"github.com/hasdev/forge-ade/app/native/pkg/theme"
)

// truncRunes cuts s to at most max runes so multi-byte UTF-8 is never split
// mid-rune (thread_find.go's preview pattern).
func truncRunes(s string, max int) string {
	if max <= 0 {
		return ""
	}
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}

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
					ui.Text(c, truncRunes(hit.Text, 60)).FontSize(11).Grow(1).SingleLine()
				})
				if a.SelectedFile == hit.Path {
					row.Background(tc.Accent.Alpha(0.08))
				} else if row.Hovered() {
					row.Background(tc.Text.Alpha(0.04))
				}
				if row.Clicked() {
					// Files-mode hits carry a line too — jump to it.
					a.openSearchResultChecked(hit.Path, hit.Line, a.FileSearchQuery)
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
// The goroutine collects results locally and applies them on the UI thread —
// render reads FileSearchHits/ContentHits/FileSearchBusy every frame, so
// they are never written from the background goroutine directly.
func (a *AppUI) runEditorSearch() {
	query := strings.TrimSpace(a.FileSearchQuery)
	if a.FileSearchMode != "content" {
		if len(query) < 2 || a.Workspace == nil {
			a.FileSearchHits = nil
			a.ContentHitsAt = ""
			return
		}
		signature := "files|" + a.Workspace.RootDir + "|" + query
		if signature == a.ContentHitsAt {
			return
		}
		a.ContentHitsAt = signature
		a.FileSearchBusy = true
		ws := a.Workspace
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		go func() {
			defer cancel()
			hits := ws.SearchFiles(ctx, query)
			a.runOnUI(func() {
				if a.ContentHitsAt == signature {
					a.FileSearchHits = hits
				}
				a.FileSearchBusy = false
				if a.Window != nil {
					a.Window.Invalidate()
				}
			})
		}()
		return
	}
	if len(query) < 2 || a.Engine == nil || a.Engine.SearchMgr == nil {
		if len(query) < 2 {
			a.ContentHits = nil
			a.ContentHitsAt = ""
		}
		return
	}
	if a.Workspace != nil {
		a.Engine.SearchMgr.SetDirectories([]string{a.Workspace.RootDir})
	}
	signature := fmt.Sprintf("content|%s|%v|%v|%v|%s", query, a.FileSearchCase, a.FileSearchRegex, a.FileSearchWholeWord, a.FileSearchGlob)
	if signature == a.ContentHitsAt {
		return
	}
	a.ContentHitsAt = signature
	a.FileSearchBusy = true
	if a.Window != nil {
		a.Window.Invalidate()
	}
	go func() {
		opts := search.SearchOptions{
			Query:          query,
			MatchCase:      a.FileSearchCase,
			UseRegex:       a.FileSearchRegex,
			MatchWholeWord: a.FileSearchWholeWord,
			Glob:           strings.TrimSpace(a.FileSearchGlob),
			Limit:          200,
		}
		hits, err := a.Engine.SearchMgr.SearchContentWithOptions(opts)
		a.runOnUI(func() {
			a.FileSearchBusy = false
			if err == nil && a.ContentHitsAt == signature {
				a.ContentHits = hits
			}
			if a.Window != nil {
				a.Window.Invalidate()
			}
		})
	}()
}

// runEditorReplaceAll applies replace-all across the workspace, routing
// through the OnConfirmReplace contract (EditorManager field the UI layer
// wires; nil → proceed as before) and keeping open editor buffers as the
// source of truth for their files so a later save cannot revert the replace.
func (a *AppUI) runEditorReplaceAll() {
	if a.Engine == nil || a.Engine.SearchMgr == nil || a.Workspace == nil {
		return
	}
	root := a.Workspace.RootDir
	if root == "" {
		return
	}
	confirm := a.Editor.OnConfirmReplace
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

		// Count matching lines first so the confirmation can quote a number.
		countOpts := opts.SearchOptions
		countOpts.Limit = 100000
		hits, err := a.Engine.SearchMgr.SearchContentWithOptions(countOpts)
		if err != nil {
			a.queueToast(ui.Toast{Title: "Search failed", Description: err.Error(), Type: "error"})
			a.runOnUI(func() {
				a.FileSearchBusy = false
				if a.Window != nil {
					a.Window.Invalidate()
				}
			})
			return
		}
		if len(hits) == 0 {
			a.runOnUI(func() {
				a.FileSearchBusy = false
				a.SearchStatusMsg = "No matches"
				if a.Window != nil {
					a.Window.Invalidate()
				}
			})
			return
		}

		if confirm != nil {
			decided := make(chan bool, 1)
			a.runOnUI(func() {
				confirm(len(hits), func(proceed bool) {
					decided <- proceed
				})
			})
			if <-decided == false {
				a.runOnUI(func() {
					a.FileSearchBusy = false
					if a.Window != nil {
						a.Window.Invalidate()
					}
				})
				return
			}
		}

		res, err := a.replaceAcrossWorkspace(root, opts)
		a.runOnUI(func() {
			a.FileSearchBusy = false
			a.ContentHitsAt = "" // force a fresh search next time
			if err != nil {
				a.SearchStatusMsg = err.Error()
				a.queueToast(ui.Toast{Title: "Replace failed", Description: err.Error(), Type: "error"})
			} else {
				a.SearchStatusMsg = fmt.Sprintf("Replaced %d occurrences in %d files", res.TotalReplacements, res.FilesChanged)
			}
			if a.Window != nil {
				a.Window.Invalidate()
			}
		})
	}()
}

// replaceAcrossWorkspace applies the replacement on disk with the same
// matching semantics as SearchManager.ReplaceAll (ripgrep-equivalent regex,
// gitignore-aware walk, binary/oversize skip), except for files open in the
// editor: for those the replacement applies to the in-memory buffer and is
// persisted through the editor manager (atomic write, mode/CRLF/BOM
// preservation, saved-mark), so editor state and disk agree and a later save
// cannot silently revert the replace.
func (a *AppUI) replaceAcrossWorkspace(root string, opts search.ReplaceOptions) (search.ReplaceResult, error) {
	var result search.ReplaceResult
	if strings.HasPrefix(root, "ssh://") {
		return result, fmt.Errorf("replace across remote workspaces is not supported yet")
	}
	re, err := buildReplaceRegex(opts.SearchOptions)
	if err != nil {
		return result, err
	}
	gi := gitignore.Load(root)

	// Collect the open documents first: their paths are excluded from the
	// disk rewrite below.
	openPaths := make(map[string]bool)
	for _, p := range a.Editor.OpenTabs() {
		if a.Editor.GetDoc(p) != nil && !strings.HasPrefix(p, "terminal://") {
			openPaths[p] = true
		}
	}

	// matchedOpen records which open paths matched on disk so their buffers
	// get the replacement applied too (a buffer with unsaved edits may no
	// longer match the on-disk text the walk saw).
	matchedOpen := make(map[string]bool)

	err = filepath.Walk(root, func(p string, info os.FileInfo, werr error) error {
		if werr != nil || info == nil {
			return nil
		}
		if info.IsDir() {
			if rkignore.Name(info.Name()) || (gi != nil && gi.MatchDir(info.Name())) {
				return filepath.SkipDir
			}
			return nil
		}
		if info.Size() > 5*1024*1024 {
			return nil
		}
		content, rerr := os.ReadFile(p)
		if rerr != nil {
			return nil
		}
		if bytes.IndexByte(content, 0) != -1 || !re.Match(content) {
			return nil
		}
		if openPaths[p] {
			matchedOpen[p] = true
			return nil
		}
		s := string(content)
		out := replaceAllPreservingCase(re, s, opts)
		if out == s {
			return nil
		}
		if werr := writeFilePreservingMode(p, []byte(out), info.Mode()); werr != nil {
			return nil
		}
		result.FilesChanged++
		result.TotalReplacements += len(re.FindAllString(s, -1))
		result.Files = append(result.Files, p)
		return nil
	})
	if err != nil {
		return result, err
	}

	// Open buffers are the source of truth: apply the replacement to their
	// current text (preserving unsaved edits) and persist through the editor
	// manager so disk and buffer agree.
	for p := range matchedOpen {
		doc := a.Editor.GetDoc(p)
		if doc == nil || doc.Editor == nil || doc.Editor.Buffer == nil {
			continue
		}
		bufText := doc.Editor.Buffer.String()
		out := replaceAllPreservingCase(re, bufText, opts)
		if out == bufText {
			continue
		}
		result.TotalReplacements += len(re.FindAllString(bufText, -1))
		if serr := a.Editor.SetDocTextAndSave(p, out); serr != nil {
			if result.Files == nil {
				result.Files = []string{p}
			} else {
				result.Files = append(result.Files, p)
			}
			result.FilesChanged++
			return result, fmt.Errorf("replacing in %s: %w", filepath.Base(p), serr)
		}
		result.FilesChanged++
		result.Files = append(result.Files, p)
	}
	return result, nil
}

// buildReplaceRegex mirrors RakitKode's SearchManager regex construction so
// the replace applies exactly the matches the user previewed.
func buildReplaceRegex(opts search.SearchOptions) (*regexp.Regexp, error) {
	pattern := opts.Query
	if !opts.UseRegex {
		pattern = regexp.QuoteMeta(pattern)
	}
	if opts.MatchWholeWord {
		pattern = `\b(?:` + pattern + `)\b`
	}
	if !opts.MatchCase {
		pattern = `(?i)` + pattern
	}
	return regexp.Compile(pattern)
}

// replaceAllPreservingCase applies the replacement, honoring PreserveCase
// the way the RakitKode manager does.
func replaceAllPreservingCase(re *regexp.Regexp, s string, opts search.ReplaceOptions) string {
	return re.ReplaceAllStringFunc(s, func(m string) string {
		if opts.PreserveCase {
			return preserveCaseReplacement(m, opts.Replacement)
		}
		return opts.Replacement
	})
}

func preserveCaseReplacement(match, replacement string) string {
	if match == strings.ToUpper(match) && strings.ToLower(match) != match {
		return strings.ToUpper(replacement)
	}
	r := []rune(match)
	if len(r) > 0 && unicode.IsUpper(r[0]) {
		rr := []rune(replacement)
		if len(rr) > 0 {
			rr[0] = unicode.ToUpper(rr[0])
		}
		return string(rr)
	}
	return replacement
}

// writeFilePreservingMode writes data to p with the file's existing mode.
func writeFilePreservingMode(p string, data []byte, mode os.FileMode) error {
	if err := os.WriteFile(p, data, mode.Perm()); err != nil {
		return err
	}
	return nil
}

// openSearchResultChecked wraps openSearchResult (app.go) so open failures
// surface as a toast instead of a silent no-op.
func (a *AppUI) openSearchResultChecked(relPath string, line int, query string) {
	if relPath == "" || a.Workspace == nil {
		return
	}
	full := a.Workspace.ResolveFullPath(relPath)
	if _, err := a.Editor.OpenFile(full, relPath); err != nil {
		a.queueToast(ui.Toast{Title: "Open failed", Description: err.Error(), Type: "error"})
		return
	}
	a.openSearchResult(relPath, line, query)
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
						ui.Text(c, truncRunes(h.Content, 60)).FontSize(11).Grow(1).SingleLine()
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

// openSearchHitAtLine opens a file and jumps to the given line (1-based),
// selecting the matched query's column range when found on that line.
func (a *AppUI) openSearchHitAtLine(relPath string, line int, query string) {
	full := a.Workspace.ResolveFullPath(relPath)
	doc, err := a.Editor.OpenFile(full, relPath)
	if err != nil || doc == nil {
		desc := "file could not be opened"
		if err != nil {
			desc = err.Error()
		}
		a.queueToast(ui.Toast{Title: "Open failed", Description: desc, Type: "error"})
		return
	}
	buf := doc.Editor.Buffer
	lineCount := buf.LineCount()
	lineIdx := line - 1
	if lineIdx < 0 {
		lineIdx = 0
	}
	if lineCount > 0 && lineIdx >= lineCount {
		lineIdx = lineCount - 1
	}

	colStart, colEnd := 0, 0
	if q := strings.TrimSpace(query); q != "" && lineIdx < lineCount {
		lineText := buf.Line(lineIdx)
		if a.FileSearchRegex {
			pattern := q
			if !a.FileSearchCase {
				pattern = "(?i)" + pattern
			}
			if re, rerr := regexp.Compile(pattern); rerr == nil {
				if loc := re.FindStringIndex(lineText); len(loc) == 2 {
					colStart, colEnd = loc[0], loc[1]
				}
			}
		} else if a.FileSearchCase {
			if idx := strings.Index(lineText, q); idx >= 0 {
				colStart, colEnd = idx, idx+len(q)
			}
		} else {
			if idx := strings.Index(strings.ToLower(lineText), strings.ToLower(q)); idx >= 0 {
				colStart, colEnd = idx, idx+len(q)
			}
		}
	}

	lineOffset := buf.Offset(mygoeditor.Pos{Line: lineIdx, Col: 0})
	if colEnd > colStart && lineOffset+colEnd <= buf.Len() {
		doc.Editor.Select(lineOffset+colStart, lineOffset+colEnd)
	}
	doc.Editor.GoTo(mygoeditor.Pos{Line: lineIdx, Col: colStart})
	a.SelectedFile = relPath
}
