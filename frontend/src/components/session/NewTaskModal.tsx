import React, { useEffect, useMemo, useRef, useState } from 'react';
import { ArrowLeft, Check, Folder, Loader2, Plus, Terminal as TerminalIcon } from 'lucide-react';
import { useSessionStore } from '../../stores/sessionStore';
import { useWorkspace } from '../../stores/workspaceStore';
import { ApiBridge } from '../../services/apiBridge';
import type { AgentCLIConfig } from '../../types';
import { cn } from '../../lib/utils';
import { ProviderIcon } from '../agent/ProviderIcon';

/**
 * New Task flow for Terminal Session mode:
 *   Select Agent → Select Workspace → Create Session → Spawn CLI → Open Terminal
 */

const FALLBACK_AGENTS: AgentCLIConfig[] = [
  { id: 'pi', name: 'Pi', executable: 'pi', args: [], enabled: true },
  { id: 'ohmypi', name: 'OhMyPi', executable: 'omp', args: [], enabled: true },
  { id: 'opencode', name: 'OpenCode', executable: 'opencode', args: [], enabled: true },
  { id: 'antigravity', name: 'Antigravity CLI', executable: 'agy', args: [], enabled: true },
  { id: 'codex', name: 'Codex', executable: 'codex', args: [], enabled: true },
  { id: 'claude-code', name: 'Claude Code', executable: 'claude', args: [], enabled: true }
];

export const NewTaskModal: React.FC = () => {
  const { isNewTaskOpen, closeNewTask, agentConfigs, createSession, newTaskWorkspacePath } = useSessionStore();
  const { recentWorkspaces, setActiveWorkspacePath, activeWorkspacePath } = useWorkspace();

  const [step, setStep] = useState<'agent' | 'workspace'>('agent');
  const [selectedAgent, setSelectedAgent] = useState<AgentCLIConfig | null>(null);
  const [title, setTitle] = useState('');
  const [creating, setCreating] = useState(false);
  const [error, setError] = useState('');
  const inputRef = useRef<HTMLInputElement>(null);

  const agents = useMemo(
    () => (agentConfigs.length > 0 ? agentConfigs.filter(a => a.enabled) : FALLBACK_AGENTS),
    [agentConfigs]
  );

  useEffect(() => {
    if (isNewTaskOpen) {
      setStep('agent');
      setSelectedAgent(null);
      setTitle('');
      setError('');
      setCreating(false);
      setTimeout(() => inputRef.current?.focus(), 30);
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

  const handlePickFolder = async () => {
    try {
      const picked = await ApiBridge.pickNativeDirectory();
      if (picked?.path) {
        await handleCreate(picked.path);
      }
    } catch (e) {
      console.warn('folder pick failed', e);
    }
  };

  const handleCreate = async (workspacePath: string, agentOverride?: AgentCLIConfig) => {
    const ag = agentOverride || selectedAgent;
    if (!ag || creating) return;
    setCreating(true);
    setError('');
    try {
      const created = await createSession(ag.id, workspacePath, title.trim());
      if (created) {
        if (workspacePath && workspacePath !== activeWorkspacePath) {
          setActiveWorkspacePath(workspacePath);
        }
        closeNewTask();
        return;
      }
      setError('Could not start the session. Check the agent CLI path in Settings → Agent CLIs.');
    } finally {
      setCreating(false);
    }
  };

  return (
    <div
      className="fixed inset-0 z-[80] flex items-start justify-center bg-black/50 pt-[12vh] backdrop-blur-[2px]"
      role="dialog"
      aria-modal="true"
      aria-label="New Task"
      data-testid="new-task-modal"
      onMouseDown={e => {
        if (e.target === e.currentTarget) closeNewTask();
      }}
    >
      <div className="w-[520px] max-w-[92vw] overflow-hidden rounded-xl border border-border bg-sidebar shadow-2xl">
        {/* Header */}
        <div className="flex items-center gap-2 border-b border-border/60 px-4 py-3">
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
          <div className="flex items-center gap-2">
            <TerminalIcon className="size-4 text-foreground-subtle" />
            <span className="text-ui-base font-medium text-foreground">
              {step === 'agent'
                ? newTaskWorkspacePath
                  ? `New Task — ${workspaceName(newTaskWorkspacePath)}`
                  : 'New Task — Choose Agent'
                : 'New Task — Select Workspace'}
            </span>
          </div>
          {/* Step indicator (only when picking workspace from root sidebar) */}
          {!newTaskWorkspacePath && (
            <div className="ml-auto flex items-center gap-1.5">
              <span className={cn('size-1.5 rounded-full', step === 'agent' ? 'bg-foreground' : 'bg-foreground-subtlest')} />
              <span className={cn('size-1.5 rounded-full', step === 'workspace' ? 'bg-foreground' : 'bg-foreground-subtlest')} />
            </div>
          )}
        </div>

        {step === 'agent' ? (
          <div className="flex flex-col gap-1 p-2">
            {error && (
              <p className="mx-2 my-1 rounded-md bg-destructive/10 px-2.5 py-1.5 text-ui-xs text-destructive">
                {error}
              </p>
            )}
            {agents.map(agent => (
              <button
                key={agent.id}
                type="button"
                onClick={() => {
                  setSelectedAgent(agent);
                  if (newTaskWorkspacePath) {
                    void handleCreate(newTaskWorkspacePath, agent);
                  } else {
                    setStep('workspace');
                  }
                }}
                className="group flex cursor-pointer items-center gap-3 rounded-lg px-3 py-2.5 text-left transition-colors hover:bg-surface-hover"
                data-testid={`new-task-agent-${agent.id}`}
              >
                <div className="flex size-8 shrink-0 items-center justify-center rounded-lg border border-border bg-background">
                  <ProviderIcon provider={agent.id} size={16} className="text-foreground-subtle group-hover:text-foreground" />
                </div>
                <div className="min-w-0 flex-1">
                  <div className="text-ui-base font-medium text-foreground">{agent.name}</div>
                  <div className="text-ui-xs text-foreground-subtlest">Terminal Session</div>
                </div>
                <code className="shrink-0 rounded bg-background px-1.5 py-0.5 font-mono text-ui-xs text-foreground-subtle">
                  {agent.executable}
                </code>
              </button>
            ))}
            {agents.length === 0 && (
              <p className="px-3 py-6 text-center text-ui-sm text-foreground-subtle">
                No agent CLIs enabled — turn one on in Settings → Agent CLIs.
              </p>
            )}
          </div>
        ) : (
          <div className="flex flex-col gap-3 p-4">
            {/* Optional session name */}
            <div className="flex flex-col gap-1.5">
              <label htmlFor="new-task-title" className="text-ui-base text-foreground-subtle">
                Session name <span className="text-ui-xs text-foreground-subtlest">(optional)</span>
              </label>
              <input
                id="new-task-title"
                ref={inputRef}
                value={title}
                onChange={e => setTitle(e.target.value)}
                placeholder={`${selectedAgent?.name ?? 'Session'} · ${workspaceName(activeWorkspacePath || '') || 'workspace'}`}
                onKeyDown={e => {
                  if (e.key === 'Enter' && activeWorkspacePath) void handleCreate(activeWorkspacePath);
                }}
                className="h-9 rounded-lg border border-border bg-background px-3 text-ui-sm text-foreground outline-none transition-colors placeholder:text-foreground-subtlest focus:border-input-border-focused"
              />
            </div>

            {/* Recent workspaces */}
            <div className="flex flex-col gap-1.5">
              <span className="text-ui-base text-foreground-subtle">Workspace</span>
              <div className="max-h-56 overflow-y-auto rounded-lg border border-border/60">
                {recentWorkspaces.map(p => (
                  <button
                    key={p}
                    type="button"
                    disabled={creating}
                    onClick={() => void handleCreate(p)}
                    className="flex w-full cursor-pointer items-center gap-2.5 border-b border-border/40 px-3 py-2 text-left transition-colors last:border-b-0 hover:bg-surface-hover disabled:opacity-50"
                    data-testid={`new-task-workspace-${workspaceName(p)}`}
                  >
                    <Folder className="size-3.5 shrink-0 text-foreground-subtle" />
                    <span className="min-w-0 flex-1 truncate text-ui-base text-foreground">{workspaceName(p)}</span>
                    <span className="min-w-0 max-w-[45%] shrink-0 truncate font-mono text-ui-xs text-foreground-subtlest" title={p}>
                      {p}
                    </span>
                    {p === activeWorkspacePath && <Check className="size-3.5 shrink-0 text-foreground-subtle" />}
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
                onClick={() => void handleCreate(activeWorkspacePath)}
                className="flex h-8 items-center gap-1.5 rounded-lg bg-brand px-4 text-ui-xs font-medium text-foreground-inverse transition-colors hover:bg-brand/80 disabled:opacity-60 cursor-pointer"
                title={activeWorkspacePath ? `Create session in ${workspaceName(activeWorkspacePath)}` : 'Open a workspace first'}
              >
                {creating ? <Loader2 className="size-3.5 animate-spin" /> : <TerminalIcon className="size-3.5" />}
                Create Session
              </button>
            </div>
          </div>
        )}
      </div>
    </div>
  );
};

export default NewTaskModal;
