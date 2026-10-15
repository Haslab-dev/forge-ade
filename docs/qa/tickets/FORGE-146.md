# FORGE-146 — Browser viewer viewport dropdown hidden behind the webview (fixed)

- **Type:** Bug · **Priority:** Low · **Status:** ✅ Fixed · **Component:** browser-viewer
- **Labels:** `user-reported`, `gui-verified`
- **Root cause:** the native WKWebView (positioned via `BrowserUseSetRect`) always composites **above** HTML overlays — the zoom/preset dropdown opened downward into the webview area with `z-50`, which is powerless against a native layer.
- **Fix:** the dropdown now opens **upward** over the HTML toolbar rows (`bottom-full`), which are plain HTML and always visible.
