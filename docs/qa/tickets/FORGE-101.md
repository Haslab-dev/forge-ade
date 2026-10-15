# FORGE-101 — Frontend test infrastructure (vitest + jsdom + Testing Library)

- **Type:** Story · **Status:** ✅ Done · **Component:** frontend
- **Delivered:** dev deps `vitest`, `jsdom`, `@testing-library/{react,dom,jest-dom}`; `frontend/vitest.config.ts` (jsdom env, app `define` globals, `@` alias); `src/test/setup.ts` (jest-dom, `matchMedia` stub); scripts `bun run test` / `test:watch`.
