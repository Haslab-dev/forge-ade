# core/

Shared, UI-agnostic domain packages consumed by every application target
(`app/native`, `app/webview`, `app/cli`, future `app/web`).

## Boundary rules

1. **No GUI imports.** `core/**` must not import Wails (`github.com/wailsapp/wails`)
   or MyGo (`github.com/egoist/mygo`). Anything presentation-specific belongs
   to the app that needs it.
2. **No app imports.** `core/**` must not import `app/**`. Dependencies point
   one way: `app/* → core/* → RakitKode`.
3. **One concern per package.** A package owns a domain (git, terminal,
   agentsession…) and exposes its manager/service plus its own tests.
4. **Events over reach-through.** Cross-package notifications travel on the
   RakitKode `events.Bus`, not by direct manager-to-manager calls.

See `docs/ARCHITECTURE.md` for the full layering diagram and
`docs/ADR/0001-monorepo.md` for why this layout exists.
