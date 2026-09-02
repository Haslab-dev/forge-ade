.PHONY: dev run-debug build package check test clean reset frontend-build frontend-dev

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

# No `services-build` step: the daemon is now embedded in the native shell
# (src/services.zig), so the app ships as a single self-contained binary.
build: frontend-build
	native build

package: frontend-build
	native package --target macos --archive

clean:
	rm -rf zig-out .zig-cache .native frontend/dist

# ── Reset: kill ALL running ForgeADE processes (incl. stale vite holding
#    5173, which serves the OLD frontend) + remove build output ──
reset:
	@echo "Killing ForgeADE daemons, native app, vite, and external agents..."
	-pkill -f 'src/server/daemon.ts' 2>/dev/null || true
	-pkill -f 'omp acp' 2>/dev/null || true
	-pkill -f 'forge-ade-native' 2>/dev/null || true
	-pkill -f 'vite --host 127.0.0.1' 2>/dev/null || true
	-pkill -f 'bun run --cwd frontend dev' 2>/dev/null || true
	-pkill -f 'node.*/frontend/node_modules/.bin/vite' 2>/dev/null || true
	-rm -rf frontend/node_modules/.vite 2>/dev/null || true
	@sleep 1
	@echo "Removing old build artifacts..."
	rm -rf zig-out .zig-cache .native frontend/dist
	@echo "Reset complete. Run 'make build' to rebuild."
