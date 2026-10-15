# FORGE-131 — `collectPatterns` domain off-by-one: nested `.gitignore` applies repo-wide

- **Type:** Bug · **Priority:** Medium · **Status:** Open · **Component:** go/gitignore
- **Labels:** `found-by-testing`
- **Location:** `internal/gitignore/gitignore.go:64` — `domain := parts[:i]` should be `parts[:i+1]` for the `.gitignore` at `parts[:i+1]`.
- **Impact:** `sub/.gitignore` patterns get an empty domain and match anywhere in the repo (verified: a `nested-ignore.txt` entry in `sub/.gitignore` also ignores a top-level file of the same name).
- **Test:** skip-marked detector `TestNestedGitignorePatternDoesNotLeakOutsideItsDir`.
