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
import { AgentChatPanel } from "../components/agent-panel";
import { cn } from "../lib/utils";
import {
  ListSessions,
  ListAgentSessions,
  RenameSession,
  RenameAgentSession,
  CreateAgentSession,
  ListAgentDefinitions,
  CreateAgentSessionFromDefinition,
  SendAgentMessage,
  RespondAgentApproval,
  RespondAgentAsk,
  SetAgentAutoApprove,
  StopAgentTurn,
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
  pending_tools?: any[];
  pending_questions?: any[];
  dialect?: string;
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
  const [localShellSessions, setLocalShellSessions] = useState<any[]>([]);
  const [showCreateModal, setShowCreateModal] = useState(false);
  const [newSessionType, setNewSessionType] = useState<"shell" | "agent">("agent");
  const [newSessionName, setNewSessionName] = useState("");
  const [newSessionRole, setNewSessionRole] = useState<"coding" | "planning" | "research" | "custom">("coding");
  const [tabMenu, setTabMenu] = useState<{ x: number; y: number; session: UnifiedSession } | null>(null);
  const [renameTarget, setRenameTarget] = useState<UnifiedSession | null>(null);
  const [renameValue, setRenameValue] = useState("");

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
    loadShells();
    // Real-time updates via agent events + terminal events; no polling.
    const unsubs = [
      "agent:updated",
      "agent:turn_start",
      "agent:turn_end",
      "agent:message_start",
      "agent:message_delta",
      "agent:message_end",
      "agent:thinking_delta",
      "agent:tool_delta",
      "agent:tool_end",
      "agent:ask",
      "session:opened",
      "session:closed",
    ].map((ev) => EventsOn(ev, () => {
      if (ev.startsWith("agent") || ev === "agent:updated") loadAgents();
      if (ev.startsWith("session") || ev === "session:closed" || ev === "session:opened") loadShells();
    }));
    return () => {
      unsubs.forEach((u) => typeof u === "function" && u());
    };
  }, []);

  async function loadAgents() {
    try {
      const list = await ListAgentSessions();
      setAgentSessions(Array.isArray(list) ? list : []);
    } catch { /* ignore */ }
  }

  async function loadShells() {
    try {
      const list = await ListSessions();
      setLocalShellSessions(Array.isArray(list) ? list : []);
    } catch { /* ignore */ }
  }

  const allSessions: UnifiedSession[] = useMemo(() => {
    const list: UnifiedSession[] = [];
    const seen = new Set<string>();
    // 1. Local shell fetch is the authoritative shell source.
    for (const s of localShellSessions) {
      if (seen.has(s.id)) continue;
      list.push({ id: s.id, name: s.name, type: "shell" });
      seen.add(s.id);
    }
    // 2. Local agent fetch is the authoritative agent source (rich data).
    for (const a of agentSessions) {
      if (seen.has(a.id)) continue;
      list.push({
        id: a.id,
        name: a.name,
        type: "agent",
        role_filter: a.role_filter || "coding",
        state: a.state || "idle",
        messages: a.messages || [],
        tasks: a.tasks || [],
        token_usage: a.token_usage || {},
        auto_approve: !!a.auto_approve,
        pending_tools: a.pending_tools || [],
        pending_questions: a.pending_questions || [],
        dialect: a.dialect || "",
        project_name: a.project_name || "",
        custom_prompt: a.custom_prompt || "",
        custom_rules: a.custom_rules || "",
      });
      seen.add(a.id);
    }
    // 3. App's merged list is a fallback for anything the local fetches missed
    // (e.g. before the local fetch resolves, or agent entries App saw first).
    for (const s of shellSessions) {
      if (seen.has(s.id)) continue;
      const type = s.type === "agent" ? "agent" : "shell";
      list.push({ id: s.id, name: s.name, type });
      seen.add(s.id);
    }
    return list;
  }, [shellSessions, agentSessions, localShellSessions]);

  const visibleSessions = useMemo(() => {
    return allSessions.filter((s) => !closedViewSessionIds.includes(s.id));
  }, [allSessions, closedViewSessionIds]);

  // Prune stale closed/selected ids for sessions that no longer exist (e.g.
  // deleted from the sidebar kill button or the backend). Without this, a
  // deleted session's id lingers in the layout store and the panel can't
  // reopen it, or the selection points at a gone session.
  const liveIds = useMemo(() => new Set(allSessions.map((s) => s.id)), [allSessions]);
  useEffect(() => {
    const staleClosed = closedViewSessionIds.filter((id) => !liveIds.has(id));
    if (staleClosed.length > 0) {
      staleClosed.forEach((id) => reopenView(id));
    }
    if (selectedSessionId && !liveIds.has(selectedSessionId)) {
      const next = allSessions.find((s) => !closedViewSessionIds.includes(s.id));
      setSelectedSessionId(next ? next.id : null);
    }
  }, [liveIds, allSessions, closedViewSessionIds, selectedSessionId, reopenView, setSelectedSessionId]);

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

  async function handleRenameSession() {
    if (!renameTarget) return;
    const name = renameValue.trim();
    if (!name) return;
    try {
      if (renameTarget.type === "agent") {
        await RenameAgentSession(renameTarget.id, name);
        await loadAgents();
      } else {
        await RenameSession(renameTarget.id, name);
        await loadShells();
      }
      setRenameTarget(null);
      setRenameValue("");
    } catch (err) {
      console.error("Failed to rename session:", err);
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

  const closeAdjacent = (sessionId: string, direction: -1 | 1) => {
    const idx = visibleSessions.findIndex((s) => s.id === sessionId);
    const target = idx + direction;
    if (idx < 0 || target < 0 || target >= visibleSessions.length) return;
    handleClosePanelTab(visibleSessions[target].id);
  };

  const closeOthers = (sessionId: string) => {
    visibleSessions
      .filter((s) => s.id !== sessionId)
      .forEach((s) => handleClosePanelTab(s.id));
  };

  const closeAll = () => {
    visibleSessions.forEach((s) => handleClosePanelTab(s.id));
  };

  return (
    <div className="flex flex-col h-full bg-[var(--bg-app)] text-[var(--fg-primary)] overflow-hidden">
      {/* Top Tabs panel */}
      <div className="flex items-center justify-between border-b border-[var(--border-default)] px-3 py-1 bg-[var(--bg-sidebar)] shrink-0 gap-2 select-none">
        <div className="flex items-center space-x-1 overflow-x-auto flex-1">
          {visibleSessions.map((s) => (
            <div
              key={s.id}
              onClick={() => setSelectedSessionId(s.id)}
              onContextMenu={(e) => {
                e.preventDefault();
                e.stopPropagation();
                setTabMenu({ x: e.clientX, y: e.clientY, session: s });
              }}
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

      {tabMenu && (
        <div
          className="fixed z-[9999] min-w-[190px] rounded-lg overflow-hidden shadow-2xl border border-[var(--border-default)] bg-[var(--bg-elevated)] text-[var(--fg-primary)] text-xs py-1"
          style={{ left: tabMenu.x, top: tabMenu.y }}
          onClick={(e) => e.stopPropagation()}
        >
          {[
            { label: "Open", icon: "↵", action: () => { setSelectedSessionId(tabMenu.session.id); setTabMenu(null); } },
            { label: "Rename", icon: "✎", action: () => { setRenameTarget(tabMenu.session); setRenameValue(tabMenu.session.name || ""); setTabMenu(null); } },
            { label: "Close", icon: "✕", action: () => { handleClosePanelTab(tabMenu.session.id); setTabMenu(null); } },
            null,
            { label: "Close Next Tab", icon: "⊟", action: () => { closeAdjacent(tabMenu.session.id, 1); setTabMenu(null); }, disabled: visibleSessions.findIndex((s) => s.id === tabMenu.session.id) >= visibleSessions.length - 1 },
            { label: "Close Prev Tab", icon: "⊟", action: () => { closeAdjacent(tabMenu.session.id, -1); setTabMenu(null); }, disabled: visibleSessions.findIndex((s) => s.id === tabMenu.session.id) <= 0 },
            { label: "Close Others", icon: "◎", action: () => { closeOthers(tabMenu.session.id); setTabMenu(null); }, disabled: visibleSessions.length <= 1 },
            { label: "Close All", icon: "⊗", action: () => { closeAll(); setTabMenu(null); }, disabled: visibleSessions.length === 0 },
          ].map((item, idx) =>
            item === null ? (
              <div key={idx} className="my-1 border-t border-[var(--border-default)]" />
            ) : (
              <button
                key={idx}
                disabled={item.disabled}
                onClick={item.action}
                className={`w-full flex items-center gap-2.5 px-3 py-1.5 text-left transition-colors ${
                  item.disabled ? "opacity-35 cursor-not-allowed" : "hover:bg-[var(--bg-surface-hover)] cursor-pointer"
                }`}
              >
                <span className="text-[10px] w-3 text-center text-[var(--fg-tertiary)] shrink-0">{item.icon}</span>
                <span>{item.label}</span>
              </button>
            )
          )}
        </div>
      )}

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

      {/* Rename session modal */}
      {renameTarget && (
        <div className="fixed inset-0 bg-black/60 backdrop-blur-sm z-50 flex items-center justify-center p-4">
          <div className="bg-[var(--bg-sidebar)] border border-[var(--border-default)] w-full max-w-sm p-4 space-y-3">
            <div className="flex items-center justify-between pb-2 border-b border-[var(--border-default)]">
              <span className="font-bold text-sm text-[var(--fg-primary)]">Rename Session</span>
              <button onClick={() => setRenameTarget(null)} className="text-[var(--fg-tertiary)] hover:text-white cursor-pointer">
                <IconX className="size-4" />
              </button>
            </div>
            <input
              autoFocus
              value={renameValue}
              onChange={(e) => setRenameValue(e.target.value)}
              onKeyDown={(e) => {
                if (e.key === "Enter") handleRenameSession();
                if (e.key === "Escape") setRenameTarget(null);
              }}
              placeholder="Session name"
              className="w-full bg-[var(--bg-panel)] border border-[var(--border-default)] p-2 text-xs text-[var(--fg-primary)] focus:outline-none focus:border-[var(--accent-primary)]"
            />
            <div className="flex items-center justify-end space-x-2 pt-1">
              <button
                onClick={() => setRenameTarget(null)}
                className="px-3 py-1 text-xs text-[var(--fg-secondary)] hover:text-white border border-[var(--border-default)] rounded cursor-pointer"
              >
                Cancel
              </button>
              <button
                onClick={handleRenameSession}
                className="px-3 py-1 text-xs font-semibold bg-[var(--accent-primary)] hover:bg-[var(--accent-hover)] text-black rounded cursor-pointer"
              >
                Rename
              </button>
            </div>
          </div>
        </div>
      )}

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

  return <AgentChatPanel session={session} onClose={onClose} onAgentLaunched={onAgentLaunched} />;
}


