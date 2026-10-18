import React from 'react';
import {
  Files,
  Search,
  GitBranch,
  Server,
  Settings
} from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';
import { ActivityBarItem } from '../../types';

export const ActivityBar: React.FC = () => {
  const { 
    activeActivity, 
    setActiveActivity, 
    openSettingsTab, 
    isLeftSidebarOpen,
    setIsLeftSidebarOpen,
    isRightActionDrawerOpen,
    setIsRightActionDrawerOpen,
    gitFiles,
    setIsSSHModalOpen,
    isGitClientOpen,
    setIsGitClientOpen
  } = useWorkspace();

  const handleActivityClick = (activity: ActivityBarItem) => {
    setIsGitClientOpen(false);
    if (activeActivity === activity && isLeftSidebarOpen) {
      setIsLeftSidebarOpen(false);
    } else {
      setActiveActivity(activity);
      setIsLeftSidebarOpen(true);
    }
  };

  const handleGitClick = () => {
    if (isGitClientOpen) {
      setIsGitClientOpen(false);
    } else {
      setIsGitClientOpen(true);
      setIsLeftSidebarOpen(false);
    }
  };

  return (
    <div className="w-[48px] min-w-[48px] bg-background border-r border-border flex flex-col items-center justify-between py-2 select-none z-10 text-foreground-subtle">
      
      {/* Core action icons: Explorer / Files, Search, Git Client */}
      <div className="flex flex-col items-center gap-1 w-full">
        {/* Explorer / Files */}
        <button
          type="button"
          onClick={() => handleActivityClick('explorer')}
          className={`w-full h-10 flex items-center justify-center relative transition-colors cursor-pointer ${
            activeActivity === 'explorer' && isLeftSidebarOpen && !isGitClientOpen
              ? 'text-foreground'
              : 'hover:text-foreground text-foreground-subtle'
          }`}
          title="Explorer (⌘⇧E)"
          aria-label="Explorer"
          aria-pressed={activeActivity === 'explorer' && isLeftSidebarOpen && !isGitClientOpen}
        >
          {activeActivity === 'explorer' && isLeftSidebarOpen && !isGitClientOpen && (
            <div className="absolute left-0 top-1 bottom-1 w-[2px] bg-foreground rounded-r" />
          )}
          <Files className="w-5 h-5 stroke-[1.6]" />
        </button>

        {/* Search */}
        <button
          type="button"
          onClick={() => handleActivityClick('search')}
          className={`w-full h-10 flex items-center justify-center relative transition-colors cursor-pointer ${
            activeActivity === 'search' && isLeftSidebarOpen && !isGitClientOpen
              ? 'text-foreground'
              : 'hover:text-foreground text-foreground-subtle'
          }`}
          title="Search in Files (⌘⇧F)"
          aria-label="Search in files"
          aria-pressed={activeActivity === 'search' && isLeftSidebarOpen && !isGitClientOpen}
        >
          {activeActivity === 'search' && isLeftSidebarOpen && !isGitClientOpen && (
            <div className="absolute left-0 top-1 bottom-1 w-[2px] bg-foreground rounded-r" />
          )}
          <Search className="w-5 h-5 stroke-[1.6]" />
        </button>

        {/* Git Client */}
        <button
          type="button"
          onClick={handleGitClick}
          className={`w-full h-10 flex items-center justify-center relative transition-colors cursor-pointer ${
            isGitClientOpen
              ? 'text-foreground'
              : 'hover:text-foreground text-foreground-subtle'
          }`}
          title="Git Client (⌘⇧G)"
          aria-label="Git Client"
          aria-pressed={isGitClientOpen}
        >
          {isGitClientOpen && (
            <div className="absolute left-0 top-1 bottom-1 w-[2px] bg-foreground rounded-r" />
          )}
          <div className="relative">
            <GitBranch className="w-5 h-5 stroke-[1.6]" />
            {gitFiles.length > 0 && (
              <span className="absolute -top-1 -right-1.5 flex h-3.5 min-w-3.5 items-center justify-center rounded-full bg-primary px-0.5 text-[9px] font-bold text-primary-foreground leading-none">
                {gitFiles.length > 99 ? '99+' : gitFiles.length}
              </span>
            )}
          </div>
        </button>
      </div>

      {/* Bottom Footer: SSH remote connect, Settings */}
      <div className="flex flex-col items-center gap-1 w-full pb-1">
        <button
          type="button"
          onClick={() => setIsSSHModalOpen(true)}
          className="w-full h-9 flex items-center justify-center text-foreground-subtle hover:text-foreground transition-colors cursor-pointer"
          title="Connect to SSH remote"
          aria-label="Connect to SSH remote"
        >
          <Server className="w-4.5 h-4.5 stroke-[1.6]" />
        </button>
        <button
          type="button"
          onClick={() => openSettingsTab('agentClis')}
          className="w-full h-9 flex items-center justify-center text-foreground-subtle hover:text-foreground transition-colors cursor-pointer"
          title="Settings (⌘,)"
          aria-label="Settings"
        >
          <Settings className="w-4.5 h-4.5 stroke-[1.6]" />
        </button>
      </div>

    </div>
  );
};
