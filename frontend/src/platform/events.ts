/**
 * Typed events matching backend internal/app/eventbridge.go
 */

export const Events = {
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

  // Agent turn events
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

export type EventName = typeof Events[keyof typeof Events];

export type FSChangeType = 'created' | 'modified' | 'deleted' | 'renamed';

export interface FSChangeEvent {
  type: FSChangeType;
  path: string;
  oldPath?: string;
}

export interface SessionOutputEvent {
  sessionId: string;
  data: string;
}

export interface SSHStatusEvent {
  id: string;
  host: string;
  connected: boolean;
  error?: string;
}
