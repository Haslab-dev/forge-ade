# Repository Guidelines

## Project Overview
ForgeADE is a native AI development workspace refactored to the **Vercel Native SDK**. It provides a high-performance native IDE shell (Zig) that hosts a modern React/Vite frontend via a WebView, powered by a TypeScript daemon (Bun) for backend services. It is currently undergoing an architectural shift, porting backend services from TypeScript to Go.

## Architecture & Data Flow
ForgeADE employs a three-tier architecture:
1.  **Native Shell (`src/`)**: Compiled Zig binary (via Native SDK). Owns windowing, security policies, native bridge (ptys, fs watcher, LSP, dialogs), and app lifecycle.
2.  **Daemon Layer (`src/server/`)**: A Bun process forked by the Zig shell. Serves as the main JSON-RPC-style backend (over HTTP POST `/api/invoke` and WebSockets `/ws`) for IDE features (Git, Agents, LSP).
3.  **Frontend (`frontend/`)**: React/Vite application loaded in a WebView. Communicates via the `window.zero` bridge for native integration (fs, clipboard, terminal) and talks to the Bun daemon for IDE logic.
4.  **Service Port Target (`internal/`)**: Contains a parallel Go implementation of agent orchestration, git engine, MCP, and search, which is the long-term target for IDE backend services.

## Key Directories
- `src/`: Native app shell code (Zig), app-core (TypeScript model), and server daemon entry.
- `frontend/`: React/Vite/TypeScript frontend source code.
- `internal/`: Business logic modules (Agent, Git, MCP, Terminal, Search). Contains Go port implementations.
- `app.zon`: App manifest (identity, capabilities, bridge permissions).
- `build.zig`: Authoritative build graph (Zig 0.16.0).

## Development Commands
- `native check`: Verify core.ts, app.native, and app.zon.
- `native test`: Run tests and model contract verification.
- `native dev`: Start live development with hot reload.
- `make build`: Build frontend + compile ReleaseFast native binary.
- `make package`: Package for macOS (DMG bundle).

## Code Conventions & Common Patterns
- **Native Bridge (`frontend/src/lib/native.ts`)**: The sole integration point. Use `invokeBackendStrict` for operations requiring robust error handling (Git/Auth); use `invokeBackend` for lenient facade operations.
- **RPC Dispatching**: Backend logic is centralized in `ForgeServer.handleMethod` (a large switch statement), routing to various service managers.
- **State Management**: Elm-style (Model/Msg/Update) for the native shell core (`src/core-state.ts`).
- **Bridge Permissions**: Any new bridge command *must* be enumerated in `app.zon`.
- **Git Engine**: Git operations stream stdout (no object graphs) and use a per-repo status cache with 5s TTL to prevent index contention.

## Important Files
- `src/main.zig`: Native shell entry point, bridge handlers, and daemon management.
- `src/server/index.ts`: ForgeServer RPC dispatcher.
- `frontend/src/lib/native.ts`: The WebView ↔ native transport and facade layer.
- `internal/agent/agent.go`: The core Go agent engine (target for all IDE backend services).
- `app.zon`: Native SDK manifest and security policy.

## Runtime/Tooling Preferences
- **Runtime**: Bun (scripts/daemon), Zig 0.16.0 (native build).
- **Frontend**: React 19, Vite 8, TypeScript 6, Tailwind CSS 4, Zustand (state), xterm.js (terminals), CodeMirror 6 (editor).
- **Package Manager**: Bun (scripts/dependencies); the native build system (`build.zig`) manages frontend install/build stages.

## Testing & QA
- **Native**: `native test` (model contract verification).
- **Frontend**: Happy-dom based tests for components; `npm test` scripts in `frontend/package.json`.
- **Linting**: oxlint and Prettier (with organize-imports plugin).
