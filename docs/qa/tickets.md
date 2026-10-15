# Issue Index — ForgeADE QA backlog

Each ticket lives in its own file under [`tickets/`](./tickets/).
Format: `ID — Title` with type and status per entry.


## Epics

- [FORGE-100](tickets/FORGE-100.md) — Epic: Quality: Test Coverage & Stabilization (v0.9.5 QA cycle)

## Stories

- [FORGE-101](tickets/FORGE-101.md) — Frontend test infrastructure (vitest + jsdom + Testing Library)
- [FORGE-102](tickets/FORGE-102.md) — Frontend unit suites: lib, services, stores
- [FORGE-103](tickets/FORGE-103.md) — Backend unit tests for the 6 untested packages
- [FORGE-104](tickets/FORGE-104.md) — GUI automation pass: agent mode, side panels, editor
- [FORGE-105](tickets/FORGE-105.md) — Wire `go test` + `vitest` + lint gates into CI
- [FORGE-128](tickets/FORGE-128.md) — GUI paths that still need fixtures
- [FORGE-140](tickets/FORGE-140.md) — Agent sandbox workspace (shipped)
- [FORGE-145](tickets/FORGE-145.md) — Native computer-use tools (shipped)
- [FORGE-147](tickets/FORGE-147.md) — Mini-JEV: offline heuristic fallback router (shipped)

## Tasks

- [FORGE-124](tickets/FORGE-124.md) — 183 oxlint warnings baseline
- [FORGE-137](tickets/FORGE-137.md) — `useEditorStore.files` pins full file content in memory
- [FORGE-138](tickets/FORGE-138.md) — Dead / duplicated code candidates

## Bugs

- [FORGE-110](tickets/FORGE-110.md) — "New Scratch File" silently does nothing
- [FORGE-111](tickets/FORGE-111.md) — Bottom-dock terminal is orphaned; both toggle entry points are no-ops
- [FORGE-112](tickets/FORGE-112.md) — Workspace state inconsistent across agent / editor / settings surfaces
- [FORGE-113](tickets/FORGE-113.md) — Editor Git side panel shows stale / wrong repo state
- [FORGE-114](tickets/FORGE-114.md) — `sendAgentPrompt` stale closure: missing `useCallback` deps
- [FORGE-115](tickets/FORGE-115.md) — Markdown highlighter group mismatch (BUG-002)
- [FORGE-116](tickets/FORGE-116.md) — `parseGitDecorations` keeps leading `(` when decoration starts with a space (BUG-018)
- [FORGE-117](tickets/FORGE-117.md) — Toast `error` kind renders default styling instead of rose (BUG-011)
- [FORGE-118](tickets/FORGE-118.md) — `apiBridge` failure modes are invisible
- [FORGE-119](tickets/FORGE-119.md) — `agentEngine`: abort doesn't cancel HTTP; diff misreports deletions; shell interpolation
- [FORGE-120](tickets/FORGE-120.md) — `goAgentSession`: no stale-diff guard in `reversePatch`; preview rows removed by tool name (BUG-006)
- [FORGE-121](tickets/FORGE-121.md) — `time.ts` epoch heuristics false-positive; corrupt timestamps show "just now"
- [FORGE-122](tickets/FORGE-122.md) — `ToastProvider` render-phase side effect + no timer cleanup
- [FORGE-123](tickets/FORGE-123.md) — Agent file edits: tree marks file modified, tab shows clean
- [FORGE-125](tickets/FORGE-125.md) — Side-pane chooser "Terminal" click is a silent no-op without a session
- [FORGE-126](tickets/FORGE-126.md) — "Delete task" performs destructive delete without confirmation
- [FORGE-127](tickets/FORGE-127.md) — Agent failure UX: raw "Failed to fetch"; Go-harness error masked by fallback chain
- [FORGE-130](tickets/FORGE-130.md) — `gitignore.findRoot` misses `.git` **files** (worktrees / submodules) (BUG-008)
- [FORGE-131](tickets/FORGE-131.md) — `collectPatterns` domain off-by-one: nested `.gitignore` applies repo-wide
- [FORGE-132](tickets/FORGE-132.md) — `editor.CheckSyntax` comment says subtract-1, code doesn't (BUG-007)
- [FORGE-133](tickets/FORGE-133.md) — `editor.Tooling` swallows every error silently (BUG-012)
- [FORGE-134](tickets/FORGE-134.md) — `workspace.Save` mutates `Folders` in place; readers can observe relative paths (BUG-009)
- [FORGE-135](tickets/FORGE-135.md) — Server-mode build cannot compile (`task build:server` / `run:server` broken)
- [FORGE-136](tickets/FORGE-136.md) — Editor split-view focus can bounce between panes via global active-tab sync
- [FORGE-139](tickets/FORGE-139.md) — Side-pane tabs are destroyed by closing the pane (user-found, fixed)
- [FORGE-141](tickets/FORGE-141.md) — CLI agent sessions pollute recent projects with agent dot-dirs
- [FORGE-142](tickets/FORGE-142.md) — Manual compaction is a no-op for the model and invisible in the UI (user-reported)
- [FORGE-143](tickets/FORGE-143.md) — Model dropdown ignores Escape
- [FORGE-144](tickets/FORGE-144.md) — Agent turn ends silently empty when the provider drops tool calls
- [FORGE-146](tickets/FORGE-146.md) — Browser viewer viewport dropdown hidden behind the webview (fixed)
