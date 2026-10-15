# Test Report — ForgeADE v0.9.5 (Build 37), cycle 2026-10-15

## 1. Baseline (before this cycle)

| Check | Result |
| --- | --- |
| `go build ./...` / `go vet ./...` / `gofmt -l .` | ✅ clean (linker warnings only, pre-existing) |
| `go test ./... -count=1` | ✅ 25 packages `ok`; 6 packages `[no test files]` (commitmsg, editor, gitignore, globalstore, ignore, workspace) |
| `bun run lint` (oxlint) | ⚠️ 183 warnings, 0 errors (baseline) |
| `bun run build` (tsc + vite) | ✅ clean |
| Frontend test suite | ❌ **none existed** |
| `go build -tags server` (server mode) | ❌ **fails to compile** → FORGE-135 |

## 2. Delivered tests

### Backend (FORGE-103) — `go test`

| Package | File | Tests | Notes |
| --- | --- | --- | --- |
| `internal/globalstore` | `globalstore_test.go` | 3 fns / 11 subtests | slug stability, dir creation |
| `internal/ignore` | `ignore_test.go` | 3 fns (table-driven) | canonical skip-dir set |
| `internal/gitignore` | `gitignore_test.go` | 6 fns / 2 subtests | pattern semantics incl. nested `.gitignore`; 2 skip-marked bug detectors (FORGE-130, FORGE-131) |
| `internal/editor` | `editor_test.go` | 10 fns / 9 subtests | fake esbuild/prettier on PATH; pins current parse (FORGE-132) & silent fallback (FORGE-133) |
| `internal/commitmsg` | `commitmsg_test.go` | 3 fns / 16 subtests | prompt build, scrub, real-git `collectDiff` integration |
| `internal/workspace` | `workspace_test.go` | 15 fns / 6 subtests | load/save round-trip, recents, manager; 1 skip-marked bug detector (FORGE-134) |

**Result: 37 passed, 3 skipped (bug detectors), 0 failed.** Full `go test ./internal/...`: 32 packages `ok`. `go vet` clean, `gofmt` clean.

### Frontend (FORGE-101/102) — `bun run test` (vitest + jsdom + Testing Library)

| Suite | File | Tests |
| --- | --- | --- |
| Fuzzy matcher | `src/lib/fuzzy.test.ts` | 12 |
| Relative time | `src/lib/time.test.ts` | 17 |
| Git decorations | `src/lib/gitDecorations.test.ts` | 6 (+1 skipped detector FORGE-116) |
| Terminal settings | `src/lib/terminalSettings.test.ts` | 5 |
| Utils (parseFilePath, tag/banner cleaning, titles) | `src/lib/utils.test.ts` | 24 |
| Diff/terminal highlighting | `src/lib/diff-highlight.test.tsx` | 15 (+1 skipped detector FORGE-115) |
| Toast system | `src/lib/toast.test.tsx` | 7 (+1 skipped detector FORGE-117) |
| Go agent session (reversePatch, mentions) | `src/services/goAgentSession.test.ts` | 15 (+1 skipped detector FORGE-120) |
| Session store (labels, defaults, modal state) | `src/stores/sessionStore.test.ts` | 8 |

**Result: 116 passed, 4 skipped (bug detectors), 0 failed** in 9 files. `tsc -b` + `vite build` still clean; lint unchanged from baseline (183 warnings / 0 errors — the new test files add none).

New dev tooling (FORGE-101): `vitest`, `jsdom`, `@testing-library/{react,dom,jest-dom}`, `frontend/vitest.config.ts`, `src/test/setup.ts`, scripts `test` / `test:watch`.

## 3. GUI automation results (FORGE-104)

Driven via a browser against `wails3 dev` (:9245, real Go bindings), DOM-snapshot driven, screenshots for layout.

| # | Scenario | Result | Evidence / tickets |
| --- | --- | --- | --- |
| 1 | App boot: sidebar, workspace list, agent home render | ✅ Pass | real sessions/workspaces loaded from Go store |
| 2 | Switch to Editor mode | ⚠️ Defects | tree "No Folder Opened" while header/label claim a workspace → FORGE-112 |
| 3 | **New Scratch File** | ❌ Fail | silent no-op, no error, no tab → **FORGE-110** |
| 4 | Right side pane toggle → Git review tab | ⚠️ Defect | renders, but stale/wrong repo state → FORGE-113 |
| 5 | Toggle terminal (header) | ❌ Fail | button goes active, nothing renders → **FORGE-111** |
| 6 | Command palette ⌘K, `> toggle` filter | ✅ Pass | commands listed (note: its "Toggle Bottom Terminal" action is a no-op → FORGE-111) |
| 7 | Composer typing, token counter, send gating | ✅ Pass | counter 0→3→14 tokens, send enables/disables correctly |
| 8 | Agent send → session → response | ❌ Fail | task created in sidebar + header, then raw `⚠️ Error: Failed to fetch`; no Go-harness error surfaced, no retry → FORGE-127 (Ollama down in env) |
| 9 | Task menu (rename/archive/delete/copy…) | ✅ Pass | all items present; delete executes — **no confirmation** → FORGE-126 |
| 10 | Delete my QA task | ✅ Pass | removed from list & header resets |
| 11 | Settings ⌘, | ⚠️ Defect | renders fully; "Workspace directory: No workspace opened" contradicts agent header → FORGE-112 |
| 12 | Theme toggle light/dark | ✅ Pass | `documentElement`/`body` classes flip, persisted |
| 13 | Automations modal | ✅ Pass | empty state + "Create automation" CTA |
| 14 | New Task modal ⌘N | ✅ Pass | internal agent + 6 CLI agents listed |
| 15 | Side-pane chooser → Terminal | ❌ Fail | button highlights, pane stays on chooser → FORGE-125 |
| 16 | @-mention & /-command menus | ➖ Inconclusive | need indexed files / custom commands → FORGE-128 |

**Screenshot review:** dark theme layout, spacing and typography of editor + Git drawer and Settings render cleanly; no broken styles observed (semantic token utilities in use).

## 4. Coverage movement

| Area | Before | After |
| --- | --- | --- |
| Go packages with tests | 25 / 31 | **31 / 31** |
| Frontend test files | 0 | 9 (+ infra) |
| Total automated checks | ~2,500 (Go only, pre-existing) | +160 new |
| Known-defect inventory | anecdotal | 21 ticketed, reproducible items |

## 5. Exit criteria check

- ✅ All new tests green; pre-existing suites unaffected (32 Go packages `ok`, frontend build/typecheck/lint unchanged).
- ✅ Every defect logged with repro steps and evidence in `tickets.md`.
- ✅ Bug-detector tests wired to ticket IDs.
- ⏳ Agent happy-path retest pending a running provider (follow-up, blocked by environment).

---

# Follow-up round — v0.9.6 (2026-10-15, later same day)

## FORGE-139 — side-pane tab persistence (user-reported) — FIXED

- Root cause: 3 of the 4 `AgentRightSidebar` mount sites unmounted the pane on drawer close, destroying `paneTabs`/`activeTab` local state. Fixed by adopting the CSS-hidden keep-mounted pattern at `AgentHomeView`, `AgentContainer` (CLI tasks) and `EditorView` (the `AgentActiveSessionView` site was already correct).
- New pure module `frontend/src/components/agent/paneTabs.ts` extracted from the component and consumed by it; 28 unit tests cover singleton upsert, file-tab mirroring, close-activation, title numbering and terminal focus rules. Frontend suite grew to 148 tests (10 files).
- GUI-verified in the running app: agent home — Git tab survives drawer close/reopen; close-by-X → chooser (and stays after reopen); editor — Git + Side chat tabs both survive close/reopen (screenshot-verified, v0.9.6 #38).

## FORGE-140 — agent sandbox workspace — SHIPPED

- New `internal/sandbox` package (`~/.forge/sandbox/<slug>` with `tmp/ thinking/ coding/ logs/`), prompt injection per turn, sandbox-scoped file writes skip the approval pause, bindings `GetSandboxDir`/`ClearSandbox`, task-menu items, Settings card, 7-day `tmp/` sweep at startup.
- Go suite grew to 47 tests; full `go test ./...` green after the change.

## Dev-environment note

A `ForgeADE.dev.app` crash report (`CODESIGNING / Invalid Page`) from this session was analyzed: kernel code-signing monitor kill of the ad-hoc dev binary during the rebuild/force-stop cycle — **not an application bug**; no ticket.
