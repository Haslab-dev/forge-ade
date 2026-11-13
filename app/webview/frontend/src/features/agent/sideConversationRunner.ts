import {
  AgentMessage,
  AgentSession,
  FileDiff,
  ThoughtStep,
  ToolExecution
} from '../../types';
import { GitFileStatus } from '../git/git.api';
import { AgentEngine } from '../../services/agentEngine';

export interface RunSideConversationOptions {
  promptText: string;
  model: string;
  mode?: 'ask' | 'plan' | 'full';
  activeSession: AgentSession;
  activeWorkspacePath: string;
  diffs: FileDiff[];
  gitFiles: GitFileStatus[];
  gitBranch: string;
  engine: AgentEngine;
  updateMessages: (updater: (prev: AgentMessage[]) => AgentMessage[]) => void;
}

export async function runSideConversationPrompt(
  opts: RunSideConversationOptions
): Promise<void> {
  const {
    promptText,
    model,
    mode = 'full',
    activeSession,
    activeWorkspacePath,
    diffs,
    gitFiles,
    gitBranch,
    engine,
    updateMessages
  } = opts;

  if (!promptText.trim()) return;

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

  updateMessages(prev => [...prev, userMsg, agentMsg]);

  try {
    const conversationHistory = (activeSession.sideConversationMessages || []).map(m => ({
      role: m.role === 'user' ? ('user' as const) : ('assistant' as const),
      content: m.content
    }));
    conversationHistory.push({ role: 'user', content: promptText });

    let accumulated = '';
    const sessionDiffs = activeSession.diffs || diffs || [];

    await engine.streamSideChat(
      model,
      conversationHistory,
      {
        workspacePath: activeWorkspacePath || '',
        activeTaskTitle: activeSession.title,
        diffs: sessionDiffs,
        gitFiles,
        gitBranch
      },
      {
        onChunk: (deltaChunk: string) => {
          accumulated += deltaChunk;
          updateMessages(prev => {
            const sideMsgs = [...prev];
            const targetIdx = sideMsgs.findIndex(m => m.id === agentMsgId);
            if (targetIdx !== -1) {
              sideMsgs[targetIdx] = {
                ...sideMsgs[targetIdx],
                content: accumulated,
                isThinking: false,
                toolStatus: undefined
              };
            }
            return sideMsgs;
          });
        },
        onThought: (thought: ThoughtStep) => {
          updateMessages(prev => {
            const sideMsgs = [...prev];
            const targetIdx = sideMsgs.findIndex(m => m.id === agentMsgId);
            if (targetIdx !== -1) {
              const thoughts = sideMsgs[targetIdx].thoughts || [];
              const existingIdx = thoughts.findIndex(t => t.id === thought.id);
              const updatedThoughts =
                existingIdx >= 0
                  ? thoughts.map((t, i) => (i === existingIdx ? { ...t, ...thought } : t))
                  : [...thoughts, thought];
              sideMsgs[targetIdx] = {
                ...sideMsgs[targetIdx],
                thoughts: updatedThoughts,
                isThinking: true
              };
            }
            return sideMsgs;
          });
        },
        onToolStart: (tool: ToolExecution) => {
          updateMessages(prev => {
            const sideMsgs = [...prev];
            const targetIdx = sideMsgs.findIndex(m => m.id === agentMsgId);
            if (targetIdx !== -1) {
              const tools = sideMsgs[targetIdx].toolExecutions || [];
              const existingIdx = tools.findIndex(t => t.id === tool.id);
              const updatedTools =
                existingIdx >= 0
                  ? tools.map((t, i) => (i === existingIdx ? { ...t, ...tool } : t))
                  : [...tools, tool];
              sideMsgs[targetIdx] = {
                ...sideMsgs[targetIdx],
                toolExecutions: updatedTools,
                isThinking: false
              };
            }
            return sideMsgs;
          });
        },
        onToolComplete: (tool: ToolExecution) => {
          updateMessages(prev => {
            const sideMsgs = [...prev];
            const targetIdx = sideMsgs.findIndex(m => m.id === agentMsgId);
            if (targetIdx !== -1) {
              const tools = sideMsgs[targetIdx].toolExecutions || [];
              const updated = tools.some(t => t.id === tool.id)
                ? tools.map(t => (t.id === tool.id ? { ...t, ...tool } : t))
                : [...tools, tool];
              sideMsgs[targetIdx] = {
                ...sideMsgs[targetIdx],
                toolExecutions: updated
              };
            }
            return sideMsgs;
          });
        },
        onToolStatus: (toolStatus: string) => {
          updateMessages(prev => {
            const sideMsgs = [...prev];
            const targetIdx = sideMsgs.findIndex(m => m.id === agentMsgId);
            if (targetIdx !== -1) {
              sideMsgs[targetIdx] = {
                ...sideMsgs[targetIdx],
                toolStatus: toolStatus || undefined,
                isThinking: !!toolStatus
              };
            }
            return sideMsgs;
          });
        }
      },
      undefined,
      mode
    );
  } catch (e: any) {
    console.error('Side conversation streaming error:', e);
    const errorMessage = `⚠️ Error: ${e.message || 'Connection failed'}`;
    updateMessages(prev => {
      const sideMsgs = [...prev];
      const targetIdx = sideMsgs.findIndex(m => m.id === agentMsgId);
      if (targetIdx !== -1) {
        sideMsgs[targetIdx] = {
          ...sideMsgs[targetIdx],
          content: errorMessage,
          isThinking: false
        };
      }
      return sideMsgs;
    });
  }
}
