package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode"

	"github.com/egoist/mygo/ui"
	mygoeditor "github.com/egoist/mygo/ui/editor"
	"github.com/hasdev/forge-ade/app/native/pkg/theme"
	"github.com/hasdev/forge-ade/core/remotefs"
)

// OpenDocument represents an open file buffer with its editor and diff states.
type OpenDocument struct {
	RelativePath string
	FullPath     string
	Original     *mygoeditor.Buffer
	Editor       *mygoeditor.State
	Diff         *mygoeditor.DiffState
	DiskContent  string
}

// EditorManager manages all open file buffers, active tabs, split layouts, and editor settings.
type EditorManager struct {
	mu             sync.RWMutex
	docs           map[string]*OpenDocument
	patchDiffs     map[string]*mygoeditor.DiffState
	readOnlyStates map[string]*mygoeditor.State
	OpenTabPaths   []string
	ActivePath     string
	SoftWrap       bool
	IsSplit        bool
	// PinnedTabs survive bulk close actions; ReadOnlyTabs render an italic
	// title and a non-editable buffer (CodeEditorPane parity).
	PinnedTabs   map[string]bool
	ReadOnlyTabs map[string]bool
	// Panes holds each split pane's document path (EditorPaneState parity);
	// PaneWeights are per-pane flex weights for the drag handles.
	Panes                 []string
	PaneWeights           []float32
	IsDiffView            bool
	DiffLayout            int    // 0: Split, 1: Unified
	DiffInline            string // "Words", "Characters", "Off"
	StatusMsg             string
	Theme                 *theme.ThemeManager
	RemoteFS              *remotefs.Manager
	WorkspaceDir          string
	OnDefinitionRequested func(symbol string, matches []PeekMatch)
}

// NewEditorManager creates and initializes an EditorManager.
func NewEditorManager() *EditorManager {
	return &EditorManager{
		docs:           make(map[string]*OpenDocument),
		patchDiffs:     make(map[string]*mygoeditor.DiffState),
		readOnlyStates: make(map[string]*mygoeditor.State),
		OpenTabPaths:   make([]string, 0),
		SoftWrap:       false,
		IsSplit:        false,
		PinnedTabs:     make(map[string]bool),
		ReadOnlyTabs:   make(map[string]bool),
		IsDiffView:     false,
		DiffLayout:     0,
		DiffInline:     "Words",
	}
}

// EditorFont returns the code font at ForgeADE metrics: Geist Mono at 13px, line-height 1.65.
func (em *EditorManager) EditorFont() ui.Font {
	size := theme.EditorCodeFontSize
	family := theme.FontMono
	if em.Theme != nil {
		family = theme.MonoFor(em.Theme.FontFamily)
	}
	return ui.Font{Family: family, Size: float32(size)}
}

// ApplyTheme attaches the editor theme and fonts to an editor state.
func (em *EditorManager) ApplyTheme(st *mygoeditor.State) {
	st.Font = em.EditorFont()
	st.LineHeight = 1.65
	if em.Theme != nil {
		st.Theme = em.Theme.EditorTheme()
	}
}

// ApplyDiffTheme attaches the editor theme and fonts to a diff state.
func (em *EditorManager) ApplyDiffTheme(d *mygoeditor.DiffState) {
	d.Font = em.EditorFont()
	d.LineHeight = 1.65
	if em.Theme != nil {
		d.Theme = em.Theme.EditorTheme()
	}
}

// OpenFile loads a file into an editor buffer if not already open, or activates it.
func (em *EditorManager) OpenFile(fullPath, relPath string) (*OpenDocument, error) {
	em.mu.Lock()
	defer em.mu.Unlock()

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

	em.ActivePath = fullPath

	if doc, exists := em.docs[fullPath]; exists {
		return doc, nil
	}

	var contentBytes []byte
	var err error
	if strings.HasPrefix(fullPath, "ssh://") && em.RemoteFS != nil {
		data, isRemote, rErr := em.RemoteFS.ReadFile(fullPath)
		if isRemote {
			contentBytes, err = data, rErr
		} else {
			contentBytes, err = os.ReadFile(fullPath)
		}
	} else {
		contentBytes, err = os.ReadFile(fullPath)
	}
	if err != nil {
		em.StatusMsg = fmt.Sprintf("Error reading %s: %v", relPath, err)
		return nil, err
	}
	content := string(contentBytes)

	origBuf := mygoeditor.NewBuffer(content)
	editBuf := mygoeditor.NewBuffer(content)

	lang := mygoeditor.LanguageFor(filepath.Base(fullPath))
	origBuf.SetLanguage(lang)
	editBuf.SetLanguage(lang)

	doc := &OpenDocument{
		RelativePath: relPath,
		FullPath:     fullPath,
		Original:     origBuf,
		DiskContent:  content,
		Editor: &mygoeditor.State{
			Buffer:     editBuf,
			DiffBase:   origBuf,
			WrapGuides: []int{100},
			SoftWrap:   em.SoftWrap,
		},
		Diff: &mygoeditor.DiffState{
			Old:           origBuf,
			New:           editBuf,
			Editable:      true,
			HunkActions:   true,
			LineSelection: true,
			Wrap:          em.SoftWrap,
		},
	}

	em.ApplyTheme(doc.Editor)
	em.ApplyDiffTheme(doc.Diff)
	em.setupEditorCallbacks(doc)

	em.docs[fullPath] = doc
	em.StatusMsg = fmt.Sprintf("Opened %s", relPath)
	return doc, nil
}

// OpenPatchDiff parses and caches a patch diff view for the given file.
// OpenPatchDiffKeyed builds or returns the cached diff for an arbitrary key
// (commit hash + path), so multiple commit diffs coexist.
func (em *EditorManager) OpenPatchDiffKeyed(key, patch string, tm *theme.ThemeManager) *mygoeditor.DiffState {
	em.mu.Lock()
	defer em.mu.Unlock()
	if tm != nil {
		em.Theme = tm
	}
	if d, ok := em.patchDiffs[key]; ok {
		return d
	}
	files, err := mygoeditor.ParsePatch(patch)
	if err != nil || len(files) == 0 {
		return nil
	}
	d := &mygoeditor.DiffState{
		Layout:        mygoeditor.DiffLayout(em.DiffLayout),
		HunkActions:   true,
		LineSelection: true,
	}
	d.SetPatch(files[0])
	if em.Theme != nil {
		d.Theme = em.Theme.EditorTheme()
	}
	d.Font = em.EditorFont()
	d.LineHeight = 1.65
	em.patchDiffs[key] = d
	return d
}

func (em *EditorManager) OpenPatchDiff(relPath, patch string, tm *theme.ThemeManager) *mygoeditor.DiffState {
	em.mu.Lock()
	defer em.mu.Unlock()
	if tm != nil {
		em.Theme = tm
	}
	if d, ok := em.patchDiffs[relPath]; ok {
		return d
	}
	files, err := mygoeditor.ParsePatch(patch)
	if err != nil || len(files) == 0 {
		return nil
	}
	d := &mygoeditor.DiffState{
		Layout:        mygoeditor.DiffLayout(em.DiffLayout),
		HunkActions:   true,
		LineSelection: true,
	}
	d.SetPatch(files[0])
	em.ApplyDiffTheme(d)
	em.patchDiffs[relPath] = d
	return d
}

// GetOrCreateBufferDiff builds or returns the cached diff between old and new text buffers.
func (em *EditorManager) GetOrCreateBufferDiff(key, oldStr, newStr string, tm *theme.ThemeManager) *mygoeditor.DiffState {
	em.mu.Lock()
	defer em.mu.Unlock()
	if tm != nil {
		em.Theme = tm
	}
	if d, ok := em.patchDiffs[key]; ok {
		return d
	}
	oldBuf := mygoeditor.NewBuffer(oldStr)
	newBuf := mygoeditor.NewBuffer(newStr)
	d := &mygoeditor.DiffState{
		Old:             oldBuf,
		New:             newBuf,
		Layout:          mygoeditor.DiffUnified,
		Inline:          mygoeditor.DiffInlineWordAlt,
		ContextLines:    3,
		ExpandUnchanged: true,
		FitContent:      true,
		HunkActions:     true,
		LineSelection:   true,
	}
	em.ApplyDiffTheme(d)
	em.patchDiffs[key] = d
	return d
}

// GetOrCreatePatchDiff builds or returns the cached diff for a parsed FilePatch.
func (em *EditorManager) GetOrCreatePatchDiff(key string, patch mygoeditor.FilePatch, tm *theme.ThemeManager) *mygoeditor.DiffState {
	em.mu.Lock()
	defer em.mu.Unlock()
	if tm != nil {
		em.Theme = tm
	}
	if d, ok := em.patchDiffs[key]; ok {
		return d
	}
	d := &mygoeditor.DiffState{
		Layout:        mygoeditor.DiffLayout(em.DiffLayout),
		HunkActions:   true,
		LineSelection: true,
		FitContent:    true,
	}
	d.SetPatch(patch)
	em.ApplyDiffTheme(d)
	em.patchDiffs[key] = d
	return d
}

// GetOrCreateReadOnlyEditor builds or returns a cached read-only syntax highlighted editor state.
func (em *EditorManager) GetOrCreateReadOnlyEditor(key, path, content string, tm *theme.ThemeManager) *mygoeditor.State {
	em.mu.Lock()
	defer em.mu.Unlock()
	if em.readOnlyStates == nil {
		em.readOnlyStates = make(map[string]*mygoeditor.State)
	}
	if st, ok := em.readOnlyStates[key]; ok {
		return st
	}
	buf := mygoeditor.NewBuffer(content)
	buf.SetLanguage(mygoeditor.LanguageFor(filepath.Base(path)))
	st := &mygoeditor.State{
		Buffer:     buf,
		ReadOnly:   true,
		FitContent: true,
		LineHeight: 1.45,
	}
	em.ApplyTheme(st)
	em.readOnlyStates[key] = st
	return st
}

// CloseTab removes an open tab.
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
	delete(em.docs, fullPath)
	delete(em.patchDiffs, fullPath)

	if em.ActivePath == fullPath {
		if len(em.OpenTabPaths) > 0 {
			em.ActivePath = em.OpenTabPaths[len(em.OpenTabPaths)-1]
		} else {
			em.ActivePath = ""
		}
	}
}

// CloseAll closes all open tabs and clears buffers.
func (em *EditorManager) CloseAll() {
	em.mu.Lock()
	defer em.mu.Unlock()
	kept := make([]string, 0, len(em.OpenTabPaths))
	for _, p := range em.OpenTabPaths {
		if em.PinnedTabs[p] {
			kept = append(kept, p) // pinned tabs survive Close All
			continue
		}
		delete(em.docs, p)
		delete(em.patchDiffs, p)
	}
	em.OpenTabPaths = kept
	if !em.containsPath(em.ActivePath) {
		if len(em.OpenTabPaths) > 0 {
			em.ActivePath = em.OpenTabPaths[len(em.OpenTabPaths)-1]
		} else {
			em.ActivePath = ""
		}
	}
}

func (em *EditorManager) containsPath(p string) bool {
	for _, t := range em.OpenTabPaths {
		if t == p {
			return true
		}
	}
	return false
}

// CloseOthers closes all tabs except keepPath.
func (em *EditorManager) CloseOthers(keepPath string) {
	em.mu.Lock()
	defer em.mu.Unlock()

	kept := make([]string, 0, len(em.OpenTabPaths))
	for _, p := range em.OpenTabPaths {
		if p == keepPath || em.PinnedTabs[p] {
			kept = append(kept, p) // keepPath + pinned tabs stay
			continue
		}
		delete(em.docs, p)
		delete(em.patchDiffs, p)
	}
	em.OpenTabPaths = kept
	if !em.containsPath(em.ActivePath) {
		em.ActivePath = keepPath
	}
}

// CloseToRight closes all tabs to the right of basePath.
func (em *EditorManager) CloseToRight(basePath string) {
	em.mu.Lock()
	defer em.mu.Unlock()

	idx := -1
	for i, p := range em.OpenTabPaths {
		if p == basePath {
			idx = i
			break
		}
	}
	if idx < 0 || idx == len(em.OpenTabPaths)-1 {
		return
	}

	kept := make([]string, 0, len(em.OpenTabPaths))
	for i, p := range em.OpenTabPaths {
		if i <= idx || em.PinnedTabs[p] {
			kept = append(kept, p)
			continue
		}
		delete(em.docs, p)
		delete(em.patchDiffs, p)
	}
	em.OpenTabPaths = kept
	if !em.containsPath(em.ActivePath) {
		em.ActivePath = basePath
	}
}

// CycleTab moves tab focus left or right.
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

// GetPatchDiff returns an open diff state or nil.
func (em *EditorManager) GetPatchDiff(path string) *mygoeditor.DiffState {
	em.mu.RLock()
	defer em.mu.RUnlock()
	return em.patchDiffs[path]
}

// OpenTabs returns a copy of open tab paths.
func (em *EditorManager) OpenTabs() []string {
	em.mu.RLock()
	defer em.mu.RUnlock()
	tabs := make([]string, len(em.OpenTabPaths))
	copy(tabs, em.OpenTabPaths)
	return tabs
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
	if strings.HasPrefix(doc.FullPath, "ssh://") && em.RemoteFS != nil {
		isRemote, err := em.RemoteFS.WriteFile(doc.FullPath, []byte(newText))
		if isRemote && err != nil {
			em.StatusMsg = fmt.Sprintf("Error saving remote %s: %v", doc.RelativePath, err)
			return err
		}
	} else {
		if err := os.WriteFile(doc.FullPath, []byte(newText), 0644); err != nil {
			em.StatusMsg = fmt.Sprintf("Error saving %s: %v", doc.RelativePath, err)
			return err
		}
	}

	doc.DiskContent = newText
	newOrig := mygoeditor.NewBuffer(newText)
	lang := mygoeditor.LanguageFor(filepath.Base(doc.FullPath))
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
	st.OnCompletion = func(r mygoeditor.CompletionRequest) {
		st.ShowCompletions(r.Offset-len(r.Prefix), WordCompletions(st.Buffer, r.Prefix))
	}

	st.OnHover = func(r mygoeditor.HoverRequest) {
		word, start, end := WordAtOffset(st.Buffer, r.Offset)
		if word == "" {
			return
		}
		count := strings.Count(st.Buffer.String(), word)
		st.ShowHover(start, end, fmt.Sprintf("Symbol '%s' (%d occurrences)", word, count), nil)
	}

	st.OnDefinition = func(offset int) {
		word, _, _ := WordAtOffset(st.Buffer, offset)
		if word == "" {
			return
		}
		matches := em.FindDefinitions(word, em.WorkspaceDir)
		if len(matches) == 1 {
			_, _ = em.OpenFile(matches[0].Path, filepath.Base(matches[0].Path))
			if active := em.ActiveDoc(); active != nil {
				active.Editor.GoTo(mygoeditor.Pos{Line: matches[0].Line, Col: matches[0].Col})
			}
		} else if len(matches) > 1 {
			if em.OnDefinitionRequested != nil {
				em.OnDefinitionRequested(word, matches)
			}
		}
	}
}

// PeekMatch represents a symbol definition found in workspace files.
type PeekMatch struct {
	Path    string `json:"path"`
	Line    int    `json:"line"`
	Col     int    `json:"col"`
	Preview string `json:"preview"`
	Symbol  string `json:"symbol"`
}

// FindDefinitions scans open files and workspace files for declarations matching the symbol.
func (em *EditorManager) FindDefinitions(symbol, wsDir string) []PeekMatch {
	if symbol == "" {
		return nil
	}
	var matches []PeekMatch

	// 1. Check open documents first
	em.mu.RLock()
	for p, doc := range em.docs {
		if doc == nil || doc.Editor == nil || doc.Editor.Buffer == nil {
			continue
		}
		lines := strings.Split(doc.Editor.Buffer.String(), "\n")
		for i, line := range lines {
			if isDeclaration(line, symbol) {
				col := strings.Index(line, symbol)
				if col < 0 {
					col = 0
				}
				matches = append(matches, PeekMatch{
					Path:    p,
					Line:    i + 1,
					Col:     col + 1,
					Preview: strings.TrimSpace(line),
					Symbol:  symbol,
				})
			}
		}
	}
	em.mu.RUnlock()

	// 2. If nothing found in open docs, search files in workspace dir
	if len(matches) == 0 && wsDir != "" {
		_ = filepath.WalkDir(wsDir, func(path string, d os.DirEntry, err error) error {
			if err != nil || d == nil {
				return nil
			}
			if d.IsDir() {
				if strings.HasPrefix(d.Name(), ".") || d.Name() == "node_modules" || d.Name() == "vendor" {
					return filepath.SkipDir
				}
				return nil
			}
			ext := filepath.Ext(path)
			if ext != ".go" && ext != ".ts" && ext != ".tsx" && ext != ".js" && ext != ".py" && ext != ".rs" && ext != ".json" {
				return nil
			}
			data, rErr := os.ReadFile(path)
			if rErr != nil {
				return nil
			}
			lines := strings.Split(string(data), "\n")
			for i, line := range lines {
				if isDeclaration(line, symbol) {
					col := strings.Index(line, symbol)
					if col < 0 {
						col = 0
					}
					matches = append(matches, PeekMatch{
						Path:    path,
						Line:    i + 1,
						Col:     col + 1,
						Preview: strings.TrimSpace(line),
						Symbol:  symbol,
					})
					if len(matches) >= 20 {
						return filepath.SkipAll
					}
				}
			}
			return nil
		})
	}

	return matches
}

func isDeclaration(line, symbol string) bool {
	patterns := []string{
		"func " + symbol,
		"type " + symbol,
		"var " + symbol,
		"const " + symbol,
		"class " + symbol,
		"interface " + symbol,
		"function " + symbol,
		"def " + symbol,
		"fn " + symbol,
		"let " + symbol + " =",
		"const " + symbol + " =",
	}
	for _, p := range patterns {
		if strings.Contains(line, p) {
			return true
		}
	}
	return false
}

// WordCompletions scans the buffer for words matching the prefix.
func WordCompletions(buf *mygoeditor.Buffer, prefix string) []mygoeditor.Completion {
	if prefix == "" {
		return nil
	}
	text := buf.String()
	words := make(map[string]bool)
	var current strings.Builder

	for _, r := range text {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			current.WriteRune(r)
		} else {
			if current.Len() > 0 {
				w := current.String()
				if strings.HasPrefix(w, prefix) && w != prefix && len(w) > len(prefix) {
					words[w] = true
				}
				current.Reset()
			}
		}
	}
	if current.Len() > 0 {
		w := current.String()
		if strings.HasPrefix(w, prefix) && w != prefix && len(w) > len(prefix) {
			words[w] = true
		}
	}

	var results []mygoeditor.Completion
	for w := range words {
		results = append(results, mygoeditor.Completion{
			Label:  w,
			Detail: "buffer word",
		})
	}
	return results
}

// WordAtOffset finds the identifier under the cursor offset.
func WordAtOffset(buf *mygoeditor.Buffer, offset int) (string, int, int) {
	text := buf.String()
	if offset < 0 || offset >= len(text) {
		return "", 0, 0
	}

	start := offset
	for start > 0 {
		r := rune(text[start-1])
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			start--
		} else {
			break
		}
	}

	end := offset
	for end < len(text) {
		r := rune(text[end])
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			end++
		} else {
			break
		}
	}

	if start == end {
		return "", 0, 0
	}
	return text[start:end], start, end
}

// SetSuggestion displays ghost-text prediction at offset in the document's editor state.
func (em *EditorManager) SetSuggestion(path string, offset int, text string) {
	em.mu.RLock()
	doc, ok := em.docs[path]
	em.mu.RUnlock()
	if ok && doc.Editor != nil {
		doc.Editor.SetSuggestion(offset, text)
	}
}

// SetInlays displays inline type and reference annotations in the document's editor state.
func (em *EditorManager) SetInlays(path string, inlays []mygoeditor.Inlay) {
	em.mu.RLock()
	doc, ok := em.docs[path]
	em.mu.RUnlock()
	if ok && doc.Editor != nil {
		doc.Editor.SetInlays(inlays)
	}
}
