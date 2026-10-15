# FORGE-120 — `goAgentSession`: no stale-diff guard in `reversePatch`; preview rows removed by tool name (BUG-006)

- **Type:** Bug · **Priority:** Medium · **Status:** Open · **Component:** agent/services
- **Labels:** `unit-tested`, `skip-marked`
- **Locations:** `frontend/src/services/goAgentSession.ts:73-89` (splice without verifying `oldText` matches the content being replaced; out-of-range hunks silently skipped); `:147-152,176-184` (`agent:tool_start/end` drop the *first* streaming preview with the same tool name — wrong row for parallel same-named tools).
- **Impact:** A stale/shifted diff silently corrupts the reconstructed "original" shown in the review pane.
- **Test:** behavior pinned by unit tests; a skipped detector asserts a stale patch should throw.
