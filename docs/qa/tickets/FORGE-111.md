# FORGE-111 — Bottom-dock terminal is orphaned; both toggle entry points are no-ops

- **Type:** Bug · **Priority:** High · **Status:** Open · **Component:** terminal/shell
- **Labels:** `gui-verified`, `dead-feature`
- **Found by:** GUI automation + code review
- **Locations:** toggle state `frontend/src/stores/workspaceStore.tsx:2493`; header button `App.tsx:184`; command palette action `frontend/src/components/modals/CommandPaletteModal.tsx:260`; component `frontend/src/components/agent/BottomTerminal.tsx` (264 lines) — **never rendered anywhere**.
- **Steps:** Press the header **Toggle terminal** button (or run `View: Toggle Bottom Terminal` in ⌘K).
- **Actual:** Button gets `active` styling; no terminal dock appears (verified: no `.xterm` in DOM).
- **Expected:** A bottom dock terminal mounts.
- **Note:** "Toggle terminal" also exists in the task ⋯ menu (`WorkspaceHeader.tsx:418`) — same dead state.
