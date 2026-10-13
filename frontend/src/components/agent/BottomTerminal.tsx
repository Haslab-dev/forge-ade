import React, { useEffect, useReducer, useRef, useState } from 'react';
import { SquareTerminal, Loader2, X, Plus } from 'lucide-react';
import { CreateShell, StopSession, WriteSession } from '../../lib/wails';
import { TerminalView } from '../terminal-view';
import { cn } from '../../lib/utils';

/**
 * Bottom dock terminal (ZCode AnimatedTerminalPanel + Terminal tab bar):
 * a full-width terminal panel docked under the conversation column, toggled
 * from the header terminal button. Multiple shells run side by side — the
 * "+" button spawns another terminal, and every PTY stays mounted (hidden
 * when inactive) so scrollback and state survive tab switches and panel
 * collapse.
 */

interface DockSession {
  id: string;
  title: string;
  cwd: string;
}

// ---------------------------------------------------------------------------
// Dock state lives at module scope, NOT in component state. AgentContainer
// swaps its inner view (CLI task ↔ chat session ↔ home) whenever the active
// task/session changes, and each swap fully unmounts this component. With
// component state that lost every terminal tab and spawned duplicate shells —
// the user's open shell (e.g. an ssh remote session) "disappeared" after any
// such switch. A module store keeps the tab list and selection across
// remounts; the PTYs live in the backend either way.
// ---------------------------------------------------------------------------
interface DockState {
  sessions: DockSession[];
  activeId: string;
}
let dockState: DockState = { sessions: [], activeId: '' };
const dockListeners = new Set<() => void>();

function setDockState(patch: Partial<DockState>) {
  dockState = { ...dockState, ...patch };
  dockListeners.forEach(emit => emit());
}

function useDockState(): DockState {
  const [, force] = useReducer((c: number) => c + 1, 0);
  useEffect(() => {
    dockListeners.add(force);
    return () => {
      dockListeners.delete(force);
    };
  }, [force]);
  return dockState;
}

export const BottomTerminal: React.FC<{
  open: boolean;
  onToggle: () => void;
  workspacePath: string;
}> = ({ open, onToggle, workspacePath }) => {
  const { sessions, activeId } = useDockState();
  const initializingRef = useRef(false);
  const [busy, setBusy] = useState(false);

  const createSession = async (title: string, cwd = workspacePath): Promise<DockSession | null> => {
    try {
      const created = await CreateShell(title, cwd || '');
      if (created?.id) {
        const sess = { id: created.id, title, cwd: cwd || '' };
        setDockState({ sessions: [...dockState.sessions, sess], activeId: created.id });
        return sess;
      }
    } catch (e) {
      console.warn('bottom terminal: failed to create shell', e);
    }
    return null;
  };

  // ZCode model: one persistent terminal PER WORKSPACE. Every PTY stays alive
  // for its whole lifetime (scrollback, jobs, env). Switching project either
  // activates that project's existing terminal or spawns one named after the
  // project — no `cd` typing, no restarts, nothing spilled into scrollback.
  // After a remount the module store still holds previous tabs, so the live
  // shells are re-attached instead of duplicated.
  useEffect(() => {
    if (!workspacePath) return;
    const existing = dockState.sessions.find(s => s.cwd === workspacePath);
    if (existing) {
      if (dockState.activeId !== existing.id) setDockState({ activeId: existing.id });
      return;
    }
    const projectName = workspacePath.split('/').filter(Boolean).pop() || 'terminal';
    if (!initializingRef.current) {
      initializingRef.current = true;
      void createSession(projectName, workspacePath).finally(() => {
        initializingRef.current = false;
      });
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [workspacePath]);

  const handleNewTerminal = async () => {
    setBusy(true);
    try {
      await createSession(`zsh ${dockState.sessions.length + 1}`, workspacePath);
    } finally {
      setBusy(false);
    }
  };

  const handleCloseSession = async (id: string) => {
    const sessions = dockState.sessions;
    const idx = sessions.findIndex(s => s.id === id);
    if (idx === -1) return;
    try { await StopSession(id); } catch {}
    const next = sessions.filter(s => s.id !== id);
    const patch: Partial<DockState> = { sessions: next };
    if (dockState.activeId === id) {
      const neighbor = next[Math.min(idx, next.length - 1)];
      if (neighbor) {
        patch.activeId = neighbor.id;
      } else if (open) {
        onToggle(); // last tab closed — collapse the dock
      }
    }
    setDockState(patch);
  };

  const handleRestart = async () => {
    const current = dockState.sessions.find(s => s.id === activeId);
    if (!current) return;
    setBusy(true);
    try {
      try { await StopSession(current.id); } catch {}
      const created = await CreateShell(current.title, current.cwd || workspacePath || '');
      if (created?.id) {
        setDockState({
          sessions: dockState.sessions.map(s => (s.id === current.id ? { ...s, id: created.id, cwd: current.cwd } : s)),
          activeId: created.id
        });
      }
    } catch (e) {
      console.warn('bottom terminal: failed to restart shell', e);
    } finally {
      setBusy(false);
    }
  };

  const handleClear = async () => {
    if (activeId) {
      try { await WriteSession(activeId, '\x0c'); } catch {}
    }
  };

  return (
    <section
      data-testid="bottom-terminal"
      data-open={open}
      className="relative shrink-0 overflow-hidden border-t border-border bg-background transition-[height] duration-200"
      style={{ height: open ? '30%' : '0px' }}
    >
      {/* Panel header — "Terminal" label, session tabs, then + and panel close */}
      <div className="flex h-9 shrink-0 items-center justify-between border-b border-border bg-background pl-3 pr-2">
        <div className="flex min-w-0 flex-1 items-center gap-2">
          <div className="flex shrink-0 items-center gap-2">
            <SquareTerminal className="size-3.5 shrink-0 text-foreground-subtle" />
            <span className="text-ui-sm font-medium text-foreground">Terminal</span>
          </div>

          {/* Session tabs */}
          <div className="flex min-w-0 items-center gap-1 overflow-x-auto [scrollbar-width:none] [&::-webkit-scrollbar]:hidden">
            {sessions.map(s => (
              <div
                key={s.id}
                className={cn(
                  'group/tab flex h-6 shrink-0 items-center gap-1.5 rounded-md px-2 text-ui-xs transition-colors',
                  s.id === activeId
                    ? 'bg-surface-hover font-medium text-foreground'
                    : 'text-foreground-subtle hover:bg-surface-hover/60 hover:text-foreground'
                )}
              >
                <button
                  type="button"
                  role="tab"
                  aria-selected={s.id === activeId}
                  onClick={() => setDockState({ activeId: s.id })}
                  className="max-w-32 cursor-pointer truncate font-mono"
                  title={s.title}
                >
                  {s.title}
                </button>
                <button
                  type="button"
                  aria-label={`Close ${s.title}`}
                  title="Close terminal"
                  onClick={() => void handleCloseSession(s.id)}
                  className="flex size-4 items-center justify-center rounded text-foreground-subtlest opacity-0 transition-opacity hover:text-foreground group-hover/tab:opacity-100 focus-visible:opacity-100 cursor-pointer"
                >
                  <X className="size-3" />
                </button>
              </div>
            ))}

            {/* New terminal (ZCode bottom-dock +) */}
            <button
              type="button"
              aria-label="New terminal"
              title="New terminal"
              onClick={() => void handleNewTerminal()}
              className="flex size-6 shrink-0 items-center justify-center rounded-md text-foreground-subtle transition-colors hover:bg-surface-hover hover:text-foreground cursor-pointer"
            >
              {busy ? <Loader2 className="size-3.5 animate-spin" /> : <Plus className="size-4" />}
            </button>
          </div>
        </div>

        <div className="flex shrink-0 items-center gap-1">
          <button
            type="button"
            className="rounded-md border border-border px-2 py-0.5 text-ui-xs text-foreground-subtle transition-colors hover:bg-surface-hover hover:text-foreground"
            onClick={() => void handleClear()}
          >
            Clear
          </button>
          <button
            type="button"
            className="flex items-center gap-1 rounded-md border border-border px-2 py-0.5 text-ui-xs text-foreground-subtle transition-colors hover:bg-surface-hover hover:text-foreground"
            onClick={() => void handleRestart()}
          >
            {busy ? <Loader2 className="size-3 animate-spin" /> : null}
            Restart
          </button>
          <button
            type="button"
            aria-label="Close terminal"
            title="Close terminal"
            className="flex size-6 items-center justify-center rounded-md text-foreground-subtle transition-colors hover:bg-surface-hover hover:text-foreground"
            onClick={onToggle}
          >
            <X className="size-3.5" />
          </button>
        </div>
      </div>

      {/* Every PTY stays mounted; inactive ones are hidden so scrollback and
          shell state survive switching tabs and collapsing the panel. */}
      <div className={open ? 'relative h-[calc(100%-36px)]' : 'h-0 overflow-hidden'}>
        {sessions.map(s => (
          <div
            key={s.id}
            className={cn('h-full w-full', s.id === activeId && open ? 'block' : 'hidden')}
          >
            <TerminalView sessionId={s.id} isActive={open && s.id === activeId} />
          </div>
        ))}
        {sessions.length === 0 && (
          <div className="flex h-full items-center justify-center text-ui-sm text-foreground-subtlest">
            {busy ? 'Starting shell…' : ''}
          </div>
        )}
      </div>
    </section>
  );
};

export default BottomTerminal;
