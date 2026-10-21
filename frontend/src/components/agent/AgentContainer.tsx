import React from 'react';
import { useWorkspace } from '../../stores/workspaceStore';
import { AgentHomeView } from './AgentHomeView';
import { AgentActiveSessionView } from './AgentActiveSessionView';
import { TerminalSessionSurface } from '../session/TerminalSessionSurface';
import { WorkflowBoardView } from './WorkflowBoardView';
import { GitClientView } from '../git/GitClientView';

/**
 * Main workspace content area (the full-height glass sidebar lives at the App
 * root so it spans the window, ZCode-style; the global persistent right sidebar
 * lives at App.tsx level so it is preserved across Agent and Editor modes).
 */
export const AgentContainer: React.FC = () => {
  const {
    activeSession,
    activeSessionId,
    activeTaskKind,
    setIsRightActionDrawerOpen,
    agentViewMode,
    setAgentViewMode
  } = useWorkspace();

  const isCliTask = activeTaskKind === 'cli';
  const isSession = Boolean(activeSessionId && activeSession);

  return (
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
            onOpenTerminal={() => {
              setIsRightActionDrawerOpen(true);
              window.dispatchEvent(new CustomEvent('forge:open-sidepane-tab', { detail: { tab: 'terminal' } }));
            }}
          />
        )}
      </div>
    </div>
  );
};
