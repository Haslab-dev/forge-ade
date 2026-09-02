# PyTauri wheel reference app — env + build + package workflow.
# Self-contained: every target uses .venv/bin/* directly, so activating the
# venv is optional. `make activate` drops you into an activated shell.

VENV     := .venv
PY       := $(VENV)/bin/python
PIP      := $(VENV)/bin/pip
JURIGGED := $(VENV)/bin/jurigged
PYINSTALLER := $(VENV)/bin/pyinstaller
APP      := tauri_app_wheel

.PHONY: env install activate shell node-install frontend-dev dev build run package icon clean help

help:
	@grep -E '^[a-zA-Z_-]+.*:.*?##' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "  \033[36m%-14s\033[0m %s\n", $$1, $$2}'

env: ## create the virtualenv (.venv)
	python3 -m venv $(VENV)

install: ## install app (editable) + jurigged + pyinstaller into .venv
	$(PIP) install --upgrade pip
	$(PIP) install -e "./python[dev]"

activate: ## print the activation command
	@echo "source $(CURDIR)/$(VENV)/bin/activate"

shell: ## open an interactive shell with the env activated
	@. $(CURDIR)/$(VENV)/bin/activate && exec $$SHELL

node-install: ## install frontend deps (bun)
	cd frontend && bun install

frontend-dev: ## vite dev server on http://localhost:1420 (strict port)
	cd frontend && bunx vite

dev: ## run the app with jurigged hot reloading (run frontend-dev in another terminal, TAURI_APP_WHEEL_DEV=1 is set for you)
	PYTHONUNBUFFERED=1 TAURI_APP_WHEEL_DEV=1 $(JURIGGED) -v --poll 0.5 -w python -m $(APP)

build: ## build frontend assets into python/src/$(APP)/frontend (production)
	cd frontend && bunx vite build

run: ## run the app in production mode (uses built frontend assets)
	$(PY) -m $(APP)

icon: ## generate macOS .icns from the app icon (needed for make package on macOS)
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
	$(PYINSTALLER) tauri_app_wheel.spec --noconfirm

clean: ## remove venv, build artifacts and node_modules
	rm -rf $(VENV) dist build frontend/node_modules icons.iconset
	find . -name "__pycache__" -type d -prune -exec rm -rf {} +
