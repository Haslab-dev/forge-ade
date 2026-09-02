import React, { useState, useEffect, useCallback, useRef } from "react";
import {
  IconRobot,
  IconPlus,
  IconX,
  IconCpu,
  IconTerminal2,
  IconSparkles,
  IconChevronDown,
  IconChevronUp,
  IconLayoutSidebarRightCollapse,
  IconLayoutSidebarRightExpand,
  IconRefresh,
  IconCheck,
  IconFileText,
  IconSettings,
  IconGitCompare,
  IconCode,
} from "@tabler/icons-react";
import {
  ListAgentSessions,
  CreateAgentSession,
  DeleteAgentSession,
  ListSessions,
  CreateShell,
  GetProviderProfiles,
  GetLLMConfig,
  SetActiveModel,
  EventsOn,
} from "../lib/wails";
import { AgentChatPanel } from "../components/agent-panel";
import { TerminalView } from "../components/terminal-view";
import { cn } from "../lib/utils";
import { useWorkspaceStore } from "../hooks/store";

interface AgentScreenProps {
  onOpenSettings?: () => void;
  onSwitchToEditor?: () => void;
}

export function AgentScreen({ onOpenSettings, onSwitchToEditor }: AgentScreenProps) {
  const { workspace } = useWorkspaceStore();
  const [agentSessions, setAgentSessions] = useState<any[]>([]);
  const [activeSessionId, setActiveSessionId] = useState<string | null>(null);
  const [shellSessions, setShellSessions] = useState<any[]>([]);
  const [activeShellId, setActiveShellId] = useState<string | null>(null);
  const [showShellDrawer, setShowShellDrawer] = useState(true);
  const [profiles, setProfiles] = useState<any[]>([]);
  const [activeModel, setActiveModel] = useState<string>("");
  const [showModelPicker, setShowModelPicker] = useState(false);
  const modelPickerRef = useRef<HTMLDivElement>(null);

  const loadSessions = useCallback(async () => {
    try {
      const agents = await ListAgentSessions();
      const list = Array.isArray(agents) ? agents : [];
      setAgentSessions(list);
      if (list.length > 0) {
        setActiveSessionId((prev) => (prev && list.some((a: any) => a.id === prev) ? prev : list[0].id));
      } else {
        setActiveSessionId(null);
      }
    } catch {
      setAgentSessions([]);
    }
  }, []);

  const loadShells = useCallback(async () => {
    try {
      const shells = await ListSessions();
      const list = Array.isArray(shells) ? shells : [];
      setShellSessions(list);
      if (list.length > 0) {
        setActiveShellId((prev) => (prev && list.some((s: any) => s.id === prev) ? prev : list[0].id));
      } else {
        setActiveShellId(null);
      }
    } catch {
      setShellSessions([]);
    }
  }, []);

  const loadModelConfig = useCallback(async () => {
    try {
      const p = await GetProviderProfiles();
      setProfiles(Array.isArray(p) ? p : []);
      const cfg = await GetLLMConfig();
      if (cfg && cfg.model) setActiveModel(cfg.model);
    } catch { /* ignore */ }
  }, []);

  useEffect(() => {
    loadSessions();
    loadShells();
    loadModelConfig();

    const unsubs = [
      "agent:updated",
      "agent:config:changed",
      "session:opened",
      "session:closed",
    ].map((ev) =>
      EventsOn(ev, () => {
        if (ev.startsWith("agent")) loadSessions();
        if (ev.startsWith("session")) loadShells();
        if (ev === "agent:config:changed") loadModelConfig();
      })
    );

    const handleOutside = (e: MouseEvent) => {
      if (modelPickerRef.current && !modelPickerRef.current.contains(e.target as Node)) {
        setShowModelPicker(false);
      }
    };
    document.addEventListener("mousedown", handleOutside);

    return () => {
      unsubs.forEach((u) => typeof u === "function" && u());
      document.removeEventListener("mousedown", handleOutside);
    };
  }, [loadSessions, loadShells, loadModelConfig]);

  const handleCreateAgent = async () => {
    try {
      const folder = workspace?.folders[0] || "";
      const baseName = `Agent ${agentSessions.length + 1}`;
      const res = await CreateAgentSession(baseName, "coding", folder);
      await loadSessions();
      if (res && res.id) setActiveSessionId(res.id);
    } catch (err) {
      console.error("Failed to create agent session:", err);
    }
  };

  const handleCreateSideShell = async () => {
    try {
      const folder = workspace?.folders[0] || "";
      const name = `Terminal ${shellSessions.length + 1}`;
      const res = await CreateShell(name, folder);
      await loadShells();
      if (res && res.id) {
        setActiveShellId(res.id);
        setShowShellDrawer(true);
      }
    } catch (err) {
      console.error("Failed to create shell:", err);
    }
  };

  const handleSelectModel = async (providerId: string, model: string) => {
    setActiveModel(model);
    setShowModelPicker(false);
    try {
      await SetActiveModel(providerId, model);
    } catch { /* ignore */ }
  };

  const activeAgentSession = agentSessions.find((a) => a.id === activeSessionId);

  return (
    <div className="flex-1 flex h-full w-full bg-[var(--bg-app)] overflow-hidden select-none font-sans">
      {/* Left Main Area: Agent Tab Bar + Active Agent Chat Stream */}
      <div className="flex-1 flex flex-col h-full overflow-hidden border-r border-[var(--border-default)]">
        {/* Agent Sub-Header Tab Bar */}
        <div className="h-9 bg-[var(--bg-sidebar)] border-b border-[var(--border-default)] flex items-center justify-between px-3 shrink-0 text-xs">
          <div className="flex items-center gap-1.5 overflow-x-auto min-w-0 pr-2">
            <div className="flex items-center gap-1 text-[var(--accent-primary)] font-bold text-xs mr-2 shrink-0">
              <IconSparkles className="size-3.5" />
              <span>Agent Space</span>
            </div>

            {agentSessions.map((session) => {
              const active = session.id === activeSessionId;
              return (
                <div
                  key={session.id}
                  onClick={() => setActiveSessionId(session.id)}
                  className={cn(
                    "flex items-center gap-1.5 px-2.5 py-1 rounded-t cursor-pointer border-b-2 text-xs transition-colors shrink-0",
                    active
                      ? "bg-[var(--bg-app)] border-[var(--accent-primary)] text-[var(--fg-primary)] font-semibold shadow-xs"
                      : "border-transparent text-[var(--fg-secondary)] hover:text-[var(--fg-primary)] hover:bg-[var(--bg-panel)]"
                  )}
                >
                  <IconRobot className={cn("size-3.5", active ? "text-blue-400" : "text-[var(--fg-tertiary)]")} />
                  <span className="truncate max-w-32">{session.name}</span>
                  <button
                    onClick={(e) => {
                      e.stopPropagation();
                      DeleteAgentSession(session.id).then(loadSessions);
                    }}
                    className="p-0.5 hover:text-red-400 rounded cursor-pointer"
                    title="Delete session"
                  >
                    <IconX className="size-3" />
                  </button>
                </div>
              );
            })}

            <button
              onClick={handleCreateAgent}
              className="p-1 hover:bg-[var(--bg-panel)] rounded text-[var(--fg-secondary)] hover:text-white cursor-pointer shrink-0"
              title="New Agent Session (+)"
            >
              <IconPlus className="size-3.5" />
            </button>
          </div>

          {/* Right Top Controls: Model Picker Dropdown & Side Terminal Toggle */}
          <div className="flex items-center gap-2 shrink-0">
            {/* Model Switcher Dropdown */}
            <div className="relative" ref={modelPickerRef}>
              <button
                onClick={() => setShowModelPicker((prev) => !prev)}
                className="flex items-center gap-1.5 px-2 py-0.5 rounded bg-[var(--bg-panel)] border border-[var(--border-default)] hover:bg-[var(--bg-surface-hover)] text-xs text-[var(--fg-primary)] cursor-pointer font-mono"
                title="Select active LLM Provider & Model"
              >
                <IconCpu className="size-3 text-purple-400" />
                <span className="truncate max-w-28 text-[11px] font-semibold">{activeModel || "Select Model"}</span>
                {showModelPicker ? <IconChevronUp className="size-3" /> : <IconChevronDown className="size-3" />}
              </button>

              {showModelPicker && (
                <div className="absolute right-0 top-full mt-1 w-64 bg-[var(--bg-sidebar)] border border-[var(--border-default)] shadow-2xl p-2 z-50 text-xs rounded-md max-h-72 overflow-y-auto">
                  <div className="flex items-center justify-between pb-1.5 border-b border-[var(--border-default)] mb-1.5">
                    <span className="font-bold text-[10px] text-[var(--fg-tertiary)] uppercase tracking-wider">Models</span>
                    {onOpenSettings && (
                      <button
                        onClick={() => {
                          setShowModelPicker(false);
                          onOpenSettings();
                        }}
                        className="text-[10px] text-[var(--accent-primary)] hover:underline cursor-pointer"
                      >
                        Manage
                      </button>
                    )}
                  </div>
                  {profiles.length === 0 ? (
                    <div className="text-[11px] text-[var(--fg-tertiary)] p-2 italic text-center">
                      No provider profiles configured.
                    </div>
                  ) : (
                    profiles.map((p) => {
                      const pid = p.id || p.Id || p.name || p.Name;
                      const models = p.selected_models || p.SelectedModels || p.available_models || p.AvailableModels || [];
                      return (
                        <div key={pid} className="mb-2">
                          <div className="text-[9px] uppercase tracking-wider text-[var(--fg-tertiary)] font-bold px-1.5 py-0.5">
                            {p.name || p.Name}
                          </div>
                          {models.length === 0 ? (
                            <div className="text-[10px] text-[var(--fg-tertiary)] px-2 italic">No models fetched</div>
                          ) : (
                            models.map((m: string) => (
                              <button
                                key={m}
                                onClick={() => handleSelectModel(pid, m)}
                                className={cn(
                                  "w-full text-left px-2 py-1 hover:bg-[var(--bg-panel)] rounded text-[11px] font-mono flex items-center justify-between cursor-pointer truncate",
                                  activeModel === m && "bg-[var(--bg-surface-active)] text-[var(--fg-on-active)] font-semibold"
                                )}
                              >
                                <span className="truncate">{m}</span>
                                {activeModel === m && <IconCheck className="size-3 text-[var(--accent-primary)]" />}
                              </button>
                            ))
                          )}
                        </div>
                      );
                    })
                  )}
                </div>
              )}
            </div>

            {/* Toggle Side Shell Drawer */}
            <button
              onClick={() => setShowShellDrawer((prev) => !prev)}
              className={cn(
                "p-1 rounded cursor-pointer transition-colors flex items-center gap-1 text-xs",
                showShellDrawer
                  ? "bg-[var(--bg-surface-active)] text-[var(--fg-primary)]"
                  : "text-[var(--fg-secondary)] hover:text-white hover:bg-[var(--bg-panel)]"
              )}
              title={showShellDrawer ? "Hide side terminal" : "Show side terminal"}
            >
              <IconTerminal2 className="size-3.5 text-cyan-400" />
              <span className="hidden sm:inline text-[11px]">Shell</span>
              {showShellDrawer ? (
                <IconLayoutSidebarRightCollapse className="size-3.5" />
              ) : (
                <IconLayoutSidebarRightExpand className="size-3.5" />
              )}
            </button>
          </div>
        </div>

        {/* Agent View Body */}
        <div className="flex-1 overflow-hidden relative">
          {activeAgentSession ? (
            <AgentChatPanel
              session={activeAgentSession}
              onClose={() => DeleteAgentSession(activeAgentSession.id).then(loadSessions)}
              onAgentLaunched={() => loadSessions()}
            />
          ) : (
            <div className="flex-1 h-full flex flex-col items-center justify-between p-6 overflow-y-auto select-text bg-[var(--bg-app)]">
              <div className="w-full max-w-2xl flex flex-col items-center pt-8 md:pt-12">
                {/* Geometric Hexagonal Agent Icon */}
                <div className="mb-6 flex items-center justify-center">
                  <div className="relative size-16 flex items-center justify-center rounded-3xl bg-blue-500/10 border border-blue-500/20 shadow-inner">
                    <svg viewBox="0 0 100 100" className="w-10 h-10 text-[var(--accent-primary)] fill-current">
                      <polygon points="50,15 75,29 75,57 50,71 25,57 25,29" fill="none" stroke="currentColor" strokeWidth="6" strokeLinejoin="round" />
                      <polygon points="75,43 100,57 100,85 75,99 50,85 50,57" fill="currentColor" fillOpacity="0.25" />
                      <polygon points="25,43 50,57 50,85 25,99 0,85 0,57" fill="currentColor" fillOpacity="0.15" />
                      <circle cx="50" cy="43" r="5" fill="currentColor" />
                    </svg>
                  </div>
                </div>

                {/* Workspace Badge */}
                {workspace?.folders[0] && (
                  <div className="mb-4 flex items-center gap-1.5 px-3 py-1.5 rounded-full bg-[var(--bg-panel)] text-xs text-[var(--fg-secondary)] border border-[var(--border-default)]">
                    <IconSparkles className="size-3 text-[var(--accent-primary)]" />
                    <span className="font-medium text-[var(--fg-primary)]">{workspace.folders[0].split("/").pop()}</span>
                    <span className="text-[var(--fg-tertiary)]">·</span>
                    <span className="font-mono text-[10px] text-[var(--fg-tertiary)] truncate max-w-xs">{workspace.folders[0]}</span>
                  </div>
                )}

                <h2 className="text-xl font-bold text-[var(--fg-primary)] tracking-tight">Autonomous Coding Agent</h2>
                <p className="text-xs text-[var(--fg-tertiary)] text-center max-w-md mt-1 mb-6">
                  Agent can plan, search codebase, edit files, and execute commands with interactive feedback.
                </p>

                {/* Quick prompt templates */}
                <div className="w-full flex flex-col gap-2">
                  <span className="text-[11px] font-semibold uppercase tracking-wider text-[var(--fg-tertiary)]">
                    Quick Start Prompts
                  </span>
                  <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
                    {[
                      { title: "Explain Architecture", desc: "Analyze workspace structure & dependency graph" },
                      { title: "Fix Bugs & Errors", desc: "Scan recent issues and propose verified fixes" },
                      { title: "Run Tests & Verify", desc: "Execute test suite and inspect build status" },
                      { title: "Review Git Changes", desc: "Inspect uncommitted diffs and draft commit log" },
                    ].map((sug, i) => (
                      <button
                        key={i}
                        onClick={async () => {
                          const folder = workspace?.folders[0] || "";
                          const name = sug.title;
                          const res = await CreateAgentSession(name, "coding", folder);
                          await loadSessions();
                          if (res && res.id) setActiveSessionId(res.id);
                        }}
                        className="p-3 rounded-xl border border-[var(--border-default)] bg-[var(--bg-sidebar)] hover:bg-[var(--bg-panel)] hover:border-[var(--accent-primary)]/50 transition-all text-left flex flex-col justify-between group cursor-pointer"
                      >
                        <span className="font-semibold text-xs text-[var(--fg-primary)] group-hover:text-[var(--accent-primary)]">
                          {sug.title}
                        </span>
                        <span className="text-[11px] text-[var(--fg-tertiary)] mt-0.5">
                          {sug.desc}
                        </span>
                      </button>
                    ))}
                  </div>
                </div>

                {/* Primary Start Button */}
                <button
                  onClick={handleCreateAgent}
                  className="mt-6 px-6 py-2.5 bg-[var(--accent-primary)] hover:bg-[var(--accent-hover)] text-black font-semibold text-xs rounded-xl shadow-md cursor-pointer flex items-center gap-2 transition-transform hover:scale-[1.02]"
                >
                  <IconPlus className="size-4" />
                  <span>Start New Agent Session</span>
                </button>
              </div>

              {/* Footer info */}
              <div className="text-[11px] text-[var(--fg-tertiary)] pt-6 flex items-center gap-2">
                <span>Model: <code className="font-mono text-purple-400">{activeModel || "default"}</code></span>
                <span>•</span>
                <span>Type <code className="font-mono bg-[var(--bg-panel)] px-1 rounded">@</code> for files and tools</span>
              </div>
            </div>
          )}
        </div>
      </div>

      {/* Right Area: Agent Side Drawer (Shell Terminal + MyADE Quick Actions) */}
      {showShellDrawer && (
        <div className="w-[340px] lg:w-[380px] flex flex-col h-full bg-[var(--bg-sidebar)] border-l border-[var(--border-default)] overflow-hidden shrink-0">
          {/* Shell & Actions Tab Header */}
          <div className="h-9 bg-[var(--bg-sidebar)] border-b border-[var(--border-default)] flex items-center justify-between px-2.5 shrink-0 text-xs">
            <div className="flex items-center gap-1 overflow-x-auto min-w-0 pr-1">
              <IconTerminal2 className="size-3.5 text-cyan-400 mr-1 shrink-0" />
              {shellSessions.map((shell) => {
                const active = shell.id === activeShellId;
                return (
                  <button
                    key={shell.id}
                    onClick={() => setActiveShellId(shell.id)}
                    className={cn(
                      "px-2 py-0.5 rounded text-[11px] font-mono transition-colors truncate max-w-28 cursor-pointer",
                      active
                        ? "bg-[var(--bg-app)] text-[var(--fg-primary)] border border-[var(--border-default)] font-semibold"
                        : "text-[var(--fg-tertiary)] hover:text-[var(--fg-primary)]"
                    )}
                  >
                    {shell.name || "Shell"}
                  </button>
                );
              })}
              <button
                onClick={handleCreateSideShell}
                className="p-1 hover:bg-[var(--bg-panel)] rounded text-[var(--fg-tertiary)] hover:text-white cursor-pointer shrink-0"
                title="New Shell (+)"
              >
                <IconPlus className="size-3" />
              </button>
            </div>

            <button
              onClick={() => setShowShellDrawer(false)}
              className="p-1 text-[var(--fg-tertiary)] hover:text-white rounded cursor-pointer"
              title="Close drawer"
            >
              <IconX className="size-3.5" />
            </button>
          </div>

          {/* Shell Terminal Viewport */}
          <div className="h-56 lg:h-64 border-b border-[var(--border-default)] min-h-0 min-w-0 overflow-hidden relative bg-[var(--terminal-background)]">
            {shellSessions.length > 0 && activeShellId ? (
              shellSessions.map((shell) => (
                <div
                  key={shell.id}
                  className={cn(
                    "absolute inset-0 overflow-hidden",
                    shell.id === activeShellId ? "block z-10" : "hidden"
                  )}
                >
                  <TerminalView sessionId={shell.id} isActive={shell.id === activeShellId} />
                </div>
              ))
            ) : (
              <div className="flex flex-col items-center justify-center h-full p-4 text-center text-xs text-[var(--fg-tertiary)]">
                <IconTerminal2 className="size-7 opacity-40 mb-1.5" />
                <p>No active terminal shells</p>
                <button
                  onClick={handleCreateSideShell}
                  className="mt-2 px-2.5 py-1 bg-[var(--bg-panel)] border border-[var(--border-default)] hover:bg-[var(--bg-surface-hover)] text-[var(--fg-primary)] rounded cursor-pointer text-xs"
                >
                  Launch Terminal
                </button>
              </div>
            )}
          </div>

          {/* Quick Actions Drawer Panel (Exact MyADE Actions Replica) */}
          <div className="flex-1 p-4 flex flex-col justify-between overflow-y-auto font-sans bg-[var(--bg-app)]">
            <div className="space-y-4">
              <div className="text-[11px] font-semibold text-[var(--fg-tertiary)] uppercase tracking-wider">
                Quick Actions
              </div>

              <div className="space-y-1.5 text-xs">
                {/* New session */}
                <button
                  onClick={handleCreateAgent}
                  className="w-full text-left p-2.5 rounded-xl hover:bg-[var(--bg-panel)] border border-transparent hover:border-[var(--border-default)] flex items-start justify-between transition-colors group cursor-pointer"
                >
                  <div className="flex items-start gap-2.5">
                    <div className="p-1 rounded-lg bg-blue-500/10 text-blue-400 mt-0.5">
                      <IconPlus className="size-3.5" />
                    </div>
                    <div>
                      <p className="font-semibold text-[var(--fg-primary)]">New session</p>
                      <p className="text-[10px] text-[var(--fg-tertiary)]">Start a new agent session</p>
                    </div>
                  </div>
                  <kbd className="text-[9px] font-mono px-1 py-0.5 rounded bg-[var(--bg-panel)] text-[var(--fg-tertiary)] border border-[var(--border-default)]">
                    ⌥T
                  </kbd>
                </button>

                {/* Open Customizations / Settings */}
                {onOpenSettings && (
                  <button
                    onClick={onOpenSettings}
                    className="w-full text-left p-2.5 rounded-xl hover:bg-[var(--bg-panel)] border border-transparent hover:border-[var(--border-default)] flex items-start justify-between transition-colors group cursor-pointer"
                  >
                    <div className="flex items-start gap-2.5">
                      <div className="p-1 rounded-lg bg-purple-500/10 text-purple-400 mt-0.5">
                        <IconSettings className="size-3.5" />
                      </div>
                      <div>
                        <p className="font-semibold text-[var(--fg-primary)]">Customizations</p>
                        <p className="text-[10px] text-[var(--fg-tertiary)]">Manage MCP tools, skills & rules</p>
                      </div>
                    </div>
                  </button>
                )}

                {/* Switch to Editor */}
                {onSwitchToEditor && (
                  <button
                    onClick={onSwitchToEditor}
                    className="w-full text-left p-2.5 rounded-xl hover:bg-[var(--bg-panel)] border border-transparent hover:border-[var(--border-default)] flex items-start justify-between transition-colors group cursor-pointer"
                  >
                    <div className="flex items-start gap-2.5">
                      <div className="p-1 rounded-lg bg-emerald-500/10 text-emerald-400 mt-0.5">
                        <IconCode className="size-3.5" />
                      </div>
                      <div>
                        <p className="font-semibold text-[var(--fg-primary)]">Switch to Editor</p>
                        <p className="text-[10px] text-[var(--fg-tertiary)]">Full workspace code editor</p>
                      </div>
                    </div>
                  </button>
                )}
              </div>
            </div>

            <div className="pt-3 border-t border-[var(--border-default)] text-[10px] text-[var(--fg-tertiary)] flex items-center justify-between">
              <span>Runtime: Native Subprocess</span>
              <span className="font-mono">v0.8.5</span>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
