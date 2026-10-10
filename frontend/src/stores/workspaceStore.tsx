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
  WorkspaceEntry,
  WindowWorkspaceState,
  TaskKind
} from '../types';
import { DEFAULT_AGENTS, DEFAULT_PRIVACY, DEFAULT_PROVIDERS } from './agentRegistryStore';
import { AgentEngine } from '../services/agentEngine';
import { ApiBridge } from '../services/apiBridge';
import { useUIStore } from '../hooks/store';
import { registerBenchStore } from '../lib/devBench';
import { EventsOn, StopAgentTurn, SetAgentAutoApprove, SetActiveModel, GetProviderProfiles, SyncAgentProviders, SaveProviderProfiles, CreateShell, OpenNewWindow, type Automation as ZAutomation, type AutomationSaveInput as ZAutomationSaveInput } from '../lib/wails';
import { showToast } from '../lib/toast';
import { formatDisplayTitle, parseFilePath } from '../lib/utils';
import { goAgentSessions, extractMentionedPaths, newThoughtStep } from '../services/goAgentSession';
import { useSessionStore } from './sessionStore';

export const DEFAULT_SLASH_COMMANDS: CustomSlashCommand[] = [
  {
    id: 'cmd-review',
    name: 'review',
    description: 'Perform a comprehensive code review focusing on bugs, security, and performance.',
    promptTemplate: 'Perform a comprehensive code review of the recently modified files. Identify potential bugs, security issues, and edge cases, and propose concrete improvements.',
    scope: 'workspace',
    enabled: true
  },
  {
    id: 'cmd-test',
    name: 'test',
    description: 'Analyze code coverage, run tests, and generate missing unit tests.',
    promptTemplate: 'Run the project test suite, analyze test output, and generate tests for uncovered critical edge cases in the active files.',
    scope: 'workspace',
    enabled: true
  },
  {
    id: 'cmd-lint',
    name: 'lint',
    description: 'Fix syntax, lint, and formatting errors in the current workspace.',
    promptTemplate: 'Check for compiler errors, lint warnings, and formatting issues. Fix any detected problems while preserving the existing logic.',
    scope: 'workspace',
    enabled: true
  },
  {
    id: 'cmd-plan',
    name: 'plan',
    description: 'Formulate an architectural implementation plan before modifying files.',
    promptTemplate: 'Carefully analyze the requested feature, identify all affected files and packages, outline architectural considerations, and provide a numbered step-by-step implementation plan before making edits.',
    scope: 'workspace',
    enabled: true
  },
  {
    id: 'cmd-commit',
    name: 'commit',
    description: 'Generate conventional commit messages and staged diff summaries.',
    promptTemplate: 'Inspect the current git status and staged changes. Summarize the changes following conventional commit syntax (feat, fix, refactor, docs) with clear bullet points.',
    scope: 'workspace',
    enabled: true
  }
];

// The Go agent harness (internal agents) persists block-based sessions into
// the same ~/.forge/sessions/<project>/ directory the frontend JSONL store
// uses: message content is an array of typed blocks and roles are
// assistant/tool. Those files are the harness transcript, not frontend chat
// sessions — detect them so they are never loaded into the chat UI.
function isGoHarnessSession(s: any): boolean {
  if (!s || !Array.isArray(s.messages)) return false;
  if (s.role_filter !== undefined || s.roleFilter !== undefined) return true;
  return s.messages.some((m: any) => m != null && Array.isArray(m.content));
}

// Defensive normalization: coerce any non-string message content (e.g. a
// legacy block array that slipped through) into plain text so the
// string-based chat renderer can never crash on it.
function normalizeLoadedSession(s: any): AgentSession | null {
  if (!s || typeof s !== 'object' || !Array.isArray(s.messages)) return null;
  const messages = s.messages.map((m: any) => {
    if (m == null || typeof m.content === 'string' || m.content === undefined || m.content === null) return m;
    if (Array.isArray(m.content)) {
      const text = m.content
        .filter((b: any) => b && b.type === 'text' && typeof b.text === 'string')
        .map((b: any) => b.text)
        .join('');
      return { ...m, content: text };
    }
    return { ...m, content: String(m.content) };
  });
  return { ...s, messages } as AgentSession;
}

export const DEFAULT_HOOKS: AgentHookConfig[] = [
  {
    id: 'hook-pre-turn',
    name: 'Pre-turn Environment Check',
    event: 'pre_turn',
    command: 'git status --porcelain',
    enabled: false,
    timeout: 5
  },
  {
    id: 'hook-post-file-write',
    name: 'Auto-format on File Write',
    event: 'post_file_write',
    command: 'go fmt ./... 2>/dev/null || prettier --write "$FORGE_FILE" 2>/dev/null',
    enabled: true,
    timeout: 10
  },
  {
    id: 'hook-post-turn',
    name: 'Post-turn Lint & Typecheck',
    event: 'post_turn',
    command: 'npm run lint --if-present 2>/dev/null || go vet ./... 2>/dev/null',
    enabled: false,
    timeout: 15
  },
  {
    id: 'hook-pre-commit',
    name: 'Pre-commit Smoke Test',
    event: 'pre_commit',
    command: 'npm test -- --bail 2>/dev/null || go test -short ./... 2>/dev/null',
    enabled: false,
    timeout: 30
  }
];

export const DEFAULT_BROWSER_SETTINGS: BrowserUseSettings = {
  enabled: true,
  headless: true,
  searchProvider: 'google',
  searchApiKey: '',
  remoteCdpUrl: '',
  viewport: '1280x800',
  maxExtractLength: 50000,
  allowJavaScript: false
};

export const DEFAULT_COMPUTER_SETTINGS: ComputerUseSettings = {
  permissionMode: 'confirm_dangerous',
  defaultShell: '/bin/zsh',
  commandTimeoutSeconds: 30,
  allowClipboard: true,
  screenCaptureScale: 1.0,
  terminalSandbox: true
};


interface WorkspaceContextType {
  mode: WorkspaceMode;
  setMode: (mode: WorkspaceMode) => void;
  /** Which task kind the unified workspace surface renders. */
  activeTaskKind: TaskKind;
  setActiveTaskKind: (kind: TaskKind) => void;
  theme: ThemeMode;
  setTheme: (theme: ThemeMode) => void;
  toggleTheme: () => void;
  
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

  // Sidebars
  isLeftSidebarOpen: boolean;
  setIsLeftSidebarOpen: (val: boolean | ((prev: boolean) => boolean)) => void;
  leftSidebarWidth: number;
  setLeftSidebarWidth: (width: number) => void;
  rightPaneWidth: number;
  setRightPaneWidth: (width: number) => void;

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
  createNewSession: (initialPrompt?: string, agentId?: string) => void;
  sendAgentPrompt: (promptText: string) => void;
  stopAgentExecution: () => void;
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
  setActiveWorkspacePath: (path: string) => void;
  activeWorkspaces: WorkspaceEntry[];
  switchWorkspace: (path: string) => Promise<void>;
  /** Switch between the Agent and Editor surfaces, carrying the folder across. */
  switchSurface: (target: 'agent' | 'editor') => void;
  closeWorkspaceFromWindow: (path: string) => Promise<void>;
  openWorkspaceInNewWindow: (path: string) => Promise<void>;

  // LSP
  diagnostics: LSPDiagnostic[];
}

const WorkspaceContext = createContext<WorkspaceContextType | undefined>(undefined);

// ── Editor session persistence (restore tabs across app restarts) ───────────
// Only code-tab METADATA is persisted — never buffer content (buffers can be
// huge, and dirty buffers would silently resurrect stale text). Terminal and
// diff tabs die with the process by design. Content is re-read from disk on
// restore via openFileInEditor.
// Files above this size are refused by openFileInEditor's guard instead of
// being loaded into the buffer (RAM spike protection).
const MAX_EDITOR_FILE_BYTES = 8 * 1024 * 1024;

// Binary/executable files can't render as source — refused BEFORE any read so
// their bytes never cross the IPC into the WebView. Extensions that have a
// dedicated viewer (ImagePreview, PdfViewer) are intentionally absent here.
const BINARY_EXTENSIONS = new Set([
  // Executables & shared libraries
  'exe', 'dll', 'so', 'dylib', 'bin', 'com', 'elf', 'msi', 'msix', 'appimage',
  'flatpak', 'deb', 'rpm', 'apk', 'ipa', 'pkg', 'app', 'run', 'node',
  // Objects & static libs
  'o', 'obj', 'a', 'lib', 'ko', 'out', 'pdb', 'rlib',
  // Bytecode
  'class', 'jar', 'war', 'ear', 'pyc', 'pyo', 'pyd', 'wasm', 'dex',
  // Archives & disk images
  'zip', 'tar', 'gz', 'tgz', 'bz2', 'xz', 'zst', 'lz4', '7z', 'rar',
  'iso', 'dmg', 'img', 'vhd', 'cab', 'arj', 'lha',
  // Databases
  'db', 'db3', 'sqlite', 'sqlite3', 'sqlite-wal', 'sqlite-shm', 'mdb', 'accdb', 'ldb',
  // Fonts
  'ttf', 'otf', 'woff', 'woff2', 'eot', 'pfb', 'pfm', 'bdf',
  // Media without an in-app viewer
  'mp4', 'm4v', 'mov', 'avi', 'mkv', 'webm', 'flv', 'wmv',
  'mp3', 'wav', 'flac', 'ogg', 'aac', 'm4a', 'opus', 'mid', 'midi',
  // Design/office (zip-based binaries)
  'psd', 'ai', 'sketch', 'fig', 'xd', 'xcf',
  'doc', 'docx', 'xls', 'xlsx', 'ppt', 'pptx', 'pages', 'key', 'numbers', 'odt', 'ods', 'odp',
  // Game/engine assets & misc compiled
  'unity', 'asset', 'uasset', 'pak', 'wad', 'blender', 'blend', '3ds', 'obj-model'
]);

const isBinaryFile = (path: string): boolean => {
  const name = path.split('/').pop() || path;
  const dot = name.lastIndexOf('.');
  if (dot <= 0) return false;
  return BINARY_EXTENSIONS.has(name.slice(dot + 1).toLowerCase());
};

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

  const setMode = useCallback((newMode: WorkspaceMode | ((prev: WorkspaceMode) => WorkspaceMode)) => {
    setModeState(prev => {
      const resolved = typeof newMode === 'function' ? newMode(prev) : newMode;
      if (prev !== 'settings' && resolved === 'settings') {
        setPreviousMode(prev);
      }
      modeRef.current = resolved;
      return resolved;
    });
  }, []);
  useEffect(() => {
    modeRef.current = mode;
  }, [mode]);

  // Theme hydrates from the persisted choice so a light session survives an
  // app restart; the Appearance toggle can still switch it live.
  const [theme, setTheme] = useState<ThemeMode>(() =>
    typeof window !== 'undefined' && localStorage.getItem('forge-ade-theme') === 'light'
      ? 'light'
      : 'dark'
  );

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

  // Agent Mode and Editor Mode are separate surfaces, each with its own
  // workspace folder. Switching surfaces carries the folder across: the
  // agent's folder opens in the editor and vice versa (see switchSurface).
  const [agentWorkspacePath, setAgentWorkspacePathState] = useState<string>(() => {
    return localStorage.getItem('forge_ade_agent_workspace')
      || localStorage.getItem('forge_ade_workspace_path')
      || localStorage.getItem('my_ade_workspace_path')
      || '';
  });
  const [editorWorkspacePath, setEditorWorkspacePathState] = useState<string>(() => {
    return localStorage.getItem('forge_ade_editor_workspace')
      || localStorage.getItem('forge_ade_workspace_path')
      || localStorage.getItem('my_ade_workspace_path')
      || '';
  });
  // The folder every surface operates on follows the active mode.
  const activeWorkspacePath = mode === 'editor' ? editorWorkspacePath : agentWorkspacePath;

  // Multi-workspace window support (Zed-style: multiple workspaces alive in one window)
  const [activeWorkspaces, setActiveWorkspaces] = useState<WorkspaceEntry[]>(() => {
    const initialPath = localStorage.getItem('forge_ade_workspace_path') || localStorage.getItem('my_ade_workspace_path') || '';
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

  const [isFolderModalOpen, setIsFolderModalOpen] = useState<boolean>(false);
  const [isSSHModalOpen, setIsSSHModalOpen] = useState<boolean>(false);

  // Recent Workspaces
  const DEFAULT_RECENT_WORKSPACES = [
    '/Users/lutfiikbalmajid/hasdev/forge-ade',
    '/Users/lutfiikbalmajid/hasdev/MyAiRouter',
    '/Users/lutfiikbalmajid/hasdev/kendali-ai'
  ];

  const [recentWorkspaces, setRecentWorkspaces] = useState<string[]>(() => {
    try {
      const saved = localStorage.getItem('forge_ade_recent_workspaces') || localStorage.getItem('my_ade_recent_workspaces');
      if (saved) {
        const parsed = JSON.parse(saved);
        if (Array.isArray(parsed) && parsed.length > 0) return parsed;
      }
    } catch {}
    return DEFAULT_RECENT_WORKSPACES;
  });

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
      editorWsPathRef.current = '';
      setEditorWorkspacePathState('');
      localStorage.removeItem('forge_ade_editor_workspace');
    } else {
      agentWsPathRef.current = '';
      setAgentWorkspacePathState('');
      localStorage.removeItem('forge_ade_agent_workspace');
    }
    localStorage.removeItem('forge_ade_workspace_path');
    localStorage.removeItem('my_ade_workspace_path');
    setFiles([]);
    // Tabs, selection, and diffs belong to the closed workspace — clear them
    // here as well as in the switch effect (which also fires on this change).
    setOpenTabs([]);
    setActiveTabId(null);
    setSelectedFile(null);
    setDiffs([]);
    setActiveDiff(null);
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
  const agentWsPathRef = useRef(agentWorkspacePath);
  const editorWsPathRef = useRef(editorWorkspacePath);
  const switchSurfaceWorkspace = useCallback(async (newPath: string, surface: 'agent' | 'editor') => {
    const current = surface === 'editor' ? editorWsPathRef.current : agentWsPathRef.current;
    if (!newPath || newPath === current) return;

    // 1. Snapshot current workspace state before leaving
    const oldPath = wsPathRef.current;
    if (oldPath) {
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

  // Switch between the Agent and Editor surfaces. Each surface owns its own
  // workspace folder and functions; the switch carries the current folder
  // across, so the incoming surface opens (or keeps) that same project.
  const switchSurface = useCallback((target: 'agent' | 'editor') => {
    if (modeRef.current === target) return;
    modeRef.current = target;
    if (target === 'editor') {
      if (agentWsPathRef.current && agentWsPathRef.current !== editorWsPathRef.current) {
        void switchSurfaceWorkspace(agentWsPathRef.current, 'editor');
      }
    } else {
      if (editorWsPathRef.current && editorWsPathRef.current !== agentWsPathRef.current) {
        void switchSurfaceWorkspace(editorWsPathRef.current, 'agent');
      }
    }
    setModeState(target);
  }, [switchSurfaceWorkspace]);

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
        if (Array.isArray(parsed) && parsed.length > 0) return parsed;
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
          .filter((s): s is AgentSession => s !== null);
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
          d: sess.diffs?.length ?? 0
        });
        if (sessionPersistRef.current.get(sess.id) === fingerprint) continue;
        sessionPersistRef.current.set(sess.id, fingerprint);
        ApiBridge.saveSessionJsonl(sess, activeWorkspacePath);
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
            return {
              ...a,
              name: def ? def.name : a.name,
              provider: a.type === 'internal' ? 'custom' : (def ? def.provider : a.provider),
              command: def ? def.command : a.command,
              args: def ? def.args : a.args,
              type: def ? def.type : a.type,
              supportedModels: (a.supportedModels && a.supportedModels.length > 0) ? a.supportedModels : (def ? def.supportedModels : []),
              supportedEfforts: (a.supportedEfforts && a.supportedEfforts.length > 0) ? a.supportedEfforts : (def ? def.supportedEfforts : undefined),
              defaultEffort: a.defaultEffort || (def ? def.defaultEffort : undefined)
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
    return 'agent-internal';
  });

  const setActiveAgentId = useCallback((id: string) => {
    setActiveAgentIdState(id);
    localStorage.setItem('forge_ade_active_agent_id', id);
  }, []);

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
          return { ...a, supportedModels: found.models };
        }
        return a;
      }));
    } catch {
      // ignore
    }
  }, [agents]);

  // Providers state & Active Model Persistence
  const [providers, setProviders] = useState<LLMProviderConfig[]>(() => {
    try {
      const saved = localStorage.getItem('forge_ade_providers') || localStorage.getItem('my_ade_providers');
      if (saved) return JSON.parse(saved);
    } catch {
      // ignore
    }
    return DEFAULT_PROVIDERS;
  });

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
          ApiBridge.saveSessionJsonl(updated, activeWorkspacePath);
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
    if (curAgent && curAgent.type !== 'internal' && curAgent.supportedModels && curAgent.supportedModels.length > 0) {
      if (!currentModel || !curAgent.supportedModels.includes(currentModel)) {
        const def = curAgent.supportedModels[0];
        setCurrentModelState(def);
        try { localStorage.setItem('forge_ade_current_model', def); } catch {}
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

  useEffect(() => {
    localStorage.setItem('my_ade_providers', JSON.stringify(providers));
  }, [providers]);

  const updateProvider = useCallback((id: string, updates: Partial<LLMProviderConfig>) => {
    setProviders(prev => prev.map(p => p.id === id ? { ...p, ...updates } : p));
  }, []);

  const addProvider = useCallback((provider: LLMProviderConfig) => {
    setProviders(prev => [...prev, provider]);
  }, []);

  // Persist locally on every change — without this, provider edits (delete,
  // add, model lists) were lost on restart because nothing wrote back.
  useEffect(() => {
    try { localStorage.setItem('forge_ade_providers', JSON.stringify(providers)); } catch {}
  }, [providers]);

  const deleteProvider = useCallback((id: string) => {
    const next = providers.filter(p => p.id !== id);
    setProviders(next);
    // Hard-delete on the Go side too: SyncAgentProviders is merge-only, and
    // hydrateProvidersFromGo resurrected deleted providers within seconds.
    const payload = next.map(p => ({
      id: p.id,
      name: p.name,
      api_key: p.apiKey || '',
      base_url: p.baseUrl || '',
      enabled: !!p.enabled,
      available_models: p.models || [],
      selected_models: p.selectedModels && p.selectedModels.length > 0 ? p.selectedModels : (p.models || []),
    }));
    SaveProviderProfiles(payload).catch(() => { /* browser dev mode */ });
  }, [providers]);

  // ---------------------------------------------------------------------------
  // Provider sync to the Go harness — the internal agent authenticates through
  // the Go LLMClient, so the localStorage provider list (with API keys and
  // model selections) must be merged into Go's provider profiles. Merge (not
  // replace) so profiles configured in Settings are never clobbered.
  // ---------------------------------------------------------------------------
  const goProfilesRef = useRef<any[]>([]);
  const lastSyncedProvidersRef = useRef<string>('');

  useEffect(() => {
    const fingerprint = JSON.stringify(providers);
    if (fingerprint === lastSyncedProvidersRef.current) return;
    lastSyncedProvidersRef.current = fingerprint;
    const payload = (providers || []).map(p => ({
      id: p.id,
      name: p.name,
      api_key: p.apiKey || '',
      base_url: p.baseUrl || '',
      enabled: !!p.enabled,
      available_models: p.models || [],
      selected_models: p.selectedModels && p.selectedModels.length > 0 ? p.selectedModels : (p.models || []),
    }));
    SyncAgentProviders(payload).then((merged) => {
      if (Array.isArray(merged)) goProfilesRef.current = merged;
    }).catch(() => { /* non-fatal: Go may be unavailable in dev browser mode */ });
  }, [providers]);

  // activateGoModel points the Go LLMClient at the provider owning `model`
  // before an internal-agent turn, so the harness uses the model shown in the
  // UI (and its API key) rather than a stale default profile.
  const activateGoModel = useCallback(async (model?: string) => {
    const target = (model || '').trim();
    if (!target) return;
    let profiles = goProfilesRef.current;
    if (!profiles || profiles.length === 0) {
      try {
        profiles = await GetProviderProfiles();
        goProfilesRef.current = Array.isArray(profiles) ? profiles : [];
      } catch { return; }
    }
    const owner = (profiles || []).find((p: any) => {
      const id = p?.id ?? p?.Id;
      if (!id) return false;
      const selected = p?.selected_models || p?.SelectedModels || [];
      const available = p?.available_models || p?.AvailableModels || [];
      return selected.includes(target) || available.includes(target);
    });
    const ownerId = owner?.id ?? owner?.Id;
    if (ownerId) {
      try { await SetActiveModel(ownerId, target); } catch { /* ignore */ }
    }
  }, []);

  // Hydrate providers from the Go profile store — the canonical merged view
  // (localStorage providers + Settings-configured ones). Runs after boot sync
  // and whenever the backend config changes, so every app install / window
  // sees providers configured elsewhere (e.g. another app bundle's settings).
  const hydrateProvidersFromGo = useCallback(async () => {
    try {
      const profiles = await GetProviderProfiles();
      if (!Array.isArray(profiles) || profiles.length === 0) return;
      goProfilesRef.current = profiles;
      setProviders(prev => {
        const next = [...prev];
        for (const gp of profiles) {
          const id = gp?.id ?? gp?.Id;
          const url = String(gp?.base_url ?? gp?.BaseUrl ?? '').replace(/\/+$/, '');
          if (!id && !url) continue;
          const mapped: LLMProviderConfig = {
            id: id || `go-${url || Math.random().toString(36).slice(2)}`,
            name: gp?.name ?? gp?.Name ?? id ?? 'Provider',
            apiKey: gp?.api_key ?? gp?.ApiKey ?? '',
            baseUrl: gp?.base_url ?? gp?.BaseUrl ?? '',
            enabled: !!(gp?.enabled ?? gp?.Enabled),
            models: gp?.available_models ?? gp?.AvailableModels ?? [],
            selectedModels: gp?.selected_models ?? gp?.SelectedModels ?? [],
          };
          const idx = next.findIndex(p =>
            (id && p.id === id) || (url && (p.baseUrl || '').replace(/\/+$/, '') === url));
          if (idx >= 0) {
            const merged = { ...next[idx], ...mapped };
            if (JSON.stringify(merged) === JSON.stringify(next[idx])) continue;
            next[idx] = merged;
          } else {
            next.push(mapped);
          }
        }
        return next;
      });
    } catch { /* browser dev mode without Go backend */ }
  }, []);

  useEffect(() => {
    // After the boot sync push has settled, pull the merged view back.
    const t = setTimeout(() => { void hydrateProvidersFromGo(); }, 1200);
    const off = EventsOn('agent:config:changed', () => {
      setTimeout(() => { void hydrateProvidersFromGo(); }, 300);
    });
    return () => { clearTimeout(t); if (typeof off === 'function') off(); };
  }, [hydrateProvidersFromGo]);

  const addModelToProvider = useCallback((providerId: string, modelName: string) => {
    setProviders(prev => prev.map(p => {
      if (p.id === providerId && !p.models.includes(modelName)) {
        const nextModels = [...p.models, modelName];
        const nextSelected = [...(p.selectedModels || p.models), modelName];
        return { ...p, models: nextModels, selectedModels: nextSelected };
      }
      return p;
    }));
  }, []);

  const deleteModelFromProvider = useCallback((providerId: string, modelName: string) => {
    setProviders(prev => prev.map(p => {
      if (p.id === providerId) {
        const nextModels = p.models.filter(m => m !== modelName);
        const nextSelected = (p.selectedModels || p.models).filter(m => m !== modelName);
        return { ...p, models: nextModels, selectedModels: nextSelected };
      }
      return p;
    }));
  }, []);

  const toggleModelSelection = useCallback((providerId: string, modelName: string) => {
    setProviders(prev => prev.map(p => {
      if (p.id === providerId) {
        const currentSelected = p.selectedModels || p.models;
        const isSelected = currentSelected.includes(modelName);
        const nextSelected = isSelected
          ? currentSelected.filter(m => m !== modelName)
          : [...currentSelected, modelName];
        return { ...p, selectedModels: nextSelected };
      }
      return p;
    }));
  }, []);

  const fetchProviderModels = useCallback(async (providerId: string) => {
    const prov = providers.find(p => p.id === providerId);
    if (!prov) return;
    const fetched = await ApiBridge.fetchModels(prov.id, prov.baseUrl, prov.apiKey);
    if (fetched.length > 0) {
      setProviders(prev => prev.map(p => {
        if (p.id === providerId) {
          const merged = Array.from(new Set([...p.models, ...fetched]));
          return { ...p, models: merged, selectedModels: merged };
        }
        return p;
      }));
      // Immediately activate the first fetched model
      setCurrentModel(fetched[0]);
    }
  }, [providers, setCurrentModel]);

  // MCPs state
  const [mcps, setMcps] = useState<MCPEntry[]>(() => {
    try {
      const saved = localStorage.getItem('forge_ade_mcps') || localStorage.getItem('my_ade_mcps');
      if (saved) return JSON.parse(saved);
    } catch {}
    return [
      {
        id: 'mcp-1',
        name: 'Filesystem MCP Server',
        command: 'npx -y @modelcontextprotocol/server-filesystem .',
        status: 'running',
        enabled: true,
        version: 'v0.8.2',
        tools: ['read_file', 'batch_edit', 'grep_search', 'directory_tree']
      },
      {
        id: 'mcp-2',
        name: 'GitHub & Git Tools MCP',
        command: 'npx -y @modelcontextprotocol/server-github',
        status: 'running',
        enabled: true,
        version: 'v1.1.0',
        tools: ['create_pr', 'list_issues', 'get_commit_history', 'view_diff']
      }
    ];
  });

  useEffect(() => {
    localStorage.setItem('forge_ade_mcps', JSON.stringify(mcps));
  }, [mcps]);

  const addMcp = useCallback((mcp: MCPEntry) => {
    setMcps(prev => [...prev, mcp]);
  }, []);

  const toggleMcp = useCallback((id: string) => {
    setMcps(prev => prev.map(m => m.id === id ? { ...m, enabled: !m.enabled } : m));
  }, []);

  const deleteMcp = useCallback((id: string) => {
    setMcps(prev => prev.filter(m => m.id !== id));
  }, []);

  // Skills state
  const [skills, setSkills] = useState<SkillEntry[]>(() => {
    try {
      const saved = localStorage.getItem('forge_ade_skills') || localStorage.getItem('my_ade_skills');
      if (saved) return JSON.parse(saved);
    } catch {}
    return [
      {
        id: 'sk-native',
        name: 'Native SDK Discovery & Authoring',
        category: 'Native SDK',
        description: 'Discovery skill for building native desktop applications with TypeScript cores, Native markup, and Zig engine.',
        enabled: true,
        trigger: 'native-sdk'
      },
      {
        id: 'sk-forge',
        name: 'Forge ADE Architecture',
        category: 'Workspace',
        description: 'High performance editor primitives, unified diff engine, Git graph, LSP diagnostics, and ACP subagent bridges.',
        enabled: true,
        trigger: 'forge-ade'
      },
      {
        id: 'sk-1',
        name: 'Clean Architecture & Refactoring',
        category: 'Code Quality',
        description: 'Auto-detects code smells and optimizes design patterns.',
        enabled: true,
        trigger: 'on_refactor'
      },
      {
        id: 'sk-2',
        name: 'Automated Test Runner',
        category: 'Testing',
        description: 'Runs automated test suites in background sandbox.',
        enabled: true,
        trigger: 'on_test_fail'
      }
    ];
  });

  useEffect(() => {
    localStorage.setItem('my_ade_skills', JSON.stringify(skills));
  }, [skills]);

  const addSkill = useCallback((skill: SkillEntry) => {
    setSkills(prev => [...prev, skill]);
  }, []);

  const toggleSkill = useCallback((id: string) => {
    setSkills(prev => prev.map(s => s.id === id ? { ...s, enabled: !s.enabled } : s));
  }, []);

  const deleteSkill = useCallback((id: string) => {
    setSkills(prev => prev.filter(s => s.id !== id && s.name !== id));
  }, []);

  // Backend Skills synchronization & creation
  const fetchBackendSkills = useCallback(async () => {
    try {
      const backendSkills = await ApiBridge.listSkills();
      if (Array.isArray(backendSkills) && backendSkills.length > 0) {
        setSkills(prev => {
          const map = new Map(prev.map(s => [s.name.toLowerCase(), s]));
          for (const b of backendSkills) {
            const existing = map.get(b.name.toLowerCase());
            map.set(b.name.toLowerCase(), {
              id: existing?.id || b.name,
              name: b.name,
              category: b.category || b.scope || 'Custom',
              description: b.description || '',
              enabled: existing ? existing.enabled : true,
              trigger: b.trigger || b.name,
              instructions: b.prompt || existing?.instructions,
              origin: b.scope || 'workspace'
            });
          }
          return Array.from(map.values());
        });
      }
    } catch (e) {
      console.warn('fetchBackendSkills failed', e);
    }
  }, []);

  const createBackendSkill = useCallback(async (req: CreateSkillRequest) => {
    try {
      const sk = await ApiBridge.createSkill(req);
      await fetchBackendSkills();
      return sk;
    } catch (e) {
      console.error('createBackendSkill failed', e);
      throw e;
    }
  }, [fetchBackendSkills]);

  const deleteBackendSkill = useCallback(async (name: string) => {
    try {
      await ApiBridge.deleteSkill(name);
      setSkills(prev => prev.filter(s => s.name !== name && s.id !== name));
      await fetchBackendSkills();
    } catch (e) {
      console.error('deleteBackendSkill failed', e);
      throw e;
    }
  }, [fetchBackendSkills]);

  const reloadSkills = useCallback(async () => {
    await ApiBridge.reloadSkills();
    await fetchBackendSkills();
  }, [fetchBackendSkills]);

  // Plugins state & operations
  const [plugins, setPlugins] = useState<PluginInfo[]>([]);

  const fetchPlugins = useCallback(async () => {
    try {
      const list = await ApiBridge.listPlugins();
      setPlugins(list);
    } catch (e) {
      console.warn('fetchPlugins failed', e);
    }
  }, []);

  const createPlugin = useCallback(async (req: CreatePluginRequest) => {
    try {
      const p = await ApiBridge.createPlugin(req);
      await fetchPlugins();
      return p;
    } catch (e) {
      console.error('createPlugin failed', e);
      throw e;
    }
  }, [fetchPlugins]);

  const togglePlugin = useCallback(async (id: string, enabled?: boolean) => {
    setPlugins(prev => prev.map(p => p.id === id ? { ...p, enabled: enabled !== undefined ? enabled : !p.enabled } : p));
    const target = plugins.find(p => p.id === id);
    const newEnabled = enabled !== undefined ? enabled : (target ? !target.enabled : true);
    await ApiBridge.togglePlugin(id, newEnabled);
    await fetchPlugins();
  }, [plugins, fetchPlugins]);

  const deletePlugin = useCallback(async (id: string) => {
    setPlugins(prev => prev.filter(p => p.id !== id));
    await ApiBridge.deletePlugin(id);
    await fetchPlugins();
  }, [fetchPlugins]);

  const reloadPlugins = useCallback(async () => {
    const list = await ApiBridge.reloadPlugins();
    setPlugins(list);
  }, []);

  // Initial load and live event subscription for plugins and skills
  useEffect(() => {
    fetchPlugins();
    fetchBackendSkills();

    const unsubPlugins = EventsOn('plugins:changed', () => {
      fetchPlugins();
      fetchBackendSkills();
    });
    const unsubSkills = EventsOn('skills:changed', () => {
      fetchBackendSkills();
    });

    return () => {
      unsubPlugins?.();
      unsubSkills?.();
    };
  }, [fetchPlugins, fetchBackendSkills]);

  // Discovery State (Antigravity, OpenCode, Pi, Claude Code, Codex)
  const [discoveredMcps, setDiscoveredMcps] = useState<any[]>([]);
  const [discoveredSkills, setDiscoveredSkills] = useState<any[]>([]);
  const [isDiscovering, setIsDiscovering] = useState<boolean>(false);

  const runDiscovery = useCallback(async () => {
    setIsDiscovering(true);
    try {
      const [mcpsRes, skillsRes] = await Promise.all([
        ApiBridge.discoverMcps(),
        ApiBridge.discoverSkills()
      ]);
      setDiscoveredMcps(mcpsRes);
      setDiscoveredSkills(skillsRes);
    } finally {
      setIsDiscovering(false);
    }
  }, []);

  // Run discovery on mount
  useEffect(() => {
    runDiscovery();
  }, [runDiscovery]);

  const importDiscoveredMcp = useCallback((disc: any) => {
    setMcps(prev => {
      const exists = prev.some(m => m.name === disc.name || m.command === disc.command);
      if (exists) return prev;
      return [...prev, {
        id: `mcp-${Date.now()}`,
        name: disc.name,
        command: disc.command,
        status: 'connected',
        enabled: true,
        origin: disc.origin,
        tools: disc.tools || ['tools']
      }];
    });
  }, []);

  const importDiscoveredSkill = useCallback((disc: any) => {
    setSkills(prev => {
      const exists = prev.some(s => s.trigger === disc.trigger || s.name === disc.name);
      if (exists) return prev;
      return [...prev, {
        id: `skill-${Date.now()}`,
        name: disc.name,
        category: disc.category || 'Discovered',
        description: disc.description,
        trigger: disc.trigger,
        instructions: disc.instructions,
        enabled: true,
        origin: disc.origin
      }];
    });
  }, []);

  // ── Agent Long-Term Memory ──────────────────────────────────────────────────
  const [memories, setMemories] = useState<AgentMemoryEntry[]>([]);

  const fetchMemories = useCallback(async () => {
    try {
      const list = await ApiBridge.listMemories();
      setMemories(list);
    } catch (e) {
      console.warn('fetchMemories failed', e);
    }
  }, []);

  const saveMemory = useCallback(async (entry: AgentMemoryEntry) => {
    try {
      await ApiBridge.saveMemory(entry);
      await fetchMemories();
    } catch (e) {
      console.error('saveMemory failed', e);
      throw e;
    }
  }, [fetchMemories]);

  const deleteMemory = useCallback(async (id: string) => {
    try {
      await ApiBridge.deleteMemory(id);
      setMemories(prev => prev.filter(m => m.id !== id));
      await fetchMemories();
    } catch (e) {
      console.error('deleteMemory failed', e);
      throw e;
    }
  }, [fetchMemories]);

  const reloadMemories = useCallback(async () => {
    const list = await ApiBridge.reloadMemories();
    setMemories(list);
  }, []);

  useEffect(() => {
    fetchMemories();
    const unsub = EventsOn('memory:changed', () => {
      fetchMemories();
    });
    return () => {
      unsub?.();
    };
  }, [fetchMemories]);

  // ── Automations (saved prompt workflows) ────────────────────────────────────
  const [automations, setAutomations] = useState<ZAutomation[]>([]);
  const [automationsLoading, setAutomationsLoading] = useState<boolean>(false);

  const fetchAutomations = useCallback(async () => {
    setAutomationsLoading(true);
    try {
      const list = await ApiBridge.listAutomations();
      setAutomations(list);
    } catch (e) {
      console.warn('fetchAutomations failed', e);
    } finally {
      setAutomationsLoading(false);
    }
  }, []);

  const saveAutomation = useCallback(async (automation: ZAutomationSaveInput) => {
    const saved = await ApiBridge.saveAutomation(automation);
    await fetchAutomations();
    return saved;
  }, [fetchAutomations]);

  const deleteAutomation = useCallback(async (id: string) => {
    await ApiBridge.deleteAutomation(id);
    await fetchAutomations();
  }, [fetchAutomations]);

  const setAutomationEnabled = useCallback(async (id: string, enabled: boolean) => {
    await ApiBridge.setAutomationEnabled(id, enabled);
    await fetchAutomations();
  }, [fetchAutomations]);

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

  const createNewSessionRef = useRef<(initialPrompt?: string, selectedAgentId?: string, forcedId?: string) => void>(null);

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

  // ── Custom Slash Commands ───────────────────────────────────────────────────
  const [customCommands, setCustomCommands] = useState<CustomSlashCommand[]>(() => {
    try {
      const saved = localStorage.getItem('forge_ade_custom_commands');
      if (saved) return JSON.parse(saved);
    } catch {}
    return DEFAULT_SLASH_COMMANDS;
  });

  useEffect(() => {
    try {
      localStorage.setItem('forge_ade_custom_commands', JSON.stringify(customCommands));
    } catch {}
  }, [customCommands]);

  const addCustomCommand = useCallback((cmd: CustomSlashCommand) => {
    setCustomCommands(prev => [...prev.filter(c => c.id !== cmd.id && c.name !== cmd.name), cmd]);
  }, []);

  const updateCustomCommand = useCallback((id: string, updates: Partial<CustomSlashCommand>) => {
    setCustomCommands(prev => prev.map(c => c.id === id ? { ...c, ...updates } : c));
  }, []);

  const deleteCustomCommand = useCallback((id: string) => {
    setCustomCommands(prev => prev.filter(c => c.id !== id));
  }, []);

  const toggleCustomCommand = useCallback((id: string) => {
    setCustomCommands(prev => prev.map(c => c.id === id ? { ...c, enabled: !c.enabled } : c));
  }, []);

  // ── Lifecycle Hooks ─────────────────────────────────────────────────────────
  const [hooks, setHooks] = useState<AgentHookConfig[]>(() => {
    try {
      const saved = localStorage.getItem('forge_ade_hooks');
      if (saved) return JSON.parse(saved);
    } catch {}
    return DEFAULT_HOOKS;
  });

  useEffect(() => {
    try {
      localStorage.setItem('forge_ade_hooks', JSON.stringify(hooks));
    } catch {}
  }, [hooks]);

  const updateHook = useCallback((id: string, updates: Partial<AgentHookConfig>) => {
    setHooks(prev => prev.map(h => h.id === id ? { ...h, ...updates } : h));
  }, []);

  const toggleHook = useCallback((id: string) => {
    setHooks(prev => prev.map(h => h.id === id ? { ...h, enabled: !h.enabled } : h));
  }, []);

  const addHook = useCallback((hook: AgentHookConfig) => {
    setHooks(prev => [...prev.filter(h => h.id !== hook.id), hook]);
  }, []);

  const deleteHook = useCallback((id: string) => {
    setHooks(prev => prev.filter(h => h.id !== id));
  }, []);

  // ── Browser Use Settings ────────────────────────────────────────────────────
  const [browserSettings, setBrowserSettings] = useState<BrowserUseSettings>(() => {
    try {
      const saved = localStorage.getItem('forge_ade_browser_settings');
      if (saved) return JSON.parse(saved);
    } catch {}
    return DEFAULT_BROWSER_SETTINGS;
  });

  useEffect(() => {
    try {
      localStorage.setItem('forge_ade_browser_settings', JSON.stringify(browserSettings));
    } catch {}
  }, [browserSettings]);

  const updateBrowserSettings = useCallback((updates: Partial<BrowserUseSettings>) => {
    setBrowserSettings(prev => ({ ...prev, ...updates }));
  }, []);

  // ── Computer Use Settings ───────────────────────────────────────────────────
  const [computerSettings, setComputerSettings] = useState<ComputerUseSettings>(() => {
    try {
      const saved = localStorage.getItem('forge_ade_computer_settings');
      if (saved) return JSON.parse(saved);
    } catch {}
    return DEFAULT_COMPUTER_SETTINGS;
  });

  useEffect(() => {
    try {
      localStorage.setItem('forge_ade_computer_settings', JSON.stringify(computerSettings));
    } catch {}
  }, [computerSettings]);

  const updateComputerSettings = useCallback((updates: Partial<ComputerUseSettings>) => {
    setComputerSettings(prev => ({ ...prev, ...updates }));
  }, []);

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

  // Diffs & Review System
  const [diffs, setDiffs] = useState<FileDiff[]>([]);
  const [activeDiff, setActiveDiff] = useState<FileDiff | null>(null);

  const addDiff = useCallback((diff: FileDiff) => {
    setDiffs(prev => [diff, ...prev.filter(d => d.filePath !== diff.filePath)]);
  }, []);

  // Git State
  const [gitBranch, setGitBranch] = useState<string>('');
  const [gitFiles, setGitFiles] = useState<Array<{ path: string; status: string; staging?: 'staged' | 'unstaged' | 'untracked'; dir?: string }>>([]);
  const [gitCommits, setGitCommits] = useState<any[]>([]);

  const refreshGitStatus = useCallback(async () => {
    const result = await ApiBridge.gitStatus(activeWorkspacePath);
    setGitBranch(result.branch);
    setGitFiles(result.files);
  }, [activeWorkspacePath]);

  const refreshGitLog = useCallback(async () => {
    const commits = await ApiBridge.gitLog(activeWorkspacePath);
    setGitCommits(commits);
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

  // Navigation & Activities
  const [activeActivity, setActiveActivity] = useState<ActivityBarItem>('explorer');

  // Quick Action Drawer (Default collapsed per user specification)
  const [isRightActionDrawerOpen, setIsRightActionDrawerOpen] = useState<boolean>(false);

  // Bottom dock terminal
  const [isBottomTerminalOpen, setIsBottomTerminalOpen] = useState<boolean>(false);

  // Left sidebar view mode + right-pane file tabs (ZCode explorer pattern)
  const [leftSidebarView, setLeftSidebarView] = useState<'tasks' | 'files'>('tasks');
  const [sideFileTabs, setSideFileTabs] = useState<Array<{ path: string; name: string; line?: number }>>([]);
  const [activeSideFile, setActiveSideFile] = useState<string | null>(null);
  const [activeSideFileLine, setActiveSideFileLine] = useState<number | undefined>(undefined);

  const openSideFile = useCallback((rawPath: string, line?: number) => {
    const parsed = parseFilePath(rawPath, activeWorkspacePath);
    const path = parsed.cleanPath || rawPath;
    const targetLine = line !== undefined ? line : parsed.line;
    const name = parsed.displayName;

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
  }, [activeWorkspacePath, setIsRightActionDrawerOpen]);

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

  // Modals & Palette
  const [isCommandPaletteOpen, setIsCommandPaletteOpen] = useState<boolean>(false);
  // Automations render as a modal overlay (not a surface mode), so they sit
  // on top of whatever task is active and never fight the CLI/forge views.
  const [isAutomationsOpen, setIsAutomationsOpen] = useState<boolean>(false);
  const [commandPaletteMode, setCommandPaletteMode] = useState<CommandPaletteMode>('files');
  const openCommandPalette = useCallback((mode: CommandPaletteMode = 'files') => {
    setCommandPaletteMode(mode);
    setIsCommandPaletteOpen(true);
  }, []);
  const [isCustomizationsModalOpen, setIsCustomizationsModalOpen] = useState<boolean>(false);
  const [isSettingsModalOpen, setIsSettingsModalOpen] = useState<boolean>(false);

  // Single Pane Settings Section
  const [settingsActiveSection, setSettingsActiveSection] = useState<string>('general');

  // Agent execution mode & Engine
  const [agentExecutionMode, setAgentExecutionMode] = useState<AgentExecutionMode>('bypass');
  const [reasoningLevel, setReasoningLevelState] = useState<AgentReasoningLevel>(() => {
    return (localStorage.getItem('forge_ade_reasoning_level') as AgentReasoningLevel) || 'max';
  });
  const setReasoningLevel = useCallback((lvl: AgentReasoningLevel) => {
    setReasoningLevelState(lvl);
    localStorage.setItem('forge_ade_reasoning_level', lvl);
  }, []);

  const [agentEngine, setAgentEngine] = useState<AgentEngineType>('pi');

  // Sidebars
  const [isLeftSidebarOpen, setIsLeftSidebarOpen] = useState<boolean>(true);

  // ── Panel widths (ZCode WorkspaceShellLayout semantics) ─────────────────────
  // Drag-resizable sidebars with localStorage persistence. Max is evaluated at
  // resize time so a window resize re-clamps naturally on the next drag.
  const LEFT_SIDEBAR_MIN_WIDTH = 240;
  const LEFT_SIDEBAR_MAX_RATIO = 0.5;
  const RIGHT_PANE_MIN_WIDTH = 360;
  const RIGHT_PANE_MAX_RATIO = 0.6;

  const clampLeftSidebarWidth = useCallback((width: number) => {
    const maxWidth = Math.max(LEFT_SIDEBAR_MIN_WIDTH, Math.floor(window.innerWidth * LEFT_SIDEBAR_MAX_RATIO));
    return Math.round(Math.min(Math.max(width, LEFT_SIDEBAR_MIN_WIDTH), maxWidth));
  }, []);

  const [leftSidebarWidth, setLeftSidebarWidthState] = useState<number>(() => {
    try {
      const saved = Number(localStorage.getItem('forge_ade_left_sidebar_width'));
      if (Number.isFinite(saved) && saved >= LEFT_SIDEBAR_MIN_WIDTH) return saved;
    } catch {}
    return 360; // ZCode-parity default width
  });

  const setLeftSidebarWidth = useCallback((width: number) => {
    const clamped = clampLeftSidebarWidth(width);
    setLeftSidebarWidthState(clamped);
    try { localStorage.setItem('forge_ade_left_sidebar_width', String(clamped)); } catch {}
  }, [clampLeftSidebarWidth]);

  const clampRightPaneWidth = useCallback((width: number) => {
    const maxWidth = Math.max(RIGHT_PANE_MIN_WIDTH, Math.floor(window.innerWidth * RIGHT_PANE_MAX_RATIO));
    return Math.round(Math.min(Math.max(width, RIGHT_PANE_MIN_WIDTH), maxWidth));
  }, []);

  const [rightPaneWidth, setRightPaneWidthState] = useState<number>(() => {
    try {
      const saved = Number(localStorage.getItem('forge_ade_side_pane_width'));
      if (Number.isFinite(saved) && saved >= RIGHT_PANE_MIN_WIDTH) return saved;
    } catch {}
    return 540;
  });

  const setRightPaneWidth = useCallback((width: number) => {
    const clamped = clampRightPaneWidth(width);
    setRightPaneWidthState(clamped);
    try { localStorage.setItem('forge_ade_side_pane_width', String(clamped)); } catch {}
  }, [clampRightPaneWidth]);

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
    // Provider-reported usage is authoritative and free — skip the full
    // transcript scan (which re-runs ~16x/sec during streaming otherwise).
    if (activeSession?.contextTokens && activeSession.contextTokens > 0) {
      const reported = activeSession.contextTokens;
      const modelLowerFast = (activeSession.model || currentModel || '').toLowerCase();
      let maxFast = 1000000;
      if (modelLowerFast.includes('2m')) maxFast = 2000000;
      else if (modelLowerFast.includes('200k') || modelLowerFast.includes('claude')) maxFast = 200000;
      else if (modelLowerFast.includes('128k') || modelLowerFast.includes('gpt-4') || modelLowerFast.includes('o1') || modelLowerFast.includes('o3') || modelLowerFast.includes('deepseek')) maxFast = 128000;
      else if (modelLowerFast.includes('64k')) maxFast = 64000;
      else if (modelLowerFast.includes('32k')) maxFast = 32000;
      const pctFast = Math.min(100, Math.max(0, (reported / maxFast) * 100));
      const fmtUsed = reported >= 1000000 ? `${(reported / 1000000).toFixed(2)}M` : reported >= 1000 ? `${(reported / 1000).toFixed(1)}K` : `${reported}`;
      const fmtMax = maxFast >= 1000000 ? `${(maxFast / 1000000).toFixed(0)}M` : `${Math.round(maxFast / 1000)}K`;
      const catAll = { tokens: reported, percent: pctFast, formattedPercent: pctFast < 0.1 && pctFast > 0 ? '<0.1%' : `${pctFast.toFixed(1)}%` };
      const catZero = { tokens: 0, percent: 0, formattedPercent: '0%' };
      return {
        usedTokens: reported,
        maxTokens: maxFast,
        percent: pctFast,
        formattedUsed: fmtUsed,
        formattedMax: fmtMax,
        categories: {
          messages: catAll,
          mcpTools: catZero,
          systemTools: catZero,
          systemPrompt: catZero,
          skills: catZero,
          metaContext: catZero
        }
      };
    }

    // Max tokens based on the selected model
    const modelLower = (activeSession?.model || currentModel || '').toLowerCase();
    let maxTokens = 1000000;
    if (modelLower.includes('2m')) maxTokens = 2000000;
    else if (modelLower.includes('200k') || modelLower.includes('claude')) maxTokens = 200000;
    else if (modelLower.includes('128k') || modelLower.includes('gpt-4') || modelLower.includes('o1') || modelLower.includes('o3') || modelLower.includes('deepseek')) maxTokens = 128000;
    else if (modelLower.includes('64k')) maxTokens = 64000;
    else if (modelLower.includes('32k')) maxTokens = 32000;

    // Real context size: the provider-reported prompt tokens of the last
    // request, surfaced by the backend as session.contextTokens. Without it
    // we fall back to the transcript size only — a brand-new chat shows 0,
    // never a static estimate.
    let usedTokens = 0;
    if (activeSession?.contextTokens && activeSession.contextTokens > 0) {
      usedTokens = activeSession.contextTokens;
    } else if (activeSession?.messages) {
      let chars = 0;
      for (const m of activeSession.messages) {
        chars += (m.content?.length || 0);
        for (const th of m.thoughts || []) chars += (th.thoughtText?.length || 0);
        for (const te of m.toolExecutions || []) chars += (te.command?.length || 0) + (te.output?.length || 0);
      }
      for (const d of activeSession.diffs || []) {
        chars += (d.originalContent?.length || 0) + (d.modifiedContent?.length || 0);
      }
      usedTokens = Math.round(chars / 4);
    }

    const percent = Math.min(100, Math.max(0, (usedTokens / maxTokens) * 100));
    const formattedUsed = usedTokens >= 1000000
      ? `${(usedTokens / 1000000).toFixed(2)}M`
      : usedTokens >= 1000 ? `${(usedTokens / 1000).toFixed(1)}K` : `${usedTokens}`;
    const formattedMax = maxTokens >= 1000000 ? `${(maxTokens / 1000000).toFixed(0)}M` : `${Math.round(maxTokens / 1000)}K`;

    const calcCat = (toks: number) => {
      const pct = usedTokens > 0 ? (toks / usedTokens) * 100 : 0;
      return { tokens: toks, percent: pct, formattedPercent: toks === 0 ? '0%' : pct < 0.1 ? '<0.1%' : `${pct.toFixed(1)}%` };
    };

    return {
      usedTokens,
      maxTokens,
      percent,
      formattedUsed,
      formattedMax,
      categories: {
        messages: calcCat(usedTokens),
        mcpTools: calcCat(0),
        systemTools: calcCat(0),
        systemPrompt: calcCat(0),
        skills: calcCat(0),
        metaContext: calcCat(0)
      }
    };
  }, [activeSession, currentModel]);

  // Diagnostics (LSP)
  const [diagnostics, setDiagnostics] = useState<LSPDiagnostic[]>([]);

  const engineRef = useRef<AgentEngine>(new AgentEngine());

  const sendSideConversationPrompt = useCallback(async (promptText: string, model?: string, mode?: 'ask' | 'plan' | 'full') => {
    if (!promptText.trim() || !activeSessionId) return;

    const userMsg: AgentMessage = {
      id: `side-user-${Date.now()}`,
      role: 'user',
      content: promptText,
      timestamp: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })
    };

    const agentMsgId = `side-agent-${Date.now()}`;
    const agentMsg: AgentMessage = {
      id: agentMsgId,
      role: 'agent',
      content: '',
      timestamp: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
      isThinking: true
    };

    setSessions(prev => prev.map(s => {
      if (s.id !== activeSessionId) return s;
      const sideMsgs = s.sideConversationMessages || [];
      return {
        ...s,
        sideConversationMessages: [...sideMsgs, userMsg, agentMsg]
      };
    }));

    try {
      const activeSessionObj = sessions.find(s => s.id === activeSessionId);
      const conversationHistory = (activeSessionObj?.sideConversationMessages || []).map(m => ({
        role: m.role === 'user' ? 'user' : 'assistant',
        content: m.content
      }));
      conversationHistory.push({ role: 'user', content: promptText });

      let accumulated = '';
      const sessionDiffs = activeSessionObj?.diffs || diffs || [];
      await engineRef.current.streamSideChat(
        model || currentModel,
        conversationHistory,
        {
          workspacePath: activeWorkspacePath || '',
          activeTaskTitle: activeSessionObj?.title,
          diffs: sessionDiffs,
          gitFiles,
          gitBranch
        },
        {
          onChunk: (deltaChunk: string) => {
            accumulated += deltaChunk;
            setSessions(prev => prev.map(s => {
              if (s.id !== activeSessionId) return s;
              const sideMsgs = [...(s.sideConversationMessages || [])];
              const targetIdx = sideMsgs.findIndex(m => m.id === agentMsgId);
              if (targetIdx !== -1) {
                sideMsgs[targetIdx] = {
                  ...sideMsgs[targetIdx],
                  content: accumulated,
                  isThinking: false,
                  toolStatus: undefined
                };
              }
              return { ...s, sideConversationMessages: sideMsgs };
            }));
          },
          onThought: (thought: ThoughtStep) => {
            setSessions(prev => prev.map(s => {
              if (s.id !== activeSessionId) return s;
              const sideMsgs = [...(s.sideConversationMessages || [])];
              const targetIdx = sideMsgs.findIndex(m => m.id === agentMsgId);
              if (targetIdx !== -1) {
                const thoughts = sideMsgs[targetIdx].thoughts || [];
                const existingIdx = thoughts.findIndex(t => t.id === thought.id);
                const updatedThoughts = existingIdx >= 0
                  ? thoughts.map((t, i) => i === existingIdx ? { ...t, ...thought } : t)
                  : [...thoughts, thought];
                sideMsgs[targetIdx] = {
                  ...sideMsgs[targetIdx],
                  thoughts: updatedThoughts,
                  isThinking: true
                };
              }
              return { ...s, sideConversationMessages: sideMsgs };
            }));
          },
          onToolStart: (tool: ToolExecution) => {
            setSessions(prev => prev.map(s => {
              if (s.id !== activeSessionId) return s;
              const sideMsgs = [...(s.sideConversationMessages || [])];
              const targetIdx = sideMsgs.findIndex(m => m.id === agentMsgId);
              if (targetIdx !== -1) {
                const tools = sideMsgs[targetIdx].toolExecutions || [];
                const existingIdx = tools.findIndex(t => t.id === tool.id);
                const updatedTools = existingIdx >= 0
                  ? tools.map((t, i) => i === existingIdx ? { ...t, ...tool } : t)
                  : [...tools, tool];
                sideMsgs[targetIdx] = {
                  ...sideMsgs[targetIdx],
                  toolExecutions: updatedTools,
                  isThinking: false
                };
              }
              return { ...s, sideConversationMessages: sideMsgs };
            }));
          },
          onToolComplete: (tool: ToolExecution) => {
            setSessions(prev => prev.map(s => {
              if (s.id !== activeSessionId) return s;
              const sideMsgs = [...(s.sideConversationMessages || [])];
              const targetIdx = sideMsgs.findIndex(m => m.id === agentMsgId);
              if (targetIdx !== -1) {
                const tools = sideMsgs[targetIdx].toolExecutions || [];
                const updated = tools.some(t => t.id === tool.id)
                  ? tools.map(t => t.id === tool.id ? { ...t, ...tool } : t)
                  : [...tools, tool];
                sideMsgs[targetIdx] = {
                  ...sideMsgs[targetIdx],
                  toolExecutions: updated
                };
              }
              return { ...s, sideConversationMessages: sideMsgs };
            }));
          },
          onToolStatus: (toolStatus: string) => {
            setSessions(prev => prev.map(s => {
              if (s.id !== activeSessionId) return s;
              const sideMsgs = [...(s.sideConversationMessages || [])];
              const targetIdx = sideMsgs.findIndex(m => m.id === agentMsgId);
              if (targetIdx !== -1) {
                sideMsgs[targetIdx] = {
                  ...sideMsgs[targetIdx],
                  toolStatus: toolStatus || undefined,
                  isThinking: !!toolStatus
                };
              }
              return { ...s, sideConversationMessages: sideMsgs };
            }));
          }
        },
        undefined,
        mode || 'full'
      );
    } catch (e: any) {
      console.error('Side conversation streaming error:', e);
      const errorMessage = `⚠️ Error: ${e.message || 'Connection failed'}`;
      setSessions(prev => prev.map(s => {
        if (s.id !== activeSessionId) return s;
        const sideMsgs = [...(s.sideConversationMessages || [])];
        const targetIdx = sideMsgs.findIndex(m => m.id === agentMsgId);
        if (targetIdx !== -1) {
          sideMsgs[targetIdx] = {
            ...sideMsgs[targetIdx],
            content: errorMessage,
            isThinking: false
          };
        }
        return { ...s, sideConversationMessages: sideMsgs };
      }));
    }
  }, [activeSessionId, activeWorkspacePath, currentModel, sessions]);

  const clearSideConversation = useCallback(() => {
    if (!activeSessionId) return;
    setSessions(prev => prev.map(s => s.id === activeSessionId ? { ...s, sideConversationMessages: [] } : s));
  }, [activeSessionId]);

  // Sync theme
  useEffect(() => {
    localStorage.setItem('forge-ade-theme', theme);
    localStorage.setItem('forge_ade_theme', theme);
    useUIStore.getState().setTheme(theme);
    if (theme === 'dark') {
      document.documentElement.classList.add('dark');
      document.documentElement.classList.remove('light');
      document.body.classList.add('dark');
      document.body.classList.remove('light');
    } else {
      document.documentElement.classList.remove('dark');
      document.documentElement.classList.add('light');
      document.body.classList.remove('dark');
      document.body.classList.add('light');
    }
  }, [theme]);

  const toggleTheme = useCallback(() => {
    setTheme(prev => (prev === 'dark' ? 'light' : 'dark'));
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
      // If switching to a workspace that has cached tabs/state, do not wipe them
      const cached = workspaceStateCacheRef.current.get(targetPath);
      if (!cached) {
        setOpenTabs([]);
        setActiveTabId(null);
        setSelectedFile(null);
        setDiffs([]);
        setActiveDiff(null);
        setIsSplitEditor(false);
        setNav({ stack: [], index: -1 });
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

    return () => {
      if (typeof unsub === 'function') unsub();
      if (gitRefreshTimer !== null) window.clearTimeout(gitRefreshTimer);
    };
  }, [refreshGitStatus]);

  // Helper to find file by path in tree
  const findFileInTree = useCallback((items: FileItem[], path: string): FileItem | null => {
    const clean = path.replace(/^\/+/, '');
    for (const item of items) {
      const itemClean = item.path.replace(/^\/+/, '');
      if (itemClean === clean || item.path === path || item.name === path || item.path.endsWith(clean)) {
        return item;
      }
      if (item.children) {
        const found = findFileInTree(item.children, path);
        if (found) return found;
      }
    }
    return null;
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
    // Diff tabs are transient views, not navigable documents — keep them out
    // of the back/forward history so it only ever contains real files.
    if (tab.type === 'diff') return;
    setNav(prev => {
      const stack = prev.stack.slice(0, prev.index + 1);
      const top = stack[stack.length - 1];
      // Nav snapshots never carry buffer content: 50 entries × full file text
      // would pin every visited file in memory for the session's lifetime
      // even after its tab is closed. Revival re-reads from disk instead.
      const snapshot: EditorTab = { ...tab, content: '' };
      if (top && top.id === tab.id) {
        stack[stack.length - 1] = snapshot;
        return { stack, index: stack.length - 1 };
      }
      stack.push(snapshot);
      const capped = stack.length > 50 ? stack.slice(stack.length - 50) : stack;
      return { stack: capped, index: capped.length - 1 };
    });
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
    if (nav.index <= 0) return;
    const target = nav.stack[nav.index - 1];
    setNav(prev => ({ ...prev, index: prev.index - 1 }));
    restoreNavTab(target);
  }, [nav, restoreNavTab]);

  const goForward = useCallback(() => {
    if (nav.index >= nav.stack.length - 1) return;
    const target = nav.stack[nav.index + 1];
    setNav(prev => ({ ...prev, index: prev.index + 1 }));
    restoreNavTab(target);
  }, [nav, restoreNavTab]);

  // Ctrl+Tab / Ctrl+Shift+Tab — cycle through open tabs in list order.
  const cycleTab = useCallback((dir: 1 | -1) => {
    const tabs = openTabsRef.current;
    if (tabs.length < 2) return;
    const cur = tabs.findIndex(t => t.id === activeTabIdRef.current);
    const next = tabs[((cur === -1 ? 0 : cur) + dir + tabs.length) % tabs.length];
    openTab(next);
  }, [openTab]);

  const openFileInEditor = useCallback(async (filePath: string, line?: number, column?: number, opts?: { silent?: boolean }) => {
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

    if (!content) {
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
  }, [files, findFileInTree, openTabs]);

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
  }, [openTabs, activeTabId]);

  const updateFolderChildren = useCallback((folderPath: string, children: FileItem[]) => {
    setFiles(prevFiles => {
      const updateNode = (items: FileItem[]): FileItem[] => {
        return items.map(item => {
          if (item.path === folderPath || item.id === folderPath || item.name === folderPath) {
            return { ...item, children };
          }
          if (item.children) {
            return { ...item, children: updateNode(item.children) };
          }
          return item;
        });
      };
      return updateNode(prevFiles);
    });
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
    const existing = openTabs.find(t => t.id === diffTabId);
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
  }, [openTabs]);

  // Full git graph pane in the editor area (lanes + commit detail).
  const openGitGraphPane = useCallback(() => {
    const graphTabId = 'tab-git-graph';
    const existing = openTabs.find(t => t.id === graphTabId);
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
  }, [openTabs, openTab]);

  // Mixed editor terminal tab. `cwd` overrides the workspace folder so tab
  // menus can open a shell right next to a file.
  const openTerminalTab = useCallback(async (shellId?: string, cwd?: string) => {
    let sid = shellId;
    if (!sid) {
      try {
        // ssh:// paths route to a remote shell on the backend; empty folder
        // falls back to the default local folder.
        const sess = await CreateShell(`zsh (${openTabs.filter(t => t.type === 'terminal').length + 1})`, cwd || activeWorkspacePath || '');
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
  }, [openTabs, activeWorkspacePath, openTab]);

  // Closing a tab removes it completely: when the active tab is closed the
  // neighbour (right, else left) becomes active; closing the last tab leaves
  // NO active tab so the pane shows its empty state instead of a phantom file.
  const closeTab = useCallback((tabId: string) => {
    const idx = openTabs.findIndex(t => t.id === tabId);
    if (idx === -1) return;
    const nextTabs = openTabs.filter(t => t.id !== tabId);
    setOpenTabs(nextTabs);
    if (activeTabId === tabId) {
      const neighbor = nextTabs[Math.min(idx, nextTabs.length - 1)];
      setActiveTabId(neighbor ? neighbor.id : null);
    }
  }, [openTabs, activeTabId]);

  // Bulk close for tab-menu actions (Close Others/Left/Right/All). One
  // functional update — calling closeTab in a loop would run on a stale
  // snapshot and only the last removal would stick.
  const closeTabs = useCallback((ids: string[]) => {
    if (ids.length === 0) return;
    const idSet = new Set(ids);
    setOpenTabs(prev => prev.filter(t => !idSet.has(t.id)));
    setActiveTabId(cur => (cur && idSet.has(cur) ? null : cur));
  }, []);

  const updateFileContent = useCallback(async (fileId: string, newContent: string) => {
    const targetTab = openTabs.find(t => t.fileId === fileId || t.filePath === fileId);
    const targetPath = targetTab ? targetTab.filePath : fileId;

    await ApiBridge.writeFile(targetPath, newContent);

    const updateRecursive = (items: FileItem[]): FileItem[] => {
      return items.map(item => {
        if (item.id === fileId || item.path === fileId || item.path === targetPath) {
          return { ...item, content: newContent, isModified: true };
        }
        if (item.children) {
          return { ...item, children: updateRecursive(item.children) };
        }
        return item;
      });
    };

    setFiles(prev => updateRecursive(prev));
    setOpenTabs(prev => prev.map(t => {
      if (t.fileId === fileId || t.filePath === fileId || t.filePath === targetPath) {
        return { ...t, content: newContent };
      }
      return t;
    }));
  }, [openTabs]);

  // Editor typing: buffer-only. The per-keystroke disk write made typing a
  // disk I/O hot path; the write happens on explicit save instead.
  const setTabBuffer = useCallback((fileId: string, content: string) => {
    setOpenTabs(prev => prev.map(t =>
      (t.fileId === fileId || t.filePath === fileId)
        ? (t.isReadOnly ? t : { ...t, content, isModified: true })
        : t
    ));
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
    const targetTab = openTabs.find(t => t.fileId === fileId || t.filePath === fileId);
    const targetPath = targetTab ? targetTab.filePath : fileId;
    await ApiBridge.writeFile(targetPath, content);
    setOpenTabs(prev => prev.map(t =>
      (t.fileId === fileId || t.filePath === fileId || t.filePath === targetPath)
        ? { ...t, content, isModified: false }
        : t
    ));
    void refreshGitStatus();
  }, [openTabs, refreshGitStatus]);

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
    const fullPath = filePath.startsWith('/') || filePath.includes(':') 
      ? filePath 
      : `${activeWorkspacePath}/${filePath.replace(/^\/+/, '')}`;

    await ApiBridge.createFile(fullPath, content);
    await revalidateFileMutations();
    await openFileInEditor(fullPath);
  }, [activeWorkspacePath, revalidateFileMutations, openFileInEditor]);

  const createFolder = useCallback(async (folderPath: string) => {
    const fullPath = folderPath.startsWith('/') || folderPath.includes(':') 
      ? folderPath 
      : `${activeWorkspacePath}/${folderPath.replace(/^\/+/, '')}`;

    await ApiBridge.createFile(fullPath, '');
    await revalidateFileMutations();
  }, [activeWorkspacePath, revalidateFileMutations]);

  const deleteFile = useCallback(async (filePath: string) => {
    await ApiBridge.deleteFile(filePath);
    setOpenTabs(prev => prev.filter(t => t.filePath !== filePath && !t.filePath.startsWith(filePath + '/')));
    closeSideFile(filePath);
    await revalidateFileMutations();
  }, [closeSideFile, revalidateFileMutations]);

  const renameFile = useCallback(async (oldPath: string, newPath: string) => {
    await ApiBridge.renameFile(oldPath, newPath);
    const newName = newPath.split('/').pop() || newPath;
    setOpenTabs(prev => prev.map(t => {
      if (t.filePath === oldPath) {
        return { ...t, filePath: newPath, fileName: newName };
      }
      if (t.filePath.startsWith(oldPath + '/')) {
        const sub = t.filePath.slice(oldPath.length);
        return { ...t, filePath: `${newPath}${sub}` };
      }
      return t;
    }));
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
    setOpenTabs(prev => prev.map(t => {
      if (t.filePath === srcPath) {
        return { ...t, filePath: destPath, fileName: newName };
      }
      if (t.filePath.startsWith(srcPath + '/')) {
        const sub = t.filePath.slice(srcPath.length);
        return { ...t, filePath: `${destPath}${sub}` };
      }
      return t;
    }));
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
    const targetDiff = diffs.find(d => d.id === diffId);
    // Git review diffs carry no proposed content (they show what is already on
    // disk) — writing their empty modifiedContent would blank the file.
    if (!targetDiff || !targetDiff.modifiedContent) return;

    await ApiBridge.writeFile(targetDiff.filePath, targetDiff.modifiedContent);
    await refreshFiles();

    setDiffs(prev => prev.map(d => d.id === diffId ? { ...d, status: 'accepted' } : d));
  }, [diffs, refreshFiles]);

  const rejectDiff = useCallback((diffId: string) => {
    setDiffs(prev => prev.map(d => d.id === diffId ? { ...d, status: 'rejected' } : d));
  }, []);

  // ---------------------------------------------------------------------------
  // Go agent harness path — internal agents run through the Go backend loop
  // (native tool calling, disk-applied edit/write tools, approval gate) like
  // the harness reference, instead of the webview LLM loop.
  // ---------------------------------------------------------------------------

  // Builds the event callbacks that map harness events into a store session's
  // last agent message. Registered once per Go session id.
  const makeGoCallbacks = useCallback((storeSessionId: string, goSessionId: string) => {
    let currentThought: ThoughtStep | null = null;
    // Text/thought deltas arrive per token; writing each into state re-rendered
    // the whole app per token. Coalesce them into one flush per ~60ms window.
    let pendingText = '';
    let thoughtDirty = false;
    let flushTimer: number | null = null;

    const flushPending = () => {
      flushTimer = null;
      const chunk = pendingText;
      const step = thoughtDirty ? currentThought : null;
      if (!chunk && !step) return;
      pendingText = '';
      thoughtDirty = false;
      updateLastAgentMsg(m => {
        const next = { ...m };
        if (chunk) {
          next.content = (next.content || '') + chunk;
          next.isThinking = false;
        }
        if (step) {
          const thoughts = (next.thoughts || []).filter(t => t.id !== step.id).concat(step);
          next.thoughts = thoughts;
          next.isThinking = true;
        }
        return next;
      });
    };

    const scheduleFlush = () => {
      if (flushTimer === null) {
        flushTimer = window.setTimeout(flushPending, 60);
      }
    };

    const updateLastAgentMsg = (fn: (m: AgentMessage) => AgentMessage) => {
      setSessions(prev => prev.map(s => {
        if (s.id !== storeSessionId) return s;
        const msgs = s.messages.map((m, idx) => {
          if (idx === s.messages.length - 1 && m.role === 'agent') return fn(m);
          return m;
        });
        return { ...s, messages: msgs };
      }));
    };

    return {
      onThinkingDelta: (delta: string) => {
        if (!currentThought) currentThought = newThoughtStep(delta);
        else {
          currentThought = {
            ...currentThought,
            thoughtText: currentThought.thoughtText + delta,
            durationSeconds: Math.max(1, Math.round((Date.now() - (currentThought.createdAtMs || Date.now())) / 1000)),
          };
        }
        thoughtDirty = true;
        scheduleFlush();
      },
      onTextDelta: (delta: string) => {
        currentThought = null;
        pendingText += delta;
        scheduleFlush();
      },
      onToolUpsert: (tool: ToolExecution) => {
        updateLastAgentMsg(m => {
          const tools = m.toolExecutions || [];
          const exists = tools.some(t => t.id === tool.id);
          const updated = exists ? tools.map(t => t.id === tool.id ? { ...t, ...tool } : t) : [...tools, tool];
          return { ...m, toolExecutions: updated, isThinking: false };
        });
      },
      onToolRemove: (toolId: string) => {
        updateLastAgentMsg(m => ({
          ...m,
          toolExecutions: (m.toolExecutions || []).filter(t => t.id !== toolId),
        }));
      },
      onDiffCreated: (diff: FileDiff) => {
        addDiff(diff);
        setSessions(prev => prev.map(s => {
          if (s.id !== storeSessionId) return s;
          // Dedupe by file (latest wins) and cap at 10 — each diff pins two
          // full file copies, so an append-only list would grow unbounded.
          const kept = (s.diffs || []).filter(d => d.filePath !== diff.filePath);
          return { ...s, diffs: [...kept, diff].slice(-10) };
        }));
      },
      onStateChange: (snap: any) => {
        const state = snap.state || snap.State || '';
        setSessions(prev => prev.map(s => {
          if (s.id !== storeSessionId || s.goSessionId !== goSessionId) return s;
          const paused = state === 'awaiting_approval' || state === 'awaiting_input';
          const msgs = paused
            ? s.messages.map((m, idx) => idx === s.messages.length - 1 && m.role === 'agent' ? { ...m, isThinking: false } : m)
            : s.messages;
          return {
            ...s,
            goState: state,
            pendingTools: Array.isArray(snap.pending_tools) ? snap.pending_tools : (Array.isArray(snap.PendingTools) ? snap.PendingTools : []),
            pendingQuestions: Array.isArray(snap.pending_questions) ? snap.pending_questions : (Array.isArray(snap.PendingQuestions) ? snap.PendingQuestions : []),
            contextTokens: (snap as any).contextTokens ?? (snap as any).ContextTokens ?? s.contextTokens,
            cacheHitRate: (snap as any).cacheHitRate ?? (snap as any).CacheHitRate ?? s.cacheHitRate,
            messages: msgs,
          };
        }));
      },
      onTurnEnd: (error?: string) => {
        // Flush any coalesced deltas before finalizing so streamed text that
        // was still inside the buffer is not dropped.
        if (flushTimer !== null) {
          window.clearTimeout(flushTimer);
          flushTimer = null;
        }
        flushPending();
        currentThought = null;
        const cancelled = !!error && /context canceled/i.test(error);
        setSessions(prev => prev.map(s => {
          if (s.id !== storeSessionId) return s;
          const msgs = s.messages.map((m, idx) => {
            if (idx === s.messages.length - 1 && m.role === 'agent') {
              if (cancelled) {
                const base = (m.content || '').trim();
                return { ...m, content: base ? `${base}\n\n*(stopped by user)*` : '*(stopped by user)*', isThinking: false };
              }
              if (error) {
                return { ...m, content: `${m.content || ''}${m.content ? '\n\n' : ''}⚠️ **Error**: ${error}`, isThinking: false };
              }
              return { ...m, isThinking: false };
            }
            return m;
          });
          const finishedSession = { ...s, status: 'completed' as const, goState: 'idle', pendingTools: [], pendingQuestions: [], messages: msgs };
          ApiBridge.saveSessionJsonl(finishedSession, activeWorkspacePath);
          return finishedSession;
        }));
      },
    };
  }, [setSessions, addDiff, activeWorkspacePath]);

  // ensureGoSession provisions (or reuses) the Go harness session for a store
  // session and registers its event callbacks.
  const ensureGoSession = useCallback(async (storeSession: AgentSession): Promise<string | null> => {
    if (storeSession.goSessionId) {
      goAgentSessions.register(storeSession.goSessionId, makeGoCallbacks(storeSession.id, storeSession.goSessionId));
      return storeSession.goSessionId;
    }
    const goId = await goAgentSessions.createSession(storeSession.title, storeSession.workspacePath || activeWorkspacePath);
    if (!goId) return null;
    setSessions(prev => prev.map(s => s.id === storeSession.id ? { ...s, goSessionId: goId, goState: 'idle' } : s));
    goAgentSessions.register(goId, makeGoCallbacks(storeSession.id, goId));
    // Seed the composer's permission mode into the fresh Go session so
    // "Full access" sessions never gate mutating tools on first use.
    SetAgentAutoApprove(goId, agentExecutionMode === 'bypass').catch(() => {});
    return goId;
  }, [activeWorkspacePath, makeGoCallbacks, agentExecutionMode]);

  // sendGoAgentMessage routes one user prompt through the Go harness.
  // sessionOverride covers the createNewSession race: the store session may
  // not be in refs yet when the first prompt fires.
  const sendGoAgentMessage = useCallback(async (storeSessionId: string, prompt: string, sessionOverride?: AgentSession) => {
    const storeSession = sessionOverride
      || sessionsRef.current.find(s => s.id === storeSessionId)
      || sessions.find(s => s.id === storeSessionId);
    if (!storeSession) return;
    const goId = await ensureGoSession(storeSession);
    if (!goId) {
      setSessions(prev => prev.map(s => {
        if (s.id !== storeSessionId) return s;
        const msgs = s.messages.map((m, idx) => {
          if (idx === s.messages.length - 1 && m.role === 'agent') {
            return { ...m, content: '⚠️ **Error**: failed to create the backend agent session.', isThinking: false };
          }
          return m;
        });
        return { ...s, status: 'completed' as const, messages: msgs };
      }));
      return;
    }
    try {
      // Point the Go harness at the model this session shows in the UI (and
      // its provider's API key) before the turn starts.
      await activateGoModel(storeSession.model || currentModel);
      await goAgentSessions.send(goId, prompt, extractMentionedPaths(prompt, storeSession.workspacePath || activeWorkspacePath));
    } catch (err: any) {
      setSessions(prev => prev.map(s => {
        if (s.id !== storeSessionId) return s;
        const msgs = s.messages.map((m, idx) => {
          if (idx === s.messages.length - 1 && m.role === 'agent') {
            return { ...m, content: `⚠️ **Error**: ${err?.message || err}`, isThinking: false };
          }
          return m;
        });
        return { ...s, status: 'completed' as const, messages: msgs };
      }));
    }
  }, [activeWorkspacePath, ensureGoSession, sessions, activateGoModel, currentModel]);

  const createNewSession = useCallback((initialPrompt?: string, _selectedAgentId?: string, forcedId?: string) => {
    const newId = forcedId || `session-${Date.now()}`;
    // The Agent UI is internal-only: composer sessions always run the
    // ForgeADE harness. External agents live as CLI tasks, not composer agents.
    const agentObj = agents.find(a => a.id === 'agent-internal') || activeAgent;

    let effectivePrompt = initialPrompt;
    if (effectivePrompt && effectivePrompt.startsWith('/')) {
      const parts = effectivePrompt.slice(1).split(/\s+/);
      const cmdName = parts[0].toLowerCase();
      const userArgs = parts.slice(1).join(' ');
      const matched = customCommands.find(c => c.enabled && c.name.toLowerCase() === cmdName);
      if (matched) {
        effectivePrompt = userArgs
          ? `${matched.promptTemplate}\n\nUser instructions: ${userArgs}`
          : matched.promptTemplate;
      }
    }

    const effectiveModel = agentObj.model || currentModel;
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
      void sendGoAgentMessage(newId, effectivePrompt, newSession);
    }
  }, [agents, activeAgent, currentModel, activeWorkspacePath, customCommands, sendGoAgentMessage, setActiveTaskKind]);

  // Wire the deferred reference used by runAutomation (createNewSession is
  // declared earlier; keep it fresh on every change).
  useEffect(() => {
    createNewSessionRef.current = createNewSession;
  }, [createNewSession]);

  const sendAgentPrompt = useCallback((promptText: string) => {
    if (!promptText.trim()) return;

    let effectivePrompt = promptText.trim();
    if (effectivePrompt.startsWith('/')) {
      const parts = effectivePrompt.slice(1).split(/\s+/);
      const cmdName = parts[0].toLowerCase();
      const userArgs = parts.slice(1).join(' ');
      const matched = customCommands.find(c => c.enabled && c.name.toLowerCase() === cmdName);
      if (matched) {
        effectivePrompt = userArgs
          ? `${matched.promptTemplate}\n\nUser instructions: ${userArgs}`
          : matched.promptTemplate;
      }
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
      return {
        ...s,
        title: updatedTitle,
        status: 'running',
        updatedAt: new Date().toISOString(),
        messages: [...s.messages, userMsg, agentPlaceholder]
      };
    }));

    // Internal-only: the ForgeADE Go harness owns every composer turn.
    void sendGoAgentMessage(activeSessionId, effectivePrompt);
  }, [activeSessionId, agents, activeAgent, activeWorkspacePath, customCommands, sendGoAgentMessage]);

  const stopAgentExecution = useCallback(() => {
    // Cancel the backend turn; the agent:turn_end event finalizes the message.
    const currentSession = activeSessionId
      ? (sessionsRef.current.find(s => s.id === activeSessionId) || sessions.find(s => s.id === activeSessionId))
      : null;
    if (currentSession?.goSessionId) {
      StopAgentTurn(currentSession.goSessionId);
      return;
    }
    if (!activeSessionId) return;
    setSessions(prev => prev.map(s => {
      if (s.id !== activeSessionId) return s;
      const msgs = [...s.messages];
      const last = msgs[msgs.length - 1];
      if (last && last.role === 'agent') {
        last.isThinking = false;
        let content = (last.content || '')
          .replace(/<tool(?:_call)?[\s\S]*$/i, '')
          .replace(/<invoke[\s\S]*$/i, '')
          .replace(/<function_call[\s\S]*$/i, '')
          .replace(/<action[\s\S]*$/i, '')
          .trim();
        content = content.replace(/\s*\(Execution stopped by user\)/gi, '').trim();
        last.content = content ? `${content} (Execution stopped by user)` : '(Execution stopped by user)';
      }
      return { ...s, status: 'idle', messages: msgs };
    }));
  }, [activeSessionId]);

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
  }, [setActiveTaskKind]);

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
        setIsLeftSidebarOpen(true);
        setActiveActivity('search');
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
  }, [switchSurface, toggleTheme, openSettingsTab, openFolder, openCommandPalette, cycleTab, setMode]);

  return (
    <WorkspaceContext.Provider
      value={{
        mode,
        setMode,
        activeTaskKind,
        setActiveTaskKind,
        theme,
        setTheme,
        toggleTheme,
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
        isLeftSidebarOpen,
        setIsLeftSidebarOpen,
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
        canGoBack: nav.index > 0,
        canGoForward: nav.index >= 0 && nav.index < nav.stack.length - 1,
        updateFileContent,
        setTabBuffer,
        saveTabToDisk,
        setDiagnostics,
        createNewSession,
        sendAgentPrompt,
        stopAgentExecution,
        deleteSession,
        savedSessions,
        openSessionFromHistory,
        deleteSessionPermanently,
        reloadSavedSessions,
        archivedSessionIds,
        toggleArchiveSession,
        activeWorkspacePath,
        setActiveWorkspacePath,
        activeWorkspaces,
        switchWorkspace: async (path: string) => setActiveWorkspacePath(path),
        switchSurface,
        closeWorkspaceFromWindow,
        openWorkspaceInNewWindow,
        diagnostics
      }}
    >
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
