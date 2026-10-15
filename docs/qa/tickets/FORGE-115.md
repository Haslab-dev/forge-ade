# FORGE-115 — Markdown highlighter group mismatch (BUG-002)

- **Type:** Bug · **Priority:** Low · **Status:** Open · **Component:** diff-view
- **Labels:** `unit-tested`, `skip-marked`
- **Location:** `frontend/src/lib/diff-highlight.tsx:52,165-168` — `renderMd` feeds `` (`[^`]*`)|(\*\*[^*]+\*\*) `` into `scan()`, which destructures `(comment, str, num, kw)`.
- **Impact:** Inline code renders with the **comment** palette (muted italic); `**bold**` renders with the **string** palette.
- **Test:** active test pins current behavior; `it.skip` asserts the desired string styling — activates on fix.
