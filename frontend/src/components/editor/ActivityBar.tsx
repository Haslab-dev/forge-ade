import React from 'react';
import {
  Files,
  GitBranch,
  Search,
  Settings,
  SquareTerminal
} from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';

export const ActivityBar: React.FC = () => {
  const { activeActivity, setActiveActivity, openSettingsTab, gitFiles } = useWorkspace();

  return (
    <div className="w-[48px] min-w-[48px] bg-white dark:bg-[#181818] border-r border-[#e5e7eb] dark:border-[#2b2b2b] flex flex-col items-center justify-between py-2 select-none z-10 text-[#6b7280] dark:text-[#858585]">
      
      {/* Core action icons: Editor/Explorer, Search, Git Source Control */}
      <div className="flex flex-col items-center gap-1 w-full">
        {/* Explorer / Files */}
        <button
          type="button"
          onClick={() => setActiveActivity('explorer')}
          className={`w-full h-10 flex items-center justify-center relative transition-colors cursor-pointer ${
            activeActivity === 'explorer'
              ? 'text-[#111827] dark:text-white'
              : 'hover:text-[#111827] dark:hover:text-white'
          }`}
          title="Explorer (⌘⇧E)"
        >
          {activeActivity === 'explorer' && (
            <div className="absolute left-0 top-1 bottom-1 w-[2px] bg-[#2563eb] dark:bg-white rounded-r" />
          )}
          <Files className="w-5 h-5 stroke-[1.6]" />
        </button>

        {/* Search */}
        <button
          type="button"
          onClick={() => setActiveActivity('search')}
          className={`w-full h-10 flex items-center justify-center relative transition-colors cursor-pointer ${
            activeActivity === 'search'
              ? 'text-[#111827] dark:text-white'
              : 'hover:text-[#111827] dark:hover:text-white'
          }`}
          title="Search in Files (⌘⇧F)"
        >
          {activeActivity === 'search' && (
            <div className="absolute left-0 top-1 bottom-1 w-[2px] bg-[#2563eb] dark:bg-white rounded-r" />
          )}
          <Search className="w-5 h-5 stroke-[1.6]" />
        </button>

        {/* Git Source Control */}
        <button
          type="button"
          onClick={() => setActiveActivity('git')}
          className={`w-full h-10 flex items-center justify-center relative transition-colors cursor-pointer ${
            activeActivity === 'git'
              ? 'text-[#111827] dark:text-white'
              : 'hover:text-[#111827] dark:hover:text-white'
          }`}
          title="Source Control (⌃⇧G)"
        >
          {activeActivity === 'git' && (
            <div className="absolute left-0 top-1 bottom-1 w-[2px] bg-[#2563eb] dark:bg-white rounded-r" />
          )}
          <div className="relative">
            <GitBranch className="w-5 h-5 stroke-[1.6]" />
            {gitFiles.length > 0 && (
              <span className="absolute -top-1 -right-2 min-w-[14px] h-3.5 px-1 bg-[#2563eb] text-white text-[9px] font-bold rounded-full flex items-center justify-center shadow-2xs">
                {gitFiles.length}
              </span>
            )}
          </div>
        </button>

        {/* Shell (integrated terminal sessions) */}
        <button
          type="button"
          onClick={() => setActiveActivity('shell')}
          className={`w-full h-10 flex items-center justify-center relative transition-colors cursor-pointer ${
            activeActivity === 'shell'
              ? 'text-[#111827] dark:text-white'
              : 'hover:text-[#111827] dark:hover:text-white'
          }`}
          title="Shell"
        >
          {activeActivity === 'shell' && (
            <div className="absolute left-0 top-1 bottom-1 w-[2px] bg-[#2563eb] dark:bg-white rounded-r" />
          )}
          <SquareTerminal className="w-5 h-5 stroke-[1.6]" />
        </button>
      </div>

      {/* Bottom Footer: Settings */}
      <div className="flex flex-col items-center gap-1 w-full pb-1">
        {/* Settings */}
        <button
          type="button"
          onClick={() => openSettingsTab('agents')}
          className="w-full h-9 flex items-center justify-center text-[#6b7280] dark:text-[#858585] hover:text-[#111827] dark:hover:text-white transition-colors cursor-pointer"
          title="Settings (⌘,)"
        >
          <Settings className="w-4.5 h-4.5 stroke-[1.6]" />
        </button>
      </div>

    </div>
  );
};
