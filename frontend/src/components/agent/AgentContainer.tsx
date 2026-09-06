import React from 'react';
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
    <div className="flex-1 flex h-full w-full overflow-hidden bg-[#181819] relative">
      {/* Primary Sidebar (Session history per project) */}
      {isLeftSidebarOpen && (
        <AgentSidebar onCollapse={() => setIsLeftSidebarOpen(false)} />
      )}

      {/* Main Agent Area: New Task View or Active Session View */}
      <div className="flex-1 flex flex-col h-full overflow-hidden">
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
