# ForgeADE

**Native AI Development Workspace**

ForgeADE is a native, lightweight, AI-first development workspace built for modern software engineers. Unlike traditional IDEs that treat AI as an extension, ForgeADE treats AI agents, terminals, Git, and projects as first-class citizens — all running locally with minimal resource usage.

Built with [Wails v3](https://v3.wails.io/) (Go + WebView), React 19, and CodeMirror 6.

## Philosophy

- **Native First** — No Electron. Go backend, lightweight WebView frontend.
- **Workspace First** — Everything belongs to a workspace. Folders are temporary workspaces.
- **AI Native** — AI is part of the architecture, not an extension.
- **Lightweight** — Fast startup, low RAM usage.
- **Offline First** — Everything works locally. Cloud features are optional.

## Features

- **Terminal Session mode** (default) — run agent CLIs (pi, claude, codex, opencode, or custom) in PTY sessions with scrollback replay, auto-naming, resume, and restart. ForgeADE never interprets agent output; the terminal renderer stays generic.
- **Code editor** — CodeMirror 6 with workspace symbol indexing (completions, outline, auto-import), inline syntax check (esbuild), formatting (prettier), diff gutters with per-hunk revert/stage.
- **Git UI** — commit graph, branches, staging, conflict resolution, per-hunk revert, and AI-generated commit messages from the staged diff.
- **Built-in AI agent** — chat with tool use, approvals, and skills, powered by configurable LLM providers.
- **ACP agents** — external agents over the Agent Client Protocol with permission prompts, model switching, and slash commands.
- **MCP** — connect Model Context Protocol servers; discover and import servers/skills from other agent tools installed on the machine.
- **Plugins** — plugin manager built on a declarative runtime (`cordis`), plus a marketplace system used by the built-in agent harness.
- **SSH/SFTP explorer** — open a remote directory as a workspace over SSH.
- **Automations** — saved prompt workflows on cron schedules.
- **Browser Use** — in-app WKWebView engine exposing `browser_*` tools to agents.
- **Search** — instant filename search (trie), ripgrep content search, and project-wide replace.

## Tech Stack

| Layer    | Technology                                                                                     |
| -------- | ---------------------------------------------------------------------------------------------- |
| Shell    | Wails v3 (beta.16), Go 1.26                                                                     |
| Frontend | React 19, TypeScript, Vite, Zustand                                                             |
| Styling  | Tailwind CSS v4 (CSS-first `@theme` tokens) — see [docs/DESIGN_SYSTEM.md](docs/DESIGN_SYSTEM.md) |
| Editor   | CodeMirror 6                                                                                    |
| Terminal | xterm.js + creack/pty                                                                           |
| Lint     | oxlint (frontend), `go vet` (backend)                                                           |

## Getting Started

Prerequisites: Go 1.26+, [Bun](https://bun.sh), Node (for version scripts), and the Wails v3 CLI:

```sh
go install github.com/wailsapp/wails/v3/cmd/wails3@latest
```

```sh
bun install --cwd frontend   # frontend dependencies
make dev                     # hot-reload development
```

### Build & Release

```sh
make build                   # debug build with devtools
make build-prod              # release build
make sign ID="..."           # codesign (see Makefile)
make notarize EMAIL=... TEAM=...   # notarize (see Makefile)
```

### Tests & Checks

```sh
make test                    # go test ./...
make vet                     # go vet ./...
cd frontend && bun run lint  # oxlint
cd frontend && bun run build # tsc -b && vite build
```

Note: a few tests in `internal/acp` are live smoke tests that require the `opencode`/`codex`/`pi`/`claude` binaries on PATH; they fail on machines without them.

## Project Structure

```
├── main.go               # Wails app bootstrap, window creation
├── app.go                # App struct, wiring, lifecycle (bindings live in *_api.go)
├── *_api.go              # Frontend API surface, one file per feature area
├── internal/             # All business logic (see docs/ARCHITECTURE.md)
│   ├── agentsession/     #   Terminal Session mode: CLI configs, sessions, disk store
│   ├── terminal/         #   PTY session manager
│   ├── agent/            #   Built-in LLM agent (turn loop, tools, approvals)
│   ├── acp/              #   Agent Client Protocol drivers
│   ├── index/            #   Workspace symbol index (completions, outline)
│   ├── git/              #   Git engine (status, graph, diffs, hunks)
│   ├── commitmsg/        #   AI commit message generation
│   ├── editor/           #   esbuild syntax check + prettier formatting
│   └── …                 #   search, mcp, plugins, skills, llm, browseruse, …
├── frontend/
│   ├── src/
│   │   ├── components/   # Feature UI (agent/, editor/, session/, settings/, modals/, …)
│   │   ├── stores/       # Zustand stores + workspace context store
│   │   ├── services/     # Agent engines, API bridge
│   │   ├── lib/          # Wails bindings facade, editor globals, design helpers
│   │   └── index.css     # Design tokens (single source of truth)
│   └── bindings/         # Generated Wails bindings (build artifact, gitignored)
├── docs/                 # ARCHITECTURE, DESIGN_SYSTEM, CONTRIBUTING
├── cmd/shelltest/        # Terminal animation stress-test CLI (make shell-test)
└── samples/              # Small polyglot fixtures for manual editor testing
```

## Documentation

- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) — system overview, backend package map, the four "session" concepts, event flow, known debt
- [docs/DESIGN_SYSTEM.md](docs/DESIGN_SYSTEM.md) — tokens, the `text-ui` scale, theming rules, do/don't
- [docs/CONTRIBUTING.md](docs/CONTRIBUTING.md) — workflow, conventions, commit style

## License

[MIT](LICENSE)
