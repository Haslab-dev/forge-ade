.PHONY: dev run-debug build package check test clean frontend-build frontend-dev

# ── Current version ────────────────────────────────────────────────
VERSION := $(shell node -p "require('./frontend/package.json').version")

# ── Development ────────────────────────────────────────────────────
dev:
	cd frontend && bun run build
	native dev

run-debug:
	NODE_OPTIONS="--inspect" native dev

frontend-dev:
	cd frontend && bun run dev

# ── Check & Test ───────────────────────────────────────────────────
check:
	native check

test:
	native test

# ── Build & Package ────────────────────────────────────────────────
frontend-build:
	cd frontend && bun run build

build: frontend-build
	native build

package: frontend-build
	native package --target macos --archive

# ── Clean ──────────────────────────────────────────────────────────
clean:
	rm -rf zig-out .zig-cache .native frontend/dist
