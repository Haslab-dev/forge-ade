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
  Go Backend (app.go — single Wails binding struct)
    │
    ├── Workspace Manager  — workspace lifecycle, .workspace YAML files, recent projects
    ├── Explorer           — file tree browsing with lazy loading, gitignore support
    ├── Search Manager     — 4-strategy search engine (see below)
    ├── Terminal Manager   — PTY session manager (shell, AI agents, Docker, SSH, custom)
    ├── Git Manager        — multi-repo git operations via go-git + shell fallback
    ├── File Watcher       — fsnotify-based real-time monitoring (recursive, gitignore-aware)
    ├── Event Bus          — decoupled pub/sub communication across all modules
    └── App Bindings       — Wails-exported methods bridging frontend ↔ backend
```

### IPC (Inter-Process Communication)

The frontend and backend communicate through Wails auto-generated bindings (`frontend/wailsjs/`). Each Go method exported from `app.go` is callable directly from TypeScript. Events flow in both directions:

- **Frontend → Backend**: Button clicks, file open requests, workspace operations, search queries, terminal input.
- **Backend → Frontend**: `EventsEmit` pushes real-time events (`fs:changed`, `session:output`, `session:opened`, `session:closed`) that the frontend subscribes to via `EventsOn`.

### Backend Module Details

| Module | Path | Responsibility |
|---|---|---|
| `app.go` | Root | Wails app bindings, orchestrates all managers |
| `internal/workspace` | `manager.go`, `workspace.go` | Workspace lifecycle (open, save, close), `.workspace` YAML, recent entries |
| `internal/explorer` | `explorer.go` | File tree with lazy loading, `ExpandPath`, directory listing, gitignore awareness |
| `internal/search` | `search.go`, `filename.go`, `content.go`, `symbol.go`, `ranking.go`, `cache.go` | 4-strategy search: filename radix tree, content inverted index, symbol index, ranking |
| `internal/terminal` | `manager.go`, `session.go`, `provider.go` | PTY session lifecycle, create/stop/rename/list sessions, `creack/pty` |
| `internal/events` | `bus.go` | Pub/sub event bus — types: `FileCreated`, `FileChanged`, `FileDeleted`, `TerminalOutput`, `TerminalOpened`, `TerminalClosed`, `TerminalResized` |
| `internal/watcher` | `watcher.go` | Recursive fsnotify watching, gitignore-aware, auto-adds new subdirectories |
| `internal/git` | `git.go` | Multi-repo git operations via `go-git/v5` with shell fallback |
| `internal/gitignore` | `gitignore.go` | `.gitignore` parser for filtering watched/indexed files |

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

ForgeADE uses a **screen-based router** in `App.tsx` rather than URL-based routing. The active screen determines which panel is rendered in the main content area.

### Screens

| Screen | Component | Triggered By |
|---|---|---|
| `welcome` | `Welcome` panel | App launch, no workspace open |
| `editor` | `Editor` panel | Opening a file, workspace loaded |
| `shell` | `ShellScreen` panel | Creating/selecting a session |

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
    ├── Sidebar (Explorer tab) ── File tree, create/delete/rename files, drag-drop
    │
    ├── Sidebar (Search tab) ── Instant filename + content + symbol search
    │
    ├── Sidebar (Sessions tab) ── Active terminal sessions, create new shell/AI agent
    │
    ├── Editor Panel ── CodeMirror 6 with multi-cursor, code folding, search panel
    │
    └── Sessions Bar (bottom) ── Session tabs with start/stop/rename/close
```

### Sidebar Tabs

The left sidebar contains three switchable tabs:

- **Explorer** — File tree with lazy loading, create/read/write/delete/rename, hidden file toggle, drag-and-drop, context menu (copy path, open in Finder, image copy, rename, move, delete).
- **Search** — Instant search with filename/content/symbol modes, fuzzy matching, keyboard shortcuts, ranked results. Opens files directly from search results.
- **Sessions** — Active session list with status indicators, create new shell/AI agent, stop, rename, and switch between sessions.

### Session Management

All executable processes — shell terminals and AI agents — are managed as **Sessions** through the same PTY-based interface:

- **Shell sessions** — System shell (zsh/bash) in a workspace folder.
- **AI agent sessions** — Claude CLI, Opencode, Gemini CLI, Codex CLI, Aider, Kilo, or any custom process.
- Each session gets a unique ID, name, PID, and provider label.
- Sessions appear as tabs in the bottom sessions bar and can be rearranged, renamed, or closed independently.

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
- **Image preview** — Renders images inline when opening image files.
- **Markdown preview** — Renders Markdown files as HTML in-editor.
- **Line number jump** — Open files with `path:line` syntax for direct navigation.
- **Global file open** — Cmd+P-style file opener from anywhere in the UI.

### Terminal + AI Agents

- **Unlimited concurrent PTY sessions** — Shell and AI agents share the same runtime interface.
- **Session tabs** — Start, stop, rename, and close sessions independently.
- **Session layouts** — Single, horizontal split, and 4-grid views.
- **xterm.js** with ANSI colors, hyperlinks, copy/paste, search, and resize support.
- **Session output streaming** — Real-time output via event bus (`session:output`, `session:closed`).

### File Explorer

- **Tree view with lazy loading** — Directories are expanded on demand.
- **File operations** — Create, read, write, delete, rename, copy, move, paste.
- **Hidden file toggle** — Show/hide dotfiles.
- **Git-aware** — Files marked as gitignored are visually distinguished.
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
- **Operations** — Stage, unstage, commit, amend, push, pull, fetch, stash, cherry-pick, revert, merge, rebase, reset, checkout.
- **Status** — Working tree status with gitignore awareness.

### Event Bus

- **Decoupled pub/sub** — All modules communicate through events, never direct calls.
- **Event types** — `FileCreated`, `FileChanged`, `FileDeleted`, `TerminalOutput`, `TerminalOpened`, `TerminalClosed`, `TerminalResized`, `fs:changed`.
- **Frontend subscriptions** — `EventsOn("fs:changed", ...)` for real-time UI updates.

### Appearance

- **Dark theme** — macOS dark appearance with system preference detection.
- **Windows Mica** — Mica backdrop effect on Windows.
- **Safe area insets** — Proper padding for notched macOS displays.

## Tech Stack

| Layer | Technology |
|---|---|
| Backend | Go 1.26, Wails v2 |
| Frontend | React 19, TypeScript, Tailwind CSS 4 |
| Editor | CodeMirror 6 |
| Terminal | xterm.js + `creack/pty` |
| Git | `go-git/v5` + shell fallback |
| Search | `armon/go-radix`, `RoaringBitmap/roaring` |
| File watching | `fsnotify` |
| Workspace files | YAML |
| Build | Vite 7, Bun |
| IPC | Wails auto-generated bindings (`wailsjs/`) |
| Event Bus | Custom pub/sub (`internal/events/bus.go`) |

## Project Structure

```
├── internal/
│   ├── events/       — Pub/sub event bus
│   ├── explorer/     — File tree browsing with lazy loading
│   ├── git/          — Multi-repo Git operations
│   ├── gitignore/    — .gitignore parser
│   ├── search/       — 4-strategy search engine (filename, content, symbol, ranking)
│   ├── terminal/     — PTY session manager (shell, AI agents, Docker, SSH)
│   ├── watcher/      — fsnotify file watcher (recursive, gitignore-aware)
│   └── workspace/    — Workspace lifecycle & settings (.workspace YAML)
├── frontend/
│   └── src/
│       ├── App.tsx        — Screen router (welcome → editor → shell)
│       ├── main.tsx       — React entry point
│       ├── components/
│       │   ├── sidebar.tsx        — Explorer, Search, Sessions tabs
│       │   ├── sessions-bar.tsx   — Bottom session tabs bar
│       │   ├── terminal-view.tsx  — xterm.js PTY renderer
│       │   ├── code-editor.tsx    — CodeMirror 6 editor with language support
│       │   ├── explorer.tsx       — File tree panel with operations
│       │   ├── shell-screen.tsx   — Session layout manager (single/horizontal/grid)
│       │   ├── editor.tsx         — Editor panel with tabs, git status, preview
│       │   ├── welcome.tsx        — Welcome screen with recent projects
│       │   └── ui/                 — Button, ScrollArea primitives
│       ├── panels/
│       │   ├── editor.tsx         — Main editor with file tabs
│       │   ├── shell-screen.tsx   — Session grid layout
│       │   ├── explorer.tsx       — File explorer panel
│       │   └── welcome.tsx        — Welcome/home screen
│       ├── hooks/
│       │   └── store.ts           — Zustand workspace & UI state stores
│       ├── types/
│       │   └── index.ts           — TypeScript type definitions
│       └── lib/
│           ├── utils.ts           — Utility functions
│           └── zoom.ts            — Editor zoom support
├── docs/
│   ├── prd.md                     — Product Requirements Document
│   ├── search-architecture.md     — Search engine design
│   ├── terminal-architecture.md   — Session & PTY architecture
│   ├── file-watcher-architecture.md — fsnotify watcher design
│   ├── new-git-architecture.md    — Git module architecture
│   └── docs.md                    — General documentation
├── cmd/              — CLI entry points
├── pkg/              — Shared utilities
├── app.go            — Wails app bindings (all backend ↔ frontend bridges)
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
