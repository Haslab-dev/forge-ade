// Engine integration test: streams from a mock OpenAI-compatible SSE provider,
// asserts event ordering, thinking deltas, tool execution, and JSONL persistence.

import { describe, expect, test } from "bun:test";
import fs from "fs";
import os from "os";
import path from "path";
import { AgentEngine } from "./engine";
import { SessionStore } from "./store";
import type { ProviderTarget } from "./llm-client";

const PORT = 45999;

/** Builds SSE chunks: thinking -> text -> glob tool call -> (second call) text. */
function startMockProvider(): { url: string; requests: number } {
  const state = { requests: 0 };
  const server = Bun.serve({
    port: PORT,
    async fetch(req) {
      state.requests += 1;
      const body = (await req.json()) as { messages: { role: string; content: string }[] };
      const isFollowUp = body.messages.some((m) => m.role === "tool");
      const chunks: string[] = [];
      if (isFollowUp) {
        chunks.push(
          sse({ choices: [{ delta: { reasoning_content: "checking results..." } }] }),
          sse({ choices: [{ delta: { content: "Found 1 files." } }] }),
          sse({ choices: [{ delta: {}, finish_reason: "stop" }] }),
          "data: [DONE]\n\n",
        );
      } else {
        chunks.push(
          sse({ choices: [{ delta: { reasoning_content: "thinking hard " } }] }),
          sse({ choices: [{ delta: { reasoning_content: "about the task" } }] }),
          sse({ choices: [{ delta: { content: "Let me look." } }] }),
          sse({
            choices: [
              {
                delta: {
                  tool_calls: [
                    { index: 0, id: "call_1", function: { name: "glob", arguments: "" } },
                    { index: 0, function: { arguments: "{\"pat" } },
                    { index: 0, function: { arguments: "tern\": \"*.ts\"}" } },
                  ],
                },
              },
            ],
          }),
          sse({ choices: [{ delta: {}, finish_reason: "tool_calls" }] }),
          "data: [DONE]\n\n",
        );
      }
      return new Response(chunks.join(""), {
        headers: { "Content-Type": "text/event-stream" },
      });
    },
  });
  void server;
  return { url: `http://127.0.0.1:${PORT}/v1`, requests: state.requests };
}
function sse(obj: unknown): string {
  return `data: ${JSON.stringify(obj)}\n\n`;
}

function makeTarget(): ProviderTarget {
  return {
    providerId: "openai",
    baseURL: `http://127.0.0.1:${PORT}/v1`,
    apiKey: "test-key",
    model: "mock-1",
  };
}

describe("AgentEngine", () => {
  test("streams thinking/text/tool events and persists JSONL transcript", async () => {
    startMockProvider();
    const dataDir = fs.mkdtempSync(path.join(os.tmpdir(), "agent-engine-test-"));
    const store = new SessionStore(dataDir);
    const events: { name: string; payload: Record<string, unknown> }[] = [];
    const engine = new AgentEngine(store, makeTarget, (name, payload) => {
      events.push({ name, payload });
    }, { dataDir });

    fs.writeFileSync(path.join(dataDir, "sample.ts"), "export const x = 1;\n", "utf-8");
    const session = engine.createSession("Test", "coding", dataDir);
    await engine.sendMessage(session.id, "list typescript files");

    const names = events.map((e) => e.name);
    expect(names[0]).toBe("session:opened");
    expect(names).toContain("agent:turn_start");
    expect(names).toContain("agent:message_start");

    // Thinking deltas streamed before text deltas.
    const firstThinking = events.findIndex((e) => e.name === "agent:message_delta" && e.payload.kind === "thinking");
    const firstText = events.findIndex((e) => e.name === "agent:message_delta" && e.payload.kind === "text");
    expect(firstThinking).toBeGreaterThan(-1);
    expect(firstText).toBeGreaterThan(firstThinking);

    // Tool ran end-to-end (glob over the temp dir finds this test file).
    const toolEnd = events.find((e) => e.name === "agent:tool_end");
    expect(toolEnd).toBeDefined();
    expect(toolEnd!.payload.name).toBe("glob");
    expect(toolEnd!.payload.isError).toBeFalsy();
    expect(String(toolEnd!.payload.result)).toContain(".ts");

    // Follow-up turn produced final answer without tools.
    expect(names.filter((n) => n === "agent:turn_end").length).toBe(1);
    const finalDelta = events.filter((e) => e.name === "agent:message_delta" && e.payload.delta === "Found 1 files.");
    expect(finalDelta.length).toBe(1);

    // Persisted transcript: header line + message entries, reloadable.
    const file = path.join(dataDir, "sessions", `${session.id}.jsonl`);
    expect(fs.existsSync(file)).toBe(true);
    const lines = fs.readFileSync(file, "utf-8").split("\n").filter(Boolean);
    const header = JSON.parse(lines[0]) as { type: string; name: string };
    expect(header.type).toBe("session");
    const messageEntries = lines.map((l) => JSON.parse(l) as { type: string }).filter((e) => e.type === "message");
    // user + assistant(1) + tool result + assistant(2)
    expect(messageEntries.length).toBeGreaterThanOrEqual(4);
    // Streaming marker must never be persisted.
    const anyRunning = messageEntries.some((e) => JSON.stringify(e).includes('"state":"running"'));
    expect(anyRunning).toBe(false);

    // Reload from disk into a fresh store: messages survive restart.
    const reloaded = new SessionStore(dataDir).load(session.id);
    expect(reloaded).not.toBeNull();
    expect(reloaded!.messages.length).toBe(messageEntries.length);

    // Index lists metadata without messages.
    const metas = new SessionStore(dataDir).listMetas();
    expect(metas.length).toBe(1);
    expect((metas[0] as unknown as { messages?: unknown }).messages).toBeUndefined();
    expect(metas[0].messageCount).toBeGreaterThan(0);

    fs.rmSync(dataDir, { recursive: true, force: true });
  });
});
