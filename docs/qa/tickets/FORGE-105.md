# FORGE-105 — Wire `go test` + `vitest` + lint gates into CI

- **Type:** Story · **Status:** To Do · **Component:** ci
- **Description:** Run `go build/vet/gofmt/test`, `bun run lint`, `bun run test`, `bun run build` on every PR. Today all of these are green but only run locally. Acceptance: red PR on regression; oxlint warnings budgeted (see FORGE-124).
