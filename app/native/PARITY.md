# ForgeADE Native — Parity Matrix vs Wails (1:1 Complete Port Audit)

Status perbandingan fitur dan komponen terhadap `app/webview/frontend/src` (referensi 1:1).  
Framework: **MyGo** (`github.com/egoist/mygo`) desktop shell native dengan internal editor & diff bawaan (`github.com/egoist/mygo/ui/editor`).  
Legend: ✅ = 1:1 Parity Lengkap, 🟡 = Subset, ❌ = Belum ada.

---

## 1. Internal Editor & Diff Component (Bawaan MyGo)
| Fitur / Lokasi | Komponen Internal MyGo | Status | Deskripsi |
|---|---|---|---|
| Main Code Editor (`editor_view.go`) | `mygoeditor.View(c, doc.Editor)` | ✅ | Full native editor with multi-pane split, tab management, breadcrumbs, minimap, cursor positioning, diagnostics, inlays |
| Tool Read File Block (`agent_view.go`) | `mygoeditor.View(c, edState)` | ✅ | `GetOrCreateReadOnlyEditor`: Read-only syntax highlighting, line numbers, and native text selection for file reads |
| SideFile Drawer Editor (`drawer.go`) | `mygoeditor.View(c, doc.Editor)` | ✅ | In-drawer editor for quickly viewing/editing workspace files |
| Git Changes Diff (`drawer.go`) | `mygoeditor.DiffView(c, d)` | ✅ | Staged & unstaged git diff with side-by-side or unified layout, inline word diffs, hunk actions |
| Commit History Diff (`surfaces.go`, `drawer.go`) | `mygoeditor.DiffView(c, d)` | ✅ | Per-file diff accordion inside commit inspector with addition/deletion stats |
| SideFile Diff Mode (`drawer.go`) | `mygoeditor.DiffView(c, d)` | ✅ | Toggle between editor view and git diff view for any workspace file |
| Agent Tool Edit Diff (`agent_view.go`) | `mygoeditor.DiffView(c, d)` | ✅ | `GetOrCreateBufferDiff`: Instant word/line diff between `OldString` and `NewString` with syntax styling |
| Tool Patch / Unified Diff (`agent_view.go`) | `mygoeditor.DiffView(c, d)` | ✅ | Multi-hunk unified diff parsing (`mygoeditor.ParsePatch`) inside agent tool execution cards |
| PR Detail Diff (`git_pr_worktrees.go`) | `mygoeditor.DiffView(c, d)` | ✅ | Multi-file PR patch breakdown into interactive per-file diff views |
| Inline AI Diff Preview (`inline_ai_bar.go`) | `mygoeditor.Buffer.Edit` | ✅ | ⌘I inline AI diff manipulation directly on editor buffer |

---

## 2. Screens & Main Surfaces
| Screen / Surface | File Native | Status | Fitur & Parity |
|---|---|---|---|
| **Agent View** | `app/native/pkg/ui/agent_view.go` | ✅ | Chat stream, Markdown renderer, thinking block (duration/tokens/copy CoT), folded tool traces, approval bar (⌘Y), turn changes card (checkpoint accept/revert), floating task tracker, thread find bar (⌘F), turn scrubber |
| **Editor View** | `app/native/pkg/ui/editor_view.go` | ✅ | Activity bar (Explorer, Search, Git, Database, Settings), file tree with context menus, multi-pane split, tab bar, status bar, minimap, inline AI bar (⌘I) |
| **Git Client Surface** | `app/native/pkg/ui/surfaces.go`, `git_pr_worktrees.go` | ✅ | Full surface in both Agent & Editor modes (⌘⇧G / ActivityBar), Toolbar (Branch, Fetch/Pull/Push, Sync), Changes tab, Commit History with 10-color graph lanes & drag resizer, Branches tab, PRs tab, Worktrees tab |
| **Workflow Kanban Board** | `app/native/pkg/ui/board_view.go` | ✅ | Backlog, Todo, In Progress, In Review, Done columns, card details, priority badges, drag/move, create task modal |
| **Settings Screen** | `app/native/pkg/ui/settings_screen.go`, `settings_integrations.go`, `subagents_settings.go` | ✅ | 13 Sections: Providers (live fetch /models), ACP Agents, Agent CLIs, MCP Servers, Skills, Subagents/Roles, Automations, Browser Use, Computer Use, Custom Slash Commands, Lifecycle Hooks, Code Indexing, Appearance, Shortcuts |
| **Automations View** | `app/native/pkg/ui/automations_view.go` | ✅ | Scheduled bots management, cron builder, preset chips, run history, Run Now trigger, enable/disable toggle |
| **Terminal Surface** | `app/native/pkg/ui/surfaces.go`, `terminal_view.go` | ✅ | Fullscreen PTY terminal session with scrollback and ANSI coloring |

---

## 3. Drawers & Secondary Panels
| Drawer / Pane | File Native | Status | Fitur & Parity |
|---|---|---|---|
| **Left Sidebar** | `app/native/pkg/ui/sidebar.go` | ✅ | + New Task (⌘N), Automations, Task search & filter pills, Project groups with hover actions (Explore/New/Delete/Reveal), Task rows with brand SVG icons, status dots, diff badges, hover action buttons (Pin/Stop/Restart/Fork/Snooze/Archive/Delete), timeline flat mode toggle, bottom dock (7 icons + theme) |
| **Right Action Drawer** | `app/native/pkg/ui/drawer.go` | ✅ | Dynamic tabs: Changes (stage/commit/AI msg/diff/hunk revert), Terminal (tabs, clear, restart, ask agent, snippets), Side Chat, SideFile preview (editor/diff/image/markdown/pdf), Browser (WKWebView), Deck/Motion (.deck viewer), Subagent (child agent drilldown) |
| **Subagent Pane** | `app/native/pkg/ui/drawer.go` | ✅ | Child agents list, live chat view, thought stream, tool executions, approval prompt, stop button (dynamic tab, only appears when subagents exist) |
| **Bottom Terminal Drawer** | `app/native/pkg/ui/terminal_view.go` | ✅ | Bottom collapsible terminal with PTY multi-tabs, clear, restart, ⌥S snippets, Ask Agent AI |

---

## 4. Modals & Dialogs (All 16 Ported)
| Modal / Dialog | File Native | Status | Trigger / Shortcut |
|---|---|---|---|
| **Command Palette Modal** | `app/native/pkg/ui/modals.go` | ✅ | ⌘K / ⌘P (files, `>` commands, `@` symbols, `:` line jump) |
| **Project Search Modal** | `app/native/pkg/ui/modals.go` | ✅ | ⇧⌘F (Files/Content mode, Aa/.* /ab toggles, glob, replace-all) |
| **Model / Provider Palette** | `app/native/pkg/ui/modals.go` | ✅ | ⌘M / Composer model pill (live provider models + effort levels) |
| **New Task / Agent Picker** | `app/native/pkg/ui/modals.go`, `sidebar.go` | ✅ | ⌘N (instant composer focus) / ⇧⌘N (agent picker with ACP & CLI agents, workspace friction modal eliminated) |
| **Usage & Quotas Modal** | `app/native/pkg/ui/modals.go` | ✅ | Footer gauge / Palette (tokens gauge, weekly/5h limits, Force Compact) |
| **Agent Question Modal** | `app/native/pkg/ui/modals.go`, `agent_view.go` | ✅ | Interactive agent question with recommended choice, options, custom input |
| **Tool Approval Bar / Permission Modal** | `app/native/pkg/ui/agent_view.go`, `modals.go` | ✅ | Approve / Always Approve / Deny (⌘Y) |
| **Rules Manager Modal** | `app/native/pkg/ui/modals.go` | ✅ | Footer rules icon / Palette (syncs AGENTS.md, .cursorrules, CLAUDE.md) |
| **Terminal Snippets Modal** | `app/native/pkg/ui/terminal_snippets_modal.go` | ✅ | ⌥S / Terminal header (catalog, search, run in active terminal, copy, add/edit) |
| **SSH Connect Modal** | `app/native/pkg/ui/ssh_modal.go` | ✅ | ⇧⌘R / Palette (Host, port, user, key_file/password/agent auth) |
| **SSH Quick Menu Modal** | `app/native/pkg/ui/ssh_modal.go` | ✅ | Remote indicator / Palette (Terminal, change dir, refresh, upload, disconnect) |
| **Remote File Uploader Modal** | `app/native/pkg/ui/ssh_modal.go` | ✅ | SSH Quick Menu / Palette (browse files/folders, queue list, size format, SFTP upload) |
| **Theme Picker Modal** | `app/native/pkg/ui/modals.go` | ✅ | Footer palette icon / Palette (theme palettes + dark/light toggle) |
| **New Board Task Modal** | `app/native/pkg/ui/modals.go` | ✅ | Kanban "+ Add task" / Palette (title, description, status, priority) |
| **New File / Folder Modals** | `app/native/pkg/ui/modals.go` | ✅ | File tree context menu (create file/folder with validation) |
| **Rename / Delete Task Modals** | `app/native/pkg/ui/modals.go` | ✅ | Task row hover menu (rename title, confirm delete session) |

---

## 5. Agent Integrations & Session Parity
| Integrasi | Native Port | Status | Detail |
|---|---|---|---|
| **Brand SVG Icons** | `icons.RenderProviderIcon` | ✅ | 1:1 SVGs from `ProviderIcon.tsx` (Google Antigravity rainbow, Claude, OpenAI, OpenCode, Pi, OhMyPi, Cursor, Forge Spark) |
| **ACP Session Resumption** | `core/acp/manager.go`, `app/native/pkg/agent/acp.go` | ✅ | Resumes existing conversation via `--conversation <id>` (AGY) / `--resume <id>` (Claude) / `session/resume` (ACP) |
| **Agent CLI Session Resumption** | `app/native/pkg/agent/agent_cli.go` | ✅ | Preserves and restarts sessions with `--continue` / `--resume` / `resume --last` |
| **Composer Slash Commands** | `app/native/pkg/ui/composer_menu.go` | ✅ | Built-in commands (`/goal`, `/plan`, `/compact`, `/schedule`, `/browser`, `/boost`) + ACP slash commands + MCP `/mcp:` + Skills `/skill:` |
| **Composer @-Mentions** | `app/native/pkg/ui/composer_menu.go` | ✅ | Fuzzy workspace file matching, inserts `@path` |
| **Permission Modes** | `app/native/pkg/ui/agent_view.go` | ✅ | 4 modes: Full Autonomy, Ask Every Time, Plan-Only, Read-Only |
| **Prompt History & Stash** | `app/native/pkg/ui/agent_view.go` | ✅ | ↑/↓ history (100 entries), Alt+S prompt stash (20 entries) |
| **Steer Queue** | `app/native/pkg/ui/agent_view.go` | ✅ | Add to steer queue, edit, delete, send while agent is running |

---

## 6. Extra Right Sidebar & Document Viewers
| Viewer / Fitur | File Native | Status | Detail |
|---|---|---|---|
| **In-App Browser Viewer** | `app/native/pkg/ui/browser_viewer.go` | ✅ | Native WebKit WebView (`mygo.WebView`), URL bar, back, forward, reload, zoom control, external browser open |
| **PDF Document Viewer** | `app/native/pkg/ui/pdf_viewer.go` | ✅ | Native PDF rendering via WebKit, page info, file size badge, "Open in Default App", "Reveal in Finder" |
| **Presentation & Motion Viewer** | `app/native/pkg/ui/deck_viewer.go` | ✅ | Supports Open Design slides (`deck.html`, `.deck.html`) & HyperFrames motion (`motion.html`, `.motion.html`, `.mp4`). PostMessage slide navigation (`od:slide`: next/prev) & motion controls (`od:motion-control`: play/pause/restart) |
| **Right Drawer Tabs Context Menu** | `app/native/pkg/ui/drawer.go` | ✅ | Tab switching, "+" popup menu with Browser, PDF, Terminal, Changes, Deck, Subagents |
| **Settings Model Synchronization** | `app/native/pkg/ui/settings_screen.go`, `model_modal.go` | ✅ | Custom providers (e.g. Kenari 98 models) & local LLMs instantly synced to Model Picker (⌘M), provider tabs, search, and active session |
| **Traffic Lights Header Collision Fix** | `app/native/pkg/ui/settings_screen.go` | ✅ | TitleBar clearance dynamically calculated (`leftPad = 80px` on macOS) to prevent collision with window controls |
| **Git Client Toolbar Deduping** | `app/native/pkg/ui/surfaces.go` | ✅ | Fixed double title render (`HistoryHistory`), single reconciled tab button per view |
| **Worked / Tools Auto-Collapse** | `app/native/pkg/ui/agent_view.go` | ✅ | Thought blocks & tool calls auto-collapse into clean summary pill on turn completion |
