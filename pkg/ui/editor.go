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

// EditorManager manages all open file buffers, active tabs, and editor settings.
type EditorManager struct {
	mu         sync.RWMutex
	docs       map[string]*OpenDocument
	ActivePath string
	SoftWrap   bool
	DiffLayout int    // 0: Split, 1: Unified
	DiffInline string // "Words", "Characters", "Off"
	StatusMsg  string
}

// NewEditorManager creates and initializes an EditorManager.
func NewEditorManager() *EditorManager {
	return &EditorManager{
		docs:       make(map[string]*OpenDocument),
		SoftWrap:   false,
		DiffLayout: 0,
		DiffInline: "Words",
	}
}

// OpenFile loads a file into an editor buffer if not already open, or activates it.
func (em *EditorManager) OpenFile(fullPath, relPath string) (*OpenDocument, error) {
	em.mu.Lock()
	defer em.mu.Unlock()

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

// ActiveDoc returns the currently active document or nil.
func (em *EditorManager) ActiveDoc() *OpenDocument {
	em.mu.RLock()
	defer em.mu.RUnlock()
	return em.docs[em.ActivePath]
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

// RenderEditor renders the active editor buffer and controls.
func (em *EditorManager) RenderEditor(c *ui.Context) {
	t := c.Theme()
	doc := em.ActiveDoc()
	if doc == nil {
		ui.Column(c).Grow(1).Center().Children(func() {
			ui.Text(c, "No file open").FontSize(18).Bold().TextColor(t.TextMuted)
			ui.Text(c, "Select a file from the workspace sidebar to begin editing.").TextColor(t.TextMuted)
		})
		return
	}

	doc.Editor.SoftWrap = em.SoftWrap

	// Sub-toolbar
	ui.Row(c).Gap(12).Padding(8, 12).AlignItems(ui.Center).BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
		ui.Text(c, doc.RelativePath).Bold()
		if doc.Editor.Buffer.Modified() {
			ui.Text(c, "• Modified").TextColor(t.Accent)
		}
		if ui.Button(c, "Save File").Clicked() {
			_ = em.SaveActiveFile()
		}
		ui.Box(c).Grow(1)
		ui.Text(c, "Wrap").TextColor(t.TextMuted)
		ui.Switch(c, &em.SoftWrap).Label("Soft wrap")
	})

	// Editor view
	editor.View(c, doc.Editor).Grow(1).FillWidth().AutoFocus()

	// Status line
	pos := doc.Editor.CursorPos()
	sels := len(doc.Editor.Selections())
	langName := "Plain Text"
	if l := doc.Editor.Buffer.Language(); l != nil {
		langName = l.Name
	}
	added, deleted := doc.Diff.Stats()

	ui.Row(c).Gap(16).Padding(4, 12).BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Children(func() {
		ui.Text(c, FormatLineCol(pos.Line, pos.Col)).FontSize(12).TextColor(t.TextMuted)
		if sels > 1 {
			ui.Text(c, fmt.Sprintf("%d selections", sels)).FontSize(12).TextColor(t.TextMuted)
		}
		if added > 0 || deleted > 0 {
			ui.Text(c, fmt.Sprintf("+%d −%d", added, deleted)).FontSize(12).TextColor(t.Accent)
		}
		ui.Box(c).Grow(1)
		if em.StatusMsg != "" {
			ui.Text(c, em.StatusMsg).FontSize(12).TextColor(t.Text)
		}
		ui.Box(c).Grow(1)
		ui.Text(c, fmt.Sprintf("%d lines · %s", doc.Editor.Buffer.LineCount(), langName)).FontSize(12).TextColor(t.TextMuted)
	})
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

	// Diff Controls
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
