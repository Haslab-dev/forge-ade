import React, { useState, useMemo, useRef, useEffect } from 'react';
import {
  MessageCirclePlus,
  Hash,
  Maximize2,
  Minimize2,
  ListFilter,
  Search,
  CalendarClock,
  Blocks,
  Folder,
  ChevronDown,
  ChevronRight,
  Trash2,
  Settings,
  PanelLeftClose,
  Check,
  Palette,
  Monitor,
  Moon,
  Sun,
  LogOut
} from 'lucide-react';
import { cn } from '../../lib/utils';
import { useWorkspace } from '../../stores/workspaceStore';
import { formatRelativeTime } from '../../lib/time';
import { AgentSession } from '../../types';

/**
 * Workspace sidebar ported 1:1 from ZCode WorkspaceSidebar:
 * h-12 drag strip, top actions (New task / Search / Automations / Plugin
 * Marketplace), organize pill-tabs toolbar, grouped task rows with status
 * dots and hover actions, and the profile + settings footer.
 */

interface AgentSidebarProps {
  onCollapse?: () => void;
}

type TaskFilter = 'all' | 'running' | 'finished';

/** Task row (ZCode TaskListItem): 16px indicator slot, title, time + hover delete. */
const TaskRow: React.FC<{
  sess: AgentSession;
  activeSessionId: string | null;
  onSelect: (sess: AgentSession) => void;
  onDelete: (id: string) => void;
  showProject?: boolean;
}> = ({ sess, activeSessionId, onSelect, onDelete, showProject }) => {
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
        'group/task-item flex cursor-pointer items-center gap-2 rounded-lg py-1 pl-2.5 pr-1 transition-[background-color,border-color,box-shadow]',
        isActive ? 'bg-selected' : 'hover:bg-surface-hover'
      )}
    >
      {/* Leading 16px indicator slot */}
      <div className="relative flex size-4 shrink-0 items-center justify-center">
        {isRunning ? (
          <span
            className="size-1.5 animate-pulse rounded-full bg-sky-500 dark:bg-sky-400"
            data-unread-indicator="true"
          />
        ) : null}
      </div>
      <span
        className={cn(
          'min-w-0 flex-1 truncate text-ui-base',
          isActive ? 'font-medium text-foreground' : 'text-foreground'
        )}
      >
        {sess.title || 'New Session'}
      </span>
      <div className="flex shrink-0 items-center gap-0.5">
        {showProject && projectLeaf ? (
          <span className="whitespace-nowrap text-ui-xs text-foreground-subtlest group-hover/task-item:hidden">
            {projectLeaf}
          </span>
        ) : null}
        <span className="whitespace-nowrap text-ui-xs text-foreground-subtlest group-hover/task-item:hidden">
          {isRunning ? 'now' : formatRelativeTime(sess.updatedAt)}
        </span>
        <button
          type="button"
          onClick={(e) => {
            e.stopPropagation();
            if (confirm(`Delete task "${sess.title || 'New Session'}"?`)) {
              onDelete(sess.id);
            }
          }}
          className="hidden size-6 items-center justify-center rounded-md p-1 text-foreground-subtlest transition-colors hover:text-destructive group-hover/task-item:flex"
          title="Delete task"
        >
          <Trash2 className="size-3.5" />
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
    setIsLeftSidebarOpen
  } = useWorkspace();

  const [collapsedFolders, setCollapsedFolders] = useState<Record<string, boolean>>({});
  const [allCollapsed, setAllCollapsed] = useState(false);
  const [taskViewMode, setTaskViewMode] = useState<'group' | 'project'>('project');
  const [taskFilter, setTaskFilter] = useState<TaskFilter>('all');
  const [filterMenuOpen, setFilterMenuOpen] = useState(false);
  const [profileMenuOpen, setProfileMenuOpen] = useState(false);
  const filterMenuRef = useRef<HTMLDivElement | null>(null);
  const profileMenuRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    const onPointerDown = (event: MouseEvent) => {
      if (filterMenuRef.current && !filterMenuRef.current.contains(event.target as Node)) {
        setFilterMenuOpen(false);
      }
      if (profileMenuRef.current && !profileMenuRef.current.contains(event.target as Node)) {
        setProfileMenuOpen(false);
      }
    };
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') {
        setFilterMenuOpen(false);
        setProfileMenuOpen(false);
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
      const at = a.updatedAt ? Date.parse(a.updatedAt) || 0 : 0;
      const bt = b.updatedAt ? Date.parse(b.updatedAt) || 0 : 0;
      if (at !== bt) return bt - at;
      return (b.id || '').localeCompare(a.id || '');
    });
  }, [sessions, savedSessions]);

  // Group sessions per project (folder name)
  const projectGroups = useMemo(() => {
    const groups: Record<string, { projectName: string; folderPath: string; sessions: AgentSession[] }> = {};

    for (const sess of allSessions) {
      const folderPath = sess.workspacePath || activeWorkspacePath || '/Users/lutfiikbalmajid/hasdev/forge-ade';
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

  // Chronological (Grouped) view: flat filtered list across all projects.

  const matchesFilter = (sess: AgentSession) => {
    if (taskFilter === 'running') return sess.status === 'running';
    if (taskFilter === 'finished') return sess.status !== 'running';
    return true;
  };

  // Chronological (Grouped) view: flat filtered list across all projects.
  const flatSessions = useMemo(() => allSessions.filter(sess => matchesFilter(sess)), [allSessions, taskFilter]);

  const toggleFolder = (projectName: string) => {
    setCollapsedFolders(prev => ({
      ...prev,
      [projectName]: !prev[projectName]
    }));
  };

  const toggleAllGroups = () => {
    const next = !allCollapsed;
    setAllCollapsed(next);
    const state: Record<string, boolean> = {};
    for (const group of projectGroups) state[group.projectName] = next;
    setCollapsedFolders(state);
  };

  const handleSelectSession = (session: AgentSession) => {
    setActiveSessionId(session.id);
    if (session.workspacePath && session.workspacePath !== activeWorkspacePath) {
      setActiveWorkspacePath(session.workspacePath);
    }
    setMode('agent');
  };

  const handleNewTask = () => {
    setActiveSessionId(null);
    setMode('agent');
  };

  const sidebarActionClasses =
    'flex h-8 w-full shrink-0 items-center justify-start gap-2 rounded-lg px-2.5 text-left text-ui-base text-foreground transition-colors cursor-pointer hover:bg-surface-hover hover:text-foreground';

  return (
    <aside
      data-testid="workspace-sidebar"
      className="flex h-full w-[264px] shrink-0 flex-col overflow-hidden border-r border-border bg-sidebar text-foreground select-none transition-colors"
    >
      {/* Drag strip (hosts macOS traffic lights) */}
      <div className="titlebar-drag h-12 w-full shrink-0" />

      {/* Top actions */}
      <div className="flex flex-col gap-1 px-2 py-3">
        {/* New task (NewTaskButtonGroup) */}
        <div
          role="group"
          data-testid="task-new-button"
          onClick={handleNewTask}
          className="group flex h-8 w-full shrink-0 cursor-pointer items-center justify-stretch gap-2 overflow-hidden rounded-lg pl-2.5 pr-2.5 transition-colors hover:bg-surface-hover hover:text-foreground active:translate-y-0"
        >
          <div className="flex min-w-0 flex-1 items-center gap-2 text-ui-base">
            <MessageCirclePlus className="size-4 shrink-0" />
            <span className="truncate">New task</span>
            <span className="ml-auto shrink-0 text-ui-xs font-normal text-foreground-subtlest">⌘ N</span>
          </div>
        </div>

        {/* Search / Command center */}
        <button
          type="button"
          onClick={() => setIsCommandPaletteOpen(true)}
          className={sidebarActionClasses}
        >
          <Search className="size-4 shrink-0" />
          <span className="min-w-0 flex-1 truncate text-left">Search</span>
          <span className="ml-auto shrink-0 text-ui-xs font-normal text-foreground-subtlest">⌘ K</span>
        </button>

        {/* Automations */}
        <button
          type="button"
          data-testid="automations-open"
          aria-pressed={mode === 'settings'}
          onClick={() => openSettingsTab('commands')}
          className={cn(sidebarActionClasses, mode === 'settings' && 'bg-selected text-foreground')}
        >
          <CalendarClock className="size-4 shrink-0" />
          <span className="min-w-0 flex-1 truncate text-left">Automations</span>
        </button>

        {/* Plugin Marketplace */}
        <button
          type="button"
          data-testid="plugin-store-sidebar-open"
          aria-pressed={mode === 'marketplace'}
          onClick={() => setMode('marketplace')}
          className={cn(
            sidebarActionClasses,
            mode === 'marketplace' && 'bg-selected text-foreground'
          )}
        >
          <Blocks className="size-4 shrink-0" />
          <span className="min-w-0 flex-1 truncate text-left">Plugin Marketplace</span>
        </button>
      </div>

      {/* Scrollable task area */}
      <div className="relative flex min-h-0 flex-1 flex-col">
        <div className="flex flex-1 min-h-0 flex-col gap-3 overflow-y-auto">
          {/* Organize toolbar */}
          <div className="pl-2 pr-2">
            <div className="flex min-w-0 items-center justify-between gap-2">
              <div className="flex min-w-0 shrink-0 items-center gap-1">
                {/* Pill tabs: Grouped | Project */}
                <div
                  role="tablist"
                  aria-label="Organize tasks"
                  className="relative flex h-7 w-fit shrink-0 overflow-hidden rounded-full bg-surface p-0.5"
                >
                  <button
                    role="tab"
                    aria-selected={taskViewMode === 'group'}
                    onClick={() => setTaskViewMode('group')}
                    className={cn(
                      'z-10 flex h-6 flex-none items-center gap-1 rounded-full py-0 pl-1.5 pr-2 text-ui-sm font-medium transition-colors',
                      taskViewMode === 'group'
                        ? 'bg-background text-foreground shadow-xs'
                        : 'text-foreground-subtle hover:text-foreground'
                    )}
                  >
                    <Hash aria-hidden="true" className="size-3 shrink-0" />
                    <span>Group</span>
                  </button>
                  <button
                    role="tab"
                    aria-selected={taskViewMode === 'project'}
                    onClick={() => setTaskViewMode('project')}
                    className={cn(
                      'z-10 flex h-6 flex-none items-center gap-1 rounded-full py-0 pl-1.5 pr-2 text-ui-sm font-medium transition-colors',
                      taskViewMode === 'project'
                        ? 'bg-background text-foreground shadow-xs'
                        : 'text-foreground-subtle hover:text-foreground'
                    )}
                  >
                    <Folder aria-hidden="true" className="size-3 shrink-0" />
                    <span>Project</span>
                  </button>
                </div>
                <button
                  type="button"
                  className="flex size-7 shrink-0 items-center justify-center rounded-lg text-foreground-subtle transition-colors hover:bg-surface-hover hover:text-foreground"
                  aria-label={allCollapsed ? 'Expand all' : 'Collapse all'}
                  title={allCollapsed ? 'Expand all' : 'Collapse all'}
                  onClick={toggleAllGroups}
                >
                  {allCollapsed ? <Maximize2 className="size-3.5" /> : <Minimize2 className="size-3.5" />}
                </button>
              </div>
              <div className="flex shrink-0 items-center gap-0.5">
                <button
                  type="button"
                  aria-label="Clear finished tasks"
                  title="Clear finished tasks"
                  className="flex size-7 items-center justify-center rounded-lg text-foreground-subtle transition-colors hover:bg-surface-hover hover:text-foreground"
                  onClick={() => {
                    const finished = allSessions.filter(s => s.status !== 'running');
                    if (finished.length === 0) return;
                    if (confirm(`Clear ${finished.length} finished task(s)?`)) {
                      finished.forEach(s => deleteSessionPermanently(s.id));
                    }
                  }}
                >
                  <Trash2 className="size-3.5" />
                </button>
                <div className="relative" ref={filterMenuRef}>
                  <button
                    type="button"
                    aria-label="Task view options"
                    title="Task view options"
                    className={cn(
                      'flex size-7 items-center justify-center rounded-lg text-foreground-subtle transition-colors hover:bg-surface-hover hover:text-foreground',
                      taskFilter !== 'all' && 'bg-surface-hover text-foreground'
                    )}
                    onClick={() => setFilterMenuOpen(v => !v)}
                  >
                    <ListFilter className="size-3.5" />
                  </button>
                  {filterMenuOpen && (
                    <div className="absolute right-0 top-full z-50 mt-1 w-40 rounded-xl border border-border bg-popover p-1 shadow-lg">
                      {([
                        { id: 'all', label: 'All tasks' },
                        { id: 'running', label: 'Running' },
                        { id: 'finished', label: 'Finished' }
                      ] as Array<{ id: TaskFilter; label: string }>).map(opt => (
                        <button
                          key={opt.id}
                          type="button"
                          className="flex w-full items-center justify-between gap-2 rounded-lg px-2 py-1.5 text-left text-ui-sm text-foreground transition-colors hover:bg-surface-hover"
                          onClick={() => {
                            setTaskFilter(opt.id);
                            setFilterMenuOpen(false);
                          }}
                        >
                          {opt.label}
                          {taskFilter === opt.id && <Check className="size-3.5 text-foreground" />}
                        </button>
                      ))}
                    </div>
                  )}
                </div>
              </div>
            </div>
          </div>

          {/* Projects & Sessions List */}
          {taskViewMode === 'group' ? (
            /* Grouped (chronological): flat task list across all projects */
            <div className="flex min-h-0 flex-col gap-0.5 px-2">
              {flatSessions.map(sess => (
                <TaskRow key={sess.id} sess={sess} activeSessionId={activeSessionId} onSelect={handleSelectSession} onDelete={deleteSessionPermanently} showProject />
              ))}
              {flatSessions.length === 0 && (
                <p className="px-2.5 py-1 text-ui-sm text-foreground-subtlest">No tasks</p>
              )}
            </div>
          ) : (
            <div className="flex min-h-0 flex-col gap-3 px-2">
            {projectGroups.map(group => {
              const isCollapsed = collapsedFolders[group.projectName];
              const visibleSessions = group.sessions.filter(matchesFilter);

              return (
                <div key={group.projectName} className="flex flex-col gap-0.5">
                  {/* Project row */}
                  <button
                    type="button"
                    onClick={() => toggleFolder(group.projectName)}
                    className="group flex h-8 w-full cursor-pointer items-center gap-2 rounded-lg pl-2.5 pr-1 text-left transition-[background-color,border-color,box-shadow] hover:bg-surface-hover"
                  >
                    <Folder className="size-4 shrink-0 text-foreground-subtle" />
                    <span className="min-w-0 flex-1 truncate text-ui-base text-foreground">
                      {group.projectName}
                    </span>
                    <span className="shrink-0 text-ui-xs text-foreground-subtlest">
                      {group.sessions.length > 0 ? formatRelativeTime(group.sessions[0].updatedAt) : ''}
                    </span>
                    {isCollapsed ? (
                      <ChevronRight className="size-3.5 shrink-0 text-foreground-subtlest" />
                    ) : (
                      <ChevronDown className="size-3.5 shrink-0 text-foreground-subtlest" />
                    )}
                  </button>

                  {/* Task rows */}
                  {!isCollapsed && (
                    <div className="flex flex-col gap-0.5 pl-4">
                      {visibleSessions.map(sess => (
                        <TaskRow key={sess.id} sess={sess} activeSessionId={activeSessionId} onSelect={handleSelectSession} onDelete={deleteSessionPermanently} />
                      ))}
                      {visibleSessions.length === 0 && (
                        <p className="px-2.5 py-1 text-ui-sm text-foreground-subtlest">No tasks</p>
                      )}
                    </div>
                  )}
                </div>
              );
            })}
            </div>
          )}
        </div>
      </div>

      {/* Footer: profile + settings */}
      <footer className="flex shrink-0 flex-col gap-2.5 px-4 pb-4 pt-2">
        <div className="flex min-w-0 gap-2">
          <div className="relative min-w-0 flex-1" ref={profileMenuRef}>
            <button
              type="button"
              aria-label="Account"
              onClick={() => setProfileMenuOpen(v => !v)}
              className="flex h-8 min-w-0 w-full items-center gap-2 overflow-hidden rounded-lg px-1 text-left transition-colors hover:bg-surface-hover"
            >
              <span className="flex size-7 shrink-0 items-center justify-center rounded-full bg-gradient-to-b from-[#3a3a3c] to-[#1c1c1e] text-ui-xs font-semibold text-white">
                LI
              </span>
              <span className="min-w-0 flex-1 truncate text-left text-ui-base font-semibold text-foreground">
                Lutfi Ikbal
              </span>
              <ChevronDown className="size-3.5 shrink-0 text-foreground-subtlest" />
            </button>
            {profileMenuOpen && (
              <div className="absolute bottom-full left-0 z-50 mb-2 w-52 rounded-xl border border-border bg-popover p-1 shadow-lg">
                <div className="px-2 pb-1 pt-1.5 text-ui-xs font-medium text-foreground-subtlest">Theme</div>
                {[
                  { id: 'dark', label: 'Dark', icon: Moon },
                  { id: 'light', label: 'Light', icon: Sun },
                  { id: 'system', label: 'System', icon: Monitor }
                ].map(opt => (
                  <button
                    key={opt.id}
                    type="button"
                    className="flex w-full items-center gap-2 rounded-lg px-2 py-1.5 text-left text-ui-sm text-foreground transition-colors hover:bg-surface-hover"
                    onClick={() => {
                      setTheme(opt.id as any);
                      setProfileMenuOpen(false);
                    }}
                  >
                    <opt.icon className="size-4 text-foreground-subtle" />
                    <span className="flex-1">{opt.label}</span>
                    {(theme || 'dark') === opt.id && <Check className="size-3.5 text-foreground" />}
                  </button>
                ))}
                <div className="mx-1 my-1 h-px bg-border" />
                <button
                  type="button"
                  className="flex w-full items-center gap-2 rounded-lg px-2 py-1.5 text-left text-ui-sm text-foreground transition-colors hover:bg-surface-hover"
                  onClick={() => {
                    setProfileMenuOpen(false);
                    openSettingsTab('general');
                  }}
                >
                  <Palette className="size-4 text-foreground-subtle" />
                  Settings
                </button>
                <button
                  type="button"
                  className="flex w-full items-center gap-2 rounded-lg px-2 py-1.5 text-left text-ui-sm text-foreground transition-colors hover:bg-surface-hover"
                  onClick={() => setProfileMenuOpen(false)}
                >
                  <LogOut className="size-4 text-foreground-subtle" />
                  Log out
                </button>
              </div>
            )}
          </div>
          <div className="flex shrink-0 items-center gap-1.5">
            <button
              type="button"
              aria-label="Hide sidebar"
              title="Hide sidebar (⌘B)"
              onClick={() => setIsLeftSidebarOpen(false)}
              className="flex size-8 shrink-0 items-center justify-center rounded-lg text-foreground-subtle transition-colors hover:bg-surface-hover hover:text-foreground"
            >
              <PanelLeftClose className="size-4" />
            </button>
            <button
              type="button"
              data-testid="task-settings-button"
              aria-label="Settings"
              title="Settings"
              onClick={() => openSettingsTab('general')}
              className="flex size-8 shrink-0 items-center justify-center rounded-lg text-foreground-subtle transition-colors hover:bg-surface-hover hover:text-foreground"
            >
              <Settings className="size-4" />
            </button>
          </div>
        </div>
      </footer>
    </aside>
  );
};

export default AgentSidebar;
