import React, { useState, useEffect, useMemo, useRef, useCallback } from "react";
import {
  IconTerminal2,
  IconRobot,
  IconPlus,
  IconX,
  IconSparkles,
  IconStack,
  IconMaximize,
  IconColumns,
  IconGridDots,
  IconCheck,
  IconSquare,
  IconBrain,
  IconShield,
  IconPaperclip,
  IconSend,
  IconFileText,
  IconCpu,
  IconSettings,
  IconChevronDown,
  IconChevronRight,
  IconChevronUp,
  IconCopy,
} from "@tabler/icons-react";
import { TerminalView } from "../components/terminal-view";
import { cn } from "../lib/utils";
import {
  ListAgentSessions,
  CreateAgentSession,
  SendAgentMessage,
  RespondAgentApproval,
  ToggleAgentTask,
  SearchFilename,
  GetProviderProfiles,
  SetActiveModel,
  GetLLMConfig,
} from "../../wailsjs/go/main/App";
import { terminal, llm } from "../../wailsjs/go/models";
import { EventsOn } from "../../wailsjs/runtime";

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
}

interface ShellScreenProps {
  sessions: terminal.Session[];
  onCreateShell: () => void;
  onCloseSession: (id: string) => void;
  onStopSession?: (id: string) => void;
  onRenameSession: (id: string, name: string) => void;
  initialSessionId?: string | null;
}

type LayoutMode = "single" | "horizontal" | "grid";

export function ShellScreen({
  sessions: shellSessions,
  onCreateShell,
  onCloseSession,
  onStopSession,
  onRenameSession,
  initialSessionId,
}: ShellScreenProps) {
  const [agentSessions, setAgentSessions] = useState<any[]>([]);
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [newSessionType, setNewSessionType] = useState<"shell" | "agent">("agent");
  const [newSessionName, setNewSessionName] = useState("");
  const [newSessionRole, setNewSessionRole] = useState<"coding" | "planning" | "research" | "custom">("coding");
  
  const [layoutMode, setLayoutMode] = useState<LayoutMode>("single");
  const [selectedSessionId, setSelectedSessionId] = useState<string | null>(initialSessionId ?? null);
  const [closedViewSessionIds, setClosedViewSessionIds] = useState<string[]>([]);

  const sessionRefs = useRef<Record<string, HTMLDivElement | null>>({});

  useEffect(() => {
    loadAgents();
    const unsub = EventsOn("agent:updated", () => loadAgents());
    return () => {
      if (typeof unsub === "function") unsub();
    };
  }, []);

  async function loadAgents() {
    try {
      const list = await ListAgentSessions();
      setAgentSessions(Array.isArray(list) ? list : []);
    } catch { /* ignore */ }
  }

  // Combine Shell and Agent sessions
  const allSessions: UnifiedSession[] = useMemo(() => {
    const list: UnifiedSession[] = [];
    for (const s of shellSessions) {
      list.push({
        id: s.id,
        name: s.name,
        type: "shell",
      });
    }
    for (const a of agentSessions) {
      list.push({
        id: a.id,
        name: a.name,
        type: "agent",
        role_filter: a.role_filter,
        state: a.state,
        messages: a.messages,
        tasks: a.tasks,
        token_usage: a.token_usage,
        auto_approve: a.auto_approve,
        pending_tool: a.pending_tool,
      });
    }
    return list;
  }, [shellSessions, agentSessions]);

  // Open sessions visible on the panel layout
  const visibleSessions = useMemo(() => {
    return allSessions.filter((s) => !closedViewSessionIds.includes(s.id));
  }, [allSessions, closedViewSessionIds]);

  // Sync default selected session if none is selected
  useEffect(() => {
    if (visibleSessions.length > 0 && (!selectedSessionId || !visibleSessions.find((s) => s.id === selectedSessionId))) {
      setSelectedSessionId(visibleSessions[0].id);
    }
  }, [visibleSessions, selectedSessionId]);

  // If initialSessionId is opened externally, make sure it is unclosed
  useEffect(() => {
    if (initialSessionId) {
      setClosedViewSessionIds((prev) => prev.filter((id) => id !== initialSessionId));
      setSelectedSessionId(initialSessionId);
    }
  }, [initialSessionId]);

  // Auto scroll active session element into view in horizontal layout mode
  useEffect(() => {
    if (selectedSessionId && layoutMode === "horizontal" && sessionRefs.current[selectedSessionId]) {
      sessionRefs.current[selectedSessionId]?.scrollIntoView({
        behavior: "smooth",
        inline: "center",
        block: "nearest",
      });
    }
  }, [selectedSessionId, layoutMode]);

  async function handleCreateAgent() {
    try {
      const name = newSessionName.trim() || `Agent (${newSessionRole})`;
      const created: any = await CreateAgentSession(name, newSessionRole, "");
      setShowCreateModal(false);
      setNewSessionName("");
      if (created && created.id) {
        setClosedViewSessionIds((prev) => prev.filter((id) => id !== created.id));
        setSelectedSessionId(created.id);
      }
      loadAgents();
    } catch (err) {
      console.error("Failed to create agent session:", err);
    }
  }

  // Close panel/tab view only — KEEPS session alive in sidebar session manager!
  function handleClosePanelTab(id: string, e?: React.MouseEvent) {
    if (e) e.stopPropagation();
    setClosedViewSessionIds((prev) => (prev.includes(id) ? prev : [...prev, id]));
    onCloseSession(id);
    const remaining = visibleSessions.filter((s) => s.id !== id);
    if (remaining.length > 0) {
      setSelectedSessionId(remaining[0].id);
    } else {
      setSelectedSessionId(null);
    }
  }

  // Selected session object
  const activeSessionObj = useMemo(() => {
    return visibleSessions.find((s) => s.id === selectedSessionId) || visibleSessions[0] || null;
  }, [visibleSessions, selectedSessionId]);

  return (
    <div className="flex flex-col h-full bg-[var(--color-bg-primary)] text-[var(--color-fg-primary)] overflow-hidden">
      {/* Top Navigation Bar */}
      <div className="flex items-center justify-between border-b border-[var(--color-border)] px-3 py-1.5 bg-[var(--color-bg-secondary)] shrink-0 gap-2">
        {/* Session Tabs */}
        <div className="flex items-center space-x-1.5 overflow-x-auto min-w-0 flex-1">
          {visibleSessions.map((s) => (
            <div
              key={s.id}
              onClick={() => setSelectedSessionId(s.id)}
              className={cn(
                "px-3 py-1.5 rounded border text-xs font-medium flex items-center space-x-2 cursor-pointer transition-all shrink-0 select-none",
                selectedSessionId === s.id
                  ? "bg-[var(--bg-surface-active)] border-[var(--border-focus)] text-[var(--fg-primary)] font-bold shadow"
                  : "bg-[var(--bg-surface)] border-[var(--border-default)] text-gray-300 hover:text-white"
              )}
            >
              {s.type === "shell" ? (
                <IconTerminal2 className="w-3.5 h-3.5 text-emerald-400" />
              ) : (
                <IconRobot className="w-3.5 h-3.5 text-purple-400" />
              )}
              <span className="truncate max-w-[130px]">{s.name}</span>
              <button
                onClick={(e) => handleClosePanelTab(s.id, e)}
                className="hover:text-white text-gray-400 hover:bg-white/20 p-0.5 rounded ml-1 cursor-pointer"
                title="Close Tab (Session Remains Alive in Sidebar)"
              >
                <IconX className="w-3.5 h-3.5" />
              </button>
            </div>
          ))}

          <button
            onClick={() => setShowCreateModal(true)}
            className="px-2.5 py-1.5 rounded bg-[var(--bg-surface-active)] hover:bg-[var(--bg-surface-hover)] border border-[var(--border-default)] text-[var(--fg-primary)] text-xs font-semibold flex items-center space-x-1 shadow shrink-0 cursor-pointer"
            title="Add New Session (Shell or Agent)"
          >
            <IconPlus className="w-3.5 h-3.5" />
            <span>New Session</span>
          </button>
        </div>

        {/* Layout Mode Switcher Buttons */}
        <div className="flex items-center space-x-1 shrink-0">
          <div className="flex items-center space-x-0.5 bg-[var(--bg-surface)] p-0.5 rounded border border-[var(--border-default)] text-xs">
            <button
              onClick={() => setLayoutMode("single")}
              className={cn(
                "p-1.5 rounded transition-colors cursor-pointer",
                layoutMode === "single" ? "bg-[var(--bg-surface-active)] text-white font-bold" : "text-gray-400 hover:text-white"
              )}
              title="Single Focused View"
            >
              <IconMaximize className="w-3.5 h-3.5" />
            </button>
            <button
              onClick={() => setLayoutMode("horizontal")}
              className={cn(
                "p-1.5 rounded transition-colors cursor-pointer",
                layoutMode === "horizontal" ? "bg-[var(--bg-surface-active)] text-white font-bold" : "text-gray-400 hover:text-white"
              )}
              title="Horizontal Scrollable Side-by-Side View"
            >
              <IconColumns className="w-3.5 h-3.5" />
            </button>
            <button
              onClick={() => setLayoutMode("grid")}
              className={cn(
                "p-1.5 rounded transition-colors cursor-pointer",
                layoutMode === "grid" ? "bg-[var(--bg-surface-active)] text-white font-bold" : "text-gray-400 hover:text-white"
              )}
              title="2x2 Grid View"
            >
              <IconGridDots className="w-3.5 h-3.5" />
            </button>
          </div>
        </div>
      </div>

      {/* Main Display Body */}
      <div className="flex-1 overflow-hidden h-full w-full relative">
        {visibleSessions.length === 0 ? (
          <div className="flex flex-col items-center justify-center h-full text-center space-y-3 text-gray-400">
            <IconSparkles className="w-12 h-12 text-purple-400 animate-pulse" />
            <h3 className="text-lg font-semibold text-white">No Active Sessions Open on Panel</h3>
            <p className="text-xs text-gray-400 max-w-sm">
              Open a session from the sidebar runtime manager or create a new session.
            </p>
            <button
              onClick={() => setShowCreateModal(true)}
              className="px-4 py-2 bg-[var(--bg-surface-active)] hover:bg-[var(--bg-surface-hover)] border border-[var(--border-default)] text-white text-xs font-semibold rounded-lg flex items-center space-x-2 shadow-lg cursor-pointer"
            >
              <IconPlus className="w-4 h-4" />
              <span>Create Shell or Agent Session</span>
            </button>
          </div>
        ) : layoutMode === "single" ? (
          /* Single Focused Session View */
          activeSessionObj ? (
            <SessionCell
              session={activeSessionObj}
              isFocused={true}
              onClose={() => handleClosePanelTab(activeSessionObj.id)}
            />
          ) : null
        ) : layoutMode === "horizontal" ? (
          /* Horizontally Scrollable Side-by-side Panel Layout */
          <div className="flex flex-row h-full w-full overflow-x-auto overflow-y-hidden divide-x divide-[var(--color-border)]">
            {visibleSessions.map((s) => (
              <div
                key={s.id}
                ref={(el) => { sessionRefs.current[s.id] = el; }}
                onClick={() => setSelectedSessionId(s.id)}
                className={cn(
                  "min-w-[420px] max-w-[650px] flex-1 h-full overflow-hidden transition-all",
                  selectedSessionId === s.id && "ring-2 ring-blue-500 z-10"
                )}
              >
                <SessionCell session={s} isFocused={selectedSessionId === s.id} onClose={() => handleClosePanelTab(s.id)} />
              </div>
            ))}
          </div>
        ) : (
          /* 2x2 Grid View Layout */
          <div className="grid grid-cols-2 grid-rows-2 h-full w-full gap-1.5 p-1.5 bg-black/40 overflow-hidden">
            {visibleSessions.slice(0, 4).map((s) => (
              <div
                key={s.id}
                onClick={() => setSelectedSessionId(s.id)}
                className={cn(
                  "h-full w-full overflow-hidden rounded border border-[var(--color-border)] transition-all",
                  selectedSessionId === s.id && "ring-2 ring-blue-500 z-10 border-blue-500"
                )}
              >
                <SessionCell session={s} isFocused={selectedSessionId === s.id} onClose={() => handleClosePanelTab(s.id)} />
              </div>
            ))}
          </div>
        )}
      </div>

      {/* New Session Picker Modal */}
      {showCreateModal && (
        <div className="fixed inset-0 bg-black/70 backdrop-blur-sm z-50 flex items-center justify-center p-4">
          <div className="bg-[var(--color-bg-secondary)] border border-[var(--color-border)] rounded-xl w-full max-w-md p-5 space-y-4 shadow-2xl">
            <div className="flex items-center justify-between border-b border-[var(--color-border)] pb-3">
              <span className="font-bold text-white text-base">Create New Session</span>
              <button onClick={() => setShowCreateModal(false)} className="text-gray-400 hover:text-white cursor-pointer">
                <IconX className="w-5 h-5" />
              </button>
            </div>

            <div className="grid grid-cols-2 gap-2 text-xs font-semibold">
              <button
                onClick={() => setNewSessionType("shell")}
                className={cn(
                  "p-3 rounded-lg border flex flex-col items-center justify-center space-y-1.5 transition-all cursor-pointer",
                  newSessionType === "shell"
                    ? "border-blue-500 bg-blue-600/20 text-white font-bold"
                    : "border-[var(--color-border)] bg-[var(--color-bg-primary)] text-gray-400 hover:text-white"
                )}
              >
                <IconTerminal2 className="w-6 h-6 text-green-400" />
                <span>Shell Terminal</span>
              </button>

              <button
                onClick={() => setNewSessionType("agent")}
                className={cn(
                  "p-3 rounded-lg border flex flex-col items-center justify-center space-y-1.5 transition-all cursor-pointer",
                  newSessionType === "agent"
                    ? "border-blue-500 bg-blue-600/20 text-white font-bold"
                    : "border-[var(--color-border)] bg-[var(--color-bg-primary)] text-gray-400 hover:text-white"
                )}
              >
                <IconRobot className="w-6 h-6 text-blue-400" />
                <span>AI Agent</span>
              </button>
            </div>

            {newSessionType === "agent" && (
              <div className="space-y-2 text-xs">
                <label className="text-gray-300 block font-medium">Agent Role Filter</label>
                <select
                  value={newSessionRole}
                  onChange={(e: any) => setNewSessionRole(e.target.value)}
                  className="w-full bg-[var(--color-bg-primary)] border border-[var(--color-border)] rounded px-3 py-1.5 text-gray-200 focus:outline-none"
                >
                  <option value="coding">Coding Agent</option>
                  <option value="planning">Planning Agent</option>
                  <option value="research">Research Agent</option>
                  <option value="custom">Custom Agent</option>
                </select>

                <label className="text-gray-300 block font-medium pt-1">Session Name (Optional)</label>
                <input
                  type="text"
                  value={newSessionName}
                  onChange={(e) => setNewSessionName(e.target.value)}
                  placeholder="My Agent Task"
                  className="w-full bg-[var(--color-bg-primary)] border border-[var(--color-border)] rounded px-3 py-1.5 text-gray-200 focus:outline-none"
                />
              </div>
            )}

            <div className="flex items-center justify-end space-x-2 pt-2">
              <button
                onClick={() => setShowCreateModal(false)}
                className="px-3 py-1.5 text-xs text-gray-300 hover:text-white cursor-pointer"
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
                className="px-4 py-1.5 bg-blue-600 hover:bg-blue-500 text-white text-xs font-semibold rounded cursor-pointer"
              >
                Launch Session
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}

/** Individual Session Cell: Either a Terminal or an Agent */
function SessionCell({
  session,
  isFocused,
  onClose,
}: {
  session: UnifiedSession;
  isFocused: boolean;
  onClose: () => void;
}) {
  if (session.type === "shell") {
    return (
      <div className="flex flex-col h-full w-full border-r border-[var(--color-border)] overflow-hidden bg-black">
        <div className="flex items-center justify-between px-3 py-1 bg-[var(--color-bg-secondary)] border-b border-[var(--color-border)] text-xs text-gray-300 select-none shrink-0">
          <div className="flex items-center space-x-1.5">
            <IconTerminal2 className="w-3.5 h-3.5 text-green-400" />
            <span className="font-semibold">{session.name}</span>
          </div>
          <button onClick={onClose} className="hover:text-white text-gray-400 cursor-pointer" title="Close Panel Tab">
            <IconX className="w-3.5 h-3.5" />
          </button>
        </div>
        <div className="flex-1 overflow-hidden">
          <TerminalView sessionId={session.id} isActive={isFocused} />
        </div>
      </div>
    );
  }

  return <AgentCell session={session} isFocused={isFocused} onClose={onClose} />;
}

/** Agent Cell with Model Picker Accordion, Thinking process, and Tool calling process */
function AgentCell({
  session,
  isFocused,
  onClose,
}: {
  session: UnifiedSession;
  isFocused: boolean;
  onClose: () => void;
}) {
  const [inputText, setInputText] = useState("");
  const [showMentionMenu, setShowMentionMenu] = useState(false);
  const [mentionResults, setMentionResults] = useState<string[]>([]);
  const [attachedFiles, setAttachedFiles] = useState<string[]>([]);
  const [profiles, setProfiles] = useState<llm.ProviderProfile[]>([]);
  const [activeModel, setActiveModelName] = useState<string>("");
  const [showModelPicker, setShowModelPicker] = useState(false);
  const chatEndRef = useRef<HTMLDivElement>(null);

  const [copiedMsgId, setCopiedMsgId] = useState<string | null>(null);

  const loadProfiles = useCallback(async () => {
    try {
      const list = await GetProviderProfiles();
      setProfiles(Array.isArray(list) ? list : []);
      const cfg = await GetLLMConfig();
      if (cfg && cfg.model) {
        setActiveModelName(cfg.model);
      }
    } catch { /* ignore */ }
  }, []);

  useEffect(() => {
    loadProfiles();
  }, [loadProfiles]);

  useEffect(() => {
    chatEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [session.messages]);

  async function handleSendMessage() {
    if (!inputText.trim() && attachedFiles.length === 0) return;
    const text = inputText;
    const files = [...attachedFiles];
    setInputText("");
    setAttachedFiles([]);
    try {
      await SendAgentMessage(session.id, text, files);
    } catch (err) {
      console.error(err);
    }
  }

  async function handleApproval(approve: boolean, autoAll: boolean = false) {
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
    <div className="flex flex-col h-full w-full border-r border-[var(--color-border)] bg-[var(--color-bg-primary)] overflow-hidden">
      {/* Header with Model Selector Accordion */}
      <div className="flex flex-col bg-[var(--color-bg-secondary)] border-b border-[var(--color-border)] text-xs text-gray-300 select-none shrink-0">
        <div className="flex items-center justify-between px-3 py-1.5">
          <div className="flex items-center space-x-2 font-semibold text-white">
            <IconRobot className="w-4 h-4 text-blue-400" />
            <span>{session.name}</span>
            <span className="font-mono text-[10px] bg-blue-950/60 border border-blue-800 text-blue-300 px-1.5 py-0.5 rounded uppercase">
              {session.role_filter || "coding"}
            </span>
          </div>

          <div className="flex items-center space-x-2">
            <button
              onClick={() => setShowModelPicker(!showModelPicker)}
              className="px-2 py-0.5 rounded bg-[var(--color-bg-tertiary)] hover:bg-[var(--color-border)] text-gray-300 hover:text-white flex items-center space-x-1.5 cursor-pointer font-mono text-[11px]"
              title="Select LLM Provider & Model"
            >
              <IconCpu className="w-3.5 h-3.5 text-purple-400" />
              <span className="text-purple-300 font-bold">{activeModel || "Model Picker"}</span>
              {showModelPicker ? <IconChevronUp className="w-3 h-3" /> : <IconChevronDown className="w-3 h-3" />}
            </button>
            <button onClick={onClose} className="hover:text-white text-gray-400 cursor-pointer" title="Close Panel Tab">
              <IconX className="w-3.5 h-3.5" />
            </button>
          </div>
        </div>

        {/* Model Picker Accordion Drawer */}
        {showModelPicker && (
          <div className="p-3 bg-[var(--color-bg-tertiary)] border-t border-[var(--color-border)] space-y-2 text-xs">
            <div className="font-bold text-white text-[11px] uppercase tracking-wider">Provider & Model Accordion</div>
            {profiles.length === 0 ? (
              <div className="text-gray-400 italic text-[11px]">
                No custom LLM providers configured. Open Global Settings to add a provider.
              </div>
            ) : (
              <div className="space-y-2 max-h-48 overflow-y-auto">
                {profiles.map((p) => (
                  <details key={p.id} open className="border border-[var(--color-border)] bg-[var(--color-bg-primary)] rounded p-2 space-y-1">
                    <summary className="cursor-pointer font-bold text-gray-200 hover:text-white flex items-center justify-between">
                      <span className="flex items-center space-x-1.5">
                        <IconCpu className="w-3.5 h-3.5 text-purple-400" />
                        <span>{p.name}</span>
                      </span>
                      <span className="text-[10px] text-gray-400 font-mono">
                        {(p.selected_models?.length || p.available_models?.length || 0)} models
                      </span>
                    </summary>
                    <div className="pt-2 grid grid-cols-1 gap-1">
                      {(p.selected_models?.length ? p.selected_models : p.available_models || []).map((m) => (
                        <button
                          key={m}
                          onClick={async () => {
                            await SetActiveModel(p.id, m);
                            setActiveModelName(m);
                            setShowModelPicker(false);
                          }}
                          className={`w-full text-left px-2 py-1 rounded text-gray-300 font-mono text-[11px] truncate cursor-pointer flex items-center justify-between ${
                            activeModel === m ? "bg-purple-600/40 text-white font-bold border border-purple-500/50" : "hover:bg-purple-600/20"
                          }`}
                        >
                          <span>{m}</span>
                          {activeModel === m ? (
                            <span className="text-[10px] bg-purple-600 text-white px-1.5 py-0.5 rounded font-bold">Active</span>
                          ) : (
                            <span className="text-[10px] text-purple-300 bg-purple-950/60 px-1 rounded">Select</span>
                          )}
                        </button>
                      ))}
                    </div>
                  </details>
                ))}
              </div>
            )}
          </div>
        )}
      </div>

      {/* Message log */}
      <div className="flex-1 overflow-y-auto p-3 space-y-3">
        {(session.messages || []).map((msg: any) => (
          <div
            key={msg.id}
            className={cn(
              "flex flex-col space-y-2 rounded-lg p-2.5 max-w-full text-xs",
              msg.role === "user"
                ? "ml-auto bg-blue-600/20 border border-blue-500/30 text-white"
                : msg.role === "tool"
                ? "bg-zinc-900 border border-zinc-700/50 font-mono text-emerald-300"
                : "bg-[var(--color-bg-secondary)] border border-[var(--color-border)] text-gray-200"
            )}
          >
            <div className="flex items-center justify-between text-[10px] text-gray-400 border-b border-[var(--color-border)] pb-1">
              <span className="font-bold uppercase tracking-wider">{msg.role}</span>
              <div className="flex items-center space-x-2">
                <span>{new Date(msg.timestamp).toLocaleTimeString()}</span>
                {msg.content && (
                  <button
                    type="button"
                    onClick={() => {
                      navigator.clipboard.writeText(msg.content);
                      setCopiedMsgId(msg.id);
                      setTimeout(() => setCopiedMsgId(null), 2000);
                    }}
                    className="p-0.5 hover:text-white text-gray-400 rounded transition-colors cursor-pointer flex items-center space-x-1"
                    title="Copy message response"
                  >
                    {copiedMsgId === msg.id ? (
                      <>
                        <IconCheck className="w-3.5 h-3.5 text-emerald-400" />
                        <span className="text-[10px] text-emerald-400 font-mono">Copied</span>
                      </>
                    ) : (
                      <>
                        <IconCopy className="w-3.5 h-3.5" />
                        <span className="text-[10px] font-mono">Copy</span>
                      </>
                    )}
                  </button>
                )}
              </div>
            </div>

            {/* THINKING PROCESS SECTION */}
            {msg.reasoning && (
              <details open className="bg-purple-950/40 border border-purple-800/60 rounded p-2 text-[11px] font-mono text-purple-300 space-y-1">
                <summary className="cursor-pointer font-bold flex items-center space-x-1.5 text-purple-400 select-none">
                  <IconBrain className="w-4 h-4 text-purple-400 animate-pulse" />
                  <span>Thinking Process & Reasoning</span>
                </summary>
                <div className="mt-1.5 whitespace-pre-wrap text-purple-200/90 leading-relaxed pl-2 border-l-2 border-purple-500/50">
                  {msg.reasoning}
                </div>
              </details>
            )}

            {/* TOOL CALLING PROCESS SECTION */}
            {msg.tool_calls && msg.tool_calls.length > 0 && (
              <div className="space-y-1.5 pt-1">
                {msg.tool_calls.map((tc: any, idx: number) => (
                  <div key={idx} className="border border-blue-900/60 bg-blue-950/40 rounded p-2 font-mono text-[11px] space-y-1">
                    <div className="flex items-center justify-between font-bold text-blue-300">
                      <span className="flex items-center space-x-1.5">
                        <IconCpu className="w-3.5 h-3.5 text-blue-400" />
                        <span>Tool Call: {tc.function?.name || tc.name}</span>
                      </span>
                      <span className="text-[10px] bg-blue-900/80 text-blue-200 px-1.5 py-0.5 rounded">Executing</span>
                    </div>
                    {tc.function?.arguments && (
                      <div className="text-[10px] text-gray-300 bg-black/50 p-1.5 rounded overflow-x-auto">
                        {tc.function.arguments}
                      </div>
                    )}
                  </div>
                ))}
              </div>
            )}

            <div className="whitespace-pre-wrap leading-relaxed">{msg.content}</div>
          </div>
        ))}

        {/* PENDING TOOL EXECUTION APPROVAL BANNER */}
        {session.pending_tool && (
          <div className="p-3 bg-amber-950/40 border border-amber-500/50 rounded-lg space-y-2 text-xs font-mono">
            <div className="font-bold text-amber-300 flex items-center space-x-1.5">
              <IconShield className="w-4 h-4 text-amber-400 animate-bounce" />
              <span>Tool Execution Requires Approval: {session.pending_tool.name}</span>
            </div>
            <div className="text-amber-200/90 text-[11px] bg-black/60 p-2 rounded">
              Args: {JSON.stringify(session.pending_tool.args)}
            </div>
            <div className="flex items-center space-x-2 pt-1">
              <button
                onClick={() => handleApproval(true, false)}
                className="px-3 py-1 bg-emerald-600 hover:bg-emerald-500 text-white rounded text-xs font-semibold flex items-center space-x-1 cursor-pointer"
              >
                <IconCheck className="w-3.5 h-3.5" />
                <span>Approve Tool</span>
              </button>
              <button
                onClick={() => handleApproval(true, true)}
                className="px-3 py-1 bg-blue-600 hover:bg-blue-500 text-white rounded text-xs font-semibold flex items-center space-x-1 cursor-pointer"
              >
                <IconCheck className="w-3.5 h-3.5" />
                <span>Always Auto-Approve</span>
              </button>
              <button
                onClick={() => handleApproval(false, false)}
                className="px-3 py-1 bg-rose-600 hover:bg-rose-500 text-white rounded text-xs font-semibold flex items-center space-x-1 cursor-pointer"
              >
                <IconX className="w-3.5 h-3.5" />
                <span>Reject</span>
              </button>
            </div>
          </div>
        )}

        <div ref={chatEndRef} />
      </div>

      {/* Input area */}
      <div className="p-2 border-t border-[var(--color-border)] bg-[var(--color-bg-secondary)] space-y-1.5 relative shrink-0">
        {/* Mentioned File Badges / Pills */}
        {attachedFiles.length > 0 && (
          <div className="flex flex-wrap gap-1.5 pb-1">
            {attachedFiles.map((file) => {
              const relName = file.split("/").slice(-2).join("/");
              return (
                <span
                  key={file}
                  className="inline-flex items-center space-x-1.5 bg-purple-950/70 text-purple-300 border border-purple-700/60 rounded-md px-2 py-0.5 text-[11px] font-mono shadow-sm"
                >
                  <IconFileText className="w-3 h-3 text-purple-400 shrink-0" />
                  <span className="truncate max-w-[220px]" title={file}>@{relName}</span>
                  <button
                    type="button"
                    onClick={() => setAttachedFiles(attachedFiles.filter((f) => f !== file))}
                    className="hover:text-white text-purple-400 cursor-pointer ml-0.5"
                  >
                    <IconX className="w-3 h-3" />
                  </button>
                </span>
              );
            })}
          </div>
        )}

        {showMentionMenu && mentionResults.length > 0 && (
          <div className="absolute bottom-full mb-1 left-2 right-2 max-h-36 overflow-y-auto bg-[var(--color-bg-tertiary)] border border-[var(--color-border)] rounded shadow-xl z-50 p-1 text-xs">
            {mentionResults.map((path) => {
              const relPath = path.split("/").slice(-2).join("/");
              return (
                <button
                  key={path}
                  type="button"
                  onClick={() => {
                    if (!attachedFiles.includes(path)) {
                      setAttachedFiles([...attachedFiles, path]);
                    }
                    const atIndex = inputText.lastIndexOf("@");
                    if (atIndex !== -1) {
                      setInputText(inputText.slice(0, atIndex) + `@${relPath} `);
                    }
                    setShowMentionMenu(false);
                  }}
                  className="w-full text-left px-2 py-1 rounded hover:bg-purple-600 hover:text-white flex items-center justify-between truncate cursor-pointer font-mono text-[11px]"
                >
                  <span className="font-semibold text-purple-300">@{relPath}</span>
                  <span className="text-[10px] opacity-60 truncate ml-2">{path}</span>
                </button>
              );
            })}
          </div>
        )}

        <textarea
          value={inputText}
          onChange={handleInputChange}
          onKeyDown={(e) => {
            if (e.key === "Enter" && !e.shiftKey) {
              e.preventDefault();
              handleSendMessage();
            }
          }}
          placeholder="Type message, @ mention files..."
          rows={2}
          className="w-full bg-[var(--color-bg-primary)] border border-[var(--color-border)] rounded p-2 text-xs text-white focus:outline-none focus:border-blue-500 resize-none"
        />

        {/* BOTTOM INDICATOR: Token, Cache, Out indicator at the bottom footer */}
        <div className="flex items-center justify-between text-[11px] pt-1">
          <div className="flex items-center space-x-3 font-mono text-gray-400">
            <span className="text-blue-400 font-semibold">Tokens: {((session.token_usage?.prompt_tokens || 0) / 1000).toFixed(1)}k</span>
            <span className="text-emerald-400 font-semibold">cache: {((session.token_usage?.cached_tokens || 0) / 1000).toFixed(1)}k</span>
            <span className="text-purple-400 font-semibold">out: {((session.token_usage?.completion_tokens || 0) / 1000).toFixed(1)}k</span>
          </div>

          <button
            onClick={handleSendMessage}
            className="px-3 py-1 bg-blue-600 hover:bg-blue-500 text-white rounded text-xs font-semibold flex items-center space-x-1 cursor-pointer"
          >
            <IconSend className="w-3.5 h-3.5" />
            <span>Send</span>
          </button>
        </div>
      </div>
    </div>
  );
}
