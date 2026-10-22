# ForgeADE

<p align="center">
  <strong>Native AI Development Workspace</strong><br>
  <em>A high-performance, lightweight desktop IDE engineered for modern AI-assisted software engineering.</em><br><br>
  <a href="https://forge-ade.pages.dev"><img src="https://img.shields.io/badge/website-forge--ade.pages.dev-white?style=flat-square&logo=cloudflare&logoColor=orange" alt="Website"></a>
  <a href="https://github.com/Haslab-dev/forge-ade/releases/tag/v1.0.6"><img src="https://img.shields.io/badge/release-v1.0.6-emerald?style=flat-square" alt="Version"></a>
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue?style=flat-square" alt="License"></a>
  <img src="https://img.shields.io/badge/go-1.26-00ADD8?style=flat-square&logo=go&logoColor=white" alt="Go">
  <img src="https://img.shields.io/badge/react-19-61DAFB?style=flat-square&logo=react&logoColor=black" alt="React 19">
</p>

---

## Overview

ForgeADE is a native, lightweight, AI-first development environment built from the ground up for software engineers who work side-by-side with autonomous AI agents. Unlike traditional IDEs that bolt AI onto heavy, resource-intensive Electron runtimes, ForgeADE couples a high-speed **Go backend** with a **native WebView** frontend powered by **React 19**, **Tailwind CSS v4**, and **CodeMirror 6** via [Wails v3](https://v3.wails.io/).

ForgeADE delivers instant startup (&lt;100ms), minimal memory consumption (~80MB base RAM), offline-first reliability, and first-class primitives for AI coding agents, PTY terminal sessions, visual Git workflows, and extensible tool protocols (MCP, Skills, Plugins).

🌐 **Official Landing Page & Tour**: [https://forge-ade.pages.dev](https://forge-ade.pages.dev)

### Core Philosophy

- **Native & Lightweight** — No Electron overhead. Native Go backend + lightweight system WebView for ultra-fast startup and minimal RAM usage.
- **AI-Native by Design** — Autonomous agents, reasoning loops, tool approvals, and task decomposition are built directly into the core architecture, not patched in as extensions.
- **Dual-Session Flexibility** — Choose between the built-in autonomous Go agent harness or run your favorite external agent CLIs (`claude`, `codex`, `pi`, `opencode`, `cmd`, `agy`) inside real PTY terminals.
- **Workspace-Centric** — Projects, symbol indexes, Git context, and session history remain cleanly scoped to the active workspace.
- **Local & Offline-First** — All core features, local PTYs, file management, and editors work completely offline. Cloud LLM providers are optional and fully configurable.

---

## Features

### 1. Built-in Autonomous Agent Harness
- **Multi-Step Reasoning Loop** — Executes complex coding prompts through autonomous iterations with configurable tool budgets (`maxToolBudget = 300`) and automatic loop detection to prevent runaway executions.
- **Approval Gate & Safety Controls** — Non-destructive tools execute automatically, while sensitive shell executions, file writes, and external operations can prompt for user confirmation.
- **Dynamic Goal & Task Decomposition** — Declare high-level goals (`/goal`) or multi-step tasks; the agent decomposes them into an atomic task checklist using `update_tasks`.
- **Live Floating Tasks Checklist** — Displays continuous task status (`pending`, `in_progress` with real-time active forms like *"Running unit tests..."*, and `completed`) above the composer.
- **Dynamic Task Replacement** — When prior tasks finish and new instructions arrive, the agent automatically replaces stale tasks with a fresh checklist for the new objective.
- **Subagent Delegation** — Spawns child subagents (`invoke_subagent`) for isolated background research, deep investigations, or independent code reviews, complete with dedicated minimal chat views.
- **TypeSafe AI Jev Router** — Intelligently classifies user intent into tool categories (`explore`, `read`, `modify`, `execute`, `git`, `web`, `skills`, `mcp`), eliminating tool hallucinations and dramatically reducing prompt token overhead.

### 2. Interactive Slash Commands & Harness Parity
Full terminal/CLI harness command parity directly in the chat composer:
- `/goal <description>` — Sets an explicit goal and automatically decomposes it into an active task checklist.
- `/plan <description>` — Activates Plan Mode: investigatory codebase analysis and architectural step formulation before any code modifications.
- `/plan-review` — Evaluates active plans, edge cases, regression risks, and test coverage gaps.
- `/skill:` & `/skill <name>` — Discovers available skills and injects executable `SKILL.md` playbooks directly into the prompt context.
- `/model` — Inspects active model details, token consumption, cache hit rate, or opens the quick model switcher.
- `/resume` — Lists and resumes prior workspace sessions.
- `/security` — Runs OMP-native security scans, credential checks, secret leakage audits, and vulnerability detection.
- `/vibe` — Activates high-velocity autonomous execution mode with automatic compiler and test fixes.
- `/settings` & `/setup` — Instant shortcut to provider configuration and model profiles.
- `/clear` — Resets active tasks and clears the floating checklist.
- `@mentions` — Seamlessly references workspace files and folders with token-capped inlining.

### 3. Extensibility: Skills, MCP & Plugins
- **Agent Skills Runtime** — Full support for reusable `SKILL.md` domain playbooks (`load_skill`, `call_skill`, `create_skill`, `list_skills`). Compatible with Antigravity, OpenCode, and custom playbooks.
- **Model Context Protocol (MCP)** — Connect stdio and SSE MCP servers. Automatically discovers and imports MCP servers and skills from other agent configurations on the machine.
- **Declarative Plugin Manager (`cordis`)** — Extends agent system prompts, contributes custom tools, and defines workflow automations through declarative manifests.
- **Long-term Workspace Memory** — Persists preferences, project decisions, and architectural notes across sessions with `learn_memory` and `recall_memory`.

### 4. Terminal Session Mode (Agent CLIs)
- Real PTY terminal management (`creack/pty` + `xterm.js`) for running external agent CLIs (`pi`, `claude`, `codex`, `opencode`, `cmd`, `agy`) or standard developer shells (`zsh`, `bash`).
- Preserves full scrollback history, automatic session naming, process restart, and disk persistence under `~/.forge/sessions/`.

### 5. High-Performance Code Editor
- Built on **CodeMirror 6** with custom light and dark semantic themes.
- Workspace symbol indexing with fuzzy symbol autocomplete, outline tree, and fast jump-to-definition.
- In-memory inline syntax checking powered by `esbuild` and formatting with `prettier`.
- Git diff gutters with inline hunk staging, review, and surgical per-hunk revert.

### 6. Visual Git Suite & AI Commit Generator
- Complete visual Git commit graph, branch management, and staged diff viewer.
- Built-in conflict resolution and visual diff inspector.
- One-click **AI Commit Message Generation** summarizing staged changes following conventional commit specifications.

### 7. In-App Browser Use Engine
- Embedded WKWebView browser panel exposing `browser_*` tools (`browser_navigate`, `browser_snapshot`, `browser_click`, `browser_type`, `browser_screenshot`) to the agent for live web research and web app testing.

### 8. Observability & Cost Analytics
- Real-time token usage meter and context window percentage visualization in the composer.
- Settings usage breakdown segmented by **Provider**, **Model**, **Session**, and **Jev Router metrics** (routing savings, classification counts, and latency).
- Cumulative prompt-cache hit rate tracking.

### 9. Remote Workspaces & Automations
- **SSH/SFTP Explorer** — Mount and edit remote directories over SSH as a first-class workspace.
- **Cron Automations** — Schedule recurring prompts, test runs, or code reviews on cron triggers.

### 10. In-App Database Viewer (SQLite, DuckDB, RocksDB)
- **Zero-Dependency Native Read-Only Engine** — High-performance SQLite inspection and execution with guaranteed read-only safety (`-readonly` flag / `mode=ro` URI) that never mutates production database files.
- **Seamless Editor Integration** — Open `.sqlite`, `.sqlite3`, `.db`, `.db3`, `.duckdb`, `.rocksdb`, `.ldb`, or `.sst` files directly in editor tabs without loading large raw binaries into editor memory.
- **Schema & DDL Inspector** — Inspect table and view schemas, column types, primary keys, not-null constraints, default values, and syntax-highlighted SQL DDL statements with one-click copy.
- **Interactive Data Grid** — Fast, paginated data grid with multi-column sorting (ASC/DESC), in-table text filtering, and an expandable cell inspector modal for inspecting large JSON payloads, text, and BLOBs.
- **Interactive SQL Query Console** — Execute arbitrary read-only SQL queries with `⌘+Enter` / `Ctrl+Enter`, real-time query latency benchmarks (ms), and instant export to CSV and formatted JSON.
- **DuckDB & RocksDB Engine Support** — Automatic format detection, directory and SSTable manifest breakdown for RocksDB/LevelDB stores, and DuckDB querying and installation guidance.

---

## Tech Stack

| Layer | Technology | Details |
| :--- | :--- | :--- |
| **Shell & Core Backend** | [Wails v3](https://v3.wails.io/) (beta.16), Go 1.26 | Modular Monolith, composition root (`internal/bootstrap`), PTY, tools |
| **Frontend Framework** | React 19, TypeScript, Vite | Feature-oriented architecture (`frontend/src/features/`), high responsiveness |
| **State Management** | Zustand (Pure Stores) | Modular domain stores (`layout`, `git`, `terminal`, `editor`, `board`, `provider`) without React Context |
| **IPC Bridge** | Contract-Driven Wails IPC | Typed request/response contracts (`contracts.ts`), typed event subscriptions, classified errors |
| **Styling & Tokens** | Tailwind CSS v4 (`@theme` tokens) | Semantic design system without hardcoded hex colors |
| **Code Editor** | CodeMirror 6 | Symbol indexing, diff gutters, syntax highlight |
| **Terminal Emulator** | xterm.js (WebGL) + `creack/pty` | Hardware-accelerated ANSI/VT100 terminal emulation |
| **Lint & Quality** | oxlint (frontend), `go vet` + `gofmt` (backend) | Blazing-fast static verification & 228 Vitest unit tests |
| **Web Landing Page** | Vite + React 19 + Tailwind v4 | Deployed to Cloudflare Pages ([forge-ade.pages.dev](https://forge-ade.pages.dev)) |

---

## Getting Started

### Prerequisites

Ensure you have the following installed on your machine:

1. **Go 1.26+** — [Download Go](https://go.dev/dl/)
2. **Bun** — [Install Bun](https://bun.sh) (`curl -fsSL https://bun.sh/install | bash`)
3. **Node.js** (v20+) — Required for version bumping scripts
4. **Wails v3 CLI**:
   ```sh
   go install github.com/wailsapp/wails/v3/cmd/wails3@latest
   ```

### 1. Clone & Install Dependencies

```sh
# Clone the repository
git clone https://github.com/Haslab-dev/forge-ade.git
cd forge-ade

# Install frontend dependencies
bun install --cwd frontend
```

### 2. Configure Providers & Models

Launch the app and open **Settings** (`Cmd+,` or click the gear icon) to configure your preferred LLM provider:
- **Supported Providers**: OpenAI, Anthropic, OpenRouter, Google Gemini, Ollama (Local), DeepSeek, or any OpenAI-compatible endpoint.
- Enter your API Key, select your default model (e.g. `claude-3-7-sonnet`, `gpt-4o`, `openrouter/...`), and customize reasoning levels.

### 3. Run in Development Mode

Run the app with live hot-reloading for both backend and frontend:

```sh
make dev
```

*Note: Wails v3 compiles the Go backend and launches Vite with hot module replacement (HMR).*

### 4. Build for Production

```sh
# Production build with devtools enabled (useful for debugging)
make build

# Clean production release binary
make build-prod
```

The resulting macOS application bundle will be created at `build/bin/ForgeADE.app`.

---

## Verification & Checks

Before submitting changes or finishing features, verify the entire codebase with:

```sh
# 1. Backend verification
go build ./... && go vet ./... && test -z "$(gofmt -l .)"
go test ./...

# 2. Frontend verification
cd frontend && bun run lint && bun run build

# 3. Web landing page verification (when modifying web/)
cd web && bun run build
```

---

## Project Structure

```
├── main.go               # Wails app bootstrap, window lifecycle, service registration
├── Makefile              # Build, development, versioning, signing, and test scripts
├── AGENTS.md             # Guidelines, non-negotiables, and verification instructions
├── internal/
│   ├── bootstrap/        # Composition root: wires dependencies & manages application lifecycle
│   ├── app/              # Thin facade DTO mapping & event bridge (*_api.go)
│   ├── board/            # Kanban task manager, disk persistence (.forge/board_tasks.json), file watcher
│   ├── quota/            # Real-time LLM quota tracking & token usage accounting
│   ├── agent/            # Built-in LLM agent harness (turn loop, tools, slash commands)
│   ├── agentsession/     # Terminal Session mode (CLI configurations, disk persistence)
│   ├── terminal/         # PTY session manager (xterm.js bridge via creack/pty)
│   ├── jev/              # TypeSafe AI Jev intelligent tool router
│   ├── tools/            # Core agent tools (read, write, edit, bash, update_tasks, skills)
│   ├── index/            # Workspace symbol indexer (completions, outlines, references)
│   ├── git/              # Git engine (status, branches, diffs, hunk staging)
│   ├── commitmsg/        # AI commit message generator
│   ├── editor/           # esbuild syntax validator & prettier formatter
│   ├── mcp/              # Model Context Protocol stdio client & server manager
│   ├── skills/           # Skill discovery, storage, and execution engine
│   ├── plugins/          # Plugin manager built on cordis runtime
│   ├── cordis/           # Declarative plugin runtime
│   ├── browseruse/       # WKWebView browser automation engine & CDP driver
│   ├── computeruse/      # Native desktop control (macOS CoreGraphics input & screenshots)
│   ├── remotefs/         # SSH/SFTP client for remote project workspaces
│   ├── automations/      # Scheduled cron prompt workflows
│   └── events/           # Typed event bus and system event constants
├── frontend/
│   ├── src/
│   │   ├── features/     # Feature-oriented modules (agent, editor, filesystem, git, terminal, board)
│   │   ├── platform/     # Contract-driven IPC (contracts.ts, events.ts, wailsClient.ts)
│   │   ├── stores/       # Pure Zustand domain stores (layout, git, terminal, editor, board, provider)
│   │   ├── components/   # UI components (agent, editor, terminal, settings, git, ...)
│   │   ├── services/     # Frontend agent engines & domain service facades
│   │   ├── lib/          # Wails bindings facade (wails.ts), utilities, design helpers
│   │   └── index.css     # CSS-first Tailwind v4 tokens & semantic design system
│   └── bindings/         # Auto-generated Wails v3 Go-to-TypeScript bindings
├── web/                  # Marketing & documentation landing page (deployed to Cloudflare Pages)
│   ├── src/              # React 19 + Tailwind v4 landing page components
│   └── public/           # Retina screenshots, icons, and Cloudflare Pages _headers
├── docs/
│   ├── ARCHITECTURE.md   # Detailed architecture, session models, and event bus specs
│   ├── DESIGN_SYSTEM.md  # Semantic tokens, typography scale, and styling guidelines
│   └── CONTRIBUTING.md   # Contribution workflows and commit conventions
└── samples/              # Polyglot test fixtures for editor and indexing validation
```

---

## Documentation

- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) — Comprehensive technical architecture, backend package map, the three session models, and event bridge.
- [docs/DESIGN_SYSTEM.md](docs/DESIGN_SYSTEM.md) — Semantic design tokens, `@theme` scale, and theming rules.
- [docs/CONTRIBUTING.md](docs/CONTRIBUTING.md) — Git workflow, branch naming, and conventional commit standards.

---

## License

[MIT](LICENSE)
