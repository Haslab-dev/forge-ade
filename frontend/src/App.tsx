import React from 'react';
import { WorkspaceProvider, useWorkspace } from './stores/workspaceStore';
import { TitleBar } from './components/shell/TitleBar';
import { StatusBar } from './components/shell/StatusBar';
import { AgentHomeView } from './components/agent/AgentHomeView';
import { AgentActiveSessionView } from './components/agent/AgentActiveSessionView';
import { EditorView } from './components/editor/EditorView';
import { CommandPaletteModal } from './components/modals/CommandPaletteModal';
import { OpenFolderModal } from './components/modals/OpenFolderModal';

const AppContent: React.FC = () => {
  const { mode, activeSession, isFolderModalOpen, setIsFolderModalOpen } = useWorkspace();

  return (
    <div className="flex flex-col h-screen w-screen bg-[#ffffff] dark:bg-[#181818] overflow-hidden select-none font-sans">
      {/* Global Title Bar / Toolbar */}
      <TitleBar />

      {/* Dynamic Viewport (Persistent Mounting to preserve PTY shell sessions & agent state) */}
      <main className="flex-1 flex overflow-hidden relative">
        {/* Agent Mode View */}
        <div className={`flex-1 flex overflow-hidden ${mode === 'agent' ? 'flex' : 'hidden'}`}>
          {activeSession && activeSession.messages.length > 0 ? (
            <AgentActiveSessionView />
          ) : (
            <AgentHomeView />
          )}
        </div>

        {/* Editor Mode View (Kept mounted so xterm/node-pty terminal, tabs, and buffers never restart) */}
        <div className={`flex-1 flex overflow-hidden ${mode === 'editor' ? 'flex' : 'hidden'}`}>
          <EditorView />
        </div>
      </main>

      {/* Global Status Bar */}
      <StatusBar />

      {/* Quick search/command palette modal */}
      <CommandPaletteModal />

      {/* Workspace Folder Picker Modal */}
      <OpenFolderModal
        isOpen={isFolderModalOpen}
        onClose={() => setIsFolderModalOpen(false)}
      />
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

