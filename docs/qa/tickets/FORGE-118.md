# FORGE-118 — `apiBridge` failure modes are invisible

- **Type:** Bug · **Priority:** Medium · **Status:** Open · **Component:** frontend/services
- **Labels:** `code-verified`
- **Locations & impact:**
  - `readFile()` returns `''` on error (`apiBridge.ts:377-397`) — indistinguishable from an empty file; review-diff reconstruction (`goAgentSession.buildDiffForTool`) shows an empty "original" instead of failing.
  - `readDirectoryTree(dirPath)` ignores `dirPath` for the primary call (`GetFileTree(-1)`, `:255-258`) — secondary roots get the wrong tree.
  - On total failure it returns a **hardcoded fake file tree** (`:310-346`) and there is a leftover personal-path check at `:231` (`!stored.includes('/Users/lutfiikbalmajid')`).
  - `saveSessionJsonl` returns `{ success: true }` even when both persistence paths failed (`:946`).
- **Fix sketch:** return typed errors / sentinels; remove fake data and dead personal path; propagate persistence failure.
