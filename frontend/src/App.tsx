import React from 'react';
import { WorkspaceProvider, useWorkspace } from './stores/workspaceStore';
import { TitleBar } from './components/shell/TitleBar';
import { StatusBar } from './components/shell/StatusBar';
import { AgentContainer } from './components/agent/AgentContainer';
import { EditorView } from './components/editor/EditorView';
import { SettingsScreen } from './components/settings/SettingsScreen';
import { CommandPaletteModal } from './components/modals/CommandPaletteModal';
import { OpenFolderModal } from './components/modals/OpenFolderModal';
import { PluginMarketplaceScreen } from './components/agent/PluginMarketplaceScreen';

const AppContent: React.FC = () => {
  const { mode, isFolderModalOpen, setIsFolderModalOpen } = useWorkspace();

  return (
    <div className="flex flex-col h-screen w-screen bg-background overflow-hidden select-none font-sans transition-colors">
      {/* Global Title Bar / Toolbar — agent mode uses the workspace header instead */}
      {mode !== 'agent' && <TitleBar />}

      {/* Dynamic Viewport (Persistent Mounting to preserve PTY shell sessions & agent state) */}
      <main className="flex-1 flex overflow-hidden relative">
        {/* Agent Mode View (Unified workspace layout with session history and active chat) */}
        <div className={`flex-1 flex overflow-hidden ${mode === 'agent' ? 'flex' : 'hidden'}`}>
          <AgentContainer />
        </div>

        {/* Editor Mode View (Kept mounted so xterm/node-pty terminal, tabs, and buffers never restart) */}
        <div className={`flex-1 flex overflow-hidden ${mode === 'editor' ? 'flex' : 'hidden'}`}>
          <EditorView />
        </div>

        {/* Settings Mode View (Dedicated screen) */}
        <div className={`flex-1 flex overflow-hidden ${mode === 'settings' ? 'flex' : 'hidden'}`}>
          <SettingsScreen />
        </div>

        {/* Plugin Marketplace screen */}
        <div className={`flex-1 flex overflow-hidden ${mode === 'marketplace' ? 'flex' : 'hidden'}`}>
          <PluginMarketplaceScreen />
        </div>
      </main>

      {/* Global Status Bar (Only active in editor mode) */}
      {mode === 'editor' && <StatusBar />}

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

