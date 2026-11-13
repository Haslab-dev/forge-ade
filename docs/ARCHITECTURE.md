# ForgeADE — Monorepo Architecture

ForgeADE is an AI Development Environment shipped as **multiple application
targets from one Go codebase**: a Wails v3 desktop app (webview), a pure-Go
MyGo desktop app (native), headless CLIs, and a reserved web target. All
targets share one domain layer (`core/`) and one dependency: the RakitKode
agent engine (external module).

Status: accepted — see [ADR-0001](./ADR/0001-monorepo.md).
History of the previous single-app layout is preserved in
`docs/ARCHITECTURE-legacy-webview.md`.

---

## 1. Layering & Dependency Rules

```
┌─────────────────────────────── app/* (delivery) ─────────────────────────────┐
│                                                                              │
│  app/webview   Wails v3 + React 19   app/native   MyGo (pure Go, GPU UI)     │
│  internal/app  the single bound service        pkg/ui      AppUI + views     │
│  internal/bootstrap composition root           pkg/engine native comp-root  │
│  frontend/     TS bindings → Go methods       pkg/…      agent, git, ui, …  │
│                                                                              │
│  app/cli       forge-agent / forge-mcp / shelltest (headless)                │
│  app/web       reserved for the future browser target                        │
└──────────────────────────────────────┬───────────────────────────────────────┘
                                       │ imports only ↓
┌──────────────────────────────────────▼───────────────────────────────────────┐
│  core/ — UI-agnostic domain packages (one concern per package)               │
│  acp · agentsession · automations · board · browseruse · commitmsg ·         │
│  computeruse · dbviewer · discovery · editor · explorer · forgemcp · git ·   │
│  quota · remotefs · terminal · watcher · workspace                           │
└──────────────────────────────────────┬───────────────────────────────────────┘
                                       │ imports only ↓
┌──────────────────────────────────────▼───────────────────────────────────────┐
│  github.com/Haslab-dev/RakitKode — agent engine (LLM, tools, skills,         │
│  plugins, memory, MCP, events bus, agentclirc) — external module             │
└──────────────────────────────────────────────────────────────────────────────┘
```

**Rules (enforced in review; `go vet` + CI keep the tree honest):**

1. `core/**` MUST NOT import `app/**`, any GUI framework (wails, mygo), or
   `frontend/**`. Core is consumable by any target, including tests and CLIs.
2. `app/webview/internal/**` is importable only inside `app/webview` — Go's
   `internal` visibility enforces this mechanically.
3. Cross-target code reuse goes through `core/` or an explicit adapter at the
   app boundary (e.g. the `agent.ShellManager` adapter in the webview and the
   mirror of it in `app/cli/forge-agent`).
4. The RakitKode engine is reached only through its public packages; the
   monorepo keeps it external (module dependency, workspace override in dev).
5. Frontend ↔ Go communicates exclusively through generated Wails bindings
   (`app/webview/frontend/bindings/**` — mirrors Go import paths; regenerate
   with `make webview-bindings`).

## 2. Repository Layout

| Path                | Contents                                                              |
| ------------------- | --------------------------------------------------------------------- |
| `core/`             | Shared domain packages (see §3)                                       |
| `app/native/`       | MyGo desktop app: `pkg/{agent,ui,git,theme,…}`, `cmd/forge-native`    |
| `app/webview/`      | Wails desktop app: `main.go`, `frontend/` (React 19 + Vite + bun), `build/` (packaging Taskfiles), `internal/{app,bootstrap}` |
| `app/cli/`          | `forge-agent` (headless sub-agent runner), `forge-mcp` (MCP stdio server), `shelltest` (PTY stress tool) |
| `app/web/`          | Reserved for the future browser target (intentionally empty)          |
| `web/`              | Marketing site (Cloudflare Pages) — unrelated to app targets          |
| `docs/`             | Architecture, ADRs, legacy docs                                       |
| `.github/`          | CI (go / lint / frontend / native-bundle) and release workflows       |

## 3. Core Package Inventory

| Package        | Concern                                                            |
| -------------- | ------------------------------------------------------------------ |
| `core/acp`     | Agent Client Protocol client (JSON-RPC sessions with agent CLIs)    |
| `core/agentsession` | External agent-CLI session manager + config/settings stores    |
| `core/automations`  | Scheduled/automated task manager                              |
| `core/board`   | Kanban task board (`.forge/board_tasks.json`)                       |
| `core/browseruse`   | Chrome automation over CDP + in-app browser                   |
| `core/commitmsg`    | AI commit-message generation (uses RakitKode llm)             |
| `core/computeruse`  | Native desktop control (macOS)                                |
| `core/dbviewer`     | Database inspection (sqlite, duckdb, rocksdb metadata)        |
| `core/discovery`    | Discovers MCP servers/skills from other agent tools           |
| `core/editor`       | Lightweight non-LSP editor language tooling                   |
| `core/explorer`     | File-tree explorer service (event-bus driven)                 |
| `core/forgemcp`     | ForgeADE MCP server (stdio)                                   |
| `core/git`          | Git engine: status/diff/conflicts/worktree/PR ops             |
| `core/quota`        | Agent account/tier quota queries                              |
| `core/remotefs`     | SSH/SFTP remote filesystem sessions                           |
| `core/terminal`     | PTY terminal manager + default-shell config                   |
| `core/watcher`      | File watcher (fsnotify → RakitKode event bus)                 |
| `core/workspace`    | Workspace/folder manager, project rules                       |

## 4. Composition Roots

Each target owns its wiring; nothing shares a god-object:

- **webview** — `internal/bootstrap.Boot()` constructs every manager and
  returns `internal/app.App`, the single Wails service
  (`application.NewService(a)` in `main.go`). All `_api.go` files expose
  methods to the frontend; an event bridge forwards `events.Bus` topics to
  webview events.
- **native** — `app/native/pkg/engine.NewEngine("")` wires the RakitKode
  pieces the native UI needs (LLM, git engine, watcher, ACP manager, SFTP,
  bus); `pkg/ui.NewAppUI(engine, dir)` builds the interface state tree.
- **cli** — `forge-agent` re-wires the same RakitKode subset headlessly and
  streams one turn to stdout (see package comment for flags).

When a manager is needed by a new target: wire it in that target's
composition root — never add cross-target singletons.

## 5. Build & Release Matrix

| Target  | Dev                        | Build                | Package         |
| ------- | -------------------------- | -------------------- | --------------- |
| native  | `make native-run`          | `make native`        | `make native-app` (codesign ad-hoc; `sign`/`notarize` for distribution) |
| webview | `make webview-dev`         | `make webview-build` | `make webview-package` (wails3) |
| cli     | —                          | `make cli`           | binaries in `build/bin/` |

Version source of truth: `app/webview/frontend/package.json`
(`make version`, `make patch-version`, …). CI runs on every push/PR (go,
lint, frontend, native bundle); tags `v*` cut draft releases with both
bundles attached.

## 6. Data & Runtime Directories

- `~/.forge-ade/` — shared runtime data (agent sessions, skills, settings,
  session output logs). Both desktop targets read/write the same directory;
  the native engine falls back to `~/.forge-native` when the former is absent.
- `<workspace>/.forge/` — per-project board data and rules.
- `<workspace>/.workspace/` — workspace index.

## 7. Testing Strategy

- **core**: unit + benchmark tests per package (`go test ./core/...`).
- **app/native**: package tests (`pkg/agent` has the deepest coverage) plus
  a build-tag-gated e2e automation harness (`make native-e2e` +
  `MYGO_AUTOMATION` socket, driven by `mygo-automation`).
- **webview frontend**: vitest (`make frontend-test`), typechecked builds
  (`make frontend-build`).
- **webview Go**: service-level tests under `internal/app`.

## 8. Conventions

- Go: module `github.com/hasdev/forge-ade`; packages named by concern, not
  layer; no `util`/`common` dumping grounds.
- Frontend: feature-oriented modules under `frontend/src/features/*`,
  zustand stores, bindings accessed only through `src/lib/wails.ts` /
  `src/platform/wailsClient.ts` wrappers.
- Commits: conventional commits (`feat(core): …`, `fix(native): …`).
- ADRs in `docs/ADR/` — any structural decision gets a numbered record.
