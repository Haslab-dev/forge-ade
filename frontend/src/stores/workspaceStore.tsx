import React from 'react';
import { create } from 'zustand';
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
  WorkspaceEntry,
  WindowWorkspaceState,
  TaskKind,
  TaskStatus,
  BoardTask,
  BoardTaskStatus,
} from '../types';
import { BuiltInThemeId } from '../lib/themePalettes';
import { applyTheme, getStoredThemePalette, saveThemePreference } from '../lib/themeEngine';
import { DEFAULT_AGENTS, DEFAULT_PRIVACY } from './agentRegistryStore';
import { AgentEngine } from '../services/agentEngine';
import { ApiBridge } from '../services/apiBridge';
import {
  StopAgentTurn,
  SetAgentAutoApprove,
  SetActiveReasoningEffort,
  CompactAgentSession,
  CreateShell,
  StopSession,
  OpenNewWindow,
  ToggleAgentTask,
  ClearAgentTasks,
  SendAgentMessage,
  RemoveRecent,
  CloseWorkspace,
  ACPStartSession,
  ACPPrompt,
  ACPCancel,
  type Automation as ZAutomation,
  type AutomationSaveInput as ZAutomationSaveInput
} from '../lib/wails';
import { showToast } from '../lib/toast';
import { formatDisplayTitle, parseFilePath } from '../lib/utils';
import { goAgentSessions, extractMentionedPaths } from '../services/goAgentSession';
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
import { useWorkspaceLifecycle } from '../features/workspace/workspaceLifecycle';

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

export interface WorkspaceContextType {
  mode: WorkspaceMode;
  setMode: (mode: WorkspaceMode | ((prev: WorkspaceMode) => WorkspaceMode)) => void;
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
  setCurrentModel: (model: string, providerId?: string) => void;
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
  openFileInEditor: (filePath: string, line?: number, column?: number, opts?: { silent?: boolean }) => Promise<void>;
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
  /** Editor buffer update: marks the tab dirty WITHOUT touching the disk. */
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

export interface WorkspaceStoreState extends WorkspaceContextType {
  nav: { stack: EditorTab[]; index: number };
  subagentSessions: Record<string, AgentSession>;
  setFiles: (files: FileItem[] | ((prev: FileItem[]) => FileItem[])) => void;
  setOpenTabs: (tabs: EditorTab[] | ((prev: EditorTab[]) => EditorTab[])) => void;
  setSessions: (sessions: AgentSession[] | ((prev: AgentSession[]) => AgentSession[])) => void;
  recordRecentFile: (path: string) => void;
  cycleTab: (dir: 1 | -1) => void;
  setSubagentSession: (id: string, sess: AgentSession) => void;
}

// ── Global Side File Opener ──────────────────────────────────────────────────
let globalOpenSideFileFn: ((rawPath: string, line?: number) => void) | null = null;

export function globalOpenSideFile(rawPath: string, line?: number) {
  if (globalOpenSideFileFn) {
    globalOpenSideFileFn(rawPath, line);
  }
}

// ── Module-level helpers and caches ──────────────────────────────────────────
const workspaceStateCache = new Map<string, WindowWorkspaceState>();
const engine = new AgentEngine();
const sessionPersistMap = new Map<string, string>();
let sessionPersistTimer: number | null = null;
const explicitModelConfig = { agentId: '', model: '' };

function scheduleSessionPersist(sessions: AgentSession[], activeWorkspacePath: string) {
  if (sessionPersistTimer !== null) {
    window.clearTimeout(sessionPersistTimer);
  }
  sessionPersistTimer = window.setTimeout(() => {
    sessionPersistTimer = null;
    try {
      const trimmed = sessions.slice(0, 40).map(s => ({
        ...s,
        messages: s.messages.length > 50 ? s.messages.slice(-50) : s.messages,
        diffs: [],
        sideConversationMessages: []
      }));
      localStorage.setItem('forge_ade_sessions', JSON.stringify(trimmed));
    } catch {}
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
      if (sessionPersistMap.get(sess.id) === fingerprint) continue;
      sessionPersistMap.set(sess.id, fingerprint);
      void ApiBridge.saveSessionJsonl(sess, sess.workspacePath ?? activeWorkspacePath);
    }
  }, 800);
}

function parseWorkspaceEntry(wsPath: string): WorkspaceEntry {
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
}

function formatSmartSessionTitle(prompt: string): string {
  if (!prompt || !prompt.trim()) return 'New Session';
  let clean = prompt
    .replace(/\[Attached File:[\s\S]*?\]/g, '')
    .replace(/\[TOOL EXECUTION RESULTS\][\s\S]*$/g, '')
    .replace(/^["'`]|["'`]$/g, '')
    .trim();

  clean = formatDisplayTitle(clean);
  clean = clean
    .replace(/^(please|can you|could you|help me|i want to|how to|how do i|tell me about|explain to me|what is|create a|build a|write a|give me)\s+/i, '')
    .replace(/[`*_#~]/g, '')
    .trim();

  if (!clean) return 'New Session';
  clean = clean.charAt(0).toUpperCase() + clean.slice(1);
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
}

// Initial state loaders
function getInitialTheme(): ThemeMode {
  if (typeof window === 'undefined') return 'dark';
  const stored = localStorage.getItem('forge-ade-theme');
  return stored === 'light' || stored === 'system' ? stored : 'dark';
}

function getInitialTaskKind(): TaskKind {
  return typeof window !== 'undefined' && localStorage.getItem('forge_ade_task_kind') === 'cli' ? 'cli' : 'forge';
}

function getCleanStoredPath(key: string): string {
  try {
    return localStorage.getItem(key) || '';
  } catch {
    return '';
  }
}

function getInitialAgentWs(): string {
  return getCleanStoredPath('forge_ade_agent_workspace')
    || getCleanStoredPath('forge_ade_workspace_path')
    || getCleanStoredPath('my_ade_workspace_path')
    || '';
}

function getInitialEditorWs(): string {
  return getCleanStoredPath('forge_ade_editor_workspace')
    || getCleanStoredPath('forge_ade_workspace_path')
    || getCleanStoredPath('my_ade_workspace_path')
    || '';
}

function getInitialRecentWorkspaces(): string[] {
  try {
    const saved = localStorage.getItem('forge_ade_recent_workspaces') || localStorage.getItem('my_ade_recent_workspaces');
    if (saved) {
      const parsed = JSON.parse(saved);
      if (Array.isArray(parsed)) return parsed.filter(p => typeof p === 'string' && p.trim());
    }
  } catch {}
  return [];
}

function getInitialRecentFiles(): Array<{ path: string; ts: number }> {
  try {
    const saved = JSON.parse(localStorage.getItem('forge_ade_recent_files') || '[]');
    if (Array.isArray(saved)) return saved.filter((r: any) => r && typeof r.path === 'string');
  } catch {}
  return [];
}

function getInitialSessions(): AgentSession[] {
  try {
    const saved = localStorage.getItem('forge_ade_sessions') || localStorage.getItem('my_ade_sessions');
    if (saved) {
      const parsed = JSON.parse(saved);
      if (Array.isArray(parsed) && parsed.length > 0) {
        return parsed
          .filter((s: any) => s && !isGoHarnessSession(s))
          .map(normalizeLoadedSession)
          .filter((s): s is AgentSession => s !== null);
      }
    }
  } catch {}
  return [];
}

function getInitialAgents(): ACPAgent[] {
  try {
    const saved = localStorage.getItem('forge_ade_agents') || localStorage.getItem('my_ade_agents');
    if (saved) {
      const parsed: ACPAgent[] = JSON.parse(saved);
      if (Array.isArray(parsed) && parsed.length > 0) {
        const hasInternal = parsed.some(a => a.id === 'agent-internal');
        const list = hasInternal ? [...parsed] : [DEFAULT_AGENTS[0], ...parsed];
        const existingIds = new Set(list.map(a => a.id));
        for (const def of DEFAULT_AGENTS) {
          if (!existingIds.has(def.id)) list.push(def);
        }
        return list.map(a => {
          const def = DEFAULT_AGENTS.find(d => d.id === a.id);
          const cleanSupported = (a.supportedModels || []).filter(m => !m.includes('myairouter_cloud'));
          return {
            ...a,
            name: def ? def.name : a.name,
            provider: a.type === 'internal' ? 'custom' : (def ? def.provider : a.provider),
            command: def ? def.command : a.command,
            args: def ? def.args : a.args,
            type: def ? def.type : a.type,
            model: (a.model || '').includes('myairouter_cloud') ? (def ? def.model : cleanSupported[0] || a.model) : (a.model || (def ? def.model : '')),
            supportedModels: cleanSupported.length > 0 ? cleanSupported : (def ? def.supportedModels : []),
            supportedEfforts: def ? def.supportedEfforts : (a.supportedEfforts || undefined),
            defaultEffort: def ? def.defaultEffort : a.defaultEffort
          };
        });
      }
    }
  } catch {}
  return DEFAULT_AGENTS;
}

function getInitialActiveAgentId(): string {
  try {
    return localStorage.getItem('forge_ade_active_agent_id') || localStorage.getItem('my_ade_active_agent_id') || 'agent-agy';
  } catch {
    return 'agent-agy';
  }
}

function getInitialCurrentModel(): string {
  try {
    return localStorage.getItem('forge_ade_current_model') || localStorage.getItem('my_ade_current_model') || '';
  } catch {
    return '';
  }
}

function getInitialReasoningLevel(): AgentReasoningLevel {
  try {
    const saved = localStorage.getItem('forge_ade_reasoning_level') as AgentReasoningLevel;
    if (saved && ['off', 'minimal', 'low', 'medium', 'high', 'xhigh', 'max'].includes(saved)) {
      return saved;
    }
  } catch {}
  return 'medium';
}

function getInitialArchivedSessionIds(): Set<string> {
  try {
    const s = localStorage.getItem('forge_archived_sessions');
    return s ? new Set(JSON.parse(s)) : new Set();
  } catch {
    return new Set();
  }
}

function computeDerived(state: {
  mode: WorkspaceMode;
  agentWorkspacePath: string;
  editorWorkspacePath: string;
  activeSessionId: string | null;
  sessions: AgentSession[];
  savedSessions: AgentSession[];
  agents: ACPAgent[];
  activeAgentId: string;
  currentModel: string;
  agentLeftSidebarOpen: boolean;
  editorLeftSidebarOpen: boolean;
  nav: { stack: EditorTab[]; index: number };
  subagentSessions: Record<string, AgentSession>;
}) {
  const activeWorkspacePath = state.mode === 'editor' ? state.editorWorkspacePath : state.agentWorkspacePath;
  const activeSession = state.activeSessionId
    ? (state.sessions.find(s => s.id === state.activeSessionId) || state.savedSessions.find(s => s.id === state.activeSessionId))
    : undefined;
  const activeAgent = state.agents.find(a => a.id === state.activeAgentId) || state.agents[0];
  const contextUsage = calculateContextUsage(activeSession, state.currentModel);
  const isLeftSidebarOpen = state.mode === 'editor' ? state.editorLeftSidebarOpen : state.agentLeftSidebarOpen;
  const canGoBack = state.nav.index > 0;
  const canGoForward = state.nav.index >= 0 && state.nav.index < state.nav.stack.length - 1;
  const subagentIds = activeSession?.subagentIds || [];
  const subagents = subagentIds.map(id => state.subagentSessions[id]).filter(Boolean) as AgentSession[];

  return {
    activeWorkspacePath,
    activeSession,
    activeAgent,
    contextUsage,
    isLeftSidebarOpen,
    canGoBack,
    canGoForward,
    subagents
  };
}

// ── Full Zustand Store Definition ────────────────────────────────────────────
export const useWorkspace = create<WorkspaceStoreState>((set, get) => {
  // Helper to update store while keeping derived properties strictly synchronized
  const updateStore = (
    updater: Partial<WorkspaceStoreState> | ((prev: WorkspaceStoreState) => Partial<WorkspaceStoreState>)
  ) => {
    set(state => {
      const partial = typeof updater === 'function' ? updater(state) : updater;
      const combined = { ...state, ...partial };
      const derived = computeDerived(combined);
      return {
        ...partial,
        ...derived
      };
    });
  };

  const initialMode: WorkspaceMode = 'agent';
  const initialAgentWs = getInitialAgentWs();
  const initialEditorWs = getInitialEditorWs();
  const initialRecentWs = getInitialRecentWorkspaces();
  const initialSessions = getInitialSessions();
  const initialActiveSessionId = initialSessions[0]?.id || null;
  const initialAgents = getInitialAgents();
  const initialActiveAgentId = getInitialActiveAgentId();
  const initialCurrentModel = getInitialCurrentModel();
  const initialReasoningLevel = getInitialReasoningLevel();
  const initialNav = { stack: [] as EditorTab[], index: -1 };
  const initialSubagentSessions: Record<string, AgentSession> = {};

  const initialDerived = computeDerived({
    mode: initialMode,
    agentWorkspacePath: initialAgentWs,
    editorWorkspacePath: initialEditorWs,
    activeSessionId: initialActiveSessionId,
    sessions: initialSessions,
    savedSessions: [],
    agents: initialAgents,
    activeAgentId: initialActiveAgentId,
    currentModel: initialCurrentModel,
    agentLeftSidebarOpen: useLayoutStore.getState().agentLeftSidebarOpen,
    editorLeftSidebarOpen: useLayoutStore.getState().editorLeftSidebarOpen,
    nav: initialNav,
    subagentSessions: initialSubagentSessions
  });

  return {
    // 1. Core Surfaces & Modes
    mode: initialMode,
    previousMode: 'agent',
    activeTaskKind: getInitialTaskKind(),
    theme: getInitialTheme(),
    themePalette: getStoredThemePalette(),
    agentWorkspacePath: initialAgentWs,
    editorWorkspacePath: initialEditorWs,
    activeWorkspaces: (initialAgentWs || initialEditorWs) ? [parseWorkspaceEntry(initialAgentWs || initialEditorWs)] : [],
    recentWorkspaces: initialRecentWs,
    recentFiles: getInitialRecentFiles(),

    // 2. Editor & Filesystem
    files: [],
    openTabs: [],
    activeTabId: null,
    selectedFile: null,
    isSplitEditor: false,
    setIsSplitEditor: (val) => updateStore(s => ({ isSplitEditor: typeof val === 'function' ? val(s.isSplitEditor) : val })),
    nav: initialNav,
    sideFileTabs: [],
    activeSideFile: null,
    activeSideFileLine: undefined,

    // 3. Agent Sessions & Execution
    sessions: initialSessions,
    savedSessions: [],
    activeSessionId: initialActiveSessionId,
    agents: initialAgents,
    activeAgentId: initialActiveAgentId,
    acpEnabled: true,
    privacySettings: DEFAULT_PRIVACY,
    currentModel: initialCurrentModel,
    agentExecutionMode: 'bypass',
    reasoningLevel: initialReasoningLevel,
    agentEngine: 'pi',
    diagnostics: [],
    subagentSessions: initialSubagentSessions,
    activeSubagentId: null,
    archivedSessionIds: getInitialArchivedSessionIds(),
    indexStatus: null,
    isReindexing: false,

    // 4. Delegated State Initializers
    gitBranch: useGitStore.getState().gitBranch,
    gitFiles: useGitStore.getState().gitFiles,
    gitCommits: useGitStore.getState().gitCommits,
    diffs: useGitStore.getState().diffs,
    activeDiff: useGitStore.getState().activeDiff,
    isGitClientOpen: useGitStore.getState().isGitClientOpen,
    isBottomTerminalOpen: useTerminalStore.getState().isBottomTerminalOpen,
    isFolderModalOpen: useLayoutStore.getState().isFolderModalOpen,
    isSSHModalOpen: useLayoutStore.getState().isSSHModalOpen,
    isRemoteUploadModalOpen: useLayoutStore.getState().isRemoteUploadModalOpen,
    remoteUploadTargetDir: useLayoutStore.getState().remoteUploadTargetDir,
    activeActivity: useLayoutStore.getState().activeActivity,
    agentLeftSidebarOpen: useLayoutStore.getState().agentLeftSidebarOpen,
    rightActionDrawerOpen: useLayoutStore.getState().rightActionDrawerOpen,
    editorLeftSidebarOpen: useLayoutStore.getState().editorLeftSidebarOpen,
    agentRightSidebarOpen: useLayoutStore.getState().rightActionDrawerOpen,
    editorRightSidebarOpen: useLayoutStore.getState().rightActionDrawerOpen,
    isRightActionDrawerOpen: useLayoutStore.getState().rightActionDrawerOpen,
    agentViewMode: useLayoutStore.getState().agentViewMode,
    leftSidebarView: useLayoutStore.getState().leftSidebarView,
    isCommandPaletteOpen: useLayoutStore.getState().isCommandPaletteOpen,
    isAutomationsOpen: useLayoutStore.getState().isAutomationsOpen,
    commandPaletteMode: useLayoutStore.getState().commandPaletteMode,
    isCustomizationsModalOpen: useLayoutStore.getState().isCustomizationsModalOpen,
    isSettingsModalOpen: useLayoutStore.getState().isSettingsModalOpen,
    settingsActiveSection: useLayoutStore.getState().settingsActiveSection,
    leftSidebarWidth: useLayoutStore.getState().leftSidebarWidth,
    rightPaneWidth: useLayoutStore.getState().rightPaneWidth,
    boardTasks: useBoardStore.getState().boardTasks,
    jiraTasks: useBoardStore.getState().boardTasks,
    providers: useProviderStore.getState().providers,
    mcps: useIntegrationsStore.getState().mcps,
    skills: useIntegrationsStore.getState().skills,
    discoveredMcps: useIntegrationsStore.getState().discoveredMcps,
    discoveredSkills: useIntegrationsStore.getState().discoveredSkills,
    isDiscovering: useIntegrationsStore.getState().isDiscovering,
    plugins: useIntegrationsStore.getState().plugins,
    memories: useIntegrationsStore.getState().memories,
    customCommands: useIntegrationsStore.getState().customCommands,
    hooks: useIntegrationsStore.getState().hooks,
    browserSettings: useIntegrationsStore.getState().browserSettings,
    computerSettings: useIntegrationsStore.getState().computerSettings,
    automations: useIntegrationsStore.getState().automations,
    automationsLoading: useIntegrationsStore.getState().automationsLoading,

    // 5. Initial Derived Values
    ...initialDerived,

    // ── Internal Setters & Updaters ──────────────────────────────────────────
    setFiles: (files) => updateStore(s => ({ files: typeof files === 'function' ? files(s.files) : files })),
    setOpenTabs: (openTabs) => updateStore(s => ({ openTabs: typeof openTabs === 'function' ? openTabs(s.openTabs) : openTabs })),
    setSessions: (sessions) => {
      updateStore(s => {
        const next = typeof sessions === 'function' ? sessions(s.sessions) : sessions;
        scheduleSessionPersist(next, s.activeWorkspacePath);
        return { sessions: next };
      });
    },
    setSubagentSession: (id, sess) => updateStore(s => ({ subagentSessions: { ...s.subagentSessions, [id]: sess } })),

    // ── Surface & Workspace Actions ──────────────────────────────────────────
    setMode: (newMode) => {
      const current = get().mode;
      const resolved = typeof newMode === 'function' ? newMode(current) : newMode;
      if (current === resolved) return;

      if (current === 'editor') {
        const currentPath = get().editorWorkspacePath;
        if (currentPath) {
          workspaceStateCache.set(currentPath, {
            openTabs: [...get().openTabs],
            activeTabId: get().activeTabId,
            selectedFile: get().selectedFile,
            diffs: [...get().diffs],
            activeDiff: get().activeDiff
          });
        }
      } else if (resolved === 'editor') {
        const currentPath = get().editorWorkspacePath;
        if (currentPath) {
          const cached = workspaceStateCache.get(currentPath);
          if (cached) {
            updateStore({
              openTabs: cached.openTabs || [],
              activeTabId: cached.activeTabId || null,
              selectedFile: cached.selectedFile || null
            });
            useGitStore.getState().setDiffs(cached.diffs || []);
            useGitStore.getState().setActiveDiff(cached.activeDiff || null);
          }
        }
      }

      const updates: Partial<WorkspaceStoreState> = { mode: resolved };
      if (current !== 'settings' && resolved === 'settings') {
        updates.previousMode = current;
      }
      updateStore(updates);
    },

    setActiveTaskKind: (kind) => {
      updateStore({ activeTaskKind: kind });
      try { localStorage.setItem('forge_ade_task_kind', kind); } catch {}
    },

    setTheme: (theme) => updateStore({ theme }),
    toggleTheme: (next) => updateStore(s => ({ theme: next ?? (s.theme === 'dark' ? 'light' : 'dark') })),
    setThemePalette: (p) => {
      updateStore({ themePalette: p });
      saveThemePreference(p, get().theme);
    },

    addRecentWorkspace: (path) => {
      if (!path || !path.trim()) return;
      updateStore(prev => {
        const filtered = prev.recentWorkspaces.filter(p => p !== path);
        const next = [path, ...filtered].slice(0, 15);
        try { localStorage.setItem('forge_ade_recent_workspaces', JSON.stringify(next)); } catch {}
        return { recentWorkspaces: next };
      });
    },

    removeRecentWorkspace: (pathToRemove) => {
      void RemoveRecent(pathToRemove).catch(() => {});
      updateStore(prev => {
        const next = prev.recentWorkspaces.filter(p => p !== pathToRemove);
        try { localStorage.setItem('forge_ade_recent_workspaces', JSON.stringify(next)); } catch {}
        return { recentWorkspaces: next };
      });
    },

    clearRecentWorkspaces: () => {
      updateStore({ recentWorkspaces: [] });
      try { localStorage.setItem('forge_ade_recent_workspaces', JSON.stringify([])); } catch {}
    },

    closeWorkspace: () => {
      const mode = get().mode;
      if (mode === 'editor') {
        const closed = get().editorWorkspacePath;
        if (closed) workspaceStateCache.delete(closed);
        localStorage.removeItem('forge_ade_editor_workspace');
        updateStore({
          editorWorkspacePath: '',
          openTabs: [],
          activeTabId: null,
          selectedFile: null
        });
        useGitStore.getState().setDiffs([]);
        useGitStore.getState().setActiveDiff(null);
      } else {
        const closed = get().agentWorkspacePath;
        if (closed) workspaceStateCache.delete(closed);
        localStorage.removeItem('forge_ade_agent_workspace');
        updateStore({ agentWorkspacePath: '' });
      }
      localStorage.removeItem('forge_ade_workspace_path');
      localStorage.removeItem('my_ade_workspace_path');
      void CloseWorkspace().catch(() => {});
      updateStore({ files: [] });
      useGitStore.getState().setGitBranch('');
      useGitStore.getState().setGitFiles([]);
      useGitStore.getState().setGitCommits([]);
    },

    disconnectSSH: async () => {
      const activeWs = get().activeWorkspacePath;
      if (activeWs && activeWs.startsWith('ssh://')) {
        const trimmed = activeWs.replace(/^ssh:\/\//, '');
        const connId = trimmed.split('/')[0];
        if (connId) await ApiBridge.disconnectSSH(connId);
      }
      get().closeWorkspace();
    },

    setActiveWorkspacePath: (newPath) => {
      const surface = get().mode === 'editor' ? 'editor' : 'agent';
      const current = surface === 'editor' ? get().editorWorkspacePath : get().agentWorkspacePath;
      if (!newPath || newPath === current) return;

      const oldPath = get().activeWorkspacePath;
      if (oldPath && (surface === 'editor' || get().mode === 'editor')) {
        workspaceStateCache.set(oldPath, {
          openTabs: [...get().openTabs],
          activeTabId: get().activeTabId,
          selectedFile: get().selectedFile,
          diffs: [...get().diffs],
          activeDiff: get().activeDiff
        });
        if (workspaceStateCache.size > 8) {
          const oldest = workspaceStateCache.keys().next().value;
          if (oldest !== undefined && oldest !== newPath) {
            workspaceStateCache.delete(oldest);
          }
        }
      }

      updateStore(prev => {
        const existing = prev.activeWorkspaces.find(w => w.path === newPath);
        const entry = existing ? { ...existing, lastActive: Date.now() } : parseWorkspaceEntry(newPath);
        const remaining = prev.activeWorkspaces.filter(w => w.path !== newPath);
        return { activeWorkspaces: [entry, ...remaining] };
      });

      if (surface === 'editor') {
        const cached = workspaceStateCache.get(newPath);
        if (cached) {
          updateStore({
            openTabs: cached.openTabs || [],
            activeTabId: cached.activeTabId || null,
            selectedFile: cached.selectedFile || null
          });
          useGitStore.getState().setDiffs(cached.diffs || []);
          useGitStore.getState().setActiveDiff(cached.activeDiff || null);
        } else {
          updateStore({
            openTabs: [],
            activeTabId: null,
            selectedFile: null
          });
          useGitStore.getState().setDiffs([]);
          useGitStore.getState().setActiveDiff(null);
        }
      }

      if (surface === 'editor') {
        updateStore({ editorWorkspacePath: newPath });
        localStorage.setItem('forge_ade_editor_workspace', newPath);
      } else {
        updateStore({ agentWorkspacePath: newPath });
        localStorage.setItem('forge_ade_agent_workspace', newPath);
      }
      localStorage.setItem('forge_ade_workspace_path', newPath);
      get().addRecentWorkspace(newPath);

      // Trigger directory read and git refresh
      void (async () => {
        try { await ApiBridge.openFolder(newPath); } catch {}
        if (get().activeWorkspacePath !== newPath) return;
        const realTree = await ApiBridge.readDirectoryTree(newPath);
        if (get().activeWorkspacePath !== newPath) return;
        updateStore({ files: realTree });
        void get().refreshGitStatus();
        void get().refreshGitLog();
      })();
    },

    switchWorkspace: async (path) => {
      get().setActiveWorkspacePath(path);
    },

    switchSurface: (target) => {
      get().setMode(target);
    },

    closeWorkspaceFromWindow: async (pathToRemove) => {
      workspaceStateCache.delete(pathToRemove);
      if (pathToRemove.startsWith('ssh://')) {
        const trimmed = pathToRemove.replace(/^ssh:\/\//, '');
        const connId = trimmed.split('/')[0];
        if (connId) void ApiBridge.disconnectSSH(connId);
      }
      updateStore(prev => ({ activeWorkspaces: prev.activeWorkspaces.filter(w => w.path !== pathToRemove) }));
      if (pathToRemove === get().activeWorkspacePath) {
        const remaining = get().activeWorkspaces.filter(w => w.path !== pathToRemove);
        if (remaining.length > 0) {
          get().setActiveWorkspacePath(remaining[0].path);
        } else {
          get().closeWorkspace();
        }
      }
    },

    openWorkspaceInNewWindow: async (targetPath) => {
      try {
        await OpenNewWindow(targetPath);
      } catch (e) {
        console.warn('Failed to open new window for workspace:', e);
        showToast('Failed to open new window', 'error');
      }
    },

    recordRecentFile: (path) => {
      if (!path) return;
      updateStore(prev => {
        const next = [{ path, ts: Date.now() }, ...prev.recentFiles.filter(r => r.path !== path)].slice(0, 40);
        try { localStorage.setItem('forge_ade_recent_files', JSON.stringify(next)); } catch {}
        return { recentFiles: next };
      });
    },

    // ── Editor File Tree & Tab Actions ───────────────────────────────────────
    refreshFiles: async () => {
      const current = get().activeWorkspacePath || (await ApiBridge.getWorkspaceInfo()).cwd;
      if (current) {
        const realTree = await ApiBridge.readDirectoryTree(current);
        updateStore({ files: realTree });
      }
    },

    revalidateFileMutations: async () => {
      try {
        await get().refreshFiles();
        void get().refreshGitStatus();
        void get().refreshGitLog();
      } catch (e) {
        console.error('File mutation revalidation failed:', e);
      }
    },

    openFolder: async (customPath) => {
      let targetPath = customPath;
      if (!targetPath) {
        try {
          const picked = await ApiBridge.pickNativeDirectory();
          if (picked?.path) targetPath = picked.path;
        } catch (err) {
          console.warn('Native folder pick failed:', err);
        }
      }
      if (targetPath) {
        if (targetPath === get().activeWorkspacePath) {
          await get().refreshFiles();
          void get().refreshGitStatus();
          void get().refreshGitLog();
        } else {
          get().setActiveWorkspacePath(targetPath);
        }
      }
    },

    openSideFile: async (rawPath, line) => {
      const parsed = parseFilePath(rawPath, get().activeWorkspacePath);
      const path = parsed.cleanPath || rawPath;
      const targetLine = line !== undefined ? line : parsed.line;
      const name = parsed.displayName;
      const isImage = isImageFile(path);

      if (isZipFile(path)) {
        showToast(`"${name}" is an archive file and cannot be opened in side preview`, 'error');
        return;
      }
      if (!isImage && isBinaryFile(path)) {
        showToast(`"${name}" is a binary file and cannot be opened in side preview`, 'error');
        return;
      }
      if (!isImage) {
        try {
          const size = await ApiBridge.fileSize(path);
          if (size > MAX_SIDE_PREVIEW_TEXT_BYTES) {
            const mb = (size / (1024 * 1024)).toFixed(1);
            showToast(`"${name}" is ${mb}MB — exceeds 2MB limit for text preview`, 'error');
            return;
          }
        } catch {}
      }

      updateStore(prev => {
        const existing = prev.sideFileTabs.find(t => t.path === path);
        const tabs = existing
          ? prev.sideFileTabs.map(t => (t.path === path ? { ...t, line: targetLine } : t))
          : [...prev.sideFileTabs, { path, name, line: targetLine }];
        return {
          sideFileTabs: tabs,
          activeSideFile: path,
          activeSideFileLine: targetLine
        };
      });
      get().setIsRightActionDrawerOpen(true);
    },

    closeSideFile: (path) => {
      updateStore(prev => {
        const rest = prev.sideFileTabs.filter(t => t.path !== path);
        const nextActive = prev.activeSideFile === path ? (rest.length > 0 ? rest[rest.length - 1].path : null) : prev.activeSideFile;
        return {
          sideFileTabs: rest,
          activeSideFile: nextActive
        };
      });
    },

    openSideBrowser: (url) => {
      get().setIsRightActionDrawerOpen(true);
      if (typeof window !== 'undefined') {
        window.dispatchEvent(new CustomEvent('forge:open-sidepane-browser', { detail: { url } }));
      }
    },

    openFileInEditor: async (filePath, line, column, opts) => {
      if (get().mode === 'agent') {
        await get().openSideFile(filePath, line);
        return;
      }

      const file = findFileInTree(get().files, filePath);
      let content = file?.content || '';
      const fileName = filePath.split('/').pop() || filePath;

      if (isBinaryFile(filePath)) {
        if (!opts?.silent) {
          showToast(`"${fileName}" is a binary file and can't be opened in the editor`, 'error');
        }
        return;
      }

      const isDb = isDatabaseFile(filePath);
      if (!content && !isDb) {
        const size = await ApiBridge.fileSize(filePath);
        if (size > MAX_EDITOR_FILE_BYTES) {
          if (!opts?.silent) {
            showToast(`"${fileName}" is ${Math.round(size / (1024 * 1024))}MB — too large for the code editor`, 'error');
          }
          return;
        }
        content = await ApiBridge.readFile(filePath);
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

      const newTab: EditorTab = {
        id: `tab-${fileItem.id}`,
        fileId: fileItem.id,
        fileName: fileItem.name,
        filePath: fileItem.path,
        content,
        type: 'code',
        line,
        column
      };

      updateStore(prev => {
        const existing = prev.openTabs.find(t => t.filePath === filePath && t.type === 'code');
        const updatedTabs = existing
          ? (line !== undefined ? prev.openTabs.map(t => (t.id === existing.id ? { ...t, line, column } : t)) : prev.openTabs)
          : [...prev.openTabs, newTab];
        return {
          selectedFile: fileItem,
          openTabs: updatedTabs
        };
      });

      const existingTab = get().openTabs.find(t => t.filePath === filePath && t.type === 'code');
      get().openTab(existingTab || newTab);
      if (!opts?.silent) get().setMode('editor');
    },

    updateFolderChildren: (folderPath, children) => {
      updateStore(prev => ({ files: updateFolderChildrenInTree(prev.files, folderPath, children) }));
    },

    openSettingsTab: (section) => {
      if (section) get().setSettingsActiveSection(section);
      get().setMode('settings');
    },

    goBackToWorkspace: () => {
      get().setMode(get().previousMode === 'settings' ? 'agent' : get().previousMode);
    },

    openDiffInEditor: (diff) => {
      useGitStore.getState().setActiveDiff(diff);
      useGitStore.getState().setDiffs(prev => {
        const idx = prev.findIndex(d => d.id === diff.id);
        if (idx === -1) return [...prev, diff];
        const next = [...prev];
        next[idx] = diff;
        return next;
      });
      const diffTabId = `tab-diff-${diff.id}`;
      const existing = get().openTabs.find(t => t.id === diffTabId);
      if (existing) {
        get().openTab(existing);
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
        updateStore(prev => ({ openTabs: [...prev.openTabs, newTab] }));
        get().openTab(newTab);
      }
      get().setMode('editor');
    },

    openGitGraphPane: () => {
      const graphTabId = 'tab-git-graph';
      const existing = get().openTabs.find(t => t.id === graphTabId);
      if (existing) {
        get().openTab(existing);
      } else {
        const newTab: EditorTab = {
          id: graphTabId,
          fileId: 'file-git-graph',
          fileName: 'Git Graph',
          filePath: '',
          type: 'git-graph',
          content: ''
        };
        updateStore(prev => ({ openTabs: [...prev.openTabs, newTab] }));
        get().openTab(newTab);
      }
      get().setMode('editor');
    },

    openTerminalTab: async (shellId, cwd) => {
      let sid = shellId;
      if (!sid) {
        try {
          const sess = await CreateShell(`zsh (${get().openTabs.filter(t => t.type === 'terminal').length + 1})`, cwd || get().activeWorkspacePath || '');
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
      updateStore(prev => ({ openTabs: [...prev.openTabs, newTab] }));
      get().openTab(newTab);
      get().setMode('editor');
    },

    closeTab: (tabId) => {
      const targetTab = get().openTabs.find(t => t.id === tabId);
      if (targetTab?.terminalSessionId) {
        void StopSession(targetTab.terminalSessionId).catch(() => {});
      }
      const nextActive = calculateActiveTabOnClose(get().openTabs, tabId, get().activeTabId);
      updateStore(prev => ({
        openTabs: prev.openTabs.filter(t => t.id !== tabId),
        activeTabId: nextActive
      }));
    },

    closeTabs: (ids) => {
      if (ids.length === 0) return;
      const idSet = new Set(ids);
      for (const id of ids) {
        const targetTab = get().openTabs.find(t => t.id === id);
        if (targetTab?.terminalSessionId) {
          void StopSession(targetTab.terminalSessionId).catch(() => {});
        }
      }
      updateStore(prev => ({
        openTabs: filterTabsOnClose(prev.openTabs, ids),
        activeTabId: prev.activeTabId && idSet.has(prev.activeTabId) ? null : prev.activeTabId
      }));
    },

    toggleTabPin: (tabId) => {
      updateStore(prev => ({
        openTabs: prev.openTabs.map(t => (t.id === tabId ? { ...t, isPinned: !t.isPinned } : t))
      }));
    },

    toggleTabReadOnly: (tabId) => {
      updateStore(prev => ({
        openTabs: prev.openTabs.map(t => (t.id === tabId ? { ...t, isReadOnly: !t.isReadOnly } : t))
      }));
    },

    setActiveTabId: (tabId) => updateStore({ activeTabId: tabId }),

    openTab: (tab) => {
      updateStore(prev => ({
        openTabs: prev.openTabs.some(t => t.id === tab.id) ? prev.openTabs : [...prev.openTabs, tab],
        activeTabId: tab.id,
        nav: pushNavHistory(prev.nav, tab)
      }));
    },

    goBack: () => {
      const res = getNavBack(get().nav);
      if (!res) return;
      const target = res.targetTab;
      updateStore(prev => ({
        nav: res.nextNav,
        activeTabId: target.id,
        openTabs: prev.openTabs.some(t => t.id === target.id) ? prev.openTabs : [...prev.openTabs, { ...target }]
      }));
      if (target.type === 'code' && target.filePath) {
        void ApiBridge.readFile(target.filePath).then(content => {
          updateStore(prev => ({
            openTabs: prev.openTabs.map(t => (t.id === target.id && !t.content ? { ...t, content } : t))
          }));
        }).catch(() => {});
      }
    },

    goForward: () => {
      const res = getNavForward(get().nav);
      if (!res) return;
      const target = res.targetTab;
      updateStore(prev => ({
        nav: res.nextNav,
        activeTabId: target.id,
        openTabs: prev.openTabs.some(t => t.id === target.id) ? prev.openTabs : [...prev.openTabs, { ...target }]
      }));
      if (target.type === 'code' && target.filePath) {
        void ApiBridge.readFile(target.filePath).then(content => {
          updateStore(prev => ({
            openTabs: prev.openTabs.map(t => (t.id === target.id && !t.content ? { ...t, content } : t))
          }));
        }).catch(() => {});
      }
    },

    cycleTab: (dir) => {
      const next = cycleActiveTab(get().openTabs, get().activeTabId, dir);
      if (next) get().openTab(next);
    },

    updateFileContent: async (fileId, newContent) => {
      const targetTab = get().openTabs.find(t => t.fileId === fileId || t.filePath === fileId);
      const targetPath = targetTab ? targetTab.filePath : fileId;
      await ApiBridge.writeFile(targetPath, newContent);
      updateStore(prev => ({
        files: updateFileContentInTree(prev.files, targetPath, newContent),
        openTabs: prev.openTabs.map(t =>
          (t.fileId === fileId || t.filePath === fileId || t.filePath === targetPath) ? { ...t, content: newContent } : t
        )
      }));
    },

    setTabBuffer: (fileId, content) => {
      updateStore(prev => ({
        openTabs: updateTabBuffer(prev.openTabs, fileId, content).updatedTabs
      }));
    },

    saveTabToDisk: async (fileId, content) => {
      const targetTab = get().openTabs.find(t => t.fileId === fileId || t.filePath === fileId);
      const targetPath = targetTab ? targetTab.filePath : fileId;
      await ApiBridge.writeFile(targetPath, content);
      updateStore(prev => ({
        openTabs: prev.openTabs.map(t =>
          (t.fileId === fileId || t.filePath === fileId || t.filePath === targetPath) ? { ...t, content, isModified: false } : t
        )
      }));
      void get().refreshGitStatus();
    },

    createFile: async (filePath, content = '') => {
      const fullPath = resolveWorkspacePath(get().activeWorkspacePath, filePath);
      await ApiBridge.createFile(fullPath, content);
      await get().revalidateFileMutations();
      await get().openFileInEditor(fullPath);
    },

    createFolder: async (folderPath) => {
      const fullPath = resolveWorkspacePath(get().activeWorkspacePath, folderPath);
      await ApiBridge.createFile(fullPath, '');
      await get().revalidateFileMutations();
    },

    deleteFile: async (filePath) => {
      await ApiBridge.deleteFile(filePath);
      updateStore(prev => ({ openTabs: updateTabsOnDelete(prev.openTabs, filePath) }));
      get().closeSideFile(filePath);
      await get().revalidateFileMutations();
    },

    renameFile: async (oldPath, newPath) => {
      await ApiBridge.renameFile(oldPath, newPath);
      const newName = newPath.split('/').pop() || newPath;
      updateStore(prev => ({
        openTabs: updateTabsOnRename(prev.openTabs, oldPath, newPath),
        sideFileTabs: prev.sideFileTabs.map(t => {
          if (t.path === oldPath) return { ...t, path: newPath, name: newName };
          if (t.path.startsWith(oldPath + '/')) return { ...t, path: `${newPath}${t.path.slice(oldPath.length)}` };
          return t;
        }),
        activeSideFile: prev.activeSideFile === oldPath ? newPath : prev.activeSideFile
      }));
      await get().revalidateFileMutations();
    },

    moveFile: async (srcPath, destPath) => {
      await ApiBridge.moveFile(srcPath, destPath);
      const newName = destPath.split('/').pop() || destPath;
      updateStore(prev => ({
        openTabs: updateTabsOnRename(prev.openTabs, srcPath, destPath),
        sideFileTabs: prev.sideFileTabs.map(t => {
          if (t.path === srcPath) return { ...t, path: destPath, name: newName };
          if (t.path.startsWith(srcPath + '/')) return { ...t, path: `${destPath}${t.path.slice(srcPath.length)}` };
          return t;
        }),
        activeSideFile: prev.activeSideFile === srcPath ? destPath : prev.activeSideFile
      }));
      await get().revalidateFileMutations();
    },

    copyFile: async (srcPath, destPath) => {
      await ApiBridge.copyPath(srcPath, destPath);
      await get().revalidateFileMutations();
    },

    // ── Agent Sessions & Prompt Execution ────────────────────────────────────
    setActiveSessionId: (id) => updateStore({ activeSessionId: id }),
    setActiveAgentId: (id) => {
      updateStore({ activeAgentId: id });
      try { localStorage.setItem('forge_ade_active_agent_id', id); } catch {}
      const targetAgent = get().agents.find(a => a.id === id);
      if (targetAgent && targetAgent.type !== 'internal') {
        const explicitModel = explicitModelConfig.agentId === id
          ? explicitModelConfig.model
          : (targetAgent.model || (targetAgent.supportedModels && targetAgent.supportedModels[0]) || '');
        get().setCurrentModel(explicitModel);
      }
    },
    toggleAgentEnabled: (id) => updateStore(prev => ({ agents: prev.agents.map(a => a.id === id ? { ...a, enabled: !a.enabled } : a) })),
    updateAgentConfig: (id, updates) => updateStore(prev => ({ agents: prev.agents.map(a => a.id === id ? { ...a, ...updates } : a) })),
    addAgent: (agent) => updateStore(prev => ({ agents: [...prev.agents, agent] })),
    deleteAgent: (id) => updateStore(prev => ({ agents: prev.agents.filter(a => a.id !== id) })),
    setAcpEnabled: (enabled) => updateStore({ acpEnabled: enabled }),
    setPrivacySettings: (settings) => updateStore(prev => ({ privacySettings: typeof settings === 'function' ? settings(prev.privacySettings) : settings })),
    setDiagnostics: (diagnostics) => updateStore({ diagnostics }),

    setCurrentModel: (model, providerId) => {
      explicitModelConfig.agentId = get().activeAgentId;
      explicitModelConfig.model = model;
      try {
        if (model) localStorage.setItem('forge_ade_current_model', model);
        else localStorage.removeItem('forge_ade_current_model');
      } catch {}

      updateStore({ currentModel: model });

      const activeSessionId = get().activeSessionId;
      if (activeSessionId) {
        updateStore(prev => ({
          sessions: prev.sessions.map(s => {
            if (s.id !== activeSessionId) return s;
            const updated = {
              ...s,
              model,
              ...(providerId ? { provider: providerId } : {}),
              updatedAt: new Date().toISOString()
            };
            try { ApiBridge.saveSessionJsonl(updated, updated.workspacePath ?? get().activeWorkspacePath); } catch {}
            return updated;
          })
        }));
      }
    },

    setAgentExecutionMode: (agentExecutionMode) => updateStore({ agentExecutionMode }),
    setReasoningLevel: (lvl) => {
      updateStore({ reasoningLevel: lvl });
      try { localStorage.setItem('forge_ade_reasoning_level', lvl); } catch {}
      void SetActiveReasoningEffort(lvl).catch(() => {});
    },
    setAgentEngine: (agentEngine) => updateStore({ agentEngine }),

    refreshAgentModels: async (targetAgentId) => {
      try {
        const targetAgents = targetAgentId ? get().agents.filter(a => a.id === targetAgentId) : get().agents;
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
        updateStore(prev => ({
          agents: prev.agents.map(a => {
            const found = modelResults.find(m => m.id === a.id);
            if (found && found.models && found.models.length > 0) {
              const cleanDiscovered = found.models.filter(m => !m.includes('myairouter_cloud'));
              const cleanCurrent = (a.supportedModels || []).filter(m => !m.includes('myairouter_cloud'));
              const merged = Array.from(new Set([...cleanDiscovered, ...cleanCurrent]));
              const currentValid = a.model && merged.includes(a.model) && !a.model.includes('myairouter_cloud');
              return {
                ...a,
                supportedModels: merged,
                model: currentValid ? a.model : cleanDiscovered[0]
              };
            }
            return a;
          })
        }));
      } catch {}
    },

    sendSideConversationPrompt: async (promptText, model, mode) => {
      const activeSessionId = get().activeSessionId;
      if (!promptText.trim() || !activeSessionId) return;
      const activeSessionObj = get().sessions.find(s => s.id === activeSessionId);
      if (!activeSessionObj) return;

      await runSideConversationPrompt({
        promptText,
        model: model || get().currentModel,
        mode: mode || 'full',
        activeSession: activeSessionObj,
        activeWorkspacePath: get().activeWorkspacePath || '',
        diffs: activeSessionObj.diffs || get().diffs || [],
        gitFiles: get().gitFiles,
        gitBranch: get().gitBranch,
        engine,
        updateMessages: (updater) => {
          updateStore(prev => ({
            sessions: prev.sessions.map(s => {
              if (s.id !== activeSessionId) return s;
              return {
                ...s,
                sideConversationMessages: updater(s.sideConversationMessages || [])
              };
            })
          }));
        }
      });
    },

    clearSideConversation: () => {
      const activeSessionId = get().activeSessionId;
      if (!activeSessionId) return;
      updateStore(prev => ({
        sessions: prev.sessions.map(s => s.id === activeSessionId ? { ...s, sideConversationMessages: [] } : s)
      }));
    },

    createNewSession: (initialPrompt, selectedAgentId, forcedId) => {
      const newId = forcedId || `session-${Date.now()}`;
      const effectiveAgentId = selectedAgentId || get().activeAgentId;
      const agentObj = get().agents.find(a => a.id === effectiveAgentId) || get().activeAgent;

      let effectivePrompt = initialPrompt;
      if (effectivePrompt && effectivePrompt.startsWith('/')) {
        const parts = effectivePrompt.slice(1).split(/\s+/);
        const cmdName = parts[0].toLowerCase();
        const userArgs = parts.slice(1).join(' ');
        const matched = get().customCommands.find(c => c && c.enabled && (c.name || '').toLowerCase() === cmdName);
        if (matched) {
          const template = matched.promptTemplate || (matched as any).prompt || '';
          effectivePrompt = userArgs ? `${template}\n\nUser instructions: ${userArgs}` : template;
        }
      }

      const effectiveModel = agentObj.type !== 'internal' ? (get().currentModel || agentObj.model || '') : (agentObj.model || get().currentModel);
      const matchedProvider = get().providers.find(p => p.enabled && (p.selectedModels || p.models || []).includes(effectiveModel));
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
        workspacePath: get().activeWorkspacePath,
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

      updateStore(prev => ({
        sessions: [newSession, ...prev.sessions],
        activeSessionId: newId,
        mode: 'agent',
        activeTaskKind: 'forge'
      }));

      if (effectivePrompt) {
        void executeAgentMessage(newId, effectivePrompt, newSession);
      }
      return newId;
    },

    sendAgentPrompt: (promptText) => {
      if (!promptText.trim()) return;
      let effectivePrompt = promptText.trim();
      if (effectivePrompt.startsWith('/')) {
        const parts = effectivePrompt.slice(1).split(/\s+/);
        const cmdName = parts[0].toLowerCase();
        const userArgs = parts.slice(1).join(' ');
        const matched = get().customCommands.find(c => c && c.enabled && (c.name || '').toLowerCase() === cmdName);
        if (matched) {
          const template = matched.promptTemplate || (matched as any).prompt || '';
          effectivePrompt = userArgs ? `${template}\n\nUser instructions: ${userArgs}` : template;
        }
      }

      if (effectivePrompt.toLowerCase() === '/compact') {
        void get().compactCurrentSession();
        return;
      }

      const activeSessionId = get().activeSessionId;
      if (!activeSessionId) {
        get().createNewSession(promptText);
        return;
      }

      const agentObj = get().agents.find(a => a.id === get().activeAgentId) || get().activeAgent;
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

      updateStore(prev => ({
        sessions: prev.sessions.map(s => {
          if (s.id !== activeSessionId) return s;
          const isDefault = !s.title || s.title === 'New Session' || s.title.startsWith('session-');
          return {
            ...s,
            title: isDefault ? formatSmartSessionTitle(promptText) : s.title,
            status: 'running',
            updatedAt: new Date().toISOString(),
            messages: [...s.messages.map(m => m.isThinking ? { ...m, isThinking: false } : m), userMsg, agentPlaceholder]
          };
        })
      }));

      void executeAgentMessage(activeSessionId, effectivePrompt);
    },

    steerAgentPrompt: async (instruction) => {
      const trimmed = instruction.trim();
      const activeSessionId = get().activeSessionId;
      if (!trimmed || !activeSessionId) return;

      const currentSession = get().sessions.find(s => s.id === activeSessionId);
      if (!currentSession) return;

      if (currentSession.status !== 'running') {
        get().sendAgentPrompt(trimmed);
        return;
      }

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

      let goId: string | null | undefined = currentSession.goSessionId;
      if (!goId) goId = await ensureGoHarness(currentSession);
      if (goId) goAgentSessions.preempt(goId);

      updateStore(prev => ({
        sessions: prev.sessions.map(s => {
          if (s.id !== activeSessionId) return s;
          const cleaned = s.messages.map(m => {
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
            messages: [...cleaned, userMsg, agentPlaceholder]
          };
        })
      }));

      if (goId) {
        await goAgentSessions.steer(goId, trimmed);
      }
    },

    stopAgentExecution: () => {
      const activeSessionId = get().activeSessionId;
      const current = activeSessionId ? get().sessions.find(s => s.id === activeSessionId) : null;
      if (current?.goSessionId) StopAgentTurn(current.goSessionId);
      if (current?.acpSessionId) void ACPCancel(current.acpSessionId).catch(() => {});
      engine.abort();

      if (!activeSessionId) return;
      updateStore(prev => ({
        sessions: prev.sessions.map(s => {
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
          const updated = { ...s, status: 'idle' as const, goState: 'idle' as const, pendingTools: [], pendingQuestions: [], messages: msgs };
          void ApiBridge.saveSessionJsonl(updated, updated.workspacePath ?? get().activeWorkspacePath);
          return updated;
        })
      }));
    },

    compactCurrentSession: async () => {
      const activeSessionId = get().activeSessionId;
      if (!activeSessionId) return;
      const sess = get().sessions.find(s => s.id === activeSessionId);
      if (!sess) return;

      if (!sess.messages || sess.messages.length <= 2) {
        showToast('Session is already compact', 'info');
        return;
      }

      if (sess.goSessionId) {
        try { await CompactAgentSession(sess.goSessionId); } catch (e) {
          console.warn('Failed to compact Go session:', e);
        }
      }

      updateStore(prev => ({
        sessions: prev.sessions.map(s => {
          if (s.id !== activeSessionId) return s;
          const msgs = s.messages;
          const keepTail = msgs.length > 4 ? 3 : 1;
          const firstMsg = msgs[0];
          const recentMsgs = msgs.slice(msgs.length - keepTail);
          const compactedCount = msgs.length - (1 + recentMsgs.length);
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

          const updated = {
            ...s,
            messages: [firstMsg, compactSummaryMsg, ...recentMsgs],
            updatedAt: new Date().toISOString()
          };
          try { ApiBridge.saveSessionJsonl(updated, updated.workspacePath ?? get().activeWorkspacePath); } catch {}
          return updated;
        })
      }));
      showToast('Session compacted: earlier turns compressed into summary', 'success');
    },

    deleteSession: (id) => {
      void get().deleteSessionPermanently(id);
    },

    deleteSessionPermanently: async (id) => {
      const deleted = get().sessions.find(s => s.id === id);
      if (deleted?.goSessionId) goAgentSessions.unregister(deleted.goSessionId);

      updateStore(prev => ({
        savedSessions: prev.savedSessions.filter(s => s.id !== id),
        sessions: prev.sessions.filter(s => s.id !== id),
        activeSessionId: prev.activeSessionId === id ? null : prev.activeSessionId
      }));
      try {
        localStorage.setItem('forge_ade_sessions', JSON.stringify(get().sessions));
      } catch {}
      await ApiBridge.deleteSessionJsonl(id, get().activeWorkspacePath);
    },

    openSessionFromHistory: (sessionToOpen) => {
      updateStore(prev => {
        const exists = prev.sessions.some(s => s.id === sessionToOpen.id);
        const next = exists ? prev.sessions : [sessionToOpen, ...prev.sessions];
        return {
          sessions: next,
          activeSessionId: sessionToOpen.id,
          mode: 'agent',
          activeTaskKind: 'forge'
        };
      });
      if (sessionToOpen.model) get().setCurrentModel(sessionToOpen.model);
      if (sessionToOpen.agentId) get().setActiveAgentId(sessionToOpen.agentId);
    },

    reloadSavedSessions: async () => {
      try {
        const diskSessions = await ApiBridge.loadSessionsJsonl(get().activeWorkspacePath);
        if (diskSessions && diskSessions.length > 0) {
          const frontendShaped = diskSessions
            .filter((s: any) => s && !isGoHarnessSession(s))
            .map(normalizeLoadedSession)
            .filter((s): s is AgentSession => s !== null);

          if (frontendShaped.length > 0) {
            updateStore(prev => ({
              savedSessions: frontendShaped,
              sessions: prev.sessions.length === 0 ? [frontendShaped[0]] : prev.sessions
            }));
          }
        }
      } catch {}
    },

    toggleArchiveSession: (id) => {
      updateStore(prev => {
        const next = new Set(prev.archivedSessionIds);
        if (next.has(id)) next.delete(id);
        else next.add(id);
        try { localStorage.setItem('forge_archived_sessions', JSON.stringify(Array.from(next))); } catch {}
        return { archivedSessionIds: next };
      });
    },

    // Subagent actions
    setActiveSubagentId: (id) => updateStore({ activeSubagentId: id }),
    openSubagentInSidebar: (subagentId) => {
      updateStore({ activeSubagentId: subagentId });
      get().setIsRightActionDrawerOpen(true);
      window.dispatchEvent(new CustomEvent('forge:open-subagent-tab', { detail: { subagentId } }));
    },
    toggleTask: async (taskId, completed) => {
      const activeSession = get().activeSession;
      if (!activeSession) return;
      updateStore(prev => ({
        sessions: prev.sessions.map(s => {
          if (s.id !== activeSession.id) return s;
          return {
            ...s,
            tasks: (s.tasks || []).map(t => t.id === taskId ? { ...t, completed, status: completed ? 'completed' : 'pending' } : t)
          };
        })
      }));
      const goId = activeSession.goSessionId || activeSession.id;
      try { await ToggleAgentTask(goId, taskId, completed); } catch (err) {
        console.error('Failed to toggle agent task:', err);
      }
    },
    clearTasks: async () => {
      const activeSession = get().activeSession;
      if (!activeSession) return;
      updateStore(prev => ({
        sessions: prev.sessions.map(s => s.id === activeSession.id ? { ...s, tasks: [] } : s)
      }));
      const goId = activeSession.goSessionId || activeSession.id;
      try { await ClearAgentTasks(goId); } catch (err) {
        console.error('Failed to clear agent tasks:', err);
      }
    },
    sendSubagentMessage: async (subagentId, prompt) => {
      if (!prompt.trim() || !subagentId) return;
      try { await SendAgentMessage(subagentId, prompt, []); } catch (err) {
        console.error('Failed to send message to subagent:', err);
      }
    },

    // ── Automations & Codebase Indexing ──────────────────────────────────────
    fetchAutomations: () => useIntegrationsStore.getState().fetchAutomations(),
    saveAutomation: (input) => useIntegrationsStore.getState().saveAutomation(input),
    deleteAutomation: (id) => useIntegrationsStore.getState().deleteAutomation(id),
    setAutomationEnabled: (id, en) => useIntegrationsStore.getState().setAutomationEnabled(id, en),
    runAutomation: (automation) => {
      const sessionId = `session-${Date.now()}`;
      get().createNewSession(automation.prompt, undefined, sessionId);
      get().setIsAutomationsOpen(false);
      const wsPath = automation.workspace || get().activeWorkspacePath || '';
      void ApiBridge.recordAutomationRun(automation.id, sessionId, wsPath)
        .then(() => get().fetchAutomations())
        .catch(() => {});
      return sessionId;
    },

    fetchIndexStatus: async () => {
      try {
        const status = await ApiBridge.getIndexStatus();
        updateStore({ indexStatus: status });
      } catch (e) {
        console.warn('fetchIndexStatus failed', e);
      }
    },
    reindexWorkspace: async () => {
      updateStore({ isReindexing: true });
      try {
        const res = await ApiBridge.reindexWorkspace();
        await get().fetchIndexStatus();
        return res;
      } finally {
        updateStore({ isReindexing: false });
      }
    },

    // ── Delegated Store Actions ──────────────────────────────────────────────
    // Layout
    setIsFolderModalOpen: (val) => useLayoutStore.getState().setIsFolderModalOpen(val),
    setIsSSHModalOpen: (val) => useLayoutStore.getState().setIsSSHModalOpen(val),
    setIsRemoteUploadModalOpen: (val) => useLayoutStore.getState().setIsRemoteUploadModalOpen(val),
    setRemoteUploadTargetDir: (val) => useLayoutStore.getState().setRemoteUploadTargetDir(val),
    openRemoteUploader: (targetDir) => {
      useLayoutStore.getState().setRemoteUploadTargetDir(targetDir || get().activeWorkspacePath || '');
      useLayoutStore.getState().setIsRemoteUploadModalOpen(true);
    },
    setActiveActivity: (act) => useLayoutStore.getState().setActiveActivity(act),
    setIsLeftSidebarOpen: (val) => {
      if (get().mode === 'editor') useLayoutStore.getState().setEditorLeftSidebarOpen(val);
      else useLayoutStore.getState().setAgentLeftSidebarOpen(val);
    },
    setAgentLeftSidebarOpen: (val) => useLayoutStore.getState().setAgentLeftSidebarOpen(val),
    setAgentRightSidebarOpen: (val) => useLayoutStore.getState().setRightActionDrawerOpen(val),
    setEditorLeftSidebarOpen: (val) => useLayoutStore.getState().setEditorLeftSidebarOpen(val),
    setEditorRightSidebarOpen: (val) => useLayoutStore.getState().setRightActionDrawerOpen(val),
    setIsRightActionDrawerOpen: (val) => useLayoutStore.getState().setRightActionDrawerOpen(val),
    setAgentViewMode: (mode) => useLayoutStore.getState().setAgentViewMode(mode),
    setLeftSidebarView: (v) => useLayoutStore.getState().setLeftSidebarView(v),
    setIsCommandPaletteOpen: (val) => useLayoutStore.getState().setIsCommandPaletteOpen(val),
    setIsAutomationsOpen: (val) => useLayoutStore.getState().setIsAutomationsOpen(val),
    setCommandPaletteMode: (mode: CommandPaletteMode) => useLayoutStore.getState().setCommandPaletteMode(mode),
    openCommandPalette: (mode?: CommandPaletteMode) => useLayoutStore.getState().openCommandPalette(mode),
    setIsCustomizationsModalOpen: (val) => useLayoutStore.getState().setIsCustomizationsModalOpen(val),
    setIsSettingsModalOpen: (val) => useLayoutStore.getState().setIsSettingsModalOpen(val),
    setSettingsActiveSection: (sec) => useLayoutStore.getState().setSettingsActiveSection(sec),
    setPreviousMode: (mode) => updateStore({ previousMode: mode }),
    setLeftSidebarWidth: (w) => useLayoutStore.getState().setLeftSidebarWidth(w),
    setRightPaneWidth: (w) => useLayoutStore.getState().setRightPaneWidth(w),

    // Git
    setIsGitClientOpen: (val) => useGitStore.getState().setIsGitClientOpen(val),
    setActiveDiff: (diff) => useGitStore.getState().setActiveDiff(diff),
    addDiff: (diff) => useGitStore.getState().addDiff(diff),
    rejectDiff: (diffId) => useGitStore.getState().rejectDiff(diffId),
    acceptDiff: async (diffId) => {
      await useGitStore.getState().acceptDiff(diffId, get().refreshFiles);
    },
    refreshGitStatus: async () => {
      await useGitStore.getState().refreshGitStatus(get().activeWorkspacePath);
    },
    refreshGitLog: async () => {
      await useGitStore.getState().refreshGitLog(get().activeWorkspacePath);
    },

    // Terminal
    setIsBottomTerminalOpen: (val) => useTerminalStore.getState().setIsBottomTerminalOpen(val),

    // Board
    refreshBoardTasks: async () => {
      await useBoardStore.getState().refreshBoardTasks(get().agentWorkspacePath || get().activeWorkspacePath || '');
    },
    addBoardTask: (task) => useBoardStore.getState().addBoardTask(task, get().agentWorkspacePath || get().activeWorkspacePath || ''),
    updateBoardTask: (id, patch) => useBoardStore.getState().updateBoardTask(id, patch, get().agentWorkspacePath || get().activeWorkspacePath || ''),
    deleteBoardTask: (id) => useBoardStore.getState().deleteBoardTask(id, get().agentWorkspacePath || get().activeWorkspacePath || ''),
    moveBoardTaskStatus: (id, status) => useBoardStore.getState().moveBoardTaskStatus(id, status, get().agentWorkspacePath || get().activeWorkspacePath || ''),
    refreshJiraTasks: async () => {
      await useBoardStore.getState().refreshBoardTasks(get().agentWorkspacePath || get().activeWorkspacePath || '');
    },
    addJiraTask: (task) => useBoardStore.getState().addBoardTask(task, get().agentWorkspacePath || get().activeWorkspacePath || ''),
    updateJiraTask: (id, patch) => useBoardStore.getState().updateBoardTask(id, patch, get().agentWorkspacePath || get().activeWorkspacePath || ''),
    deleteJiraTask: (id) => useBoardStore.getState().deleteBoardTask(id, get().agentWorkspacePath || get().activeWorkspacePath || ''),
    moveJiraTaskStatus: (id, status) => useBoardStore.getState().moveBoardTaskStatus(id, status, get().agentWorkspacePath || get().activeWorkspacePath || ''),

    // Providers
    updateProvider: (id, updates) => useProviderStore.getState().updateProvider(id, updates),
    addProvider: (provider) => useProviderStore.getState().addProvider(provider),
    deleteProvider: (id) => useProviderStore.getState().deleteProvider(id),
    addModelToProvider: (providerId, modelName) => useProviderStore.getState().addModelToProvider(providerId, modelName),
    deleteModelFromProvider: (providerId, modelName) => useProviderStore.getState().deleteModelFromProvider(providerId, modelName),
    toggleModelSelection: (providerId, modelName) => useProviderStore.getState().toggleModelSelection(providerId, modelName),
    selectAllModels: (providerId, models) => useProviderStore.getState().selectAllModels(providerId, models),
    deselectAllModels: (providerId, models) => useProviderStore.getState().deselectAllModels(providerId, models),
    fetchProviderModels: (providerId) => useProviderStore.getState().fetchProviderModels(providerId),

    // Integrations
    addMcp: (mcp) => useIntegrationsStore.getState().addMcp(mcp),
    toggleMcp: (id) => useIntegrationsStore.getState().toggleMcp(id),
    deleteMcp: (id) => useIntegrationsStore.getState().deleteMcp(id),
    addSkill: (skill) => useIntegrationsStore.getState().addSkill(skill),
    toggleSkill: (id) => useIntegrationsStore.getState().toggleSkill(id),
    deleteSkill: (id) => useIntegrationsStore.getState().deleteSkill(id),
    createBackendSkill: (req) => useIntegrationsStore.getState().createBackendSkill(req),
    deleteBackendSkill: (name) => useIntegrationsStore.getState().deleteBackendSkill(name),
    reloadSkills: () => useIntegrationsStore.getState().reloadSkills(),
    runDiscovery: () => useIntegrationsStore.getState().runDiscovery(),
    importDiscoveredMcp: (disc) => useIntegrationsStore.getState().importDiscoveredMcp(disc),
    importDiscoveredSkill: (disc) => useIntegrationsStore.getState().importDiscoveredSkill(disc),
    fetchPlugins: () => useIntegrationsStore.getState().fetchPlugins(),
    createPlugin: (req) => useIntegrationsStore.getState().createPlugin(req),
    togglePlugin: (id, en) => useIntegrationsStore.getState().togglePlugin(id, en),
    deletePlugin: (id) => useIntegrationsStore.getState().deletePlugin(id),
    reloadPlugins: () => useIntegrationsStore.getState().reloadPlugins(),
    fetchMemories: () => useIntegrationsStore.getState().fetchMemories(),
    saveMemory: (entry) => useIntegrationsStore.getState().saveMemory(entry),
    deleteMemory: (id) => useIntegrationsStore.getState().deleteMemory(id),
    reloadMemories: () => useIntegrationsStore.getState().reloadMemories(),
    addCustomCommand: (cmd) => useIntegrationsStore.getState().addCustomCommand(cmd),
    updateCustomCommand: (id, up) => useIntegrationsStore.getState().updateCustomCommand(id, up),
    deleteCustomCommand: (id) => useIntegrationsStore.getState().deleteCustomCommand(id),
    toggleCustomCommand: (id) => useIntegrationsStore.getState().toggleCustomCommand(id),
    updateHook: (id, up) => useIntegrationsStore.getState().updateHook(id, up),
    toggleHook: (id) => useIntegrationsStore.getState().toggleHook(id),
    addHook: (hook) => useIntegrationsStore.getState().addHook(hook),
    deleteHook: (id) => useIntegrationsStore.getState().deleteHook(id),
    updateBrowserSettings: (up) => useIntegrationsStore.getState().updateBrowserSettings(up),
    updateComputerSettings: (up) => useIntegrationsStore.getState().updateComputerSettings(up)
  };
});

// Wrap setState to ensure external calls (e.g., tests, imperative resets) also keep derived properties synchronized
const origSetState = useWorkspace.setState;
useWorkspace.setState = (updater: any, replace?: any) => {
  origSetState((state) => {
    const partial = typeof updater === 'function' ? updater(state) : updater;
    const combined = { ...state, ...partial };
    return {
      ...partial,
      ...computeDerived(combined)
    };
  }, replace);
};

// ── Two-way Synchronization with Sub-stores ──────────────────────────────────
useGitStore.subscribe((gitState) => {
  useWorkspace.setState({
    gitBranch: gitState.gitBranch,
    gitFiles: gitState.gitFiles,
    gitCommits: gitState.gitCommits,
    diffs: gitState.diffs,
    activeDiff: gitState.activeDiff,
    isGitClientOpen: gitState.isGitClientOpen,
  });
});

useTerminalStore.subscribe((termState) => {
  useWorkspace.setState({
    isBottomTerminalOpen: termState.isBottomTerminalOpen,
  });
});

useLayoutStore.subscribe((layoutState) => {
  const currentMode = useWorkspace.getState().mode;
  useWorkspace.setState({
    isFolderModalOpen: layoutState.isFolderModalOpen,
    isSSHModalOpen: layoutState.isSSHModalOpen,
    isRemoteUploadModalOpen: layoutState.isRemoteUploadModalOpen,
    remoteUploadTargetDir: layoutState.remoteUploadTargetDir,
    activeActivity: layoutState.activeActivity,
    agentLeftSidebarOpen: layoutState.agentLeftSidebarOpen,
    editorLeftSidebarOpen: layoutState.editorLeftSidebarOpen,
    agentRightSidebarOpen: layoutState.rightActionDrawerOpen,
    editorRightSidebarOpen: layoutState.rightActionDrawerOpen,
    isRightActionDrawerOpen: layoutState.rightActionDrawerOpen,
    isLeftSidebarOpen: currentMode === 'editor' ? layoutState.editorLeftSidebarOpen : layoutState.agentLeftSidebarOpen,
    agentViewMode: layoutState.agentViewMode,
    leftSidebarView: layoutState.leftSidebarView,
    isCommandPaletteOpen: layoutState.isCommandPaletteOpen,
    isAutomationsOpen: layoutState.isAutomationsOpen,
    commandPaletteMode: layoutState.commandPaletteMode,
    isCustomizationsModalOpen: layoutState.isCustomizationsModalOpen,
    isSettingsModalOpen: layoutState.isSettingsModalOpen,
    settingsActiveSection: layoutState.settingsActiveSection,
    leftSidebarWidth: layoutState.leftSidebarWidth,
    rightPaneWidth: layoutState.rightPaneWidth,
  });
});

useBoardStore.subscribe((boardState) => {
  useWorkspace.setState({
    boardTasks: boardState.boardTasks,
    jiraTasks: boardState.boardTasks,
  });
});

useIntegrationsStore.subscribe((intState) => {
  useWorkspace.setState({
    mcps: intState.mcps,
    skills: intState.skills,
    discoveredMcps: intState.discoveredMcps,
    discoveredSkills: intState.discoveredSkills,
    isDiscovering: intState.isDiscovering,
    plugins: intState.plugins,
    memories: intState.memories,
    customCommands: intState.customCommands,
    hooks: intState.hooks,
    browserSettings: intState.browserSettings,
    computerSettings: intState.computerSettings,
    automations: intState.automations,
    automationsLoading: intState.automationsLoading,
  });
});

useProviderStore.subscribe((provState) => {
  useWorkspace.setState({
    providers: provState.providers,
  });
});

// Register global open side file callback
globalOpenSideFileFn = (rawPath, line) => {
  void useWorkspace.getState().openSideFile(rawPath, line);
};

// ── Agent Message Execution Helpers ──────────────────────────────────────────
async function ensureGoHarness(storeSession: AgentSession): Promise<string | null> {
  const store = useWorkspace.getState();
  const makeGoCallbacks = (storeSessionId: string, goSessionId: string) => {
    return createAgentCallbacks({
      storeSessionId,
      goSessionId,
      activeWorkspacePath: store.activeWorkspacePath,
      addDiff: (d) => store.addDiff(d),
      updateSession: (sessionId, updater) => {
        store.setSessions(prev => prev.map(s => s.id === sessionId ? updater(s) : s));
      }
    });
  };

  if (storeSession.goSessionId) {
    goAgentSessions.register(storeSession.goSessionId, makeGoCallbacks(storeSession.id, storeSession.goSessionId));
    return storeSession.goSessionId;
  }

  const goId = await goAgentSessions.createSession(storeSession.title, storeSession.workspacePath || store.activeWorkspacePath);
  if (!goId) return null;
  storeSession.goSessionId = goId;
  store.setSessions(prev => prev.map(s => s.id === storeSession.id ? { ...s, goSessionId: goId, goState: 'idle' } : s));
  goAgentSessions.register(goId, makeGoCallbacks(storeSession.id, goId));
  SetAgentAutoApprove(goId, store.agentExecutionMode === 'bypass').catch(() => {});
  SetActiveReasoningEffort(store.reasoningLevel).catch(() => {});
  return goId;
}

async function executeAgentMessage(storeSessionId: string, prompt: string, sessionOverride?: AgentSession) {
  const store = useWorkspace.getState();
  const storeSession = sessionOverride || store.sessions.find(s => s.id === storeSessionId);
  if (!storeSession) return;

  const agentObj = store.agents.find(a => a.id === storeSession.agentId) || store.activeAgent;

  // External ACP agents
  if (agentObj.type !== 'internal') {
    let acpId = storeSession.acpSessionId;
    const startOrResumeACPSession = async (previousId?: string): Promise<string> => {
      let permMode: 'supervised' | 'auto' | 'auto_edits' | 'full' = 'supervised';
      if (store.agentExecutionMode === 'bypass') permMode = 'full';
      else if (store.agentExecutionMode === 'plan') permMode = 'auto_edits';
      else if (store.agentExecutionMode === 'code') permMode = 'auto';

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

      const rawModel = storeSession.model || store.currentModel;
      const effectiveModel = (rawModel && rawModel !== 'default' && rawModel !== 'Default') ? rawModel : '';
      const resumeRemoteId = storeSession.remoteSessionId || previousId || storeSession.acpSessionId;

      const acpSession = await ACPStartSession({
        agentId: agentObj.id,
        title: storeSession.title || 'ACP Session',
        command: cmd,
        args: agentArgs,
        cwd: storeSession.workspacePath || store.activeWorkspacePath || '.',
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
        store.setSessions(prev => prev.map(s => s.id === storeSession.id ? {
          ...s,
          acpSessionId: newAcpId,
          remoteSessionId: acpSession.remoteSessionId || acpSession.id,
          availableCommands: s.availableCommands && s.availableCommands.length > 0 ? s.availableCommands : initialCmds,
          goState: 'idle'
        } : s));
        goAgentSessions.register(newAcpId, createAgentCallbacks({
          storeSessionId: storeSession.id,
          goSessionId: newAcpId,
          activeWorkspacePath: store.activeWorkspacePath,
          addDiff: (d) => store.addDiff(d),
          updateSession: (sessionId, updater) => {
            store.setSessions(prev => prev.map(s => s.id === sessionId ? updater(s) : s));
          }
        }));
        return newAcpId;
      }
      throw new Error(`Failed to initialize or resume ACP session with agent "${agentObj.name}"`);
    };

    if (!acpId) {
      try {
        acpId = await startOrResumeACPSession();
      } catch (err: any) {
        console.error('[workspaceStore] ACP start session error:', err);
        store.setSessions(prev => prev.map(s => {
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
      goAgentSessions.register(acpId, createAgentCallbacks({
        storeSessionId: storeSession.id,
        goSessionId: acpId,
        activeWorkspacePath: store.activeWorkspacePath,
        addDiff: (d) => store.addDiff(d),
        updateSession: (sessionId, updater) => {
          store.setSessions(prev => prev.map(s => s.id === sessionId ? updater(s) : s));
        }
      }));
      await ACPPrompt(acpId, prompt);
    } catch (err: any) {
      const errMsg = err?.message || String(err);
      if (errMsg.includes('session not found') || errMsg.includes('no active remote session') || errMsg.includes('client closed') || errMsg.includes('connection closed')) {
        try {
          const freshAcpId = await startOrResumeACPSession(acpId);
          goAgentSessions.register(freshAcpId, createAgentCallbacks({
            storeSessionId: storeSession.id,
            goSessionId: freshAcpId,
            activeWorkspacePath: store.activeWorkspacePath,
            addDiff: (d) => store.addDiff(d),
            updateSession: (sessionId, updater) => {
              store.setSessions(prev => prev.map(s => s.id === sessionId ? updater(s) : s));
            }
          }));
          await ACPPrompt(freshAcpId, prompt);
          return;
        } catch (resumeErr: any) {
          console.error('[workspaceStore] ACP auto-resume failed:', resumeErr);
        }
      }
      console.error('[workspaceStore] ACP prompt error:', err);
      store.setSessions(prev => prev.map(s => {
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
    return;
  }

  // Internal Go agent harness
  let goId: string | null = null;
  try {
    goId = await Promise.race([
      ensureGoHarness(storeSession),
      new Promise<null>(resolve => setTimeout(() => resolve(null), 2500))
    ]);
  } catch {
    goId = null;
  }

  if (!goId) {
    await streamAgentSessionDirectly({
      engine,
      storeSessionId,
      promptText: prompt,
      sessionOverride,
      sessions: store.sessions,
      currentModel: store.currentModel,
      activeWorkspacePath: store.activeWorkspacePath,
      gitFiles: store.gitFiles,
      gitBranch: store.gitBranch,
      addDiff: (d) => store.addDiff(d),
      updateSession: (sessionId, updater) => {
        store.setSessions(prev => prev.map(s => s.id === sessionId ? updater(s) : s));
      }
    });
    return;
  }

  try {
    await useProviderStore.getState().activateGoModel(storeSession.model || store.currentModel);
    await goAgentSessions.send(goId, prompt, extractMentionedPaths(prompt, storeSession.workspacePath || store.activeWorkspacePath));
  } catch (err: any) {
    console.warn('[workspaceStore] Go send failed, falling back to direct stream:', err);
    await streamAgentSessionDirectly({
      engine,
      storeSessionId,
      promptText: prompt,
      sessionOverride,
      sessions: store.sessions,
      currentModel: store.currentModel,
      activeWorkspacePath: store.activeWorkspacePath,
      gitFiles: store.gitFiles,
      gitBranch: store.gitBranch,
      addDiff: (d) => store.addDiff(d),
      updateSession: (sessionId, updater) => {
        store.setSessions(prev => prev.map(s => s.id === sessionId ? updater(s) : s));
      }
    });
  }
}

// ── Lifecycle-Only WorkspaceProvider (Zero Context Re-renders) ───────────────
export const WorkspaceProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  useWorkspaceLifecycle();
  return <>{children}</>;
};
