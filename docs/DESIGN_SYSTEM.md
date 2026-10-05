# Design System

ForgeADE's design system is CSS-custom-property tokens in `frontend/src/index.css`, bridged into Tailwind CSS v4 utilities via `@theme` blocks. Dark and light mode are both first-class: `.dark` / `.light` classes on `<html>`/`<body>` swap the token values, and every component reads tokens only.

## Token layers

1. **Raw palettes** — `:root`/`.dark` and `.light` blocks define semantic CSS variables: surfaces (`--bg-app`, `--bg-sidebar`, `--bg-panel`, `--bg-elevated`, `--bg-surface`, `--bg-surface-hover`, `--bg-surface-active`, `--bg-overlay`), foregrounds (`--fg-primary/secondary/tertiary/disabled/inverse/link`), borders (`--border-default/subtle/focus`), accents (`--accent-primary/hover`), statuses, and graph colors (`--graph-c0…c9`).
2. **Tailwind bridge** — `@theme inline` maps them to utilities: `bg-app`, `bg-panel`, `bg-elevated`, `bg-surface`, `hover:bg-surface-hover`, `bg-selected`, `text-fg-primary/secondary/tertiary/disabled`, `border-border`, `text-accent-primary`, `bg-accent-primary/20`, `text-link`, `text-status-*`, `text-full-access`, plus the shadcn-compatible aliases (`bg-background`, `bg-popover`, `bg-card`, `text-muted-foreground`, …).

The `text-ui-*` sizes are registered as real Tailwind theme keys (`--text-ui-*` with paired line-heights), so Tailwind generates them natively — there is no manual `@layer utilities` copy and no tailwind-merge customization.

## The `text-ui` type scale

The UI is a dense, IDE-like interface. All UI text uses the fixed scale — never raw px:

| Class           | Size | Use for                                        |
| --------------- | ---- | ---------------------------------------------- |
| `text-ui-xs`    | 11px | Metadata, badges, kbd hints, file tree         |
| `text-ui-sm`    | 12px | Secondary labels, buttons, list rows           |
| `text-ui-base`  | 13px | Default body/UI text (`--ui-font-size`)        |
| `text-ui-lg`    | 15px | Emphasized text, section headers               |
| `text-ui-xl`    | 17px | Titles, empty-state headings                   |

Only exception: display numerals (e.g. `text-[30px]` stat figures) sit outside the scale on purpose.

## Rules

**Do**

- Use semantic utilities (`bg-panel`, `text-fg-secondary`, `border-border`, `text-ui-sm`).
- Use opacity modifiers for tints: `bg-primary/10`, `bg-success/10`.
- Use the "contrast CTA" recipe for the primary action button: `bg-foreground text-background` (near-black in light mode, near-white in dark).
- Escape hatch for a token that has no utility yet: `bg-[var(--some-var)]` is acceptable **temporarily** — prefer adding the `--color-*` mapping to `@theme inline` instead.

**Don't**

- Hardcode hex in components. Known, intentional exceptions: language logo colors (TypeScript blue, Go cyan…), the Z.ai avatar, markdown code-block surfaces, toggle knobs, the white "webpage loading" placeholder in BrowserViewer.
- Use `bg-white dark:bg-[#hex]` pairs. They only look right in one theme; the tokens exist precisely to replace them.
- Invent new font sizes. Map to the scale: ≤11.5px→`xs`, 12→`sm`, 13→`base`, 14–16→`lg`.

## Dark/light theming

`workspaceStore` owns the theme state (hydrated from localStorage so the choice survives restarts) and toggles `.dark`/`.light` on `documentElement` and `body`. Never hardcode a `dark` class on a component subtree — that forks the theme and breaks light mode (this was the bug behind the old split-theme render). Components must not branch on the theme in JS for colors; if a non-CSS surface needs theme colors (e.g. the xterm theme), read them at runtime via `getComputedStyle` + `paletteFromCss()` (see `components/terminal-view.tsx`) and re-apply on theme change.

## Reference

- Token definitions and the `@theme` bridge: `frontend/src/index.css`
- Live examples of correct usage: `components/agent/WorkspaceHeader.tsx`, `components/session/TerminalSessionSurface.tsx`
