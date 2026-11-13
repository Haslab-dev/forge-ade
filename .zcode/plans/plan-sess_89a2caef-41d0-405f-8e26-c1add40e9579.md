# Refactor Menyeluruh — forge-ade

## Konteks & Temuan

Codebase adalah aplikasi desktop Wails v3 (Go 1.26 + React 19/TS + Tailwind v4) yang berfungsi, tapi menumpuk utang teknis dari porting cepat:

- **Backend**: `app.go` = god object 2.800 baris / ~190 method binding / 28 field. Logika persistence session (~300 baris) dan orkestrasi LLM commit-message salah tempat di root. 14 salinan ctx-guard, 3 path resolver, 2 binary-locator duplikat.
- **Frontend**: ~7.800 baris komponen legacy **terverifikasi mati** (0 importer: `components/sidebar.tsx` 2.031, `global-settings-modal.tsx` 1.454, `agent-chat.tsx`, `git-panel.tsx`, `panels/{editor,shell-screen,git-graph-panel,agent-screen,welcome}.tsx`, `components/zui/*`, dll). `style.css` + `App.css` tidak di-import dan nilainya **konflik** dengan token live. 427 `text-[Npx]` arbitrary vs token `text-ui` yang sudah ada; 105 pola `bg-white dark:bg-[#hex]` melanggar aturan token sendiri.
- **Repo**: file sampah ter-commit (~4,4 MB PNG duplikat/screenshot, `test.txt`, `test.workspace`, `ref/`, `tools_test/`, `test_tools/`, `frontend/package.json.md5`), dual lockfile (bun+npm), README basi (salah versi Wails, link `docs/` mati), tanpa LICENSE/CI/target test-lint.

**Keputusan (default karena tidak ada jawaban):** UI = konsistensi token tanpa redesign; kedalaman = refactor menyeluruh tanpa ubah perilaku (god-store & multi-service **tidak** dilakukan, dicatat sebagai debt); docs = English; hasil = commit bertahap di branch `refactor/codebase-cleanup`.

**Aturan main:** binding Wails tidak berubah (semua method tetap di `*App` package main — split hanya per file, pola yang sudah dibuktikan `session_api.go`/`browser_api.go`). Verifikasi tiap fase: `go build ./... && go vet ./... && go test ./...` + `cd frontend && bun run lint && bun run build`.

---

## Fase 1 — Hygiene repo (commit 1: `chore: remove accidental files and tidy repo hygiene`)

1. Buat branch `refactor/codebase-cleanup`.
2. `git rm`: `test.txt`, `test.workspace`, `frontend/src/test.txt`, `test_tools/`, `tools_test/`, `internal/terminal/ok-yes.txt`, `appIcon.png` (duplikat byte-identik `build/appicon.png`), `shell-agent.png`, `text-editor.png`, `frontend/package.json.md5`, `package-lock.json` (bun.lock dipertahankan — Taskfile default bun).
3. `git rm -r --cached ref/` + tambah `ref/` ke `.gitignore` (snapshot desain tetap lokal, keluar dari git).
4. Rapikan `.gitignore`: hapus entri duplikat (`.commandcode/` 2×, `frontend/dist` 2×, `.cortex` 2×), tambah `*.md5`, `ref/`, `.DS_Store`.
5. Tambah `LICENSE` (MIT — README sudah mengklaim MIT), `.editorconfig`.
6. Makefile: tambah target `test` (`go test ./...`), `vet` (`go vet ./...`), `fmt` (`gofmt -l -w`). Pindahkan `shell_test/` → `cmd/shelltest/` (lokasi idiomatik Go) + update target `shell-test`.
7. `samples/` dipertahankan (fixture kecil, 7 file); `shell_test/` dipindah, bukan dihapus.

## Fase 2 — Backend Go (commit 2: `refactor(go): split app.go into per-feature api files` ; commit 3: `refactor(go): extract event bridge, editor tooling, and commit-message package`)

**2a. Split `app.go` (2.800 → ~300 baris)** per banner yang sudah ada, mempertahankan persis semua nama method export (binding & frontend tidak terpengaruh):
`workspace_api.go` (workspace + 4 dialog), `files_api.go`, `remote_api.go` (SSH), `sessions_api.go` (PTY, menyerap `session_api.go`), `search_api.go`, `index_api.go`, `agent_api.go`, `providers_api.go`, `skills_api.go`, `plugins_api.go` (plugins + memory), `mcp_api.go`, `git_api.go`, `acp_api.go`, `automations_api.go`, `browser_api.go` (tetap). `app.go` menyisa: struct, `NewApp`, lifecycle, event-setup.

**2b. Pindahkan logika salah tempat ke `internal/`:**
- Persistence session disk (app.go:1583–1833: `SaveAgentSessionDisk`/`Load`/`Delete` + 3 helper, pengetahuan 4 layout storage) → `internal/agentsession/disk.go`. Method App jadi delegasi tipis (test `app_test.go` tetap jalan).
- `GenerateAICommitMessage` (app.go:2354–2467) → `internal/commitmsg` (fungsi murni terima llm client + diff).
- `CheckSyntax`/`FormatCode`/`findEsbuild`/`findPrettier` → `internal/editor` dengan root workspace di-inject (sekalian memperbaiki bug `os.Getwd()` pada app yang dilaunch dari Finder); satu locator binary dibagi dua.

**2c. Bersih-bersih internal (tanpa ubah API export):**
- Event bridge (app.go:1180–1190, 1299–1472) → `eventbridge.go`: ctx-guard satu tempat di `emitEvent`, tabel pemetaan konstanta `events.*` → string Wails, daftar event agent jadi data.
- Hapus 14 ctx-guard tersebar; konsolidasi 3 resolver path → 1; `SearchFilename` delegasi ke `WithOptions`; fix `OpenSSHWorkspace` yang menelan error; `AcpCloseSession` jadi alias eksplisit `AcpCancel` (method tetap ada agar binding stabil).
- Dua sistem plugin (`plugins`+`cordis` vs `harness/plugins.go`) **tidak** digabung — terlalu berisiko; didokumentasikan sebagai debt.

## Fase 3 — Frontend: hapus kode mati (commit 4: `refactor(ui): delete dead legacy components and stylesheets`)

1. **Prasyarat**: ekstrak globals dari `panels/editor.tsx` → `lib/editorGlobals.ts` (`globalOpenFile`, `globalOpenDiff`, `globalOpenConflict`, `refreshFileDiff`, `syncExternal*`) dan repoint satu importer live: `components/terminal-view.tsx`. Pindahkan `openInBrowser` dari `panels/browser-panel.tsx` → `lib/browser.ts`.
2. **Hapus** (~7.800 baris, semua terverifikasi 0 importer): `components/{sidebar,agent-chat,git-panel,agent-panel,global-settings-modal,sessions-bar,simple-modal}.tsx`, `panels/{editor,shell-screen,git-graph-panel,agent-screen,welcome,browser-panel}.tsx`, `components/zui/`, dead zustand stores di `hooks/store.ts` (simpan `useUIStore`), `style.css`, `App.css`, `happy-dom` dari devDeps.
3. Verifikasi dengan `bun run build` (tsc akan menangkap importer nyasar).

## Fase 4 — Frontend: konsistensi design system (commit 5: `style(ui): migrate to text-ui scale and semantic tokens`)

Token live di `index.css` sudah bagus (palet semantik dark/light, `@theme` bridge, skala `text-ui` 11/12/13/15/17) — yang kurang adalah penegakannya:

1. **Migrasi `text-[Npx]` → `text-ui-*`** di komponen live (427 instance; prioritas: `editor/FileTree.tsx` 36, `ForgeSettingsTab.tsx` 22, `agent/AgentActiveSessionView.tsx` 21, `diff/DiffViewer.tsx` 18, `GitGraphPane.tsx` 11, `terminal-view.tsx`, modal-modals). Pemetaan: ≤11.5px→xs, 12–12.5→sm, 13–13.5→base, 14–16→lg, ≥17→xl.
2. **Hapus warna hardcoded**: ganti pola `bg-white dark:bg-[#hex]` & hex inline dengan token semantik (`bg-panel`, `bg-elevated`, `text-foreground-secondary`, dst). Exemplar: `SettingsScreen.tsx:609,1382`, `TitleBar.tsx:172`, `SidebarFileExplorer.tsx:197`, `CommandPaletteModal.tsx:119–179`, `OpenFolderModal.tsx`, `AtMention.tsx:27`, `terminal-view.tsx:194`, `diff-view.tsx`. Ganti `text-[var(--x)]` → utilitas `@theme` yang sudah dipetakan.
3. Palet branch hardcoded di file live dicek ulang setelah editor.tsx mati (live `GitGraphPane` sudah pakai token `--graph-c*`).
4. Konsolidasi dua diff viewer (`diff/DiffViewer.tsx` live vs `components/diff-view.tsx` live) **ditunda** jika keduanya punya jalur pemakaian berbeda — dicek importer saat eksekusi; kalau `diff-view.tsx` ternyata hanya dipakai kode mati, ikut fase 3.
5. Dua library icon & god-store `workspaceStore.tsx` **tidak** disentuh (debt terdokumentasi).

## Fase 5 — Dokumentasi (commit 6: `docs: rewrite README, add ARCHITECTURE, DESIGN_SYSTEM, CONTRIBUTING, LICENSE`)

1. **README.md** ditulis ulang: stack benar (Wails v3 beta.16, Go 1.26, React 19, Tailwind v4, bun), quickstart (`make dev` / `wails3 dev`), build/test/lint, peta struktur proyek aktual, daftar fitur terkini (Terminal Session mode, agent, ACP, MCP, plugins, automations, browser-use, SSH/SFTP, git UI).
2. **docs/ARCHITECTURE.md**: overview sistem, tabel 25 package `internal/`, penjelasan 4 konsep "session" (terminal PTY / agentsession / agent chat / ACP), alur event, layering frontend, daftar debt yang diketahui (2 sistem plugin, 2 agent engine, god-store, dual icon lib, `Promise<any>` di wails.ts).
3. **docs/DESIGN_SYSTEM.md**: referensi token (palet, surface, `text-ui`, graph colors, status), aturan wajib (tanpa hex/npx hardcoded), contoh do/don't, aturan dark/light.
4. **docs/CONTRIBUTING.md**: workflow dev, konvensi commit (conventional, sesuai riwayat), gaya kode (gofmt, oxlint, prettier), cara test; plus **AGENTS.md** ringkas untuk agent AI.
5. `frontend/README.md` diganti dari boilerplate Vite.

## Fase 6 — Verifikasi akhir

- `go build ./... && go vet ./... && gofmt -l . && go test ./...` — semua hijau.
- `cd frontend && bun run lint && bun run build` — hijau.
- Build app (`wails3 build` / `make build`), luncurkan, screenshot sanity-check: mode Terminal Session, Editor, dan panel gelap/terang tetap render benar (verifikasi visual token migration).
- Laporan akhir: ringkasan perubahan per commit + sisa debt terdokumentasi.

## Risiko & mitigasi

- **Binding berubah?** Tidak — split per file dengan receiver `*App` yang sama; nama method export dijaga identik.
- **Test rusak saat split?** Method App jadi delegasi tipis; `app_test.go` dan 48 test `internal/` jadi pemeriksa regresi tiap fase.
- **Regresi visual token?** Pemetaan size konsisten + verifikasi visual dengan screenshot dark/light di akhir.
- **Commit granular** per fase → mudah di-review dan di-revert per bagian.