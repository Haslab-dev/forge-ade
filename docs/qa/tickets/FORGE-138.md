# FORGE-138 — Dead / duplicated code candidates

- **Type:** Task · **Priority:** Low · **Status:** Open · **Component:** frontend
- **Candidates:** `components/diff-view.tsx` (377 lines) vs `components/diff/DiffViewer.tsx` (857) — verify usage; `services/lspService.lintFile` JSON path duplicates the CodeEditorPane linter; `isInvalidHardcodedPath` (`workspaceStore.tsx:846`) always returns false; hardcoded fake file-tree fallback (tracked under FORGE-118).
