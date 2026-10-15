# FORGE-147 — Mini-JEV: offline heuristic fallback router (shipped)

- **Type:** Story · **Priority:** Medium · **Status:** ✅ Done · **Component:** go/jev
- **Description:** previously, when the JEV API key was missing or the endpoint failed, routing degraded to "offer ALL ~50 tool schemas" (token cost + tool hallucinations) or — with no key — the same. Shipped **HeuristicRoute** ("mini JEV"): an offline keyword classifier (English + Indonesian: "jalankan", "cari file", "perbaiki", "buka situs", …) that scores intent per category and filters through the same `filterToolsByCategory`, so universal tools (coordination + computer-use) and explicit tool citations behave identically to the remote path.
- **Hooked into every degraded path:** no API key / remote backoff / request error / non-200 / decode error → mini-JEV instead of all-tools.
- **Tests:** `TestHeuristicRoute` (explicit mention, screen intent → all, execute/git/web keywords EN+ID, chat → universal set only).
