# FORGE-112 — Workspace state inconsistent across agent / editor / settings surfaces

- **Type:** Bug · **Priority:** Medium · **Status:** Open · **Component:** workspace-store
- **Labels:** `gui-verified`, `state-consistency`
- **Found by:** GUI automation
- **Steps:** With agent mode showing workspace "General (~/.forge-ade/general)" → Switch to Editor → observe tree/header; open Settings → General.
- **Actual:** Editor tree: "No Folder Opened / Standalone Code Editor Mode" while the editor project header reads "forge-ade"; Settings shows "Workspace directory: **No workspace opened**"; header "Open workspace in Finder" is disabled (implies `activeWorkspacePath` is empty).
- **Expected:** One source of truth for the active workspace, consistently reflected (or a deliberate, explained difference between agent-workspace and editor-workspace).
