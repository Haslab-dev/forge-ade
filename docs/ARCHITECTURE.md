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
| `automations_api.go` | Saved prompt workflows                                          |
| `system_api.go`      | Data dir, path resolution, new window, command execution        |
| `browser_api.go`     | Browser Use viewer                                              |
| `eventbridge.go`     | Bus subscriptions, event forwarding, git-dirty sweep            |

Rules of thumb:

- API methods stay thin. Business logic belongs in `internal/*` so it is testable without the Wails runtime.
- Never rename or remove exported `App` methods without regenerating/auditing frontend bindings — they are the wire contract. Adding is safe.

### internal/ package map

The agent **engine** was extracted into the public Go module
[`github.com/Haslab-dev/RakitKode`](https://github.com/Haslab-dev/RakitKode)
(repo `Haslab-dev/RakitKode`, CLI binary `rakit`, home `~/.rakit`). forge-ade
consumes it like any other Go module — during local development via `go.work`
pointing at `../rakitkode`, at release via a pinned version tag. The desktop
app keeps only the Wails-facing packages:

| Package        | Lines | Purpose |
| -------------- | ----- | ------- |
| `agentsession` | 2,187 | Terminal Session mode: CLI configs, session records, settings, disk store |
| `git`          | 1,835 | Git engine: status, graph, diffs, hunks |
| `terminal`     | 1,081 | PTY session manager (adapts to the engine's `agent.ShellManager`) |
| `discovery`    | 765   | Discover MCP servers/skills from other agent tools |
| `remotefs`     | 730   | SSH/SFTP client + manager |
| `automations`  | 683   | Saved prompt workflows + cron scheduler |
| `workspace`    | 496   | Workspace model + recents persistence |
| `watcher`      | 409   | Recursive file watcher |
| `explorer`     | 322   | File tree / directory listing |
| `editor`       | 241   | esbuild syntax check + prettier formatting (workspace-root injected) |
| `commitmsg`    | 133   | AI commit message generation |
| `browseruse`   | 3,804 | In-app WKWebView engine + CDP fallback (desktop-only) |
| `computeruse`  | 843   | Native macOS computer-use tools (desktop-only) |

Moved to the module (import as `github.com/Haslab-dev/RakitKode/<pkg>`):
`agent` (turn loop, sessions, approvals, subagents), `harness` (turn runtime,
permissions, scheduling), `tools` (tool registry + core tool set),
`llm` (provider client), `mcp`, `plugins` + `cordis`, `skills`, `memory`,
`sandbox`, `search`, `index`, `events` (bus), `store` (was `globalstore`;
root is injectable via `SetRoot`/`RAKIT_HOME` — the app pins
`~/.forge-ade/rakit` in `NewApp`), `ignore`, `gitignore`, `conffmt`, `jev`,
`agentclirc`, plus the `engine` facade and the `cmd/rakit` CLI.
Desktop-only tool glue (`browser_tools.go`, `computeruse_tools.go`) lives in
`internal/app`.

### Home directory layout

One app home (`~/.forge-ade`), four entries — `migrateLayout` in
`internal/app/app.go` moves the pre-0.4 flat layout on first launch:

```
~/.forge-ade/
├── config/   hand-editable config + credentials: app-settings.json,
│             providers_config.json, profiles.json, mcp_servers.json,
│             agent-clis.json, automations.json, skills/, plugins/,
│             marketplaces/
├── state/    session-derived, regenerable: agentsessions/,
│             acp_sessions.json, recent.json, jev_stats.json,
│             screenshots/, browser-profile/
├── rakit/    RakitKode engine store root (per-project buckets)
└── general/  the default user workspace (user files — never touched)
```

Credentials for the external coding agents (OpenCode, Pi, OMP, Claude Code,
Codex) live in the canonical
`~/.config/agent-config/agent-config.json` and are generated into each
tool's native config via `rakit agentconfig sync`.

### The three "session" concepts

The word "session" appears in three distinct layers — keep them straight:

1. **Terminal PTY sessions** (`internal/terminal`) — raw shells/agent-CLI processes; `WriteSession`/`StopSession`/`ResizeSession` bindings route here first.
2. **Agent sessions** (`internal/agentsession`) — Terminal Session mode records: which agent CLI, launch config, status, disk persistence (`~/.forge-ade/state/agentsessions/`; workspace `.forge/` copies are legacy read-only). Engine per-project state lives under `~/.forge-ade/rakit/` via the module's `store` package — never inside user workspaces.
3. **Built-in agent chat sessions** (module `agent`) — the LLM chat loop with tools/approvals.

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
follows `activeTaskKind` in `workspaceStore`.

## Known debt

Deliberate simplifications, listed so they are findable:

1. **Two plugin systems** — `internal/plugins`+`cordis` (user plugins) and `internal/harness/plugins.go` (marketplace, ported from a reference app). Merging them is a large behavioral change; not attempted in the 2026-10 refactor.
2. **Two agent execution paths** — `services/agentEngine.ts` (side-chat feature) and `services/goAgentSession.ts` (Go harness — main composer).
3. **God store** — `stores/workspaceStore.tsx` (~3.6k lines) mixes many domains; candidates for zustand slices.
4. **Untyped API boundary** — most `lib/wails.ts` wrappers return `Promise<any>`; generated models exist in `frontend/bindings/**/models.ts` and should replace hand-duplicated shapes in `src/types.ts`.
5. **Two icon libraries** — `lucide-react` and `@tabler/icons-react` are both in use.
