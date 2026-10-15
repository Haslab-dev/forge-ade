# FORGE-119 — `agentEngine`: abort doesn't cancel HTTP; diff misreports deletions; shell interpolation

- **Type:** Bug · **Priority:** Medium · **Status:** Open · **Component:** agent/services
- **Labels:** `code-verified`, `security`
- **Locations:**
  - `abort()` only flips a flag — no `AbortController`, the SSE fetch keeps running to completion (`agentEngine.ts:46-48`).
  - `generateDiff` forces `additions: Math.max(1, additions)` (`:88`) — pure deletions are misreported as having an addition; the naive index-aligned compare also overcounts.
  - Model-controlled patterns are interpolated into shell strings for grep/find tools (`:345,359`) — command-injection surface on the webview path.
  - Gemini API key is passed in the URL query string.
- **Note:** the Go harness is the primary path (`goAgentSession`); this is the fallback/side-chat engine (documented debt, `docs/ARCHITECTURE.md`).
