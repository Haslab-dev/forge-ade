import React, { useCallback, useEffect, useMemo, useRef, useState } from "react";
import {
  IconBrain,
  IconChevronDown,
  IconChevronUp,
  IconChevronRight,
  IconCopy,
  IconShield,
  IconCheck,
  IconX,
  IconTerminal2,
  IconSparkles,
  IconPlayerPlay,
  IconArrowDown,
} from "@tabler/icons-react";
import { marked } from "marked";
import { RespondAgentAsk } from "../lib/native";
import { globalOpenFile } from "../panels/editor";
import { cn } from "../lib/utils";

// ---------------------------------------------------------------------------
// Markdown Parser & Cache
// ---------------------------------------------------------------------------
const MAX_MARKDOWN_CACHE = 1000;
const markdownCache = new Map<string, string>();
function renderMarkdown(src: string): string {
  const cached = markdownCache.get(src);
  if (cached !== undefined) return cached;
  let html: string;
  try {
    html = marked.parse(src, { async: false }) as string;
  } catch {
    html = src;
  }
  if (markdownCache.size > MAX_MARKDOWN_CACHE) {
    markdownCache.clear();
  }
  markdownCache.set(src, html);
  return html;
}

function CopyButton({ text, className = "" }: { text: string; className?: string }) {
  const [copied, setCopied] = useState(false);
  const handleCopy = (e: React.MouseEvent) => {
    e.stopPropagation();
    navigator.clipboard.writeText(text).then(() => {
      setCopied(true);
      setTimeout(() => setCopied(false), 2000);
    }).catch(() => {});
  };
  return (
    <button
      onClick={handleCopy}
      className={cn(
        "p-1 rounded hover:bg-[var(--bg-surface-hover)] text-[var(--fg-tertiary)] hover:text-white transition-colors cursor-pointer",
        className
      )}
      title="Copy content"
    >
      {copied ? <IconCheck className="size-3.5 text-emerald-400" /> : <IconCopy className="size-3.5" />}
    </button>
  );
}

function fmtBlockDuration(ms: number): string {
  if (!ms || ms <= 0) return "";
  const totalSec = Math.floor(ms / 1000);
  const h = Math.floor(totalSec / 3600);
  const m = Math.floor((totalSec % 3600) / 60);
  const s = totalSec % 60;
  if (h > 0) return `${h}h ${m}m ${s}s`;
  if (m > 0) return `${m}m ${s}s`;
  return `${s}s`;
}

function itemDuration(item: any, running: boolean, fallbackEnd?: number): number {
  const start = item?.startTs;
  if (!start) return 0;
  let end = item?.endTs;
  if (!end) end = running ? Date.now() : fallbackEnd || start;
  return Math.max(0, end - start);
}

// ---------------------------------------------------------------------------
// Turn & Message Model
// ---------------------------------------------------------------------------
export interface ToolCallView {
  id: string;
  name: string;
  arguments: any;
  result: string;
  is_error?: boolean;
}

export type TurnItem =
  | { kind: "text"; text: string; startTs?: number }
  | { kind: "thinking"; text: string; open?: boolean; startTs?: number }
  | { kind: "tool"; tool: ToolCallView; startTs?: number }
  | { kind: "notice"; text: string; startTs?: number }
  | {
      kind: "usage";
      at: number;
      promptTokens: number;
      completionTokens: number;
      cachedTokens: number;
      durationMs: number;
      /** Kept for uniform item handling; usage items render no timer. */
      startTs?: number | undefined;
    };

export interface TurnImage {
  mimeType: string;
  data?: string;
  url?: string;
  name?: string;
}

export interface Turn {
  prompt: string;
  timestamp?: string;
  items: TurnItem[];
  images?: TurnImage[];
}

function blockText(b: any): string {
  return b?.text ?? "";
}

function msgTs(msg: any): number {
  const t = msg?.timestamp ?? msg?.Timestamp;
  if (!t) return 0;
  const d = new Date(t).getTime();
  return isNaN(d) ? 0 : d;
}

function buildTurns(messages: any[]): Turn[] {
  const turns: Turn[] = [];
  let current: Turn | null = null;
  let lastTs = 0;

  const flush = () => {
    if (current && (current.prompt || current.items.length > 0)) turns.push(current);
    current = null;
  };

  const closeOpenItems = (ts: number) => {
    if (!current || !ts) return;
    for (const it of current.items) {
      if (it.startTs && !(it as any).endTs) (it as any).endTs = ts;
    }
  };

  for (const msg of messages || []) {
    const role = msg.role;
    const blocks = Array.isArray(msg.content) ? msg.content : [];
    const ts = msgTs(msg);
    if (ts) lastTs = ts;

    if (role === "user") {
      flush();
      const textBlocks = blocks.filter((b: any) => b.type === "text" || !b.type);
      const prompt =
        textBlocks.map((b: any) => blockText(b)).join("\n") ||
        (typeof msg.content === "string" ? msg.content : "");
      const images: TurnImage[] = blocks
        .filter((b: any) => b.type === "image")
        .map((b: any) => ({
          mimeType: b.mime_type || "image/png",
          data: b.data,
          url: b.url,
          name: b.name,
        }));
      current = { prompt, timestamp: msg.timestamp, items: [], images: images.length > 0 ? images : undefined };
      continue;
    }
    // Local command output arrives as a synthetic system message — render it
    // as a standalone dimmed notice between turns.
    if (role === "system") {
      flush();
      const notice = blocks.map(blockText).filter(Boolean).join("\n");
      if (notice) turns.push({ prompt: "", items: [{ kind: "notice", text: notice }] });
      continue;
    }
    if (!current) current = { prompt: "", timestamp: msg.timestamp, items: [] };

    if (role === "assistant") {
      for (const b of blocks) {
        const t = b.type;
        if (t === "text" && blockText(b)) {
          const last = current.items[current.items.length - 1];
          if (last?.kind === "text") {
            (last as any).text += blockText(b);
          } else {
            if (last && last.startTs && !(last as any).endTs && ts) {
              (last as any).endTs = ts;
            }
            current.items.push({ kind: "text", text: blockText(b), startTs: ts || undefined });
          }
        } else if (t === "thinking" && blockText(b)) {
          const last = current.items[current.items.length - 1];
          if (last?.kind === "thinking") {
            (last as any).text += blockText(b);
          } else {
            if (last && last.startTs && !(last as any).endTs && ts) {
              (last as any).endTs = ts;
            }
            current.items.push({ kind: "thinking", text: blockText(b), startTs: ts || undefined });
          }
        } else if (t === "tool_call") {
          const last = current.items[current.items.length - 1];
          if (last && last.startTs && !(last as any).endTs && ts) {
            (last as any).endTs = ts;
          }
          // Contract §3: arguments arrive as a raw JSON string streamed
          // incrementally; parse when possible so titles/expanders work live.
          let parsedArgs: unknown = b.arguments ?? {};
          if (typeof parsedArgs === "string") {
            try { parsedArgs = JSON.parse(parsedArgs); } catch { /* partial JSON mid-stream */ }
          }
          const tc: ToolCallView = {
            id: b.tool_call_id ?? "",
            name: b.name ?? "tool",
            arguments: parsedArgs,
            result: "",
          };
          const existing = current.items.find(
            (it) => it.kind === "tool" && it.tool.id && it.tool.id === tc.id
          );
          if (!existing) current.items.push({ kind: "tool", tool: tc, startTs: ts || undefined });
        }
      }
      // Per-response usage footer (oh-my-pi style) — attached by the engine
      // when the LLM call for this message completes.
      const u = msg.usage;
      if (u && typeof u.promptTokens === "number" && typeof u.completionTokens === "number") {
        current.items.push({
          kind: "usage",
          at: typeof u.at === "number" ? u.at : Date.parse(msg.timestamp ?? "") || 0,
          promptTokens: u.promptTokens,
          completionTokens: u.completionTokens,
          cachedTokens: typeof u.cachedTokens === "number" ? u.cachedTokens : 0,
          durationMs: typeof u.durationMs === "number" ? u.durationMs : 0,
        });
      }
    } else if (role === "tool") {
      const last = current.items[current.items.length - 1];
      if (last && last.startTs && !(last as any).endTs && ts) {
        (last as any).endTs = ts;
      }
      for (const b of blocks) {
        if (b.type !== "tool_result") continue;
        const resultText = blockText(b);
        const isErr = !!b.is_error;
        let target: ToolCallView | null = null;
        const byId = current.items.find(
          (it): it is { kind: "tool"; tool: ToolCallView } =>
            it.kind === "tool" && !!it.tool.id && it.tool.id === (b.tool_call_id ?? "") && !it.tool.result
        );
        let toolItem: { kind: "tool"; tool: ToolCallView } | undefined = byId;
        if (byId) target = byId.tool;
        else {
          toolItem = [...current.items]
            .reverse()
            .find((it): it is { kind: "tool"; tool: ToolCallView } => it.kind === "tool" && !it.tool.result);
          if (toolItem) target = toolItem.tool;
        }
        if (target) {
          target.result = resultText;
          target.is_error = isErr;
        } else if (resultText) {
          current.items.push({
            kind: "tool",
            tool: { id: b.tool_call_id ?? "", name: "tool", arguments: {}, result: resultText, is_error: isErr },
            startTs: ts || undefined,
          });
        }
        if (toolItem && ts) (toolItem as any).endTs = ts;
      }
    }
  }

  if (current && lastTs) closeOpenItems(lastTs);
  flush();
  return turns;
}

// ---------------------------------------------------------------------------
// Tool Badges & Formatting
// ---------------------------------------------------------------------------
const TOOL_BADGE: Record<string, { label: string; cls: string }> = {
  write: { label: "WRITE", cls: "text-emerald-400 border-emerald-500/40 bg-emerald-500/10" },
  write_file: { label: "WRITE", cls: "text-emerald-400 border-emerald-500/40 bg-emerald-500/10" },
  edit: { label: "EDIT", cls: "text-amber-400 border-amber-500/40 bg-amber-500/10" },
  edit_file: { label: "EDIT", cls: "text-amber-400 border-amber-500/40 bg-amber-500/10" },
  read: { label: "READ", cls: "text-sky-400 border-sky-500/40 bg-sky-500/10" },
  read_file: { label: "READ", cls: "text-sky-400 border-sky-500/40 bg-sky-500/10" },
  list_dir: { label: "LS", cls: "text-sky-400 border-sky-500/40 bg-sky-500/10" },
  search_files: { label: "GLOB", cls: "text-purple-400 border-purple-500/40 bg-purple-500/10" },
  grep_code: { label: "GREP", cls: "text-fuchsia-400 border-fuchsia-500/40 bg-fuchsia-500/10" },
  bash: { label: "EXEC", cls: "text-orange-400 border-orange-500/40 bg-orange-500/10" },
  exec: { label: "EXEC", cls: "text-orange-400 border-orange-500/40 bg-orange-500/10" },
};

function toolBadge(name: string): { label: string; cls: string } {
  return (
    TOOL_BADGE[name] ?? {
      label: (name || "TOOL").toUpperCase().slice(0, 8),
      cls: "text-[var(--fg-secondary)] border-[var(--border-default)] bg-[var(--bg-surface)]",
    }
  );
}

function toolTitle(tc: ToolCallView): string {
  const args = tc.arguments;
  if (args && typeof args === "object") {
    if (args.path) return String(args.path);
    if (args.command) return String(args.command);
    if (args.pattern) return String(args.pattern);
    if (args.query) return String(args.query);
  }
  return "";
}

const MAX_TOOL_LINES = 200;

function renderToolLines(raw: string): { lines: string[]; total: number } {
  if (!raw) return { lines: [], total: 0 };
  let obj: any = null;
  try {
    obj = JSON.parse(raw);
  } catch {
    const l = raw.split("\n").filter((x) => x.trim());
    return { lines: l.slice(0, MAX_TOOL_LINES), total: l.length };
  }
  if (typeof obj !== "object" || obj === null) return { lines: [String(obj)], total: 1 };

  const lines: string[] = [];
  if (Array.isArray(obj.matches)) {
    for (const m of obj.matches.slice(0, 8)) {
      const p = m.path ?? "";
      const ln = m.line ?? 0;
      const c = m.content ?? "";
      lines.push(`${p}${ln ? ":" + ln : ""}  ${String(c).trim()}`);
    }
    return { lines, total: obj.matches.length };
  }
  if (Array.isArray(obj.entries)) {
    for (const e of obj.entries.slice(0, 10)) lines.push(e.isDir || e.is_dir ? `${e.name}/` : e.name);
    return { lines, total: obj.entries.length };
  }
  if (typeof obj.stdout === "string" && obj.stdout.trim()) {
    const l = obj.stdout.trim().split("\n");
    return { lines: l.slice(0, MAX_TOOL_LINES), total: l.length };
  }
  if (typeof obj.content === "string" && obj.content.trim()) {
    const l = obj.content.split("\n");
    return { lines: l.slice(0, MAX_TOOL_LINES), total: l.length };
  }
  if (typeof obj.path === "string") lines.push(obj.path);
  if (typeof obj.status === "string") lines.push(obj.status);
  if (typeof obj.replacements === "number") lines.push(`Edited ${obj.replacements} hunk(s)`);
  if (typeof obj.exit_code === "number") lines.push(`exited with code ${obj.exit_code}`);
  if (typeof obj.count === "number") lines.push(`found ${obj.count} match(es)`);
  if (lines.length === 0) return { lines: [], total: 0 };
  return { lines, total: lines.length };
}

// ---------------------------------------------------------------------------
// Terminal Tool Call Component (no bubble/card border)
// ---------------------------------------------------------------------------
const TerminalToolRow = React.memo(function TerminalToolRow({
  toolCall,
  expanded,
  onToggle,
  running,
  durationMs,
}: {
  toolCall: ToolCallView;
  expanded: boolean;
  onToggle: () => void;
  running?: boolean;
  durationMs?: number;
}) {
  const badge = toolBadge(toolCall.name);
  const title = toolTitle(toolCall);
  const { lines, total } = renderToolLines(toolCall.result);
  const hasResult = toolCall.result !== "" && toolCall.result != null;

  return (
    <div className="font-mono text-xs my-1 select-text">
      {/* Terminal Line Header */}
      <button
        onClick={onToggle}
        className="w-full flex items-center gap-2 py-0.5 text-left hover:bg-[var(--bg-surface-hover)] rounded px-1 -mx-1 transition-colors cursor-pointer group"
      >
        <span className="text-amber-400 font-bold shrink-0">⚡</span>
        <span className={`text-[10px] font-bold uppercase tracking-wider px-1 py-0.2 rounded border ${badge.cls}`}>
          {badge.label}
        </span>
        {title ? (
          <span className="text-[var(--fg-primary)] truncate flex-1 font-medium">{title}</span>
        ) : (
          <span className="text-[var(--fg-tertiary)] flex-1">{toolCall.name}</span>
        )}

        {running ? (
          <span className="text-cyan-400 text-[11px] font-mono shrink-0 animate-pulse flex items-center gap-1">
            <span className="animate-spin inline-block">⠋</span> running
          </span>
        ) : hasResult ? (
          <span className={`text-[11px] font-mono shrink-0 flex items-center gap-1 ${toolCall.is_error ? "text-red-400" : "text-emerald-400"}`}>
            {toolCall.is_error ? <><IconX className="size-3" /> error</> : <><IconCheck className="size-3" /> done</>}
          </span>
        ) : null}

        {durationMs != null && durationMs > 0 && (
          <span className="text-[10px] text-[var(--fg-tertiary)] shrink-0 font-mono">
            {fmtBlockDuration(durationMs)}
          </span>
        )}

        {expanded ? (
          <IconChevronDown className="size-3 text-[var(--fg-tertiary)] shrink-0" />
        ) : (
          <IconChevronRight className="size-3 text-[var(--fg-tertiary)] opacity-60 group-hover:opacity-100 shrink-0" />
        )}
      </button>

      {/* Terminal Output Stream */}
      {expanded && (
        <div className="ml-5 pl-2.5 my-1 border-l border-[var(--border-default)] font-mono text-[11px] text-[var(--fg-secondary)] max-h-60 overflow-y-auto pr-1">
          {lines.length === 0 && <div className="text-[var(--fg-tertiary)] italic">(no output)</div>}
          {lines.map((l, i) => (
            <div key={i} className="flex gap-2 leading-relaxed whitespace-pre-wrap break-all">
              <span className="text-[var(--fg-tertiary)] select-none opacity-50">│</span>
              <span>{l}</span>
            </div>
          ))}
          {total > lines.length && (
            <div className="text-[10px] text-[var(--fg-tertiary)] pt-1 italic">
              … and {total - lines.length} more lines
            </div>
          )}
        </div>
      )}
    </div>
  );
});

// ---------------------------------------------------------------------------
// Terminal Thinking Block (no card/bubble)
// ---------------------------------------------------------------------------
const TerminalThinkingBlock = React.memo(function TerminalThinkingBlock({
  text,
  open,
  onToggle,
  running,
  durationMs,
}: {
  text: string;
  open: boolean;
  onToggle: () => void;
  running?: boolean;
  durationMs?: number;
}) {
  const lines = text.split("\n");
  const firstLine = lines[0] || "Analyzing workspace context...";

  return (
    <div className="my-1.5 font-mono text-xs select-text">
      <button
        onClick={onToggle}
        className="flex items-center gap-1.5 text-[11px] text-[var(--fg-tertiary)] hover:text-[var(--fg-secondary)] py-0.5 cursor-pointer text-left transition-colors"
      >
        {running ? (
          <span className="text-sky-400 font-bold animate-spin shrink-0">✻</span>
        ) : (
          <span className="text-sky-400 font-bold shrink-0">{open ? "▼" : "▶"}</span>
        )}
        <span className="font-semibold text-sky-400/90">
          {running ? "Thinking" : "Thought process"}
        </span>
        {durationMs != null && durationMs > 0 && (
          <span className="text-[10px] text-[var(--fg-tertiary)] font-mono">
            ({fmtBlockDuration(durationMs)})
          </span>
        )}
        {!open && !running && (
          <span className="text-[10px] text-[var(--fg-tertiary)] truncate max-w-sm italic ml-1">
            — {firstLine}
          </span>
        )}
      </button>

      {(open || running) && (
        <div className="ml-3 pl-3 my-1 border-l-2 border-sky-500/30 font-mono text-[11px] leading-relaxed text-[var(--fg-secondary)] whitespace-pre-wrap max-h-72 overflow-y-auto select-text">
          {text}
        </div>
      )}
    </div>
  );
});

// ---------------------------------------------------------------------------
// Structured Ask Card
// ---------------------------------------------------------------------------
export function AskCard({ sessionId, questions }: { sessionId: string; questions: any[] }) {
  const [answers, setAnswers] = useState<Record<string, any>>({});
  const [submitted, setSubmitted] = useState(false);

  const submit = async () => {
    setSubmitted(true);
    try {
      await RespondAgentAsk(sessionId, answers);
    } catch (err) {
      console.error("Failed to submit ask answers:", err);
      setSubmitted(false);
    }
  };

  return (
    <div className="border border-[var(--border-default)] bg-[var(--bg-panel)] p-3 space-y-3 shadow-md max-w-md my-2 rounded">
      <div className="flex items-center space-x-2 text-xs font-semibold text-sky-400">
        <IconShield className="size-4" />
        <span>Agent requests your input</span>
      </div>
      {questions.map((q: any, qi: number) => (
        <div key={q.id || qi} className="space-y-1.5">
          <div className="text-xs font-medium text-[var(--fg-primary)]">{q.question}</div>
          <div className="flex flex-wrap gap-1.5">
            {(q.options || []).map((opt: string, oi: number) => {
              const selected = Array.isArray(answers[q.id]) ? answers[q.id].includes(opt) : answers[q.id] === opt;
              const toggle = () => {
                const key = q.id;
                if (q.multi) {
                  const cur: string[] = Array.isArray(answers[key]) ? answers[key] : [];
                  setAnswers((p) => ({
                    ...p,
                    [key]: cur.includes(opt) ? cur.filter((c) => c !== opt) : [...cur, opt],
                  }));
                } else {
                  setAnswers((p) => ({ ...p, [key]: opt }));
                }
              };
              return (
                <button
                  key={oi}
                  onClick={toggle}
                  className={`px-2 py-1 text-[11px] rounded border cursor-pointer ${
                    selected
                      ? "bg-sky-500/20 border-sky-500 text-sky-300"
                      : "bg-[var(--bg-surface)] border-[var(--border-default)] text-[var(--fg-secondary)] hover:bg-[var(--bg-surface-hover)]"
                  }`}
                >
                  {opt}
                  {!q.multi && oi === (q.recommended ?? 0) && (
                    <span className="ml-1 text-[9px] text-sky-400/80">(Recommended)</span>
                  )}
                </button>
              );
            })}
          </div>
        </div>
      ))}
      <div className="flex items-center justify-end pt-1">
        <button
          onClick={submit}
          disabled={submitted}
          className="px-3 py-1 text-xs font-semibold bg-sky-500 hover:bg-sky-600 text-black rounded cursor-pointer disabled:opacity-50"
        >
          <IconCheck className="size-3.5 inline mr-1" />
          <span>Submit answers</span>
        </button>
      </div>
    </div>
  );
}
const TextBlockView = React.memo(function TextBlockView({
  text,
  streaming,
  dur,
}: {
  text: string;
  streaming?: boolean;
  dur?: number;
}) {
  const html = useMemo(() => renderMarkdown(text), [text]);
  return (
    <div className="group/msg relative select-text font-sans">
      <div className="absolute top-0 right-0 opacity-0 group-hover/msg:opacity-100 transition-opacity z-10">
        <CopyButton text={text} className="bg-[var(--bg-panel)] border border-[var(--border-default)] shadow-xs" />
      </div>
      <div
        className="text-[13px] leading-[1.65] text-[var(--fg-primary)] markdown-body select-text"
        dangerouslySetInnerHTML={{
          __html: html + (streaming ? '<span class="animate-pulse text-[var(--accent-primary)]">▌</span>' : ""),
        }}
      />
      {dur && dur > 0 ? (
        <div className="text-[10px] font-mono text-[var(--fg-tertiary)] mt-1 select-none">
          done in {fmtBlockDuration(dur)}
        </div>
      ) : null}
    </div>
  );
});

const TurnView = React.memo(function TurnView({
  turn,
  ti,
  isLastTurn,
  running,
  fallbackEnd,
  expandedReasoning,
  expandedToolCalls,
  onToggleReasoning,
  onToggleToolCall,
}: {
  turn: Turn;
  ti: number;
  isLastTurn: boolean;
  running: boolean;
  fallbackEnd?: number;
  expandedReasoning: Record<string, boolean>;
  expandedToolCalls: Record<string, boolean>;
  onToggleReasoning: (key: string) => void;
  onToggleToolCall: (key: string) => void;
}) {
  return (
    <div
      className="space-y-2 border-b border-[var(--border-default)]/30 pb-4 last:border-0 last:pb-0"
      style={{ contentVisibility: isLastTurn ? "visible" : "auto", containIntrinsicSize: "0 80px" }}
    >
      {/* User Prompt */}
      {(turn.prompt || (turn.images && turn.images.length > 0)) && (
        <div className="group relative flex items-start gap-2 pt-1 font-mono text-sm leading-relaxed select-text">
          <span className="text-cyan-400 font-bold select-none text-base leading-tight">❯</span>
          <div className="flex-1 space-y-2 min-w-0">
            {turn.images && turn.images.length > 0 && (
              <div className="flex flex-wrap gap-2 pt-0.5">
                {turn.images.map((img, imgi) => {
                  const src = img.url || (img.data ? `data:${img.mimeType || "image/png"};base64,${img.data}` : "");
                  return (
                    <div
                      key={imgi}
                      className="relative rounded-lg overflow-hidden border border-[var(--border-default)] bg-[var(--bg-panel)] shadow-sm max-w-64 max-h-48 group/img"
                    >
                      <img
                        src={src}
                        alt={img.name || `Image ${imgi + 1}`}
                        className="object-contain max-h-40 w-auto rounded"
                      />
                      {img.name && (
                        <div className="absolute bottom-0 inset-x-0 bg-black/65 backdrop-blur-xs text-[9.5px] font-mono px-1.5 py-0.5 truncate text-white/90">
                          {img.name}
                        </div>
                      )}
                    </div>
                  );
                })}
              </div>
            )}
            {turn.prompt && (
              <div className="text-[var(--fg-primary)] font-medium whitespace-pre-wrap break-words">
                {turn.prompt}
              </div>
            )}
          </div>
          {turn.prompt && <CopyButton text={turn.prompt} className="opacity-0 group-hover:opacity-100 transition-opacity" />}
        </div>
      )}

      {/* Turn Items: Thinking, Tools, Prose Output */}
      <div className="space-y-2 pt-1 pl-4">
        {turn.items.map((item, ii) => {
          const isLast = ii === turn.items.length - 1;

          if (item.kind === "usage") {
            const pad = (v: number) => String(v).padStart(2, "0");
            const d = new Date(item.at);
            const stamp = `${d.getFullYear()}-${pad(d.getMonth() + 1)}-${pad(d.getDate())} ${pad(d.getHours())}:${pad(d.getMinutes())}:${pad(d.getSeconds())}`;
            const seconds = Math.max(item.durationMs, 1) / 1000;
            const tps = (item.completionTokens / seconds).toFixed(1);
            const dur = item.durationMs >= 1000
              ? `${(item.durationMs / 1000).toFixed(1)}s`
              : `${item.durationMs}ms`;
            return (
              <div
                key={ii}
                className="pt-0.5 font-mono text-[10px] text-[var(--fg-tertiary)]/80 select-none flex flex-wrap gap-x-3"
                title="tokens for this response"
              >
                <span>{stamp}</span>
                <span>in: {item.promptTokens}</span>
                <span>out: {item.completionTokens}</span>
                {item.cachedTokens > 0 && <span>cache {item.cachedTokens >= 1000 ? `${Math.round(item.cachedTokens / 1000)}K` : item.cachedTokens}</span>}
                {item.durationMs > 0 && <span>t: {dur}</span>}
                <span>tok/s: {tps}/s</span>
              </div>
            );
          }

          if (item.kind === "notice") {
            const [firstLine, ...restLines] = item.text.split("\n");
            return (
              <div
                key={ii}
                className="my-1 rounded-md border border-[var(--accent-primary)]/30 bg-[var(--bg-panel)] overflow-hidden select-text"
              >
                <div className="flex items-center gap-1.5 px-2.5 py-1 bg-[var(--accent-primary)]/10 border-b border-[var(--border-default)]">
                  <IconTerminal2 className="size-3 text-[var(--accent-primary)]" />
                  <span className="text-[10px] font-mono font-semibold uppercase tracking-wider text-[var(--accent-primary)]">
                    {firstLine}
                  </span>
                </div>
                <pre className="px-2.5 py-1.5 text-[11px] leading-relaxed font-mono text-[var(--fg-secondary)] whitespace-pre-wrap break-words max-h-64 overflow-y-auto">
                  {restLines.length > 0 ? restLines.join("\n") : firstLine}
                </pre>
              </div>
            );
          }

          if (item.kind === "text") {
            const dur = itemDuration(item, running && isLast, isLast ? fallbackEnd : undefined);
            const streaming = running && isLast && item.text.length > 0;
            return <TextBlockView key={ii} text={item.text} streaming={streaming} dur={dur} />;
          }

          if (item.kind === "thinking") {
            const isThinkingRunning = isLast && running;
            const rKey = `r-${ti}-${ii}`;
            return (
              <TerminalThinkingBlock
                key={ii}
                text={item.text}
                open={expandedReasoning[rKey] ?? false}
                onToggle={() => onToggleReasoning(rKey)}
                running={isThinkingRunning}
                durationMs={itemDuration(item, false, isLast ? fallbackEnd : undefined)}
              />
            );
          }

          // Tool execution
          const isRunning = !item.tool.result && running;
          const tcKey = `tc-${ti}-${ii}`;
          return (
            <TerminalToolRow
              key={ii}
              toolCall={item.tool}
              running={isRunning}
              expanded={expandedToolCalls[tcKey] ?? true}
              onToggle={() => onToggleToolCall(tcKey)}
              durationMs={itemDuration(item, false, isLast ? fallbackEnd : undefined)}
            />
          );
        })}
      </div>
    </div>
  );
});
// ---------------------------------------------------------------------------
// Revamped Developer Agent Terminal Body (No Cards, No Bubbles)
// ---------------------------------------------------------------------------
export function AgentChatBody({
  messages,
  session,
  onApprove,
  onDeny,
}: {
  messages: any[];
  session: any;
  onApprove?: (approve: boolean, autoAll: boolean) => void;
  onDeny?: () => void;
}) {
  const [expandedReasoning, setExpandedReasoning] = useState<Record<string, boolean>>({});
  const [expandedToolCalls, setExpandedToolCalls] = useState<Record<string, boolean>>({});
  const scrollRef = useRef<HTMLDivElement>(null);
  const pinnedRef = useRef(true);
  const [canScrollUp, setCanScrollUp] = useState(false);
  const [canScrollDown, setCanScrollDown] = useState(false);

  const turns = useMemo(() => buildTurns(messages || []), [messages]);

  const handleScroll = useCallback(() => {
    const el = scrollRef.current;
    if (!el) return;
    const bottom = el.scrollHeight - el.scrollTop - el.clientHeight < 40;
    pinnedRef.current = bottom;
    const canUp = el.scrollTop > 40;
    const canDown = !bottom && el.scrollHeight > el.clientHeight + 40;
    setCanScrollUp((prev) => (prev !== canUp ? canUp : prev));
    setCanScrollDown((prev) => (prev !== canDown ? canDown : prev));
  }, []);

  const handleToggleReasoning = useCallback((key: string) => {
    setExpandedReasoning((p) => ({ ...p, [key]: !p[key] }));
  }, []);

  const handleToggleToolCall = useCallback((key: string) => {
    setExpandedToolCalls((p) => ({ ...p, [key]: !(p[key] ?? true) }));
  }, []);

  const scrollUpStep = () => {
    const el = scrollRef.current;
    if (!el) return;
    el.scrollBy({ top: -200, behavior: "smooth" });
  };

  const scrollDownDirect = () => {
    const el = scrollRef.current;
    if (!el) return;
    pinnedRef.current = true;
    el.scrollTo({ top: el.scrollHeight, behavior: "smooth" });
  };

  useEffect(() => {
    if (!pinnedRef.current) return;
    const el = scrollRef.current;
    if (!el) return;
    const raf = requestAnimationFrame(() => {
      el.scrollTop = el.scrollHeight;
    });
    return () => cancelAnimationFrame(raf);
  }, [messages, turns]);
  const state = session?.state || "idle";

  const handleChatClick = (e: React.MouseEvent) => {
    const target = e.target as HTMLElement | null;
    if (!target) return;
    const anchor = target.closest("a");
    if (anchor) {
      const href = anchor.getAttribute("href");
      if (href) {
        if (href.startsWith("http://") || href.startsWith("https://")) {
          return;
        }
        e.preventDefault();
        e.stopPropagation();
        const cleanPath = href.replace(/^file:\/\//, "");
        globalOpenFile(cleanPath);
        return;
      }
    }
    if (target.tagName === "CODE") {
      const txt = target.innerText.trim();
      if (/^[\w./\\-]+(?::\d+)*$/.test(txt) && (txt.includes("/") || txt.includes("\\") || txt.includes("."))) {
        e.preventDefault();
        e.stopPropagation();
        globalOpenFile(txt);
      }
    }
  };

  return (
    <div className="relative flex-1 min-h-0 flex flex-col font-sans">
      <div
        ref={scrollRef}
        onScroll={handleScroll}
        onClick={handleChatClick}
        className="flex-1 overflow-y-auto p-4 space-y-5 font-mono select-text"
      >
        {messages?.length === 0 ? (
          <div className="flex flex-col items-center justify-center h-full text-center space-y-3 text-[var(--fg-tertiary)] select-none py-12">
            <IconTerminal2 className="size-12 stroke-[1.2] text-[var(--accent-primary)] opacity-80" />
            <div>
              <h3 className="text-sm font-semibold text-[var(--fg-primary)] font-mono">Agent Terminal Active</h3>
              <p className="text-xs max-w-sm mt-1 text-[var(--fg-secondary)]">
                Direct developer execution stream with tool calling, filesystem inspection, and git integration.
              </p>
            </div>
            <div className="flex items-center gap-2 text-[11px] font-mono text-[var(--fg-tertiary)] pt-2">
              <span className="px-1.5 py-0.5 bg-[var(--bg-panel)] border border-[var(--border-default)] rounded">/commit</span>
              <span className="px-1.5 py-0.5 bg-[var(--bg-panel)] border border-[var(--border-default)] rounded">/plan</span>
              <span className="px-1.5 py-0.5 bg-[var(--bg-panel)] border border-[var(--border-default)] rounded">@file</span>
            </div>
          </div>
        ) : (
          turns.map((turn, ti) => {
            const isLastTurn = ti === turns.length - 1;
            const isRunning = state !== "idle" && isLastTurn;
            const updTs = (() => {
              const t = session?.updated_at ?? session?.updatedAt;
              if (!t) return 0;
              const d = new Date(t).getTime();
              return isNaN(d) ? 0 : d;
            })();
            const fallbackEnd = updTs || undefined;

            return (
              <TurnView
                key={ti}
                turn={turn}
                ti={ti}
                isLastTurn={isLastTurn}
                running={isRunning}
                fallbackEnd={fallbackEnd}
                expandedReasoning={expandedReasoning}
                expandedToolCalls={expandedToolCalls}
                onToggleReasoning={handleToggleReasoning}
                onToggleToolCall={handleToggleToolCall}
              />
            );
          })
        )}
      </div>

      {/* Floating Scroll Controls: Move up (little by little) & Scroll down directly */}
      {(canScrollUp || canScrollDown) && (
        <div className="absolute right-3.5 bottom-3 flex flex-col gap-1.5 z-30 select-none shadow-lg">
          <button
            onClick={scrollUpStep}
            title="Move up (scroll little by little)"
            className="p-1.5 rounded-full bg-[var(--bg-elevated)]/95 hover:bg-[var(--bg-surface-hover)] border border-[var(--border-default)] text-[var(--fg-secondary)] hover:text-[var(--fg-primary)] backdrop-blur-md cursor-pointer transition-all hover:scale-110 active:scale-95 shadow-sm"
          >
            <IconChevronUp className="size-3.5" />
          </button>
          <button
            onClick={scrollDownDirect}
            title="Scroll down directly (jump to bottom)"
            className="p-1.5 rounded-full bg-[var(--bg-elevated)]/95 hover:bg-[var(--bg-surface-hover)] border border-[var(--border-default)] text-cyan-400 hover:text-cyan-300 backdrop-blur-md cursor-pointer transition-all hover:scale-110 active:scale-95 shadow-sm"
          >
            <IconArrowDown className="size-3.5" />
          </button>
        </div>
      )}
    </div>
  );
}
