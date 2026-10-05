import React, { useEffect, useRef } from 'react';
import { X } from 'lucide-react';
import { GitGraphPane } from '../editor/GitGraphPane';

interface GitGraphModalProps {
  open: boolean;
  onClose: () => void;
}

/**
 * Git Graph modal view (ZCode branch-menu "Git Graph" entry): hosts the
 * existing self-contained GitGraphPane in a centered dialog so the graph is
 * reachable from agent mode without switching to the editor.
 */
export const GitGraphModal: React.FC<GitGraphModalProps> = ({ open, onClose }) => {
  const closeBtnRef = useRef<HTMLButtonElement>(null);

  useEffect(() => {
    if (!open) return;
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') onClose();
    };
    document.addEventListener('keydown', onKey);
    const t = window.setTimeout(() => closeBtnRef.current?.focus(), 30);
    return () => {
      document.removeEventListener('keydown', onKey);
      window.clearTimeout(t);
    };
  }, [open, onClose]);

  if (!open) return null;

  return (
    <div
      className="fixed inset-0 z-[60] bg-black/60 backdrop-blur-sm flex items-center justify-center p-4 md:p-8"
      onClick={onClose}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-label="Git Graph"
        className="w-full max-w-6xl h-[80vh] rounded-2xl border border-border bg-background shadow-2xl overflow-hidden flex flex-col animate-in fade-in zoom-in-95 duration-100"
        onClick={e => e.stopPropagation()}
      >
        <div className="h-12 min-h-12 px-4 flex items-center justify-between border-b border-border bg-surface dark:bg-card">
          <span className="text-sm font-semibold text-foreground">Git Graph</span>
          <button
            ref={closeBtnRef}
            type="button"
            aria-label="Close Git Graph"
            onClick={onClose}
            className="flex size-7 items-center justify-center rounded-md text-foreground-subtle transition-colors hover:bg-surface-hover hover:text-foreground"
            title="Close (Esc)"
          >
            <X className="size-4" aria-hidden="true" />
          </button>
        </div>
        {/* The pane's own close button targets its editor tab; the modal owns
            closing here, so hide it and keep the pane's graph controls. */}
        <div className="flex-1 min-h-0 [&_button[title='Close Git Graph']]:hidden">
          <GitGraphPane />
        </div>
      </div>
    </div>
  );
};
