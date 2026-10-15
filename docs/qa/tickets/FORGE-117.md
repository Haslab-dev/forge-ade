# FORGE-117 — Toast `error` kind renders default styling instead of rose (BUG-011)

- **Type:** Bug · **Priority:** Low · **Status:** Open · **Component:** ui/toast
- **Labels:** `unit-tested`, `skip-marked`
- **Location:** `frontend/src/lib/toast.tsx:21-27` — the kind normalizer handles `danger/warn/info/success`; `error` falls through to `default`, so the rose branch at `:48` (`item.kind === "error" || item.kind === "danger"`) is unreachable for `error`.
- **Test:** active test documents the sky styling; skipped test asserts rose — activates on fix.
