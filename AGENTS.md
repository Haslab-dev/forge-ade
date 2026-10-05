# AGENTS.md

Notes for AI coding agents working in this repository.

## What this is

Wails v3 desktop app (Go 1.26 + React 19/TS + Tailwind v4). Go backend in `internal/*`; frontend in `frontend/src`. Read `docs/ARCHITECTURE.md` before structural changes and `docs/DESIGN_SYSTEM.md` before styling changes.

## Non-negotiables

- Exported methods on `*App` (package main, `*_api.go`) are the wire contract — do not rename or remove them; the frontend bindings depend on them.
- All frontend→backend calls go through `frontend/src/lib/wails.ts`.
- Styling uses semantic token utilities (`bg-panel`, `text-fg-secondary`, `text-ui-sm`, …). Never introduce hardcoded hex, raw px font sizes, or `bg-white dark:bg-[#hex]` pairs.
- `bun` is the package manager (no npm lockfile).

## Verify before finishing

```sh
go build ./... && go vet ./... && gofmt -l .
go test ./...
cd frontend && bun run lint && bun run build
```

`internal/acp` smoke tests fail without `opencode`/`codex`/`pi`/`claude` binaries on PATH — not your fault, not a regression.

## Conventions

- Commits: conventional commits (`feat(agentsession): …`, `fix(go): …`, `style(ui): …`).
- Go business logic → `internal/<feature>/` with tests; `*_api.go` files stay thin.
- Frontend events: emit via `internal/events` bus; forward through `eventbridge.go`.
- Four different things are called "session" (PTY, agentsession, agent chat, ACP) — see docs/ARCHITECTURE.md before touching session code.
