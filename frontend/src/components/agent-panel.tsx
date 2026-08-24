import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  IconChevronDown,
  IconArrowDown,
  IconArrowUp,
  IconBolt,
  IconSend,
  IconX,
  IconSquare,
  IconTrashX,
  IconSearch,
  IconTerminal,
  IconSparkles,
  IconCheck,
  IconCpu,
  IconShield,
  IconFile,
  IconAlertTriangle,
} from "@tabler/icons-react";
import { cn } from "../lib/utils";
import {
  GetProviderProfiles,
  GetLLMConfig,
  SetActiveModel,
  ListAgentDefinitions,
  ListAgentSessionsForFolder,
  GetAgentSession,
  ApplyAgentDefinitionToSession,
  SearchFilename,
  ListSlashCommands,
  ExecuteSlashCommand,
  GenerateAICommitMessage,
  EventsOn,
  SetAgentAutoApprove,
  StopAgentTurn,
  ClearAgentSession,
  RespondAgentApproval,
  SendAgentMessage,
  type AgentMessage,
  type SlashCommand,
  type SessionMeta,
} from "../lib/native";
import { AgentChatBody, AskCard } from "./agent-chat";

// ---------------------------------------------------------------------------
// Token Usage Metric Badge
// ---------------------------------------------------------------------------
function TokenUsageBadge({ usage }: { usage: Record<string, number> | undefined }) {
  const inTok = usage?.prompt_tokens ?? usage?.PromptTokens ?? 0;
  const outTok = usage?.completion_tokens ?? usage?.CompletionTokens ?? 0;
  const hit = usage?.prompt_cache_hit_tokens ?? usage?.PromptCacheHitTokens ?? 0;
  const miss = usage?.prompt_cache_miss_tokens ?? usage?.PromptCacheMissTokens ?? 0;
  const cached = hit > 0 ? hit : usage?.cached_tokens ?? usage?.CachedTokens ?? 0;
  const hitPct = hit > 0 ? Math.round((hit / (hit + miss)) * 100) : null;
  if (inTok + outTok + cached === 0) return null;
  return (
    <span
      className="flex items-center gap-2 px-1.5 py-0.5 bg-[var(--bg-panel)] border border-[var(--border-default)] text-[10px] font-mono text-[var(--fg-tertiary)] rounded"
      title="Token usage: input / output / cached"
    >
      <span className="flex items-center gap-0.5">
        <IconArrowDown className="size-2.5 text-sky-400" />
        {inTok.toLocaleString()}
      </span>
      <span className="flex items-center gap-0.5">
        <IconArrowUp className="size-2.5 text-emerald-400" />
        {outTok.toLocaleString()}
      </span>
      {cached > 0 && (
        <span className="flex items-center gap-0.5" title={hitPct !== null ? `Cache: ${hit.toLocaleString()} hit / ${miss.toLocaleString()} miss` : "Cached tokens"}>
          <IconBolt className="size-2.5 text-amber-400" />
          {cached.toLocaleString()}
          {hitPct !== null ? ` (${hitPct}%)` : ""}
        </span>
      )}
    </span>
  );
}

// ---------------------------------------------------------------------------
// Live Activity Indicator
// ---------------------------------------------------------------------------
const STATUS_WORDS = [
  "Executing tools", "Inspecting code", "Analyzing AST", "Formulating plan", "Refactoring",
  "Resolving references", "Synthesizing", "Deliberating", "Evaluating",
];

function AgentStatusBar({ running }: { running: boolean }) {
  const [wordIdx, setWordIdx] = useState(0);
  const [elapsed, setElapsed] = useState(0);

  useEffect(() => {
    if (!running) return;
    const iv = setInterval(() => setWordIdx((i) => (i + 1) % STATUS_WORDS.length), 2000);
    return () => clearInterval(iv);
  }, [running]);

  useEffect(() => {
    if (!running) {
      setElapsed(0);
      return;
    }
    const start = Date.now();
    const iv = setInterval(() => setElapsed(Date.now() - start), 1000);
    return () => clearInterval(iv);
  }, [running]);

  const totalSec = Math.floor(elapsed / 1000);
  const h = Math.floor(totalSec / 3600);
  const m = Math.floor((totalSec % 3600) / 60);
  const s = totalSec % 60;
  const timer = h > 0 ? `${h}m ${m}s` : m > 0 ? `${m}m ${s}s` : `${s}s`;

  return (
    <div className="flex items-center gap-2 px-1 pb-1.5 text-[10px] font-mono text-[var(--fg-tertiary)] select-none">
      <span className="text-cyan-400 font-bold animate-spin">⠋</span>
      <span className="text-[var(--fg-secondary)] font-medium">{STATUS_WORDS[wordIdx]}…</span>
      <span className="text-[var(--fg-tertiary)]/50">•</span>
      <span className="text-[var(--fg-tertiary)]">{timer}</span>
      <span className="text-[var(--fg-tertiary)]/50">•</span>
      <span className="text-[var(--fg-tertiary)]">esc to stop</span>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Per-turn usage status line — mirrors terminal-agent footer conventions:
//   2026-08-23 16:28:19  in: 529  out: 41  cache 203K  tok/s: 3.6/s
// ---------------------------------------------------------------------------

function formatTokenCount(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${Math.round(n / 1_000)}K`;
  return String(n);
}

function UsageStatsLine({ usage, contextWindow }: { usage: NonNullable<AgentSessionLike["lastUsage"]>; contextWindow?: number }) {
  const ts = new Date(usage.at);
  const pad = (v: number) => String(v).padStart(2, "0");
  const stamp = `${ts.getFullYear()}-${pad(ts.getMonth() + 1)}-${pad(ts.getDate())} ${pad(ts.getHours())}:${pad(ts.getMinutes())}:${pad(ts.getSeconds())}`;
  const seconds = Math.max(usage.durationMs, 1) / 1000;
  const tps = (usage.completionTokens / seconds).toFixed(1);
  const ctxPct = contextWindow ? ((usage.promptTokens / contextWindow) * 100).toFixed(1) : null;
  const ctxLabel = ctxPct ? ` · ctx: ${ctxPct}%/${formatTokenCount(contextWindow ?? 0)}` : "";
  return (
    <div className="flex items-center gap-2 px-1 pb-1 text-[10px] font-mono text-[var(--fg-tertiary)] select-none">
      <span>{stamp}</span>
      <span>in: {usage.promptTokens}</span>
      <span>out: {usage.completionTokens}</span>
      {usage.cachedTokens > 0 && <span>cache {formatTokenCount(usage.cachedTokens)}</span>}
      <span>tok/s: {tps}/s</span>
      <span>{ctxLabel.replace(" · ", "")}</span>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Tool Approval Card
// ---------------------------------------------------------------------------
interface PendingTool {
  id?: string;
  name?: string;
  args?: unknown;
}

function ApprovalCard({
  tools,
  onRespond,
}: {
  tools: PendingTool[];
  onRespond: (approve: boolean, autoAll: boolean) => void;
}) {
  return (
    <div className="mx-3 mb-2 border border-amber-500/40 bg-amber-500/10 rounded-xl p-2.5 space-y-2 shrink-0">
      <div className="flex items-center gap-1.5 text-[11px] font-semibold text-amber-300 font-mono">
        <IconShield className="size-3.5" />
        <span>Tool approval required{tools.length > 1 ? ` (${tools.length})` : ""}</span>
      </div>
      <div className="space-y-0.5 max-h-24 overflow-y-auto">
        {tools.map((t, i) => {
          let argText = "";
          if (typeof t.args === "string") argText = t.args;
          else if (t.args != null) {
            try { argText = JSON.stringify(t.args); } catch { argText = ""; }
          }
          return (
            <div key={t.id ?? i} className="font-mono text-[11px] text-[var(--fg-secondary)] truncate">
              <span className="text-amber-400">{t.name || "tool"}</span>
              {argText ? ` ${argText}` : ""}
            </div>
          );
        })}
      </div>
      <div className="flex items-center justify-end gap-1.5 pt-0.5">
        <button
          onClick={() => onRespond(false, false)}
          className="px-2 py-1 text-[11px] rounded border border-[var(--border-default)] text-[var(--fg-secondary)] hover:text-white hover:bg-[var(--bg-surface-hover)] cursor-pointer transition-colors"
        >
          Deny
        </button>
        <button
          onClick={() => onRespond(true, true)}
          className="px-2 py-1 text-[11px] rounded border border-amber-500/50 text-amber-300 hover:bg-amber-500/20 cursor-pointer transition-colors"
        >
          Approve all
        </button>
        <button
          onClick={() => onRespond(true, false)}
          className="px-2.5 py-1 text-[11px] font-semibold bg-amber-500 hover:bg-amber-400 text-black rounded cursor-pointer transition-colors"
        >
          Approve
        </button>
      </div>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Error Banner
// ---------------------------------------------------------------------------
function ErrorBanner({ message, onDismiss }: { message: string; onDismiss: () => void }) {
  return (
    <div className="mx-3 mb-2 flex items-start gap-2 border border-red-500/40 bg-red-500/10 rounded-xl px-2.5 py-2 shrink-0">
      <IconAlertTriangle className="size-3.5 text-red-400 shrink-0 mt-0.5" />
      <div className="flex-1 min-w-0 font-mono text-[11px] text-red-300 whitespace-pre-wrap break-words select-text">
        {message}
      </div>
      <button
        onClick={onDismiss}
        className="p-0.5 text-red-300/70 hover:text-white cursor-pointer rounded transition-colors shrink-0"
        title="Dismiss error"
      >
        <IconX className="size-3" />
      </button>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Agent Terminal Panel — streams via daemon WS events; deltas patch a local
// live-transcript buffer, full session is refetched only once per turn end.
// ---------------------------------------------------------------------------

/** Loose structural view of a session — callers pass tabs/meta objects. */
interface AgentSessionLike {
  id: string;
  name?: string;
  role?: string;
  role_filter?: string;
  projectFolder?: string;
  dialect?: string;
  state?: string;
  autoApprove?: boolean;
  auto_approve?: boolean;
  token_usage?: Record<string, number>;
  contextWindow?: number;
  lastUsage?: {
    at: number;
    promptTokens: number;
    completionTokens: number;
    cachedTokens: number;
    durationMs: number;
  };
}

export function AgentChatPanel({
  session,
  onClose,
  onAgentLaunched,
}: {
  session: AgentSessionLike;
  onClose: () => void;
  onAgentLaunched?: (sessionId: string) => void;
}) {
  const sessionId = session.id;
  const [meta, setMeta] = useState<AgentSessionLike>(session);
  // Transcript = last fully fetched session messages + live turn overlay.
  const [baseMessages, setBaseMessages] = useState<AgentMessage[]>([]);
  const [liveFinalized, setLiveFinalized] = useState<AgentMessage[]>([]);
  const [liveCurrent, setLiveCurrent] = useState<AgentMessage | null>(null);
  const [optimisticUser, setOptimisticUser] = useState<AgentMessage | null>(null);
  const [running, setRunning] = useState(false);
  const [pendingTools, setPendingTools] = useState<PendingTool[] | null>(null);
  const [pendingAsk, setPendingAsk] = useState<Record<string, unknown>[] | null>(null);
  const [slashCommands, setSlashCommands] = useState<SlashCommand[]>([]);
  const [showSlashMenu, setShowSlashMenu] = useState(false);
  const [slashHighlight, setSlashHighlight] = useState(0);
  const [errorBanner, setErrorBanner] = useState<string | null>(null);

  const [inputText, setInputText] = useState("");
  const [mentionFiles, setMentionFiles] = useState<string[]>([]);
  const [showMentionMenu, setShowMentionMenu] = useState(false);
  const [mentionResults, setMentionResults] = useState<string[]>([]);
  const [profiles, setProfiles] = useState<Record<string, unknown>[]>([]);
  const [activeModel, setActiveModelName] = useState<string>("");
  const [activeProviderId, setActiveProviderId] = useState<string>("");
  const [showModelPicker, setShowModelPicker] = useState(false);
  const [modelSearchQuery, setModelSearchQuery] = useState("");
  const [agentDefs, setAgentDefs] = useState<Record<string, unknown>[]>([]);
  const [showAgentPicker, setShowAgentPicker] = useState(false);

  const modelPickerRef = useRef<HTMLDivElement>(null);
  const agentPickerRef = useRef<HTMLDivElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const mentionTimerRef = useRef<number | undefined>(undefined);
  const slashTimerRef = useRef<number | undefined>(undefined);
  const listRefreshTimerRef = useRef<number | undefined>(undefined);
  // Streaming tool index → tool_call_id, so tool_delta/tool_end can find the
  // right tool_call block inside the optimistic assistant message.
  const toolIndexRef = useRef<Map<number, string>>(new Map());

  // ------------------------------------------------------------------
  // Full session fetch — once on open and once per turn end (contract §4).
  // ------------------------------------------------------------------
  useEffect(() => {
    let cancelled = false;
    setBaseMessages([]);
    setLiveFinalized([]);
    setLiveCurrent(null);
    setOptimisticUser(null);
    setPendingTools(null);
    setPendingAsk(null);
    setErrorBanner(null);
    toolIndexRef.current = new Map();
    GetAgentSession(sessionId).then((full) => {
      if (cancelled || !full) return;
      setMeta((prev) => ({ ...prev, ...full }));
      setBaseMessages(Array.isArray(full.messages) ? full.messages : []);
      setRunning(full.state === "running");
    }).catch(() => {});
    return () => { cancelled = true; };
  }, [sessionId]);

  // ------------------------------------------------------------------
  // Session list refresh — coalesced ≥500ms so event bursts trigger one
  // ListAgentSessionsForFolder call, then merge this session's fresh meta.
  // ------------------------------------------------------------------
  const scheduleListRefresh = useCallback(() => {
    clearTimeout(listRefreshTimerRef.current);
    listRefreshTimerRef.current = window.setTimeout(async () => {
      listRefreshTimerRef.current = undefined;
      try {
        const folder = String(meta.projectFolder ?? "");
        const list = await ListAgentSessionsForFolder(folder);
        const found = list.find((s) => s.id === sessionId);
        if (found) setMeta((prev) => ({ ...prev, ...found }));
      } catch { /* ignore */ }
    }, 500);
  }, [sessionId, meta.projectFolder]);

  const loadProfiles = useCallback(async () => {
    try {
      const list = await GetProviderProfiles();
      setProfiles(Array.isArray(list) ? list : []);
      const cfg = await GetLLMConfig();
      if (cfg) {
        if (cfg.model) setActiveModelName(String(cfg.model));
        if (cfg.provider_id) setActiveProviderId(String(cfg.provider_id));
      }
    } catch { /* ignore */ }
  }, []);

  const loadAgentDefs = useCallback(async () => {
    try {
      const list = await ListAgentDefinitions();
      setAgentDefs(Array.isArray(list) ? list : []);
    } catch { /* ignore */ }
  }, []);

  // ------------------------------------------------------------------
  // WS event wiring — deltas patch the live buffer; NO refetch per delta.
  // ------------------------------------------------------------------
  useEffect(() => {
    const mine = (payload: Record<string, unknown>) =>
      payload && typeof payload === "object" && (payload.id === sessionId || payload.id == null);

    /** Append a delta to a typed block (creating it at the tail if absent). */
    const appendDelta = (kind: "text" | "thinking", delta: string) => {
      if (!delta) return;
      setLiveCurrent((cur) => {
        const msg: AgentMessage = cur ?? {
          id: `live-${Date.now()}`,
          role: "assistant",
          content: [],
          timestamp: new Date().toISOString(),
          state: "running",
        };
        const content = [...msg.content];
        const idx = content.map((b) => b.type).lastIndexOf(kind);
        if (idx >= 0) {
          content[idx] = { ...content[idx], text: (content[idx].text ?? "") + delta };
        } else {
          content.push({ type: kind, text: delta });
        }
        return { ...msg, content };
      });
    };

    const unsubs = [
      EventsOn("agent:turn_start", (payload: Record<string, unknown>) => {
        if (!mine(payload)) return;
        toolIndexRef.current = new Map();
        setLiveFinalized([]);
        setLiveCurrent(null);
        setPendingTools(null);
        setPendingAsk(null);
        setErrorBanner(null);
        setRunning(true);
        scheduleListRefresh();
      }),
      EventsOn("agent:message_delta", (payload: Record<string, unknown>) => {
        if (!mine(payload)) return;
        appendDelta(payload.kind === "thinking" ? "thinking" : "text", String(payload.delta ?? ""));
      }),
      EventsOn("agent:thinking_delta", (payload: Record<string, unknown>) => {
        if (!mine(payload)) return;
        appendDelta("thinking", String(payload.delta ?? ""));
      }),
      EventsOn("agent:message_end", (payload: Record<string, unknown>) => {
        if (!mine(payload)) return;
        const finalMsg = payload.message as AgentMessage | undefined;
        if (finalMsg) {
          setLiveFinalized((prev) => [...prev, finalMsg]);
          setLiveCurrent(null);
        }
      }),
      EventsOn("agent:tool_start", (payload: Record<string, unknown>) => {
        if (!mine(payload)) return;
        const index = Number(payload.index ?? 0);
        const toolCallId = String(payload.toolCallId ?? payload.tool_call_id ?? `tool-${index}`);
        toolIndexRef.current.set(index, toolCallId);
        const rawArgs = payload.args;
        const argText = typeof rawArgs === "string" ? rawArgs : JSON.stringify(rawArgs ?? {});
        setLiveCurrent((cur) => {
          const msg: AgentMessage = cur ?? {
            id: `live-${Date.now()}`,
            role: "assistant",
            content: [],
            timestamp: new Date().toISOString(),
            state: "running",
          };
          return {
            ...msg,
            content: [...msg.content, { type: "tool_call", tool_call_id: toolCallId, name: String(payload.name ?? "tool"), arguments: argText }],
          };
        });
      }),
      EventsOn("agent:tool_delta", (payload: Record<string, unknown>) => {
        if (!mine(payload)) return;
        const index = Number(payload.index ?? 0);
        const toolCallId = toolIndexRef.current.get(index) ?? "";
        const chunk = String(payload.args ?? "");
        if (!chunk) return;
        setLiveCurrent((cur) => {
          if (!cur) return cur;
          const content = [...cur.content];
          const idx = content.findIndex((b) => b.type === "tool_call" && b.tool_call_id === toolCallId);
          if (idx < 0) return cur;
          content[idx] = { ...content[idx], arguments: (content[idx].arguments ?? "") + chunk };
          return { ...cur, content };
        });
      }),
      EventsOn("agent:tool_end", (payload: Record<string, unknown>) => {
        if (!mine(payload)) return;
        const index = Number(payload.index ?? 0);
        const toolCallId = toolIndexRef.current.get(index)
          ?? String(payload.toolCallId ?? payload.tool_call_id ?? "");
        // Attach the result as a synthetic tool message — the transcript model
        // already renders tool_result blocks against their matching tool_call.
        const resultMsg: AgentMessage = {
          id: `live-toolres-${toolCallId}-${Date.now()}`,
          role: "tool",
          content: [{
            type: "tool_result",
            tool_call_id: toolCallId,
            text: typeof payload.result === "string" ? payload.result : JSON.stringify(payload.result ?? ""),
            is_error: Boolean(payload.isError ?? payload.is_error),
          }],
          timestamp: new Date().toISOString(),
        };
        setLiveFinalized((prev) => [...prev, resultMsg]);
        scheduleListRefresh();
      }),
      EventsOn("agent:approval_required", (payload: Record<string, unknown>) => {
        if (!mine(payload)) return;
        setPendingTools(Array.isArray(payload.pendingTools) ? payload.pendingTools as PendingTool[] : []);
      }),
      EventsOn("agent:ask", (payload: Record<string, unknown>) => {
        if (!mine(payload)) return;
        setPendingAsk(Array.isArray(payload.questions) ? payload.questions as Record<string, unknown>[] : []);
      }),
      EventsOn("agent:error", (payload: Record<string, unknown>) => {
        if (!mine(payload)) return;
        const message = String(payload.message ?? "Unknown agent error");
        // Expected control-flow states are not user-facing failures — they'd
        // just flash a red banner during normal double-sends or stops.
        if (/already running|turn was stopped|abort/i.test(message)) {
          console.info("[agent]", message);
          return;
        }
        setErrorBanner(message);
      }),
      EventsOn("agent:notice", (payload: Record<string, unknown>) => {
        if (!mine(payload)) return;
        const message = String(payload.message ?? "");
        if (!message) return;
        // Single source of truth for command cards: the daemon persists the
        // notice and broadcasts it here. Local appends are NOT done in the
        // send handler, so this never doubles up.
        setBaseMessages((prev) => [
          ...prev,
          {
            id: `sys-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`,
            role: "system",
            content: [{ type: "text", text: message }],
            timestamp: new Date().toISOString(),
          },
        ]);
        setOptimisticUser(null);
        setRunning(false);
      }),
      EventsOn("agent:turn_end", (payload: Record<string, unknown>) => {
        if (!mine(payload)) return;
        setRunning(false);
        setPendingTools(null);
        setPendingAsk(null);
        scheduleListRefresh();
        // Reconcile once: replace base + live overlay with persisted transcript.
        GetAgentSession(sessionId).then((full) => {
          if (!full) return;
          setMeta((prev) => ({ ...prev, ...full }));
          setBaseMessages(Array.isArray(full.messages) ? full.messages : []);
          setLiveFinalized([]);
          setLiveCurrent(null);
          setOptimisticUser(null);
        }).catch(() => {});
      }),
      EventsOn("agent:updated", scheduleListRefresh),
      EventsOn("session:opened", scheduleListRefresh),
      EventsOn("session:closed", scheduleListRefresh),
      EventsOn("agent:config:changed", () => {
        loadProfiles();
        loadAgentDefs();
      }),
    ];

    loadProfiles();
    loadAgentDefs();

    return () => {
      clearTimeout(mentionTimerRef.current);
      clearTimeout(listRefreshTimerRef.current);
      unsubs.forEach((u) => typeof u === "function" && u());
    };
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [sessionId, scheduleListRefresh, loadProfiles, loadAgentDefs]);

  // Close dropdowns on outside click
  useEffect(() => {
    const handleOutsideClick = (e: MouseEvent) => {
      if (modelPickerRef.current && !modelPickerRef.current.contains(e.target as Node)) {
        setShowModelPicker(false);
      }
      if (agentPickerRef.current && !agentPickerRef.current.contains(e.target as Node)) {
        setShowAgentPicker(false);
      }
    };
    window.addEventListener("mousedown", handleOutsideClick);
    return () => window.removeEventListener("mousedown", handleOutsideClick);
  }, []);

  // Autosize textarea
  useEffect(() => {
    const el = textareaRef.current;
    if (!el) return;
    el.style.height = "auto";
    el.style.height = `${Math.min(el.scrollHeight, 160)}px`;
  }, [inputText]);

  async function handleSelectModel(providerId: string, model: string) {
    setActiveModelName(model);
    setActiveProviderId(providerId);
    setShowModelPicker(false);
    setModelSearchQuery("");
    try {
      await SetActiveModel(providerId, model);
    } catch { /* ignore */ }
  }

  async function handleLaunchAgentDef(def: Record<string, unknown>) {
    setShowAgentPicker(false);
    try {
      await ApplyAgentDefinitionToSession(sessionId, String(def.id ?? ""));
      onAgentLaunched?.(sessionId);
    } catch (err) {
      console.error("Failed to apply agent:", err);
    }
  }

  function pushOptimisticUser(text: string) {
    setOptimisticUser({
      id: `local-user-${Date.now()}`,
      role: "user",
      content: [{ type: "text", text }],
      timestamp: new Date().toISOString(),
    });
  }

  async function handleSendMessage(customText?: string) {
    const text = customText ?? inputText;
    if (!text.trim()) return;

    // Backend slash commands (/whoami, /login, /logout, /usage, skills, ...)
    // run through the daemon bridge. Local UI commands below keep their
    // handlers and take precedence.
    const firstToken = text.trim().split(/\s+/)[0] ?? "";
    const firstTokenLower = firstToken.toLowerCase();
    if (firstToken.startsWith("/") && !["/commit", "/clear", "/model"].includes(firstTokenLower)) {
      let known: SlashCommand[] = [];
      try {
        known = await ListSlashCommands();
      } catch { /* bridge unavailable — fall through to plain message */ }
      if (known.some((c) => c.name.toLowerCase() === firstTokenLower)) {
        setInputText("");
        setMentionFiles([]);
        setShowMentionMenu(false);
        // Echo shows immediately; the output card arrives via agent:notice,
        // which is also what clears this state. Never set running here.
        pushOptimisticUser(text);
        try {
          await ExecuteSlashCommand(sessionId, text);
        } catch (err) {
          console.error(err);
          setRunning(false);
          setOptimisticUser(null);
          setErrorBanner(err instanceof Error ? err.message : String(err));
        }
        return;
      }
    }

    if (text.startsWith("/commit")) {
      setInputText("");
      pushOptimisticUser(text);
      try {
        const commitMsg = await GenerateAICommitMessage("", activeProviderId, activeModel);
        await SendAgentMessage(sessionId, `/commit\n\nProposed commit message:\n\`${commitMsg}\``, []);
      } catch {
        await SendAgentMessage(sessionId, text, []);
      }
      return;
    }

    if (text.startsWith("/clear")) {
      setInputText("");
      setBaseMessages([]);
      setLiveFinalized([]);
      setLiveCurrent(null);
      setOptimisticUser(null);
      ClearAgentSession(sessionId);
      return;
    }

    if (text.startsWith("/model")) {
      setInputText("");
      setShowModelPicker(true);
      return;
    }

    setInputText("");
    setMentionFiles([]);
    setShowMentionMenu(false);
    pushOptimisticUser(text);
    // Optimistically enter running state so the composer flips to Stop
    // immediately; the agent:turn_start echo confirms it server-side.
    setRunning(true);
    try {
      await SendAgentMessage(sessionId, text, mentionFiles);
    } catch (err) {
      setRunning(false);
      console.error(err);
      setErrorBanner(err instanceof Error ? err.message : String(err));
    }
  }

  async function handleApproval(approve: boolean, autoAll = false) {
    setPendingTools(null);
    try {
      await RespondAgentApproval(sessionId, approve, autoAll);
    } catch (err) {
      console.error(err);
    }
  }

  const handleStop = () => {
    try {
      StopAgentTurn(sessionId);
    } catch (err) {
      console.error(err);
    }
  };

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape" && running) {
        e.preventDefault();
        e.stopPropagation();
        handleStop();
      }
    };
    window.addEventListener("keydown", onKey, true);
    return () => window.removeEventListener("keydown", onKey, true);
  }, [running, sessionId]);

  async function handleInputChange(e: React.ChangeEvent<HTMLTextAreaElement>) {
    const val = e.target.value;
    setInputText(val);

    // Slash command autocomplete while typing the first token.
    const firstToken = val.split(" ")[0] ?? "";
    if (firstToken.startsWith("/")) {
      clearTimeout(slashTimerRef.current);
      slashTimerRef.current = window.setTimeout(async () => {
        try {
          const cmds = await ListSlashCommands(firstToken);
          setSlashCommands(cmds);
          setSlashHighlight(0);
          setShowSlashMenu(cmds.length > 0);
        } catch {
          setSlashCommands([]);
          setShowSlashMenu(false);
        }
      }, 150);
    } else {
      clearTimeout(slashTimerRef.current);
      setShowSlashMenu(false);
    }

    const atIndex = val.lastIndexOf("@");
    if (atIndex !== -1 && atIndex >= val.length - 25 && !val.slice(atIndex).includes(" ")) {
      const query = val.slice(atIndex + 1);
      clearTimeout(mentionTimerRef.current);
      mentionTimerRef.current = window.setTimeout(async () => {
        try {
          const results = await SearchFilename(query, 8);
          setMentionResults(results.map((r: Record<string, unknown>) => String(r.path ?? r.Path ?? "")));
          setShowMentionMenu(true);
        } catch {
          setMentionResults([]);
        }
      }, 150);
    } else {
      setShowMentionMenu(false);
    }
  }

  function pickMention(path: string) {
    const atIndex = inputText.lastIndexOf("@");
    if (atIndex !== -1) setInputText(inputText.slice(0, atIndex).replace(/\s$/, "") + " ");
    else setInputText("");
    setMentionFiles((prev) => (prev.includes(path) ? prev : [...prev, path]));
    setShowMentionMenu(false);
    textareaRef.current?.focus();
  }

  function pickSlashCommand(name: string) {
    const rest = inputText.split(" ").slice(1).join(" ");
    setInputText(rest ? `${name} ${rest}` : `${name} `);
    setShowSlashMenu(false);
    textareaRef.current?.focus();
  }

  // Filtered models for dropdown
  type ProfileRow = Record<string, unknown> & { filteredModels?: string[] };
  const filteredProfiles = useMemo<ProfileRow[]>(() => {
    if (!modelSearchQuery.trim()) return profiles;
    const q = modelSearchQuery.toLowerCase();
    return profiles.map((p) => {
      const allModels = Array.isArray(p.selected_models) ? p.selected_models : Array.isArray(p.available_models) ? p.available_models : [];
      const matched = allModels.filter((m) => String(m).toLowerCase().includes(q));
      return { ...p, filteredModels: matched };
    }).filter((p) => Array.isArray(p.filteredModels) && p.filteredModels.length > 0);
  }, [profiles, modelSearchQuery]);

  // Full transcript = persisted base + optimistic user prompt + live turn.
  const displayMessages = useMemo(
    () => [
      ...baseMessages,
      ...(optimisticUser ? [optimisticUser] : []),
      ...liveFinalized,
      ...(liveCurrent ? [liveCurrent] : []),
    ],
    [baseMessages, optimisticUser, liveFinalized, liveCurrent]
  );

  const yoloOn = Boolean(meta.autoApprove ?? meta.auto_approve ?? session.autoApprove ?? session.auto_approve);
  const providerProfile = profiles.find((p) => p.id === activeProviderId);
  const providerLabel = String(providerProfile?.name ?? providerProfile?.provider ?? activeProviderId);

  const chatSession = { ...session, ...meta, state: running ? "running" : String(meta.state ?? "idle") };

  return (
    <div className="flex flex-col h-full w-full bg-[var(--bg-app)] relative overflow-hidden font-sans text-xs">
      {/* Terminal Top Control Bar */}
      <div className="flex items-center justify-between px-3 py-1.5 bg-[var(--bg-sidebar)] border-b border-[var(--border-default)] select-none shrink-0 font-mono">
        <div className="flex items-center space-x-2 min-w-0">
          <IconTerminal className="size-3.5 text-cyan-400 shrink-0" />
          <span className="font-bold text-[var(--fg-primary)] truncate max-w-44 text-[11px]">{String(meta.name ?? session.name)}</span>

          {/* Per-response usage analytics (latest call) */}
          {meta.lastUsage && (
            <div className="min-w-0 overflow-hidden">
              <UsageStatsLine usage={meta.lastUsage} contextWindow={meta.contextWindow} />
            </div>
          )}

          {/* Role Picker */}
          <div className="relative" ref={agentPickerRef}>
            <button
              onClick={() => setShowAgentPicker(!showAgentPicker)}
              className="px-1.5 py-0.5 bg-[var(--accent)]/15 border border-[var(--accent)]/40 text-[var(--accent)] hover:bg-[var(--accent)]/25 rounded text-[10px] uppercase tracking-wide flex items-center gap-1 cursor-pointer transition-colors"
            >
              <span>{String(meta.role ?? session.role ?? "coding")}</span>
              <IconChevronDown className="size-2.5 opacity-70" />
            </button>
            {showAgentPicker && (
              <div className="absolute top-full left-0 mt-1 z-40 w-56 bg-[var(--bg-sidebar)] border border-[var(--border-default)] shadow-2xl p-1 rounded font-sans text-xs max-h-60 overflow-y-auto">
                <div className="text-[10px] uppercase font-bold text-[var(--fg-tertiary)] px-2 py-1 tracking-wider">Agent Role</div>
                {agentDefs.map((def, i) => (
                  <button
                    key={String(def.id ?? i)}
                    onClick={() => handleLaunchAgentDef(def)}
                    className="w-full text-left px-2 py-1.5 hover:bg-[var(--bg-panel)] rounded text-[var(--fg-primary)] cursor-pointer"
                  >
                    <div className="font-semibold text-[11px]">{String(def.name ?? def.id)}</div>
                    {typeof def.description === "string" && (
                      <div className="text-[9px] text-[var(--fg-tertiary)] truncate">{def.description}</div>
                    )}
                  </button>
                ))}
              </div>
            )}
          </div>
        </div>

        {/* Right Tools: Token Usage, Clear, Close */}
        <div className="flex items-center space-x-1.5">
          {/* Token usage badge */}
          {(Object.keys(meta.token_usage ?? {}).length > 0 || Object.keys(session.token_usage ?? {}).length > 0) && (
            <TokenUsageBadge usage={meta.token_usage ?? session.token_usage} />
          )}

          {/* Clear Button */}
          <button
            onClick={() => {
              if (window.confirm("Clear conversation history for this session?")) {
                ClearAgentSession(sessionId);
              }
            }}
            className="p-1 text-[var(--fg-secondary)] hover:text-white hover:bg-[var(--bg-surface-hover)] border border-[var(--border-default)] rounded cursor-pointer transition-colors"
            title="Clear transcript"
          >
            <IconTrashX className="size-3" />
          </button>

          {onClose && (
            <button onClick={onClose} className="hover:text-white cursor-pointer rounded p-1 text-[var(--fg-tertiary)]">
              <IconX className="size-3.5" />
            </button>
          )}
        </div>
      </div>

      {/* Terminal Message Stream (No cards, no bubbles) */}
      <AgentChatBody
        messages={displayMessages}
        session={chatSession}
        onApprove={handleApproval}
        onDeny={() => handleApproval(false)}
      />

      {/* Composer Dock: approval / ask / error surface above the input bar */}
      <div className="shrink-0 relative pb-3">
        {running && <AgentStatusBar running={true} />}

        {errorBanner && <ErrorBanner message={errorBanner} onDismiss={() => setErrorBanner(null)} />}

        {pendingTools && pendingTools.length > 0 && (
          <ApprovalCard tools={pendingTools} onRespond={handleApproval} />
        )}

        {pendingAsk && pendingAsk.length > 0 && (
          <div className="mx-3 mb-2">
            <AskCard sessionId={sessionId} questions={pendingAsk} />
          </div>
        )}

        {/* Floating Rounded Composer Bar */}
        <div className="px-3 relative">
          <div className="rounded-2xl border border-[var(--border-default)] focus-within:border-[var(--accent-primary)] bg-[var(--bg-panel)] shadow-lg transition-colors">
            {/* @-file mention chips */}
            {mentionFiles.length > 0 && (
              <div className="flex flex-wrap gap-1 px-2.5 pt-2.5">
                {mentionFiles.map((f) => (
                  <span
                    key={f}
                    className="flex items-center gap-1 pl-1.5 pr-0.5 py-0.5 bg-[var(--accent)]/10 border border-[var(--accent)]/30 rounded-full text-[10px] font-mono text-[var(--accent)] max-w-52"
                    title={f}
                  >
                    <IconFile className="size-2.5 shrink-0" />
                    <span className="truncate">{f.split("/").pop()}</span>
                    <button
                      onClick={() => setMentionFiles((prev) => prev.filter((x) => x !== f))}
                      className="p-0.5 rounded-full hover:bg-[var(--bg-surface-hover)] cursor-pointer shrink-0"
                      title={`Remove ${f}`}
                    >
                      <IconX className="size-2.5" />
                    </button>
                  </span>
                ))}
              </div>
            )}

            <textarea
              ref={textareaRef}
              value={inputText}
              onChange={handleInputChange}
              onKeyDown={(e) => {
                if (showSlashMenu && slashCommands.length > 0) {
                  if (e.key === "ArrowDown") {
                    e.preventDefault();
                    setSlashHighlight((p) => (p + 1) % slashCommands.length);
                    return;
                  }
                  if (e.key === "ArrowUp") {
                    e.preventDefault();
                    setSlashHighlight((p) => (p - 1 + slashCommands.length) % slashCommands.length);
                    return;
                  }
                  if (e.key === "Tab" || (e.key === "Enter" && !e.shiftKey)) {
                    e.preventDefault();
                    const picked = slashCommands[slashHighlight];
                    if (picked) pickSlashCommand(picked.name);
                    return;
                  }
                  if (e.key === "Escape") {
                    e.preventDefault();
                    setShowSlashMenu(false);
                    return;
                  }
                }
                if (e.key === "Enter" && !e.shiftKey) {
                  e.preventDefault();
                  if (!running) handleSendMessage();
                }
              }}
              placeholder="Ask agent, type /commands or @file..."
              rows={1}
              className="w-full bg-transparent text-xs text-[var(--fg-primary)] focus:outline-none resize-none font-mono leading-relaxed px-3 pt-2.5 pb-1 max-h-40"
            />

            {/* Bottom controls: model pill — spacer — YOLO — send/stop */}
            <div className="flex items-center gap-1.5 px-2 pb-2 pt-0.5">
              {/* Model pill */}
              <div className="relative min-w-0" ref={modelPickerRef}>
                <button
                  onClick={() => setShowModelPicker(!showModelPicker)}
                  className="flex items-center gap-1.5 px-2 py-1 bg-[var(--bg-app)] hover:bg-[var(--bg-surface-hover)] border border-[var(--border-default)] text-[var(--fg-secondary)] hover:text-[var(--fg-primary)] rounded-full cursor-pointer font-mono text-[10px] transition-colors max-w-48"
                  title={`${providerLabel || "provider"} · ${activeModel || "model"}`}
                >
                  <IconCpu className="size-3 text-purple-400 shrink-0" />
                  <span className="truncate">
                    {[providerLabel, activeModel].filter(Boolean).join(" · ") || "Model"}
                  </span>
                  <IconChevronDown className="size-2.5 opacity-60 shrink-0" />
                </button>

                {showModelPicker && (
                  <div className="absolute bottom-full left-0 mb-2 z-50 w-72 bg-[var(--bg-sidebar)] border border-[var(--border-default)] shadow-2xl p-2 rounded text-xs font-sans max-h-80 flex flex-col">
                    <div className="relative mb-2">
                      <IconSearch className="size-3 absolute left-2 top-2 text-[var(--fg-tertiary)]" />
                      <input
                        autoFocus
                        value={modelSearchQuery}
                        onChange={(e) => setModelSearchQuery(e.target.value)}
                        placeholder="Search models..."
                        className="w-full bg-[var(--bg-panel)] border border-[var(--border-default)] pl-7 pr-2 py-1 text-[11px] text-[var(--fg-primary)] rounded focus:outline-none focus:border-[var(--accent-primary)] font-mono"
                      />
                    </div>

                    <div className="overflow-y-auto flex-1 space-y-2 pr-0.5">
                      {filteredProfiles.map((p, pi) => {
                        const pid = String(p.id ?? p.name ?? pi);
                        const models = (Array.isArray(p.filteredModels) ? p.filteredModels : Array.isArray(p.selected_models) ? p.selected_models : p.available_models ?? []) as string[];
                        if (models.length === 0) return null;
                        return (
                          <div key={pid}>
                            <div className="px-2 py-0.5 text-[9px] uppercase tracking-wider text-[var(--fg-tertiary)] font-bold">
                              {String(p.name ?? pid)}
                            </div>
                            {models.map((m) => {
                              const isActive = activeModel === m;
                              return (
                                <button
                                  key={m}
                                  onClick={() => handleSelectModel(pid, m)}
                                  className={cn(
                                    "w-full text-left px-2 py-1 hover:bg-[var(--bg-panel)] rounded text-[var(--fg-primary)] truncate font-mono text-[11px] flex items-center justify-between cursor-pointer group",
                                    isActive && "bg-[var(--bg-surface-active)] text-[var(--fg-on-active)]"
                                  )}
                                >
                                  <span className="truncate">{m}</span>
                                  {isActive && <IconCheck className="size-3 text-emerald-400 shrink-0 ml-1" />}
                                </button>
                              );
                            })}
                          </div>
                        );
                      })}
                    </div>
                  </div>
                )}
              </div>

              {/* Context window usage chip */}
              {meta.lastUsage && meta.contextWindow ? (
                <span className="text-[10px] font-mono text-[var(--fg-tertiary)] whitespace-nowrap">
                  ctx: {((meta.lastUsage.promptTokens / meta.contextWindow) * 100).toFixed(1)}%/{formatTokenCount(meta.contextWindow)}
                </span>
              ) : null}

              <div className="flex-1" />

              {/* YOLO toggle — LEFT of send, bolt icon, red when ON */}
              <button
                onClick={() => {
                  const next = !yoloOn;
                  SetAgentAutoApprove(sessionId, next);
                  setMeta((prev) => ({ ...prev, autoApprove: next }));
                }}
                title={yoloOn ? "YOLO Mode ACTIVE: All tool actions auto-approved" : "YOLO Mode OFF: Click to auto-approve tools"}
                className={cn(
                  "p-1.5 rounded-full border cursor-pointer flex items-center transition-colors",
                  yoloOn
                    ? "bg-red-500 border-red-400 text-white shadow-[0_0_8px_rgba(239,68,68,0.6)]"
                    : "bg-[var(--bg-app)] border-[var(--border-default)] text-[var(--fg-tertiary)] hover:text-[var(--fg-secondary)]"
                )}
              >
                <IconBolt className="size-3.5" fill={yoloOn ? "currentColor" : "none"} />
              </button>

              {/* Send morphs to Stop while a turn runs for this session */}
              {running ? (
                <button
                  onClick={handleStop}
                  className="p-1.5 bg-red-600 hover:bg-red-500 text-white rounded-full cursor-pointer transition-colors"
                  title="Stop turn (Esc)"
                >
                  <IconSquare className="size-3" fill="currentColor" />
                </button>
              ) : (
                <button
                  onClick={() => handleSendMessage()}
                  disabled={!inputText.trim()}
                  className="p-1.5 bg-[var(--accent-primary)] hover:bg-[var(--accent-hover)] text-black rounded-full cursor-pointer disabled:opacity-40 transition-colors"
                  title="Send (Enter)"
                >
                  <IconSend className="size-3" />
                </button>
              )}
            </div>
          </div>

          {/* Slash command autocomplete popup */}
          {showSlashMenu && slashCommands.length > 0 && (
            <div className="absolute bottom-full mb-2 left-3 right-3 z-50 bg-[var(--bg-sidebar)] border border-[var(--border-default)] shadow-2xl max-h-48 overflow-y-auto p-1 space-y-0.5 rounded-xl font-mono text-xs">
              <div className="text-[9px] uppercase tracking-wider text-[var(--fg-tertiary)] font-bold px-2 py-0.5">Commands</div>
              {slashCommands.map((c, ci) => (
                <button
                  key={c.name}
                  onClick={() => pickSlashCommand(c.name)}
                  onMouseEnter={() => setSlashHighlight(ci)}
                  className={cn(
                    "w-full flex items-center gap-2 text-left px-2 py-1.5 rounded cursor-pointer",
                    ci === slashHighlight
                      ? "bg-[var(--bg-surface-active)] text-[var(--fg-on-active)]"
                      : "text-[var(--fg-secondary)] hover:bg-[var(--bg-panel)]"
                  )}
                >
                  <span className={cn("shrink-0", ci === slashHighlight ? "" : "text-[var(--accent-primary)]")}>{c.name}</span>
                  <span className="text-[10px] opacity-60 truncate">{c.description}</span>
                </button>
              ))}
            </div>
          )}

          {/* File mentions autocomplete menu */}
          {showMentionMenu && mentionResults.length > 0 && (
            <div className="absolute bottom-full mb-2 left-3 right-3 z-50 bg-[var(--bg-sidebar)] border border-[var(--border-default)] shadow-2xl max-h-36 overflow-y-auto p-1 space-y-0.5 rounded-xl font-mono text-xs">
              <div className="text-[9px] uppercase tracking-wider text-[var(--fg-tertiary)] font-bold px-2 py-0.5">Workspace Files</div>
              {mentionResults.map((p) => (
                <button
                  key={p}
                  onClick={() => pickMention(p)}
                  className="w-full text-left px-2 py-1.5 hover:bg-[var(--bg-panel)] rounded text-[var(--fg-secondary)] hover:text-[var(--fg-primary)] text-[11px] font-mono truncate cursor-pointer"
                >
                  {p}
                </button>
              ))}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
