// Streaming LLM client — OpenAI-compatible SSE plus a native Anthropic adapter.
// Port of the reference Go streaming client: incremental content/reasoning/tool-call
// deltas, first-byte sniffing for providers that ignore stream:true, and in-stream
// error payloads surfaced instead of silently producing empty output.

import fs from "fs";

import type {
  LLMMessage,
  LLMResponse,
  ToolCallDelta,
  ToolDefinition,
} from "./types";

export interface StreamCallbacks {
  onChunk?: (deltaContent: string, deltaReasoning: string) => void;
  onToolCallDelta?: (delta: ToolCallDelta) => void;
}

export interface ProviderTarget {
  providerId: string;
  baseURL: string;
  apiKey: string;
  model: string;
  /** From the model catalog (models.json); drives ctx% and max_tokens. */
  contextWindow?: number | undefined;
  maxTokens?: number | undefined;
}

export class LLMAPIError extends Error {
  status: number;
  retryable: boolean;
  constructor(message: string, status: number) {
    super(message);
    this.status = status;
    this.retryable = status === 429 || status >= 500 || status === 408;
  }
}

function isAnthropic(target: ProviderTarget): boolean {
  return target.providerId === "anthropic";
}

function toolDefsOpenAI(tools: ToolDefinition[]): any[] {
  return tools.map((t) => ({ type: "function", function: t.function }));
}

/** Maps our block messages to provider payloads. */
function toOpenAIMessages(messages: LLMMessage[]): any[] {
  return messages.map((m) => {
    if (m.role === "assistant" && m.tool_calls?.length) {
      return { role: "assistant", content: m.content || null, tool_calls: m.tool_calls };
    }
    if (m.role === "tool") {
      return { role: "tool", content: m.content, tool_call_id: m.tool_call_id };
    }
    return { role: m.role, content: m.content };
  });
}

// ---------------------------------------------------------------------------
// OpenAI-compatible streaming
// ---------------------------------------------------------------------------

interface Accumulator {
  content: string;
  reasoning: string;
  toolCalls: Map<number, { id: string; name: string; args: string }>;
  toolOrder: number[];
  promptTokens: number;
  completionTokens: number;
  cachedTokens: number;
  stopReason: string;
}

function newAccumulator(): Accumulator {
  return {
    content: "",
    reasoning: "",
    toolCalls: new Map(),
    toolOrder: [],
    promptTokens: 0,
    completionTokens: 0,
    cachedTokens: 0,
    stopReason: "stop",
  };
}

/** Extracts cached-prompt tokens from provider usage payloads (OpenAI + Anthropic shapes). */
function cachedTokensFromUsage(usage: Record<string, unknown> | undefined | null): number {
  if (!usage || typeof usage !== "object") return 0;
  const details = usage.prompt_tokens_details as { cached_tokens?: unknown } | undefined;
  if (details && typeof details.cached_tokens === "number") return details.cached_tokens;
  if (typeof usage.cache_read_input_tokens === "number") return usage.cache_read_input_tokens;
  return 0;
}

// ---------------------------------------------------------------------------
// Hang protection: overall deadline per LLM call + idle timeout between chunks.
// Without these a wedged provider blocks the session's turn loop forever.
// ---------------------------------------------------------------------------

const OVERALL_TIMEOUT_MS = 300_000;
const IDLE_TIMEOUT_MS = 90_000;

class TimeoutError extends Error {
  constructor(kind: "overall" | "idle") {
    super(kind === "idle" ? `stream stalled: no data for ${IDLE_TIMEOUT_MS / 1000}s` : "stream exceeded overall deadline");
  }
}

interface StreamGuard {
  signal: AbortSignal;
  /** Reads one chunk, resetting the idle timer; rejects on stall/deadline/outer abort. */
  read(reader: ReadableStreamDefaultReader<Uint8Array>): Promise<{ done: boolean; value: Uint8Array | undefined }>;
  /** Clears the deadline timers once the stream is fully consumed. */
  dispose(): void;
}

function makeStreamGuard(outer: AbortSignal): StreamGuard {
  const ctrl = new AbortController();
  const forward = () => ctrl.abort(outer.reason);
  if (outer.aborted) forward();
  else outer.addEventListener("abort", forward, { once: true });
  let idleTimer: ReturnType<typeof setTimeout> | undefined = undefined;
  const armIdle = () => {
    clearTimeout(idleTimer);
    idleTimer = setTimeout(() => ctrl.abort(new TimeoutError("idle")), IDLE_TIMEOUT_MS);
  };
  const overallTimer = setTimeout(() => ctrl.abort(new TimeoutError("overall")), OVERALL_TIMEOUT_MS);
  const dispose = () => {
    clearTimeout(idleTimer);
    clearTimeout(overallTimer);
  };
  return {
    signal: ctrl.signal,
    async read(reader) {
      armIdle();
      try {
        return await reader.read();
      } catch (err) {
        if (ctrl.signal.reason instanceof TimeoutError) throw ctrl.signal.reason;
        throw err;
      } finally {
        clearTimeout(idleTimer);
      }
    },
    dispose,
  };
}

async function streamOpenAI(
  target: ProviderTarget,
  messages: LLMMessage[],
  tools: ToolDefinition[],
  cb: StreamCallbacks,
  signal: AbortSignal,
): Promise<LLMResponse> {
  const body: Record<string, unknown> = {
    model: target.model,
    messages: toOpenAIMessages(messages),
    stream: true,
    // Ask for usage in the stream; providers that don't support it ignore this.
    stream_options: { include_usage: true },
  };
  if (tools.length > 0) {
    body.tools = toolDefsOpenAI(tools);
  }

  const payload = JSON.stringify(body);
  if (process.env.FORGE_DEBUG_LLM) {
    // Diagnostics: capture the exact request so failures can be replayed via curl.
    try {
      const dumpPath = `/tmp/forge-llm-${Date.now()}.json`;
      fs.writeFileSync(dumpPath, payload);
      console.log(`[llm] request dumped to ${dumpPath} (${payload.length} bytes)`);
    } catch {}
  }

  const guard = makeStreamGuard(signal);
  let res: Response;
  try {
    res = await fetch(`${target.baseURL.replace(/\/+$/, "")}/chat/completions`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        Authorization: `Bearer ${target.apiKey}`,
      },
      body: payload,
      signal: guard.signal,
    });
  } catch (err: unknown) {
    throw asLLError(err);
  }
  if (!res.ok) {
    const text = await res.text().catch(() => "");
    throw new LLMAPIError(`provider ${res.status}: ${text.slice(0, 400)}`, res.status);
  }

  // Some providers ignore stream:true and reply with plain JSON; peek the first
  // non-space byte WITHOUT buffering the whole body so real streams stay live.
  const reader = res.body!.getReader();
  const decoder = new TextDecoder();
  try {
    let prefix = "";
    while (true) {
      const { done, value } = await guard.read(reader);
      if (done) break;
      prefix += decoder.decode(value, { stream: true });
      const trimmed = prefix.trimStart();
      if (trimmed.length === 0) continue;
      if (trimmed[0] === "{") {
        // Non-streaming fallback: consume the rest and parse as one JSON body.
        const rest = await readRemaining(guard, reader, decoder, prefix);
        return parseOpenAIJSONBody(rest, cb);
      }
      break;
    }

    const acc = newAccumulator();
    let buffer = prefix;
    while (true) {
      const { done, value } = await guard.read(reader);
      if (done) break;
      buffer += decoder.decode(value, { stream: true });
      buffer = consumeSSE(buffer, acc, cb);
    }
    consumeSSE(buffer + "\n\n", acc, cb); // flush trailing event

    if (acc.content === "" && acc.reasoning === "" && acc.toolCalls.size === 0) {
      throw new LLMAPIError("empty stream — provider returned nothing", 200);
    }
    return finalize(acc);
  } finally {
    guard.dispose();
  }
}


async function readRemaining(
  guard: StreamGuard,
  reader: ReadableStreamDefaultReader<Uint8Array>,
  decoder: TextDecoder,
  initial: string,
): Promise<string> {
  let out = initial;
  while (true) {
    const { done, value } = await guard.read(reader);
    if (done) break;
    out += decoder.decode(value, { stream: true });
  }
  return out;
}

/** Normalizes any thrown value into an LLMAPIError with retry classification. */
function asLLError(err: unknown): LLMAPIError {
  if (err instanceof TimeoutError) return new LLMAPIError(err.message, 408);
  if (err instanceof LLMAPIError) return err;
  const message = err instanceof Error ? err.message : String(err);
  return new LLMAPIError(`network error: ${message}`, 0);
}

function parseOpenAIJSONBody(body: string, _cb: StreamCallbacks): LLMResponse {
  let parsed: any;
  try {
    parsed = JSON.parse(body.trim());
  } catch {
    console.error("[llm] non-stream JSON body (first 600 chars):", body.slice(0, 600));
    throw new LLMAPIError("provider returned unparseable JSON", 200);
  }
  if (parsed?.error?.message) {
    console.error("[llm] in-body error payload:", JSON.stringify(parsed.error).slice(0, 600));
    throw new LLMAPIError(parsed.error.message, 200);
  }
  const choice = parsed?.choices?.[0] ?? {};
  const msg = choice.message ?? {};
  const acc = newAccumulator();
  acc.content = String(msg.content ?? "");
  acc.reasoning = String(msg.reasoning_content ?? msg.reasoning ?? "");
  for (const tc of msg.tool_calls ?? []) {
    const idx = acc.toolCalls.size;
    acc.toolCalls.set(idx, {
      id: tc.id || `call_${idx}`,
      name: tc.function?.name || "",
      args: tc.function?.arguments || "{}",
    });
    acc.toolOrder.push(idx);
  }
  acc.promptTokens = parsed?.usage?.prompt_tokens ?? estimateTokens(body) >> 1;
  acc.completionTokens = parsed?.usage?.completion_tokens ?? 0;
  acc.cachedTokens = cachedTokensFromUsage(parsed?.usage);
  acc.stopReason = choice.finish_reason === "tool_calls" ? "tool_use" : "stop";
  return finalize(acc);
}

/** Consumes complete SSE events from buffer; returns the remainder. */
function consumeSSE(buffer: string, acc: Accumulator, cb: StreamCallbacks): string {
  let idx: number;
  while ((idx = buffer.indexOf("\n")) >= 0) {
    let lineEnd = idx;
    if (buffer[lineEnd - 1] === "\r") lineEnd -= 1;
    const line = buffer.slice(0, lineEnd);
    buffer = buffer.slice(idx + 1);
    const trimmed = line.trim();
    if (!trimmed.startsWith("data:")) continue;
    const data = trimmed.slice(5).trim();
    if (!data || data === "[DONE]") continue;
    let chunk: any;
    try {
      chunk = JSON.parse(data);
    } catch {
      continue;
    }
    if (chunk.error?.message) {
      console.error("[llm] in-stream error payload:", JSON.stringify(chunk.error).slice(0, 600));
      throw new LLMAPIError(chunk.error.message, 200);
    }
    const delta = chunk.choices?.[0]?.delta;
    if (!delta) {
      if (chunk.usage) {
        acc.promptTokens = chunk.usage.prompt_tokens ?? acc.promptTokens;
        acc.completionTokens = chunk.usage.completion_tokens ?? acc.completionTokens;
        acc.cachedTokens = Math.max(acc.cachedTokens, cachedTokensFromUsage(chunk.usage));
      }
      continue;
    }
    const contentDelta = String(delta.content ?? "");
    let reasoningDelta = String(delta.reasoning_content ?? "");
    if (!reasoningDelta) reasoningDelta = String(delta.reasoning ?? "");
    if (contentDelta || reasoningDelta) {
      acc.content += contentDelta;
      acc.reasoning += reasoningDelta;
      cb.onChunk?.(contentDelta, reasoningDelta);
    }
    for (const tc of delta.tool_calls ?? []) {
      const i = Number(tc.index ?? 0);
      let entry = acc.toolCalls.get(i);
      if (!entry) {
        entry = { id: "", name: "", args: "" };
        acc.toolCalls.set(i, entry);
        acc.toolOrder.push(i);
      }
      if (tc.id) entry.id = tc.id;
      if (tc.function?.name) entry.name += tc.function.name;
      if (tc.function?.arguments) entry.args += tc.function.arguments;
      cb.onToolCallDelta?.({
        index: i,
        id: tc.id || entry.id,
        name: tc.function?.name || "",
        argsFragment: tc.function?.arguments || "",
      });
    }
    const finish = chunk.choices?.[0]?.finish_reason;
    if (finish && finish !== "stop") acc.stopReason = finish === "tool_calls" ? "tool_use" : finish;
  }
  return buffer;
}

function finalize(acc: Accumulator): LLMResponse {
  const toolCalls = acc.toolOrder
    .map((i) => acc.toolCalls.get(i)!)
    .filter((t) => t.name)
    .map((t, i) => ({
      id: t.id || `call_${i}`,
      type: "function" as const,
      function: {
        name: t.name,
        args: safeParseArgs(t.args),
      },
    }));
  return {
    content: acc.content,
    reasoning: acc.reasoning,
    toolCalls,
    promptTokens: acc.promptTokens,
    completionTokens: acc.completionTokens,
    cachedTokens: acc.cachedTokens,
    stopReason: toolCalls.length > 0 ? "tool_use" : acc.stopReason,
  };
}

function safeParseArgs(raw: string): Record<string, any> {
  if (!raw) return {};
  try {
    const v = JSON.parse(raw);
    return typeof v === "object" && v !== null ? v : {};
  } catch {
    return {};
  }
}

// ---------------------------------------------------------------------------
// Anthropic native streaming (/v1/messages)
// ---------------------------------------------------------------------------

function toAnthropicMessages(messages: LLMMessage[]): { system: string; messages: any[] } {
  let system = "";
  const out: any[] = [];
  for (const m of messages) {
    if (m.role === "system") {
      system += (system ? "\n\n" : "") + m.content;
      continue;
    }
    if (m.role === "assistant") {
      const blocks: any[] = [];
      if (m.content) blocks.push({ type: "text", text: m.content });
      for (const tc of m.tool_calls ?? []) {
        blocks.push({ type: "tool_use", id: tc.id, name: tc.function.name, input: safeParse(tc.function.arguments) });
      }
      out.push({ role: "assistant", content: blocks.length ? blocks : [{ type: "text", text: "" }] });
      continue;
    }
    if (m.role === "tool") {
      // Attach as a user turn with tool_result block.
      const last = out[out.length - 1];
      const block = { type: "tool_result", tool_use_id: m.tool_call_id, content: m.content };
      if (last?.role === "user" && Array.isArray(last.content) && last.content[0]?.type === "tool_result") {
        last.content.push(block);
      } else {
        out.push({ role: "user", content: [block] });
      }
      continue;
    }
    out.push({ role: m.role, content: m.content });
  }
  return { system, messages: out };
}

function safeParse(raw: any): Record<string, any> {
  if (typeof raw !== "string") return raw && typeof raw === "object" ? raw : {};
  return safeParseArgs(raw);
}

async function streamAnthropic(
  target: ProviderTarget,
  messages: LLMMessage[],
  tools: ToolDefinition[],
  cb: StreamCallbacks,
  signal: AbortSignal,
): Promise<LLMResponse> {
  const { system, messages: anthroMsgs } = toAnthropicMessages(messages);
  const body: Record<string, any> = {
    model: target.model,
    max_tokens: target.maxTokens ?? 8192,
    stream: true,
    messages: anthroMsgs,
  };
  if (system) body.system = system;
  if (tools.length > 0) {
    body.tools = tools.map((t) => ({
      name: t.function.name,
      description: t.function.description,
      input_schema: t.function.parameters,
    }));
  }

  const guard = makeStreamGuard(signal);
  let res: Response;
  try {
    res = await fetch(`${target.baseURL.replace(/\/+$/, "")}/messages`, {
      method: "POST",
      headers: {
        "Content-Type": "application/json",
        "x-api-key": target.apiKey,
        "anthropic-version": "2023-06-01",
      },
      body: JSON.stringify(body),
      signal: guard.signal,
    });
  } catch (err: unknown) {
    throw asLLError(err);
  }
  if (!res.ok) {
    const text = await res.text().catch(() => "");
    throw new LLMAPIError(`provider ${res.status}: ${text.slice(0, 400)}`, res.status);
  }

  const acc = newAccumulator();
  acc.stopReason = "stop";
  const reader = res.body!.getReader();
  const decoder = new TextDecoder();
  let buffer = "";
  const toolInputs = new Map<number, string>();

  const handleEvent = (data: string) => {
    let ev: any;
    try {
      ev = JSON.parse(data);
    } catch {
      return;
    }
    switch (ev.type) {
      case "content_block_delta": {
        const d = ev.delta ?? {};
        if (d.type === "text_delta" && d.text) {
          acc.content += d.text;
          cb.onChunk?.(d.text, "");
        } else if (d.type === "thinking_delta" && d.thinking) {
          acc.reasoning += d.thinking;
          cb.onChunk?.("", d.thinking);
        } else if (d.type === "input_json_delta") {
          const prev = toolInputs.get(ev.index) ?? "";
          toolInputs.set(ev.index, prev + (d.partial_json ?? ""));
        }
        break;
      }
      case "content_block_start": {
        if (ev.content_block?.type === "tool_use") {
          const i = ev.index;
          acc.toolCalls.set(i, {
            id: ev.content_block.id,
            name: ev.content_block.name,
            args: "",
          });
          acc.toolOrder.push(i);
          cb.onToolCallDelta?.({ index: i, id: ev.content_block.id, name: ev.content_block.name, argsFragment: "" });
        }
        break;
      }
      case "content_block_stop": {
        const i = ev.index;
        const tc = acc.toolCalls.get(i);
        if (tc) {
          tc.args = toolInputs.get(i) ?? "";
          cb.onToolCallDelta?.({ index: i, id: tc.id, name: "", argsFragment: tc.args });
        }
        break;
      }
      case "message_delta": {
        if (ev.delta?.stop_reason === "tool_use") acc.stopReason = "tool_use";
        if (ev.usage) acc.completionTokens = ev.usage.output_tokens ?? acc.completionTokens;
        if (ev.usage) acc.cachedTokens = Math.max(acc.cachedTokens, cachedTokensFromUsage(ev.usage));
        break;
      }
      case "message_start": {
        acc.promptTokens = ev.message?.usage?.input_tokens ?? 0;
        acc.cachedTokens = Math.max(acc.cachedTokens, cachedTokensFromUsage(ev.message?.usage));
        break;
      }
      case "error": {
        throw new LLMAPIError(ev.error?.message || "stream error", 200);
      }
    }
  };

  try {
    while (true) {
      const { done, value } = await guard.read(reader);
      if (done) break;
      buffer += decoder.decode(value, { stream: true });
      let nl: number;
      while ((nl = buffer.indexOf("\n")) >= 0) {
        const line = buffer.slice(0, nl).trim();
        buffer = buffer.slice(nl + 1);
        if (line.startsWith("data:")) {
          const data = line.slice(5).trim();
          if (data && data !== "[DONE]") handleEvent(data);
        }
      }
    }
  } finally {
    guard.dispose();
  }

  return finalize(acc);
}

// ---------------------------------------------------------------------------
// Entry point
// ---------------------------------------------------------------------------

export async function streamChat(
  target: ProviderTarget,
  messages: LLMMessage[],
  tools: ToolDefinition[],
  cb: StreamCallbacks,
  signal: AbortSignal,
): Promise<LLMResponse> {
  if (isAnthropic(target)) {
    return streamAnthropic(target, messages, tools, cb, signal);
  }
  return streamOpenAI(target, messages, tools, cb, signal);
}

const CHARS_PER_TOKEN = 4;

export function estimateTokens(text: string): number {
  return Math.ceil((text || "").length / CHARS_PER_TOKEN);
}
