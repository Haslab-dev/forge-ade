import React, { useEffect, useMemo, useState } from 'react';
import {
  Check, CheckCircle2, ChevronDown, ChevronRight, Copy, ExternalLink, FileCode,
  GitBranch, Loader2, Minus, Plus, RefreshCw, RotateCcw, Sparkles
} from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';
import { ApiBridge } from '../../services/apiBridge';
import { showToast } from '../../lib/toast';
import { parseUnifiedDiff, parseUnifiedDiffByFiles, ParsedDiffLine } from '../diff/DiffViewer';
import { FileDiff } from '../../types';

/**
 * Git Control — the side panel's git surface (Zed-style): Changes (staged /
 * tracked / untracked groups, per-file staging, inline diff accordions) and
 * History (commits expandable to their changed files).
 */

type GitFile = {
  path: string;
  status: string;
  staging?: 'staged' | 'unstaged' | 'untracked';
  dir?: string;
  additions?: number;
  deletions?: number;
};

interface CommitEntry {
  hash: string;
  short_hash?: string;
  message: string;
  author: string;
  timestamp: number | string;
}

function relativeTime(ts: number | string | undefined): string {
  if (!ts) return '';
  const t = typeof ts === 'number' ? ts : Number(new Date(ts).getTime());
  if (!Number.isFinite(t) || t <= 0) return '';
  const diff = Date.now() - t;
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return 'now';
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  if (days < 30) return `${days}d ago`;
  return new Date(t).toLocaleDateString();
}

const COLLAPSE_THRESHOLD = 6;

/** Unified diff body: colored add/del rows, collapsible unmodified runs. */
const DiffBody: React.FC<{ text: string }> = ({ text }) => {
  const [expanded, setExpanded] = useState<Record<number, boolean>>({});
  const parsed = useMemo(() => parseUnifiedDiff(text), [text]);
  const contentLines = parsed.lines.filter(l => l.type !== 'file-header');

  const segments = useMemo(() => {
    const segs: Array<{ collapsed: boolean; count: number; lines: ParsedDiffLine[] }> = [];
    let run: ParsedDiffLine[] = [];
    const flush = () => {
      if (run.length === 0) return;
      segs.push({ collapsed: run.length > COLLAPSE_THRESHOLD, count: run.length, lines: run });
      run = [];
    };
    for (const l of contentLines) {
      if (l.type === 'normal') run.push(l);
      else {
        flush();
        segs.push({ collapsed: false, count: 1, lines: [l] });
      }
    }
    flush();
    return segs;
  }, [contentLines]);

  const renderRows = (rows: ParsedDiffLine[], keyPrefix: string) =>
    rows.map((line, i) => {
      if (line.type === 'header') {
        return (
          <div key={`${keyPrefix}-${i}`} className="px-3 py-0.5 text-ui-xs font-mono text-foreground-subtlest bg-surface-hover/60 select-none">
            {line.text}
          </div>
        );
      }
      const isAdd = line.type === 'add';
      const isDel = line.type === 'del';
      return (
        <div
          key={`${keyPrefix}-${i}`}
          className={`flex items-start ${
            isAdd ? 'bg-success/10 dark:bg-success/15' : isDel ? 'bg-destructive/10 dark:bg-destructive/15' : ''
          }`}
        >
          <span className="w-10 text-right pr-2 text-ui-xs leading-[18px] text-foreground-subtlest select-none shrink-0">
            {line.origLine ?? ''}
          </span>
          <span className="w-10 text-right pr-2 text-ui-xs leading-[18px] text-foreground-subtlest select-none shrink-0">
            {line.modLine ?? ''}
          </span>
          <span className={`flex-1 whitespace-pre pr-2 leading-[18px] font-mono ${isDel ? 'text-foreground/70' : 'text-foreground/90'}`}>
            {line.text}
          </span>
        </div>
      );
    });

  return (
    <div className="max-h-80 overflow-y-auto overflow-x-auto py-1 font-mono text-ui-xs leading-[18px] select-text bg-surface dark:bg-[#151516]">
      {contentLines.length === 0 ? (
        <div className="p-3 text-center text-ui-xs text-foreground-subtlest">{text || 'No diff detected.'}</div>
      ) : (
        segments.map((seg, sIdx) => {
          const key = `s${sIdx}`;
          if (seg.collapsed && !expanded[sIdx]) {
            return (
              <button
                key={key}
                type="button"
                onClick={() => setExpanded(prev => ({ ...prev, [sIdx]: true }))}
                className="w-[calc(100%-16px)] mx-2 flex items-center gap-2 px-3 py-1 my-0.5 rounded-md bg-surface-hover hover:bg-border text-foreground-subtle text-ui-xs font-sans cursor-pointer select-none transition-colors text-left"
                title="Expand unmodified lines"
              >
                <ChevronDown className="w-3 h-3 shrink-0" aria-hidden="true" />
                <span>{seg.count} unmodified lines</span>
              </button>
            );
          }
          return <React.Fragment key={key}>{renderRows(seg.lines, key)}</React.Fragment>;
        })
      )}
    </div>
  );
};

interface GitControlPanelProps {
  /** Opens a FileDiff in the side pane's full-diff tab (not the editor). */
  onOpenFullDiff: (diff: FileDiff) => void;
}

export const GitControlPanel: React.FC<GitControlPanelProps> = ({ onOpenFullDiff }) => {
  const {
    gitBranch,
    gitFiles,
    gitCommits,
    refreshGitStatus,
    refreshGitLog,
    activeWorkspacePath
  } = useWorkspace();

  const [tab, setTab] = useState<'changes' | 'history'>('changes');
  const [commitMessage, setCommitMessage] = useState('');
  const [feedback, setFeedback] = useState<string | null>(null);
  const [busy, setBusy] = useState<'stage' | 'commit' | 'fetch' | 'ai' | 'refresh' | null>(null);
  const [confirmDiscard, setConfirmDiscard] = useState<string | null>(null);
  const [collapsed, setCollapsed] = useState<Record<string, boolean>>({});
  const [copiedHash, setCopiedHash] = useState<string | null>(null);

  // Inline diff accordions (Changes): diff text cache keyed by `${staging}:${path}`.
  const [expandedFiles, setExpandedFiles] = useState<Record<string, boolean>>({});
  const [diffCache, setDiffCache] = useState<Record<string, { loading: boolean; text: string }>>({});

  // History: expanded commits → changed-file list.
  const [expandedCommits, setExpandedCommits] = useState<Record<string, boolean>>({});
  const [commitFiles, setCommitFiles] = useState<Record<string, { loading: boolean; files: Array<{ path: string; additions: number; deletions: number }>; error?: string }>>({});

  const cwd = activeWorkspacePath || '';

  const staged = useMemo(() => gitFiles.filter(f => f.staging === 'staged'), [gitFiles]);
  const tracked = useMemo(() => gitFiles.filter(f => f.staging === 'unstaged'), [gitFiles]);
  const untracked = useMemo(() => gitFiles.filter(f => f.staging === 'untracked'), [gitFiles]);
  const totalAdd = gitFiles.reduce((a, f) => a + (f.additions || 0), 0);
  const totalDel = gitFiles.reduce((a, f) => a + (f.deletions || 0), 0);

  useEffect(() => {
    if (tab === 'history') void refreshGitLog();
  }, [tab, refreshGitLog]);

  const flash = (msg: string) => {
    setFeedback(msg);
    window.setTimeout(() => setFeedback(null), 3000);
  };

  const stageFiles = async (paths: string[], unstage = false) => {
    if (paths.length === 0 || busy) return;
    setBusy('stage');
    try {
      await Promise.all(paths.map(p => (unstage ? ApiBridge.gitUnstage(p, cwd) : ApiBridge.gitStage(p, cwd))));
      await refreshGitStatus();
    } catch (err) {
      showToast(err instanceof Error ? err.message : 'Staging failed', 'error');
    } finally {
      setBusy(null);
    }
  };

  const discardFile = async (path: string) => {
    setConfirmDiscard(null);
    try {
      await ApiBridge.gitDiscard(path, cwd);
      await refreshGitStatus();
    } catch (err) {
      showToast(err instanceof Error ? err.message : 'Discard failed', 'error');
    }
  };

  const diffKeyOf = (f: GitFile) => `${f.staging}:${f.path}`;

  const toggleFileDiff = async (f: GitFile, e?: React.MouseEvent) => {
    e?.stopPropagation();
    const key = diffKeyOf(f);
    const next = !expandedFiles[key];
    setExpandedFiles(prev => ({ ...prev, [key]: next }));
    if (next && !diffCache[key]) {
      setDiffCache(prev => ({ ...prev, [key]: { loading: true, text: '' } }));
      try {
        const text = f.staging === 'untracked'
          ? await ApiBridge.readFile(f.path)
          : await ApiBridge.gitDiff(f.path, cwd, f.staging === 'staged');
        setDiffCache(prev => ({ ...prev, [key]: { loading: false, text: text || '(No difference detected)' } }));
      } catch (err: any) {
        setDiffCache(prev => ({ ...prev, [key]: { loading: false, text: `Error: ${err?.message || 'Failed to load diff'}` } }));
      }
    }
  };

  /** "Open full diff" — editor pane, pinned to this panel's workspace. */
  const openFullDiff = async (f: GitFile) => {
    try {
      let modifiedContent = '';
      let additions = f.additions || 0;
      let deletions = f.deletions || 0;
      if (f.staging === 'untracked') {
        modifiedContent = await ApiBridge.readFile(f.path);
        additions = modifiedContent.split('\n').length;
      } else {
        modifiedContent = await ApiBridge.gitDiff(f.path, cwd, f.staging === 'staged');
      }
      onOpenFullDiff({
        id: `git-${f.staging}-${f.path}`,
        filePath: f.path,
        fileName: f.path.split('/').pop() || f.path,
        originalContent: '',
        modifiedContent: modifiedContent || '(No difference detected)',
        additions,
        deletions,
        status: 'pending',
        timestamp: new Date().toISOString(),
        kind: 'git'
      });
    } catch {
      showToast('Failed to load diff', 'error');
    }
  };

  const refresh = async () => {
    setBusy('refresh');
    try {
      await refreshGitStatus();
      if (tab === 'history') await refreshGitLog();
    } finally {
      setBusy(null);
    }
  };

  const fetchRemote = async () => {
    setBusy('fetch');
    try {
      await ApiBridge.gitFetch(cwd);
      flash('✓ Fetched from remote');
    } catch (err) {
      showToast(err instanceof Error ? err.message : 'Fetch failed', 'error');
    } finally {
      setBusy(null);
    }
  };

  const generateAiMessage = async () => {
    setBusy('ai');
    try {
      const res = await ApiBridge.gitAiCommitMessage(cwd);
      if (res?.message) setCommitMessage(res.message);
    } catch (err: any) {
      showToast(err?.message || 'AI commit message failed', 'error');
    } finally {
      setBusy(null);
    }
  };

  const commit = async () => {
    if (!commitMessage.trim() || busy) return;
    setBusy('commit');
    try {
      const res = await ApiBridge.gitCommit(commitMessage.trim(), cwd);
      if (res?.success) {
        setCommitMessage('');
        flash('✓ Committed');
        await refreshGitStatus();
        void refreshGitLog();
      } else {
        showToast(res?.output || 'Commit failed', 'error');
      }
    } catch (err: any) {
      showToast(err?.message || 'Commit failed', 'error');
    } finally {
      setBusy(null);
    }
  };

  const copyHash = (hash: string) => {
    void navigator.clipboard.writeText(hash);
    setCopiedHash(hash);
    window.setTimeout(() => setCopiedHash(null), 1500);
  };

  /** Expand a commit → parse its changed-file list out of the commit diff. */
  const toggleCommit = async (hash: string) => {
    const next = !expandedCommits[hash];
    setExpandedCommits(prev => ({ ...prev, [hash]: next }));
    if (!next || commitFiles[hash]) return;
    setCommitFiles(prev => ({ ...prev, [hash]: { loading: true, files: [] } }));
    try {
      const raw = await ApiBridge.gitCommitDiff(hash, cwd);
      const sections = parseUnifiedDiffByFiles(raw || '');
      const files = sections.map(s => ({
        path: s.filePath,
        additions: s.additions,
        deletions: s.deletions
      }));
      setCommitFiles(prev => ({ ...prev, [hash]: { loading: false, files } }));
    } catch (err: any) {
      setCommitFiles(prev => ({ ...prev, [hash]: { loading: false, files: [], error: err?.message || 'Failed to load commit files' } }));
    }
  };

  /** Click a changed file inside a commit → diff in the editor pane. */
  const openCommitFileDiff = async (hash: string, path: string) => {
    try {
      const text = await ApiBridge.gitCommitFileDiff(hash, path, cwd);
      onOpenFullDiff({
        id: `git-commit-${hash}-${path}`,
        filePath: path,
        fileName: path.split('/').pop() || path,
        originalContent: '',
        modifiedContent: text || '(No difference detected)',
        additions: 0,
        deletions: 0,
        status: 'pending',
        timestamp: new Date().toISOString(),
        kind: 'git'
      });
    } catch (err: any) {
      showToast(err?.message || 'Failed to load commit file diff', 'error');
    }
  };

  const groups: Array<{ key: string; label: string; files: GitFile[]; checked: boolean }> = [
    { key: 'staged', label: 'Staged', files: staged, checked: true },
    { key: 'tracked', label: 'Tracked', files: tracked, checked: false },
    { key: 'untracked', label: 'Untracked', files: untracked, checked: false }
  ];

  const renderRow = (f: GitFile, isStaged: boolean) => {
    const name = f.path.split('/').pop() || f.path;
    const dir = f.dir || f.path.slice(0, Math.max(0, f.path.length - name.length - 1));
    const key = diffKeyOf(f);
    const isOpen = !!expandedFiles[key];
    const cached = diffCache[key];
    const isDiscarding = confirmDiscard === f.path;
    return (
      <div key={f.path} className="group">
        <div
          className={`flex items-center gap-2 h-7 pl-1.5 pr-2 cursor-pointer transition-colors ${
            isOpen ? 'bg-surface-hover' : isDiscarding ? 'bg-destructive/10' : 'hover:bg-surface-hover'
          }`}
          onClick={(e) => void toggleFileDiff(f, e)}
          title={`${f.path} — click to ${isOpen ? 'collapse' : 'expand'} diff`}
        >
          <span className="text-foreground-subtle shrink-0 w-3.5 flex justify-center">
            {isOpen ? <ChevronDown className="w-3.5 h-3.5" /> : <ChevronRight className="w-3.5 h-3.5" />}
          </span>
          <FileCode className="w-3.5 h-3.5 text-foreground-subtle shrink-0" aria-hidden="true" />
          <span className="truncate text-ui-sm text-foreground">{name}</span>
          {dir && <span className="truncate text-ui-xs text-foreground-subtle hidden sm:inline">{dir}</span>}
          <span className="ml-auto flex items-center gap-2 shrink-0 font-mono text-ui-xs">
            {(f.additions || 0) > 0 && <span className="text-success">+{f.additions}</span>}
            {(f.deletions || 0) > 0 && <span className="text-destructive">−{f.deletions}</span>}
          </span>
          <div
            className="flex items-center gap-0.5 shrink-0 opacity-0 group-hover:opacity-100 transition-opacity"
            onClick={e => e.stopPropagation()}
          >
            <button
              type="button"
              onClick={() => void stageFiles([f.path], isStaged)}
              disabled={busy === 'stage'}
              title={isStaged ? 'Unstage' : 'Stage'}
              className="p-0.5 rounded hover:bg-border text-foreground-subtle hover:text-foreground transition-colors cursor-pointer disabled:opacity-40"
            >
              {isStaged ? <Minus className="w-3.5 h-3.5" /> : <Plus className="w-3.5 h-3.5" />}
            </button>
            {f.staging !== 'untracked' && (
              <button
                type="button"
                onClick={() => setConfirmDiscard(f.path)}
                title="Discard changes"
                className="p-0.5 rounded hover:bg-destructive/10 text-foreground-subtle hover:text-destructive transition-colors cursor-pointer"
              >
                <RotateCcw className="w-3.5 h-3.5" />
              </button>
            )}
            <button
              type="button"
              onClick={(e) => { e.stopPropagation(); void openFullDiff(f); }}
              title="Open full diff in editor"
              className="p-0.5 rounded hover:bg-border text-foreground-subtle hover:text-foreground transition-colors cursor-pointer"
            >
              <ExternalLink className="w-3.5 h-3.5" />
            </button>
          </div>
          <button
            type="button"
            role="checkbox"
            aria-checked={isStaged}
            aria-label={isStaged ? `Unstage ${name}` : `Stage ${name}`}
            onClick={(e) => { e.stopPropagation(); void stageFiles([f.path], isStaged); }}
            className={`size-3.5 shrink-0 rounded-[3px] border flex items-center justify-center transition-colors cursor-pointer ${
              isStaged ? 'bg-accent-primary border-accent-primary text-white' : 'border-foreground-subtle/50 hover:border-foreground'
            }`}
          >
            {isStaged && <Check className="w-3 h-3" aria-hidden="true" />}
          </button>
        </div>

        {isDiscarding && (
          <div className="flex items-center gap-1 px-6 py-1 bg-destructive/10">
            <span className="text-ui-xs text-destructive font-semibold">Revert changes to {name}?</span>
            <button
              type="button"
              onClick={() => void discardFile(f.path)}
              className="px-1.5 py-0.5 rounded bg-destructive text-white text-ui-xs font-medium cursor-pointer"
            >
              Yes
            </button>
            <button
              type="button"
              onClick={() => setConfirmDiscard(null)}
              className="px-1.5 py-0.5 rounded bg-surface-hover text-foreground-subtle text-ui-xs cursor-pointer"
            >
              No
            </button>
          </div>
        )}

        {isOpen && (
          <div className="border-t border-border">
            {cached && !cached.loading ? (
              <>
                <div className="flex items-center justify-between px-3 py-1 bg-surface-hover/60 border-b border-border">
                  <span className="font-mono text-ui-xs text-foreground-subtle truncate" title={f.path}>{f.path}</span>
                  <button
                    type="button"
                    onClick={(e) => { e.stopPropagation(); void openFullDiff(f); }}
                    className="flex items-center gap-1 px-1.5 py-0.5 rounded hover:bg-border text-foreground-subtle hover:text-foreground text-ui-xs font-sans transition-colors cursor-pointer"
                  >
                    <ExternalLink className="w-3 h-3" aria-hidden="true" /> Full diff
                  </button>
                </div>
                <DiffBody text={cached.text} />
              </>
            ) : (
              <div className="p-3 flex items-center justify-center gap-2 text-ui-xs text-foreground-subtle">
                <Loader2 className="w-3.5 h-3.5 animate-spin text-success" aria-hidden="true" />
                Loading diff for {name}...
              </div>
            )}
          </div>
        )}
      </div>
    );
  };

  const renderGroup = (g: { key: string; label: string; files: GitFile[]; checked: boolean }) => {
    if (g.files.length === 0) return null;
    const isCollapsed = !!collapsed[g.key];
    const add = g.files.reduce((a, f) => a + (f.additions || 0), 0);
    const del = g.files.reduce((a, f) => a + (f.deletions || 0), 0);
    return (
      <div className="mb-1">
        <div className="flex items-center gap-1.5 h-7 px-2 text-ui-xs font-semibold text-foreground-subtle uppercase tracking-wide">
          <button
            type="button"
            onClick={() => setCollapsed(prev => ({ ...prev, [g.key]: !prev[g.key] }))}
            className="flex items-center gap-1.5 cursor-pointer hover:text-foreground transition-colors"
            aria-expanded={!isCollapsed}
          >
            {isCollapsed ? <ChevronRight className="w-3 h-3" /> : <ChevronDown className="w-3 h-3" />}
            <span>{g.label}</span>
            <span className="text-foreground-subtle">({g.files.length})</span>
          </button>
          <span className="ml-auto font-mono normal-case">
            {add > 0 && <span className="text-success">+{add}</span>}
            {del > 0 && <span className="text-destructive ml-2">−{del}</span>}
          </span>
          <button
            type="button"
            onClick={() => void stageFiles(g.files.map(f => f.path), g.checked)}
            disabled={busy === 'stage'}
            title={g.checked ? `Unstage all ${g.label.toLowerCase()} files` : `Stage all ${g.label.toLowerCase()} files`}
            className="p-0.5 rounded hover:bg-border text-foreground-subtle hover:text-foreground cursor-pointer disabled:opacity-40"
          >
            {g.checked ? <Minus className="w-3.5 h-3.5" /> : <Plus className="w-3.5 h-3.5" />}
          </button>
        </div>
        {!isCollapsed && g.files.map(f => renderRow(f, g.checked))}
      </div>
    );
  };

  return (
    <div className="flex-1 min-h-0 flex flex-col overflow-hidden">
      {/* Tab bar: Changes | History */}
      <div className="flex items-stretch border-b border-border shrink-0">
        {([['changes', `Changes (${gitFiles.length})`], ['history', 'History']] as const).map(([id, label]) => (
          <button
            key={id}
            type="button"
            onClick={() => setTab(id)}
            aria-selected={tab === id}
            role="tab"
            className={`flex-1 h-9 text-ui-sm font-medium transition-colors cursor-pointer ${
              tab === id
                ? 'text-foreground border-b-2 border-primary'
                : 'text-foreground-subtle hover:text-foreground'
            }`}
          >
            {label}
          </button>
        ))}
      </div>

      {tab === 'changes' ? (
        <>
          {/* Summary row */}
          <div className="h-9 px-2 flex items-center justify-between gap-2 border-b border-border shrink-0">
            <span className="flex items-center gap-2 text-ui-sm text-foreground-subtle">
              View Diff
              <span className="font-mono text-ui-xs">
                {totalAdd > 0 && <span className="text-success">+{totalAdd.toLocaleString()}</span>}
                {totalDel > 0 && <span className="text-destructive ml-1">−{totalDel.toLocaleString()}</span>}
              </span>
            </span>
            <div className="flex items-center gap-1">
              <button
                type="button"
                onClick={() => void refresh()}
                disabled={busy === 'refresh'}
                title="Refresh git status"
                className="p-1 rounded text-foreground-subtle hover:text-foreground hover:bg-surface-hover transition-colors cursor-pointer disabled:opacity-40"
              >
                <RefreshCw className={`w-3.5 h-3.5 ${busy === 'refresh' ? 'animate-spin' : ''}`} />
              </button>
              <button
                type="button"
                onClick={() => void stageFiles(gitFiles.filter(f => f.staging !== 'staged').map(f => f.path))}
                disabled={busy === 'stage' || staged.length === gitFiles.length}
                className="px-2 py-1 rounded-md bg-surface-hover hover:bg-border text-foreground text-ui-xs font-medium transition-colors cursor-pointer disabled:opacity-40"
              >
                {busy === 'stage' ? <Loader2 className="w-3 h-3 animate-spin" /> : 'Stage All'}
              </button>
            </div>
          </div>

          {/* File groups with inline diffs */}
          <div className="flex-1 min-h-0 overflow-y-auto py-1">
            {gitFiles.length === 0 ? (
              <div className="py-16 text-center space-y-2">
                <CheckCircle2 className="w-8 h-8 text-success mx-auto opacity-75" />
                <p className="text-xs font-medium text-foreground-subtle">No changes detected</p>
                <p className="text-ui-xs text-foreground-subtlest max-w-[220px] mx-auto">
                  Workspace tree matches git index cleanly.
                </p>
              </div>
            ) : (
              groups.map(renderGroup)
            )}
          </div>

          {/* Branch + fetch */}
          <div className="h-8 px-2 flex items-center justify-between border-t border-border shrink-0">
            <span className="flex items-center gap-1.5 text-ui-xs text-foreground-subtle min-w-0">
              <GitBranch className="w-3.5 h-3.5 shrink-0" />
              <span className="truncate">{gitBranch || 'main'}</span>
            </span>
            <button
              type="button"
              onClick={() => void fetchRemote()}
              disabled={busy === 'fetch'}
              className="flex items-center gap-1 px-1.5 py-0.5 rounded text-ui-xs text-foreground-subtle hover:text-foreground hover:bg-surface-hover transition-colors cursor-pointer disabled:opacity-40"
              title="Fetch from remote"
            >
              <RefreshCw className={`w-3 h-3 ${busy === 'fetch' ? 'animate-spin' : ''}`} />
              Fetch
            </button>
          </div>

          {/* Commit box */}
          <div className="p-2 border-t border-border space-y-1.5 shrink-0">
            {feedback && (
              <div className="text-ui-xs px-2 py-1 rounded bg-surface-hover text-foreground-subtle font-mono">
                {feedback}
              </div>
            )}
            <textarea
              value={commitMessage}
              onChange={e => setCommitMessage(e.target.value)}
              onKeyDown={e => {
                if (e.key === 'Enter' && (e.metaKey || e.ctrlKey)) void commit();
              }}
              placeholder="Enter commit message"
              rows={2}
              className="w-full resize-none bg-surface border border-border rounded-md px-2 py-1.5 text-ui-sm text-foreground placeholder-foreground-subtlest focus:outline-hidden focus:border-primary font-mono"
            />
            <div className="flex items-center gap-1.5">
              <button
                type="button"
                onClick={() => void generateAiMessage()}
                disabled={busy === 'ai'}
                title="Generate commit message with AI"
                className="p-1.5 rounded-md bg-surface-hover hover:bg-border text-primary transition-colors cursor-pointer disabled:opacity-40"
              >
                {busy === 'ai' ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <Sparkles className="w-3.5 h-3.5" />}
              </button>
              <button
                type="button"
                onClick={() => void commit()}
                disabled={!commitMessage.trim() || busy === 'commit'}
                title={staged.length > 0 ? `Commit ${staged.length} staged file(s)` : 'Stage all changes and commit'}
                className="ml-auto px-3 py-1.5 rounded-md bg-success text-white text-ui-xs font-medium transition-colors cursor-pointer disabled:opacity-40 flex items-center gap-1.5"
              >
                {busy === 'commit' ? <Loader2 className="w-3 h-3 animate-spin" /> : <Check className="w-3 h-3" />}
                {staged.length > 0 ? `Commit Staged (${staged.length})` : 'Commit All'}
              </button>
            </div>
          </div>
        </>
      ) : (
        /* History: commits expandable to their changed files */
        <div className="flex-1 min-h-0 overflow-y-auto py-1">
          {gitCommits.length === 0 ? (
            <div className="py-16 text-center text-ui-xs text-foreground-subtlest">
              No commit history available.
            </div>
          ) : (
            gitCommits.map((c: CommitEntry) => {
              const hash = c.short_hash || c.hash.slice(0, 7);
              const isOpen = !!expandedCommits[c.hash];
              const files = commitFiles[c.hash];
              return (
                <div key={c.hash} className="border-b border-border/40 last:border-0">
                  <div
                    role="button"
                    tabIndex={0}
                    onClick={() => void toggleCommit(c.hash)}
                    onKeyDown={(e) => { if (e.key === 'Enter' || e.key === ' ') { e.preventDefault(); void toggleCommit(c.hash); } }}
                    className="w-full text-left px-3 py-2 hover:bg-surface-hover transition-colors cursor-pointer group"
                  >
                    <p className="text-ui-sm text-foreground truncate">
                      <span className="inline-block w-3 mr-1 text-foreground-subtle">
                        {files?.loading ? <Loader2 className="w-3 h-3 animate-spin" aria-hidden="true" /> : (isOpen ? <ChevronDown className="w-3 h-3" aria-hidden="true" /> : <ChevronRight className="w-3 h-3" aria-hidden="true" />)}
                      </span>
                      {c.message}
                    </p>
                    <p className="mt-0.5 flex items-center gap-1.5 text-ui-xs text-foreground-subtle font-mono">
                      <span className="truncate not-italic">{c.author}</span>
                      <span aria-hidden="true">•</span>
                      <span>{relativeTime(c.timestamp)}</span>
                      <span aria-hidden="true">•</span>
                      <span>{hash}</span>
                      {files && !files.loading && files.files.length > 0 && (
                        <>
                          <span aria-hidden="true">•</span>
                          <span>{files.files.length} file{files.files.length === 1 ? '' : 's'}</span>
                        </>
                      )}
                      <button
                        type="button"
                        onClick={(e) => { e.stopPropagation(); copyHash(c.hash); }}
                        title="Copy full hash"
                        className="ml-auto opacity-0 group-hover:opacity-100 p-0.5 rounded hover:bg-border text-foreground-subtle hover:text-foreground transition-colors cursor-pointer shrink-0"
                      >
                        <Copy className="w-3 h-3" aria-hidden="true" />
                      </button>
                      {copiedHash === c.hash && <span className="text-success">copied</span>}
                    </p>
                  </div>

                  {isOpen && (
                    <div className="pb-1">
                      {files?.loading && (
                        <div className="px-6 py-1.5 text-ui-xs text-foreground-subtle flex items-center gap-1.5">
                          <Loader2 className="w-3 h-3 animate-spin" aria-hidden="true" /> Loading changed files...
                        </div>
                      )}
                      {files?.error && (
                        <div className="px-6 py-1.5 text-ui-xs text-destructive">{files.error}</div>
                      )}
                      {files && !files.loading && files.files.length === 0 && !files.error && (
                        <div className="px-6 py-1.5 text-ui-xs text-foreground-subtlest">No file changes parsed.</div>
                      )}
                      {files && !files.loading && files.files.map(file => (
                        <button
                          key={file.path}
                          type="button"
                          onClick={() => void openCommitFileDiff(c.hash, file.path)}
                          title={`${file.path} — open diff in editor`}
                          className="w-full flex items-center gap-2 pl-8 pr-3 h-7 hover:bg-surface-hover transition-colors cursor-pointer text-left"
                        >
                          <FileCode className="w-3.5 h-3.5 text-foreground-subtle shrink-0" aria-hidden="true" />
                          <span className="truncate text-ui-sm text-foreground">
                            {file.path.split('/').pop()}
                          </span>
                          <span className="truncate text-ui-xs text-foreground-subtle hidden sm:inline">
                            {file.path.slice(0, Math.max(0, file.path.length - (file.path.split('/').pop()?.length || 0) - 1))}
                          </span>
                          <span className="ml-auto flex items-center gap-2 shrink-0 font-mono text-ui-xs">
                            {file.additions > 0 && <span className="text-success">+{file.additions}</span>}
                            {file.deletions > 0 && <span className="text-destructive">−{file.deletions}</span>}
                          </span>
                        </button>
                      ))}
                    </div>
                  )}
                </div>
              );
            })
          )}
        </div>
      )}
    </div>
  );
};
