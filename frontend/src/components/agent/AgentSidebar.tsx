import React, { useState, useMemo, useRef, useEffect } from 'react';
import {
  Plus,
  Search,
  CalendarClock,
  Folder,
  ChevronDown,
  ChevronRight,
  PanelLeft,
  Check,
  ListFilter,
  Clock,
  CheckCircle2,
  Trash2,
  Archive,
  Maximize2,
  Minimize2,
  ListTodo,
  CirclePlus,
  RotateCcw,
  Square,
  Kanban,
  FolderGit2,
  Pin,
  BellOff,
  X
} from 'lucide-react';
import { cn, formatDisplayTitle } from '../../lib/utils';
import { useWorkspace } from '../../stores/workspaceStore';
import { useSessionStore, sessionStatusLabel } from '../../stores/sessionStore';
import { formatSessionTime, getSessionEpoch } from '../../lib/time';
import { AgentSession } from '../../types';
import type { TerminalSessionRecord } from '../../types';
import { ProviderIcon } from './ProviderIcon';
import { SidebarFileExplorer } from './SidebarFileExplorer';
import { ProfileRow } from '../shell/ProfileRow';

function loadPinnedSessions(): Set<string> {
  try {
    const raw = localStorage.getItem('forge:pinned-sessions');
    if (!raw) return new Set();
    const parsed = JSON.parse(raw);
    return new Set(Array.isArray(parsed) ? parsed : []);
  } catch {
    return new Set();
  }
}

function savePinnedSessions(set: Set<string>) {
  try {
    localStorage.setItem('forge:pinned-sessions', JSON.stringify(Array.from(set)));
  } catch {}
}

function loadSnoozedSessions(): Record<string, number> {
  try {
    const raw = localStorage.getItem('forge:snoozed-sessions');
    if (!raw) return {};
    const parsed = JSON.parse(raw);
    return parsed && typeof parsed === 'object' ? parsed : {};
  } catch {
    return {};
  }
}

function saveSnoozedSessions(record: Record<string, number>) {
  try {
    localStorage.setItem('forge:snoozed-sessions', JSON.stringify(record));
  } catch {}
}

function HighlightMatch({ text, query }: { text: string; query?: string }) {
  if (!query || !query.trim()) return <>{text}</>;
  const escaped = query.trim().replace(/[-/\\^$*+?.()|[\]{}]/g, '\\$&');
  const parts = text.split(new RegExp(`(${escaped})`, 'gi'));
  return (
    <>
      {parts.map((part, i) =>
        part.toLowerCase() === query.trim().toLowerCase() ? (
          <mark key={i} className="bg-primary/25 text-primary rounded-xs px-0.5 font-semibold">
            {part}
          </mark>
        ) : (
          part
        )
      )}
    </>
  );
}

/**
 * Workspace sidebar ported 1:1 from ZCode WorkspaceSidebar:
 * macOS window drag strip, top actions (+ New Task, Search, Automations, Plugins, Skills),
 * Tasks section with archive/collapse controls, project trees, and user profile footer.
 *
 * One unified task list: ForgeADE chat tasks and agent CLI terminal tasks are
 * merged per project; selecting a task shows it in the workspace surface.
 */

interface AgentSidebarProps {
  onCollapse?: () => void;
}

type TaskFilter = 'all' | 'running' | 'finished';

type UnifiedEntry =
  | { kind: 'forge'; epoch: number; sess: AgentSession }
  | { kind: 'cli'; epoch: number; sess: TerminalSessionRecord };

type ListSortBy = 'updated' | 'created';

/** Epoch for the chosen sort: "updated" = last activity, "created" = birth. */
function entryEpochFor(entry: UnifiedEntry, sortBy: ListSortBy): number {
  if (entry.kind === 'cli') {
    const s = entry.sess;
    if (sortBy === 'created') return s.createdAt || 0;
    return s.startedAt || s.endedAt || s.createdAt || 0;
  }
  if (sortBy === 'created') {
    const parsed = Date.parse(entry.sess.createdAt || '');
    return Number.isNaN(parsed) ? 0 : parsed;
  }
  return entry.epoch;
}

/** Task row: 16px indicator slot with active/running dot, title, time + hover archive/delete. */
const TaskRow: React.FC<{
  sess: AgentSession;
  activeSessionId: string | null;
  isArchived: boolean;
  isPinned: boolean;
  isSnoozed: boolean;
  onSelect: (sess: AgentSession) => void;
  onDelete: (id: string) => void;
  onToggleArchive: (id: string) => void;
  onTogglePin: (id: string) => void;
  onToggleSnooze: (id: string) => void;
  showProject?: boolean;
  searchQuery?: string;
}> = ({
  sess,
  activeSessionId,
  isArchived,
  isPinned,
  isSnoozed,
  onSelect,
  onDelete,
  onToggleArchive,
  onTogglePin,
  onToggleSnooze,
  showProject,
  searchQuery
}) => {
  const isActive = sess.id === activeSessionId;
  const isRunning = sess.status === 'running';
  const isThinking = sess.goState === 'thinking';
  const isAwaitingApproval = sess.goState === 'awaiting_approval' || (sess.pendingTools && sess.pendingTools.length > 0);
  const isCompleted = sess.status === 'completed';
  const projectLeaf = sess.workspacePath
    ? sess.workspacePath.split('/').filter(Boolean).pop()
    : undefined;

  const diffStats = useMemo(() => {
    if (!sess.diffs || sess.diffs.length === 0) return null;
    let additions = 0;
    let deletions = 0;
    for (const d of sess.diffs) {
      additions += d.additions || 0;
      deletions += d.deletions || 0;
    }
    if (additions === 0 && deletions === 0) return null;
    return { additions, deletions };
  }, [sess.diffs]);

  const displayTitle = formatDisplayTitle(sess.title) || 'New Session';

  return (
    <div
      role="button"
      tabIndex={0}
      onClick={() => onSelect(sess)}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          onSelect(sess);
        }
      }}
      className={cn(
        'group/task-item flex cursor-pointer items-center gap-2 rounded-[8px] py-1.5 pl-2.5 pr-2 transition-colors select-none',
        isActive
          ? 'bg-foreground/10 text-foreground font-medium'
          : 'text-foreground/80 hover:bg-foreground/10 hover:text-foreground',
        isArchived && !isActive ? 'opacity-65' : '',
        isSnoozed && !isActive ? 'opacity-50' : ''
      )}
    >
      {/* Dynamic agent icon & status */}
      <div className="relative flex size-3.5 shrink-0 items-center justify-center">
        <ProviderIcon
          provider={sess.agentId || sess.agentType || sess.provider || 'internal'}
          size={13}
          className={cn(
            'shrink-0 transition-opacity',
            isActive ? 'opacity-100' : 'opacity-70 group-hover/task-item:opacity-100'
          )}
        />
        {isAwaitingApproval ? (
          <span className="absolute -top-1 -right-1 size-2 rounded-full bg-amber-500 animate-bounce ring-2 ring-background" title="Permission required" />
        ) : isThinking ? (
          <span className="absolute -top-1 -right-1 size-2 rounded-full bg-primary animate-ping ring-2 ring-background" title="Thinking" />
        ) : isRunning ? (
          <span className="absolute -top-0.5 -right-0.5 size-1.5 animate-pulse rounded-full bg-primary" />
        ) : isCompleted ? (
          <span className="absolute -top-0.5 -right-0.5 size-1.5 rounded-full bg-success/80" />
        ) : null}
      </div>

      <span
        className={cn(
          'min-w-0 flex-1 truncate text-ui-sm flex items-center gap-1.5',
          isActive ? 'font-medium text-foreground' : 'font-normal text-foreground/95'
        )}
        title={displayTitle}
      >
        <span className="truncate">
          <HighlightMatch text={displayTitle} query={searchQuery} />
        </span>
        {isPinned && (
          <Pin className="size-2.5 text-primary shrink-0 rotate-45 fill-primary" />
        )}
      </span>

      {/* Diff stats badge */}
      {diffStats && (
        <span className="inline-flex items-center gap-0.5 rounded px-1 py-0.2 text-[10px] font-mono font-medium bg-surface/80 border border-border/40 shrink-0">
          <span className="text-diff-addition">+{diffStats.additions}</span>
          <span className="text-diff-deletion">-{diffStats.deletions}</span>
        </span>
      )}

      {isAwaitingApproval && (
        <span className="shrink-0 rounded bg-amber-500/15 text-amber-500 border border-amber-500/25 px-1 py-0.2 text-[10px] font-medium leading-none">
          Approval
        </span>
      )}

      <div className="flex shrink-0 items-center gap-1">
        {showProject && projectLeaf ? (
          <span className="whitespace-nowrap text-ui-xs text-foreground-subtle group-hover/task-item:hidden">
            {projectLeaf}
          </span>
        ) : null}
        <span className="whitespace-nowrap text-ui-xs text-foreground-subtle group-hover/task-item:hidden">
          {formatSessionTime(sess)}
        </span>
        <button
          type="button"
          onClick={(e) => {
            e.stopPropagation();
            onTogglePin(sess.id);
          }}
          className={cn(
            "size-5 items-center justify-center rounded p-0.5 transition-colors cursor-pointer",
            isPinned
              ? "flex text-primary hover:text-primary hover:bg-white/5"
              : "hidden text-foreground-subtle hover:text-foreground hover:bg-white/5 group-hover/task-item:flex"
          )}
          title={isPinned ? "Unpin task" : "Pin task to top"}
        >
          <Pin className={cn("size-3", isPinned ? "fill-primary" : "")} />
        </button>
        <button
          type="button"
          onClick={(e) => {
            e.stopPropagation();
            onToggleSnooze(sess.id);
          }}
          className={cn(
            "size-5 items-center justify-center rounded p-0.5 transition-colors cursor-pointer",
            isSnoozed
              ? "flex text-amber-500 hover:text-amber-400 hover:bg-white/5"
              : "hidden text-foreground-subtle hover:text-foreground hover:bg-white/5 group-hover/task-item:flex"
          )}
          title={isSnoozed ? "Unsnooze task" : "Snooze task for 24h"}
        >
          <BellOff className="size-3" />
        </button>
        <button
          type="button"
          onClick={(e) => {
            e.stopPropagation();
            onToggleArchive(sess.id);
          }}
          className="hidden size-5 items-center justify-center rounded p-0.5 text-foreground-subtle hover:text-foreground hover:bg-white/5 transition-colors group-hover/task-item:flex cursor-pointer"
          title={isArchived ? "Unarchive task" : "Archive task"}
        >
          <Archive className={cn("size-3.5", isArchived ? "text-foreground" : "")} />
        </button>
        <button
          type="button"
          onClick={(e) => {
            e.stopPropagation();
            onDelete(sess.id);
          }}
          className="hidden size-5 items-center justify-center rounded p-0.5 text-foreground-subtle hover:text-destructive hover:bg-white/5 transition-colors group-hover/task-item:flex cursor-pointer"
          title="Delete task"
        >
          <Trash2 className="size-3.5" />
        </button>
      </div>
    </div>
  );
};

/** Terminal session row — same layout language as TaskRow: status dot, agent
    name, task title, compact status, hover stop/restart/delete actions. */
const TerminalSessionRow: React.FC<{
  session: TerminalSessionRecord;
  activeSessionId: string | null;
  isArchived: boolean;
  isPinned: boolean;
  isSnoozed: boolean;
  onToggleArchive: (id: string) => void;
  onTogglePin: (id: string) => void;
  onToggleSnooze: (id: string) => void;
  onSelect: (s: TerminalSessionRecord) => void;
  onStop: (id: string) => void;
  onRestart: (id: string) => void;
  onDelete: (id: string) => void;
  searchQuery?: string;
}> = ({
  session,
  activeSessionId,
  isArchived,
  isPinned,
  isSnoozed,
  onToggleArchive,
  onTogglePin,
  onToggleSnooze,
  onSelect,
  onStop,
  onRestart,
  onDelete,
  searchQuery
}) => {
  const isActive = session.id === activeSessionId;
  const isRunning = session.status === 'running' || session.status === 'starting';
  const displayTitle = formatDisplayTitle(session.title) || session.agentName || 'Terminal Session';

  return (
    <div
      role="button"
      tabIndex={0}
      onClick={() => onSelect(session)}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault();
          onSelect(session);
        }
      }}
      data-testid={`terminal-session-row-${session.agentId}`}
      className={cn(
        'group/session-item flex cursor-pointer items-center gap-2 rounded-[8px] py-1.5 pl-2.5 pr-2 transition-colors select-none',
        isActive
          ? 'bg-foreground/10 text-foreground font-medium'
          : 'text-foreground/80 hover:bg-foreground/10 hover:text-foreground',
        isArchived && !isActive ? 'opacity-65' : '',
        isSnoozed && !isActive ? 'opacity-50' : ''
      )}
    >
      {/* Dynamic agent icon */}
      <div className="relative flex size-3.5 shrink-0 items-center justify-center">
        <ProviderIcon
          provider={session.agentId || session.agentName || 'internal'}
          size={13}
          className={cn(
            'shrink-0 transition-opacity',
            isActive ? 'opacity-100' : 'opacity-70 group-hover/session-item:opacity-100'
          )}
        />
        {isRunning && (
          <span className="absolute -top-0.5 -right-0.5 size-1.5 animate-pulse rounded-full bg-primary">
            <span className="sr-only">Running</span>
          </span>
        )}
      </div>

      {/* Session title */}
      <span
        className={cn(
          'min-w-0 flex-1 truncate text-ui-sm flex items-center gap-1.5',
          isActive ? 'font-medium text-foreground' : 'font-normal text-foreground/95'
        )}
        title={displayTitle}
      >
        <span className="truncate">
          <HighlightMatch text={displayTitle} query={searchQuery} />
        </span>
        {isPinned && (
          <Pin className="size-2.5 text-primary shrink-0 rotate-45 fill-primary" />
        )}
      </span>

      <div className="flex shrink-0 items-center gap-1">
        <span className="flex items-center gap-1.5 whitespace-nowrap text-ui-xs text-foreground-subtle group-hover/session-item:hidden">
          {isRunning && <span className="size-1.5 shrink-0 rounded-full bg-success animate-status-pulse" aria-hidden="true" />}
          {sessionStatusLabel(session)}
        </span>
        {isRunning ? (
          <button
            type="button"
            onClick={(e) => {
              e.stopPropagation();
              onStop(session.id);
            }}
            className="hidden size-5 items-center justify-center rounded p-0.5 text-foreground-subtle hover:text-foreground hover:bg-white/5 transition-colors group-hover/session-item:flex cursor-pointer"
            title="Stop session"
          >
            <Square className="size-3" />
          </button>
        ) : (
          <button
            type="button"
            onClick={(e) => {
              e.stopPropagation();
              onRestart(session.id);
            }}
            className="hidden size-5 items-center justify-center rounded p-0.5 text-foreground-subtle hover:text-foreground hover:bg-white/5 transition-colors group-hover/session-item:flex cursor-pointer"
            title="Restart session"
          >
            <RotateCcw className="size-3" />
          </button>
        )}
        <button
          type="button"
          onClick={(e) => {
            e.stopPropagation();
            onTogglePin(session.id);
          }}
          className={cn(
            "size-5 items-center justify-center rounded p-0.5 transition-colors cursor-pointer",
            isPinned
              ? "flex text-primary hover:text-primary hover:bg-white/5"
              : "hidden text-foreground-subtle hover:text-foreground hover:bg-white/5 group-hover/session-item:flex"
          )}
          title={isPinned ? "Unpin session" : "Pin session to top"}
        >
          <Pin className={cn("size-3", isPinned ? "fill-primary" : "")} />
        </button>
        <button
          type="button"
          onClick={(e) => {
            e.stopPropagation();
            onToggleSnooze(session.id);
          }}
          className={cn(
            "size-5 items-center justify-center rounded p-0.5 transition-colors cursor-pointer",
            isSnoozed
              ? "flex text-amber-500 hover:text-amber-400 hover:bg-white/5"
              : "hidden text-foreground-subtle hover:text-foreground hover:bg-white/5 group-hover/session-item:flex"
          )}
          title={isSnoozed ? "Unsnooze session" : "Snooze session for 24h"}
        >
          <BellOff className="size-3" />
        </button>
        <button
          type="button"
          onClick={(e) => {
            e.stopPropagation();
            onToggleArchive(session.id);
          }}
          className="hidden size-5 items-center justify-center rounded p-0.5 text-foreground-subtle hover:text-foreground hover:bg-white/5 transition-colors group-hover/session-item:flex cursor-pointer"
          title={isArchived ? 'Unarchive session' : 'Archive session'}
        >
          <Archive className={cn('size-3', isArchived && 'text-foreground')} />
        </button>
        <button
          type="button"
          onClick={(e) => {
            e.stopPropagation();
            onDelete(session.id);
          }}
          className="hidden size-5 items-center justify-center rounded p-0.5 text-foreground-subtle hover:text-destructive hover:bg-white/5 transition-colors group-hover/session-item:flex cursor-pointer"
          title="Delete session"
        >
          <Trash2 className="size-3" />
        </button>
      </div>
    </div>
  );
};

export const AgentSidebar: React.FC<AgentSidebarProps> = () => {
  const {
    sessions,
    savedSessions,
    activeSessionId,
    setActiveSessionId,
    activeTaskKind,
    setActiveTaskKind,
    activeWorkspacePath,
    setActiveWorkspacePath,
    activeWorkspaces,
    closeWorkspace,
    closeWorkspaceFromWindow,
    removeRecentWorkspace,
    deleteSessionPermanently,
    setIsCommandPaletteOpen,
    openSettingsTab,
    setMode,
    switchSurface,
    mode,
    theme,
    archivedSessionIds,
    toggleArchiveSession,
    leftSidebarView,
    setLeftSidebarView,
    leftSidebarWidth,
    setIsLeftSidebarOpen,
    toggleTheme,
    isAutomationsOpen,
    setIsAutomationsOpen,
    agentViewMode,
    setAgentViewMode,
  } = useWorkspace();
  const {
    sessions: terminalSessions,
    activeSessionId: activeTerminalSessionId,
    selectSession,
    stopSession,
    restartSession,
    deleteSession,
    openNewTask
  } = useSessionStore();

  const [collapsedGroups, setCollapsedGroups] = useState<Record<string, boolean>>({});
  const [listView, setListView] = useState<'project' | 'timeline'>('project');
  const [sortBy, setSortBy] = useState<ListSortBy>('updated');
  const [viewMenuOpen, setViewMenuOpen] = useState(false);
  const [allCollapsed, setAllCollapsed] = useState(false);
  const [showArchived, setShowArchived] = useState(false);
  const [taskFilter, setTaskFilter] = useState<TaskFilter>('all');
  const [sidebarTab, setSidebarTab] = useState<'active' | 'pinned' | 'snoozed' | 'archived'>('active');
  const [searchQuery, setSearchQuery] = useState('');
  const [pinnedIds, setPinnedIds] = useState<Set<string>>(() => loadPinnedSessions());
  const [snoozedMap, setSnoozedMap] = useState<Record<string, number>>(() => loadSnoozedSessions());

  const togglePin = (id: string) => {
    setPinnedIds(prev => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      savePinnedSessions(next);
      return next;
    });
  };

  const toggleSnooze = (id: string) => {
    setSnoozedMap(prev => {
      const next = { ...prev };
      if (next[id] && next[id] > Date.now()) {
        delete next[id];
      } else {
        next[id] = Date.now() + 24 * 60 * 60 * 1000; // 24h
      }
      saveSnoozedSessions(next);
      return next;
    });
  };

  // Progressive reveal: 5 rows per project group, +5 per "Show more" click.
  const [groupVisibleCounts, setGroupVisibleCounts] = useState<Record<string, number>>({});
  const visibleLimitFor = (key: string) => groupVisibleCounts[key] ?? 5;
  const showMore = (key: string) =>
    setGroupVisibleCounts(prev => ({ ...prev, [key]: visibleLimitFor(key) + 5 }));
  const [filterMenuOpen, setFilterMenuOpen] = useState(false);
  const filterMenuRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    const onPointerDown = (event: MouseEvent) => {
      if (filterMenuRef.current && !filterMenuRef.current.contains(event.target as Node)) {
        setFilterMenuOpen(false);
      }
    };
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setFilterMenuOpen(false);
      }
    };
    document.addEventListener('mousedown', onPointerDown);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('mousedown', onPointerDown);
      document.removeEventListener('keydown', onKey);
    };
  }, []);

  const matchesFilter = (sess: AgentSession) => {
    const id = sess.id;
    const isArchived = archivedSessionIds.has(id);
    const isSnoozed = (snoozedMap[id] || 0) > Date.now();
    const isPinned = pinnedIds.has(id);

    // Search query
    if (searchQuery.trim()) {
      const q = searchQuery.toLowerCase().trim();
      const title = sess.title || '';
      const path = sess.workspacePath || '';
      if (!title.toLowerCase().includes(q) && !path.toLowerCase().includes(q)) {
        return false;
      }
    }

    if (sidebarTab === 'pinned') return isPinned;
    if (sidebarTab === 'snoozed') return isSnoozed;
    if (sidebarTab === 'archived') return isArchived;
    // 'active' tab
    if (isArchived || isSnoozed) return false;

    if (taskFilter === 'running') return sess.status === 'running';
    if (taskFilter === 'finished') return sess.status !== 'running';
    return true;
  };

  // Unified task list: ForgeADE chat tasks and agent CLI terminal tasks are
  // both "tasks" — merged per project, newest activity first.
  type UnifiedEntry =
    | { kind: 'forge'; epoch: number; sess: AgentSession }
    | { kind: 'cli'; epoch: number; sess: TerminalSessionRecord };
  interface UnifiedGroup {
    projectName: string;
    folderPath: string;
    entries: UnifiedEntry[];
  }

  const projectGroups = useMemo<UnifiedGroup[]>(() => {
    const groups = new Map<string, UnifiedGroup>();
    const groupFor = (folderPath: string): UnifiedGroup => {
      const cleanPath = (folderPath || '').trim();
      const projectName = cleanPath && cleanPath !== 'general'
        ? (cleanPath.split('/').filter(Boolean).pop() || 'General')
        : 'General';
      let g = groups.get(projectName);
      if (!g) {
        g = { projectName, folderPath: cleanPath === 'general' ? '' : cleanPath, entries: [] };
        groups.set(projectName, g);
      } else if (!g.folderPath && cleanPath && cleanPath !== 'general') {
        g.folderPath = cleanPath;
      }
      return g;
    };

    // Always seed General workspace group so it is permanently visible and persistent
    groupFor('');

    // Seed active workspace(s)
    if (activeWorkspacePath) {
      groupFor(activeWorkspacePath);
    }
    for (const ws of activeWorkspaces) {
      if (ws.path) {
        groupFor(ws.path);
      }
    }

    // Dedupe by id — the same session lives both on disk (savedSessions) and
    // in memory (sessions); the runtime copy wins. Duplicate React keys made
    // rows render with each other's state (everything looked active).
    const forgeById = new Map<string, AgentSession>();
    for (const sess of savedSessions) {
      if (sess && sess.id) forgeById.set(sess.id, sess);
    }
    for (const sess of sessions) {
      if (sess && sess.id) forgeById.set(sess.id, sess);
    }
    for (const sess of forgeById.values()) {
      let folderPath = (sess.workspacePath || '').trim();
      if (folderPath === 'general') folderPath = '';
      groupFor(folderPath).entries.push({ kind: 'forge', epoch: getSessionEpoch(sess), sess });
    }
    const cliById = new Map<string, TerminalSessionRecord>();
    for (const s of terminalSessions) {
      if (s && s.id) cliById.set(s.id, s);
    }
    for (const s of cliById.values()) {
      let folderPath = (s.workspacePath || s.workingDirectory || '').trim();
      if (folderPath === 'general') folderPath = '';
      groupFor(folderPath).entries.push({ kind: 'cli', epoch: s.createdAt || 0, sess: s });
    }

    for (const g of groups.values()) {
      g.entries.sort((a, b) => {
        if (sidebarTab === 'active') {
          const aPin = pinnedIds.has(a.sess.id);
          const bPin = pinnedIds.has(b.sess.id);
          if (aPin && !bPin) return -1;
          if (!aPin && bPin) return 1;
        }
        return entryEpochFor(b, sortBy) - entryEpochFor(a, sortBy);
      });
    }
    // Groups float by their newest task under the active sort; General workspace stays persistent
    return Array.from(groups.values()).sort((a, b) => {
      const maxA = Math.max(...a.entries.map(e => entryEpochFor(e, sortBy)), 0);
      const maxB = Math.max(...b.entries.map(e => entryEpochFor(e, sortBy)), 0);
      if (maxA !== maxB) return maxB - maxA;
      if (a.projectName === 'General') return -1;
      if (b.projectName === 'General') return 1;
      return a.projectName.localeCompare(b.projectName);
    });
  }, [sessions, savedSessions, terminalSessions, activeWorkspacePath, activeWorkspaces, sortBy, sidebarTab, pinnedIds]);

  const allEntries = useMemo(() => {
    return projectGroups.flatMap(g => g.entries);
  }, [projectGroups]);

  const pinnedCount = useMemo(() => {
    return allEntries.filter(e => pinnedIds.has(e.sess.id)).length;
  }, [allEntries, pinnedIds]);

  const snoozedCount = useMemo(() => {
    return allEntries.filter(e => (snoozedMap[e.sess.id] || 0) > Date.now()).length;
  }, [allEntries, snoozedMap]);

  const archivedCount = useMemo(() => {
    return allEntries.filter(e => archivedSessionIds.has(e.sess.id)).length;
  }, [allEntries, archivedSessionIds]);

  const handleDeleteWorkspace = async (group: UnifiedGroup) => {
    if (group.projectName === 'General' || !group.folderPath) return;
    for (const entry of group.entries) {
      if (entry.kind === 'cli') {
        void deleteSession(entry.sess.id);
      } else {
        void deleteSessionPermanently(entry.sess.id);
      }
    }
    if (group.folderPath) {
      void closeWorkspaceFromWindow(group.folderPath);
      removeRecentWorkspace(group.folderPath);
    }
    if (activeWorkspacePath && activeWorkspacePath.split('/').filter(Boolean).pop() === group.projectName) {
      void closeWorkspaceFromWindow(activeWorkspacePath);
      removeRecentWorkspace(activeWorkspacePath);
    }
  };

  const entryMatchesFilter = (entry: UnifiedEntry): boolean => {
    if (entry.kind === 'cli') {
      const id = entry.sess.id;
      const isArchived = archivedSessionIds.has(id);
      const isSnoozed = (snoozedMap[id] || 0) > Date.now();
      const isPinned = pinnedIds.has(id);

      if (searchQuery.trim()) {
        const q = searchQuery.toLowerCase().trim();
        const title = entry.sess.title || '';
        const name = entry.sess.agentName || '';
        const path = entry.sess.workspacePath || '';
        if (!title.toLowerCase().includes(q) && !name.toLowerCase().includes(q) && !path.toLowerCase().includes(q)) {
          return false;
        }
      }

      if (sidebarTab === 'pinned') return isPinned;
      if (sidebarTab === 'snoozed') return isSnoozed;
      if (sidebarTab === 'archived') return isArchived;
      if (isArchived || isSnoozed) return false;

      if (taskFilter === 'running') return entry.sess.status === 'running' || entry.sess.status === 'starting';
      if (taskFilter === 'finished') return entry.sess.status !== 'running' && entry.sess.status !== 'starting';
      return true;
    }
    return matchesFilter(entry.sess);
  };

  const toggleGroup = (groupKey: string) => {
    setCollapsedGroups(prev => ({
      ...prev,
      [groupKey]: !prev[groupKey]
    }));
    // Collapse/expand re-collapses the reveal: back to the 5-row limit.
    setGroupVisibleCounts(prev => {
      const next = { ...prev };
      delete next[groupKey];
      return next;
    });
  };

  const toggleAllGroups = () => {
    const next = !allCollapsed;
    setAllCollapsed(next);
    const state: Record<string, boolean> = {};
    for (const group of projectGroups) state[group.projectName] = next;
    setCollapsedGroups(state);
  };

  const runningTerminalCount = terminalSessions.filter(
    s => s.status === 'running' || s.status === 'starting'
  ).length;

  const handleSelectTerminalSession = (s: TerminalSessionRecord) => {
    selectSession(s.id);
    setActiveTaskKind('cli');
    setAgentViewMode('session');
    if (mode === 'editor' || mode === 'settings') switchSurface('agent');
    const wsPath = s.workspacePath || s.workingDirectory;
    if (wsPath && wsPath !== 'general' && wsPath !== activeWorkspacePath) {
      setActiveWorkspacePath(wsPath);
    } else if (!wsPath || wsPath === 'general') {
      closeWorkspace();
    }
  };

  const handleSelectSession = (session: AgentSession) => {
    setActiveSessionId(session.id);
    setActiveTaskKind('forge');
    setAgentViewMode('session');
    if (mode === 'editor' || mode === 'settings') switchSurface('agent');
    if (session.workspacePath && session.workspacePath !== 'general' && session.workspacePath !== activeWorkspacePath) {
      setActiveWorkspacePath(session.workspacePath);
    } else if (!session.workspacePath || session.workspacePath === 'general') {
      closeWorkspace();
    }
  };

  const handleNewTask = () => {
    // New Task via sidebar creates a fresh session in Workspace General (home dir, no git branch)
    closeWorkspace();
    setActiveSessionId(null);
    setActiveTaskKind('forge');
    setAgentViewMode('session');
    if (mode === 'editor' || mode === 'settings') switchSurface('agent');
    else setMode('agent');
  };

  const sidebarActionClasses =
    'flex h-7.5 w-full shrink-0 items-center justify-start gap-2.5 rounded-[6px] px-2.5 text-left text-ui-sm font-medium text-foreground/85 transition-colors cursor-pointer hover:bg-foreground/5 hover:text-foreground';

  return (
    <aside
      data-testid="workspace-sidebar"
      style={{ width: `${leftSidebarWidth}px` }}
      className="bg-sidebar flex h-full shrink-0 flex-col overflow-hidden border-r border-sidebar-border text-sidebar-foreground select-none transition-colors"
    >
      {/* Window-controls strip: traffic lights live here (full-height sidebar,
          ZCode parity) + sidebar collapse + back/forward history. */}
      <div className="sidebar-window-strip flex shrink-0 items-center gap-0.5 pl-[76px] pr-2">
        <button
          type="button"
          onClick={() => setIsLeftSidebarOpen(false)}
          aria-label="Hide sidebar"
          title="Hide sidebar (⌘B)"
          className="flex size-7 items-center justify-center rounded-md text-foreground-subtle hover:bg-foreground/5 hover:text-foreground transition-colors cursor-pointer"
        >
          <PanelLeft className="size-4" />
        </button>
      </div>

      {leftSidebarView === 'files' ? (
        /* File explorer overrides the whole sidebar body (ZCode behavior) */
        <SidebarFileExplorer onBack={() => setLeftSidebarView('tasks')} />
      ) : (
      <>
      {/* Top actions: + New Task, Search, Automations, Plugins, Skills */}
      <div className="flex flex-col gap-0.5 px-2 pb-2 pt-1">
        {/* + New Task */}
        <button
          type="button"
          onClick={handleNewTask}
          className={cn(sidebarActionClasses, mode === 'agent' && activeTaskKind === 'forge' && !activeSessionId ? 'font-medium text-foreground' : '')}
        >
          <Plus className="size-4 shrink-0 text-foreground" />
          <span className="min-w-0 flex-1 truncate">New task</span>
          <span className="ml-auto shrink-0 text-ui-xs font-normal text-foreground-subtlest">⌘N</span>
        </button>

        {/* Search / Command Center */}
        <button
          type="button"
          onClick={() => setIsCommandPaletteOpen(true)}
          className={sidebarActionClasses}
          title="Open Command Center (⌘K)"
        >
          <Search className="size-4 shrink-0 text-foreground/75" />
          <span className="min-w-0 flex-1 truncate">Search</span>
          <span className="ml-auto shrink-0 text-ui-xs font-normal text-foreground-subtlest">⌘K</span>
        </button>

        {/* Automations (ZCode CalendarClock entry, right after Search) */}
        <button
          type="button"
          onClick={() => setIsAutomationsOpen(true)}
          aria-haspopup="dialog"
          className={cn(sidebarActionClasses, isAutomationsOpen ? 'bg-foreground/10 text-foreground' : '')}
        >
          <CalendarClock className="size-4 shrink-0 text-foreground/75" />
          <span className="min-w-0 flex-1 truncate">Automations</span>
        </button>

        {/* Workflow Board (Kanban & Tasks) */}
        <button
          type="button"
          onClick={() => setAgentViewMode('board')}
          className={cn(sidebarActionClasses, agentViewMode === 'board' ? 'bg-foreground/10 text-foreground font-semibold' : '')}
          title="Workflow Kanban Board & Tasks"
        >
          <Kanban className="size-4 shrink-0 text-foreground/75" />
          <span className="min-w-0 flex-1 truncate">Board</span>
        </button>

        {/* Git Client */}
        <button
          type="button"
          onClick={() => setAgentViewMode('git')}
          className={cn(sidebarActionClasses, agentViewMode === 'git' ? 'bg-foreground/10 text-foreground font-semibold' : '')}
          title="Git Client (SourceTree & Git Desktop UX)"
        >
          <FolderGit2 className="size-4 shrink-0 text-foreground/75" />
          <span className="min-w-0 flex-1 truncate">Git Client</span>
        </button>

      </div>

      {/* Scrollable task area */}
      <div className="relative flex min-h-0 flex-1 flex-col pt-2">
        {/* ZCode Group | Project segmented control + list controls */}
        <div className="flex items-center justify-between px-3 pb-2">
          <div className="relative" ref={filterMenuRef}>
            <button
              type="button"
              onClick={() => setFilterMenuOpen(prev => !prev)}
              aria-haspopup="menu"
              aria-expanded={filterMenuOpen}
              aria-label="View and sort options"
              title="View & sort"
              className={cn(
                'flex size-5 items-center justify-center rounded transition-colors cursor-pointer',
                filterMenuOpen ? 'bg-surface-hover text-foreground' : 'text-foreground-subtle hover:bg-foreground/5 hover:text-foreground'
              )}
            >
              <ListFilter className="size-3.5" />
            </button>
            {filterMenuOpen && (
              <div
                role="menu"
                aria-label="View and sort"
                className="absolute left-0 top-full z-50 mt-1.5 w-56 rounded-xl border border-border bg-popover p-1.5 shadow-lg"
              >
                <div className="px-2.5 pb-1 pt-0.5 text-ui-sm font-medium text-foreground-subtle">View</div>
                {([
                  { id: 'project', label: 'By project', Icon: Folder },
                  { id: 'timeline', label: 'Timeline', Icon: Clock }
                ] as const).map(opt => (
                  <button
                    key={opt.id}
                    type="button"
                    role="menuitemradio"
                    aria-checked={listView === opt.id}
                    onClick={() => { setListView(opt.id); setFilterMenuOpen(false); }}
                    className="flex w-full cursor-pointer items-center gap-2.5 rounded-lg px-2.5 py-1.5 text-left text-ui-sm text-foreground transition-colors hover:bg-foreground/5"
                  >
                    <opt.Icon className="size-4 shrink-0 text-foreground-subtle" />
                    <span className="min-w-0 flex-1">{opt.label}</span>
                    {listView === opt.id && <Check className="size-3.5 shrink-0 text-foreground" />}
                  </button>
                ))}
                <div className="mx-1 my-1 h-px bg-border" />
                <div className="px-2.5 pb-1 pt-0.5 text-ui-sm font-medium text-foreground-subtle">Sort by</div>
                {([
                  { id: 'updated', label: 'Updated', Icon: CheckCircle2 },
                  { id: 'created', label: 'Created', Icon: CirclePlus }
                ] as const).map(opt => (
                  <button
                    key={opt.id}
                    type="button"
                    role="menuitemradio"
                    aria-checked={sortBy === opt.id}
                    onClick={() => { setSortBy(opt.id); setFilterMenuOpen(false); }}
                    className="flex w-full cursor-pointer items-center gap-2.5 rounded-lg px-2.5 py-1.5 text-left text-ui-sm text-foreground transition-colors hover:bg-foreground/5"
                  >
                    <opt.Icon className="size-4 shrink-0 text-foreground-subtle" />
                    <span className="min-w-0 flex-1">{opt.label}</span>
                    {sortBy === opt.id && <Check className="size-3.5 shrink-0 text-foreground" />}
                  </button>
                ))}
              </div>
            )}
          </div>
          <div className="flex items-center gap-1">
            <button
              type="button"
              aria-label={showArchived ? "Hide archived sessions" : "Show archived sessions"}
              title={showArchived ? "Hide archived sessions" : "Show archived sessions"}
              className={cn(
                "flex size-5 items-center justify-center rounded transition-colors cursor-pointer",
                showArchived
                  ? "bg-surface-hover text-foreground"
                  : "text-foreground-subtle hover:bg-foreground/5 hover:text-foreground"
              )}
              onClick={() => setShowArchived(prev => !prev)}
            >
              <Archive className="size-3.5" />
            </button>
            <button
              type="button"
              aria-label={allCollapsed ? "Expand all groups" : "Collapse all groups"}
              title={allCollapsed ? "Expand all groups" : "Collapse all groups"}
              className="flex size-5 items-center justify-center rounded text-foreground-subtle hover:bg-foreground/5 hover:text-foreground transition-colors cursor-pointer"
              onClick={toggleAllGroups}
            >
              {allCollapsed ? <Maximize2 className="size-3.5" /> : <Minimize2 className="size-3.5" />}
            </button>
          </div>
        </div>

        {/* Search input bar */}
        <div className="px-2.5 pb-2">
          <div className="relative flex items-center">
            <Search className="absolute left-2 size-3.5 text-muted-foreground pointer-events-none" />
            <input
              type="text"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder="Search tasks..."
              className="w-full h-7 rounded-md bg-surface border border-border/60 pl-7 pr-6 text-ui-xs text-foreground placeholder:text-muted-foreground/60 focus:outline-hidden focus:border-primary/50"
            />
            {searchQuery && (
              <button
                type="button"
                onClick={() => setSearchQuery('')}
                className="absolute right-1.5 size-4 rounded flex items-center justify-center text-muted-foreground hover:text-foreground cursor-pointer"
              >
                <X className="size-3" />
              </button>
            )}
          </div>
        </div>

        {/* Filter segmented tabs: Active, Pinned, Snoozed, Archived */}
        <div className="flex items-center gap-1 px-2.5 pb-2 text-ui-xs overflow-x-auto scrollbar-none">
          {(['active', 'pinned', 'snoozed', 'archived'] as const).map(tab => {
            const isSelected = sidebarTab === tab;
            const count = tab === 'pinned' ? pinnedCount : tab === 'snoozed' ? snoozedCount : tab === 'archived' ? archivedCount : 0;
            return (
              <button
                key={tab}
                type="button"
                onClick={() => setSidebarTab(tab)}
                className={cn(
                  "px-2 py-0.5 rounded-full capitalize transition-colors cursor-pointer shrink-0",
                  isSelected
                    ? "bg-foreground/15 text-foreground font-medium"
                    : "text-foreground-subtle hover:bg-foreground/5 hover:text-foreground"
                )}
              >
                {tab} {count > 0 ? `(${count})` : ''}
              </button>
            );
          })}
        </div>

        {/* Section header (ZCode: "Projects") */}
        <div className="flex items-center justify-between px-3.5 pb-2 text-ui-xs font-semibold uppercase tracking-wider text-foreground-subtlest">
          <span>{listView === 'project' ? 'Projects' : 'Timeline'}</span>
          {runningTerminalCount > 0 && (
            <span className="flex items-center gap-1.5 text-ui-xs font-normal lowercase tracking-normal text-foreground-subtlest">
              <span className="size-1.5 animate-pulse rounded-full bg-primary" />
              {runningTerminalCount} running
            </span>
          )}
        </div>

        <div className="flex flex-1 min-h-0 flex-col gap-1 overflow-y-auto px-2">

          {/* Timeline: every task across projects, one chronological list. */}
        {listView === 'timeline' && (() => {
          const all = projectGroups
            .flatMap(g => g.entries)
            .filter(entryMatchesFilter)
            .sort((a, b) => {
              if (sidebarTab === 'active') {
                const aPin = pinnedIds.has(a.sess.id);
                const bPin = pinnedIds.has(b.sess.id);
                if (aPin && !bPin) return -1;
                if (!aPin && bPin) return 1;
              }
              return entryEpochFor(b, sortBy) - entryEpochFor(a, sortBy);
            });
          const limit = visibleLimitFor('timeline');
          const shown = all.slice(0, limit);
          const hiddenCount = all.length - shown.length;
          return (
            <div className="flex flex-col gap-0.5">
              {shown.map(entry => entry.kind === 'cli' ? (
                <TerminalSessionRow
                  key={`${entry.kind}-${entry.sess.id}`}
                  session={entry.sess}
                  activeSessionId={activeTaskKind === 'cli' ? activeTerminalSessionId : null}
                  isArchived={archivedSessionIds.has(entry.sess.id)}
                  isPinned={pinnedIds.has(entry.sess.id)}
                  isSnoozed={(snoozedMap[entry.sess.id] || 0) > Date.now()}
                  onToggleArchive={toggleArchiveSession}
                  onTogglePin={togglePin}
                  onToggleSnooze={toggleSnooze}
                  onSelect={handleSelectTerminalSession}
                  onStop={(id) => void stopSession(id)}
                  onRestart={(id) => void restartSession(id)}
                  onDelete={(id) => void deleteSession(id)}
                  searchQuery={searchQuery}
                />
              ) : (
                <TaskRow
                  key={`${entry.kind}-${entry.sess.id}`}
                  sess={entry.sess}
                  activeSessionId={activeTaskKind === 'forge' ? activeSessionId : null}
                  isArchived={archivedSessionIds.has(entry.sess.id)}
                  isPinned={pinnedIds.has(entry.sess.id)}
                  isSnoozed={(snoozedMap[entry.sess.id] || 0) > Date.now()}
                  onSelect={handleSelectSession}
                  onDelete={deleteSessionPermanently}
                  onToggleArchive={toggleArchiveSession}
                  onTogglePin={togglePin}
                  onToggleSnooze={toggleSnooze}
                  showProject
                  searchQuery={searchQuery}
                />
              ))}
              {all.length === 0 && (
                <p className="px-2.5 py-1 text-ui-xs text-foreground-subtlest">No tasks</p>
              )}
              {hiddenCount > 0 && (
                <button
                  type="button"
                  onClick={() => showMore('timeline')}
                  className="flex w-full cursor-pointer items-center rounded-[6px] px-2.5 py-1 text-left text-ui-xs text-foreground-subtlest transition-colors hover:bg-foreground/5 hover:text-foreground"
                >
                  Show more
                </button>
              )}
            </div>
          );
        })()}

        {/* By project: grouped per folder (ZCode parity) */}
          {listView === 'project' &&
            projectGroups.map(group => {
              const isCollapsed = collapsedGroups[group.projectName];
              const visibleEntries = group.entries.filter(entryMatchesFilter);
              const limit = visibleLimitFor(group.projectName);
              const shownEntries = visibleEntries.slice(0, limit);
              const hiddenCount = visibleEntries.length - shownEntries.length;

              return (
                <div key={group.projectName} className="flex flex-col gap-0.5">
                  {/* Project row + hover actions (Show files -> file explorer, circle-plus -> new task) */}
                  <div className="group flex items-center gap-0.5">
                    <button
                      type="button"
                      onClick={() => toggleGroup(group.projectName)}
                      className="flex h-7 min-w-0 flex-1 cursor-pointer items-center gap-1.5 rounded-[6px] px-2 text-left transition-colors hover:bg-foreground/5"
                    >
                      {isCollapsed ? (
                        <ChevronRight className="size-3 shrink-0 text-foreground-subtlest" />
                      ) : (
                        <ChevronDown className="size-3 shrink-0 text-foreground-subtlest" />
                      )}
                      <Folder className="size-3.5 shrink-0 text-foreground-subtle" />
                      <span className="min-w-0 flex-1 truncate text-ui-sm font-medium text-foreground/90">
                        {group.projectName}
                      </span>
                    </button>
                    <div className="flex shrink-0 items-center gap-0.5 pr-1 opacity-0 transition-opacity group-hover:opacity-100 group-focus-within:opacity-100">
                      {group.folderPath && (
                        <button
                          type="button"
                          title="Show files"
                          aria-label="Show files"
                          onClick={(e) => {
                            e.stopPropagation();
                            if (group.folderPath !== activeWorkspacePath) {
                              setActiveWorkspacePath(group.folderPath);
                            }
                            setLeftSidebarView('files');
                          }}
                          className="flex size-6 items-center justify-center rounded-md text-foreground-subtle transition-colors hover:bg-foreground/5 hover:text-foreground cursor-pointer"
                        >
                          <ListTodo className="size-3.5" />
                        </button>
                      )}
                      <button
                        type="button"
                        title="New task"
                        aria-label="New task"
                        onClick={(e) => {
                          e.stopPropagation();
                          if (group.projectName === 'General' || !group.folderPath) {
                            handleNewTask();
                          } else {
                            openNewTask(group.folderPath);
                          }
                        }}
                        className="flex size-6 items-center justify-center rounded-md text-foreground-subtle transition-colors hover:bg-foreground/5 hover:text-foreground cursor-pointer"
                      >
                        <CirclePlus className="size-3.5" />
                      </button>
                      {group.projectName !== 'General' && Boolean(group.folderPath) && (
                        <button
                          type="button"
                          title="Delete workspace"
                          aria-label="Delete workspace"
                          data-testid={`delete-workspace-${group.projectName}`}
                          onClick={(e) => {
                            e.stopPropagation();
                            void handleDeleteWorkspace(group);
                          }}
                          className="flex size-6 items-center justify-center rounded-md text-foreground-subtle transition-colors hover:bg-destructive/10 hover:text-destructive cursor-pointer"
                        >
                          <Trash2 className="size-3.5" />
                        </button>
                      )}
                    </div>
                  </div>

                  {/* Unified task rows: CLI terminals + ForgeADE chats, newest first */}
                  {!isCollapsed && (
                    <div className="flex flex-col gap-0.5 pl-2">
                      {shownEntries.map(entry => entry.kind === 'cli' ? (
                        <TerminalSessionRow
                          key={`${entry.kind}-${entry.sess.id}`}
                          session={entry.sess}
                          activeSessionId={activeTaskKind === 'cli' ? activeTerminalSessionId : null}
                          isArchived={archivedSessionIds.has(entry.sess.id)}
                          isPinned={pinnedIds.has(entry.sess.id)}
                          isSnoozed={(snoozedMap[entry.sess.id] || 0) > Date.now()}
                          onToggleArchive={toggleArchiveSession}
                          onTogglePin={togglePin}
                          onToggleSnooze={toggleSnooze}
                          onSelect={handleSelectTerminalSession}
                          onStop={(id) => void stopSession(id)}
                          onRestart={(id) => void restartSession(id)}
                          onDelete={(id) => void deleteSession(id)}
                          searchQuery={searchQuery}
                        />
                      ) : (
                        <TaskRow
                          key={`${entry.kind}-${entry.sess.id}`}
                          sess={entry.sess}
                          activeSessionId={activeTaskKind === 'forge' ? activeSessionId : null}
                          isArchived={archivedSessionIds.has(entry.sess.id)}
                          isPinned={pinnedIds.has(entry.sess.id)}
                          isSnoozed={(snoozedMap[entry.sess.id] || 0) > Date.now()}
                          onSelect={handleSelectSession}
                          onDelete={deleteSessionPermanently}
                          onToggleArchive={toggleArchiveSession}
                          onTogglePin={togglePin}
                          onToggleSnooze={toggleSnooze}
                          searchQuery={searchQuery}
                        />
                      ))}
                      {visibleEntries.length === 0 && (
                        <p className="px-2.5 py-1 text-ui-xs text-foreground-subtlest">No tasks</p>
                      )}
                      {hiddenCount > 0 && (
                        <button
                          type="button"
                          onClick={() => showMore(group.projectName)}
                          className="flex w-full cursor-pointer items-center rounded-[6px] px-2.5 py-1 text-left text-ui-xs text-foreground-subtlest transition-colors hover:bg-foreground/5 hover:text-foreground"
                        >
                          Show more
                        </button>
                      )}
                    </div>
                  )}
                </div>
              );
            })}
        </div>
      </div>
      </>
      )}

      {/* Profile footer (ZCode parity): avatar, name, theme, settings */}
      <ProfileRow
        theme={theme}
        onToggleTheme={toggleTheme}
        onOpenSettings={() => openSettingsTab('general')}
      />
    </aside>
  );
};

export default AgentSidebar;
