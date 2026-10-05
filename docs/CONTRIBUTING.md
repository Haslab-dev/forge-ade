# Contributing

## Workflow

1. Branch from `main` (`feat/…`, `fix/…`, `refactor/…`).
2. Make the change; keep commits per logical phase (see commit style below).
3. Run the checks (below) — all green before opening a PR.
4. PR into `main` with a short description and screenshots for UI changes.

## Checks

```sh
make test                            # go test ./...
make vet                             # go vet ./...
cd frontend && bun run lint          # oxlint
cd frontend && bun run build         # tsc -b && vite build
gofmt -l .                           # must print nothing
```

A few tests in `internal/acp` require external agent binaries (`opencode`, `codex`, `pi`, `claude`) on PATH — failures on machines without them are expected.

## Commit style

Conventional commits, matching the existing history:

```
feat(agentsession): auto-name sessions from the first terminal input
fix(agentsession): restart now continues the CLI conversation
refactor(go): split app.go into per-feature api files
style(ui): migrate to text-ui scale and semantic tokens
chore: remove accidental files and tidy repo hygiene
docs: rewrite README, add ARCHITECTURE
```

Scope names loosely follow the feature area (`agentsession`, `go`, `ui`, `editor`, …).

## Code style

### Go

- `gofmt`/`goimports` clean; `go vet` clean.
- Business logic lives in `internal/*` packages with tests; root-level `*_api.go` files stay thin binding layers (see docs/ARCHITECTURE.md).
- Never rename or remove exported `App` methods — they are the frontend wire contract.
- Prefer explicit errors over swallowed ones; if an error is intentionally ignored, write `_ = …` with a brief why (or a comment).

### TypeScript / React

- oxlint must pass (warnings are tolerated, errors are not).
- Function components + hooks; Zustand for cross-feature state.
- All backend calls go through `lib/wails.ts` — never import generated `bindings/` directly elsewhere.

### Styling

Follow [docs/DESIGN_SYSTEM.md](DESIGN_SYSTEM.md): semantic token utilities only, `text-ui-*` for all UI text, no hardcoded hex or `bg-white dark:` pairs in new code.

## Adding a feature — where things go

1. **Backend logic** → a new or existing `internal/<feature>/` package, with tests.
2. **Binding** → a method on `*App` in the matching `*_api.go` (create one per feature if none exists), wiring any new manager in `NewApp()`.
3. **Frontend API** → a wrapper in `lib/wails.ts` (+ `services/apiBridge.ts` if normalization is needed).
4. **Events to the frontend** → emit on the bus in `internal/events`; add the forwarding entry in `eventbridge.go`.
