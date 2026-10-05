import React, { useEffect } from 'react';
import { WorkspaceProvider, useWorkspace } from './stores/workspaceStore';
import { useSessionStore } from './stores/sessionStore';
import { WorkspaceHeader } from './components/agent/WorkspaceHeader';
import { StatusBar } from './components/shell/StatusBar';
import { AgentContainer } from './components/agent/AgentContainer';
import { EditorView } from './components/editor/EditorView';
import { SettingsScreen } from './components/settings/SettingsScreen';
import { CommandPaletteModal } from './components/modals/CommandPaletteModal';
import { SSHConnectModal } from './components/modals/SSHConnectModal';

/** Header left inset clearing the macOS traffic-lights corner. */
const HEADER_TRAFFIC_LIGHTS_INSET_PX = 116;

const AppContent: React.FC = () => {
  const {
    mode,
    activeSession,
    activeSessionId,
    activeWorkspacePath,
    gitBranch,
    isLeftSidebarOpen,
    setIsLeftSidebarOpen,
    isRightActionDrawerOpen,
    setIsRightActionDrawerOpen,
    isBottomTerminalOpen,
    setIsBottomTerminalOpen,
    openTabs,
    activeTabId,
    selectedFile,
    isSSHModalOpen,
    setIsSSHModalOpen
  } = useWorkspace();
  const { sessions: terminalSessions, activeSessionId: activeTerminalSessionId } = useSessionStore();

  const isTerminalMode = mode === 'terminal';
  const isSession = Boolean(activeSessionId && activeSession);
  const activeTerminal = isTerminalMode
    ? terminalSessions.find(s => s.id === activeTerminalSessionId)
    : undefined;
  const projectName = (activeWorkspacePath || '').split('/').filter(Boolean).pop() || 'forge-ade';

  const activeTab = openTabs.find(t => t.id === activeTabId);
  const currentFileName = activeTab?.fileName || selectedFile?.name || '';

  const headerTitle = mode === 'editor'
    ? currentFileName
    : isTerminalMode
      ? (activeTerminal ? (activeTerminal.title || activeTerminal.agentName || projectName) : '')
      : isSession
        ? (activeSession?.title || 'Active Task Session')
        : '';

  // ⌘B / Ctrl+B toggles the left sidebar from anywhere in the app
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'b') {
        e.preventDefault();
        setIsLeftSidebarOpen(prev => !prev);
      }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  }, [setIsLeftSidebarOpen]);

  return (
    <div className="flex flex-col h-screen w-screen bg-background overflow-hidden select-none font-sans transition-colors">
      {/* Single Unified Header for both Agent and Editor */}
      {mode !== 'settings' && (
        <WorkspaceHeader
          variant={
            mode === 'editor'
              ? (currentFileName ? 'task' : 'draft')
              : isTerminalMode
                ? (activeTerminal ? 'task' : 'draft')
                : isSession
                  ? 'task'
                  : 'draft'
          }
          title={headerTitle}
          projectName={projectName}
          gitBranch={gitBranch || 'main'}
          isSidePaneOpen={isRightActionDrawerOpen}
          onToggleSidePane={() => setIsRightActionDrawerOpen(prev => !prev)}
          onToggleTerminal={() => setIsBottomTerminalOpen(prev => !prev)}
          sidebarOpen={isLeftSidebarOpen}
          onToggleSidebar={() => setIsLeftSidebarOpen(prev => !prev)}
          leftSafeInset={HEADER_TRAFFIC_LIGHTS_INSET_PX}
        />
      )}

      {/* Dynamic Viewport (Persistent Mounting to preserve PTY shell sessions, agent sessions & state) */}
      <main className="flex-1 flex overflow-hidden relative">
        <div className={`flex-1 flex overflow-hidden ${mode === 'terminal' || mode === 'agent' || mode === 'automations' ? 'flex' : 'hidden'}`}>
          <AgentContainer />
        </div>

        <div className={`flex-1 flex overflow-hidden ${mode === 'editor' ? 'flex' : 'hidden'}`}>
          <EditorView />
        </div>

        <div className={`flex-1 flex overflow-hidden ${mode === 'settings' ? 'flex' : 'hidden'}`}>
          <SettingsScreen />
        </div>
      </main>

      {/* Global Status Bar (Only active in editor mode) */}
      {mode === 'editor' && <StatusBar />}

      {/* Quick search/command palette modal */}
      <CommandPaletteModal />

      {/* SSH Remote Connect Modal */}
      <SSHConnectModal isOpen={isSSHModalOpen} onClose={() => setIsSSHModalOpen(false)} />
    </div>
  );
};

export default function App() {
  return (
    <WorkspaceProvider>
      <AppContent />
    </WorkspaceProvider>
  );
}
