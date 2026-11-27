package editor

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	mygoeditor "github.com/egoist/mygo/ui/editor"
	"github.com/hasdev/forge-ade/app/native/pkg/theme"
	"github.com/hasdev/forge-ade/core/remotefs"
)

const maxDiffCacheEntries = 64

// utf8BOM is stripped from file content at open and re-added on save so the
// byte-for-byte round trip preserves the original encoding.
const utf8BOM = "\uFEFF"

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
	// HasCRLF records the file's dominant line ending, detected from the raw
	// bytes at open; saves restore it (buffers hold \n internally).
	HasCRLF bool
	// HasBOM records a leading UTF-8 BOM stripped at open; saves re-add it.
	HasBOM bool
	// ModTime is the on-disk mtime at open (refreshed after each save) so the
	// save path can detect external modifications.
	ModTime time.Time

	minimapMu         sync.Mutex
	minimapReady      bool
	minimapVersion    uint64
	minimapMaxBuckets int
	minimapBuckets    []MinimapBucket
}

// detectCRLF reports whether CRLF is the file's dominant line ending:
// CRLF wins when \r\n occurs more often than bare \n. Mixed files default
// to LF, as most editors do.
func detectCRLF(raw []byte) bool {
	crlf := bytes.Count(raw, []byte("\r\n"))
	lf := bytes.Count(raw, []byte("\n"))
	return crlf > lf-crlf
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

	// Discard-confirmation contract (UI wave wires the modal):
	//
	//   ConfirmBeforeDiscard turns the close paths into Request* checks: when
	//   a target buffer is Modified, the close is staged in PendingDiscard
	//   and OnConfirmDiscard is invoked instead of closing.
	//   OnConfirmDiscard receives the dirty paths and a decide callback —
	//   call ResolveDiscard(true) to force-close them, ResolveDiscard(false)
	//   to keep them open.
	ConfirmBeforeDiscard bool
	OnConfirmDiscard     func(paths []string, decide func(discard bool))
	PendingDiscard       []string

	// OnConfirmOverwrite is consulted before a save clobbers a file that
	// changed on disk since it was opened (mtime mismatch). nil → proceed.
	OnConfirmOverwrite func(path string, decide func(proceed bool))
	pendingOverwrite   string

	// OnConfirmReplace is consulted before a workspace-wide Replace All
	// writes files; matchCount quotes how many matching lines were found.
	// nil → proceed without asking. (The RakitKode SearchManager is an
	// external module, so this replace-confirm contract lives here where
	// the UI layer can wire it.)
	OnConfirmReplace func(matchCount int, decide func(proceed bool))

	// OnSaveError surfaces save failures from internal paths (overwrite
	// resolution) so the UI layer can toast them; nil → StatusMsg only.
	OnSaveError func(path string, err error)

	// OnTabClosed fires after a tab was fully closed, including the bulk
	// close paths, so the UI layer can release per-file resources such as
	// PDF/deck webviews. nil → ignored.
	OnTabClosed func(path string)

	// OnStateChanged requests a redraw after async resolutions (discard /
	// overwrite decisions). nil → ignored.
	OnStateChanged func()

	// TerminalSessions binds editor terminal tabs (terminal://… paths) to
	// TerminalState tab IDs so each editor terminal tab owns a distinct PTY.
	TerminalSessions map[string]int
}

// NewEditorManager creates and initializes an EditorManager.
func NewEditorManager() *EditorManager {
	return &EditorManager{
		docs:             make(map[string]*OpenDocument),
		patchDiffs:       make(map[string]*mygoeditor.DiffState),
		patchHashes:      make(map[string]uint64),
		readOnlyStates:   make(map[string]*mygoeditor.State),
		readOnlyHashes:   make(map[string]uint64),
		OpenTabPaths:     make([]string, 0),
		SoftWrap:         false,
		IsSplit:          false,
		PinnedTabs:       make(map[string]bool),
		ReadOnlyTabs:     make(map[string]bool),
		IsDiffView:       false,
		DiffLayout:       0,
		DiffInline:       "Words",
		TerminalSessions: make(map[string]int),
	}
}

// fireTabClosed invokes OnTabClosed outside the manager lock.
func (em *EditorManager) fireTabClosed(path string) {
	if em.OnTabClosed != nil {
		em.OnTabClosed(path)
	}
}

// fireStateChanged invokes OnStateChanged (nil-safe redraw request).
func (em *EditorManager) fireStateChanged() {
	if em.OnStateChanged != nil {
		em.OnStateChanged()
	}
}

// BindTerminalSession records which TerminalState tab (by ID) backs an editor
// terminal tab, so closing or re-showing the tab targets the same PTY.
func (em *EditorManager) BindTerminalSession(tabPath string, sessionID int) {
	em.mu.Lock()
	defer em.mu.Unlock()
	if em.TerminalSessions == nil {
		em.TerminalSessions = make(map[string]int)
	}
	em.TerminalSessions[tabPath] = sessionID
}

// TerminalSessionFor returns the TerminalState tab ID bound to an editor
// terminal tab.
func (em *EditorManager) TerminalSessionFor(tabPath string) (int, bool) {
	em.mu.RLock()
	defer em.mu.RUnlock()
	id, ok := em.TerminalSessions[tabPath]
	return id, ok
}

// TerminalSessionCount returns how many editor terminal tabs are currently tracked.
func (em *EditorManager) TerminalSessionCount() int {
	em.mu.RLock()
	defer em.mu.RUnlock()
	return len(em.TerminalSessions)
}

func (em *EditorManager) unbindTerminalLocked(tabPath string) {
	delete(em.TerminalSessions, tabPath)
}

// WorkspaceEditorSnapshot preserves open tabs, layout, and terminal bindings for a workspace.
type WorkspaceEditorSnapshot struct {
	OpenTabPaths     []string
	ActivePath       string
	PinnedTabs       map[string]bool
	ReadOnlyTabs     map[string]bool
	Panes            []string
	PaneWeights      []float32
	ActivePane       int
	TerminalSessions map[string]int
}

// TakeSnapshot creates an isolated snapshot of the current workspace editor tabs and layout.
func (em *EditorManager) TakeSnapshot() *WorkspaceEditorSnapshot {
	if em == nil {
		return nil
	}
	em.mu.RLock()
	defer em.mu.RUnlock()

	snap := &WorkspaceEditorSnapshot{
		OpenTabPaths:     append([]string{}, em.OpenTabPaths...),
		ActivePath:       em.ActivePath,
		PinnedTabs:       make(map[string]bool, len(em.PinnedTabs)),
		ReadOnlyTabs:     make(map[string]bool, len(em.ReadOnlyTabs)),
		Panes:            append([]string{}, em.Panes...),
		PaneWeights:      append([]float32{}, em.PaneWeights...),
		ActivePane:       em.ActivePane,
		TerminalSessions: make(map[string]int, len(em.TerminalSessions)),
	}
	for k, v := range em.PinnedTabs {
		snap.PinnedTabs[k] = v
	}
	for k, v := range em.ReadOnlyTabs {
		snap.ReadOnlyTabs[k] = v
	}
	for k, v := range em.TerminalSessions {
		snap.TerminalSessions[k] = v
	}
	return snap
}

// RestoreSnapshot restores open tabs, layout, and terminal bindings from a snapshot.
func (em *EditorManager) RestoreSnapshot(snap *WorkspaceEditorSnapshot) {
	if em == nil || snap == nil {
		return
	}
	em.mu.Lock()
	defer em.mu.Unlock()

	em.OpenTabPaths = append([]string{}, snap.OpenTabPaths...)
	em.ActivePath = snap.ActivePath
	em.PinnedTabs = make(map[string]bool, len(snap.PinnedTabs))
	for k, v := range snap.PinnedTabs {
		em.PinnedTabs[k] = v
	}
	em.ReadOnlyTabs = make(map[string]bool, len(snap.ReadOnlyTabs))
	for k, v := range snap.ReadOnlyTabs {
		em.ReadOnlyTabs[k] = v
	}
	em.Panes = append([]string{}, snap.Panes...)
	em.PaneWeights = append([]float32{}, snap.PaneWeights...)
	em.ActivePane = snap.ActivePane
	if em.TerminalSessions == nil {
		em.TerminalSessions = make(map[string]int)
	}
	for k, v := range snap.TerminalSessions {
		em.TerminalSessions[k] = v
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
	if strings.HasPrefix(fullPath, "ssh://") {
		if em.RemoteFS == nil {
			err = fmt.Errorf("remote filesystem not available")
		} else {
			data, isRemote, rErr := em.RemoteFS.ReadFile(fullPath)
			if isRemote {
				contentBytes, err = data, rErr
			} else {
				err = fmt.Errorf("remote host connection not found for %s", fullPath)
			}
		}
	} else {
		contentBytes, err = os.ReadFile(fullPath)
	}
	if err != nil {
		em.StatusMsg = fmt.Sprintf("Error reading %s: %v", relPath, err)
		return nil, err
	}

	// Preserve the file's own conventions: a leading UTF-8 BOM and the
	// dominant CRLF line ending are stripped from the buffers and restored
	// at save time.
	hasBOM := bytes.HasPrefix(contentBytes, []byte(utf8BOM))
	if hasBOM {
		contentBytes = contentBytes[len(utf8BOM):]
	}
	hasCRLF := detectCRLF(contentBytes)
	content := string(contentBytes)

	origBuf := mygoeditor.NewBuffer(content)
	editBuf := mygoeditor.NewBuffer(content)

	lang := mygoeditor.LanguageFor(filepath.Base(fullPath))
	origBuf.SetLanguage(lang)
	editBuf.SetLanguage(lang)
	lineEnding := "\n"
	if hasCRLF {
		lineEnding = "\r\n"
	}
	origBuf.SetLineEnding(lineEnding)
	editBuf.SetLineEnding(lineEnding)

	doc := &OpenDocument{
		RelativePath: relPath,
		FullPath:     fullPath,
		Original:     origBuf,
		DiskContent:  content,
		HasCRLF:      hasCRLF,
		HasBOM:       hasBOM,
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
	if !strings.HasPrefix(fullPath, "ssh://") {
		if info, err := os.Stat(fullPath); err == nil {
			doc.ModTime = info.ModTime()
		}
	}

	em.ApplyTheme(doc.Editor)
	em.ApplyDiffTheme(doc.Diff)
	em.setupEditorCallbacks(doc)
	em.applyReadOnlyLocked(doc)

	em.docs[fullPath] = doc
	em.StatusMsg = fmt.Sprintf("Opened %s", relPath)
	return doc, nil
}

// OpenTerminalTab opens or activates a terminal tab in the editor (CodeEditorPane parity).
func (em *EditorManager) OpenTerminalTab(title string) string {
	em.mu.Lock()
	defer em.mu.Unlock()

	termCount := 0
	for _, p := range em.OpenTabPaths {
		if strings.HasPrefix(p, "terminal://") {
			termCount++
		}
	}
	baseTitle := title
	if baseTitle == "" {
		if termCount > 0 {
			baseTitle = fmt.Sprintf("Terminal %d", termCount+1)
		} else {
			baseTitle = "Terminal"
		}
	}
	tabID := fmt.Sprintf("terminal://%s", baseTitle)
	if title != "" {
		counter := 2
		for {
			collision := false
			for _, p := range em.OpenTabPaths {
				if p == tabID {
					collision = true
					break
				}
			}
			if !collision {
				break
			}
			tabID = fmt.Sprintf("terminal://%s (%d)", baseTitle, counter)
			counter++
		}
	}

	found := false
	for _, p := range em.OpenTabPaths {
		if p == tabID {
			found = true
			break
		}
	}
	if !found {
		em.OpenTabPaths = append(em.OpenTabPaths, tabID)
	}

	em.ActivePath = tabID
	if len(em.Panes) > 1 {
		if em.ActivePane < 0 || em.ActivePane >= len(em.Panes) {
			em.ActivePane = 0
		}
		em.Panes[em.ActivePane] = tabID
	}
	return tabID
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
	return em.GetOrCreateBufferDiffWithLang(key, "", oldStr, newStr, tm)
}

// GetOrCreateBufferDiffWithLang builds or returns the cached diff between old
// and new text buffers with syntax highlighting inferred from filePath.
func (em *EditorManager) GetOrCreateBufferDiffWithLang(key, filePath, oldStr, newStr string, tm *theme.ThemeManager) *mygoeditor.DiffState {
	em.mu.Lock()
	defer em.mu.Unlock()
	if tm != nil {
		em.Theme = tm
	}
	sig := fnv64a(filePath+"\x00"+oldStr, newStr)
	if d, ok := em.patchDiffs[key]; ok && em.patchHashes[key] == sig {
		em.ApplyDiffTheme(d)
		return d
	}
	oldBuf := mygoeditor.NewBuffer(oldStr)
	newBuf := mygoeditor.NewBuffer(newStr)
	if filePath != "" {
		if lang := mygoeditor.LanguageFor(filepath.Base(filePath)); lang != nil {
			oldBuf.SetLanguage(lang)
			newBuf.SetLanguage(lang)
		}
	}
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

// CloseTab removes an open tab. When OnTabClosed is set it fires afterwards
// (outside the manager lock) so the UI layer can release per-file resources.
func (em *EditorManager) CloseTab(fullPath string) {
	em.mu.Lock()
	closed := em.closeTabLocked(fullPath)
	em.mu.Unlock()
	if closed {
		em.fireTabClosed(fullPath)
	}
}

// closeTabLocked removes the tab's state; reports whether it existed.
func (em *EditorManager) closeTabLocked(fullPath string) bool {
	found := false
	newTabs := make([]string, 0, len(em.OpenTabPaths))
	for _, p := range em.OpenTabPaths {
		if p != fullPath {
			newTabs = append(newTabs, p)
		} else {
			found = true
		}
	}
	em.OpenTabPaths = newTabs
	delete(em.docs, fullPath)
	delete(em.PinnedTabs, fullPath)
	delete(em.ReadOnlyTabs, fullPath)
	em.deletePatchDiffLocked(fullPath)
	em.unbindTerminalLocked(fullPath)

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
	return found
}

// ── Close-with-confirmation contract ───────────────────────────────────────
//
// The Request* methods are the UI-facing close entry points: when
// ConfirmBeforeDiscard is set and any target buffer has unsaved changes, the
// close is staged in PendingDiscard and OnConfirmDiscard fires instead, and
// ResolveDiscard completes or cancels it.

// RequestClose closes one tab, asking before discarding unsaved changes.
func (em *EditorManager) RequestClose(path string) {
	if em.requestDiscardConfirm([]string{path}) {
		return
	}
	em.CloseTab(path)
}

// RequestCloseOthers closes every tab except keepPath (pinned tabs stay),
// asking before discarding unsaved changes.
func (em *EditorManager) RequestCloseOthers(keepPath string) {
	tabs := em.OpenTabs()
	targets := make([]string, 0, len(tabs))
	for _, p := range tabs {
		if p != keepPath && em.closeable(p) {
			targets = append(targets, p)
		}
	}
	if em.requestDiscardConfirm(targets) {
		return
	}
	em.CloseOthers(keepPath)
}

// RequestCloseAll closes every unpinned tab, asking before discarding
// unsaved changes.
func (em *EditorManager) RequestCloseAll() {
	tabs := em.OpenTabs()
	targets := make([]string, 0, len(tabs))
	for _, p := range tabs {
		if em.closeable(p) {
			targets = append(targets, p)
		}
	}
	if em.requestDiscardConfirm(targets) {
		return
	}
	em.CloseAll()
}

// RequestCloseLeft closes every tab left of basePath (pinned tabs stay),
// asking before discarding unsaved changes.
func (em *EditorManager) RequestCloseLeft(basePath string) {
	tabs := em.OpenTabs()
	targets := make([]string, 0, len(tabs))
	seen := false
	for _, p := range tabs {
		if p == basePath {
			seen = true
			continue
		}
		if !seen && em.closeable(p) {
			targets = append(targets, p)
		}
	}
	if em.requestDiscardConfirm(targets) {
		return
	}
	for _, p := range targets {
		em.CloseTab(p)
	}
}

// RequestCloseRight closes every tab right of basePath (pinned tabs stay),
// asking before discarding unsaved changes.
func (em *EditorManager) RequestCloseRight(basePath string) {
	tabs := em.OpenTabs()
	targets := make([]string, 0, len(tabs))
	seen := false
	for _, p := range tabs {
		if p == basePath {
			seen = true
			continue
		}
		if seen && em.closeable(p) {
			targets = append(targets, p)
		}
	}
	if em.requestDiscardConfirm(targets) {
		return
	}
	em.CloseToRight(basePath)
}

// ResolveDiscard completes a pending discard confirmation: discard=true
// force-closes the pending paths (firing OnTabClosed per path), false keeps
// them open. Either way PendingDiscard clears and a redraw is requested.
func (em *EditorManager) ResolveDiscard(discard bool) {
	em.mu.Lock()
	pending := em.PendingDiscard
	em.PendingDiscard = nil
	em.mu.Unlock()
	if discard {
		for _, p := range pending {
			em.CloseTab(p)
		}
	}
	em.fireStateChanged()
}

// closeable reports whether a tab may be closed by bulk actions (pinned
// tabs are protected).
func (em *EditorManager) closeable(fullPath string) bool {
	em.mu.RLock()
	defer em.mu.RUnlock()
	return em.closeableLocked(fullPath)
}

// requestDiscardConfirm stages a discard confirmation when
// ConfirmBeforeDiscard is set and any target buffer is Modified: it merges
// the dirty paths into PendingDiscard and invokes OnConfirmDiscard
// (nil-checked). Clean targets close immediately, as they are safe; the
// reported true means the dirty subset waits for ResolveDiscard.
func (em *EditorManager) requestDiscardConfirm(targets []string) bool {
	if !em.ConfirmBeforeDiscard || len(targets) == 0 {
		return false
	}
	em.mu.Lock()
	dirty := em.collectDirtyLocked(targets)
	if len(dirty) == 0 {
		em.mu.Unlock()
		return false
	}
	merged := make([]string, len(em.PendingDiscard), len(em.PendingDiscard)+len(dirty))
	copy(merged, em.PendingDiscard)
	for _, p := range dirty {
		if !containsString(merged, p) {
			merged = append(merged, p)
		}
	}
	em.PendingDiscard = merged
	cb := em.OnConfirmDiscard
	em.mu.Unlock()

	for _, p := range targets {
		if !containsString(dirty, p) {
			em.CloseTab(p)
		}
	}
	if cb != nil {
		cb(dirty, em.ResolveDiscard)
	}
	em.fireStateChanged()
	return true
}

// collectDirtyLocked returns the target paths whose buffers have unsaved
// changes.
func (em *EditorManager) collectDirtyLocked(targets []string) []string {
	var dirty []string
	for _, p := range targets {
		if doc, ok := em.docs[p]; ok && doc != nil && doc.Editor != nil && doc.Editor.Buffer != nil && doc.Editor.Buffer.Modified() {
			dirty = append(dirty, p)
		}
	}
	return dirty
}

func containsString(list []string, s string) bool {
	for _, v := range list {
		if v == s {
			return true
		}
	}
	return false
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
	kept := make([]string, 0, len(em.OpenTabPaths))
	var removed []string
	for _, p := range em.OpenTabPaths {
		if em.PinnedTabs[p] {
			kept = append(kept, p) // pinned tabs survive Close All
			continue
		}
		removed = append(removed, p)
		delete(em.docs, p)
		delete(em.ReadOnlyTabs, p)
		em.unbindTerminalLocked(p)
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
	em.mu.Unlock()
	for _, p := range removed {
		em.fireTabClosed(p)
	}
}

// CloseAllForWorkspace closes every tab whose path is not under rootDir —
// including pinned tabs — for workspace switches; PinnedTabs surviving a
// previous workspace must not leak into the new one.
func (em *EditorManager) CloseAllForWorkspace(rootDir string) {
	em.mu.Lock()
	kept := make([]string, 0, len(em.OpenTabPaths))
	var removed []string
	for _, p := range em.OpenTabPaths {
		if pathUnderDir(p, rootDir) {
			kept = append(kept, p)
			continue
		}
		removed = append(removed, p)
		delete(em.docs, p)
		delete(em.PinnedTabs, p)
		delete(em.ReadOnlyTabs, p)
		em.deletePatchDiffLocked(p)
		em.unbindTerminalLocked(p)
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
	em.mu.Unlock()
	for _, p := range removed {
		em.fireTabClosed(p)
	}
}

// pathUnderDir reports whether path is inside dir (dir "" closes everything).
func pathUnderDir(path, dir string) bool {
	if dir == "" {
		return false
	}
	if strings.HasPrefix(dir, "ssh://") && strings.HasPrefix(path, "ssh://") {
		cleanDir := strings.TrimRight(dir, "/") + "/"
		return path == dir || strings.HasPrefix(path, cleanDir)
	}
	rel, err := filepath.Rel(dir, path)
	if err != nil {
		return false
	}
	return rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
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
	kept := make([]string, 0, len(em.OpenTabPaths))
	var removed []string
	for _, p := range em.OpenTabPaths {
		if p == keepPath || em.PinnedTabs[p] {
			kept = append(kept, p) // keepPath + pinned tabs stay
			continue
		}
		removed = append(removed, p)
		delete(em.docs, p)
		delete(em.ReadOnlyTabs, p)
		em.deletePatchDiffLocked(p)
		em.unbindTerminalLocked(p)
	}
	em.OpenTabPaths = kept
	if !em.containsPath(em.ActivePath) {
		em.ActivePath = keepPath
	}
	em.mu.Unlock()
	for _, p := range removed {
		em.fireTabClosed(p)
	}
}

// CloseToRight closes all tabs to the right of basePath.
func (em *EditorManager) CloseToRight(basePath string) {
	em.mu.Lock()
	idx := -1
	for i, p := range em.OpenTabPaths {
		if p == basePath {
			idx = i
			break
		}
	}
	if idx < 0 || idx == len(em.OpenTabPaths)-1 {
		em.mu.Unlock()
		return
	}

	kept := make([]string, 0, len(em.OpenTabPaths))
	var removed []string
	for i, p := range em.OpenTabPaths {
		if i <= idx || em.PinnedTabs[p] {
			kept = append(kept, p)
			continue
		}
		removed = append(removed, p)
		delete(em.docs, p)
		delete(em.ReadOnlyTabs, p)
		em.deletePatchDiffLocked(p)
		em.unbindTerminalLocked(p)
	}
	em.OpenTabPaths = kept
	if !em.containsPath(em.ActivePath) {
		em.ActivePath = basePath
	}
	em.mu.Unlock()
	for _, p := range removed {
		em.fireTabClosed(p)
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
// The saved-mark on the active buffer clears so Modified() turns false, and
// the file's line endings and BOM are restored before the bytes hit disk.
// When OnConfirmOverwrite is set and the file changed on disk since it was
// opened, the save is routed through that callback instead of proceeding.
func (em *EditorManager) SaveActiveFile() error {
	em.mu.Lock()
	doc := em.docs[em.ActivePath]
	if doc == nil {
		em.mu.Unlock()
		return fmt.Errorf("no active document to save")
	}

	// External-change conflict: the file changed on disk behind the buffer.
	// With a confirm hook, defer to it; without one, save anyway (previous
	// behavior).
	if em.OnConfirmOverwrite != nil && !strings.HasPrefix(doc.FullPath, "ssh://") && !doc.ModTime.IsZero() {
		if info, err := os.Stat(doc.FullPath); err == nil && !info.ModTime().Equal(doc.ModTime) {
			path := doc.FullPath
			em.pendingOverwrite = path
			cb := em.OnConfirmOverwrite
			em.mu.Unlock()
			cb(path, em.ResolveOverwrite)
			return nil
		}
	}

	err := em.saveDocLocked(doc, doc.Editor.Buffer.String())
	saveErrCb := em.OnSaveError
	savePath := doc.FullPath
	em.mu.Unlock()
	if err != nil && saveErrCb != nil {
		saveErrCb(savePath, err)
	}
	return err
}

// ResolveOverwrite completes a pending external-change confirmation:
// proceed=true adopts the on-disk state and saves over it, false leaves the
// buffer untouched. Either way the pending state clears and a redraw fires.
func (em *EditorManager) ResolveOverwrite(proceed bool) {
	em.mu.Lock()
	path := em.pendingOverwrite
	em.pendingOverwrite = ""
	doc := em.docs[path]
	if !proceed || path == "" || doc == nil {
		em.mu.Unlock()
		em.fireStateChanged()
		return
	}
	// Overwrite confirmed: adopt the current on-disk mtime as the baseline so
	// the conflict check passes and the save proceeds.
	if info, err := os.Stat(path); err == nil {
		doc.ModTime = info.ModTime()
	}
	err := em.saveDocLocked(doc, doc.Editor.Buffer.String())
	cb := em.OnSaveError
	em.mu.Unlock()
	if err != nil && cb != nil {
		cb(path, err)
	}
	em.fireStateChanged()
}

// SetDocTextAndSave applies newText to an open document and persists it
// (workspace Replace All keeps open buffers as the source of truth for their
// files, so editor state and disk agree afterwards).
func (em *EditorManager) SetDocTextAndSave(path, newText string) error {
	em.mu.Lock()
	defer em.mu.Unlock()
	doc := em.docs[path]
	if doc == nil {
		return fmt.Errorf("document %s is not open", path)
	}
	doc.Editor.Buffer.Set(newText)
	return em.saveDocLocked(doc, doc.Editor.Buffer.String())
}

// SaveAllDirty persists every modified open buffer (AutoSave). Clean buffers
// are skipped. Each save funnels through saveDocLocked so atomic writes,
// CRLF/BOM restoration, MarkSaved, and the diff-baseline rebuild apply
// uniformly. Save errors surface through OnSaveError when wired; the first
// error is also returned.
func (em *EditorManager) SaveAllDirty() (saved int, err error) {
	type saveFailure struct {
		path string
		err  error
	}
	var failures []saveFailure

	em.mu.Lock()
	for _, doc := range em.docs {
		if doc == nil || doc.Editor == nil || doc.Editor.Buffer == nil {
			continue
		}
		if !doc.Editor.Buffer.Modified() {
			continue
		}
		if saveErr := em.saveDocLocked(doc, doc.Editor.Buffer.String()); saveErr != nil {
			failures = append(failures, saveFailure{path: doc.FullPath, err: saveErr})
		} else {
			saved++
		}
	}
	em.mu.Unlock()

	if len(failures) > 0 {
		err = failures[0].err
		if cb := em.OnSaveError; cb != nil {
			for _, f := range failures {
				cb(f.path, f.err)
			}
		}
		em.fireStateChanged()
	}
	return saved, err
}

// saveDocLocked persists doc with the given (LF) buffer text: restores the
// file's CRLF/BOM conventions, writes atomically preserving the file mode,
// marks the buffer saved, rebuilds the diff baseline, and refreshes the
// stored mtime. Callers hold em.mu.
func (em *EditorManager) saveDocLocked(doc *OpenDocument, newText string) error {
	out := newText
	if doc.HasBOM {
		out = utf8BOM + out
	}
	if doc.HasCRLF {
		out = strings.ReplaceAll(out, "\n", "\r\n")
	}

	if strings.HasPrefix(doc.FullPath, "ssh://") {
		if em.RemoteFS == nil {
			err := fmt.Errorf("remote filesystem not available")
			em.StatusMsg = fmt.Sprintf("Error saving remote %s: %v", doc.RelativePath, err)
			return err
		}
		isRemote, err := em.RemoteFS.WriteFile(doc.FullPath, []byte(out))
		if isRemote && err != nil {
			em.StatusMsg = fmt.Sprintf("Error saving remote %s: %v", doc.RelativePath, err)
			return err
		}
		if !isRemote {
			err := fmt.Errorf("remote connection not found for %s", doc.FullPath)
			em.StatusMsg = fmt.Sprintf("Error saving remote %s: %v", doc.RelativePath, err)
			return err
		}
	} else {
		if err := writeFileAtomic(doc.FullPath, []byte(out)); err != nil {
			em.StatusMsg = fmt.Sprintf("Error saving %s: %v", doc.RelativePath, err)
			return err
		}
		if info, err := os.Stat(doc.FullPath); err == nil {
			doc.ModTime = info.ModTime()
		}
	}

	doc.DiskContent = newText
	// The write succeeded: the current buffer text is the saved text.
	doc.Editor.Buffer.MarkSaved()

	newOrig := mygoeditor.NewBuffer(newText)
	lang := mygoeditor.LanguageFor(filepath.Base(doc.FullPath))
	newOrig.SetLanguage(lang)
	lineEnding := "\n"
	if doc.HasCRLF {
		lineEnding = "\r\n"
	}
	newOrig.SetLineEnding(lineEnding)

	doc.Original = newOrig
	doc.Editor.DiffBase = newOrig
	doc.Diff.Old = newOrig
	em.StatusMsg = fmt.Sprintf("Saved %s", doc.RelativePath)
	return nil
}

// ReloadIfChangedOnDisk checks whether fullPath changed on disk (e.g. edited
// by a running agent) and reloads the clean buffer in place. Returns true if
// the buffer was updated.
func (em *EditorManager) ReloadIfChangedOnDisk(fullPath string) bool {
	if em == nil || fullPath == "" || strings.HasPrefix(fullPath, "ssh://") {
		return false
	}
	info, err := os.Stat(fullPath)
	if err != nil {
		return false
	}
	em.mu.Lock()
	doc, ok := em.docs[fullPath]
	if !ok || doc == nil || doc.Editor == nil || doc.Editor.Buffer == nil {
		em.mu.Unlock()
		return false
	}
	if doc.Editor.Buffer.Modified() || info.ModTime().Equal(doc.ModTime) {
		em.mu.Unlock()
		return false
	}
	em.mu.Unlock()

	raw, err := os.ReadFile(fullPath)
	if err != nil {
		return false
	}
	newText := string(raw)

	em.mu.Lock()
	defer em.mu.Unlock()
	doc, ok = em.docs[fullPath]
	if !ok || doc == nil || doc.Editor == nil || doc.Editor.Buffer == nil || doc.Editor.Buffer.Modified() {
		return false
	}
	doc.ModTime = info.ModTime()
	doc.DiskContent = newText
	doc.Editor.Buffer.Set(newText)
	doc.Editor.Buffer.MarkSaved()
	return true
}

// writeFileAtomic replaces path's content without clobbering its permissions:
// the existing mode is preserved, bytes land in a same-directory temp file,
// and a rename swaps it into place. The temp file is cleaned up on failure.
func writeFileAtomic(path string, data []byte) error {
	mode := os.FileMode(0644)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	tmp := fmt.Sprintf("%s.forge-tmp-%d", path, os.Getpid())
	if err := os.WriteFile(tmp, data, mode); err != nil {
		return err
	}
	if err := os.Rename(tmp, path); err != nil {
		os.Remove(tmp)
		return err
	}
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

// FindReferences scans open files and workspace files for usages and call sites of the symbol.
func (em *EditorManager) FindReferences(symbol, wsDir string) []PeekMatch {
	if symbol == "" {
		return nil
	}
	var matches []PeekMatch

	em.mu.RLock()
	for p, doc := range em.docs {
		if doc == nil || doc.Editor == nil || doc.Editor.Buffer == nil {
			continue
		}
		lines := strings.Split(doc.Editor.Buffer.String(), "\n")
		for i, line := range lines {
			if strings.Contains(line, symbol) {
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
				if len(matches) >= 50 {
					break
				}
			}
		}
	}
	em.mu.RUnlock()

	if len(matches) < 50 && wsDir != "" {
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
				if strings.Contains(line, symbol) {
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
					if len(matches) >= 50 {
						return filepath.SkipAll
					}
				}
			}
			return nil
		})
	}

	return matches
}

// FindTypeDefinitions scans open files and workspace files for type declarations.
func (em *EditorManager) FindTypeDefinitions(symbol, wsDir string) []PeekMatch {
	if symbol == "" {
		return nil
	}
	var matches []PeekMatch

	em.mu.RLock()
	for p, doc := range em.docs {
		if doc == nil || doc.Editor == nil || doc.Editor.Buffer == nil {
			continue
		}
		lines := strings.Split(doc.Editor.Buffer.String(), "\n")
		for i, line := range lines {
			if isTypeDeclaration(line, symbol) {
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
			if ext != ".go" && ext != ".ts" && ext != ".tsx" && ext != ".js" && ext != ".py" && ext != ".rs" {
				return nil
			}
			data, rErr := os.ReadFile(path)
			if rErr != nil {
				return nil
			}
			lines := strings.Split(string(data), "\n")
			for i, line := range lines {
				if isTypeDeclaration(line, symbol) {
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

// FindImplementations scans open files and workspace files for implementations.
func (em *EditorManager) FindImplementations(symbol, wsDir string) []PeekMatch {
	if symbol == "" {
		return nil
	}
	var matches []PeekMatch

	em.mu.RLock()
	for p, doc := range em.docs {
		if doc == nil || doc.Editor == nil || doc.Editor.Buffer == nil {
			continue
		}
		lines := strings.Split(doc.Editor.Buffer.String(), "\n")
		for i, line := range lines {
			if isImplementation(line, symbol) {
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
			if ext != ".go" && ext != ".ts" && ext != ".tsx" && ext != ".js" && ext != ".py" && ext != ".rs" {
				return nil
			}
			data, rErr := os.ReadFile(path)
			if rErr != nil {
				return nil
			}
			lines := strings.Split(string(data), "\n")
			for i, line := range lines {
				if isImplementation(line, symbol) {
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

func isTypeDeclaration(line, symbol string) bool {
	patterns := []string{
		"type " + symbol,
		"interface " + symbol,
		"class " + symbol,
		"struct " + symbol,
		"enum " + symbol,
	}
	for _, p := range patterns {
		if strings.Contains(line, p) {
			return true
		}
	}
	return false
}

func isImplementation(line, symbol string) bool {
	if strings.Contains(line, "func (") && strings.Contains(line, ") "+symbol+"(") {
		return true
	}
	if strings.Contains(line, "implements ") && strings.Contains(line, symbol) {
		return true
	}
	if strings.Contains(line, "impl ") && strings.Contains(line, symbol) {
		return true
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
