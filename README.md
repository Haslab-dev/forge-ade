# ForgeADE (pytauri migration)

ForgeADE is an AI development workspace: React/Vite frontend + a **Python
backend** hosted by [pytauri](https://pytauri.github.io/pytauri/) using
**pytauri-wheel** (precompiled Tauri dylib — no Rust toolchain) and packaged
with **PyInstaller**.

This is the `migrate/pytauri` branch: the whole Native SDK backend (fs, git,
PTY terminal, LSP, agent/LLM, MCP, external ACP agents, skills, usage,
search, syntax) is re-hosted as Python commands in `python/src/forge_ade/`.
The React frontend is unchanged apart from the zero-bridge shim at the top of
`frontend/src/lib/native.ts`, which routes `window.zero` through Tauri IPC.

The previous Native SDK implementation is preserved on the
`backup/pre-pytauri` branch; the vendored pytauri-wheel reference example
lives in `reference/`.

## Layout

- `python/src/forge_ade/` — the backend: one `bridge` Tauri command
  dispatching every method by name (`fs.readDir`, `terminal.spawn`,
  `services.ListAgentSessions`, ...), the same `{ok, result}` JSON contract
  and the same event names (`terminal.data`, `fs.change`, `services.agent`)
  the Native SDK used.
- `frontend/` — the React app (unchanged); builds to `frontend/dist` which
  `python/src/forge_ade/Tauri.toml` points at for production.
- `reference/` — vendored upstream tauri-app-wheel example (toolchain docs).
- `run_app.py` + `forge_ade.spec` — PyInstaller packaging.

## Workflow

```bash
make env            # python3 -m venv .venv
make install        # forge_ade (editable) + jurigged + pyinstaller
make node-install   # bun install for the frontend
```

### Development (hot reload)

```bash
make frontend-dev   # terminal A: vite on :5173
make dev            # terminal B: the Tauri window loads vite, jurigged
                    # hot-patches Python edits without restart
```

### Production

```bash
make build   # vite build -> frontend/dist
make run     # python -m forge_ade
```

### Package (PyInstaller)

```bash
make package   # optional first: make icon (macOS .icns)
# -> dist/ForgeADE.app
```

### Interactive shell with the env

```bash
make shell     # or: source .venv/bin/activate
```

## Bridge contract

`window.zero.invoke(command, params)` → Tauri `invoke("bridge",
{command, payload})` → Python handler → `{"ok": true, "result": ...}` /
`{"ok": false, "error": {"message": ...}}` (JSON string). Backend → frontend
events: `emit_str_to("main", name, json)` with the Native SDK event names.
