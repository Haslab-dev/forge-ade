# FORGE-132 — `editor.CheckSyntax` comment says subtract-1, code doesn't (BUG-007)

- **Type:** Bug · **Priority:** Low · **Status:** Open · **Component:** go/editor
- **Labels:** `unit-tested`, `needs-verification`
- **Location:** `internal/editor/editor.go:122-124` vs `:143-148`.
- **Impact:** Either the comment is stale or every esbuild diagnostic column is off by one in the editor gutter. Verify against a real esbuild output and fix whichever is wrong.
- **Test:** fake-esbuild test pins the current parse (Column passes through unchanged).
