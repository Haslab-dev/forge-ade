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
  IconCheck,
  IconCpu,
  IconShield,
  IconFile,
  IconAlertTriangle,
  IconPaperclip,
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
  GetExternalAgentState,
  SetExternalAgentConfig,
  type ExternalAgentState,
  type ExternalConfigOption,
  type AgentMessage,
  type ContentBlock,
  type AttachmentPayload,
  type SlashCommand,
} from "../lib/native";
import { AgentChatBody, AskCard } from "./agent-chat";
interface AttachedFileItem {
  id: string;
  name: string;
  type: "image" | "file";
  mimeType: string;
  data: string; // base64 string
  size: number;
  previewUrl?: string;
}
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

function ExtConfigPill({
  opt,
  open,
  onToggle,
  onSelect,
}: {
  opt: ExternalConfigOption;
  open: boolean;
  onToggle: () => void;
  onSelect: (value: string | boolean) => void;
}) {
  const [search, setSearch] = useState("");
  const ref = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (!open) return;
    const onDown = (e: MouseEvent) => {
      if (ref.current && !ref.current.contains(e.target as Node)) onToggle();
    };
    window.addEventListener("mousedown", onDown);
    return () => window.removeEventListener("mousedown", onDown);
  }, [open, onToggle]);

  if (opt.type === "boolean") {
    const on = Boolean(opt.currentValue);
    return (
      <button
        onClick={() => onSelect(!on)}
        title={opt.description ?? opt.name}
        className={cn(
          "px-2 py-1 bg-[var(--bg-app)] hover:bg-[var(--bg-surface-hover)] border rounded-full cursor-pointer font-mono text-[10px] transition-colors shrink-0",
          on
            ? "border-emerald-500/50 text-emerald-400"
            : "border-[var(--border-default)] text-[var(--fg-secondary)]",
        )}
      >
        {opt.name}: {on ? "on" : "off"}
      </button>
    );
  }

  const options = opt.options ?? [];
  const current = options.find((o) => o.value === String(opt.currentValue));
  const q = search.trim().toLowerCase();
  const filtered = q
    ? options.filter((o) => o.name.toLowerCase().includes(q) || o.value.toLowerCase().includes(q))
    : options;

  return (
    <div className="relative min-w-0" ref={ref}>
      <button
        onClick={onToggle}
        className="flex items-center gap-1.5 px-2 py-1 bg-[var(--bg-app)] hover:bg-[var(--bg-surface-hover)] border border-[var(--border-default)] text-[var(--fg-secondary)] hover:text-[var(--fg-primary)] rounded-full cursor-pointer font-mono text-[10px] transition-colors max-w-44"
        title={opt.description ? `${opt.name} — ${opt.description}` : opt.name}
      >
        <span className="truncate">{current?.name ?? String(opt.currentValue ?? opt.name)}</span>
        {options.length > 0 && <IconChevronDown className="size-2.5 opacity-60 shrink-0" />}
      </button>

      {open && options.length > 0 && (
        <div className="absolute bottom-full left-0 mb-2 z-50 w-72 bg-[var(--bg-sidebar)] border border-[var(--border-default)] shadow-2xl p-2 rounded text-xs font-sans max-h-80 flex flex-col">
          {options.length > 8 && (
            <div className="relative mb-2">
              <IconSearch className="size-3 absolute left-2 top-2 text-[var(--fg-tertiary)]" />
              <input
                autoFocus
                value={search}
                onChange={(e) => setSearch(e.target.value)}
                placeholder={`Search ${opt.name.toLowerCase()}...`}
                className="w-full bg-[var(--bg-panel)] border border-[var(--border-default)] pl-7 pr-2 py-1 text-[11px] text-[var(--fg-primary)] rounded focus:outline-none focus:border-[var(--accent-primary)] font-mono"
              />
            </div>
          )}
          <div className="overflow-y-auto flex-1">
            {filtered.map((o) => {
              const isActive = o.value === String(opt.currentValue);
              return (
                <button
                  key={o.value}
                  onClick={() => onSelect(o.value)}
                  className={cn(
                    "w-full text-left px-2 py-1 hover:bg-[var(--bg-panel)] rounded text-[var(--fg-primary)] truncate font-mono text-[11px] flex items-center justify-between cursor-pointer",
                    isActive && "bg-[var(--bg-surface-active)] text-[var(--fg-on-active)]"
                  )}
                  title={o.description ?? o.name}
                >
                  <span className="truncate">{o.name}</span>
                  {isActive && <IconCheck className="size-3 text-emerald-400 shrink-0 ml-1" />}
                </button>
              );
            })}
            {filtered.length === 0 && (
              <div className="px-2 py-1.5 text-[10px] text-[var(--fg-tertiary)]">No matches</div>
            )}
          </div>
        </div>
      )}
    </div>
  );
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
  const [attachedFiles, setAttachedFiles] = useState<AttachedFileItem[]>([]);
  const [isDraggingOver, setIsDraggingOver] = useState(false);
  const [showClearConfirm, setShowClearConfirm] = useState(false);
  const modelPickerRef = useRef<HTMLDivElement>(null);
  const agentPickerRef = useRef<HTMLDivElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const fileInputRef = useRef<HTMLInputElement>(null);
/**
 * Pill + dropdown for one ACP session config option (model, mode, thinking
 * level, ...). Booleans toggle inline; selects open a searchable list.
 */

  const mentionTimerRef = useRef<number | undefined>(undefined);
  const slashTimerRef = useRef<number | undefined>(undefined);
  // External ACP sessions: config options (model/mode/thinking) + commands.
  const isExternalSession = String(meta.role ?? session.role ?? "").startsWith("external:");
  const [extState, setExtState] = useState<ExternalAgentState>({ configOptions: [], availableCommands: [] });
  const [openExtPicker, setOpenExtPicker] = useState<string | null>(null);
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

    const liveBuffer = { text: "", thinking: "", toolDeltas: new Map<string, string>() };
    let rafHandle: number | null = null;

    const flushLiveDeltas = () => {
      rafHandle = null;
      if (!liveBuffer.text && !liveBuffer.thinking && liveBuffer.toolDeltas.size === 0) return;
      const textChunk = liveBuffer.text;
      const thinkingChunk = liveBuffer.thinking;
      const toolMap = new Map(liveBuffer.toolDeltas);
      liveBuffer.text = "";
      liveBuffer.thinking = "";
      liveBuffer.toolDeltas.clear();

      setLiveCurrent((cur) => {
        const msg: AgentMessage = cur ?? {
          id: `live-${Date.now()}`,
          role: "assistant",
          content: [],
          timestamp: new Date().toISOString(),
          state: "running",
        };
        const content = [...msg.content];
        if (textChunk) {
          const idx = content.map((b) => b.type).lastIndexOf("text");
          if (idx >= 0) {
            content[idx] = { ...content[idx], text: (content[idx].text ?? "") + textChunk };
          } else {
            content.push({ type: "text", text: textChunk });
          }
        }
        if (thinkingChunk) {
          const idx = content.map((b) => b.type).lastIndexOf("thinking");
          if (idx >= 0) {
            content[idx] = { ...content[idx], text: (content[idx].text ?? "") + thinkingChunk };
          } else {
            content.push({ type: "thinking", text: thinkingChunk });
          }
        }
        for (const [tcId, chunk] of toolMap.entries()) {
          const idx = content.findIndex((b) => b.type === "tool_call" && b.tool_call_id === tcId);
          if (idx >= 0) {
            content[idx] = { ...content[idx], arguments: (content[idx].arguments ?? "") + chunk };
          }
        }
        return { ...msg, content };
      });
    };

    const scheduleDeltaFlush = () => {
      if (rafHandle === null) {
        rafHandle = requestAnimationFrame(flushLiveDeltas);
      }
    };

    const unsubs = [
      EventsOn("agent:turn_start", (payload: Record<string, unknown>) => {
        if (!mine(payload)) return;
        if (rafHandle !== null) {
          cancelAnimationFrame(rafHandle);
          rafHandle = null;
        }
        liveBuffer.text = "";
        liveBuffer.thinking = "";
        liveBuffer.toolDeltas.clear();
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
        const delta = String(payload.delta ?? "");
        if (!delta) return;
        if (payload.kind === "thinking") {
          liveBuffer.thinking += delta;
        } else {
          liveBuffer.text += delta;
        }
        scheduleDeltaFlush();
      }),
      EventsOn("agent:thinking_delta", (payload: Record<string, unknown>) => {
        if (!mine(payload)) return;
        // Ignored to avoid duplicate text (covered by agent:message_delta)
      }),
      EventsOn("agent:message_end", (payload: Record<string, unknown>) => {
        if (!mine(payload)) return;
        flushLiveDeltas();
        const finalMsg = payload.message as AgentMessage | undefined;
        if (finalMsg) {
          setLiveFinalized((prev) => [...prev, finalMsg]);
          setLiveCurrent(null);
        }
      }),
      EventsOn("agent:tool_start", (payload: Record<string, unknown>) => {
        if (!mine(payload)) return;
        flushLiveDeltas();
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
          const existing = msg.content.find((b) => b.type === "tool_call" && b.tool_call_id === toolCallId);
          if (existing) {
            return {
              ...msg,
              content: msg.content.map((b) =>
                b.type === "tool_call" && b.tool_call_id === toolCallId
                  ? { ...b, name: String(payload.name ?? b.name), arguments: argText || b.arguments }
                  : b
              ),
            };
          }
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
        if (!chunk || !toolCallId) return;
        liveBuffer.toolDeltas.set(
          toolCallId,
          (liveBuffer.toolDeltas.get(toolCallId) ?? "") + chunk
        );
        scheduleDeltaFlush();
      }),
      EventsOn("agent:tool_end", (payload: Record<string, unknown>) => {
        if (!mine(payload)) return;
        flushLiveDeltas();
        const index = Number(payload.index ?? 0);
        const toolCallId = toolIndexRef.current.get(index)
          ?? String(payload.toolCallId ?? payload.tool_call_id ?? "");
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
        flushLiveDeltas();
        setRunning(false);
        setPendingTools(null);
        setPendingAsk(null);
        scheduleListRefresh();
        GetAgentSession(sessionId).then((full) => {
          if (!full) return;
          setMeta((prev) => ({ ...prev, ...full }));
          setBaseMessages(Array.isArray(full.messages) ? full.messages : []);
          setLiveFinalized([]);
          setLiveCurrent(null);
          setOptimisticUser(null);
        }).catch(() => {});
      }),
      EventsOn("agent:updated", (payload: Record<string, unknown>) => {
        scheduleListRefresh();
        if (payload?.id === sessionId || payload?.id == null) {
          GetAgentSession(sessionId).then((fresh) => {
            if (fresh) {
              setMeta((prev) => ({ ...prev, ...fresh }));
              if (Array.isArray(fresh.messages) && fresh.messages.length === 0) {
                setBaseMessages([]);
                setLiveFinalized([]);
                setLiveCurrent(null);
                setOptimisticUser(null);
              }
            }
          }).catch(() => {});
        }
      }),
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
      if (rafHandle !== null) cancelAnimationFrame(rafHandle);
      clearTimeout(mentionTimerRef.current);
      clearTimeout(listRefreshTimerRef.current);
      unsubs.forEach((u) => typeof u === "function" && u());
    };
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

  // Load + live-refresh the external agent's ACP state (config options and
  // slash commands arrive asynchronously after the adapter connects).
  useEffect(() => {
    if (!isExternalSession) return;
    let cancelled = false;
    GetExternalAgentState(sessionId)
      .then((st) => {
        if (!cancelled) setExtState(st);
      })
      .catch(() => {});
    const unsub = EventsOn("agent:external_state", (payload: Record<string, unknown>) => {
      if (payload?.id !== sessionId) return;
      setExtState({
        configOptions: Array.isArray(payload.configOptions) ? (payload.configOptions as ExternalConfigOption[]) : [],
        availableCommands: Array.isArray(payload.availableCommands)
          ? (payload.availableCommands as ExternalAgentState["availableCommands"])
          : [],
      });
    });
    return () => {
      cancelled = true;
      if (typeof unsub === "function") unsub();
    };
  }, [sessionId, isExternalSession]);

  async function handleSelectExtConfig(configId: string, value: string | boolean) {
    setOpenExtPicker(null);
    try {
      setExtState(await SetExternalAgentConfig(sessionId, configId, value));
    } catch (err) {
      console.error("Failed to set external agent config:", err);
    }
  }



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

  const handleClearTranscript = useCallback(async () => {
    try {
      setBaseMessages([]);
      setLiveFinalized([]);
      setLiveCurrent(null);
      setOptimisticUser(null);
      setPendingTools(null);
      setPendingAsk(null);
      setErrorBanner(null);
      setMentionFiles([]);
      setAttachedFiles([]);
      setMeta((prev) => ({ ...prev, messages: [], token_usage: {}, messageCount: 0, lastMessagePreview: "" }));
      await ClearAgentSession(sessionId);
      const fresh = await GetAgentSession(sessionId);
      if (fresh) {
        setMeta((prev) => ({ ...prev, ...fresh }));
        setBaseMessages(Array.isArray(fresh.messages) ? fresh.messages : []);
      }
      scheduleListRefresh();
    } catch (err) {
      console.error("Failed to clear agent transcript:", err);
    }
  }, [sessionId, scheduleListRefresh]);

  async function processSelectedFiles(files: FileList | File[]) {
    const items: AttachedFileItem[] = [];
    for (let i = 0; i < files.length; i++) {
      const file = files[i];
      if (!file) continue;
      const isImg = file.type.startsWith("image/") || /\.(png|jpe?g|gif|webp|bmp|svg|ico)$/i.test(file.name);
      try {
        const reader = new FileReader();
        const base64 = await new Promise<string>((resolve, reject) => {
          reader.onload = () => {
            const res = String(reader.result ?? "");
            const b64 = res.includes(",") ? res.split(",")[1] : res;
            resolve(b64);
          };
          reader.onerror = reject;
          reader.readAsDataURL(file);
        });

        items.push({
          id: `att-${Date.now()}-${i}-${Math.random().toString(36).slice(2, 6)}`,
          name: file.name,
          type: isImg ? "image" : "file",
          mimeType: file.type || (isImg ? "image/png" : "text/plain"),
          data: base64,
          size: file.size,
          previewUrl: isImg ? URL.createObjectURL(file) : undefined,
        });
      } catch (err) {
        console.error("Failed to read file:", file.name, err);
      }
    }
    if (items.length > 0) {
      setAttachedFiles((prev) => [...prev, ...items]);
    }
  }

  function pushOptimisticUser(text: string, currentAttachments: AttachedFileItem[] = []) {
    const blocks: ContentBlock[] = [];
    if (text.trim()) {
      blocks.push({ type: "text", text });
    }
    for (const att of currentAttachments) {
      if (att.type === "image" && att.data) {
        blocks.push({
          type: "image",
          mime_type: att.mimeType,
          data: att.data,
          name: att.name,
        });
      }
    }
    if (blocks.length === 0) {
      blocks.push({ type: "text", text });
    }
    setOptimisticUser({
      id: `local-user-${Date.now()}`,
      role: "user",
      content: blocks,
      timestamp: new Date().toISOString(),
    });
  }

  async function handleSendMessage(customText?: string) {
    const text = customText ?? inputText;
    const currentAttachments = [...attachedFiles];
    if (!text.trim() && currentAttachments.length === 0) return;

    const attachmentsPayload: AttachmentPayload[] = currentAttachments.map((a) => ({
      name: a.name,
      type: a.type,
      mimeType: a.mimeType,
      data: a.data,
      size: a.size,
    }));

    if (isExternalSession) {
      setInputText("");
      setMentionFiles([]);
      setAttachedFiles([]);
      setShowMentionMenu(false);
      setShowSlashMenu(false);
      pushOptimisticUser(text, currentAttachments);
      setRunning(true);
      try {
        await SendAgentMessage(sessionId, text, mentionFiles, attachmentsPayload);
      } catch (err) {
        setRunning(false);
        console.error(err);
        setErrorBanner(err instanceof Error ? err.message : String(err));
      }
      return;
    }

    const firstToken = text.trim().split(/\s+/)[0] ?? "";
    const firstTokenLower = firstToken.toLowerCase();
    if (firstToken.startsWith("/") && !["/commit", "/clear", "/model"].includes(firstTokenLower)) {
      let known: SlashCommand[] = [];
      try {
        known = await ListSlashCommands();
      } catch { /* bridge unavailable */ }
      if (known.some((c) => c.name.toLowerCase() === firstTokenLower)) {
        setInputText("");
        setMentionFiles([]);
        setAttachedFiles([]);
        setShowMentionMenu(false);
        pushOptimisticUser(text, currentAttachments);
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
      setAttachedFiles([]);
      pushOptimisticUser(text, currentAttachments);
      try {
        const commitMsg = await GenerateAICommitMessage("", activeProviderId, activeModel);
        await SendAgentMessage(sessionId, `/commit\n\nProposed commit message:\n\`${commitMsg}\``, [], attachmentsPayload);
      } catch {
        await SendAgentMessage(sessionId, text, [], attachmentsPayload);
      }
      return;
    }

    if (text.startsWith("/clear")) {
      setInputText("");
      handleClearTranscript();
      return;
    }

    if (text.startsWith("/model")) {
      setInputText("");
      setShowModelPicker(true);
      return;
    }

    setInputText("");
    setMentionFiles([]);
    setAttachedFiles([]);
    setShowMentionMenu(false);
    pushOptimisticUser(text, currentAttachments);
    setRunning(true);
    try {
      await SendAgentMessage(sessionId, text, mentionFiles, attachmentsPayload);
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
          const cmds: SlashCommand[] = isExternalSession
            ? extState.availableCommands
                .filter((c) => ("/" + c.name.toLowerCase()).startsWith(firstToken.toLowerCase()))
                .map((c) => ({ name: "/" + c.name, description: c.description ?? "", kind: "command" }))
            : await ListSlashCommands(firstToken);
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

        </div>

        {/* Right Tools: Token Usage, Clear, Close */}
        <div className="flex items-center space-x-1.5">
          {/* Token usage badge */}
          {(Object.keys(meta.token_usage ?? {}).length > 0 || Object.keys(session.token_usage ?? {}).length > 0) && (
            <TokenUsageBadge usage={meta.token_usage ?? session.token_usage} />
          )}

          {/* Clear Button */}
          <button
            onClick={() => setShowClearConfirm(true)}
            className="p-1 text-[var(--fg-secondary)] hover:text-white hover:bg-[var(--bg-surface-hover)] border border-[var(--border-default)] rounded cursor-pointer transition-colors"
            title="Clear transcript (keeps session active)"
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
          <div
            onDragOver={(e) => {
              e.preventDefault();
              setIsDraggingOver(true);
            }}
            onDragLeave={() => setIsDraggingOver(false)}
            onDrop={(e) => {
              e.preventDefault();
              setIsDraggingOver(false);
              if (e.dataTransfer.files && e.dataTransfer.files.length > 0) {
                processSelectedFiles(e.dataTransfer.files);
              }
            }}
            className={cn(
              "rounded-2xl border bg-[var(--bg-panel)] shadow-lg transition-colors",
              isDraggingOver
                ? "border-[var(--accent-primary)] ring-2 ring-[var(--accent-primary)]/30 bg-[var(--accent-primary)]/5"
                : "border-[var(--border-default)] focus-within:border-[var(--accent-primary)]"
            )}
          >
            {/* Attachment preview chips & @-file mentions */}
            {(mentionFiles.length > 0 || attachedFiles.length > 0) && (
              <div className="flex flex-wrap gap-1.5 px-2.5 pt-2.5">
                {/* @-file mention chips */}
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

                {/* Uploaded / attached files and images */}
                {attachedFiles.map((att) => (
                  <span
                    key={att.id}
                    className={cn(
                      "flex items-center gap-1.5 pl-1.5 pr-0.5 py-0.5 rounded-lg text-[10px] font-mono border max-w-64 select-none",
                      att.type === "image"
                        ? "bg-purple-500/10 border-purple-500/30 text-purple-300"
                        : "bg-sky-500/10 border-sky-500/30 text-sky-300"
                    )}
                    title={`${att.name} (${(att.size / 1024).toFixed(1)} KB)`}
                  >
                    {att.type === "image" && att.previewUrl ? (
                      <img src={att.previewUrl} alt={att.name} className="size-4 object-cover rounded shrink-0" />
                    ) : (
                      <IconFile className="size-3 shrink-0" />
                    )}
                    <span className="truncate">{att.name}</span>
                    <span className="text-[8.5px] opacity-60 shrink-0">
                      {att.size < 1024 ? `${att.size}B` : `${Math.round(att.size / 1024)}K`}
                    </span>
                    <button
                      onClick={() => setAttachedFiles((prev) => prev.filter((x) => x.id !== att.id))}
                      className="p-0.5 rounded-full hover:bg-black/30 cursor-pointer shrink-0"
                      title={`Remove ${att.name}`}
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
              onPaste={(e) => {
                if (e.clipboardData.files && e.clipboardData.files.length > 0) {
                  e.preventDefault();
                  processSelectedFiles(e.clipboardData.files);
                }
              }}
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
              {!isExternalSession && (
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
                              const isActive = activeProviderId === pid && activeModel === m;
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
              )}

              {/* External ACP config options: mode / model / thinking / … */}
              {isExternalSession &&
                extState.configOptions.map((opt) => (
                  <ExtConfigPill
                    key={opt.id}
                    opt={opt}
                    open={openExtPicker === opt.id}
                    onToggle={() => setOpenExtPicker(openExtPicker === opt.id ? null : opt.id)}
                    onSelect={(v) => handleSelectExtConfig(opt.id, v)}
                  />
                ))}

            {!isExternalSession && (
              <>
            <div className="relative" ref={agentPickerRef}>
              <button
                onClick={() => setShowAgentPicker(!showAgentPicker)}
                className="px-1.5 py-0.5 bg-[var(--accent)]/15 border border-[var(--accent)]/40 text-[var(--accent)] hover:bg-[var(--accent)]/25 rounded text-[10px] uppercase tracking-wide flex items-center gap-1 cursor-pointer transition-colors"
              >
                <span>{String(meta.role ?? session.role ?? "coding")}</span>
                <IconChevronDown className="size-2.5 opacity-70" />
              </button>
              {showAgentPicker && (
                <div className="absolute bottom-full left-0 mb-1 z-40 w-56 bg-[var(--bg-sidebar)] border border-[var(--border-default)] shadow-2xl p-1 rounded font-sans text-xs max-h-60 overflow-y-auto">
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
              </>
            )}

            {/* Attach Files / Images Button */}
            <button
              type="button"
              onClick={() => fileInputRef.current?.click()}
              className="flex items-center gap-1 px-2 py-1 bg-[var(--bg-app)] hover:bg-[var(--bg-surface-hover)] border border-[var(--border-default)] text-[var(--fg-secondary)] hover:text-[var(--fg-primary)] rounded-full cursor-pointer font-mono text-[10px] transition-colors"
              title="Attach files or images (multiple supported)"
            >
              <IconPaperclip className="size-3 text-cyan-400 shrink-0" />
              <span>Attach</span>
            </button>

            <input
              ref={fileInputRef}
              type="file"
              multiple
              accept="image/*,.ts,.tsx,.js,.jsx,.json,.py,.go,.rs,.zig,.md,.txt,.html,.css,.sql,.sh,.yaml,.yml,.toml,.csv"
              className="hidden"
              onChange={(e) => {
                if (e.target.files && e.target.files.length > 0) {
                  processSelectedFiles(e.target.files);
                  e.target.value = "";
                }
              }}
            />

              {/* Context window usage chip */}
              {meta.lastUsage && meta.contextWindow ? (
                <span className="text-[10px] font-mono text-[var(--fg-tertiary)] whitespace-nowrap">
                  ctx: {((meta.lastUsage.promptTokens / meta.contextWindow) * 100).toFixed(1)}%/{formatTokenCount(meta.contextWindow)}
                </span>
              ) : null}

              <div className="flex-1" />
              {!isExternalSession && (
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
              )}

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
                  disabled={!inputText.trim() && attachedFiles.length === 0}
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

      {/* Clear Transcript Confirmation Modal (In-App Modal for Desktop & Web) */}
      {showClearConfirm && (
        <div
          className="fixed inset-0 bg-black/60 backdrop-blur-xs z-[9999] flex items-center justify-center p-4 select-none font-sans"
          onClick={() => setShowClearConfirm(false)}
        >
          <div
            className="bg-[var(--bg-elevated)] border border-[var(--border-default)] w-full max-w-sm rounded-xl shadow-2xl p-4 flex flex-col gap-3.5 animate-in fade-in zoom-in-95 duration-150"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="flex items-start gap-3">
              <div className="p-2 rounded-full bg-red-500/10 border border-red-500/30 text-red-400 shrink-0">
                <IconTrashX className="size-5" />
              </div>
              <div className="space-y-1 min-w-0 flex-1">
                <h3 className="text-sm font-semibold text-[var(--fg-primary)]">Clear Conversation History?</h3>
                <p className="text-xs text-[var(--fg-secondary)] leading-relaxed">
                  This will permanently remove all messages and tool outputs from this session. Your session settings and model role will be preserved.
                </p>
              </div>
            </div>

            <div className="flex items-center justify-end gap-2 pt-1 border-t border-[var(--border-default)]/60">
              <button
                type="button"
                onClick={() => setShowClearConfirm(false)}
                className="px-3 py-1.5 rounded text-xs text-[var(--fg-secondary)] hover:text-[var(--fg-primary)] hover:bg-[var(--bg-surface-hover)] transition-colors cursor-pointer"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={() => {
                  setShowClearConfirm(false);
                  handleClearTranscript();
                }}
                className="px-3.5 py-1.5 bg-red-600 hover:bg-red-500 text-white rounded text-xs font-semibold shadow-sm transition-colors cursor-pointer flex items-center gap-1.5"
              >
                <IconTrashX className="size-3.5" />
                <span>Clear History</span>
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
