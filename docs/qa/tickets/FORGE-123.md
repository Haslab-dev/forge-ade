# FORGE-123 — Agent file edits: tree marks file modified, tab shows clean

- **Type:** Bug · **Priority:** Low · **Status:** Open · **Component:** editor/store
- **Labels:** `code-verified`
- **Location:** `frontend/src/stores/workspaceStore.tsx:3464-3482` — `updateFileContent` (agent write path) writes disk and sets tab `content` **without** `isModified`, while the same helper marks `FileItem.isModified: true` in the tree.
- **Impact:** After an agent edit to an open file, the explorer shows a dirty indicator but the editor tab claims a clean state.
