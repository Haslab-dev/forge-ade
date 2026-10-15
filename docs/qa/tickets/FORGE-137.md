# FORGE-137 — `useEditorStore.files` pins full file content in memory

- **Type:** Task · **Priority:** Low · **Status:** Open · **Component:** frontend/stores
- **Location:** `frontend/src/hooks/store.ts:61-66` (carries its own NOTE requesting an eviction path). Currently not the primary open-file holder — keep it that way or add eviction.
