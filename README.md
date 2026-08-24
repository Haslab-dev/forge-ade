# ForgeADE Native
test
ForgeADE is a native AI development workspace refactored to the **Vercel Native SDK**, using `src/core.ts` for native app logic and state, declarative Native markup in `src/app.native`, and a rich React/Vite frontend loaded via WebView.

## Architecture

- **Manifest (`app.zon`)**: Declares app identity, permissions (`view`, `command`, `window`), capabilities (`webview`, `native_views`, `gpu_surfaces`, `js_bridge`), security policies, frontend assets, and shell windows.
- **Native Core (`src/core.ts`)**: Pure TypeScript app-core (`Model`, `Msg`, `update`, `subscriptions`) compiled ahead of time into native code (no JS runtime in the shipped binary).
- **Native Markup (`src/app.native`)**: Declarative UI container and status bar bound to the compiled core model.
- **WebView Frontend (`frontend/`)**: React + Vite + TypeScript interface communicating with the desktop host through the `window.zero` bridge (`frontend/src/lib/native.ts`).

## Development

```sh
# Run typecheck and subset verification on core.ts, app.native, and app.zon
native check

# Run tests and model contract verification
native test

# Build frontend and compile ReleaseFast native binary
make build
# or:
cd frontend && bun run build && cd .. && native build

# Start live development with hot reload
native dev

# Package for macOS (DMG bundle)
make package
```

## Bridge API

The frontend connects to native capabilities via `window.zero.invoke(...)`:
- **Dialogs**: `native-sdk.dialog.openFile`, `native-sdk.dialog.saveFile`, `native-sdk.dialog.showMessage`
- **Clipboard**: `native-sdk.clipboard.readText`, `native-sdk.clipboard.writeText`
- **OS / Shell**: `native-sdk.os.openUrl`, `native-sdk.os.revealPath`
- **Windows / WebViews**: `window.zero.windows`, `native-sdk.webview.*`
- **Forge APIs**: Workspace management, file operations, terminal sessions, Git, AI agents, MCP, and search tooling.
