import React from 'react';
import {
  Search,
  Sparkles,
  ChevronLeft,
  ChevronRight,
  PanelLeft,
  PanelRight,
  Columns2
} from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';
import { EditorProjectDropdown } from '../editor/EditorProjectDropdown';

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
    selectedFile,
    openTabs,
    activeTabId,
    isRightActionDrawerOpen,
    setIsRightActionDrawerOpen,
    openSettingsTab,
    goBack,
    goForward,
    canGoBack,
    canGoForward
  } = useWorkspace();

  const activeTab = openTabs.find(t => t.id === activeTabId);
  const currentFileName = activeTab?.fileName || selectedFile?.name || '';
  const workspaceName = activeWorkspacePath ? activeWorkspacePath.split('/').pop() || '' : '';

  return (
    <header className="h-[42px] min-h-[42px] bg-[#ffffff] dark:bg-[#181818] border-b border-[#e5e7eb] dark:border-[#2b2b2b] flex items-center justify-between px-3 select-none z-30 transition-colors duration-150 font-sans">
      
      {/* Left Area: Mode Switcher + Nav */}
      <div className="flex items-center gap-3">

        {/* Mode Segment Switcher: Agent | Editor */}
        <div className="flex items-center bg-[#f1f3f5] dark:bg-[#252526] p-0.5 rounded-lg border border-[#e2e5e9] dark:border-[#333333]">
          <button
            type="button"
            onClick={() => setMode('agent')}
            className={`px-2.5 py-0.5 text-xs font-medium rounded-md transition-all duration-150 flex items-center gap-1.5 cursor-pointer ${
              mode === 'agent'
                ? 'bg-white dark:bg-[#1e1e1e] text-[#111827] dark:text-[#f3f4f6] shadow-xs font-semibold'
                : 'text-[#6b7280] dark:text-[#9ca3af] hover:text-[#111827] dark:hover:text-white'
            }`}
          >
            <span>Agent</span>
          </button>
          <button
            type="button"
            onClick={() => setMode('editor')}
            className={`px-2.5 py-0.5 text-xs font-medium rounded-md transition-all duration-150 flex items-center gap-1.5 cursor-pointer ${
              mode === 'editor'
                ? 'bg-white dark:bg-[#1e1e1e] text-[#111827] dark:text-[#f3f4f6] shadow-xs font-semibold'
                : 'text-[#6b7280] dark:text-[#9ca3af] hover:text-[#111827] dark:hover:text-white'
            }`}
          >
            <span>Editor</span>
          </button>
        </div>

        {/* Navigation Tools */}
        <div className="flex items-center gap-0.5 text-[#6b7280] dark:text-[#9ca3af]">
          <button
            type="button"
            onClick={() => setIsCommandPaletteOpen(true)}
            className="p-1 hover:bg-[#f3f4f6] dark:hover:bg-[#282828] rounded-md transition-colors cursor-pointer"
            title="Search (⌘K)"
          >
            <Search className="w-3.5 h-3.5" />
          </button>
          <button
            type="button"
            className="p-1 hover:bg-[#f3f4f6] dark:hover:bg-[#282828] rounded-md transition-colors cursor-pointer"
            title="Toggle Primary Sidebar"
          >
            <PanelLeft className="w-3.5 h-3.5" />
          </button>
        </div>

        {/* Back / Forward Arrows — navigate through opened/closed tab history */}
        <div className="flex items-center text-[#9ca3af] dark:text-[#666] ml-1">
          <button
            type="button"
            onClick={goBack}
            disabled={!canGoBack}
            className={`p-1 rounded-md transition-colors cursor-pointer ${
              canGoBack
                ? 'hover:text-[#111827] dark:hover:text-white hover:bg-[#f3f4f6] dark:hover:bg-[#282828]'
                : 'opacity-40 cursor-default'
            }`}
            title="Go Back"
          >
            <ChevronLeft className="w-3.5 h-3.5" />
          </button>
          <button
            type="button"
            onClick={goForward}
            disabled={!canGoForward}
            className={`p-1 rounded-md transition-colors cursor-pointer ${
              canGoForward
                ? 'hover:text-[#111827] dark:hover:text-white hover:bg-[#f3f4f6] dark:hover:bg-[#282828]'
                : 'opacity-40 cursor-default'
            }`}
            title="Go Forward"
          >
            <ChevronRight className="w-3.5 h-3.5" />
          </button>
        </div>
      </div>

      {/* Center Area: Quick Command / File Bar */}
      <div className="flex-1 max-w-xl mx-4 flex items-center gap-2">
        {mode === 'editor' && <EditorProjectDropdown />}

        <button
          type="button"
          onClick={() => setIsCommandPaletteOpen(true)}
          className="flex-1 h-[28px] px-3 bg-[#f8fafc] dark:bg-[#222224] hover:bg-[#f1f5f9] dark:hover:bg-[#28282b] border border-[#e2e8f0] dark:border-[#383838] rounded-lg flex items-center justify-between text-xs text-[#475569] dark:text-[#9ca3af] transition-all cursor-pointer group shadow-2xs"
        >
          <div className="flex items-center gap-2 truncate">
            <Search className="w-3.5 h-3.5 text-[#9ca3af] group-hover:text-[#2563eb] transition-colors shrink-0" />
            <span className="truncate font-medium text-[#334155] dark:text-[#e2e8f0]">
              {mode === 'agent'
                ? (activeSession?.title || 'New Task')
                : [workspaceName, currentFileName].filter(Boolean).join(' - ') || 'Standalone Code Editor'}
            </span>
          </div>

          <div className="flex items-center gap-1.5 shrink-0">
            <kbd className="text-[10px] px-1.5 py-0.2 rounded bg-white dark:bg-[#2d2d30] border border-[#e2e8f0] dark:border-[#444] text-[#64748b] dark:text-[#94a3b8] font-mono shadow-2xs">
              ⌘ P
            </kbd>
            <Sparkles className="w-3.5 h-3.5 text-[#7c3aed] hover:scale-110 transition-transform" />
          </div>
        </button>
      </div>

      {/* Right Area: Layout Panels + User Avatar */}
      <div className="flex items-center gap-1 text-[#6b7280] dark:text-[#9ca3af]">
        <button
          type="button"
          onClick={() => setIsSplitEditor(prev => !prev)}
          className={`p-1.5 rounded-md transition-colors cursor-pointer ${
            isSplitEditor 
              ? 'text-[#2563eb] dark:text-[#60a5fa] bg-[#eff6ff] dark:bg-[#1e293b]' 
              : 'hover:bg-[#f3f4f6] dark:hover:bg-[#282828] hover:text-[#111827] dark:hover:text-white'
          }`}
          title="Split Editor"
        >
          <Columns2 className="w-3.5 h-3.5" />
        </button>

        <button
          type="button"
          onClick={() => setIsRightActionDrawerOpen(!isRightActionDrawerOpen)}
          className={`p-1.5 rounded-md transition-colors cursor-pointer ${
            isRightActionDrawerOpen 
              ? 'text-[#2563eb] dark:text-[#60a5fa] bg-[#eff6ff] dark:bg-[#1e293b]' 
              : 'hover:bg-[#f3f4f6] dark:hover:bg-[#282828] hover:text-[#111827] dark:hover:text-white'
          }`}
          title="Toggle Secondary Side Bar"
        >
          <PanelRight className="w-3.5 h-3.5" />
        </button>

        {/* User Profile Avatar */}
        <div className="ml-1.5 relative">
          <button
            type="button"
            onClick={() => openSettingsTab('privacy')}
            className="w-6 h-6 rounded-full bg-[#7c3aed] text-white flex items-center justify-center text-[10px] font-bold shadow-xs hover:opacity-90 transition-opacity cursor-pointer"
            title="User Profile (WO)"
          >
            WO
          </button>
          <span className="absolute bottom-0 right-0 w-2 h-2 rounded-full bg-[#10b981] border border-white dark:border-[#181818]" />
        </div>
      </div>

    </header>
  );
};

