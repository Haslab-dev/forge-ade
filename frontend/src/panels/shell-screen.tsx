import React, { useState, useEffect, useLayoutEffect, useMemo, useRef, useCallback } from "react";
import {
  IconTerminal2,
  IconRobot,
  IconPlus,
  IconX,
  IconSparkles,
  IconMaximize,
  IconColumns,
  IconGridDots,
  IconCpu,
  IconChevronDown,
  IconChevronUp,
  IconChevronRight,
  IconSend,
  IconBrain,
  IconShield,
  IconCopy,
  IconCheck,
  IconSquare,
  IconSearch,
  IconArrowDown,
  IconArrowUp,
  IconBolt,
} from "@tabler/icons-react";
import { TerminalView } from "../components/terminal-view";
import { cn } from "../lib/utils";
import {
  ListAgentSessions,
  CreateAgentSession,
  ListAgentDefinitions,
  CreateAgentSessionFromDefinition,
  SendAgentMessage,
  RespondAgentApproval,
  ToggleAgentTask,
  SearchFilename,
  GetProviderProfiles,
  GetLLMConfig,
  SetActiveModel,
  EventsOn,
} from "../lib/wails";
import { useSessionLayoutStore } from "../hooks/store";
import { marked } from "marked";
import { useToast } from "../lib/toast";

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
        <IconArrowDown className="size-2.5" />
        {inTok.toLocaleString()}
      </span>
      <span className="flex items-center gap-0.5">
        <IconArrowUp className="size-2.5" />
        {outTok.toLocaleString()}
      </span>
      <span className="flex items-center gap-0.5" title="Cached tokens">
        <IconBolt className="size-2.5" />
        {cached.toLocaleString()}
      </span>
    </span>
  );
}

interface UnifiedSession {
  id: string;
  name: string;
  type: "shell" | "agent";
  role_filter?: string;
  state?: string;
  messages?: any[];
  tasks?: any[];
  token_usage?: any;
  auto_approve?: boolean;
  pending_tool?: any;
  project_name?: string;
  custom_prompt?: string;
  custom_rules?: string;
}

interface ShellScreenProps {
  sessions: any[];
  onCreateShell: () => void;
  onCloseSession: (id: string) => void;
  onStopSession?: (id: string) => void;
  onRenameSession: (id: string, name: string) => void;
  initialSessionId?: string | null;
  projectFolder?: string;
}

export function ShellScreen({
  sessions: shellSessions,
  onCreateShell,
  onCloseSession,
  onStopSession,
  onRenameSession,
  initialSessionId,
  projectFolder,
}: ShellScreenProps) {
  const [agentSessions, setAgentSessions] = useState<any[]>([]);
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [newSessionType, setNewSessionType] = useState<"shell" | "agent">("agent");
  const [newSessionName, setNewSessionName] = useState("");
  const [newSessionRole, setNewSessionRole] = useState<"coding" | "planning" | "research" | "custom">("coding");

  const {
    layoutMode,
    closedViewSessionIds,
    selectedSessionId,
    panelShares,
    setLayoutMode,
    closeView,
    reopenView,
    setSelectedSessionId,
    setPanelShare,
  } = useSessionLayoutStore();

  const sessionRefs = useRef<Record<string, HTMLDivElement | null>>({});

  useEffect(() => {
    loadAgents();
    // Real-time updates via agent:updated events; polling as a fallback.
    const unsubscribe = EventsOn("agent:updated", () => {
      loadAgents();
    });
    const timer = setInterval(loadAgents, 3000);
    return () => {
      clearInterval(timer);
      if (typeof unsubscribe === "function") unsubscribe();
    };
  }, []);

  async function loadAgents() {
    try {
      const list = await ListAgentSessions();
      setAgentSessions(Array.isArray(list) ? list : []);
    } catch { /* ignore */ }
  }

  const allSessions: UnifiedSession[] = useMemo(() => {
    const list: UnifiedSession[] = [];
    for (const s of shellSessions) {
      list.push({ id: s.id, name: s.name, type: "shell" });
    }
    for (const a of agentSessions) {
      list.push({
        id: a.id,
        name: a.name,
        type: "agent",
        role_filter: a.role_filter || a.RoleFilter || "coding",
        state: a.state || a.State || "idle",
        messages: a.messages || a.Messages || [],
        tasks: a.tasks || a.Tasks || [],
        token_usage: a.token_usage || a.TokenUsage || {},
        auto_approve: a.auto_approve || a.AutoApprove || false,
        pending_tool: a.pending_tool || a.PendingTool || null,
        project_name: a.project_name || a.ProjectName || "",
        custom_prompt: a.custom_prompt || a.CustomPrompt || "",
        custom_rules: a.custom_rules || a.CustomRules || "",
      });
    }
    return list;
  }, [shellSessions, agentSessions]);

  const visibleSessions = useMemo(() => {
    return allSessions.filter((s) => !closedViewSessionIds.includes(s.id));
  }, [allSessions, closedViewSessionIds]);

  useEffect(() => {
    if (visibleSessions.length > 0 && (!selectedSessionId || !visibleSessions.find((s) => s.id === selectedSessionId))) {
      setSelectedSessionId(visibleSessions[0].id);
    }
  }, [visibleSessions, selectedSessionId]);

  useEffect(() => {
    if (initialSessionId) {
      reopenView(initialSessionId);
      setSelectedSessionId(initialSessionId);
    }
  }, [initialSessionId]);

  async function handleCreateAgent() {
    try {
      const name = newSessionName.trim() || `Agent (${newSessionRole})`;
      const created: any = await CreateAgentSession(name, newSessionRole, projectFolder ?? "");
      setShowCreateModal(false);
      setNewSessionName("");
      if (created && created.id) {
        reopenView(created.id);
        setSelectedSessionId(created.id);
      }
      loadAgents();
    } catch (err) {
      console.error("Failed to create agent session:", err);
    }
  }

  function handleClosePanelTab(id: string, e?: React.MouseEvent) {
    if (e) e.stopPropagation();
    closeView(id);
    onCloseSession(id);
    const remaining = visibleSessions.filter((s) => s.id !== id);
    if (remaining.length > 0) {
      setSelectedSessionId(remaining[0].id);
    } else {
      setSelectedSessionId(null);
    }
  }

  const handleAgentLaunched = useCallback((newSessionId: string) => {
    reopenView(newSessionId);
    setSelectedSessionId(newSessionId);
    loadAgents();
  }, []);

  const activeSessionObj = useMemo(() => {
    return visibleSessions.find((s) => s.id === selectedSessionId) || visibleSessions[0] || null;
  }, [visibleSessions, selectedSessionId]);

  return (
    <div className="flex flex-col h-full bg-[var(--bg-app)] text-[var(--fg-primary)] overflow-hidden">
      {/* Top Tabs panel */}
      <div className="flex items-center justify-between border-b border-[var(--border-default)] px-3 py-1 bg-[var(--bg-sidebar)] shrink-0 gap-2 select-none">
        <div className="flex items-center space-x-1 overflow-x-auto flex-1">
          {visibleSessions.map((s) => (
            <div
              key={s.id}
              onClick={() => setSelectedSessionId(s.id)}
              className={cn(
                "px-3 py-1.5 border-r border-[var(--border-default)] text-xs flex items-center space-x-2 cursor-pointer transition-colors shrink-0",
                selectedSessionId === s.id
                  ? "bg-[var(--bg-app)] text-[var(--fg-primary)] font-semibold border-b-[2px] border-b-[var(--accent-primary)]"
                  : "text-[var(--fg-secondary)] hover:bg-[var(--bg-surface-hover)]"
              )}
            >
              {s.type === "shell" ? (
                <IconTerminal2 className="size-3.5 text-cyan-400" />
              ) : (
                <IconRobot className="size-3.5 text-blue-400" />
              )}
              <span className="truncate max-w-[130px]">{s.name}</span>
              <button
                onClick={(e) => handleClosePanelTab(s.id, e)}
                className="hover:bg-[var(--bg-surface-active)] rounded-sm p-0.5"
              >
                <IconX className="size-3" />
              </button>
            </div>
          ))}

          <button
            onClick={() => setShowCreateModal(true)}
            className="px-2 py-1 hover:bg-[var(--bg-surface-hover)] rounded border border-[var(--border-default)] text-[var(--fg-secondary)] hover:text-[var(--fg-primary)] text-xs flex items-center space-x-1 cursor-pointer"
          >
            <IconPlus className="size-3" />
            <span>New Session</span>
          </button>
        </div>

        {/* Layout controls */}
        <div className="flex items-center space-x-0.5 bg-[var(--bg-panel)] p-0.5 border border-[var(--border-default)] text-xs">
          <button
            onClick={() => setLayoutMode("single")}
            className={cn(
              "p-1 rounded cursor-pointer",
              layoutMode === "single" ? "bg-[var(--bg-surface-active)] text-white" : "text-gray-400 hover:text-white"
            )}
            title="Single panel"
          >
            <IconMaximize className="size-3.5" />
          </button>
          <button
            onClick={() => setLayoutMode("horizontal")}
            className={cn(
              "p-1 rounded cursor-pointer",
              layoutMode === "horizontal" ? "bg-[var(--bg-surface-active)] text-white" : "text-gray-400 hover:text-white"
            )}
            title="Split side-by-side"
          >
            <IconColumns className="size-3.5" />
          </button>
          <button
            onClick={() => setLayoutMode("grid")}
            className={cn(
              "p-1 rounded cursor-pointer",
              layoutMode === "grid" ? "bg-[var(--bg-surface-active)] text-white" : "text-gray-400 hover:text-white"
            )}
            title="Grid layout"
          >
            <IconGridDots className="size-3.5" />
          </button>
        </div>
      </div>

      {/* Main sessions body */}
      <div className="flex-1 overflow-hidden relative">
        {visibleSessions.length === 0 ? (
          <div className="flex flex-col items-center justify-center h-full text-center space-y-3 text-[var(--fg-tertiary)] select-none">
            <IconSparkles className="size-10 text-[var(--fg-disabled)] animate-pulse" />
            <h3 className="text-xs font-semibold text-[var(--fg-secondary)]">No active sessions</h3>
            <button
              onClick={() => setShowCreateModal(true)}
              className="px-3 py-1.5 bg-[var(--accent-primary)] hover:bg-[var(--accent-hover)] text-black text-xs font-semibold flex items-center space-x-1.5 shadow"
            >
              <IconPlus className="size-3.5" />
              <span>Launch session</span>
            </button>
          </div>
        ) : layoutMode === "single" ? (
          activeSessionObj ? (
            <SessionCell
              session={activeSessionObj}
              isFocused={true}
              onClose={() => handleClosePanelTab(activeSessionObj.id)}
              projectFolder={projectFolder}
              onAgentLaunched={handleAgentLaunched}
            />
          ) : null
        ) : layoutMode === "horizontal" ? (
          <div className="flex flex-row h-full w-full overflow-hidden select-none">
            {visibleSessions.slice(0, 3).map((s, idx) => {
              const share = panelShares[s.id] || 1;
              return (
                <React.Fragment key={s.id}>
                  <div
                    onClick={() => setSelectedSessionId(s.id)}
                    style={{ flex: `${share} 1 0%` }}
                    className={cn(
                      "h-full overflow-hidden border-r border-[var(--border-default)]",
                      selectedSessionId === s.id && "ring-1 ring-[var(--accent-primary)]/50 z-10"
                    )}
                  >
                    <SessionCell
                      session={s}
                      isFocused={selectedSessionId === s.id}
                      onClose={() => handleClosePanelTab(s.id)}
                      projectFolder={projectFolder}
                      onAgentLaunched={handleAgentLaunched}
                    />
                  </div>
                </React.Fragment>
              );
            })}
          </div>
        ) : (
          <div className="grid grid-cols-2 grid-rows-2 h-full w-full gap-1 p-1 bg-black/20 overflow-hidden">
            {visibleSessions.slice(0, 4).map((s) => (
              <div
                key={s.id}
                onClick={() => setSelectedSessionId(s.id)}
                className={cn(
                  "h-full w-full overflow-hidden border border-[var(--border-default)]",
                  selectedSessionId === s.id && "ring-1 ring-[var(--accent-primary)] z-10"
                )}
              >
                <SessionCell
                  session={s}
                  isFocused={selectedSessionId === s.id}
                  onClose={() => handleClosePanelTab(s.id)}
                  projectFolder={projectFolder}
                  onAgentLaunched={handleAgentLaunched}
                />
              </div>
            ))}
          </div>
        )}
      </div>

      {/* Launch session modal */}
      {showCreateModal && (
        <div className="fixed inset-0 bg-black/60 backdrop-blur-sm z-50 flex items-center justify-center p-4">
          <div className="bg-[var(--bg-sidebar)] border border-[var(--border-default)] w-full max-w-sm p-4 space-y-4">
            <div className="flex items-center justify-between pb-2 border-b border-[var(--border-default)]">
              <span className="font-bold text-sm text-[var(--fg-primary)]">Launch Session</span>
              <button onClick={() => setShowCreateModal(false)} className="text-[var(--fg-tertiary)] hover:text-white cursor-pointer">
                <IconX className="size-4" />
              </button>
            </div>

            <div className="grid grid-cols-2 gap-2 text-xs font-semibold">
              <button
                onClick={() => setNewSessionType("shell")}
                className={cn(
                  "p-3 border flex flex-col items-center justify-center space-y-1 transition-all cursor-pointer",
                  newSessionType === "shell"
                    ? "border-[var(--accent-primary)] bg-[var(--accent-primary)]/10 text-[var(--fg-primary)]"
                    : "border-[var(--border-default)] bg-[var(--bg-panel)] text-[var(--fg-secondary)]"
                )}
              >
                <IconTerminal2 className="size-6 text-cyan-400" />
                <span>Shell Terminal</span>
              </button>

              <button
                onClick={() => setNewSessionType("agent")}
                className={cn(
                  "p-3 border flex flex-col items-center justify-center space-y-1 transition-all cursor-pointer",
                  newSessionType === "agent"
                    ? "border-[var(--accent-primary)] bg-[var(--accent-primary)]/10 text-[var(--fg-primary)]"
                    : "border-[var(--border-default)] bg-[var(--bg-panel)] text-[var(--fg-secondary)]"
                )}
              >
                <IconRobot className="size-6 text-blue-400" />
                <span>AI Agent</span>
              </button>
            </div>

            {newSessionType === "agent" && (
              <div className="space-y-2 text-xs">
                <label className="text-[var(--fg-secondary)] block font-medium">Agent Role Filter</label>
                <select
                  value={newSessionRole}
                  onChange={(e: any) => setNewSessionRole(e.target.value)}
                  className="w-full bg-[var(--bg-panel)] border border-[var(--border-default)] px-3 py-1.5 text-[var(--fg-primary)] focus:outline-none"
                >
                  <option value="coding">Coding Agent</option>
                  <option value="planning">Planning Agent</option>
                  <option value="research">Research Agent</option>
                  <option value="custom">Custom Agent</option>
                </select>

                <label className="text-[var(--fg-secondary)] block font-medium pt-1">Session Name (Optional)</label>
                <input
                  type="text"
                  value={newSessionName}
                  onChange={(e) => setNewSessionName(e.target.value)}
                  placeholder="Coding Agent Session"
                  className="w-full bg-[var(--bg-panel)] border border-[var(--border-default)] px-3 py-1.5 text-[var(--fg-primary)] focus:outline-none focus:border-[var(--accent-primary)]"
                />
              </div>
            )}

            <div className="flex items-center justify-end space-x-2 pt-2 border-t border-[var(--border-default)]">
              <button
                onClick={() => setShowCreateModal(false)}
                className="px-3 py-1.5 text-xs text-[var(--fg-secondary)] hover:text-white cursor-pointer"
              >
                Cancel
              </button>
              <button
                onClick={() => {
                  if (newSessionType === "shell") {
                    setShowCreateModal(false);
                    onCreateShell();
                  } else {
                    handleCreateAgent();
                  }
                }}
                className="px-4 py-1.5 bg-[var(--accent-primary)] hover:bg-[var(--accent-hover)] text-black text-xs font-semibold cursor-pointer"
              >
                Launch
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

function SessionCell({
  session,
  isFocused,
  onClose,
  projectFolder,
  onAgentLaunched,
}: {
  session: UnifiedSession;
  isFocused: boolean;
  onClose: () => void;
  projectFolder?: string;
  onAgentLaunched?: (sessionId: string) => void;
}) {
  if (session.type === "shell") {
    return (
      <div className="flex flex-col h-full w-full bg-[var(--terminal-background)] relative">
        {/* Terminal Header */}
        <div className="flex items-center justify-between px-3 py-1.5 bg-[var(--bg-sidebar)] border-b border-[var(--border-default)] text-xs text-[var(--fg-secondary)] select-none shrink-0">
          <div className="flex items-center space-x-1.5">
            <IconTerminal2 className="size-3.5 text-cyan-400" />
            <span className="font-semibold">{session.name}</span>
          </div>
          <button onClick={onClose} className="hover:text-white cursor-pointer">
            <IconX className="size-3.5" />
          </button>
        </div>
        {/* Terminal frame */}
        <div className="flex-1 min-h-0 min-w-0 overflow-hidden">
          <TerminalView sessionId={session.id} isActive={isFocused} />
        </div>
      </div>
    );
  }

  return <AgentCell session={session} isFocused={isFocused} onClose={onClose} projectFolder={projectFolder} onAgentLaunched={onAgentLaunched} />;
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
      // Attach the tool result to the most recent timeline entry without a
      // result (the assistant-declared tool call it answers).
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
      // Attach tool call declarations from the assistant message.
      for (const tc of toolCalls || []) {
        const fn = tc.function || tc.Function || {};
        current.toolCalls.push({
          id: tc.id || `tc-${turns.length}-${current.toolCalls.length}`,
          name: fn.name || fn.Name || "tool",
          arguments: fn.arguments || fn.Arguments || "{}",
          result: "",
        });
      }
      // Merge streaming chunks of the same assistant message.
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

  // Build a readable title: "Search \"query\"" / "Read element.rs" / etc.
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
        <IconSearch className="size-3.5 text-[var(--accent-primary)] shrink-0" />
        <span className="font-medium truncate flex-1">{title}</span>
        {running ? (
          <span className="flex items-center gap-1 text-[11px] text-purple-400 font-mono shrink-0">
            <span className="inline-block size-1.5 rounded-full bg-purple-400 animate-pulse" />
            running
          </span>
        ) : hasResult ? (
          <span className="text-[11px] text-emerald-400 font-mono shrink-0">✓ done</span>
        ) : null}
        {expanded ? <IconChevronDown className="size-3 text-[var(--fg-tertiary)] shrink-0" /> : <IconChevronRight className="size-3 text-[var(--fg-tertiary)] shrink-0" />}
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

function AgentCell({
  session,
  isFocused,
  onClose,
  projectFolder,
  onAgentLaunched,
}: {
  session: UnifiedSession;
  isFocused: boolean;
  onClose: () => void;
  projectFolder?: string;
  onAgentLaunched?: (sessionId: string) => void;
}) {
  const { toast } = useToast();
  const [inputText, setInputText] = useState("");
  const [showMentionMenu, setShowMentionMenu] = useState(false);
  const [mentionResults, setMentionResults] = useState<string[]>([]);
  const [profiles, setProfiles] = useState<any[]>([]);
  const [activeModel, setActiveModelName] = useState<string>("");
  const [showModelPicker, setShowModelPicker] = useState(false);
  const [agentDefs, setAgentDefs] = useState<any[]>([]);
  const [showAgentPicker, setShowAgentPicker] = useState(false);
  const [activeAgentName, setActiveAgentName] = useState<string>(session.name || "");
  const [expandedReasoning, setExpandedReasoning] = useState<Record<string, boolean>>({});
  const [expandedToolCalls, setExpandedToolCalls] = useState<Record<string, boolean>>({});
  const chatEndRef = useRef<HTMLDivElement>(null);
  const scrollContainerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    loadProfiles();
    loadAgentDefs();
    // Refresh when provider/agent config changes in global settings so the
    // model list and active model stay in sync.
    const unsubscribe = EventsOn("agent:config:changed", () => {
      loadProfiles();
      loadAgentDefs();
    });
    return () => {
      if (typeof unsubscribe === "function") unsubscribe();
    };
  }, []);

  useEffect(() => {
    if (chatEndRef.current && scrollContainerRef.current) {
      scrollContainerRef.current.scrollTop = scrollContainerRef.current.scrollHeight;
    }
  }, [session.messages]);

  async function loadProfiles() {
    try {
      const list = await GetProviderProfiles();
      setProfiles(Array.isArray(list) ? list : []);
      const cfg = await GetLLMConfig();
      if (cfg && cfg.model) {
        setActiveModelName(cfg.model);
      }
    } catch { /* ignore */ }
  }

  async function handleSelectModel(providerId: string, model: string) {
    setActiveModelName(model);
    setShowModelPicker(false);
    try {
      await SetActiveModel(providerId, model);
    } catch { /* ignore */ }
  }

  async function handleLaunchAgentDef(def: any) {
    setShowAgentPicker(false);
    try {
      const created: any = await CreateAgentSessionFromDefinition(def.id || def.ID, projectFolder ?? "");
      if (created && created.id) {
        onAgentLaunched?.(created.id);
      }
    } catch (err) {
      console.error("Failed to launch agent:", err);
    }
  }

  async function loadAgentDefs() {
    try {
      const list = await ListAgentDefinitions();
      setAgentDefs(Array.isArray(list) ? list : []);
    } catch { /* ignore */ }
  }

  async function handleSendMessage() {
    if (!inputText.trim()) return;
    const text = inputText;
    setInputText("");
    try {
      await SendAgentMessage(session.id, text, []);
    } catch (err) {
      console.error(err);
    }
  }

  async function handleApproval(approve: boolean, autoAll = false) {
    try {
      await RespondAgentApproval(session.id, approve, autoAll);
    } catch (err) {
      console.error(err);
    }
  }

  async function handleInputChange(e: React.ChangeEvent<HTMLTextAreaElement>) {
    const val = e.target.value;
    setInputText(val);
    const atIndex = val.lastIndexOf("@");
    if (atIndex !== -1 && atIndex >= val.length - 20) {
      const query = val.slice(atIndex + 1);
      try {
        const results = await SearchFilename(query, 8);
        setMentionResults(results.map((r: any) => r.path ?? r.Path));
        setShowMentionMenu(true);
      } catch {
        setMentionResults([]);
      }
    } else {
      setShowMentionMenu(false);
    }
  }

  return (
    <div className="flex flex-col h-full w-full bg-[var(--bg-app)] relative overflow-hidden">
      {/* Header bar */}
      <div className="flex items-center justify-between px-3 py-1.5 bg-[var(--bg-sidebar)] border-b border-[var(--border-default)] text-xs text-[var(--fg-secondary)] select-none shrink-0">
        <div className="flex items-center space-x-2 font-semibold text-[var(--fg-primary)]">
          <IconRobot className="size-3.5 text-blue-400" />
          <span>{session.name}</span>
          <span className="text-[10px] bg-blue-950/40 border border-blue-900 text-blue-400 px-1.5 py-0.5 rounded font-mono uppercase">
            {session.role_filter}
          </span>
        </div>

        <div className="flex items-center space-x-1">
          {session.state === "thinking" && (
            <span className="flex items-center gap-1 text-[10px] text-purple-400 font-mono">
              <span className="inline-block size-2 rounded-full bg-purple-400 animate-pulse" />
              thinking…
            </span>
          )}
          {(session.token_usage?.total_tokens ?? session.token_usage?.TotalTokens ?? 0) > 0 && (
            <TokenUsageBadge usage={session.token_usage} />
          )}
          {agentDefs.length > 0 && (
            <div className="relative">
              <button
                onClick={() => setShowAgentPicker(!showAgentPicker)}
                className="px-2 py-0.5 bg-[var(--bg-panel)] hover:bg-[var(--bg-surface-hover)] border border-[var(--border-default)] text-[var(--fg-secondary)] rounded flex items-center space-x-1 cursor-pointer"
                title="Launch pre-configured agent"
              >
                <IconRobot className="size-3 text-blue-400" />
                <span className="text-[10px]">Agent</span>
                {showAgentPicker ? <IconChevronUp className="size-3" /> : <IconChevronDown className="size-3" />}
              </button>
              {showAgentPicker && (
                <div className="absolute top-7 right-0 z-30 w-56 bg-[var(--bg-sidebar)] border border-[var(--border-default)] shadow-xl p-1 text-xs max-h-56 overflow-y-auto">
                  {agentDefs.map((def) => (
                    <button
                      key={def.id || def.ID}
                      onClick={() => handleLaunchAgentDef(def)}
                      className="w-full text-left px-2 py-1.5 hover:bg-[var(--bg-panel)] rounded text-[var(--fg-primary)] cursor-pointer"
                    >
                      <div className="font-semibold text-[11px]">{def.name || def.Name}</div>
                      {def.description && (
                        <div className="text-[9px] text-[var(--fg-tertiary)] truncate">{def.description}</div>
                      )}
                    </button>
                  ))}
                </div>
              )}
            </div>
          )}
          <button
            onClick={() => setShowModelPicker(!showModelPicker)}
            className="px-2 py-0.5 bg-[var(--bg-panel)] hover:bg-[var(--bg-surface-hover)] border border-[var(--border-default)] text-[var(--fg-secondary)] rounded flex items-center space-x-1 cursor-pointer font-mono"
          >
            <IconCpu className="size-3 text-purple-400" />
            <span className="text-[10px]">{activeModel || "Model"}</span>
            {showModelPicker ? <IconChevronUp className="size-3" /> : <IconChevronDown className="size-3" />}
          </button>

          <button onClick={onClose} className="hover:text-white cursor-pointer rounded p-0.5">
            <IconX className="size-3.5" />
          </button>
        </div>
      </div>

      {/* Model Picker dropdown */}
      {showModelPicker && (
        <div className="absolute top-8 right-12 z-30 w-64 bg-[var(--bg-sidebar)] border border-[var(--border-default)] shadow-xl p-2 text-xs max-h-72 overflow-y-auto">
          <div className="font-bold text-[10px] text-[var(--fg-tertiary)] uppercase tracking-wider mb-1.5 px-2">Models</div>
          {profiles.map((p) => {
            const pid = p.id || p.Id || p.name || p.Name;
            const models = p.selected_models || p.SelectedModels || p.available_models || p.AvailableModels || [];
            if (models.length === 0) {
              return (
                <div key={pid} className="px-2 py-1 text-[var(--fg-tertiary)] font-mono text-[11px]">
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
                    className={cn(
                      "w-full text-left px-2 py-1 hover:bg-[var(--bg-panel)] rounded text-[var(--fg-primary)] truncate font-mono text-[11px] cursor-pointer",
                      activeModel === m && "bg-[var(--bg-surface-active)] text-white"
                    )}
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

      {/* Messages layout */}
      <div ref={scrollContainerRef} className="flex-1 overflow-y-auto p-4 space-y-4">
        {session.messages?.length === 0 ? (
          <div className="flex flex-col items-center justify-center h-full text-center space-y-2 text-[var(--fg-tertiary)] select-none">
            <IconBrain className="size-12 stroke-[1.2] text-[var(--fg-disabled)] animate-pulse" />
            <h3 className="text-sm font-semibold text-[var(--fg-secondary)]">Zed Assistant Session</h3>
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
                    <IconCopy className="size-3.5" />
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
                        {(expandedReasoning[`r-${ti}`] ?? true) ? <IconChevronDown className="size-3.5" /> : <IconChevronRight className="size-3.5" />}
                        <IconBrain className="size-3.5 text-purple-400" />
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
                        <IconCopy className="size-3.5" />
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
        {session.pending_tool && (
          <div className="border border-[var(--border-default)] bg-[var(--bg-panel)] p-3 space-y-3 shadow-md max-w-md">
            <div className="flex items-center space-x-2 text-xs font-semibold text-amber-400">
              <IconShield className="size-4" />
              <span>Permission Request</span>
            </div>
            <div className="text-xs font-mono bg-black/30 p-2 border border-[var(--border-default)] text-[var(--fg-primary)] overflow-x-auto">
              {JSON.stringify(session.pending_tool)}
            </div>
            <div className="flex items-center justify-end space-x-2 pt-1">
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
                <IconCheck className="size-3.5" />
                <span>Approve</span>
              </button>
            </div>
          </div>
        )}
        <div ref={chatEndRef} />
      </div>

      {/* Input container */}
      <div className="p-3 border-t border-[var(--border-default)] bg-[var(--bg-sidebar)] shrink-0 relative">
        <textarea
          value={inputText}
          onChange={handleInputChange}
          onKeyDown={(e) => {
            if (e.key === "Tab" && e.shiftKey) {
              if (session.pending_tool) {
                e.preventDefault();
                handleApproval(true);
              }
            } else if (e.key === "Enter" && !e.shiftKey) {
              e.preventDefault();
              handleSendMessage();
            }
          }}
          placeholder="Ask anything, type @ to mention files..."
          rows={2}
          className="w-full bg-[var(--bg-panel)] border border-[var(--border-default)] p-2 text-xs text-[var(--fg-primary)] focus:outline-none focus:border-[var(--accent-primary)] resize-none"
        />

        {/* File mentions autocomplete box */}
        {showMentionMenu && mentionResults.length > 0 && (
          <div className="absolute bottom-16 left-3 right-3 z-30 bg-[var(--bg-sidebar)] border border-[var(--border-default)] shadow-xl max-h-32 overflow-y-auto p-1 space-y-0.5">
            {mentionResults.map((p) => (
              <button
                key={p}
                onClick={() => {
                  const atIndex = inputText.lastIndexOf("@");
                  setInputText(inputText.slice(0, atIndex) + "@" + p + " ");
                  setShowMentionMenu(false);
                }}
                className="w-full text-left px-2 py-1.5 hover:bg-[var(--bg-panel)] rounded text-[var(--fg-secondary)] hover:text-[var(--fg-primary)] text-xs font-mono truncate"
              >
                {p}
              </button>
            ))}
          </div>
        )}

        <div className="flex items-center justify-end pt-1">
          <button
            onClick={handleSendMessage}
            className="px-3 py-1 bg-[var(--accent-primary)] hover:bg-[var(--accent-hover)] text-black text-xs font-semibold flex items-center space-x-1 cursor-pointer"
          >
            <IconSend className="size-3.5" />
            <span>Send</span>
          </button>
        </div>
      </div>
    </div>
  );
}
