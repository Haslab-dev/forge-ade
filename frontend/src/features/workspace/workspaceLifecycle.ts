import { useEffect, useRef } from 'react';
import { useWorkspace } from '../../stores/workspaceStore';
import { useSessionStore } from '../../stores/sessionStore';
import { useUIStore } from '../../hooks/store';
import { applyTheme } from '../../lib/themeEngine';
import {
  EventsOn,
  GetRecentProjects,
  SetAgentAutoApprove,
  StopSession,
  GetAgentSession,
  type Automation as ZAutomation
} from '../../lib/wails';
import { ApiBridge } from '../../services/apiBridge';
import { registerBenchStore } from '../../lib/devBench';
import { setOpenBrowserHandler } from '../../lib/browser';
import { convertGoSessionToAgentSession } from '../agent/sessionConverters';

const sessionKeyFor = (wsPath: string) => `forge_ade_editor_session:${wsPath}`;

interface PersistedEditorSession {
  tabs: any[];
  activeTabId: string | null;
}

function readPersistedSession(wsPath: string): PersistedEditorSession | null {
  try {
    const raw = localStorage.getItem(sessionKeyFor(wsPath));
    if (!raw) return null;
    const parsed = JSON.parse(raw);
    if (!parsed || !Array.isArray(parsed.tabs)) return null;
    const tabs = parsed.tabs.filter(
      (t: any) => t && typeof t.filePath === 'string' && t.filePath && t.type === 'code'
    );
    return {
      tabs,
      activeTabId: typeof parsed.activeTabId === 'string' ? parsed.activeTabId : null
    };
  } catch {
    return null;
  }
}

/**
 * useWorkspaceLifecycle coordinates top-level application lifecycles,
 * IPC event subscriptions, keyboard hotkeys, and theme engine synchronization.
 * It connects to Zustand stores directly without React Context.
 */
export function useWorkspaceLifecycle() {
  const mode = useWorkspace(s => s.mode);
  const theme = useWorkspace(s => s.theme);
  const themePalette = useWorkspace(s => s.themePalette);
  const activeWorkspacePath = useWorkspace(s => s.activeWorkspacePath);
  const activeSession = useWorkspace(s => s.activeSession);
  const activeSessionId = useWorkspace(s => s.activeSessionId);
  const agentExecutionMode = useWorkspace(s => s.agentExecutionMode);
  const openTabs = useWorkspace(s => s.openTabs);
  const activeTabId = useWorkspace(s => s.activeTabId);
  const sessions = useWorkspace(s => s.sessions);
  const automations = useWorkspace(s => s.automations);

  // 1. Initial terminal session boot
  const bootModeAppliedRef = useRef(false);
  useEffect(() => {
    if (bootModeAppliedRef.current) return;
    bootModeAppliedRef.current = true;
    const sessionStore = useSessionStore.getState();
    void sessionStore.load();
    void sessionStore.loadAgentConfigs();
    void sessionStore.loadSettings();

    // Ensure Go backend knows and watches the active workspace on boot
    const wsStore = useWorkspace.getState();
    const wsPath = wsStore.activeWorkspacePath;
    if (wsPath) {
      void ApiBridge.openFolder(wsPath)
        .catch(() => {})
        .finally(() => {
          void wsStore.refreshFiles();
          void wsStore.refreshGitStatus();
          void wsStore.refreshGitLog();
        });
    }
  }, []);

  // 2. Sync recent workspaces from backend manager on mount
  useEffect(() => {
    void GetRecentProjects().then(entries => {
      if (Array.isArray(entries) && entries.length > 0) {
        const paths = entries
          .map((e: any) => e?.path || e?.Path)
          .filter((p: any) => typeof p === 'string' && p.trim());
        if (paths.length > 0) {
          const store = useWorkspace.getState();
          for (const path of paths) {
            store.addRecentWorkspace(path);
          }
        }
      }
    }).catch(() => {});
  }, []);

  // 3. Initial agent discovery & availability check
  useEffect(() => {
    void useWorkspace.getState().refreshAgentModels();
    void useWorkspace.getState().checkAgentsAvailability();
  }, []);

  // 4. Reload saved sessions when activeWorkspacePath changes
  useEffect(() => {
    void useWorkspace.getState().reloadSavedSessions();
  }, [activeWorkspacePath]);

  // 5. Theme synchronization
  useEffect(() => {
    localStorage.setItem('forge-ade-theme', theme);
    localStorage.setItem('forge_ade_theme', theme);
    useUIStore.getState().setTheme(theme);
    applyTheme(themePalette, theme);
  }, [theme, themePalette]);

  // 6. Go agent session auto-approve sync with permission mode
  const activeGoSessionId = activeSession?.goSessionId;
  useEffect(() => {
    if (!activeGoSessionId) return;
    SetAgentAutoApprove(activeGoSessionId, agentExecutionMode === 'bypass').catch(() => {});
  }, [activeGoSessionId, agentExecutionMode]);

  // 7. Automations listener and scheduler claimer
  useEffect(() => {
    const unsubFire = EventsOn('automations:fire', (data: any) => {
      const id = data?.id;
      const token = data?.token;
      if (!id || !token) return;
      void (async () => {
        const store = useWorkspace.getState();
        let automation = store.automations.find(a => a.id === id);
        if (!automation) {
          const list = await ApiBridge.listAutomations();
          automation = (list || []).find(a => a.id === id);
        }
        if (!automation) return;
        const claimed = await ApiBridge.claimAutomationFire(id, token);
        if (claimed) store.runAutomation(automation);
      })();
    });

    const unsubChanged = EventsOn('automations:changed', () => {
      void useWorkspace.getState().fetchAutomations();
    });

    return () => {
      unsubFire?.();
      unsubChanged?.();
    };
  }, []);

  // 8. Codebase indexing status listener
  useEffect(() => {
    void useWorkspace.getState().fetchIndexStatus();
    const unsub = EventsOn('index:changed', () => {
      void useWorkspace.getState().fetchIndexStatus();
    });
    return () => {
      unsub?.();
    };
  }, []);

  // 9. File activations on activeTabId
  useEffect(() => {
    const tab = openTabs.find(t => t.id === activeTabId);
    if (tab && tab.type === 'code' && tab.filePath) {
      useWorkspace.getState().recordRecentFile?.(tab.filePath);
    }
  }, [activeTabId, openTabs]);

  // 10. External fs:changed watcher & ssh:disconnected
  useEffect(() => {
    const isHeavyDir = (filePath: string): boolean => {
      const parts = filePath.split(/[/\\]/);
      return parts.some(p =>
        p === 'node_modules' ||
        p === '.git' ||
        p === 'dist' ||
        p === 'build' ||
        p === '.next' ||
        p === '.nuxt' ||
        p === 'vendor' ||
        p === 'target' ||
        p === '.zig-cache' ||
        p === 'zig-out'
      );
    };

    let gitRefreshTimer: number | null = null;
    const scheduleGitRefresh = () => {
      if (gitRefreshTimer !== null) window.clearTimeout(gitRefreshTimer);
      gitRefreshTimer = window.setTimeout(() => {
        gitRefreshTimer = null;
        void useWorkspace.getState().refreshGitStatus();
      }, 500);
    };

    const unsubFs = EventsOn('fs:changed', (event: any) => {
      const changedPath: string = event?.path || event?.Path || '';
      if (!changedPath) return;

      const store = useWorkspace.getState();
      const currentWs = store.activeWorkspacePath;
      if (currentWs && !changedPath.startsWith(currentWs)) return;

      if (!isHeavyDir(changedPath)) {
        void store.refreshFiles();
      }

      const matchingTab = store.openTabs.find(t => t.filePath === changedPath);
      if (matchingTab && !matchingTab.isModified) {
        ApiBridge.readFile(changedPath).then(newContent => {
          store.setOpenTabs(currentTabs =>
            currentTabs.map(t =>
              t.id === matchingTab.id ? { ...t, content: newContent } : t
            )
          );
        }).catch(err => console.warn('Failed to reload external file change:', err));
      }

      scheduleGitRefresh();
    });

    const unsubSSH = EventsOn('ssh:disconnected', (event: any) => {
      const connId: string = event?.id || '';
      const reason: string = event?.reason || 'Connection lost';
      console.warn(`[ssh] remote server ${connId} disconnected:`, reason);
      window.dispatchEvent(new CustomEvent('forge:ssh-disconnected', { detail: event }));
    });

    return () => {
      if (typeof unsubFs === 'function') unsubFs();
      if (typeof unsubSSH === 'function') unsubSSH();
      if (gitRefreshTimer !== null) window.clearTimeout(gitRefreshTimer);
    };
  }, [activeWorkspacePath]);

  // 11. Editor session restore across restarts
  const restoredForRef = useRef<Set<string>>(new Set());
  const activeWorkspaceRestoreRef = useRef(activeWorkspacePath);
  useEffect(() => {
    activeWorkspaceRestoreRef.current = activeWorkspacePath;
    const path = activeWorkspacePath;
    if (!path || restoredForRef.current.has(path)) return;
    restoredForRef.current.add(path);

    const saved = readPersistedSession(path);
    if (!saved || saved.tabs.length === 0) return;

    let cancelled = false;
    (async () => {
      const store = useWorkspace.getState();
      for (const tab of saved.tabs) {
        if (cancelled || activeWorkspaceRestoreRef.current !== path) return;
        try {
          await store.openFileInEditor(tab.filePath, tab.line, tab.column, { silent: true });
        } catch {}
      }
      if (cancelled || activeWorkspaceRestoreRef.current !== path) return;
      const savedActive = saved.activeTabId;
      if (savedActive && store.openTabs.some(t => t.id === savedActive)) {
        store.setActiveTabId(savedActive);
      }
    })();

    return () => { cancelled = true; };
  }, [activeWorkspacePath]);

  // 12. Debounced persist of active workspace code tabs
  const persistSessionTimerRef = useRef<number | null>(null);
  const openTabsMetaKey = openTabs.map(t => `${t.id}:${t.filePath}:${t.line}:${t.column}:${t.isPinned}:${t.isReadOnly}`).join('|');
  useEffect(() => {
    const path = activeWorkspacePath;
    if (!path) return;
    if (persistSessionTimerRef.current !== null) window.clearTimeout(persistSessionTimerRef.current);
    persistSessionTimerRef.current = window.setTimeout(() => {
      persistSessionTimerRef.current = null;
      const store = useWorkspace.getState();
      const tabs = store.openTabs.filter(t => t.type === 'code' && t.filePath);
      const payload: PersistedEditorSession = {
        tabs: tabs.map(t => ({
          id: t.id,
          fileId: t.fileId,
          fileName: t.fileName,
          filePath: t.filePath,
          line: t.line,
          column: t.column,
          isPinned: t.isPinned,
          isReadOnly: t.isReadOnly,
          type: 'code' as const
        })),
        activeTabId: store.activeTabId
      };
      try { localStorage.setItem(sessionKeyFor(path), JSON.stringify(payload)); } catch {}
    }, 400);

    return () => {
      if (persistSessionTimerRef.current !== null) window.clearTimeout(persistSessionTimerRef.current);
    };
  }, [openTabsMetaKey, activeTabId, activeWorkspacePath]);

  // 13. Subagents listeners
  useEffect(() => {
    const unsubUpdated = EventsOn('agent:updated', (data: any) => {
      const id = data?.session_id;
      if (!id) return;
      const store = useWorkspace.getState();
      if (store.activeSession?.subagentIds?.includes(id)) {
        void GetAgentSession(id).then(snap => {
          if (snap) {
            const sess = convertGoSessionToAgentSession(snap);
            store.setSubagentSession?.(id, sess);
          }
        }).catch(() => {});
      }
    });

    const unsubCmds = EventsOn('agent:available_commands', (data: any) => {
      const sessId = data?.sessionId || data?.session_id;
      const cmds = data?.commands;
      if (!sessId || !Array.isArray(cmds)) return;
      const store = useWorkspace.getState();
      store.setSessions(prev => prev.map(s => {
        if (s.id === sessId || s.acpSessionId === sessId || s.remoteSessionId === sessId) {
          return { ...s, availableCommands: cmds };
        }
        return s;
      }));
    });

    const unsubAsk = EventsOn('agent:ask', (data: any) => {
      const sessId = data?.sessionId || data?.session_id;
      if (!sessId) return;
      const store = useWorkspace.getState();

      const rawOptions: any[] = Array.isArray(data.options) ? data.options : [];
      let options: any[] = [];
      if (rawOptions.length > 0) {
        options = rawOptions.map((opt: any, i: number) => {
          if (typeof opt === 'string') {
            return {
              id: opt,
              label: opt,
              isRecommended: i === 0,
            };
          }
          return {
            id: opt.optionId || opt.id || `opt_${i + 1}`,
            label: opt.name || opt.label || opt.optionId || `Option ${i + 1}`,
            description: opt.description || (opt.kind === 'allow_once' ? 'Allow this time only' : opt.kind === 'allow_always' ? 'Always allow this tool in this session' : undefined),
            isRecommended: i === 0 || opt.kind === 'allow_once' || !!opt.isRecommended,
          };
        });
      } else {
        options = [
          { id: 'allow', label: 'Allow execution', description: 'Proceed with requested tool run', isRecommended: true },
          { id: 'deny', label: 'Deny execution', description: 'Reject and cancel this tool call' },
        ];
      }

      const qItem = {
        id: data.requestId || `ask_${Date.now()}`,
        header: (data.tool || 'PERMISSION').toUpperCase(),
        question: data.title || data.desc || `Agent requests permission to run ${data.tool || 'tool'}`,
        subtitle: data.desc && data.title ? data.desc : 'Please choose an option to continue.',
        options,
        isMultiSelect: false,
        allowCustom: true,
        acpRequestId: data.requestId,
      };

      store.setSessions(prev => prev.map(s => {
        if (s.id === sessId || s.acpSessionId === sessId || s.remoteSessionId === sessId || s.goSessionId === sessId) {
          const current = s.pendingQuestions || [];
          const filtered = current.filter((q: any) => q.id !== qItem.id && q.acpRequestId !== qItem.acpRequestId);
          return {
            ...s,
            goState: 'awaiting_input',
            pendingQuestions: [...filtered, qItem],
          };
        }
        return s;
      }));
    });

    return () => {
      unsubUpdated?.();
      unsubCmds?.();
      unsubAsk?.();
    };
  }, []);

  // 14. Dev benchmark harness registration
  useEffect(() => {
    registerBenchStore({
      openFile: path => useWorkspace.getState().openFileInEditor(path),
      closeActiveTab: () => {
        const cur = useWorkspace.getState().activeTabId;
        if (cur) useWorkspace.getState().closeTab(cur);
      },
      activeTabPath: () => {
        const store = useWorkspace.getState();
        return store.openTabs.find(t => t.id === store.activeTabId)?.filePath ?? null;
      },
      tabCount: () => useWorkspace.getState().openTabs.length,
      refreshFiles: () => useWorkspace.getState().refreshFiles()
    });
  }, []);

  // 15. Global browser handler
  useEffect(() => {
    const openSideBrowser = (url?: string) => {
      useWorkspace.getState().setIsRightActionDrawerOpen(true);
      if (typeof window !== 'undefined') {
        window.dispatchEvent(new CustomEvent('forge:open-sidepane-browser', { detail: { url } }));
      }
    };
    setOpenBrowserHandler(openSideBrowser);
    return () => {
      setOpenBrowserHandler(null);
    };
  }, []);

  // 16. Global Keyboard Shortcuts
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      const mod = e.metaKey || e.ctrlKey;
      const key = e.key.toLowerCase();
      const store = useWorkspace.getState();

      if (mod && e.shiftKey && key === 'p') {
        e.preventDefault();
        store.openCommandPalette('commands');
      } else if (mod && e.shiftKey && key === 'o') {
        e.preventDefault();
        store.openCommandPalette('symbols');
      } else if (mod && e.shiftKey && key === 'f') {
        e.preventDefault();
        store.setMode('editor');
        store.setIsGitClientOpen(false);
        store.setIsLeftSidebarOpen(true);
        store.setActiveActivity('search');
      } else if (mod && e.shiftKey && key === 'e') {
        e.preventDefault();
        store.setMode('editor');
        store.setIsGitClientOpen(false);
        store.setIsLeftSidebarOpen(true);
        store.setActiveActivity('explorer');
      } else if (mod && e.shiftKey && key === 'g') {
        e.preventDefault();
        store.setMode('editor');
        store.setIsGitClientOpen(false);
        store.setIsLeftSidebarOpen(true);
        store.setActiveActivity('git');
      } else if (mod && e.shiftKey && key === 'j') {
        e.preventDefault();
        store.setIsRightActionDrawerOpen(prev => {
          if (!prev) store.setIsLeftSidebarOpen(false);
          return !prev;
        });
      } else if (e.ctrlKey && e.key === 'Tab') {
        e.preventDefault();
        store.cycleTab?.(e.shiftKey ? -1 : 1);
      } else if (mod && key === 'k') {
        e.preventDefault();
        store.openCommandPalette('files');
      } else if (mod && key === 'p') {
        e.preventDefault();
        store.openCommandPalette('files');
      } else if (mod && key === 'n') {
        e.preventDefault();
        useSessionStore.getState().openNewTask();
      } else if (mod && key === 't') {
        e.preventDefault();
        useSessionStore.getState().openNewTask();
      } else if (e.altKey && key === 't') {
        e.preventDefault();
        useSessionStore.getState().openNewTask();
      } else if (mod && e.key === '1') {
        e.preventDefault();
        store.switchSurface('agent');
      } else if (mod && e.key === '2') {
        e.preventDefault();
        store.switchSurface('editor');
      } else if (mod && key === 'b') {
        e.preventDefault();
        store.setIsLeftSidebarOpen(prev => !prev);
      } else if (mod && e.key === '/') {
        e.preventDefault();
        store.toggleTheme();
      } else if (mod && e.key === ',') {
        e.preventDefault();
        store.openSettingsTab('general');
      } else if (mod && key === 'o') {
        e.preventDefault();
        void store.openFolder();
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, []);
}
