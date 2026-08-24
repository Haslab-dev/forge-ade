// Agent engine shared types — ported from the reference agent runtime.
// Block-based message model: every non-empty message carries a content array.

export type ContentBlockType = "text" | "thinking" | "tool_call" | "tool_result";

export interface ContentBlock {
  type: ContentBlockType;
  text?: string;
  tool_call_id?: string;
  name?: string;
  /** Raw JSON object string for tool_call arguments (streamed incrementally). */
  arguments?: string;
  is_error?: boolean | undefined;
}

export type MessageRole = "user" | "assistant" | "system" | "tool";
export type SessionState = "idle" | "running" | "awaiting_approval" | "awaiting_input";

export interface AgentMessage {
  id: string;
  role: MessageRole;
  content: ContentBlock[];
  timestamp: string;
  /** Streaming marker on the live assistant message; stripped before persist. */
  state?: "running" | "done";
  /** Token/time stats attached when the LLM call for this message completes. */
  usage?: {
    at: number;
    promptTokens: number;
    completionTokens: number;
    cachedTokens: number;
    durationMs: number;
  } | undefined;
}

export interface Observation {
  ts: number;
  summary: string;
}

export interface TaskItem {
  id: string;
  title: string;
  completed: boolean;
  active?: boolean | undefined;
}

export interface PendingAskQuestion {
  id: string;
  label: string;
  description?: string | undefined;
  options?: { label: string; description?: string | undefined }[] | undefined;
  multi?: boolean | undefined;
}

// ---------------------------------------------------------------------------
// Session
// ---------------------------------------------------------------------------

export interface SessionMeta {
  id: string;
  name: string;
  role: string;
  projectFolder: string;
  dialect: string;
  autoApprove: boolean;
  createdAt: number;
  updatedAt: number;
  messageCount: number;
  lastMessagePreview: string;
  state: SessionState;
  /** Assumed model context window (tokens) for the ctx% readout. */
  contextWindow: number;
  /** Usage of the most recent LLM call; drives the in/out/cache/tok-s line. */
  lastUsage?: TurnUsage | undefined;
}

export interface Session extends SessionMeta {
  messages: AgentMessage[];
  customPrompt?: string | undefined;
  customRules?: string | undefined;
  modelOverride?: string | undefined;
  summary?: string | undefined;
  observations: Observation[];
  todos: TaskItem[];
  pendingApproval?: PendingToolCall[] | undefined;
  pendingAsk?: PendingAskQuestion[] | undefined;
  totalUsage: TokenUsage;
  lastUsage?: TurnUsage | undefined;
}

export interface AgentDefinition {
  id: string;
  name: string;
  role_filter?: string | undefined;
  description: string;
  prompt: string;
  rules: string;
  model?: string | undefined;
  color?: string | undefined;
}

export interface PendingToolCall {
  id: string;
  name: string;
  args: Record<string, unknown>;
}

export interface TokenUsage {
  promptTokens: number;
  completionTokens: number;
  requests: number;
}

/** Usage snapshot of the most recent LLM call, for the status line. */
export interface TurnUsage {
  /** Epoch ms when the call finished. */
  at: number;
  promptTokens: number;
  completionTokens: number;
  cachedTokens: number;
  durationMs: number;
}

/** Conservative default context window when the profile doesn't declare one. */
export const DEFAULT_CONTEXT_WINDOW = 128_000;

export function emptySession(partial: {
  id: string;
  name: string;
  role: string;
  projectFolder: string;
  createdAt: number;
}): Session {
  return {
    ...partial,
    dialect: "",
    autoApprove: false,
    updatedAt: partial.createdAt,
    messageCount: 0,
    lastMessagePreview: "",
    state: "idle",
    contextWindow: DEFAULT_CONTEXT_WINDOW,
    messages: [],
    observations: [],
    todos: [],
    totalUsage: { promptTokens: 0, completionTokens: 0, requests: 0 },
  };
}

// ---------------------------------------------------------------------------
// JSONL session file entries (append-only, oh-my-pi-style tree-ready shape)
// ---------------------------------------------------------------------------

export interface SessionEntryBase {
  id: string;
  parentId: string | null;
  timestamp: string;
}

export interface SessionHeaderEntry {
  type: "session";
  version: number;
  id: string;
  name: string;
  role: string;
  projectFolder: string;
  createdAt: number;
}

export interface SessionMessageEntry extends SessionEntryBase {
  type: "message";
  message: AgentMessage;
}

export interface SessionCompactionEntry extends SessionEntryBase {
  type: "compaction";
  summary: string;
  keptFromMessageId: string;
}

export type SessionFileEntry = SessionMessageEntry | SessionCompactionEntry;

// ---------------------------------------------------------------------------
// LLM wire types (OpenAI-compatible)
// ---------------------------------------------------------------------------

export interface ToolDefinition {
  type: "function";
  function: {
    name: string;
    description: string;
    parameters: Record<string, any>;
  };
}

export interface ToolCall {
  id: string;
  type: "function";
  function: { name: string; args: Record<string, any> };
}

export interface LLMMessage {
  role: "system" | "user" | "assistant" | "tool";
  content: string;
  tool_calls?: { id: string; type: "function"; function: { name: string; arguments: string } }[];
  tool_call_id?: string;
}

export interface LLMResponse {
  content: string;
  reasoning: string;
  toolCalls: ToolCall[];
  promptTokens: number;
  completionTokens: number;
  cachedTokens: number;
  stopReason: string;
}

export interface ToolCallDelta {
  index: number;
  id: string;
  name: string;
  argsFragment: string;
}
