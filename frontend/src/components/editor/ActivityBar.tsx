import React from 'react';
import {
  Files,
  Search,
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
    setIsLeftSidebarOpen
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
    <div className="w-[48px] min-w-[48px] bg-background border-r border-border flex flex-col items-center justify-between py-2 select-none z-10 text-foreground-subtle dark:text-foreground-subtle">
      
      {/* Core action icons: Explorer / Files, Search */}
      <div className="flex flex-col items-center gap-1 w-full">
        {/* Explorer / Files */}
        <button
          type="button"
          onClick={() => handleActivityClick('explorer')}
          className={`w-full h-10 flex items-center justify-center relative transition-colors cursor-pointer ${
            activeActivity === 'explorer' && isLeftSidebarOpen
              ? 'text-foreground'
              : 'hover:text-foreground dark:hover:text-white'
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
              : 'hover:text-foreground dark:hover:text-white'
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
      </div>

      {/* Bottom Footer: Settings */}
      <div className="flex flex-col items-center gap-1 w-full pb-1">
        <button
          type="button"
          onClick={() => openSettingsTab('agentClis')}
          className="w-full h-9 flex items-center justify-center text-foreground-subtle dark:text-foreground-subtle hover:text-foreground dark:hover:text-white transition-colors cursor-pointer"
          title="Settings (⌘,)"
          aria-label="Settings"
        >
          <Settings className="w-4.5 h-4.5 stroke-[1.6]" />
        </button>
      </div>

    </div>
  );
};
