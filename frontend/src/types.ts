export type AppMode = 'agent' | 'editor' | 'settings';
// Agent Mode and Editor Mode are two separate surfaces, each with its own
// workspace folder and functions. Switching surfaces carries the folder
// across (agent's folder opens in the editor and vice versa).
export type WorkspaceMode = 'agent' | 'editor' | 'settings' | 'automations';
// The persisted application-level experience (legacy Settings field; the UI
// no longer offers a mode switch — kept for settings-file compatibility).
export type ForgeMode = 'terminal' | 'agent-ui';
/** Which kind of task the workspace surface is showing. */
export type TaskKind = 'forge' | 'cli';
export type AppTheme = 'light' | 'dark';
export type ThemeMode = 'light' | 'dark' | 'system';

/** Prefix-driven mode of the unified command palette. */
export type CommandPaletteMode = 'files' | 'commands' | 'symbols' | 'line';
export type ActivityBarItem = 'explorer' | 'search' | 'git' | 'shell' | 'debug' | 'extensions' | 'account' | 'settings';

export type AgentExecutionMode = 'code' | 'ask' | 'plan' | 'bypass';
export type AgentReasoningLevel = 'off' | 'minimal' | 'low' | 'medium' | 'high' | 'xhigh' | 'max';
export type AgentEngineType = 'internal' | 'claude' | 'codex' | 'opencode' | 'commandcode' | 'pi' | 'ohmypi' | 'cursor';

// ── Workflow Task Board for Agent Mode ──────────────────────────────────────

export type BoardTaskStatus = 'todo' | 'in_progress' | 'in_review' | 'in_testing' | 'done';
export type BoardTaskPriority = 'low' | 'medium' | 'high' | 'urgent';

export interface BoardTask {
  id: string;
  key: string;
  title: string;
  description?: string;
  status: BoardTaskStatus;
  priority: BoardTaskPriority;
  assignee?: string;
  labels?: string[];
  sessionId?: string;
  taskKind?: 'forge' | 'cli';
  workspacePath?: string;
  createdAt: number;
  updatedAt: number;
  completedAt?: number;
  branch?: string;
  filesTouched?: number;
}

// Backward-compatibility aliases
export type JiraTaskStatus = BoardTaskStatus;
export type JiraTaskPriority = BoardTaskPriority;
export type JiraTask = BoardTask;

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
  multilinePasteMode?: 'ask' | 'bracketed' | 'stringify' | 'singleLine';
}

export interface JevSettings {
  /** Master toggle for TypeSafe AI Jev tool routing. When disabled or key missing/error, cascades to local small model. */
  enabled?: boolean;
  /** Typesafe AI Jev API key. Empty disables remote routing (local heuristic/small model only). */
  apiKey: string;
  /** Optional endpoint override; empty uses the default cloud endpoint. */
  endpoint?: string;
}

export interface AppSettings {
  defaultMode: ForgeMode;
  terminal: TerminalSettings;
  jev?: JevSettings;
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
  tokenCount?: number;
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
  turnTexts?: Record<number, string>;
  isThinking?: boolean;
  toolStatus?: string;
  currentTurn?: number;
  agentId?: string;
  agentName?: string;
}

export type TaskStatus = 'pending' | 'in_progress' | 'completed';

export interface SessionTaskItem {
  id: string;
  title: string;
  status?: TaskStatus;
  active_form?: string;
  completed?: boolean;
}

export interface BackgroundTaskItem {
  id: string;
  task_id?: string;
  command: string;
  cwd?: string;
  status: 'running' | 'completed' | 'failed' | 'stopped';
  exit_code?: number;
  started_at: string;
  finished_at?: string;
  output_tail?: string;
  output_file?: string;
  output_size?: number;
  session_id?: string;
}

export interface AgentScheduleItem {
  id: string;
  session_id: string;
  prompt: string;
  task_id?: string;
  duration_seconds?: number;
  cron_expression?: string;
  created_at: string;
  next_run: string;
  active: boolean;
  status?: string;
}

export interface SessionProgress {
  current_goal?: string;
  completed_steps?: string[];
  active_todos?: string[];
}

export interface SubagentInfo {
  id: string;
  name: string;
  role: string;
  state: string;
  folder?: string;
  parentSessionId?: string;
  messages?: AgentMessage[];
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
  remoteSessionId?: string;
  /** Go agent harness session id (internal agents) — the authoritative transcript backend. */
  goSessionId?: string;
  /** Live Go harness session state: idle | thinking | executing | awaiting_approval | awaiting_input */
  goState?: string;
  /** Tool calls paused by the Go harness approval gate. */
  pendingTools?: any[];
  /** Structured questions from the harness `ask` tool. */
  pendingQuestions?: any[];
  /** Structured task items for the floating task tracker and progress monitoring */
  tasks?: SessionTaskItem[];
  /** Detached background tasks / dev servers running for this session */
  backgroundTasks?: BackgroundTaskItem[];
  /** Active timers and cron schedules */
  schedules?: AgentScheduleItem[];
  /** Compact execution progress (goal, completed steps, active todos) */
  progress?: SessionProgress;
  /** Parent agent session ID if this session was spawned as a subagent */
  parentSessionId?: string;
  /** Spawned subagent session IDs */
  subagentIds?: string[];
  /** Cumulative token usage stats reported by provider */
  tokenUsage?: TokenUsageStats;
}

export interface TokenUsageStats {
  prompt_tokens?: number;
  completion_tokens?: number;
  cached_tokens?: number;
  prompt_cache_hit_tokens?: number;
  prompt_cache_miss_tokens?: number;
  total_tokens?: number;
}

export interface ModelUsageAgg {
  model: string;
  provider: string;
  prompt_tokens: number;
  completion_tokens: number;
  cached_tokens: number;
  total_tokens: number;
  turns: number;
  sessions: number;
}

export interface ProviderUsageAgg {
  provider: string;
  prompt_tokens: number;
  completion_tokens: number;
  cached_tokens: number;
  total_tokens: number;
  models: string[];
  sessions: number;
}

export interface SessionUsageItem {
  id: string;
  name: string;
  role: string;
  model: string;
  provider: string;
  prompt_tokens: number;
  completion_tokens: number;
  cached_tokens: number;
  total_tokens: number;
  context_tokens: number;
  cache_hit_rate?: number;
  turns: number;
  created_at: string;
  updated_at: string;
}

export interface JevUsageStats {
  enabled: boolean;
  has_key: boolean;
  endpoint: string;
  model: string;
  total_requests?: number;
  total_routes: number;
  category_counts: Record<string, number>;
  prompt_tokens?: number;
  completion_tokens?: number;
  total_tokens?: number;
  tokens_saved: number;
  avg_latency_ms: number;
}

export interface UsageSummaryStats {
  totals: {
    total_tokens: number;
    prompt_tokens: number;
    completion_tokens: number;
    cached_tokens: number;
    session_count: number;
    turn_count: number;
  };
  by_model: ModelUsageAgg[];
  by_provider: ProviderUsageAgg[];
  by_session: SessionUsageItem[];
  jev?: JevUsageStats;
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
  /** Buffer has unsaved edits (VS Code dirty dot). */
  isModified?: boolean;
  /** Pinned tabs survive bulk closes (Close Others/Left/Right/Clean/All). */
  isPinned?: boolean;
  /** Read-only tab: CodeMirror is non-editable and buffers refuse writes. */
  isReadOnly?: boolean;
}

// Internal Agent + External Harnesses (Claude Code, Codex, OpenCode, Command Code, Pi, Oh My Pi, Cursor)
export interface ACPAgent {
  id: string;
  name: string;
  type: 'internal' | 'claude' | 'codex' | 'opencode' | 'commandcode' | 'pi' | 'ohmypi' | 'cursor' | 'custom' | string;
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

export interface AgentDefinition {
  id: string;
  name: string;
  description?: string;
  role_filter?: 'coding' | 'planning' | 'research' | 'custom' | string;
  model?: string;
  prompt?: string;
  rules?: string;
  color?: string;
  allow_bash?: boolean;
  read_only?: boolean;
  tools?: string[];
  mcp_servers?: string[];
  skills?: string[];
  created_at?: string;
  updated_at?: string;
}

export interface MCPEntry {
  id: string;
  name: string;
  command: string;
  args?: string[];
  env?: Record<string, string>;
  url?: string;
  type?: string;
  status: 'running' | 'connected' | 'standby' | 'error';
  enabled: boolean;
  version?: string;
  origin?: 'antigravity' | 'opencode' | 'pi' | 'claude' | 'codex' | 'commandcode' | 'custom';
  tools: string[];
  lastError?: string;
}

export interface MCPEnvVar {
  key: string;
  label: string;
  placeholder: string;
  required: boolean;
  secret: boolean;
  default_value?: string;
  detected_from?: string;
}

export interface MCPPreset {
  id: string;
  name: string;
  label: string;
  description: string;
  command: string;
  args: string[];
  env_vars: MCPEnvVar[];
  website_url?: string;
  icon?: string;
}

export interface DiscoveredMCP {
  id: string;
  name: string;
  command: string;
  args?: string[];
  env?: Record<string, string>;
  url?: string;
  type?: string;
  origin: 'antigravity' | 'opencode' | 'pi' | 'claude' | 'codex' | 'commandcode';
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
  origin?: 'antigravity' | 'opencode' | 'pi' | 'claude' | 'commandcode' | 'custom' | 'workspace' | 'global' | 'plugin' | string;
  path?: string;
}

export interface DiscoveredSkill {
  id: string;
  name: string;
  category: string;
  description: string;
  instructions?: string;
  trigger: string;
  origin: 'antigravity' | 'opencode' | 'pi' | 'claude' | 'commandcode';
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

/** One declaration from the workspace symbol index (outline / `@` palette). */
export interface WorkspaceSymbol {
  name: string;
  kind:
    | 'function'
    | 'class'
    | 'interface'
    | 'enum'
    | 'struct'
    | 'type'
    | 'variable'
    | 'constant'
    | 'method'
    | 'package'
    | 'unknown';
  file: string;
  /** 1-based, matches what openFileInEditor expects. */
  line: number;
  column: number;
  /** Enclosing declaration name when nested; empty at top level. */
  scope?: string;
  exported?: boolean;
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

/** One known file/dir of an external agent CLI's configuration. */
export interface AgentCliConfigPath {
  purpose: string;
  path: string;
  exists: boolean;
}

/** A model provider configured inside an external agent CLI. */
export interface AgentCliProviderInfo {
  id: string;
  name?: string;
  baseUrl?: string;
  models?: string[];
}

/** An MCP server configured inside an external agent CLI. */
export interface AgentCliMCPServer {
  name: string;
  type: 'local' | 'remote' | string;
  command?: string;
  url?: string;
}

/** One skill discovered for an external agent CLI. */
export interface AgentCliSkill {
  name: string;
  description?: string;
  path?: string;
}

/** Everything ForgeADE reads about one agent CLI's local configuration. */
export interface AgentCliRuntimeConfig {
  id: string;
  name: string;
  executable: string;
  installed: boolean;
  platform: string;
  defaultModel?: string;
  /** Global config directory — one-stop folder to open in the editor. */
  configRoot?: string;
  paths: AgentCliConfigPath[];
  providers: AgentCliProviderInfo[];
  mcpServers: AgentCliMCPServer[];
  skills: AgentCliSkill[];
  error?: string;
}
