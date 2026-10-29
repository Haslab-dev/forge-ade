import React, { useEffect, useMemo, useRef, useState } from 'react';
import { ArrowLeft, Check, Folder, Loader2, Plus, Search, Sparkles, Terminal as TerminalIcon } from 'lucide-react';
import { useSessionStore } from '../../stores/sessionStore';
import { useWorkspace } from '../../stores/workspaceStore';
import { DEFAULT_AGENTS } from '../../stores/agentRegistryStore';
import { ApiBridge } from '../../services/apiBridge';
import type { ACPAgent, AgentCLIConfig } from '../../types';
import { cn } from '../../lib/utils';
import { ProviderIcon } from '../agent/ProviderIcon';

/**
 * Unified New Task modal:
 * Grouped into:
 *   1. Agent ACP (Agent Client Protocol & Native autonomous agents)
 *   2. Agent CLI (Interactive terminal PTY sessions)
 * Followed by Workspace selection → Launch.
 */

const FALLBACK_AGENTS: AgentCLIConfig[] = [
  { id: 'commandcode', name: 'Command Code', executable: 'cmd', args: [], enabled: true },
  { id: 'ohmypi', name: 'OhMyPi', executable: 'omp', args: [], enabled: true },
  { id: 'opencode', name: 'OpenCode', executable: 'opencode', args: [], enabled: true },
  { id: 'pi', name: 'Pi', executable: 'pi', args: [], enabled: true },
  { id: 'antigravity', name: 'Antigravity CLI', executable: 'agy', args: [], enabled: true },
  { id: 'codex', name: 'Codex', executable: 'codex', args: [], enabled: true },
  { id: 'claude-code', name: 'Claude Code', executable: 'claude', args: [], enabled: true }
];

export const NewTaskModal: React.FC = () => {
  const { isNewTaskOpen, closeNewTask, agentConfigs, createSession, newTaskWorkspacePath } = useSessionStore();
  const {
    recentWorkspaces,
    setActiveWorkspacePath,
    activeWorkspacePath,
    setActiveTaskKind,
    setActiveSessionId,
    setMode,
    switchSurface,
    mode,
    agents: workspaceAgents,
    setActiveAgentId,
    createNewSession
  } = useWorkspace();

  const [step, setStep] = useState<'agent' | 'workspace'>('agent');
  const [selectedAgent, setSelectedAgent] = useState<
    | { kind: 'acp'; agent: ACPAgent }
    | { kind: 'cli'; agent: AgentCLIConfig }
    | null
  >(null);
  const [activeTab, setActiveTab] = useState<'all' | 'acp' | 'cli'>('all');
  const [searchQuery, setSearchQuery] = useState('');
  const [title, setTitle] = useState('');
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState('');
  const [acpUninstalledOpen, setAcpUninstalledOpen] = useState(false);
  const [cliUninstalledOpen, setCliUninstalledOpen] = useState(false);
  const inputRef = useRef<HTMLInputElement>(null);

  // Agent ACP list (dynamic from workspaceStore, fallback to DEFAULT_AGENTS)
  const allAcpAgents = useMemo(() => {
    const list = workspaceAgents && workspaceAgents.length > 0 ? workspaceAgents : DEFAULT_AGENTS;
    return list.filter(a => a.enabled !== false);
  }, [workspaceAgents]);

  const availableAcpAgents = useMemo(() => {
    return allAcpAgents.filter(a => a.available !== false || a.type === 'internal');
  }, [allAcpAgents]);

  const uninstalledAcpAgents = useMemo(() => {
    return allAcpAgents.filter(a => a.available === false && a.type !== 'internal');
  }, [allAcpAgents]);

  // Agent CLI list (dynamic from sessionStore, fallback to FALLBACK_AGENTS)
  const allCliAgents = useMemo(() => {
    return (agentConfigs.length > 0 ? agentConfigs : FALLBACK_AGENTS).filter(a => a.enabled !== false);
  }, [agentConfigs]);

  const availableCliAgents = useMemo(() => {
    return allCliAgents.filter(a => a.available !== false);
  }, [allCliAgents]);

  const uninstalledCliAgents = useMemo(() => {
    return allCliAgents.filter(a => a.available === false);
  }, [allCliAgents]);

  const q = searchQuery.trim().toLowerCase();

  const filterAcpFn = (a: ACPAgent) => {
    if (!q) return true;
    return (
      a.name.toLowerCase().includes(q) ||
      (a.description && a.description.toLowerCase().includes(q)) ||
      (a.command && a.command.toLowerCase().includes(q)) ||
      (a.model && a.model.toLowerCase().includes(q)) ||
      (a.provider && a.provider.toLowerCase().includes(q)) ||
      a.id.toLowerCase().includes(q)
    );
  };

  const filterCliFn = (a: AgentCLIConfig) => {
    if (!q) return true;
    return (
      a.name.toLowerCase().includes(q) ||
      (a.executable && a.executable.toLowerCase().includes(q)) ||
      a.id.toLowerCase().includes(q)
    );
  };

  const filteredAvailableAcpAgents = useMemo(() => availableAcpAgents.filter(filterAcpFn), [availableAcpAgents, q]);
  const filteredUninstalledAcpAgents = useMemo(() => uninstalledAcpAgents.filter(filterAcpFn), [uninstalledAcpAgents, q]);

  const filteredAvailableCliAgents = useMemo(() => availableCliAgents.filter(filterCliFn), [availableCliAgents, q]);
  const filteredUninstalledCliAgents = useMemo(() => uninstalledCliAgents.filter(filterCliFn), [uninstalledCliAgents, q]);

  useEffect(() => {
    if (isNewTaskOpen) {
      setStep('agent');
      setSelectedAgent(null);
      setActiveTab('all');
      setAcpUninstalledOpen(false);
      setCliUninstalledOpen(false);
      setSearchQuery('');
      setTitle('');
      setError('');
      setCreating(false);
      setTimeout(() => inputRef.current?.focus(), 40);
    }
  }, [isNewTaskOpen]);

  useEffect(() => {
    if (!isNewTaskOpen) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.preventDefault();
        closeNewTask();
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [isNewTaskOpen, closeNewTask]);

  if (!isNewTaskOpen) return null;

  const workspaceName = (p: string) => p.split('/').filter(Boolean).pop() || p;

  const handleSelectACPAgent = (ag: ACPAgent, workspacePath?: string, promptOrTitle?: string) => {
    const ws = workspacePath || newTaskWorkspacePath || activeWorkspacePath;
    setActiveAgentId(ag.id);
    setActiveTaskKind('forge');
    if (ws && ws !== activeWorkspacePath) {
      setActiveWorkspacePath(ws);
    }
    if (mode === 'editor' || mode === 'settings') switchSurface('agent');
    else setMode('agent');

    if (promptOrTitle && promptOrTitle.trim()) {
      createNewSession(promptOrTitle.trim(), ag.id);
    } else {
      setActiveSessionId(null);
    }
    closeNewTask();
  };

  const handleCreateCLI = async (workspacePath: string, ag: AgentCLIConfig, customTitle?: string) => {
    if (creating) return;
    setCreating(true);
    setError('');
    const sessionTitle = (customTitle !== undefined ? customTitle : title).trim();
    try {
      const created = await createSession(ag.id, workspacePath, sessionTitle);
      if (created) {
        if (mode === 'editor' || mode === 'settings') switchSurface('agent');
        setActiveTaskKind('cli');
        if (workspacePath && workspacePath !== activeWorkspacePath) {
          setActiveWorkspacePath(workspacePath);
        }
        setMode('agent');
        closeNewTask();
        return;
      }
      const reason = useSessionStore.getState().lastCreateError;
      setError(reason || 'Could not start the session. Check the agent CLI path in Settings → Agent CLIs.');
    } finally {
      setCreating(false);
    }
  };

  const handlePickFolder = async () => {
    try {
      const picked = await ApiBridge.pickNativeDirectory();
      if (picked?.path) {
        if (selectedAgent?.kind === 'acp') {
          handleSelectACPAgent(selectedAgent.agent, picked.path, title.trim());
        } else if (selectedAgent?.kind === 'cli') {
          await handleCreateCLI(picked.path, selectedAgent.agent);
        }
      }
    } catch (e) {
      console.warn('folder pick failed', e);
    }
  };

  return (
    <div
      className="fixed inset-0 z-[80] flex items-start justify-center bg-black/55 pt-[10vh] backdrop-blur-[2px]"
      role="dialog"
      aria-modal="true"
      aria-label="New Task"
      data-testid="new-task-modal"
      onMouseDown={e => {
        if (e.target === e.currentTarget) closeNewTask();
      }}
    >
      <div className="w-[580px] max-w-[94vw] overflow-hidden rounded-xl border border-border bg-sidebar shadow-2xl flex flex-col max-h-[84vh]">
        {/* Header */}
        <div className="flex items-center gap-2 border-b border-border/60 px-4 py-3 shrink-0">
          {step === 'workspace' && (
            <button
              type="button"
              onClick={() => setStep('agent')}
              className="flex size-6 items-center justify-center rounded-md text-foreground-subtle transition-colors hover:bg-surface-hover hover:text-foreground cursor-pointer"
              aria-label="Back to agent selection"
            >
              <ArrowLeft className="size-4" />
            </button>
          )}
          <div className="flex items-center gap-2 min-w-0 flex-1">
            <TerminalIcon className="size-4 shrink-0 text-foreground-subtle" />
            <span className="truncate text-ui-base font-medium text-foreground">
              {step === 'agent'
                ? newTaskWorkspacePath
                  ? `New Task — ${workspaceName(newTaskWorkspacePath)}`
                  : 'New Task'
                : 'New Task — Select Workspace'}
            </span>
          </div>
          {/* Step indicator */}
          {!newTaskWorkspacePath && (
            <div className="ml-auto flex items-center gap-1.5 shrink-0">
              <span className={cn('size-1.5 rounded-full', step === 'agent' ? 'bg-foreground' : 'bg-foreground-subtlest')} />
              <span className={cn('size-1.5 rounded-full', step === 'workspace' ? 'bg-foreground' : 'bg-foreground-subtlest')} />
            </div>
          )}
        </div>

        {step === 'agent' ? (
          <div className="flex flex-col min-h-0 flex-1">
            {/* Search & Segmented Filter */}
            <div className="flex flex-col border-b border-border/50 bg-panel/30 shrink-0">
              <div className="flex items-center gap-2 px-3 py-2">
                <Search className="size-3.5 shrink-0 text-foreground-subtle" />
                <input
                  ref={inputRef}
                  type="text"
                  value={searchQuery}
                  onChange={e => setSearchQuery(e.target.value)}
                  placeholder="Filter agents by name, command, or protocol..."
                  className="h-6 flex-1 bg-transparent text-ui-xs text-foreground placeholder:text-foreground-subtlest outline-none"
                />
                {searchQuery && (
                  <button
                    type="button"
                    onClick={() => setSearchQuery('')}
                    className="text-ui-xs text-foreground-subtle hover:text-foreground cursor-pointer px-1"
                  >
                    Clear
                  </button>
                )}
              </div>
              <div className="flex items-center gap-1 px-3 pb-2 pt-0.5">
                <button
                  type="button"
                  onClick={() => setActiveTab('all')}
                  className={cn(
                    'px-2.5 py-1 rounded-md text-ui-xs font-medium transition-colors cursor-pointer',
                    activeTab === 'all'
                      ? 'bg-panel border border-border/80 text-foreground shadow-xs'
                      : 'text-foreground-subtle hover:text-foreground hover:bg-surface-hover'
                  )}
                >
                  All ({filteredAvailableAcpAgents.length + filteredAvailableCliAgents.length})
                </button>
                <button
                  type="button"
                  onClick={() => setActiveTab('acp')}
                  className={cn(
                    'px-2.5 py-1 rounded-md text-ui-xs font-medium transition-colors cursor-pointer flex items-center gap-1.5',
                    activeTab === 'acp'
                      ? 'bg-panel border border-border/80 text-foreground shadow-xs'
                      : 'text-foreground-subtle hover:text-foreground hover:bg-surface-hover'
                  )}
                >
                  <span>Agent ACP</span>
                  <span className="rounded-full bg-accent-primary/20 text-accent-primary px-1.5 py-0.2 text-[10px] font-bold">
                    {filteredAvailableAcpAgents.length}
                  </span>
                </button>
                <button
                  type="button"
                  onClick={() => setActiveTab('cli')}
                  className={cn(
                    'px-2.5 py-1 rounded-md text-ui-xs font-medium transition-colors cursor-pointer flex items-center gap-1.5',
                    activeTab === 'cli'
                      ? 'bg-panel border border-border/80 text-foreground shadow-xs'
                      : 'text-foreground-subtle hover:text-foreground hover:bg-surface-hover'
                  )}
                >
                  <span>Agent CLI</span>
                  <span className="rounded-full bg-surface text-foreground-subtle border border-border/50 px-1.5 py-0.2 text-[10px] font-semibold">
                    {filteredAvailableCliAgents.length}
                  </span>
                </button>
              </div>
            </div>

            {/* Scrollable Agent Groups */}
            <div className="flex flex-col p-2 overflow-y-auto min-h-0 flex-1">
              {error && (
                <p className="mx-2 my-1 rounded-md bg-destructive/10 px-2.5 py-1.5 text-ui-xs text-destructive">
                  {error}
                </p>
              )}

              {/* 1. AGENT ACP */}
              {(activeTab === 'all' || activeTab === 'acp') && (
                <div className="flex flex-col">
                  <div className="flex items-center justify-between px-2 pt-2 pb-1.5 text-ui-xs font-semibold uppercase tracking-wider text-foreground-subtle">
                    <div className="flex items-center gap-1.5">
                      <span>Agent ACP</span>
                      <span className="rounded bg-accent-primary/10 px-1.5 py-0.2 text-[10px] font-medium normal-case tracking-normal text-accent-primary">
                        Agent Client Protocol & Native
                      </span>
                    </div>
                    <span className="font-mono text-ui-xs font-normal normal-case text-foreground-subtlest">
                      {filteredAvailableAcpAgents.length} available
                    </span>
                  </div>

                  <div className="flex flex-col gap-0.5">
                    {filteredAvailableAcpAgents.map(agent => (
                      <button
                        key={agent.id}
                        type="button"
                        onClick={() => {
                          if (newTaskWorkspacePath) {
                            handleSelectACPAgent(agent, newTaskWorkspacePath);
                          } else {
                            setSelectedAgent({ kind: 'acp', agent });
                            setStep('workspace');
                          }
                        }}
                        className="group flex cursor-pointer items-center gap-3 rounded-lg px-2.5 py-2 text-left transition-colors hover:bg-surface-hover"
                        data-testid={agent.id === 'agent-internal' ? 'new-task-agent-forgeade' : `new-task-agent-${agent.id}`}
                      >
                        <div className="flex size-8 shrink-0 items-center justify-center rounded-lg border border-border bg-background">
                          {agent.id === 'agent-internal' ? (
                            <Sparkles className="size-4 text-primary" />
                          ) : (
                            <ProviderIcon provider={agent.icon || agent.provider || agent.id} size={16} className="text-foreground-subtle group-hover:text-foreground" />
                          )}
                        </div>
                        <div className="min-w-0 flex-1">
                          <div className="flex items-center gap-2">
                            <span className="text-ui-base font-medium text-foreground truncate">{agent.name}</span>
                            {agent.type === 'internal' ? (
                              <span className="rounded border border-border/80 bg-surface px-1.5 py-0.2 text-[10px] font-medium uppercase tracking-wider text-foreground-subtle">
                                Internal
                              </span>
                            ) : (
                              <span className="rounded border border-accent-primary/30 bg-accent-primary/15 px-1.5 py-0.2 text-[10px] font-bold uppercase tracking-wider text-accent-primary">
                                ACP
                              </span>
                            )}
                          </div>
                          <div className="text-ui-xs text-foreground-subtlest truncate">
                            {agent.description || (agent.type === 'internal' ? 'Internal agent — chat with full workspace tools' : 'Connected via Agent Client Protocol')}
                          </div>
                        </div>
                        {agent.model && (
                          <code className="hidden sm:inline-block shrink-0 max-w-[140px] truncate rounded bg-background px-1.5 py-0.5 font-mono text-ui-xs text-foreground-subtle border border-border/40" title={agent.model}>
                            {agent.model.split('/').pop()}
                          </code>
                        )}
                      </button>
                    ))}
                    {filteredAvailableAcpAgents.length === 0 && (
                      <p className="px-3 py-3 text-center text-ui-xs text-foreground-subtlest">
                        {searchQuery ? `No ACP agents match "${searchQuery}"` : 'No ACP agents configured.'}
                      </p>
                    )}

                    {filteredUninstalledAcpAgents.length > 0 && (
                      <div className="mt-1 border-t border-border/40 pt-1">
                        <button
                          type="button"
                          onClick={() => setAcpUninstalledOpen(prev => !prev)}
                          className="flex w-full items-center justify-between rounded-md px-2.5 py-1 text-3xs font-medium text-foreground-subtle hover:bg-surface-hover hover:text-foreground cursor-pointer transition-colors"
                        >
                          <span>{acpUninstalledOpen ? 'Hide uninstalled ACP agents' : `Other ACP agents (${filteredUninstalledAcpAgents.length} not installed)`}</span>
                          <span className="text-[10px]">{acpUninstalledOpen ? '▲' : '▼'}</span>
                        </button>
                        {acpUninstalledOpen && (
                          <div className="mt-1 flex flex-col gap-0.5">
                            {filteredUninstalledAcpAgents.map(agent => (
                              <button
                                key={agent.id}
                                type="button"
                                onClick={() => {
                                  if (newTaskWorkspacePath) {
                                    handleSelectACPAgent(agent, newTaskWorkspacePath);
                                  } else {
                                    setSelectedAgent({ kind: 'acp', agent });
                                    setStep('workspace');
                                  }
                                }}
                                className="group flex cursor-pointer items-center gap-3 rounded-lg px-2.5 py-2 text-left transition-colors hover:bg-surface-hover opacity-60 hover:opacity-100"
                                data-testid={`new-task-agent-${agent.id}`}
                              >
                                <div className="flex size-8 shrink-0 items-center justify-center rounded-lg border border-border bg-background">
                                  <ProviderIcon provider={agent.icon || agent.provider || agent.id} size={16} className="text-foreground-subtle group-hover:text-foreground grayscale" />
                                </div>
                                <div className="min-w-0 flex-1">
                                  <div className="flex items-center gap-2">
                                    <span className="text-ui-base font-medium text-foreground truncate">{agent.name}</span>
                                    <span className="rounded border border-accent-primary/30 bg-accent-primary/15 px-1.5 py-0.2 text-[10px] font-bold uppercase tracking-wider text-accent-primary">
                                      ACP
                                    </span>
                                    <span className="rounded bg-destructive/15 px-1.5 py-0.2 text-[10px] font-medium text-destructive">
                                      Not Installed
                                    </span>
                                  </div>
                                  <div className="text-ui-xs text-foreground-subtlest truncate">
                                    {agent.command ? `CLI executable "${agent.command}" not found on PATH` : agent.description}
                                  </div>
                                </div>
                                {agent.model && (
                                  <code className="hidden sm:inline-block shrink-0 max-w-[140px] truncate rounded bg-background px-1.5 py-0.5 font-mono text-ui-xs text-foreground-subtle border border-border/40" title={agent.model}>
                                    {agent.model.split('/').pop()}
                                  </code>
                                )}
                              </button>
                            ))}
                          </div>
                        )}
                      </div>
                    )}
                  </div>
                </div>
              )}

              {/* DIVIDER */}
              {activeTab === 'all' && (
                <div className="mx-2 my-2.5 h-px bg-border/60" />
              )}

              {/* 2. AGENT CLI */}
              {(activeTab === 'all' || activeTab === 'cli') && (
                <div className="flex flex-col">
                  <div className="flex items-center justify-between px-2 pt-1 pb-1.5 text-ui-xs font-semibold uppercase tracking-wider text-foreground-subtle">
                    <div className="flex items-center gap-1.5">
                      <span>Agent CLI</span>
                      <span className="rounded bg-surface px-1.5 py-0.2 text-[10px] font-medium normal-case tracking-normal text-foreground-subtle border border-border/50">
                        Terminal PTY Session
                      </span>
                    </div>
                    <span className="font-mono text-ui-xs font-normal normal-case text-foreground-subtlest">
                      {filteredAvailableCliAgents.length} available
                    </span>
                  </div>

                  <div className="flex flex-col gap-0.5">
                    {filteredAvailableCliAgents.map(agent => (
                      <button
                        key={agent.id}
                        type="button"
                        onClick={() => {
                          if (newTaskWorkspacePath) {
                            void handleCreateCLI(newTaskWorkspacePath, agent);
                          } else {
                            setSelectedAgent({ kind: 'cli', agent });
                            setStep('workspace');
                          }
                        }}
                        className="group flex cursor-pointer items-center gap-3 rounded-lg px-2.5 py-2 text-left transition-colors hover:bg-surface-hover"
                        data-testid={`new-task-agent-${agent.id}`}
                      >
                        <div className="flex size-8 shrink-0 items-center justify-center rounded-lg border border-border bg-background">
                          <ProviderIcon provider={agent.id} size={16} className="text-foreground-subtle group-hover:text-foreground" />
                        </div>
                        <div className="min-w-0 flex-1">
                          <div className="flex items-center gap-2">
                            <span className="text-ui-base font-medium text-foreground truncate">{agent.name}</span>
                            <span className="rounded border border-border/80 bg-surface px-1.5 py-0.2 text-[10px] font-medium uppercase tracking-wider text-foreground-subtle">
                              CLI
                            </span>
                          </div>
                          <div className="text-ui-xs text-foreground-subtlest truncate">
                            Interactive shell session in workspace
                          </div>
                        </div>
                        <code className="shrink-0 rounded bg-background px-1.5 py-0.5 font-mono text-ui-xs text-foreground-subtle border border-border/40">
                          {agent.executable}
                        </code>
                      </button>
                    ))}
                    {filteredAvailableCliAgents.length === 0 && (
                      <p className="px-3 py-3 text-center text-ui-xs text-foreground-subtlest">
                        {searchQuery ? `No CLI agents match "${searchQuery}"` : 'No agent CLIs enabled — turn one on in Settings → Agent CLIs.'}
                      </p>
                    )}

                    {filteredUninstalledCliAgents.length > 0 && (
                      <div className="mt-1 border-t border-border/40 pt-1">
                        <button
                          type="button"
                          onClick={() => setCliUninstalledOpen(prev => !prev)}
                          className="flex w-full items-center justify-between rounded-md px-2.5 py-1 text-3xs font-medium text-foreground-subtle hover:bg-surface-hover hover:text-foreground cursor-pointer transition-colors"
                        >
                          <span>{cliUninstalledOpen ? 'Hide uninstalled CLI agents' : `Other CLI agents (${filteredUninstalledCliAgents.length} not installed)`}</span>
                          <span className="text-[10px]">{cliUninstalledOpen ? '▲' : '▼'}</span>
                        </button>
                        {cliUninstalledOpen && (
                          <div className="mt-1 flex flex-col gap-0.5">
                            {filteredUninstalledCliAgents.map(agent => (
                              <button
                                key={agent.id}
                                type="button"
                                onClick={() => {
                                  if (newTaskWorkspacePath) {
                                    void handleCreateCLI(newTaskWorkspacePath, agent);
                                  } else {
                                    setSelectedAgent({ kind: 'cli', agent });
                                    setStep('workspace');
                                  }
                                }}
                                className="group flex cursor-pointer items-center gap-3 rounded-lg px-2.5 py-2 text-left transition-colors hover:bg-surface-hover opacity-60 hover:opacity-100"
                                data-testid={`new-task-agent-${agent.id}`}
                              >
                                <div className="flex size-8 shrink-0 items-center justify-center rounded-lg border border-border bg-background">
                                  <ProviderIcon provider={agent.id} size={16} className="text-foreground-subtle group-hover:text-foreground grayscale" />
                                </div>
                                <div className="min-w-0 flex-1">
                                  <div className="flex items-center gap-2">
                                    <span className="text-ui-base font-medium text-foreground truncate">{agent.name}</span>
                                    <span className="rounded border border-border/80 bg-surface px-1.5 py-0.2 text-[10px] font-medium uppercase tracking-wider text-foreground-subtle">
                                      CLI
                                    </span>
                                    <span className="rounded bg-destructive/15 px-1.5 py-0.2 text-[10px] font-medium text-destructive">
                                      Not Installed
                                    </span>
                                  </div>
                                  <div className="text-ui-xs text-foreground-subtlest truncate">
                                    CLI executable &quot;{agent.executable}&quot; not found on PATH
                                  </div>
                                </div>
                                <code className="shrink-0 rounded bg-background px-1.5 py-0.5 font-mono text-ui-xs text-foreground-subtle border border-border/40">
                                  {agent.executable}
                                </code>
                              </button>
                            ))}
                          </div>
                        )}
                      </div>
                    )}
                  </div>
                </div>
              )}
            </div>
          </div>
        ) : (
          <div className="flex flex-col gap-3 p-4 overflow-y-auto">
            {/* Selected Agent Header Banner */}
            <div className="flex items-center gap-3 rounded-lg border border-border/80 bg-panel/50 p-2.5">
              <div className="flex size-8 shrink-0 items-center justify-center rounded-lg border border-border bg-background">
                {selectedAgent?.kind === 'acp' ? (
                  selectedAgent.agent.id === 'agent-internal' ? (
                    <Sparkles className="size-4 text-primary" />
                  ) : (
                    <ProviderIcon provider={selectedAgent.agent.icon || selectedAgent.agent.provider || selectedAgent.agent.id} size={16} />
                  )
                ) : selectedAgent?.agent ? (
                  <ProviderIcon provider={selectedAgent.agent.id} size={16} />
                ) : (
                  <TerminalIcon className="size-4 text-foreground-subtle" />
                )}
              </div>
              <div className="min-w-0 flex-1">
                <div className="flex items-center gap-2">
                  <span className="text-ui-base font-medium text-foreground">
                    {selectedAgent?.kind === 'acp' ? selectedAgent.agent.name : selectedAgent?.agent.name}
                  </span>
                  {selectedAgent?.kind === 'acp' ? (
                    selectedAgent.agent.type === 'internal' ? (
                      <span className="rounded border border-border/80 bg-surface px-1.5 py-0.2 text-[10px] font-medium uppercase text-foreground-subtle">
                        Internal
                      </span>
                    ) : (
                      <span className="rounded border border-accent-primary/30 bg-accent-primary/15 px-1.5 py-0.2 text-[10px] font-bold uppercase text-accent-primary">
                        Agent ACP
                      </span>
                    )
                  ) : (
                    <span className="rounded border border-border/80 bg-surface px-1.5 py-0.2 text-[10px] font-medium uppercase text-foreground-subtle">
                      Agent CLI
                    </span>
                  )}
                </div>
                <div className="text-ui-xs text-foreground-subtlest truncate">
                  {selectedAgent?.kind === 'acp'
                    ? (selectedAgent.agent.description || 'Autonomous agent chat with full workspace tools')
                    : `Terminal PTY session · ${selectedAgent?.agent.executable}`}
                </div>
              </div>
              {selectedAgent?.kind === 'cli' && (
                <code className="shrink-0 rounded bg-background px-1.5 py-0.5 font-mono text-ui-xs text-foreground-subtle border border-border/40">
                  {selectedAgent.agent.executable}
                </code>
              )}
            </div>

            {/* Optional session name / topic */}
            <div className="flex flex-col gap-1.5">
              <label htmlFor="new-task-title" className="text-ui-sm text-foreground-subtle">
                {selectedAgent?.kind === 'acp' ? 'Session topic or prompt' : 'Session name'} <span className="text-ui-xs text-foreground-subtlest">(optional)</span>
              </label>
              <input
                id="new-task-title"
                ref={inputRef}
                value={title}
                onChange={e => setTitle(e.target.value)}
                placeholder={
                  selectedAgent?.kind === 'acp'
                    ? 'e.g. Plan and implement new feature'
                    : `${selectedAgent?.agent.name ?? 'Session'} · ${workspaceName(activeWorkspacePath || '') || 'workspace'}`
                }
                onKeyDown={e => {
                  if (e.key === 'Enter' && activeWorkspacePath) {
                    if (selectedAgent?.kind === 'acp') {
                      handleSelectACPAgent(selectedAgent.agent, activeWorkspacePath, title.trim());
                    } else if (selectedAgent?.kind === 'cli') {
                      void handleCreateCLI(activeWorkspacePath, selectedAgent.agent);
                    }
                  }
                }}
                className="h-9 rounded-lg border border-border bg-background px-3 text-ui-sm text-foreground outline-none transition-colors placeholder:text-foreground-subtlest focus:border-input-border-focused"
              />
            </div>

            {/* Recent workspaces */}
            <div className="flex flex-col gap-1.5">
              <span className="text-ui-sm text-foreground-subtle">Workspace</span>
              <div className="max-h-52 overflow-y-auto rounded-lg border border-border/60 bg-panel/30">
                {recentWorkspaces.filter(p => !p.startsWith('ssh://')).map(p => (
                  <button
                    key={p}
                    type="button"
                    disabled={creating}
                    onClick={() => {
                      if (selectedAgent?.kind === 'acp') {
                        handleSelectACPAgent(selectedAgent.agent, p, title.trim());
                      } else if (selectedAgent?.kind === 'cli') {
                        void handleCreateCLI(p, selectedAgent.agent);
                      }
                    }}
                    className="flex w-full cursor-pointer items-center gap-2.5 border-b border-border/40 px-3 py-2 text-left transition-colors last:border-b-0 hover:bg-surface-hover disabled:opacity-50"
                    data-testid={`new-task-workspace-${workspaceName(p)}`}
                  >
                    <Folder className="size-3.5 shrink-0 text-foreground-subtle" />
                    <span className="min-w-0 flex-1 truncate text-ui-base text-foreground">{workspaceName(p)}</span>
                    <span className="min-w-0 max-w-[45%] shrink-0 truncate font-mono text-ui-xs text-foreground-subtlest" title={p}>
                      {p}
                    </span>
                    {p === activeWorkspacePath && <Check className="size-3.5 shrink-0 text-accent-primary" />}
                  </button>
                ))}
                {recentWorkspaces.length === 0 && (
                  <p className="px-3 py-4 text-center text-ui-xs text-foreground-subtlest">No recent folders</p>
                )}
              </div>
            </div>

            {error && <p className="text-ui-xs text-destructive">{error}</p>}

            {/* Actions */}
            <div className="flex items-center justify-between gap-2 pt-1">
              <button
                type="button"
                disabled={creating}
                onClick={() => void handlePickFolder()}
                className="flex h-8 items-center gap-1.5 rounded-lg border border-border px-3 text-ui-xs font-medium text-foreground-subtle transition-colors hover:bg-surface-hover hover:text-foreground disabled:opacity-60 cursor-pointer"
              >
                <Plus className="size-3.5" />
                Choose folder…
              </button>
              <button
                type="button"
                disabled={creating || !activeWorkspacePath}
                onClick={() => {
                  if (selectedAgent?.kind === 'acp') {
                    handleSelectACPAgent(selectedAgent.agent, activeWorkspacePath, title.trim());
                  } else if (selectedAgent?.kind === 'cli') {
                    void handleCreateCLI(activeWorkspacePath, selectedAgent.agent);
                  }
                }}
                className="flex h-8 items-center gap-1.5 rounded-lg bg-brand px-4 text-ui-xs font-medium text-foreground-inverse transition-colors hover:bg-brand/80 disabled:opacity-60 cursor-pointer"
                title={activeWorkspacePath ? `Start in ${workspaceName(activeWorkspacePath)}` : 'Open a workspace first'}
              >
                {creating ? (
                  <Loader2 className="size-3.5 animate-spin" />
                ) : selectedAgent?.kind === 'acp' ? (
                  <Sparkles className="size-3.5" />
                ) : (
                  <TerminalIcon className="size-3.5" />
                )}
                {selectedAgent?.kind === 'acp' ? 'Start Task' : 'Create Session'}
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
};

export default NewTaskModal;
