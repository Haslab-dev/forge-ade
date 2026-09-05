export type AppMode = 'agent' | 'editor';
export type WorkspaceMode = 'agent' | 'editor';
export type AppTheme = 'light' | 'dark';
export type ThemeMode = 'light' | 'dark';
export type ActivityBarItem = 'explorer' | 'search' | 'git' | 'debug' | 'extensions' | 'account' | 'settings';

export type AgentExecutionMode = 'code' | 'ask' | 'plan' | 'bypass';
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
}

export interface AgentMessage {
  id: string;
  role: 'user' | 'agent' | 'system';
  content: string;
  timestamp: string;
  thoughts?: ThoughtStep[];
  toolExecutions?: ToolExecution[];
  isThinking?: boolean;
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
  messages: AgentMessage[];
  diffs?: FileDiff[];
}

export interface EditorTab {
  id: string;
  fileId: string;
  fileName: string;
  filePath: string;
  type: 'code' | 'preview' | 'diff' | 'settings';
  content?: string;
  diffId?: string;
  line?: number;
  column?: number;
}

export interface TerminalTab {
  id: string;
  title: string;
  type: 'problems' | 'output' | 'debug' | 'terminal' | 'ports';
  shellName?: string;
}

// Internal Agent + ACP agents (Pi, OhMyPi/OMP, OpenCode)
export interface ACPAgent {
  id: string;
  name: string;
  type: 'internal' | 'pi' | 'ohmypi' | 'opencode';
  description: string;
  icon: string;
  isDefault?: boolean;
  isStarred?: boolean;
  enabled: boolean;
  provider: string;
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
  origin?: 'antigravity' | 'opencode' | 'pi' | 'claude' | 'custom';
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
  enabled: boolean;
  category: string;
}

export interface SubAgentEntry {
  id: string;
  name: string;
  model: string;
  role: string;
  status: 'active' | 'idle' | 'paused';
  maxSteps: number;
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
