import React, { useEffect, useMemo, useState } from 'react';
import { RotateCcw, Square, Terminal as TerminalIcon } from 'lucide-react';
import { useSessionStore, sessionStatusLabel } from '../../stores/sessionStore';
import { useWorkspace } from '../../stores/workspaceStore';
import { TerminalView } from '../terminal-view';
import { BottomTerminal } from '../agent/BottomTerminal';
import { GetAgentTerminalSessionOutput } from '../../lib/wails';
import type { TerminalSessionRecord } from '../../types';
import { cn } from '../../lib/utils';

/**
 * CLI task surface — renders the terminal of the active agent CLI task inside
 * the unified workspace. The app shell (header + sidebar) stays the shared
 * AgentContainer chrome; this component only owns the terminal:
 *
 *   Main    → terminal (the agent CLI owns the interaction)
 *   Status  → agent + process status
 *
 * Every session terminal stays mounted (hidden when inactive) so scrollback
 * and process output survive switching sessions and modes.
 */

export const TerminalSessionSurface: React.FC = () => {
  const { sessions, activeSessionId, restartSession, stopSession } = useSessionStore();
  const { activeWorkspacePath, isBottomTerminalOpen, setIsBottomTerminalOpen } = useWorkspace();

  const activeSession = useMemo(
    () => sessions.find(s => s.id === activeSessionId),
    [sessions, activeSessionId]
  );

  // Sessions whose terminal must stay mounted: the active session or currently running.
  // Inactive terminated sessions are unmounted to conserve WebKit memory and canvas resources.
  const [mountedIds, setMountedIds] = useState<Set<string>>(() => new Set());
  useEffect(() => {
    setMountedIds(prev => {
      const next = new Set<string>();
      if (activeSessionId) next.add(activeSessionId);
      for (const s of sessions) {
        if (s.status === 'running' || s.status === 'starting') {
          next.add(s.id);
        }
      }
      if (next.size !== prev.size || !Array.from(next).every(id => prev.has(id))) {
        return next;
      }
      return prev;
    });
  }, [sessions, activeSessionId]);

  const projectName = (activeWorkspacePath || '').split('/').filter(Boolean).pop() || 'forge-ade';

  return (
    <div className="relative flex h-full w-full min-w-0 flex-1 flex-col overflow-hidden">
      {/* Terminals — every mounted session stays rendered; the active one shows */}
      <main className="relative min-h-0 flex-1 overflow-hidden bg-app">
        {Array.from(mountedIds).map(id => {
          const s = sessions.find(x => x.id === id);
          const isActive = id === activeSessionId;
          return (
            <div key={id} className={cn('h-full w-full', isActive ? 'block' : 'hidden')}>
              <TerminalView
                sessionId={id}
                isActive={isActive}
                ptyAlive={s?.status === 'running' || s?.status === 'starting'}
                historyProvider={() => GetAgentTerminalSessionOutput(id)}
              />
              {/* The ended overlay only covers the active view. */}
              {isActive && s && (s.status === 'exited' || s.status === 'failed' || s.status === 'terminated') && (
                <SessionEndedOverlay session={s} onRestart={() => void restartSession(s.id)} />
              )}
            </div>
          );
        })}

        {/* Empty state: unified New Task entry point */}
        {!activeSession && <SessionEmptyState />}
      </main>

      {/* Bottom dock terminal (toggled from header or shortcuts) */}
      <BottomTerminal
        open={isBottomTerminalOpen}
        onToggle={() => setIsBottomTerminalOpen(false)}
        workspacePath={activeWorkspacePath || ''}
      />

      {/* Status bar: agent + process status */}
      <footer className="flex h-[26px] shrink-0 items-center justify-between border-t border-border bg-background px-3 text-ui-xs text-foreground-subtle select-none">
        <div className="flex min-w-0 items-center gap-2">
          {activeSession ? (
            <>
              <span className="font-semibold text-foreground">{activeSession.agentName}</span>
              <span className="text-foreground-subtlest">•</span>
              <span className="truncate font-mono text-ui-xs" title={activeSession.workingDirectory}>
                {activeSession.workingDirectory || projectName}
              </span>
              <span className="text-foreground-subtlest">•</span>
              <StatusPill session={activeSession} />
              {activeSession.status === 'failed' && activeSession.error && (
                <span className="truncate text-destructive" title={activeSession.error}>
                  — {activeSession.error}
                </span>
              )}
            </>
          ) : (
            <span className="text-foreground-subtlest">No active session</span>
          )}
        </div>

        <div className="flex shrink-0 items-center gap-1.5">
          {activeSession && (activeSession.status === 'running' || activeSession.status === 'starting') ? (
            <button
              type="button"
              onClick={() => void stopSession(activeSession.id)}
              className="flex h-5 items-center gap-1 rounded border border-border px-1.5 text-ui-xs text-foreground-subtle transition-colors hover:bg-surface-hover hover:text-foreground cursor-pointer"
              title="Stop the agent CLI"
            >
              <Square className="size-2.5" />
              Stop
            </button>
          ) : activeSession ? (
            <button
              type="button"
              onClick={() => void restartSession(activeSession.id)}
              className="flex h-5 items-center gap-1 rounded border border-border px-1.5 text-ui-xs text-foreground-subtle transition-colors hover:bg-surface-hover hover:text-foreground cursor-pointer"
              title="Restart the agent CLI"
            >
              <RotateCcw className="size-2.5" />
              Restart
            </button>
          ) : null}
        </div>
      </footer>

    </div>
  );
};

const StatusPill: React.FC<{ session: TerminalSessionRecord }> = ({ session }) => {
  const running = session.status === 'running' || session.status === 'starting';
  return (
    <span className="flex items-center gap-1.5 text-ui-xs text-foreground-subtle">
      <span
        className={cn(
          'size-1.5 rounded-full',
          running ? 'animate-pulse bg-primary' : session.status === 'failed' ? 'bg-destructive' : 'bg-foreground-subtlest/70'
        )}
      />
      {sessionStatusLabel(session)}
      {session.status === 'exited' && typeof session.exitCode === 'number' && session.exitCode !== 0 && (
        <span className="text-foreground-subtlest">· code {session.exitCode}</span>
      )}
    </span>
  );
};

/** Overlay when the active session's process has ended (restart affordance). */
const SessionEndedOverlay: React.FC<{ session: TerminalSessionRecord; onRestart: () => void }> = ({
  session,
  onRestart
}) => {
  const [dismissed, setDismissed] = useState(false);
  useEffect(() => setDismissed(false), [session.id, session.status]);
  if (dismissed) return null;
  const label =
    session.status === 'failed'
      ? 'Failed to start'
      : session.status === 'terminated'
        ? 'Session terminated'
        : 'Session ended';
  return (
    <div className="absolute bottom-3 left-1/2 z-10 flex -translate-x-1/2 items-center gap-2 rounded-full border border-border bg-sidebar/95 px-3 py-1.5 shadow-xl backdrop-blur">
      <span className="text-ui-xs text-foreground-subtle">{label}</span>
      {typeof session.exitCode === 'number' && session.exitCode !== 0 && (
        <span className="text-ui-xs text-foreground-subtlest">· exit {session.exitCode}</span>
      )}
      <button
        type="button"
        onClick={onRestart}
        className="flex items-center gap-1 rounded-full bg-brand px-2.5 py-0.5 text-ui-xs font-medium text-foreground-inverse transition-colors hover:bg-brand/80 cursor-pointer"
      >
        <RotateCcw className="size-3" />
        Restart
      </button>
      <button
        type="button"
        aria-label="Dismiss"
        onClick={() => setDismissed(true)}
        className="text-ui-xs text-foreground-subtlest transition-colors hover:text-foreground cursor-pointer"
      >
        ✕
      </button>
    </div>
  );
};

/** Empty state: the unified New Task entry point. */
const SessionEmptyState: React.FC = () => {
  return (
    <div className="absolute inset-0 z-10 flex flex-col items-center justify-center gap-6 bg-background/97 px-6">
      <div className="flex flex-col items-center gap-2 text-center">
        <div className="flex size-12 items-center justify-center rounded-2xl border border-border bg-sidebar">
          <TerminalIcon className="size-6 text-foreground-subtle" />
        </div>
        <h2 className="text-ui-lg font-semibold text-foreground">No active CLI task</h2>
        <p className="max-w-sm text-ui-sm leading-5 text-foreground-subtle">
          Start a task to continue — ForgeADE for the internal agent chat, or an agent CLI in its own terminal.
        </p>
      </div>

      <button
        type="button"
        onClick={() => useSessionStore.getState().openNewTask()}
        className="flex h-9 items-center gap-1.5 rounded-lg bg-brand px-4 text-ui-sm font-medium text-foreground-inverse transition-colors hover:bg-brand/80 cursor-pointer"
        data-testid="empty-state-new-task"
      >
        <TerminalIcon className="size-4" />
        New Task
      </button>
    </div>
  );
};

export default TerminalSessionSurface;
