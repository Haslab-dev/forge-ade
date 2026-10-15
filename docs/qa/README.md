# ForgeADE QA Board — v0.9.5 (Build 37) QA Cycle

> Jira-style tracking for the 2026-10-15 QA pass. Project key: **`FORGE`**.
> Detailed tickets live in [`tickets.md`](./tickets.md); plan in [`qa-plan.md`](./qa-plan.md); execution evidence in [`test-report.md`](./test-report.md).

## Epic

| Key | Summary | Status |
| --- | --- | --- |
| FORGE-100 | Quality: Test Coverage & Stabilization (v0.9.5 QA cycle) | In Progress |

## Sprint summary

| Type | Count |
| --- | --- |
| Stories delivered | 4 (FORGE-101…104) |
| Automated tests added | **160** (Go: 40 — 37 pass + 3 bug-detector skips; Frontend: 120 — 116 pass + 4 bug-detector skips) |
| Bugs found | **21** (2 High, 3 build-breaking/blocking, 9 Medium, 10 Low) |
| Hygiene / tech-debt tickets | 5 |

## Ticket index

### Test infrastructure (this cycle's deliverables)

| Key | T | P | Summary | Status |
| --- | --- | --- | --- | --- |
| FORGE-101 | Story | — | Frontend test infrastructure: vitest + jsdom + Testing Library | ✅ Done |
| FORGE-102 | Story | — | Frontend unit suites: lib, services, stores (9 files / 120 tests) | ✅ Done |
| FORGE-103 | Story | — | Backend unit tests for the 6 untested `internal/*` packages | ✅ Done |
| FORGE-104 | Story | — | GUI automation pass: agent mode, side panels, editor, settings | ✅ Done |
| FORGE-105 | Story | — | Wire `go test` + `vitest` + lint gates into CI | To Do |

### Bugs — frontend

| Key | T | P | Summary | Status |
| --- | --- | --- | --- | --- |
| FORGE-110 | Bug | High | "New Scratch File" silently does nothing (tab never inserted into `openTabs`) | Open |
| FORGE-111 | Bug | High | Bottom-dock terminal is orphaned — both toggle entry points are no-ops | Open |
| FORGE-112 | Bug | Medium | Workspace state inconsistent across agent / editor / settings surfaces | Open |
| FORGE-113 | Bug | Medium | Editor Git side panel shows stale/wrong repo state | Open |
| FORGE-114 | Bug | Medium | `sendAgentPrompt` stale closure — missing `useCallback` deps | Open |
| FORGE-115 | Bug | Low | Markdown highlighter group mismatch (inline code styled as comment) | Open |
| FORGE-116 | Bug | Low | `parseGitDecorations` keeps leading `(` when decoration starts with a space | Open |
| FORGE-117 | Bug | Low | Toast `error` kind renders default styling instead of rose | Open |
| FORGE-118 | Bug | Medium | `apiBridge` failure modes are invisible (readFile `''`, fake fallback tree, fake success) | Open |
| FORGE-119 | Bug | Medium | `agentEngine`: abort doesn't cancel HTTP; diff misreports deletions; shell interpolation | Open |
| FORGE-120 | Bug | Medium | `goAgentSession`: reversePatch has no stale-diff guard; preview rows removed by name | Open |
| FORGE-121 | Bug | Low | `time.ts` epoch heuristics false-positive; corrupt timestamps show "just now" | Open |
| FORGE-122 | Bug | Low | `ToastProvider` render-phase side effect + no timer cleanup on unmount | Open |
| FORGE-123 | Bug | Low | Agent file edits: tree marks file modified, tab shows clean | Open |
| FORGE-125 | Bug | Low | Side-pane chooser "Terminal" click is a silent no-op without a session | Open |
| FORGE-126 | Bug | Low | "Delete task" performs destructive delete without confirmation | Open |
| FORGE-127 | Bug | Medium | Agent failure UX: raw "Failed to fetch" shown; Go-harness error masked by fallback | Open |
| FORGE-136 | Bug | Low | Editor split-view focus can bounce between panes via global active-tab sync | Open |

### Bugs — backend

| Key | T | P | Summary | Status |
| --- | --- | --- | --- | --- |
| FORGE-130 | Bug | Medium | `gitignore.findRoot` misses `.git` **files** (worktrees / submodules) | Open |
| FORGE-131 | Bug | Medium | `collectPatterns` domain off-by-one — nested `.gitignore` applies repo-wide | Open |
| FORGE-132 | Bug | Low | `editor.CheckSyntax` comment says subtract-1, code doesn't — column suspect | Open |
| FORGE-133 | Bug | Low | `editor.Tooling` swallows every error silently (no user feedback) | Open |
| FORGE-134 | Bug | Low | `workspace.Save` mutates `Folders` in place — readers can observe relative paths | Open |
| FORGE-135 | Bug | High | `task build:server` / `run:server` cannot compile (`DefaultApplicationMenu` under `server` tag) | Open |

### Hygiene & follow-ups

| Key | T | P | Summary | Status |
| --- | --- | --- | --- | --- |
| FORGE-124 | Task | Low | 183 oxlint warnings baseline — add CI gate, fix exhaustive-deps class | Open |
| FORGE-128 | Story | Low | Untested GUI paths needing fixtures (@-mentions, slash commands, browser pane, SSH, DB viewer) | To Do |
| FORGE-137 | Task | Low | `useEditorStore.files` pins full file content in memory (documented landmine) | Open |
| FORGE-138 | Task | Low | Dead/duplicated code candidates (diff-view, LSPService, `isInvalidHardcodedPath`) | Open |

## Bug-ID cross-reference (used in test skip markers)

Skip-marked tests in the suites assert the **desired** behavior and are skipped until the bug is fixed — fixing the bug automatically activates the test.

| Legacy ID (in tests) | Ticket |
| --- | --- |
| BUG-002 | FORGE-115 |
| BUG-006 | FORGE-120 |
| BUG-007 | FORGE-132 |
| BUG-008 | FORGE-130 |
| BUG-009 | FORGE-134 |
| BUG-011 | FORGE-117 |
| BUG-012 | FORGE-133 |
| BUG-018 | FORGE-116 |

> Note: test files reference tickets as `FORGE-xxx` after this report was written; older checkouts may still show `BUG-xxx`.
