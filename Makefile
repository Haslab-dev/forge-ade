.PHONY: env install activate shell node-install frontend-dev dev build run package icon clean reset help

APP      := forge_ade
VENV     := .venv
PY       := $(VENV)/bin/python
PIP      := $(VENV)/bin/pip
JURIGGED := $(VENV)/bin/jurigged
PYINSTALLER := $(VENV)/bin/pyinstaller

help:
	@grep -E '^[a-zA-Z_-]+.*:.*?##' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

# ── Environment ────────────────────────────────────────────────────
env: ## create the virtualenv (.venv)
	python3 -m venv $(VENV)

install: ## install the app (editable) + jurigged + pyinstaller into .venv
	$(PIP) install --upgrade pip
	$(PIP) install -e "./python[dev]"

activate: ## print the activation command
	@echo "source $(CURDIR)/$(VENV)/bin/activate"

shell: ## open an interactive shell with the env activated
	@. $(CURDIR)/$(VENV)/bin/activate && exec $$SHELL

node-install: ## install frontend deps (bun)
	cd frontend && bun install

# ── Development ────────────────────────────────────────────────────
frontend-dev: ## vite dev server on :5173 (the Tauri window loads this in dev)
	cd frontend && bunx vite --host 127.0.0.1

dev: ## run the app with jurigged hot reloading (run frontend-dev in another terminal)
	PYTHONUNBUFFERED=1 FORGE_ADE_DEV=1 $(JURIGGED) -v --poll 0.5 -w python -m $(APP)

# ── Build & Package ────────────────────────────────────────────────
build: ## build the frontend (production assets)
	cd frontend && bun run build

run: ## run the app in production mode (uses frontend/dist)
	$(PY) -m $(APP)

icon: ## generate macOS .icns from the app icon (needed for make package)
	mkdir -p icons.iconset
	sips -z 16 16     python/src/$(APP)/icons/icon.png --out icons.iconset/icon_16x16.png
	sips -z 32 32     python/src/$(APP)/icons/icon.png --out icons.iconset/icon_16x16@2x.png
	sips -z 32 32     python/src/$(APP)/icons/icon.png --out icons.iconset/icon_32x32.png
	sips -z 64 64     python/src/$(APP)/icons/icon.png --out icons.iconset/icon_32x32@2x.png
	sips -z 128 128   python/src/$(APP)/icons/icon.png --out icons.iconset/icon_128x128.png
	sips -z 256 256   python/src/$(APP)/icons/icon.png --out icons.iconset/icon_128x128@2x.png
	sips -z 256 256   python/src/$(APP)/icons/icon.png --out icons.iconset/icon_256x256.png
	sips -z 512 512   python/src/$(APP)/icons/icon.png --out icons.iconset/icon_256x256@2x.png
	sips -z 512 512   python/src/$(APP)/icons/icon.png --out icons.iconset/icon_512x512.png
	sips -z 1024 1024 python/src/$(APP)/icons/icon.png --out icons.iconset/icon_512x512@2x.png
	iconutil -c icns icons.iconset -o icons/$(APP).icns
	rm -rf icons.iconset

package: ## PyInstaller onedir app bundle -> dist/
	$(PYINSTALLER) forge_ade.spec --noconfirm

# ── Cleanup ────────────────────────────────────────────────────────
clean: ## remove venv, build artifacts
	rm -rf $(VENV) dist build icons.iconset
	find python -name "__pycache__" -type d -prune -exec rm -rf {} +

reset: ## kill stale dev processes (vite holding 5173, old native app, agents)
	-pkill -f 'forge-ade-native' 2>/dev/null || true
	-pkill -f 'vite --host 127.0.0.1' 2>/dev/null || true
	-pkill -f 'bunx vite' 2>/dev/null || true
	-pkill -f 'node.*/frontend/node_modules/.bin/vite' 2>/dev/null || true
	-pkill -f 'jurigged' 2>/dev/null || true
	-rm -rf frontend/node_modules/.vite 2>/dev/null || true
	@echo "Reset complete. Run 'make dev' (with 'make frontend-dev') to restart."
