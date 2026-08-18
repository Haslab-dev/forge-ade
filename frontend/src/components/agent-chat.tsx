import React, { useEffect, useMemo, useRef, useState } from "react";
import {
  IconBrain,
  IconChevronDown,
  IconChevronRight,
  IconCopy,
  IconShield,
  IconCheck,
  IconArrowUp,
  IconArrowDown,
} from "@tabler/icons-react";
import { marked } from "marked";
import { RespondAgentAsk } from "../lib/wails";
import { globalOpenFile } from "../panels/editor";
import { cn } from "../lib/utils";

// ---------------------------------------------------------------------------
// Markdown
// ---------------------------------------------------------------------------
// Memoize parsed HTML per source string. During streaming the same text block
// re-renders on every delta; without this cache the whole accumulated answer
// is re-parsed through marked on every keystroke of output.
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


// Format a duration like command-code CLI: "3s", "1m 24s", "2h 3m 10s".
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

// Resolve an item's duration: endTs - startTs, or fallbackEnd - startTs, or
// now - startTs while running.
function itemDuration(item: any, running: boolean, fallbackEnd?: number): number {
  const start = item?.startTs;
  if (!start) return 0;
  let end = item?.endTs;
  if (!end) end = running ? Date.now() : fallbackEnd || start;
  return Math.max(0, end - start);
}

// ---------------------------------------------------------------------------
// Message model — the agent transcript is a flat list of block-based messages.
// We rebuild it into ordered turn items so prose and tool calls interleave
// exactly like a real agent TUI: text → tool badge → result lines → text.
// ---------------------------------------------------------------------------
interface ToolCallView {
  id: string;
  name: string;
  arguments: any;
  result: string;
  is_error?: boolean;
}

type TurnItem =
  | { kind: "text"; text: string; startTs?: number }
  | { kind: "thinking"; text: string; startTs?: number }
  | { kind: "tool"; tool: ToolCallView; startTs?: number };

interface Turn {
  prompt: string;
  items: TurnItem[];
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

  // Close the previous message's open items with the current message's
  // timestamp so every block gets a real duration (thinking/tool/answer).
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
      const prompt =
        blockText(blocks.find((b: any) => b.type === "text")) ||
        (typeof msg.content === "string" ? msg.content : "");
      current = { prompt, items: [] };
      continue;
    }
    if (!current) current = { prompt: "", items: [] };

    if (role === "assistant") {
      for (const b of blocks) {
        const t = b.type;
        if (t === "text" && blockText(b)) {
          const last = current.items[current.items.length - 1];
          if (last?.kind === "text") {
            (last as any).text += blockText(b);
          } else {
            // Close preceding open items (e.g. thinking block before text)
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
          const tc: ToolCallView = {
            id: b.tool_call_id ?? "",
            name: b.name ?? "tool",
            arguments: b.arguments ?? {},
            result: "",
          };
          // Dedupe by id (a replayed turn re-declares the same call).
          const existing = current.items.find(
            (it) => it.kind === "tool" && it.tool.id && it.tool.id === tc.id
          );
          if (!existing) current.items.push({ kind: "tool", tool: tc, startTs: ts || undefined });
        }
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
        // Attach to the matching tool call by id, else the last one without a result.
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
          // Orphan result — synthesize a tool item so nothing is lost.
          current.items.push({
            kind: "tool",
            tool: { id: b.tool_call_id ?? "", name: "tool", arguments: {}, result: resultText, is_error: isErr },
            startTs: ts || undefined,
          });
        }
        // The tool finished when its result message arrived.
        if (toolItem && ts) (toolItem as any).endTs = ts;
      }
    }
  }
  // Close any still-open items (the final answer / last thinking) with the
  // last known message timestamp so they get a real duration.
  if (current && lastTs) closeOpenItems(lastTs);
  flush();
  return turns;
}

// ---------------------------------------------------------------------------
// Tool badge + title — uppercase tool name with a bracket path/command, like
// a real agent TUI:
//   EDIT  [frontend/src/panels/editor.tsx]
//   SHELL [cd frontend && npx tsc ...]
//   SEARCH ["pattern" in src]
// ---------------------------------------------------------------------------
const TOOL_BADGE: Record<string, { label: string; cls: string }> = {
  write: { label: "WRITE", cls: "text-emerald-400 border-emerald-500/40 bg-emerald-500/10" },
  write_file: { label: "WRITE", cls: "text-emerald-400 border-emerald-500/40 bg-emerald-500/10" },
  create_file: { label: "WRITE", cls: "text-emerald-400 border-emerald-500/40 bg-emerald-500/10" },
  edit: { label: "EDIT", cls: "text-emerald-400 border-emerald-500/40 bg-emerald-500/10" },
  edit_file: { label: "EDIT", cls: "text-emerald-400 border-emerald-500/40 bg-emerald-500/10" },
  read: { label: "READ", cls: "text-sky-400 border-sky-500/40 bg-sky-500/10" },
  read_multiple: { label: "READ×N", cls: "text-sky-400 border-sky-500/40 bg-sky-500/10" },
  read_directory_files: { label: "READ×N", cls: "text-sky-400 border-sky-500/40 bg-sky-500/10" },
  read_file: { label: "READ", cls: "text-sky-400 border-sky-500/40 bg-sky-500/10" },
  view_file: { label: "READ", cls: "text-sky-400 border-sky-500/40 bg-sky-500/10" },
  cat: { label: "READ", cls: "text-sky-400 border-sky-500/40 bg-sky-500/10" },
  list_dir: { label: "LIST", cls: "text-sky-400 border-sky-500/40 bg-sky-500/10" },
  ls: { label: "LIST", cls: "text-sky-400 border-sky-500/40 bg-sky-500/10" },
  find: { label: "FIND", cls: "text-sky-400 border-sky-500/40 bg-sky-500/10" },
  glob: { label: "GLOB", cls: "text-sky-400 border-sky-500/40 bg-sky-500/10" },
  search: { label: "SEARCH", cls: "text-fuchsia-400 border-fuchsia-500/40 bg-fuchsia-500/10" },
  search_workspace: { label: "SEARCH", cls: "text-fuchsia-400 border-fuchsia-500/40 bg-fuchsia-500/10" },
  rg: { label: "SEARCH", cls: "text-fuchsia-400 border-fuchsia-500/40 bg-fuchsia-500/10" },
  grep: { label: "SEARCH", cls: "text-fuchsia-400 border-fuchsia-500/40 bg-fuchsia-500/10" },
  bash: { label: "SHELL", cls: "text-amber-400 border-amber-500/40 bg-amber-500/10" },
  run_shell: { label: "SHELL", cls: "text-amber-400 border-amber-500/40 bg-amber-500/10" },
  exec: { label: "SHELL", cls: "text-amber-400 border-amber-500/40 bg-amber-500/10" },
  run_command: { label: "SHELL", cls: "text-amber-400 border-amber-500/40 bg-amber-500/10" },
  git_status: { label: "GIT", cls: "text-orange-400 border-orange-500/40 bg-orange-500/10" },
  todo: { label: "TODO", cls: "text-amber-400 border-amber-500/40 bg-amber-500/10" },
  ask: { label: "ASK", cls: "text-purple-400 border-purple-500/40 bg-purple-500/10" },
};

function toolBadge(name: string): { label: string; cls: string } {
  return (
    TOOL_BADGE[name] ?? {
      label: (name || "TOOL").toUpperCase().slice(0, 10),
      cls: "text-[var(--fg-secondary)] border-[var(--border-default)] bg-[var(--bg-surface)]",
    }
  );
}

// Title after the badge: for file tools show the path, for shell show the
// command, for search show the pattern.
function toolTitle(tc: ToolCallView): string {
  const args = tc.arguments;
  if (args && typeof args === "object") {
    if (args.path) return String(args.path);
    if (args.pattern) return typeof args.pattern === "string" ? args.pattern : JSON.stringify(args.pattern);
    if (args.query) return typeof args.query === "string" ? args.query : JSON.stringify(args.query);
    if (args.command) return String(args.command);
  }
  return "";
}

// ---------------------------------------------------------------------------
// Tool result — show the human content with a tree prefix, like the TUI:
//   │ Edited /path/file.tsx (1 replacement)
//   └ ... +3 lines
// ---------------------------------------------------------------------------
const MAX_TOOL_LINES = 500; // cap expanded tool output so huge shells don't kill the DOM

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
    for (const e of obj.entries.slice(0, 8)) lines.push(e.is_dir ? `${e.name}/` : e.name);
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
  if (typeof obj.status === "string" && obj.status !== "written") lines.push(obj.status);
  if (typeof obj.replacements === "number") lines.push(`Edited with ${obj.replacements} replacement${obj.replacements === 1 ? "" : "s"}`);
  if (typeof obj.exit_code === "number") lines.push(`exit ${obj.exit_code}`);
  if (typeof obj.count === "number") lines.push(`${obj.count} results`);
  if (lines.length === 0) return { lines: [], total: 0 };
  return { lines, total: lines.length };
}

// ---------------------------------------------------------------------------
// Tool call row — a real-TUI-style badge line + tree-prefixed result.
// ---------------------------------------------------------------------------
function ToolCallRow({
  toolCall,
  expanded,
  onToggle,
  running,
  showMore,
  onShowMore,
  onShowLess,
  durationMs,
}: {
  toolCall: ToolCallView;
  expanded: boolean;
  onToggle: () => void;
  running?: boolean;
  showMore: boolean;
  onShowMore: () => void;
  onShowLess: () => void;
  durationMs?: number;
}) {
  const badge = toolBadge(toolCall.name);
  const title = toolTitle(toolCall);
  const { lines, total } = renderToolLines(toolCall.result);
  const hasResult = toolCall.result !== "" && toolCall.result != null;
  // Collapse default → open shows 8 lines → "show more" shows up to 40 with
  // scroll → "show less" returns to 8. No body is rendered while collapsed.
  const previewLimit = 8;
  const showMoreLimit = 40;
  const visible = showMore ? lines.slice(0, showMoreLimit) : lines.slice(0, previewLimit);
  const totalMore = total - (showMore ? showMoreLimit : previewLimit);

  return (
    <div className="rounded-md border border-[var(--border-default)] overflow-hidden bg-[var(--bg-panel)]">
      {/* Header: badge + title + status + chevron */}
      <button
        onClick={onToggle}
        className="w-full flex items-center gap-2 px-2.5 py-[6px] text-[13px] hover:bg-[var(--bg-surface-hover)] cursor-pointer text-left"
      >
        <span className={`shrink-0 text-[10px] font-bold tracking-wide border rounded px-1.5 py-0.5 ${badge.cls}`}>
          {badge.label}
        </span>
        {title ? (
          <span className="font-mono text-[12px] text-[var(--fg-primary)] truncate flex-1">{title}</span>
        ) : (
          <span className="font-mono text-[12px] text-[var(--fg-tertiary)] flex-1">{toolCall.name}</span>
        )}

        {running ? (
          <span className="text-[11px] text-purple-400 font-mono shrink-0 animate-pulse">⠶</span>
        ) : hasResult ? (
          <span className={`text-[11px] font-mono shrink-0 ${toolCall.is_error ? "text-red-400" : "text-emerald-400"}`}>
            {toolCall.is_error ? "✗ failed" : "✓ done"}
          </span>
        ) : null}
        {durationMs != null && durationMs > 0 && (
          <span className="text-[10px] font-mono text-[var(--fg-tertiary)] shrink-0">
            work for {fmtBlockDuration(durationMs)}
          </span>
        )}
        {expanded ? (
          <IconChevronDown className="size-3 text-[var(--fg-tertiary)] shrink-0" />
        ) : (
          <IconChevronRight className="size-3 text-[var(--fg-tertiary)] shrink-0" />
        )}
      </button>

      {/* Body — rendered ONLY when expanded (default is collapsed) */}
      {expanded && (
        <div className="px-2.5 pb-2 pt-1 border-t border-[var(--border-default)] font-mono text-[12px] text-[var(--fg-secondary)]">
          {lines.length === 0 && <div className="text-[var(--fg-tertiary)]">(no output)</div>}
          <div className={cn("overflow-y-auto pr-1", showMore && "max-h-[320px]")}>
            {visible.map((l, i) => (
              <div key={i} className="flex gap-2 leading-relaxed">
                <span className="text-[var(--fg-tertiary)] select-none">
                  {i === visible.length - 1 && total > visible.length ? "└" : "│"}
                </span>
                <span className="whitespace-pre-wrap break-all min-w-0">{l}</span>
              </div>
            ))}
          </div>
          {total > (showMore ? showMoreLimit : previewLimit) &&
            (showMore ? (
              <div className="flex items-center justify-between mt-0.5">
                <span className="text-[var(--fg-tertiary)] text-[11px] pl-5">
                  … +{totalMore} more lines (scrollable)
                </span>
                <button
                  onClick={(e) => { e.stopPropagation(); onShowLess(); }}
                  className="text-[var(--fg-tertiary)] text-[11px] pl-5 mt-0.5 hover:text-[var(--fg-primary)] cursor-pointer"
                >
                  show less
                </button>
              </div>
            ) : (
              <button
                onClick={(e) => { e.stopPropagation(); onShowMore(); }}
                className="text-[var(--fg-tertiary)] text-[11px] pl-5 mt-0.5 hover:text-[var(--fg-primary)] cursor-pointer"
              >
                … +{total - previewLimit} more lines — show more
              </button>
            ))}
        </div>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Thinking block — collapsible accordion, click to reveal content.
// ---------------------------------------------------------------------------
function ThinkingBlock({ text, open, onToggle, durationMs }: { text: string; open: boolean; onToggle: () => void; durationMs?: number }) {
  const lines = text.split("\n");
  const collapsed = lines.slice(0, 10);
  const hasMore = lines.length > 10;
  return (
    <div className="rounded-lg border border-[var(--border-default)] overflow-hidden bg-[var(--bg-panel)] select-text">
      <div className="flex items-center justify-between px-3 py-1.5 hover:bg-[var(--bg-surface-hover)]">
        <button
          onClick={onToggle}
          className="flex-1 flex items-center gap-2 text-[12px] font-medium text-[var(--fg-tertiary)] cursor-pointer text-left"
        >
          {open ? <IconChevronDown className="size-3.5" /> : <IconChevronRight className="size-3.5" />}
          <IconBrain className="size-3.5 text-purple-400" />
          <span>Thinking</span>
          {durationMs != null && durationMs > 0 && (
            <span className="text-[10px] font-mono text-[var(--fg-tertiary)] shrink-0 ml-2">
              Thought for {fmtBlockDuration(durationMs)}
            </span>
          )}
        </button>
        <CopyButton text={text} className="shrink-0 ml-2" />
      </div>
      {(open || lines.length > 0) && (
        <div className="px-3 pb-2.5 pt-1.5 text-[13px] leading-relaxed text-[var(--fg-secondary)] whitespace-pre-wrap border-t border-[var(--border-default)] font-mono select-text cursor-text">
          {open ? (
            <div className="max-h-[320px] overflow-y-auto pr-1 select-text cursor-text">{text}</div>
          ) : (
            collapsed.join("\n")
          )}
          {!open && hasMore && (
            <button
              onClick={onToggle}
              className="block mt-1 text-[11px] text-[var(--fg-tertiary)] hover:text-[var(--fg-primary)] cursor-pointer"
            >
              … view {lines.length - 10} more lines
            </button>
          )}
        </div>
      )}
    </div>
  );
}

// ---------------------------------------------------------------------------
// Ask card — the agent paused for structured input.
// ---------------------------------------------------------------------------
function AskCard({ sessionId, questions }: { sessionId: string; questions: any[] }) {
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
    <div className="border border-[var(--border-default)] bg-[var(--bg-panel)] p-3 space-y-3 shadow-md max-w-md">
      <div className="flex items-center space-x-2 text-xs font-semibold text-sky-400">
        <IconShield className="size-4" />
        <span>The agent needs your input</span>
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

// ---------------------------------------------------------------------------
// The shared chat body — renders turns like a real agent TUI: prompt card,
// then interleaved prose + tool badges + thinking + result lines.
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
  // Whether a tool's body is in "show more" mode (up to 40 lines + scroll)
  // vs the default 8-line preview.
  const [expandedToolMore, setExpandedToolMore] = useState<Record<string, boolean>>({});
  const scrollRef = useRef<HTMLDivElement>(null);
  const pinnedRef = useRef(true);
  const [showScrollBtns, setShowScrollBtns] = useState(false);
  const [atBottom, setAtBottom] = useState(true);

  const turns = useMemo(() => buildTurns(messages || []), [messages]);

  const handleScroll = () => {
    const el = scrollRef.current;
    if (!el) return;
    const bottom = el.scrollHeight - el.scrollTop - el.clientHeight < 40;
    pinnedRef.current = bottom;
    setAtBottom(bottom);
    // Show buttons only when content overflows the viewport.
    setShowScrollBtns(el.scrollHeight > el.clientHeight + 20);
  };

  const scrollUp = () => {
    const el = scrollRef.current;
    if (!el) return;
    el.scrollBy({ top: -Math.max(200, el.clientHeight * 0.8), behavior: "smooth" });
  };

  const scrollDown = () => {
    const el = scrollRef.current;
    if (!el) return;
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
          // Keep external HTTP web links standard
          return;
        }
        e.preventDefault();
        e.stopPropagation();
        const cleanPath = href.replace(/^file:\/\//, "");
        globalOpenFile(cleanPath);
        return;
      }
    }
    // Also handle inline code text clicks if they look like file paths e.g. `src/App.tsx:12`
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
    <div className="relative flex-1 min-h-0 flex flex-col">
      <div ref={scrollRef} onScroll={handleScroll} onClick={handleChatClick} className="flex-1 overflow-y-auto p-4 space-y-4">
      {messages?.length === 0 ? (
        <div className="flex flex-col items-center justify-center h-full text-center space-y-2 text-[var(--fg-tertiary)] select-none">
          <IconBrain className="size-12 stroke-[1.2] text-[var(--fg-disabled)] animate-pulse" />
          <h3 className="text-sm font-semibold text-[var(--fg-secondary)]">Agent Assistant</h3>
          <p className="text-xs max-w-xs mt-1">
            Ask coding questions, draft features, or request file changes using natural language.
          </p>
        </div>
      ) : (
        turns.map((turn, ti) => (
          <div key={ti} className="space-y-2.5">
            {turn.prompt && (
              <div className="group relative rounded-xl border border-[var(--border-default)] bg-[var(--bg-panel)] px-4 py-3 text-[15px] leading-relaxed text-[var(--fg-primary)] selectable-text">
                {turn.prompt}
                <button
                  onClick={() => navigator.clipboard.writeText(turn.prompt)}
                  className="absolute top-2 right-2 p-1 rounded hover:bg-[var(--bg-surface-hover)] text-[var(--fg-tertiary)] hover:text-white opacity-0 group-hover:opacity-100 transition-opacity cursor-pointer"
                  title="Copy"
                >
                  <IconCopy className="size-3.5" />
                </button>
              </div>
            )}

            {turn.items.map((item, ii) => {
              const running = state !== "idle";
              // Backend bumps session.updated_at continuously while streaming,
              // so it's the best "end of turn" timestamp for the final answer /
              // last thinking block (which share their message's start ts).
              const updTs = (() => {
                const t = session?.updated_at ?? session?.updatedAt;
                if (!t) return 0;
                const d = new Date(t).getTime();
                return isNaN(d) ? 0 : d;
              })();
              const fallbackEnd = updTs || undefined;
              const isLast = ii === turn.items.length - 1;
              if (item.kind === "text") {
                const dur = itemDuration(item, running && isLast, isLast ? fallbackEnd : undefined);
                return (
                  <div key={ii} className="group/msg relative select-text cursor-text">
                    <div className="absolute top-0 right-0 opacity-0 group-hover/msg:opacity-100 transition-opacity z-10">
                      <CopyButton text={item.text} className="bg-[var(--bg-panel)] border border-[var(--border-default)] shadow-xs" />
                    </div>
                    <div
                      className="text-[14px] leading-[1.7] text-[var(--fg-primary)] markdown-body select-text cursor-text"
                      dangerouslySetInnerHTML={{ __html: renderMarkdown(item.text) }}
                    />
                    {dur > 0 && (
                      <div className="text-[10px] font-mono text-[var(--fg-tertiary)] mt-0.5 select-none">
                        done: {fmtBlockDuration(dur)}
                      </div>
                    )}
                  </div>
                );
              }
              if (item.kind === "thinking") {
                return (
                  <ThinkingBlock
                    key={ii}
                    text={item.text}
                    open={expandedReasoning[`r-${ti}-${ii}`] ?? false}
                    onToggle={() =>
                      setExpandedReasoning((p) => ({ ...p, [`r-${ti}-${ii}`]: !p[`r-${ti}-${ii}`] }))
                    }
                    durationMs={itemDuration(item, false, isLast ? fallbackEnd : undefined)}
                  />
                );
              }
              // tool
              const isRunning = !item.tool.result && running;
              return (
                <ToolCallRow
                  key={ii}
                  toolCall={item.tool}
                  running={isRunning}
                  expanded={!!expandedToolCalls[`tc-${ti}-${ii}`]}
                  onToggle={() =>
                    setExpandedToolCalls((p) => ({ ...p, [`tc-${ti}-${ii}`]: !p[`tc-${ti}-${ii}`] }))
                  }
                  showMore={!!expandedToolMore[`tc-${ti}-${ii}`]}
                  onShowMore={() =>
                    setExpandedToolMore((p) => ({ ...p, [`tc-${ti}-${ii}`]: true }))
                  }
                  onShowLess={() =>
                    setExpandedToolMore((p) => ({ ...p, [`tc-${ti}-${ii}`]: false }))
                  }
                  durationMs={itemDuration(item, isRunning)}
                />
              );
            })}
          </div>
        ))
      )}

      {session?.pending_questions && session.pending_questions.length > 0 && (
        <AskCard sessionId={session.id} questions={session.pending_questions} />
      )}

      {session?.pending_tools && session.pending_tools.length > 0 && (
        <div className="border border-[var(--border-default)] bg-[var(--bg-panel)] p-3 space-y-3 shadow-md max-w-md">
          <div className="flex items-center space-x-2 text-xs font-semibold text-amber-400">
            <IconShield className="size-4" />
            <span>Permission Request</span>
          </div>
          <div className="space-y-1.5">
            {session.pending_tools.map((tc: any, i: number) => {
              const name = tc?.name || "tool";
              const rawArgs = tc?.arguments ?? "{}";
              let argsText = rawArgs;
              if (typeof argsText !== "string") argsText = JSON.stringify(argsText);
              return (
                <div
                  key={i}
                  className="text-xs font-mono bg-black/30 p-2 border border-[var(--border-default)] text-[var(--fg-primary)] overflow-x-auto whitespace-pre-wrap break-all"
                >
                  <div className="text-amber-300/90 font-semibold">{name}</div>
                  {argsText}
                </div>
              );
            })}
          </div>
          <div className="flex items-center justify-end space-x-2 pt-1">
            <button
              onClick={() => onDeny?.()}
              className="px-2.5 py-1 text-xs text-[var(--fg-secondary)] hover:text-white border border-[var(--border-default)] rounded hover:bg-[var(--bg-surface-hover)] cursor-pointer"
            >
              Deny
            </button>
            <button
              onClick={() => onApprove?.(true, true)}
              className="px-2.5 py-1 text-xs text-[var(--fg-secondary)] border border-[var(--border-default)] rounded hover:bg-[var(--bg-surface-hover)] cursor-pointer"
              title="Approve this and all future tool calls"
            >
              Always allow
            </button>
            <button
              onClick={() => onApprove?.(true, false)}
              className="px-3 py-1 text-xs font-semibold bg-amber-500 hover:bg-amber-600 text-black rounded flex items-center space-x-1 cursor-pointer"
            >
              <IconCheck className="size-3.5" />
              <span>Approve</span>
            </button>
          </div>
        </div>
      )}
      </div>

      {/* Scroll controls — top-right, only when content overflows */}
      {showScrollBtns && (
        <div className="absolute top-2 right-2 flex flex-col gap-1 select-none z-10">
          <button
            onClick={scrollUp}
            title="Scroll up"
            className="p-1.5 rounded bg-black/60 border border-[var(--border-default)] text-white hover:text-white hover:bg-black/80 cursor-pointer backdrop-blur-sm"
          >
            <IconArrowUp className="size-3.5" />
          </button>
          <button
            onClick={scrollDown}
            title="Scroll to bottom"
            disabled={atBottom}
            className="p-1.5 rounded bg-black/60 border border-[var(--border-default)] text-white hover:text-white hover:bg-black/80 cursor-pointer backdrop-blur-sm disabled:opacity-40 disabled:cursor-default"
          >
            <IconArrowDown className="size-3.5" />
          </button>
        </div>
      )}
    </div>
  );
}
