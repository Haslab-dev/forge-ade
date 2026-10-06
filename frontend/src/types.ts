export type AppMode = 'agent' | 'editor' | 'settings';
// 'terminal' is the default experience (Terminal Session mode); 'agent' is
// the native Agent UI, kept dormant behind the mode switch.
export type WorkspaceMode = 'terminal' | 'agent' | 'editor' | 'settings' | 'automations';
// The persisted application-level experience (Settings → Agent → Default
// Mode). Maps to workspace modes: 'terminal' → 'terminal', 'agent-ui' → 'agent'.
export type ForgeMode = 'terminal' | 'agent-ui';
export type AppTheme = 'light' | 'dark';
export type ThemeMode = 'light' | 'dark';
export type ActivityBarItem = 'explorer' | 'search' | 'git' | 'shell' | 'debug' | 'extensions' | 'account' | 'settings';

export type AgentExecutionMode = 'code' | 'ask' | 'plan' | 'bypass';
export type AgentReasoningLevel = 'off' | 'minimal' | 'low' | 'medium' | 'high' | 'xhigh' | 'max';
export type AgentEngineType = 'internal' | 'claude' | 'codex' | 'opencode' | 'pi' | 'ohmypi' | 'cursor';

// ── Terminal Session mode ────────────────────────────────────────────────────

export type TerminalSessionStatus = 'starting' | 'running' | 'exited' | 'failed' | 'terminated';

/** One agent CLI process in its own PTY. Sessions are independent. */
export interface TerminalSessionRecord {
  id: string;
  agentId: string;
  agentName: string;
  title: string;
  /** True when the title was auto-derived (folder name / first input) and may still be renamed. */
  titleAuto?: boolean;
  workspacePath?: string;
  workingDirectory: string;
  executable: string;
  args?: string[];
  pid?: number;
  createdAt: number;
  startedAt?: number;
  endedAt?: number;
  status: TerminalSessionStatus;
  exitCode?: number;
  error?: string;
}

/** User-configurable launch definition for one agent CLI. */
export interface AgentCLIConfig {
  id: string;
  name: string;
  executable: string;
  args: string[];
  /** Appended on restart so the CLI continues its previous conversation (e.g. --continue). */
  resumeArgs?: string[];
  workingDirectory?: string;
  environment?: Record<string, string>;
  enabled: boolean;
}

export interface TerminalSettings {
  shell: string;
  fontFamily: string;
  fontSize: number;
  cursorStyle: 'block' | 'bar' | 'underline';
  cursorBlink: boolean;
  scrollback: number;
}

export interface AppSettings {
  defaultMode: ForgeMode;
  terminal: TerminalSettings;
}

export interface LSPDiagnostic {
  id: string;
  filePath: string;
  line: number;
  column: number;
  severity: 'error' | 'warning' | 'info';
  message: string;
  source?: string;
}

export interface FileItem {
  id: string;
  name: string;
  path: string;
  type: 'file' | 'folder';
  children?: FileItem[];
  content?: string;
  language?: string;
  isModified?: boolean;
}

export interface ThoughtStep {
  id: string;
  durationSeconds: number;
  thoughtText: string;
  timestamp: string;
  turn?: number;
  createdAtMs?: number;
}

export interface FileDiff {
  id: string;
  filePath: string;
  fileName: string;
  originalContent: string;
  modifiedContent: string;
  additions: number;
  deletions: number;
  status: 'pending' | 'accepted' | 'rejected';
  timestamp: string;
  /** 'git' = read-only review of on-disk state (no Accept/Reject); agent diffs default to 'agent'. */
  kind?: 'git' | 'agent';
}

export interface ToolExecution {
  id: string;
  toolName: string;
  command?: string;
  output?: string;
  status: 'running' | 'completed' | 'failed';
  readFiles?: string[];
  subtasks?: string[];
  diff?: FileDiff;
  turn?: number;
  createdAtMs?: number;
}

export interface AgentMessage {
  id: string;
  role: 'user' | 'agent' | 'system';
  content: string;
  timestamp: string;
  thoughts?: ThoughtStep[];
  toolExecutions?: ToolExecution[];
  isThinking?: boolean;
  toolStatus?: string;
  currentTurn?: number;
  agentId?: string;
  agentName?: string;
}

export interface AgentSession {
  id: string;
  title: string;
  createdAt: string;
  updatedAt: string;
  status: 'idle' | 'running' | 'completed' | 'paused';
  model: string;
  provider?: string;
  agentId: string;
  agentType?: string;
  workspacePath: string;
  reasoningLevel?: AgentReasoningLevel;
  executionMode?: AgentExecutionMode;
  messages: AgentMessage[];
  sideConversationMessages?: AgentMessage[];
  diffs?: FileDiff[];
  contextTokens?: number;
  /** Session-average prompt-cache hit rate (0..1), from cumulative provider usage. */
  cacheHitRate?: number;
  acpSessionId?: string;
  /** Go agent harness session id (internal agents) — the authoritative transcript backend. */
  goSessionId?: string;
  /** Live Go harness session state: idle | thinking | executing | awaiting_approval | awaiting_input */
  goState?: string;
  /** Tool calls paused by the Go harness approval gate. */
  pendingTools?: any[];
  /** Structured questions from the harness `ask` tool. */
  pendingQuestions?: any[];
}

export interface ContextUsageCategory {
  tokens: number;
  percent: number;
  formattedPercent: string;
}

export interface ContextUsageInfo {
  usedTokens: number;
  maxTokens: number;
  percent: number;
  formattedUsed: string;
  formattedMax: string;
  categories: {
    messages: ContextUsageCategory;
    mcpTools: ContextUsageCategory;
    systemTools: ContextUsageCategory;
    systemPrompt: ContextUsageCategory;
    skills: ContextUsageCategory;
    metaContext: ContextUsageCategory;
  };
}

export interface EditorTab {
  id: string;
  fileId: string;
  fileName: string;
  filePath: string;
  type: 'code' | 'preview' | 'diff' | 'settings' | 'git-graph' | 'terminal';
  content?: string;
  diffId?: string;
  terminalSessionId?: string;
  line?: number;
  column?: number;
}

// Internal Agent + External Harnesses (Claude Code, Codex, OpenCode, Pi, Oh My Pi, Cursor)
export interface ACPAgent {
  id: string;
  name: string;
  type: 'internal' | 'claude' | 'codex' | 'opencode' | 'pi' | 'ohmypi' | 'cursor' | 'custom' | string;
  description: string;
  icon: string;
  isDefault?: boolean;
  isStarred?: boolean;
  enabled: boolean;
  provider: string;
  command?: string;
  args?: string[];
  env?: Record<string, string>;
  endpoint?: string;
  apiKey?: string;
  model?: string;
  supportedModels?: string[];
  supportedEfforts?: AgentReasoningLevel[];
  defaultEffort?: AgentReasoningLevel;
  status?: 'connected' | 'disconnected' | 'connecting' | 'error';
  handshakeError?: string;
}

export interface ModelItem {
  id: string;
  name: string;
  enabled: boolean;
  providerId: string;
  description?: string;
}

export interface LLMProviderConfig {
  id: string;
  name: string;
  baseUrl?: string;
  apiKey?: string;
  enabled: boolean;
  models: string[];
  selectedModels?: string[];
}

export interface MCPEntry {
  id: string;
  name: string;
  command: string;
  status: 'running' | 'connected' | 'standby' | 'error';
  enabled: boolean;
  version?: string;
  origin?: 'antigravity' | 'opencode' | 'pi' | 'claude' | 'codex' | 'custom';
  tools: string[];
}

export interface DiscoveredMCP {
  id: string;
  name: string;
  command: string;
  args?: string[];
  origin: 'antigravity' | 'opencode' | 'pi' | 'claude' | 'codex';
  originLabel: string;
  configPath?: string;
  tools: string[];
}

export interface SkillEntry {
  id: string;
  name: string;
  category: string;
  description: string;
  instructions?: string;
  enabled: boolean;
  trigger: string;
  origin?: 'antigravity' | 'opencode' | 'pi' | 'claude' | 'custom' | 'workspace' | 'global' | 'plugin' | string;
}

export interface DiscoveredSkill {
  id: string;
  name: string;
  category: string;
  description: string;
  instructions?: string;
  trigger: string;
  origin: 'antigravity' | 'opencode' | 'pi' | 'claude';
  originLabel: string;
  filePath?: string;
}

export interface RuleEntry {
  id: string;
  title: string;
  content: string;
  description?: string;
  severity?: 'error' | 'warning' | 'info';
  enabled: boolean;
  category: string;
}

export interface SubagentSession {
  id: string;
  name: string;
  model: string;
  role: string;
  status: 'active' | 'idle' | 'paused';
  maxSteps: number;
}

export interface PluginToolDef {
  name: string;
  description: string;
  parameters?: Record<string, any>;
  handler_type?: 'command' | 'script' | 'inline';
  command?: string;
  script?: string;
  timeout_seconds?: number;
}

export interface PluginSkillDef {
  name: string;
  description: string;
  body: string;
}

export interface PluginInfo {
  id: string;
  name: string;
  description: string;
  version: string;
  author?: string;
  enabled: boolean;
  source: 'workspace' | 'global' | 'runtime' | 'builtin';
  path?: string;
  dir?: string;
  system_prompt?: string;
  tools?: PluginToolDef[];
  skills?: PluginSkillDef[];
  created_at?: string;
  updated_at?: string;
}

export interface CreatePluginRequest {
  id: string;
  name: string;
  description: string;
  version?: string;
  author?: string;
  scope?: 'workspace' | 'global';
  system_prompt?: string;
  tools?: PluginToolDef[];
  skills?: PluginSkillDef[];
  files?: Record<string, string>;
}

export interface CreateSkillRequest {
  name: string;
  description: string;
  body: string;
  scope?: 'workspace' | 'global';
  scripts?: Record<string, string>;
}

export interface AgentMemoryEntry {
  id: string;
  key: string;
  content: string;
  category: 'project' | 'architecture' | 'preference' | 'rule' | string;
  scope: 'workspace' | 'global';
  created_at?: string;
  updated_at?: string;
}

export interface CustomSlashCommand {
  id: string;
  name: string; // e.g. "review"
  description: string;
  promptTemplate: string;
  scope: 'workspace' | 'global';
  enabled: boolean;
}

export interface AgentHookConfig {
  id: string;
  name: string;
  event: 'pre_turn' | 'post_turn' | 'post_file_write' | 'pre_commit';
  command: string;
  enabled: boolean;
  timeout?: number;
}

export interface BrowserUseSettings {
  enabled: boolean;
  headless: boolean;
  searchProvider: 'google' | 'duckduckgo' | 'bing' | 'tavily' | 'searxng';
  searchApiKey?: string;
  remoteCdpUrl?: string;
  viewport: '1280x800' | '1920x1080' | '390x844';
  maxExtractLength: number;
  allowJavaScript: boolean;
}

export interface ComputerUseSettings {
  permissionMode: 'auto' | 'confirm_dangerous' | 'always_confirm';
  defaultShell: string;
  commandTimeoutSeconds: number;
  allowClipboard: boolean;
  screenCaptureScale: number;
  terminalSandbox: boolean;
}

export interface IndexingStatusInfo {
  built: boolean;
  symbols?: number;
  languages?: Record<string, number>;
  symbols_language?: Record<string, number>;
}

export interface PrivacySettings {
  shareTerminalActivity: boolean;
  shareUserEdits: boolean;
}

export interface Workspace {
  name: string;
  folders: string[];
  isTemporary: boolean;
  filePath: string;
  theme?: string;
  settings?: any;
}

export interface RecentEntry {
  path: string;
  name: string;
  lastOpened: string;
  pinned?: boolean;
}

export interface EditorFile {
  id: string;
  name: string;
  path: string;
  type: "file" | "shell" | "agent" | "diff" | "conflict";
  content?: string | null;
  savedContent?: string;
  isModified?: boolean;
  modified?: boolean;
  language?: string;
  diffPath?: string;
  diffHash?: string;
  conflictPath?: string;
  conflictStatus?: string;
}

export interface ShortcutKeybinding {
  id: string;
  name?: string;
  label?: string;
  key: string;
  category?: string;
}

export interface WorkspaceEntry {
  path: string;
  name: string;
  type: 'local' | 'remote';
  host?: string;
  lastActive: number;
}

export interface WindowWorkspaceState {
  openTabs: EditorTab[];
  activeTabId: string | null;
  selectedFile: FileItem | null;
  diffs: FileDiff[];
  activeDiff: FileDiff | null;
}
