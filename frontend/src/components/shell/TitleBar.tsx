import React from 'react';
import { 
  Search, 
  Columns, 
  Sparkles, 
  Layers,
  FolderOpen
} from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';

export const TitleBar: React.FC = () => {
  const { 
    mode, 
    setMode, 
    activeSession, 
    activeSessionId, 
    isSplitEditor, 
    setIsSplitEditor, 
    setIsCommandPaletteOpen,
    activeWorkspacePath,
    openFolder
  } = useWorkspace();

  // Compute breadcrumb title
  const getBreadcrumbTitle = () => {
    if (mode === 'agent') {
      if (activeSessionId && activeSession) {
        return activeSession.title;
      }
      return 'New Space';
    } else {
      return `${activeWorkspacePath} - Preview Readme.md`;
    }
  };

  return (
    <header className="h-[44px] min-h-[44px] bg-[#ffffff] dark:bg-[#181818] border-b border-[#e5e7eb] dark:border-[#2b2b2b] flex items-center justify-between px-3.5 select-none z-30 transition-colors duration-150">
      {/* Left controls: Mode Switcher + Navigation */}
      <div className="flex items-center gap-3">
        {/* Mode Segment Switcher: Agent | Editor */}
        <div className="flex items-center bg-[#f1f3f5] dark:bg-[#252526] p-0.5 rounded-lg border border-[#e2e5e9] dark:border-[#333333]">
          <button
            type="button"
            onClick={() => setMode('agent')}
            className={`px-3 py-1 text-xs font-medium rounded-md transition-all duration-150 flex items-center gap-1.5 ${
              mode === 'agent'
                ? 'bg-white dark:bg-[#1e1e1e] text-[#1c1e21] dark:text-[#f3f4f6] shadow-xs font-semibold'
                : 'text-[#6b7280] dark:text-[#9ca3af] hover:text-[#1c1e21] dark:hover:text-white'
            }`}
          >
            <Sparkles className="w-3 h-3 text-[#3b82f6]" />
            <span>Agent</span>
          </button>
          <button
            type="button"
            onClick={() => setMode('editor')}
            className={`px-3 py-1 text-xs font-medium rounded-md transition-all duration-150 flex items-center gap-1.5 ${
              mode === 'editor'
                ? 'bg-white dark:bg-[#1e1e1e] text-[#1c1e21] dark:text-[#f3f4f6] shadow-xs font-semibold'
                : 'text-[#6b7280] dark:text-[#9ca3af] hover:text-[#1c1e21] dark:hover:text-white'
            }`}
          >
            <Layers className="w-3 h-3 text-[#10b981]" />
            <span>Editor</span>
          </button>
        </div>

        {/* Quick Tools: Search */}
        <div className="flex items-center gap-1 text-[#6b7280] dark:text-[#9ca3af] ml-1">
          <button
            type="button"
            onClick={() => setIsCommandPaletteOpen(true)}
            className="p-1.5 hover:bg-[#e5e7eb] dark:hover:bg-[#2a2d2e] rounded-md transition-colors text-xs flex items-center gap-1"
            title="Search files and actions (⌘K / ⌘P)"
          >
            <Search className="w-3.5 h-3.5" />
          </button>
        </div>

        {/* Breadcrumb / Space Title */}
        <div className="ml-2 flex items-center gap-1.5 text-xs text-[#374151] dark:text-[#d1d5db] font-medium tracking-tight">
          <span className="truncate max-w-[260px] font-semibold">{getBreadcrumbTitle()}</span>
          {mode === 'editor' && (
            <span className="text-[10px] px-1.5 py-0.5 rounded bg-[#f3f4f6] dark:bg-[#2a2d2e] text-[#6b7280] dark:text-[#9ca3af] font-mono">
              ⌘P
            </span>
          )}
        </div>
      </div>

      {/* Right Controls: Open Folder + Split View Toggle */}
      <div className="flex items-center gap-2">
        <button
          type="button"
          onClick={() => openFolder()}
          className="flex items-center gap-1.5 px-2.5 py-1 rounded-lg bg-[#f1f5f9] dark:bg-[#252526] hover:bg-[#e2e8f0] dark:hover:bg-[#333333] text-xs text-[#334155] dark:text-[#e2e8f0] font-medium transition-colors cursor-pointer"
          title="Open or Pick Workspace Folder (⌘O)"
        >
          <FolderOpen className="w-3.5 h-3.5 text-[#2563eb] dark:text-[#38bdf8]" />
          <span className="hidden sm:inline">Open Folder</span>
        </button>

        {/* Quick trigger for Split */}
        <button
          type="button"
          onClick={() => setIsSplitEditor(prev => !prev)}
          className={`p-1.5 rounded-lg transition-colors ${
            isSplitEditor 
              ? 'text-[#2563eb] dark:text-[#60a5fa] bg-[#eff6ff] dark:bg-[#1e293b]' 
              : 'text-[#6b7280] dark:text-[#9ca3af] hover:bg-[#e5e7eb] dark:hover:bg-[#2a2d2e]'
          }`}
          title="Split View / Side-by-side"
        >
          <Columns className="w-4 h-4" />
        </button>
      </div>
    </header>
  );
};

