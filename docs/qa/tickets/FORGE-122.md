# FORGE-122 — `ToastProvider` render-phase side effect + no timer cleanup

- **Type:** Bug · **Priority:** Low · **Status:** Open · **Component:** ui/toast
- **Labels:** `code-verified`
- **Location:** `frontend/src/lib/toast.tsx:36` — `moduleToast = toast;` assigned during render (StrictMode double-render / multi-provider races decide the winner); `:30-32` — the 3 s `setTimeout` is never cleared on unmount → setState-after-unmount window.
- **Unit test:** documents that the module escape hatch silently no-ops before any provider renders (by design today).
