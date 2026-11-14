# ForgeADE

**Next-Generation AI Development Environment (ADE)** — Built for autonomous coding agents, multi-agent orchestration, integrated code editing, git management, interactive terminals, and rich media viewers.

> **Bahasa Indonesia:** ForgeADE adalah AI Development Environment desktop performa tinggi. Fokus pengembangan saat ini sepenuhnya dipusatkan pada **arsitektur native pure-Go (`app/native/`)** berbasis GPU UI engine, menggantikan versi webview legacy untuk menghasilkan footprint memori minimal (<30MB RAM) dan startup instan tanpa overhead runtime Electron/Node.

![CI](https://github.com/Haslab-dev/forge-ade/actions/workflows/ci.yml/badge.svg?branch=main)
![Go Version](https://img.shields.io/badge/Go-1.24%2B-00ADD8?logo=go)
![Platform](https://img.shields.io/badge/Platform-macOS%20%7C%20Linux%20%7C%20Windows-555)
![Status](https://img.shields.io/badge/Native%20Status-Active%20%26%201%3A1%20Parity-success)

---

> [!IMPORTANT]
> ### 🚨 ARCHITECTURE DIRECTIVE FOR DEVELOPERS & AI AGENTS
>
> 1. **PRIMARY & ACTIVE TARGET: `app/native/`**
>    - Seluruh pengembangan fitur baru, perbaikan bug, dan optimasi UI/UX **hanya difokuskan pada versi Native Pure-Go** (`app/native/`).
>    - Dibangun menggunakan **MyGo** (`github.com/egoist/mygo`) dengan native GPU rendering, internal editor & diff component bawaan, native WKWebView/WebKit untuk web/PDF/presentation preview, dan direct PTY terminal.
>    - Menghasilkan **single executable binary** tanpa ketergantungan pada runtime Node.js, Chromium, atau Electron.
>
> 2. **DEPRECATED / ARCHIVED / OBSOLETE TARGET: `app/webview/`**
>    - Versi Wails v3 + React 19 / Vite (`app/webview/`) sudah **OBSOLETE / ARCHIVED** dan tidak lagi dikembangkan secara aktif.
>    - Kode di `app/webview/` dipertahankan semata-mata sebagai **referensi historis dan benchmark parity 1:1** (lihat [app/native/PARITY.md](app/native/PARITY.md)).
>    - **DO NOT** menghabiskan waktu menambahkan fitur baru atau me-refactor `app/webview/` kecuali ada instruksi eksplisit. Semua pekerjaan harus langsung ditujukan ke `app/native/`.

---

## Architecture Overview

ForgeADE menerapkan arsitektur **Modular Monolith dengan Hexagonal Boundaries** yang memisahkan logic domain inti dari delivery target:

```
┌──────────────────────────────────────────────────────────────────────────────┐
│                         DELIVERY / APPLICATION LAYER                         │
│                                                                              │
│   [ACTIVE / PRIMARY TARGET]                [OBSOLETE / ARCHIVED REFERENCE]   │
│   app/native/                              app/webview/                      │
│   ├── cmd/forge-native                     ├── main.go (Wails v3 backend)    │
│   └── pkg/                                 └── frontend/ (React 19 + Vite)   │
│       ├── ui/ (MyGo GPU views & modals)                                      │
│       ├── agent/ (session & prompt engine) ───────────────────────────────── │
│       ├── editor/ (virtualized buffer)     [HEADLESS CLI TOOLS]              │
│       ├── git/ (diff, status, branches)    app/cli/                          │
│       ├── terminal/ (PTY & snippets)       ├── forge-agent (sub-agent CLI)   │
│       └── settings/ (provider sync)        └── forge-mcp (MCP stdio server)  │
└──────────────────────────────────────┬───────────────────────────────────────┘
                                       │ imports only ↓
┌──────────────────────────────────────▼───────────────────────────────────────┐
│                    core/ — SHARED UI-AGNOSTIC DOMAIN LAYER                   │
│                                                                              │
│   acp · agentsession · automations · board · browseruse · commitmsg ·        │
│   computeruse · dbviewer · discovery · editor · explorer · forgemcp · git ·  │
│   quota · remotefs · terminal · watcher · workspace                          │
└──────────────────────────────────────┬───────────────────────────────────────┘
                                       │ imports only ↓
┌──────────────────────────────────────▼───────────────────────────────────────┐
│              RakitKode Engine (github.com/Haslab-dev/RakitKode)              │
│   External agent execution engine: LLM client, streaming turns, tools,      │
│   skills catalog, MCP bridge, memory, lifecycle hooks, and events bus.       │
└──────────────────────────────────────────────────────────────────────────────┘
```

### Dependency & Layering Rules
1. `core/**` **TIDAK BOLEH** mengimpor `app/**` atau library UI apa pun (`mygo`, `wails`, `react`). Core murni berisi domain business logic yang UI-agnostic.
2. `app/native/` mengonsumsi `core/**` dan `RakitKode` secara langsung melalui composition root di `app/native/pkg/engine/`.
3. Komunikasi antar subsystem diatur melalui typed domain contract atau event bus (`events.Bus`), bukan direct import antar controller UI.

---

## Repository Layout

```
forge-ade/
├── app/
│   ├── native/            ★ [ACTIVE TARGET] Native desktop app (MyGo + GPU UI)
│   │   ├── cmd/
│   │   │   └── forge-native/  Entry point binary native desktop
│   │   ├── pkg/
│   │   │   ├── ui/        Screens, main chat, drawer, modals, theme, PDF/Deck viewer
│   │   │   ├── agent/     Autonomous agent runner, ACP client, CLI manager, prompt history
│   │   │   ├── editor/    Editor state, tabs, multi-pane split, minimap
│   │   │   ├── git/       Git operations, branch/PR/worktree managers, graph lanes
│   │   │   ├── terminal/  PTY terminal sessions, command snippets catalog
│   │   │   └── settings/  LLM credentials, custom providers, active model sync
│   │   └── PARITY.md      1:1 Parity audit matrix against webview reference
│   ├── webview/           ⚠ [OBSOLETE / ARCHIVED] Wails v3 + React 19 desktop app
│   │   ├── frontend/      React 19, Tailwind v4, CodeMirror 6 UI (reference only)
│   │   └── internal/      Wails service facade & transport bridge
│   ├── cli/               Headless CLI utilities (forge-agent, forge-mcp, shelltest)
│   └── web/               Reserved for future web platform target
├── core/                  Shared UI-agnostic Go domain packages (17 packages)
├── docs/                  ARCHITECTURE.md, ADR/, DESIGN_SYSTEM.md
├── web/                   Marketing landing page (deployed to Cloudflare Pages)
├── dist/                  Output directory for built binaries (`forge-native`)
└── Makefile               Automation entry point
```

---

## Core Feature Set (Native ForgeADE)

Native ForgeADE (`app/native/`) memiliki kesetaraan fitur penuh (1:1 feature parity) terhadap versi webview:

- 🤖 **Multi-Engine Agent Workspace**:
  - Dukungan **ForgeADE Internal Agent** (OpenAI, Anthropic, Gemini, Ollama, dan kustom gateway seperti Kenari dengan 98+ model live fetch).
  - Integrasi **Agent Client Protocol (ACP)**: Google Antigravity (AGY), Claude Code, Gemini CLI, dsb. dengan session resumption otomatis.
  - Headless **Agent CLI Sessions** (PTY terminal runner dengan auto-restart).
  - 4 Permission Modes: *Full Autonomy*, *Ask Every Time*, *Plan-Only*, *Read-Only*.
  - Prompt Stash (`Alt+S`), Prompt History (`↑`/`↓`), Steer Queue, dan Reasoning Effort Selector (`Off`/`Low`/`Med`/`High`).
  - Auto-collapsing thought block & tool traces setelah giliran selesai.
- 📝 **Native Code Editor & Diff**:
  - Menggunakan native editor engine internal MyGo (`github.com/egoist/mygo/ui/editor`).
  - Multi-pane splitting, file tabs, minimap, cursor navigation, syntax highlighting.
  - Side-by-side & unified git diff viewer.
- 🌿 **Git Client Surface**:
  - Toolbar lengkap: Branch switcher, Fetch, Pull, Push, Sync remote.
  - Changes pane (staged/unstaged hunk inspection & staging).
  - Commit History dengan 10-color graph lanes & drag-resizer commit inspector.
  - Branches, Pull Requests / Merge Requests, dan Worktrees manager.
- 💻 **Integrated PTY Terminal & Command Snippets**:
  - Multi-tab bottom & side drawer terminal dengan zero latency.
  - Modal katalog snippet & riwayat perintah (`⌥S` / `Alt+S`).
  - Ask Agent AI langsung dari error terminal.
- 🌐 **Rich Document & Media Viewers**:
  - **In-App Browser**: Native WebKit webview dengan navigasi URL, zoom, reload, dan inspect.
  - **PDF Document Viewer**: Native PDF preview dengan info metadata, Open in System Viewer, dan Reveal in Finder.
  - **Presentation & Motion Viewer**: Viewer untuk Open Design slide decks (`deck.html`) dan HyperFrames motion compositions (`motion.html`, `.mp4`) lengkap dengan slide navigation & playback controls.
- 📋 **Workflow Kanban Board**:
  - Tracking task dan backlog proyek (`.forge/board_tasks.json`) dengan live file watcher.
- ⚡ **16 System Modals**:
  - Command Palette (`⌘K`/`⌘P`), Project Search (`⇧⌘F`), Model Picker (`⌘M`), New Task (`⌘N`), SSH Remote Explorer (`⇧⌘R`), Rules Manager, Usage Gauges, Theme Switcher, dsb.

---

## Development Quickstart

### Prasyarat
- Go 1.24 atau lebih baru
- macOS (Metal/WebKit), Linux, atau Windows (DirectX/WebView2)
- Tidak memerlukan Node.js atau Bun untuk menjalankan versi Native!

### Menjalankan Native App
```bash
# Build dan jalankan langsung native app
go run ./app/native/cmd/forge-native

# Atau kompilasi ke folder dist/
go build -o dist/forge-native ./app/native/cmd/forge-native
./dist/forge-native
```

### Melalui Makefile
```bash
# Menjalankan native app
make native-run

# Mengompilasi bundle .app untuk macOS
make native-app
```

### Verifikasi Kode (Testing & Linting)
Pastikan selalu memvalidasi perubahan sebelum membuat commit:
```bash
# Build & syntax check semua paket
go build ./... && go vet ./... && gofmt -l .

# Jalankan seluruh test unit native
go test ./app/native/...

# Jalankan seluruh test domain core
go test ./core/...
```

---

## Status Folder `app/webview/` (Legacy Reference)

| Direktori | Status | Catatan |
|---|---|---|
| `app/native/` | 🟢 **ACTIVE** | Target utama seluruh pengembangan aplikasi desktop. |
| `app/webview/` | 🔴 **OBSOLETE** | Diarsipkan. Jangan menambah fitur ke sini. |
| `app/cli/` | 🟢 **ACTIVE** | Headless runner untuk integrasi CI dan sub-agent. |
| `core/` | 🟢 **ACTIVE** | Shared domain logic untuk seluruh target. |

---

## License

Didistribusikan di bawah lisensi Apache 2.0. Lihat berkas [LICENSE](LICENSE) untuk informasi lebih lanjut.
