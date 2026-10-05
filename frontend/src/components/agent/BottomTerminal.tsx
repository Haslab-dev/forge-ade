import React, { useEffect, useRef, useState } from 'react';
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

export const BottomTerminal: React.FC<{
  open: boolean;
  onToggle: () => void;
  workspacePath: string;
}> = ({ open, onToggle, workspacePath }) => {
  const [sessions, setSessions] = useState<DockSession[]>([]);
  const [activeId, setActiveId] = useState<string>('');
  const [initializing, setInitializing] = useState(false);

  const sessionsRef = useRef(sessions);
  useEffect(() => { sessionsRef.current = sessions; }, [sessions]);

  const createSession = async (title: string, cwd = workspacePath): Promise<DockSession | null> => {
    try {
      const created = await CreateShell(title, cwd || '');
      if (created?.id) {
        const sess = { id: created.id, title, cwd: cwd || '' };
        setSessions(prev => [...prev, sess]);
        setActiveId(created.id);
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
  useEffect(() => {
    if (!workspacePath) return;
    const existing = sessionsRef.current.find(s => s.cwd === workspacePath);
    if (existing) {
      setActiveId(existing.id);
      return;
    }
    const projectName = workspacePath.split('/').filter(Boolean).pop() || 'terminal';
    void createSession(projectName, workspacePath);
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [workspacePath]);

  const handleNewTerminal = async () => {
    setInitializing(true);
    try {
      await createSession(`zsh ${sessions.length + 1}`, workspacePath);
    } finally {
      setInitializing(false);
    }
  };

  const handleCloseSession = async (id: string) => {
    const idx = sessions.findIndex(s => s.id === id);
    if (idx === -1) return;
    try { await StopSession(id); } catch {}
    const next = sessions.filter(s => s.id !== id);
    setSessions(next);
    if (activeId === id) {
      const neighbor = next[Math.min(idx, next.length - 1)];
      if (neighbor) {
        setActiveId(neighbor.id);
      } else if (open) {
        onToggle(); // last tab closed — collapse the dock
      }
    }
  };

  const handleRestart = async () => {
    const current = sessions.find(s => s.id === activeId);
    if (!current) return;
    setInitializing(true);
    try {
      try { await StopSession(current.id); } catch {}
      const created = await CreateShell(current.title, workspacePath || '');
      if (created?.id) {
        setSessions(prev => prev.map(s => (s.id === current.id ? { ...s, id: created.id, cwd: workspacePath || '' } : s)));
        setActiveId(created.id);
      }
    } finally {
      setInitializing(false);
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
                  onClick={() => setActiveId(s.id)}
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
              {initializing ? <Loader2 className="size-3.5 animate-spin" /> : <Plus className="size-4" />}
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
            {initializing ? <Loader2 className="size-3 animate-spin" /> : null}
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
            {initializing ? 'Starting shell…' : ''}
          </div>
        )}
      </div>
    </section>
  );
};

export default BottomTerminal;
