# ForgeADE

**AI Development Environment** — one domain codebase, multiple delivery
targets: a Wails desktop app, a pure-Go native desktop app, headless CLIs,
and (reserved) a web target.

> ForgeADE adalah AI Development Environment desktop. Satu codebase domain
> (`core/`), banyak target aplikasi — webview (Wails), native (MyGo), dan CLI.

![CI](https://github.com/Haslab-dev/forge-ade/actions/workflows/ci.yml/badge.svg?branch=main)

---

## Monorepo layout

```
forge-ade/
├── core/            Shared Go domain (17 UI-agnostic packages)
├── app/
│   ├── native/      MyGo desktop app — GPU-rendered, single binary
│   ├── webview/     Wails v3 desktop app — React 19 + Vite frontend
│   ├── cli/         forge-agent (headless sub-agent), forge-mcp, shelltest
│   └── web/         Reserved for the future browser target
├── web/             Marketing site (Cloudflare Pages)
├── docs/            ARCHITECTURE.md · ADR/ · legacy docs
└── Makefile         Single entry point for every target
```

Dependency direction (enforced by review + Go `internal` visibility):

```
app/*  →  core/*  →  RakitKode (agent engine, external module)
```

See [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for the full picture and
[docs/ADR/0001-monorepo.md](docs/ADR/0001-monorepo.md) for why this layout.

## Quickstart

```bash
# Native desktop app (pure Go, no node toolchain needed)
make native-run           # or: make native-app  → build/forge-native.app

# Webview desktop app (Wails; needs bun + wails3)
make frontend-install     # once
make webview-dev          # hot-reload dev mode

# Headless sub-agent (CI / scripts / other apps)
make cli
dist/forge-agent --workspace . --prompt "Summarize this repo" --quiet
```

## Make targets

`make help` prints everything. The essentials:

| Area      | Targets                                                          |
| --------- | ---------------------------------------------------------------- |
| Native    | `native` · `native-dev` · `native-e2e` · `native-app` · `native-run` |
| Webview   | `webview-dev` · `webview-build` · `webview-package` · `webview-bindings` |
| Frontend  | `frontend-install` · `frontend-build` · `frontend-test`          |
| CLI       | `cli`                                                            |
| Quality   | `test` · `cover` · `vet` · `fmt` · `lint`                        |
| Version   | `version` · `patch-version` · `minor-version` · `major-version` · `bump-build` |
| Release   | `sign` · `notarize` · `clean`                                    |

## Development notes

- **Go module:** `github.com/hasdev/forge-ade` (single module). Local dev of
  the agent engine uses `go.work` with a sibling `../rakitkode` checkout;
  CI builds against the pinned module version.
- **Bindings:** the webview frontend talks to Go through generated Wails
  bindings under `app/webview/frontend/bindings/` (regenerate with
  `make webview-bindings` after changing service methods).
- **Data:** `~/.forge-ade/` holds shared runtime data; per-project state
  lives in `<workspace>/.forge/` and `<workspace>/.workspace/`.
- **Commits:** conventional commits (`feat(core):`, `fix(native):`, …).
  Structural decisions require an ADR in `docs/ADR/`.

## Contributing

See [CONTRIBUTING.md](CONTRIBUTING.md). PRs run the full CI pipeline
(Go vet/build/test, golangci-lint, frontend build+test, native bundle).

## License

See [LICENSE](LICENSE).
