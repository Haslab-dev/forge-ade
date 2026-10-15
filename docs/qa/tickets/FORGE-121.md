# FORGE-121 — `time.ts` epoch heuristics false-positive; corrupt timestamps show "just now"

- **Type:** Bug · **Priority:** Low · **Status:** Open · **Component:** ui
- **Labels:** `unit-tested`
- **Location:** `frontend/src/lib/time.ts:15-18,47-55` — any 12–13-digit run in a string is treated as an epoch-ms (a 13-digit run from year 2001 passes the `>1e12` guard); sessions with unparseable timestamps always display "just now" (epoch 0), breaking sort/display expectations.
- **Tests:** pin current behavior, including the boundary (`999999999999` rejected).
