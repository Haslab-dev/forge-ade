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
    <header className="h-[44px] min-h-[44px] bg-background border-b border-border flex items-center justify-between px-3 select-none z-30 transition-colors ">
      
      {/* Left Area: Sidebar Toggle + Mode Switcher + Nav */}
      <div className="flex items-center gap-2">
        {/* Toggle Left Sidebar Button (both in Agent and Editor) */}
        {mode !== 'settings' && (
          <button
            type="button"
            onClick={() => setIsLeftSidebarOpen(prev => !prev)}
            className={`p-1.5 rounded-[6px] transition-colors cursor-pointer ${
              isLeftSidebarOpen
                ? 'text-foreground hover:bg-surface-hover'
                : 'text-foreground-subtle hover:bg-surface-hover hover:text-foreground'
            }`}
            title={isLeftSidebarOpen ? 'Hide Left Sidebar (⌘B)' : 'Show Left Sidebar (⌘B)'}
          >
            <PanelLeft className="w-4 h-4" />
          </button>
        )}

        {/* Mode Segment Switcher: Agent | Editor */}
        <div className="flex items-center bg-surface-hover p-0.5 rounded-[8px] border border-border">
          <button
            type="button"
            onClick={() => setMode('agent')}
            className={`px-3 py-1 text-xs font-medium rounded-[6px] transition-all duration-150 flex items-center gap-1.5 cursor-pointer ${
              mode === 'agent'
                ? 'bg-card dark:bg-surface-hover text-foreground font-semibold shadow-xs'
                : 'text-foreground-subtle hover:text-foreground'
            }`}
          >
            <span>Agent</span>
          </button>
          <button
            type="button"
            onClick={() => setMode('editor')}
            className={`px-3 py-1 text-xs font-medium rounded-[6px] transition-all duration-150 flex items-center gap-1.5 cursor-pointer ${
              mode === 'editor'
                ? 'bg-card dark:bg-surface-hover text-foreground font-semibold shadow-xs'
                : 'text-foreground-subtle hover:text-foreground'
            }`}
          >
            <span>Editor</span>
          </button>
        </div>

        {/* Back / Forward History Navigation */}
        <div className="flex items-center text-foreground-subtlest">
          <button
            type="button"
            onClick={goBack}
            disabled={!canGoBack}
            className={`p-1 rounded-md transition-colors cursor-pointer ${
              canGoBack
                ? 'hover:text-foreground hover:bg-surface-hover'
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
                ? 'hover:text-foreground hover:bg-surface-hover'
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
          className="flex-1 h-[28px] px-3 bg-surface hover:bg-surface-hover border border-border rounded-[8px] flex items-center justify-between text-xs text-foreground-subtle transition-all cursor-pointer group shadow-2xs"
        >
          <div className="flex items-center gap-2 truncate">
            <Search className="w-3.5 h-3.5 text-foreground-subtlest group-hover:text-primary dark:group-hover:text-success transition-colors shrink-0" />
            <span className="truncate font-medium text-foreground">
              {mode === 'agent'
                ? (activeSession?.title || 'New Task')
                : mode === 'settings'
                ? 'Settings & Preferences'
                : [workspaceName, currentFileName].filter(Boolean).join(' - ') || 'Standalone Code Editor'}
            </span>
          </div>

          <div className="flex items-center gap-1.5 shrink-0">
            <kbd className="text-[10px] px-1.5 py-0.2 rounded bg-card dark:bg-surface-hover border border-border text-foreground-subtle font-mono shadow-2xs">
              ⌘ P
            </kbd>
            <Sparkles className="w-3.5 h-3.5 text-primary dark:text-foreground-subtle group-hover:text-primary dark:group-hover:text-success transition-colors" />
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
                ? 'bg-success text-white dark:text-black ring-2 ring-success/30'
                : 'bg-border border border-border text-foreground hover:bg-[#D1D5DB] dark:hover:bg-surface-hover'
            }`}
            title="Settings (Models, Tools, MCP)"
          >
            LI
          </button>
          <span className="absolute bottom-0 right-0 w-2 h-2 rounded-full bg-success border border-white dark:border-border" />
        </div>
      </div>

    </header>
  );
};
