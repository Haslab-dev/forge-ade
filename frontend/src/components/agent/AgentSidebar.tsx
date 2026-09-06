import React, { useState, useMemo } from 'react';
import { 
  Plus, 
  Search, 
  Folder, 
  ChevronDown, 
  ChevronRight, 
  Trash2, 
  PanelLeft, 
  ArrowLeft, 
  ArrowRight,
  FolderKanban,
  Hash
} from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';
import { AgentSession } from '../../types';

interface AgentSidebarProps {
  onCollapse?: () => void;
}

export const AgentSidebar: React.FC<AgentSidebarProps> = ({ onCollapse }) => {
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
    goBack,
    goForward,
    canGoBack,
    canGoForward
  } = useWorkspace();

  const [activeTab, setActiveTab] = useState<'project' | 'group'>('project');
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
    <aside className="w-[260px] min-w-[260px] h-full bg-white dark:bg-[#181819] text-[#333333] dark:text-[#cccccc] flex flex-col border-r border-[#e5e7eb] dark:border-[#262627] select-none font-sans text-xs transition-colors duration-150">
      
      {/* Top Bar: macOS Traffic Lights + Nav Actions */}
      <div className="h-[42px] min-h-[42px] px-3.5 flex items-center justify-between border-b border-[#e5e7eb] dark:border-[#232324]">
        {/* macOS Traffic Lights */}
        <div className="flex items-center gap-2">
          <div className="w-3 h-3 rounded-full bg-[#ff5f56] hover:opacity-80 cursor-pointer shadow-xs" title="Close" />
          <div className="w-3 h-3 rounded-full bg-[#ffbd2e] hover:opacity-80 cursor-pointer shadow-xs" title="Minimize" />
          <div className="w-3 h-3 rounded-full bg-[#27c93f] hover:opacity-80 cursor-pointer shadow-xs" title="Maximize" />
        </div>

        {/* Sidebar Toggle & Back/Forward Navigation */}
        <div className="flex items-center gap-1 text-[#6b7280] dark:text-[#888888]">
          <button
            type="button"
            onClick={onCollapse}
            className="p-1 rounded hover:bg-[#f3f4f6] dark:hover:bg-[#28282a] hover:text-[#111827] dark:hover:text-white transition-colors cursor-pointer"
            title="Toggle Sidebar"
          >
            <PanelLeft className="w-3.5 h-3.5" />
          </button>
          <button
            type="button"
            onClick={goBack}
            disabled={!canGoBack}
            className={`p-1 rounded transition-colors ${canGoBack ? 'hover:bg-[#f3f4f6] dark:hover:bg-[#28282a] hover:text-[#111827] dark:hover:text-white cursor-pointer' : 'opacity-30 cursor-default'}`}
            title="Back"
          >
            <ArrowLeft className="w-3.5 h-3.5" />
          </button>
          <button
            type="button"
            onClick={goForward}
            disabled={!canGoForward}
            className={`p-1 rounded transition-colors ${canGoForward ? 'hover:bg-[#f3f4f6] dark:hover:bg-[#28282a] hover:text-[#111827] dark:hover:text-white cursor-pointer' : 'opacity-30 cursor-default'}`}
            title="Forward"
          >
            <ArrowRight className="w-3.5 h-3.5" />
          </button>
        </div>
      </div>

      {/* Top Actions: New Task, Search */}
      <div className="p-2 space-y-0.5 border-b border-[#e5e7eb] dark:border-[#232324]">
        <button
          type="button"
          onClick={handleNewTask}
          className={`w-full flex items-center justify-between px-2.5 py-1.5 rounded-lg text-left transition-colors cursor-pointer ${
            activeSessionId === null
              ? 'bg-[#e5e7eb] dark:bg-[#262628] text-[#111827] dark:text-white font-medium'
              : 'text-[#4b5563] dark:text-[#cccccc] hover:bg-[#f3f4f6] dark:hover:bg-[#222223] hover:text-[#111827] dark:hover:text-white'
          }`}
        >
          <div className="flex items-center gap-2">
            <Plus className="w-4 h-4 text-[#6b7280] dark:text-[#999999]" />
            <span className="font-medium">New task</span>
          </div>
          <kbd className="text-[10px] text-[#9ca3af] dark:text-[#777777] font-mono">⌘ N</kbd>
        </button>

        <button
          type="button"
          onClick={() => setIsCommandPaletteOpen(true)}
          className="w-full flex items-center justify-between px-2.5 py-1.5 rounded-lg text-[#4b5563] dark:text-[#cccccc] hover:bg-[#f3f4f6] dark:hover:bg-[#222223] hover:text-[#111827] dark:hover:text-white text-left transition-colors cursor-pointer"
        >
          <div className="flex items-center gap-2">
            <Search className="w-4 h-4 text-[#6b7280] dark:text-[#999999]" />
            <span>Search</span>
          </div>
          <kbd className="text-[10px] text-[#9ca3af] dark:text-[#777777] font-mono">⌘ K</kbd>
        </button>
      </div>

      {/* Segment Filter: [# Group] [📁 Project] & Expand/Collapse All */}
      <div className="px-3 pt-2.5 pb-1.5 flex items-center justify-between">
        <div className="flex items-center gap-1 bg-[#f3f4f6] dark:bg-[#1e1e1f] p-0.5 rounded-lg border border-[#e5e7eb] dark:border-[#2b2b2d]">
          <button
            type="button"
            onClick={() => setActiveTab('group')}
            className={`flex items-center gap-1 px-2 py-0.5 rounded-md text-[11px] font-medium transition-colors cursor-pointer ${
              activeTab === 'group'
                ? 'bg-white dark:bg-[#2b2b2d] text-[#111827] dark:text-white shadow-2xs'
                : 'text-[#6b7280] dark:text-[#888888] hover:text-[#111827] dark:hover:text-[#dddddd]'
            }`}
          >
            <Hash className="w-3 h-3" />
            <span>Group</span>
          </button>
          <button
            type="button"
            onClick={() => setActiveTab('project')}
            className={`flex items-center gap-1 px-2 py-0.5 rounded-md text-[11px] font-medium transition-colors cursor-pointer ${
              activeTab === 'project'
                ? 'bg-white dark:bg-[#2b2b2d] text-[#111827] dark:text-white shadow-2xs'
                : 'text-[#6b7280] dark:text-[#888888] hover:text-[#111827] dark:hover:text-[#dddddd]'
            }`}
          >
            <Folder className="w-3 h-3 text-[#d97706]" />
            <span>Project</span>
          </button>
        </div>

        <div className="flex items-center gap-1 text-[#6b7280] dark:text-[#777777]">
          <button
            type="button"
            onClick={() => {
              const allCollapsed = Object.keys(collapsedFolders).length === projectGroups.length;
              if (allCollapsed) {
                setCollapsedFolders({});
              } else {
                const next: Record<string, boolean> = {};
                projectGroups.forEach(g => { next[g.projectName] = true; });
                setCollapsedFolders(next);
              }
            }}
            className="p-1 rounded hover:bg-[#f3f4f6] dark:hover:bg-[#262628] hover:text-[#111827] dark:hover:text-white transition-colors cursor-pointer"
            title="Expand / Collapse all"
          >
            <FolderKanban className="w-3.5 h-3.5" />
          </button>
        </div>
      </div>

      {/* Projects & Grouped Sessions Tree */}
      <div className="flex-1 overflow-y-auto px-2 py-1 space-y-3">
        {/* Section Header */}
        <div className="px-2 pt-1 text-[11px] font-medium text-[#6b7280] dark:text-[#777777] uppercase tracking-wider">
          Projects
        </div>

        {projectGroups.map(group => {
          const isCollapsed = collapsedFolders[group.projectName];

          return (
            <div key={group.projectName} className="space-y-0.5">
              {/* Project Folder Header */}
              <button
                type="button"
                onClick={() => toggleFolder(group.projectName)}
                className="w-full flex items-center justify-between px-2 py-1 rounded-md text-[#4b5563] dark:text-[#aaaaaa] hover:text-[#111827] dark:hover:text-white hover:bg-[#f3f4f6] dark:hover:bg-[#222223] text-left transition-colors cursor-pointer group"
              >
                <div className="flex items-center gap-1.5 min-w-0">
                  {isCollapsed ? (
                    <ChevronRight className="w-3.5 h-3.5 text-[#9ca3af] dark:text-[#777777] shrink-0" />
                  ) : (
                    <ChevronDown className="w-3.5 h-3.5 text-[#9ca3af] dark:text-[#777777] shrink-0" />
                  )}
                  <Folder className="w-3.5 h-3.5 text-[#d97706] shrink-0" />
                  <span className="font-semibold text-xs text-[#1f2937] dark:text-[#dddddd] truncate">
                    {group.projectName}
                  </span>
                </div>
                <span className="text-[10px] text-[#9ca3af] dark:text-[#666666] font-mono">
                  {group.sessions.length}
                </span>
              </button>

              {/* Sessions List */}
              {!isCollapsed && (
                <div className="pl-4 space-y-0.5 border-l border-[#e5e7eb] dark:border-[#242426] ml-3.5">
                  {group.sessions.map(sess => {
                    const isActive = sess.id === activeSessionId;
                    const isRunning = sess.status === 'running';

                    return (
                      <div
                        key={sess.id}
                        onClick={() => handleSelectSession(sess)}
                        className={`group flex items-center justify-between px-2.5 py-1.5 rounded-lg text-xs cursor-pointer transition-all ${
                          isActive
                            ? 'bg-[#e5e7eb] dark:bg-[#2b2b2e] text-[#111827] dark:text-white font-medium shadow-xs'
                            : 'text-[#4b5563] dark:text-[#bbbbbb] hover:bg-[#f3f4f6] dark:hover:bg-[#202022] hover:text-[#111827] dark:hover:text-white'
                        }`}
                      >
                        <div className="flex items-center gap-2 min-w-0 flex-1 pr-2">
                          {isRunning ? (
                            <span className="w-2 h-2 rounded-full bg-[#ef4444] animate-pulse shrink-0" />
                          ) : (
                            <span className="w-1.5 h-1.5 rounded-full bg-transparent shrink-0" />
                          )}
                          <span className="truncate text-xs leading-snug">
                            {sess.title || 'New Session'}
                          </span>
                        </div>

                        <div className="flex items-center gap-1.5 shrink-0">
                          <span className="text-[10px] text-[#9ca3af] dark:text-[#777777] group-hover:hidden">
                            {sess.updatedAt || sess.createdAt || 'now'}
                          </span>
                          <button
                            type="button"
                            onClick={(e) => {
                              e.stopPropagation();
                              deleteSessionPermanently(sess.id);
                            }}
                            className="hidden group-hover:block p-0.5 text-[#9ca3af] hover:text-[#ef4444] rounded transition-colors"
                            title="Delete session"
                          >
                            <Trash2 className="w-3 h-3" />
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

    </aside>
  );
};
