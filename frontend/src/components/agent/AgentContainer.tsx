import React from 'react';
import { useWorkspace } from '../../stores/workspaceStore';
import { AgentHomeView } from './AgentHomeView';
import { AgentActiveSessionView } from './AgentActiveSessionView';
import { PanelResizeHandle } from './PanelResizeHandle';
import { TerminalSessionSurface } from '../session/TerminalSessionSurface';
import { AgentRightSidebar } from './AgentRightSidebar';

import { WorkflowBoardView } from './WorkflowBoardView';
import { GitClientView } from '../git/GitClientView';

/**
 * Main workspace content area (the full-height glass sidebar lives at the App
 * root so it spans the window, ZCode-style). This component owns the center
 * surface plus the right side pane.
 */
export const AgentContainer: React.FC = () => {
  const {
    activeSession,
    activeSessionId,
    mode,
    activeTaskKind,
    isRightActionDrawerOpen,
    setIsRightActionDrawerOpen,
    rightPaneWidth,
    setRightPaneWidth,
    agentViewMode,
    setAgentViewMode
  } = useWorkspace();

  const isCliTask = activeTaskKind === 'cli';
  const isSession = Boolean(activeSessionId && activeSession);

  return (
    // Row layout: main surface BESIDE the CLI right pane — a column parent
    // pushed the pane below the fold where overflow-hidden hid it entirely.
    <div className="relative flex-1 flex h-full w-full overflow-hidden bg-background">
      {/* Main Area: Workflow Kanban Board, Git Client, CLI terminal, or Agent session/chat */}
      <div className="flex flex-1 flex-col overflow-hidden relative min-w-0">
        {agentViewMode === 'board' ? (
          <WorkflowBoardView />
        ) : agentViewMode === 'git' ? (
          <GitClientView onClose={() => setAgentViewMode('session')} />
        ) : isCliTask ? (
          <TerminalSessionSurface />
        ) : isSession ? (
          <AgentActiveSessionView />
        ) : (
          <AgentHomeView
            onToggleRightSidebar={() => setIsRightActionDrawerOpen(prev => !prev)}
            onOpenTerminal={() => setIsRightActionDrawerOpen(prev => !prev)}
          />
        )}
      </div>

      {/* Right side pane for CLI tasks — opens on Review (git changes) so the
          pane is never an empty void. Kept mounted while a CLI task is active
          and CSS-hidden when the drawer closes so open tabs persist (the pane
          must survive close/reopen — tabs only die via their X button). */}
      {isCliTask && (
        <div className={`h-full flex shrink-0${isRightActionDrawerOpen ? '' : ' hidden'}`}>
          <PanelResizeHandle
            edge="left"
            ariaLabel="Resize side panel"
            getStartWidth={() => rightPaneWidth}
            onResize={setRightPaneWidth}
            min={360}
            max={() => Math.max(360, Math.floor(window.innerWidth * 0.6))}
            className="-mr-1"
          />
          <AgentRightSidebar initialTab="review" />
        </div>
      )}
    </div>
  );
};
