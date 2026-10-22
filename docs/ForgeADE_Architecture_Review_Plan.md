# ForgeADE Architecture Review & Refactoring Plan

**Status:** Completed & Merged into main (v1.0.5)  
**Scope:** Go 1.26 backend, Wails v3 IPC/runtime, React 19 + TypeScript frontend  
**Purpose:** Menetapkan batas tanggung jawab backend dan frontend, menertibkan kontrak lintas-layer, dan membuat performa serta lifecycle lebih mudah diuji tanpa rewrite total.

> Dokumen ini menyatukan review arsitektur Go backend dan React frontend berdasarkan dokumentasi struktur serta inspeksi statis terhadap implementasi yang direview. Konfirmasi kembali temuan terhadap branch terbaru sebelum mengubah kode. Potensi bottleneck performa adalah target profiling, bukan klaim bahwa masalah runtime sudah terbukti.

---

## 1. Executive Summary

ForgeADE memiliki fondasi yang layak dipertahankan:

- Go menangani filesystem, Git, PTY, proses CLI, remote access, database, browser/computer automation, dan koordinasi agent.
- Wails v3 menjadi batas RPC antara Go dan React.
- Backend sudah mempunyai package domain yang cukup terpisah, misalnya `git`, `terminal`, `workspace`, `explorer`, `remotefs`, `browseruse`, `computeruse`, `acp`, dan `automations`.
- RakitKode dipisahkan sebagai agent engine, bukan dicampur seluruhnya ke UI desktop.
- Frontend menggunakan TypeScript, generated Wails bindings, Zustand pada sebagian state, Effect-TS pada sebagian service, serta CodeMirror 6 dengan lifecycle imperatif.

**Rekomendasi utama:** jangan melakukan rewrite Clean Architecture dari nol. Perjelas ownership state, pisahkan composition root dari adapter Wails, konsolidasikan batas API frontend, dan buat kontrak event/error eksplisit.

Prioritas refactor:

1. **P0 — Correctness:** operasi gagal tidak boleh terlihat sukses; error dan fallback harus dapat dibedakan.
2. **P1 — Boundary:** satu jalur IPC canonical, dependency backend yang eksplisit, dan event bertipe.
3. **P1 — State ownership:** satu pemilik per state domain; hindari context/store yang terlalu terpusat.
4. **P2 — Lifecycle:** runtime session dipisahkan dari lifecycle tampilan UI.
5. **P2 — Measured performance:** ukur streaming, editor, terminal, startup, dan perubahan surface sebelum optimasi.

## 2. Arsitektur Saat Ini

```text
┌──────────────────────────── React 19 / WebView ─────────────────────────────┐
│ Feature UI: Agent | Editor | Git | Terminal | Remote | Settings | Automation│
│                 │                                                          │
│                 ▼                                                          │
│       Stores / Hooks / Domain Services                                     │
│                 │                                                          │
│                 ▼                                                          │
│       Wails client / API facade / Effect integration                      │
└─────────────────┬──────────────────────────────────────────────────────────┘
                  │ Wails v3 method invocation
                  ▼
┌────────────────────────── Go / Wails Runtime ───────────────────────────────┐
│ internal/app/*_api.go  +  eventbridge.go                                    │
│                 │                                                          │
│                 ▼                                                          │
│ Domain packages: workspace, explorer, git, terminal, agentsession, acp,    │
│ remotefs, dbviewer, watcher, automations, browseruse, computeruse, ...      │
│                 │                                                          │
│                 ▼                                                          │
│ Infrastructure / OS / process I/O / RakitKode integration                  │
└────────────────────────────────────────────────────────────────────────────┘
```

Secara umum, pemisahan native backend dan presentation frontend sudah benar. Risiko utamanya berada pada **pusat koordinasi yang terlalu besar**, **kontrak error yang tidak seragam**, dan **ownership state/lifecycle yang tumpang tindih**.

---

## 3. Backend Review — Go

### 3.1 `internal/app` berpotensi menjadi God Object

`internal/app` berisi banyak file API dan wiring untuk workspace, explorer, filesystem, Git, terminal, agent, MCP, plugins, skills, memory, indexing, remote filesystem, database, browser/computer use, automations, dan lainnya. Pola exported methods di `*App` cocok untuk Wails, tetapi berisiko menjadikan `App` sekaligus RPC facade, composition root, koordinator banyak subsystem, dan tempat business logic bertumpuk.

**Rekomendasi:** pertahankan `internal/app/*_api.go` sebagai adapter IPC yang tipis. Method yang diekspos sebaiknya melakukan validasi boundary dasar, memanggil domain service/manager, lalu mengubah hasil menjadi DTO untuk frontend. Business logic baru jangan ditambahkan ke `App` jika memiliki domain yang jelas.

Tidak perlu memindahkan setiap file ke package baru. Mulai dari method yang memiliki banyak branching, mengorkestrasi beberapa subsystem, atau sulit dites tanpa membuat `App` besar.

### 3.2 Pisahkan bootstrap/composition root dari Wails adapter

`main.go` dan `internal/app/app.go` menangani lifecycle aplikasi dan inisialisasi dependencies. Pola ini masih bisa bekerja, tetapi wiring yang tumbuh di constructor akan membuat startup semakin sulit dipahami dan diuji.

Target konseptual:

```text
main.go
  └── internal/bootstrap
        ├── Create infrastructure and shared dependencies
        ├── Initialize domain managers
        ├── Wire domain/event subscriptions
        └── Create internal/app.App
              ├── Wails RPC adapters (*_api.go)
              ├── eventbridge.go
              └── startup/shutdown hooks
```

**Prinsip:** constructor/bootstrap mengembalikan error ke pemanggil. Hindari `log.Fatalf` di constructor atau package yang bukan entry point karena itu membuat kegagalan sulit ditangani atau diuji. Penentuan apakah kegagalan harus menghentikan aplikasi dilakukan di `main.go`.

### 3.3 Pertahankan package domain yang sudah sehat

Package seperti berikut sudah mencerminkan batas tanggung jawab yang masuk akal dan tidak perlu digabung hanya untuk mengurangi jumlah package:

- `internal/git`: status, diff, konflik, graph, discard.
- `internal/terminal`: PTY process/session management.
- `internal/agentsession`: konfigurasi dan persistensi external CLI sessions.
- `internal/acp`: Agent Client Protocol management.
- `internal/workspace` dan `internal/explorer`: workspace lifecycle dan filesystem exploration.
- `internal/remotefs`: SSH/SFTP.
- `internal/browseruse` dan `internal/computeruse`: browser automation dan kontrol OS.
- `internal/automations`: scheduling/automation.

Package adalah batas domain yang valid jika API-nya jelas dan tidak memiliki dependency melingkar. Hindari membuat satu package `services` generik untuk menampung semua logic; itu hanya memindahkan God Object ke nama lain.

### 3.4 Bedakan tiga jenis session

ForgeADE memiliki tiga konsep yang berbeda:

1. **PTY terminal session** — proses shell dan stream input/output.
2. **Agent CLI session** — sesi tool CLI pihak ketiga yang berjalan dalam terminal/runtime terkait.
3. **Built-in agent chat session** — percakapan/turn loop yang dijalankan RakitKode.

Jangan satukan manager mereka hanya karena semua memakai istilah *session*. Mereka memiliki resource, lifecycle, persistence, dan state machine berbeda. Jika diperlukan tampilan lintas-sesi, buat DTO/view model atau registry agregasi di atas manager yang terpisah, tanpa menggabungkan implementasi inti.

### 3.5 Perjelas lifecycle dan cancellation ownership

Background workers, watchers, PTY readers, event subscriptions, browser sessions, dan automation schedulers membutuhkan aturan shutdown yang konsisten.

- Tetapkan owner yang memulai dan menghentikan setiap goroutine/worker.
- Ikat worker ke lifecycle context yang sesuai.
- Pastikan shutdown dapat melepas event subscriptions dan resource OS.
- Bedakan pembatalan lifecycle dari pembatalan request bisnis. Operasi API yang tidak boleh dibatalkan oleh navigasi UI tidak boleh bergantung pada lifecycle komponen React.
- Uji shutdown, proses yang keluar, dan worker yang gagal saat startup.

Jangan menerapkan satu kebijakan “tidak pernah cancel” pada semua proses. Cancel untuk teardown/lifecycle berbeda dari membatalkan operasi karena komponen UI unmount.

### 3.6 Event bridge perlu mempertahankan typed contract

`eventbridge.go` merupakan batas yang tepat untuk menerjemahkan event internal menjadi event frontend Wails. Pastikan typing tidak hilang ketika data melewati bridge.

Disarankan:

- Nama event didefinisikan sebagai constants atau registry terpusat.
- Payload menggunakan structs bertipe pada backend.
- Event yang dikirim ke frontend memiliki DTO yang terdokumentasi.
- Hindari `map[string]any` dan string-key lookups untuk payload domain inti bila kontrak bisa dibuat eksplisit.
- Event frontend tidak menjadi jalur untuk mengeksekusi business logic tersembunyi.

Tidak semua event memerlukan framework event baru. Typed structs, constants, dan adapter yang sederhana sudah memadai.

### 3.7 Integrasi RakitKode sebagai dependency boundary

RakitKode sebaiknya tetap menjadi owner engine LLM/agent loop, tool registry, MCP/skills, dan kemampuan yang memang milik engine. ForgeADE memiliki tanggung jawab integrasi desktop: lifecycle, workspace context, izin/approval UI, terminal, native tools, dan presentasi sesi.

Hindari menduplikasi logic agent engine di `internal/app`. Jika ForgeADE membutuhkan perilaku khusus, utamakan adapter atau extension point yang eksplisit daripada fork logic yang sama di beberapa tempat.

---

## 4. Frontend Review — React 19 / TypeScript

### 4.1 Ownership state terlalu terpusat

`src/stores/workspaceStore.tsx` menggabungkan banyak concern: workspace, surface/navigation, editor tabs/layout, agent state, Git/diff, dan bagian integrasi/UI. `src/hooks/store.ts` juga memuat Zustand stores untuk beberapa state global. Beberapa konsep yang berdekatan berpotensi memiliki lebih dari satu sumber kebenaran.

Risiko:

- Perubahan context dapat memicu render ulang consumer yang hanya membutuhkan bagian state lain.
- Sinkronisasi event backend, store, dan komponen menjadi sulit ditelusuri.
- Perubahan pada satu fitur menyentuh provider pusat yang besar.

**Rekomendasi:** tetapkan satu owner per domain. Gunakan Zustand selectors yang granular untuk state lintas-komponen dan berfrekuensi perubahan tinggi. Pertahankan local state untuk state lokal dan Context untuk dependency atau state dengan subscription scope kecil.

Target ownership awal:

| Store / Owner | Tanggung jawab |
|---|---|
| `workspaceStore` | Workspace aktif, recents, workspace metadata |
| `editorStore` | Tabs, active tab, pane layout, metadata buffer |
| `agentStore` | Transcript, status execution, tool/subagent state |
| `sessionStore` | PTY session metadata dan status yang dibutuhkan UI |
| `gitStore` bila diperlukan | Cache/status Git untuk consumer frontend |
| `integrationsStore` bila diperlukan | MCP, plugins, skills metadata/config state |
| `uiStore` | Modal, selected surface, panel visibility, UI preferences |

Ini bukan instruksi untuk membuat semua store sekaligus. Gabungkan atau pisahkan berdasarkan ownership dan pola konsumsi nyata.

### 4.2 `apiBridge.ts` terlalu besar dan mencampur kebijakan komunikasi

`src/services/apiBridge.ts` mencakup berbagai domain, sementara jalur komunikasi frontend memiliki generated Wails bindings, facade `lib/wails.ts`, wrapper Effect-TS, serta fallback di sejumlah operasi. Nilai default yang dikembalikan ketika operasi gagal dapat menyamarkan perbedaan antara “data memang kosong” dan “operasi gagal”.

**Rekomendasi:** pertahankan satu transport/client Wails yang canonical dan pecah service berdasarkan domain. Effect-TS dapat dipertahankan bila digunakan secara konsisten; jangan menggabungkan semua kebijakan error dan fallback ke satu file besar.

Struktur konseptual:

```text
src/
├── platform/
│   └── wailsClient.ts
├── features/
│   ├── workspace/workspace.api.ts
│   ├── filesystem/filesystem.api.ts
│   ├── git/git.api.ts
│   ├── terminal/terminal.api.ts
│   ├── agent/agent.api.ts
│   ├── integrations/integrations.api.ts
│   ├── remote/remote.api.ts
│   └── database/database.api.ts
└── effect/   # Pertahankan bila memang menjadi shared infrastructure
```

Tidak semua feature membutuhkan file `api.ts`, store, hook, dan types sekaligus. Tambahkan file jika ada boundary nyata, bukan untuk memenuhi template folder.

### 4.3 Standardisasi hasil operasi dan error

Operasi backend harus membedakan hasil sukses, hasil kosong yang sah, dan kegagalan. Jangan mengembalikan `success: true`, array kosong, `false`, atau nilai default hanya untuk menyembunyikan kegagalan transport/operasi.

Gunakan salah satu kontrak yang konsisten—misalnya typed result/error atau exception yang dinormalisasi pada satu boundary—dan pastikan setiap consumer menangani kegagalan sesuai konteks UI.

Fallback hanya dipertahankan jika runtime alternatif memang requirement produk. Jika fallback hanya mencoba jalur lain saat Wails binding seharusnya tersedia, lebih baik gagal dengan error yang jelas daripada membuat masalah kontrak sulit ditemukan.

### 4.4 `AgentActiveSessionView.tsx` memegang banyak lifecycle

Komponen sesi agent menangani lebih dari rendering transcript, termasuk durasi execution, notifikasi, pencarian thread, auto-scroll, event browser, dan sinkronisasi background tasks.

Pisahkan berdasarkan lifecycle yang benar-benar independen, misalnya:

- `AgentSessionView`: komposisi layout.
- `AgentTimeline`: tampilan transcript dan tool output.
- `useAgentExecution`: status/durasi execution.
- `useAgentSessionEvents`: subscription dan sinkronisasi event.
- `useAgentAutoScroll`: scroll-follow dan interaksi scroll pengguna.
- `useAgentThreadSearch`: pencarian/shortcut.
- `useBackgroundTasks`: sinkronisasi tasks/schedules bila domain tersebut memang tampil di area ini.

Jangan membuat hook kecil untuk setiap handler. Tujuannya memisahkan tanggung jawab dengan lifecycle yang berbeda dan mempermudah testing.

### 4.5 Persistent mounting: pertahankan runtime, kendalikan aktivitas UI

`App.tsx` mempertahankan beberapa surface tetap mounted demi menjaga session dan state. Ini bisa menjadi pilihan yang benar bagi terminal/agent. Namun, runtime yang wajib berjalan harus dibedakan dari efek presentasi yang hanya diperlukan ketika surface terlihat.

Contoh aktivitas UI yang patut dievaluasi saat surface tersembunyi: auto-scroll, `ResizeObserver`, animasi, timer khusus presentasi, dan pengukuran layout. Jangan menghentikan proses agent atau terminal hanya karena user berpindah surface.

### 4.6 Pertahankan CodeMirror; perjelas boundary integrasinya

`CodeEditorPane.tsx` menggunakan API imperatif CodeMirror, cleanup lifecycle editor, debounce buffer flushing, dan `requestAnimationFrame` untuk sebagian pembaruan visual. Pola ini sesuai untuk editor performa tinggi; tidak perlu rewrite editor menjadi controlled React input.

Area yang perlu diaudit:

- Sinkronisasi dokumen ketika tab berubah atau agent melakukan external edit.
- Pemulihan selection dan undo history saat konten disinkronkan.
- Ownership antara dokumen CodeMirror, buffer metadata/store, dan perubahan eksternal.
- Biaya perhitungan diff/change markers pada file besar.

Jangan menyimpan seluruh isi buffer ke global store pada setiap karakter tanpa kebutuhan konkret. Lindungi sinkronisasi dengan tests sebelum mengubah cara editor menangani external edits.

### 4.7 Typed event adapters dan subscription lifecycle

Event Wails untuk agent, terminal, workspace, watcher, dan Git perlu payload types yang jelas. Komponen presentasi tidak seharusnya menjadi tempat tunggal bagi event global yang harus tetap diproses saat surface tersembunyi.

Alur frontend yang disarankan:

```text
Wails Events
     │
     ▼
Typed Event Adapters / Domain Subscriptions
     │
     ▼
Feature Store / Domain State
     │
     ▼
Selectors / Hooks
     │
     ▼
React Components
```

Subscription harus memiliki cleanup yang eksplisit. Event yang merupakan runtime/session state sebaiknya dimiliki oleh domain service/store yang sesuai; event presentasi boleh mengikuti lifecycle komponen.

---

## 5. Target Architecture Lintas-Layer

```text
React Feature Components
        │
        ▼
Feature Hooks / Selectors / Stores
        │
        ▼
Feature API / Domain Services (TypeScript)
        │
        ▼
Canonical Wails Client + Generated Bindings
        │
        ▼
internal/app/*_api.go  (thin IPC adapters)
        │
        ▼
Go Domain Packages / Managers
        │
        ├── Filesystem / Workspace / Git
        ├── PTY / Agent CLI / ACP
        ├── Remote FS / DB
        ├── Browser / Computer Use
        └── Automations / RakitKode integration

Go Domain Events
        │
        ▼
Typed Event Bridge (Wails)
        │
        ▼
Frontend Domain Event Adapters
        │
        ▼
Feature Store / UI Selectors
```

### Kontrak arsitektur

1. **UI tidak mengakses generated bindings secara acak.** Operasi melewati canonical frontend client/domain API.
2. **Wails adapter tidak memiliki business logic kompleks.** Method RPC meneruskan kerja ke package domain.
3. **Setiap state punya owner.** Jangan memelihara salinan canonical state yang sama pada Context, Zustand, dan komponen.
4. **Event punya kontrak payload.** Hindari string-key/dynamic payload untuk domain inti bila typed DTO memungkinkan.
5. **Runtime session terpisah dari UI lifecycle.** Pergantian surface tidak boleh menghentikan proses yang harus terus berjalan.
6. **Error tidak disamarkan sebagai sukses.** Fallback harus eksplisit dan teruji.
7. **Generated bindings tetap generated.** Jangan mengedit generated output secara manual; perbaiki sumber Go dan regenerate.
8. **Optimasi dibuktikan dengan pengukuran.** Jangan menganggap context rerender, persistent mounting, ataupun diff calculation sebagai bottleneck terkonfirmasi sebelum profiling.

---

## 6. Struktur Folder Target

Struktur berikut adalah arah evolusi, bukan migrasi massal yang harus dilakukan sekaligus.

### Backend Go

```text
.
├── main.go
├── internal/
│   ├── bootstrap/          # Dependency wiring dan inisialisasi komposisi
│   ├── app/                # Adapter Wails: *_api.go, eventbridge.go
│   ├── workspace/          # Domain workspace
│   ├── explorer/           # Directory/file exploration
│   ├── watcher/            # File-system watcher
│   ├── git/                # Git operations/status/diff
│   ├── terminal/           # PTY runtime
│   ├── agentsession/       # External CLI session configuration/persistence
│   ├── acp/                # Agent Client Protocol
│   ├── browseruse/         # Browser automation
│   ├── computeruse/        # Native computer interaction
│   ├── remotefs/            # SSH/SFTP
│   ├── dbviewer/            # SQLite inspection
│   └── ...                  # Domain packages lain tetap terpisah
└── cmd/
```

Tidak perlu menambahkan package `bootstrap` jika wiring saat ini masih sederhana. Tambahkan ketika dependency initialization sudah membuat `App` sulit dipahami atau dites.

### Frontend React

```text
frontend/src/
├── app/                    # Application shell/composition
├── platform/
│   └── wailsClient.ts      # Canonical transport boundary
├── features/
│   ├── workspace/
│   ├── editor/
│   ├── agent/
│   ├── terminal/
│   ├── git/
│   ├── filesystem/
│   ├── integrations/       # MCP, plugins, skills, discovery
│   ├── remote/
│   ├── database/
│   └── automations/
├── shared/
│   ├── components/
│   ├── hooks/
│   ├── lib/
│   └── types/
├── effect/                 # Infrastruktur bersama jika tetap dibutuhkan
├── stores/                 # Hanya state lintas-domain yang benar-benar global
└── test/
```

Direktori yang sudah ada boleh dipertahankan selama migrasi. Pindahkan modul hanya ketika hal tersebut memperbaiki ownership atau dependency direction. Tidak semua feature harus memiliki seluruh subdirektori.

---

## 7. Refactoring Roadmap

### Phase 0 — Baseline dan guardrails

- Catat command verifikasi yang lulus pada branch awal.
- Tambahkan tests untuk kontrak error/fallback pada operasi penting.
- Catat baseline render/latency untuk editor, streaming agent, terminal output, startup, dan perpindahan surface.
- Jangan membuat perubahan struktur besar sebelum baseline tersedia.

**Acceptance:** tersedia baseline build/test serta repro yang stabil untuk area yang akan disentuh.

### Phase 1 — Benahi correctness dan error contracts

- Audit method frontend/backend yang mengembalikan default saat terjadi error.
- Pastikan kegagalan tidak menghasilkan status sukses.
- Standardisasi typed error/result dan error-to-UI mapping.
- Hapus fallback yang tidak didukung requirement produk.
- Tambahkan regression tests.

**Acceptance:** consumer dapat membedakan keberhasilan, hasil kosong yang sah, dan kegagalan. Tidak ada fake-success pada jalur yang direfactor.

### Phase 2 — Canonical frontend API boundary

- Tetapkan satu Wails client.
- Pecah facade besar berdasarkan domain.
- Konsolidasikan penggunaan Effect-TS dan error handling.
- Pastikan komponen tidak memanggil generated bindings secara langsung di luar boundary yang ditetapkan.

**Acceptance:** dependency transport dapat ditelusuri per domain dan kebijakan fallback tidak berbeda secara tak sengaja antar-method.

### Phase 3 — State ownership frontend

- Inventarisasi state di `workspaceStore.tsx`, `hooks/store.ts`, `sessionStore.ts`, dan store lain.
- Tetapkan owner untuk active workspace, theme, editor tabs, agent sessions, dan panel/navigation.
- Migrasikan satu domain dalam satu waktu menggunakan selectors.
- Hapus state duplikat setelah consumer berpindah dan tests lulus.

**Acceptance:** satu konsep canonical state memiliki satu owner yang jelas; perubahan satu domain tidak memicu update pada consumer yang tidak terkait tanpa alasan.

### Phase 4 — Backend composition dan Wails adapter

- Audit kompleksitas `App` dan `NewApp()`.
- Pisahkan bootstrap jika wiring sudah terlalu besar.
- Pindahkan orchestration/business logic yang tepat ke domain package/manager.
- Pertahankan `*_api.go` sebagai RPC adapter tipis.
- Perjelas lifecycle startup/shutdown dan worker ownership.

**Acceptance:** constructor gagal dengan error yang dapat ditangani; domain logic dapat dites tanpa harus membuat seluruh Wails application jika tidak diperlukan.

### Phase 5 — Typed event flow dan lifecycle UI

- Audit event names dan payload Go/frontend.
- Definisikan DTO/payload bertipe untuk event inti.
- Pisahkan subscription runtime dari subscription presentasi.
- Pisahkan lifecycle pada agent view.
- Pastikan cleanup berjalan ketika owner memang dihentikan.

**Acceptance:** event tidak bergantung pada komponen tersembunyi untuk memperbarui state session; subscription dan worker berhenti pada lifecycle yang tepat.

### Phase 6 — Profiling dan optimasi terukur

- Gunakan React Profiler untuk streaming agent, navigation, Git panel, dan perubahan editor.
- Ukur latency input pada file besar serta efek sinkronisasi dokumen.
- Profiling CPU/memory backend untuk startup, watcher, terminal stream, dan indexing.
- Optimalkan hanya bottleneck yang dapat direproduksi.

**Acceptance:** setiap optimasi memiliki baseline dan hasil sesudah perubahan. Jika tidak ada perbaikan terukur atau ada regresi correctness, jangan pertahankan hanya karena kodenya tampak lebih kompleks.

---

## 8. Testing & Verification

Jalankan command sesuai tooling yang tersedia pada branch aktif.

### Backend

```bash
gofmt -l .
go vet ./...
go test ./...
go build ./...
```

Tambahkan/pertahankan tests khusus untuk:

- error propagation dari `*_api.go` ke frontend contract;
- bootstrap dan startup failure;
- session shutdown, process exit, dan worker cleanup;
- event payload/type mapping;
- operasi Git/filesystem yang sebelumnya mempunyai fallback.

### Frontend

```bash
cd frontend
bun run lint
bun run build
# Jalankan script test yang tersedia di package.json, misalnya:
bun run test
```

Pastikan nama script test cocok dengan `package.json` branch terkini.

Regression targets:

- error state dibedakan dari empty state;
- workspace switching ketika agent atau terminal masih aktif;
- event subscription tidak ganda setelah mount/unmount;
- state antar-session tidak bocor;
- external file edits tidak merusak buffer, selection, atau undo history;
- streaming transcript tidak menyebabkan pekerjaan UI berlebih yang terukur;
- surface tersembunyi tetap mempertahankan runtime yang diperlukan tanpa menjalankan seluruh efek presentasi tanpa kebutuhan.

---

## 9. Keputusan yang Tidak Disarankan

- Jangan rewrite semua folder untuk memenuhi diagram arsitektur baru.
- Jangan menambahkan library state management kedua/ketiga tanpa masalah yang jelas.
- Jangan menyimpan seluruh editor buffer ke global store setiap karakter tanpa alasan konkret.
- Jangan menggabungkan PTY session, external agent CLI session, dan built-in agent session menjadi satu manager generik.
- Jangan menghentikan agent/terminal runtime hanya karena React surface tersembunyi.
- Jangan menghapus Effect-TS sebelum evaluasi apakah ia memberikan value yang nyata.
- Jangan melakukan virtualisasi transcript atau optimasi worker sebelum profiling.
- Jangan menjadikan `internal/app` atau `apiBridge.ts` sebagai satu tempat untuk semua aturan bisnis.
- Jangan mengubah generated Wails bindings secara manual.

---

## 10. Definition of Done

Refactor dianggap selesai untuk suatu domain ketika:

- ownership state/dependency dapat dijelaskan dengan jelas;
- tidak ada business logic baru yang ditempatkan pada adapter hanya demi kemudahan;
- error dan hasil kosong memiliki makna yang berbeda;
- event payload serta cleanup lifecycle memiliki tests;
- build, lint, dan tests terkait lulus;
- perilaku workspace/session/editor tetap benar;
- perubahan performa yang diklaim didukung oleh pengukuran;
- struktur baru mengurangi coupling yang nyata, bukan hanya memindahkan kode ke lebih banyak file.

## 11. Kesimpulan

ForgeADE tidak membutuhkan Clean Architecture yang berlapis-lapis di setiap fitur. Ia membutuhkan **domain boundaries yang jelas dan konsisten dari Go sampai React**.

Pertahankan package backend yang sudah terpisah. Tipiskan `internal/app` menjadi adapter Wails dan pindahkan composition/wiring keluar ketika kompleksitas menuntutnya. Di frontend, tetapkan ownership state, jadikan Wails client sebagai jalur transport canonical, pecah API berdasarkan domain, serta pisahkan lifecycle session dari lifecycle presentasi UI. Pertahankan CodeMirror dan runtime session yang sudah bekerja; ubah hanya bagian yang memiliki masalah terukur atau boundary yang tidak jelas.

Review ini berbasis dokumentasi arsitektur dan inspeksi statis, bukan hasil menjalankan seluruh test suite atau profiling runtime. Performa belum boleh dinyatakan meningkat sebelum baseline dan pengukuran sesudah refactor dibandingkan.
