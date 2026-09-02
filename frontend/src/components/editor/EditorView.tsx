import React from 'react';
import { ActivityBar } from './ActivityBar';
import { FileTree } from './FileTree';
import { CodeEditorPane } from './CodeEditorPane';
import { MarkdownPreview } from './MarkdownPreview';
import { DevinSettingsTab } from './DevinSettingsTab';
import { ImageViewer } from './ImageViewer';
import { PdfViewer } from './PdfViewer';
import { XTermTerminal } from '../terminal/XTermTerminal';
import { useWorkspace } from '../../stores/workspaceStore';

import { DiffViewer } from '../diff/DiffViewer';

const IMAGE_EXTENSIONS = ['.png', '.jpg', '.jpeg', '.gif', '.bmp', '.webp', '.ico', '.svg'];
const PDF_EXTENSIONS = ['.pdf'];

export const EditorView: React.FC = () => {
  const { isSplitEditor, activeTabId, openTabs, diffs, activeDiff } = useWorkspace();

  const activeTab = openTabs.find(t => t.id === activeTabId);
  const targetDiff = activeTab?.diffId ? diffs.find(d => d.id === activeTab.diffId) : activeDiff;

  const isImageFile = activeTab?.fileName ? IMAGE_EXTENSIONS.some(ext => activeTab.fileName.toLowerCase().endsWith(ext)) : false;
  const isPdfFile = activeTab?.fileName ? PDF_EXTENSIONS.some(ext => activeTab.fileName.toLowerCase().endsWith(ext)) : false;

  return (
    <div className="flex-1 flex flex-col h-[calc(100vh-68px)] overflow-hidden bg-white dark:bg-[#1e1e1e] select-none">
      
      {/* Top Main Work Area */}
      <div className="flex-1 flex overflow-hidden">
        {/* Leftmost Activity Bar */}
        <ActivityBar />

        {/* Project File Tree Explorer */}
        <FileTree />

        {/* Center Main Editor Pane */}
        <div className="flex-1 flex overflow-hidden bg-white dark:bg-[#1e1e1e]">
          {activeTab?.type === 'settings' ? (
            <DevinSettingsTab />
          ) : activeTab?.type === 'diff' && targetDiff ? (
            <div className="flex-1 p-4 overflow-hidden bg-[#f8fafc] dark:bg-[#141414]">
              <DiffViewer diff={targetDiff} />
            </div>
          ) : isImageFile && activeTab?.filePath ? (
            <ImageViewer filePath={activeTab.filePath} fileName={activeTab.fileName} />
          ) : isPdfFile && activeTab?.filePath ? (
            <PdfViewer filePath={activeTab.filePath} fileName={activeTab.fileName} />
          ) : isSplitEditor ? (
            <>
              <CodeEditorPane />
              <MarkdownPreview />
            </>
          ) : (
            activeTab?.type === 'preview' ? (
              <MarkdownPreview />
            ) : (
              <CodeEditorPane />
            )
          )}
        </div>
      </div>

      {/* Bottom Integrated XTerm Terminal Drawer */}
      <XTermTerminal />

    </div>
  );
};
