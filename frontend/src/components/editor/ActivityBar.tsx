import React from 'react';
import {
  Files,
  Search,
  GitBranch,
  FolderGit2,
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
    if (activeActivity === activity && isLeftSidebarOpen) {
      setIsLeftSidebarOpen(false);
    } else {
      setActiveActivity(activity);
      setIsLeftSidebarOpen(true);
    }
  };

  return (
    <div className="w-[48px] min-w-[48px] bg-background border-r border-border flex flex-col items-center justify-between py-2 select-none z-10 text-foreground-subtle">
      
      {/* Core action icons: Explorer / Files, Search, Source Control */}
      <div className="flex flex-col items-center gap-1 w-full">
        {/* Explorer / Files */}
        <button
          type="button"
          onClick={() => handleActivityClick('explorer')}
          className={`w-full h-10 flex items-center justify-center relative transition-colors cursor-pointer ${
            activeActivity === 'explorer' && isLeftSidebarOpen
              ? 'text-foreground'
              : 'hover:text-foreground'
          }`}
          title="Explorer (⌘⇧E)"
          aria-label="Explorer"
          aria-pressed={activeActivity === 'explorer'}
        >
          {activeActivity === 'explorer' && isLeftSidebarOpen && (
            <div className="absolute left-0 top-1 bottom-1 w-[2px] bg-foreground rounded-r" />
          )}
          <Files className="w-5 h-5 stroke-[1.6]" />
        </button>

        {/* Search */}
        <button
          type="button"
          onClick={() => handleActivityClick('search')}
          className={`w-full h-10 flex items-center justify-center relative transition-colors cursor-pointer ${
            activeActivity === 'search' && isLeftSidebarOpen
              ? 'text-foreground'
              : 'hover:text-foreground'
          }`}
          title="Search in Files (⌘⇧F)"
          aria-label="Search in files"
          aria-pressed={activeActivity === 'search'}
        >
          {activeActivity === 'search' && isLeftSidebarOpen && (
            <div className="absolute left-0 top-1 bottom-1 w-[2px] bg-foreground rounded-r" />
          )}
          <Search className="w-5 h-5 stroke-[1.6]" />
        </button>

        {/* Source Control / Git */}
        <button
          type="button"
          onClick={() => setIsRightActionDrawerOpen(prev => !prev)}
          className={`w-full h-10 flex items-center justify-center relative transition-colors cursor-pointer ${
            isRightActionDrawerOpen
              ? 'text-foreground'
              : 'hover:text-foreground'
          }`}
          title="Source Control / Git (⌘/)"
          aria-label="Source Control"
          aria-pressed={isRightActionDrawerOpen}
        >
          {isRightActionDrawerOpen && (
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

        {/* Full Git Client (Git Desktop / SourceTree view) */}
        <button
          type="button"
          onClick={() => setIsGitClientOpen(prev => !prev)}
          className={`w-full h-10 flex items-center justify-center relative transition-colors cursor-pointer ${
            isGitClientOpen
              ? 'text-primary'
              : 'hover:text-foreground text-foreground-subtle'
          }`}
          title="Git Desktop Client (SourceTree style)"
          aria-label="Git Client"
          aria-pressed={isGitClientOpen}
        >
          {isGitClientOpen && (
            <div className="absolute left-0 top-1 bottom-1 w-[2px] bg-primary rounded-r" />
          )}
          <FolderGit2 className="w-5 h-5 stroke-[1.6]" />
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
