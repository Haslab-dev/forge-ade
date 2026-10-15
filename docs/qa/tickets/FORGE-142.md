# FORGE-142 — Manual compaction is a no-op for the model and invisible in the UI (user-reported)

- **Type:** Bug · **Priority:** Medium · **Status:** Open (diagnosed, fix proposed) · **Component:** agent/context
- **Labels:** `user-reported`, `code-verified`, `gui-verified`
- **Evidence:** `windowTranscriptWithSummary(sess.Messages, 40)` already windows every LLM request (agent.go:1491/:1413), so `CompactSession` (agent.go:789) never changes what the model receives for sessions ≤41 messages and barely changes it for longer ones. The "Context window usage" chip is driven by `sess.ContextTokens` (last request's provider-reported prompt size), which compaction neither resets nor re-measures — the number never drops. Clicking Compact with no active session is also a fully silent no-op (no toast, button not disabled).
- **Proposed fix:** reset `contextTokens` on compact; honest toast/disable when the auto-window already covers the transcript; unify on the Go summary (drop the frontend's duplicate local compaction).
