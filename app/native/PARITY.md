# ForgeADE Native — Parity Matrix vs Wails

Status perbandingan fitur terhadap `app/webview/frontend/src` (referensi 1:1).
Legend: ✅ = 1:1, 🟡 = ada tapi subset, ❌ = belum ada.

## Agent Mode
| Fitur | Status | Catatan |
|---|---|---|
| Sidebar: New task/Automations rows, Task list header + search/filter | ✅ | |
| Project selector + sort + quick actions + filter pills | ✅ | |
| Project groups, hover actions (explore/new/delete), reveal 6+Show more | ✅ | |
| Task rows: brand ikon provider, status dot, pin, diff badge, Approval chip, progress bar | ✅ | |
| Hover actions: Stop/Restart/Pin/Fork/Snooze/Archive/Delete | ✅ | |
| Bots section + footer dock (help/usage/board/git/rules/settings/theme) | ✅ | |
| Timeline list mode (flat chronological) | ❌ | group mode saja |
| Notification sound on user message | ❌ | minor |
| Composer: slash menu (built-in + ACP + MCP + skills) | ✅ | katalog: /goal /plan /compact /schedule /browser /boost + ACP commands + /mcp: + /skill: |
| Slash actions (settings/setup/usage/model/clear/compact) | 🟡 | /model via chip; sisanya dikirim ke agent |
| @-mention menu | ✅ | fuzzy workspace files, insert @path |
| "+" popover: Attach Files/Active/Workspace | 🟡 | webview + skills/mcp/git/terminal insertions |
| Permission mode chip 4-mode | ✅ | |
| Stash: Alt+S + chip + popover + persist | ✅ | |
| Model picker fetch /models live dari provider | ✅ | |
| Effort selector (Off/Low/Med/High) | ✅ | di ⌘M |
| Auto-Verify chip | ✅ | |
| Context gauge + Usage modal + Force Compact | ✅ | |
| Queue/Steer/Stop/Send | ✅ | |
| Prompt history ↑/↓ persist 100 | ✅ | |
| Prompt stash Alt+S persist 20 | ✅ | |
| Composer drafts per session persist | ✅ | |
| Steer queue persist + edit/delete/steer | ✅ | drag-reorder ❌ |
| Drag & drop files ke composer | 🟡 | via window OnFileDrop |
| Paste image | ✅ | |
| Message: user bubble + copy, attachment chips | ✅ | |
| Work pill (Working/Worked Ns · N steps) | ✅ | |
| Thinking block (duration/tokens, copy CoT) | ✅ | |
| Tool cards: bash/read/edit/write/search views, failed state | ✅ | |
| Folded tool traces + summary + Show/Hide | ✅ | |
| Approval bar (Approve/Always/Deny, ⌘Y) | ✅ | |
| Questions card + full modal (options/recommended/custom) | ✅ | |
| Turn changes card + per-file → drawer diff + Accept/Revert all (checkpoint) | ✅ | |
| Find bar ⌘F + turn scrubber rail | ✅ | |
| Floating task tracker | ✅ | |
| Subagent spawn cards (dedicated render) | 🟡 | tampil sebagai generic tool rows |
| Thumbs up/down feedback | ❌ | minor |
| XML dialect auto untuk provider kustom | ✅ | webview: manual SetAgentDialect |

## Right Drawer
| Fitur | Status |
|---|---|
| Tabs + per-tab × + "+" menu + drag resize + close | ✅ |
| Changes pane: stage/unstage/commit/AI message/diff/hunk revert | ✅ |
| Side chat + quick prompts | ✅ (model picker 🟡 label saja) |
| Files pane (filter, open-files strip, new file/folder) | ✅ |
| Side file: editor + Diff toggle + image/markdown preview | ✅ |
| Browser tab (navigate/zoom/close) | ✅ |
| Subagent pane (list) | 🟡 webview: full chat + permission card |
| Terminal tab: Clear/Restart/± tab | ✅ Ask Agent ❌ |
| Deck (deck.html) + HTML preview tab | 🟡 deck → browser tab |

## Editor Mode
| Fitur | Status |
|---|---|
| Activity bar + badges, file tree lazy + context menus, rename/delete | ✅ |
| Search pane: Files/Content mode, Aa/.* /ab toggles, glob, replace-all | ✅ |
| Git pane (branch/changes/history), Database pane | ✅ |
| Tab bar: pin/read-only/drag-reorder/close menu | ✅ |
| Split panes N + drag divider | ✅ (cross-pane tab drag ❌) |
| Breadcrumbs, minimap, inline AI ⌘I (chips/accept/reject/retry) | ✅ |
| Format Document (Shift+Alt+F), Edit with AI button | ✅ |
| Status bar: Follow Agent, chips, branch, model | ✅ |
| Ghost text AI completion | ❌ butuh streaming ke editor buffer |
| CodeLens + AI diagnostic autofix | ❌ |
| Go to Definition/References + Peek | ❌ |
| Change gutter (saved-vs-buffer diff bars) | ❌ mygo editor: cek native support |

## Git Client
| Fitur | Status |
|---|---|
| Toolbar: branch pill, Fetch/Pull/Push, remote badge, refresh | ✅ |
| Changes/History/Branches/PRs/Worktrees tabs | ✅ |
| PRs: list/detail/diff/comments/review/merge/close/reopen/checkout/create | ✅ |
| Worktrees: list/create/remove(force) | ✅ |
| Graph lanes (10-color) + drag resizer | ✅ |
| Hunk revert + conflict resolve + checkpoints | ✅ |

## Modals & Settings
| Fitur | Status |
|---|---|
| Palette: files/commands(@ > : modes)/symbols/line | ✅ |
| Project search ⇧⌘F (toggles/glob/replace-all/grouped) | ✅ |
| New Task 2-step (agents + workspace) | ✅ |
| SSH connect + quick menu + upload | ✅ |
| Rules manager + sync AGENTS.md | ✅ |
| Usage & Quota + Force Compact | ✅ |
| Settings: Providers (fetch models), ACP agents, MCP CRUD, Skills, Plugins, Memory, Automations, Sub-Agents, Appearance, Shortcuts | ✅ |
| Skills create + discovery import | ❌ |
| Sub-agents create/edit | ❌ (list+delete saja) |
| Browser Use / Computer Use / Hooks / Indexing / Custom commands sections | ❌ |
| Terminal snippets modal | ❌ |
| Automations: run now + cron builder + history | 🟡 |

## Persistensi
| Item | Status |
|---|---|
| Theme (appearance/palette/chat width/diff scheme) | ✅ ui-prefs.json |
| Sidebar/drawer widths + visibility, follow-agent, view modes, filter/sort | ✅ ui-prefs.json |
| Prompt history/stash/steer queue/composer drafts | ✅ JSON per store |
| Sessions, recent workspaces, SSH hosts, settings, board, rules | ✅ |

## Diketahui belum 1:1 (prioritas berikutnya)
1. Ghost-text completion + CodeLens + AI diagnostic autofix (butuh streaming ke editor buffer)
2. Go-to-Definition/References + Peek dialog (butuh index query per lokasi kursor)
3. Subagent chat pane penuh (permission card + stop) — kini list saja
4. Automations editor penuh (cron builder + Run now + history table)
5. Terminal snippets modal
6. Deck/HTML preview tab khusus (deck kini diarahkan ke browser tab)
7. Timeline sidebar mode, thumbs feedback, notification sound (minor)
