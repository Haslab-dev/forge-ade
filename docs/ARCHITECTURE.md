# Architecture

ForgeADE is a Wails v3 desktop app: a Go process hosting a WebView that renders a React frontend. The Go side owns all business logic in `internal/*`; the frontend talks to it through Wails service bindings and receives updates through Wails events.

```
┌────────────────────────── WebView (React) ──────────────────────────┐
│  components/  ← stores/ ← services/ ← lib/wails.ts (bindings facade)│
└───────────────┬─────────────────────────────▲───────────────────────┘
        call()/bindings            EventsOn(name, cb)
┌───────────────▼─────────────────────────────┴───────────────────────┐
│  Go: *_api.go (App methods, one file per feature)                    │
│      eventbridge.go (internal bus → Wails events)                    │
│  internal/* packages (all business logic)                            │
└──────────────────────────────────────────────────────────────────────┘
```

## Backend

### The App struct and API files

`app.go` holds only the `App` struct (all manager fields), `NewApp()` wiring, and service lifecycle (`ServiceStartup`/`ServiceShutdown`). Every exported method on `*App` becomes a frontend binding, grouped by feature into one file each:

| File                 | Area                                                            |
| -------------------- | --------------------------------------------------------------- |
| `workspace_api.go`   | Workspaces, recents, native dialogs, workspace-open lifecycle   |
| `explorer_api.go`    | File tree + git status annotation                               |
| `files_api.go`       | File CRUD, clipboard, path resolution                           |
| `remote_api.go`      | SSH/SFTP connections                                            |
| `editor_tooling.go`  | Syntax check / formatting (delegates to `internal/editor`)      |
| `terminal_api.go`    | PTY shells + AI terminal sessions                               |
| `session_api.go`     | Terminal Session mode (agent CLIs, app settings)                |
| `sessiondisk_api.go` | Session disk persistence (delegates to `internal/agentsession`) |
| `search_api.go`      | Filename/content search, replace                                |
| `index_api.go`       | Symbol index queries                                            |
| `agent_api.go`       | Built-in agent sessions + definitions                           |
| `providers_api.go`   | LLM provider profiles, active model                             |
| `skills_api.go`      | Skills CRUD                                                     |
| `plugins_api.go`     | Plugin manager CRUD                                             |
| `memory_api.go`      | Memory entries                                                  |
| `mcp_api.go`         | MCP servers + multi-source discovery                            |
| `git_api.go`         | Git operations + AI commit messages                             |
| `acp_api.go`         | ACP external agents                                             |
| `automations_api.go` | Saved prompt workflows                                          |
| `system_api.go`      | Data dir, path resolution, new window, command execution        |
| `browser_api.go`     | Browser Use viewer                                              |
| `eventbridge.go`     | Bus subscriptions, event forwarding, git-dirty sweep            |

Rules of thumb:

- API methods stay thin. Business logic belongs in `internal/*` so it is testable without the Wails runtime.
- Never rename or remove exported `App` methods without regenerating/auditing frontend bindings — they are the wire contract. Adding is safe.

### internal/ package map

| Package        | Lines | Purpose |
| -------------- | ----- | ------- |
| `acp`          | 5,276 | Agent Client Protocol manager + per-agent drivers (pi, claude, codex, opencode) |
| `index`        | 4,357 | Workspace symbol index: JS/CSS parsers, store, dependency graph |
| `agent`        | 4,042 | Built-in LLM agent: turn loop, dialects, definitions |
| `browseruse`   | 3,657 | In-app WKWebView engine + CDP fallback, `browser_*` tools |
| `tools`        | 3,501 | Agent tool registry (core/edit/background/browser tools) |
| `harness`      | 2,852 | Marketplace plugin system + turn state + permissions used by the agent |
| `agentsession` | 2,155 | Terminal Session mode: CLI configs, session records, settings, disk store |
| `plugins`      | 2,084 | Plugin manager + executor (built on `cordis`) |
| `git`          | 1,867 | Git engine: status, graph, diffs, hunks |
| `cordis`       | 1,392 | Declarative plugin runtime (cordis.yml, fibers, contexts) |
| `search`       | 1,319 | Filename trie + ripgrep content search + replace |
| `llm`          | 1,164 | LLM client + provider profiles |
| `terminal`     | 819   | PTY session manager |
| `mcp`          | 805   | MCP stdio client + manager |
| `discovery`    | 765   | Discover MCP servers/skills from other agent tools |
| `remotefs`     | 730   | SSH/SFTP client + manager |
| `automations`  | 683   | Saved prompt workflows + cron scheduler |
| `workspace`    | 496   | Workspace model + recents persistence |
| `skills`       | 449   | Skill CRUD + workspace skills |
| `watcher`      | 409   | Recursive file watcher |
| `explorer`     | 322   | File tree / directory listing |
| `memory`       | 316   | Memory entries store |
| `editor`       | 241   | esbuild syntax check + prettier formatting (workspace-root injected) |
| `events`       | 152   | Typed event bus + event constants |
| `commitmsg`    | 133   | AI commit message generation |
| `gitignore`    | 101   | go-git-based gitignore matcher |
| `ignore`       | 97    | Canonical skip-dir list |

### The four "session" concepts

The word "session" appears in four distinct layers — keep them straight:

1. **Terminal PTY sessions** (`internal/terminal`) — raw shells/agent-CLI processes; `WriteSession`/`StopSession`/`ResizeSession` bindings route here first.
2. **Agent sessions** (`internal/agentsession`) — Terminal Session mode records: which agent CLI, launch config, status, disk persistence (`~/.forge/sessions/<project>/`; workspace `.forge/` copies are legacy read-only). All per-project helper state lives under `~/.forge/` via `internal/globalstore` — never inside user workspaces.
3. **Built-in agent chat sessions** (`internal/agent`) — the LLM chat loop with tools/approvals.
4. **ACP sessions** (`internal/acp`) — conversations with external agents over ACP.

The generic bindings (`WriteSession`, `StopSession`) try (1) and fall back to (2) so one terminal renderer drives both. `ResizeSession` does the same inline in `terminal_api.go`.

### Event flow

Internal packages emit on a typed bus (`internal/events`). `eventbridge.go` subscribes once and forwards to the frontend via Wails events, owning three things:

- the single "is the app up" guard inside `emitEvent`,
- the mapping table from bus constants to frontend event names (`forwardedEvents`),
- the agent event list forwarded verbatim (`agentEventTypes`).

File-system events additionally set `gitDirtyFlag`, swept lazily by git status consumers (`sweepGitDirty`) so bursts of writes never trigger per-event `git status`.

### Frontend layering

- `lib/wails.ts` — the only module importing generated `bindings/`; every backend call goes through it.
- `services/apiBridge.ts` — normalizing wrapper over `wails.ts`.
- `stores/workspaceStore.tsx` — the large workspace context (mode, tabs, agent sessions, settings). Zustand slices live in `stores/sessionStore.ts` and `hooks/store.ts`.
- `components/` — feature UI. `App.tsx` switches between the unified Workspace surface (ForgeADE agent chat + agent CLI terminals, selected per task), Editor, Automations, and Settings surfaces.

## Modes and tasks

There is one workspace surface. A **task** is either `'forge'` (the internal
ForgeADE agent chat, rendered by `AgentActiveSessionView`/`AgentHomeView`, run
by the Go harness) or `'cli'` (an external agent CLI — omp, opencode, pi, … —
in its own PTY, rendered by `TerminalSessionSurface`). Which kind is visible
follows `activeTaskKind` in `workspaceStore`; the old separate
Terminal Session / Agent UI modes and the ACP composer path were removed
(ACP backend bindings remain for other consumers).

## Known debt

Deliberate simplifications, listed so they are findable:

1. **Two plugin systems** — `internal/plugins`+`cordis` (user plugins) and `internal/harness/plugins.go` (marketplace, ported from a reference app). Merging them is a large behavioral change; not attempted in the 2026-10 refactor.
2. **Two agent execution paths** — `services/agentEngine.ts` (webview/ACP; now only the side-chat feature) and `services/goAgentSession.ts` (Go harness — the only main-composer path since the ACP mode was removed).
3. **God store** — `stores/workspaceStore.tsx` (~3.6k lines) mixes many domains; candidates for zustand slices.
4. **Untyped API boundary** — most `lib/wails.ts` wrappers return `Promise<any>`; generated models exist in `frontend/bindings/**/models.ts` and should replace hand-duplicated shapes in `src/types.ts`.
5. **Two icon libraries** — `lucide-react` and `@tabler/icons-react` are both in use.
6. **`internal/acp` live smoke tests** — require external agent binaries on PATH; skipped/failing environments are expected.
