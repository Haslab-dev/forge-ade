# Tickets — ForgeADE QA cycle 2026-10-15

Jira-style tickets for epic **FORGE-100**. Fields: `Type`, `Priority`, `Status`, `Component`, `Labels`, `Found by` (unit test / code review / GUI automation / build), and `Steps to Reproduce` where applicable.

---

## FORGE-100 — Epic: Quality: Test Coverage & Stabilization (v0.9.5 QA cycle)

- **Type:** Epic · **Status:** In Progress
- **Description:** Establish automated coverage for backend + frontend, exercise agent mode / side panels / editor end-to-end, and drive found defects to closure. Delivered FORGE-101…104 this cycle; 21 defects filed.

---

## Test infrastructure stories

### FORGE-101 — Frontend test infrastructure (vitest + jsdom + Testing Library)

- **Type:** Story · **Status:** ✅ Done · **Component:** frontend
- **Delivered:** dev deps `vitest`, `jsdom`, `@testing-library/{react,dom,jest-dom}`; `frontend/vitest.config.ts` (jsdom env, app `define` globals, `@` alias); `src/test/setup.ts` (jest-dom, `matchMedia` stub); scripts `bun run test` / `test:watch`.

### FORGE-102 — Frontend unit suites: lib, services, stores

- **Type:** Story · **Status:** ✅ Done · **Component:** frontend
- **Delivered:** 9 files / 120 tests (116 pass, 4 skip-marked bug detectors). Covers `fuzzy`, `time`, `utils` (parseFilePath/cleanRawToolTags/cleanPiBanner/formatDisplayTitle), `gitDecorations`, `terminalSettings`, `diff-highlight`, `toast`, `goAgentSession` (reversePatch/extractMentionedPaths/newThoughtStep), `sessionStore`.

### FORGE-103 — Backend unit tests for the 6 untested packages

- **Type:** Story · **Status:** ✅ Done · **Component:** go
- **Delivered:** 40 tests (37 pass + 3 skip-marked detectors) across `globalstore`, `ignore`, `gitignore`, `editor`, `commitmsg`, `workspace`. Includes a real-git integration test for `commitmsg.collectDiff` and fake-esbuild/fake-prettier harnesses for `editor`. Discovered FORGE-131 in the process.

### FORGE-104 — GUI automation pass: agent mode, side panels, editor

- **Type:** Story · **Status:** ✅ Done · **Component:** frontend/e2e
- **Delivered:** 16 executed scenarios (see `test-report.md` §3); 5 defects filed (FORGE-110, -111, -112, -113, -125), 1 UX defect (FORGE-126), 1 failure-path defect (FORGE-127). Recommend formalizing as scripted E2E under FORGE-105.

### FORGE-105 — Wire `go test` + `vitest` + lint gates into CI

- **Type:** Story · **Status:** To Do · **Component:** ci
- **Description:** Run `go build/vet/gofmt/test`, `bun run lint`, `bun run test`, `bun run build` on every PR. Today all of these are green but only run locally. Acceptance: red PR on regression; oxlint warnings budgeted (see FORGE-124).

---

## Bugs — frontend

### FORGE-110 — "New Scratch File" silently does nothing

- **Type:** Bug · **Priority:** High · **Status:** Open · **Component:** editor/filetree
- **Labels:** `gui-verified`, `silent-failure`
- **Found by:** GUI automation + code review
- **Location:** `frontend/src/components/editor/FileTree.tsx:1030` calls `openTab(...)`; `frontend/src/stores/workspaceStore.tsx:3104-3124`.
- **Steps:** Agent surface → Switch to Editor → (no folder state) → click **New Scratch File**.
- **Actual:** Nothing happens; pane keeps "No file open"; no console error.
- **Expected:** A `scratchpad.ts` tab opens with the starter comment.
- **Root cause:** `openTab()` only sets `activeTabId` + nav history; it never inserts the tab into `openTabs`. Every other caller (`openFileInEditor` at `workspaceStore.tsx:~3216`, `restoreNavTab` at `:3127`) does its own `setOpenTabs` first — the scratch handler is the only one that doesn't, so `activeTabId` points at a tab that doesn't exist.
- **Fix sketch:** in the scratch handler, prepend the tab to `openTabs` (or teach `openTab` to insert when the id is unknown).

### FORGE-111 — Bottom-dock terminal is orphaned; both toggle entry points are no-ops

- **Type:** Bug · **Priority:** High · **Status:** Open · **Component:** terminal/shell
- **Labels:** `gui-verified`, `dead-feature`
- **Found by:** GUI automation + code review
- **Locations:** toggle state `frontend/src/stores/workspaceStore.tsx:2493`; header button `App.tsx:184`; command palette action `frontend/src/components/modals/CommandPaletteModal.tsx:260`; component `frontend/src/components/agent/BottomTerminal.tsx` (264 lines) — **never rendered anywhere**.
- **Steps:** Press the header **Toggle terminal** button (or run `View: Toggle Bottom Terminal` in ⌘K).
- **Actual:** Button gets `active` styling; no terminal dock appears (verified: no `.xterm` in DOM).
- **Expected:** A bottom dock terminal mounts.
- **Note:** "Toggle terminal" also exists in the task ⋯ menu (`WorkspaceHeader.tsx:418`) — same dead state.

### FORGE-112 — Workspace state inconsistent across agent / editor / settings surfaces

- **Type:** Bug · **Priority:** Medium · **Status:** Open · **Component:** workspace-store
- **Labels:** `gui-verified`, `state-consistency`
- **Found by:** GUI automation
- **Steps:** With agent mode showing workspace "General (~/.forge-ade/general)" → Switch to Editor → observe tree/header; open Settings → General.
- **Actual:** Editor tree: "No Folder Opened / Standalone Code Editor Mode" while the editor project header reads "forge-ade"; Settings shows "Workspace directory: **No workspace opened**"; header "Open workspace in Finder" is disabled (implies `activeWorkspacePath` is empty).
- **Expected:** One source of truth for the active workspace, consistently reflected (or a deliberate, explained difference between agent-workspace and editor-workspace).

### FORGE-113 — Editor Git side panel shows stale / wrong repo state

- **Type:** Bug · **Priority:** Medium · **Status:** Open · **Component:** git/side-panel
- **Labels:** `gui-verified`
- **Found by:** GUI automation + disk cross-check
- **Steps:** Editor mode → toggle side pane → Git tab (project label reads "forge-ade", which has modified + untracked files; the alternate candidate root `~/.forge-ade/general` is **not a git repo**).
- **Actual:** Panel shows branch `main`, "No changes detected", Stage All disabled.
- **Expected:** Either forge-ade's pending changes, or an explicit "no repository" empty state — never a confident "clean" for an unresolved root.

### FORGE-114 — `sendAgentPrompt` stale closure: missing `useCallback` deps

- **Type:** Bug · **Priority:** Medium · **Status:** Open · **Component:** agent/store
- **Labels:** `code-verified`, `flagged-by-oxlint`
- **Location:** `frontend/src/stores/workspaceStore.tsx:4306` (deps array); `createNewSession` used at `:4268` (defined `:4098`); `formatSmartSessionTitle` used at `:4293` (non-memoized, defined `:1160`).
- **Impact:** When the no-active-session branch runs, the captured `createNewSession` can be stale → the new session can be created with a stale model / workspace path. `formatSmartSessionTitle` closure is stale by definition (recreated every render, never in deps).
- **Fix sketch:** hoist `formatSmartSessionTitle` to module scope; add `createNewSession` to deps.

### FORGE-115 — Markdown highlighter group mismatch (BUG-002)

- **Type:** Bug · **Priority:** Low · **Status:** Open · **Component:** diff-view
- **Labels:** `unit-tested`, `skip-marked`
- **Location:** `frontend/src/lib/diff-highlight.tsx:52,165-168` — `renderMd` feeds `` (`[^`]*`)|(\*\*[^*]+\*\*) `` into `scan()`, which destructures `(comment, str, num, kw)`.
- **Impact:** Inline code renders with the **comment** palette (muted italic); `**bold**` renders with the **string** palette.
- **Test:** active test pins current behavior; `it.skip` asserts the desired string styling — activates on fix.

### FORGE-116 — `parseGitDecorations` keeps leading `(` when decoration starts with a space (BUG-018)

- **Type:** Bug · **Priority:** Low · **Status:** Open · **Component:** git/ui
- **Labels:** `unit-tested`, `skip-marked`
- **Location:** `frontend/src/lib/gitDecorations.ts:5` — `replace(/^\(|\)$/g, '')` is anchored to offset 0, but git's `%d` emits `" (HEAD -> main, …)"` **with a leading space**.
- **Impact:** Branch chips can render as `(HEAD -> main`.
- **Test:** active test pins current behavior; skipped test asserts paren stripping — activates on fix.

### FORGE-117 — Toast `error` kind renders default styling instead of rose (BUG-011)

- **Type:** Bug · **Priority:** Low · **Status:** Open · **Component:** ui/toast
- **Labels:** `unit-tested`, `skip-marked`
- **Location:** `frontend/src/lib/toast.tsx:21-27` — the kind normalizer handles `danger/warn/info/success`; `error` falls through to `default`, so the rose branch at `:48` (`item.kind === "error" || item.kind === "danger"`) is unreachable for `error`.
- **Test:** active test documents the sky styling; skipped test asserts rose — activates on fix.

### FORGE-118 — `apiBridge` failure modes are invisible

- **Type:** Bug · **Priority:** Medium · **Status:** Open · **Component:** frontend/services
- **Labels:** `code-verified`
- **Locations & impact:**
  - `readFile()` returns `''` on error (`apiBridge.ts:377-397`) — indistinguishable from an empty file; review-diff reconstruction (`goAgentSession.buildDiffForTool`) shows an empty "original" instead of failing.
  - `readDirectoryTree(dirPath)` ignores `dirPath` for the primary call (`GetFileTree(-1)`, `:255-258`) — secondary roots get the wrong tree.
  - On total failure it returns a **hardcoded fake file tree** (`:310-346`) and there is a leftover personal-path check at `:231` (`!stored.includes('/Users/lutfiikbalmajid')`).
  - `saveSessionJsonl` returns `{ success: true }` even when both persistence paths failed (`:946`).
- **Fix sketch:** return typed errors / sentinels; remove fake data and dead personal path; propagate persistence failure.

### FORGE-119 — `agentEngine`: abort doesn't cancel HTTP; diff misreports deletions; shell interpolation

- **Type:** Bug · **Priority:** Medium · **Status:** Open · **Component:** agent/services
- **Labels:** `code-verified`, `security`
- **Locations:**
  - `abort()` only flips a flag — no `AbortController`, the SSE fetch keeps running to completion (`agentEngine.ts:46-48`).
  - `generateDiff` forces `additions: Math.max(1, additions)` (`:88`) — pure deletions are misreported as having an addition; the naive index-aligned compare also overcounts.
  - Model-controlled patterns are interpolated into shell strings for grep/find tools (`:345,359`) — command-injection surface on the webview path.
  - Gemini API key is passed in the URL query string.
- **Note:** the Go harness is the primary path (`goAgentSession`); this is the fallback/side-chat engine (documented debt, `docs/ARCHITECTURE.md`).

### FORGE-120 — `goAgentSession`: no stale-diff guard in `reversePatch`; preview rows removed by tool name (BUG-006)

- **Type:** Bug · **Priority:** Medium · **Status:** Open · **Component:** agent/services
- **Labels:** `unit-tested`, `skip-marked`
- **Locations:** `frontend/src/services/goAgentSession.ts:73-89` (splice without verifying `oldText` matches the content being replaced; out-of-range hunks silently skipped); `:147-152,176-184` (`agent:tool_start/end` drop the *first* streaming preview with the same tool name — wrong row for parallel same-named tools).
- **Impact:** A stale/shifted diff silently corrupts the reconstructed "original" shown in the review pane.
- **Test:** behavior pinned by unit tests; a skipped detector asserts a stale patch should throw.

### FORGE-121 — `time.ts` epoch heuristics false-positive; corrupt timestamps show "just now"

- **Type:** Bug · **Priority:** Low · **Status:** Open · **Component:** ui
- **Labels:** `unit-tested`
- **Location:** `frontend/src/lib/time.ts:15-18,47-55` — any 12–13-digit run in a string is treated as an epoch-ms (a 13-digit run from year 2001 passes the `>1e12` guard); sessions with unparseable timestamps always display "just now" (epoch 0), breaking sort/display expectations.
- **Tests:** pin current behavior, including the boundary (`999999999999` rejected).

### FORGE-122 — `ToastProvider` render-phase side effect + no timer cleanup

- **Type:** Bug · **Priority:** Low · **Status:** Open · **Component:** ui/toast
- **Labels:** `code-verified`
- **Location:** `frontend/src/lib/toast.tsx:36` — `moduleToast = toast;` assigned during render (StrictMode double-render / multi-provider races decide the winner); `:30-32` — the 3 s `setTimeout` is never cleared on unmount → setState-after-unmount window.
- **Unit test:** documents that the module escape hatch silently no-ops before any provider renders (by design today).

### FORGE-123 — Agent file edits: tree marks file modified, tab shows clean

- **Type:** Bug · **Priority:** Low · **Status:** Open · **Component:** editor/store
- **Labels:** `code-verified`
- **Location:** `frontend/src/stores/workspaceStore.tsx:3464-3482` — `updateFileContent` (agent write path) writes disk and sets tab `content` **without** `isModified`, while the same helper marks `FileItem.isModified: true` in the tree.
- **Impact:** After an agent edit to an open file, the explorer shows a dirty indicator but the editor tab claims a clean state.

### FORGE-125 — Side-pane chooser "Terminal" click is a silent no-op without a session

- **Type:** Bug · **Priority:** Low · **Status:** Open · **Component:** side-panel
- **Labels:** `gui-verified`
- **Steps:** Agent home (no active task) → open right side pane → click **Terminal** in the "Choose a tab" chooser.
- **Actual:** Button gets `[active]` styling; the pane stays on the chooser; no empty-state, no error.
- **Expected:** Either create a terminal session, or show an explicit empty state ("No terminal session — start one from a task").

### FORGE-126 — "Delete task" performs destructive delete without confirmation

- **Type:** Bug · **Priority:** Low · **Status:** Open · **Component:** agent/ui
- **Labels:** `gui-verified`
- **Steps:** Active task → header ⋯ → **Delete task**.
- **Actual:** Task disappears immediately; no confirm dialog (verified `role="dialog"` absent).
- **Expected:** Confirm (or undo) for a destructive, hard-to-recover action.

### FORGE-127 — Agent failure UX: raw "Failed to fetch"; Go-harness error masked by fallback chain

- **Type:** Bug · **Priority:** Medium · **Status:** Open · **Component:** agent/ui
- **Labels:** `gui-verified`, `error-handling`
- **Steps:** With the configured provider unreachable (Ollama stopped) → send any prompt.
- **Actual:** Task is created; the message list renders `⚠️ **Error**: Failed to fetch` (the raw browser `TypeError` text); no retry affordance; the turn leaves no persisted session file under `~/.forge/sessions/general/`.
- **Expected:** A friendly, actionable error ("Provider at http://localhost:11434 is unreachable — check Model settings"), plus surfacing whichever layer actually failed. The current Go-harness→browser-LLM fallback (`sendGoAgentMessage` 2.5 s race + 30 s watchdog, `workspaceStore.tsx`) makes failures ambiguous by construction.

### FORGE-136 — Editor split-view focus can bounce between panes via global active-tab sync

- **Type:** Bug · **Priority:** Low · **Status:** Open · **Component:** editor
- **Labels:** `code-verified`, `needs-repro`
- **Location:** `frontend/src/components/editor/EditorView.tsx:105-112` (global `activeTabId` follows the focused pane) + `:175` (`handlePaneTabSelect` also sets the global active tab; `openTab` listed as a dead dep).
- **Impact:** In split view, selecting a pane tab can trigger the sync effect and bounce focus between panes; needs a scripted repro.

---

## Bugs — backend

### FORGE-130 — `gitignore.findRoot` misses `.git` **files** (worktrees / submodules) (BUG-008)

- **Type:** Bug · **Priority:** Medium · **Status:** Open · **Component:** go/gitignore
- **Labels:** `unit-tested`, `skip-marked`
- **Location:** `internal/gitignore/gitignore.go:37` — requires `info.IsDir()`; linked worktrees/submodules have a `.git` **file**.
- **Impact:** `Load` returns nil for those repos → gitignored files are treated as tracked (tree decoration, indexing).
- **Test:** `TestWorktreeGitFileIsDetected` (skipped detector).

### FORGE-131 — `collectPatterns` domain off-by-one: nested `.gitignore` applies repo-wide

- **Type:** Bug · **Priority:** Medium · **Status:** Open · **Component:** go/gitignore
- **Labels:** `found-by-testing`
- **Location:** `internal/gitignore/gitignore.go:64` — `domain := parts[:i]` should be `parts[:i+1]` for the `.gitignore` at `parts[:i+1]`.
- **Impact:** `sub/.gitignore` patterns get an empty domain and match anywhere in the repo (verified: a `nested-ignore.txt` entry in `sub/.gitignore` also ignores a top-level file of the same name).
- **Test:** skip-marked detector `TestNestedGitignorePatternDoesNotLeakOutsideItsDir`.

### FORGE-132 — `editor.CheckSyntax` comment says subtract-1, code doesn't (BUG-007)

- **Type:** Bug · **Priority:** Low · **Status:** Open · **Component:** go/editor
- **Labels:** `unit-tested`, `needs-verification`
- **Location:** `internal/editor/editor.go:122-124` vs `:143-148`.
- **Impact:** Either the comment is stale or every esbuild diagnostic column is off by one in the editor gutter. Verify against a real esbuild output and fix whichever is wrong.
- **Test:** fake-esbuild test pins the current parse (Column passes through unchanged).

### FORGE-133 — `editor.Tooling` swallows every error silently (BUG-012)

- **Type:** Bug · **Priority:** Low · **Status:** Open · **Component:** go/editor
- **Labels:** `unit-tested`
- **Locations:** `internal/editor/editor.go:96` (`CheckSyntax` silently no-ops when esbuild is missing), `:176-241` (`FormatCode` returns the original content **with nil error** on temp-file, exec, and read-back failures).
- **Impact:** "Format on save" can silently do nothing; syntax check silently unavailable — users get no signal.
- **Tests:** pin the silent-fallback contract; desired behavior (error or surfaced status) is a product decision to make.

### FORGE-134 — `workspace.Save` mutates `Folders` in place; readers can observe relative paths (BUG-009)

- **Type:** Bug · **Priority:** Low · **Status:** Open · **Component:** go/workspace
- **Labels:** `unit-tested`, `skip-marked`
- **Location:** `internal/workspace/workspace.go:142-181` — under the write lock, `w.Folders` is swapped to relative paths for `yaml.Marshal` and restored after; mutex-obeying readers (`GetFolders`) are shielded, but direct-field readers (e.g. the bindings layer marshaling the struct) can observe `"./relative"` paths mid-save. Additionally `relFolders` is snapshotted in a separate RLock before the Lock upgrade (stale-read window).
- **Test:** skipped detector asserts the invariant "readers never observe non-absolute folder paths".

### FORGE-135 — Server-mode build cannot compile (`task build:server` / `run:server` broken)

- **Type:** Bug · **Priority:** High · **Status:** Open · **Component:** build
- **Labels:** `build-verified`
- **Repro:** `go build -tags server .` (also with `server,production`) →
  `./main.go:144:32: undefined: application.DefaultApplicationMenu`
- **Impact:** The Taskfile-advertised pure-HTTP server mode (and the Docker server image flow that depends on it) is unbuildable at HEAD.
- **Fix sketch:** guard the menu/window setup behind a build tag (e.g. `//go:build !server` variants or a runtime check), as the Wails `server` tag excludes GUI APIs.

---

## Hygiene & follow-ups

### FORGE-124 — 183 oxlint warnings baseline

- **Type:** Task · **Priority:** Low · **Status:** Open · **Component:** frontend
- **Description:** `bun run lint` reports 183 warnings / 0 errors (notably `react-hooks/exhaustive-deps` — one of which is real: FORGE-114). Triage the classes, fix or inline-disable with reasons, then enforce `--max-warnings` in CI (FORGE-105).

### FORGE-128 — GUI paths that still need fixtures

- **Type:** Story · **Priority:** Low · **Status:** To Do · **Component:** e2e
- **Description:** @-mention menu and /-command menu need an indexed workspace / custom commands; side-pane **Browser viewer** needs the in-app browser engine; SSH modal needs a reachable host; DB viewer needs sample DBs. Script these with fixtures once FORGE-105 lands.

### FORGE-137 — `useEditorStore.files` pins full file content in memory

- **Type:** Task · **Priority:** Low · **Status:** Open · **Component:** frontend/stores
- **Location:** `frontend/src/hooks/store.ts:61-66` (carries its own NOTE requesting an eviction path). Currently not the primary open-file holder — keep it that way or add eviction.

### FORGE-138 — Dead / duplicated code candidates

- **Type:** Task · **Priority:** Low · **Status:** Open · **Component:** frontend
- **Candidates:** `components/diff-view.tsx` (377 lines) vs `components/diff/DiffViewer.tsx` (857) — verify usage; `services/lspService.lintFile` JSON path duplicates the CodeEditorPane linter; `isInvalidHardcodedPath` (`workspaceStore.tsx:846`) always returns false; hardcoded fake file-tree fallback (tracked under FORGE-118).
