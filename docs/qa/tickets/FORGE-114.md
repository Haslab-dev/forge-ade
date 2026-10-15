# FORGE-114 — `sendAgentPrompt` stale closure: missing `useCallback` deps

- **Type:** Bug · **Priority:** Medium · **Status:** Open · **Component:** agent/store
- **Labels:** `code-verified`, `flagged-by-oxlint`
- **Location:** `frontend/src/stores/workspaceStore.tsx:4306` (deps array); `createNewSession` used at `:4268` (defined `:4098`); `formatSmartSessionTitle` used at `:4293` (non-memoized, defined `:1160`).
- **Impact:** When the no-active-session branch runs, the captured `createNewSession` can be stale → the new session can be created with a stale model / workspace path. `formatSmartSessionTitle` closure is stale by definition (recreated every render, never in deps).
- **Fix sketch:** hoist `formatSmartSessionTitle` to module scope; add `createNewSession` to deps.
