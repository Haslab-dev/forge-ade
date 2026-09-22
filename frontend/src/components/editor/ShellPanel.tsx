import React, { useState, useEffect, useCallback, useRef } from 'react';
import { Plus, X, Terminal as TerminalIcon } from 'lucide-react';
import { TerminalView } from '../terminal-view';
import { CreateShell, StopSession, ListShells, EventsOn } from '../../lib/wails';
import { useWorkspace } from '../../stores/workspaceStore';

interface ShellSession {
  id: string;
  name: string;
}

// Shell sidebar panel — hosts multiple live PTY shell sessions in the sidebar.
// Sessions run in the Go backend (shared with the agent tooling) and every
// open session stays mounted here so switching never restarts a shell.
export const ShellPanel: React.FC = () => {
  const { activeWorkspacePath, activeActivity } = useWorkspace();
  const [shells, setShells] = useState<ShellSession[]>([]);
  const [activeId, setActiveId] = useState<string | null>(null);
  const [error, setError] = useState<string | null>(null);
  const [booting, setBooting] = useState(true);
  // Monotonic counter so names never collide after closes.
  const nameCounterRef = useRef(0);
  // The panel stays mounted (hidden) inside the sidebar; boot lazily on the
  // first activation so no PTY is spawned until the user opens the tab.
  const bootedRef = useRef(false);

  const addShell = useCallback(async () => {
    setError(null);
    nameCounterRef.current += 1;
    const name = `zsh (${nameCounterRef.current})`;
    try {
      const sess = await CreateShell(name, activeWorkspacePath || '');
      if (!sess?.id) throw new Error('backend returned no session');
      setShells(prev => [...prev, { id: sess.id, name: sess.name || name }]);
      setActiveId(sess.id);
      return sess.id;
    } catch (err: any) {
      nameCounterRef.current -= 1;
      setError(`Failed to start shell: ${err?.message || err}`);
      return null;
    }
  }, [activeWorkspacePath]);

  // Initial load (first activation only): adopt running shell sessions for
  // this workspace; start one when none exist (VS Code behaviour of an
  // always-ready first terminal).
  useEffect(() => {
    if (activeActivity !== 'shell' || bootedRef.current) return;
    bootedRef.current = true;
    let cancelled = false;
    (async () => {
      try {
        const all = await ListShells();
        if (cancelled) return;
        const mine = (Array.isArray(all) ? all : [])
          .filter((s: any) => s?.type === 'shell' || s?.Type === 'shell')
          .filter((s: any) => {
            const folder = s.folder || s.Folder || '';
            return !folder || !activeWorkspacePath || folder.startsWith(activeWorkspacePath) || activeWorkspacePath.startsWith(folder);
          })
          .map((s: any) => ({ id: s.id, name: s.name || 'zsh' }));
        if (cancelled) return;
        nameCounterRef.current = mine.length;
        if (mine.length > 0) {
          setShells(mine);
          setActiveId(mine[mine.length - 1].id);
        } else {
          await addShell();
        }
      } catch (err: any) {
        if (!cancelled) setError(`Failed to load shells: ${err?.message || err}`);
      } finally {
        if (!cancelled) setBooting(false);
      }
    })();
    return () => {
      cancelled = true;
    };
    // Shell sessions intentionally survive workspace switches.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeActivity]);

  // Follow sessions closed on the backend (e.g. the user typed `exit`).
  useEffect(() => {
    const unsub = EventsOn('session:closed', (payload: any) => {
      const id = payload?.id;
      if (!id) return;
      setShells(prev => {
        if (!prev.some(s => s.id === id)) return prev;
        const next = prev.filter(s => s.id !== id);
        setActiveId(cur => {
          if (cur !== id) return cur;
          if (next.length === 0) return null;
          const idx = prev.findIndex(s => s.id === id);
          return next[Math.min(idx, next.length - 1)].id;
        });
        return next;
      });
    });
    return () => {
      unsub?.();
    };
  }, []);

  const closeShell = useCallback(async (id: string) => {
    try {
      await StopSession(id);
    } catch { /* already gone */ }
    setShells(prev => {
      const next = prev.filter(s => s.id !== id);
      setActiveId(cur => {
        if (cur !== id) return cur;
        if (next.length === 0) return null;
        const idx = prev.findIndex(s => s.id === id);
        return next[Math.min(idx, next.length - 1)].id;
      });
      return next;
    });
  }, []);

  return (
    <div className="flex-1 flex flex-col min-h-0 overflow-hidden bg-white dark:bg-background">
      {/* Header */}
      <div className="h-[35px] min-h-[35px] px-3 flex items-center justify-between border-b border-border dark:border-border text-xs font-semibold text-foreground dark:text-white uppercase tracking-wider shrink-0">
        <span>Shell</span>
        <button
          type="button"
          onClick={() => addShell()}
          className="p-1 rounded hover:bg-surface-hover dark:hover:bg-surface-hover text-foreground-subtle hover:text-foreground dark:hover:text-white transition-colors cursor-pointer"
          title="New Shell Session"
        >
          <Plus className="w-3.5 h-3.5" />
        </button>
      </div>

      {/* Session tabs */}
      {shells.length > 0 && (
        <div className="flex items-stretch overflow-x-auto border-b border-border dark:border-border shrink-0 select-none">
          {shells.map(s => {
            const isActive = s.id === activeId;
            return (
              <div
                key={s.id}
                onClick={() => setActiveId(s.id)}
                title={s.name}
                className={`group flex items-center gap-1.5 pl-2.5 pr-1.5 py-1.5 text-[11px] font-mono cursor-pointer border-r border-border dark:border-border whitespace-nowrap transition-colors ${
                  isActive
                    ? 'text-foreground dark:text-white bg-background dark:bg-surface-hover border-b-2 border-b-[#2563eb]'
                    : 'text-foreground-subtle dark:text-foreground-subtle hover:text-foreground dark:hover:text-white hover:bg-background dark:hover:bg-card'
                }`}
              >
                <TerminalIcon className="w-3 h-3 shrink-0" />
                <span className="max-w-[90px] truncate">{s.name}</span>
                <button
                  type="button"
                  onClick={(e) => { e.stopPropagation(); closeShell(s.id); }}
                  className="p-0.5 rounded hover:bg-surface-hover dark:hover:bg-surface-hover text-foreground-subtle hover:text-destructive cursor-pointer opacity-0 group-hover:opacity-100 transition-opacity"
                  title="Kill Session"
                >
                  <X className="w-3 h-3" />
                </button>
              </div>
            );
          })}
        </div>
      )}

      {/* Error / empty states */}
      {error && (
        <div className="px-3 py-2 text-[11px] text-destructive border-b border-border dark:border-border flex items-center justify-between gap-2">
          <span className="min-w-0">{error}</span>
          <button
            type="button"
            onClick={() => addShell()}
            className="px-1.5 py-0.5 rounded bg-primary text-white text-[10px] cursor-pointer shrink-0 hover:bg-[#1d4ed8]"
          >
            Retry
          </button>
        </div>
      )}
      {!booting && shells.length === 0 && !error && (
        <div className="flex-1 flex flex-col items-center justify-center gap-2 text-foreground-subtle text-xs">
          <TerminalIcon className="w-6 h-6" />
          <span>No shell sessions</span>
          <button
            type="button"
            onClick={() => addShell()}
            className="px-2 py-1 rounded bg-primary hover:bg-[#1d4ed8] text-white text-[11px] cursor-pointer"
          >
            New Shell
          </button>
        </div>
      )}

      {/* Terminals — every session stays mounted; only the active one is visible */}
      <div className="flex-1 min-h-0 relative bg-[#0c0c0c]">
        {shells.map(s => (
          <div
            key={s.id}
            className="absolute inset-0 flex"
            style={{ display: s.id === activeId ? 'flex' : 'none' }}
          >
            <TerminalView sessionId={s.id} isActive={s.id === activeId} />
          </div>
        ))}
      </div>
    </div>
  );
};
