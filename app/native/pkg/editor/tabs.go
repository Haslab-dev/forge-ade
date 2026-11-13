package editor

// tabs.go adds the CodeEditorPane tab-management behaviors of the webview:
// pinned tabs (skipped by close actions, pin glyph on the tab), read-only
// tabs (italic title, buffer not editable), drag-to-reorder, and multi-pane
// splits (Panes + weights, split right / close pane).

import (
	"sort"

	mygoeditor "github.com/egoist/mygo/ui/editor"
)

// TogglePin pins/unpins a tab; pinned tabs survive Close Others/All/Left/
// Right, as forge-ade's pin action does.
func (em *EditorManager) TogglePin(fullPath string) {
	em.mu.Lock()
	defer em.mu.Unlock()
	if em.PinnedTabs == nil {
		em.PinnedTabs = make(map[string]bool)
	}
	if em.PinnedTabs[fullPath] {
		delete(em.PinnedTabs, fullPath)
	} else {
		em.PinnedTabs[fullPath] = true
	}
}

func (em *EditorManager) IsPinned(fullPath string) bool {
	em.mu.RLock()
	defer em.mu.RUnlock()
	return em.PinnedTabs[fullPath]
}

// ToggleReadOnly flips a tab's buffer editability; read-only tabs render an
// italic title, like the webview's read-only tab style.
func (em *EditorManager) ToggleReadOnly(fullPath string) {
	em.mu.Lock()
	defer em.mu.Unlock()
	if em.ReadOnlyTabs == nil {
		em.ReadOnlyTabs = make(map[string]bool)
	}
	now := !em.ReadOnlyTabs[fullPath]
	em.ReadOnlyTabs[fullPath] = now
	if doc, ok := em.docs[fullPath]; ok && doc != nil {
		doc.Editor.ReadOnly = now
	}
}

func (em *EditorManager) IsReadOnly(fullPath string) bool {
	em.mu.RLock()
	defer em.mu.RUnlock()
	return em.ReadOnlyTabs[fullPath]
}

// closeableLocked reports whether a tab may be closed by bulk actions
// (pinned tabs are protected).
func (em *EditorManager) closeableLocked(fullPath string) bool {
	return !em.PinnedTabs[fullPath]
}

func (em *EditorManager) containsLocked(p string) bool {
	for _, t := range em.OpenTabPaths {
		if t == p {
			return true
		}
	}
	return false
}

// MoveTab reorders tabs (drag-to-reorder and move-between-panes support).
func (em *EditorManager) MoveTab(from, to int) {
	em.mu.Lock()
	defer em.mu.Unlock()
	if from < 0 || from >= len(em.OpenTabPaths) || to < 0 || to >= len(em.OpenTabPaths) || from == to {
		return
	}
	tabs := append([]string(nil), em.OpenTabPaths...)
	moved := tabs[from]
	tabs = append(tabs[:from], tabs[from+1:]...)
	tabs = append(tabs[:to], append([]string{moved}, tabs[to:]...)...)
	em.OpenTabPaths = tabs
	// Pinned tabs bubble to the front, as pinned-tab behavior does.
	sort.SliceStable(tabs, func(i, j int) bool {
		pi := em.PinnedTabs[tabs[i]]
		pj := em.PinnedTabs[tabs[j]]
		if pi != pj {
			return pi
		}
		return false
	})
	em.OpenTabPaths = tabs
}

// TabIndex returns the position of a tab, -1 when absent.
func (em *EditorManager) TabIndex(fullPath string) int {
	em.mu.RLock()
	defer em.mu.RUnlock()
	for i, p := range em.OpenTabPaths {
		if p == fullPath {
			return i
		}
	}
	return -1
}

// ── Multi-pane splits (EditorPaneState parity) ─────────────────────────────

// SplitRight adds a pane to the right of the last pane holding the active
// document, as the Columns2 action does.
func (em *EditorManager) SplitRight() {
	em.mu.Lock()
	defer em.mu.Unlock()
	if em.ActivePath == "" {
		return
	}
	if em.Panes == nil {
		em.Panes = []string{em.ActivePath}
	} else if len(em.Panes) == 0 {
		em.Panes = []string{em.ActivePath}
	} else {
		em.Panes = append(em.Panes, em.ActivePath)
	}
	em.PaneWeights = nil // equal weights
	em.IsSplit = true
}

// ClosePane removes pane i; the pane's document stays open as a tab.
func (em *EditorManager) ClosePane(i int) {
	em.mu.Lock()
	defer em.mu.Unlock()
	if i < 0 || i >= len(em.Panes) {
		return
	}
	em.Panes = append(em.Panes[:i], em.Panes[i+1:]...)
	if len(em.Panes) <= 1 {
		em.Panes = nil
		em.PaneWeights = nil
		em.IsSplit = false
		return
	}
	if len(em.PaneWeights) == len(em.Panes)+1 {
		em.PaneWeights = append(em.PaneWeights[:i], em.PaneWeights[i+1:]...)
	}
}

// PaneDocs resolves the pane layout: nil when single-pane.
func (em *EditorManager) PaneDocs() []string {
	em.mu.RLock()
	defer em.mu.RUnlock()
	if len(em.Panes) <= 1 {
		return nil
	}
	return append([]string(nil), em.Panes...)
}

// PaneWeight resolves pane i's flex weight (default 1).
func (em *EditorManager) PaneWeight(i int) float32 {
	em.mu.RLock()
	defer em.mu.RUnlock()
	if i < 0 || i >= len(em.PaneWeights) {
		return 1
	}
	if em.PaneWeights[i] < 0.1 {
		return 0.1
	}
	return em.PaneWeights[i]
}

// SetPaneWeight adjusts a pane's weight (drag resize between panes).
func (em *EditorManager) SetPaneWeight(i int, w float32) {
	em.mu.Lock()
	defer em.mu.Unlock()
	if i < 0 || i >= len(em.PaneWeights) {
		return
	}
	em.PaneWeights[i] = w
}

// EnsurePanes seeds single-pane layout from the legacy IsSplit flag.
func (em *EditorManager) EnsurePanes() {
	em.mu.Lock()
	defer em.mu.Unlock()
	if em.IsSplit && len(em.Panes) == 0 && em.ActivePath != "" {
		secondary := ""
		for _, p := range em.OpenTabPaths {
			if p != em.ActivePath {
				secondary = p
				break
			}
		}
		em.Panes = []string{em.ActivePath, secondary}
	}
}

// applyReadOnlyLocked marks restored docs read-only when the tab flag says so.
func (em *EditorManager) applyReadOnlyLocked(doc *OpenDocument) {
	if em.ReadOnlyTabs[doc.FullPath] {
		doc.Editor.ReadOnly = true
	}
}

var _ = mygoeditor.Edit{} // keep editor import aligned with buffer edits
