import React, { createContext, useContext, useState, useEffect, useCallback, useRef, useMemo } from 'react';
import {
  WorkspaceMode,
  ThemeMode,
  AgentSession,
  AgentMessage,
  FileItem,
  EditorTab,
  ActivityBarItem,
  CommandPaletteMode,
  LSPDiagnostic,
  AgentExecutionMode,
  AgentReasoningLevel,
  AgentEngineType,
  ACPAgent,
  PrivacySettings,
  FileDiff,
  LLMProviderConfig,
  MCPEntry,
  SkillEntry,
  ContextUsageInfo,
  PluginInfo,
  CreatePluginRequest,
  CreateSkillRequest,
  AgentMemoryEntry,
  CustomSlashCommand,
  AgentHookConfig,
  BrowserUseSettings,
  ComputerUseSettings,
  IndexingStatusInfo,
  ThoughtStep,
  ToolExecution,
  TextBlock,
  WorkspaceEntry,
  WindowWorkspaceState,
  TaskKind,
  TaskStatus,
  SessionTaskItem,
  SessionProgress,
  BoardTask,
  BoardTaskStatus,
  BoardTaskPriority,
  JiraTask,
  JiraTaskStatus,
  JiraTaskPriority
} from '../types';
import { BuiltInThemeId } from '../lib/themePalettes';
import { applyTheme, getStoredThemePalette, saveThemePreference } from '../lib/themeEngine';
import { DEFAULT_AGENTS, DEFAULT_PRIVACY, DEFAULT_PROVIDERS } from './agentRegistryStore';
import { AgentEngine } from '../services/agentEngine';
import { ApiBridge } from '../services/apiBridge';
import { useUIStore } from '../hooks/store';
import { registerBenchStore } from '../lib/devBench';
import { EventsOn, StopAgentTurn, SetAgentAutoApprove, SetActiveModel, SetActiveReasoningEffort, GetProviderProfiles, SyncAgentProviders, SaveProviderProfiles, DeleteProviderProfile, CompactAgentSession, CreateShell, StopSession, OpenNewWindow, GetAgentSession, ToggleAgentTask, ClearAgentTasks, SendAgentMessage, GetRecentProjects, RemoveRecent, ListMCPServers, SaveMCPServer, DeleteMCPServer, ListConnectedMCPTools, ReconnectMCP, CloseWorkspace, ACPStartSession, ACPPrompt, ACPCancel, type Automation as ZAutomation, type AutomationSaveInput as ZAutomationSaveInput } from '../lib/wails';
import { showToast } from '../lib/toast';
import { setOpenBrowserHandler } from '../lib/browser';
import { formatDisplayTitle, parseFilePath, cleanRawToolTags } from '../lib/utils';
import { goAgentSessions, extractMentionedPaths, newThoughtStep } from '../services/goAgentSession';
import { useSessionStore } from './sessionStore';
import {
  DEFAULT_SLASH_COMMANDS,
  DEFAULT_HOOKS,
  DEFAULT_BROWSER_SETTINGS,
  DEFAULT_COMPUTER_SETTINGS,
} from './workspaceConstants';
import {
  isGoHarnessSession,
  normalizeLoadedSession,
  convertGoSessionToAgentSession,
} from '../features/agent/sessionConverters';
import {
  MAX_EDITOR_FILE_BYTES,
  MAX_SIDE_PREVIEW_TEXT_BYTES,
  BINARY_EXTENSIONS,
  ZIP_EXTENSIONS,
  IMAGE_EXTENSIONS,
  isZipFile,
  isImageFile,
  isDatabaseFile,
  isBinaryFile,
} from '../features/filesystem/fileTypes';
import { useBoardStore } from './boardStore';
import { useIntegrationsStore } from './integrationsStore';
import { useProviderStore } from './providerStore';
import { useGitStore } from './gitStore';
import { useTerminalStore } from './terminalStore';
import { useLayoutStore } from './layoutStore';
import { calculateContextUsage } from '../features/agent/contextUsage';
import { runSideConversationPrompt } from '../features/agent/sideConversationRunner';
import { createAgentCallbacks } from '../features/agent/agentCallbacks';
import { streamAgentSessionDirectly } from '../features/agent/directStreamRunner';
import {
  pushNavHistory,
  getNavBack,
  getNavForward,
  cycleActiveTab,
  calculateActiveTabOnClose,
  filterTabsOnClose,
  updateTabBuffer,
  updateTabsOnRename,
  updateTabsOnDelete,
} from '../features/editor/tabNavigation';
import {
  findFileInTree,
  updateFolderChildrenInTree,
  updateFileContentInTree,
  resolveWorkspacePath,
} from '../features/filesystem/fileOperations';

export {
  DEFAULT_SLASH_COMMANDS,
  DEFAULT_HOOKS,
  DEFAULT_BROWSER_SETTINGS,
  DEFAULT_COMPUTER_SETTINGS,
  isGoHarnessSession,
  normalizeLoadedSession,
  convertGoSessionToAgentSession,
  MAX_EDITOR_FILE_BYTES,
  MAX_SIDE_PREVIEW_TEXT_BYTES,
  BINARY_EXTENSIONS,
  ZIP_EXTENSIONS,
  IMAGE_EXTENSIONS,
  isZipFile,
  isImageFile,
  isDatabaseFile,
  isBinaryFile,
  useBoardStore,
  useIntegrationsStore,
  useProviderStore,
  useGitStore,
  useTerminalStore,
  useLayoutStore,
  calculateContextUsage,
  findFileInTree,
  updateFolderChildrenInTree,
  updateFileContentInTree,
  resolveWorkspacePath,
  pushNavHistory,
  getNavBack,
  getNavForward,
  cycleActiveTab,
  calculateActiveTabOnClose,
  filterTabsOnClose,
  updateTabBuffer,
  updateTabsOnRename,
  updateTabsOnDelete,
};

interface WorkspaceContextType {
  mode: WorkspaceMode;
  setMode: (mode: WorkspaceMode) => void;
  /** Which task kind the unified workspace surface renders. */
  activeTaskKind: TaskKind;
  setActiveTaskKind: (kind: TaskKind) => void;
  theme: ThemeMode;
  setTheme: (theme: ThemeMode) => void;
  toggleTheme: (next?: ThemeMode) => void;
  themePalette: BuiltInThemeId;
  setThemePalette: (palette: BuiltInThemeId) => void;
  
  // Recent Workspaces
  recentWorkspaces: string[];
  addRecentWorkspace: (path: string) => void;
  removeRecentWorkspace: (path: string) => void;
  clearRecentWorkspaces: () => void;
  closeWorkspace: () => void;
  disconnectSSH: () => Promise<void>;
  
  // Agent Sessions
  activeSessionId: string | null;
  setActiveSessionId: (id: string | null) => void;
  sessions: AgentSession[];
  activeSession: AgentSession | undefined;

  // Custom Agents & ACP
  agents: ACPAgent[];
  activeAgentId: string;
  setActiveAgentId: (id: string) => void;
  activeAgent: ACPAgent;
  toggleAgentEnabled: (id: string) => void;
  updateAgentConfig: (id: string, updates: Partial<ACPAgent>) => void;
  refreshAgentModels: (agentId?: string) => Promise<void>;
  addAgent: (agent: ACPAgent) => void;
  deleteAgent: (id: string) => void;
  acpEnabled: boolean;
  setAcpEnabled: (enabled: boolean) => void;
  privacySettings: PrivacySettings;
  setPrivacySettings: (settings: PrivacySettings | ((prev: PrivacySettings) => PrivacySettings)) => void;

  // LLM Providers & Models Config
  providers: LLMProviderConfig[];
  updateProvider: (id: string, updates: Partial<LLMProviderConfig>) => void;
  addProvider: (provider: LLMProviderConfig) => void;
  deleteProvider: (id: string) => void;
  addModelToProvider: (providerId: string, modelName: string) => void;
  deleteModelFromProvider: (providerId: string, modelName: string) => void;
  toggleModelSelection: (providerId: string, modelName: string) => void;
  selectAllModels: (providerId: string, modelsToSelect?: string[]) => void;
  deselectAllModels: (providerId: string, modelsToDeselect?: string[]) => void;
  fetchProviderModels: (providerId: string) => Promise<void>;

  // MCPs & Skills Discovery
  mcps: MCPEntry[];
  addMcp: (mcp: MCPEntry) => void;
  toggleMcp: (id: string) => void;
  deleteMcp: (id: string) => void;
  skills: SkillEntry[];
  addSkill: (skill: SkillEntry) => void;
  toggleSkill: (id: string) => void;
  deleteSkill: (id: string) => void;
  createBackendSkill: (req: CreateSkillRequest) => Promise<any>;
  deleteBackendSkill: (name: string) => Promise<void>;
  reloadSkills: () => Promise<void>;
  discoveredMcps: any[];
  discoveredSkills: any[];
  isDiscovering: boolean;
  runDiscovery: () => Promise<void>;
  importDiscoveredMcp: (disc: any) => void;
  importDiscoveredSkill: (disc: any) => void;

  // Plugins System
  plugins: PluginInfo[];
  fetchPlugins: () => Promise<void>;
  createPlugin: (req: CreatePluginRequest) => Promise<PluginInfo | null>;
  togglePlugin: (id: string, enabled?: boolean) => Promise<void>;
  deletePlugin: (id: string) => Promise<void>;
  reloadPlugins: () => Promise<void>;

  // Agent Long-Term Memory
  memories: AgentMemoryEntry[];
  fetchMemories: () => Promise<void>;
  saveMemory: (entry: AgentMemoryEntry) => Promise<void>;
  deleteMemory: (id: string) => Promise<void>;
  reloadMemories: () => Promise<void>;

  // Custom Slash Commands
  customCommands: CustomSlashCommand[];
  addCustomCommand: (cmd: CustomSlashCommand) => void;
  updateCustomCommand: (id: string, updates: Partial<CustomSlashCommand>) => void;
  deleteCustomCommand: (id: string) => void;
  toggleCustomCommand: (id: string) => void;

  // Lifecycle Hooks
  hooks: AgentHookConfig[];
  updateHook: (id: string, updates: Partial<AgentHookConfig>) => void;
  toggleHook: (id: string) => void;
  addHook: (hook: AgentHookConfig) => void;
  deleteHook: (id: string) => void;

  // Browser & Computer Use Settings
  browserSettings: BrowserUseSettings;
  updateBrowserSettings: (updates: Partial<BrowserUseSettings>) => void;
  computerSettings: ComputerUseSettings;
  updateComputerSettings: (updates: Partial<ComputerUseSettings>) => void;

  // Automations (saved prompt workflows)
  automations: ZAutomation[];
  automationsLoading: boolean;
  fetchAutomations: () => Promise<void>;
  saveAutomation: (automation: ZAutomationSaveInput) => Promise<ZAutomation>;
  deleteAutomation: (id: string) => Promise<void>;
  setAutomationEnabled: (id: string, enabled: boolean) => Promise<void>;
  runAutomation: (automation: ZAutomation) => string | undefined;

  // Codebase Indexing
  indexStatus: IndexingStatusInfo | null;
  fetchIndexStatus: () => Promise<void>;
  reindexWorkspace: () => Promise<{ built: boolean; symbols?: number }>;
  isReindexing: boolean;

  // Agent input configuration
  agentExecutionMode: AgentExecutionMode;
  setAgentExecutionMode: (mode: AgentExecutionMode) => void;
  reasoningLevel: AgentReasoningLevel;
  setReasoningLevel: (level: AgentReasoningLevel) => void;
  agentEngine: AgentEngineType;
  setAgentEngine: (engine: AgentEngineType) => void;
  contextUsage: ContextUsageInfo;

  // Sidebars (Separated per Mode)
  isLeftSidebarOpen: boolean;
  setIsLeftSidebarOpen: (val: boolean | ((prev: boolean) => boolean)) => void;
  agentLeftSidebarOpen: boolean;
  setAgentLeftSidebarOpen: (val: boolean | ((prev: boolean) => boolean)) => void;
  agentRightSidebarOpen: boolean;
  setAgentRightSidebarOpen: (val: boolean | ((prev: boolean) => boolean)) => void;
  editorLeftSidebarOpen: boolean;
  setEditorLeftSidebarOpen: (val: boolean | ((prev: boolean) => boolean)) => void;
  editorRightSidebarOpen: boolean;
  setEditorRightSidebarOpen: (val: boolean | ((prev: boolean) => boolean)) => void;
  leftSidebarWidth: number;
  setLeftSidebarWidth: (width: number) => void;
  rightPaneWidth: number;
  setRightPaneWidth: (width: number) => void;

  // Agent Mode Views (Session vs Jira Board vs Git Client)
  agentViewMode: 'session' | 'board' | 'git';
  setAgentViewMode: (mode: 'session' | 'board' | 'git') => void;

  // Workflow Task Board
  boardTasks: BoardTask[];
  refreshBoardTasks: () => Promise<void>;
  addBoardTask: (task: Omit<BoardTask, 'id' | 'key' | 'createdAt' | 'updatedAt'>) => BoardTask;
  updateBoardTask: (id: string, patch: Partial<BoardTask>) => void;
  deleteBoardTask: (id: string) => void;
  moveBoardTaskStatus: (id: string, status: BoardTaskStatus) => void;

  // Legacy Jira aliases
  jiraTasks: BoardTask[];
  refreshJiraTasks: () => Promise<void>;
  addJiraTask: (task: Omit<BoardTask, 'id' | 'key' | 'createdAt' | 'updatedAt'>) => BoardTask;
  updateJiraTask: (id: string, patch: Partial<BoardTask>) => void;
  deleteJiraTask: (id: string) => void;
  moveJiraTaskStatus: (id: string, status: BoardTaskStatus) => void;

  // Dedicated Git Client View / Modal
  isGitClientOpen: boolean;
  setIsGitClientOpen: (val: boolean | ((prev: boolean) => boolean)) => void;

  // Side conversation (sub chat session)
  sendSideConversationPrompt: (promptText: string, model?: string, mode?: 'ask' | 'plan' | 'full') => Promise<void>;
  clearSideConversation: () => void;

  // Diffs & Review System
  diffs: FileDiff[];
  activeDiff: FileDiff | null;
  setActiveDiff: (diff: FileDiff | null) => void;
  acceptDiff: (diffId: string) => void;
  rejectDiff: (diffId: string) => void;
  addDiff: (diff: FileDiff) => void;
  openDiffInEditor: (diff: FileDiff) => void;
  openGitGraphPane: () => void;

  // Git State
  gitBranch: string;
  gitFiles: Array<{ path: string; status: string; staging?: 'staged' | 'unstaged' | 'untracked'; dir?: string; additions?: number; deletions?: number }>;
  gitCommits: any[];
  refreshGitStatus: () => Promise<void>;
  refreshGitLog: () => Promise<void>;

  // Workspace / Files
  files: FileItem[];
  openTabs: EditorTab[];
  activeTabId: string | null;
  selectedFile: FileItem | null;
  isSplitEditor: boolean;
  setIsSplitEditor: (val: boolean | ((prev: boolean) => boolean)) => void;
  createFile: (filePath: string, content?: string) => Promise<void>;
  createFolder: (folderPath: string) => Promise<void>;
  deleteFile: (filePath: string) => Promise<void>;
  renameFile: (oldPath: string, newPath: string) => Promise<void>;
  moveFile: (srcPath: string, destPath: string) => Promise<void>;
  copyFile: (srcPath: string, destPath: string) => Promise<void>;
  openFolder: (path?: string) => Promise<void>;
  refreshFiles: () => Promise<void>;
  revalidateFileMutations: () => Promise<void>;

  // Folder modal
  isFolderModalOpen: boolean;
  setIsFolderModalOpen: (val: boolean) => void;

  // SSH Remote modal
  isSSHModalOpen: boolean;
  setIsSSHModalOpen: (val: boolean) => void;

  // Remote File Uploader modal
  isRemoteUploadModalOpen: boolean;
  setIsRemoteUploadModalOpen: (val: boolean) => void;
  remoteUploadTargetDir: string;
  setRemoteUploadTargetDir: (val: string) => void;
  openRemoteUploader: (targetDir?: string) => void;

  // Navigation & Activities
  activeActivity: ActivityBarItem;
  setActiveActivity: (activity: ActivityBarItem) => void;

  // Right quick drawer
  isRightActionDrawerOpen: boolean;
  setIsRightActionDrawerOpen: (val: boolean | ((prev: boolean) => boolean)) => void;

  // Left sidebar view mode: task list or ZCode-style file explorer
  leftSidebarView: 'tasks' | 'files';
  setLeftSidebarView: (v: 'tasks' | 'files') => void;

  // Files opened from the explorer, shown as tabs in the right side pane
  sideFileTabs: Array<{ path: string; name: string; line?: number }>;
  activeSideFile: string | null;
  activeSideFileLine?: number;
  openSideFile: (path: string, line?: number) => void;
  closeSideFile: (path: string) => void;
  openSideBrowser: (url?: string) => void;

  // Bottom dock terminal (header "..." menu owns the toggle)
  isBottomTerminalOpen: boolean;
  setIsBottomTerminalOpen: (val: boolean | ((prev: boolean) => boolean)) => void;

  // Models & Modals
  currentModel: string;
  setCurrentModel: (model: string) => void;
  isCommandPaletteOpen: boolean;
  setIsCommandPaletteOpen: (val: boolean) => void;
  /** Automations modal overlay (opened from the sidebar, closed on run/Esc). */
  isAutomationsOpen: boolean;
  setIsAutomationsOpen: (val: boolean | ((prev: boolean) => boolean)) => void;
  /** Which mode the palette opens in: files (quick open), commands (>), symbols (@), line (:). */
  commandPaletteMode: CommandPaletteMode;
  openCommandPalette: (mode?: CommandPaletteMode) => void;
  /** Files activated most recently across all workspaces (quick-open ranking); newest first. */
  recentFiles: Array<{ path: string; ts: number }>;
  isCustomizationsModalOpen: boolean;
  setIsCustomizationsModalOpen: (val: boolean) => void;
  isSettingsModalOpen: boolean;
  setIsSettingsModalOpen: (val: boolean) => void;

  // Unified Settings Pane
  settingsActiveSection: string;
  setSettingsActiveSection: (sec: string) => void;
  previousMode: WorkspaceMode;
  setPreviousMode: (mode: WorkspaceMode) => void;
  goBackToWorkspace: () => void;

  // Actions
  openFileInEditor: (filePath: string, line?: number, column?: number) => Promise<void>;
  updateFolderChildren: (folderPath: string, children: FileItem[]) => void;
  openSettingsTab: (section?: string) => void;
  openTerminalTab: (shellId?: string, cwd?: string) => Promise<void>;
  closeTab: (tabId: string) => void;
  /** Bulk close (tab context menu) — one atomic update. */
  closeTabs: (ids: string[]) => void;
  /** Tab menu toggles: pinned tabs survive bulk closes; read-only blocks edits. */
  toggleTabPin: (tabId: string) => void;
  toggleTabReadOnly: (tabId: string) => void;
  setActiveTabId: (tabId: string | null) => void;
  openTab: (tab: EditorTab) => void;
  goBack: () => void;
  goForward: () => void;
  canGoBack: boolean;
  canGoForward: boolean;
  updateFileContent: (fileId: string, newContent: string) => Promise<void>;
  /** Editor buffer update: marks the tab dirty WITHOUT touching the disk
      (VS Code semantics — the disk write happens on save). */
  setTabBuffer: (fileId: string, content: string) => void;
  /** Writes the editor buffer to disk and clears the dirty flag. */
  saveTabToDisk: (fileId: string, content: string) => Promise<void>;
  setDiagnostics: (diags: LSPDiagnostic[]) => void;
  createNewSession: (initialPrompt?: string, agentId?: string, forcedId?: string) => string;
  sendAgentPrompt: (promptText: string) => void;
  steerAgentPrompt: (instruction: string) => Promise<void>;
  stopAgentExecution: () => void;
  compactCurrentSession: () => Promise<void>;
  deleteSession: (id: string) => void;

  // Persistent Sessions & History
  savedSessions: AgentSession[];
  openSessionFromHistory: (session: AgentSession) => void;
  deleteSessionPermanently: (id: string) => Promise<void>;
  reloadSavedSessions: () => Promise<void>;
  archivedSessionIds: Set<string>;
  toggleArchiveSession: (id: string) => void;

  // Path info & Multi-Workspace Window (Zed-style)
  activeWorkspacePath: string;
  agentWorkspacePath: string;
  editorWorkspacePath: string;
  setActiveWorkspacePath: (path: string) => void;
  activeWorkspaces: WorkspaceEntry[];
  switchWorkspace: (path: string) => Promise<void>;
  /** Switch between the Agent and Editor surfaces, carrying the folder across. */
  switchSurface: (target: 'agent' | 'editor') => void;
  closeWorkspaceFromWindow: (path: string) => Promise<void>;
  openWorkspaceInNewWindow: (path: string) => Promise<void>;

  // LSP
  diagnostics: LSPDiagnostic[];

  // Subagents & Floating Task Tracker
  subagents: AgentSession[];
  activeSubagentId: string | null;
  setActiveSubagentId: (id: string | null) => void;
  openSubagentInSidebar: (subagentId: string) => void;
  toggleTask: (taskId: string, completed: boolean) => Promise<void>;
  clearTasks: () => Promise<void>;
  sendSubagentMessage: (subagentId: string, prompt: string) => Promise<void>;
}

const WorkspaceContext = createContext<WorkspaceContextType | undefined>(undefined);

// ── Editor session persistence (restore tabs across app restarts) ───────────
// Only code-tab METADATA is persisted — never buffer content (buffers can be
// huge, and dirty buffers would silently resurrect stale text). Terminal and
// diff tabs die with the process by design. Content is re-read from disk on
// restore via openFileInEditor.
// (File classification constants and predicates are imported from features/filesystem/fileTypes)

const sessionKeyFor = (wsPath: string) => `forge_ade_editor_session:${wsPath}`;

interface PersistedEditorSession {
  tabs: EditorTab[];
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

let globalOpenSideFileFn: ((rawPath: string, line?: number) => void) | null = null;

export function globalOpenSideFile(rawPath: string, line?: number) {
  if (globalOpenSideFileFn) {
    globalOpenSideFileFn(rawPath, line);
  }
}

export const WorkspaceProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  // Agent Mode and Editor Mode are separate surfaces (each with its own
  // workspace folder); tasks of kind 'forge' (chat) or 'cli' (terminal) pick
  // the view inside the Agent surface.
  const [mode, setModeState] = useState<WorkspaceMode>('agent');
  const [previousMode, setPreviousMode] = useState<WorkspaceMode>('agent');

  // Keyboard handlers and setActiveWorkspacePath routing read the live mode.
  // Kept in sync synchronously (not via effect) so calls that switch surface
  // and then set the workspace in the same tick route to the NEW surface.
  const modeRef = useRef<WorkspaceMode>('agent');

  useEffect(() => {
    modeRef.current = mode;
  }, [mode]);

  // Theme hydrates from the persisted choice so a light session survives an
  // app restart; the Appearance toggle can still switch it live. 'system'
  // follows the OS and is resolved by the theme engine.
  const [theme, setTheme] = useState<ThemeMode>(() => {
    if (typeof window === 'undefined') return 'dark';
    const stored = localStorage.getItem('forge-ade-theme');
    return stored === 'light' || stored === 'system' ? stored : 'dark';
  });

  const [themePalette, setThemePaletteState] = useState<BuiltInThemeId>(() => getStoredThemePalette());
  const setThemePalette = useCallback((p: BuiltInThemeId) => {
    setThemePaletteState(p);
    saveThemePreference(p, theme);
  }, [theme]);

  // Which task kind the workspace surface renders: 'forge' = internal
  // ForgeADE agent chat, 'cli' = external agent CLI in a terminal.
  const [activeTaskKind, setActiveTaskKindState] = useState<TaskKind>(
    () => (localStorage.getItem('forge_ade_task_kind') === 'cli' ? 'cli' : 'forge')
  );
  const setActiveTaskKind = useCallback((kind: TaskKind) => {
    setActiveTaskKindState(kind);
    try { localStorage.setItem('forge_ade_task_kind', kind); } catch {}
  }, []);

  // Terminal Session boot: load application settings (terminal rendering) and
  // the persisted session history once at startup.
  const bootModeAppliedRef = useRef(false);
  useEffect(() => {
    if (bootModeAppliedRef.current) return;
    bootModeAppliedRef.current = true;
    const sessionStore = useSessionStore.getState();
    void sessionStore.load();
    void sessionStore.loadAgentConfigs();
    void sessionStore.loadSettings();
  }, []);

  const parseWorkspaceEntry = useCallback((wsPath: string): WorkspaceEntry => {
    const isRemote = wsPath.startsWith('ssh://');
    if (isRemote) {
      const rest = wsPath.replace(/^ssh:\/\//, '');
      const slashIdx = rest.indexOf('/');
      const hostPart = slashIdx !== -1 ? rest.substring(0, slashIdx) : rest;
      const remoteDir = slashIdx !== -1 ? rest.substring(slashIdx) : '/';
      const folderName = remoteDir.split('/').filter(Boolean).pop() || 'remote-root';
      const cleanHost = hostPart.includes('@') ? hostPart.split('@')[1] : hostPart;
      return {
        path: wsPath,
        name: folderName,
        type: 'remote',
        host: cleanHost.split(':')[0],
        lastActive: Date.now()
      };
    }
    const cleanPath = wsPath.replace(/\/+$/, '');
    const folderName = cleanPath.split('/').filter(Boolean).pop() || wsPath || 'workspace';
    return {
      path: wsPath,
      name: folderName,
      type: 'local',
      lastActive: Date.now()
    };
  }, []);

  // Helper to purge legacy dummy paths if any
  const isInvalidHardcodedPath = useCallback((_p: string | null | undefined): boolean => {
    return false;
  }, []);

  const getCleanStoredPath = useCallback((key: string): string => {
    try {
      const val = localStorage.getItem(key);
      if (val && isInvalidHardcodedPath(val)) {
        localStorage.removeItem(key);
        return '';
      }
      return val || '';
    } catch {
      return '';
    }
  }, [isInvalidHardcodedPath]);

  // Agent Mode and Editor Mode are separate surfaces, each with its own
  // workspace folder. Switching surfaces carries the folder across: the
  // agent's folder opens in the editor and vice versa (see switchSurface).
  const [agentWorkspacePath, setAgentWorkspacePathState] = useState<string>(() => {
    return getCleanStoredPath('forge_ade_agent_workspace')
      || getCleanStoredPath('forge_ade_workspace_path')
      || getCleanStoredPath('my_ade_workspace_path')
      || '';
  });
  const [editorWorkspacePath, setEditorWorkspacePathState] = useState<string>(() => {
    return getCleanStoredPath('forge_ade_editor_workspace')
      || getCleanStoredPath('forge_ade_workspace_path')
      || getCleanStoredPath('my_ade_workspace_path')
      || '';
  });
  // The folder every surface operates on follows the active mode.
  const activeWorkspacePath = mode === 'editor' ? editorWorkspacePath : agentWorkspacePath;
  const agentWsPathRef = useRef(agentWorkspacePath);
  const editorWsPathRef = useRef(editorWorkspacePath);
  useEffect(() => { agentWsPathRef.current = agentWorkspacePath; }, [agentWorkspacePath]);
  useEffect(() => { editorWsPathRef.current = editorWorkspacePath; }, [editorWorkspacePath]);

  // Multi-workspace window support (Zed-style: multiple workspaces alive in one window)
  const [activeWorkspaces, setActiveWorkspaces] = useState<WorkspaceEntry[]>(() => {
    const initialPath = getCleanStoredPath('forge_ade_workspace_path') || getCleanStoredPath('my_ade_workspace_path') || '';
    if (!initialPath) return [];
    return [parseWorkspaceEntry(initialPath)];
  });

  // Cache editor state (tabs, active tab, selected file, diffs) per workspace path
  const workspaceStateCacheRef = useRef<Map<string, WindowWorkspaceState>>(new Map());

  const openTabsRef = useRef<EditorTab[]>([]);
  const activeTabIdRef = useRef<string | null>(null);
  const selectedFileRef = useRef<FileItem | null>(null);
  const diffsRef = useRef<FileDiff[]>([]);
  const activeDiffRef = useRef<FileDiff | null>(null);

  // Mirrors activeWorkspacePath so async workspace loads can detect a switch
  // that happened mid-flight and discard the stale response.
  const wsPathRef = useRef(activeWorkspacePath);
  useEffect(() => {
    wsPathRef.current = activeWorkspacePath;
  }, [activeWorkspacePath]);

  const isFolderModalOpen = useLayoutStore(s => s.isFolderModalOpen);
  const setIsFolderModalOpen = useLayoutStore(s => s.setIsFolderModalOpen);
  const isSSHModalOpen = useLayoutStore(s => s.isSSHModalOpen);
  const setIsSSHModalOpen = useLayoutStore(s => s.setIsSSHModalOpen);
  const isRemoteUploadModalOpen = useLayoutStore(s => s.isRemoteUploadModalOpen);
  const setIsRemoteUploadModalOpen = useLayoutStore(s => s.setIsRemoteUploadModalOpen);
  const remoteUploadTargetDir = useLayoutStore(s => s.remoteUploadTargetDir);
  const setRemoteUploadTargetDir = useLayoutStore(s => s.setRemoteUploadTargetDir);

  const openRemoteUploader = useCallback((targetDir?: string) => {
    setRemoteUploadTargetDir(targetDir || activeWorkspacePath || '');
    setIsRemoteUploadModalOpen(true);
  }, [activeWorkspacePath]);

  // Recent Workspaces
  const [recentWorkspaces, setRecentWorkspaces] = useState<string[]>(() => {
    try {
      const saved = localStorage.getItem('forge_ade_recent_workspaces') || localStorage.getItem('my_ade_recent_workspaces');
      if (saved) {
        const parsed = JSON.parse(saved);
        if (Array.isArray(parsed)) {
          const cleaned = parsed.filter(p => typeof p === 'string' && !isInvalidHardcodedPath(p));
          if (cleaned.length !== parsed.length) {
            try { localStorage.setItem('forge_ade_recent_workspaces', JSON.stringify(cleaned)); } catch {}
          }
          return cleaned;
        }
      }
    } catch {}
    return [];
  });

  // Sync recent workspaces from backend manager on mount
  useEffect(() => {
    void GetRecentProjects().then(entries => {
      if (Array.isArray(entries) && entries.length > 0) {
        const paths = entries
          .map((e: any) => e?.path || e?.Path)
          .filter((p: any) => typeof p === 'string' && p.trim() && !isInvalidHardcodedPath(p));
        if (paths.length > 0) {
          setRecentWorkspaces(prev => {
            const merged = Array.from(new Set([...prev, ...paths])).slice(0, 15);
            try { localStorage.setItem('forge_ade_recent_workspaces', JSON.stringify(merged)); } catch {}
            return merged;
          });
        }
      }
    }).catch(() => {});
  }, [isInvalidHardcodedPath]);

  const addRecentWorkspace = useCallback((path: string) => {
    if (!path || !path.trim()) return;
    setRecentWorkspaces(prev => {
      const filtered = prev.filter(p => p !== path);
      const next = [path, ...filtered].slice(0, 15);
      try { localStorage.setItem('forge_ade_recent_workspaces', JSON.stringify(next)); } catch {}
      return next;
    });
  }, []);

  const removeRecentWorkspace = useCallback((pathToRemove: string) => {
    void RemoveRecent(pathToRemove).catch(() => {});
    setRecentWorkspaces(prev => {
      const next = prev.filter(p => p !== pathToRemove);
      try { localStorage.setItem('forge_ade_recent_workspaces', JSON.stringify(next)); } catch {}
      return next;
    });
  }, []);

  const clearRecentWorkspaces = useCallback(() => {
    setRecentWorkspaces([]);
    try { localStorage.setItem('forge_ade_recent_workspaces', JSON.stringify([])); } catch {}
  }, []);

  // ── Recent files (quick-open ranking) ───────────────────────────────────────
  // One global list; the palette filters entries down to the active workspace
  // so paths from other folders never leak into its suggestions.
  const [recentFiles, setRecentFiles] = useState<Array<{ path: string; ts: number }>>(() => {
    try {
      const saved = JSON.parse(localStorage.getItem('forge_ade_recent_files') || '[]');
      if (Array.isArray(saved)) {
        return saved.filter((r: any) => r && typeof r.path === 'string');
      }
    } catch {}
    return [];
  });

  const recordRecentFile = useCallback((path: string) => {
    if (!path) return;
    setRecentFiles(prev => {
      const next = [{ path, ts: Date.now() }, ...prev.filter(r => r.path !== path)].slice(0, 40);
      try { localStorage.setItem('forge_ade_recent_files', JSON.stringify(next)); } catch {}
      return next;
    });
  }, []);

  const closeWorkspace = useCallback(() => {
    // Close the active surface's workspace only.
    if (modeRef.current === 'editor') {
      const closedPath = editorWsPathRef.current;
      if (closedPath) {
        workspaceStateCacheRef.current.delete(closedPath);
      }
      editorWsPathRef.current = '';
      setEditorWorkspacePathState('');
      localStorage.removeItem('forge_ade_editor_workspace');

      // Tabs, selection, and diffs belong to the closed workspace — clear them
      // here as well as in the switch effect (which also fires on this change).
      setOpenTabs([]);
      setActiveTabId(null);
      setSelectedFile(null);
      setDiffs([]);
      setActiveDiff(null);
    } else {
      const closedPath = agentWsPathRef.current;
      if (closedPath) {
        workspaceStateCacheRef.current.delete(closedPath);
      }
      agentWsPathRef.current = '';
      setAgentWorkspacePathState('');
      localStorage.removeItem('forge_ade_agent_workspace');
    }
    localStorage.removeItem('forge_ade_workspace_path');
    localStorage.removeItem('my_ade_workspace_path');
    void CloseWorkspace().catch(() => {});
    setFiles([]);
    useGitStore.getState().setGitBranch('');
    useGitStore.getState().setGitFiles([]);
    useGitStore.getState().setGitCommits([]);
  }, []);

  const disconnectSSH = useCallback(async () => {
    if (activeWorkspacePath && activeWorkspacePath.startsWith('ssh://')) {
      const trimmed = activeWorkspacePath.replace(/^ssh:\/\//, '');
      const connId = trimmed.split('/')[0];
      if (connId) {
        await ApiBridge.disconnectSSH(connId);
      }
    }
    closeWorkspace();
  }, [activeWorkspacePath, closeWorkspace]);

  // Seamless Zed-style workspace switching preserving tabs per workspace.
  // `surface` picks which mode's folder changes (agent/editor keep separate
  // workspaces); the derived activeWorkspacePath drives the open/tree/git flow.
  // Path refs update synchronously so back-to-back switches in one tick see
  // the latest folders instead of this render's stale closures.
  const switchSurfaceWorkspace = useCallback(async (newPath: string, surface: 'agent' | 'editor') => {
    const current = surface === 'editor' ? editorWsPathRef.current : agentWsPathRef.current;
    if (!newPath || newPath === current) return;

    // 1. Snapshot current workspace state before leaving
    const oldPath = wsPathRef.current;
    if (oldPath && (surface === 'editor' || modeRef.current === 'editor')) {
      workspaceStateCacheRef.current.set(oldPath, {
        openTabs: [...openTabsRef.current],
        activeTabId: activeTabIdRef.current,
        selectedFile: selectedFileRef.current,
        diffs: [...diffsRef.current],
        activeDiff: activeDiffRef.current
      });
      // Each cache entry pins that workspace's open buffers in memory — cap
      // it so touring many workspaces can't accumulate all of their file
      // contents. Insertion order makes the first key the oldest.
      if (workspaceStateCacheRef.current.size > 8) {
        const oldest = workspaceStateCacheRef.current.keys().next().value;
        if (oldest !== undefined && oldest !== newPath) {
          workspaceStateCacheRef.current.delete(oldest);
        }
      }
    }

    // 2. Add / update activeWorkspaces list
    setActiveWorkspaces(prev => {
      const existing = prev.find(w => w.path === newPath);
      const entry = existing ? { ...existing, lastActive: Date.now() } : parseWorkspaceEntry(newPath);
      const remaining = prev.filter(w => w.path !== newPath);
      return [entry, ...remaining];
    });

    // 3. Restore cached state for the new workspace if available
    if (surface === 'editor') {
      const cached = workspaceStateCacheRef.current.get(newPath);
      if (cached) {
        setOpenTabs(cached.openTabs || []);
        setActiveTabId(cached.activeTabId || null);
        setSelectedFile(cached.selectedFile || null);
        setDiffs(cached.diffs || []);
        setActiveDiff(cached.activeDiff || null);
      } else {
        setOpenTabs([]);
        setActiveTabId(null);
        setSelectedFile(null);
        setDiffs([]);
        setActiveDiff(null);
      }
    }

    // 4. Update the surface's folder & trigger the backend load via the
    // derived activeWorkspacePath effect.
    wsPathRef.current = newPath;
    if (surface === 'editor') {
      editorWsPathRef.current = newPath;
      setEditorWorkspacePathState(newPath);
      localStorage.setItem('forge_ade_editor_workspace', newPath);
    } else {
      agentWsPathRef.current = newPath;
      setAgentWorkspacePathState(newPath);
      localStorage.setItem('forge_ade_agent_workspace', newPath);
    }
    localStorage.setItem('forge_ade_workspace_path', newPath);
    addRecentWorkspace(newPath);
  }, [parseWorkspaceEntry, addRecentWorkspace]);

  // Close a specific workspace tab from this window (Zed-style close button '✕')
  const closeWorkspaceFromWindow = useCallback(async (pathToRemove: string) => {
    workspaceStateCacheRef.current.delete(pathToRemove);

    if (pathToRemove.startsWith('ssh://')) {
      const trimmed = pathToRemove.replace(/^ssh:\/\//, '');
      const connId = trimmed.split('/')[0];
      if (connId) {
        void ApiBridge.disconnectSSH(connId);
      }
    }

    setActiveWorkspaces(prev => {
      const next = prev.filter(w => w.path !== pathToRemove);
      return next;
    });

    // If removing currently active workspace, switch to next available or close
    if (pathToRemove === wsPathRef.current) {
      const remaining = activeWorkspaces.filter(w => w.path !== pathToRemove);
      if (remaining.length > 0) {
        void switchSurfaceWorkspace(remaining[0].path, modeRef.current === 'editor' ? 'editor' : 'agent');
      } else {
        closeWorkspace();
      }
    }
  }, [activeWorkspaces, switchSurfaceWorkspace, closeWorkspace]);

  // Open workspace in a separate native window (Zed-style arrow '↗')
  const openWorkspaceInNewWindow = useCallback(async (targetPath: string) => {
    try {
      await OpenNewWindow(targetPath);
    } catch (e) {
      console.warn('Failed to open new window for workspace:', e);
      showToast('Failed to open new window', 'error');
    }
  }, []);

  // Routes to the active surface's folder. Callers everywhere in the app keep
  // using this single entry point; Agent and Editor each keep their own folder.
  const setActiveWorkspacePath = useCallback((newPath: string) => {
    void switchSurfaceWorkspace(newPath, modeRef.current === 'editor' ? 'editor' : 'agent');
  }, [switchSurfaceWorkspace]);

  const setMode = useCallback((newMode: WorkspaceMode | ((prev: WorkspaceMode) => WorkspaceMode)) => {
    const currentMode = modeRef.current;
    const resolved = typeof newMode === 'function' ? newMode(currentMode) : newMode;
    if (currentMode === resolved) return;

    if (currentMode === 'editor') {
      const currentPath = editorWsPathRef.current;
      if (currentPath) {
        workspaceStateCacheRef.current.set(currentPath, {
          openTabs: [...openTabsRef.current],
          activeTabId: activeTabIdRef.current,
          selectedFile: selectedFileRef.current,
          diffs: [...diffsRef.current],
          activeDiff: activeDiffRef.current
        });
      }
    } else if (resolved === 'editor') {
      const currentPath = editorWsPathRef.current;
      if (currentPath) {
        const cached = workspaceStateCacheRef.current.get(currentPath);
        if (cached) {
          setOpenTabs(cached.openTabs || []);
          setActiveTabId(cached.activeTabId || null);
          setSelectedFile(cached.selectedFile || null);
          setDiffs(cached.diffs || []);
          setActiveDiff(cached.activeDiff || null);
        }
      }
    }

    if (currentMode !== 'settings' && resolved === 'settings') {
      setPreviousMode(currentMode);
    }
    modeRef.current = resolved;
    setModeState(resolved);
  }, []);

  // Switch between the Agent and Editor surfaces. Agent Mode and Editor Mode
  // are completely disconnected: each keeps its own workspace folder, tabs, and
  // sidebars independently per user preference.
  const switchSurface = useCallback((target: 'agent' | 'editor') => {
    setMode(target);
  }, [setMode]);

  // Smart auto-renaming session helper (20-30 chars, ChatGPT/Gemini style)
  const formatSmartSessionTitle = (prompt: string): string => {
    if (!prompt || !prompt.trim()) return 'New Session';
    
    // Remove attachments or system markers
    let clean = prompt
      .replace(/\[Attached File:[\s\S]*?\]/g, '')
      .replace(/\[TOOL EXECUTION RESULTS\][\s\S]*$/g, '')
      .replace(/^["'`]|["'`]$/g, '')
      .trim();

    clean = formatDisplayTitle(clean);

    // Strip leading conversational filler words
    clean = clean
      .replace(/^(please|can you|could you|help me|i want to|how to|how do i|tell me about|explain to me|what is|create a|build a|write a|give me)\s+/i, '')
      .replace(/[`*_#~]/g, '')
      .trim();

    if (!clean) return 'New Session';

    // Capitalize first character
    clean = clean.charAt(0).toUpperCase() + clean.slice(1);

    // Limit to 20-28 characters max without cutting words awkwardly
    if (clean.length > 28) {
      const trimmed = clean.slice(0, 26);
      const lastSpace = trimmed.lastIndexOf(' ');
      if (lastSpace > 12) {
        clean = trimmed.slice(0, lastSpace).trim();
      } else {
        clean = trimmed.trim();
      }
      return `${clean}...`;
    }

    return clean;
  };

  // Persistent Real Sessions — seeded from disk/localStorage only. Demo
  // sessions were removed: they resurrected fake tasks with hardcoded paths
  // on every fresh start and got persisted into the user's real history.
  const [sessions, setSessions] = useState<AgentSession[]>(() => {
    try {
      const saved = localStorage.getItem('forge_ade_sessions') || localStorage.getItem('my_ade_sessions');
      if (saved) {
        const parsed = JSON.parse(saved);
        if (Array.isArray(parsed) && parsed.length > 0) {
          return parsed.map(normalizeLoadedSession).filter((s): s is AgentSession => s !== null);
        }
      }
    } catch {
      // ignore
    }
    return [];
  });

  const [savedSessions, setSavedSessions] = useState<AgentSession[]>([]);

  const [activeSessionId, setActiveSessionId] = useState<string | null>(() => {
    return null;
  });

  const sessionsRef = useRef<AgentSession[]>(sessions);
  useEffect(() => {
    sessionsRef.current = sessions;
  }, [sessions]);

  // Load all sessions from disk (~/.forge/sessions/[project-name]/ and workspace/.forge/sessions/)
  const reloadSavedSessions = useCallback(async () => {
    try {
      const diskSessions = await ApiBridge.loadSessionsJsonl(activeWorkspacePath);
      if (diskSessions && diskSessions.length > 0) {
        // The Go agent harness persists its own block-based sessions into the
        // same directory (content arrays, assistant/tool roles). The frontend
        // keeps its own copy of every conversation, so skip those files —
        // loading them would duplicate history and crash the string-based
        // chat renderer.
        const frontendShaped = diskSessions
          .filter((s: any) => s && !isGoHarnessSession(s))
          .map(normalizeLoadedSession)
          .filter((s): s is AgentSession => s !== null)
          .map(s => {
            if (isInvalidHardcodedPath(s.workspacePath) && activeWorkspacePath) {
              const pName = s.workspacePath?.split('/').filter(Boolean).pop();
              if (pName === activeWorkspacePath.split('/').filter(Boolean).pop()) {
                return { ...s, workspacePath: activeWorkspacePath };
              }
            }
            return s;
          });
        if (frontendShaped.length > 0) {
          setSavedSessions(frontendShaped);
          setSessions(prev => {
            if (prev.length === 0 && frontendShaped.length > 0) {
              return [frontendShaped[0]];
            }
            return prev;
          });
        }
      }
    } catch {
      // ignore
    }
  }, [activeWorkspacePath]);

  useEffect(() => {
    reloadSavedSessions();
  }, [reloadSavedSessions]);

  // Sync session changes to localStorage and disk. Streaming updates the
  // sessions array on every text delta — persisting every session on every
  // tick stalled the whole UI. Persist only sessions whose payload actually
  // changed, coalesced through a trailing debounce.
  const sessionPersistRef = useRef<Map<string, string>>(new Map());
  const sessionPersistTimerRef = useRef<number | null>(null);
  useEffect(() => {
    if (sessionPersistTimerRef.current !== null) {
      window.clearTimeout(sessionPersistTimerRef.current);
    }
    sessionPersistTimerRef.current = window.setTimeout(() => {
      sessionPersistTimerRef.current = null;
      try {
        // localStorage holds a TRIMMED copy (40 newest sessions, last 50
        // messages each, no diff payloads) so the quota survives long use —
        // full transcripts live in the per-session JSONL files on disk.
        const trimmed = sessions.slice(0, 40).map(s => ({
          ...s,
          messages: s.messages.length > 50 ? s.messages.slice(-50) : s.messages,
          diffs: [],
          sideConversationMessages: []
        }));
        localStorage.setItem('forge_ade_sessions', JSON.stringify(trimmed));
      } catch {
        // ignore
      }
      for (const sess of sessions) {
        if (!sess || !sess.id) continue;
        const fingerprint = JSON.stringify({
          t: sess.title,
          s: sess.status,
          g: sess.goState,
          m: sess.messages.length,
          last: sess.messages.length > 0 ? sess.messages[sess.messages.length - 1] : null,
          d: sess.diffs?.length ?? 0,
          k: sess.tasks
        });
        if (sessionPersistRef.current.get(sess.id) === fingerprint) continue;
        sessionPersistRef.current.set(sess.id, fingerprint);
        ApiBridge.saveSessionJsonl(sess, sess.workspacePath ?? activeWorkspacePath);
      }
    }, 800);
    return () => {
      if (sessionPersistTimerRef.current !== null) {
        window.clearTimeout(sessionPersistTimerRef.current);
        sessionPersistTimerRef.current = null;
      }
    };
  }, [sessions, activeWorkspacePath]);

  // ACP & Agent Registry (ForgeADE Internal + Pi, OhMyPi/OMP, OpenCode, Custom)
  const [agents, setAgents] = useState<ACPAgent[]>(() => {
    try {
      const saved = localStorage.getItem('forge_ade_agents') || localStorage.getItem('my_ade_agents');
      if (saved) {
        const parsed: ACPAgent[] = JSON.parse(saved);
        if (Array.isArray(parsed) && parsed.length > 0) {
          const hasInternal = parsed.some(a => a.id === 'agent-internal');
          const list = hasInternal ? [...parsed] : [DEFAULT_AGENTS[0], ...parsed];
          // Ensure all default agents are present in the list
          const existingIds = new Set(list.map(a => a.id));
          for (const def of DEFAULT_AGENTS) {
            if (!existingIds.has(def.id)) {
              list.push(def);
            }
          }
          return list.map(a => {
            const def = DEFAULT_AGENTS.find(d => d.id === a.id);
            const isStaleModel = (a.model || '').includes('myairouter_cloud');
            const cleanSupported = (a.supportedModels || []).filter(m => !m.includes('myairouter_cloud'));
            return {
              ...a,
              name: def ? def.name : a.name,
              provider: a.type === 'internal' ? 'custom' : (def ? def.provider : a.provider),
              command: def ? def.command : a.command,
              args: def ? def.args : a.args,
              type: def ? def.type : a.type,
              model: isStaleModel ? (def ? def.model : cleanSupported[0] || a.model) : (a.model || (def ? def.model : '')),
              supportedModels: cleanSupported.length > 0 ? cleanSupported : (def ? def.supportedModels : []),
              supportedEfforts: def ? def.supportedEfforts : (a.supportedEfforts || undefined),
              defaultEffort: def ? def.defaultEffort : a.defaultEffort
            };
          });
        }
      }
    } catch {
      // ignore
    }
    return DEFAULT_AGENTS;
  });

  const [activeAgentId, setActiveAgentIdState] = useState<string>(() => {
    const saved = localStorage.getItem('forge_ade_active_agent_id') || localStorage.getItem('my_ade_active_agent_id');
    if (saved) {
      return saved;
    }
    return 'agent-agy';
  });

  const setActiveAgentId = useCallback((id: string) => {
    setActiveAgentIdState(id);
    localStorage.setItem('forge_ade_active_agent_id', id);
    const targetAgent = agents.find(a => a.id === id);
    if (targetAgent && targetAgent.type !== 'internal') {
      const explicitModel = explicitModelRef.current.agentId === id
        ? explicitModelRef.current.model
        : (targetAgent.model || (targetAgent.supportedModels && targetAgent.supportedModels[0]) || '');
      setCurrentModelState(explicitModel);
      try {
        if (explicitModel) {
          localStorage.setItem('forge_ade_current_model', explicitModel);
        } else {
          localStorage.removeItem('forge_ade_current_model');
        }
      } catch {}
    }
  }, [agents]);

  const [acpEnabled, setAcpEnabled] = useState<boolean>(true);
  const [privacySettings, setPrivacySettings] = useState<PrivacySettings>(() => {
    try {
      const saved = localStorage.getItem('forge_ade_privacy') || localStorage.getItem('my_ade_privacy');
      if (saved) return JSON.parse(saved);
    } catch {
      // ignore
    }
    return DEFAULT_PRIVACY;
  });

  useEffect(() => {
    localStorage.setItem('forge_ade_privacy', JSON.stringify(privacySettings));
  }, [privacySettings]);

  useEffect(() => {
    localStorage.setItem('forge_ade_agents', JSON.stringify(agents));
  }, [agents]);

  // ACP Handshake & Real Model Verification (Dynamic discovery like Kybern)
  const refreshAgentModels = useCallback(async (targetAgentId?: string) => {
    try {
      const targetAgents = targetAgentId ? agents.filter(a => a.id === targetAgentId) : agents;
      const modelResults = await Promise.all(
        targetAgents.map(async (ag) => {
          try {
            const models = await ApiBridge.getAgentModels(ag.id);
            return { id: ag.id, models };
          } catch {
            return { id: ag.id, models: [] };
          }
        })
      );
      setAgents(prev => prev.map(a => {
        const found = modelResults.find(m => m.id === a.id);
        if (found && found.models && found.models.length > 0) {
          const cleanDiscovered = found.models.filter(m => !m.includes('myairouter_cloud'));
          const cleanCurrent = (a.supportedModels || []).filter(m => !m.includes('myairouter_cloud'));
          const mergedModels = Array.from(new Set([...cleanDiscovered, ...cleanCurrent]));
          const currentValid = a.model && mergedModels.includes(a.model) && !a.model.includes('myairouter_cloud');
          return {
            ...a,
            supportedModels: mergedModels,
            model: currentValid ? a.model : cleanDiscovered[0]
          };
        }
        return a;
      }));
    } catch {
      // ignore
    }
  }, [agents]);

  // Initial startup: auto-discover local CLI models from disk configurations
  useEffect(() => {
    void refreshAgentModels();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  // Providers & Active Model Management (Zustand Domain Store Delegation)
  const {
    providers,
    setProviders,
    deletedProviderIds,
    updateProvider,
    addProvider,
    deleteProvider,
    addModelToProvider,
    deleteModelFromProvider,
    toggleModelSelection,
    selectAllModels,
    deselectAllModels,
    fetchProviderModels: storeFetchProviderModels,
    activateGoModel,
    hydrateProvidersFromGo,
    initProviderSync,
  } = useProviderStore();

  useEffect(() => {
    return initProviderSync();
  }, [initProviderSync]);

  const [currentModel, setCurrentModelState] = useState<string>(() => {
    try {
      return localStorage.getItem('forge_ade_current_model') || localStorage.getItem('my_ade_current_model') || '';
    } catch {
      return '';
    }
  });
  // Explicit user picks: agentId -> model. The sync effect must never override these.
  const explicitModelRef = useRef<{ agentId: string; model: string }>({ agentId: '', model: '' });

  const setCurrentModel = useCallback((model: string, providerId?: string) => {
    setCurrentModelState(model);
    explicitModelRef.current = { agentId: activeAgentId, model };
    try {
      if (model) {
        localStorage.setItem('forge_ade_current_model', model);
      } else {
        localStorage.removeItem('forge_ade_current_model');
      }
    } catch {}

    // Persist model & provider to the currently active session
    if (activeSessionId) {
      setSessions(prev => prev.map(s => {
        if (s.id !== activeSessionId) return s;
        const updated = {
          ...s,
          model,
          ...(providerId ? { provider: providerId } : {}),
          updatedAt: new Date().toISOString()
        };
        try {
          ApiBridge.saveSessionJsonl(updated, updated.workspacePath ?? activeWorkspacePath);
        } catch {}
        return updated;
      }));
    }
  }, [activeSessionId, activeWorkspacePath, activeAgentId]);

  // Sync currentModel with active agent / enabled providers only when no activeSession model exists
  useEffect(() => {
    if (activeSessionId) {
      const activeSess = sessions.find(s => s.id === activeSessionId) || savedSessions.find(s => s.id === activeSessionId);
      if (activeSess && activeSess.model && activeSess.model !== currentModel) {
        setCurrentModelState(activeSess.model);
        if (activeSess.agentId && activeSess.agentId !== activeAgentId) {
          setActiveAgentIdState(activeSess.agentId);
        }
        return;
      }
    }

    const curAgent = agents.find(a => a.id === activeAgentId);
    if (curAgent && curAgent.type !== 'internal') {
      // External ACP agents: never override an explicit user pick for this agent
      if (explicitModelRef.current.agentId === activeAgentId && explicitModelRef.current.model === currentModel && currentModel) {
        return;
      }
      // If currentModel is empty or not in curAgent.supportedModels, sync to curAgent's real configured model
      const models = curAgent.supportedModels || [];
      if (!currentModel || (models.length > 0 && !models.includes(currentModel))) {
        const nextModel = curAgent.model || models[0] || '';
        if (nextModel) {
          setCurrentModelState(nextModel);
          try { localStorage.setItem('forge_ade_current_model', nextModel); } catch {}
        }
      }
      return;
    }

    // Internal agent: never override an explicit user pick. Without this the
    // effect fights the dropdown (pick → effect reverts → flicker).
    if (explicitModelRef.current.agentId === activeAgentId && explicitModelRef.current.model === currentModel && currentModel) {
      return;
    }

    const enabled = providers.filter(p => p.enabled);
    const validModels: string[] = Array.from(new Set(
      enabled.flatMap(p => (p.selectedModels && p.selectedModels.length > 0 ? p.selectedModels : p.models || []))
    ));

    if (validModels.length > 0) {
      if (!currentModel || !validModels.includes(currentModel)) {
        const defaultModel = validModels[0];
        setCurrentModelState(defaultModel);
        try { localStorage.setItem('forge_ade_current_model', defaultModel); } catch {}
      }
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [agents, activeAgentId, providers, activeSessionId]);

  const fetchProviderModels = useCallback(async (providerId: string) => {
    const prov = providers.find(p => p.id === providerId);
    if (!prov) return;
    const fetched = await ApiBridge.fetchModels(prov.id, prov.baseUrl, prov.apiKey);
    if (fetched.length > 0) {
      await storeFetchProviderModels(providerId);
      setCurrentModel(fetched[0]);
    }
  }, [providers, storeFetchProviderModels, setCurrentModel]);

  // ── Integrations (MCP, Skills, Plugins, Memory, Automations, Slash Commands, Hooks, Browser/Computer Settings)
  const {
    mcps,
    setMcps,
    fetchBackendMCPServers,
    addMcp,
    toggleMcp,
    deleteMcp,
    skills,
    setSkills,
    addSkill,
    toggleSkill,
    deleteSkill,
    fetchBackendSkills,
    createBackendSkill,
    deleteBackendSkill,
    reloadSkills,
    plugins,
    fetchPlugins,
    createPlugin,
    togglePlugin,
    deletePlugin,
    reloadPlugins,
    discoveredMcps,
    discoveredSkills,
    isDiscovering,
    runDiscovery,
    importDiscoveredMcp,
    importDiscoveredSkill,
    memories,
    fetchMemories,
    saveMemory,
    deleteMemory,
    reloadMemories,
    automations,
    automationsLoading,
    fetchAutomations,
    saveAutomation,
    deleteAutomation,
    setAutomationEnabled,
    customCommands,
    addCustomCommand,
    updateCustomCommand,
    deleteCustomCommand,
    toggleCustomCommand,
    hooks,
    updateHook,
    toggleHook,
    addHook,
    deleteHook,
    browserSettings,
    updateBrowserSettings,
    computerSettings,
    updateComputerSettings,
    initIntegrationsListeners,
  } = useIntegrationsStore();

  useEffect(() => {
    return initIntegrationsListeners();
  }, [initIntegrationsListeners]);

  // Running an automation seeds a new agent session with its prompt (same
  // path as typing the prompt) and records the run against its history.
  const runAutomation = useCallback((automation: ZAutomation) => {
    const sessionId = `session-${Date.now()}`;
    createNewSessionRef.current?.(automation.prompt, undefined, sessionId);
    setIsAutomationsOpen(false);
    const wsPath = automation.workspace || activeWorkspacePath || '';
    void ApiBridge.recordAutomationRun(automation.id, sessionId, wsPath).then(() => fetchAutomations()).catch(() => {});
    return sessionId;
  }, [activeWorkspacePath, fetchAutomations]);

  const createNewSessionRef = useRef<(initialPrompt?: string, selectedAgentId?: string, forcedId?: string) => string>(null);

  useEffect(() => {
    fetchAutomations();
    const unsub = EventsOn('automations:changed', () => {
      fetchAutomations();
    });
    return () => {
      unsub?.();
    };
  }, [fetchAutomations]);

  // Scheduled fires arrive as events: claim the token (only one window wins)
  // and run through the SAME composer path as "Run now", so the internal
  // agent session is fully visible — streaming, approvals, history.
  useEffect(() => {
    const unsub = EventsOn('automations:fire', (data: any) => {
      const id = data?.id;
      const token = data?.token;
      if (!id || !token) return;
      void (async () => {
        let automation = automations.find(a => a.id === id);
        if (!automation) {
          // List not loaded yet — fetch once before giving up on this fire.
          const list = await ApiBridge.listAutomations();
          automation = (list || []).find(a => a.id === id);
        }
        if (!automation) return;
        const claimed = await ApiBridge.claimAutomationFire(id, token);
        if (claimed) runAutomation(automation);
      })();
    });
    return () => {
      unsub?.();
    };
  }, [automations, runAutomation]);



  // ── Codebase Indexing ───────────────────────────────────────────────────────
  const [indexStatus, setIndexStatus] = useState<IndexingStatusInfo | null>(null);
  const [isReindexing, setIsReindexing] = useState<boolean>(false);

  const fetchIndexStatus = useCallback(async () => {
    try {
      const status = await ApiBridge.getIndexStatus();
      setIndexStatus(status);
    } catch (e) {
      console.warn('fetchIndexStatus failed', e);
    }
  }, []);

  const reindexWorkspace = useCallback(async () => {
    setIsReindexing(true);
    try {
      const res = await ApiBridge.reindexWorkspace();
      await fetchIndexStatus();
      return res;
    } finally {
      setIsReindexing(false);
    }
  }, [fetchIndexStatus]);

  useEffect(() => {
    fetchIndexStatus();
    const unsub = EventsOn('index:changed', () => {
      fetchIndexStatus();
    });
    return () => {
      unsub?.();
    };
  }, [fetchIndexStatus]);

  const activeAgent = agents.find(a => a.id === activeAgentId) || agents[0];

  const toggleAgentEnabled = useCallback((id: string) => {
    setAgents(prev => prev.map(a => a.id === id ? { ...a, enabled: !a.enabled } : a));
  }, []);

  const updateAgentConfig = useCallback((id: string, updates: Partial<ACPAgent>) => {
    setAgents(prev => prev.map(a => a.id === id ? { ...a, ...updates } : a));
  }, []);

  const addAgent = useCallback((agent: ACPAgent) => {
    setAgents(prev => [...prev.filter(a => a.id !== agent.id), agent]);
  }, []);

  const deleteAgent = useCallback((id: string) => {
    if (id === 'agent-internal') return;
    setAgents(prev => prev.filter(a => a.id !== id));
    if (activeAgentId === id) {
      setActiveAgentId('agent-internal');
    }
  }, [activeAgentId, setActiveAgentId]);

  // Diffs & Review System & Git State (delegated to useGitStore Zustand store)
  const gitBranch = useGitStore(s => s.gitBranch);
  const gitFiles = useGitStore(s => s.gitFiles);
  const gitCommits = useGitStore(s => s.gitCommits);
  const diffs = useGitStore(s => s.diffs);
  const activeDiff = useGitStore(s => s.activeDiff);
  const isGitClientOpen = useGitStore(s => s.isGitClientOpen);
  const setIsGitClientOpen = useGitStore(s => s.setIsGitClientOpen);
  const setDiffs = useGitStore(s => s.setDiffs);
  const setActiveDiff = useGitStore(s => s.setActiveDiff);
  const addDiff = useGitStore(s => s.addDiff);
  const rejectDiff = useGitStore(s => s.rejectDiff);

  const refreshGitStatus = useCallback(async () => {
    await useGitStore.getState().refreshGitStatus(activeWorkspacePath);
  }, [activeWorkspacePath]);

  const refreshGitLog = useCallback(async () => {
    await useGitStore.getState().refreshGitLog(activeWorkspacePath);
  }, [activeWorkspacePath]);

  // File tree & editor tabs
  const [files, setFiles] = useState<FileItem[]>([]);
  const [openTabs, setOpenTabs] = useState<EditorTab[]>([]);
  const [activeTabId, setActiveTabId] = useState<string | null>(null);
  const [selectedFile, setSelectedFile] = useState<FileItem | null>(null);
  const [isSplitEditor, setIsSplitEditor] = useState<boolean>(false);

  useEffect(() => { openTabsRef.current = openTabs; }, [openTabs]);
  useEffect(() => { activeTabIdRef.current = activeTabId; }, [activeTabId]);
  useEffect(() => { selectedFileRef.current = selectedFile; }, [selectedFile]);
  useEffect(() => { diffsRef.current = diffs; }, [diffs]);
  useEffect(() => { activeDiffRef.current = activeDiff; }, [activeDiff]);

  // Record file activations on activeTabId — NOT on every openTabs write
  // (typing mutates openTabs per keystroke via setTabBuffer and would run this
  // constantly). The refs above keep the effect off the openTabs dependency.
  useEffect(() => {
    const tab = openTabsRef.current.find(t => t.id === activeTabId);
    if (tab && tab.type === 'code' && tab.filePath) recordRecentFile(tab.filePath);
  }, [activeTabId, recordRecentFile]);

  // Navigation & Activities (delegated to useLayoutStore Zustand store)
  const activeActivity = useLayoutStore(s => s.activeActivity);
  const setActiveActivity = useLayoutStore(s => s.setActiveActivity);

  // ── Separated Sidebars per Mode (Agent Mode vs Editor Mode) ────────────────
  const agentLeftSidebarOpen = useLayoutStore(s => s.agentLeftSidebarOpen);
  const setAgentLeftSidebarOpen = useLayoutStore(s => s.setAgentLeftSidebarOpen);
  const rightActionDrawerOpen = useLayoutStore(s => s.rightActionDrawerOpen);
  const setRightActionDrawerOpen = useLayoutStore(s => s.setRightActionDrawerOpen);
  const agentRightSidebarOpen = rightActionDrawerOpen;
  const setAgentRightSidebarOpen = setRightActionDrawerOpen;

  const editorLeftSidebarOpen = useLayoutStore(s => s.editorLeftSidebarOpen);
  const setEditorLeftSidebarOpen = useLayoutStore(s => s.setEditorLeftSidebarOpen);
  const editorRightSidebarOpen = rightActionDrawerOpen;
  const setEditorRightSidebarOpen = setRightActionDrawerOpen;

  // Mode-aware reactive accessors
  const isLeftSidebarOpen = mode === 'editor' ? editorLeftSidebarOpen : agentLeftSidebarOpen;
  const setIsLeftSidebarOpen = useCallback((val: boolean | ((prev: boolean) => boolean)) => {
    if (modeRef.current === 'editor') {
      setEditorLeftSidebarOpen(val);
    } else {
      setAgentLeftSidebarOpen(val);
    }
  }, [setEditorLeftSidebarOpen, setAgentLeftSidebarOpen]);

  const isRightActionDrawerOpen = rightActionDrawerOpen;
  const setIsRightActionDrawerOpen = setRightActionDrawerOpen;

  // Agent Mode Views (Session vs Jira Board vs Git Client)
  const agentViewMode = useLayoutStore(s => s.agentViewMode);
  const setAgentViewMode = useLayoutStore(s => s.setAgentViewMode);

  // ── Workflow Board Tasks (Zustand Domain Store Delegation) ─────────────────
  const {
    boardTasks,
    refreshBoardTasks: storeRefreshBoardTasks,
    addBoardTask: storeAddBoardTask,
    updateBoardTask: storeUpdateBoardTask,
    deleteBoardTask: storeDeleteBoardTask,
    moveBoardTaskStatus: storeMoveBoardTaskStatus,
    initBoardEventListener,
  } = useBoardStore();

  const currentBoardWs = agentWorkspacePath || activeWorkspacePath || '';

  const refreshBoardTasks = useCallback(async () => {
    await storeRefreshBoardTasks(currentBoardWs);
  }, [storeRefreshBoardTasks, currentBoardWs]);

  useEffect(() => {
    void refreshBoardTasks();
  }, [refreshBoardTasks]);

  useEffect(() => {
    return initBoardEventListener(() => agentWorkspacePath || activeWorkspacePath || '');
  }, [initBoardEventListener, agentWorkspacePath, activeWorkspacePath]);

  const addBoardTask = useCallback((input: Omit<BoardTask, 'id' | 'key' | 'createdAt' | 'updatedAt'>) => {
    return storeAddBoardTask(input, currentBoardWs);
  }, [storeAddBoardTask, currentBoardWs]);

  const updateBoardTask = useCallback((id: string, patch: Partial<BoardTask>) => {
    storeUpdateBoardTask(id, patch, currentBoardWs);
  }, [storeUpdateBoardTask, currentBoardWs]);

  const deleteBoardTask = useCallback((id: string) => {
    storeDeleteBoardTask(id, currentBoardWs);
  }, [storeDeleteBoardTask, currentBoardWs]);

  const moveBoardTaskStatus = useCallback((id: string, status: BoardTaskStatus) => {
    storeMoveBoardTaskStatus(id, status, currentBoardWs);
  }, [storeMoveBoardTaskStatus, currentBoardWs]);

  // Backward compatibility aliases
  const jiraTasks = boardTasks;
  const refreshJiraTasks = refreshBoardTasks;
  const addJiraTask = addBoardTask;
  const updateJiraTask = updateBoardTask;
  const deleteJiraTask = deleteBoardTask;
  const moveJiraTaskStatus = moveBoardTaskStatus;

  // Bottom dock terminal (delegated to useTerminalStore Zustand store)
  const isBottomTerminalOpen = useTerminalStore(s => s.isBottomTerminalOpen);
  const setIsBottomTerminalOpen = useTerminalStore(s => s.setIsBottomTerminalOpen);

  // Left sidebar view mode + right-pane file tabs (ZCode explorer pattern)
  const leftSidebarView = useLayoutStore(s => s.leftSidebarView);
  const setLeftSidebarView = useLayoutStore(s => s.setLeftSidebarView);
  const [sideFileTabs, setSideFileTabs] = useState<Array<{ path: string; name: string; line?: number }>>([]);
  const [activeSideFile, setActiveSideFile] = useState<string | null>(null);
  const [activeSideFileLine, setActiveSideFileLine] = useState<number | undefined>(undefined);

  const openSideFile = useCallback(async (rawPath: string, line?: number) => {
    const parsed = parseFilePath(rawPath, activeWorkspacePath);
    const path = parsed.cleanPath || rawPath;
    const targetLine = line !== undefined ? line : parsed.line;
    const name = parsed.displayName;
    const isImage = isImageFile(path);

    // 1. Guard against zip archives
    if (isZipFile(path)) {
      showToast(`"${name}" is an archive file and cannot be opened in side preview`, 'error');
      return;
    }

    // 2. Guard against binaries (excluding images)
    if (!isImage && isBinaryFile(path)) {
      showToast(`"${name}" is a binary file and cannot be opened in side preview`, 'error');
      return;
    }

    // 3. Guard against files > 2MB (excluding images)
    if (!isImage) {
      try {
        const size = await ApiBridge.fileSize(path);
        if (size > MAX_SIDE_PREVIEW_TEXT_BYTES) {
          const mb = (size / (1024 * 1024)).toFixed(1);
          showToast(`"${name}" is ${mb}MB — exceeds 2MB limit for text preview`, 'error');
          return;
        }
      } catch {
        // file stat error fallback
      }
    }

    setSideFileTabs(prev => {
      const existing = prev.find(t => t.path === path);
      if (existing) {
        return prev.map(t => (t.path === path ? { ...t, line: targetLine } : t));
      }
      return [...prev, { path, name, line: targetLine }];
    });
    setActiveSideFile(path);
    setActiveSideFileLine(targetLine);
    setIsRightActionDrawerOpen(true);
  }, [activeWorkspacePath, setIsRightActionDrawerOpen, showToast]);

  useEffect(() => {
    globalOpenSideFileFn = openSideFile;
    return () => {
      globalOpenSideFileFn = null;
    };
  }, [openSideFile]);

  const closeSideFile = useCallback((path: string) => {
    setSideFileTabs(prev => {
      const rest = prev.filter(t => t.path !== path);
      setActiveSideFile(current => {
        if (current !== path) return current;
        return rest.length > 0 ? rest[rest.length - 1].path : null;
      });
      return rest;
    });
  }, []);

  const openSideBrowser = useCallback((url?: string) => {
    setIsRightActionDrawerOpen(true);
    if (typeof window !== 'undefined') {
      window.dispatchEvent(new CustomEvent('forge:open-sidepane-browser', { detail: { url } }));
    }
  }, [setIsRightActionDrawerOpen]);

  useEffect(() => {
    setOpenBrowserHandler(openSideBrowser);
    return () => {
      setOpenBrowserHandler(null);
    };
  }, [openSideBrowser]);

  // Modals & Palette (delegated to useLayoutStore Zustand store)
  const isCommandPaletteOpen = useLayoutStore(s => s.isCommandPaletteOpen);
  const setIsCommandPaletteOpen = useLayoutStore(s => s.setIsCommandPaletteOpen);
  const isAutomationsOpen = useLayoutStore(s => s.isAutomationsOpen);
  const setIsAutomationsOpen = useLayoutStore(s => s.setIsAutomationsOpen);
  const commandPaletteMode = useLayoutStore(s => s.commandPaletteMode);
  const setCommandPaletteMode = useLayoutStore(s => s.setCommandPaletteMode);
  const openCommandPalette = useLayoutStore(s => s.openCommandPalette);
  const isCustomizationsModalOpen = useLayoutStore(s => s.isCustomizationsModalOpen);
  const setIsCustomizationsModalOpen = useLayoutStore(s => s.setIsCustomizationsModalOpen);
  const isSettingsModalOpen = useLayoutStore(s => s.isSettingsModalOpen);
  const setIsSettingsModalOpen = useLayoutStore(s => s.setIsSettingsModalOpen);
  const settingsActiveSection = useLayoutStore(s => s.settingsActiveSection);
  const setSettingsActiveSection = useLayoutStore(s => s.setSettingsActiveSection);

  // Agent execution mode & Engine
  const [agentExecutionMode, setAgentExecutionMode] = useState<AgentExecutionMode>('bypass');
  const [reasoningLevel, setReasoningLevelState] = useState<AgentReasoningLevel>(() => {
    const saved = localStorage.getItem('forge_ade_reasoning_level') as AgentReasoningLevel;
    if (saved && ['off', 'minimal', 'low', 'medium', 'high', 'xhigh', 'max'].includes(saved)) {
      return saved;
    }
    return 'medium';
  });
  const setReasoningLevel = useCallback((lvl: AgentReasoningLevel) => {
    setReasoningLevelState(lvl);
    localStorage.setItem('forge_ade_reasoning_level', lvl);
    SetActiveReasoningEffort(lvl).catch(() => {});
  }, []);

  const [agentEngine, setAgentEngine] = useState<AgentEngineType>('pi');

  // ── Panel widths (delegated to useLayoutStore Zustand store) ────────────────
  const leftSidebarWidth = useLayoutStore(s => s.leftSidebarWidth);
  const setLeftSidebarWidth = useLayoutStore(s => s.setLeftSidebarWidth);
  const rightPaneWidth = useLayoutStore(s => s.rightPaneWidth);
  const setRightPaneWidth = useLayoutStore(s => s.setRightPaneWidth);

  // Real Dynamic Context Usage calculation
  const activeSession = activeSessionId ? (sessions.find(s => s.id === activeSessionId) || savedSessions.find(s => s.id === activeSessionId)) : undefined;

  // Keep the active Go session's approval gate in sync with the composer's
  // permission mode: Full access (bypass) auto-approves mutating tools, the
  // other modes gate. Covers mode changes and switching between sessions
  // (the flag is persisted per Go session, so a stale session must be
  // re-synced when it becomes active again).
  const activeGoSessionId = activeSession?.goSessionId;
  useEffect(() => {
    if (!activeGoSessionId) return;
    SetAgentAutoApprove(activeGoSessionId, agentExecutionMode === 'bypass').catch(() => {});
  }, [activeGoSessionId, agentExecutionMode]);

  const contextUsage: ContextUsageInfo = useMemo(() => {
    return calculateContextUsage(activeSession, currentModel);
  }, [activeSession, currentModel]);

  // Diagnostics (LSP)
  const [diagnostics, setDiagnostics] = useState<LSPDiagnostic[]>([]);

  const engineRef = useRef<AgentEngine>(new AgentEngine());

  const sendSideConversationPrompt = useCallback(async (promptText: string, model?: string, mode?: 'ask' | 'plan' | 'full') => {
    if (!promptText.trim() || !activeSessionId) return;
    const activeSessionObj = sessions.find(s => s.id === activeSessionId);
    if (!activeSessionObj) return;

    await runSideConversationPrompt({
      promptText,
      model: model || currentModel,
      mode: mode || 'full',
      activeSession: activeSessionObj,
      activeWorkspacePath: activeWorkspacePath || '',
      diffs: activeSessionObj.diffs || diffs || [],
      gitFiles,
      gitBranch,
      engine: engineRef.current,
      updateMessages: (updater) => {
        setSessions(prev => prev.map(s => {
          if (s.id !== activeSessionId) return s;
          return {
            ...s,
            sideConversationMessages: updater(s.sideConversationMessages || [])
          };
        }));
      }
    });
  }, [activeSessionId, activeWorkspacePath, currentModel, sessions, diffs, gitFiles, gitBranch]);

  const clearSideConversation = useCallback(() => {
    if (!activeSessionId) return;
    setSessions(prev => prev.map(s => s.id === activeSessionId ? { ...s, sideConversationMessages: [] } : s));
  }, [activeSessionId]);

  // Sync theme
  useEffect(() => {
    localStorage.setItem('forge-ade-theme', theme);
    localStorage.setItem('forge_ade_theme', theme);
    useUIStore.getState().setTheme(theme);
    applyTheme(themePalette, theme);
  }, [theme, themePalette]);

  const toggleTheme = useCallback((next?: ThemeMode) => {
    setTheme(prev => next ?? (prev === 'dark' ? 'light' : 'dark'));
  }, []);

  // Refresh files from disk
  const refreshFiles = useCallback(async () => {
    const currentPath = activeWorkspacePath || (await ApiBridge.getWorkspaceInfo()).cwd;
    if (currentPath) {
      const realTree = await ApiBridge.readDirectoryTree(currentPath);
      setFiles(realTree);
    }
  }, [activeWorkspacePath]);

  // Initial load / workspace switch — the single entry point that opens the
  // folder in the backend, loads the file tree, and refreshes git state.
  // Every way a workspace can change (header dropdown, home screen, session
  // history, ⌘O, command palette) lands here.
  useEffect(() => {
    const targetPath = activeWorkspacePath;
    wsPathRef.current = targetPath;

    const initWorkspace = async () => {
      // Tabs and editor buffers belong strictly to the editor surface.
      // Do not wipe or mutate openTabs when initializing an agent workspace.
      if (modeRef.current === 'editor') {
        const cached = workspaceStateCacheRef.current.get(targetPath);
        if (cached) {
          setOpenTabs(cached.openTabs || []);
          setActiveTabId(cached.activeTabId || null);
          setSelectedFile(cached.selectedFile || null);
          setDiffs(cached.diffs || []);
          setActiveDiff(cached.activeDiff || null);
        } else if (targetPath) {
          setOpenTabs([]);
          setActiveTabId(null);
          setSelectedFile(null);
          setDiffs([]);
          setActiveDiff(null);
          setIsSplitEditor(false);
          setNav({ stack: [], index: -1 });
        }
      }

      if (!targetPath) return;
      try {
        await ApiBridge.openFolder(targetPath);
      } catch (e) {
        console.warn('Backend OpenFolder sync failed:', e);
      }
      // A fast switch must not let an earlier slower tree response overwrite a newer one.
      if (wsPathRef.current !== targetPath) return;
      const realTree = await ApiBridge.readDirectoryTree(targetPath);
      if (wsPathRef.current !== targetPath) return;
      setFiles(realTree);
      refreshGitStatus();
      refreshGitLog();
    };
    initWorkspace();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [activeWorkspacePath, refreshGitStatus, refreshGitLog]);

  // External changes watcher listener (Issue 3)
  // Editor is aware of external changes, but skips large folders like node_modules, .git, dist, etc.
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

    // A build (go build, vite, …) fires dozens of events per second; coalesce
    // them into one git status refresh per quiet window.
    let gitRefreshTimer: number | null = null;
    const scheduleGitRefresh = () => {
      if (gitRefreshTimer !== null) window.clearTimeout(gitRefreshTimer);
      gitRefreshTimer = window.setTimeout(() => {
        gitRefreshTimer = null;
        refreshGitStatus().catch(() => {});
      }, 600);
    };

    const unsub = EventsOn('fs:changed', async (event: any) => {
      const changedPath: string = event?.path || '';
      if (!changedPath) return;

      // Skip large folders (node_modules, etc.) - they are only updated via manual explorer refresh
      if (isHeavyDir(changedPath)) {
        return;
      }

      // 1. If an open tab matches the changed file on disk, reload its content.
      // Resolved via ref OUTSIDE the updater — side effects inside a state
      // updater run twice under StrictMode and fire during render.
      const matchingTab = openTabsRef.current.find(t => t.filePath === changedPath && t.type === 'code');
      if (matchingTab && !matchingTab.isModified) {
        // VS Code semantics: an externally changed file with UNSAVED edits is
        // left alone — clobbering the buffer destroyed user work silently.
        ApiBridge.readFile(changedPath).then(newContent => {
          setOpenTabs(currentTabs =>
            currentTabs.map(t =>
              t.id === matchingTab.id ? { ...t, content: newContent } : t
            )
          );
        }).catch(err => console.warn('Failed to reload external file change:', err));
      }

      // 2. Debounced background refresh of git status for non-heavy files
      scheduleGitRefresh();
    });

    const unsubSSH = EventsOn('ssh:disconnected', (event: any) => {
      const connId: string = event?.id || '';
      const reason: string = event?.reason || 'Connection lost';
      console.warn(`[ssh] remote server ${connId} disconnected:`, reason);
      window.dispatchEvent(new CustomEvent('forge:ssh-disconnected', { detail: event }));
    });

    return () => {
      if (typeof unsub === 'function') unsub();
      if (typeof unsubSSH === 'function') unsubSSH();
      if (gitRefreshTimer !== null) window.clearTimeout(gitRefreshTimer);
    };
  }, [refreshGitStatus]);

  // Helper to find file by path in tree
  const findFileInTreeCb = useCallback((items: FileItem[], path: string): FileItem | null => {
    return findFileInTree(items, path);
  }, []);

  // Open / Pick folder directly via native OS dialog (no artificial modal).
  // The workspace-switch effect performs the backend open, tree load, and git
  // refresh — refreshing them here with the pre-switch closure showed the
  // previous workspace's git state after switching.
  const openFolder = useCallback(async (customPath?: string) => {
    let targetPath = customPath;
    if (!targetPath) {
      try {
        const picked = await ApiBridge.pickNativeDirectory();
        if (picked?.path) {
          targetPath = picked.path;
        }
      } catch (err) {
        console.warn('Native folder pick failed:', err);
      }
    }
    if (targetPath) {
      if (targetPath === wsPathRef.current) {
        // Same workspace re-opened: just reload tree + git status.
        await refreshFiles();
        refreshGitStatus();
        refreshGitLog();
      } else {
        setActiveWorkspacePath(targetPath);
      }
    }
  }, [setActiveWorkspacePath, refreshFiles, refreshGitStatus, refreshGitLog]);

  // Back/forward navigation history. Each entry is a snapshot of the tab at
  // activation time, so Go Back can reopen a tab the user closed.
  const [nav, setNav] = useState<{ stack: EditorTab[]; index: number }>({ stack: [], index: -1 });

  // Single entry point for "show this tab in the editor": activates it and
  // records it in the history (browser semantics — a new navigation truncates
  // the forward entries).
  const openTab = useCallback((tab: EditorTab) => {
    setActiveTabId(tab.id);
    setNav(prev => pushNavHistory(prev, tab));
  }, []);

  const restoreNavTab = useCallback((entry: EditorTab) => {
    setOpenTabs(prev => (prev.some(t => t.id === entry.id) ? prev : [...prev, { ...entry }]));
    setActiveTabId(entry.id);
    // Revived-from-history tabs have no content (see openTab) — re-read.
    if (entry.type === 'code' && entry.filePath) {
      void ApiBridge.readFile(entry.filePath)
        .then(content => {
          setOpenTabs(prev => prev.map(t => (t.id === entry.id && !t.content ? { ...t, content } : t)));
        })
        .catch(() => {});
    }
  }, []);

  const goBack = useCallback(() => {
    const res = getNavBack(nav);
    if (!res) return;
    setNav(res.nextNav);
    restoreNavTab(res.targetTab);
  }, [nav, restoreNavTab]);

  const goForward = useCallback(() => {
    const res = getNavForward(nav);
    if (!res) return;
    setNav(res.nextNav);
    restoreNavTab(res.targetTab);
  }, [nav, restoreNavTab]);

  // Ctrl+Tab / Ctrl+Shift+Tab — cycle through open tabs in list order.
  const cycleTab = useCallback((dir: 1 | -1) => {
    const next = cycleActiveTab(openTabsRef.current, activeTabIdRef.current, dir);
    if (next) openTab(next);
  }, [openTab]);

  const openFileInEditor = useCallback(async (filePath: string, line?: number, column?: number, opts?: { silent?: boolean }) => {
    // Isolated agent surface: In agent mode, show all file/attachment views in the right sidebar
    if (modeRef.current === 'agent') {
      await openSideFile(filePath, line);
      return;
    }

    let file = findFileInTree(files, filePath);
    let content = file?.content || '';

    const fileName = filePath.split('/').pop() || filePath;

    // Binary files: refuse immediately — no stat, no read, no bytes over IPC.
    if (isBinaryFile(filePath)) {
      if (!opts?.silent) {
        showToast(`"${fileName}" is a binary file and can't be opened in the editor`, 'error');
      }
      return;
    }

    const isDb = isDatabaseFile(filePath);

    if (!content && !isDb) {
      // Large-file guard: an 8MB text file balloons to 30-50MB of RAM once it
      // becomes JS strings + CodeMirror doc copies. Refuse instead of spiking.
      const size = await ApiBridge.fileSize(filePath);
      if (size > MAX_EDITOR_FILE_BYTES) {
        if (!opts?.silent) {
          showToast(
            `"${fileName}" is ${Math.round(size / (1024 * 1024))}MB — too large for the code editor`,
            'error'
          );
        }
        return;
      }
      content = await ApiBridge.readFile(filePath);
      // Fail-safe when the size was unknowable before reading (ssh without
      // stat, stat races): bound the READ result itself so a giant file can
      // never reach the buffer.
      if (content.length > MAX_EDITOR_FILE_BYTES) {
        if (!opts?.silent) {
          showToast(`"${fileName}" is too large to open in the code editor`, 'error');
        }
        return;
      }
    }

    const fileItem: FileItem = file || {
      id: `file-${Date.now()}`,
      name: fileName,
      path: filePath,
      type: 'file',
      content,
      isModified: false
    };

    setSelectedFile(fileItem);

    // Functional upsert: two rapid opens (double click, palette + click) must
    // not race into two tabs for the same file.
    setOpenTabs(prev => {
      const existing = prev.find(t => t.filePath === filePath && t.type === 'code');
      if (existing) {
        return line !== undefined
          ? prev.map(t => (t.id === existing.id ? { ...t, line, column } : t))
          : prev;
      }
      return [...prev, {
        // Must equal the id passed to openTab below — a mismatch makes
        // activeTabId point at a tab that doesn't exist, so the editor pane
        // never switches to the opened file.
        id: `tab-${fileItem.id}`,
        fileId: fileItem.id,
        fileName: fileItem.name,
        filePath: fileItem.path,
        content,
        type: 'code',
        line,
        column
      }];
    });
    openTab({
      id: `tab-${fileItem.id}`,
      fileId: fileItem.id,
      fileName: fileItem.name,
      filePath: fileItem.path,
      content,
      type: 'code',
      line,
      column
    });

    // Session restore re-opens many files at boot; flipping the surface then
    // would yank the user out of whatever surface they started on.
    if (!opts?.silent) setMode('editor');
  }, [files, findFileInTree, openTab, setMode]);

  // ── Editor session restore + persistence ────────────────────────────────────
  // Identity of openFileInEditor churns with files/openTabs; the restore loop
  // must not observe that (it would cancel itself after the first reopened
  // tab), so it goes through a ref.
  const openFileInEditorRef = useRef(openFileInEditor);
  useEffect(() => { openFileInEditorRef.current = openFileInEditor; }, [openFileInEditor]);

  // Restart restore: a workspace this window hasn't seen yet gets its
  // persisted editor session back. In-memory-cached workspaces (workspace
  // switches within one app run) are skipped — they were already restored
  // synchronously by switchSurfaceWorkspace.
  const restoredForRef = useRef<Set<string>>(new Set());
  const activeWorkspaceRestoreRef = useRef(activeWorkspacePath);
  useEffect(() => {
    activeWorkspaceRestoreRef.current = activeWorkspacePath;
    const path = activeWorkspacePath;
    if (!path || restoredForRef.current.has(path)) return;
    restoredForRef.current.add(path);
    if (workspaceStateCacheRef.current.has(path)) return;

    const saved = readPersistedSession(path);
    if (!saved || saved.tabs.length === 0) return;

    let cancelled = false;
    (async () => {
      for (const tab of saved.tabs) {
        if (cancelled || activeWorkspaceRestoreRef.current !== path) return;
        try {
          await openFileInEditorRef.current(tab.filePath, tab.line, tab.column, { silent: true });
        } catch {
          // File may have been deleted since the last session — skip it.
        }
      }
      if (cancelled || activeWorkspaceRestoreRef.current !== path) return;
      const savedActive = saved.activeTabId;
      if (savedActive && openTabsRef.current.some(t => t.id === savedActive)) {
        setActiveTabId(savedActive);
      }
    })();

    return () => { cancelled = true; };
  }, [activeWorkspacePath]);

  // Debounced persist of the ACTIVE workspace's code tabs (metadata only).
  // wsPathRef is updated synchronously on switch, so the write always lands
  // under the workspace whose tabs are currently in the refs.
  const persistSessionTimerRef = useRef<number | null>(null);
  const openTabsMetaKey = useMemo(
    () => openTabs.map(t => `${t.id}:${t.filePath}:${t.line}:${t.column}:${t.isPinned}:${t.isReadOnly}`).join('|'),
    [openTabs]
  );
  useEffect(() => {
    const path = wsPathRef.current;
    if (!path) return;
    if (persistSessionTimerRef.current !== null) window.clearTimeout(persistSessionTimerRef.current);
    persistSessionTimerRef.current = window.setTimeout(() => {
      persistSessionTimerRef.current = null;
      const tabs = openTabsRef.current.filter(t => t.type === 'code' && t.filePath);
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
        activeTabId: activeTabIdRef.current
      };
      try { localStorage.setItem(sessionKeyFor(path), JSON.stringify(payload)); } catch {}
    }, 400);
    return () => {
      if (persistSessionTimerRef.current !== null) window.clearTimeout(persistSessionTimerRef.current);
    };
  }, [openTabsMetaKey, activeTabId]);

  const updateFolderChildren = useCallback((folderPath: string, children: FileItem[]) => {
    setFiles(prevFiles => updateFolderChildrenInTree(prevFiles, folderPath, children));
  }, []);

  const openSettingsTab = useCallback((section?: string) => {
    if (section) {
      setSettingsActiveSection(section);
    }
    setMode('settings');
  }, [setMode]);

  const goBackToWorkspace = useCallback(() => {
    setMode(previousMode === 'settings' ? 'agent' : previousMode);
  }, [previousMode, setMode]);

  const openDiffInEditor = useCallback((diff: FileDiff) => {
    setActiveDiff(diff);
    // Register in diffs so EditorView's `diffs.find(d => d.id === tab.diffId)`
    // resolves — without this, git-sourced diff tabs silently fall through to
    // the code editor (agent diffs only worked because they were pre-registered).
    setDiffs(prev => {
      const idx = prev.findIndex(d => d.id === diff.id);
      if (idx === -1) return [...prev, diff];
      const next = [...prev];
      next[idx] = diff;
      return next;
    });
    const diffTabId = `tab-diff-${diff.id}`;
    const existing = openTabsRef.current.find(t => t.id === diffTabId);
    if (existing) {
      openTab(existing);
    } else {
      const newTab: EditorTab = {
        id: diffTabId,
        fileId: `file-diff-${diff.id}`,
        fileName: `Diff: ${diff.fileName}`,
        filePath: diff.filePath,
        type: 'diff',
        diffId: diff.id,
        content: diff.modifiedContent
      };
      setOpenTabs(prev => [...prev, newTab]);
      openTab(newTab);
    }
    setMode('editor');
  }, [openTab, setMode]);

  // Full git graph pane in the editor area (lanes + commit detail).
  const openGitGraphPane = useCallback(() => {
    const graphTabId = 'tab-git-graph';
    const existing = openTabsRef.current.find(t => t.id === graphTabId);
    if (existing) {
      openTab(existing);
    } else {
      const newTab: EditorTab = {
        id: graphTabId,
        fileId: 'file-git-graph',
        fileName: 'Git Graph',
        filePath: '',
        type: 'git-graph',
        content: ''
      };
      setOpenTabs(prev => [...prev, newTab]);
      openTab(newTab);
    }
    setMode('editor');
  }, [openTab, setMode]);

  // Mixed editor terminal tab. `cwd` overrides the workspace folder so tab
  // menus can open a shell right next to a file.
  const openTerminalTab = useCallback(async (shellId?: string, cwd?: string) => {
    let sid = shellId;
    if (!sid) {
      try {
        // ssh:// paths route to a remote shell on the backend; empty folder
        // falls back to the default local folder.
        const sess = await CreateShell(`zsh (${openTabsRef.current.filter(t => t.type === 'terminal').length + 1})`, cwd || activeWorkspacePath || '');
        if (sess?.id) sid = sess.id;
      } catch (err) {
        console.warn('Failed to spawn shell for tab:', err);
        showToast(err instanceof Error ? err.message : String(err), 'error');
        return;
      }
    }
    const tabId = `tab-terminal-${sid || Date.now()}`;
    const newTab: EditorTab = {
      id: tabId,
      fileId: `file-terminal-${sid || Date.now()}`,
      fileName: 'Terminal',
      filePath: 'terminal',
      type: 'terminal',
      terminalSessionId: sid,
      content: ''
    };
    setOpenTabs(prev => [...prev, newTab]);
    openTab(newTab);
    setMode('editor');
  }, [activeWorkspacePath, openTab, setMode]);

  // Closing a tab removes it completely: when the active tab is closed the
  // neighbour (right, else left) becomes active; closing the last tab leaves
  // NO active tab so the pane shows its empty state instead of a phantom file.
  const closeTab = useCallback((tabId: string) => {
    const targetTab = openTabsRef.current.find(t => t.id === tabId);
    if (targetTab?.terminalSessionId) {
      void StopSession(targetTab.terminalSessionId).catch(() => {});
    }
    setActiveTabId(cur => calculateActiveTabOnClose(openTabsRef.current, tabId, cur));
    setOpenTabs(prev => prev.filter(t => t.id !== tabId));
  }, []);

  // Bulk close for tab-menu actions (Close Others/Left/Right/All). One
  // functional update — calling closeTab in a loop would run on a stale
  // snapshot and only the last removal would stick.
  const closeTabs = useCallback((ids: string[]) => {
    if (ids.length === 0) return;
    const idSet = new Set(ids);
    for (const id of ids) {
      const targetTab = openTabsRef.current.find(t => t.id === id);
      if (targetTab?.terminalSessionId) {
        void StopSession(targetTab.terminalSessionId).catch(() => {});
      }
    }
    setOpenTabs(prev => filterTabsOnClose(prev, ids));
    setActiveTabId(cur => (cur && idSet.has(cur) ? null : cur));
  }, []);

  const updateFileContent = useCallback(async (fileId: string, newContent: string) => {
    const targetTab = openTabsRef.current.find(t => t.fileId === fileId || t.filePath === fileId);
    const targetPath = targetTab ? targetTab.filePath : fileId;

    await ApiBridge.writeFile(targetPath, newContent);
    setFiles(prev => updateFileContentInTree(prev, targetPath, newContent));
    setOpenTabs(prev => prev.map(t => {
      if (t.fileId === fileId || t.filePath === fileId || t.filePath === targetPath) {
        return { ...t, content: newContent };
      }
      return t;
    }));
  }, []);

  // Editor typing: buffer-only. The per-keystroke disk write made typing a
  // disk I/O hot path; the write happens on explicit save instead.
  const setTabBuffer = useCallback((fileId: string, content: string) => {
    setOpenTabs(prev => updateTabBuffer(prev, fileId, content).updatedTabs);
  }, []);

  // Tab context-menu toggles (Zed-style tab pane menu).
  const toggleTabPin = useCallback((tabId: string) => {
    setOpenTabs(prev => prev.map(t => (t.id === tabId ? { ...t, isPinned: !t.isPinned } : t)));
  }, []);

  const toggleTabReadOnly = useCallback((tabId: string) => {
    setOpenTabs(prev => prev.map(t => {
      if (t.id !== tabId) return t;
      const isReadOnly = !t.isReadOnly;
      // Making a dirty tab read-only keeps its buffer text; it just can't be
      // edited further until the flag is flipped back.
      return { ...t, isReadOnly };
    }));
  }, []);

  const saveTabToDisk = useCallback(async (fileId: string, content: string) => {
    const targetTab = openTabsRef.current.find(t => t.fileId === fileId || t.filePath === fileId);
    const targetPath = targetTab ? targetTab.filePath : fileId;
    await ApiBridge.writeFile(targetPath, content);
    setOpenTabs(prev => prev.map(t =>
      (t.fileId === fileId || t.filePath === fileId || t.filePath === targetPath)
        ? { ...t, content, isModified: false }
        : t
    ));
    void refreshGitStatus();
  }, [refreshGitStatus]);

  const revalidateFileMutations = useCallback(async () => {
    try {
      await refreshFiles();
      void refreshGitStatus();
      void refreshGitLog();
    } catch (e) {
      console.error('File mutation revalidation failed:', e);
    }
  }, [refreshFiles, refreshGitStatus, refreshGitLog]);

  const createFile = useCallback(async (filePath: string, content = '') => {
    const fullPath = resolveWorkspacePath(activeWorkspacePath, filePath);
    await ApiBridge.createFile(fullPath, content);
    await revalidateFileMutations();
    await openFileInEditor(fullPath);
  }, [activeWorkspacePath, revalidateFileMutations, openFileInEditor]);

  const createFolder = useCallback(async (folderPath: string) => {
    const fullPath = resolveWorkspacePath(activeWorkspacePath, folderPath);
    await ApiBridge.createFile(fullPath, '');
    await revalidateFileMutations();
  }, [activeWorkspacePath, revalidateFileMutations]);

  const deleteFile = useCallback(async (filePath: string) => {
    await ApiBridge.deleteFile(filePath);
    setOpenTabs(prev => updateTabsOnDelete(prev, filePath));
    closeSideFile(filePath);
    await revalidateFileMutations();
  }, [closeSideFile, revalidateFileMutations]);

  const renameFile = useCallback(async (oldPath: string, newPath: string) => {
    await ApiBridge.renameFile(oldPath, newPath);
    const newName = newPath.split('/').pop() || newPath;
    setOpenTabs(prev => updateTabsOnRename(prev, oldPath, newPath));
    setSideFileTabs(prev => prev.map(t => {
      if (t.path === oldPath) {
        return { ...t, path: newPath, name: newName };
      }
      if (t.path.startsWith(oldPath + '/')) {
        const sub = t.path.slice(oldPath.length);
        return { ...t, path: `${newPath}${sub}` };
      }
      return t;
    }));
    setActiveSideFile(prev => (prev === oldPath ? newPath : prev));
    await revalidateFileMutations();
  }, [revalidateFileMutations]);

  const moveFile = useCallback(async (srcPath: string, destPath: string) => {
    await ApiBridge.moveFile(srcPath, destPath);
    const newName = destPath.split('/').pop() || destPath;
    setOpenTabs(prev => updateTabsOnRename(prev, srcPath, destPath));
    setSideFileTabs(prev => prev.map(t => {
      if (t.path === srcPath) {
        return { ...t, path: destPath, name: newName };
      }
      if (t.path.startsWith(srcPath + '/')) {
        const sub = t.path.slice(srcPath.length);
        return { ...t, path: `${destPath}${sub}` };
      }
      return t;
    }));
    setActiveSideFile(prev => (prev === srcPath ? destPath : prev));
    await revalidateFileMutations();
  }, [revalidateFileMutations]);

  const copyFile = useCallback(async (srcPath: string, destPath: string) => {
    await ApiBridge.copyPath(srcPath, destPath);
    await revalidateFileMutations();
  }, [revalidateFileMutations]);

  const acceptDiff = useCallback(async (diffId: string) => {
    await useGitStore.getState().acceptDiff(diffId, refreshFiles);
  }, [refreshFiles]);

  // ---------------------------------------------------------------------------
  // Go agent harness path — internal agents run through the Go backend loop
  // (native tool calling, disk-applied edit/write tools, approval gate) like
  // the harness reference, instead of the webview LLM loop.
  // ---------------------------------------------------------------------------

  // Builds the event callbacks that map harness events into a store session's
  // last agent message. Registered once per Go session id.
  const makeGoCallbacks = useCallback((storeSessionId: string, goSessionId: string) => {
    return createAgentCallbacks({
      storeSessionId,
      goSessionId,
      activeWorkspacePath,
      addDiff,
      updateSession: (sessionId, updater) => {
        setSessions(prev => prev.map(s => s.id === sessionId ? updater(s) : s));
      }
    });
  }, [activeWorkspacePath, addDiff, setSessions]);

  // ensureGoSession provisions (or reuses) the Go harness session for a store
  // session and registers its event callbacks.
  const ensureGoSession = useCallback(async (storeSession: AgentSession): Promise<string | null> => {
    if (storeSession.goSessionId) {
      goAgentSessions.register(storeSession.goSessionId, makeGoCallbacks(storeSession.id, storeSession.goSessionId));
      return storeSession.goSessionId;
    }
    const goId = await goAgentSessions.createSession(storeSession.title, storeSession.workspacePath || activeWorkspacePath);
    if (!goId) return null;
    storeSession.goSessionId = goId;
    setSessions(prev => prev.map(s => s.id === storeSession.id ? { ...s, goSessionId: goId, goState: 'idle' } : s));
    goAgentSessions.register(goId, makeGoCallbacks(storeSession.id, goId));
    // Seed the composer's permission mode into the fresh Go session so
    // "Full access" sessions never gate mutating tools on first use.
    SetAgentAutoApprove(goId, agentExecutionMode === 'bypass').catch(() => {});
    SetActiveReasoningEffort(reasoningLevel).catch(() => {});
    return goId;
  }, [activeWorkspacePath, makeGoCallbacks, agentExecutionMode, reasoningLevel]);

  // streamMainSessionDirectly provides robust direct streaming for the main
  // session when the Go harness is unreachable, unlinked, or timing out.
  const streamMainSessionDirectly = useCallback(async (storeSessionId: string, promptText: string, sessionOverride?: AgentSession) => {
    await streamAgentSessionDirectly({
      engine: engineRef.current,
      storeSessionId,
      promptText,
      sessionOverride,
      sessions,
      currentModel,
      activeWorkspacePath,
      gitFiles,
      gitBranch,
      addDiff,
      updateSession: (sessionId, updater) => {
        setSessions(prev => prev.map(s => s.id === sessionId ? updater(s) : s));
      }
    });
  }, [sessions, currentModel, activeWorkspacePath, gitFiles, gitBranch, addDiff, setSessions]);

  // sendGoAgentMessage routes one user prompt through the Go harness with
  // automatic fallback to direct streaming if the harness is unavailable or slow.
  const sendGoAgentMessage = useCallback(async (storeSessionId: string, prompt: string, sessionOverride?: AgentSession) => {
    const storeSession = sessionOverride
      || sessionsRef.current.find(s => s.id === storeSessionId)
      || sessions.find(s => s.id === storeSessionId);
    if (!storeSession) return;

    let goId: string | null = null;
    try {
      goId = await Promise.race([
        ensureGoSession(storeSession),
        new Promise<null>(resolve => setTimeout(() => resolve(null), 2500))
      ]);
    } catch {
      goId = null;
    }

    if (!goId) {
      return streamMainSessionDirectly(storeSessionId, prompt, sessionOverride);
    }

    try {
      await activateGoModel(storeSession.model || currentModel);
      await goAgentSessions.send(goId, prompt, extractMentionedPaths(prompt, storeSession.workspacePath || activeWorkspacePath));

      // Liveness note: the Go harness owns this turn end-to-end. Slow providers
      // (reasoning models, remote endpoints, tool-call generation) can
      // legitimately take well over 30s before the first token — killing the
      // turn here produced silent empty replies and a double-send fallback
      // loop (FORGE-144). The harness surfaces its own errors, so we only
      // log a liveness warning and let the turn finish.
      window.setTimeout(() => {
        const current = sessionsRef.current.find(s => s.id === storeSessionId);
        if (!current || current.status !== 'running') return;
        const lastMsg = current.messages[current.messages.length - 1];
        if (
          lastMsg &&
          lastMsg.role === 'agent' &&
          !lastMsg.content &&
          (!lastMsg.thoughts || lastMsg.thoughts.length === 0) &&
          (!lastMsg.toolExecutions || lastMsg.toolExecutions.length === 0)
        ) {
          console.warn('[workspaceStore] Go turn has no deltas after 120s — provider slow; harness keeps waiting and will surface errors itself.');
        }
      }, 120000);
    } catch (err: any) {
      console.warn('[workspaceStore] Go send failed, falling back to direct stream:', err);
      return streamMainSessionDirectly(storeSessionId, prompt, sessionOverride);
    }
  }, [activeWorkspacePath, ensureGoSession, sessions, activateGoModel, currentModel, streamMainSessionDirectly]);

  // sendACPAgentMessage routes a turn to an external ACP agent process via stdio JSON-RPC.
  const sendACPAgentMessage = useCallback(async (storeSessionId: string, prompt: string, sessionOverride?: AgentSession) => {
    const storeSession = sessionOverride
      || sessionsRef.current.find(s => s.id === storeSessionId)
      || sessions.find(s => s.id === storeSessionId);
    if (!storeSession) return;

    const agentObj = agents.find(a => a.id === storeSession.agentId) || activeAgent;
    let acpId = storeSession.acpSessionId;

    const startOrResumeACPSession = async (previousId?: string): Promise<string> => {
      let permMode: 'supervised' | 'auto' | 'auto_edits' | 'full' = 'supervised';
      if (agentExecutionMode === 'bypass') permMode = 'full';
      else if (agentExecutionMode === 'plan') permMode = 'auto_edits';
      else if (agentExecutionMode === 'code') permMode = 'auto';

      const cmd = agentObj.command || (
        agentObj.id === 'agent-agy' ? 'agy' :
        agentObj.id === 'agent-claude' ? 'claude' :
        agentObj.id === 'agent-codex' ? 'codex' :
        agentObj.id === 'agent-opencode' ? 'opencode' :
        agentObj.id === 'agent-commandcode' ? 'cmd' :
        agentObj.id === 'agent-pi' ? 'pi-acp' :
        agentObj.id === 'agent-ohmypi' ? 'omp' : 'agy'
      );

      let agentArgs = agentObj.args || [];
      if ((agentObj.id === 'agent-ohmypi' || agentObj.id === 'agent-commandcode') && agentArgs.length === 0) {
        agentArgs = ['acp'];
      }

      const rawModel = storeSession.model || currentModel;
      const effectiveModel = (rawModel && rawModel !== 'default' && rawModel !== 'Default') ? rawModel : '';
      const resumeRemoteId = storeSession.remoteSessionId || previousId || storeSession.acpSessionId;

      const acpSession = await ACPStartSession({
        agentId: agentObj.id,
        title: storeSession.title || 'ACP Session',
        command: cmd,
        args: agentArgs,
        cwd: storeSession.workspacePath || activeWorkspacePath || '.',
        model: effectiveModel,
        permissionMode: permMode,
        sessionId: previousId || storeSession.acpSessionId,
        resumeSessionId: resumeRemoteId
      });

      if (acpSession && acpSession.id) {
        const newAcpId = acpSession.id;
        acpId = newAcpId;
        storeSession.acpSessionId = newAcpId;
        storeSession.remoteSessionId = acpSession.remoteSessionId || acpSession.id;
        const initialCmds = (acpSession as any).availableCommands || agentObj.commands || [];
        setSessions(prev => prev.map(s => s.id === storeSession.id ? {
          ...s,
          acpSessionId: newAcpId,
          remoteSessionId: acpSession.remoteSessionId || acpSession.id,
          availableCommands: s.availableCommands && s.availableCommands.length > 0 ? s.availableCommands : initialCmds,
          goState: 'idle'
        } : s));
        goAgentSessions.register(newAcpId, makeGoCallbacks(storeSession.id, newAcpId));
        return newAcpId;
      }
      throw new Error(`Failed to initialize or resume ACP session with agent "${agentObj.name}"`);
    };

    if (!acpId) {
      try {
        acpId = await startOrResumeACPSession();
      } catch (err: any) {
        console.error('[workspaceStore] ACP start session error:', err);
        setSessions(prev => prev.map(s => {
          if (s.id !== storeSessionId) return s;
          const msgs = s.messages.map((m, idx) => {
            if (m.role === 'agent' && (idx === s.messages.length - 1 || m.isThinking)) {
              return {
                ...m,
                isThinking: false,
                content: `⚠️ **ACP Agent Error**: Could not launch **${agentObj.name}** (\`${agentObj.command || 'cli'}\`).\n\n${err?.message || String(err)}\n\nPlease ensure \`${agentObj.command || 'cli'}\` is installed in your system PATH.`
              };
            }
            return m;
          });
          return { ...s, status: 'completed' as const, messages: msgs };
        }));
        return;
      }
    }

    try {
      goAgentSessions.register(acpId, makeGoCallbacks(storeSession.id, acpId));
      await ACPPrompt(acpId, prompt);
    } catch (err: any) {
      const errMsg = err?.message || String(err);
      if (errMsg.includes('session not found') || errMsg.includes('no active remote session') || errMsg.includes('client closed') || errMsg.includes('connection closed')) {
        console.warn(`[workspaceStore] ACP session ${acpId} not active in backend, auto-resuming...`);
        try {
          const freshAcpId = await startOrResumeACPSession(acpId);
          goAgentSessions.register(freshAcpId, makeGoCallbacks(storeSession.id, freshAcpId));
          await ACPPrompt(freshAcpId, prompt);
          return;
        } catch (resumeErr: any) {
          console.error('[workspaceStore] ACP auto-resume failed:', resumeErr);
          setSessions(prev => prev.map(s => {
            if (s.id !== storeSessionId) return s;
            const msgs = s.messages.map((m, idx) => {
              if (m.role === 'agent' && (idx === s.messages.length - 1 || m.isThinking)) {
                return {
                  ...m,
                  isThinking: false,
                  content: (m.content || '') + `\n\n⚠️ **ACP Resume Error**: ${resumeErr?.message || String(resumeErr)}`
                };
              }
              return m;
            });
            return { ...s, status: 'completed' as const, messages: msgs };
          }));
          return;
        }
      }

      console.error('[workspaceStore] ACP prompt error:', err);
      setSessions(prev => prev.map(s => {
        if (s.id !== storeSessionId) return s;
        const msgs = s.messages.map((m, idx) => {
          if (m.role === 'agent' && (idx === s.messages.length - 1 || m.isThinking)) {
            return {
              ...m,
              isThinking: false,
              content: (m.content || '') + `\n\n⚠️ **ACP Error**: ${errMsg}`
            };
          }
          return m;
        });
        return { ...s, status: 'completed' as const, messages: msgs };
      }));
    }
  }, [agents, activeAgent, agentExecutionMode, activeWorkspacePath, currentModel, makeGoCallbacks, setSessions, sessions]);

  // sendAgentMessage unifies routing between the internal Go harness and external ACP agents.
  const sendAgentMessage = useCallback((storeSessionId: string, prompt: string, sessionOverride?: AgentSession) => {
    const storeSession = sessionOverride
      || sessionsRef.current.find(s => s.id === storeSessionId)
      || sessions.find(s => s.id === storeSessionId);
    if (!storeSession) return;

    const agentObj = agents.find(a => a.id === storeSession.agentId) || activeAgent;
    if (agentObj.type !== 'internal') {
      return sendACPAgentMessage(storeSessionId, prompt, sessionOverride);
    }
    return sendGoAgentMessage(storeSessionId, prompt, sessionOverride);
  }, [agents, activeAgent, sendACPAgentMessage, sendGoAgentMessage, sessions]);

  const createNewSession = useCallback((initialPrompt?: string, selectedAgentId?: string, forcedId?: string): string => {
    const newId = forcedId || `session-${Date.now()}`;
    const effectiveAgentId = selectedAgentId || activeAgentId;
    const agentObj = agents.find(a => a.id === effectiveAgentId) || activeAgent;

    let effectivePrompt = initialPrompt;
    if (effectivePrompt && effectivePrompt.startsWith('/')) {
      const parts = effectivePrompt.slice(1).split(/\s+/);
      const cmdName = parts[0].toLowerCase();
      const userArgs = parts.slice(1).join(' ');
      const matched = customCommands.find(c => c && c.enabled && (c.name || '').toLowerCase() === cmdName);
      if (matched) {
        const template = matched.promptTemplate || (matched as any).prompt || '';
        effectivePrompt = userArgs
          ? `${template}\n\nUser instructions: ${userArgs}`
          : template;
      }
    }

    const effectiveModel = agentObj.type !== 'internal' ? (currentModel || agentObj.model || '') : (agentObj.model || currentModel);
    const matchedProvider = providers.find(p => p.enabled && (p.selectedModels || p.models || []).includes(effectiveModel));
    const effectiveProvider = agentObj.type !== 'internal' ? agentObj.provider : (matchedProvider ? matchedProvider.id : 'custom');

    const newSession: AgentSession = {
      id: newId,
      title: initialPrompt ? formatSmartSessionTitle(initialPrompt) : 'New Session',
      status: initialPrompt ? 'running' : 'idle',
      createdAt: new Date().toISOString(),
      updatedAt: new Date().toISOString(),
      model: effectiveModel,
      provider: effectiveProvider,
      agentId: agentObj.id,
      agentType: agentObj.type,
      workspacePath: activeWorkspacePath,
      availableCommands: agentObj.commands || [],
      messages: initialPrompt
        ? [
            {
              id: `msg-user-${Date.now()}`,
              role: 'user',
              content: initialPrompt,
              timestamp: 'Just now'
            },
            {
              id: `msg-agent-${Date.now()}`,
              role: 'agent',
              agentId: agentObj.id,
              agentName: agentObj.name,
              content: '',
              timestamp: new Date().toISOString(),
              isThinking: true,
              thoughts: []
            }
          ]
        : [],
      diffs: []
    };

    setSessions(prev => [newSession, ...prev]);
    setActiveSessionId(newId);
    setMode('agent');
    setActiveTaskKind('forge');

    if (effectivePrompt) {
      void sendAgentMessage(newId, effectivePrompt, newSession);
    }
    return newId;
  }, [agents, activeAgentId, activeAgent, currentModel, activeWorkspacePath, customCommands, sendAgentMessage, setActiveTaskKind, providers, setMode]);

  // Wire the deferred reference used by runAutomation (createNewSession is
  // declared earlier; keep it fresh on every change).
  useEffect(() => {
    createNewSessionRef.current = createNewSession;
  }, [createNewSession]);

  const compactCurrentSession = useCallback(async () => {
    if (!activeSessionId) return;
    const sess = sessions.find(s => s.id === activeSessionId) || sessionsRef.current.find(s => s.id === activeSessionId);
    if (!sess) return;

    if (!sess.messages || sess.messages.length <= 2) {
      showToast('Session is already compact', 'info');
      return;
    }

    // Call Go harness if session has a Go harness session ID
    if (sess.goSessionId) {
      try {
        await CompactAgentSession(sess.goSessionId);
      } catch (e) {
        console.warn('Failed to compact Go session:', e);
      }
    }

    // Compact frontend session messages
    setSessions(prev => prev.map(s => {
      if (s.id !== activeSessionId) return s;

      const msgs = s.messages;
      const keepTail = msgs.length > 4 ? 3 : 1;
      const firstMsg = msgs[0];
      const recentMsgs = msgs.slice(msgs.length - keepTail);
      const compactedCount = msgs.length - (1 + recentMsgs.length);

      // Extract user requests and goals from truncated messages
      const earlierRequests = msgs.slice(1, msgs.length - keepTail)
        .filter(m => m.role === 'user' && m.content?.trim())
        .map(m => m.content.trim().slice(0, 120))
        .slice(0, 8);

      const summaryLines = [
        `**Session Compacted**: Earlier ${compactedCount} turns were compressed to free up the context window.`,
        '',
        earlierRequests.length > 0 ? `**Prior Objectives**:\n${earlierRequests.map((r, i) => `${i + 1}. ${r}`).join('\n')}` : '',
        '',
        '_The recent conversation turns and active context are preserved._'
      ].filter(Boolean).join('\n');

      const compactSummaryMsg: AgentMessage = {
        id: `compact-msg-${Date.now()}`,
        role: 'agent',
        agentId: s.agentId || 'agent-internal',
        agentName: 'ForgeADE System',
        content: `📦 ${summaryLines}`,
        timestamp: 'Just now',
        isThinking: false
      };

      const compactedMessages = [firstMsg, compactSummaryMsg, ...recentMsgs];

      const updated = {
        ...s,
        messages: compactedMessages,
        updatedAt: new Date().toISOString()
      };

      try {
        ApiBridge.saveSessionJsonl(updated, updated.workspacePath ?? activeWorkspacePath);
      } catch {}

      return updated;
    }));

    showToast('Session compacted: earlier turns compressed into summary', 'success');
  }, [activeSessionId, sessions, activeWorkspacePath]);

  const sendAgentPrompt = useCallback((promptText: string) => {
    if (!promptText.trim()) return;

    let effectivePrompt = promptText.trim();
    if (effectivePrompt.startsWith('/')) {
      const parts = effectivePrompt.slice(1).split(/\s+/);
      const cmdName = parts[0].toLowerCase();
      const userArgs = parts.slice(1).join(' ');
      const matched = customCommands.find(c => c && c.enabled && (c.name || '').toLowerCase() === cmdName);
      if (matched) {
        const template = matched.promptTemplate || (matched as any).prompt || '';
        effectivePrompt = userArgs
          ? `${template}\n\nUser instructions: ${userArgs}`
          : template;
      }
    }

    if (effectivePrompt.toLowerCase() === '/compact') {
      void compactCurrentSession();
      return;
    }

    if (!activeSessionId) {
      createNewSession(promptText);
      return;
    }

    const agentObj = agents.find(a => a.id === activeAgentId) || activeAgent;

    const userMsg: AgentMessage = {
      id: `msg-${Date.now()}`,
      role: 'user',
      content: promptText,
      timestamp: 'Just now'
    };

    const agentPlaceholder: AgentMessage = {
      id: `msg-${Date.now() + 1}`,
      role: 'agent',
      agentId: agentObj.id,
      agentName: agentObj.name,
      content: '',
      timestamp: new Date().toISOString(),
      isThinking: true,
      thoughts: []
    };

    setSessions(prev => prev.map(s => {
      if (s.id !== activeSessionId) return s;
      const isDefaultTitle = !s.title || s.title === 'New Session' || s.title.startsWith('session-');
      const updatedTitle = isDefaultTitle ? formatSmartSessionTitle(promptText) : s.title;
      const cleanedPrevMessages = s.messages.map(m => m.isThinking ? { ...m, isThinking: false } : m);
      return {
        ...s,
        title: updatedTitle,
        status: 'running',
        updatedAt: new Date().toISOString(),
        messages: [...cleanedPrevMessages, userMsg, agentPlaceholder]
      };
    }));

    // Route through unified message sender (supports internal Go harness & external ACP agents)
    void sendAgentMessage(activeSessionId, effectivePrompt);
  }, [activeSessionId, agents, activeAgent, customCommands, sendAgentMessage, compactCurrentSession, createNewSession, activeAgentId]);

  const steerAgentPrompt = useCallback(async (instruction: string) => {
    const trimmed = instruction.trim();
    if (!trimmed || !activeSessionId) return;

    const currentSession = sessionsRef.current.find(s => s.id === activeSessionId) || sessions.find(s => s.id === activeSessionId);
    if (!currentSession) return;

    // If the agent turn is not actively running, execute as a regular prompt turn
    if (currentSession.status !== 'running') {
      sendAgentPrompt(trimmed);
      return;
    }

    // Mid-turn steering behaves like a normal user prompt with its own agent response placeholder
    const userMsg: AgentMessage = {
      id: `msg-${Date.now()}`,
      role: 'user',
      content: trimmed,
      timestamp: 'Just now'
    };

    const agentPlaceholder: AgentMessage = {
      id: `msg-${Date.now() + 1}`,
      role: 'agent',
      agentId: 'agent-internal',
      agentName: 'ForgeADE',
      content: '',
      timestamp: new Date().toISOString(),
      isThinking: true,
      thoughts: []
    };

    const goId = currentSession.goSessionId || (await ensureGoSession(currentSession));
    if (goId) {
      goAgentSessions.preempt(goId);
    }

    setSessions(prev => prev.map(s => {
      if (s.id !== activeSessionId) return s;
      const cleanedPrevMessages = s.messages.map(m => {
        if (m.isThinking) {
          return {
            ...m,
            isThinking: false,
            toolStatus: undefined,
            thoughts: (m.thoughts || []).map(th => ({
              ...th,
              durationSeconds: th.durationSeconds && th.durationSeconds > 0
                ? th.durationSeconds
                : Math.max(1, Math.round((Date.now() - (th.createdAtMs || Date.now())) / 1000))
            }))
          };
        }
        return m;
      });
      return {
        ...s,
        updatedAt: new Date().toISOString(),
        messages: [...cleanedPrevMessages, userMsg, agentPlaceholder]
      };
    }));

    if (goId) {
      await goAgentSessions.steer(goId, trimmed);
    }
  }, [activeSessionId, sessions, sendAgentPrompt, ensureGoSession]);

  const stopAgentExecution = useCallback(() => {
    // Cancel the backend turn; the agent:turn_end event finalizes the message.
    const currentSession = activeSessionId
      ? (sessionsRef.current.find(s => s.id === activeSessionId) || sessions.find(s => s.id === activeSessionId))
      : null;
    if (currentSession?.goSessionId) {
      StopAgentTurn(currentSession.goSessionId);
    }
    if (currentSession?.acpSessionId) {
      ACPCancel(currentSession.acpSessionId).catch(() => {});
    }
    engineRef.current.abort();
    if (!activeSessionId) return;
    setSessions(prev => prev.map(s => {
      if (s.id !== activeSessionId) return s;
      const msgs = s.messages.map((m, idx) => {
        if (m.role === 'agent' && (idx === s.messages.length - 1 || m.isThinking)) {
          let content = (m.content || '')
            .replace(/<tool(?:_call)?[\s\S]*$/i, '')
            .replace(/<invoke[\s\S]*$/i, '')
            .replace(/<function_call[\s\S]*$/i, '')
            .replace(/<action[\s\S]*$/i, '')
            .replace(/\s*\(\*?stopped by user\*?\)/gi, '')
            .replace(/\s*\(Execution stopped by user\)/gi, '')
            .trim();
          return {
            ...m,
            isThinking: false,
            content: content ? `${content}\n\n*(stopped by user)*` : '*(stopped by user)*'
          };
        }
        return m;
      });
      const updatedSession = { ...s, status: 'idle' as const, goState: 'idle', pendingTools: [], pendingQuestions: [], messages: msgs };
      ApiBridge.saveSessionJsonl(updatedSession, updatedSession.workspacePath ?? activeWorkspacePath);
      return updatedSession;
    }));
  }, [activeSessionId, sessions, activeWorkspacePath]);

  const openSessionFromHistory = useCallback((sessionToOpen: AgentSession) => {
    setSessions(prev => {
      const exists = prev.some(s => s.id === sessionToOpen.id);
      if (exists) return prev;
      return [sessionToOpen, ...prev];
    });
    setActiveSessionId(sessionToOpen.id);
    if (sessionToOpen.model) {
      setCurrentModelState(sessionToOpen.model);
      try { localStorage.setItem('forge_ade_current_model', sessionToOpen.model); } catch {}
    }
    if (sessionToOpen.agentId) {
      setActiveAgentIdState(sessionToOpen.agentId);
      try { localStorage.setItem('forge_ade_active_agent_id', sessionToOpen.agentId); } catch {}
    }
    setMode('agent');
    setActiveTaskKind('forge');
  }, [setActiveTaskKind, setMode]);

  const deleteSessionPermanently = useCallback(async (id: string) => {
    // 0. Release the Go-side streaming state for this session (callbacks,
    //    stream previews, sync timers) so deleted sessions stop consuming memory.
    const deleted = sessionsRef.current.find(s => s.id === id);
    if (deleted?.goSessionId) goAgentSessions.unregister(deleted.goSessionId);

    // 1. Immediately update in-memory state and localStorage to prevent any resurrection
    setSavedSessions(prev => prev.filter(s => s.id !== id));
    setSessions(prev => {
      const next = prev.filter(s => s.id !== id);
      try {
        localStorage.setItem('forge_ade_sessions', JSON.stringify(next));
      } catch {}
      return next;
    });

    if (activeSessionId === id) {
      setActiveSessionId(null);
    }

    // 2. Remove session file from disk
    await ApiBridge.deleteSessionJsonl(id, activeWorkspacePath);
  }, [activeWorkspacePath, activeSessionId]);

  const [archivedSessionIds, setArchivedSessionIds] = useState<Set<string>>(() => {
    try {
      const s = localStorage.getItem('forge_archived_sessions');
      return s ? new Set(JSON.parse(s)) : new Set();
    } catch {
      return new Set();
    }
  });

  const toggleArchiveSession = useCallback((id: string) => {
    setArchivedSessionIds(prev => {
      const next = new Set(prev);
      if (next.has(id)) next.delete(id);
      else next.add(id);
      try {
        localStorage.setItem('forge_archived_sessions', JSON.stringify(Array.from(next)));
      } catch {}
      return next;
    });
  }, []);

  const deleteSession = useCallback((id: string) => {
    deleteSessionPermanently(id);
  }, [deleteSessionPermanently]);

  // ── Subagents & Floating Task Tracker ─────────────────────────────────────
  const [subagentSessions, setSubagentSessions] = useState<Record<string, AgentSession>>({});
  const [activeSubagentId, setActiveSubagentId] = useState<string | null>(null);

  const subagents = useMemo(() => {
    const ids = activeSession?.subagentIds || [];
    return ids.map(id => subagentSessions[id]).filter(Boolean) as AgentSession[];
  }, [activeSession?.subagentIds, subagentSessions]);

  useEffect(() => {
    const ids = activeSession?.subagentIds;
    if (!ids || ids.length === 0) return;
    for (const id of ids) {
      if (!subagentSessions[id]) {
        void GetAgentSession(id).then(snap => {
          if (snap) {
            const sess = convertGoSessionToAgentSession(snap);
            setSubagentSessions(prev => ({ ...prev, [id]: sess }));
          }
        }).catch(() => {});
      }
    }
  }, [activeSession?.subagentIds, subagentSessions]);

  useEffect(() => {
    const unsub = EventsOn('agent:updated', (data: any) => {
      const id = data?.session_id;
      if (!id) return;
      if (activeSession?.subagentIds?.includes(id)) {
        void GetAgentSession(id).then(snap => {
          if (snap) {
            const sess = convertGoSessionToAgentSession(snap);
            setSubagentSessions(prev => ({ ...prev, [id]: sess }));
          }
        }).catch(() => {});
      }
    });
    return unsub;
  }, [activeSession?.subagentIds]);

  useEffect(() => {
    const unsub = EventsOn('agent:available_commands', (data: any) => {
      const sessId = data?.sessionId || data?.session_id;
      const cmds = data?.commands;
      if (!sessId || !Array.isArray(cmds)) return;
      setSessions(prev => prev.map(s => {
        if (s.id === sessId || s.acpSessionId === sessId || s.remoteSessionId === sessId) {
          return { ...s, availableCommands: cmds };
        }
        return s;
      }));
    });
    return unsub;
  }, []);

  const toggleTask = useCallback(async (taskId: string, completed: boolean) => {
    if (!activeSession) return;
    setSessions(prev => prev.map(s => {
      if (s.id !== activeSession.id) return s;
      const updatedTasks = (s.tasks || []).map(t => {
        if (t.id === taskId) {
          return { ...t, completed, status: completed ? ('completed' as TaskStatus) : ('pending' as TaskStatus) };
        }
        return t;
      });
      return { ...s, tasks: updatedTasks };
    }));
    const goId = activeSession.goSessionId || activeSession.id;
    try {
      await ToggleAgentTask(goId, taskId, completed);
    } catch (err) {
      console.error('Failed to toggle agent task:', err);
    }
  }, [activeSession]);

  const clearTasks = useCallback(async () => {
    if (!activeSession) return;
    setSessions(prev => prev.map(s => {
      if (s.id !== activeSession.id) return s;
      return { ...s, tasks: [] };
    }));
    const goId = activeSession.goSessionId || activeSession.id;
    try {
      await ClearAgentTasks(goId);
    } catch (err) {
      console.error('Failed to clear agent tasks:', err);
    }
  }, [activeSession]);

  const openSubagentInSidebar = useCallback((subagentId: string) => {
    setActiveSubagentId(subagentId);
    setIsRightActionDrawerOpen(true);
    window.dispatchEvent(new CustomEvent('forge:open-subagent-tab', { detail: { subagentId } }));
  }, [setIsRightActionDrawerOpen]);

  const sendSubagentMessage = useCallback(async (subagentId: string, prompt: string) => {
    if (!prompt.trim() || !subagentId) return;
    try {
      await SendAgentMessage(subagentId, prompt, []);
    } catch (err) {
      console.error('Failed to send message to subagent:', err);
    }
  }, []);

  // Dev benchmark harness hooks (window.__forgeBench) — no-op in prod builds.
  useEffect(() => {
    registerBenchStore({
      openFile: path => openFileInEditor(path),
      closeActiveTab: () => {
        if (activeTabIdRef.current) closeTab(activeTabIdRef.current);
      },
      activeTabPath: () => openTabsRef.current.find(t => t.id === activeTabIdRef.current)?.filePath ?? null,
      tabCount: () => openTabsRef.current.length,
      refreshFiles
    });
  }, [openFileInEditor, closeTab, refreshFiles]);

  // Global Keyboard Shortcuts
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      const mod = e.metaKey || e.ctrlKey;
      const key = e.key.toLowerCase();

      // Shifted combos must be tested first — Cmd+Shift+P would otherwise be
      // swallowed by the plain Cmd+P quick-open branch below.
      if (mod && e.shiftKey && key === 'p') {
        e.preventDefault();
        openCommandPalette('commands');
      } else if (mod && e.shiftKey && key === 'o') {
        e.preventDefault();
        openCommandPalette('symbols');
      } else if (mod && e.shiftKey && key === 'f') {
        e.preventDefault();
        // Project-wide search lives in the editor sidebar's search activity.
        setMode('editor');
        setIsGitClientOpen(false);
        setIsLeftSidebarOpen(true);
        setActiveActivity('search');
      } else if (mod && e.shiftKey && key === 'e') {
        e.preventDefault();
        setMode('editor');
        setIsGitClientOpen(false);
        setIsLeftSidebarOpen(true);
        setActiveActivity('explorer');
      } else if (mod && e.shiftKey && key === 'g') {
        e.preventDefault();
        setMode('editor');
        setIsGitClientOpen(prev => {
          if (!prev) setIsLeftSidebarOpen(false);
          return !prev;
        });
      } else if (e.ctrlKey && e.key === 'Tab') {
        e.preventDefault();
        cycleTab(e.shiftKey ? -1 : 1);
      } else if (mod && key === 'k') {
        e.preventDefault();
        openCommandPalette('files');
      } else if (mod && key === 'p') {
        e.preventDefault();
        openCommandPalette('files');
      } else if (mod && key === 'n') {
        e.preventDefault();
        // One unified New Task flow: ForgeADE chat or an agent CLI.
        useSessionStore.getState().openNewTask();
      } else if (mod && key === 't') {
        e.preventDefault();
        useSessionStore.getState().openNewTask();
      } else if (e.altKey && key === 't') {
        e.preventDefault();
        useSessionStore.getState().openNewTask();
      } else if (mod && e.key === '1') {
        e.preventDefault();
        switchSurface('agent');
      } else if (mod && e.key === '2') {
        e.preventDefault();
        switchSurface('editor');
      } else if (mod && key === 'b') {
        // Toggle the left sidebar (promised by the sidebar tooltip, ZCode parity).
        e.preventDefault();
        setIsLeftSidebarOpen(prev => !prev);
      } else if (mod && e.key === '/') {
        e.preventDefault();
        toggleTheme();
      } else if (mod && e.key === ',') {
        e.preventDefault();
        openSettingsTab('general');
      } else if (mod && key === 'o') {
        e.preventDefault();
        openFolder();
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [switchSurface, toggleTheme, openSettingsTab, openFolder, openCommandPalette, cycleTab, setMode, setIsLeftSidebarOpen]);

  const switchWorkspace = useCallback(async (path: string) => {
    setActiveWorkspacePath(path);
  }, [setActiveWorkspacePath]);

  const canGoBack = nav.index > 0;
  const canGoForward = nav.index >= 0 && nav.index < nav.stack.length - 1;

  const contextValue = useMemo(() => ({
    mode,
    setMode,
    activeTaskKind,
    setActiveTaskKind,
    theme,
    setTheme,
    toggleTheme,
    themePalette,
    setThemePalette,
    recentWorkspaces,
    addRecentWorkspace,
    removeRecentWorkspace,
    clearRecentWorkspaces,
    closeWorkspace,
    disconnectSSH,
    activeSessionId,
    setActiveSessionId,
    sessions,
    activeSession,
    agents,
    activeAgentId,
    setActiveAgentId,
    activeAgent,
    toggleAgentEnabled,
    updateAgentConfig,
    refreshAgentModels,
    addAgent,
    deleteAgent,
    acpEnabled,
    setAcpEnabled,
    privacySettings,
    setPrivacySettings,
    providers,
    updateProvider,
    addProvider,
    deleteProvider,
    addModelToProvider,
    deleteModelFromProvider,
    toggleModelSelection,
    selectAllModels,
    deselectAllModels,
    fetchProviderModels,
    mcps,
    addMcp,
    toggleMcp,
    deleteMcp,
    skills,
    addSkill,
    toggleSkill,
    deleteSkill,
    createBackendSkill,
    deleteBackendSkill,
    reloadSkills,
    discoveredMcps,
    discoveredSkills,
    isDiscovering,
    runDiscovery,
    importDiscoveredMcp,
    importDiscoveredSkill,
    plugins,
    fetchPlugins,
    createPlugin,
    togglePlugin,
    deletePlugin,
    reloadPlugins,
    memories,
    fetchMemories,
    saveMemory,
    deleteMemory,
    reloadMemories,
    customCommands,
    addCustomCommand,
    updateCustomCommand,
    deleteCustomCommand,
    toggleCustomCommand,
    hooks,
    updateHook,
    toggleHook,
    addHook,
    deleteHook,
    browserSettings,
    updateBrowserSettings,
    computerSettings,
    updateComputerSettings,
    automations,
    automationsLoading,
    fetchAutomations,
    saveAutomation,
    deleteAutomation,
    setAutomationEnabled,
    runAutomation,
    indexStatus,
    fetchIndexStatus,
    reindexWorkspace,
    isReindexing,
    diffs,
    activeDiff,
    setActiveDiff,
    acceptDiff,
    rejectDiff,
    addDiff,
    openDiffInEditor,
    openGitGraphPane,
    gitBranch,
    gitFiles,
    gitCommits,
    refreshGitStatus,
    refreshGitLog,
    files,
    openTabs,
    activeTabId,
    selectedFile,
    isSplitEditor,
    setIsSplitEditor,
    createFile,
    createFolder,
    deleteFile,
    renameFile,
    moveFile,
    copyFile,
    openFolder,
    refreshFiles,
    revalidateFileMutations,
    isFolderModalOpen,
    setIsFolderModalOpen,
    isSSHModalOpen,
    setIsSSHModalOpen,
    isRemoteUploadModalOpen,
    setIsRemoteUploadModalOpen,
    remoteUploadTargetDir,
    setRemoteUploadTargetDir,
    openRemoteUploader,
    activeActivity,
    setActiveActivity,
    isRightActionDrawerOpen,
    setIsRightActionDrawerOpen,
    isBottomTerminalOpen,
    setIsBottomTerminalOpen,
    leftSidebarView,
    setLeftSidebarView,
    sideFileTabs,
    activeSideFile,
    activeSideFileLine,
    openSideFile,
    closeSideFile,
    openSideBrowser,
    isLeftSidebarOpen,
    setIsLeftSidebarOpen,
    agentLeftSidebarOpen,
    setAgentLeftSidebarOpen,
    agentRightSidebarOpen,
    setAgentRightSidebarOpen,
    editorLeftSidebarOpen,
    setEditorLeftSidebarOpen,
    editorRightSidebarOpen,
    setEditorRightSidebarOpen,
    agentViewMode,
    setAgentViewMode,
    boardTasks,
    refreshBoardTasks,
    addBoardTask,
    updateBoardTask,
    deleteBoardTask,
    moveBoardTaskStatus,
    jiraTasks,
    refreshJiraTasks,
    addJiraTask,
    updateJiraTask,
    deleteJiraTask,
    moveJiraTaskStatus,
    isGitClientOpen,
    setIsGitClientOpen,
    leftSidebarWidth,
    setLeftSidebarWidth,
    rightPaneWidth,
    setRightPaneWidth,
    agentExecutionMode,
    setAgentExecutionMode,
    reasoningLevel,
    setReasoningLevel,
    sendSideConversationPrompt,
    clearSideConversation,
    agentEngine,
    setAgentEngine,
    contextUsage,
    currentModel,
    setCurrentModel,
    isCommandPaletteOpen,
    setIsCommandPaletteOpen,
    isAutomationsOpen,
    setIsAutomationsOpen,
    commandPaletteMode,
    openCommandPalette,
    recentFiles,
    isCustomizationsModalOpen,
    setIsCustomizationsModalOpen,
    isSettingsModalOpen,
    setIsSettingsModalOpen,
    settingsActiveSection,
    setSettingsActiveSection,
    previousMode,
    setPreviousMode,
    goBackToWorkspace,
    openFileInEditor,
    updateFolderChildren,
    openSettingsTab,
    openTerminalTab,
    closeTab,
    closeTabs,
    toggleTabPin,
    toggleTabReadOnly,
    setActiveTabId,
    openTab,
    goBack,
    goForward,
    canGoBack,
    canGoForward,
    updateFileContent,
    setTabBuffer,
    saveTabToDisk,
    setDiagnostics,
    createNewSession,
    sendAgentPrompt,
    steerAgentPrompt,
    stopAgentExecution,
    compactCurrentSession,
    deleteSession,
    savedSessions,
    openSessionFromHistory,
    deleteSessionPermanently,
    reloadSavedSessions,
    archivedSessionIds,
    toggleArchiveSession,
    activeWorkspacePath,
    agentWorkspacePath,
    editorWorkspacePath,
    setActiveWorkspacePath,
    activeWorkspaces,
    switchWorkspace,
    switchSurface,
    closeWorkspaceFromWindow,
    openWorkspaceInNewWindow,
    diagnostics,
    subagents,
    activeSubagentId,
    setActiveSubagentId,
    openSubagentInSidebar,
    toggleTask,
    clearTasks,
    sendSubagentMessage
  }), [
    mode,
    setMode,
    activeTaskKind,
    setActiveTaskKind,
    theme,
    setTheme,
    toggleTheme,
    themePalette,
    setThemePalette,
    recentWorkspaces,
    addRecentWorkspace,
    removeRecentWorkspace,
    clearRecentWorkspaces,
    closeWorkspace,
    disconnectSSH,
    activeSessionId,
    setActiveSessionId,
    sessions,
    activeSession,
    agents,
    activeAgentId,
    setActiveAgentId,
    activeAgent,
    toggleAgentEnabled,
    updateAgentConfig,
    refreshAgentModels,
    addAgent,
    deleteAgent,
    acpEnabled,
    setAcpEnabled,
    privacySettings,
    setPrivacySettings,
    providers,
    updateProvider,
    addProvider,
    deleteProvider,
    addModelToProvider,
    deleteModelFromProvider,
    toggleModelSelection,
    selectAllModels,
    deselectAllModels,
    fetchProviderModels,
    mcps,
    addMcp,
    toggleMcp,
    deleteMcp,
    skills,
    addSkill,
    toggleSkill,
    deleteSkill,
    createBackendSkill,
    deleteBackendSkill,
    reloadSkills,
    discoveredMcps,
    discoveredSkills,
    isDiscovering,
    runDiscovery,
    importDiscoveredMcp,
    importDiscoveredSkill,
    plugins,
    fetchPlugins,
    createPlugin,
    togglePlugin,
    deletePlugin,
    reloadPlugins,
    memories,
    fetchMemories,
    saveMemory,
    deleteMemory,
    reloadMemories,
    customCommands,
    addCustomCommand,
    updateCustomCommand,
    deleteCustomCommand,
    toggleCustomCommand,
    hooks,
    updateHook,
    toggleHook,
    addHook,
    deleteHook,
    browserSettings,
    updateBrowserSettings,
    computerSettings,
    updateComputerSettings,
    automations,
    automationsLoading,
    fetchAutomations,
    saveAutomation,
    deleteAutomation,
    setAutomationEnabled,
    runAutomation,
    indexStatus,
    fetchIndexStatus,
    reindexWorkspace,
    isReindexing,
    diffs,
    activeDiff,
    setActiveDiff,
    acceptDiff,
    rejectDiff,
    addDiff,
    openDiffInEditor,
    openGitGraphPane,
    gitBranch,
    gitFiles,
    gitCommits,
    refreshGitStatus,
    refreshGitLog,
    files,
    openTabs,
    activeTabId,
    selectedFile,
    isSplitEditor,
    setIsSplitEditor,
    createFile,
    createFolder,
    deleteFile,
    renameFile,
    moveFile,
    copyFile,
    openFolder,
    refreshFiles,
    revalidateFileMutations,
    isFolderModalOpen,
    setIsFolderModalOpen,
    isSSHModalOpen,
    setIsSSHModalOpen,
    isRemoteUploadModalOpen,
    setIsRemoteUploadModalOpen,
    remoteUploadTargetDir,
    setRemoteUploadTargetDir,
    openRemoteUploader,
    activeActivity,
    setActiveActivity,
    isRightActionDrawerOpen,
    setIsRightActionDrawerOpen,
    isBottomTerminalOpen,
    setIsBottomTerminalOpen,
    leftSidebarView,
    setLeftSidebarView,
    sideFileTabs,
    activeSideFile,
    activeSideFileLine,
    openSideFile,
    closeSideFile,
    openSideBrowser,
    isLeftSidebarOpen,
    setIsLeftSidebarOpen,
    agentLeftSidebarOpen,
    setAgentLeftSidebarOpen,
    agentRightSidebarOpen,
    setAgentRightSidebarOpen,
    editorLeftSidebarOpen,
    setEditorLeftSidebarOpen,
    editorRightSidebarOpen,
    setEditorRightSidebarOpen,
    agentViewMode,
    setAgentViewMode,
    boardTasks,
    refreshBoardTasks,
    addBoardTask,
    updateBoardTask,
    deleteBoardTask,
    moveBoardTaskStatus,
    jiraTasks,
    refreshJiraTasks,
    addJiraTask,
    updateJiraTask,
    deleteJiraTask,
    moveJiraTaskStatus,
    isGitClientOpen,
    setIsGitClientOpen,
    leftSidebarWidth,
    setLeftSidebarWidth,
    rightPaneWidth,
    setRightPaneWidth,
    agentExecutionMode,
    setAgentExecutionMode,
    reasoningLevel,
    setReasoningLevel,
    sendSideConversationPrompt,
    clearSideConversation,
    agentEngine,
    setAgentEngine,
    contextUsage,
    currentModel,
    setCurrentModel,
    isCommandPaletteOpen,
    setIsCommandPaletteOpen,
    isAutomationsOpen,
    setIsAutomationsOpen,
    commandPaletteMode,
    openCommandPalette,
    recentFiles,
    isCustomizationsModalOpen,
    setIsCustomizationsModalOpen,
    isSettingsModalOpen,
    setIsSettingsModalOpen,
    settingsActiveSection,
    setSettingsActiveSection,
    previousMode,
    setPreviousMode,
    goBackToWorkspace,
    openFileInEditor,
    updateFolderChildren,
    openSettingsTab,
    openTerminalTab,
    closeTab,
    closeTabs,
    toggleTabPin,
    toggleTabReadOnly,
    setActiveTabId,
    openTab,
    goBack,
    goForward,
    canGoBack,
    canGoForward,
    updateFileContent,
    setTabBuffer,
    saveTabToDisk,
    setDiagnostics,
    createNewSession,
    sendAgentPrompt,
    steerAgentPrompt,
    stopAgentExecution,
    compactCurrentSession,
    deleteSession,
    savedSessions,
    openSessionFromHistory,
    deleteSessionPermanently,
    reloadSavedSessions,
    archivedSessionIds,
    toggleArchiveSession,
    activeWorkspacePath,
    agentWorkspacePath,
    editorWorkspacePath,
    setActiveWorkspacePath,
    activeWorkspaces,
    switchWorkspace,
    switchSurface,
    closeWorkspaceFromWindow,
    openWorkspaceInNewWindow,
    diagnostics,
    subagents,
    activeSubagentId,
    setActiveSubagentId,
    openSubagentInSidebar,
    toggleTask,
    clearTasks,
    sendSubagentMessage
  ]);

  return (
    <WorkspaceContext.Provider value={contextValue}>
      {children}
    </WorkspaceContext.Provider>
  );
};

export const useWorkspace = () => {
  const context = useContext(WorkspaceContext);
  if (!context) {
    throw new Error('useWorkspace must be used within a WorkspaceProvider');
  }
  return context;
};
