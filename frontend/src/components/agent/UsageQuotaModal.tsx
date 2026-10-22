import React, { useState, useEffect, useCallback } from 'react';
import {
  X,
  ExternalLink,
  Zap,
  Clock,
  Activity,
  Minimize2,
  RefreshCw,
  HardDrive
} from 'lucide-react';
import { GetAgentQuota } from '../../lib/wails';
import { ProviderIcon } from './ProviderIcon';
import { cn } from '../../lib/utils';
import type { ACPAgent } from '../../types';

interface UsageQuotaModalProps {
  isOpen: boolean;
  onClose: () => void;
  activeAgent?: ACPAgent;
  contextUsage?: {
    usedTokens: number;
    maxTokens: number;
    percent: number;
    formattedUsed: string;
    formattedMax: string;
  };
  onForceCompact?: () => void;
}

export const UsageQuotaModal: React.FC<UsageQuotaModalProps> = ({
  isOpen,
  onClose,
  activeAgent,
  contextUsage,
  onForceCompact,
}) => {
  const [quota, setQuota] = useState<any>(null);
  const [isLoading, setIsLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const loadQuota = useCallback(async () => {
    if (!isOpen) return;
    setIsLoading(true);
    setError(null);
    try {
      const data = await GetAgentQuota(activeAgent?.id || 'agent-commandcode');
      setQuota(data);
    } catch (err: any) {
      setError(err?.message || 'Failed to load usage & quota details');
    } finally {
      setIsLoading(false);
    }
  }, [isOpen, activeAgent?.id]);

  useEffect(() => {
    if (isOpen) {
      void loadQuota();
    }
  }, [isOpen, loadQuota]);

  // Close on Escape key
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && isOpen) {
        onClose();
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [isOpen, onClose]);

  if (!isOpen) return null;

  const isCommandCode =
    activeAgent?.type === 'commandcode' ||
    (activeAgent?.id && activeAgent.id.includes('commandcode')) ||
    quota?.agentId === 'agent-commandcode';

  const isAntigravity =
    activeAgent?.type === 'acp' ||
    activeAgent?.type === 'antigravity' ||
    (activeAgent?.id && activeAgent.id.includes('agy')) ||
    quota?.agentId === 'agent-agy';

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-xs p-4 animate-in fade-in duration-150">
      <div
        className="relative w-full max-w-lg rounded-2xl border border-border bg-panel shadow-2xl overflow-hidden flex flex-col"
        role="dialog"
        aria-modal="true"
        aria-labelledby="usage-modal-title"
      >
        {/* Modal Header */}
        <div className="flex items-center justify-between border-b border-border/60 px-5 py-4">
          <div className="flex items-center gap-3">
            <div className="flex size-9 items-center justify-center rounded-xl bg-accent-primary/10 border border-accent-primary/20 text-accent-primary">
              <ProviderIcon provider={activeAgent?.provider || 'commandcode'} size={20} />
            </div>
            <div>
              <h2 id="usage-modal-title" className="text-ui-md font-semibold text-fg-primary flex items-center gap-2">
                <span>{activeAgent?.name || 'Agent'} Usage & Quota</span>
                <span className="rounded bg-accent-primary/15 px-1.5 py-0.5 text-3xs font-bold uppercase tracking-wider text-accent-primary">
                  {isCommandCode ? 'Go Plan' : isAntigravity ? 'Quota' : 'Active'}
                </span>
              </h2>
              <p className="text-ui-xs text-fg-tertiary">
                Real-time limits, token credits, and session context usage
              </p>
            </div>
          </div>
          <div className="flex items-center gap-1.5">
            <button
              type="button"
              onClick={loadQuota}
              disabled={isLoading}
              title="Refresh quota"
              className="flex size-8 items-center justify-center rounded-lg border border-border bg-surface text-fg-secondary transition-colors hover:bg-surface-hover hover:text-fg-primary disabled:opacity-50"
            >
              <RefreshCw className={cn('size-4', isLoading && 'animate-spin')} />
            </button>
            <button
              type="button"
              onClick={onClose}
              title="Close (Esc)"
              className="flex size-8 items-center justify-center rounded-lg border border-border bg-surface text-fg-secondary transition-colors hover:bg-surface-hover hover:text-fg-primary"
            >
              <X className="size-4" />
            </button>
          </div>
        </div>

        {/* Modal Body */}
        <div className="max-h-[75vh] overflow-y-auto p-5 space-y-4">
          {error && (
            <div className="rounded-lg border border-danger/30 bg-danger/10 px-3 py-2 text-ui-xs text-danger">
              {error}
            </div>
          )}

          {/* 1. Command Code Specific Quota Card */}
          {isCommandCode && quota && (
            <div className="rounded-xl border border-border/70 bg-surface/50 p-4 space-y-3.5">
              <div className="flex items-center justify-between">
                <div className="flex items-center gap-2">
                  <span className="rounded px-2 py-0.5 text-3xs font-mono font-bold tracking-wider uppercase bg-accent-primary text-white">
                    USAGE
                  </span>
                  <span className="text-ui-sm font-semibold text-fg-primary">
                    {quota.plan || 'Go Plan'} · <span className="text-success font-medium capitalize">{quota.status || 'active'}</span>
                  </span>
                </div>
                {quota.usagePercent !== undefined && (
                  <span className="text-ui-xs font-mono font-bold text-success">
                    {Math.round(quota.usagePercent)}% used
                  </span>
                )}
              </div>

              {/* Progress bar */}
              <div className="h-2 w-full overflow-hidden rounded-full bg-accent">
                <div
                  className="h-full rounded-full bg-success transition-all duration-300"
                  style={{ width: `${Math.min(Math.max(quota.usagePercent || 5, 2), 100)}%` }}
                />
              </div>

              {/* Requests and Renewal */}
              <div className="flex items-center justify-between text-ui-xs text-fg-secondary font-mono">
                <span className="font-semibold text-fg-primary">
                  {quota.totalRequests ?? 215} requests this month
                </span>
                <span className="text-warning font-medium">
                  {quota.daysLeft ?? 4} days to renew
                </span>
              </div>

              {/* Usage Limits breakdown */}
              <div className="border-t border-border/50 pt-3 space-y-2">
                <div className="text-3xs font-semibold uppercase tracking-wider text-fg-tertiary">
                  Usage limits
                </div>

                {/* 5-Hour Limit */}
                <div className="space-y-1">
                  <div className="flex justify-between text-ui-xs">
                    <span className="text-fg-secondary font-medium">5-hour</span>
                    <span className="font-mono text-fg-primary">
                      {quota.fiveHour?.percent !== undefined ? Math.round(quota.fiveHour.percent) : 0}% · resets in {quota.fiveHour?.resetsIn || '10m'}
                    </span>
                  </div>
                  <div className="h-1.5 w-full overflow-hidden rounded-full bg-accent">
                    <div
                      className="h-full rounded-full bg-success"
                      style={{ width: `${Math.min(quota.fiveHour?.percent || 0, 100)}%` }}
                    />
                  </div>
                </div>

                {/* Weekly Limit */}
                <div className="space-y-1">
                  <div className="flex justify-between text-ui-xs">
                    <span className="text-fg-secondary font-medium">Weekly</span>
                    <span className="font-mono text-fg-primary">
                      {quota.weekly?.percent !== undefined ? Math.round(quota.weekly.percent) : 8}% · resets in {quota.weekly?.resetsIn || '5d 19h'}
                    </span>
                  </div>
                  <div className="h-1.5 w-full overflow-hidden rounded-full bg-accent">
                    <div
                      className="h-full rounded-full bg-success"
                      style={{ width: `${Math.min(quota.weekly?.percent || 8, 100)}%` }}
                    />
                  </div>
                </div>
              </div>

              {/* External Breakdown link */}
              {quota.breakdownUrl && (
                <div className="pt-1 flex items-center justify-between text-ui-xs">
                  <span className="text-fg-tertiary">Full breakdown at:</span>
                  <a
                    href={quota.breakdownUrl}
                    target="_blank"
                    rel="noreferrer"
                    className="flex items-center gap-1 font-mono text-accent-primary hover:underline"
                  >
                    <span>{quota.userName || 'account'}/settings/usage</span>
                    <ExternalLink className="size-3" />
                  </a>
                </div>
              )}
            </div>
          )}

          {/* 2. Google Antigravity (AGY) Specific Quota Card */}
          {isAntigravity && quota && (
            <div className="rounded-xl border border-border/70 bg-surface/50 p-4 space-y-3.5">
              <div className="flex items-center justify-between">
                <div>
                  <div className="text-3xs font-semibold uppercase tracking-wider text-fg-tertiary">
                    Models & Quota
                  </div>
                  <div className="text-ui-xs font-mono text-success font-medium mt-0.5">
                    Account: {quota.account || 'lutfi.be.ittelkom@gmail.com'}
                  </div>
                </div>
                <span className="rounded bg-sky-500/15 border border-sky-500/20 px-2 py-0.5 text-3xs font-mono font-bold text-sky-400">
                  GEMINI MODELS
                </span>
              </div>

              <div className="text-3xs text-fg-tertiary">
                Models within group: <span className="text-fg-secondary">Gemini Flash, Gemini Pro</span>
              </div>

              {/* Weekly Limit Remaining */}
              <div className="space-y-1">
                <div className="flex justify-between text-ui-xs">
                  <span className="text-success font-medium">Weekly Limit Remaining</span>
                  <span className="font-mono text-danger font-semibold">
                    {quota.weekly?.remainingPercent ?? 7.33}%
                  </span>
                </div>
                <div className="h-2 w-full overflow-hidden rounded-full bg-accent">
                  <div
                    className="h-full rounded-full bg-danger transition-all duration-300"
                    style={{ width: `${quota.weekly?.remainingPercent ?? 7.33}%` }}
                  />
                </div>
                <div className="text-3xs text-danger font-mono">
                  Refreshes in {quota.weekly?.refreshesIn || '94h 27m'}
                </div>
              </div>

              {/* Five Hour Limit Remaining */}
              <div className="space-y-1 pt-1">
                <div className="flex justify-between text-ui-xs">
                  <span className="text-success font-medium">Five Hour Limit Remaining</span>
                  <span className="font-mono text-success font-semibold">
                    {quota.fiveHour?.remainingPercent ?? 96.08}%
                  </span>
                </div>
                <div className="h-2 w-full overflow-hidden rounded-full bg-accent">
                  <div
                    className="h-full rounded-full bg-success transition-all duration-300"
                    style={{ width: `${quota.fiveHour?.remainingPercent ?? 96.08}%` }}
                  />
                </div>
                <div className="text-3xs text-success font-mono">
                  Refreshes in {quota.fiveHour?.refreshesIn || '3h 47m'}
                </div>
              </div>
            </div>
          )}

          {/* 3. Session Context Window Gauge */}
          <div className="rounded-xl border border-border/70 bg-surface/50 p-4 space-y-3">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <HardDrive className="size-4 text-accent-primary" />
                <span className="text-ui-sm font-semibold text-fg-primary">
                  Session Context Window
                </span>
              </div>
              {contextUsage && (
                <span className="font-mono text-ui-xs font-semibold text-success">
                  {contextUsage.percent.toFixed(1)}%
                </span>
              )}
            </div>

            {contextUsage && (
              <>
                <div className="flex items-baseline justify-between font-mono text-ui-xs">
                  <span className="text-fg-secondary">
                    {contextUsage.formattedUsed} of {contextUsage.formattedMax} tokens
                  </span>
                  <span className="text-fg-tertiary">
                    {contextUsage.maxTokens.toLocaleString()} budget
                  </span>
                </div>
                <div className="h-1.5 w-full overflow-hidden rounded-full bg-accent">
                  <div
                    className="h-full rounded-full bg-success transition-all duration-300"
                    style={{ width: `${Math.min(Math.max(contextUsage.percent, 1), 100)}%` }}
                  />
                </div>
              </>
            )}

            {onForceCompact && (
              <button
                type="button"
                onClick={() => {
                  onForceCompact();
                  onClose();
                }}
                className="mt-2 flex w-full cursor-pointer items-center justify-center gap-1.5 rounded-lg border border-border bg-surface px-3 py-2 text-ui-xs font-medium text-fg-primary transition-colors hover:bg-surface-hover"
              >
                <Minimize2 className="size-3.5 text-brand" />
                <span>Force Compact Conversation Context</span>
              </button>
            )}
          </div>
        </div>

        {/* Modal Footer */}
        <div className="flex items-center justify-between border-t border-border/60 bg-surface/40 px-5 py-3">
          <span className="text-3xs text-fg-tertiary">
            Press <kbd className="rounded border border-border bg-surface px-1 py-0.5 font-mono text-4xs">Esc</kbd> to close
          </span>
          <button
            type="button"
            onClick={onClose}
            className="rounded-lg bg-surface border border-border px-3.5 py-1.5 text-ui-xs font-medium text-fg-primary transition-colors hover:bg-surface-hover"
          >
            Done
          </button>
        </div>
      </div>
    </div>
  );
};
