# Contributing to ForgeADE

Thanks for contributing! This monorepo ships several application targets
from one domain codebase — the rules below keep it coherent.

## Getting started

```bash
git clone https://github.com/Haslab-dev/forge-ade && cd forge-ade
make help        # discover every target
make test        # Go tests
make frontend-install && make frontend-build   # webview toolchain
```

Local development of the RakitKode engine uses `go.work` with a sibling
checkout: clone `Haslab-dev/RakitKode` as `../rakitkode` next to this repo.

## Ground rules

1. **Dependency direction** — `app/* → core/* → RakitKode`. Never the other
   way; never between app targets. `core/**` must not import any GUI
   framework (wails, mygo) — see `docs/ARCHITECTURE.md` §1.
2. **Composition over singletons** — each target wires its own managers in
   its composition root (`internal/bootstrap.Boot`, `pkg/engine.NewEngine`,
   `app/cli/forge-agent`).
3. **Conventional commits** — `feat(core):`, `fix(native):`,
   `feat(webview):`, `docs:`, `refactor:`, `chore:`. The scope is the tree
   you touched.
4. **Structural decisions get ADRs** — if you move directories, add a
   target, or change the layering, write `docs/ADR/000N-<slug>.md` first.
5. **Tests travel with changes** — bug fixes need a regression test;
   `core/` packages carry their own unit tests; frontend changes need
   `bun run build` + `bun run test` green.

## Pull requests

- Keep PRs scoped to one target or `core/` where possible; cross-cutting
  refactors are fine but say so in the description.
- CI must be green: `go vet`, `go build ./...`, `go test ./...`,
  frontend `bun run build` + `bun run test`, native bundle on macOS.
- Update docs (`README`, `docs/ARCHITECTURE.md`, package READMEs) when the
  layout or public behavior changes.

## Release

Versions live in `app/webview/frontend/package.json` — bump with
`make patch-version` / `minor-version` / `major-version`, then tag
`vX.Y.Z`; the release workflow builds both desktop bundles and drafts a
GitHub release.
