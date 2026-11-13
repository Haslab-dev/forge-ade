# ADR-0001: Merge ForgeADE web + native apps into a single monorepo

- **Status:** Accepted
- **Date:** 2026-11-13
- **Deciders:** Haslab-dev

## Context

ForgeADE existed as two repositories:

- `Haslab-dev/forge-ade` — the Wails v3 desktop app (`module
  github.com/hasdev/forge-ade`) with 20 `internal/` packages, of which only
  `internal/app` (and one bootstrap test) depended on the Wails runtime.
- `Haslab-dev/ForgeADE-Native` (`module
  github.com/hasdev/forge-ade/forge-ade-native`) — a pure-Go MyGo desktop
  app built to reach UI parity with the Wails app, consuming 9 of those
  `internal/` packages through a fragile relative `replace ../forge-ade`
  directive.

The split caused real friction: the native app's module path was namespaced
under a repository it did not live in; `internal/` coupling worked only by
accident of the sibling `replace`; every shared change required two commits
in two repos; and version/build orchestration diverged (two Makefiles, two
remotes, no CI).

## Decision

Adopt a single monorepo hosted in `Haslab-dev/forge-ade` (branch
`monorepo` until validated) with a target-oriented layout:

```
core/            shared UI-agnostic domain packages (former internal/, minus the Wails-coupled ones)
app/native/      MyGo desktop app (merged via git subtree — history preserved)
app/webview/     Wails desktop app (main.go, frontend, packaging, internal/{app,bootstrap})
app/cli/         headless targets (forge-agent, forge-mcp, shelltest)
app/web/         reserved for the future browser target
```

with **one root Go module** (`github.com/hasdev/forge-ade`) and the
RakitKode engine kept as an external dependency (workspace override
`use ../rakitkode` in dev).

Import paths were rewritten mechanically (`internal/*` → `core/*`,
native self-imports → `app/native/*`) and the Wails TS bindings tree was
mirrored to the new paths so the frontend keeps consuming generated
bindings unchanged in shape.

## Alternatives Considered

- **Multi-module + go.work** (separate `core` module): rejected — daily
  overhead of cross-module version bumps and replace directives for no
  current need; can be revisited if external consumers appear.
- **New clean repository**: rejected — loses the forge-ade commit history
  and tags, and the existing marketing-site deployment wiring.
- **Keep two repos**: rejected — the coupling is permanent (both targets
  ship the same domain), so the split only added cost.

## Consequences

**Positive**

- One commit crosses all layers; refactors of `core/` update both apps.
- `internal/` visibility is used correctly: `app/webview/internal` is
  reachable only from the webview target.
- Single CI pipeline, single release train, single Makefile.
- forge-native history is preserved verbatim under `app/native/` via the
  subtree merge (`git log --follow app/native/pkg` works).

**Negative / follow-ups**

- Wails bindings regeneration currently emits JS-flavoured output on
  wails3 beta.28 while the tree uses the older TS generation; the tree is
  mirrored manually until the frontend adopts the new generator.
- The old `ForgeADE-Native` repository is frozen; archive it after the
  monorepo branch is validated and merged.
- `go.work` still points at the sibling RakitKode checkout for local
  development; CI resolves the pinned version from the registry instead.

## References

- `docs/ARCHITECTURE.md` — target architecture and dependency rules
- `docs/ARCHITECTURE-legacy-webview.md` — pre-monorepo deep dive (kept)
