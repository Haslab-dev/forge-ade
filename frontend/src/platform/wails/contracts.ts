/**
 * Contract-Driven IPC: Strongly-typed Wails Events and Data Contracts
 * Mirroring Go backend events in internal/app/eventbridge.go
 */

import type { BoardTask, AgentSession, GitFileStatus } from '../../types';

// Wails Event Names (canonical string constants)
export const WailsEventNames = {
  FSChanged: 'fs:changed',
  BrowserFrame: 'browser:frame',
  BrowserStatus: 'browser:status',
  AgentConfigChanged: 'agent:config:changed',
  SessionOutput: 'session:output',
  SessionOpened: 'session:opened',
  SessionClosed: 'session:closed',
  AgentSessionUpdated: 'agentsession:updated',
  AgentSessionDeleted: 'agentsession:deleted',
  SSHHandshake: 'ssh:handshake',
  SSHConnected: 'ssh:connected',
  SSHDisconnected: 'ssh:disconnected',
  SSHError: 'ssh:error',
  BoardUpdated: 'board:updated',
  GitStatus: 'git:status',
  ACPState: 'acp:state',
  AgentUpdated: 'agent:updated',
  AgentStarted: 'agent:started',
  AgentStopped: 'agent:stopped',
  AgentTurnStart: 'agent:turn_start',
  AgentTurnEnd: 'agent:turn_end',
  AgentMessageStart: 'agent:message_start',
  AgentMessageDelta: 'agent:message_delta',
  AgentMessageEnd: 'agent:message_end',
  AgentThinkingStart: 'agent:thinking_start',
  AgentThinkingDelta: 'agent:thinking_delta',
  AgentThinkingEnd: 'agent:thinking_end',
  AgentToolStart: 'agent:tool_start',
  AgentToolDelta: 'agent:tool_delta',
  AgentToolEnd: 'agent:tool_end',
  AgentAsk: 'agent:ask',
  AgentTaskUpdate: 'agent:task_update',
  AgentScheduleUpdate: 'agent:schedule_update',
  AgentAvailableCommands: 'agent:available_commands',
} as const;

export type WailsEventName = typeof WailsEventNames[keyof typeof WailsEventNames];

export type FSChangeType = 'created' | 'modified' | 'deleted' | 'renamed';

export interface FSChangeEvent {
  type: FSChangeType;
  path: string;
  oldPath?: string;
}

export interface SessionOutputEvent {
  id: string;
  data: string;
}

export interface SessionLifecycleEvent {
  id: string;
  title?: string;
  cwd?: string;
}

export interface SSHStatusEvent {
  host?: string;
  user?: string;
  message?: string;
  error?: string;
}

export interface BoardUpdatedEvent {
  tasks: BoardTask[];
  timestamp: number;
}

export interface GitStatusEvent {
  branch: string;
  files: GitFileStatus[];
  ahead?: number;
  behind?: number;
}

export interface AgentTurnEvent {
  sessionId: string;
  turnId?: string;
  turnIndex?: number;
}

export interface AgentDeltaEvent {
  sessionId: string;
  delta: string;
  messageId?: string;
}

export interface AgentToolEvent {
  sessionId: string;
  toolId: string;
  name: string;
  args?: Record<string, unknown> | string;
  result?: unknown;
  status?: string;
}

export interface AgentTaskUpdateEvent {
  sessionId: string;
  taskId: string;
  title: string;
  status: string;
}

/**
 * Event-to-Payload map for contract-driven type safety.
 */
export interface WailsEventPayloadMap {
  'fs:changed': FSChangeEvent;
  'browser:frame': { image: string };
  'browser:status': { url?: string; title?: string; loading?: boolean };
  'agent:config:changed': Record<string, unknown>;
  'session:output': SessionOutputEvent;
  'session:opened': SessionLifecycleEvent;
  'session:closed': SessionLifecycleEvent;
  'agentsession:updated': AgentSession;
  'agentsession:deleted': { id: string };
  'ssh:handshake': SSHStatusEvent;
  'ssh:connected': SSHStatusEvent;
  'ssh:disconnected': SSHStatusEvent;
  'ssh:error': SSHStatusEvent;
  'board:updated': BoardUpdatedEvent;
  'git:status': GitStatusEvent;
  'acp:state': Record<string, unknown>;
  'agent:updated': AgentSession;
  'agent:started': { sessionId: string };
  'agent:stopped': { sessionId: string };
  'agent:turn_start': AgentTurnEvent;
  'agent:turn_end': AgentTurnEvent;
  'agent:message_start': { sessionId: string; messageId: string; role: string };
  'agent:message_delta': AgentDeltaEvent;
  'agent:message_end': { sessionId: string; messageId: string };
  'agent:thinking_start': { sessionId: string };
  'agent:thinking_delta': AgentDeltaEvent;
  'agent:thinking_end': { sessionId: string };
  'agent:tool_start': AgentToolEvent;
  'agent:tool_delta': AgentToolEvent;
  'agent:tool_end': AgentToolEvent;
  'agent:ask': { sessionId: string; question: string; options?: string[] };
  'agent:task_update': AgentTaskUpdateEvent;
  'agent:schedule_update': Record<string, unknown>;
  'agent:available_commands': { commands: string[] };
  [key: string]: unknown;
}

/** Standard contract response envelope */
export type IpcResult<T, E = Error> =
  | { success: true; data: T }
  | { success: false; error: E };
