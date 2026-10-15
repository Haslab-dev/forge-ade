# FORGE-125 — Side-pane chooser "Terminal" click is a silent no-op without a session

- **Type:** Bug · **Priority:** Low · **Status:** Open · **Component:** side-panel
- **Labels:** `gui-verified`
- **Steps:** Agent home (no active task) → open right side pane → click **Terminal** in the "Choose a tab" chooser.
- **Actual:** Button gets `[active]` styling; the pane stays on the chooser; no empty-state, no error.
- **Expected:** Either create a terminal session, or show an explicit empty state ("No terminal session — start one from a task").
