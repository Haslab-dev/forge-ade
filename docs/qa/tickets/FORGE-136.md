# FORGE-136 — Editor split-view focus can bounce between panes via global active-tab sync

- **Type:** Bug · **Priority:** Low · **Status:** Open · **Component:** editor
- **Labels:** `code-verified`, `needs-repro`
- **Location:** `frontend/src/components/editor/EditorView.tsx:105-112` (global `activeTabId` follows the focused pane) + `:175` (`handlePaneTabSelect` also sets the global active tab; `openTab` listed as a dead dep).
- **Impact:** In split view, selecting a pane tab can trigger the sync effect and bounce focus between panes; needs a scripted repro.
