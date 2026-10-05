import React, { useMemo, useState } from 'react';
import {
  CalendarClock,
  Clock,
  Pause,
  Check,
  ChevronLeft,
  ChevronRight,
  Copy,
  ExternalLink,
  Loader2,
  MoreHorizontal,
  Pencil,
  Play,
  Plus,
  RefreshCw,
  Trash2
} from 'lucide-react';
import { cn } from '../../lib/utils';
import { useWorkspace } from '../../stores/workspaceStore';
import { useToast } from '../../lib/toast';
import type { Automation, AutomationRun } from '../../lib/wails';

/**
 * Automations (ZCode clone): saved prompt workflows with run history.
 * One page, two views — the card grid list and the full-page editor with a
 * Settings | History segmented tab, matching ZCode's AutomationsSection and
 * AutomationEditView patterns (132px card grid, split create button,
 * run-now toast navigation, colored-dot run table).
 */

const AUTOMATION_CREATE_LIMIT = 20;

type View =
  | { mode: 'list' }
  | { mode: 'create' }
  | { mode: 'edit'; automation: Automation };

interface Draft {
  id: string;
  name: string;
  description: string;
  prompt: string;
  workspace: string;
  cronExpr: string;
  enabled: boolean;
}

function emptyDraft(workspace: string): Draft {
  return { id: '', name: '', description: '', prompt: '', workspace, cronExpr: '', enabled: true };
}

type ScheduleFreq = 'hourly' | 'daily' | 'weekdays' | 'weekly' | 'monthly' | 'custom';

interface ScheduleState {
  has: boolean;
  freq: ScheduleFreq;
  hour: number;
  minute: number;
  weekday: number;  // 0-6 (Sun=0)
  monthDay: number; // 1-28
  custom: string;
}

const DEFAULT_SCHEDULE: ScheduleState = { has: false, freq: 'daily', hour: 9, minute: 0, weekday: 1, monthDay: 1, custom: '' };

const WEEKDAY_LABELS = ['Sunday', 'Monday', 'Tuesday', 'Wednesday', 'Thursday', 'Friday', 'Saturday'];
const FREQ_LABELS: Array<{ id: ScheduleFreq; label: string }> = [
  { id: 'hourly', label: 'Hourly' },
  { id: 'daily', label: 'Daily' },
  { id: 'weekdays', label: 'Weekdays' },
  { id: 'weekly', label: 'Weekly' },
  { id: 'monthly', label: 'Monthly' },
  { id: 'custom', label: 'Custom' }
];

function buildCron(s: ScheduleState): string {
  const hhmm = `${s.minute} ${s.hour}`;
  switch (s.freq) {
    case 'hourly': return `${s.minute} * * * *`;
    case 'daily': return `${hhmm} * * *`;
    case 'weekdays': return `${hhmm} * * 1-5`;
    case 'weekly': return `${hhmm} * * ${s.weekday}`;
    case 'monthly': return `${hhmm} ${s.monthDay} * *`;
    case 'custom': return s.custom.trim();
  }
}

function parseHHMM(minute: number, hour: number): string {
  return `${String(hour).padStart(2, '0')}:${String(minute).padStart(2, '0')}`;
}

/** Human description of a cron expression (ZCode schedule-badge text). */
export function describeCron(cron: string): string {
  const parts = cron.trim().split(/\s+/);
  if (parts.length !== 5) return cron;
  const [m, h, dom, , dow] = parts;
  const at = (hh: string) => `${String(hh).padStart(2, '0')}:${String(m).padStart(2, '0')}`;
  if (/^\*\/\d+$/.test(m) && h === '*' && dom === '*' && dow === '*') {
    return `Every ${m.replace('*/', '')} minutes`;
  }
  if (m !== '*' && h !== '*' && dom === '*' && dow === '*') {
    return `Every day at ${at(h)}`;
  }
  if (m !== '*' && h !== '*' && dom === '*' && dow === '1-5') {
    return `Weekdays at ${at(h)}`;
  }
  if (m !== '*' && h !== '*' && dom === '*' && /^\d$/.test(dow)) {
    return `Weekly on ${WEEKDAY_LABELS[Number(dow)] || dow} at ${at(h)}`;
  }
  if (m !== '*' && h !== '*' && dom !== '*') {
    return `Monthly on day ${dom} at ${at(h)}`;
  }
  return cron;
}

function nextRunLabel(nextRunAt?: string): string {
  if (!nextRunAt) return '';
  const t = new Date(nextRunAt).getTime();
  if (isNaN(t)) return '';
  const mins = Math.round((t - Date.now()) / 60000);
  if (mins <= 0) return 'Next run imminent';
  if (mins < 60) return `Next run in ${mins} min`;
  const hours = Math.round(mins / 60);
  if (hours < 24) return `Next run in ${hours}h`;
  return `Next run in ${Math.round(hours / 24)}d`;
}

/** Best-effort reverse: cron to builder state (custom fallback keeps raw). */
function scheduleFromCron(cron: string): ScheduleState {
  if (!cron) return { ...DEFAULT_SCHEDULE };
  const parts = cron.trim().split(/\s+/);
  if (parts.length !== 5) return { ...DEFAULT_SCHEDULE, has: true, freq: 'custom', custom: cron };
  const [m, h, dom, , dow] = parts;
  const minute = /^\d+$/.test(m) ? Number(m) : NaN;
  const hour = /^\d+$/.test(h) ? Number(h) : NaN;
  if (!isNaN(minute) && !isNaN(hour)) {
    if (dom === '*' && dow === '*') return { has: true, freq: h === '*' ? 'hourly' : 'daily', hour: h === '*' ? 0 : hour, minute, weekday: 1, monthDay: 1, custom: cron };
    if (dom === '*' && dow === '1-5') return { has: true, freq: 'weekdays', hour, minute, weekday: 1, monthDay: 1, custom: cron };
    if (dom === '*' && /^\d$/.test(dow)) return { has: true, freq: 'weekly', hour, minute, weekday: Number(dow) % 7, monthDay: 1, custom: cron };
    if (/^\d+$/.test(dom) && Number(dom) <= 28 && h !== '*') return { has: true, freq: 'monthly', hour, minute, weekday: 1, monthDay: Number(dom), custom: cron };
  }
  return { ...DEFAULT_SCHEDULE, has: true, freq: 'custom', custom: cron };
}

function formatDateTime(ts: string): string {
  const d = new Date(ts);
  if (isNaN(d.getTime())) return '—';
  return d.toLocaleString(undefined, {
    month: 'short', day: 'numeric', hour: '2-digit', minute: '2-digit'
  });
}

const FIELD_LABEL = 'text-ui-base font-normal leading-5 text-foreground-subtle';

export const AutomationsView: React.FC = () => {
  const {
    automations,
    automationsLoading,
    fetchAutomations,
    saveAutomation,
    deleteAutomation,
    setAutomationEnabled,
    runAutomation,
    activeWorkspacePath,
    setActiveSessionId
  } = useWorkspace();
  const { toast } = useToast();

  const [view, setView] = useState<View>({ mode: 'list' });
  const [draft, setDraft] = useState<Draft>(emptyDraft(activeWorkspacePath || ''));
  const [editorTab, setEditorTab] = useState<'settings' | 'history'>('settings');
  const [menuFor, setMenuFor] = useState<string | null>(null);
  const [confirmDelete, setConfirmDelete] = useState<Automation | null>(null);
  const [busyRunId, setBusyRunId] = useState<string | null>(null);
  const [saving, setSaving] = useState(false);
  const [fieldErrors, setFieldErrors] = useState<Record<string, boolean>>({});
  const [schedule, setSchedule] = useState<ScheduleState>({ ...DEFAULT_SCHEDULE });
  const [statusFilter, setStatusFilter] = useState<'all' | 'active' | 'paused'>('all');

  const openEditor = (automation?: Automation) => {
    setFieldErrors({});
    setEditorTab('settings');
    if (automation) {
      setDraft({
        id: automation.id,
        name: automation.name,
        description: automation.description || '',
        prompt: automation.prompt,
        workspace: automation.workspace || activeWorkspacePath || '',
        cronExpr: automation.cronExpr || '',
        enabled: automation.enabled ?? true
      });
      setSchedule(scheduleFromCron(automation.cronExpr || ''));
      setView({ mode: 'edit', automation });
    } else {
      setDraft(emptyDraft(activeWorkspacePath || ''));
      setSchedule({ ...DEFAULT_SCHEDULE });
      setView({ mode: 'create' });
    }
  };

  const handleRun = (automation: Automation) => {
    if (busyRunId) {
      toast('A run is already in progress', 'warn');
      return;
    }
    setBusyRunId(automation.id);
    try {
      const sessionId = runAutomation(automation);
      if (sessionId) {
        setActiveSessionId(sessionId);
        toast(`Running “${automation.name}”…`, 'info');
      }
    } finally {
      window.setTimeout(() => setBusyRunId(null), 600);
    }
  };

  const handleSave = async () => {
    const errors: Record<string, boolean> = {};
    if (!draft.name.trim()) errors.name = true;
    if (!draft.prompt.trim()) errors.prompt = true;
    setFieldErrors(errors);
    if (Object.keys(errors).length > 0) return;

    setSaving(true);
    try {
      const cron = schedule.has ? buildCron(schedule) : '';
      if (schedule.has && !cron) {
        toast('Invalid cron expression', 'warn');
        return;
      }
      await saveAutomation({
        id: draft.id,
        name: draft.name.trim(),
        description: draft.description.trim(),
        prompt: draft.prompt,
        workspace: draft.workspace || activeWorkspacePath || '',
        cronExpr: cron,
        enabled: schedule.has ? true : (draft.enabled ?? true)
      });
      toast(draft.id ? 'Automation saved' : 'Automation created', 'success');
      setView({ mode: 'list' });
    } catch (e: any) {
      toast(e?.message || String(e), 'error');
    } finally {
      setSaving(false);
    }
  };

  const handleDelete = async (automation: Automation) => {
    setConfirmDelete(null);
    try {
      await deleteAutomation(automation.id);
      if (view.mode === 'edit' && view.automation.id === automation.id) {
        setView({ mode: 'list' });
      }
      toast('Automation deleted', 'default');
    } catch (e: any) {
      toast(e?.message || String(e), 'error');
    }
  };

  const sortedRuns = useMemo<AutomationRun[]>(
    () => (view.mode === 'edit' ? view.automation.runs || [] : []),
    [view]
  );

  // ────────────────────────────────────────────────────────────────────
  // Editor (create + edit share one full-page view, ZCode style)
  // ────────────────────────────────────────────────────────────────────
  if (view.mode === 'create' || view.mode === 'edit') {
    const isEdit = view.mode === 'edit';
    return (
      <div className="flex h-full min-h-0 flex-1 flex-col overflow-hidden bg-background">
        {/* Breadcrumb bar */}
        <div className="titlebar-drag flex h-12 shrink-0 items-center gap-2 border-b border-border px-4">
          <button
            type="button"
            onClick={() => setView({ mode: 'list' })}
            className="flex items-center gap-1 rounded-md px-1.5 py-1 text-ui-sm text-foreground-subtle transition-colors hover:bg-surface-hover hover:text-foreground"
          >
            <ChevronLeft className="size-3.5" />
            Automations
          </button>
          <ChevronRight className="size-3 text-foreground-subtlest" />
          <span className="truncate text-ui-sm font-medium text-foreground">
            {isEdit ? view.automation.name : 'New automation'}
          </span>
        </div>

        <div className="min-h-0 flex-1 overflow-y-auto [scrollbar-gutter:stable]">
          <div className="mx-auto flex w-full max-w-4xl flex-col px-4 py-6 md:px-6">
            {/* Page header + segmented tabs + actions */}
            <div className="flex items-center justify-between gap-3">
              <div className="flex items-center gap-4">
                <h1 className="text-ui-xl font-semibold text-foreground">
                  {isEdit ? 'Edit automation' : 'New automation'}
                </h1>
                {isEdit && (
                  <div className="flex items-center rounded-lg bg-surface p-0.5">
                    {(['settings', 'history'] as const).map(t => (
                      <button
                        key={t}
                        type="button"
                        onClick={() => setEditorTab(t)}
                        aria-pressed={editorTab === t}
                        className={cn(
                          'rounded-md px-3 py-1 text-ui-sm font-medium capitalize transition-colors',
                          editorTab === t ? 'bg-background text-foreground shadow-2xs' : 'text-foreground-subtle hover:text-foreground'
                        )}
                      >
                        {t}
                      </button>
                    ))}
                  </div>
                )}
              </div>
              <div className="flex shrink-0 items-center gap-2">
                {isEdit && (
                  <button
                    type="button"
                    onClick={() => handleRun(view.automation)}
                    className="flex h-8 items-center gap-1.5 rounded-lg bg-white/[0.06] px-3 text-ui-sm font-medium text-foreground transition-colors hover:bg-white/10"
                  >
                    {busyRunId === view.automation.id ? (
                      <Loader2 className="size-3.5 animate-spin" />
                    ) : (
                      <Play className="size-3.5" />
                    )}
                    Run now
                  </button>
                )}
                <button
                  type="button"
                  onClick={() => void handleSave()}
                  disabled={saving}
                  className="flex h-8 items-center gap-1.5 rounded-lg bg-white px-3 text-ui-sm font-medium text-black transition-opacity hover:opacity-90 disabled:opacity-50"
                >
                  {saving ? <Loader2 className="size-3.5 animate-spin" /> : <Check className="size-3.5" />}
                  {isEdit ? 'Save' : 'Create automation'}
                </button>
              </div>
            </div>

            {editorTab === 'settings' ? (
              <form
                className="mt-6 flex flex-col gap-4"
                onSubmit={e => {
                  e.preventDefault();
                  void handleSave();
                }}
              >
                <div className="flex flex-col gap-1.5">
                  <label className={FIELD_LABEL} htmlFor="automation-name">Name</label>
                  <input
                    id="automation-name"
                    type="text"
                    value={draft.name}
                    maxLength={200}
                    onChange={e => setDraft(d => ({ ...d, name: e.target.value }))}
                    placeholder="Untitled automation"
                    className={cn(
                      'h-9 rounded-xl bg-input px-3 text-ui-sm text-foreground placeholder-foreground-subtlest outline-none border border-transparent',
                      fieldErrors.name && 'border-destructive'
                    )}
                  />
                </div>

                <div className="flex flex-col gap-1.5">
                  <label className={FIELD_LABEL} htmlFor="automation-description">Description</label>
                  <input
                    id="automation-description"
                    type="text"
                    value={draft.description}
                    maxLength={300}
                    onChange={e => setDraft(d => ({ ...d, description: e.target.value }))}
                    placeholder="What does this automation do?"
                    className="h-9 rounded-xl bg-input px-3 text-ui-sm text-foreground placeholder-foreground-subtlest outline-none border border-transparent"
                  />
                </div>

                <div className="flex flex-col gap-1.5">
                  <label className={FIELD_LABEL} htmlFor="automation-workspace">Project</label>
                  <WorkspaceSelect
                    value={draft.workspace || activeWorkspacePath || ''}
                    onChange={v => setDraft(d => ({ ...d, workspace: v }))}
                  />
                </div>

                {/* Schedule (ZCode schedule builder) */}
                <div className="flex flex-col gap-1.5">
                  <label className={FIELD_LABEL}>Schedule</label>
                  {!schedule.has ? (
                    <button
                      type="button"
                      onClick={() => setSchedule(st => ({ ...st, has: true }))}
                      className="flex h-9 w-full items-center gap-2 rounded-xl border border-border bg-input px-3 text-ui-sm text-foreground-subtle transition-colors hover:bg-surface-hover hover:text-foreground"
                    >
                      <Clock className="size-3.5" />
                      Add schedule
                    </button>
                  ) : (
                    <div className="rounded-xl border border-border bg-input p-3">
                      <div className="flex flex-wrap items-center gap-2">
                        {FREQ_LABELS.map(f => (
                          <button
                            key={f.id}
                            type="button"
                            onClick={() => setSchedule(st => ({ ...st, freq: f.id }))}
                            className={cn(
                              'rounded-full px-3 py-1 text-ui-sm font-medium transition-colors',
                              schedule.freq === f.id
                                ? 'bg-[var(--color-background-button-secondary)] text-foreground'
                                : 'text-foreground-subtle hover:bg-surface-hover hover:text-foreground'
                            )}
                          >
                            {f.label}
                          </button>
                        ))}
                        <div className="ml-auto flex items-center gap-2">
                          {schedule.freq === 'daily' || schedule.freq === 'weekdays' || schedule.freq === 'weekly' || schedule.freq === 'monthly' ? (
                            <input
                              type="time"
                              value={parseHHMM(schedule.minute, schedule.hour)}
                              onChange={e => {
                                const [h, m] = (e.target.value || '09:00').split(':').map(Number);
                                setSchedule(st => ({ ...st, hour: isNaN(h) ? 9 : h, minute: isNaN(m) ? 0 : m }));
                              }}
                              className="h-8 rounded-lg border border-border bg-background px-2 text-ui-sm text-foreground outline-none"
                              aria-label="Run time"
                            />
                          ) : null}
                          {schedule.freq === 'hourly' ? (
                            <input
                              type="number"
                              min={0}
                              max={59}
                              value={schedule.minute}
                              onChange={e => setSchedule(st => ({ ...st, minute: Math.max(0, Math.min(59, Number(e.target.value) || 0)) }))}
                              className="h-8 w-20 rounded-lg border border-border bg-background px-2 text-ui-sm text-foreground outline-none"
                              aria-label="Minute of hour"
                            />
                          ) : null}
                          {schedule.freq === 'weekly' ? (
                            <select
                              value={schedule.weekday}
                              onChange={e => setSchedule(st => ({ ...st, weekday: Number(e.target.value) }))}
                              aria-label="Day of week"
                              className="h-8 rounded-lg border border-border bg-background px-2 text-ui-sm text-foreground outline-none"
                            >
                              {WEEKDAY_LABELS.map((label, idx) => (
                                <option key={idx} value={idx}>{label}</option>
                              ))}
                            </select>
                          ) : null}
                          {schedule.freq === 'monthly' ? (
                            <input
                              type="number"
                              min={1}
                              max={28}
                              value={schedule.monthDay}
                              onChange={e => setSchedule(st => ({ ...st, monthDay: Math.max(1, Math.min(28, Number(e.target.value) || 1)) }))}
                              className="h-8 w-20 rounded-lg border border-border bg-background px-2 text-ui-sm text-foreground outline-none"
                              aria-label="Day of month"
                            />
                          ) : null}
                          <button
                            type="button"
                            onClick={() => setSchedule(st => ({ ...st, has: false }))}
                            aria-label="Remove schedule"
                            title="Remove schedule"
                            className="flex size-8 items-center justify-center rounded-lg text-foreground-subtle transition-colors hover:bg-surface-hover hover:text-destructive cursor-pointer"
                          >
                            <Trash2 className="size-3.5" />
                          </button>
                        </div>
                      </div>

                      {schedule.freq === 'custom' ? (
                        <input
                          type="text"
                          value={schedule.custom}
                          onChange={e => setSchedule(st => ({ ...st, custom: e.target.value }))}
                          placeholder="*/15 * * * *"
                          className="mt-2 h-8 w-full rounded-lg border border-border bg-background px-2 font-mono text-ui-sm text-foreground placeholder-foreground-subtlest outline-none"
                          aria-label="Cron expression"
                        />
                      ) : null}

                      <div className="mt-2 flex items-center gap-2 text-ui-xs text-foreground-subtlest">
                        <Clock className="size-3" />
                        <span className="font-mono">{buildCron(schedule) || '—'}</span>
                        <span>·</span>
                        <span>{describeCron(buildCron(schedule))}</span>
                      </div>
                    </div>
                  )}
                </div>

                <div className="flex flex-col gap-1.5">
                  <label className={FIELD_LABEL} htmlFor="automation-prompt">Instructions</label>
                  <div className="overflow-hidden rounded-xl border border-border bg-input">
                    <textarea
                      id="automation-prompt"
                      value={draft.prompt}
                      rows={8}
                      onChange={e => setDraft(d => ({ ...d, prompt: e.target.value }))}
                      placeholder="e.g. Review commits from the last 24 hours and summarize likely bugs and fixes"
                      className={cn(
                        'w-full resize-y bg-transparent p-3 text-ui-sm leading-relaxed text-foreground placeholder-foreground-subtlest outline-none border-0',
                        fieldErrors.prompt && 'border-destructive'
                      )}
                    />
                    <div className="flex items-center justify-between border-t border-border px-3 py-1.5">
                      <span className="flex items-center gap-1.5 text-ui-xs text-foreground-subtlest">
                        <CalendarClock className="size-3.5" />
                        Runs in a new session with this prompt
                      </span>
                      <span className="font-mono text-ui-xs text-foreground-subtlest">{draft.prompt.length}</span>
                    </div>
                  </div>
                </div>
              </form>
            ) : (
              /* ── History tab (ZCode run table: colored dot + trigger + open session) ── */
              <div className="mt-6">
                {sortedRuns.length === 0 ? (
                  <div className="flex min-h-[226px] items-center justify-center rounded-xl border border-dashed border-border bg-background">
                    <p className="text-ui-base text-foreground-subtlest">No runs yet. Launch this automation to see its history.</p>
                  </div>
                ) : (
                  <table className="w-full text-left text-ui-sm">
                    <thead className="bg-surface text-foreground-subtle">
                      <tr className="h-[30px] border-b border-border">
                        <th className="px-4 font-normal">Triggered</th>
                        <th className="px-4 font-normal">Status</th>
                        <th className="px-4 font-normal" />
                      </tr>
                    </thead>
                    <tbody>
                      {sortedRuns.map(run => (
                        <tr key={run.id} className="h-[46px] border-b border-border hover:bg-surface-hover">
                          <td className="px-4 text-foreground">{formatDateTime(run.startedAt)}</td>
                          <td className="px-4">
                            <span className="inline-flex items-center gap-1.5">
                              <span className="size-1.5 rounded-full bg-primary" />
                              <span className="text-foreground">Running</span>
                            </span>
                          </td>
                          <td className="px-4 text-right">
                            {run.sessionId ? (
                              <button
                                type="button"
                                onClick={() => setActiveSessionId(run.sessionId!)}
                                className="inline-flex items-center gap-1 rounded-md px-2 py-1 text-ui-xs text-foreground-subtle transition-colors hover:bg-surface-hover hover:text-foreground"
                              >
                                <ExternalLink className="size-3" />
                                Open session
                              </button>
                            ) : null}
                          </td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                )}
              </div>
            )}
          </div>
        </div>
      </div>
    );
  }

  // ────────────────────────────────────────────────────────────────────
  // List
  // ────────────────────────────────────────────────────────────────────
  const atLimit = automations.length >= AUTOMATION_CREATE_LIMIT;

  return (
    <div className="flex h-full min-h-0 flex-1 flex-col overflow-hidden bg-background">
      <div className="titlebar-drag flex h-12 shrink-0 items-center border-b border-border px-4">
        <span className="text-ui-sm font-medium text-foreground">Automations</span>
      </div>

      <div className="min-h-0 flex-1 overflow-y-auto [scrollbar-gutter:stable]">
        <div className="mx-auto flex w-full max-w-5xl flex-col px-4 py-6 md:px-6">
          {/* Page title + subtitle (ZCode AutomationsPageTitle) */}
          <div className="flex flex-col gap-3">
            <h1 className="text-[30px] font-medium leading-[34px] tracking-[0.114px] text-foreground">Automations</h1>
            <p className="text-ui-base leading-5 text-foreground-subtlest">
              Save prompts you run often and launch them into a new session with one click.
            </p>
          </div>

          {automations.length === 0 ? (
            /* Empty state (ZCode: big bordered card, no tab row) */
            <div className="mt-8 flex h-[226px] w-full items-center justify-center rounded-2xl border border-border bg-background px-4">
              <div className="flex flex-col items-center gap-5">
                <p className="text-ui-base font-medium leading-5 text-foreground-subtlest">No automations yet.</p>
                <button
                  type="button"
                  onClick={() => openEditor()}
                  className="flex h-9 items-center gap-2 rounded-lg bg-primary px-4 text-ui-sm font-medium text-white transition-colors hover:bg-primary/90"
                >
                  <Plus className="size-4" />
                  Create automation
                </button>
              </div>
            </div>
          ) : (
            <>
              {/* Tab row: count + refresh + split create button */}
              <div className="mt-8 flex items-center justify-between">
                <div className="flex items-center gap-3">
                  <span className="text-ui-base font-medium leading-5 text-foreground-subtle">
                    {automationsLoading ? 'Loading…' : `Saved automations (${automations.length})`}
                  </span>
                  {(['all', 'active', 'paused'] as const).map(f => (
                    <button
                      key={f}
                      type="button"
                      onClick={() => setStatusFilter(f)}
                      className={cn(
                        'rounded-full px-2.5 py-0.5 text-ui-sm font-medium capitalize transition-colors',
                        statusFilter === f
                          ? 'bg-[var(--color-background-button-secondary)] text-foreground'
                          : 'text-foreground-subtle hover:bg-surface-hover hover:text-foreground'
                      )}
                    >
                      {f}
                    </button>
                  ))}
                </div>
                <div className="flex items-center gap-2">
                  <button
                    type="button"
                    aria-label="Refresh automations"
                    title="Refresh"
                    onClick={() => void fetchAutomations()}
                    className="flex size-8 items-center justify-center rounded-lg border border-border text-foreground-subtle transition-colors hover:bg-surface-hover hover:text-foreground"
                  >
                    <RefreshCw className={cn('size-3.5', automationsLoading && 'animate-spin')} />
                  </button>
                  {/* Split create button (ZCode AutomationCreateDropdown) */}
                  <div className="flex items-stretch overflow-hidden rounded-lg">
                    <button
                      type="button"
                      onClick={() => {
                        if (atLimit) {
                          toast(`You can keep up to ${AUTOMATION_CREATE_LIMIT} automations`, 'warn');
                          return;
                        }
                        openEditor();
                      }}
                      className="flex h-8 items-center gap-1.5 bg-primary px-3 text-ui-sm font-medium text-white transition-colors hover:bg-primary/90"
                    >
                      {atLimit ? <Copy className="size-3.5" /> : <Plus className="size-3.5" />}
                      Create automation
                    </button>
                    <button
                      type="button"
                      aria-label="Create in chat"
                      title="Create in chat"
                      onClick={() => {
                        if (atLimit) {
                          toast(`You can keep up to ${AUTOMATION_CREATE_LIMIT} automations`, 'warn');
                          return;
                        }
                        openEditor();
                      }}
                      className="flex w-6 items-center justify-center border-l border-white/20 bg-primary text-white transition-colors hover:bg-primary/90"
                    >
                      <ChevronRight className="size-3.5" />
                    </button>
                  </div>
                </div>
              </div>

              {/* Section header */}
              <h2 className="mb-3 mt-5 text-ui-base font-medium leading-5 text-foreground-subtle">Task created</h2>

              {/* Card grid (ZCode: 132px fixed rows, 2-col at lg) */}
              <div className="grid grid-cols-1 auto-rows-[132px] gap-x-4 gap-y-4 lg:grid-cols-2">
                {automations.filter(a => {
                  const scheduled = !!a.cronExpr;
                  if (statusFilter === 'active') return a.enabled !== false;
                  if (statusFilter === 'paused') return scheduled && a.enabled === false;
                  return true;
                }).map(a => {
                  const runCount = a.runs?.length || 0;
                  const scheduled = !!a.cronExpr;
                  const paused = scheduled && a.enabled === false;
                  return (
                    <div
                      key={a.id}
                      role="button"
                      tabIndex={0}
                      onClick={() => openEditor(a)}
                      onKeyDown={e => {
                        if (e.key === 'Enter' || e.key === ' ') {
                          e.preventDefault();
                          openEditor(a);
                        }
                      }}
                      className="group relative flex h-full min-h-0 cursor-pointer gap-3 overflow-hidden rounded-xl border border-border bg-background p-3 text-left transition-colors hover:bg-surface-hover"
                    >
                      <div className="flex h-full min-w-0 flex-1 flex-col gap-3">
                        <span className="block truncate pr-12 text-ui-base font-medium leading-5 text-foreground">{a.name}</span>
                        <p className="line-clamp-2 h-10 text-ui-base font-normal leading-5 text-foreground-subtle">
                          {a.description || a.prompt}
                        </p>
                        <div className="mt-auto flex h-6 min-w-0 items-center gap-2 text-ui-base leading-5">
                          {scheduled ? (
                            <span className={cn(
                              'inline-flex min-w-0 shrink-0 items-center gap-1 rounded-lg py-0.5 pl-1 pr-2 text-ui-xs font-medium',
                              paused ? 'bg-surface text-foreground-subtlest opacity-60' : 'bg-success/10 text-success'
                            )}>
                              <Clock className="size-3.5 shrink-0" />
                              <span className="truncate">
                                {paused ? 'Paused · ' : ''}
                                {describeCron(a.cronExpr!)}
                                {!paused && a.nextRunAt ? ` · ${nextRunLabel(a.nextRunAt)}` : ''}
                              </span>
                            </span>
                          ) : (
                            <span className="inline-flex shrink-0 items-center gap-1 rounded-lg bg-success/10 py-0.5 pl-1 pr-2 text-ui-xs font-medium text-success">
                              <CalendarClock className="size-3.5" />
                              <span className="max-w-28 truncate">{(a.workspace || activeWorkspacePath || '').split('/').filter(Boolean).pop() || 'Project'}</span>
                            </span>
                          )}
                          <span className="inline-flex shrink-0 items-center whitespace-nowrap rounded-md bg-surface px-1.5 py-0.5 text-ui-xs font-normal text-foreground-subtle">
                            {runCount === 0 ? 'Never ran' : `Ran ${runCount} time${runCount === 1 ? '' : 's'}`}
                          </span>
                        </div>
                      </div>

                      {/* Hover actions menu */}
                      <div className="absolute right-3 top-3 flex items-center gap-1">
                        {busyRunId === a.id && <Loader2 className="size-3.5 animate-spin text-foreground-subtle" />}
                        <div className="relative">
                          <button
                            type="button"
                            aria-label={`Actions for ${a.name}`}
                            onClick={e => {
                              e.stopPropagation();
                              setMenuFor(cur => (cur === a.id ? null : a.id));
                            }}
                            className="flex size-6 items-center justify-center rounded-md text-foreground-subtle opacity-0 transition-opacity hover:bg-surface-hover hover:text-foreground focus-visible:opacity-100 group-hover:opacity-100 md:opacity-0 md:group-hover:opacity-100 cursor-pointer"
                          >
                            <MoreHorizontal className="size-4" />
                          </button>
                          {menuFor === a.id && (
                            <>
                              <div className="fixed inset-0 z-40" onClick={e => { e.stopPropagation(); setMenuFor(null); }} />
                              <div role="menu" className="absolute right-0 top-full z-50 mt-1 w-44 rounded-xl border border-border bg-popover p-1 text-ui-sm shadow-lg">
                                <button
                                  type="button"
                                  role="menuitem"
                                  onClick={e => {
                                    e.stopPropagation();
                                    setMenuFor(null);
                                    handleRun(a);
                                  }}
                                  className="flex w-full items-center gap-2 rounded-lg px-2.5 py-1.5 text-left text-foreground transition-colors hover:bg-surface-hover"
                                >
                                  <Play className="size-4 text-foreground-subtle" />
                                  Run now
                                </button>
                                {a.cronExpr && (
                                  <button
                                    type="button"
                                    role="menuitem"
                                    onClick={e => {
                                      e.stopPropagation();
                                      setMenuFor(null);
                                      void setAutomationEnabled(a.id, a.enabled === false);
                                    }}
                                    className="flex w-full items-center gap-2 rounded-lg px-2.5 py-1.5 text-left text-foreground transition-colors hover:bg-surface-hover"
                                  >
                                    {a.enabled === false ? (
                                      <>
                                        <Play className="size-4 text-foreground-subtle" />
                                        Resume
                                      </>
                                    ) : (
                                      <>
                                        <Pause className="size-4 text-foreground-subtle" />
                                        Pause
                                      </>
                                    )}
                                  </button>
                                )}
                                <button
                                  type="button"
                                  role="menuitem"
                                  onClick={e => {
                                    e.stopPropagation();
                                    setMenuFor(null);
                                    openEditor(a);
                                  }}
                                  className="flex w-full items-center gap-2 rounded-lg px-2.5 py-1.5 text-left text-foreground transition-colors hover:bg-surface-hover"
                                >
                                  <Pencil className="size-4 text-foreground-subtle" />
                                  Edit
                                </button>
                                <div className="mx-1 my-1 h-px bg-border" />
                                <button
                                  type="button"
                                  role="menuitem"
                                  onClick={e => {
                                    e.stopPropagation();
                                    setMenuFor(null);
                                    setConfirmDelete(a);
                                  }}
                                  className="flex w-full items-center gap-2 rounded-lg px-2.5 py-1.5 text-left text-destructive transition-colors hover:bg-destructive/10"
                                >
                                  <Trash2 className="size-4" />
                                  Delete
                                </button>
                              </div>
                            </>
                          )}
                        </div>
                      </div>
                    </div>
                  );
                })}
              </div>
            </>
          )}
        </div>
      </div>

      {/* Delete confirm (ZCode: shared destructive confirm dialog) */}
      {confirmDelete && (
        <div className="fixed inset-0 z-[70] flex items-center justify-center bg-black/60 p-4 backdrop-blur-sm" onClick={() => setConfirmDelete(null)}>
          <div
            role="dialog"
            aria-modal="true"
            aria-label="Delete automation"
            className="w-full max-w-md rounded-xl border border-border bg-card p-4 shadow-2xl"
            onClick={e => e.stopPropagation()}
          >
            <h3 className="text-ui-base font-semibold text-foreground">Delete “{confirmDelete.name}”?</h3>
            <p className="mt-1.5 text-ui-sm leading-5 text-foreground-subtle">
              This will remove the automation and its run history. This cannot be undone.
            </p>
            <div className="mt-4 flex items-center justify-end gap-2">
              <button
                type="button"
                onClick={() => setConfirmDelete(null)}
                className="h-8 rounded-lg border border-border px-3 text-ui-sm text-foreground transition-colors hover:bg-surface-hover"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={() => void handleDelete(confirmDelete)}
                className="flex h-8 items-center gap-1.5 rounded-lg bg-destructive px-3 text-ui-sm font-medium text-white transition-opacity hover:opacity-90"
              >
                <Trash2 className="size-3.5" />
                Delete
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
};

/** Project picker: recent workspaces + current (ZCode locks project in edit;
 *  Forge-ade allows re-targeting). */
const WorkspaceSelect: React.FC<{ value: string; onChange: (v: string) => void }> = ({ value, onChange }) => {
  const { recentWorkspaces, activeWorkspacePath } = useWorkspace();
  const options = useMemo(() => {
    const all = [activeWorkspacePath, ...recentWorkspaces].filter(Boolean);
    return Array.from(new Set(all));
  }, [activeWorkspacePath, recentWorkspaces]);

  return (
    <div className="relative">
      <select
        value={value}
        aria-label="Project"
        onChange={e => onChange(e.target.value)}
        className="h-9 w-full appearance-none rounded-xl border border-border bg-input px-3 pr-8 text-ui-sm text-foreground outline-none transition-colors hover:border-border focus:border-input-border-focused"
      >
        {options.map(ws => (
          <option key={ws} value={ws}>
            {ws.split('/').filter(Boolean).pop() || ws}
          </option>
        ))}
        {!options.includes(value) && value && <option value={value}>{value}</option>}
      </select>
      <ChevronRight className="pointer-events-none absolute right-3 top-1/2 size-3.5 -translate-y-1/2 rotate-90 text-foreground-subtlest" />
    </div>
  );
};
