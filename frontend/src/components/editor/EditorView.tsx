import React, { useState, useEffect } from 'react';
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

export interface EditorPaneState {
  id: string;
  activeTabId: string | null;
  tabIds: string[];
}

export type SplitLayoutOrientation = 'row' | 'col';

export const EditorView: React.FC = () => {
  const { isSplitEditor, setIsSplitEditor, activeTabId, setActiveTabId, openTabs, diffs, activeDiff, openTab, closeTab } = useWorkspace();
  const [isPreviewActive, setIsPreviewActive] = useState(false);

  // Panes model (Zed / VSCode style):
  // Each pane maintains its own tabIds and its activeTabId.
  // When splitting, only the currently active tab in that pane is split into the new pane.
  const [orientation, setOrientation] = useState<SplitLayoutOrientation>('row');
  const [panes, setPanes] = useState<EditorPaneState[]>([
    { id: 'pane-primary', activeTabId: activeTabId, tabIds: openTabs.map(t => t.id) }
  ]);
  const [focusedPaneId, setFocusedPaneId] = useState<string>('pane-primary');

  // Keep primary pane synced with openTabs when not split or when new tabs are opened
  useEffect(() => {
    setPanes(prevPanes => {
      const allOpenTabIds = new Set(openTabs.map(t => t.id));
      
      // Clean up closed tabs from all panes
      const cleaned = prevPanes.map(pane => {
        const remainingTabIds = pane.tabIds.filter(id => allOpenTabIds.has(id));
        let nextActive = pane.activeTabId;
        if (nextActive && !allOpenTabIds.has(nextActive)) {
          nextActive = remainingTabIds.length > 0 ? remainingTabIds[remainingTabIds.length - 1] : null;
        }
        return {
          ...pane,
          tabIds: remainingTabIds,
          activeTabId: nextActive
        };
      });

      // Find any newly added tabs that aren't in any pane yet
      const assignedTabIds = new Set(cleaned.flatMap(p => p.tabIds));
      const unassigned = openTabs.filter(t => !assignedTabIds.has(t.id));

      if (unassigned.length > 0) {
        // Add to focused pane or first pane
        return cleaned.map(pane => {
          if (pane.id === focusedPaneId || (cleaned.length === 1 && pane.id === cleaned[0].id)) {
            const nextTabIds = [...pane.tabIds, ...unassigned.map(t => t.id)];
            return {
              ...pane,
              tabIds: nextTabIds,
              activeTabId: activeTabId || unassigned[unassigned.length - 1].id
            };
          }
          return pane;
        });
      }

      // If activeTabId changed globally, update activeTab in focused pane
      if (activeTabId) {
        return cleaned.map(pane => {
          if (pane.id === focusedPaneId && pane.tabIds.includes(activeTabId)) {
            return { ...pane, activeTabId };
          }
          return pane;
        });
      }

      return cleaned;
    });
  }, [openTabs, activeTabId, focusedPaneId]);

  // Remove empty panes if multiple panes exist
  useEffect(() => {
    if (panes.length > 1) {
      const nonEmpty = panes.filter(p => p.tabIds.length > 0);
      if (nonEmpty.length !== panes.length && nonEmpty.length > 0) {
        setPanes(nonEmpty);
        if (!nonEmpty.some(p => p.id === focusedPaneId)) {
          setFocusedPaneId(nonEmpty[0].id);
        }
      }
    }
  }, [panes, focusedPaneId]);

  // Sync split editor toggle
  useEffect(() => {
    if (!isSplitEditor && panes.length > 1) {
      // Consolidate into single pane
      const mergedTabIds = Array.from(new Set(openTabs.map(t => t.id)));
      setPanes([
        { id: 'pane-primary', activeTabId: activeTabId || (mergedTabIds[0] ?? null), tabIds: mergedTabIds }
      ]);
      setFocusedPaneId('pane-primary');
    }
  }, [isSplitEditor, openTabs, activeTabId]);

  // Split handlers (Zed / VSCode style):
  // Splits the current open tab into a new pane. Does NOT duplicate all open tabs!
  const splitCurrentFile = (direction: 'right' | 'left' | 'down' | 'up', sourcePaneId: string) => {
    const sourcePane = panes.find(p => p.id === sourcePaneId);
    const activeFileTabId = sourcePane?.activeTabId || activeTabId;
    if (!activeFileTabId) return;

    const newPaneId = `pane-${Date.now()}`;
    const newPane: EditorPaneState = {
      id: newPaneId,
      activeTabId: activeFileTabId,
      tabIds: [activeFileTabId]
    };

    const isHorizontal = direction === 'right' || direction === 'left';
    setOrientation(isHorizontal ? 'row' : 'col');
    setIsSplitEditor(true);

    setPanes(prevPanes => {
      const idx = prevPanes.findIndex(p => p.id === sourcePaneId);
      if (idx === -1) return [...prevPanes, newPane];

      const next = [...prevPanes];
      if (direction === 'right' || direction === 'down') {
        next.splice(idx + 1, 0, newPane);
      } else {
        next.splice(idx, 0, newPane);
      }
      return next;
    });

    setFocusedPaneId(newPaneId);
  };

  const handlePaneTabSelect = (paneId: string, tab: EditorTab) => {
    setFocusedPaneId(paneId);
    setPanes(prev => prev.map(p => p.id === paneId ? { ...p, activeTabId: tab.id } : p));
    openTab(tab);
  };

  const handleClosePaneTab = (paneId: string, tabId: string) => {
    setPanes(prevPanes => {
      const targetPane = prevPanes.find(p => p.id === paneId);
      if (!targetPane) return prevPanes;

      const remaining = targetPane.tabIds.filter(id => id !== tabId);
      let nextActive = targetPane.activeTabId;
      if (nextActive === tabId) {
        const closedIdx = targetPane.tabIds.indexOf(tabId);
        nextActive = remaining[Math.min(closedIdx, remaining.length - 1)] || null;
      }

      // If pane becomes empty and other panes exist, close the pane
      if (remaining.length === 0 && prevPanes.length > 1) {
        const remainingPanes = prevPanes.filter(p => p.id !== paneId);
        if (focusedPaneId === paneId) {
          setFocusedPaneId(remainingPanes[0].id);
        }
        return remainingPanes;
      }

      // Also close tab globally if it's not present in any other pane
      const inOtherPanes = prevPanes.some(p => p.id !== paneId && p.tabIds.includes(tabId));
      if (!inOtherPanes) {
        closeTab(tabId);
      }

      return prevPanes.map(p => p.id === paneId ? { ...p, tabIds: remaining, activeTabId: nextActive } : p);
    });
  };

  const focusedPane = panes.find(p => p.id === focusedPaneId) || panes[0];
  const currentTabId = focusedPane?.activeTabId || activeTabId;
  const activeTab = openTabs.find(t => t.id === currentTabId) || openTabs[0];
  const targetDiff = activeTab?.diffId ? diffs.find(d => d.id === activeTab.diffId) : activeDiff;
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
          ) : isPdfFile && activeTab?.filePath ? (
            <PdfViewer filePath={activeTab.filePath} fileName={activeTab.fileName} />
          ) : (activeTab?.type === 'preview' || isPreviewActive) && panes.length <= 1 ? (
            <MarkdownPreview onBackToEditor={() => setIsPreviewActive(false)} />
          ) : panes.length > 1 ? (
            /* Multi-pane Zed / VSCode style split view */
            <div
              className={`flex-1 flex overflow-hidden ${
                orientation === 'col'
                  ? 'flex-col divide-y divide-[#e5e7eb] dark:divide-[#282828]'
                  : 'flex-row divide-x divide-[#e5e7eb] dark:divide-[#282828]'
              }`}
            >
              {panes.map((pane) => {
                const paneActiveTab = openTabs.find(t => t.id === pane.activeTabId) || openTabs.find(t => pane.tabIds.includes(t.id));
                const paneTabsList = openTabs.filter(t => pane.tabIds.includes(t.id));

                return (
                  <div
                    key={pane.id}
                    onClick={() => setFocusedPaneId(pane.id)}
                    className="flex-1 flex overflow-hidden min-w-[240px] min-h-[180px] relative"
                  >
                    <CodeEditorPane
                      tabId={paneActiveTab?.id}
                      paneTabs={paneTabsList}
                      onTabSelect={(t) => handlePaneTabSelect(pane.id, t)}
                      onClosePaneTab={(tId) => handleClosePaneTab(pane.id, tId)}
                      onSplitRight={() => splitCurrentFile('right', pane.id)}
                      onSplitLeft={() => splitCurrentFile('left', pane.id)}
                      onSplitDown={() => splitCurrentFile('down', pane.id)}
                      onSplitUp={() => splitCurrentFile('up', pane.id)}
                      onTogglePreview={() => setIsPreviewActive(prev => !prev)}
                      isPreview={isPreviewActive}
                    />
                  </div>
                );
              })}
            </div>
          ) : (
            /* Single Pane */
            <CodeEditorPane
              tabId={activeTab?.id}
              paneTabs={openTabs}
              onTabSelect={(t) => openTab(t)}
              onClosePaneTab={(tId) => closeTab(tId)}
              onSplitRight={() => splitCurrentFile('right', panes[0]?.id || 'pane-primary')}
              onSplitLeft={() => splitCurrentFile('left', panes[0]?.id || 'pane-primary')}
              onSplitDown={() => splitCurrentFile('down', panes[0]?.id || 'pane-primary')}
              onSplitUp={() => splitCurrentFile('up', panes[0]?.id || 'pane-primary')}
              onTogglePreview={() => setIsPreviewActive(prev => !prev)}
              isPreview={isPreviewActive}
            />
          )}
        </div>
      </div>

    </div>
  );
};
