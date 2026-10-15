# FORGE-113 — Editor Git side panel shows stale / wrong repo state

- **Type:** Bug · **Priority:** Medium · **Status:** Open · **Component:** git/side-panel
- **Labels:** `gui-verified`
- **Found by:** GUI automation + disk cross-check
- **Steps:** Editor mode → toggle side pane → Git tab (project label reads "forge-ade", which has modified + untracked files; the alternate candidate root `~/.forge-ade/general` is **not a git repo**).
- **Actual:** Panel shows branch `main`, "No changes detected", Stage All disabled.
- **Expected:** Either forge-ade's pending changes, or an explicit "no repository" empty state — never a confident "clean" for an unresolved root.
