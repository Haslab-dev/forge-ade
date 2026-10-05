# ForgeADE Frontend

React 19 + TypeScript + Vite + Tailwind CSS v4 UI for the ForgeADE Wails desktop app.

## Structure

- `src/components/` — feature UI: `agent/` (agent + Terminal Session surfaces), `editor/`, `session/`, `settings/`, `modals/`, `shell/` (title/status bars)
- `src/stores/` — state: `workspaceStore.tsx` (workspace context), `sessionStore.ts` (terminal sessions), `agentRegistryStore.ts`
- `src/services/` — `apiBridge.ts` (backend calls), `goAgentSession.ts` (Go harness chat), `agentEngine.ts` (legacy webview engine)
- `src/lib/` — `wails.ts` (the only importer of generated bindings), `editorGlobals.ts`, `browser.ts`, `toast.tsx`
- `src/index.css` — design tokens and the Tailwind `@theme` bridge (see `../docs/DESIGN_SYSTEM.md`)
- `bindings/` — generated Wails bindings (build artifact, gitignored)

## Commands

```sh
bun install
bun run dev      # vite dev server (usually driven by `make dev` at the repo root)
bun run build    # tsc -b && vite build
bun run lint     # oxlint
```
