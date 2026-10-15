# FORGE-124 — 183 oxlint warnings baseline

- **Type:** Task · **Priority:** Low · **Status:** Open · **Component:** frontend
- **Description:** `bun run lint` reports 183 warnings / 0 errors (notably `react-hooks/exhaustive-deps` — one of which is real: FORGE-114). Triage the classes, fix or inline-disable with reasons, then enforce `--max-warnings` in CI (FORGE-105).
