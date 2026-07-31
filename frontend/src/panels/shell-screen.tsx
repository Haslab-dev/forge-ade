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
            <span className="px-1.5 py-0.5 bg-[var(--bg-panel)] border border-[var(--border-default)] text-[10px] font-mono text-[var(--fg-tertiary)] rounded" title="Token usage">
              {(session.token_usage?.total_tokens ?? session.token_usage?.TotalTokens ?? 0).toLocaleString()} tok
            </span>
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
          session.messages?.map((msg, i) => {
            const isUser = msg.role === "user" || msg.Role === "user";
            const isTool = msg.role === "tool" || msg.Role === "tool";
            const text = msg.content || msg.Content || "";
            const reasoning = msg.reasoning || msg.Reasoning || "";
            const toolCalls = msg.tool_calls || msg.ToolCalls || [];
            const msgKey = msg.id || `${i}-${msg.role}`;
            const isThinking = expandedReasoning[msgKey];
            const showTools = expandedToolCalls[msgKey];
            return (
              <div key={msgKey} className="flex flex-col space-y-1 select-text">
                <span className="text-[10px] font-bold font-mono tracking-wider uppercase text-[var(--fg-tertiary)]">
                  {isUser ? "USER" : isTool ? "TOOL" : "ASSISTANT"}
                </span>

                {/* Reasoning accordion */}
                {reasoning && (
                  <div className="border border-[var(--border-default)] rounded overflow-hidden">
                    <button
                      onClick={() => setExpandedReasoning((p) => ({ ...p, [msgKey]: !p[msgKey] }))}
                      className="w-full flex items-center gap-1.5 px-2 py-1 text-[10px] font-mono uppercase tracking-wide text-[var(--fg-tertiary)] hover:bg-[var(--bg-surface-hover)] cursor-pointer"
                    >
                      {isThinking ? <IconChevronDown className="size-3" /> : <IconChevronRight className="size-3" />}
                      <IconBrain className="size-3 text-purple-400" />
                      <span>Thinking</span>
                    </button>
                    {isThinking && (
                      <div className="px-2 pb-2 text-[11px] text-[var(--fg-secondary)] leading-relaxed whitespace-pre-wrap border-t border-[var(--border-default)] pt-2 font-mono">
                        {reasoning}
                      </div>
                    )}
                  </div>
                )}

                {/* Tool calls accordion */}
                {toolCalls.length > 0 && (
                  <div className="border border-[var(--border-default)] rounded overflow-hidden">
                    <button
                      onClick={() => setExpandedToolCalls((p) => ({ ...p, [msgKey]: !p[msgKey] }))}
                      className="w-full flex items-center gap-1.5 px-2 py-1 text-[10px] font-mono uppercase tracking-wide text-[var(--fg-tertiary)] hover:bg-[var(--bg-surface-hover)] cursor-pointer"
                    >
                      {showTools ? <IconChevronDown className="size-3" /> : <IconChevronRight className="size-3" />}
                      <IconShield className="size-3 text-amber-400" />
                      <span>Tool Calls ({toolCalls.length})</span>
                    </button>
                    {showTools && (
                      <div className="px-2 pb-2 space-y-1.5 border-t border-[var(--border-default)] pt-2">
                        {toolCalls.map((tc: any, ti: number) => {
                          const fn = tc.function || tc.Function || {};
                          const name = fn.name || fn.Name || "tool";
                          let argsText = fn.arguments || fn.Arguments || "{}";
                          if (typeof argsText !== "string") argsText = JSON.stringify(argsText);
                          let parsed: any = null;
                          try { parsed = JSON.parse(argsText); } catch { /* keep raw */ }
                          return (
                            <div key={ti} className="text-[11px] font-mono bg-black/30 border border-[var(--border-default)] rounded p-1.5">
                              <div className="text-amber-400 font-semibold">{name}</div>
                              <pre className="text-[var(--fg-secondary)] whitespace-pre-wrap break-all mt-1">{parsed ? JSON.stringify(parsed, null, 2) : argsText}</pre>
                            </div>
                          );
                        })}
                      </div>
                    )}
                  </div>
                )}

                <div className="text-sm text-[var(--fg-primary)] leading-relaxed whitespace-pre-wrap font-sans selectable-text">
                  {text}
                </div>
              </div>
            );
          })
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
