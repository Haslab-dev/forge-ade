package ui

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

// OpenDocument represents an open file buffer with its editor and diff states.
type OpenDocument struct {
	RelativePath string
	FullPath     string
	Original     *editor.Buffer
	Editor       *editor.State
	Diff         *editor.DiffState
	DiskContent  string
}

// EditorManager manages all open file buffers, active tabs, split layouts, and editor settings.
type EditorManager struct {
	mu           sync.RWMutex
	docs         map[string]*OpenDocument
	patchDiffs   map[string]*editor.DiffState
	OpenTabPaths []string
	ActivePath   string
	SoftWrap     bool
	IsSplit      bool
	IsDiffView   bool
	DiffLayout   int    // 0: Split, 1: Unified
	DiffInline   string // "Words", "Characters", "Off"
	StatusMsg    string
	Theme        *ThemeManager
}

// NewEditorManager creates and initializes an EditorManager.
func NewEditorManager() *EditorManager {
	return &EditorManager{
		docs:         make(map[string]*OpenDocument),
		patchDiffs:   make(map[string]*editor.DiffState),
		OpenTabPaths: make([]string, 0),
		SoftWrap:     false,
		IsSplit:      false,
		IsDiffView:   false,
		DiffLayout:   0,
		DiffInline:   "Words",
	}
}

// editorFont returns the code font at ForgeADE's cmSetup metrics: Geist Mono
// at 13px (the default --font-size-code), line-height 1.65.
func (em *EditorManager) editorFont() ui.Font {
	size := EditorCodeFontSize
	family := FontMono
	if em.Theme != nil {
		family = monoFor(em.Theme.FontFamily)
	}
	return ui.Font{Family: family, Size: float32(size)}
}

// applyTheme attaches the editor theme and fonts to an editor state.
func (em *EditorManager) applyTheme(st *editor.State) {
	st.Font = em.editorFont()
	st.LineHeight = 1.65 // cmSetup: lineHeight 1.65
	if em.Theme != nil {
		st.Theme = em.Theme.EditorTheme()
	}
}

// applyTheme attaches the editor theme and fonts to a diff state.
func (em *EditorManager) applyDiffTheme(d *editor.DiffState) {
	d.Font = em.editorFont()
	d.LineHeight = 1.65
	if em.Theme != nil {
		d.Theme = em.Theme.EditorTheme()
	}
}

// OpenFile loads a file into an editor buffer if not already open, or activates it.
func (em *EditorManager) OpenFile(fullPath, relPath string) (*OpenDocument, error) {
	em.mu.Lock()
	defer em.mu.Unlock()

	// Ensure path is tracked in open tabs list
	found := false
	for _, p := range em.OpenTabPaths {
		if p == fullPath {
			found = true
			break
		}
	}
	if !found {
		em.OpenTabPaths = append(em.OpenTabPaths, fullPath)
	}

	if doc, exists := em.docs[fullPath]; exists {
		em.ActivePath = fullPath
		return doc, nil
	}

	contentBytes, err := os.ReadFile(fullPath)
	if err != nil {
		return nil, fmt.Errorf("read file %s: %w", fullPath, err)
	}

	content := string(contentBytes)
	origBuf := editor.NewBuffer(content)
	editBuf := editor.NewBuffer(content)

	lang := editor.LanguageFor(filepath.Base(fullPath))
	origBuf.SetLanguage(lang)
	editBuf.SetLanguage(lang)

	doc := &OpenDocument{
		RelativePath: relPath,
		FullPath:     fullPath,
		Original:     origBuf,
		DiskContent:  content,
		Editor: &editor.State{
			Buffer:     editBuf,
			DiffBase:   origBuf,
			WrapGuides: []int{100},
			SoftWrap:   em.SoftWrap,
		},
		Diff: &editor.DiffState{
			Old:           origBuf,
			New:           editBuf,
			Editable:      true,
			HunkActions:   true,
			LineSelection: true,
			Wrap:          em.SoftWrap,
		},
	}

	em.applyTheme(doc.Editor)
	em.applyDiffTheme(doc.Diff)
	em.setupEditorCallbacks(doc)
	em.docs[fullPath] = doc
	em.ActivePath = fullPath
	return doc, nil
}

// OpenPatchDiff builds (or returns the cached) diff of a git patch for the
// changes drawer, rendered with MyGo's internal diff viewer.
func (em *EditorManager) OpenPatchDiff(relPath, patch string, tm *ThemeManager) *editor.DiffState {
	em.mu.Lock()
	defer em.mu.Unlock()
	if tm != nil {
		em.Theme = tm
	}
	if d, ok := em.patchDiffs[relPath]; ok {
		return d
	}
	files, err := editor.ParsePatch(patch)
	if err != nil || len(files) == 0 {
		return nil
	}
	d := &editor.DiffState{
		Layout:        editor.DiffLayout(em.DiffLayout),
		HunkActions:   true,
		LineSelection: true,
	}
	d.SetPatch(files[0])
	em.applyDiffTheme(d)
	em.patchDiffs[relPath] = d
	return d
}

// CloseTab closes an open tab and activates another if available.
func (em *EditorManager) CloseTab(fullPath string) {
	em.mu.Lock()
	defer em.mu.Unlock()

	newTabs := make([]string, 0, len(em.OpenTabPaths))
	for _, p := range em.OpenTabPaths {
		if p != fullPath {
			newTabs = append(newTabs, p)
		}
	}
	em.OpenTabPaths = newTabs

	if em.ActivePath == fullPath {
		if len(em.OpenTabPaths) > 0 {
			em.ActivePath = em.OpenTabPaths[len(em.OpenTabPaths)-1]
		} else {
			em.ActivePath = ""
		}
	}
}

// CloseAll closes every open tab and patch diff (workspace switch).
func (em *EditorManager) CloseAll() {
	em.mu.Lock()
	defer em.mu.Unlock()
	em.docs = make(map[string]*OpenDocument)
	em.patchDiffs = make(map[string]*editor.DiffState)
	em.OpenTabPaths = em.OpenTabPaths[:0]
	em.ActivePath = ""
	em.IsDiffView = false
}

// CycleTab activates the next (dir 1) or previous (dir -1) open tab, as
// Ctrl+Tab does in the original.
func (em *EditorManager) CycleTab(dir int) {
	em.mu.Lock()
	defer em.mu.Unlock()
	n := len(em.OpenTabPaths)
	if n < 2 {
		return
	}
	idx := -1
	for i, p := range em.OpenTabPaths {
		if p == em.ActivePath {
			idx = i
			break
		}
	}
	if idx < 0 {
		em.ActivePath = em.OpenTabPaths[0]
		return
	}
	idx = (idx + dir + n) % n
	em.ActivePath = em.OpenTabPaths[idx]
}

// ActiveDoc returns the currently active document or nil.
func (em *EditorManager) ActiveDoc() *OpenDocument {
	em.mu.RLock()
	defer em.mu.RUnlock()
	return em.docs[em.ActivePath]
}

// GetDoc returns the document for a given path or nil.
func (em *EditorManager) GetDoc(path string) *OpenDocument {
	em.mu.RLock()
	defer em.mu.RUnlock()
	return em.docs[path]
}

// SaveActiveFile persists modifications to disk and resets the diff baseline.
func (em *EditorManager) SaveActiveFile() error {
	em.mu.Lock()
	defer em.mu.Unlock()

	doc := em.docs[em.ActivePath]
	if doc == nil {
		return fmt.Errorf("no active document to save")
	}

	newText := doc.Editor.Buffer.String()
	if err := os.WriteFile(doc.FullPath, []byte(newText), 0644); err != nil {
		em.StatusMsg = fmt.Sprintf("Error saving %s: %v", doc.RelativePath, err)
		return err
	}

	// Update original buffer to clear diff status
	doc.DiskContent = newText
	newOrig := editor.NewBuffer(newText)
	lang := editor.LanguageFor(filepath.Base(doc.FullPath))
	newOrig.SetLanguage(lang)

	doc.Original = newOrig
	doc.Editor.DiffBase = newOrig
	doc.Diff.Old = newOrig
	em.StatusMsg = fmt.Sprintf("Saved %s", doc.RelativePath)
	return nil
}

// setupEditorCallbacks attaches autocompletion, hover, and diagnostics hooks.
func (em *EditorManager) setupEditorCallbacks(doc *OpenDocument) {
	st := doc.Editor
	st.OnCompletion = func(r editor.CompletionRequest) {
		st.ShowCompletions(r.Offset-len(r.Prefix), wordCompletions(st.Buffer, r.Prefix))
	}
	st.OnHover = func(r editor.HoverRequest) {
		word, start, end := wordAtOffset(st.Buffer, r.Offset)
		if word == "" {
			return
		}
		count := strings.Count(st.Buffer.String(), word)
		st.ShowHover(start, end, fmt.Sprintf("Symbol '%s' (%d occurrences)", word, count), nil)
	}
}

// todosAndDiagnostics scans the buffer for TODO, FIXME, and BUG markers.
func todosAndDiagnostics(b *editor.Buffer) []editor.Diagnostic {
	var out []editor.Diagnostic
	for i := 0; i < b.LineCount(); i++ {
		line := b.Line(i)
		for _, kw := range []string{"TODO", "FIXME", "BUG", "WARN"} {
			if at := strings.Index(line, kw); at >= 0 {
				start := b.Offset(editor.Pos{Line: i, Col: at})
				sev := editor.SeverityWarning
				if kw == "BUG" {
					sev = editor.SeverityError
				}
				out = append(out, editor.Diagnostic{
					Start:    start,
					End:      start + len(line) - at,
					Severity: sev,
					Message:  strings.TrimSpace(line[at:]),
					Source:   strings.ToLower(kw),
				})
			}
		}
	}
	return out
}

// RenderTabBar renders ForgeADE's 35px tab bar: h-full px-3 text-xs
// font-medium tabs with dirty dots, close-on-hover, and the pane actions.
func (em *EditorManager) RenderTabBar(c *ui.Context) {
	tc := ThemeColors{}
	if em.Theme != nil {
		tc = em.Theme.Colors()
	}

	ui.Row(c).Height(EditorTabBarHeight).Padding(0, 8).AlignItems(ui.Center).
		Background(tc.Surface).BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
		// Open tab items: h-full px-3 gap-2 text-xs font-medium border-r
		for _, tabPath := range em.OpenTabPaths {
			path := tabPath
			doc := em.GetDoc(path)
			if doc == nil {
				continue
			}

			isActive := (path == em.ActivePath)
			isModified := doc.Editor.Buffer.Modified()
			baseName := filepath.Base(doc.RelativePath)

			tab := ui.Row(c.Key("tab-"+path)).Height(EditorTabBarHeight).Padding(0, 12).Gap(8).
				AlignItems(ui.Center).BorderWidth(0, 1, 0, 0).BorderColor(tc.Border)
			tab.Children(func() {
				ui.Icon(c, FileIconSVG(doc.RelativePath)).Width(13).Height(13).TextColor(tc.IconMuted)
				label := ui.Text(c, baseName).FontSize(12)
				if isActive {
					label.FontWeight(500)
				}
				if isModified {
					ui.Box(c).Size(6, 6).Radius(3).Background(tc.Warning)
				}
			})
			if isActive {
				tab.Background(tc.SurfaceOverlay)
			} else if tab.Hovered() {
				tab.Background(tc.Text.Alpha(0.04))
			}
			if tab.Clicked() {
				em.ActivePath = path
			}
		}

		if len(em.OpenTabPaths) == 0 {
			ui.Text(c, "No open files").FontSize(12).TextColor(tc.TextMuted)
		}

		ui.Box(c).Grow(1)

		// Right actions: Split, Diff, Wrap, Save
		actionBtn := func(icon *ui.SVG, label string, active bool, onPick func()) {
			btn := ui.Row(c).Height(24).Radius(6).Padding(0, 8).Gap(4).AlignItems(ui.Center)
			btn.Children(func() {
				ic := ui.Icon(c, icon).Width(13).Height(13).TextColor(tc.IconMuted)
				if active {
					ic.TextColor(tc.Accent)
				}
				if label != "" {
					ui.Text(c, label).FontSize(12).TextColor(tc.TextMuted)
				}
			})
			if active {
				btn.Background(tc.Accent.Alpha(0.1))
			} else if btn.Hovered() {
				btn.Background(tc.Text.Alpha(0.05))
			}
			if btn.Clicked() {
				onPick()
			}
		}
		splitLabel := "Split"
		if em.IsSplit {
			splitLabel = "Single"
		}
		actionBtn(TablerColumns, splitLabel, false, func() { em.IsSplit = !em.IsSplit })
		actionBtn(TablerGitCompare, "Diff", em.IsDiffView, func() { em.IsDiffView = !em.IsDiffView })
		actionBtn(TablerRefresh, "Wrap", em.SoftWrap, func() { em.SoftWrap = !em.SoftWrap })
		actionBtn(TablerDeviceFloppy, "Save", false, func() { _ = em.SaveActiveFile() })
	})
}

// RenderEditorView renders the tab bar, editor buffer(s) (split or single), or diff view.
func (em *EditorManager) RenderEditorView(c *ui.Context) {
	t := c.Theme()

	ui.Column(c).Fill().Children(func() {
		// 1. Tab Bar
		em.RenderTabBar(c)

		// 2. Viewport
		if em.IsDiffView {
			em.RenderDiff(c)
			return
		}

		if em.IsSplit && len(em.OpenTabPaths) >= 2 {
			// Multi-Pane Split View with a 3px divider
			ui.Row(c).Grow(1).AlignItems(ui.Stretch).Children(func() {
				// Left Pane
				ui.Column(c).Grow(1).FillHeight().Children(func() {
					em.RenderEditor(c)
				})

				// Divider (w-[3px] bg-border hover:bg-primary/60)
				ui.Box(c).Width(3).FillHeight().Background(t.Border)

				// Right Pane: second tab in list
				var secondaryPath string
				for _, p := range em.OpenTabPaths {
					if p != em.ActivePath {
						secondaryPath = p
						break
					}
				}
				secondaryDoc := em.GetDoc(secondaryPath)
				ui.Column(c).Grow(1).FillHeight().Children(func() {
					if secondaryDoc != nil {
						secondaryDoc.Editor.SoftWrap = em.SoftWrap
						em.applyTheme(secondaryDoc.Editor)
						ui.Row(c).Height(28).Padding(0, 12).AlignItems(ui.Center).
							BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
							ui.Text(c, secondaryDoc.RelativePath).FontSize(12).FontWeight(500)
						})
						editor.View(c, secondaryDoc.Editor).Grow(1).FillWidth()
					}
				})
			})
			return
		}

		// Single Editor View
		em.RenderEditor(c)
	})
}

// RenderEditor renders the active editor buffer, breadcrumbs, and status bar.
func (em *EditorManager) RenderEditor(c *ui.Context) {
	t := c.Theme()
	tc := ThemeColors{}
	if em.Theme != nil {
		tc = em.Theme.Colors()
	}
	doc := em.ActiveDoc()
	if doc == nil {
		ui.Column(c).Grow(1).Center().Gap(6).Children(func() {
			ui.Text(c, "No file open").FontSize(17).FontWeight(600).TextColor(t.TextMuted)
			ui.Text(c, "Select a file from the workspace sidebar or press ⌘P to open.").FontSize(13).TextColor(t.TextMuted)
		})
		return
	}

	doc.Editor.SoftWrap = em.SoftWrap
	doc.Editor.SetDiagnostics(doc.Editor.Buffer.Version(), todosAndDiagnostics(doc.Editor.Buffer))
	em.applyTheme(doc.Editor)

	ui.Column(c).Grow(1).FillWidth().Children(func() {
		// Breadcrumbs Path Bar
		ui.Row(c).Height(28).Gap(6).Padding(0, 12).AlignItems(ui.Center).
			Background(tc.Canvas).BorderWidth(0, 0, 1, 0).BorderColor(tc.Border).Children(func() {
			ui.Icon(c, TablerFolder).Width(13).Height(13).TextColor(tc.IconMuted)
			parts := strings.Split(doc.RelativePath, "/")
			for i, part := range parts {
				if i > 0 {
					ui.Icon(c, TablerChevronRight).Width(10).Height(10).TextColor(tc.IconMuted)
				}
				isLast := (i == len(parts)-1)
				if isLast {
					ui.Icon(c, FileIconSVG(doc.RelativePath)).Width(13).Height(13).TextColor(tc.IconMuted)
					ui.Text(c, part).FontSize(12).FontWeight(500)
				} else {
					ui.Text(c, part).FontSize(12).TextColor(t.TextMuted)
				}
			}
			ui.Box(c).Grow(1)
			langName := "Plain Text"
			if l := doc.Editor.Buffer.Language(); l != nil {
				langName = l.Name
			}
			ui.Text(c, langName).FontSize(10).FontWeight(500).TextColor(t.TextMuted)
		})

		// Virtualized Code Editor View (MyGo internal editor)
		editor.View(c, doc.Editor).Grow(1).FillWidth().AutoFocus()

	})
}

// RenderDiff renders the git-style diff of changes in the active buffer
// through MyGo's internal diff viewer.
func (em *EditorManager) RenderDiff(c *ui.Context) {
	t := c.Theme()
	doc := em.ActiveDoc()
	if doc == nil {
		ui.Column(c).Grow(1).Center().Children(func() {
			ui.Text(c, "No file open").FontSize(17).FontWeight(600).TextColor(t.TextMuted)
		})
		return
	}

	d := doc.Diff
	d.Wrap = em.SoftWrap
	d.Layout = editor.DiffLayout(em.DiffLayout)

	switch em.DiffInline {
	case "Characters":
		d.Inline = editor.DiffInlineChar
	case "Off":
		d.Inline = editor.DiffInlineNone
	default:
		d.Inline = editor.DiffInlineWordAlt
	}
	em.applyDiffTheme(d)

	// Diff Controls Subheader
	ui.Row(c).Gap(12).Padding(6, 12).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
		ui.Text(c, fmt.Sprintf("Diff: %s", doc.RelativePath)).FontSize(13).FontWeight(500)
		ui.Segmented(c, &em.DiffLayout, "Split", "Unified")
		ui.Select(c, &em.DiffInline, []string{"Words", "Characters", "Off"})
		ui.Box(c).Grow(1)
		added, deleted := d.Stats()
		if added > 0 || deleted > 0 {
			ui.Text(c, fmt.Sprintf("+%d −%d", added, deleted)).FontSize(11).TextColor(t.TextMuted)
		}
		ui.Text(c, "Wrap").FontSize(12).TextColor(t.TextMuted)
		ui.Switch(c, &em.SoftWrap).Label("Soft wrap")
	})

	if len(d.Hunks()) == 0 {
		ui.Column(c).Grow(1).Center().Gap(4).Children(func() {
			ui.Text(c, "No uncommitted changes").FontSize(16).FontWeight(600).TextColor(t.TextMuted)
			ui.Text(c, "Buffer matches the saved disk baseline.").FontSize(13).TextColor(t.TextMuted)
		})
		return
	}

	editor.DiffView(c, d).Grow(1).FillWidth()
}

// wordCompletions generates basic token completions from buffer text.
func wordCompletions(b *editor.Buffer, prefix string) []editor.Completion {
	seen := map[string]bool{}
	var out []editor.Completion
	words := strings.FieldsFunc(b.String(), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsDigit(r) && r != '_'
	})
	for _, w := range words {
		if len(w) < 3 || seen[w] || w == prefix {
			continue
		}
		seen[w] = true
		out = append(out, editor.Completion{Label: w, Kind: editor.CompletionText, Detail: "word"})
	}
	return out
}

// wordAtOffset retrieves the identifier token around an offset.
func wordAtOffset(b *editor.Buffer, offset int) (string, int, int) {
	text := b.String()
	if offset < 0 || offset >= len(text) {
		return "", 0, 0
	}
	start := offset
	for start > 0 && isIdentRune(rune(text[start-1])) {
		start--
	}
	end := offset
	for end < len(text) && isIdentRune(rune(text[end])) {
		end++
	}
	if start >= end {
		return "", 0, 0
	}
	return text[start:end], start, end
}

func isIdentRune(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}
