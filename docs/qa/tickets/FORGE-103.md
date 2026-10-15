# FORGE-103 — Backend unit tests for the 6 untested packages

- **Type:** Story · **Status:** ✅ Done · **Component:** go
- **Delivered:** 40 tests (37 pass + 3 skip-marked detectors) across `globalstore`, `ignore`, `gitignore`, `editor`, `commitmsg`, `workspace`. Includes a real-git integration test for `commitmsg.collectDiff` and fake-esbuild/fake-prettier harnesses for `editor`. Discovered FORGE-131 in the process.
