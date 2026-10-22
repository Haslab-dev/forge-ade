# AGENTS.md

Notes for AI coding agents working in this repository.

## What this is

Wails v3 desktop app (Go 1.26 + React 19/TS + Tailwind v4 + CodeMirror 6).
- **Backend Architecture**: Modular Monolith with Hexagonal boundaries. Composition root in `internal/bootstrap/`; thin Wails transport facade in `internal/app/`; domain logic in dedicated packages (`internal/board/`, `internal/quota/`, `internal/agentsession/`, `internal/git/`, `internal/terminal/`, `internal/workspace/`, `internal/browseruse/`, `internal/computeruse/`).
- **Frontend Architecture**: Feature-Oriented Architecture (`frontend/src/features/`) with pure Zustand domain stores (`frontend/src/stores/`) and Contract-Driven IPC (`frontend/src/platform/wails/`).
- **Marketing / Web Landing Page**: Located in `web/` (Vite + React 19 + Tailwind v4), deployed to Cloudflare Pages (`https://forge-ade.pages.dev`).
- Read `docs/ARCHITECTURE.md` before structural changes and `docs/DESIGN_SYSTEM.md` before styling changes.

## Non-negotiables

- Exported methods on `*App` (package main, `*_api.go`) are the wire contract — do not rename or remove them; the frontend bindings depend on them.
- All frontend→backend calls go through `frontend/src/lib/wails.ts` and `frontend/src/platform/wails/`.
- Styling uses semantic token utilities (`bg-panel`, `text-fg-secondary`, `text-ui-sm`, …). Never introduce hardcoded hex, raw px font sizes, or `bg-white dark:bg-[#hex]` pairs.
- State management: Use pure Zustand stores (`useLayoutStore`, `useGitStore`, `useTerminalStore`, `useEditorStore`, `useBoardStore`, `useProviderStore`, `useIntegrationsStore`, `useWorkspaceStore`). Do not wrap the application in heavy React Context Providers that trigger cascading re-renders.
- `bun` is the package manager (no npm lockfile).

## Verify before finishing

```sh
go build ./... && go vet ./... && gofmt -l .
go test ./...
cd frontend && bun run lint && bun run build
# When touching web/:
cd web && bun run build
```

## Conventions

- Commits: conventional commits (`feat(agentsession): …`, `fix(go): …`, `style(ui): …`, `chore(release): …`).
- Go business logic → `internal/<feature>/` with tests; `*_api.go` files stay thin.
- Subsystems must not import `internal/app` to talk to each other; cross-subsystem orchestration belongs in application services or typed domain contracts.
- Frontend events: emit via `internal/events` bus; forward through `eventbridge.go`.
- Workflow Kanban Board: project tasks and backlog are tracked in `.forge/board_tasks.json`. Monitored in real time by `internal/board/watcher.go` (`BoardWatcher`).
- Three different things are called "session" (PTY, agentsession, agent chat) — see `docs/ARCHITECTURE.md` before touching session code.
