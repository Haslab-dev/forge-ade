export type AppMode = 'agent' | 'editor' | 'settings';
export type WorkspaceMode = 'agent' | 'editor' | 'settings';
export type AppTheme = 'light' | 'dark';
export type ThemeMode = 'light' | 'dark';
export type ActivityBarItem = 'explorer' | 'search' | 'git' | 'shell' | 'debug' | 'extensions' | 'account' | 'settings';

export type AgentExecutionMode = 'code' | 'ask' | 'plan' | 'bypass';
export type AgentReasoningLevel = 'low' | 'high' | 'max';
export type AgentEngineType = 'internal' | 'pi' | 'ohmypi' | 'opencode';

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
  agentId: string;
  workspacePath: string;
  reasoningLevel?: AgentReasoningLevel;
  executionMode?: AgentExecutionMode;
  messages: AgentMessage[];
  sideConversationMessages?: AgentMessage[];
  diffs?: FileDiff[];
  contextTokens?: number;
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

// Internal Agent + ACP agents (Pi, OhMyPi/OMP, OpenCode)
export interface ACPAgent {
  id: string;
  name: string;
  type: 'internal' | 'pi' | 'ohmypi' | 'opencode' | 'custom' | string;
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
