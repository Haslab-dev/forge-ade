package editor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	mygoeditor "github.com/egoist/mygo/ui/editor"
	"github.com/hasdev/forge-ade/app/native/pkg/theme"
	"github.com/hasdev/forge-ade/core/remotefs"
)

const maxDiffCacheEntries = 64

// MinimapBucket represents one vertical slice of a document's minimap.
type MinimapBucket struct {
	IndentRatio float32
	LengthRatio float32
}

// OpenDocument represents an open file buffer with its editor and diff states.
type OpenDocument struct {
	RelativePath string
	FullPath     string
	Original     *mygoeditor.Buffer
	Editor       *mygoeditor.State
	Diff         *mygoeditor.DiffState
	DiskContent  string

	minimapMu         sync.Mutex
	minimapReady      bool
	minimapVersion    uint64
	minimapMaxBuckets int
	minimapBuckets    []MinimapBucket
}

// MinimapSummary returns a cached compact representation of the buffer for minimap rendering.
func (d *OpenDocument) MinimapSummary(maxBuckets int) []MinimapBucket {
	if d == nil || d.Editor == nil || d.Editor.Buffer == nil || maxBuckets <= 0 {
		return nil
	}
	buf := d.Editor.Buffer
	ver := buf.Version()

	d.minimapMu.Lock()
	defer d.minimapMu.Unlock()
	if d.minimapReady && d.minimapVersion == ver && d.minimapMaxBuckets == maxBuckets {
		return d.minimapBuckets
	}

	total := buf.LineCount()
	if total <= 0 {
		d.minimapBuckets = nil
		d.minimapVersion = ver
		d.minimapMaxBuckets = maxBuckets
		d.minimapReady = true
		return nil
	}

	step := (total + maxBuckets - 1) / maxBuckets
	if step < 1 {
		step = 1
	}
	buckets := make([]MinimapBucket, 0, min(total, maxBuckets))
	for start := 0; start < total && len(buckets) < maxBuckets; start += step {
		end := start + step
		if end > total {
			end = total
		}
		var bestLen, bestIndent int
		for li := start; li < end; li++ {
			line := buf.Line(li)
			indent := 0
			for indent < len(line) && (line[indent] == ' ' || line[indent] == '\t') {
				indent++
			}
			contentLen := len(strings.TrimRight(line, " \t\r")) - indent
			if contentLen > bestLen {
				bestLen = contentLen
				bestIndent = indent
			}
		}
		if bestLen <= 0 {
			buckets = append(buckets, MinimapBucket{})
			continue
		}
		indentRatio := float32(bestIndent) / 40.0
		if indentRatio > 0.55 {
			indentRatio = 0.55
		}
		lenRatio := float32(bestLen) / 80.0
		if lenRatio < 0.08 {
			lenRatio = 0.08
		} else if lenRatio > 1.0 {
			lenRatio = 1.0
		}
		buckets = append(buckets, MinimapBucket{
			IndentRatio: indentRatio,
			LengthRatio: lenRatio,
		})
	}

	d.minimapBuckets = buckets
	d.minimapVersion = ver
	d.minimapMaxBuckets = maxBuckets
	d.minimapReady = true
	return d.minimapBuckets
}

// EditorManager manages all open file buffers, active tabs, split layouts, and editor settings.
type EditorManager struct {
	mu             sync.RWMutex
	docs           map[string]*OpenDocument
	patchDiffs     map[string]*mygoeditor.DiffState
	patchHashes    map[string]uint64
	patchOrder     []string
	readOnlyStates map[string]*mygoeditor.State
	readOnlyHashes map[string]uint64
	readOnlyOrder  []string
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
	ActivePane            int
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
		patchHashes:    make(map[string]uint64),
		readOnlyStates: make(map[string]*mygoeditor.State),
		readOnlyHashes: make(map[string]uint64),
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
	if len(em.Panes) > 1 {
		if em.ActivePane < 0 || em.ActivePane >= len(em.Panes) {
			em.ActivePane = 0
		}
		em.Panes[em.ActivePane] = fullPath
	}

	if doc, exists := em.docs[fullPath]; exists {
		em.applyReadOnlyLocked(doc)
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
	em.applyReadOnlyLocked(doc)

	em.docs[fullPath] = doc
	em.StatusMsg = fmt.Sprintf("Opened %s", relPath)
	return doc, nil
}

func fnv64a(parts ...string) uint64 {
	const (
		offset64 = 14695981039346656037
		prime64  = 1099511628211
	)
	var h uint64 = offset64
	for pi, s := range parts {
		if pi > 0 {
			h ^= 0xff
			h *= prime64
		}
		for i := 0; i < len(s); i++ {
			h ^= uint64(s[i])
			h *= prime64
		}
	}
	return h
}

func filePatchSignature(patch mygoeditor.FilePatch) uint64 {
	const (
		offset64 = 14695981039346656037
		prime64  = 1099511628211
	)
	h := fnv64a(patch.OldName, patch.NewName, patch.OldMode, patch.NewMode)
	h ^= uint64(len(patch.Hunks))
	h *= prime64
	for _, hunk := range patch.Hunks {
		h ^= uint64(hunk.OldStart) ^ (uint64(hunk.NewStart) << 16) ^ (uint64(hunk.OldLines) << 32) ^ (uint64(hunk.NewLines) << 48)
		h *= prime64
		for _, line := range hunk.Lines {
			for i := 0; i < len(line); i++ {
				h ^= uint64(line[i])
				h *= prime64
			}
			h ^= '\n'
			h *= prime64
		}
	}
	return h
}

func (em *EditorManager) storePatchDiffLocked(key string, sig uint64, d *mygoeditor.DiffState) {
	if em.patchDiffs == nil {
		em.patchDiffs = make(map[string]*mygoeditor.DiffState)
	}
	if em.patchHashes == nil {
		em.patchHashes = make(map[string]uint64)
	}
	if _, existed := em.patchDiffs[key]; !existed {
		for len(em.patchOrder) >= maxDiffCacheEntries {
			evict := em.patchOrder[0]
			em.patchOrder = em.patchOrder[1:]
			delete(em.patchDiffs, evict)
			delete(em.patchHashes, evict)
		}
		em.patchOrder = append(em.patchOrder, key)
	}
	em.patchDiffs[key] = d
	em.patchHashes[key] = sig
}

func (em *EditorManager) deletePatchDiffLocked(key string) {
	delete(em.patchDiffs, key)
	delete(em.patchHashes, key)
}

func (em *EditorManager) clearDiffCachesLocked() {
	em.patchDiffs = make(map[string]*mygoeditor.DiffState)
	em.patchHashes = make(map[string]uint64)
	em.patchOrder = nil
	em.readOnlyStates = make(map[string]*mygoeditor.State)
	em.readOnlyHashes = make(map[string]uint64)
	em.readOnlyOrder = nil
}

func (em *EditorManager) openPatchDiff(key, patch string, tm *theme.ThemeManager) *mygoeditor.DiffState {
	em.mu.Lock()
	defer em.mu.Unlock()
	if tm != nil {
		em.Theme = tm
	}
	sig := fnv64a(patch)
	if d, ok := em.patchDiffs[key]; ok && em.patchHashes[key] == sig {
		em.ApplyDiffTheme(d)
		d.Layout = mygoeditor.DiffLayout(em.DiffLayout)
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
	em.storePatchDiffLocked(key, sig, d)
	return d
}

// OpenPatchDiffKeyed builds or returns the cached diff for an arbitrary key
// (commit hash + path), so multiple commit diffs coexist.
func (em *EditorManager) OpenPatchDiffKeyed(key, patch string, tm *theme.ThemeManager) *mygoeditor.DiffState {
	return em.openPatchDiff(key, patch, tm)
}

// OpenPatchDiff parses and caches a patch diff view for the given file.
func (em *EditorManager) OpenPatchDiff(relPath, patch string, tm *theme.ThemeManager) *mygoeditor.DiffState {
	return em.openPatchDiff(relPath, patch, tm)
}

// GetOrCreateBufferDiff builds or returns the cached diff between old and new text buffers.
func (em *EditorManager) GetOrCreateBufferDiff(key, oldStr, newStr string, tm *theme.ThemeManager) *mygoeditor.DiffState {
	em.mu.Lock()
	defer em.mu.Unlock()
	if tm != nil {
		em.Theme = tm
	}
	sig := fnv64a(oldStr, newStr)
	if d, ok := em.patchDiffs[key]; ok && em.patchHashes[key] == sig {
		em.ApplyDiffTheme(d)
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
	em.storePatchDiffLocked(key, sig, d)
	return d
}

// GetOrCreatePatchDiff builds or returns the cached diff for a parsed FilePatch.
func (em *EditorManager) GetOrCreatePatchDiff(key string, patch mygoeditor.FilePatch, tm *theme.ThemeManager) *mygoeditor.DiffState {
	em.mu.Lock()
	defer em.mu.Unlock()
	if tm != nil {
		em.Theme = tm
	}
	sig := filePatchSignature(patch)
	if d, ok := em.patchDiffs[key]; ok && em.patchHashes[key] == sig {
		em.ApplyDiffTheme(d)
		d.Layout = mygoeditor.DiffLayout(em.DiffLayout)
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
	em.storePatchDiffLocked(key, sig, d)
	return d
}

// GetOrCreateReadOnlyEditor builds or returns a cached read-only syntax highlighted editor state.
func (em *EditorManager) GetOrCreateReadOnlyEditor(key, path, content string, tm *theme.ThemeManager) *mygoeditor.State {
	em.mu.Lock()
	defer em.mu.Unlock()
	if tm != nil {
		em.Theme = tm
	}
	if em.readOnlyStates == nil {
		em.readOnlyStates = make(map[string]*mygoeditor.State)
	}
	if em.readOnlyHashes == nil {
		em.readOnlyHashes = make(map[string]uint64)
	}
	sig := fnv64a(path, content)
	if st, ok := em.readOnlyStates[key]; ok && em.readOnlyHashes[key] == sig {
		em.ApplyTheme(st)
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
	if _, existed := em.readOnlyStates[key]; !existed {
		for len(em.readOnlyOrder) >= maxDiffCacheEntries {
			evict := em.readOnlyOrder[0]
			em.readOnlyOrder = em.readOnlyOrder[1:]
			delete(em.readOnlyStates, evict)
			delete(em.readOnlyHashes, evict)
		}
		em.readOnlyOrder = append(em.readOnlyOrder, key)
	}
	em.readOnlyStates[key] = st
	em.readOnlyHashes[key] = sig
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
	delete(em.PinnedTabs, fullPath)
	delete(em.ReadOnlyTabs, fullPath)
	em.deletePatchDiffLocked(fullPath)

	if em.ActivePath == fullPath {
		if len(em.OpenTabPaths) > 0 {
			em.ActivePath = em.OpenTabPaths[len(em.OpenTabPaths)-1]
		} else {
			em.ActivePath = ""
		}
	}

	if len(em.Panes) > 0 {
		used := make(map[string]bool, len(em.Panes))
		for _, p := range em.Panes {
			if p != "" && p != fullPath {
				used[p] = true
			}
		}
		keptPanes := make([]string, 0, len(em.Panes))
		keptWeights := make([]float32, 0, len(em.Panes))
		for i, p := range em.Panes {
			if p != fullPath {
				keptPanes = append(keptPanes, p)
				if i < len(em.PaneWeights) {
					keptWeights = append(keptWeights, em.PaneWeights[i])
				}
				continue
			}
			replacement := ""
			for _, candidate := range em.OpenTabPaths {
				if !used[candidate] {
					replacement = candidate
					used[candidate] = true
					break
				}
			}
			if replacement != "" {
				keptPanes = append(keptPanes, replacement)
				if i < len(em.PaneWeights) {
					keptWeights = append(keptWeights, em.PaneWeights[i])
				}
			}
		}
		if len(keptPanes) <= 1 {
			em.Panes = nil
			em.PaneWeights = nil
			em.IsSplit = false
			em.ActivePane = 0
		} else {
			em.Panes = keptPanes
			if len(keptWeights) == len(keptPanes) {
				em.PaneWeights = keptWeights
			} else {
				em.PaneWeights = nil
			}
			if em.ActivePane >= len(em.Panes) {
				em.ActivePane = len(em.Panes) - 1
			}
		}
	}
}

// RenameOpenTab updates tab/buffer/pane keys in place so unsaved edits survive renames.
func (em *EditorManager) RenameOpenTab(oldFull, newFull, newRel string) {
	em.mu.Lock()
	defer em.mu.Unlock()
	if oldFull == "" || newFull == "" || oldFull == newFull {
		return
	}
	for i, p := range em.OpenTabPaths {
		if p == oldFull {
			em.OpenTabPaths[i] = newFull
		}
	}
	if doc, ok := em.docs[oldFull]; ok && doc != nil {
		delete(em.docs, oldFull)
		doc.FullPath = newFull
		doc.RelativePath = newRel
		lang := mygoeditor.LanguageFor(filepath.Base(newFull))
		if doc.Original != nil {
			doc.Original.SetLanguage(lang)
		}
		if doc.Editor != nil && doc.Editor.Buffer != nil {
			doc.Editor.Buffer.SetLanguage(lang)
		}
		em.docs[newFull] = doc
	}
	if em.PinnedTabs[oldFull] {
		delete(em.PinnedTabs, oldFull)
		em.PinnedTabs[newFull] = true
	}
	if em.ReadOnlyTabs[oldFull] {
		delete(em.ReadOnlyTabs, oldFull)
		em.ReadOnlyTabs[newFull] = true
	}
	for i, p := range em.Panes {
		if p == oldFull {
			em.Panes[i] = newFull
		}
	}
	if em.ActivePath == oldFull {
		em.ActivePath = newFull
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
		delete(em.ReadOnlyTabs, p)
	}
	em.OpenTabPaths = kept
	em.clearDiffCachesLocked()
	if !em.containsPath(em.ActivePath) {
		if len(em.OpenTabPaths) > 0 {
			em.ActivePath = em.OpenTabPaths[len(em.OpenTabPaths)-1]
		} else {
			em.ActivePath = ""
		}
	}
	if len(em.OpenTabPaths) <= 1 {
		em.Panes = nil
		em.PaneWeights = nil
		em.IsSplit = false
		em.ActivePane = 0
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
		delete(em.ReadOnlyTabs, p)
		em.deletePatchDiffLocked(p)
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
		delete(em.ReadOnlyTabs, p)
		em.deletePatchDiffLocked(p)
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
			rel := filepath.Base(matches[0].Path)
			if em.WorkspaceDir != "" {
				if r, err := filepath.Rel(em.WorkspaceDir, matches[0].Path); err == nil && !strings.HasPrefix(r, "..") {
					rel = r
				}
			}
			_, _ = em.OpenFile(matches[0].Path, rel)
			if active := em.ActiveDoc(); active != nil {
				line := max(0, matches[0].Line-1)
				col := max(0, matches[0].Col-1)
				active.Editor.GoTo(mygoeditor.Pos{Line: line, Col: col})
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
	if buf == nil {
		return "", 0, 0
	}
	text := buf.String()
	if offset < 0 || offset > len(text) {
		return "", 0, 0
	}

	isIdent := func(r rune) bool {
		return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
	}

	// Align offset inside a UTF-8 rune boundary if needed.
	for offset > 0 && offset < len(text) && !utf8.RuneStart(text[offset]) {
		offset--
	}

	start := offset
	for start > 0 {
		r, size := utf8.DecodeLastRuneInString(text[:start])
		if r == utf8.RuneError && size <= 1 {
			break
		}
		if isIdent(r) {
			start -= size
		} else {
			break
		}
	}

	end := offset
	for end < len(text) {
		r, size := utf8.DecodeRuneInString(text[end:])
		if r == utf8.RuneError && size <= 1 {
			break
		}
		if isIdent(r) {
			end += size
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
