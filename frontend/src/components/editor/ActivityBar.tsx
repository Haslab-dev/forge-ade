import React from 'react';
import { Files, GitBranch, Puzzle, Search } from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';

export const ActivityBar: React.FC = () => {
  const { activeActivity, setActiveActivity } = useWorkspace();

  return (
    <div className="w-[48px] min-w-[48px] bg-[#f8fafc] dark:bg-[#181818] border-r border-[#e2e8f0] dark:border-[#2b2b2b] flex flex-col items-center justify-between py-2 select-none z-10 text-[#64748b] dark:text-[#858585]">
      {/* Top action icons */}
      <div className="flex flex-col items-center gap-1 w-full">
        {/* Explorer */}
        <button
          type="button"
          onClick={() => setActiveActivity('explorer')}
          className={`w-full h-11 flex items-center justify-center relative transition-colors ${
            activeActivity === 'explorer'
              ? 'text-[#0f172a] dark:text-white'
              : 'hover:text-[#0f172a] dark:hover:text-white'
          }`}
          title="Explorer (⌘⇧E)"
        >
          {activeActivity === 'explorer' && (
            <div className="absolute left-0 top-0 bottom-0 w-[2px] bg-[#2563eb] dark:bg-white" />
          )}
          <Files className="w-5 h-5 stroke-[1.75]" />
        </button>

        {/* Search */}
        <button
          type="button"
          onClick={() => setActiveActivity('search')}
          className={`w-full h-11 flex items-center justify-center relative transition-colors ${
            activeActivity === 'search'
              ? 'text-[#0f172a] dark:text-white'
              : 'hover:text-[#0f172a] dark:hover:text-white'
          }`}
          title="Search (⌘⇧F)"
        >
          {activeActivity === 'search' && (
            <div className="absolute left-0 top-0 bottom-0 w-[2px] bg-[#2563eb] dark:bg-white" />
          )}
          <Search className="w-5 h-5 stroke-[1.75]" />
        </button>

        {/* Source Control */}
        <button
          type="button"
          onClick={() => setActiveActivity('git')}
          className={`w-full h-11 flex items-center justify-center relative transition-colors ${
            activeActivity === 'git'
              ? 'text-[#0f172a] dark:text-white'
              : 'hover:text-[#0f172a] dark:hover:text-white'
          }`}
          title="Source Control (⌃⇧G)"
        >
          {activeActivity === 'git' && (
            <div className="absolute left-0 top-0 bottom-0 w-[2px] bg-[#2563eb] dark:bg-white" />
          )}
          <div className="relative">
            <GitBranch className="w-5 h-5 stroke-[1.75]" />
            <span className="absolute -top-1 -right-1.5 w-3.5 h-3.5 bg-[#2563eb] dark:bg-[#007acc] text-white text-[9px] font-bold rounded-full flex items-center justify-center">
              1
            </span>
          </div>
        </button>
      </div>
    </div>
  );
};
