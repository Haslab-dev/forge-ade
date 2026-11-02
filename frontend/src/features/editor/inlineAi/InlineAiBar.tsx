import React, { useState, useEffect, useRef } from 'react';
import {
  Sparkles,
  ArrowRight,
  Check,
  X,
  RotateCcw,
  Square,
  Loader2,
  Wand2,
  FileCode,
  ShieldAlert
} from 'lucide-react';
import { cn } from '../../../lib/utils';
import { InlineAiSessionState } from './inlineAiTypes';

interface InlineAiBarProps {
  state: InlineAiSessionState;
  activeModel: string;
  onPromptChange: (text: string) => void;
  onSubmit: (prompt: string) => void;
  onStop: () => void;
  onAccept: () => void;
  onReject: () => void;
  onClose: () => void;
}

export const InlineAiBar: React.FC<InlineAiBarProps> = ({
  state,
  activeModel,
  onPromptChange,
  onSubmit,
  onStop,
  onAccept,
  onReject,
  onClose
}) => {
  const inputRef = useRef<HTMLInputElement>(null);
  const [localPrompt, setLocalPrompt] = useState(state.prompt || '');

  useEffect(() => {
    setLocalPrompt(state.prompt || '');
  }, [state.prompt]);

  useEffect(() => {
    if (state.isOpen && state.status === 'idle') {
      setTimeout(() => inputRef.current?.focus(), 50);
    }
  }, [state.isOpen, state.status]);

  if (!state.isOpen) return null;

  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'Enter' && !e.shiftKey) {
      e.preventDefault();
      if (state.status === 'diff') {
        onAccept();
      } else if (localPrompt.trim() && state.status !== 'generating') {
        onSubmit(localPrompt);
      }
    } else if (e.key === 'Escape') {
      e.preventDefault();
      if (state.status === 'diff') {
        onReject();
      } else {
        onClose();
      }
    }
  };

  const handleQuickChip = (promptText: string) => {
    setLocalPrompt(promptText);
    onPromptChange(promptText);
    onSubmit(promptText);
  };

  const placeholderText = state.targetSymbol
    ? `Instruct AI on ${state.targetSymbol}... (e.g. 'Refactor to async', 'Add tests')`
    : state.originalText.trim()
      ? "Instruct AI on selected code (e.g. 'Add error handling', 'Explain', 'Optimize')..."
      : "Generate code at cursor (e.g. 'Create helper to parse JWT')...";

  return (
    <div
      role="dialog"
      aria-label="Inline AI Assistant"
      className="z-50 my-1 w-full max-w-xl rounded-xl border border-border/80 bg-surface/95 p-2 shadow-2xl backdrop-blur-md transition-all animate-in fade-in zoom-in-95"
    >
      {/* Top Header / Status Row */}
      <div className="flex items-center gap-2 pb-1.5">
        <div className="flex size-5 items-center justify-center rounded-md bg-primary/15 text-primary">
          <Sparkles className="size-3.5" />
        </div>
        <span className="text-ui-xs font-semibold text-foreground">
          {state.status === 'diff'
            ? 'Proposed Changes'
            : state.status === 'generating'
              ? 'Generating transformation...'
              : 'Inline AI Edit'}
        </span>
        {state.targetSymbol && (
          <span className="rounded bg-foreground/5 px-1.5 py-0.5 font-mono text-[11px] text-foreground-subtle">
            {state.targetSymbol}
          </span>
        )}
        <span className="ml-auto rounded-full border border-border/60 bg-foreground/5 px-2 py-0.5 text-[10px] font-medium text-foreground-subtle">
          {activeModel}
        </span>
        <button
          type="button"
          onClick={state.status === 'diff' ? onReject : onClose}
          className="flex size-5 items-center justify-center rounded text-foreground-subtle hover:bg-foreground/5 hover:text-foreground cursor-pointer"
          title="Close (Esc)"
        >
          <X className="size-3.5" />
        </button>
      </div>

      {/* Input or Diff Action Bar */}
      {state.status === 'diff' ? (
        <div className="flex items-center justify-between gap-2 pt-1">
          <span className="text-ui-xs text-foreground-subtle">
            Review changes above in editor
          </span>
          <div className="flex items-center gap-1.5">
            <button
              type="button"
              onClick={onReject}
              className="flex h-7 items-center gap-1.5 rounded-lg border border-border/80 bg-surface px-2.5 text-ui-xs font-medium text-foreground hover:bg-destructive/10 hover:text-destructive cursor-pointer transition-colors"
            >
              <X className="size-3" />
              Reject (Esc)
            </button>
            <button
              type="button"
              onClick={onAccept}
              className="flex h-7 items-center gap-1.5 rounded-lg bg-primary px-3 text-ui-xs font-medium text-primary-foreground hover:bg-primary/90 cursor-pointer shadow-xs transition-colors"
            >
              <Check className="size-3" />
              Accept (⌘Enter)
            </button>
          </div>
        </div>
      ) : (
        <>
          <div className="relative flex items-center gap-1.5">
            <input
              ref={inputRef}
              type="text"
              value={localPrompt}
              onChange={(e) => {
                setLocalPrompt(e.target.value);
                onPromptChange(e.target.value);
              }}
              onKeyDown={handleKeyDown}
              disabled={state.status === 'generating'}
              placeholder={placeholderText}
              className="h-8 flex-1 rounded-lg border border-border/70 bg-background px-3 text-ui-xs text-foreground placeholder:text-foreground-subtle/50 focus:border-primary/60 focus:outline-hidden disabled:opacity-60"
            />
            {state.status === 'generating' ? (
              <button
                type="button"
                onClick={onStop}
                className="flex h-8 items-center gap-1.5 rounded-lg bg-destructive/15 px-2.5 text-ui-xs font-medium text-destructive hover:bg-destructive/25 cursor-pointer"
                title="Stop generation"
              >
                <Square className="size-3 fill-destructive" />
                Stop
              </button>
            ) : (
              <button
                type="button"
                onClick={() => localPrompt.trim() && onSubmit(localPrompt)}
                disabled={!localPrompt.trim()}
                className="flex size-8 shrink-0 items-center justify-center rounded-lg bg-primary text-primary-foreground hover:bg-primary/90 disabled:opacity-40 cursor-pointer transition-colors"
                title="Generate (Enter)"
              >
                <ArrowRight className="size-3.5" />
              </button>
            )}
          </div>

          {/* Quick Action Chips when Idle */}
          {state.status === 'idle' && (
            <div className="flex flex-wrap items-center gap-1 pt-2">
              <button
                type="button"
                onClick={() => handleQuickChip('Refactor this to be more clean, idiomatic, and readable')}
                className="rounded-md border border-border/50 bg-foreground/5 px-2 py-0.5 text-[11px] text-foreground-subtle hover:bg-foreground/10 hover:text-foreground cursor-pointer transition-colors"
              >
                ✨ Refactor
              </button>
              <button
                type="button"
                onClick={() => handleQuickChip('Write comprehensive unit tests with test cases for edge cases')}
                className="rounded-md border border-border/50 bg-foreground/5 px-2 py-0.5 text-[11px] text-foreground-subtle hover:bg-foreground/10 hover:text-foreground cursor-pointer transition-colors"
              >
                🧪 Unit Tests
              </button>
              <button
                type="button"
                onClick={() => handleQuickChip('Add clear docstrings and comments explaining logic and parameters')}
                className="rounded-md border border-border/50 bg-foreground/5 px-2 py-0.5 text-[11px] text-foreground-subtle hover:bg-foreground/10 hover:text-foreground cursor-pointer transition-colors"
              >
                📝 Add Docs
              </button>
              <button
                type="button"
                onClick={() => handleQuickChip('Optimize performance and reduce memory allocations')}
                className="rounded-md border border-border/50 bg-foreground/5 px-2 py-0.5 text-[11px] text-foreground-subtle hover:bg-foreground/10 hover:text-foreground cursor-pointer transition-colors"
              >
                ⚡ Optimize
              </button>
            </div>
          )}

          {/* Error Message */}
          {state.status === 'error' && state.error && (
            <div className="mt-1.5 flex items-center gap-1.5 text-[11px] text-destructive">
              <ShieldAlert className="size-3.5 shrink-0" />
              <span>{state.error}</span>
            </div>
          )}
        </>
      )}
    </div>
  );
};
