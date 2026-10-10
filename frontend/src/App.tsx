import React, { Suspense, lazy, useEffect, useState } from 'react';
import { WorkspaceProvider, useWorkspace } from './stores/workspaceStore';
import { useSessionStore } from './stores/sessionStore';
import { WorkspaceHeader } from './components/agent/WorkspaceHeader';
import { StatusBar } from './components/shell/StatusBar';
import { AgentSidebar } from './components/agent/AgentSidebar';
import { PanelResizeHandle } from './components/agent/PanelResizeHandle';
import { AgentContainer } from './components/agent/AgentContainer';
import { CommandPaletteModal } from './components/modals/CommandPaletteModal';
import { NewTaskModal } from './components/session/NewTaskModal';
import { SSHConnectModal } from './components/modals/SSHConnectModal';

// The editor carries the ~1MB CodeMirror chunk and Settings is a 3k-line
// surface — neither is needed at startup on the agent surface. Both lazy-load
// on FIRST VISIT and then stay mounted (persistent-mount semantics that keep
// PTY scrollback, split panes and tab state alive across surface switches).
const EditorView = lazy(() =>
  import('./components/editor/EditorView').then(m => ({ default: m.EditorView }))
);
const SettingsScreen = lazy(() =>
  import('./components/settings/SettingsScreen').then(m => ({ default: m.SettingsScreen }))
);

const SurfaceFallback: React.FC = () => (
  <div className="flex-1 flex items-center justify-center bg-app">
    <span className="text-ui-sm text-foreground-subtle animate-pulse">Loading…</span>
  </div>
);

/** Header left inset clearing the macOS traffic-lights corner (only needed
    when no full-height sidebar carries the window controls). */
const HEADER_TRAFFIC_LIGHTS_INSET_PX = 116;

const AppContent: React.FC = () => {
  const {
    mode,
    activeTaskKind,
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
    leftSidebarWidth,
    setLeftSidebarWidth,
    openTabs,
    activeTabId,
    selectedFile,
    isSSHModalOpen,
    setIsSSHModalOpen
  } = useWorkspace();
  const { sessions: terminalSessions, activeSessionId: activeTerminalSessionId } = useSessionStore();

  // First-visit lazy mount: the chunk loads when the surface is first opened;
  // afterwards the surface stays mounted (hidden) like before.
  const [editorEverOpened, setEditorEverOpened] = useState(() => mode === 'editor');
  const [settingsEverOpened, setSettingsEverOpened] = useState(() => mode === 'settings');
  useEffect(() => {
    if (mode === 'editor') setEditorEverOpened(true);
    if (mode === 'settings') setSettingsEverOpened(true);
  }, [mode]);

  const isCliTask = activeTaskKind === 'cli';
  const isSession = Boolean(activeSessionId && activeSession);
  const activeTerminal = isCliTask
    ? terminalSessions.find(s => s.id === activeTerminalSessionId)
    : undefined;
  const projectName = (activeWorkspacePath || '').split('/').filter(Boolean).pop() || 'forge-ade';

  const activeTab = openTabs.find(t => t.id === activeTabId);
  const currentFileName = activeTab?.fileName || selectedFile?.name || '';

  const headerTitle = mode === 'editor'
    ? currentFileName
    : isCliTask
      ? (activeTerminal ? (activeTerminal.title || activeTerminal.agentName || projectName) : '')
      : isSession
        ? (activeSession?.title || 'Active Task Session')
        : '';

  // ZCode parity: in the workspace surface the sidebar is a full-height glass
  // column that carries the macOS traffic lights; the header then sits beside
  // it (no inset). The editor keeps its own internal sidebar column, and
  // Settings ships its own — in both, the header/strip clears the lights.
  const workspaceSidebarVisible = mode !== 'editor' && mode !== 'settings' && isLeftSidebarOpen;

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
    <div className="app-ambient flex h-screen w-screen overflow-hidden select-none font-sans transition-colors">
      {/* Full-height glass sidebar (workspace surface) — window controls strip on top */}
      {workspaceSidebarVisible && (
        <>
          <AgentSidebar />
          <PanelResizeHandle
            edge="right"
            ariaLabel="Resize sidebar"
            getStartWidth={() => leftSidebarWidth}
            onResize={setLeftSidebarWidth}
            min={240}
            max={() => Math.max(240, Math.floor(window.innerWidth * 0.5))}
            className="-mr-1"
          />
        </>
      )}

      {/* Main column: header + content */}
      <div className="flex min-w-0 flex-1 flex-col overflow-hidden">
        {/* Single Unified Header for both Agent and Editor */}
        {mode !== 'settings' && (
          <WorkspaceHeader
            variant={
              mode === 'editor'
                ? (currentFileName ? 'task' : 'draft')
                : isCliTask
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
            leftSafeInset={workspaceSidebarVisible ? 0 : HEADER_TRAFFIC_LIGHTS_INSET_PX}
            hideSidebarToggle={workspaceSidebarVisible}
          />
        )}

        {/* Dynamic Viewport (Persistent Mounting to preserve PTY shell sessions, agent sessions & state) */}
        <main className="flex-1 flex overflow-hidden relative">
          <div className={`flex-1 flex overflow-hidden ${mode === 'agent' || mode === 'automations' ? 'flex' : 'hidden'}`}>
            <AgentContainer />
          </div>

          <div className={`flex-1 flex overflow-hidden ${mode === 'editor' ? 'flex' : 'hidden'}`}>
            {editorEverOpened && (
              <Suspense fallback={<SurfaceFallback />}>
                <EditorView />
              </Suspense>
            )}
          </div>

          <div className={`flex-1 flex overflow-hidden ${mode === 'settings' ? 'flex' : 'hidden'}`}>
            {settingsEverOpened && (
              <Suspense fallback={<SurfaceFallback />}>
                <SettingsScreen />
              </Suspense>
            )}
          </div>
        </main>

        {/* Global Status Bar (Only active in editor mode) */}
        {mode === 'editor' && <StatusBar />}
      </div>

      {/* Quick search/command palette modal */}
      <CommandPaletteModal />

      {/* Unified New Task flow (ForgeADE chat or an agent CLI) — mounted once
          at the root so every entry point works from any surface. */}
      <NewTaskModal />

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
