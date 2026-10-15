# FORGE-134 — `workspace.Save` mutates `Folders` in place; readers can observe relative paths (BUG-009)

- **Type:** Bug · **Priority:** Low · **Status:** Open · **Component:** go/workspace
- **Labels:** `unit-tested`, `skip-marked`
- **Location:** `internal/workspace/workspace.go:142-181` — under the write lock, `w.Folders` is swapped to relative paths for `yaml.Marshal` and restored after; mutex-obeying readers (`GetFolders`) are shielded, but direct-field readers (e.g. the bindings layer marshaling the struct) can observe `"./relative"` paths mid-save. Additionally `relFolders` is snapshotted in a separate RLock before the Lock upgrade (stale-read window).
- **Test:** skipped detector asserts the invariant "readers never observe non-absolute folder paths".
