# Own Computer-Use Tools for ForgeADE (native macOS, no ZCode dependency)

## Why
The ZCode computer-use MCP server can't run inside ForgeADE (it requires the ZCode permission-broker socket — FORGE evidence in the MCP test). ForgeADE already ships its own native browser tools (`internal/browseruse` → `browser_*`), so we build the equivalent for desktop control: `computer_*` tools backed by a new `internal/computeruse` package.

## Tools (v1, main display, macOS only via `//go:build darwin`)
| Tool | Purpose |
|---|---|
| `computer_screenshot` | Capture screen (optionally region `x,y,w,h`); saves PNG to the project sandbox `tmp/` (auto-pruned after 7 days); returns path + dimensions — **and the image is injected into the model's context** (see Enabler) |
| `computer_click` | Left/right/middle click (with optional double) at `x,y` |
| `computer_type` | Type text into the focused control |
| `computer_key` | Key or chord (`Return`, `cmd+t`, …) |
| `computer_scroll` | Scroll at `x,y`, direction + amount |
| `computer_window_list` | List visible windows (title, app, bounds, pid) for targeting |

## Implementation

### 1. `internal/computeruse` (new, ~450 lines)
- **Manager** with `AXIsProcessTrustedWithOptions` check (prompts the TCC dialog on first use); every action returns a clear error when Accessibility is not granted.
- **Input**: cgo CoreGraphics shim (`-framework CoreGraphics`, matching the browseruse cgo style) — `CGEventCreateMouseEvent/CGEventPost` for click/scroll, `CGEventCreateKeyboardEvent` + key-code map for keys/combos.
- **Screenshot**: `/usr/bin/screencapture -x [-R x,y,w,h] <path>` (files_api `osascript`/`open` precedent) into `sandbox.Sub(projectPath, DirTmp)`; dimensions decoded via `image/png`.
- **Windows**: `CGWindowListCopyWindowInfo` via the same cgo shim.
- Tests: coordinate clamping, key parsing, arg validation (pure parts); permission-gated paths `t.Skip` when AX untrusted.

### 2. Enabler — screenshots visible to the model (`internal/agent/agent.go`)
`messagesToLLM`'s tool-result case currently sends text only. Extend it: when a tool result map contains `screenshot_path` pointing to an existing PNG ≤4MB (same cap as user attachments), build multimodal content parts (`{"type":"text",…}` + `{"type":"image_url","image_url":{"url":"data:image/png;base64,…"}}`) exactly like the existing user-image path (agent.go:4182-4226). Without this the model can't see the screen and computer use is pointless.

### 3. Registration + gating
- `internal/tools/computeruse_tools.go` → `RegisterComputerUseTools(r, mgr)` copying the `RegisterBrowserTools` shape; wired in `official_plugins.go` next to `wireBrowserUse`.
- Extend `isMutatingTool` (agent.go:4523) with the click/type/key/scroll/window tools so **every action pauses for approval unless Full access** (screenshot stays read-only/approval-free). This keeps the existing Full-access switch as the safety model.

### 4. Settings UI
Small "Computer Use (native)" card in Settings (under Browser Use): Accessibility permission status (live check), "Open System Settings" button when not granted, and the tool list. Mirrors the Agent-sandbox card pattern.

### 5. Verify + docs
- `go build/vet/gofmt/test ./...`; frontend lint/test/build (Settings card).
- Live GUI round-trip in the running app if the provider cooperates: prompt the agent to screenshot + click a known target; verify the sandbox tmp PNG + approval pause in non-Full-access mode.
- Docs: ARCHITECTURE.md package map row, FORGE-145 story ticket, note the ZCode MCP server stays as-is (it works only under ZCode).

**Out of scope (v1):** multi-display selection, drag, OCR/text extraction (the model reads screenshots visually), Windows/Linux backends.

Key decisions I made (flag if you disagree): screenshots are **approval-free** (read-only) while all input actions are **gated**; screenshots go to the **sandbox tmp/** so they're auto-pruned; v1 targets the **main display only**.