# Performance & Memory

Engineering budgets and measurement methodology for ForgeADE. Numbers below are
**measured baselines**, not targets-aspirations: re-run the commands and update
the tables when hardware or code changes.

The golden rule from profiling practice: **RAM numbers alone mean nothing
without recovery checks.** After every stress run, verify DOM node counts and
process RSS return to near their baselines — a persistent climb is a leak,
not "usage".

## What holds memory in a Wails app

Do not assume DOM is the whole story. RAM lives in four separate places, each
measured differently:

| Holder                | What accumulates                                | How to measure                                          |
| --------------------- | ----------------------------------------------- | ------------------------------------------------------- |
| WebView (JS heap)     | tabs' buffers, CodeMirror docs, stores, caches  | Safari Web Inspector → Memory (WKWebView hides `performance.memory`) |
| WebView (DOM)         | explorer rows, chat messages, terminal buffers  | `__forgeBench.domNodes()`                                |
| Go process            | symbol index, file-tree snapshots, watcher maps | `__forgeBench.rss()` → `GetProcessRSS` binding (`ps -o rss`) |
| Child processes       | git, ripgrep, language servers, PTYs            | Activity Monitor / `ps` filtered by parent pid           |

## Backend baselines (Apple M1, measured)

```sh
go test -bench "BenchmarkGetTree|BenchmarkListDir" -benchtime 3x ./internal/explorer/
```

| Benchmark            | Result                                          |
| -------------------- | ----------------------------------------------- |
| GetTree 10k files    | 76 ms/op, 16.3 MB/op allocated, 166k allocs     |
| GetTree 25k files    | 130 ms/op, 30.4 MB/op allocated, 304k allocs    |
| ListDirectory (expand)| 0.18 ms/op, 39 KB/op                           |

Reading: a full `GetFileTree(-1)` on open/refresh is the expensive backend
step (the frontend lazy-loads folders at 0.2 ms, which is why expanding is
cheap). If workspace-open latency regresses, benchmark this first.

Generate matching synthetic workspaces for manual/in-app testing:

```sh
go run ./scripts/benchws -out /tmp/bench-ws-10k -files 10000
go run ./scripts/benchws -out /tmp/bench-ws-100k -files 100000
```

## Frontend bundle (measured)

| Chunk                 | Before          | After lazy-split     |
| --------------------- | --------------- | -------------------- |
| index (startup JS)    | 683 KB (169 gz) | **503 KB (131 gz)**  |
| codemirror (editor)   | 1,074 KB, eager | 1,074 KB, **on first editor visit** |
| EditorView            | in index        | 135 KB (33 gz), lazy |
| SettingsScreen        | in index        | 71 KB (15 gz), lazy  |
| MarkdownPreview       | in index        | 4 KB, lazy           |

Startup on the agent surface now downloads and parses ~180 KB less JS and
skips CodeMirror entirely until the editor is first opened (⌘2). Editor and
Settings stay mounted after their first visit — PTY scrollback, split panes,
and tab state survive surface switches exactly as before.

## In-app harness (dev builds only)

`bun run dev` exposes `window.__forgeBench` (see `frontend/src/lib/devBench.ts`):

```js
__forgeBench.report()                       // DOM nodes + Go RSS snapshot
await __forgeBench.openCloseLoop('/src/index.ts', 20)
                                            // open/close a file N times;
                                            // reports avg ms + RSS delta (~0 expected)
await __forgeBench.refreshLoop(10)          // repeated full tree reloads
__forgeBench.trace(true)                    // warn on every long task >50ms
```

Acceptance checks when running against a synthetic workspace:

1. **Startup → interactive**: cold start to first paint on the agent surface;
   compare before/after any new startup code.
2. **Open 10k / 100k workspace**: time from workspace switch to tree painted
   (`refreshLoop` approximates the tree half). DOM nodes must stay O(visible
   rows) — virtualized explorer ≈ tens of nodes, not file count.
3. **Expand/collapse stress**: rapidly toggle deep folders; DOM count must
   return to baseline; no stale rows (virtualizer + memoized rows).
4. **Open/close loop**: `openCloseLoop` RSS delta must hover near 0 MB.
   Closed buffers must not be retained by nav history or caches.
5. **Git churn while scrolling**: run `git status` bursts (checkout between
   branches) while the explorer scrolls — UI stays responsive (git refresh is
   debounced 600 ms in workspaceStore).
6. **Idle CPU**: after startup settles, CPU should sit at ~0% (event-driven
   watcher, no polling).

For JS-heap retention checks (heap is invisible to `GetProcessRSS`), use
Safari Web Inspector → Storage/Memory snapshots before and after
`openCloseLoop`, looking for retained `EditorTab`/string objects.

## Incremental indexing & watcher — behavior and limits

- Watcher (`internal/watcher`) publishes per-path create/write/delete events;
  the index store updates **only the changed file** (`Store.Update` /
  `Store.Remove` — no full rebuild). Atomic saves (write-temp + rename)
  surface as rename→create on the same path and re-index correctly.
- Known watch budget: `maxWatchDepth = 8`, `maxWatchedDirs = 500`. Deeper or
  denser trees are not watched; changes there land only after
  **Preferences → Rebuild Symbol Index** (`ReindexWorkspace`), which is the
  manual full-reconciliation path.
- Git status refresh after FS bursts is debounced (600 ms quiet window) so
  builds never trigger per-event `git status` storms.

## Large-file guards

- **Binary/executable files are refused before any read** — no stat, no IPC, no
  bytes reach the WebView (extension blocklist in `workspaceStore.tsx`;
  extensions with dedicated viewers — images, PDF — are exempt).
- Pre-read: `GetFileSize` (os.Stat, or SFTP stat over `ssh://`) refuses files
  > 8 MB before any buffer is allocated.
- Post-read fail-safe: when the size cannot be known before reading, the
  read result itself is bounded by the same limit — a giant file can never
  reach CodeMirror.
- Back/forward nav snapshots and persisted editor sessions carry **metadata
  only** (no buffer content); revived tabs re-read from disk. The per-
  workspace in-memory session cache is capped at 8 workspaces.
