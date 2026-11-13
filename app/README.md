# app/

Application targets built from the shared `core/` domain. Each subdirectory
is a delivery surface with its own composition root and packaging:

| Target          | Stack                          | Entry point                        | Dev                  |
| --------------- | ------------------------------ | ---------------------------------- | -------------------- |
| `native/`       | Pure Go + MyGo UI              | `cmd/forge-native`                 | `make native-run`    |
| `webview/`      | Wails v3 + React 19 + Vite     | `main.go` (`internal/app` service) | `make webview-dev`   |
| `cli/`          | Headless Go binaries           | `forge-agent`, `forge-mcp`, `shelltest` | `make cli`      |
| `web/`          | *(reserved — empty)*           | —                                  | —                    |

Rules of the road:

- Targets import `core/` (and RakitKode) — never each other.
- Wails-coupled code lives only under `webview/internal/` (enforced by Go
  `internal` visibility).
- Per-target docs: `native/README.md`, `../docs/ARCHITECTURE.md`.
