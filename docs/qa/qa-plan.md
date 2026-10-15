# QA Plan — ForgeADE v0.9.5 (Build 37)

- **Cycle date:** 2026-10-15
- **Epic:** FORGE-100 (Quality: Test Coverage & Stabilization)
- **Environments:** macOS 15.5 (arm64), Go 1.27 toolchain, bun 1.4.2, Wails v3.0.0-beta.28, `wails3 dev` (vite :9245)

## 1. Objectives

1. Establish a runnable automated test suite for backend **and** frontend (neither existed for the frontend; 6 `internal/*` packages had zero tests).
2. Exercise the three flagship surfaces — **agent mode**, **side panels**, **editor** — end to end and record defects.
3. Produce a Jira-style defect log with reproducible evidence.

## 2. Scope

| Area | In scope | Approach |
| --- | --- | --- |
| Go business logic (`internal/*`) | All 31 packages; deep tests for the 6 untested ones | Unit / integration tests (`go test`) |
| Frontend pure logic (`lib/*`, `services/*`, `stores/*`) | fuzzy, time, utils, gitDecorations, terminalSettings, diff-highlight, toast, goAgentSession, sessionStore | vitest unit tests (jsdom + Testing Library) |
| Agent mode (forge chat) | Session creation, composer, send pipeline, error surfacing, task list, task menu | GUI automation (browser driver over `wails3 dev`) |
| Side panels | Right Quick-Action drawer (Git/Terminal/Side-conversation/Browser chooser), left sidebar, activity bar, bottom terminal toggle | GUI automation + code inspection |
| Editor | Mode switch, file tree empty state, scratch file, split actions, Git panel, terminal dock | GUI automation + code inspection |
| Shell surfaces | Command palette (⌘K), New Task modal (⌘N), Automations modal, Settings (⌘,), theme toggle | GUI automation |
| Build system | `go build/vet/gofmt`, `wails3` server-mode build, frontend `tsc + vite build`, oxlint | Scripted runs |
| Out of scope | Real LLM turn quality (provider unreachable in env), SSH/SFTP remotes, in-app browser engine, MCP servers, DB viewer | Deferred (FORGE-128) |

## 3. Entry / exit criteria

**Entry:** clean worktree on `main` @ `cafeebe` (v0.9.5, build 37); `go build ./...`, `go vet`, `gofmt`, existing `go test ./...`, frontend `lint` + `build` all green (verified before starting).

**Exit:** all new tests green; no regressions in pre-existing suites; every defect logged with a ticket; skip-marked tests wired to ticket IDs so fixes auto-activate them.

## 4. Strategy highlights

- **Bug-detector tests:** where current behavior is defective, the *desired* behavior is written as a skipped test referencing its `FORGE-xxx` ticket. Fixing the bug flips the test on — no silent enshrinement of bugs.
- **Behavior-pinning:** deterministic current behavior that is *questionable but not clearly wrong* (e.g. `parseFilePath` trailing-punctuation edge) is pinned by an active test with a comment, so future refactors can't drift silently.
- **GUI automation** drove the real app served by `wails3 dev` at `http://localhost:9245` (Wails HTTP bindings), using DOM-snapshot-driven actions plus screenshots for layout review. Backend state was cross-checked on disk (`~/.forge/sessions/`, `git status`) to classify findings.

## 5. Risks / limitations

- The configured chat model (`qwen2.5-coder:latest`, Ollama) was not running during the cycle, so a *successful* agent turn could not be observed; failure-path UX was tested instead (FORGE-127).
- `@`-mention and `/`-command menus need a workspace with indexed files / custom commands; the QA workspace had neither (deferred, FORGE-128).
- GUI session ran against the user's real app data; only self-created artifacts were mutated (one task created and deleted via the UI).
