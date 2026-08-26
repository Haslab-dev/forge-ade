// (approval-gated) -> repeat until done; bounded steps, transient-error retries,
// observation memory, transcript windowing, rolling compaction, auto-title.

import path from "path";
import fs from "fs";
import os from "os";
import { randomUUID } from "crypto";
import type {
  SessionMeta,
  AgentMessage,
  ContentBlock,
  LLMMessage,
  PendingAskQuestion,
  TurnUsage,
  Session,
  ToolDefinition,
  ToolCall,
} from "./types";
import { emptySession } from "./types";
import { streamChat, estimateTokens } from "./llm-client";
import { recordUsage } from "../usage";
import type { SessionStore } from "./store";
import type { ProviderTarget } from "./llm-client";
import { buildCoreTools } from "./tools";
import { registerExtendedTools } from "./tools-extended";
import type { LSPManager } from "../lsp";
import type { ToolContext, ToolHandler, ToolResult } from "./tools";
import { truncate } from "./tools";
import type { EditorManager } from "../editor";

const MAX_REASONING_STEPS = 120;
const MAX_TOOL_BUDGET = 300;
const COMPACT_THRESHOLD_CHARS = 240_000;
const COMPACT_KEEP_MIN = 20;
const TOOL_RESULT_PRUNE_LIMIT = 4000;
const TOOL_RESULT_KEEP_PER_SIDE = 1500;
const WINDOW_TAIL_MESSAGES = 60;
export function resolveMentionedFiles(mentionedFiles: string[], projectFolder: string): string {
  if (!mentionedFiles || mentionedFiles.length === 0) return "";
  const parts: string[] = [];

  for (const rel of mentionedFiles) {
    const trimmed = rel.trim();
    if (!trimmed) continue;
    const abs = path.isAbsolute(trimmed) ? trimmed : path.resolve(projectFolder, trimmed);
    const displayPath = path.isAbsolute(trimmed) ? path.relative(projectFolder, trimmed) || trimmed : trimmed;
    const ext = path.extname(displayPath).replace(/^\./, "") || "txt";

    try {
      if (!fs.existsSync(abs)) {
        parts.push(`[attached file: ${displayPath} (file does not exist on disk)]`);
        continue;
      }
      const stat = fs.statSync(abs);
      if (stat.isDirectory()) {
        parts.push(`[attached directory: ${displayPath}]`);
        continue;
      }
      if (stat.size > 500 * 1024) {
        const buf = fs.readFileSync(abs, "utf-8");
        const lines = buf.split("\n").slice(0, 200).join("\n");
        parts.push(`[attached file: ${displayPath} (truncated, showing first 200 lines of ${stat.size} bytes)]\n\`\`\`${ext}\n${lines}\n\`\`\``);
        continue;
      }
      const content = fs.readFileSync(abs, "utf-8");
      parts.push(`[attached file: ${displayPath}]\n\`\`\`${ext}\n${content}\n\`\`\``);
    } catch (err) {
      parts.push(`[attached file: ${displayPath} (could not read: ${(err as Error).message})]`);
    }
  }

  return parts.join("\n\n");
}

const TOOL_COST_POINTS: Record<string, number> = { cheap: 1, medium: 3, high: 10 };

const MUTATING_TOOLS = new Set(["write", "edit"]);

export type EmitFn = (eventName: string, payload: Record<string, unknown>) => void;
export interface EngineConfig {
  dataDir?: string;
  lsp?: LSPManager | undefined;
  editor?: EditorManager | undefined;
}

/** Subset of MCPManager the engine depends on (DI seam). */
export interface McpToolSource {
  listConnectedTools(): Array<{ name: string; description: string; parameters?: Record<string, unknown> | undefined }>;
  callQualifiedTool(name: string, args: Record<string, unknown>): Promise<string>;
  listServers(): Array<{
    name: string;
    source: string;
    enabled?: boolean | undefined;
    connected: boolean;
    error?: string | undefined;
  }>;
}

/** Subset of SkillsManager the engine depends on. */
export interface SkillLoader {
  listSkills(projectFolder?: string): Array<{ name: string; description: string }>;
  loadSkillBody(name: string, projectFolder?: string): { body: string; baseDir: string } | null;
}

interface RunningTurn {
  abort: AbortController;
  paused: boolean;
}

/** Matches names produced by MCPManager.qualifyTool: mcp_<server>_<tool>. */
function isMcpToolName(name: string): boolean {
  if (!name.startsWith("mcp_")) return false;
  const rest = name.slice("mcp_".length);
  const sep = rest.indexOf("_");
  return sep > 0 && sep < rest.length - 1;
}

function newId(prefix: string): string {
  return `${prefix}-${Date.now()}-${randomUUID().slice(0, 8)}`;
}

export class AgentEngine {
  private store: SessionStore;
  private emit: EmitFn;
  private running = new Map<string, RunningTurn>();
  /** Session id -> message count at last compaction size check. */
  private lastCompactCheck = new Map<string, number>();
  private dataDir = path.join(os.homedir(), ".forge-ade");
  private getTarget: () => ProviderTarget | null;
  private tools = new Map<string, ToolHandler>();
  /** Live sessions keyed by id; loaded from disk on demand. */
  private live = new Map<string, Session>();
  private mcp: McpToolSource | null = null;
  private skills: SkillLoader | null = null;
  private lsp: LSPManager | null = null;
  private editor: EditorManager | null = null;
  /** Nested task delegation depth (task tool reentrancy guard). */
  private taskDepth = 0;

  constructor(
    store: SessionStore,
    getTarget: () => ProviderTarget | null,
    emit: EmitFn,
    config?: EngineConfig & { mcp?: McpToolSource | undefined; skills?: SkillLoader | undefined },
  ) {
    this.store = store;
    this.getTarget = getTarget;
    this.emit = emit;
    this.tools = buildCoreTools();
    registerExtendedTools(this.tools, {
      dataDir: config?.dataDir ?? path.join(os.homedir(), ".forge-ade"),
      getLsp: () =>
        this.lsp && this.editor
          ? {
              getDiagnostics: (filePath?: string) => this.lsp!.getDiagnostics(filePath),
              searchIndexSymbols: (query: string, folders: string[]) =>
                this.editor!.searchIndexSymbols(query, folders),
            }
          : null,
    });
    this.registerTaskTool();
    this.mcp = config?.mcp ?? null;
    this.lsp = config?.lsp ?? null;
    this.editor = config?.editor ?? null;
    this.skills = config?.skills ?? null;
    if (config?.dataDir) this.dataDir = config.dataDir;
  }

  // ---------------------------------------------------------------------------
  // Sessions
  // ---------------------------------------------------------------------------

  createSession(name: string, role: string, projectFolder: string): Session {
    const s = emptySession({
      id: newId("agent"),
      name: name || "Agent Session",
      role: role || "coding",
      projectFolder: projectFolder || process.cwd(),
      createdAt: Date.now(),
    });
    this.store.createFileFor(s);
    this.live.set(s.id, s);
    this.emit("session:opened", { ...this.metaOf(s) });
    return s;
  }

  getSession(id: string): Session | null {
    if (this.live.has(id)) return this.live.get(id)!;
    const s = this.store.load(id);
    if (!s) return null;
    const runningTurn = this.running.get(id);
    s.state = runningTurn ? "running" : "idle";
    this.live.set(id, s);
    return s;
  }

  listSessions(): SessionMeta[] {
    return this.store.listMetas();
  }

  listSessionsForFolder(folder: string): SessionMeta[] {
    const norm = folder ? folder : process.cwd();
    return this.listSessions().filter((m) => sameDir(m.projectFolder, norm));
  }

  updateSession(
    id: string,
    patch: {
      name?: string | undefined;
      role?: string | undefined;
      customPrompt?: string | undefined;
      customRules?: string | undefined;
    },
  ): Session | null {
    const s = this.getSession(id);
    if (!s) return null;
    if (patch.name) s.name = patch.name;
    if (patch.customPrompt !== undefined) s.customPrompt = patch.customPrompt;
    if (patch.customRules !== undefined) s.customRules = patch.customRules;
    this.persistHeader(s);
    this.emit("agent:updated", { id });
    return s;
  }

  deleteSession(id: string): void {
    this.running.get(id)?.abort.abort();
    this.live.delete(id);
    this.store.deleteSession(id);
    this.emit("session:closed", { id });
  }

  clearSession(id: string): void {
    const s = this.getSession(id);
    if (!s) return;
    s.messages = [];
    s.summary = undefined;
    s.observations = [];
    s.totalUsage = { promptTokens: 0, completionTokens: 0, requests: 0 };
    s.lastUsage = undefined;
    this.store.clearSession(id);
    this.syncMetaCounters(s);
    this.emit("agent:updated", { id });
  }

  setDialect(id: string, dialect: string): void {
    const s = this.getSession(id);
    if (!s) return;
    s.dialect = dialect === "xml" ? "xml" : "";
    this.persistHeader(s);
  }

  setAutoApprove(id: string, enabled: boolean): void {
    const s = this.getSession(id);
    if (!s) return;
    s.autoApprove = enabled;
    this.persistHeader(s);
    this.emit("agent:updated", { id });
  }

  toggleTask(id: string, taskId: string, active: boolean): void {
    const s = this.getSession(id);
    if (!s) return;
    const t = s.todos.find((x) => x.id === taskId);
    if (!t) return;
    t.active = active;
    this.persistHeader(s);
  }

  // ---------------------------------------------------------------------------
  // Messaging / turn control
  // ---------------------------------------------------------------------------

  async sendMessage(
    sessionId: string,
    content: string,
    mentionedFiles: string[] = [],
    attachments: any[] = [],
  ): Promise<void> {
    const s = this.getSession(sessionId);
    if (!s) return;
    if (this.running.has(sessionId)) {
      this.emit("agent:error", { id: sessionId, message: "a turn is already running — press stop or wait for it to finish" });
      return;
    }
    let text = content;
    const skillHit = this.resolveSkillInvocation(content, s.projectFolder);
    if (skillHit) text = `${skillHit.text}\n\n${skillHit.skillContext}`;
    const resolvedMentionContext = resolveMentionedFiles(mentionedFiles, s.projectFolder || process.cwd());
    if (resolvedMentionContext) {
      text += `\n\n[attached context]\n${resolvedMentionContext}`;
    }
    for (const att of attachments) {
      if (att.type === "file" && att.data) {
        try {
          const decoded = Buffer.from(att.data, "base64").toString("utf-8");
          const ext = path.extname(att.name || "").replace(/^\./, "") || "txt";
          text += `\n\n[attached file: ${att.name}]\n\`\`\`${ext}\n${decoded}\n\`\`\``;
        } catch {
          text += `\n\n[attached file: ${att.name}]`;
        }
      }
    }
    const userBlocks: ContentBlock[] = [{ type: "text", text }];
    for (const att of attachments) {
      if (att.type === "image" && att.data) {
        userBlocks.push({
          type: "image",
          mime_type: att.mimeType,
          data: att.data,
          name: att.name,
        });
      }
    }
    const isFirstUserMessage = !s.messages.some((m) => m.role === "user");
    const userMsg: AgentMessage = {
      id: newId("msg"),
      role: "user",
      content: userBlocks,
      timestamp: new Date().toISOString(),
    };
    s.messages.push(userMsg);
    this.store.appendMessage(sessionId, userMsg);
    this.syncMetaCounters(s);

    const turn = this.startTurn(sessionId);
    try {
      await this.runTurnLoop(s, turn);
    } finally {
      this.finishRunning(sessionId);
    }

    // AI rename: on the opener, then once more at message 4 when there is
    // enough context to infer what the session is actually about. Runs after
    // the turn so a slow title call never blocks or races the response.
    const userCount = s.messages.filter((m) => m.role === "user").length;
    if (isFirstUserMessage || userCount === 4) this.maybeGenerateTitle(s.id);
  }

  stopTurn(sessionId: string): void {
    const turn = this.running.get(sessionId);
    const s = this.getSession(sessionId);
    if (turn) {
      turn.abort.abort();
    }
    if (s) {
      s.state = "idle";
      s.pendingApproval = undefined;
      s.pendingAsk = undefined;
      this.persistHeader(s);
    }
    this.emit("agent:turn_end", { id: sessionId, ok: false, stopped: true });
  }

  respondApproval(sessionId: string, approve: boolean, autoAll: boolean): void {
    const s = this.getSession(sessionId);
    if (!s || s.state !== "awaiting_approval" || !s.pendingApproval) return;
    if (autoAll) {
      s.autoApprove = true;
      this.persistHeader(s);
      this.emit("agent:updated", { id: sessionId });
    }
    if (!approve) {
      // Synthesize denial tool results so the transcript stays pair-intact.
      for (const pending of s.pendingApproval) {
        this.appendToolResultMessage(s, pending.id, pending.name, "denied by user", true);
      }
      s.pendingApproval = undefined;
      s.state = "idle";
      this.persistHeader(s);
      this.emit("agent:turn_end", { id: sessionId, ok: true });
      return;
    }
    const approved = s.pendingApproval;
    s.pendingApproval = undefined;
    s.state = "running";
    const turn = this.startTurn(sessionId);
    void this.executeApprovedAndContinue(s, turn, approved);
  }

  respondAsk(sessionId: string, answers: Record<string, unknown>): void {
    const s = this.getSession(sessionId);
    if (!s || s.state !== "awaiting_input" || !s.pendingAsk) return;
    const questions = s.pendingAsk;
    s.pendingAsk = undefined;
    const answerText = questions
      .map((q) => `${q.label}: ${JSON.stringify(answers[q.id] ?? null)}`)
      .join("\n");
    // Resume the paused turn by injecting answers as a user message.
    const msg: AgentMessage = {
      id: newId("msg"),
      role: "user",
      content: [{ type: "text", text: `[answers to your questions]\n${answerText}` }],
      timestamp: new Date().toISOString(),
    };
    s.messages.push(msg);
    this.store.appendMessage(s.id, msg);
    s.state = "running";
    const turn = this.startTurn(sessionId);
    void this.resumeLoop(s, turn);
  }

  // ---------------------------------------------------------------------------
  // Turn loop
  // ---------------------------------------------------------------------------

  private startTurn(sessionId: string): RunningTurn {
    const turn: RunningTurn = { abort: new AbortController(), paused: false };
    this.running.set(sessionId, turn);
    const s = this.live.get(sessionId);
    if (s) {
      s.state = "running";
      // Publish immediately so session lists show the running state and the
      // UI can offer Stop instead of a send that would bounce off the guard.
      this.persistHeader(s);
      this.emit("agent:updated", { id: sessionId });
    }
    this.emit("agent:turn_start", { id: sessionId });
    return turn;
  }

  private finishRunning(sessionId: string): void {
    this.running.delete(sessionId);
    const s = this.live.get(sessionId);
    if (s && s.state === "running") {
      s.state = "idle";
      this.persistHeader(s);
      this.emit("agent:updated", { id: sessionId });
    }
  }

  private async runTurnLoop(s: Session, turn: RunningTurn): Promise<void> {
    let budget = MAX_TOOL_BUDGET;
    for (let step = 0; step < MAX_REASONING_STEPS; step++) {
      if (turn.abort.signal.aborted) break;

      const target = this.getTarget();
      if (!target || !target.apiKey) {
        this.emitAgentError(s.id, "no active LLM provider configured — add a provider profile in settings");
        break;
      }

      const assistant: AgentMessage = {
        id: newId("msg"),
        role: "assistant",
        content: [],
        timestamp: new Date().toISOString(),
        state: "running",
      };
      s.messages.push(assistant);
      this.emit("agent:message_start", { id: s.id, messageId: assistant.id });

      const result = await this.streamOnce(s, target, assistant, turn);
      delete assistant.state;

      if (result.error) {
        if (isEmptyMessage(assistant)) s.messages.pop();
        else this.persistAssistant(s, assistant);
        this.emitAgentError(s.id, result.error.message);
        break;
      }

      s.totalUsage.requests += 1;
      s.totalUsage.promptTokens += result.usage.promptTokens;
      s.totalUsage.completionTokens += result.usage.completionTokens;
      s.lastUsage = result.usage; // status-line snapshot (in/out/cache, tok/s)
      assistant.usage = result.usage; // per-response footer in the transcript
      if (target.contextWindow) s.contextWindow = target.contextWindow;
      recordUsage(this.dataDir, {
        ts: result.usage.at,
        provider: target.providerId,
        model: target.model,
        workspace: s.projectFolder,
        sessionId: s.id,
        inputTokens: result.usage.promptTokens,
        outputTokens: result.usage.completionTokens,
        cachedTokens: result.usage.cachedTokens,
        latencyMs: result.usage.durationMs,
      });
      this.persistAssistant(s, assistant);
      this.emit("agent:message_end", { id: s.id, messageId: assistant.id, message: assistant });

      const toolCalls = collectToolCalls(assistant);
      if (toolCalls.length === 0) break; // final text answer

      const isKnownTool = (name: string) => this.tools.has(name) || isMcpToolName(name);
      const costOf = (tc: ToolCall) => {
        const handler = this.tools.get(tc.function.name);
        return TOOL_COST_POINTS[handler?.cost ?? "medium"] ?? 3;
      };
      const planned = toolCalls.filter((tc) => isKnownTool(tc.function.name));
      for (const tc of planned) budget -= costOf(tc);

      // Approval gate: pause before executing mutating calls unless YOLO.
      const needsApproval = s.autoApprove
        ? []
        : planned.filter((tc) => this.tools.get(tc.function.name)?.mutating);
      if (needsApproval.length > 0 && budget > -50) {
        s.pendingApproval = needsApproval.map((tc) => ({
          id: tc.id,
          name: tc.function.name,
          args: tc.function.args,
        }));
        s.state = "awaiting_approval";
        this.persistHeader(s);
        this.emit("agent:approval_required", {
          id: s.id,
          pendingTools: s.pendingApproval,
        });
        turn.paused = true;
        return; // resumed via respondApproval
      }
      if (budget <= -50) {
        this.appendToolResultMessage(s, "budget", "system", "tool budget exhausted for this turn", true);
        break;
      }

      const { allOk, paused } = await this.executeToolCalls(s, turn, planned);
      if (paused) return; // resumed via respondAsk
      if (!allOk) break;
      this.maybeCompact(s);
    }
    this.persistHeader(s);
    this.emit("agent:turn_end", {
      id: s.id,
      ok: true,
      usage: s.lastUsage,
      contextWindow: s.contextWindow,
      totalUsage: s.totalUsage,
    });
  }

  private async resumeLoop(s: Session, turn: RunningTurn): Promise<void> {
    try {
      await this.runTurnLoop(s, turn);
    } finally {
      this.finishRunning(s.id);
    }
  }

  private async executeApprovedAndContinue(
    s: Session,
    turn: RunningTurn,
    approved: { id: string; name: string; args: Record<string, unknown> }[],
  ): Promise<void> {
    try {
      const handlers: ToolCall[] = approved.map((p) => ({
        id: p.id,
        type: "function" as const,
        function: { name: p.name, args: p.args },
      }));
      const { allOk, paused } = await this.executeToolCalls(s, turn, handlers);
      if (!allOk || paused) {
        this.persistHeader(s);
        this.emit("agent:turn_end", { id: s.id, ok: false });
        return;
      }
      await this.runTurnLoop(s, turn);
    } finally {
      this.finishRunning(s.id);
    }
  }

  /** Streams one LLM call into `assistant`, emitting deltas. */
  private async streamOnce(
    s: Session,
    target: ProviderTarget,
    assistant: AgentMessage,
    turn: RunningTurn,
  ): Promise<{ error?: Error; usage: TurnUsage }> {
    const usage: TurnUsage = {
      at: 0,
      promptTokens: 0,
      completionTokens: 0,
      cachedTokens: 0,
      durationMs: 0,
    };
    let lastAttemptErr: Error | null = null;
    for (let attempt = 0; attempt < 3; attempt++) {
      const startedAt = Date.now();
      // Rebuild request messages fresh each attempt (retryable provider blips).
      const reqMessages = this.buildRequestMessages(s, assistant);
      const mcpDefs = this.mcp
        ? this.mcp.listConnectedTools().map((t) => ({
            type: "function" as const,
            function: {
              name: t.name,
              description: t.description || `MCP tool ${t.name}`,
              parameters:
                t.parameters && Object.keys(t.parameters).length > 0
                  ? t.parameters
                  : { type: "object", properties: {} },
            },
          }))
        : [];
      const toolDefs: ToolDefinition[] = [
        ...[...this.tools.values()].map((t) => t.definition),
        ASK_TOOL_DEF,
        TODO_TOOL_DEF,
        ...mcpDefs,
      ];
      // Deterministic order: providers are sensitive to tool list churn.
      toolDefs.sort((a, b) => a.function.name.localeCompare(b.function.name));

      const streamedToolStarts = new Set<number>();
      try {
        const resp = await streamChat(
          target,
          reqMessages,
          toolDefs,
          {
            onChunk: (contentDelta, reasoningDelta) => {
              if (reasoningDelta) {
                this.appendBlock(assistant, "thinking", reasoningDelta);
                this.emit("agent:message_delta", { id: s.id, kind: "thinking", delta: reasoningDelta });
                this.emit("agent:thinking_delta", { id: s.id, delta: reasoningDelta });
              }
              if (contentDelta) {
                this.appendBlock(assistant, "text", contentDelta);
                this.emit("agent:message_delta", { id: s.id, kind: "text", delta: contentDelta });
              }
            },
            onToolCallDelta: (delta) => {
              // First fragment for an index announces the call; the rest stream args.
              if (!streamedToolStarts.has(delta.index) && delta.name) {
                streamedToolStarts.add(delta.index);
                this.emit("agent:tool_start", {
                  id: s.id,
                  index: delta.index,
                  toolCallId: delta.id,
                  name: delta.name,
                  args: "",
                });
              } else if (delta.argsFragment) {
                this.emit("agent:tool_delta", { id: s.id, index: delta.index, args: delta.argsFragment });
              }
            },
          },
          turn.abort.signal,
        );
        usage.promptTokens = resp.promptTokens;
        usage.completionTokens = resp.completionTokens;
        usage.cachedTokens = resp.cachedTokens;
        usage.at = Date.now();
        usage.durationMs = usage.at - startedAt;
        // Providers that omit usage in streams get honest estimates instead of
        // zeros: ~4 chars/token over the exact request/response payloads.
        if (usage.promptTokens === 0) {
          usage.promptTokens =
            estimateTokens(reqMessages.map((m) => m.content).join("\n")) +
            estimateTokens(JSON.stringify(toolDefs));
        }
        if (usage.completionTokens === 0) {
          usage.completionTokens = estimateTokens(
            assistant.content.map((b) => b.text ?? (b.arguments ?? "")).join("\n"),
          );
        }
        // Materialize accumulated tool calls into blocks.
        for (const tc of resp.toolCalls) {
          assistant.content.push({
            type: "tool_call",
            tool_call_id: tc.id,
            name: tc.function.name,
            arguments: JSON.stringify(tc.function.args),
          });
          this.emit("agent:tool_start", {
            id: s.id,
            index: assistant.content.filter((b) => b.type === "tool_call").length - 1,
            toolCallId: tc.id,
            name: tc.function.name,
            args: JSON.stringify(tc.function.args),
          });
        }
        return { usage };
      } catch (err) {
        lastAttemptErr = err as Error;
        const status = (err as { status?: number }).status ?? 0;
        const message = (err as Error).message ?? "";
        // Empty-stream failures already retried MAX_EMPTY_STREAM_RETRIES
        // times inside llm-client; re-retrying here multiplies provider
        // requests up to 9x per step.
        const clientExhausted = status === 503 && message.includes("empty stream");
        const retryable = !clientExhausted && (status === 429 || status >= 500 || status === 408 || status === 0);
        if (!retryable || attempt === 2) break;
        const backoffMs = 800 * Math.pow(2, attempt);
        await new Promise((r) => setTimeout(r, backoffMs));
      }
    }
    return { error: lastAttemptErr ?? new Error("unknown streaming failure"), usage };
  }

  private appendBlock(msg: AgentMessage, kind: "text" | "thinking", delta: string): void {
    const last = msg.content[msg.content.length - 1];
    if (last && last.type === kind) {
      last.text = (last.text || "") + delta;
    } else {
      msg.content.push({ type: kind, text: delta });
    }
  }

  // ---------------------------------------------------------------------------
  // Tool execution
  // ---------------------------------------------------------------------------

  private async executeToolCalls(
    s: Session,
    turn: RunningTurn,
    toolCalls: ToolCall[],
  ): Promise<{ allOk: boolean; paused: boolean }> {
    let allOk = true;
    for (let i = 0; i < toolCalls.length; i++) {
      if (turn.abort.signal.aborted) return { allOk, paused: false };
      const tc = toolCalls[i];

      if (tc.function.name === "ask") {
        const questions = parseAskQuestions(tc.function.args);
        if (questions.length === 0) {
          this.appendToolResultMessage(s, tc.id, "ask", "no questions provided", true);
          allOk = false;
          continue;
        }
        s.pendingAsk = questions;
        s.state = "awaiting_input";
        this.appendToolResultMessage(s, tc.id, "ask", "questions shown to the user; the turn pauses until they answer", false);
        this.persistHeader(s);
        this.emit("agent:ask", { id: s.id, questions });
        return { allOk, paused: true };
      }

      if (tc.function.name === "todo") {
        const result = runTodoOp(s, tc.function.args);
        this.appendToolResultMessage(s, tc.id, "todo", result.content, result.isError);
        if (result.isError) allOk = false;
        this.persistHeader(s);
        continue;
      }

      if (isMcpToolName(tc.function.name)) {
        const startedAt = Date.now();
        let content: string;
        let isError = false;
        try {
          if (!this.mcp) throw new Error("MCP support unavailable in this session");
          content = await this.mcp.callQualifiedTool(tc.function.name, tc.function.args ?? {});
          content = truncateForStorage(content);
        } catch (err) {
          content = `mcp call failed: ${(err as Error).message}`;
          isError = true;
        }
        const durationMs = Date.now() - startedAt;
        if (isError) allOk = false;
        this.appendToolResultMessage(s, tc.id, tc.function.name, content, isError);
        this.emit("agent:tool_end", {
          id: s.id,
          index: i,
          toolCallId: tc.id,
          name: tc.function.name,
          result: content.slice(0, 2000),
          isError,
          durationMs,
        });
        continue;
      }

      const handler = this.tools.get(tc.function.name);
      if (!handler) {
        this.appendToolResultMessage(s, tc.id, tc.function.name, `unknown tool: ${tc.function.name}`, true);
        continue;
      }
      const startedAt = Date.now();
      let result: ToolResult;
      try {
        result = await handler.run(tc.function.args ?? {}, this.toolContext(s));
      } catch (err) {
        result = { content: `tool crashed: ${(err as Error).message}`, isError: true };
      }
      if (result.isError) allOk = false;
      const durationMs = Date.now() - startedAt;
      this.appendToolResultMessage(s, tc.id, tc.function.name, result.content, result.isError);
      this.extractObservation(s, tc.function.name, result.content, result.isError, durationMs);
      this.emit("agent:tool_end", {
        id: s.id,
        index: i,
        toolCallId: tc.id,
        name: tc.function.name,
        result: result.content.slice(0, 2000),
        isError: result.isError,
        durationMs,
      });
    }
    return { allOk, paused: false };
  }

  private appendToolResultMessage(
    s: Session,
    toolCallId: string,
    name: string,
    content: string,
    isError: boolean,
  ): void {
    const msg: AgentMessage = {
      id: newId("msg"),
      role: "tool",
      content: [
        {
          type: "tool_result",
          tool_call_id: toolCallId,
          name,
          text: truncateForStorage(content),
          is_error: isError,
        },
      ],
      timestamp: new Date().toISOString(),
    };
    s.messages.push(msg);
    this.store.appendMessage(s.id, msg);
  }

  private toolContext(s: Session): ToolContext {
    return { projectFolder: s.projectFolder, sessionId: s.id, dataDir: this.dataDir };
  }

  /**
   * `task` — delegate a focused subtask to an isolated sub-session (omp
   * parity). The subagent runs the full turn loop with the same provider and
   * tools; its final assistant message becomes the tool result. Depth is
   * capped so nested delegations cannot recurse unboundedly.
   */
  private registerTaskTool(): void {
    this.tools.set("task", {
      definition: {
        type: "function",
        function: {
          name: "task",
          description:
            "Delegate a focused subtask to an isolated subagent session with its own transcript. " +
            "Give it complete instructions; the final report comes back as the tool result.",
          parameters: {
            type: "object",
            properties: {
              description: { type: "string", description: "Short label for the subtask" },
              prompt: { type: "string", description: "Complete, self-contained instructions for the subagent" },
            },
            required: ["description", "prompt"],
          },
        },
      },
      cost: "high",
      mutating: false,
      run: async (args) => {
        if (this.taskDepth >= 2) {
          return { content: "task failed: delegation depth limit reached (2)", isError: true };
        }
        const description = typeof args.description === "string" ? args.description : "subtask";
        const prompt = typeof args.prompt === "string" ? args.prompt : "";
        if (!prompt.trim()) return { content: "task failed: empty prompt", isError: true };

        const parent = this.live.get(this.currentTaskParentId());
        const child = this.createSession(`task: ${description}`.slice(0, 60), "coding", parent?.projectFolder ?? process.cwd());
        this.taskDepth += 1;
        try {
          await this.sendMessage(child.id, prompt);
          const done = this.getSession(child.id);
          const lastAssistant = [...(done?.messages ?? [])].reverse().find((m) => m.role === "assistant");
          const text = lastAssistant?.content.find((b) => b.type === "text")?.text ?? "";
          return {
            content: truncate(text.trim() || "(subagent finished without a text response)"),
            isError: false,
          };
        } catch (err) {
          return { content: `task failed: ${(err as Error).message}`, isError: true };
        } finally {
          this.taskDepth -= 1;
        }
      },
    });
  }

  private currentTaskParentId(): string {
    // The parent of a running delegation is whichever session is mid-turn.
    for (const id of this.running.keys()) return id;
    return "";
  }


  // ---------------------------------------------------------------------------
  // Context building (memory management)
  // ---------------------------------------------------------------------------

  private buildSystemPrompt(s: Session): string {
    const parts: string[] = [];
    parts.push(rolePrompt(s.role));
    if (s.customPrompt) parts.push(`\n# Operator instructions\n${s.customPrompt}`);
    if (s.customRules) parts.push(`\n# Rules\n${s.customRules}`);
    parts.push(projectContext(s.projectFolder));
    const skills = this.buildSkillsContext();
    if (skills) parts.push(skills);
    if (s.todos.length > 0) {
      const lines = s.todos.map((t) => `- [${t.completed ? "done" : t.active ? "active" : "pending"}] ${t.title}`);
      parts.push(`\n# Current progress\n${lines.join("\n")}`);
    }
    if (s.summary) parts.push(`\n# Earlier conversation digest\n${s.summary}`);
    if (s.observations.length > 0) {
      const obs = s.observations.slice(-30).reverse().map((o) => `- ${o.summary}`);
      parts.push(`\n# Working memory (facts learned this session)\n${obs.join("\n")}`);
    }
    return parts.join("\n");
  }

  /** Full stored transcript -> bounded LLM window with intact tool pairs. */
  private buildRequestMessages(s: Session, liveAssistant: AgentMessage): LLMMessage[] {
    const history = s.messages.slice(0, -1); // exclude the live streaming assistant
    const windowed = windowTranscript(history, WINDOW_TAIL_MESSAGES);
    const out: LLMMessage[] = [{ role: "system", content: this.buildSystemPrompt(s) }];
    for (const m of windowed) {
      if (m.role === "user" || m.role === "system") {
        const imageBlocks = m.content.filter((b) => b.type === "image" && b.data);
        if (imageBlocks.length > 0) {
          const parts: any[] = [];
          const text = blockText(m);
          if (text) parts.push({ type: "text", text });
          for (const img of imageBlocks) {
            parts.push({
              type: "image",
              mime_type: img.mime_type || "image/png",
              data: img.data,
              url: img.url,
            });
          }
          out.push({ role: m.role, content: parts });
        } else {
          out.push({ role: m.role, content: blockText(m) });
        }
        continue;
      }
      if (m.role === "assistant") {
        const text = blockText(m);
        const calls = m.content
          .filter((b): b is ContentBlock & { type: "tool_call" } => b.type === "tool_call")
          .map((b) => ({
            id: b.tool_call_id ?? "",
            type: "function" as const,
            function: { name: b.name ?? "", arguments: b.arguments ?? "{}" },
          }));
        if (calls.length > 0) {
          out.push({ role: "assistant", content: text, tool_calls: calls });
        } else if (text) {
          out.push({ role: "assistant", content: text });
        }
        continue;
      }
      // tool result
      for (const b of m.content) {
        if (b.type !== "tool_result") continue;
        out.push({
          role: "tool",
          content: pruneToolResult(b.text ?? ""),
          tool_call_id: b.tool_call_id ?? "",
        });
      }
    }
    void liveAssistant;
    return out;
  }

  // ---------------------------------------------------------------------------
  // Compaction + observations
  // ---------------------------------------------------------------------------

  private maybeCompact(s: Session): void {
    // Cheap growth gate: skip the O(transcript) stringify until the session
    // has grown meaningfully since the last check.
    const last = this.lastCompactCheck.get(s.id) ?? 0;
    if (s.messages.length - last < 8) return;
    this.lastCompactCheck.set(s.id, s.messages.length);

    const totalChars = s.messages.reduce((sum, m) => sum + JSON.stringify(m.content).length, 0);
    if (totalChars < COMPACT_THRESHOLD_CHARS) return;
    const keepFrom = Math.max(0, s.messages.length - COMPACT_KEEP_MIN);
    const toSummarize = s.messages.slice(0, keepFrom);
    if (toSummarize.length === 0) return;
    const keptAnchor = s.messages[keepFrom]?.id;
    if (!keptAnchor) return;
    const plain = toSummarize
      .map((m) => `${m.role}: ${blockText(m).slice(0, 500)}`)
      .join("\n")
      .slice(0, 60_000);
    const target = this.getTarget();
    if (!target || !target.apiKey) return;
    // Fire-and-forget: digest lands in the next turn at the earliest.
    void streamChat(
      target,
      [
        { role: "system", content: "Summarize the following agent conversation excerpt into a dense digest: goals, decisions, file paths touched, outcomes, open items. Max 400 words." },
        { role: "user", content: plain },
      ],
      [],
      {},
      new AbortController().signal,
    )
      .then((resp) => {
        if (!resp.content) return;
        const previous = s.summary ? `${s.summary}\n\n` : "";
        s.summary = previous + resp.content.trim();
        s.messages.splice(0, keepFrom);
        this.store.appendCompaction(s.id, s.summary, keptAnchor);
        this.persistHeader(s);
        console.log(`[agent] compacted session ${s.id}: -${keepFrom} messages`);
      })
      .catch(() => {});
  }

  /** Skills section of the system prompt, from the injected discovery source. */
  private buildSkillsContext(): string {
    if (!this.skills) return "";
    const skills = this.skills.listSkills();
    if (skills.length === 0) return "";
    const lines = skills.map((sk) => `- /skill:${sk.name}${sk.description ? `: ${sk.description}` : ""}`);
    return `\n# Available skills\nThe user or you may invoke these with /skill:<name>:\n${lines.join("\n")}`;
  }

  /**
   * Detects a `/skill:<name>` invocation in the user draft (leading or
   * mid-prompt) and returns the skill body to inject as context.
   */
  private resolveSkillInvocation(text: string, folder: string): { text: string; skillContext: string } | null {
    if (!this.skills) return null;
    const match = /(^|\s)\/skill:([^\s/]+)/.exec(text);
    if (!match) return null;
    const loaded = this.skills.loadSkillBody(match[2], folder);
    if (!loaded) return null;
    const remaining = text.replace(match[0], " ").trim();
    return {
      text: remaining,
      skillContext: `[skill ${match[2]}]\n${loaded.body.slice(0, 16_000)}\n[base dir: ${loaded.baseDir}]`,
    };
  }

  private extractObservation(
    s: Session,
    toolName: string,
    content: string,
    isError: boolean,
    durationMs: number,
  ): void {
    const head = content.split("\n")[0]?.trim() ?? "";
    let summary: string;
    switch (toolName) {
      case "search":
        summary = `search → ${head.slice(0, 140)}`;
        break;
      case "read":
        summary = `read ${head.slice(0, 100)}`;
        break;
      case "write":
      case "edit":
        summary = `${toolName} applied`;
        break;
      case "bash":
        summary = isError ? `bash failed: ${head.slice(0, 120)}` : `bash ok (${durationMs}ms): ${head.slice(0, 100)}`;
        break;
      default:
        summary = `${toolName}: ${head.slice(0, 100)}`;
    }
    s.observations.push({ ts: Date.now(), summary });
    if (s.observations.length > 30) s.observations.splice(0, s.observations.length - 30);
    this.persistHeader(s);
  }

  // ---------------------------------------------------------------------------
  // Misc helpers
  // ---------------------------------------------------------------------------

  private persistAssistant(s: Session, msg: AgentMessage): void {
    this.store.appendMessage(s.id, msg);
    this.syncMetaCounters(s);
  }

  private persistHeader(s: Session): void {
    this.store.updateHeader(s.id, {
      name: s.name,
      role: s.role,
      dialect: s.dialect,
      autoApprove: s.autoApprove,
      customPrompt: s.customPrompt,
      customRules: s.customRules,
      modelOverride: s.modelOverride,
      summary: s.summary,
      observations: s.observations,
      todos: s.todos,
      totalUsage: s.totalUsage,
      contextWindow: s.contextWindow,
      lastUsage: s.lastUsage,
      state: s.state,
    });
  }

  private syncMetaCounters(s: Session): void {
    s.messageCount = s.messages.length;
    const last = [...s.messages].reverse().find((m) => blockText(m));
    s.lastMessagePreview = last ? blockText(last!).slice(0, 120) : "";
    this.persistMetaLine(s);
  }

  private persistMetaLine(s: Session): void {
    // Store recomputes preview/count from its own counters; reuse header update.
    this.store.updateHeader(s.id, {
      name: s.name,
      role: s.role,
      state: s.state,
    });
  }

  private metaOf(s: Session) {
    return {
      id: s.id,
      name: s.name,
      role: s.role,
      projectFolder: s.projectFolder,
      dialect: s.dialect,
      autoApprove: s.autoApprove,
      createdAt: s.createdAt,
      updatedAt: s.updatedAt,
      messageCount: s.messageCount,
      lastMessagePreview: s.lastMessagePreview,
      state: s.state,
    };
  }

  private emitAgentError(sessionId: string, message: string): void {
    console.error(`[agent:${sessionId}] ${message}`);
    this.emit("agent:error", { id: sessionId, message });
  }

  private titleInFlight = new Set<string>();

  /**
   * AI session rename. Builds context from up to the first three user
   * messages (not just the opener — short openers like "hi" produce useless
   * titles), asks for a concise topic label, sanitizes hard, and skips the
   * update entirely when nothing changed.
   */
  private maybeGenerateTitle(sessionId: string): void {
    const target = this.getTarget();
    if (!target || !target.apiKey) return;
    if (this.titleInFlight.has(sessionId)) return;
    const s = this.getSession(sessionId);
    if (!s) return;

    const userMessages = s.messages
      .filter((m) => m.role === "user")
      .slice(0, 3)
      .map((m) => blockText(m).replace(/\s+/g, " ").trim().slice(0, 400))
      .filter(Boolean);
    if (userMessages.length === 0) return;

    this.titleInFlight.add(sessionId);
    void streamChat(
      target,
      [
        {
          role: "system",
          content:
            "You name coding-agent conversations. Infer a title that captures what the user is actually " +
            "working on. Rules: 2-6 words, plain text only — no quotes, no backticks, no surrounding " +
            "punctuation, no 'Conversation about' style prefixes. Reply with the title and nothing else.",
        },
        { role: "user", content: userMessages.join("\n---\n") },
      ],
      [],
      {},
      new AbortController().signal,
    )
      .then((resp) => {
        const raw = resp.content.trim().split("\n")[0] ?? "";
        const cleaned = raw
          .replace(/[`"'*]/g, "")
          .replace(/^(title|conversation)[:\s-]*/i, "")
          .replace(/\s+/g, " ")
          .trim()
          .slice(0, 60)
          .trimEnd();
        if (!cleaned || cleaned.toLowerCase() === s.name.toLowerCase()) return;
        // Reject degenerate outputs: single generic word or a bare echo of
        // the first few user words adds no information.
        if (cleaned.split(" ").length < 2 && cleaned.length < 4) return;
        s.name = cleaned;
        this.store.updateHeader(sessionId, { name: cleaned });
        this.emit("agent:updated", { id: sessionId });
        this.emit("agent:title_changed", { id: sessionId, name: cleaned });
      })
      .catch(() => {
        // Title generation is cosmetic; never surface provider errors.
      })
      .finally(() => {
        this.titleInFlight.delete(sessionId);
      });
  }
}

// ---------------------------------------------------------------------------
// Module-level helpers
// ---------------------------------------------------------------------------

// Session-coupled tool definitions and runners
// ---------------------------------------------------------------------------

const TODO_TOOL_DEF: ToolDefinition = {
  type: "function",
  function: {
    name: "todo",
    description:
      "Manage this session's durable todo list. Ops: init {list:[{phase,items}]}, start {task}, done {task}, drop {task}, view.",
    parameters: {
      type: "object",
      properties: {
        op: { type: "string" },
        task: { type: "string" },
        list: { type: "array", items: { type: "object", properties: { phase: { type: "string" }, items: { type: "array", items: { type: "string" } } } } },
      },
      required: ["op"],
    },
  },
};

const ASK_TOOL_DEF: ToolDefinition = {
  type: "function",
  function: {
    name: "ask",
    description:
      "Ask the user structured follow-up questions when a task is ambiguous instead of guessing. Pauses the turn until they answer.",
    parameters: {
      type: "object",
      properties: {
        questions: {
          type: "array",
          items: {
            type: "object",
            properties: {
              id: { type: "string" },
              label: { type: "string" },
              description: { type: "string" },
            },
            required: ["id", "label"],
          },
        },
      },
      required: ["questions"],
    },
  },
};

function parseAskQuestions(args: Record<string, unknown> | undefined): PendingAskQuestion[] {
  const raw = Array.isArray(args?.questions) ? args!.questions : [];
  return raw.map((q, i) => {
    const item = q as { id?: unknown; label?: unknown; description?: unknown };
    return {
      id: typeof item.id === "string" ? item.id : `q${i}`,
      label: typeof item.label === "string" ? item.label : `Question ${i + 1}`,
      description: typeof item.description === "string" ? item.description : undefined,
    };
  });
}

function runTodoOp(s: Session, args: Record<string, unknown> | undefined): ToolResult {
  const op = typeof args?.op === "string" ? args.op : "view";
  switch (op) {
    case "init": {
      const list = Array.isArray(args?.list) ? args!.list : [];
      s.todos = [];
      for (const phase of list) {
        const p = phase as { phase?: unknown; items?: unknown };
        const phaseName = typeof p.phase === "string" ? p.phase : "";
        for (const item of Array.isArray(p.items) ? p.items : []) {
          s.todos.push({
            id: randomUUID().slice(0, 8),
            title: `${phaseName}: ${String(item)}`,
            completed: false,
          });
        }
      }
      break;
    }
    case "start":
    case "done":
    case "drop": {
      const title = typeof args?.task === "string" ? args.task : "";
      const t = s.todos.find((x) => x.title.includes(title));
      if (!t) return { content: `no todo matching "${title}"`, isError: true };
      if (op === "done" || op === "drop") t.completed = true;
      if (op === "start") t.active = true;
      break;
    }
    default:
      break;
  }
  const lines = s.todos.map((t) => `[${t.completed ? "x" : t.active ? ">" : " "}] ${t.title}`);
  return { content: lines.length > 0 ? lines.join("\n") : "(empty)", isError: false };
}

function isEmptyMessage(m: AgentMessage): boolean {
  return m.content.every((b) => !(b.text ?? "").trim() && b.type !== "tool_call");
}

function collectToolCalls(m: AgentMessage): ToolCall[] {
  return m.content
    .filter((b): b is ContentBlock & { type: "tool_call"; tool_call_id: string; name: string; arguments: string } =>
      b.type === "tool_call")
    .map((b) => ({
      id: b.tool_call_id,
      type: "function" as const,
      function: { name: b.name, args: safeParse(b.arguments) },
    }));
}

function safeParse(raw: string | undefined): Record<string, unknown> {
  if (!raw) return {};
  try {
    const v = JSON.parse(raw);
    return v && typeof v === "object" ? (v as Record<string, unknown>) : {};
  } catch {
    return {};
  }
}

function blockText(m: AgentMessage): string {
  return m.content
    .filter((b) => b.type === "text" || b.type === "thinking")
    .map((b) => b.text ?? "")
    .join("");
}

function pruneToolResult(text: string): string {
  if (text.length <= TOOL_RESULT_PRUNE_LIMIT) return text;
  const head = text.slice(0, TOOL_RESULT_KEEP_PER_SIDE);
  const tail = text.slice(-TOOL_RESULT_KEEP_PER_SIDE);
  return `${head}\n... [pruned ${text.length - 2 * TOOL_RESULT_KEEP_PER_SIDE} chars] ...\n${tail}`;
}

/** First user message + recent tail; advances past orphaned tool pairs. */
export function windowTranscript(messages: AgentMessage[], tail: number): AgentMessage[] {
  if (messages.length <= tail) return messages;
  const firstUserIdx = messages.findIndex((m) => m.role === "user");
  const head = firstUserIdx >= 0 ? [messages[firstUserIdx]] : [];
  let cut = messages.length - tail;
  // Never start the window inside a tool_call/tool_result group: advance the
  // cut until it sits right after a non-tool message boundary.
  while (
    cut < messages.length &&
    (messages[cut].role === "tool" ||
      hasToolCalls(messages[cut - 1]) ||
      (messages[cut].role === "assistant" && hasToolCalls(messages[cut])))
  ) {
    cut++;
  }
  const body = messages.slice(cut);
  const merged = [...head, ...body];
  // De-dup if head is already included.
  const seen = new Set<string>();
  return merged.filter((m) => {
    if (seen.has(m.id)) return false;
    seen.add(m.id);
    return true;
  });
}

function hasToolCalls(m: AgentMessage | undefined): boolean {
  return Boolean(m?.content.some((b) => b.type === "tool_call"));
}

function truncateForStorage(content: string): string {
  if (content.length <= 64_000) return content;
  return `${content.slice(0, 48_000)}\n... [truncated] ...\n${content.slice(-16_000)}`;
}

function sameDir(a: string, b: string): boolean {
  try {
    return fs.realpathSync(a) === fs.realpathSync(b);
  } catch {
    return a === b;
  }
}

function rolePrompt(role: string): string {
  switch (role) {
    case "architect":
      return "You are a senior software architect. Focus on design, module boundaries, and long-term maintainability.";
    case "reviewer":
      return "You are a meticulous code reviewer. Identify bugs, security issues, and spec violations with evidence.";
    case "ops":
      return "You are a DevOps engineer. Automate, script, and verify infrastructure changes carefully.";
    default:
      return "You are an expert coding agent embedded in a developer workspace. Work incrementally, verify changes, and keep responses concise.";
  }
}

function projectContext(folder: string): string {
  const parts = [`\n# Workspace\n${folder}`];
  try {
    const gitBranch = fs.existsSync(path.join(folder, ".git"));
    if (gitBranch) {
      parts.push("This is a git repository. Prefer small, verifiable changes.");
    }
  } catch {}
  const agentsMd = findAgentsGuidance(folder);
  if (agentsMd) parts.push(`\n# Project guidance (${agentsMd.file})\n${agentsMd.body.slice(0, 4000)}`);
  return parts.join("\n");
}

function findAgentsGuidance(folder: string): { file: string; body: string } | null {
  for (const candidate of ["AGENTS.md", ".agents/AGENTS.md"]) {
    try {
      const p = path.join(folder, candidate);
      if (fs.existsSync(p)) {
        return { file: candidate, body: fs.readFileSync(p, "utf-8") };
      }
    } catch {}
  }
  return null;
}


