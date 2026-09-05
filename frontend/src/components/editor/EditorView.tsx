import React, { useState, useMemo } from 'react';
import { ActivityBar } from './ActivityBar';
import { FileTree } from './FileTree';
import { CodeEditorPane } from './CodeEditorPane';
import { MarkdownPreview } from './MarkdownPreview';
import { ForgeSettingsTab } from './ForgeSettingsTab';
import { PdfViewer } from './PdfViewer';
import { useWorkspace } from '../../stores/workspaceStore';
import { DiffViewer } from '../diff/DiffViewer';
import { GitGraphPane } from './GitGraphPane';
import { EditorTab } from '../../types';

const PDF_EXTENSIONS = ['.pdf'];

export const EditorView: React.FC = () => {
  const { isSplitEditor, setIsSplitEditor, activeTabId, openTabs, diffs, activeDiff, openTab } = useWorkspace();
  const [isPreviewActive, setIsPreviewActive] = useState(false);
  // Which tab each split pane displays (null → the tab at that position).
  const [splitPaneTabIds, setSplitPaneTabIds] = useState<Array<string | null>>([null, null, null]);

  const activeTab = openTabs.find(t => t.id === activeTabId) || openTabs[0];
  const targetDiff = activeTab?.diffId ? diffs.find(d => d.id === activeTab.diffId) : activeDiff;
  const isPdfFile = activeTab?.fileName ? PDF_EXTENSIONS.some(ext => activeTab.fileName.toLowerCase().endsWith(ext)) : false;

  // Split panes: up to 3 (minimum 2). Pane i shows its pinned tab if the user
  // switched it, else the i-th open tab, else the active tab — so opening a
  // 3rd file while split grows a third pane with that file.
  const splitTabs = useMemo(() => {
    const count = Math.min(Math.max(openTabs.length, 2), 3);
    return Array.from({ length: count }, (_, i) => {
      const pinned = splitPaneTabIds[i];
      return (pinned ? openTabs.find(t => t.id === pinned) : undefined) ?? openTabs[i] ?? activeTab;
    });
  }, [openTabs, splitPaneTabIds, activeTab]);

  const handlePaneTabSelect = (paneIdx: number, tab: EditorTab) => {
    setSplitPaneTabIds(prev => prev.map((id, i) => (i === paneIdx ? tab.id : id)));
    openTab(tab);
  };

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
          ) : isPdfFile && activeTab?.filePath ? (
            <PdfViewer filePath={activeTab.filePath} fileName={activeTab.fileName} />
          ) : isSplitEditor ? (
            /* Split: one pane per open tab (2–3). Clicking a tab inside a
               pane switches THAT pane's file. */
            <div className="flex-1 flex overflow-hidden divide-x divide-[#e5e7eb] dark:divide-[#282828]">
              {splitTabs.map((tab, idx) => (
                <div key={idx} className="flex-1 flex overflow-hidden min-w-[280px]">
                  <CodeEditorPane
                    tabId={tab?.id}
                    onTabSelect={(t) => handlePaneTabSelect(idx, t)}
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
