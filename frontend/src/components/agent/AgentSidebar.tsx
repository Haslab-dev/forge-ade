import React, { useState, useMemo } from 'react';
import { 
  MessageSquarePlus, 
  Hash,
  Maximize2,
  ListFilter,
  GripVertical,
  Plus,
  Search, 
  CalendarClock, 
  LayoutGrid, 
  Folder, 
  ChevronDown, 
  ChevronRight, 
  Trash2,
  Settings
} from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';
import { formatRelativeTime } from '../../lib/time';
import { AgentSession } from '../../types';

interface AgentSidebarProps {
  onCollapse?: () => void;
}

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
    setMode
  } = useWorkspace();

  const [collapsedFolders, setCollapsedFolders] = useState<Record<string, boolean>>({});
  const [taskViewMode, setTaskViewMode] = useState<'group' | 'project'>('project');

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

  const toggleFolder = (projectName: string) => {
    setCollapsedFolders(prev => ({
      ...prev,
      [projectName]: !prev[projectName]
    }));
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

  return (
    <aside className="w-[264px] shrink-0 h-full flex flex-col gap-[4px] p-[12px_8px_12px_8px] bg-sidebar border-r border-white/10 dark:border-white/10 text-foreground select-none overflow-hidden transition-colors">
      
      {/* Action: New Task */}
      <button
        type="button"
        onClick={handleNewTask}
        className={`w-full flex flex-row items-center gap-[12px] p-[9px_12px] rounded-[8px] transition-colors cursor-pointer text-left ${
          activeSessionId === null
            ? 'bg-white/10 dark:bg-white/10 text-foreground font-medium'
            : 'text-foreground-subtle hover:bg-white/5 dark:hover:bg-white/5 hover:text-foreground'
        }`}
      >
        <MessageSquarePlus className="w-[16px] h-[16px] shrink-0 text-foreground-subtle" />
        <span className="text-[14px] flex-1">New task</span>
        <span className="text-[12px] text-foreground-subtlest font-normal whitespace-nowrap">⌘ N</span>
      </button>

      {/* Action: Search */}
      <button
        type="button"
        onClick={() => setIsCommandPaletteOpen(true)}
        className="w-full flex flex-row items-center gap-[12px] p-[9px_12px] rounded-[8px] text-foreground-subtle hover:bg-white/5 dark:hover:bg-white/5 hover:text-foreground transition-colors cursor-pointer text-left"
      >
        <Search className="w-[16px] h-[16px] shrink-0 text-foreground-subtle" />
        <span className="text-[14px] font-normal flex-1">Search</span>
        <span className="text-[12px] text-foreground-subtlest font-normal whitespace-nowrap">⌘ K</span>
      </button>

      {/* Action: Automations */}
      <button
        type="button"
        onClick={() => openSettingsTab('commands')}
        className="w-full flex flex-row items-center gap-[12px] p-[9px_12px] rounded-[8px] text-foreground-subtle hover:bg-white/5 dark:hover:bg-white/5 hover:text-foreground transition-colors cursor-pointer text-left"
      >
        <CalendarClock className="w-[16px] h-[16px] shrink-0 text-foreground-subtle" />
        <span className="text-[14px] font-normal flex-1">Automations</span>
      </button>

      {/* Action: Plugin Marketplace */}
      <button
        type="button"
        onClick={() => setMode('marketplace')}
        className="w-full flex flex-row items-center gap-[12px] p-[9px_12px] rounded-[8px] text-foreground-subtle hover:bg-white/5 dark:hover:bg-white/5 hover:text-foreground transition-colors cursor-pointer text-left"
      >
        <LayoutGrid className="w-[16px] h-[16px] shrink-0 text-foreground-subtle" />
        <span className="text-[14px] font-normal flex-1">Plugin Marketplace</span>
      </button>

      {/* Group | Project view toggle + view actions */}
      <div className="w-full flex items-center justify-between px-2 pt-4 pb-1">
        <div className="flex items-center gap-1">
          <button
            type="button"
            onClick={() => setTaskViewMode('group')}
            className={`flex items-center gap-1.5 rounded-full px-2.5 py-[5px] text-[13px] transition-colors cursor-pointer ${
              taskViewMode === 'group'
                ? 'bg-white/10 dark:bg-white/10 text-foreground'
                : 'text-foreground-subtle hover:text-foreground'
            }`}
          >
            <Hash className="w-3.5 h-3.5" />
            Group
          </button>
          <button
            type="button"
            onClick={() => setTaskViewMode('project')}
            className={`flex items-center gap-1.5 rounded-full px-2.5 py-[5px] text-[13px] transition-colors cursor-pointer ${
              taskViewMode === 'project'
                ? 'bg-white/10 dark:bg-white/10 text-foreground'
                : 'text-foreground-subtle hover:text-foreground'
            }`}
          >
            <Folder className="w-3.5 h-3.5" />
            Project
          </button>
        </div>
        <div className="flex items-center gap-0.5 text-foreground-subtlest">
          <button type="button" className="p-1 rounded hover:bg-white/5 dark:hover:bg-white/5 hover:text-foreground transition-colors cursor-pointer" title="Expand all">
            <Maximize2 className="w-3.5 h-3.5" />
          </button>
          <button type="button" className="p-1 rounded hover:bg-white/5 dark:hover:bg-white/5 hover:text-foreground transition-colors cursor-pointer" title="Filter">
            <ListFilter className="w-3.5 h-3.5" />
          </button>
          <button type="button" className="p-1 rounded hover:bg-white/5 dark:hover:bg-white/5 hover:text-foreground transition-colors cursor-pointer" title="Clear finished">
            <Trash2 className="w-3.5 h-3.5" />
          </button>
        </div>
      </div>

      {/* Projects Label */}
      <div className="w-full flex items-center justify-between p-[8px_12px_4px_12px]">
        <div className="flex items-center gap-1 text-[12px] text-foreground-subtle font-normal">
          Projects
          <ChevronDown className="w-3 h-3 text-foreground-subtlest" />
        </div>
        <div className="flex items-center gap-0.5 text-foreground-subtlest">
          <button type="button" className="p-1 rounded hover:bg-white/5 dark:hover:bg-white/5 hover:text-foreground transition-colors cursor-pointer" title="Reorder">
            <GripVertical className="w-3.5 h-3.5" />
          </button>
          <button type="button" className="p-1 rounded hover:bg-white/5 dark:hover:bg-white/5 hover:text-foreground transition-colors cursor-pointer" title="Add project">
            <Plus className="w-3.5 h-3.5" />
          </button>
        </div>
      </div>

      {/* Projects & Sessions List */}
      <div className="flex-1 overflow-y-auto flex flex-col gap-1 pr-1">
        {projectGroups.map(group => {
          const isCollapsed = collapsedFolders[group.projectName];

          return (
            <div key={group.projectName} className="flex flex-col gap-0.5">
              {/* Folder Row */}
              <button
                type="button"
                onClick={() => toggleFolder(group.projectName)}
                className="w-full flex flex-row items-center gap-[12px] p-[9px_12px] rounded-[8px] text-left hover:bg-white/5 dark:hover:bg-white/5 transition-colors cursor-pointer group"
              >
                <Folder className="w-[15px] h-[15px] shrink-0 text-foreground-subtle" />
                <span className="text-[14px] font-normal flex-1 text-foreground truncate">
                  {group.projectName}
                </span>
                <div className="flex items-center gap-1">
                  <span className="text-[13px] text-foreground-subtlest font-mono">
                    {group.sessions.length > 0 ? formatRelativeTime(group.sessions[0].updatedAt) : ''}
                  </span>
                  {isCollapsed ? (
                    <ChevronRight className="w-3.5 h-3.5 text-foreground-subtlest" />
                  ) : (
                    <ChevronDown className="w-3.5 h-3.5 text-foreground-subtlest" />
                  )}
                </div>
              </button>

              {/* Sessions List under this project */}
              {!isCollapsed && (
                <div className="flex flex-col gap-0.5 pl-6">
                  {group.sessions.map(sess => {
                    const isActive = sess.id === activeSessionId;
                    const isRunning = sess.status === 'running';

                    return (
                      <div
                        key={sess.id}
                        onClick={() => handleSelectSession(sess)}
                        className={`group h-8 w-full flex flex-row items-center gap-[10px] px-2.5 rounded-lg transition-colors cursor-pointer text-left ${
                          isActive
                            ? 'bg-white/10 dark:bg-white/10 text-foreground font-medium'
                            : 'text-foreground-subtle hover:bg-white/5 dark:hover:bg-white/5 hover:text-foreground'
                        }`}
                      >
                        {isRunning ? (
                          <div className="w-1.5 h-1.5 rounded-full bg-success animate-pulse shrink-0" />
                        ) : null}
                        <span className="text-[14px] flex-1 truncate">
                          {sess.title || 'New Session'}
                        </span>
                        <div className="flex items-center gap-1.5 shrink-0">
                          <span className={`text-[12px] whitespace-nowrap group-hover:hidden ${
                            isActive ? 'text-success' : 'text-foreground-subtlest'
                          }`}>
                            {isRunning ? 'now' : formatRelativeTime(sess.updatedAt)}
                          </span>
                          <button
                            type="button"
                            onClick={(e) => {
                              e.stopPropagation();
                              deleteSessionPermanently(sess.id);
                            }}
                            className="hidden group-hover:block p-1 text-foreground-subtlest hover:text-destructive dark:hover:text-destructive transition-colors"
                            title="Delete task session"
                          >
                            <Trash2 className="w-3.5 h-3.5" />
                          </button>
                        </div>
                      </div>
                    );
                  })}
                </div>
              )}
            </div>
          );
        })}
      </div>

      {/* Bottom Settings Link */}
      <div className="pt-2 border-t border-white/10 dark:border-white/10">
        <button
          type="button"
          onClick={() => openSettingsTab('model')}
          className="w-full flex items-center gap-3 p-[9px_12px] rounded-[8px] text-foreground-subtle hover:bg-white/5 dark:hover:bg-white/5 hover:text-foreground transition-colors cursor-pointer text-left"
        >
          <Settings className="w-[16px] h-[16px] shrink-0 text-foreground-subtle" />
          <span className="text-[14px]">Settings</span>
        </button>
      </div>

    </aside>
  );
};

export default AgentSidebar;
