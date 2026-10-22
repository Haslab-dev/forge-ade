# ForgeADE Architecture & File Structure

ForgeADE adalah aplikasi desktop AI Development Environment (ADE) berbasis **Wails v3** (Go 1.26 backend + WebKit/WebView React 19 frontend). Dokumen ini merangkum struktur pohon berkas (*file tree*) lengkap untuk Go backend dan frontend, serta penjelasan arsitektur sistem secara mendalam.

---

## 1. High-Level Architecture

Arsitektur ForgeADE mengadopsi pola *Separation of Concerns* antara native backend execution (Go) dan responsive presentation/UI layer (React).

```
┌──────────────────────────────── WebView (React 19 + Tailwind v4) ────────────────────────────────┐
│                                                                                                  │
│  UI Surfaces: Workspace (Agent Chat / CLI) | Editor (CodeMirror 6) | Automations | Settings     │
│                                                                                                  │
│  Components (agent/, editor/, git/, session/, modals/)                                           │
│         │                                      ▲                                                 │
│         ▼                                      │                                                 │
│  State / Stores (workspaceStore, sessionStore, Effect-TS services)                               │
│         │                                      │                                                 │
│         ▼                                      │                                                 │
│  Facade: lib/wails.ts  ◄───────────────►  apiBridge.ts                                           │
└─────────┬──────────────────────────────────────┴─────────────────────────────────────────────────┘
          │ Method Invocation (IPC)              ▲ EventsOn(name, callback)
          ▼                                      │
┌──────────────────────────────── Wails v3 Runtime Bridge ─────────────────────────────────────────┐
│  internal/app/*_api.go (Exported methods on *App struct)                                         │
│  internal/app/eventbridge.go (Maps typed Go bus events -> Wails frontend events)                 │
└─────────┬──────────────────────────────────────▲─────────────────────────────────────────────────┘
          │                                      │ Bus.Emit(...)
          ▼                                      │
┌──────────────────────────────── Go Backend (internal/*) ─────────────────────────────────────────┐
│                                                                                                  │
│  • Session & Terminal : internal/terminal (PTY), internal/agentsession (CLI configurations)      │
│  • File & VCS         : internal/explorer, internal/workspace, internal/watcher, internal/git   │
│  • Remote & Database  : internal/remotefs (SSH/SFTP), internal/dbviewer (SQLite)                │
│  • Agent & Automations: internal/acp, internal/automations, internal/discovery, commitmsg        │
│  • Desktop OS Control : internal/browseruse (In-app WKWebView/CDP), internal/computeruse (macOS) │
│                                                                                                  │
│  Extracted Agent Engine: github.com/Haslab-dev/RakitKode (LLM, Tools, Harness, MCP, Skills)      │
└──────────────────────────────────────────────────────────────────────────────────────────────────┘
```

---

## 2. Go Backend: Complete File Tree

Struktur direktori dan file Go backend mencakup root aplikasi (`main.go`, `go.mod`), CLI tooling harness (`cmd/`), dan implementasi fitur backend di `internal/`:

```
.
├── go.mod
├── go.sum
├── main.go
├── cmd/
│   └── shelltest
│       ├── main.go
│       ├── scenes_anim.go
│       ├── scenes_color.go
│       └── scenes_cursor.go
└── internal/
    ├── acp
    │   ├── drivers
    │   ├── client.go
    │   ├── manager.go
    │   ├── manager_test.go
    │   └── types.go
    ├── agentsession
    │   ├── config.go
    │   ├── disk.go
    │   ├── manager.go
    │   ├── manager_test.go
    │   ├── settings.go
    │   └── types.go
    ├── app
    │   ├── acp_api.go
    │   ├── agent_api.go
    │   ├── agent_quota.go
    │   ├── agent_quota_test.go
    │   ├── agentclirc_api.go
    │   ├── app.go
    │   ├── app_test.go
    │   ├── automations_api.go
    │   ├── board_api.go
    │   ├── board_test.go
    │   ├── board_tools.go
    │   ├── board_watcher.go
    │   ├── board_watcher_test.go
    │   ├── browser_api.go
    │   ├── browser_tools.go
    │   ├── cli_models_enricher.go
    │   ├── cli_models_enricher_test.go
    │   ├── commandcode_discovery.go
    │   ├── commandcode_discovery_test.go
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
    ├── automations
    │   ├── automations.go
    │   ├── automations_test.go
    │   └── cron.go
    ├── browseruse
    │   ├── cdp.go
    │   ├── cdp_engine.go
    │   ├── iab_click_test.go
    │   ├── iab_counter_test.go
    │   ├── iab_darwin_test.go
    │   ├── iab_hidden_test.go
    │   ├── iab_longwatch_test.go
    │   ├── iab_stormwrite_test.go
    │   ├── inapp_darwin.go
    │   ├── inapp_other.go
    │   ├── manager.go
    │   ├── manager_test.go
    │   ├── shared.go
    │   ├── snapshot.go
    │   └── testsupport_darwin.go
    ├── commitmsg
    │   ├── commitmsg.go
    │   └── commitmsg_test.go
    ├── computeruse
    │   ├── computeruse.go
    │   ├── computeruse_test.go
    │   ├── input_darwin.go
    │   ├── input_other.go
    │   ├── keys_darwin.go
    │   └── live_darwin_test.go
    ├── dbviewer
    │   ├── dbviewer.go
    │   └── dbviewer_test.go
    ├── discovery
    │   ├── discovery.go
    │   └── discovery_test.go
    ├── editor
    │   ├── editor.go
    │   └── editor_test.go
    ├── explorer
    │   ├── bench_test.go
    │   ├── explorer.go
    │   └── explorer_test.go
    ├── git
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
    ├── remotefs
    │   ├── client.go
    │   ├── manager.go
    │   ├── sshcommand_test.go
    │   ├── types.go
    │   └── upload_test.go
    ├── terminal
    │   ├── manager.go
    │   ├── manager_test.go
    │   ├── provider.go
    │   ├── provider_test.go
    │   └── session.go
    ├── watcher
    │   ├── watcher.go
    │   └── watcher_test.go
    └── workspace
        ├── manager.go
        ├── workspace.go
        └── workspace_test.go
```

---

## 3. Go Backend Architecture

Backend Go bertanggung jawab penuh atas OS-level process management, I/O filesystem, Git operations, terminal pseudo-TTY (PTY), native automation, dan koordinasi agen AI.

### 3.1 Entry Point & App Lifecycle (`main.go`, `internal/app/app.go`)
- **Memory & GC Tuning**: `main.go` mengonfigurasi `debug.SetGCPercent(50)` dan `debug.SetMemoryLimit(96 MiB)` agar konsumsi memori pada macOS tetap optimal dan tidak menumpuk heap.
- **Wails v3 Service**: Menginstansiasi `app.NewApp()` dan mendaftarkannya sebagai core service di Wails runtime.
- **Dev Server Fallback**: Menyediakan middleware reverse-proxy dengan fallback HTML loading spinner ketika Vite dev server sedang booting.
- **Single Instance & CLI Launch**: Menangani argumen path (`os.Args[1]`) untuk membuka folder proyek atau berkas `.workspace`.

### 3.2 Wire Contract Layer (`internal/app/*_api.go`)
Setiap method yang diekspor pada `*App` otomatis dibuatkan binding TypeScript oleh Wails v3. Method dibuat tipis (*thin facade*) dan mendelegasikan logika ke `internal/*`:

| File API | Domain / Tanggung Jawab |
| :--- | :--- |
| `workspace_api.go` | Manajemen folder workspace, berkas `.workspace`, recent projects, native dialogs. |
| `explorer_api.go` | Pembacaan direktori rekursif berkinerja tinggi, anotasi git-status, metadata berkas. |
| `files_api.go` | Operasi CRUD berkas, clipboard system, resolusi path absolut/relatif. |
| `git_api.go` | Status git, log commit, diff/hunk inspection, discard, commit message generator. |
| `terminal_api.go` | Manajemen PTY shell sessions (bash/zsh) dan bridging input/output terminal. |
| `session_api.go` | Agent CLI terminal session mode (omp, opencode, pi, claude code, codex). |
| `sessiondisk_api.go` | Persistensi rekaman sesi agen ke disk (`~/.forge-ade/state/agentsessions/`). |
| `browser_api.go` & `browser_tools.go` | Otomasi browser in-app (WKWebView / Chrome DevTools Protocol). |
| `computeruse_tools.go` | Native macOS mouse/keyboard actions dan window capture. |
| `database_api.go` | Inspeksi database SQLite lokal (schema, table rows, query runner). |
| `remote_api.go` | Koneksi remote SSH dan SFTP file browsing/uploading. |
| `mcp_api.go` & `discovery` | Model Context Protocol server management & auto-discovery dari tools lain. |
| `board_api.go`, `board_watcher.go` | Real-time Kanban board watcher untuk `.forge/board_tasks.json`. |
| `automations_api.go` | Workflow otomatisasi prompt tersimpan dan cron task scheduling. |
| `eventbridge.go` | Subskripsi typed event bus internal dan forwarding ke Wails WebView events. |

### 3.3 Core Internal Packages (`internal/*`)
1. **`agentsession`**: Menangani state & persistensi eksternal agent CLI (misal: OpenCode, Claude Code, Pi). Mengelola config, settings, dan riwayat sesi disk.
2. **`git`**: Engine git native tanpa dependensi exec berat; menghitung status git beranotasi cepat, branch graph lane visualizer, parser diff & per-hunk discard, dan deteksi konflik.
3. **`terminal`**: PTY session manager berbasis `creack/pty`, mengalirkan ANSI stream dan menangani resize window.
4. **`browseruse`**: Engine otomasi web dual-mode: *In-App* menggunakan native macOS WKWebView (inapp_darwin.go) dan fallback remote CDP engine (*Chrome DevTools Protocol*).
5. **`computeruse`**: Native macOS accessibility & event automation: simulasi mouse click, drag, scroll, key strokes, dan screenshot layar.
6. **`remotefs`**: SSH client & SFTP subsystem manager untuk browsing dan transfer file remote.
7. **`explorer` & `watcher`**: Tree listing dengan cache performa tinggi serta file-system watcher berbasis `fsnotify` dengan lazy git-dirty debounce sweep.
8. **`editor`**: Integrasi syntax validator (esbuild) dan formatting tools (Prettier) yang di-inject berdasarkan root workspace.
9. **`acp`**: Agent Client Protocol manager untuk orchestrating model/driver agent terdistribusi.
10. **`RakitKode` Integration**: Mesin inferensi LLM, tool registry, prompt execution, dan turn loop didelegasikan ke module public Go `github.com/Haslab-dev/RakitKode`.

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
│   ├── stores
│   │   ├── agentRegistryStore.ts
│   │   ├── composerDraftStore.ts
│   │   ├── promptStashStore.ts
│   │   ├── sessionStore.test.ts
│   │   ├── sessionStore.ts
│   │   └── workspaceStore.tsx
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

## 5. Frontend Architecture

Frontend dibangun dengan arsitektur modular, reaktif, dan terisolasi dari platform-specific code melalui facade bindings.

### 5.1 Communication & IPC Facade (`src/lib/wails.ts`, `src/services/apiBridge.ts`)
- **`src/lib/wails.ts`**: Satu-satunya titik masuk (*single gateway*) impor untuk seluruh binding Wails di `bindings/`. Mencegah coupling langsung komponen UI dengan generated code Wails.
- **`src/services/apiBridge.ts`**: Normalizer wrapper yang menyediakan typed promises dan error fallback bagi operasi UI.
- **`src/effect/`**: Lapisan fungsional berbasis Effect-TS untuk error handling, runtime environment, dan service bridge.

### 5.2 State Management (`src/stores/` & `src/hooks/`)
- **`workspaceStore.tsx`**: Central workspace context store yang mengatur:
  - Active workspace root & recents.
  - Active mode: `workspace` (Agent chat / CLI split), `editor`, `automations`, `settings`.
  - Active task kind: `forge` (built-in agent chat) vs `cli` (external terminal CLI agent).
  - Split pane layouts, pinned tabs, and open files.
- **`sessionStore.ts`**: Zustand store untuk melacak PTY terminal sessions, active process states, exit codes, and output buffers.
- **`agentRegistryStore.ts`**: Registrasi model AI, provider configs, dan subagent capabilities.
- **`composerDraftStore.ts` & `promptStashStore.ts`**: Menyimpan draft pesan dan stash prompt pengguna.

### 5.3 UI Surfaces & Components (`src/components/`)
1. **`agent/` (AI Agent Workspace Surface)**:
   - `AgentActiveSessionView.tsx`: Tampilan percakapan aktif dengan agen (markdown streaming, collapsible tool execution blocks, subagent chat panes).
   - `AgentTaskInputBar.tsx`: Input komposer multimodal dengan fitur `@-mention` (file, context, symbol).
   - `BottomTerminal.tsx`: Terminal drawer terintegrasi di bawah jendela chat agen.
   - `BrowserViewer.tsx`: Web viewer interaktif untuk memonitor aksi browser automation.
   - `WorkflowBoardView.tsx`: Visual Kanban board task tracking yang tersinkronisasi otomatis dengan `.forge/board_tasks.json`.
2. **`editor/` (Code Editor Surface)**:
   - `CodeEditorPane.tsx`: Editor teks berkinerja tinggi berbasis **CodeMirror 6** dengan dynamic syntax highlighting (Go, TS, Python, Rust, SQL, YAML, dll.).
   - `FileTree.tsx`: Explorer file pohon virtualized dengan git-status badges (modified, untracked, ignored).
   - `GitControlPanel.tsx` & `GitGraphPane.tsx`: Git branch graph dan commit operations.
   - `DatabaseViewer.tsx`: Viewer tabel basis data SQLite.
   - `MarkdownPreview.tsx`, `ImagePreview.tsx`, `PdfViewer.tsx`, `DeckMotionViewer.tsx`: Previewer multi-tipe berkas.
3. **`git/` (VCS Viewers)**:
   - `GitClientView.tsx`: Tampilan visual git client lengkap.
   - `GitDiffPane.tsx` & `GitAccordionDiffList.tsx`: Side-by-side & unified hunk diff viewer dengan syntax highlight.
4. **`session/` (Terminal Surface)**:
   - `TerminalSessionSurface.tsx`: Terminal emulator menggunakan **Xterm.js** dengan akselerasi hardware `@xterm/addon-webgl`.
5. **`modals/` & `common/`**:
   - `CommandPaletteModal.tsx`: Spotlight-style palette untuk navigasi cepat file dan aksi editor.
   - `SSHConnectModal.tsx` & `RemoteFileUploaderModal.tsx`: Dialog koneksi remote host.
   - Design System Tokens: Seluruh styling menggunakan semantic tokens Tailwind CSS v4 (`bg-panel`, `text-fg-secondary`, `border-border-subtle`).

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

## 8. Development & Verification Commands

Sebelum melakukan commit atau rilis, jalankan siklus verifikasi penuh:

```bash
# Verifikasi backend Go
go build ./... && go vet ./... && gofmt -l .
go test ./...

# Verifikasi frontend React
cd frontend && bun run lint && bun run build
```
