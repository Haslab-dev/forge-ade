# FORGE-133 — `editor.Tooling` swallows every error silently (BUG-012)

- **Type:** Bug · **Priority:** Low · **Status:** Open · **Component:** go/editor
- **Labels:** `unit-tested`
- **Locations:** `internal/editor/editor.go:96` (`CheckSyntax` silently no-ops when esbuild is missing), `:176-241` (`FormatCode` returns the original content **with nil error** on temp-file, exec, and read-back failures).
- **Impact:** "Format on save" can silently do nothing; syntax check silently unavailable — users get no signal.
- **Tests:** pin the silent-fallback contract; desired behavior (error or surfaced status) is a product decision to make.
