import React, { useState } from 'react';
import { ActivityBar } from './ActivityBar';
import { FileTree } from './FileTree';
import { CodeEditorPane } from './CodeEditorPane';
import { MarkdownPreview } from './MarkdownPreview';
import { ForgeSettingsTab } from './ForgeSettingsTab';
import { ImagePreview } from './ImagePreview';
import { PdfViewer } from './PdfViewer';
import { useWorkspace } from '../../stores/workspaceStore';
import { DiffViewer } from '../diff/DiffViewer';
import { GitGraphPane } from './GitGraphPane';

const IMAGE_EXTENSIONS = ['.png', '.jpg', '.jpeg', '.gif', '.bmp', '.webp', '.ico', '.svg'];
const PDF_EXTENSIONS = ['.pdf'];

export const EditorView: React.FC = () => {
  const { isSplitEditor, setIsSplitEditor, activeTabId, openTabs, diffs, activeDiff } = useWorkspace();
  const [isPreviewActive, setIsPreviewActive] = useState(false);

  const activeTab = openTabs.find(t => t.id === activeTabId) || openTabs[0];
  const targetDiff = activeTab?.diffId ? diffs.find(d => d.id === activeTab.diffId) : activeDiff;

  const isImageFile = activeTab?.fileName ? IMAGE_EXTENSIONS.some(ext => activeTab.fileName.toLowerCase().endsWith(ext)) : false;
  const isPdfFile = activeTab?.fileName ? PDF_EXTENSIONS.some(ext => activeTab.fileName.toLowerCase().endsWith(ext)) : false;

  return (
    <div className="flex-1 flex flex-col h-[calc(100vh-66px)] overflow-hidden bg-white dark:bg-[#181818] select-none font-sans">
      
      {/* Top Main Work Area */}
      <div className="flex-1 flex overflow-hidden">
        {/* Core Activity Bar (Editor, Search, Git, Settings) */}
        <ActivityBar />

        {/* Project File Tree & Activity Panel */}
        <FileTree />

        {/* Main Editor Panes Grid */}
        <div className="flex-1 flex overflow-hidden bg-white dark:bg-[#181818]">
          {activeTab?.type === 'settings' ? (
            <ForgeSettingsTab />
          ) : activeTab?.type === 'diff' && targetDiff ? (
            <div className="flex-1 p-4 overflow-hidden bg-[#f8fafc] dark:bg-[#141414]">
              <DiffViewer diff={targetDiff} />
            </div>
          ) : activeTab?.type === 'git-graph' ? (
            <GitGraphPane />
          ) : isImageFile && activeTab?.filePath ? (
            /* Full-width image pane (base64 via Wails — the old /api endpoint doesn't exist) */
            <ImagePreview filePath={activeTab.filePath} fileName={activeTab.fileName} />
          ) : isPdfFile && activeTab?.filePath ? (
            <PdfViewer filePath={activeTab.filePath} fileName={activeTab.fileName} />
          ) : isSplitEditor ? (
            /* Split: one pane per open tab (up to 3); with a single tab both
               sides show the same document, VS Code style. */
            <div className="flex-1 flex overflow-hidden divide-x divide-[#e5e7eb] dark:divide-[#282828]">
              {(openTabs.length >= 2 ? openTabs.slice(0, 3) : [activeTab, activeTab]).map((tab, idx) => (
                <div key={idx} className="flex-1 flex overflow-hidden min-w-[280px]">
                  <CodeEditorPane
                    tabId={tab?.id}
                    onSplitRight={() => setIsSplitEditor(prev => !prev)}
                    onTogglePreview={() => setIsPreviewActive(prev => !prev)}
                    isPreview={isPreviewActive && idx === 1}
                  />
                </div>
              ))}
            </div>
          ) : activeTab?.type === 'preview' || isPreviewActive ? (
            <MarkdownPreview />
          ) : (
            <CodeEditorPane
              onSplitRight={() => setIsSplitEditor(true)}
              onTogglePreview={() => setIsPreviewActive(prev => !prev)}
              isPreview={false}
            />
          )}
        </div>
      </div>

    </div>
  );
};
