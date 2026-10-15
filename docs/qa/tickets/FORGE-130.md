# FORGE-130 — `gitignore.findRoot` misses `.git` **files** (worktrees / submodules) (BUG-008)

- **Type:** Bug · **Priority:** Medium · **Status:** Open · **Component:** go/gitignore
- **Labels:** `unit-tested`, `skip-marked`
- **Location:** `internal/gitignore/gitignore.go:37` — requires `info.IsDir()`; linked worktrees/submodules have a `.git` **file**.
- **Impact:** `Load` returns nil for those repos → gitignored files are treated as tracked (tree decoration, indexing).
- **Test:** `TestWorktreeGitFileIsDetected` (skipped detector).
