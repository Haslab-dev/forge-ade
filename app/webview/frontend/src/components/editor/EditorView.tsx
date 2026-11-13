import React, { Suspense, lazy, useCallback, useState, useEffect, useMemo } from 'react';
import { AlertCircle, RefreshCw, Loader2 } from 'lucide-react';
import { ActivityBar } from './ActivityBar';
import { FileTree } from './FileTree';
import { CodeEditorPane } from './CodeEditorPane';
import { ForgeSettingsTab } from './ForgeSettingsTab';
import { PdfViewer } from './PdfViewer';
import { useWorkspace } from '../../stores/workspaceStore';
import { ApiBridge } from '../../services/apiBridge';
import { showToast } from '../../lib/toast';
import type { SSHConfig } from '../../lib/wails';
import { DiffViewer } from '../diff/DiffViewer';
import { EditorTab } from '../../types';
import { GitClientView } from '../git/GitClientView';

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
    mode,
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
    activeWorkspacePath,
    refreshFiles,
    disconnectSSH,
    setIsSSHModalOpen,
    isGitClientOpen,
    setIsGitClientOpen
  } = useWorkspace();
  const [isPreviewActive, setIsPreviewActive] = useState(false);

  // SSH Disconnection Banner State
  const [isSSHDisconnected, setIsSSHDisconnected] = useState(false);
  const [sshDisconnectReason, setSSHDisconnectReason] = useState<string | null>(null);
  const [isSSHReconnecting, setIsSSHReconnecting] = useState(false);

  const isRemoteWorkspace = activeWorkspacePath.startsWith('ssh://');
  const remoteHostLabel = useMemo(() => {
    if (!isRemoteWorkspace) return '';
    const rest = activeWorkspacePath.replace(/^ssh:\/\//, '');
    const slashIdx = rest.indexOf('/');
    const hostPart = slashIdx !== -1 ? rest.substring(0, slashIdx) : rest;
    return hostPart.includes('@')
      ? hostPart.split('@')[1]
      : hostPart.replace(/^ssh-/, '').split('-')[0];
  }, [activeWorkspacePath, isRemoteWorkspace]);

  useEffect(() => {
    setIsSSHDisconnected(false);
    setSSHDisconnectReason(null);
  }, [activeWorkspacePath]);

  useEffect(() => {
    const onDisconnect = (e: any) => {
      if (activeWorkspacePath.startsWith('ssh://')) {
        setIsSSHDisconnected(true);
        setSSHDisconnectReason(e.detail?.reason || 'Connection lost');
      }
    };
    window.addEventListener('forge:ssh-disconnected', onDisconnect);
    return () => window.removeEventListener('forge:ssh-disconnected', onDisconnect);
  }, [activeWorkspacePath]);

  const handleReconnectSSH = async () => {
    setIsSSHReconnecting(true);
    try {
      const rest = activeWorkspacePath.replace(/^ssh:\/\//, '');
      const slashIdx = rest.indexOf('/');
      const connId = slashIdx !== -1 ? rest.substring(0, slashIdx) : rest;
      const remotePath = slashIdx !== -1 ? rest.substring(slashIdx) : '/';
      let cfg: SSHConfig | null = null;
      try {
        const stored = localStorage.getItem('forge_saved_ssh_configs');
        if (stored) {
          const list: SSHConfig[] = JSON.parse(stored);
          cfg = list.find((c) => c.id === connId || activeWorkspacePath.includes(c.host)) || null;
        }
      } catch {}

      if (cfg) {
        const res = await ApiBridge.openSSHWorkspace({ ...cfg, remotePath });
        if (res && res.connected) {
          setIsSSHDisconnected(false);
          setSSHDisconnectReason(null);
          await refreshFiles();
          showToast(`Reconnected to ${cfg.host}`, 'success');
        } else {
          showToast(`Reconnect failed: ${res?.error || 'failed'}`, 'error');
        }
      } else {
        setIsSSHModalOpen(true);
      }
    } catch (err: any) {
      showToast(`Reconnect error: ${err?.message || 'failed'}`, 'error');
    } finally {
      setIsSSHReconnecting(false);
    }
  };

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
      let hasCleanedChanges = false;
      
      // Clean up closed tabs from all panes
      const cleaned = prevPanes.map(pane => {
        const remainingTabIds = pane.tabIds.filter(id => allOpenTabIds.has(id));
        let nextActive = pane.activeTabId;
        if (nextActive && !allOpenTabIds.has(nextActive)) {
          nextActive = remainingTabIds.length > 0 ? remainingTabIds[remainingTabIds.length - 1] : null;
        }
        if (remainingTabIds.length !== pane.tabIds.length || nextActive !== pane.activeTabId) {
          hasCleanedChanges = true;
          return {
            ...pane,
            tabIds: remainingTabIds,
            activeTabId: nextActive
          };
        }
        return pane;
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
      let hasActiveTabChange = false;
      const updated = cleaned.map(pane => {
        if (activeTabId && pane.id === focusedPaneId && pane.tabIds.includes(activeTabId) && pane.activeTabId !== activeTabId) {
          hasActiveTabChange = true;
          return { ...pane, activeTabId };
        }
        return pane;
      });

      if (!hasCleanedChanges && !hasActiveTabChange) {
        return prevPanes;
      }

      return updated;
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
  const splitCurrentFile = useCallback((direction: 'right' | 'left' | 'down' | 'up', sourcePaneId: string) => {
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
  }, [panes, activeTabId, setIsSplitEditor, openTab, focusedPaneId]);

  const handlePaneTabSelect = useCallback((paneId: string, tab: EditorTab) => {
    setFocusedPaneId(paneId);
    setPanes(prev => prev.map(p => p.id === paneId ? { ...p, activeTabId: tab.id } : p));
    openTab(tab);
  }, [openTab]);

  // Drag & drop: move a tab between panes (or reorder within one). Empty
  // source panes collapse; the moved tab becomes active in its target.
  // Param order matches the onMoveTab prop contract in CodeEditorPane —
  // swapping these made every drag a silent no-op.
  const movePaneTab = useCallback((tabId: string, fromPaneId: string, toPaneId: string, insertIndex?: number) => {
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
  }, []);

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
  const closePaneTabsBatch = useCallback((paneId: string, mode: 'others' | 'left' | 'right' | 'clean' | 'all', keepTabId?: string) => {
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
  }, [panes, openTabs, closeTabs]);

  const handleClosePaneTab = useCallback((paneId: string, tabId: string) => {
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
        if (remainingPanes.length <= 1) {
          setIsSplitEditor(false);
        }
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
  }, [focusedPaneId, closeTab]);

  const closePane = useCallback((targetPaneId: string) => {
    setPanes(prevPanes => {
      const remainingPanes = prevPanes.filter(p => p.id !== targetPaneId);
      if (remainingPanes.length <= 1) {
        setIsSplitEditor(false);
      }
      if (focusedPaneId === targetPaneId && remainingPanes.length > 0) {
        setFocusedPaneId(remainingPanes[0].id);
      }
      return remainingPanes.length > 0 ? remainingPanes : prevPanes;
    });
  }, [focusedPaneId, setIsSplitEditor]);

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

        {/* Project File Tree & Activity Panel (Only active when in editor mode) */}
        {mode === 'editor' && isLeftSidebarOpen && !isGitClientOpen && <FileTree />}

        {/* Main Editor Panes Grid */}
        <div className="flex-1 flex flex-col overflow-hidden bg-background">
          {/* SSH Disconnection Warning Banner */}
          {isRemoteWorkspace && isSSHDisconnected && (
            <div className="bg-destructive/15 border-b border-destructive/30 px-4 py-2 flex items-center justify-between text-xs text-foreground shrink-0 z-20 animate-in fade-in">
              <div className="flex items-center gap-2 min-w-0">
                <AlertCircle className="w-4 h-4 text-destructive shrink-0" />
                <span className="truncate">
                  SSH remote connection to <strong>{remoteHostLabel}</strong> was disconnected {sshDisconnectReason ? `(${sshDisconnectReason})` : ''}.
                </span>
              </div>
              <div className="flex items-center gap-2 shrink-0">
                <button
                  type="button"
                  disabled={isSSHReconnecting}
                  onClick={handleReconnectSSH}
                  className="px-2.5 py-1 rounded bg-primary text-primary-foreground font-medium hover:bg-primary/90 flex items-center gap-1.5 cursor-pointer disabled:opacity-50 text-xs"
                >
                  {isSSHReconnecting ? <Loader2 className="w-3.5 h-3.5 animate-spin" /> : <RefreshCw className="w-3.5 h-3.5" />}
                  Reconnect Now
                </button>
                <button
                  type="button"
                  onClick={disconnectSSH}
                  className="px-2 py-1 rounded bg-surface-subtle hover:bg-surface-hover text-foreground-subtle hover:text-foreground cursor-pointer text-xs"
                >
                  Close Workspace
                </button>
              </div>
            </div>
          )}

          {isGitClientOpen ? (
            <GitClientView onClose={() => setIsGitClientOpen(false)} />
          ) : activeTab?.type === 'settings' ? (
            <ForgeSettingsTab />
          ) : activeTab?.type === 'diff' && targetDiff ? (
            <div className="flex-1 min-h-0 p-4 flex flex-col overflow-hidden bg-background">
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
                    // Focus events from any surface inside (CodeMirror,
                    // terminal, tab buttons) also claim pane focus.
                    onFocusCapture={() => setFocusedPaneId(pane.id)}
                    style={{ flexGrow: paneSizes[pane.id] ?? (100 / panes.length), flexBasis: 0, flexShrink: 1 }}
                    className="flex overflow-hidden min-w-[120px] min-h-[80px] relative"
                  >
                    <CodeEditorPane
                      tabId={paneActiveTab?.id}
                      paneTabs={paneTabsList}
                      paneId={pane.id}
                      onMoveTab={movePaneTab}
                      paneFocus={pane.id === focusedPaneId ? 'focused' : 'unfocused'}
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
                      onClosePane={() => closePane(pane.id)}
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
              paneFocus="solo"
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
      </div>

    </div>
  );
};
