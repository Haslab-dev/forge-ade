# FORGE-126 — "Delete task" performs destructive delete without confirmation

- **Type:** Bug · **Priority:** Low · **Status:** Open · **Component:** agent/ui
- **Labels:** `gui-verified`
- **Steps:** Active task → header ⋯ → **Delete task**.
- **Actual:** Task disappears immediately; no confirm dialog (verified `role="dialog"` absent).
- **Expected:** Confirm (or undo) for a destructive, hard-to-recover action.
