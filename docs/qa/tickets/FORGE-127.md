# FORGE-127 — Agent failure UX: raw "Failed to fetch"; Go-harness error masked by fallback chain

- **Type:** Bug · **Priority:** Medium · **Status:** Open · **Component:** agent/ui
- **Labels:** `gui-verified`, `error-handling`
- **Steps:** With the configured provider unreachable (Ollama stopped) → send any prompt.
- **Actual:** Task is created; the message list renders `⚠️ **Error**: Failed to fetch` (the raw browser `TypeError` text); no retry affordance; the turn leaves no persisted session file under `~/.forge/sessions/general/`.
- **Expected:** A friendly, actionable error ("Provider at http://localhost:11434 is unreachable — check Model settings"), plus surfacing whichever layer actually failed. The current Go-harness→browser-LLM fallback (`sendGoAgentMessage` 2.5 s race + 30 s watchdog, `workspaceStore.tsx`) makes failures ambiguous by construction.
