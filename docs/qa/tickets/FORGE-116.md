# FORGE-116 — `parseGitDecorations` keeps leading `(` when decoration starts with a space (BUG-018)

- **Type:** Bug · **Priority:** Low · **Status:** Open · **Component:** git/ui
- **Labels:** `unit-tested`, `skip-marked`
- **Location:** `frontend/src/lib/gitDecorations.ts:5` — `replace(/^\(|\)$/g, '')` is anchored to offset 0, but git's `%d` emits `" (HEAD -> main, …)"` **with a leading space**.
- **Impact:** Branch chips can render as `(HEAD -> main`.
- **Test:** active test pins current behavior; skipped test asserts paren stripping — activates on fix.
