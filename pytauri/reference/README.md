# PyTauri-Wheel reference app (migrate/pytauri)

Vendored from https://github.com/pytauri/pytauri/tree/main/examples/tauri-app-wheel
(adapted: PyPI `pytauri-wheel == 0.8.*` — precompiled Tauri dylib, **no Rust
toolchain needed**; concrete npm versions instead of pnpm `catalog:`).

This is the toolchain bootstrap for migrating ForgeADE's backend to Python.

## Setup

```bash
cd pytauri
make env          # python3 -m venv .venv
make install      # app (editable) + jurigged + pyinstaller into .venv
make node-install # bun install for the frontend
```

## Development (hot reload)

```bash
make frontend-dev   # terminal A: vite dev server on :1420 (strict port)
make dev            # terminal B: app via jurigged, TAURI_APP_WHEEL_DEV=1
```

Edit Python in `python/src/tauri_app_wheel/` — jurigged hot-patches function
bodies into the running app (body edits only; NEW commands / signature
changes need an app restart, because `@commands.command()` validates pydantic
signatures at registration).

## Production

```bash
make build   # vite build -> python/src/tauri_app_wheel/frontend
make run     # python -m tauri_app_wheel
```

## Package (PyInstaller)

```bash
make package  # optional first: make icon (generates the macOS .icns)
# -> dist/tauri-app-wheel.app and dist/tauri-app-wheel/
```

## Interactive shell with the env activated

```bash
make shell    # or: source .venv/bin/activate
```

## Layout

- `frontend/` — Vite frontend (vanilla TS, calls Python via
  `tauri-plugin-pytauri-api`'s `pyInvoke`); builds into the Python package.
- `python/src/tauri_app_wheel/` — the app: `Commands` registry, plugin init,
  `Tauri.toml`, `capabilities/`, `icons/`.
- `tauri_app_wheel.spec` + `run_app.py` — PyInstaller packaging.
