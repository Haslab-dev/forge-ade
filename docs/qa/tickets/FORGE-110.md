# FORGE-110 — "New Scratch File" silently does nothing

- **Type:** Bug · **Priority:** High · **Status:** Open · **Component:** editor/filetree
- **Labels:** `gui-verified`, `silent-failure`
- **Found by:** GUI automation + code review
- **Location:** `frontend/src/components/editor/FileTree.tsx:1030` calls `openTab(...)`; `frontend/src/stores/workspaceStore.tsx:3104-3124`.
- **Steps:** Agent surface → Switch to Editor → (no folder state) → click **New Scratch File**.
- **Actual:** Nothing happens; pane keeps "No file open"; no console error.
- **Expected:** A `scratchpad.ts` tab opens with the starter comment.
- **Root cause:** `openTab()` only sets `activeTabId` + nav history; it never inserts the tab into `openTabs`. Every other caller (`openFileInEditor` at `workspaceStore.tsx:~3216`, `restoreNavTab` at `:3127`) does its own `setOpenTabs` first — the scratch handler is the only one that doesn't, so `activeTabId` points at a tab that doesn't exist.
- **Fix sketch:** in the scratch handler, prepend the tab to `openTabs` (or teach `openTab` to insert when the id is unknown).
