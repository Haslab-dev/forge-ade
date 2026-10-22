import {
  AgentSession,
  FileDiff,
  ThoughtStep,
  ToolExecution
} from '../../types';
import { GitFileStatus } from '../git/git.api';
import { AgentEngine } from '../../services/agentEngine';
import { ApiBridge } from '../../services/apiBridge';

export interface DirectStreamOptions {
  engine: AgentEngine;
  storeSessionId: string;
  promptText: string;
  sessionOverride?: AgentSession;
  sessions: AgentSession[];
  currentModel: string;
  activeWorkspacePath: string;
  gitFiles: GitFileStatus[];
  gitBranch: string;
  addDiff: (diff: FileDiff) => void;
  updateSession: (sessionId: string, updater: (s: AgentSession) => AgentSession) => void;
}

export async function streamAgentSessionDirectly(opts: DirectStreamOptions): Promise<void> {
  const {
    engine,
    storeSessionId,
    promptText,
    sessionOverride,
    sessions,
    currentModel,
    activeWorkspacePath,
    gitFiles,
    gitBranch,
    addDiff,
    updateSession
  } = opts;

  const storeSession =
    sessionOverride || sessions.find(s => s.id === storeSessionId);
  if (!storeSession) return;

  let targetAgentMsgId = '';
  for (let i = storeSession.messages.length - 1; i >= 0; i--) {
    if (storeSession.messages[i].role === 'agent') {
      targetAgentMsgId = storeSession.messages[i].id;
      break;
    }
  }

  if (!targetAgentMsgId) {
    targetAgentMsgId = `msg-agent-${Date.now()}`;
    updateSession(storeSessionId, s => ({
      ...s,
      messages: [
        ...s.messages,
        {
          id: targetAgentMsgId,
          role: 'agent',
          agentId: 'agent-internal',
          agentName: 'ForgeADE Internal',
          content: '',
          timestamp: new Date().toISOString(),
          isThinking: true,
          thoughts: []
        }
      ]
    }));
  }

  const conversationHistory: { role: string; content: string }[] = [];
  for (const m of storeSession.messages) {
    if (m.id === targetAgentMsgId) continue;
    if (m.role === 'user') {
      conversationHistory.push({ role: 'user', content: m.content || '' });
    } else if (m.role === 'agent' && m.content) {
      conversationHistory.push({ role: 'assistant', content: m.content });
    }
  }
  if (
    conversationHistory.length === 0 ||
    conversationHistory[conversationHistory.length - 1].role !== 'user'
  ) {
    conversationHistory.push({ role: 'user', content: promptText });
  }

  let accumulated = '';
  const sessionDiffs = storeSession.diffs || [];

  try {
    await engine.streamSideChat(
      storeSession.model || currentModel,
      conversationHistory,
      {
        workspacePath: storeSession.workspacePath || activeWorkspacePath || '',
        activeTaskTitle: storeSession.title,
        diffs: sessionDiffs,
        gitFiles,
        gitBranch
      },
      {
        onChunk: (deltaChunk: string) => {
          accumulated += deltaChunk;
          updateSession(storeSessionId, s => {
            const msgs = s.messages.map(m => {
              if (m.id !== targetAgentMsgId) return m;
              return {
                ...m,
                content: accumulated,
                isThinking: false,
                toolStatus: undefined
              };
            });
            return { ...s, messages: msgs };
          });
        },
        onThought: (thought: ThoughtStep) => {
          updateSession(storeSessionId, s => {
            const msgs = s.messages.map(m => {
              if (m.id !== targetAgentMsgId) return m;
              const thoughts = m.thoughts || [];
              const existingIdx = thoughts.findIndex(t => t.id === thought.id);
              const updatedThoughts =
                existingIdx >= 0
                  ? thoughts.map((t, i) => (i === existingIdx ? { ...t, ...thought } : t))
                  : [...thoughts, thought];
              return {
                ...m,
                thoughts: updatedThoughts,
                isThinking: true
              };
            });
            return { ...s, messages: msgs };
          });
        },
        onToolStart: (tool: ToolExecution) => {
          updateSession(storeSessionId, s => {
            const msgs = s.messages.map(m => {
              if (m.id !== targetAgentMsgId) return m;
              const tools = m.toolExecutions || [];
              const existingIdx = tools.findIndex(t => t.id === tool.id);
              const updatedTools =
                existingIdx >= 0
                  ? tools.map((t, i) => (i === existingIdx ? { ...t, ...tool } : t))
                  : [...tools, tool];
              return {
                ...m,
                toolExecutions: updatedTools,
                isThinking: false
              };
            });
            return { ...s, messages: msgs };
          });
        },
        onToolComplete: (tool: ToolExecution) => {
          updateSession(storeSessionId, s => {
            if (tool.diff) {
              addDiff(tool.diff);
            }
            const msgs = s.messages.map(m => {
              if (m.id !== targetAgentMsgId) return m;
              const tools = m.toolExecutions || [];
              const updated = tools.some(t => t.id === tool.id)
                ? tools.map(t => (t.id === tool.id ? { ...t, ...tool } : t))
                : [...tools, tool];
              return {
                ...m,
                toolExecutions: updated
              };
            });
            return { ...s, messages: msgs };
          });
        },
        onToolStatus: (status: string) => {
          updateSession(storeSessionId, s => {
            const msgs = s.messages.map(m => {
              if (m.id !== targetAgentMsgId) return m;
              return {
                ...m,
                toolStatus: status || undefined,
                isThinking: !!status
              };
            });
            return { ...s, messages: msgs };
          });
        }
      },
      undefined,
      'full'
    );
  } catch (err: any) {
    updateSession(storeSessionId, s => {
      const msgs = s.messages.map(m => {
        if (m.id !== targetAgentMsgId) return m;
        return {
          ...m,
          content: (m.content || '') + `\n\n⚠️ **Error**: ${err?.message || String(err)}`,
          isThinking: false
        };
      });
      return { ...s, status: 'completed' as const, messages: msgs };
    });
  } finally {
    updateSession(storeSessionId, s => {
      const msgs = s.messages.map(m => {
        if (m.id === targetAgentMsgId) {
          return { ...m, isThinking: false, toolStatus: undefined };
        }
        return m;
      });
      const finished = { ...s, status: 'completed' as const, goState: 'idle', messages: msgs };
      ApiBridge.saveSessionJsonl(finished, finished.workspacePath ?? activeWorkspacePath);
      return finished;
    });
  }
}
