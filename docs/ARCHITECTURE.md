# ForgeADE Architecture & File Structure

ForgeADE adalah aplikasi desktop AI Development Environment (ADE) berbasis **Wails v3** (Go 1.26 backend + WebKit/WebView React 19 frontend). Dokumen ini merangkum struktur pohon berkas (*file tree*) lengkap untuk Go backend dan frontend, serta penjelasan arsitektur sistem secara mendalam.

---

## 1. High-Level Architecture

Arsitektur ForgeADE mengadopsi pola **Modular Monolith** untuk backend Go, **Feature-Oriented Architecture** untuk frontend React, dan **Contract-Driven IPC** sebagai penghubung keduanya.

```
┌─────────────────────────── React 19 Frontend (Feature-Oriented) ───────────────────────────┐
│                                                                                            │
│  UI Surfaces: Workspace (Agent Chat / CLI) | Editor (CodeMirror 6) | Automations | Settings │
│                                                                                            │
│  Feature Modules (frontend/src/features/*):                                                │
│  ├── workspace/ (lifecycle, workspaces)       ├── agent/ (streaming, callbacks, context)   │
│  ├── editor/ (tabNavigation, dirty buffers)   ├── filesystem/ (file ops, types)            │
│  ├── git/ (branches, staging, graph)          ├── terminal/ (pty session commands)         │
│  └── automations/, board/, database/, integrations/, remote/                               │
│                                                                                            │
│  Pure Zustand Stores (frontend/src/stores/*):                                              │
│  ├── useLayoutStore       ├── useGitStore       ├── useTerminalStore                       │
│  ├── useEditorStore       ├── useBoardStore     ├── useProviderStore                       │
│  ├── useIntegrationsStore └── useWorkspaceStore                                            │
│                                                                                            │
│  Contract-Driven IPC (frontend/src/platform/wails/*):                                      │
│  ├── contracts.ts (Typed request/response & service errors)                                │
│  ├── events.ts (Typed event registry & listeners)                                          │
│  └── wailsClient.ts & apiBridge.ts (Domain service facades)                                │
└──────────────────────────────────────────────┬─────────────────────────────────────────────┘
                                               │ Typed Wails IPC & Events
                                               ▼
┌────────────────────────────── Wails v3 Transport Adapter ──────────────────────────────────┐
│  internal/app/*_api.go (Thin wire facade methods on *App struct)                           │
│  internal/app/eventbridge.go (Typed event forwarding from internal Go Bus to Wails)        │
└──────────────────────────────────────────────┬─────────────────────────────────────────────┘
                                               │ Method Delegation & Event Bus
                                               ▼
┌───────────────────────────── Go Backend: Modular Monolith ─────────────────────────────────┐
│                                                                                            │
│  Composition Root:                                                                         │
│  └── internal/bootstrap/bootstrap.go (Dependency wiring, configuration, lifecycle)         │
│                                                                                            │
│  Domain Packages (internal/*):                                                             │
│  • board/        : Kanban task manager, disk persistence (.forge/board_tasks.json), watcher │
│  • quota/        : Real-time LLM quota tracking & token usage accounting                   │
│  • agentsession/ : Terminal Session mode (CLI configurations, enricher, commandcode)       │
│  • terminal/     : PTY session manager (xterm.js bridge via creack/pty)                    │
│  • workspace/    : Workspace folder management & manager lifecycle                         │
│  • git/          : Native git engine (status, branches, diffs, hunk staging, graph)        │
│  • browseruse/   : WKWebView native subview & Chrome DevTools Protocol automation          │
│  • computeruse/  : Native desktop control (macOS CoreGraphics input & screenshots)         │
│  • explorer/ & watcher/ : High-speed directory tree indexing & fsnotify file watcher       │
│  • remotefs/     : SSH/SFTP client for remote workspaces                                   │
│  • dbviewer/     : Zero-dependency SQLite / DuckDB / RocksDB inspector                     │
│  • automations/  : Scheduled cron prompt workflows (5-field local cron)                    │
│                                                                                            │
│  Extracted Agent Engine: github.com/Haslab-dev/RakitKode (LLM, Tools, Harness, MCP, Skills) │
└────────────────────────────────────────────────────────────────────────────────────────────┘
```

---

## 2. Go Backend: Complete File Tree

Struktur direktori dan file Go backend mencakup root aplikasi (`main.go`, `go.mod`), CLI tooling harness (`cmd/`), composition root (`internal/bootstrap`), dan implementasi modul domain di `internal/`:

```
.
├── go.mod
├── go.sum
├── main.go
├── cmd/
│   ├── forge-mcp
│   │   └── main.go
│   └── shelltest
│       ├── main.go
│       ├── scenes_anim.go
│       ├── scenes_color.go
│       └── scenes_cursor.go
└── internal/
    ├── bootstrap/
    │   ├── bootstrap.go
    │   └── bootstrap_test.go
    ├── board/
    │   ├── manager.go
    │   ├── manager_test.go
    │   ├── task.go
    │   ├── watcher.go
    │   └── watcher_test.go
    ├── quota/
    │   ├── quota.go
    │   └── quota_test.go
    ├── acp/
    │   ├── drivers/
    │   ├── client.go
    │   ├── manager.go
    │   ├── manager_test.go
    │   └── types.go
    ├── agentsession/
    │   ├── commandcode.go
    │   ├── config.go
    │   ├── disk.go
    │   ├── enricher.go
    │   ├── manager.go
    │   ├── manager_test.go
    │   ├── settings.go
    │   └── types.go
    ├── app/
    │   ├── acp_api.go
    │   ├── agent_api.go
    │   ├── agent_quota.go
    │   ├── agentclirc_api.go
    │   ├── app.go
    │   ├── app_test.go
    │   ├── automations_api.go
    │   ├── board_api.go
    │   ├── board_tools.go
    │   ├── board_watcher.go
    │   ├── browser_api.go
    │   ├── browser_tools.go
    │   ├── cli_models_enricher.go
    │   ├── commandcode_discovery.go
    │   ├── computeruse_tools.go
    │   ├── database_api.go
    │   ├── editor_tooling.go
    │   ├── eventbridge.go
    │   ├── explorer_api.go
    │   ├── files_api.go
    │   ├── git_api.go
    │   ├── index_api.go
    │   ├── mcp_api.go
    │   ├── memory_api.go
    │   ├── official_plugins.go
    │   ├── plugins_api.go
    │   ├── providers_api.go
    │   ├── remote_api.go
    │   ├── sandbox_api.go
    │   ├── search_api.go
    │   ├── session_api.go
    │   ├── sessiondisk_api.go
    │   ├── shellmanager.go
    │   ├── skills_api.go
    │   ├── system_api.go
    │   ├── terminal_api.go
    │   ├── toolargs.go
    │   └── workspace_api.go
    ├── automations/
    │   ├── automations.go
    │   ├── automations_test.go
    │   └── cron.go
    ├── browseruse/
    │   ├── cdp.go
    │   ├── cdp_engine.go
    │   ├── inapp_darwin.go
    │   ├── inapp_other.go
    │   ├── manager.go
    │   ├── manager_test.go
    │   ├── shared.go
    │   └── snapshot.go
    ├── commitmsg/
    │   ├── commitmsg.go
    │   └── commitmsg_test.go
    ├── computeruse/
    │   ├── computeruse.go
    │   ├── computeruse_test.go
    │   ├── input_darwin.go
    │   ├── input_other.go
    │   ├── keys_darwin.go
    │   └── live_darwin_test.go
    ├── dbviewer/
    │   ├── dbviewer.go
    │   └── dbviewer_test.go
    ├── discovery/
    │   ├── discovery.go
    │   └── discovery_test.go
    ├── editor/
    │   ├── editor.go
    │   └── editor_test.go
    ├── explorer/
    │   ├── bench_test.go
    │   ├── explorer.go
    │   └── explorer_test.go
    ├── forgemcp/
    │   ├── ipc.go
    │   ├── server.go
    │   ├── server_test.go
    │   ├── tools_board.go
    │   ├── tools_browser.go
    │   ├── tools_workspace.go
    │   └── types.go
    ├── git/
    │   ├── conflict.go
    │   ├── conflict_test.go
    │   ├── diff.go
    │   ├── diff_test.go
    │   ├── discard_test.go
    │   ├── git_test.go
    │   ├── graph.go
    │   ├── status.go
    │   ├── status_cache_test.go
    │   └── status_numstat_test.go
    ├── remotefs/
    │   ├── client.go
    │   ├── manager.go
    │   ├── sshcommand_test.go
    │   ├── types.go
    │   └── upload_test.go
    ├── terminal/
    │   ├── manager.go
    │   ├── manager_test.go
    │   ├── provider.go
    │   ├── provider_test.go
    │   └── session.go
    ├── watcher/
    │   ├── watcher.go
    │   └── watcher_test.go
    └── workspace/
        ├── manager.go
        ├── workspace.go
        └── workspace_test.go
```

---

## 3. Go Backend Architecture

Backend Go bertanggung jawab penuh atas OS-level process management, I/O filesystem, Git operations, terminal pseudo-TTY (PTY), native automation, dan koordinasi agen AI.

### 3.1 Entry Point, Composition Root & App Lifecycle (`main.go`, `internal/bootstrap/`)
- **Composition Root (`internal/bootstrap/bootstrap.go`)**: Bertanggung jawab penuh atas instansiasi service dependencies, wiring bus event, konfigurasi data directory (`~/.forge`), dan mengelola siklus hidup komponen aplikasi.
- **Thin Facade (`internal/app/app.go`)**: `*App` bertindak sebagai adapter Wails v3 murni. Tidak ada business logic yang menempel langsung di facade — semua delegasi mengalir ke paket domain internal.
- **Memory & GC Tuning**: `main.go` mengonfigurasi `debug.SetGCPercent(50)` dan `debug.SetMemoryLimit(96 MiB)` agar konsumsi memori pada macOS tetap optimal (&lt;80MB base RSS) dan tidak menumpuk heap.
- **Wails v3 Service**: Menginstansiasi `app.NewApp()` dan mendaftarkannya sebagai core service di Wails runtime.
- **Dev Server Fallback**: Menyediakan middleware reverse-proxy dengan fallback HTML loading spinner ketika Vite dev server sedang booting.

### 3.2 Wire Contract Layer (`internal/app/*_api.go`)
Setiap method yang diekspor pada `*App` otomatis dibuatkan binding TypeScript oleh Wails v3. Method dibuat tipis (*thin facade*) dan mendelegasikan logika ke paket domain di `internal/*`:

| File API | Domain / Delegasi Logika |
| :--- | :--- |
| `workspace_api.go` | Manajemen folder workspace, berkas `.workspace`, recent projects, native dialogs via `internal/workspace`. |
| `explorer_api.go` | Pembacaan direktori rekursif berkinerja tinggi, anotasi git-status, metadata berkas via `internal/explorer`. |
| `files_api.go` | Operasi CRUD berkas, clipboard system, resolusi path absolut/relatif. |
| `git_api.go` | Status git, log commit, diff/hunk inspection, discard, commit message generator via `internal/git`. |
| `terminal_api.go` | Manajemen PTY shell sessions (bash/zsh) dan bridging input/output terminal via `internal/terminal`. |
| `session_api.go` | Agent CLI terminal session mode (claude, codex, pi, opencode, agy) via `internal/agentsession`. |
| `sessiondisk_api.go` | Persistensi rekaman sesi agen ke disk (`~/.forge/sessions/`). |
| `browser_api.go` & `browser_tools.go` | Otomasi browser in-app (WKWebView / Chrome DevTools Protocol) via `internal/browseruse`. |
| `computeruse_tools.go` | Native macOS mouse/keyboard actions dan window capture via `internal/computeruse`. |
| `database_api.go` | Inspeksi database SQLite lokal (schema, table rows, query runner) via `internal/dbviewer`. |
| `remote_api.go` | Koneksi remote SSH dan SFTP file browsing/uploading via `internal/remotefs`. |
| `mcp_api.go` & `discovery` | Model Context Protocol server management & auto-discovery dari tools lain via `internal/discovery`. |
| `board_api.go` | Delegasi operasi Kanban Board ke domain `internal/board` (`Manager` dan `Watcher`). |
| `agent_quota.go` | Pelacakan batas dan konsumsi kuota model didelegasikan ke domain `internal/quota`. |
| `automations_api.go` | Workflow otomatisasi prompt tersimpan dan cron task scheduling via `internal/automations`. |
| `eventbridge.go` | Subskripsi typed event bus internal dan forwarding ke Wails WebView events. |

### 3.3 Core Domain Packages (`internal/*`)
1. **`bootstrap`**: Composition root aplikasi; mengikat event bus, storage paths, domain managers, dan dependency injection secara eksplisit.
2. **`board`**: Engine Kanban Board; memproses operasi CRUD tasks, sinkronisasi file `.forge/board_tasks.json`, dan watcher berbasis fsnotify dengan debouncing.
3. **`quota`**: Pelacak kuota real-time dan akuntansi token LLM per provider/model dengan thread-safe concurrency.
4. **`agentsession`**: Menangani state & persistensi eksternal agent CLI (misal: OpenCode, Claude Code, Pi, AGY). Mengelola config, settings, model enrichment, dan riwayat sesi disk.
5. **`git`**: Engine git native tanpa dependensi exec berat; menghitung status git beranotasi cepat, branch graph lane visualizer, parser diff & per-hunk discard, dan deteksi konflik.
6. **`terminal`**: PTY session manager berbasis `creack/pty`, mengalirkan ANSI stream dan menangani resize window.
7. **`browseruse`**: Engine otomasi web dual-mode: *In-App* menggunakan native macOS WKWebView (inapp_darwin.go) dan fallback remote CDP engine (*Chrome DevTools Protocol*).
8. **`computeruse`**: Native macOS accessibility & event automation: simulasi mouse click, drag, scroll, key strokes, dan screenshot layar.
9. **`remotefs`**: SSH client & SFTP subsystem manager untuk browsing dan transfer file remote.
10. **`explorer` & `watcher`**: Tree listing dengan cache performa tinggi serta file-system watcher berbasis `fsnotify` dengan lazy git-dirty debounce sweep.
11. **`editor`**: Integrasi syntax validator (esbuild) dan formatting tools (Prettier) yang di-inject berdasarkan root workspace.
12. **`acp`**: Agent Client Protocol manager untuk orchestrating model/driver agent terdistribusi.
13. **`RakitKode` Integration**: Mesin inferensi LLM, tool registry, prompt execution, dan turn loop didelegasikan ke module public Go `github.com/Haslab-dev/RakitKode`.
14. **`forgemcp`**: Native Model Context Protocol (MCP) server dan HTTP IPC bridge daemon (`127.0.0.1:42110`). Menyediakan MCP tools (`forge_board_*`, `forge_browser_*`, `forge_workspace_info`) untuk agent eksternal (Claude Code, Cursor, Windsurf, OpenCode, Antigravity) baik secara live GUI bridge maupun headless standalone fallback.

---

## 4. Frontend: Complete File Tree

Struktur direktori frontend berbasis React 19, Vite, dan TypeScript:

```
frontend/
├── bindings
│   └── github.com
│       ├── Haslab-dev
│       │   └── RakitKode
│       │       ├── agent
│       │       │   ├── index.ts
│       │       │   └── models.ts
│       │       ├── agentclirc
│       │       │   ├── index.ts
│       │       │   └── models.ts
│       │       ├── index
│       │       │   ├── index.ts
│       │       │   └── models.ts
│       │       ├── llm
│       │       │   ├── index.ts
│       │       │   └── models.ts
│       │       ├── mcp
│       │       │   ├── index.ts
│       │       │   └── models.ts
│       │       ├── memory
│       │       │   ├── index.ts
│       │       │   └── models.ts
│       │       ├── plugins
│       │       │   ├── index.ts
│       │       │   └── models.ts
│       │       ├── search
│       │       │   ├── index.ts
│       │       │   └── models.ts
│       │       ├── skills
│       │       │   ├── index.ts
│       │       │   └── models.ts
│       │       └── tools
│       │           ├── index.ts
│       │           └── models.ts
│       ├── hasdev
│       │   └── forge-ade
│       │       └── internal
│       │           ├── acp
│       │           │   ├── index.ts
│       │           │   └── models.ts
│       │           ├── agentsession
│       │           │   ├── index.ts
│       │           │   └── models.ts
│       │           ├── app
│       │           │   ├── app.ts
│       │           │   ├── index.ts
│       │           │   └── models.ts
│       │           ├── automations
│       │           │   ├── index.ts
│       │           │   └── models.ts
│       │           ├── computeruse
│       │           │   ├── index.ts
│       │           │   └── models.ts
│       │           ├── dbviewer
│       │           │   ├── index.ts
│       │           │   └── models.ts
│       │           ├── discovery
│       │           │   ├── index.ts
│       │           │   └── models.ts
│       │           ├── editor
│       │           │   ├── index.ts
│       │           │   └── models.ts
│       │           ├── git
│       │           │   ├── index.ts
│       │           │   └── models.ts
│       │           ├── remotefs
│       │           │   ├── index.ts
│       │           │   └── models.ts
│       │           ├── terminal
│       │           │   ├── index.ts
│       │           │   └── models.ts
│       │           └── workspace
│       │               ├── index.ts
│       │               └── models.ts
│       └── wailsapp
│           └── wails
│               └── v3
│                   ├── internal
│                   │   ├── eventcreate.ts
│                   │   └── eventdata.d.ts
│                   └── pkg
│                       └── application
│                           ├── index.ts
│                           └── models.ts
├── public
│   ├── commandcode.svg
│   ├── favicon.svg
│   └── icons.svg
├── src
│   ├── assets
│   │   ├── commandcode.svg
│   │   ├── hero.png
│   │   ├── notification-completion.mp3
│   │   ├── notification-input.mp3
│   │   ├── react.svg
│   │   └── vite.svg
│   ├── features
│   │   ├── agent/
│   │   │   ├── agent.api.ts
│   │   │   ├── agentCallbacks.ts
│   │   │   ├── contextUsage.ts
│   │   │   ├── directStreamRunner.ts
│   │   │   ├── sessionConverters.ts
│   │   │   └── sideConversationRunner.ts
│   │   ├── automations/
│   │   ├── board/
│   │   ├── database/
│   │   ├── editor/
│   │   │   ├── tabNavigation.ts
│   │   │   └── tabNavigation.test.ts
│   │   ├── filesystem/
│   │   │   ├── fileOperations.ts
│   │   │   ├── fileTypes.ts
│   │   │   └── filesystem.api.ts
│   │   ├── git/
│   │   ├── integrations/
│   │   ├── remote/
│   │   ├── terminal/
│   │   └── workspace/
│   │       ├── workspace.api.ts
│   │       └── workspaceLifecycle.ts
│   ├── platform
│   │   └── wails/
│   │       ├── contracts.ts
│   │       ├── events.ts
│   │       └── wailsClient.ts
│   ├── stores
│   │   ├── boardStore.ts
│   │   ├── composerDraftStore.ts
│   │   ├── editorStore.ts
│   │   ├── gitStore.ts
│   │   ├── integrationsStore.ts
│   │   ├── layoutStore.ts
│   │   ├── promptStashStore.ts
│   │   ├── providerStore.ts
│   │   ├── sessionStore.ts
│   │   ├── terminalStore.ts
│   │   ├── uiStore.ts
│   │   ├── workspaceConstants.ts
│   │   └── workspaceStore.tsx
│   ├── components
│   │   ├── agent
│   │   │   ├── AgentActiveSessionView.tsx
│   │   │   ├── AgentContainer.tsx
│   │   │   ├── AgentHomeView.tsx
│   │   │   ├── AgentRightSidebar.tsx
│   │   │   ├── AgentSidebar.tsx
│   │   │   ├── AgentTaskInputBar.tsx
│   │   │   ├── AtMentionMenu.tsx
│   │   │   ├── BottomTerminal.tsx
│   │   │   ├── BranchSelector.tsx
│   │   │   ├── BrowserViewer.tsx
│   │   │   ├── ContextSelectorModal.tsx
│   │   │   ├── FloatingTaskTracker.tsx
│   │   │   ├── HtmlRenderFrame.tsx
│   │   │   ├── MarkdownRenderer.tsx
│   │   │   ├── ModelSwitcherDropdown.tsx
│   │   │   ├── PanelResizeHandle.tsx
│   │   │   ├── ProviderIcon.tsx
│   │   │   ├── SidebarFileExplorer.tsx
│   │   │   ├── SubagentChatPane.tsx
│   │   │   ├── ThreadFindBar.tsx
│   │   │   ├── ToolBlockViews.tsx
│   │   │   ├── UsageQuotaModal.tsx
│   │   │   ├── WorkLog.tsx
│   │   │   ├── WorkflowBoardView.tsx
│   │   │   ├── WorkspaceHeader.tsx
│   │   │   ├── WorkspaceSwitcherDropdown.tsx
│   │   │   ├── chronologicalTimeline.test.ts
│   │   │   ├── paneTabs.test.ts
│   │   │   └── paneTabs.ts
│   │   ├── automations
│   │   │   └── AutomationsView.tsx
│   │   ├── common
│   │   │   ├── ErrorBoundary.tsx
│   │   │   ├── ThemePicker.tsx
│   │   │   └── composer-ui.tsx
│   │   ├── diff
│   │   │   └── DiffViewer.tsx
│   │   ├── editor
│   │   │   ├── ActivityBar.tsx
│   │   │   ├── CodeEditorPane.tsx
│   │   │   ├── DatabaseViewer.tsx
│   │   │   ├── DeckMotionViewer.tsx
│   │   │   ├── EditorProjectDropdown.tsx
│   │   │   ├── EditorView.tsx
│   │   │   ├── FileTree.tsx
│   │   │   ├── ForgeSettingsTab.tsx
│   │   │   ├── GitControlPanel.tsx
│   │   │   ├── GitGraphLane.tsx
│   │   │   ├── GitGraphPane.tsx
│   │   │   ├── ImagePreview.tsx
│   │   │   ├── MarkdownPreview.tsx
│   │   │   ├── PdfViewer.tsx
│   │   │   └── cmSetup.ts
│   │   ├── git
│   │   │   ├── GitAccordionDiffList.tsx
│   │   │   ├── GitClientView.tsx
│   │   │   ├── GitCommitDiffView.tsx
│   │   │   ├── GitDiffPane.tsx
│   │   │   ├── gitDiffUtils.test.ts
│   │   │   └── gitDiffUtils.tsx
│   │   ├── modals
│   │   │   ├── CommandPaletteModal.tsx
│   │   │   ├── GitGraphModal.tsx
│   │   │   ├── OpenFolderModal.tsx
│   │   │   ├── RemoteFileUploaderModal.test.tsx
│   │   │   ├── RemoteFileUploaderModal.tsx
│   │   │   └── SSHConnectModal.tsx
│   │   ├── session
│   │   │   ├── NewTaskModal.tsx
│   │   │   └── TerminalSessionSurface.tsx
│   │   ├── settings
│   │   │   ├── SettingsScreen.tsx
│   │   │   ├── SubagentsSettingsSection.tsx
│   │   │   └── settingsPrimitives.tsx
│   │   ├── shell
│   │   │   ├── ProfileRow.tsx
│   │   │   └── StatusBar.tsx
│   │   ├── diff-view.tsx
│   │   ├── resizable-split.tsx
│   │   ├── terminal-throttler.test.ts
│   │   └── terminal-view.tsx
│   ├── effect
│   │   ├── effect.test.ts
│   │   ├── errors.ts
│   │   ├── hooks.ts
│   │   ├── index.ts
│   │   ├── runtime.ts
│   │   ├── services.ts
│   │   └── wailsBridge.ts
│   ├── hooks
│   │   └── store.ts
│   ├── lib
│   │   ├── browser.ts
│   │   ├── devBench.ts
│   │   ├── diff-highlight.test.tsx
│   │   ├── diff-highlight.tsx
│   │   ├── editorGlobals.ts
│   │   ├── file-icons.tsx
│   │   ├── fuzzy.test.ts
│   │   ├── fuzzy.ts
│   │   ├── gitDecorations.test.ts
│   │   ├── gitDecorations.ts
│   │   ├── notificationSounds.ts
│   │   ├── terminalSettings.test.ts
│   │   ├── terminalSettings.ts
│   │   ├── themeEngine.ts
│   │   ├── themePalettes.ts
│   │   ├── time.test.ts
│   │   ├── time.ts
│   │   ├── timeline.ts
│   │   ├── toast.test.tsx
│   │   ├── toast.tsx
│   │   ├── utils.test.ts
│   │   ├── utils.ts
│   │   └── wails.ts
│   ├── services
│   │   ├── agentEngine.ts
│   │   ├── apiBridge.ts
│   │   ├── goAgentSession.test.ts
│   │   ├── goAgentSession.ts
│   │   └── lspService.ts
│   ├── test
│   │   └── setup.ts
│   ├── types
│   │   └── pluginMarketplace.ts
│   ├── App.tsx
│   ├── index.css
│   ├── main.tsx
│   ├── types.ts
│   └── vite-env.d.ts
├── .gitignore
├── .oxlintrc.json
├── README.md
├── bun.lock
├── index.html
├── package.json
├── tsconfig.app.json
├── tsconfig.json
├── tsconfig.node.json
├── vite.config.ts
└── vitest.config.ts
```

---

## 5. Frontend Architecture: Feature-Oriented & Pure Zustand

Frontend dibangun dengan arsitektur **Feature-Oriented** yang mengorganisasi kode berdasarkan domain bisnis, bukan jenis file.

### 5.1 Contract-Driven IPC (`src/platform/wails/`)
- **`contracts.ts`**: Menetapkan type contract resmi untuk method request/response Wails serta klasifikasi error tertata (`WorkspaceError`, `TerminalError`, `GitError`, `DatabaseError`, `AgentError`).
- **`events.ts`**: Event subscription terpusat dengan handler yang mengembalikan fungsi *teardown/unsubscribe* aman untuk mencegah memory leaks.
- **`wailsClient.ts`**: Thin client wrapper yang mengisolasi pemanggilan RPC Wails dan menormalisasi error transport.

### 5.2 Pure Zustand State Architecture (`src/stores/`)
Untuk mengeliminasi cascading re-render di seluruh pohon aplikasi, monolithic React Context Provider telah dihapus sepenuhnya dan digantikan oleh kumpulan dedicated Zustand stores:

1. **`useLayoutStore`**: Mengontrol panel layouts, sidebar widths, collapsible drawers, surface mode (`workspace`, `editor`, `automations`, `settings`), dan preferensi tema.
2. **`useGitStore`**: State cabang git aktif, unstaged/staged file sets, log commit, dan loading diff.
3. **`useTerminalStore`**: Manajemen terminal drawer, active shell tab, dan toggle bottom dock.
4. **`useEditorStore`**: Mengelola open file tabs, active tab ID, split-view mode, dan dirty file buffers.
5. **`useBoardStore`**: Kanban board state terisolasi (daftar tasks, status transitions, live updates dari watcher).
6. **`useProviderStore`**: Konfigurasi model AI, provider credentials, API keys, dan batas kuota.
7. **`useIntegrationsStore`**: MCP servers registry, skill playbooks, dan custom tool bindings.
8. **`useWorkspaceStore`**: Core workspace state (folder path, recent workspaces, active activities) dengan backward-compatible selector hook `useWorkspace()`.

### 5.3 Feature Modules (`src/features/`)
Setiap domain fungsional memiliki modul tersendiri:
- **`features/workspace/`**: Logika inisialisasi lifecycle workspace (`workspaceLifecycle.ts`), recent workspaces, dan workspace switching.
- **`features/editor/`**: Logika tab navigasi murni (`tabNavigation.ts`), keyboard navigation, dan close calculation.
- **`features/filesystem/`**: Operasi CRUD filesystem (`fileOperations.ts`), model berkas, dan icon lookup.
- **`features/agent/`**: Eksekusi streaming langsung (`directStreamRunner.ts`), callback sinkronisasi (`agentCallbacks.ts`), dan telemetri konteks (`contextUsage.ts`).
- **`features/git/`**, **`features/terminal/`**, **`features/board/`**, **`features/automations/`**, **`features/database/`**, **`features/remote/`**, **`features/integrations/`**.

---

## 6. The Three "Session" Concepts

Dalam arsitektur ForgeADE, terdapat tiga konsep independen yang disebut "Session":

1. **Terminal PTY Sessions (`internal/terminal`)**:
   - Proses shell mentah (zsh, bash) yang dialokasikan via pseudo-terminal (`pty`).
   - Diatur oleh `terminal_api.go` (`WriteSession`, `StopSession`, `ResizeSession`).
2. **Agent CLI Sessions (`internal/agentsession`)**:
   - Eksekusi CLI agen pihak ketiga (OpenCode, Claude Code, Pi) di dalam terminal runner.
   - Persistensi status dan konfigurasi disimpan di `~/.forge-ade/state/agentsessions/`.
3. **Built-in Agent Chat Sessions (`github.com/Haslab-dev/RakitKode/agent`)**:
   - Sesi interaktif LLM turn-loop dengan pemanggilan alat (*tool calling*), user approval, subagent delegation, dan memory injection.

---

## 7. Event Flow & Synchronization

```
[File Change / Git Action / Agent Output]
                   │
                   ▼
       internal/events (Typed Bus)
                   │
                   ▼
       internal/app/eventbridge.go
                   │
    ┌──────────────┴──────────────┐
    ▼                             ▼
gitDirtyFlag = true        wailsApp.Events.Emit(name, payload)
(Lazy debounced sweep)            │
                                  ▼
                     frontend/src/lib/wails.ts (EventsOn)
                                  │
                                  ▼
                   workspaceStore / sessionStore
                                  │
                                  ▼
                             UI Re-render
```

---

## 8. Web Landing Page & Documentation Site (`web/`)

Proyek dilengkapi dengan landing page mandiri di direktori `web/` yang dideploy ke Cloudflare Pages:

- **Framework**: Vite + React 19 + Tailwind CSS v4 + TypeScript.
- **Aesthetic Direction**: High-density engineering aesthetic (monokromatik, deep graphite, tanpa gradien neon "AI Slop").
- **Live Assets**: Memuat 7 screenshot beresolusi Retina (2782x1822) dari UI aplikasi desktop asli di `web/public/`.
- **Hosting**: Cloudflare Pages ([https://forge-ade.pages.dev](https://forge-ade.pages.dev)).
- **Deployment**: Dideploy via Wrangler CLI (`bunx wrangler pages deploy dist --project-name=forge-ade`) atau otomatis melalui GitHub integration Cloudflare Pages.

---

## 9. Development & Verification Commands

Sebelum melakukan commit atau rilis, jalankan siklus verifikasi penuh:

```bash
# Verifikasi backend Go
go build ./... && go vet ./... && gofmt -l .
go test ./...

# Verifikasi frontend desktop React
cd frontend && bun run test && bun run lint && bun run build

# Verifikasi web landing page
cd web && bun run build
```
