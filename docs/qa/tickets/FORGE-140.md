# FORGE-140 — Agent sandbox workspace (shipped)

- **Type:** Story · **Priority:** Medium · **Status:** ✅ Done · **Component:** go/agent, frontend
- **Description:** The built-in agent had no scratch space — scratch edits landed in the user's workspace or nowhere. Shipped a per-project sandbox layout:
  - `internal/sandbox` — `~/.forge/sandbox/<project-slug>/` with `tmp/` (auto-pruned after 7 days at app startup), `thinking/`, `coding/`, `logs/`. Provisioned on workspace open, session create, and lazily per turn.
  - Agent integration: `buildProjectContext` injects a "Sandbox workspace" section (absolute path + subdir purposes) into every turn's system prompt; **file write/edit calls targeting the sandbox skip the approval pause** (bash stays gated — commands can't be path-scoped).
  - Bindings: `GetSandboxDir(projectPath)`, `ClearSandbox(projectPath)` (`internal/app/sandbox_api.go`).
  - Frontend: task menu "Open sandbox folder" / "Copy sandbox path"; Settings → General "Agent sandbox" card with Reveal + two-step Clear (`ConfirmActionButton`).
  - Tests: 7 Go tests (`internal/sandbox`) covering provisioning idempotency, same-name project separation, `Under` containment (incl. fail-closed), and sweep semantics (old tmp removed, fresh kept, other subdirs never touched).
