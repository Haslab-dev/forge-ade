# AGENTS.md

Notes for AI coding agents working in this repository.

> [!IMPORTANT]
> **PRIMARY ACTIVE TARGET: `app/native/`**
> All active development, UI/UX polishing, bug fixes, and feature additions MUST be done directly in **`app/native/`** (pure-Go MyGo desktop application).
> The Wails v3 webview version (`app/webview/`) is **OBSOLETE / ARCHIVED** and serves only as a historical reference and 1:1 parity benchmark (see `app/native/PARITY.md`). Do NOT spend time developing for `app/webview/` unless explicitly requested.

## Architecture

- **Primary Desktop App**: `app/native/` (Pure Go + MyGo GPU UI).
  - Composition root & engine wiring: `app/native/pkg/engine/`
  - UI layer (Screens, Modals, Viewers): `app/native/pkg/ui/`
  - Internal editor & diff: `github.com/egoist/mygo/ui/editor`
  - Document & media viewers: `pdf_viewer.go`, `deck_viewer.go`, `browser_viewer.go`
  - Domain sub-packages: `app/native/pkg/{agent, editor, git, terminal, settings, theme, icons, fuzzy, workspace, database, palette}`
- **Shared Domain Core**: `core/` (17 UI-agnostic packages: ACP, agent session, automations, board, browseruse, commitmsg, computeruse, dbviewer, discovery, editor, explorer, forgemcp, git, quota, remotefs, terminal, watcher, workspace).
  - Rule: `core/**` MUST NOT import `app/**` or GUI libraries.
- **Agent Engine**: `github.com/Haslab-dev/RakitKode` (external module managing LLM streaming, tools, skills, plugins, memory, MCP, and events bus).
- **Headless CLIs**: `app/cli/` (`forge-agent`, `forge-mcp`).
- **Legacy Reference**: `app/webview/` (Wails v3 + React 19 / Vite — **ARCHIVED / OBSOLETE**).

## Non-negotiables

- Focus active work on `app/native/`. Keep single-binary zero-overhead architecture intact (<30MB RAM footprint).
- Styling in native uses semantic token utilities (`tc.Surface`, `tc.Border`, `tc.Accent`, `tc.Text`, …). Never hardcode raw hex colors in UI elements.
- Keep wire contracts intact.
- Workflow Kanban Board: project tasks and backlog are tracked in `.forge/board_tasks.json`. Monitored in real time by `internal/board/watcher.go` / `core/board`.

## Verify before finishing

```sh
# 1. Build and vet all packages
go build ./... && go vet ./... && gofmt -l .

# 2. Run unit tests
go test ./app/native/...
go test ./core/...

# 3. Build native binary
go build -o dist/forge-native ./app/native/cmd/forge-native
```

## Conventions

- Commits: conventional commits (`feat(native): …`, `fix(ui): …`, `feat(core): …`, `chore: …`).
- Go business logic belongs in `core/` or `app/native/pkg/<domain>/`.
- Three different things are called "session" (PTY terminal session, agentsession CLI runner, agent chat session) — see `docs/ARCHITECTURE.md`.
