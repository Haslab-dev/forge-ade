import React, { useState, useMemo } from 'react';
import { 
  MessageSquarePlus, 
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
    <aside className="w-[340px] shrink-0 h-full flex flex-col gap-[4px] p-[20px_16px_16px_20px] bg-[#FFFFFF] dark:bg-[#161617] border-r border-[#E5E7EB] dark:border-[#333336] text-[#111827] dark:text-[#F2F2F2] select-none font-[Inter,system-ui,sans-serif] overflow-hidden transition-colors">
      
      {/* Action: New Task */}
      <button
        type="button"
        onClick={handleNewTask}
        className={`w-full flex flex-row items-center gap-[12px] p-[9px_12px] rounded-[8px] transition-colors cursor-pointer text-left ${
          activeSessionId === null
            ? 'bg-[#EAEBED] dark:bg-[#2A2A2D] text-[#111827] dark:text-[#F2F2F2] font-medium'
            : 'text-[#6B7280] dark:text-[#9B9B9F] hover:bg-[#F3F4F6] dark:hover:bg-[#1E1E20] hover:text-[#111827] dark:hover:text-[#F2F2F2]'
        }`}
      >
        <MessageSquarePlus className="w-[17px] h-[17px] shrink-0 text-[#6B7280] dark:text-[#9B9B9F]" />
        <span className="text-[15px] flex-1">New task</span>
        <span className="text-[13px] text-[#9CA3AF] dark:text-[#6B6B70] font-normal whitespace-nowrap">⌘ N</span>
      </button>

      {/* Action: Search */}
      <button
        type="button"
        onClick={() => setIsCommandPaletteOpen(true)}
        className="w-full flex flex-row items-center gap-[12px] p-[9px_12px] rounded-[8px] text-[#6B7280] dark:text-[#9B9B9F] hover:bg-[#F3F4F6] dark:hover:bg-[#1E1E20] hover:text-[#111827] dark:hover:text-[#F2F2F2] transition-colors cursor-pointer text-left"
      >
        <Search className="w-[17px] h-[17px] shrink-0 text-[#6B7280] dark:text-[#9B9B9F]" />
        <span className="text-[15px] font-normal flex-1">Search</span>
        <span className="text-[13px] text-[#9CA3AF] dark:text-[#6B6B70] font-normal whitespace-nowrap">⌘ K</span>
      </button>

      {/* Action: Automations */}
      <button
        type="button"
        onClick={() => openSettingsTab('commands')}
        className="w-full flex flex-row items-center gap-[12px] p-[9px_12px] rounded-[8px] text-[#6B7280] dark:text-[#9B9B9F] hover:bg-[#F3F4F6] dark:hover:bg-[#1E1E20] hover:text-[#111827] dark:hover:text-[#F2F2F2] transition-colors cursor-pointer text-left"
      >
        <CalendarClock className="w-[17px] h-[17px] shrink-0 text-[#6B7280] dark:text-[#9B9B9F]" />
        <span className="text-[15px] font-normal flex-1">Automations</span>
      </button>

      {/* Action: Plugin Marketplace */}
      <button
        type="button"
        onClick={() => openSettingsTab('plugins')}
        className="w-full flex flex-row items-center gap-[12px] p-[9px_12px] rounded-[8px] text-[#6B7280] dark:text-[#9B9B9F] hover:bg-[#F3F4F6] dark:hover:bg-[#1E1E20] hover:text-[#111827] dark:hover:text-[#F2F2F2] transition-colors cursor-pointer text-left"
      >
        <LayoutGrid className="w-[17px] h-[17px] shrink-0 text-[#6B7280] dark:text-[#9B9B9F]" />
        <span className="text-[15px] font-normal flex-1">Plugin Marketplace</span>
      </button>

      {/* Projects Label */}
      <div className="w-full flex flex-row gap-0 p-[20px_12px_8px_12px] text-[13px] text-[#9CA3AF] dark:text-[#6B6B70] font-normal">
        Projects
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
                className="w-full flex flex-row items-center gap-[12px] p-[9px_12px] rounded-[8px] text-left hover:bg-[#F3F4F6] dark:hover:bg-[#1E1E20] transition-colors cursor-pointer group"
              >
                <Folder className="w-[16px] h-[16px] shrink-0 text-[#6B7280] dark:text-[#9B9B9F]" />
                <span className="text-[15px] font-normal flex-1 text-[#111827] dark:text-[#F2F2F2] truncate">
                  {group.projectName}
                </span>
                <div className="flex items-center gap-1">
                  <span className="text-[13px] text-[#9CA3AF] dark:text-[#6B6B70] font-mono">
                    {group.sessions.length > 0 ? (group.sessions[0].updatedAt || '1d') : ''}
                  </span>
                  {isCollapsed ? (
                    <ChevronRight className="w-3.5 h-3.5 text-[#9CA3AF] dark:text-[#6B6B70]" />
                  ) : (
                    <ChevronDown className="w-3.5 h-3.5 text-[#9CA3AF] dark:text-[#6B6B70]" />
                  )}
                </div>
              </button>

              {/* Sessions List under this project */}
              {!isCollapsed && (
                <div className="flex flex-col gap-0.5 pl-3 ml-3 border-l border-[#E5E7EB] dark:border-[#333336]">
                  {group.sessions.map(sess => {
                    const isActive = sess.id === activeSessionId;
                    const isRunning = sess.status === 'running';

                    return (
                      <div
                        key={sess.id}
                        onClick={() => handleSelectSession(sess)}
                        className={`group w-full flex flex-row items-center gap-[10px] p-[8px_10px] rounded-[8px] transition-colors cursor-pointer text-left ${
                          isActive
                            ? 'bg-[#EAEBED] dark:bg-[#2A2A2D] text-[#111827] dark:text-[#F2F2F2] font-medium'
                            : 'text-[#4B5563] dark:text-[#9B9B9F] hover:bg-[#F3F4F6] dark:hover:bg-[#1E1E20] hover:text-[#111827] dark:hover:text-[#F2F2F2]'
                        }`}
                      >
                        {isRunning ? (
                          <div className="w-2 h-2 rounded-full bg-[#16A34A] dark:bg-[#4ADE80] animate-pulse shrink-0" />
                        ) : null}
                        <span className="text-[14px] flex-1 truncate">
                          {sess.title || 'New Session'}
                        </span>
                        <div className="flex items-center gap-1.5 shrink-0">
                          <span className={`text-[12px] whitespace-nowrap group-hover:hidden ${
                            isActive ? 'text-[#16A34A] dark:text-[#4ADE80]' : 'text-[#9CA3AF] dark:text-[#6B6B70]'
                          }`}>
                            {isRunning ? 'now' : (sess.updatedAt || 'now')}
                          </span>
                          <button
                            type="button"
                            onClick={(e) => {
                              e.stopPropagation();
                              deleteSessionPermanently(sess.id);
                            }}
                            className="hidden group-hover:block p-1 text-[#9CA3AF] dark:text-[#6B6B70] hover:text-[#DC2626] dark:hover:text-[#EF4444] transition-colors"
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
      <div className="pt-2 border-t border-[#E5E7EB] dark:border-[#333336]">
        <button
          type="button"
          onClick={() => openSettingsTab('model')}
          className="w-full flex items-center gap-3 p-[9px_12px] rounded-[8px] text-[#6B7280] dark:text-[#9B9B9F] hover:bg-[#F3F4F6] dark:hover:bg-[#1E1E20] hover:text-[#111827] dark:hover:text-[#F2F2F2] transition-colors cursor-pointer text-left"
        >
          <Settings className="w-[17px] h-[17px] shrink-0 text-[#6B7280] dark:text-[#9B9B9F]" />
          <span className="text-[15px]">Settings</span>
        </button>
      </div>

    </aside>
  );
};

export default AgentSidebar;
