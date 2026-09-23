import React from 'react';
import { PanelLeftOpen } from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';
import { AgentSidebar } from './AgentSidebar';
import { AgentHomeView } from './AgentHomeView';
import { AgentActiveSessionView } from './AgentActiveSessionView';

export const AgentContainer: React.FC = () => {
  const {
    activeSession,
    activeSessionId,
    isLeftSidebarOpen,
    setIsLeftSidebarOpen,
    isRightActionDrawerOpen,
    setIsRightActionDrawerOpen
  } = useWorkspace();

  return (
    <div className="dark relative flex-1 flex h-full w-full overflow-hidden bg-background">
      {/* Collapsed rail (ZCode WorkspaceSidebarCollapsedRail): logo strip reopens the sidebar */}
      {!isLeftSidebarOpen && (
        <aside
          data-testid="workspace-sidebar-rail"
          className="flex h-full w-12 shrink-0 flex-col items-center overflow-hidden border-r border-border bg-sidebar select-none"
        >
          <div className="titlebar-drag flex h-12 w-full shrink-0 items-start justify-start pl-[9px] pt-[18px]">
            <button
              type="button"
              aria-label="Show sidebar"
              title="Show sidebar (⌘B)"
              onClick={() => setIsLeftSidebarOpen(true)}
              className="flex size-6 items-center justify-center rounded-md text-foreground-subtle transition-colors hover:bg-surface-hover hover:text-foreground"
            >
              <PanelLeftOpen className="size-4" />
            </button>
          </div>
        </aside>
      )}

      {/* Primary Sidebar (Session history per project) */}
      {isLeftSidebarOpen && <AgentSidebar />}

      {/* Main Agent Area: New Task View or Active Session View */}
      <div className="flex-1 flex flex-col h-full overflow-hidden relative">
        {/* Traffic-light safe drag corner when the header hosts the top edge */}
        {!isLeftSidebarOpen && (
          <div className="titlebar-drag absolute left-0 top-0 z-40 h-12 w-[76px]" />
        )}
        {activeSessionId && activeSession ? (
          <AgentActiveSessionView />
        ) : (
          <AgentHomeView
            onToggleRightSidebar={() => setIsRightActionDrawerOpen(prev => !prev)}
            onOpenTerminal={() => setIsRightActionDrawerOpen(prev => !prev)}
          />
        )}
      </div>
    </div>
  );
};
