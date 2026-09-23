import React, { useEffect, useState } from 'react';
import { SquareTerminal, Loader2, X } from 'lucide-react';
import { CreateShell, StopSession, WriteSession } from '../../lib/wails';
import { TerminalView } from '../terminal-view';

/**
 * Bottom dock terminal (ZCode AnimatedTerminalPanel): a full-width terminal
 * panel docked under the conversation column, toggled from the header
 * terminal button. The PTY session stays alive while the panel is collapsed.
 */
export const BottomTerminal: React.FC<{
  open: boolean;
  onToggle: () => void;
  workspacePath: string;
}> = ({ open, onToggle, workspacePath }) => {
  const [sessionId, setSessionId] = useState<string>('');
  const [initializing, setInitializing] = useState(false);

  const ensureSession = async () => {
    if (sessionId || initializing) return sessionId;
    setInitializing(true);
    try {
      const created = await CreateShell('Bottom Shell', workspacePath || '');
      if (created?.id) {
        setSessionId(created.id);
        return created.id;
      }
    } catch (e) {
      console.warn('bottom terminal: failed to create shell', e);
    } finally {
      setInitializing(false);
    }
    return undefined;
  };

  // Lazily create the PTY the first time the panel opens.
  useEffect(() => {
    if (open) void ensureSession();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [open]);

  const handleRestart = async () => {
    setInitializing(true);
    try {
      if (sessionId) {
        try { await StopSession(sessionId); } catch {}
      }
      const created = await CreateShell('Bottom Shell', workspacePath || '');
      if (created?.id) setSessionId(created.id);
    } finally {
      setInitializing(false);
    }
  };

  const handleClear = async () => {
    if (sessionId) {
      try { await WriteSession(sessionId, '\x0c'); } catch {}
    }
  };

  return (
    <section
      data-testid="bottom-terminal"
      data-open={open}
      className="relative shrink-0 overflow-hidden border-t border-border bg-background transition-[height] duration-200"
      style={{ height: open ? '30%' : '0px' }}
    >
      {/* Panel header — mirrors the side-pane terminal header */}
      <div className="flex h-9 items-center justify-between border-b border-border bg-background px-3">
        <div className="flex min-w-0 items-center gap-2">
          <SquareTerminal className="size-3.5 shrink-0 text-foreground-subtle" />
          <span className="text-ui-sm font-medium text-foreground">Terminal</span>
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
      {/* Keep the terminal mounted (height 0 when closed) so the PTY survives. */}
      <div className={open ? 'h-[calc(100%-36px)]' : 'h-0 overflow-hidden'}>
        {sessionId ? (
          <TerminalView sessionId={sessionId} isActive={open} />
        ) : (
          <div className="flex h-full items-center justify-center text-ui-sm text-foreground-subtlest">
            {initializing ? 'Starting shell…' : ''}
          </div>
        )}
      </div>
    </section>
  );
};

export default BottomTerminal;
