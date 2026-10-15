# FORGE-135 — Server-mode build cannot compile (`task build:server` / `run:server` broken)

- **Type:** Bug · **Priority:** High · **Status:** Open · **Component:** build
- **Labels:** `build-verified`
- **Repro:** `go build -tags server .` (also with `server,production`) →
  `./main.go:144:32: undefined: application.DefaultApplicationMenu`
- **Impact:** The Taskfile-advertised pure-HTTP server mode (and the Docker server image flow that depends on it) is unbuildable at HEAD.
- **Fix sketch:** guard the menu/window setup behind a build tag (e.g. `//go:build !server` variants or a runtime check), as the Wails `server` tag excludes GUI APIs.
