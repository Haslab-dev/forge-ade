// External agent session manager — the seam between ForgeADE's agent surface
// (Session/AgentMessage model, JSONL store, event stream) and external coding
// agents spoken to over ACP.
//
// External sessions are ordinary rows in the shared SessionStore marked with
// role `external:<agent-id>`, so every existing list/transcript UI works
import fs from "fs";
import path from "path";
import { recordUsage } from "../usage";
import type { AgentMessage, ContentBlock, ContentBlockType, Session, SessionMeta } from "../agent/types";
import { emptySession } from "../agent/types";
import type { SessionStore } from "../agent/store";
import { findExternalAgent, EXTERNAL_AGENTS } from "./registry";
import { AcpConnection } from "./client";

export const EXTERNAL_ROLE_PREFIX = "external:";

export function isExternalRole(role: string | undefined | null): boolean {
  return typeof role === "string" && role.startsWith(EXTERNAL_ROLE_PREFIX);
}

export interface ExternalAgentInfo {
  id: string;
  name: string;
  description: string;
}

interface ExternalAgentStateEntry {
  configOptions: unknown[];
  availableCommands: unknown[];
}

/** Live turn context: blocks stream into this assistant message. */
interface TurnCtx {
  assistant: AgentMessage;
  /** Tool call ids whose tool_end has been emitted (dedupe). */
  endedTools: Set<string>;
  /** Captured tool outputs, persisted as paired tool_result messages. */
  results: Map<string, { text: string; isError: boolean }>;
  /** Wall-clock start of the turn for latency stats. */
  startedAt: number;
}

function newId(): string {
  return `agent-${Date.now()}-${Math.random().toString(36).slice(2, 8)}`;
}

export class ExternalAgentManager {
  /** In-memory authoritative copies of open external sessions. */
  private sessions = new Map<string, Session>();
  private conns = new Map<string, AcpConnection>();
  private acpSessionIds = new Map<string, string>();
  private connecting = new Map<string, Promise<AcpConnection>>();
  private turns = new Map<string, TurnCtx>();

  private loadStateCache(): void {
    if (this.stateCacheLoaded) return;
    this.stateCacheLoaded = true;
    if (!this.dataDir) return;
    try {
      const file = path.join(this.dataDir, "acp", "agent-state-cache.json");
      const parsed = JSON.parse(fs.readFileSync(file, "utf-8"));
      if (parsed && typeof parsed === "object") {
        for (const [k, v] of Object.entries(parsed)) {
          this.stateCache.set(k, v as ExternalAgentStateEntry);
        }
      }
    } catch {}
  }

  private saveStateCache(): void {
    if (!this.dataDir) return;
    try {
      const dir = path.join(this.dataDir, "acp");
      fs.mkdirSync(dir, { recursive: true });
      fs.writeFileSync(
        path.join(dir, "agent-state-cache.json"),
        JSON.stringify(Object.fromEntries(this.stateCache)),
        "utf-8",
      );
    } catch {}
  }

  private rememberState(agentKey: string, state: ExternalAgentStateEntry): void {
    if (state.configOptions.length === 0 && state.availableCommands.length === 0) return;
    this.loadStateCache();
    this.stateCache.set(agentKey, state);
    this.saveStateCache();
  }

  /** Last-known config options/commands per agent id — served instantly
   *  while the adapter process is still spawning. */
  private stateCache = new Map<string, ExternalAgentStateEntry>();
  private stateCacheLoaded = false;

  constructor(
    private store: SessionStore,
    private emit: (event: string, payload: unknown) => void,
    private dataDir?: string,
  ) {}

  /** User-chosen config values (model/mode/thinking), reapplied on reconnect. */
  private selections = new Map<string, Record<string, string | boolean>>();

  listAgents(): ExternalAgentInfo[] {
    return EXTERNAL_AGENTS.map(({ id, name, description }) => ({ id, name, description }));
  }

  async createExternalSession(agentKey: string, name: string, projectFolder: string): Promise<Session> {
    const def = findExternalAgent(agentKey);
    if (!def) throw new Error(`Unknown external agent: ${agentKey}`);
    const s = emptySession({
      id: newId(),
      name: name || def.name,
      role: EXTERNAL_ROLE_PREFIX + def.id,
      projectFolder: projectFolder || process.cwd(),
      createdAt: Date.now(),
    });
    this.store.createFileFor(s);
    this.sessions.set(s.id, s);
    this.emit("session:opened", this.metaOf(s));
    // Connect eagerly (background) so a missing/broken adapter surfaces as a
    // system notice instead of failing silently on the first prompt.
    this.ensureConnection(s.id).catch((err) => {
      this.systemNotice(s.id, `${def.name} could not start: ${err instanceof Error ? err.message : String(err)}`);
    });
    return s;
  }

  getSession(id: string): Session | null {
    const cached = this.sessions.get(id);
    if (cached) return cached;
    const loaded = this.store.load(id);
    if (!loaded || !isExternalRole(loaded.role)) return null;
    this.sessions.set(id, loaded);
    return loaded;
  }

  stopTurn(id: string): void {
    const acpId = this.acpSessionIds.get(id);
    const conn = this.conns.get(id);
    if (conn && acpId) conn.cancel(acpId);
  }

  async sendMessage(id: string, content: string): Promise<void> {
    const s = this.getSession(id);
    if (!s) return;
    if (this.turns.has(id)) {
      this.emit("agent:error", { id, message: "a turn is already running — press stop or wait for it to finish" });
      return;
    }
    // Echo the user message into the transcript first, matching internal turns.
    const userMsg: AgentMessage = {
      id: `msg-${Date.now()}-u`,
      role: "user",
      content: [{ type: "text", text: content }],
      timestamp: new Date().toISOString(),
    };
    s.messages.push(userMsg);
    this.store.appendMessage(id, userMsg);

    const def = findExternalAgent(s.role.slice(EXTERNAL_ROLE_PREFIX.length));

    // Auto-title from the first prompt: "[AgentName] first 20 chars…"
    const isFirstPrompt = s.messages.filter((m) => m.role === "user").length === 1;
    if (isFirstPrompt) {
      const trimmed = content.trim().replace(/\s+/g, " ");
      const title = `[${def?.name ?? "Agent"}] ${trimmed.slice(0, 20)}${trimmed.length > 20 ? "…" : ""}`;
      s.name = title;
      this.store.updateHeader(id, { name: title });
      this.emit("agent:title_changed", { id, name: title });
      this.emit("agent:updated", { id });
    }

    let conn: AcpConnection;
    try {
      conn = await this.ensureConnection(id);
    } catch (err) {
      this.systemNotice(
        id,
        `${def?.name ?? "external agent"} could not start: ${err instanceof Error ? err.message : String(err)}`,
      );
      return;
    }
    const acpId = this.acpSessionIds.get(id);
    if (!acpId) {
      this.systemNotice(id, `${def?.name ?? "external agent"} has no active ACP session`);
      return;
    }

    const assistant: AgentMessage = {
      id: `msg-${Date.now()}-a-${Math.random().toString(36).slice(2, 6)}`,
      role: "assistant",
      content: [],
      timestamp: new Date().toISOString(),
      state: "running",
    };
    s.messages.push(assistant);
    this.turns.set(id, { assistant, endedTools: new Set(), results: new Map(), startedAt: Date.now() });
    s.state = "running";
    this.emit("agent:turn_start", { id });
    this.emit("agent:message_start", { id, messageId: assistant.id });
    this.emit("agent:updated", { id });

    try {
      const result = await conn.prompt(acpId, content);
      this.recordTurnUsage(id, s, def?.id ?? "unknown", result.usage);
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      this.appendBlock(assistant, "text", `\n[external agent error] ${message}`);
      this.emit("agent:error", { id, message });
    } finally {
      this.finishTurn(id, assistant);
    }
  }

  /** Mirrors the internal engine's usage accounting: session totals + the
   *  global Usage journal so external turns show up in observability. */
  private recordTurnUsage(
    id: string,
    s: Session,
    agentId: string,
    usage?: { inputTokens?: number; outputTokens?: number; cachedReadTokens?: number },
  ): void {
    if (!usage) return;
    const turn = this.turns.get(id);
    const at = Date.now();
    const snapshot = {
      at,
      promptTokens: Math.max(0, Number(usage.inputTokens) || 0),
      completionTokens: Math.max(0, Number(usage.outputTokens) || 0),
      cachedTokens: Math.max(0, Number(usage.cachedReadTokens) || 0),
      durationMs: turn ? Math.max(0, at - turn.startedAt) : 0,
    };
    s.lastUsage = snapshot;
    if (turn) turn.assistant.usage = snapshot;
    s.totalUsage.promptTokens += snapshot.promptTokens;
    s.totalUsage.completionTokens += snapshot.completionTokens;
    s.totalUsage.requests += 1;
    this.store.updateHeader(id, { totalUsage: { ...s.totalUsage }, lastUsage: { ...snapshot } });
    if (this.dataDir) {
      const modelOpt = this.getExternalState(id).configOptions.find(
        (c: any) => c.category === "model" || c.id === "model",
      ) as any;
      recordUsage(this.dataDir, {
        ts: at,
        provider: `acp:${agentId}`,
        model: String(modelOpt?.currentValue ?? "default"),
        workspace: s.projectFolder,
        sessionId: id,
        inputTokens: snapshot.promptTokens,
        outputTokens: snapshot.completionTokens,
        cachedTokens: snapshot.cachedTokens,
        latencyMs: snapshot.durationMs,
      });
    }
    this.emit("agent:updated", { id });
  }

  deleteSession(id: string): void {
    const conn = this.conns.get(id);
    if (conn) {
      conn.close();
      this.conns.delete(id);
    }
    this.acpSessionIds.delete(id);
    this.sessions.delete(id);
    this.turns.delete(id);
    this.store.deleteSession(id);
    this.emit("session:closed", { id });
  }

  clearSession(id: string): void {
    const s = this.getSession(id);
    if (!s) return;
    s.messages = [];
    s.summary = undefined;
    s.observations = [];
    s.todos = [];
    this.store.clearSession(id);
    this.emit("agent:updated", { id });
  }
  /** Spawns + initializes the adapter and opens an ACP session (deduped). */
  private ensureConnection(id: string): Promise<AcpConnection> {
    const existing = this.conns.get(id);
    if (existing && this.acpSessionIds.has(id)) return Promise.resolve(existing);
    let pending = this.connecting.get(id);
    if (!pending) {
      pending = this.connectInternal(id).finally(() => this.connecting.delete(id));
      this.connecting.set(id, pending);
    }
    return pending;
  }

  private async connectInternal(id: string): Promise<AcpConnection> {
    const s = this.getSession(id);
    if (!s) throw new Error("external session not found");
    const def = findExternalAgent(s.role.slice(EXTERNAL_ROLE_PREFIX.length));
    if (!def) throw new Error(`unknown external agent role: ${s.role}`);
    const conn = new AcpConnection(def, s.projectFolder || process.cwd(), {
      onUpdate: (_acpId, update) => this.onAcpUpdate(id, update),
      onSessionState: (_acpId, state) => {
        this.rememberState(def.id, state);
        this.emit("agent:external_state", { id, ...state });
      },
      onExit: (code) => {
        console.error(`[acp:${id}] agent process exited (code ${code ?? "signal"})`);
        this.onProcessExit(id);
      },
      onError: (message) => console.error(`[acp:${id}] ${message}`),
    });
    await conn.start();
    const acpId = await conn.newSession(s.projectFolder || process.cwd());
    // Re-apply user selections (model/mode/thinking) from before a restart.
    const sel = this.selections.get(id);
    if (sel) {
      for (const [configId, value] of Object.entries(sel)) {
        await conn.setConfigOption(acpId, configId, value).catch(() => {});
      }
    }
    this.conns.set(id, conn);
    this.acpSessionIds.set(id, acpId);
    const state = conn.getState(acpId);
    this.rememberState(def.id, state);
    this.emit("agent:external_state", { id, ...state });
    return conn;
  }

  private onProcessExit(id: string): void {
    this.conns.delete(id);
    this.acpSessionIds.delete(id);
    const turn = this.turns.get(id);
    if (turn) {
      this.appendBlock(turn.assistant, "text", "\n[external agent process exited]");
      this.finishTurn(id, turn.assistant);
    }
  }

  // -- event mapping ------------------------------------------------------------
  // ACP session/update → ForgeADE content blocks + streaming events, so the
  // existing chat UI renders external turns exactly like internal ones.

  private onAcpUpdate(id: string, update: any): void {
    const turn = this.turns.get(id);
    if (!turn) return; // update outside a turn (e.g. auth prompts) — ignore for now
    const kind: string = update?.sessionUpdate ?? "";
    switch (kind) {
      case "agent_message_chunk": {
        const text = update.content?.text ?? "";
        if (text) {
          this.appendBlock(turn.assistant, "text", text);
          this.emit("agent:message_delta", { id, kind: "text", delta: text });
        }
        break;
      }
      case "agent_thought_chunk": {
        const text = update.content?.text ?? "";
        if (text) {
          this.appendBlock(turn.assistant, "thinking", text);
          this.emit("agent:message_delta", { id, kind: "thinking", delta: text });
        }
        break;
      }
      case "tool_call": {
        const tcId = String(update.toolCallId ?? `tc-${Date.now()}`);
        const existing = turn.assistant.content.find(
          (b) => b.type === "tool_call" && b.tool_call_id === tcId,
        );
        if (!existing) {
          const block: ContentBlock = {
            type: "tool_call",
            tool_call_id: tcId,
            name: String(update.title ?? update.kind ?? "tool"),
            arguments: safeJson(update.rawInput ?? {}),
          };
          const initial = extractToolText(update);
          if (initial) block.text = truncate(initial, 8000);
          turn.assistant.content.push(block);
          this.emit("agent:tool_start", {
            id,
            index: turn.assistant.content.length - 1,
            toolCallId: tcId,
            name: block.name,
            args: update.rawInput ?? {},
          });
        }
        break;
      }
      case "usage_update": {
        // Live context-window telemetry: {size, used} in tokens.
        const sess = this.getSession(id);
        const size = Number(update?.size);
        if (sess && Number.isFinite(size) && size > 0 && sess.contextWindow !== size) {
          sess.contextWindow = size;
          this.store.updateHeader(id, { contextWindow: size });
        }
        break;
      }
      case "tool_call_update": {
        const tcId = update.toolCallId != null ? String(update.toolCallId) : null;
        let idx = tcId
          ? turn.assistant.content.findIndex((b) => b.type === "tool_call" && b.tool_call_id === tcId)
          : -1;
        // Some adapters stream tool_call_update without an initial tool_call.
        if (idx < 0 && tcId) {
          const block: ContentBlock = {
            type: "tool_call",
            tool_call_id: tcId,
            name: String(update.title ?? update.kind ?? "tool"),
            arguments: safeJson(update.rawInput ?? {}),
          };
          turn.assistant.content.push(block);
          idx = turn.assistant.content.length - 1;
          this.emit("agent:tool_start", {
            id,
            index: idx,
            toolCallId: tcId,
            name: block.name,
            args: update.rawInput ?? {},
          });
        }
        if (!tcId) break;
        const block = turn.assistant.content[idx];
        const outText = extractToolText(update);
        if (outText) {
          block.text = truncate(outText, 8000);
        }
        // Close the tool's live spinner once it settles.
        const settled = update.status === "completed" || update.status === "failed";
        if ((settled || outText) && !turn.endedTools.has(tcId)) {
          turn.endedTools.add(tcId);
          const resultText = typeof outText === "string" ? outText : "";
          turn.results.set(tcId, {
            text: resultText,
            isError: update.status === "failed" || Boolean(block.is_error),
          });
          this.emit("agent:tool_end", {
            id,
            index: idx,
            toolCallId: tcId,
            result: resultText,
            isError: update.status === "failed" || Boolean(block.is_error),
          });
          }
        break;
      }
      default:
        // plan / current_mode_update / available_commands_update — ignored.
        break;
    }
    this.emit("agent:updated", { id });
  }

  private finishTurn(id: string, assistant: AgentMessage): void {
    const turn = this.turns.get(id);
    this.turns.delete(id);
    delete assistant.state;
    const s = this.getSession(id);
    if (s) {
      s.state = "idle";
      if (assistant.content.length > 0) {
        this.store.appendMessage(id, assistant);
      } else {
        // Nothing streamed — drop the placeholder from the live copy.
        const idx = s.messages.indexOf(assistant);
        if (idx >= 0) s.messages.splice(idx, 1);
      }
      // Persist tool outputs as paired tool_result messages — the renderer
      // attaches results to tool_call views by id from role:"tool" messages.
      if (turn && turn.results.size > 0) {
        const toolMsg: AgentMessage = {
          id: `${assistant.id}-tools`,
          role: "tool",
          content: [...turn.results.entries()].map(([tcId, r]) => ({
            type: "tool_result" as const,
            tool_call_id: tcId,
            text: r.text,
            is_error: r.isError,
          })),
          timestamp: new Date().toISOString(),
        };
        s.messages.push(toolMsg);
        this.store.appendMessage(id, toolMsg);
      }
    }
    this.emit("agent:message_end", { id, messageId: assistant.id, message: assistant });
    this.emit("agent:turn_end", { id, ok: true });
    this.emit("agent:updated", { id });
  }

  private appendBlock(msg: AgentMessage, type: ContentBlockType, text: string): void {
    const last = msg.content[msg.content.length - 1];
    if (last && last.type === type && typeof last.text === "string") {
      last.text += text;
    } else {
      msg.content.push({ type, text });
    }
  }

  private metaOf(s: Session): SessionMeta {
    return {
      id: s.id,
      name: s.name,
      role: s.role,
      projectFolder: s.projectFolder,
      dialect: "",
      autoApprove: false,
      createdAt: s.createdAt,
      updatedAt: s.updatedAt,
      messageCount: s.messages.length,
      lastMessagePreview: s.lastMessagePreview,
      state: s.state,
      contextWindow: s.contextWindow,
    };
  }

  /** Persists a system notice into the transcript and broadcasts it. */
  private systemNotice(id: string, message: string): void {
    const s = this.getSession(id);
    if (s) {
      const msg: AgentMessage = {
        id: `msg-${Date.now()}-sys`,
        role: "system",
        content: [{ type: "text", text: message }],
        timestamp: new Date().toISOString(),
      };
      s.messages.push(msg);
      this.store.appendMessage(id, msg);
    }
    this.emit("agent:notice", { id, message });
    this.emit("agent:updated", { id });
  }
  getExternalState(id: string): { configOptions: unknown[]; availableCommands: unknown[] } {
    const acpId = this.acpSessionIds.get(id);
    const conn = this.conns.get(id);
    if (acpId && conn) return conn.getState(acpId);
    // Adapter still spawning (npx cold start) — serve the agent's last-known
    // options so the chat renders model/mode/thinking pills immediately.
    const s = this.getSession(id);
    if (s) {
      this.loadStateCache();
      const cached = this.stateCache.get(s.role.slice(EXTERNAL_ROLE_PREFIX.length));
      if (cached) return cached;
    }
    return { configOptions: [], availableCommands: [] };
  }

  async setExternalConfig(id: string, configId: string, value: string | boolean): Promise<{ configOptions: unknown[]; availableCommands: unknown[] }> {
    const conn = await this.ensureConnection(id);
    const acpId = this.acpSessionIds.get(id);
    if (!acpId) throw new Error("external agent has no active ACP session");
    const sel = this.selections.get(id) ?? {};
    sel[configId] = value;
    this.selections.set(id, sel);
    const state = await conn.setConfigOption(acpId, configId, value);
    const s = this.getSession(id);
    if (s) this.rememberState(s.role.slice(EXTERNAL_ROLE_PREFIX.length), state);
    return state;
  }
}

function safeJson(value: unknown): string {
  try {
    return JSON.stringify(value ?? {});
  } catch {
    return "{}";
  }
}
/**
 * Extracts human-readable output from an ACP tool_call/tool_call_update.
 * Per spec, results ride in `content[]` as {type:"content",content:{text}}
 * or diff blocks — not in an `output` field.
 */
function extractToolText(update: any): string {
  const parts: string[] = [];
  const arr = Array.isArray(update?.content) ? update.content : [];
  for (const c of arr) {
    if (c?.type === "content" && typeof c.content?.text === "string") {
      parts.push(c.content.text);
    } else if (c?.type === "diff" && typeof c.newText === "string") {
      parts.push(c.path ? `${c.path}\n${c.newText}` : c.newText);
    } else if (typeof c?.text === "string") {
      parts.push(c.text);
    }
  }
  return parts.join("\n");
}

function truncate(text: string, max: number): string {
  return text.length <= max ? text : `${text.slice(0, max)}…`;
}
