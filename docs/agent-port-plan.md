# Port Plan: Core Agent Engine → ForgeADE GUI Agent

## Status: IMPLEMENTED ✅

Ported and verified (2026-08-02): all backend + frontend changes build, all Go tests
pass (including a live MCP stdio round-trip test against a mock server), the full
Wails app binary builds, and the frontend bundle + generated bindings are current.

## Goal

Port the agent engine concepts into ForgeADE's
Wails/Go backend + React frontend: a proper agent turn loop, native tool calling,
thinking/reasoning capture + display, structured responses, "dialect" tool-calling,
MCP server connection + tool invocation, and skill invocation as first-class tools.

## Reference model

- `packages/agent/src/agent.ts` — `Agent` class: state machine + event emitter.
- `packages/agent/src/agent-loop.ts` — `agentLoop()`/`runLoop()`: the turn loop.
- `packages/agent/src/types.ts` — `AgentEvent` union: `turn_start/turn_end`,
  `message_start/update/end`, `tool_execution_start/update/end`.
- `packages/ai/src/types.ts` — `AssistantMessage` content blocks: `text`, `thinking`,
  `redactedThinking`, `image`, `toolCall`; `ToolResultMessage` pairs by `toolCallId`.
- `packages/ai/src/dialect/` — in-band (text) tool-calling dialects for providers
  without native tools (anthropic/xml/glm/kimi/deepseek/qwen3/gemini/gemma...):
  scanner (parse text → tool calls) + renderer (re-encode prior calls as text).
- `packages/coding-agent/src/mcp/client.ts` + `transports/stdio.ts` — MCP stdio
  JSON-RPC 2.0 over subprocess stdin/stdout, `initialize`/`tools/list`/`tools/call`.
- `packages/coding-agent/src/extensibility/skills.ts` — skill discovery + `/skill:<name>`
  invocation → SKILL.md body injected as a user message.
- TUI: thinking blocks + tool-call timeline rows rendered from `AgentEvent`s.

## Current ForgeADE gaps (from code audit)

1. Agent loop is recursion-based, resends all messages each turn, no max-iteration
   guard, no per-message/event granularity — only `{session_id}` on `agent:updated`.
2. Tool calls aren't linked by `tool_call_id`; `role:"tool"` messages lose the ID;
   assistant `tool_calls` are re-sent unexecuted → provider rejects them.
3. MCP is config-only: no protocol client, no transport, no tool listing, no invocation.
4. Skills are injected as system-prompt text only; no skill invocation mechanism.
5. No "dialect" (in-band tool calling) — native tool calling only.
6. No stop/abort binding; cancelFuncs only used on session delete.
7. Duplicated chat UI (AgentCell vs AgentTabCell); no streamed event deltas.

## Architecture for the port

### A. Backend — `internal/agent/` (primary work)

Replace the recursive `runAgentTurn` with an event-driven loop modeled on
the reference agent loop:

- **Event model** — add to `internal/events/bus.go` and use as the single stream:
  - `agent:started` / `agent:done`
  - `agent:turn_start` / `agent:turn_end`
  - `agent:message_start` / `agent:message_delta` / `agent:message_end`
  - `agent:tool_start` / `agent:tool_delta` / `agent:tool_end`
  - `agent:thinking_start` / `agent:thinking_delta` / `agent:thinking_end`
- **Message model** — extend `AgentMessage` to a block-based shape mirroring
  the reference assistant-message model:
  - `type: "text" | "thinking" | "tool_call" | "tool_result"`, with `tool_call_id`,
    `name`, `arguments`, `is_error`.
  - Keep JSON backward-compat with the existing persisted `agent_sessions.json`.
- **The loop** (`runAgentLoop`):
  1. Build context: system prompt + project context + skills + MCP tool defs.
  2. Stream from `llm.ChatWithStream` with a rich callback that pushes
     `text_delta`/`thinking_delta`/`toolcall_delta` events (currently only
     content/reasoning strings are surfaced).
  3. On `done`: finalize assistant message (blocks), append tool results that
     reference the original `tool_call_id`.
  4. Approval gate: mutating tools pause the whole batch (fixed: currently only the
     first tool is gated and only it runs on approve).
  5. Loop with a **max-iteration guard** (e.g. 32) and a stop/abort binding
     (`StopAgentTurn(id)`) that cancels the context.

### B. MCP protocol — `internal/mcp/client.go` → expand

Port the MCP stdio JSON-RPC 2.0 client:

- `internal/mcp/jsonrpc.go` — request/response/notification framing, ID allocator.
- `internal/mcp/stdio.go` — spawn `command args` subprocess, newline-delimited JSON
  over stdin/stdout, pending-request map, read loop, close/terminate.
- `Manager.ConnectAll()` — on startup, spawn enabled servers, `initialize`, then
  `tools/list` → populate `m.tools`.
- `Manager.CallTool(name, args)` → route `server_name/tool_name` → `tools/call`.
- Expose MCP tools to the agent as dynamic entries in `tools.Registry` (it has a
  `Register` hook) so the LLM sees them alongside built-ins.
- Add `AgentTool` wrapper that marks MCP-sourced tools.

### C. Dialect (in-band tool calling) — `internal/agent/dialect.go` (new)

Port the concept, simplified to the two most common owned dialects:

- **XML dialect** (Anthropic/DSML style): scanner parses `<invoke name="..."><parameter>`
  blocks from the text stream; renderer re-encodes prior tool calls/results as XML.
- **JSON/GLM dialect**: `{ "tool": "name", "args": {...} }` lines.
- Selection: per-model/provider preference (e.g. DeepSeek → XML) or explicit config.
- When a dialect is active: send **no native `tools`**, append the dialect's tool
  catalog instructions to the system prompt, parse streamed text into tool calls,
  and feed results back through the same loop.

### D. Skills — `internal/skills/skills.go` → add invocation

- Add `/skill:<name>` parsing (leading + mid-prompt forms)
  `parseSkillInvocation`).
- On invocation, inject the SKILL.md body + base dir as a **user message** (not
  system-prompt text) so the model resolves the skill's scripts/templates paths.
- Keep the existing auto-load of all skills into the system prompt as a fallback.

### E. Frontend — `frontend/src/panels/shell-screen.tsx` + `editor.tsx`

- Subscribe to the new granular events and apply **deltas** to the live session in
  the Zustand store (or a per-session reducer), replacing the 3s poll + full-refetch
  for streaming.
- Render:
  - **Thinking**: streaming collapsible block (default open per existing taste),
    with `thinking_start/delta/end` deltas.
  - **Tool calls**: a real timeline row per `tool_call` (icon + name + status
    running/completed/failed), args pretty-printed, result expandable — driven by
    `tool_start/tool_end`.
  - **Responses**: markdown body streamed from `message_delta` (no re-render flash).
  - **Approval**: keep the custom modal, but show the whole pending batch.
- Consolidate `AgentCell` + `AgentTabCell` into one shared chat component.

## Milestones

1. Backend event model + block-based messages (persist-compatible).
2. Rewritten turn loop: streaming events, tool_call_id correlation, batch approval,
   max-iteration guard, stop/abort binding.
3. MCP stdio protocol + tool discovery + invocation.
4. Dialect (in-band tool calling) for providers without native tools.
5. Skill invocation.
6. Frontend: streamed events + thinking/tool timeline/markdown rendering.
7. `wails generate` refresh of bindings; build + e2e test against a live session.

## Out of scope (deferred)

- Telemetry/OTel, compaction/summarization, steering/follow-up queues, checkpointing.
- Non-stdio MCP transports (HTTP/SSE) beyond what's trivial — stdio first.

## Agent engine feature checklist

The agent engine feature checklist:
The README's core-agent claims relevant to this port, and their status in ForgeADE:

| Feature | Ported? | Where |
|---|---|---|
| "Agent runtime with tool calling and state management" (pi-agent-core) | ✅ | `internal/agent/agent.go` — event-driven turn loop, state machine |
| Streaming events: message/turn/tool lifecycle | ✅ | `internal/events/bus.go` + `agent.go` — `agent:turn_start/end`, `agent:message_*`, `agent:thinking_*`, `agent:tool_*` emitted via Wails |
| Thinking/reasoning captured and rendered | ✅ | `ContentBlock{type:"thinking"}` + streamed `agent:thinking_delta` + collapsible "Thinking" block (default open) in both chat surfaces |
| Tool calls rendered as cards / timeline rows | ✅ | `ToolCallRow` in `shell-screen.tsx` + `editor.tsx` — running/done states, args, result |
| Tool results paired by ID (tool_call_id) | ✅ | `messagesToLLM` + `messageToolResults` correlate `tool_result` ↔ `tool_call_id` |
| Permission prompts gating destructive tools | ✅ | Batch approval gate (`PendingTools`), Approve / Deny / Always-allow |
| MCP servers connected and tools callable | ✅ | `internal/mcp/stdio.go` + `connection.go` — JSON-RPC 2.0 stdio, `initialize`/`tools/list`/`tools/call`; verified by live subprocess test |
| Skills discovered and invocable | ✅ | `/skill:<name>` parsing (leading + mid-prompt) → SKILL.md body + base dir injected as user message |
| In-band tool-calling dialect for providers without native tools | ✅ | `internal/agent/dialect.go` — XML `<invoke>` scanner + transcript renderer, per-session toggle |
| Prompt cards + document-style turns (Zed-like) | ✅ (pre-existing, now block-aware) | `buildTurns` renders prompt card → tool timeline → markdown response |
| Abort/cancel a running agent turn | ✅ | `StopAgentTurn` binding + stop button in both chat headers |

### Not ported (out of scope / infra-heavy)

- **40+ providers / model catalog** — ForgeADE already has its own provider system.
- **LSP ops, DAP debugger, browser, web_search, git tools, subagents, advisor,
  collab, hindsight, hashline edits, AST edit/grep, TTS/image gen, commit analysis** —
  these are separate subsystems beyond the core agent engine requested.
- **Compaction/summarization, steering/follow-up queues, telemetry** — deferred.
- **HTTP/SSE MCP transports** — stdio implemented; remote/URL servers error cleanly.

