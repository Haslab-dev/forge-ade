import React from 'react';
import { useWorkspace } from '../../stores/workspaceStore';
import { AgentSidebar } from './AgentSidebar';
import { AgentHomeView } from './AgentHomeView';
import { AgentActiveSessionView } from './AgentActiveSessionView';
import { PanelResizeHandle } from './PanelResizeHandle';
import { AutomationsView } from '../automations/AutomationsView';
import { TerminalSessionSurface } from '../session/TerminalSessionSurface';
import { AgentRightSidebar } from './AgentRightSidebar';

export const AgentContainer: React.FC = () => {
  const {
    activeSession,
    activeSessionId,
    mode,
    isLeftSidebarOpen,
    isRightActionDrawerOpen,
    setIsRightActionDrawerOpen,
    leftSidebarWidth,
    setLeftSidebarWidth,
    rightPaneWidth,
    setRightPaneWidth
  } = useWorkspace();

  const isTerminalMode = mode === 'terminal';
  const isSession = Boolean(activeSessionId && activeSession);

  return (
    <div className="dark relative flex-1 flex flex-col h-full w-full overflow-hidden bg-background">
      {/* Body: sidebar + main area */}
      <div className="flex flex-1 min-h-0">
        {/* Primary Sidebar (mode-aware: terminal sessions or agent tasks) +
            ZCode-style drag separator. The handle overlaps the sidebar's
            border-r (-ml-1) so the divider reads as a single hairline. */}
        {isLeftSidebarOpen && (
          <>
            <AgentSidebar />
            <PanelResizeHandle
              edge="right"
              ariaLabel="Resize sidebar"
              getStartWidth={() => leftSidebarWidth}
              onResize={setLeftSidebarWidth}
              min={240}
              max={() => Math.max(240, Math.floor(window.innerWidth * 0.5))}
              className="-ml-1"
            />
          </>
        )}

        {/* Main Area: Terminal Session surface (default) or Agent UI content */}
        <div className="flex-1 flex flex-col h-full overflow-hidden relative min-w-0">
          {isTerminalMode ? (
            <TerminalSessionSurface />
          ) : mode === 'automations' ? (
            <AutomationsView />
          ) : isSession ? (
            <AgentActiveSessionView />
          ) : (
            <AgentHomeView
              onToggleRightSidebar={() => setIsRightActionDrawerOpen(prev => !prev)}
              onOpenTerminal={() => setIsRightActionDrawerOpen(prev => !prev)}
            />
          )}
        </div>

        {/* Right side pane for Terminal Session mode (matches Agent UI mode) */}
        {isTerminalMode && isRightActionDrawerOpen && (
          <>
            <PanelResizeHandle
              edge="left"
              ariaLabel="Resize side panel"
              getStartWidth={() => rightPaneWidth}
              onResize={setRightPaneWidth}
              min={360}
              max={() => Math.max(360, Math.floor(window.innerWidth * 0.6))}
              className="-mr-1"
            />
            <AgentRightSidebar initialTab={null} />
          </>
        )}
      </div>
    </div>
  );
};
