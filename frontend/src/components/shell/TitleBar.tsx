import React, { useEffect } from 'react';
import {
  Search,
  Sparkles,
  ChevronLeft,
  ChevronRight,
  PanelLeft
} from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';
import { EditorProjectDropdown } from '../editor/EditorProjectDropdown';

export const TitleBar: React.FC = () => {
  const {
    mode,
    setMode,
    activeSession,
    setIsCommandPaletteOpen,
    isLeftSidebarOpen,
    setIsLeftSidebarOpen,
    activeWorkspacePath,
    selectedFile,
    openTabs,
    activeTabId,
    openSettingsTab,
    goBack,
    goForward,
    canGoBack,
    canGoForward
  } = useWorkspace();

  const activeTab = openTabs.find(t => t.id === activeTabId);
  const currentFileName = activeTab?.fileName || selectedFile?.name || '';
  const workspaceName = activeWorkspacePath ? activeWorkspacePath.split('/').pop() || '' : '';

  // Global shortcut ⌘B / Ctrl+B to toggle left sidebar
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'b') {
        e.preventDefault();
        setIsLeftSidebarOpen(prev => !prev);
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [setIsLeftSidebarOpen]);

  return (
    <header className="h-[44px] min-h-[44px] bg-[#FFFFFF] dark:bg-[#161617] border-b border-[#E5E7EB] dark:border-[#333336] flex items-center justify-between px-3 select-none z-30 transition-colors font-[Inter,system-ui,sans-serif]">
      
      {/* Left Area: Sidebar Toggle + Mode Switcher + Nav */}
      <div className="flex items-center gap-2">
        {/* Toggle Left Sidebar Button (both in Agent and Editor) */}
        {mode !== 'settings' && (
          <button
            type="button"
            onClick={() => setIsLeftSidebarOpen(prev => !prev)}
            className={`p-1.5 rounded-[6px] transition-colors cursor-pointer ${
              isLeftSidebarOpen
                ? 'text-[#111827] dark:text-[#F2F2F2] hover:bg-[#F3F4F6] dark:hover:bg-[#2A2A2D]'
                : 'text-[#6B7280] dark:text-[#9B9B9F] hover:bg-[#F3F4F6] dark:hover:bg-[#2A2A2D] hover:text-[#111827] dark:hover:text-[#F2F2F2]'
            }`}
            title={isLeftSidebarOpen ? 'Hide Left Sidebar (⌘B)' : 'Show Left Sidebar (⌘B)'}
          >
            <PanelLeft className="w-4 h-4" />
          </button>
        )}

        {/* Mode Segment Switcher: Agent | Editor */}
        <div className="flex items-center bg-[#F3F4F6] dark:bg-[#1E1E20] p-0.5 rounded-[8px] border border-[#E5E7EB] dark:border-[#333336]">
          <button
            type="button"
            onClick={() => setMode('agent')}
            className={`px-3 py-1 text-xs font-medium rounded-[6px] transition-all duration-150 flex items-center gap-1.5 cursor-pointer ${
              mode === 'agent'
                ? 'bg-[#FFFFFF] dark:bg-[#2A2A2D] text-[#111827] dark:text-[#F2F2F2] font-semibold shadow-xs'
                : 'text-[#6B7280] dark:text-[#9B9B9F] hover:text-[#111827] dark:hover:text-[#F2F2F2]'
            }`}
          >
            <span>Agent</span>
          </button>
          <button
            type="button"
            onClick={() => setMode('editor')}
            className={`px-3 py-1 text-xs font-medium rounded-[6px] transition-all duration-150 flex items-center gap-1.5 cursor-pointer ${
              mode === 'editor'
                ? 'bg-[#FFFFFF] dark:bg-[#2A2A2D] text-[#111827] dark:text-[#F2F2F2] font-semibold shadow-xs'
                : 'text-[#6B7280] dark:text-[#9B9B9F] hover:text-[#111827] dark:hover:text-[#F2F2F2]'
            }`}
          >
            <span>Editor</span>
          </button>
        </div>

        {/* Back / Forward History Navigation */}
        <div className="flex items-center text-[#9CA3AF] dark:text-[#6B6B70]">
          <button
            type="button"
            onClick={goBack}
            disabled={!canGoBack}
            className={`p-1 rounded-md transition-colors cursor-pointer ${
              canGoBack
                ? 'hover:text-[#111827] dark:hover:text-[#F2F2F2] hover:bg-[#F3F4F6] dark:hover:bg-[#2A2A2D]'
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
                ? 'hover:text-[#111827] dark:hover:text-[#F2F2F2] hover:bg-[#F3F4F6] dark:hover:bg-[#2A2A2D]'
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
          className="flex-1 h-[28px] px-3 bg-[#F9FAFB] dark:bg-[#1E1E20] hover:bg-[#F3F4F6] dark:hover:bg-[#2A2A2D] border border-[#E5E7EB] dark:border-[#333336] rounded-[8px] flex items-center justify-between text-xs text-[#6B7280] dark:text-[#9B9B9F] transition-all cursor-pointer group shadow-2xs"
        >
          <div className="flex items-center gap-2 truncate">
            <Search className="w-3.5 h-3.5 text-[#9CA3AF] dark:text-[#6B6B70] group-hover:text-[#2563EB] dark:group-hover:text-[#4ADE80] transition-colors shrink-0" />
            <span className="truncate font-medium text-[#111827] dark:text-[#F2F2F2]">
              {mode === 'agent'
                ? (activeSession?.title || 'New Task')
                : mode === 'settings'
                ? 'Settings & Preferences'
                : [workspaceName, currentFileName].filter(Boolean).join(' - ') || 'Standalone Code Editor'}
            </span>
          </div>

          <div className="flex items-center gap-1.5 shrink-0">
            <kbd className="text-[10px] px-1.5 py-0.2 rounded bg-[#FFFFFF] dark:bg-[#2A2A2D] border border-[#E5E7EB] dark:border-[#333336] text-[#6B7280] dark:text-[#9B9B9F] font-mono shadow-2xs">
              ⌘ P
            </kbd>
            <Sparkles className="w-3.5 h-3.5 text-[#7C3AED] dark:text-[#9B9B9F] group-hover:text-[#2563EB] dark:group-hover:text-[#4ADE80] transition-colors" />
          </div>
        </button>
      </div>

      {/* Right Area: User Profile Avatar / Settings Trigger */}
      <div className="flex items-center">
        <div className="relative">
          <button
            type="button"
            onClick={() => openSettingsTab('model')}
            className={`w-7 h-7 rounded-full flex items-center justify-center text-[11px] font-bold shadow-xs transition-all cursor-pointer ${
              mode === 'settings'
                ? 'bg-[#16A34A] dark:bg-[#4ADE80] text-white dark:text-black ring-2 ring-[#4ADE80]/30'
                : 'bg-[#E5E7EB] dark:bg-[#2A2A2D] border border-[#E5E7EB] dark:border-[#333336] text-[#111827] dark:text-[#F2F2F2] hover:bg-[#D1D5DB] dark:hover:bg-[#333336]'
            }`}
            title="Settings (Models, Tools, MCP)"
          >
            LI
          </button>
          <span className="absolute bottom-0 right-0 w-2 h-2 rounded-full bg-[#16A34A] dark:bg-[#4ADE80] border border-white dark:border-[#161617]" />
        </div>
      </div>

    </header>
  );
};
