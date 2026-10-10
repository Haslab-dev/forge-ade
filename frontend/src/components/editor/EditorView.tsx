import React, { Suspense, lazy, useState, useEffect } from 'react';
import { ActivityBar } from './ActivityBar';
import { FileTree } from './FileTree';
import { CodeEditorPane } from './CodeEditorPane';
import { ForgeSettingsTab } from './ForgeSettingsTab';
import { PdfViewer } from './PdfViewer';
import { useWorkspace } from '../../stores/workspaceStore';
import { DiffViewer } from '../diff/DiffViewer';
import { EditorTab } from '../../types';
import { AgentRightSidebar } from '../agent/AgentRightSidebar';
import { PanelResizeHandle } from '../agent/PanelResizeHandle';

// Rarely-opened panes load on demand — `marked` and the git-graph machinery
// never touch startup or plain code editing.
const MarkdownPreview = lazy(() =>
  import('./MarkdownPreview').then(m => ({ default: m.MarkdownPreview }))
);
const GitGraphPane = lazy(() =>
  import('./GitGraphPane').then(m => ({ default: m.GitGraphPane }))
);

const PaneFallback: React.FC = () => (
  <div className="flex-1 flex items-center justify-center bg-app text-foreground-subtle">
    <span className="text-ui-sm animate-pulse">Loading pane…</span>
  </div>
);

const PDF_EXTENSIONS = ['.pdf'];

export interface EditorPaneState {
  id: string;
  activeTabId: string | null;
  tabIds: string[];
}

export type SplitLayoutOrientation = 'row' | 'col';

export const EditorView: React.FC = () => {
  const { 
    isSplitEditor, 
    setIsSplitEditor, 
    isLeftSidebarOpen, 
    activeTabId, 
    setActiveTabId, 
    openTabs, 
    diffs, 
    activeDiff, 
    openTab, 
    closeTab,
    closeTabs,
    isRightActionDrawerOpen,
    rightPaneWidth,
    setRightPaneWidth
  } = useWorkspace();
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

  // Drag & drop: move a tab between panes (or reorder within one). Empty
  // source panes collapse; the moved tab becomes active in its target.
  const movePaneTab = (fromPaneId: string, tabId: string, toPaneId: string, insertIndex?: number) => {
    const insertAt = (ids: string[], id: string, at?: number) => {
      const nextIds = ids.filter(x => x !== id);
      const index = at === undefined ? nextIds.length : Math.max(0, Math.min(at, nextIds.length));
      nextIds.splice(index, 0, id);
      return nextIds;
    };
    if (fromPaneId === toPaneId) {
      // Reorder within the same pane — the cross-pane branch below would
      // strip the tab without re-inserting it.
      setPanes(prev => prev.map(p => (
        p.id === fromPaneId ? { ...p, tabIds: insertAt(p.tabIds, tabId, insertIndex) } : p
      )));
      setActiveTabId(tabId);
      return;
    }
    setPanes(prev => {
      const from = prev.find(p => p.id === fromPaneId);
      if (!from || !from.tabIds.includes(tabId)) return prev;
      let next = prev.map(p => {
        if (p.id === fromPaneId) {
          const remaining = p.tabIds.filter(id => id !== tabId);
          return {
            ...p,
            tabIds: remaining,
            activeTabId: p.activeTabId === tabId ? (remaining[remaining.length - 1] ?? null) : p.activeTabId
          };
        }
        if (p.id === toPaneId) {
          return { ...p, tabIds: insertAt(p.tabIds, tabId, insertIndex), activeTabId: tabId };
        }
        return p;
      });
      if (next.length > 1) {
        const nonEmpty = next.filter(p => p.tabIds.length > 0);
        if (nonEmpty.length > 0 && nonEmpty.length < next.length) next = nonEmpty;
      }
      return next;
    });
    setActiveTabId(tabId);
    setFocusedPaneId(toPaneId);
  };

  // Resizable panes: proportional flexGrow per pane, dragged via dividers.
  const [paneSizes, setPaneSizes] = useState<Record<string, number>>({});
  useEffect(() => {
    // (Re)normalise when the pane count changes; drags keep stored ratios.
    setPaneSizes(prev => {
      const missing = panes.some(p => prev[p.id] === undefined);
      if (!missing) return prev;
      const next: Record<string, number> = {};
      panes.forEach(p => { next[p.id] = 100 / panes.length; });
      return next;
    });
  }, [panes]);

  const startPaneResize = (e: React.MouseEvent, aId: string, bId: string) => {
    e.preventDefault();
    const total = orientation === 'row' ? window.innerWidth : window.innerHeight;
    const horizontal = orientation === 'row';
    const startPos = horizontal ? e.clientX : e.clientY;
    const startA = paneSizes[aId] ?? 50;
    const startB = paneSizes[bId] ?? 50;
    const onMove = (ev: MouseEvent) => {
      const delta = ((horizontal ? ev.clientX : ev.clientY) - startPos) / total * 100;
      const a = Math.max(12, Math.min(startA + startB - 12, startA + delta));
      setPaneSizes(prev => ({ ...prev, [aId]: a, [bId]: startA + startB - a }));
    };
    const onUp = () => {
      window.removeEventListener('mousemove', onMove);
      window.removeEventListener('mouseup', onUp);
      document.body.style.cursor = '';
    };
    window.addEventListener('mousemove', onMove);
    window.addEventListener('mouseup', onUp);
    document.body.style.cursor = horizontal ? 'col-resize' : 'row-resize';
  };

  // Tab context-menu bulk closes (Zed semantics): pinned tabs survive every
  // bulk mode, "clean" closes only non-dirty tabs. Closing is delegated to
  // the store per tab; pane lists self-clean via the sync effect above.
  const closePaneTabsBatch = (paneId: string, mode: 'others' | 'left' | 'right' | 'clean' | 'all', keepTabId?: string) => {
    const pane = panes.find(p => p.id === paneId);
    if (!pane) return;
    const byId = new Map(openTabs.map(t => [t.id, t]));
    const keepIdx = keepTabId ? pane.tabIds.indexOf(keepTabId) : -1;
    const toClose = pane.tabIds
      .map((id, i) => ({ id, i }))
      .filter(({ id, i }) => {
        const tab = byId.get(id);
        if (!tab || tab.isPinned) return false;
        switch (mode) {
          case 'others': return id !== keepTabId;
          case 'left': return keepIdx >= 0 && i < keepIdx;
          case 'right': return keepIdx >= 0 && i > keepIdx;
          case 'clean': return !tab.isModified;
          case 'all': return true;
        }
      })
      .map(x => x.id);
    // Close the pane-active tab last so neighbour activation lands sanely.
    const paneActive = pane.activeTabId;
    const ordered = paneActive && toClose.includes(paneActive)
      ? [...toClose.filter(id => id !== paneActive), paneActive]
      : toClose;
    closeTabs(toClose);
  };

  const handleClosePaneTab = (paneId: string, tabId: string) => {    setPanes(prevPanes => {
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
    <div className="flex-1 flex flex-col h-full overflow-hidden bg-background select-none font-sans">
      
      {/* Top Main Work Area */}
      <div className="flex-1 flex overflow-hidden">
        {/* Core Activity Bar (Editor, Search, Git, Settings) */}
        <ActivityBar />

        {/* Project File Tree & Activity Panel */}
        {isLeftSidebarOpen && <FileTree />}

        {/* Main Editor Panes Grid */}
        <div className="flex-1 flex overflow-hidden bg-background">
          {activeTab?.type === 'settings' ? (
            <ForgeSettingsTab />
          ) : activeTab?.type === 'diff' && targetDiff ? (
            <div className="flex-1 min-h-0 p-4 flex flex-col overflow-hidden bg-background dark:bg-[#141414]">
              <DiffViewer
                diff={targetDiff}
                onClose={() => {
                  const tab = openTabs.find(t => t.diffId === targetDiff.id) || openTabs.find(t => t.id === 'tab-git-diff');
                  if (tab) closeTab(tab.id);
                }}
              />
            </div>
          ) : activeTab?.type === 'git-graph' ? (
            <Suspense fallback={<PaneFallback />}><GitGraphPane /></Suspense>
          ) : isPdfFile && activeTab?.filePath ? (
            <PdfViewer filePath={activeTab.filePath} fileName={activeTab.fileName} />
          ) : (activeTab?.type === 'preview' || isPreviewActive) && panes.length <= 1 ? (
            <Suspense fallback={<PaneFallback />}>
              <MarkdownPreview onBackToEditor={() => setIsPreviewActive(false)} />
            </Suspense>
          ) : panes.length > 1 ? (
            /* Multi-pane Zed / VSCode style split view */
            <div
              className={`flex-1 flex overflow-hidden ${orientation === 'col' ? 'flex-col' : 'flex-row'}`}
            >
              {panes.map((pane, paneIdx) => {
                const paneActiveTab = openTabs.find(t => t.id === pane.activeTabId) || openTabs.find(t => pane.tabIds.includes(t.id));
                // Pane order is the source of truth — rendering openTabs order
                // made reorder/move look like a no-op.
                const byId = new Map(openTabs.map(t => [t.id, t]));
                const paneTabsList = pane.tabIds.map(id => byId.get(id)).filter((t): t is NonNullable<typeof t> => !!t);

                return (
                  <React.Fragment key={pane.id}>
                  {paneIdx > 0 && (
                    <div
                      role="separator"
                      aria-orientation={orientation === 'row' ? 'vertical' : 'horizontal'}
                      onMouseDown={(e) => startPaneResize(e, panes[paneIdx - 1].id, pane.id)}
                      className={`flex-none bg-border hover:bg-primary/60 dark:hover:bg-primary/60 transition-colors ${
                        orientation === 'row' ? 'w-[3px] cursor-col-resize' : 'h-[3px] cursor-row-resize'
                      }`}
                    />
                  )}
                  <div
                    onClick={() => setFocusedPaneId(pane.id)}
                    style={{ flexGrow: paneSizes[pane.id] ?? (100 / panes.length), flexBasis: 0, flexShrink: 1 }}
                    className="flex overflow-hidden min-w-[120px] min-h-[80px] relative"
                  >
                    <CodeEditorPane
                      tabId={paneActiveTab?.id}
                      paneTabs={paneTabsList}
                      paneId={pane.id}
                      onMoveTab={movePaneTab}
                      onTabSelect={(t) => handlePaneTabSelect(pane.id, t)}
                      onClosePaneTab={(tId) => handleClosePaneTab(pane.id, tId)}
                      onCloseOthers={(keep) => closePaneTabsBatch(pane.id, 'others', keep)}
                      onCloseLeft={(keep) => closePaneTabsBatch(pane.id, 'left', keep)}
                      onCloseRight={(keep) => closePaneTabsBatch(pane.id, 'right', keep)}
                      onCloseClean={() => closePaneTabsBatch(pane.id, 'clean')}
                      onCloseAll={() => closePaneTabsBatch(pane.id, 'all')}
                      onSplitRight={() => splitCurrentFile('right', pane.id)}
                      onSplitLeft={() => splitCurrentFile('left', pane.id)}
                      onSplitDown={() => splitCurrentFile('down', pane.id)}
                      onSplitUp={() => splitCurrentFile('up', pane.id)}
                      onTogglePreview={() => setIsPreviewActive(prev => !prev)}
                      isPreview={isPreviewActive}
                    />
                  </div>
                  </React.Fragment>
                );
              })}
            </div>
          ) : (
            /* Single Pane */
            <CodeEditorPane
              tabId={activeTab?.id}
              paneTabs={(() => {
                const primary = panes.find(p => p.id === (panes[0]?.id || 'pane-primary'));
                if (!primary || primary.tabIds.length === 0) return openTabs;
                const byId = new Map(openTabs.map(t => [t.id, t]));
                const ordered = primary.tabIds.map(id => byId.get(id)).filter((t): t is NonNullable<typeof t> => !!t);
                // Include tabs not yet assigned to the pane (defensive).
                const extra = openTabs.filter(t => !primary.tabIds.includes(t.id));
                return [...ordered, ...extra];
              })()}
              paneId={panes[0]?.id || 'pane-primary'}
              onMoveTab={movePaneTab}
              onTabSelect={(t) => openTab(t)}
              onClosePaneTab={(tId) => closeTab(tId)}
              onCloseOthers={(keep) => closePaneTabsBatch(panes[0]?.id || 'pane-primary', 'others', keep)}
              onCloseLeft={(keep) => closePaneTabsBatch(panes[0]?.id || 'pane-primary', 'left', keep)}
              onCloseRight={(keep) => closePaneTabsBatch(panes[0]?.id || 'pane-primary', 'right', keep)}
              onCloseClean={() => closePaneTabsBatch(panes[0]?.id || 'pane-primary', 'clean')}
              onCloseAll={() => closePaneTabsBatch(panes[0]?.id || 'pane-primary', 'all')}
              onSplitRight={() => splitCurrentFile('right', panes[0]?.id || 'pane-primary')}
              onSplitLeft={() => splitCurrentFile('left', panes[0]?.id || 'pane-primary')}
              onSplitDown={() => splitCurrentFile('down', panes[0]?.id || 'pane-primary')}
              onSplitUp={() => splitCurrentFile('up', panes[0]?.id || 'pane-primary')}
              onTogglePreview={() => setIsPreviewActive(prev => !prev)}
              isPreview={isPreviewActive}
            />
          )}
        </div>

        {/* Right side pane for Editor Mode (matches Agent mode Review/Chat/Terminal) */}
        {isRightActionDrawerOpen && (
          <>
            <PanelResizeHandle
              edge="left"
              ariaLabel="Resize side panel"
              getStartWidth={() => rightPaneWidth}
              onResize={setRightPaneWidth}
              min={360}
              max={() => Math.max(360, Math.floor(window.innerWidth * 0.6))}
              className="-mr-1"
            />
            <AgentRightSidebar initialTab="review" />
          </>
        )}
      </div>

    </div>
  );
};
