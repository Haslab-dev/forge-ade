# FORGE-146 — Browser viewer viewport dropdown hidden behind the webview (fixed)

- **Type:** Bug · **Priority:** Low · **Status:** ✅ Fixed · **Component:** browser-viewer
- **Labels:** `user-reported`, `gui-verified`
- **Root cause:** the native WKWebView (positioned via `BrowserUseSetRect`) always composites **above** HTML overlays — the zoom/preset dropdown opened downward into the webview area with `z-50`, which is powerless against a native layer.
- **Fix:** the dropdown opens **downward** ("to below not top") with native WKWebView temporary concealment via `BrowserUseSetRect(..., false)` while `zoomMenuOpen` is active, allowing the HTML dropdown overlay to composite cleanly in front of the browser canvas without obstructing toolbar rows.
