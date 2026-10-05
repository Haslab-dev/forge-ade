import React, { useState, useMemo, useRef, useEffect } from 'react';
import {
  Plus,
  Search,
  Blocks,
  CalendarClock,
  Sparkles,
  Folder,
  ChevronDown,
  ChevronRight,
  Trash2,
  Archive,
  Settings,
  Check,
  Maximize2,
  Minimize2,
  ListTodo,
  CirclePlus,
  RotateCcw,
  Square
} from 'lucide-react';
import { cn, formatDisplayTitle } from '../../lib/utils';
import { useWorkspace } from '../../stores/workspaceStore';
import { useSessionStore, sessionStatusLabel } from '../../stores/sessionStore';
import { formatRelativeTime, formatSessionTime, getSessionEpoch } from '../../lib/time';
import { AgentSession } from '../../types';
import type { TerminalSessionRecord } from '../../types';
import { ProviderIcon } from './ProviderIcon';
import { SidebarFileExplorer } from './SidebarFileExplorer';

/**
 * Workspace sidebar ported 1:1 from ZCode WorkspaceSidebar:
 * macOS window drag strip, top actions (+ New Task, Search, Automations, Plugins, Skills),
 * Tasks section with archive/collapse controls, project trees, and user profile footer.
 *
 * Mode-aware: the experience (Terminal Session — the default — vs the native
 * Agent UI) is chosen by Settings → Agent → Default Mode and applied at
 * launch. The sidebar simply reflects it: terminal sessions or agent tasks.
 */

interface AgentSidebarProps {
  onCollapse?: () => void;
}

type TaskFilter = 'all' | 'running' | 'finished';

/** Task row: 16px indicator slot with active/running dot, title, time + hover archive/delete. */
const TaskRow: React.FC<{
  sess: AgentSession;
  activeSessionId: string | null;
  isArchived: boolean;
  onSelect: (sess: AgentSession) => void;
  onDelete: (id: string) => void;
  onToggleArchive: (id: string) => void;
  showProject?: boolean;
}> = ({ sess, activeSessionId, isArchived, onSelect, onDelete, onToggleArchive, showProject }) => {
  const isActive = sess.id === activeSessionId;
  const isRunning = sess.status === 'running';
  const projectLeaf = sess.workspacePath
    ? sess.workspacePath.split('/').filter(Boolean).pop()
    : undefined;

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
          ? 'bg-[#232323] text-foreground font-medium'
          : 'text-foreground/80 hover:bg-[#1a1a1a] hover:text-foreground',
        isArchived && !isActive ? 'opacity-65' : ''
      )}
    >
      {/* Dynamic agent icon */}
      <div className="relative flex size-3.5 shrink-0 items-center justify-center">
        <ProviderIcon
          provider={sess.agentId || sess.agentType || sess.provider || 'internal'}
          size={13}
          className={cn(
            'shrink-0 transition-opacity',
            isActive ? 'opacity-100' : 'opacity-70 group-hover/task-item:opacity-100'
          )}
        />
        {isRunning && (
          <span className="absolute -top-0.5 -right-0.5 size-1.5 animate-pulse rounded-full bg-[#38bdf8]">
            <span className="sr-only">Running</span>
          </span>
        )}
      </div>
      <span
        className={cn(
          'min-w-0 flex-1 truncate text-ui-sm',
          isActive ? 'font-medium text-foreground' : 'font-normal text-foreground/85'
        )}
        title={formatDisplayTitle(sess.title) || 'New Session'}
      >
        {formatDisplayTitle(sess.title) || 'New Session'}
      </span>
      <div className="flex shrink-0 items-center gap-1">
        {showProject && projectLeaf ? (
          <span className="whitespace-nowrap text-ui-xs text-foreground-subtlest group-hover/task-item:hidden">
            {projectLeaf}
          </span>
        ) : null}
        <span className="whitespace-nowrap text-ui-xs text-foreground-subtlest group-hover/task-item:hidden">
          {formatSessionTime(sess)}
        </span>
        <button
          type="button"
          onClick={(e) => {
            e.stopPropagation();
            onToggleArchive(sess.id);
          }}
          className="hidden size-5 items-center justify-center rounded p-0.5 text-foreground-subtle hover:text-foreground hover:bg-white/5 transition-colors group-hover/task-item:flex"
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
          className="hidden size-5 items-center justify-center rounded p-0.5 text-foreground-subtle hover:text-destructive hover:bg-white/5 transition-colors group-hover/task-item:flex"
          title="Delete task"
        >
          <Trash2 className="size-3.5" />
        </button>
      </div>
    </div>
  );
};

/** Status dot color per terminal session lifecycle. */
const TERMINAL_DOT_CLASSES: Record<TerminalSessionRecord['status'], string> = {
  starting: 'bg-[#38bdf8] animate-pulse',
  running: 'bg-[#38bdf8] animate-pulse',
  exited: 'bg-foreground-subtlest/60',
  failed: 'bg-destructive',
  terminated: 'bg-warning/70'
};

/** Terminal session row — same layout language as TaskRow: status dot, agent
    name, task title, compact status, hover stop/restart/delete actions. */
const TerminalSessionRow: React.FC<{
  session: TerminalSessionRecord;
  activeSessionId: string | null;
  onSelect: (s: TerminalSessionRecord) => void;
  onStop: (id: string) => void;
  onRestart: (id: string) => void;
  onDelete: (id: string) => void;
}> = ({ session, activeSessionId, onSelect, onStop, onRestart, onDelete }) => {
  const isActive = session.id === activeSessionId;
  const isRunning = session.status === 'running' || session.status === 'starting';

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
          ? 'bg-[#232323] text-foreground font-medium'
          : 'text-foreground/80 hover:bg-[#1a1a1a] hover:text-foreground'
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
          <span className="absolute -top-0.5 -right-0.5 size-1.5 animate-pulse rounded-full bg-[#38bdf8]">
            <span className="sr-only">Running</span>
          </span>
        )}
      </div>

      {/* Session title */}
      <span
        className={cn(
          'min-w-0 flex-1 truncate text-ui-sm',
          isActive ? 'font-medium text-foreground' : 'font-normal text-foreground/85'
        )}
        title={formatDisplayTitle(session.title) || session.agentName}
      >
        {formatDisplayTitle(session.title) || session.agentName || 'Terminal Session'}
      </span>

      <div className="flex shrink-0 items-center gap-1">
        <span className="whitespace-nowrap text-ui-xs text-foreground-subtlest group-hover/session-item:hidden">
          {sessionStatusLabel(session)}
        </span>
        {isRunning ? (
          <button
            type="button"
            onClick={(e) => {
              e.stopPropagation();
              onStop(session.id);
            }}
            className="hidden size-5 items-center justify-center rounded p-0.5 text-foreground-subtle hover:text-foreground hover:bg-white/5 transition-colors group-hover/session-item:flex"
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
            className="hidden size-5 items-center justify-center rounded p-0.5 text-foreground-subtle hover:text-foreground hover:bg-white/5 transition-colors group-hover/session-item:flex"
            title="Restart session"
          >
            <RotateCcw className="size-3" />
          </button>
        )}
        <button
          type="button"
          onClick={(e) => {
            e.stopPropagation();
            onDelete(session.id);
          }}
          className="hidden size-5 items-center justify-center rounded p-0.5 text-foreground-subtle hover:text-destructive hover:bg-white/5 transition-colors group-hover/session-item:flex"
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
    activeWorkspacePath,
    setActiveWorkspacePath,
    deleteSessionPermanently,
    setIsCommandPaletteOpen,
    openSettingsTab,
    setMode,
    mode,
    theme,
    setTheme,
    isLeftSidebarOpen,
    leftSidebarWidth,
    archivedSessionIds,
    toggleArchiveSession,
    leftSidebarView,
    setLeftSidebarView
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

  // The segmented control maps 1:1 to the workspace mode: Terminal Session
  // (default experience) vs the native Agent UI.
  const isTerminalMode = mode === 'terminal';

  const [collapsedGroups, setCollapsedGroups] = useState<Record<string, boolean>>({});
  // ZCode sidebar "Group | Project" view: group = all projects grouped,
  // project = flat list of the active project's tasks only.
  const [taskView, setTaskView] = useState<'group' | 'project'>('group');
  const [allCollapsed, setAllCollapsed] = useState(false);
  const [showArchived, setShowArchived] = useState(false);
  const [taskFilter, setTaskFilter] = useState<TaskFilter>('all');
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

  // Merge runtime sessions and disk sessions
  const allSessions = useMemo(() => {
    const map = new Map<string, AgentSession>();
    for (const s of savedSessions) {
      if (s && s.id) map.set(s.id, s);
    }
    for (const s of sessions) {
      if (s && s.id) map.set(s.id, s);
    }
    return Array.from(map.values()).sort((a, b) => {
      if (a.status === 'running' && b.status !== 'running') return -1;
      if (b.status === 'running' && a.status !== 'running') return 1;
      const at = getSessionEpoch(a);
      const bt = getSessionEpoch(b);
      if (at !== bt) return bt - at;
      return (b.id || '').localeCompare(a.id || '');
    });
  }, [sessions, savedSessions]);

  const matchesFilter = (sess: AgentSession) => {
    const isArchived = archivedSessionIds.has(sess.id);
    if (!showArchived && isArchived) return false;
    if (taskFilter === 'running') return sess.status === 'running';
    if (taskFilter === 'finished') return sess.status !== 'running';
    return true;
  };

  // Group sessions per project (folder name)
  const projectGroups = useMemo(() => {
    const groups: Record<string, { projectName: string; folderPath: string; sessions: AgentSession[] }> = {};

    for (const sess of allSessions) {
      const folderPath = sess.workspacePath || activeWorkspacePath || '';
      const projectName = folderPath.split('/').filter(Boolean).pop() || 'General';

      if (!groups[projectName]) {
        groups[projectName] = {
          projectName,
          folderPath,
          sessions: []
        };
      }
      groups[projectName].sessions.push(sess);
    }

    const desiredOrder = ['forge-ade', 'MyAiRouter', 'kendali-ai'];
    const sortedKeys = Object.keys(groups).sort((a, b) => {
      const idxA = desiredOrder.indexOf(a);
      const idxB = desiredOrder.indexOf(b);
      if (idxA !== -1 && idxB !== -1) return idxA - idxB;
      if (idxA !== -1) return -1;
      if (idxB !== -1) return 1;
      return a.localeCompare(b);
    });

    return sortedKeys.map(k => groups[k]);
  }, [allSessions, activeWorkspacePath]);

  const toggleGroup = (groupKey: string) => {
    setCollapsedGroups(prev => ({
      ...prev,
      [groupKey]: !prev[groupKey]
    }));
  };

  const toggleAllGroups = () => {
    const next = !allCollapsed;
    setAllCollapsed(next);
    const state: Record<string, boolean> = {};
    const groups = isTerminalMode ? terminalGroups : projectGroups;
    for (const group of groups) state[group.projectName] = next;
    setCollapsedGroups(state);
  };

  // Terminal sessions grouped by project (same grouping pattern as tasks).
  const terminalGroups = useMemo(() => {
    const groups: Record<string, { projectName: string; folderPath: string; sessions: TerminalSessionRecord[] }> = {};
    for (const s of terminalSessions) {
      const folderPath = s.workspacePath || s.workingDirectory || activeWorkspacePath || '';
      const projectName = folderPath.split('/').filter(Boolean).pop() || 'General';
      if (!groups[projectName]) {
        groups[projectName] = { projectName, folderPath, sessions: [] };
      }
      groups[projectName].sessions.push(s);
    }
    return Object.values(groups).sort((a, b) => {
      if (a.folderPath === activeWorkspacePath) return -1;
      if (b.folderPath === activeWorkspacePath) return 1;
      return a.projectName.localeCompare(b.projectName);
    });
  }, [terminalSessions, activeWorkspacePath]);

  const runningTerminalCount = terminalSessions.filter(
    s => s.status === 'running' || s.status === 'starting'
  ).length;

  const handleSelectTerminalSession = (s: TerminalSessionRecord) => {
    selectSession(s.id);
    if (s.workspacePath && s.workspacePath !== activeWorkspacePath) {
      setActiveWorkspacePath(s.workspacePath);
    }
    if (mode !== 'terminal') setMode('terminal');
  };

  const handleSelectSession = (session: AgentSession) => {
    setActiveSessionId(session.id);
    if (session.workspacePath && session.workspacePath !== activeWorkspacePath) {
      setActiveWorkspacePath(session.workspacePath);
    }
    setMode('agent');
  };

  const handleNewTask = () => {
    if (isTerminalMode) {
      openNewTask();
      return;
    }
    setActiveSessionId(null);
    setMode('agent');
  };

  const sidebarActionClasses =
    'flex h-7.5 w-full shrink-0 items-center justify-start gap-2.5 rounded-[6px] px-2.5 text-left text-ui-sm font-medium text-foreground/85 transition-colors cursor-pointer hover:bg-surface-hover hover:text-foreground';

  return (
    <aside
      data-testid="workspace-sidebar"
      style={{ width: `${leftSidebarWidth}px` }}
      className="flex h-full shrink-0 flex-col overflow-hidden border-r border-border bg-sidebar text-foreground select-none transition-colors"
    >
      {/* Sidebar drag strip below the main header — window-drag band.
          The sidebar collapse/expand toggle lives in the main header. */}
      <div className="titlebar-drag h-9 w-full shrink-0" />

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
          className={cn(sidebarActionClasses, !isTerminalMode && mode === 'agent' && !activeSessionId ? 'font-medium text-foreground' : '')}
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
          onClick={() => setMode(mode === 'automations' ? 'terminal' : 'automations')}
          aria-pressed={mode === 'automations'}
          className={cn(
            sidebarActionClasses,
            mode === 'automations' ? 'bg-[#232323] text-foreground' : ''
          )}
        >
          <CalendarClock className="size-4 shrink-0 text-foreground/75" />
          <span className="min-w-0 flex-1 truncate">Automations</span>
        </button>

        {/* Plugins (personal, managed in Settings → Plugins) — ZCode position */}
        <button
          type="button"
          onClick={() => openSettingsTab('plugins')}
          className={sidebarActionClasses}
        >
          <Blocks className="size-4 shrink-0 text-foreground/75" />
          <span className="min-w-0 flex-1 truncate">Plugins</span>
        </button>

        <button
          type="button"
          onClick={() => openSettingsTab('skills')}
          className={sidebarActionClasses}
        >
          <Sparkles className="size-4 shrink-0 text-foreground/75" />
          <span className="min-w-0 flex-1 truncate">Skills</span>
        </button>

      </div>

      {/* Scrollable task area */}
      <div className="relative flex min-h-0 flex-1 flex-col pt-2">
        {/* ZCode Group | Project segmented control + list controls */}
        <div className="flex items-center justify-between px-3 pb-2">
          <div className="flex items-center rounded-lg bg-[#161616] p-0.5">
            <button
              type="button"
              aria-pressed={taskView === 'group'}
              onClick={() => setTaskView('group')}
              className={cn(
                'flex h-6 items-center gap-1.5 rounded-md px-2 text-ui-xs transition-colors cursor-pointer',
                taskView === 'group' ? 'bg-[#2a2a2a] font-medium text-foreground' : 'text-foreground-subtle hover:text-foreground'
              )}
            >
              <span className="font-mono text-ui-xs">#</span>
              Group
            </button>
            <button
              type="button"
              aria-pressed={taskView === 'project'}
              onClick={() => setTaskView('project')}
              className={cn(
                'flex h-6 items-center gap-1.5 rounded-md px-2 text-ui-xs transition-colors cursor-pointer',
                taskView === 'project' ? 'bg-[#2a2a2a] font-medium text-foreground' : 'text-foreground-subtle hover:text-foreground'
              )}
            >
              <Folder className="size-3" />
              Project
            </button>
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
                  : "text-foreground-subtle hover:bg-surface-hover hover:text-foreground"
              )}
              onClick={() => setShowArchived(prev => !prev)}
            >
              <Archive className="size-3.5" />
            </button>
            <button
              type="button"
              aria-label={allCollapsed ? "Expand all groups" : "Collapse all groups"}
              title={allCollapsed ? "Expand all groups" : "Collapse all groups"}
              className="flex size-5 items-center justify-center rounded text-foreground-subtle hover:bg-surface-hover hover:text-foreground transition-colors cursor-pointer"
              onClick={toggleAllGroups}
            >
              {allCollapsed ? <Maximize2 className="size-3.5" /> : <Minimize2 className="size-3.5" />}
            </button>
          </div>
        </div>

        {/* Section header (ZCode: "Projects") */}
        <div className="flex items-center justify-between px-3.5 pb-2 text-ui-xs font-semibold uppercase tracking-wider text-foreground-subtlest">
          <span>{taskView === 'group' ? 'Projects' : activeWorkspacePath ? (activeWorkspacePath.split('/').filter(Boolean).pop() || 'Project') : 'Project'}</span>
          {isTerminalMode && runningTerminalCount > 0 && (
            <span className="flex items-center gap-1.5 text-ui-xs font-normal lowercase tracking-normal text-foreground-subtlest">
              <span className="size-1.5 animate-pulse rounded-full bg-[#38bdf8]" />
              {runningTerminalCount} running
            </span>
          )}
        </div>

        <div className="flex flex-1 min-h-0 flex-col gap-1 overflow-y-auto px-2">
          {/* ZCode "Project" view: flat list of the active project's tasks or sessions */}
          {taskView === 'project' && (() => {
            if (isTerminalMode) {
              const flat = (terminalGroups.find(g => g.folderPath === activeWorkspacePath) || terminalGroups[0])?.sessions || [];
              return (
                <div className="flex flex-col gap-0.5">
                  {flat.map(s => (
                    <TerminalSessionRow
                      key={s.id}
                      session={s}
                      activeSessionId={activeTerminalSessionId}
                      onSelect={handleSelectTerminalSession}
                      onStop={(id) => void stopSession(id)}
                      onRestart={(id) => void restartSession(id)}
                      onDelete={(id) => void deleteSession(id)}
                    />
                  ))}
                  {flat.length === 0 && (
                    <p className="px-2.5 py-1 text-ui-xs text-foreground-subtlest">No sessions</p>
                  )}
                </div>
              );
            }
            const flat = (projectGroups.find(g => g.folderPath === activeWorkspacePath) || projectGroups[0])?.sessions.filter(matchesFilter) || [];
            return (
              <div className="flex flex-col gap-0.5">
                {flat.map(sess => (
                  <TaskRow
                    key={sess.id}
                    sess={sess}
                    activeSessionId={activeSessionId}
                    isArchived={archivedSessionIds.has(sess.id)}
                    onSelect={handleSelectSession}
                    onDelete={deleteSessionPermanently}
                    onToggleArchive={toggleArchiveSession}
                    showProject
                  />
                ))}
                {flat.length === 0 && (
                  <p className="px-2.5 py-1 text-ui-xs text-foreground-subtlest">No tasks</p>
                )}
              </div>
            );
          })()}

          {/* Grouped by Project (ZCode "Group" view) */}
          {taskView === 'group' &&
            (isTerminalMode ? (
              terminalGroups.map(group => {
                const isCollapsed = collapsedGroups[group.projectName];
                return (
                  <div key={group.projectName} className="flex flex-col gap-0.5">
                    {/* Project row + hover actions (Show files -> file explorer, circle-plus -> new task) */}
                    <div className="group flex items-center gap-0.5">
                      <button
                        type="button"
                        onClick={() => toggleGroup(group.projectName)}
                        className="flex h-7 min-w-0 flex-1 cursor-pointer items-center gap-1.5 rounded-[6px] px-2 text-left transition-colors hover:bg-surface-hover"
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
                      <div className="flex shrink-0 items-center gap-0.5 pr-1 opacity-0 transition-opacity group-hover:opacity-100">
                        <button
                          type="button"
                          title="Show files"
                          aria-label="Show files"
                          onClick={(e) => {
                            e.stopPropagation();
                            if (group.folderPath && group.folderPath !== activeWorkspacePath) {
                              setActiveWorkspacePath(group.folderPath);
                            }
                            setLeftSidebarView('files');
                          }}
                          className="flex size-6 items-center justify-center rounded-md text-foreground-subtle transition-colors hover:bg-surface-hover hover:text-foreground cursor-pointer"
                        >
                          <ListTodo className="size-3.5" />
                        </button>
                        <button
                          type="button"
                          title="New task"
                          aria-label="New task"
                          onClick={(e) => {
                            e.stopPropagation();
                            openNewTask(group.folderPath);
                          }}
                          className="flex size-6 items-center justify-center rounded-md text-foreground-subtle transition-colors hover:bg-surface-hover hover:text-foreground cursor-pointer"
                        >
                          <CirclePlus className="size-3.5" />
                        </button>
                      </div>
                    </div>

                    {!isCollapsed && (
                      <div className="flex flex-col gap-0.5 pl-2">
                        {group.sessions.map(s => (
                          <TerminalSessionRow
                            key={s.id}
                            session={s}
                            activeSessionId={activeTerminalSessionId}
                            onSelect={handleSelectTerminalSession}
                            onStop={(id) => void stopSession(id)}
                            onRestart={(id) => void restartSession(id)}
                            onDelete={(id) => void deleteSession(id)}
                          />
                        ))}
                        {group.sessions.length === 0 && (
                          <p className="px-2.5 py-1 text-ui-xs text-foreground-subtlest">No sessions</p>
                        )}
                      </div>
                    )}
                  </div>
                );
              })
            ) : (
              projectGroups.map(group => {
                const isCollapsed = collapsedGroups[group.projectName];
                const visibleSessions = group.sessions.filter(matchesFilter);

                return (
                  <div key={group.projectName} className="flex flex-col gap-0.5">
                    {/* Project row + hover actions (Show files -> file explorer, circle-plus -> new task) */}
                    <div className="group flex items-center gap-0.5">
                      <button
                        type="button"
                        onClick={() => toggleGroup(group.projectName)}
                        className="flex h-7 min-w-0 flex-1 cursor-pointer items-center gap-1.5 rounded-[6px] px-2 text-left transition-colors hover:bg-surface-hover"
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
                      <div className="flex shrink-0 items-center gap-0.5 pr-1 opacity-0 transition-opacity group-hover:opacity-100">
                        <button
                          type="button"
                          title="Show files"
                          aria-label="Show files"
                          onClick={(e) => {
                            e.stopPropagation();
                            if (group.folderPath && group.folderPath !== activeWorkspacePath) {
                              setActiveWorkspacePath(group.folderPath);
                            }
                            setLeftSidebarView('files');
                          }}
                          className="flex size-6 items-center justify-center rounded-md text-foreground-subtle transition-colors hover:bg-surface-hover hover:text-foreground cursor-pointer"
                        >
                          <ListTodo className="size-3.5" />
                        </button>
                        <button
                          type="button"
                          title="New task"
                          aria-label="New task"
                          onClick={(e) => {
                            e.stopPropagation();
                            if (group.folderPath && group.folderPath !== activeWorkspacePath) {
                              setActiveWorkspacePath(group.folderPath);
                            }
                            setLeftSidebarView('tasks');
                            setActiveSessionId(null);
                            setMode('agent');
                          }}
                          className="flex size-6 items-center justify-center rounded-md text-foreground-subtle transition-colors hover:bg-surface-hover hover:text-foreground cursor-pointer"
                        >
                          <CirclePlus className="size-3.5" />
                        </button>
                      </div>
                    </div>

                    {/* Task rows */}
                    {!isCollapsed && (
                      <div className="flex flex-col gap-0.5 pl-2">
                        {visibleSessions.map(sess => (
                          <TaskRow
                            key={sess.id}
                            sess={sess}
                            activeSessionId={activeSessionId}
                            isArchived={archivedSessionIds.has(sess.id)}
                            onSelect={handleSelectSession}
                            onDelete={deleteSessionPermanently}
                            onToggleArchive={toggleArchiveSession}
                          />
                        ))}
                        {visibleSessions.length === 0 && (
                          <p className="px-2.5 py-1 text-ui-xs text-foreground-subtlest">No tasks</p>
                        )}
                      </div>
                    )}
                  </div>
                );
              })
            ))}
        </div>
      </div>
      </>
      )}

      {/* Footer: settings entry only (profile removed) */}
      <footer className="flex shrink-0 items-center justify-end border-t border-border/40 px-3 py-2.5">
        <button
          type="button"
          data-testid="task-settings-button"
          aria-label="Settings"
          title="Settings"
          onClick={() => openSettingsTab('general')}
          className="flex size-7 shrink-0 items-center justify-center rounded-md text-foreground-subtle hover:bg-surface-hover hover:text-foreground transition-colors cursor-pointer"
        >
          <Settings className="size-4" />
        </button>
      </footer>
    </aside>
  );
};

export default AgentSidebar;
