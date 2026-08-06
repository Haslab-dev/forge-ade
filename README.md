# ForgeADE

**Native AI Development Workspace**

ForgeADE is a native, lightweight, AI-first development workspace built for modern software engineers. Unlike traditional IDEs that treat AI as an extension, ForgeADE treats AI agents, terminals, Git, and projects as first-class citizens — all running locally with minimal resource usage.

Built with [Wails](https://wails.io/) (Go + WebView), React, and CodeMirror 6.

## Philosophy

- **Native First** — No Electron. Go backend, lightweight WebView frontend.
- **Workspace First** — Everything belongs to a workspace. Folders are temporary workspaces.
- **AI Native** — AI is part of the architecture, not an extension.
- **Lightweight** — Fast startup, low RAM usage.
- **Offline First** — Everything works locally. Cloud features are optional.

## Architecture

```
Frontend (React + TypeScript + Tailwind)
    │
    ▼
  IPC (Wails Bindings — auto-generated bridge)
    │
    ▼
  Go Backend (go — single Wails binding struct)
    │
    ├── Workspace Manager  — workspace lifecycle, .workspace YAML files, recent projects
    ├── Explorer           — file tree browsing with lazy loading, gitignore support
    ├── Search Manager     — 4-strategy search engine (see below)
    ├── Terminal Manager   — PTY session manager (shell, external agent CLIs)
    ├── Agent Manager      — native agent engine (turn loop, tools, MCP, skills, dialect)
    ├── Git Manager        — CLI-based git engine: status, diffs, hunks, conflicts, graph
    ├── File Watcher       — fsnotify-based real-time monitoring (recursive, gitignore-aware)
    ├── Event Bus          — decoupled pub/sub communication across all modules
    └── App Bindings       — Wails-exported methods bridging frontend ↔ backend
```

### IPC (Inter-Process Communication)

The frontend and backend communicate through Wails auto-generated bindings (`frontend/wailsjs/`). Each Go method exported from `go` is callable directly from TypeScript. Events flow in both directions:

- **Frontend → Backend**: Button clicks, file open requests, workspace operations, search queries, terminal input, agent messages.
- **Backend → Frontend**: `EventsEmit` pushes real-time events (`fs:changed`, `session:output`, `session:opened`, `session:closed`, `agent:*`) that the frontend subscribes to via `EventsOn`.

### Backend Module Details

| Module               | Path                                                     | Responsibility                                                                                                                        |
| -------------------- | -------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------------------- |
| `go`             | Root                                                     | Wails app bindings, orchestrates all managers                                                                                         |
| `internal/workspace` | `manager.go`, `workspace.go`                             | Workspace lifecycle (open, save, close), `.workspace` YAML, recent entries                                                            |
| `internal/explorer`  | `explorer.go`                                            | File tree with lazy loading, `ExpandPath`, directory listing, gitignore awareness                                                     |
| `internal/search`    | `search.go`, `filename.go`, `symbol.go`, `glob.go`       | Search: filename trie, content search (ripgrep + pure-Go), symbol index, ranking                                                      |
| `internal/terminal`  | `manager.go`, `session.go`, `provider.go`                | PTY session lifecycle, create/stop/rename/list sessions, `creack/pty`                                                                 |
| `internal/agent`     | `agent.go`, `message.go`, `dialect.go`, `definitions.go` | Native agent engine: turn loop, block-based messages, in-band dialect, agent definitions, session store                               |
| `internal/llm`       | `provider.go`                                            | Multi-provider LLM client: OpenAI-compatible chat + SSE streaming, provider profiles, model fetch                                     |
| `internal/tools`     | `core_tools.go`, `registry.go`                           | Core tool surface: read/write/edit/bash/search/find/glob/todo/ask/git_status + MCP tool registration                                  |
| `internal/mcp`       | `client.go`, `connection.go`, `stdio.go`                 | MCP stdio JSON-RPC 2.0 client: connect, initialize, tools/list, tools/call                                                            |
| `internal/skills`    | `skills.go`                                              | SKILL.md discovery + `/skill:<name>` invocation                                                                                       |
| `internal/events`    | `bus.go`                                                 | Pub/sub event bus — workspace, file, editor, git, terminal, and granular agent event types                                            |
| `internal/watcher`   | `watcher.go`                                             | Recursive fsnotify watching, gitignore-aware, auto-adds new subdirectories                                                            |
| `internal/git`       | `status.go`, `diff.go`, `graph.go`, `conflict.go`        | Git operations via the `git` CLI: porcelain v2 status, unified-diff parsing with hunk-level revert, commit graph, conflict resolution |
| `internal/gitignore` | `gitignore.go`                                           | `.gitignore` parser (go-git based) for filtering watched/indexed files                                                                |

### Search Architecture (4 Independent Strategies)

ForgeADE uses four independent search strategies for maximum speed and relevance:

1. **Filename Search** — Radix tree (`go-radix`) for O(length(query)) prefix + fuzzy filename lookups. Instant results as you type.
2. **Content Search** — Inverted index with Roaring Bitmaps for fast full-text search. Falls back to ripgrep (`rg`) when available, with a pure-Go grep implementation as backup. Supports regex, case-sensitive, and whole-word modes.
3. **Symbol Search** — Indexes function/class/interface/struct/method/variable/enum/constant symbols for Go, JS, TS, Rust, Python, etc. Enables "Go to Symbol" (`Ctrl+T`) style navigation.
4. **Ranking Engine** — Scores results using multiple signals: filename match, exact match, path match, recently opened boost, git-modified boost, current workspace/folder boost. Caches recent searches for instant repeat queries.

### File Watcher

Real-time `fsnotify`-based monitoring that recursively watches all workspace directories. On file change:

- The search index is updated incrementally (no full rebuild).
- The explorer tree is refreshed.
- Git status is re-checked.
- The frontend is notified via `fs:changed` event.
- New subdirectories created at runtime are automatically added to the watch set.

## Routing (Frontend)

ForgeADE uses a **screen-based router** in `tsx` rather than URL-based routing. The active screen determines which panel is rendered in the main content area.

### Screens

| Screen    | Component           | Triggered By                     |
| --------- | ------------------- | -------------------------------- |
| `welcome` | `Welcome` panel     | App launch, no workspace open    |
| `editor`  | `Editor` panel      | Opening a file, workspace loaded |
| `shell`   | `ShellScreen` panel | Creating/selecting a session     |

### Navigation Flow

```
Welcome Screen
    │
    ├── Open Folder ──→ Editor (temporary workspace)
    │
    ├── Open Workspace ──→ Editor (persistent .workspace file)
    │
    └── Open Recent ──→ Editor (restores workspace)
          │
          └── Pin / Unpin recent entries

Editor Screen
    │
    ├── Sidebar (Explorer tab) ── File tree with git status badges, create/delete/rename, drag-drop
    │
    ├── Sidebar (Search tab) ── Instant filename + content + symbol search
    │
    ├── Sidebar (Git tab) ── Stage/unstage/commit, conflicts, AI commit messages, commit graph
    │
    ├── Editor Panel ── CodeMirror 6 with multi-cursor, diff gutter, overview ruler, search panel
    │
    └── Sessions Bar (bottom) ── Session tabs with start/stop/rename/close
```

### Sidebar Tabs

The left sidebar contains three switchable tabs:

- **Explorer** — File tree with lazy loading, git status badges (folder dots, `U`/`M`/`D` file signs), create/read/write/delete/rename, hidden file toggle, drag-and-drop, context menu (copy path, open in Finder, image copy, rename, move, delete).
- **Search** — Instant search with filename/content/symbol modes, fuzzy matching, keyboard shortcuts, ranked results. Opens files directly from search results, jumping to the matched line.
- **Git** — Working tree overview (staged / unstaged / untracked / conflicts), stage, unstage, discard (with confirm), commit with AI-generated messages, and a visual commit graph.

### Session Management

All executable processes — shell terminals and native AI agent sessions — are managed as **Sessions**:

- **Shell sessions** — System shell (zsh/bash) in a workspace folder, via PTY.
- **Native AI agent sessions** — ForgeADE's built-in agent engine (not an external CLI). Runs the agent turn loop, tool calling, MCP, and skills.
- **External agent CLIs** — Also supported as PTY sessions (Claude CLI, Opencode, Codex CLI, Kilo, Command Code, or any custom process).
- Each session gets a unique ID, name, and state, and appears in the **Session Manager** (sidebar) and the **Session panel** (bottom tabs).
- **Stable ordering** — Sessions are ordered by creation time, so the active panel never jumps position while you type.
- **Rename** — Right-click a session tab in the Session panel, or click the pencil icon in the sidebar, to rename any shell or agent session.

## Features

### Workspace Management

- **Multi-root workspaces** — Open multiple folders as one environment, saved as `.workspace` YAML files.
- **Temporary workspaces** — Open a folder without saving; converted to persistent on save.
- **Recent projects** — Pinned/favorited entries with last-opened timestamps, workspace and folder support.
- **Save / Save As** — Convert temporary workspaces to persistent `.workspace` files.

### Editor

- **CodeMirror 6** with Go, JS, TS, Rust, Python, Markdown, JSON, Java, C, C++, C#, Kotlin, Swift, Shell, HTML, Vue, PHP, and more.
- **Multi-cursor editing**, code folding, bracket matching, auto-indentation.
- **Search panel** (Cmd+F) with regex, case-sensitive, whole-word support.
- **File tabs** with git status indicators (modified, staged), dirty tracking, close/save/rename.
- **Diff gutter** — VS Code-style change markers in the gutter: a green dot for added/changed lines and a red dot for removed lines. Click a dot to open a hunk popover with **Revert** (restores that hunk to HEAD), **Stage** (stages the file), **Previous/Next change**, and **Close**.
- **Overview ruler** — Small colored dots on the right edge of the editor show exactly where changes are without scrolling; click any dot to jump to that line.
- **Conflict resolution** — Files with merge conflicts open a dedicated tab with Current / Incoming / Ancestor panes and one-click **Accept Current**, **Accept Incoming**, or **Mark Resolved** actions.
- **Path bar** — Click-to-copy full file path strip above the editor.
- **Image preview** — Renders images inline when opening image files.
- **Markdown preview** — Renders Markdown files as HTML in-editor.
- **Line number jump** — Open files with `path:line` syntax for direct navigation.
- **Global file open** — Cmd+P-style file opener from anywhere in the UI.

### Terminal + AI Agents

- **Unlimited concurrent PTY sessions** — Shell and AI agents share the same runtime interface.
- **Session tabs** — Start, stop, rename, and close sessions independently.
- **Session layouts** — Single, horizontal split, and 4-grid views.
- **xterm.js** with ANSI colors, hyperlinks, copy/paste, and resize support.
- **PTY size synchronization** — The backend PTY tracks xterm's rendered geometry via xterm's `onResize` event (driven by `fit()`), so TUI/agent CLIs that redraw with `ESC[2K`/`ESC[1A` (React Ink, log-update) never wrap against a mismatched column width.
- **Stateful UTF-8 decoding** — PTY reads are decoded with a stateful UTF-8 decoder (`golang.org/x/text/encoding/unicode`) instead of per-chunk `string()`, so multi-byte characters split across read boundaries (spinners, braille glyphs, box-drawing) don't corrupt into `�`.
- **Session output streaming** — Real-time output via event bus (`session:output`, `session:closed`).

### File Explorer

- **Tree view with lazy loading** — Directories are expanded on demand.
- **File operations** — Create files, **create folders**, read, write, delete (files and folders recursively), rename, copy, move, paste.
- **Hidden file toggle** — Show/hide dotfiles.
- **Git-aware** — Files marked as gitignored are visually distinguished.
- **Git status badges** — Folders containing uncommitted changes show a green dot; files show `U` (untracked/added, green), `M` (modified, blue), or `D` (deleted, red) — mirroring VS Code's source-control decorations.
- **Drag and drop** — Reorganize files and folders.
- **Context menu** — Open in Finder, copy path, copy content, copy image, rename, move, delete.

### Search

- **Instant filename search** — Radix tree with prefix matching and fuzzy scoring.
- **Full-text content search** — Inverted index with Roaring Bitmaps, ripgrep fallback.
- **Symbol search** — Index functions, classes, interfaces, methods, variables for quick navigation.
- **Ranking** — Recently opened and git-modified files are boosted in results.
- **Search caching** — Recent queries are cached for instant repeat lookups.

### File Watcher

- **Real-time monitoring** — `fsnotify` watches all workspace directories recursively.
- **Incremental indexing** — Only changed files are re-indexed; no full rebuild.
- **Auto-refresh** — Explorer, search index, and git status update automatically on file change.
- **New directory handling** — Subdirectories created at runtime are automatically watched.

### Git Integration

- **Multi-repo aware** — Each workspace can contain multiple independent Git repositories.
- **Operations** — Stage, unstage, commit, push, pull, fetch, merge, and discard.
- **Status** — Working tree status (staged, unstaged, untracked, conflicts) via porcelain v2.
- **Hunk-level revert** — Restore individual diff hunks to HEAD from the editor's diff gutter, without discarding the whole file.
- **Conflict resolution** — Inline resolution UI (accept current / incoming / mark resolved) plus conflict detection in the git panel and explorer.
- **Commit graph** — Visual SVG lane graph of the current branch's commit history with commit dots and merge rings.
- **AI commit messages** — Generate commit messages from the staged diff using configured LLM providers.

### AI Agent Engine

- **Event-driven turn loop** — Each user message runs an agent loop with streaming deltas, a 32-iteration guard against runaway tool loops, and a **Stop** button to cancel a running turn.
- **Core tool surface** — The canonical tool set is the primary tool namespace: `read`, `write`, `edit`, `bash`, `search`, `find`, `glob`, `todo`, `ask`, `git_status`. Legacy names (`read_file`, `write_file`, `run_shell`, `search_workspace`, …) remain as aliases.
- **`ask` — structured follow-up questions** — The agent pauses the turn with an option picker; your answer is injected back and the turn resumes.
- **`todo` — session task list** — Ordered mutations (init/append/start/done/drop/block/unblock/rm/view) over the session's tasks, rendered in the chat.
- **Block-based messages** — Each assistant message carries interleaved `text`, `thinking`, `tool_call`, and `tool_result` blocks, so thinking and tool calls render alongside the response.
- **Thinking display** — Streamed `agent:thinking_delta` events render into a collapsible "Thinking" block (open by default) in the chat.
- **Tool-call timeline** — Each tool call renders as a TUI-style badge row (`READ`/`WRITE`/`EDIT`/`SHELL`/`SEARCH`) with a path/command title, a `│`/`└` tree-prefixed result (JSON keys stripped, real content shown), running `⠶` spinner, and `✓ done` / `✗ failed` states.
- **Batch approval gate** — Mutating tool calls (`write`/`edit`/`bash`) pause the whole batch for approval (Approve / Deny / Always allow).
- **YOLO mode** — A one-click toggle (left of Send) that always approves tool calls without prompting.
- **Agent mode re-selection** — Re-selecting a pre-configured agent in the chat re-configures the **current** session's context (role, prompt, rules, model) — it never spawns a new session, and the session title is preserved.
- **Auto session title** — New sessions are auto-named by the LLM with a short, ChatGPT-style title derived from the first message.
- **Shared chat view** — The Workspace editor tab and the Session panel render the **same** agent chat component (header, token badge, model picker, agent picker, `@`-mentions, YOLO, input), so both surfaces always look and behave identically.
- **MCP (Model Context Protocol) client** — Connects to configured stdio MCP servers over JSON-RPC 2.0 (`initialize` → `tools/list` → `tools/call`), registers their tools into the agent's tool registry, and reconnects on demand from Settings → MCP.
- **Skill invocation** — `/skill:<name>` in a prompt (leading or mid-prompt form) injects the skill's SKILL.md body and directory into the conversation so the model follows the skill's playbook and can resolve its scripts.
- **In-band tool-calling dialect** — XML `<invoke>` dialect for providers without reliable native tools. The model's text output is parsed back into tool calls by a streaming scanner.
- **Project-scoped session history** — Agent sessions are linked to their project folder; the sidebar and sessions bar show only the history for the current workspace, hiding sessions from other projects.
- **Granular events** — `agent:turn_start/end`, `agent:message_start/delta/end`, `agent:thinking_delta`, `agent:tool_delta/end`, `agent:ask` are streamed to the frontend via the event bus (no polling).
- **Smooth scrolling** — The chat auto-scrolls to the bottom only while you're pinned there; scrolling up to read doesn't get yanked down, and rAF-scheduled scrolls eliminate flicker.

### Event Bus

- **Decoupled pub/sub** — All modules communicate through events, never direct calls.
- **Event types** — `FileCreated`, `FileChanged`, `FileDeleted`, `TerminalOutput`, `TerminalOpened`, `TerminalClosed`, `TerminalResized`, `fs:changed`, `agent:updated`, `agent:turn_start/end`, `agent:message_start/delta/end`, `agent:thinking_*`, `agent:tool_*`.
- **Frontend subscriptions** — `EventsOn("fs:changed", ...)` for real-time UI updates.

### Appearance

- **Dark theme** — macOS dark appearance with system preference detection.
- **Windows Mica** — Mica backdrop effect on Windows.
- **Safe area insets** — Proper padding for notched macOS displays.

## Tech Stack

| Layer           | Technology                                 |
| --------------- | ------------------------------------------ |
| Backend         | Go 1.26, Wails v2                          |
| Frontend        | React 19, TypeScript, Tailwind CSS 4       |
| Editor          | CodeMirror 6                               |
| Terminal        | xterm.js + `creack/pty`                    |
| Git             | `go-git/v5` + shell fallback               |
| Search          | `armon/go-radix`, `RoaringBitmap/roaring`  |
| File watching   | `fsnotify`                                 |
| Workspace files | YAML                                       |
| Build           | Vite 8, Bun                                |
| IPC             | Wails auto-generated bindings (`wailsjs/`) |
| Event Bus       | Custom pub/sub (`internal/events/bus.go`)  |

## Project Structure

```
├── internal/
│   ├── agent/        — AI agent sessions (Claude CLI, Opencode, Codex, custom)
│   ├── events/       — Pub/sub event bus
│   ├── explorer/     — File tree browsing with lazy loading
│   ├── git/          — CLI-based git engine (status, diff/hunks, conflicts, graph)
│   ├── gitignore/    — .gitignore parser
│   ├── llm/          — LLM provider profiles & model configuration
│   ├── mcp/          — MCP server/tool integration
│   ├── plugins/      — Plugin registry
│   ├── search/       — Search engine (filename, content, symbol, ranking)
│   ├── skills/       — Skill definitions
│   ├── terminal/     — PTY session manager (shell, AI agents)
│   ├── tools/        — Core tool registry (core_tools.go), MCP tool registration
│   ├── watcher/      — fsnotify file watcher (recursive, gitignore-aware)
│   └── workspace/    — Workspace lifecycle & settings (.workspace YAML)
├── frontend/
│   └── src/
│       ├── tsx            — Screen router (welcome → editor → shell)
│       ├── main.tsx           — React entry point
│       ├── components/
│       │   ├── sidebar.tsx            — Explorer / Search / Git tabs + Session Manager
│       │   ├── git-panel.tsx          — Staged/unstaged/untracked/conflicts + commit
│       │   ├── diff-view.tsx          — Unified diff renderer
│       │   ├── sessions-bar.tsx       — Bottom session tabs bar
│       │   ├── terminal-view.tsx      — xterm.js PTY renderer
│       │   ├── agent-chat.tsx         — Agent chat body (turns, tool badges, thinking)
│       │   ├── agent-panel.tsx        — Shared agent chat panel (header + body + input)
│       │   └── ...                    — resizable-split, modals, toast
│       ├── panels/
│       │   ├── editor.tsx             — CodeMirror 6 editor with tabs, diff gutter,
│       │   │                            overview ruler, conflict resolution
│       │   ├── git-graph-panel.tsx    — SVG commit lane graph
│       │   ├── agent-screen.tsx       — AI agent chat panel
│       │   ├── shell-screen.tsx       — Session layout manager (single/horizontal/grid)
│       │   └── welcome.tsx            — Welcome screen with recent projects
│       ├── hooks/
│       │   └── store.ts               — Zustand workspace & UI state stores
│       ├── types.ts                   — TypeScript type definitions
│       └── lib/
│           ├── wails.ts               — Typed wrappers for Wails bindings
│           ├── file-icons.tsx         — Language-aware file icons
│           └── toast.tsx / utils.ts   — UI helpers
├── docs/
│   ├── prd.md                     — Product Requirements Document
│   ├── search-architecture.md     — Search engine design
│   ├── terminal-architecture.md   — Session & PTY architecture
│   ├── file-watcher-architecture.md — fsnotify watcher design
│   └── new-git-architecture.md    — Git module architecture
├── cmd/              — CLI entry points
├── pkg/              — Shared utilities
├── go            — Wails app bindings (all backend ↔ frontend bridges)
├── main.go           — Application entry point (Wails bootstrap)
└── README.md
```

## Getting Started

### Prerequisites

- Go 1.26+
- Node.js / Bun
- [Wails CLI](https://wails.io/docs/gettingstarted/installation)

### Development

```bash
wails dev
```

This starts the Vite dev server with hot reload for the frontend and the Go backend in live mode.

### Build

```bash
wails build
```

Produces a native distributable binary in `build/bin/`.

## Design Docs

Detailed architecture decisions are documented in `docs/`:

- [Product Requirements](./docs/prd.md)
- [Search Architecture](./docs/search-architecture.md)
- [Terminal Architecture](./docs/terminal-architecture.md)
- [File Watcher Architecture](./docs/file-watcher-architecture.md)
- [New Git Architecture](./docs/new-git-architecture.md)

## License

MIT
