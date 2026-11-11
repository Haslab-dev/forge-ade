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
	OpenTabPaths []string
	ActivePath   string
	SoftWrap     bool
	IsSplit      bool
	IsDiffView   bool
	DiffLayout   int    // 0: Split, 1: Unified
	DiffInline   string // "Words", "Characters", "Off"
	StatusMsg    string
}

// NewEditorManager creates and initializes an EditorManager.
func NewEditorManager() *EditorManager {
	return &EditorManager{
		docs:         make(map[string]*OpenDocument),
		OpenTabPaths: make([]string, 0),
		SoftWrap:     false,
		IsSplit:      false,
		IsDiffView:   false,
		DiffLayout:   0,
		DiffInline:   "Words",
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

	em.setupEditorCallbacks(doc)
	em.docs[fullPath] = doc
	em.ActivePath = fullPath
	return doc, nil
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

// setupEditorCallbacks attaches autocompletion and diagnostics hooks.
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

// RenderTabBar renders the horizontal strip of open tabs matching Zed/ForgeADE.
func (em *EditorManager) RenderTabBar(c *ui.Context) {
	t := c.Theme()

	ui.Row(c).Gap(4).Padding(4, 8).AlignItems(ui.Center).Background(t.Surface).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
		// Open tab items
		for _, tabPath := range em.OpenTabPaths {
			path := tabPath
			doc := em.GetDoc(path)
			if doc == nil {
				continue
			}

			isActive := (path == em.ActivePath)
			isModified := doc.Editor.Buffer.Modified()
			baseName := filepath.Base(doc.RelativePath)

			tabLabel := baseName
			if isModified {
				tabLabel += " •"
			}

			var tabBtn ui.Element
			if isActive {
				tabBtn = ui.PrimaryButton(c, tabLabel).FontSize(12)
			} else {
				tabBtn = ui.Button(c, tabLabel).FontSize(12)
			}
			if tabBtn.Clicked() {
				em.ActivePath = path
			}

			// Tab close icon button
			closeBtn := ui.Button(c, "×").FontSize(11)
			if closeBtn.Clicked() {
				em.CloseTab(path)
			}
		}

		if len(em.OpenTabPaths) == 0 {
			ui.Text(c, "No open files").FontSize(11).TextColor(t.TextMuted)
		}

		ui.Box(c).Grow(1)

		// Split Editor button
		splitIcon := "◫ Split"
		if em.IsSplit {
			splitIcon = "▣ Single"
		}
		if ui.Button(c, splitIcon).FontSize(11).Clicked() {
			em.IsSplit = !em.IsSplit
		}

		// Diff View toggle button
		var diffBtn ui.Element
		if em.IsDiffView {
			diffBtn = ui.PrimaryButton(c, "Diff").FontSize(11)
		} else {
			diffBtn = ui.Button(c, "Diff").FontSize(11)
		}
		if diffBtn.Clicked() {
			em.IsDiffView = !em.IsDiffView
		}

		// Soft wrap switch
		ui.Text(c, "Wrap").FontSize(11).TextColor(t.TextMuted)
		ui.Switch(c, &em.SoftWrap)

		// Save File button
		if ui.Button(c, "💾 Save").FontSize(11).Clicked() {
			_ = em.SaveActiveFile()
		}
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
			// Multi-Pane Split View
			ui.Row(c).Grow(1).AlignItems(ui.Stretch).Children(func() {
				// Left Pane
				ui.Column(c).Grow(1).FillHeight().BorderWidth(0, 1, 0, 0).BorderColor(t.Border).Children(func() {
					em.RenderEditor(c)
				})

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
						ui.Row(c).Padding(6, 12).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
							ui.Text(c, secondaryDoc.RelativePath).Bold().FontSize(12)
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

// RenderEditor renders the active editor buffer and controls.
func (em *EditorManager) RenderEditor(c *ui.Context) {
	t := c.Theme()
	doc := em.ActiveDoc()
	if doc == nil {
		ui.Column(c).Grow(1).Center().Children(func() {
			ui.Text(c, "No file open").FontSize(18).Bold().TextColor(t.TextMuted)
			ui.Text(c, "Select a file from the workspace sidebar or press ⌘P to open.").TextColor(t.TextMuted)
		})
		return
	}

	doc.Editor.SoftWrap = em.SoftWrap

	// Virtualized Code Editor View
	editor.View(c, doc.Editor).Grow(1).FillWidth().AutoFocus()
}

// RenderDiff renders the git-style diff of changes in the active buffer.
func (em *EditorManager) RenderDiff(c *ui.Context) {
	t := c.Theme()
	doc := em.ActiveDoc()
	if doc == nil {
		ui.Column(c).Grow(1).Center().Children(func() {
			ui.Text(c, "No file open").FontSize(18).Bold().TextColor(t.TextMuted)
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

	// Diff Controls Subheader
	ui.Row(c).Gap(12).Padding(8, 12).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
		ui.Text(c, fmt.Sprintf("Diff: %s", doc.RelativePath)).Bold()
		ui.Segmented(c, &em.DiffLayout, "Split", "Unified")
		ui.Select(c, &em.DiffInline, []string{"Words", "Characters", "Off"})
		ui.Box(c).Grow(1)
		ui.Text(c, "Wrap").TextColor(t.TextMuted)
		ui.Switch(c, &em.SoftWrap).Label("Soft wrap")
	})

	if len(d.Hunks()) == 0 {
		ui.Column(c).Grow(1).Center().Children(func() {
			ui.Text(c, "No uncommitted changes").FontSize(16).Bold().TextColor(t.TextMuted)
			ui.Text(c, "Buffer matches the saved disk baseline.").TextColor(t.TextMuted)
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
