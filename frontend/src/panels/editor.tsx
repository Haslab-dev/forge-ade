import React, { useEffect, useRef, useState, useMemo } from "react";
import { useEditorStore, useUIStore } from "../hooks/store";
import { getFileIcon } from "../lib/file-icons";
import { useToast } from "../lib/toast";
import { ReadFile, ReadFileBase64, WriteFile, GetProviderProfiles, GetLLMConfig, SendAgentMessage, RespondAgentApproval, ListAgentSessions, SetActiveModel, EventsOn, CheckSyntax, FormatCode } from "../lib/wails";
import { TerminalView } from "../components/terminal-view";
import {
  X,
  Copy,
  Eye,
  FileCode2,
  Image as ImageIcon,
  FileText as FileTextIcon,
  Globe,
  Settings,
  Cpu,
  ChevronDown,
  ChevronUp,
  ChevronRight,
  Brain,
  Shield,
  Check,
  Send,
  Search,
  ArrowDown,
  ArrowUp,
  Zap,
} from "lucide-react";
import { EditorState } from "@codemirror/state";
import { EditorView, keymap } from "@codemirror/view";
import { defaultKeymap, history, historyKeymap, indentMore, indentLess, indentWithTab, toggleComment, toggleBlockComment } from "@codemirror/commands";
import { linter, Diagnostic } from "@codemirror/lint";
import { javascript } from "@codemirror/lang-javascript";
import { go } from "@codemirror/lang-go";
import { python } from "@codemirror/lang-python";
import { rust } from "@codemirror/lang-rust";
import { json } from "@codemirror/lang-json";
import { html } from "@codemirror/lang-html";
import { xml } from "@codemirror/lang-xml";
import { markdown } from "@codemirror/lang-markdown";
import { java } from "@codemirror/lang-java";
import { cpp } from "@codemirror/lang-cpp";
import { sql } from "@codemirror/lang-sql";
import { php } from "@codemirror/lang-php";
import { vue } from "@codemirror/lang-vue";
import { oneDark } from "@codemirror/theme-one-dark";
import { marked } from "marked";

let onBeforeOpenFileCallback: (() => void) | null = null;
export function setOnBeforeOpenFile(cb: () => void) {
  onBeforeOpenFileCallback = cb;
}

// Module-level reference to the active editor view, so other modules (e.g.
// format-on-save from App.tsx) can dispatch content into the open editor.
let globalEditorView: EditorView | null = null;
export function setGlobalEditorView(view: EditorView | null) {
  globalEditorView = view;
}
export function applyFormattedContent(content: string) {
  if (globalEditorView && content !== globalEditorView.state.doc.toString()) {
    globalEditorView.dispatch({ changes: { from: 0, to: globalEditorView.state.doc.length, insert: content } });
  }
}

// Render markdown to HTML for chat responses.
function renderMarkdown(src: string): string {
  try {
    return marked.parse(src, { async: false }) as string;
  } catch {
    return src;
  }
}

// Token usage breakdown: ↓ input, ↑ output, ⚡ cached.
function TokenUsageBadge({ usage }: { usage: any }) {
  const inTok = usage?.prompt_tokens ?? usage?.PromptTokens ?? 0;
  const outTok = usage?.completion_tokens ?? usage?.CompletionTokens ?? 0;
  const cached = usage?.cached_tokens ?? usage?.CachedTokens ?? 0;
  if (inTok + outTok + cached === 0) return null;
  return (
    <span className="flex items-center gap-2 px-1.5 py-0.5 bg-[var(--bg-panel)] border border-[var(--border-default)] text-[10px] font-mono text-[var(--fg-tertiary)] rounded" title="Token usage: input / output / cached">
      <span className="flex items-center gap-0.5">
        <ArrowDown className="size-2.5" />
        {inTok.toLocaleString()}
      </span>
      <span className="flex items-center gap-0.5">
        <ArrowUp className="size-2.5" />
        {outTok.toLocaleString()}
      </span>
      <span className="flex items-center gap-0.5" title="Cached tokens">
        <Zap className="size-2.5" />
        {cached.toLocaleString()}
      </span>
    </span>
  );
}

export async function globalOpenFile(path: string) {
  if (onBeforeOpenFileCallback) onBeforeOpenFileCallback();
  const { files, setFiles, setActiveFileIndex } = useEditorStore.getState();

  const existingIdx = files.findIndex((f) => f.path === path);
  if (existingIdx !== -1) {
    setActiveFileIndex(existingIdx);
    return;
  }

  try {
    const name = path.split(/[/\\]/).pop() || "Untitled";
    const ext = name.split(".").pop()?.toLowerCase();
    const isBinary = ["png", "jpg", "jpeg", "gif", "pdf", "ico"].includes(ext || "");
    
    let content = "";
    if (!isBinary) {
      content = await ReadFile(path);
    }
    
    const newFile = {
      id: path,
      name,
      path,
      type: "file" as "file",
      content,
      modified: false,
    };

    setFiles((prev) => [...prev, newFile]);
    // Read fresh state after setFiles to get the correct new index
    setActiveFileIndex(useEditorStore.getState().files.length - 1);
  } catch (err) {
    console.error("Failed to open file:", err);
  }
}

export function Editor() {
  const { files, activeFileIndex, setFiles, setActiveFileIndex } = useEditorStore();
  const editorRef = useRef<HTMLDivElement>(null);
  const viewRef = useRef<EditorView | null>(null);
  
  const activeFile = files[activeFileIndex];

  // Binary/Viewer states
  const [imageBase64, setImageBase64] = useState<string | null>(null);
  const [pdfBase64, setPdfBase64] = useState<string | null>(null);
  const [htmlMode, setHtmlMode] = useState<"edit" | "preview">("edit");

  // Load binary contents when active tab changes
  useEffect(() => {
    setImageBase64(null);
    setPdfBase64(null);

    if (!activeFile || activeFile.type !== "file") return;

    const ext = activeFile.name.split(".").pop()?.toLowerCase();
    if (["png", "jpg", "jpeg", "gif", "ico"].includes(ext || "")) {
      ReadFileBase64(activeFile.path).then((data) => {
        setImageBase64(data);
      }).catch(console.error);
    } else if (ext === "pdf") {
      ReadFileBase64(activeFile.path).then((data) => {
        setPdfBase64(data);
      }).catch(console.error);
    }
    
    if (ext === "html" || ext === "htm") {
      setHtmlMode("edit");
    }
  }, [activeFileIndex, activeFile?.path]);

  // CodeMirror instance mounting
  useEffect(() => {
    if (!editorRef.current || !activeFile || activeFile.type !== "file") {
      if (viewRef.current) {
        viewRef.current.destroy();
        viewRef.current = null;
      }
      return;
    }

    const ext = activeFile.name.split(".").pop()?.toLowerCase() || "";
    const isBinary = ["png", "jpg", "jpeg", "gif", "pdf", "ico"].includes(ext);
    if (isBinary || (ext === "html" && htmlMode === "preview")) {
      if (viewRef.current) {
        viewRef.current.destroy();
        viewRef.current = null;
      }
      return;
    }

    const getLanguageExtension = (path: string) => {
      const ext = path.split(".").pop()?.toLowerCase();
      switch (ext) {
        case "js":
        case "jsx":
        case "ts":
        case "tsx":
        case "mjs":
        case "cjs":
        case "mts":
        case "cts":
          return javascript();
        case "go":
          return go();
        case "py":
          return python();
        case "rs":
          return rust();
        case "json":
          return json();
        case "html":
        case "htm":
          return html();
        case "xml":
          return xml();
        case "md":
          return markdown();
        case "java":
          return java();
        case "cpp":
        case "h":
        case "hpp":
        case "cc":
          return cpp();
        case "sql":
          return sql();
        case "php":
          return php();
        case "vue":
          return vue();
        default:
          return [];
      }
    };

    const updateListener = EditorView.updateListener.of((update) => {
      if (update.docChanged) {
        const newContent = update.state.doc.toString();
        setFiles((prev) => {
          const next = [...prev];
          if (next[activeFileIndex]) {
            next[activeFileIndex] = {
              ...next[activeFileIndex],
              content: newContent,
              modified: true,
            };
          }
          return next;
        });
      }
    });

    // Lightweight syntax diagnostics via esbuild (no LSP) — JS/TS only.
    const syntaxLinter = linter(async (view) => {
      const path = activeFile.path;
      const ext = path.split(".").pop()?.toLowerCase() || "";
      if (!["js", "jsx", "ts", "tsx", "mjs", "cjs", "mts", "cts"].includes(ext)) return [];
      try {
        const diags = await CheckSyntax(path, view.state.doc.toString());
        return (Array.isArray(diags) ? diags : []).map((d: any) => ({
          from: Math.max(0, view.state.doc.line(Math.max(1, d.line)).from + Math.max(0, d.column - 1)),
          to: Math.max(0, view.state.doc.line(Math.max(1, d.line)).to),
          severity: "error" as const,
          message: d.message || "Syntax error",
        }));
      } catch {
        return [];
      }
    }, { delay: 350 });

    // Format via backend (biome/prettier when available) — Cmd+Shift+F.
    const formatKeymap = keymap.of([{
      key: "Mod-Shift-f",
      run: (view) => {
        FormatCode(activeFile.path, view.state.doc.toString()).then((formatted) => {
          if (formatted && formatted !== view.state.doc.toString()) {
            view.dispatch({ changes: { from: 0, to: view.state.doc.length, insert: formatted } });
          }
        }).catch(() => {});
        return true;
      },
    }]);

    // Comment toggling — Cmd+/ line comment, Cmd+Shift+/ block comment.
    const commentKeymap = keymap.of([
      { key: "Mod-/", run: toggleComment },
      { key: "Mod-Alt-/", run: toggleBlockComment },
    ]);

    // VSCode-style editing basics: history (undo/redo), Tab indent / Shift-Tab
    // outdent, and bracket indentation (Cmd+] / Cmd+[).
    const basicKeymap = keymap.of([
      indentWithTab,
      { key: "Shift-Tab", run: indentLess },
      { key: "Mod-]", run: indentMore },
      { key: "Mod-[", run: indentLess },
      ...historyKeymap,
    ]);

    const state = EditorState.create({
      doc: activeFile.content,
      extensions: [
        history(),
        keymap.of(defaultKeymap),
        getLanguageExtension(activeFile.path),
        oneDark,
        updateListener,
        EditorView.lineWrapping,
        syntaxLinter,
        formatKeymap,
        commentKeymap,
        basicKeymap,
      ],
    });

    if (viewRef.current) {
      viewRef.current.setState(state);
    } else {
      viewRef.current = new EditorView({
        state,
        parent: editorRef.current,
      });
    }
    setGlobalEditorView(viewRef.current);
  }, [activeFileIndex, activeFile?.path, htmlMode]);

  useEffect(() => {
    return () => {
      if (viewRef.current) {
        viewRef.current.destroy();
        viewRef.current = null;
      }
      setGlobalEditorView(null);
    };
  }, []);

  const closeTab = (idx: number) => {
    setFiles((prev) => {
      const next = [...prev];
      next.splice(idx, 1);
      return next;
    });
    setActiveFileIndex((prev) => {
      if (prev === idx) return Math.max(0, prev - 1);
      if (prev > idx) return prev - 1;
      return prev;
    });
  };

  const isMarkdown = activeFile?.path.endsWith(".md") ?? false;
  const isHtml = !!(activeFile?.path.endsWith(".html") || activeFile?.path.endsWith(".htm"));

  return (
    <div className="flex flex-col h-full bg-[var(--bg-app)] overflow-hidden">
      {/* Top Tab Bar (Clean flush minimal tabs) */}
      <div className="flex items-center bg-[var(--bg-sidebar)] border-b border-[var(--border-default)] shrink-0 overflow-x-auto select-none">
        {files.map((file, i) => (
          <div
            key={file.id}
            onClick={() => setActiveFileIndex(i)}
            className={`flex items-center gap-1.5 px-3 py-1.5 text-xs border-r border-[var(--border-default)] cursor-pointer whitespace-nowrap group shrink-0 ${
              i === activeFileIndex
                ? "bg-[var(--bg-app)] text-[var(--fg-primary)] font-semibold border-b-[2px] border-b-[var(--accent-primary)]"
                : "text-[var(--fg-secondary)] hover:bg-[var(--bg-surface-hover)]"
            }`}
          >
            {file.type === "file" ? (
              getFileIcon(file.name, "size-3.5")
            ) : file.type === "shell" ? (
              <span className="text-cyan-400 font-mono text-[10px]">$&gt;</span>
            ) : (
              <span className="text-blue-400 font-mono text-[10px]">🤖</span>
            )}
            <span>{file.name}</span>
            {file.modified && <span className="text-amber-400 text-[10px]">●</span>}
            <button
              onClick={(e) => {
                e.stopPropagation();
                closeTab(i);
              }}
              className="p-0.5 hover:bg-[var(--bg-surface-active)] rounded-sm ml-1 opacity-0 group-hover:opacity-100 transition-opacity"
            >
              <X className="size-3" />
            </button>
          </div>
        ))}

        {files.length === 0 && (
          <div className="px-4 py-1.5 text-xs text-[var(--fg-tertiary)] italic">
            No tabs open
          </div>
        )}
      </div>

      {/* Editor Content Area */}
      <div className="flex-1 overflow-hidden relative">
        {activeFile ? (
          activeFile.type === "shell" ? (
            <div className="h-full w-full bg-[var(--terminal-background)]">
              <TerminalView sessionId={activeFile.id} isActive={activeFileIndex === useEditorStore.getState().activeFileIndex} />
            </div>
          ) : activeFile.type === "agent" ? (
            <AgentTabCell sessionId={activeFile.id} />
          ) : (
            // STANDARD FILE VIEWER
            <div className="flex flex-col h-full">
              {/* Path metadata and preview toggles */}
              <div className="flex items-center justify-between px-3 py-1 border-b border-[var(--border-default)] bg-[var(--bg-panel)] text-[10px] text-[var(--fg-tertiary)] select-none">
                <span
                  onClick={() => navigator.clipboard.writeText(activeFile.path)}
                  className="truncate cursor-pointer hover:text-[var(--fg-primary)] flex items-center gap-1"
                  title="Copy Path"
                >
                  <Copy className="size-3" />
                  {activeFile.path}
                </span>

                <div className="flex items-center gap-1.5">
                  {isHtml && (
                    <button
                      onClick={() => setHtmlMode(htmlMode === "edit" ? "preview" : "edit")}
                      className="flex items-center gap-1 px-1.5 py-0.5 hover:bg-[var(--bg-surface-hover)] rounded text-[var(--fg-secondary)] hover:text-[var(--fg-primary)]"
                    >
                      <Eye className="size-3" />
                      <span>{htmlMode === "edit" ? "Preview" : "Edit Code"}</span>
                    </button>
                  )}
                </div>
              </div>

              {/* Viewers */}
              <div className="flex-1 overflow-hidden relative bg-[var(--bg-app)]">
                {imageBase64 ? (
                  <div className="h-full w-full flex items-center justify-center p-4">
                    <div className="border border-[var(--border-default)] bg-black/40 p-2 shadow-lg flex flex-col items-center">
                      <img
                        src={`data:image/${activeFile.name.split(".").pop()};base64,${imageBase64}`}
                        className="max-h-[350px] max-w-full object-contain selectable-text"
                        alt={activeFile.name}
                      />
                      <span className="text-[10px] text-[var(--fg-tertiary)] mt-2 font-mono">
                        {activeFile.name}
                      </span>
                    </div>
                  </div>
                ) : pdfBase64 ? (
                  <div className="h-full w-full p-2">
                    <embed
                      src={`data:application/pdf;base64,${pdfBase64}`}
                      type="application/pdf"
                      className="w-full h-full border border-[var(--border-default)]"
                    />
                  </div>
                ) : isHtml && htmlMode === "preview" ? (
                  <iframe
                    srcDoc={activeFile.content}
                    title="HTML Preview"
                    sandbox="allow-scripts"
                    className="w-full h-full bg-white text-black"
                  />
                ) : (
                  // CodeMirror
                  <div ref={editorRef} className="h-full w-full" />
                )}
              </div>
            </div>
          )
        ) : (
          // Welcome background empty state
          <div className="flex flex-col items-center justify-center h-full text-center p-6 select-none text-[var(--fg-tertiary)]">
            <FileCode2 className="size-16 stroke-[1.2] text-[var(--fg-disabled)] mb-3 animate-pulse" />
            <h3 className="text-sm font-semibold text-[var(--fg-secondary)]">Forge Workspace Tab Panel</h3>
            <p className="text-xs max-w-xs mt-1">
              Select files, open terminals, or start assistant chats from the Session Manager in the sidebar.
            </p>
          </div>
        )}
      </div>
    </div>
  );
}

// Group a flat message list into document-style turns: prompt → tool timeline → response.
function buildTurns(messages: any[]): Array<{
  prompt: string;
  toolCalls: any[];
  assistant: { text: string; reasoning: string } | null;
}> {
  const turns: Array<{
    prompt: string;
    toolCalls: any[];
    assistant: { text: string; reasoning: string } | null;
  }> = [];
  let current: (typeof turns)[number] | null = null;

  const flush = () => {
    if (current && (current.prompt || current.toolCalls.length > 0 || current.assistant)) {
      turns.push(current);
    }
    current = null;
  };

  for (const msg of messages || []) {
    const role = msg.role || msg.Role;
    const text = msg.content || msg.Content || "";
    const reasoning = msg.reasoning || msg.Reasoning || "";
    const toolCalls = msg.tool_calls || msg.ToolCalls || [];

    if (role === "user") {
      flush();
      current = { prompt: text, toolCalls: [], assistant: null };
    } else if (role === "tool") {
      if (!current) current = { prompt: "", toolCalls: [], assistant: null };
      const last = [...current.toolCalls].reverse().find((tc) => !tc.result);
      if (last) {
        last.result = text;
      } else {
        current.toolCalls.push({
          id: msg.id || `tool-${turns.length}-${current.toolCalls.length}`,
          name: "tool",
          arguments: "",
          result: text,
        });
      }
    } else if (role === "assistant") {
      if (!current) current = { prompt: "", toolCalls: [], assistant: null };
      for (const tc of toolCalls || []) {
        const fn = tc.function || tc.Function || {};
        current.toolCalls.push({
          id: tc.id || `tc-${turns.length}-${current.toolCalls.length}`,
          name: fn.name || fn.Name || "tool",
          arguments: fn.arguments || fn.Arguments || "{}",
          result: "",
        });
      }
      if (!current.assistant) {
        current.assistant = { text, reasoning };
      } else {
        current.assistant.text += text;
        if (reasoning) current.assistant.reasoning += reasoning;
      }
    }
  }
  flush();
  return turns;
}

// One row in the tool-call timeline.
function ToolCallRow({
  toolCall,
  onToggle,
  expanded,
  running,
}: {
  toolCall: any;
  onToggle: () => void;
  expanded: boolean;
  running?: boolean;
}) {
  const name = toolCall.name || "tool";
  let argsText = toolCall.arguments || "{}";
  if (typeof argsText !== "string") argsText = JSON.stringify(argsText);
  let args: any = null;
  try { args = JSON.parse(argsText); } catch { /* keep raw */ }

  let title = name;
  if (args) {
    if (args.pattern) title = `${name} ${typeof args.pattern === "string" ? args.pattern : JSON.stringify(args.pattern)}`;
    else if (args.query) title = `${name} "${args.query}"`;
    else if (args.path) title = `${name} ${String(args.path).split("/").pop()}`;
    else if (args.command) title = `${name} ${String(args.command).slice(0, 60)}`;
  }
  const hasResult = !!toolCall.result;

  return (
    <div className="rounded-lg border border-[var(--border-default)] bg-[var(--bg-panel)] overflow-hidden">
      <button
        onClick={onToggle}
        className="w-full flex items-center gap-2.5 px-2.5 py-[7px] text-[13px] text-[var(--fg-secondary)] hover:bg-[var(--bg-surface-hover)] cursor-pointer text-left"
      >
        <Search className="size-3.5 text-[var(--accent-primary)] shrink-0" />
        <span className="font-medium truncate flex-1">{title}</span>
        {running ? (
          <span className="flex items-center gap-1 text-[11px] text-purple-400 font-mono shrink-0">
            <span className="inline-block size-1.5 rounded-full bg-purple-400 animate-pulse" />
            running
          </span>
        ) : hasResult ? (
          <span className="text-[11px] text-emerald-400 font-mono shrink-0">✓ done</span>
        ) : null}
        {expanded ? <ChevronDown className="size-3 text-[var(--fg-tertiary)] shrink-0" /> : <ChevronRight className="size-3 text-[var(--fg-tertiary)] shrink-0" />}
      </button>
      {(expanded || hasResult) && (
        <div className="px-3 pb-2.5 pt-1.5 border-t border-[var(--border-default)] space-y-1.5">
          <pre className="text-[12px] font-mono text-[var(--fg-tertiary)] whitespace-pre-wrap break-all">
            {args ? JSON.stringify(args, null, 2) : argsText}
          </pre>
          {hasResult && (
            <pre className="text-[12px] font-mono text-[var(--fg-secondary)] whitespace-pre-wrap break-all bg-black/20 rounded p-2 max-h-60 overflow-y-auto">
              {toolCall.result}
            </pre>
          )}
        </div>
      )}
    </div>
  );
}

// Embedded AI Agent chat client inside tabs
function AgentTabCell({ sessionId }: { sessionId: string }) {
  const { toast } = useToast();
  const [inputText, setInputText] = useState("");
  const [sessions, setSessions] = useState<any[]>([]);
  const [profiles, setProfiles] = useState<any[]>([]);
  const [activeModel, setActiveModel] = useState<string>("");
  const [showModelPicker, setShowModelPicker] = useState(false);
  const [expandedReasoning, setExpandedReasoning] = useState<Record<string, boolean>>({});
  const [expandedToolCalls, setExpandedToolCalls] = useState<Record<string, boolean>>({});
  const chatEndRef = useRef<HTMLDivElement>(null);
  const scrollContainerRef = useRef<HTMLDivElement>(null);

  const session = useMemo(() => {
    return sessions.find((s) => s.id === sessionId) || null;
  }, [sessions, sessionId]);

  useEffect(() => {
    loadAgent();
    loadProfiles();
    // Real-time updates via agent:updated; polling as a fallback.
    const unsubscribe = EventsOn("agent:updated", () => loadAgent());
    const timer = setInterval(loadAgent, 3000);
    return () => {
      clearInterval(timer);
      if (typeof unsubscribe === "function") unsubscribe();
    };
  }, [sessionId]);

  useEffect(() => {
    if (chatEndRef.current && scrollContainerRef.current) {
      scrollContainerRef.current.scrollTop = scrollContainerRef.current.scrollHeight;
    }
  }, [session?.messages]);

  async function loadAgent() {
    try {
      const list = await ListAgentSessions();
      setSessions(Array.isArray(list) ? list : []);
    } catch { /* ignore */ }
  }

  async function loadProfiles() {
    try {
      const list = await GetProviderProfiles();
      setProfiles(Array.isArray(list) ? list : []);
      const cfg = await GetLLMConfig();
      if (cfg && cfg.model) {
        setActiveModel(cfg.model);
      }
    } catch { /* ignore */ }
  }

  async function handleSelectModel(providerId: string, model: string) {
    setActiveModel(model);
    setShowModelPicker(false);
    try {
      await SetActiveModel(providerId, model);
    } catch { /* ignore */ }
  }

  async function handleSendMessage() {
    if (!inputText.trim()) return;
    const text = inputText;
    setInputText("");
    try {
      await SendAgentMessage(sessionId, text, []);
    } catch (err) {
      console.error(err);
    }
  }

  async function handleApproval(approve: boolean, autoAll = false) {
    try {
      await RespondAgentApproval(sessionId, approve, autoAll);
    } catch (err) {
      console.error(err);
    }
  }

  return (
    <div className="flex flex-col h-full w-full bg-[var(--bg-app)] relative overflow-hidden select-text text-xs">
      {/* Model header */}
      <div className="flex items-center justify-between px-3 py-1.5 bg-[var(--bg-sidebar)] border-b border-[var(--border-default)] text-[10px] text-[var(--fg-secondary)] shrink-0 select-none">
        <span className="font-semibold uppercase tracking-wider text-[var(--fg-tertiary)]">AI Assistant</span>
        <div className="flex items-center gap-1.5">
          {session?.state === "thinking" && (
            <span className="flex items-center gap-1 text-[10px] text-purple-400 font-mono">
              <span className="inline-block size-1.5 rounded-full bg-purple-400 animate-pulse" />
              thinking…
            </span>
          )}
          {(session?.token_usage?.total_tokens ?? session?.token_usage?.TotalTokens ?? 0) > 0 && (
            <TokenUsageBadge usage={session.token_usage} />
          )}
          <button
            onClick={() => setShowModelPicker(!showModelPicker)}
            className="px-2 py-0.5 bg-[var(--bg-panel)] hover:bg-[var(--bg-surface-hover)] border border-[var(--border-default)] text-[10px] rounded flex items-center space-x-1 cursor-pointer font-mono"
          >
            <Cpu className="size-3 text-purple-400" />
            <span>{activeModel || "Model"}</span>
            {showModelPicker ? <ChevronUp className="size-3" /> : <ChevronDown className="size-3" />}
          </button>
        </div>
      </div>

      {showModelPicker && (
        <div className="absolute top-8 right-3 z-30 w-64 bg-[var(--bg-sidebar)] border border-[var(--border-default)] shadow-xl p-2 select-none text-[11px] max-h-72 overflow-y-auto">
          <div className="font-bold text-[9px] text-[var(--fg-tertiary)] uppercase tracking-wider mb-1.5 px-2">Models</div>
          {profiles.map((p) => {
            const pid = p.id || p.Id || p.name || p.Name;
            const models = p.selected_models || p.SelectedModels || p.available_models || p.AvailableModels || [];
            if (models.length === 0) {
              return (
                <div key={pid} className="px-2 py-1 text-[var(--fg-tertiary)] font-mono text-[10px]">
                  {p.name || p.Name} — no models
                </div>
              );
            }
            return (
              <div key={pid} className="mb-1">
                <div className="px-2 py-0.5 text-[9px] uppercase tracking-wider text-[var(--fg-tertiary)] font-semibold">
                  {p.name || p.Name}
                </div>
                {models.map((m: string) => (
                  <button
                    key={m}
                    onClick={() => handleSelectModel(pid, m)}
                    className={`w-full text-left px-2 py-1 hover:bg-[var(--bg-panel)] rounded text-[var(--fg-primary)] truncate font-mono text-[10px] cursor-pointer ${
                      activeModel === m ? "bg-[var(--bg-surface-active)] text-white" : ""
                    }`}
                  >
                    <span className="mr-1.5">{activeModel === m ? "●" : "○"}</span>
                    {m}
                  </button>
                ))}
              </div>
            );
          })}
        </div>
      )}

      {/* Messages */}
      <div ref={scrollContainerRef} className="flex-1 overflow-y-auto p-4 space-y-4">
        {!session || !session.messages || session.messages.length === 0 ? (
          <div className="flex flex-col items-center justify-center h-full text-center space-y-2 text-[var(--fg-tertiary)] select-none">
            <Brain className="size-12 stroke-[1.2] text-[var(--fg-disabled)] animate-pulse" />
            <h3 className="text-sm font-semibold text-[var(--fg-secondary)]">AI Assistant Session</h3>
            <p className="text-xs max-w-xs mt-1">
              Ask coding questions, draft features, or request file changes using natural language.
            </p>
          </div>
        ) : (
          buildTurns(session.messages || []).map((turn, ti) => (
            <div key={ti} className="space-y-3">
              {/* Prompt card */}
              {turn.prompt && (
                <div className="group relative rounded-xl border border-[var(--border-default)] bg-[var(--bg-panel)] px-4 py-3 text-[15px] leading-relaxed text-[var(--fg-primary)] selectable-text">
                  {turn.prompt}
                  <button
                    onClick={() => {
                      navigator.clipboard.writeText(turn.prompt).then(() => toast("Copied to clipboard"));
                    }}
                    className="absolute top-2 right-2 p-1 rounded hover:bg-[var(--bg-surface-hover)] text-[var(--fg-tertiary)] hover:text-white opacity-0 group-hover:opacity-100 transition-opacity cursor-pointer"
                    title="Copy"
                  >
                    <Copy className="size-3.5" />
                  </button>
                </div>
              )}

              {/* Tool call timeline */}
              {turn.toolCalls.length > 0 && (
                <div className="space-y-0.5">
                  {turn.toolCalls.map((tc, tci) => {
                    const isLastWithoutResult =
                      tci === turn.toolCalls.length - 1 &&
                      !tc.result &&
                      (session.state === "thinking" || session.state === "executing" || session.state === "awaiting_approval");
                    return (
                      <ToolCallRow
                        key={`${ti}-${tci}`}
                        toolCall={tc}
                        running={isLastWithoutResult}
                        onToggle={() => {
                          const key = tc.id || `${ti}-${tci}`;
                          setExpandedToolCalls((p) => ({ ...p, [`tc-${key}`]: !p[`tc-${key}`] }));
                        }}
                        expanded={!!expandedToolCalls[`tc-${tc.id || `${ti}-${tci}`}`]}
                      />
                    );
                  })}
                </div>
              )}

              {/* Assistant response */}
              {turn.assistant && (
                <div className="space-y-2">
                  {turn.assistant.reasoning && (
                    <div className="rounded-lg border border-[var(--border-default)] overflow-hidden">
                      <button
                        onClick={() => setExpandedReasoning((p) => ({ ...p, [`r-${ti}`]: !p[`r-${ti}`] }))}
                        className="w-full flex items-center gap-2 px-3 py-1.5 text-[12px] font-medium text-[var(--fg-tertiary)] hover:bg-[var(--bg-surface-hover)] cursor-pointer"
                      >
                        {(expandedReasoning[`r-${ti}`] ?? true) ? <ChevronDown className="size-3.5" /> : <ChevronRight className="size-3.5" />}
                        <Brain className="size-3.5 text-purple-400" />
                        <span>Thinking</span>
                      </button>
                      {(expandedReasoning[`r-${ti}`] ?? true) && (
                        <div className="px-3 pb-2.5 pt-1.5 text-[13px] leading-relaxed text-[var(--fg-secondary)] whitespace-pre-wrap border-t border-[var(--border-default)] font-mono">
                          {turn.assistant.reasoning}
                        </div>
                      )}
                    </div>
                  )}

                  {turn.assistant?.text && (
                    <div className="group relative">
                      <button
                        onClick={() => {
                          navigator.clipboard.writeText(turn.assistant!.text).then(() => toast("Copied to clipboard"));
                        }}
                        className="absolute top-0 right-0 p-1 rounded hover:bg-[var(--bg-surface-hover)] text-[var(--fg-tertiary)] hover:text-white opacity-0 group-hover:opacity-100 transition-opacity cursor-pointer"
                        title="Copy"
                      >
                        <Copy className="size-3.5" />
                      </button>
                      <div
                        className="text-[15px] leading-[1.7] text-[var(--fg-primary)] markdown-body"
                        dangerouslySetInnerHTML={{ __html: renderMarkdown(turn.assistant.text) }}
                      />
                    </div>
                  )}
                </div>
              )}
            </div>
          ))
        )}

        {/* Pending tools card */}
        {session?.pending_tool && (
          <div className="border border-[var(--border-default)] bg-[var(--bg-panel)] p-3 space-y-3 shadow-md max-w-md">
            <div className="flex items-center space-x-2 text-xs font-semibold text-amber-400 select-none">
              <Shield className="size-4" />
              <span>Permission Request</span>
            </div>
            <div className="text-xs font-mono bg-black/30 p-2 border border-[var(--border-default)] text-[var(--fg-primary)] overflow-x-auto selectable-text">
              {JSON.stringify(session.pending_tool)}
            </div>
            <div className="flex items-center justify-end space-x-2 pt-1 select-none">
              <button
                onClick={() => handleApproval(false)}
                className="px-2.5 py-1 text-xs text-[var(--fg-secondary)] hover:text-white border border-[var(--border-default)] rounded hover:bg-[var(--bg-surface-hover)] cursor-pointer"
              >
                Deny
              </button>
              <button
                onClick={() => handleApproval(true)}
                className="px-3 py-1 text-xs font-semibold bg-amber-500 hover:bg-amber-600 text-black rounded flex items-center space-x-1 cursor-pointer"
              >
                <Check className="size-3.5" />
                <span>Approve</span>
              </button>
            </div>
          </div>
        )}
        <div ref={chatEndRef} />
      </div>

      {/* Input container */}
      <div className="p-3 border-t border-[var(--border-default)] bg-[var(--bg-sidebar)] shrink-0 select-none">
        <textarea
          value={inputText}
          onChange={(e) => setInputText(e.target.value)}
          onKeyDown={(e) => {
            if (e.key === "Tab" && e.shiftKey) {
              if (session?.pending_tool) {
                e.preventDefault();
                handleApproval(true);
              }
            } else if (e.key === "Enter" && !e.shiftKey) {
              e.preventDefault();
              handleSendMessage();
            }
          }}
          placeholder="Ask anything..."
          rows={2}
          className="w-full bg-[var(--bg-panel)] border border-[var(--border-default)] p-2 text-xs text-[var(--fg-primary)] focus:outline-none focus:border-[var(--accent-primary)] resize-none"
        />

        <div className="flex items-center justify-end pt-1">
          <button
            onClick={handleSendMessage}
            className="px-3 py-1 bg-[var(--accent-primary)] hover:bg-[var(--accent-hover)] text-black text-xs font-semibold flex items-center space-x-1 cursor-pointer"
          >
            <Send className="size-3.5" />
            <span>Send</span>
          </button>
        </div>
      </div>
    </div>
  );
}
