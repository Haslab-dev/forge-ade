import {
  AgentMessage,
  AgentSession,
  FileDiff,
  TextBlock,
  ThoughtStep,
  ToolExecution
} from '../../types';
import { cleanRawToolTags } from '../../lib/utils';
import { newThoughtStep } from '../../services/goAgentSession';
import { ApiBridge } from '../../services/apiBridge';

export interface AgentCallbacksConfig {
  storeSessionId: string;
  goSessionId: string;
  activeWorkspacePath: string;
  addDiff: (diff: FileDiff) => void;
  updateSession: (sessionId: string, updater: (s: AgentSession) => AgentSession) => void;
}

export function createAgentCallbacks(config: AgentCallbacksConfig) {
  const { storeSessionId, goSessionId, activeWorkspacePath, addDiff, updateSession } = config;

  let currentThought: ThoughtStep | null = null;
  let currentTextBlock: TextBlock | null = null;
  let itemSeq = 0;
  let pendingText = '';
  let pendingTextTurn = 1;
  let thoughtDirty = false;
  let textDirty = false;
  let flushTimer: number | null = null;

  const updateLastAgentMsg = (fn: (m: AgentMessage) => AgentMessage) => {
    updateSession(storeSessionId, s => {
      let targetIdx = -1;
      for (let i = s.messages.length - 1; i >= 0; i--) {
        if (s.messages[i].role === 'agent') {
          targetIdx = i;
          break;
        }
      }
      if (targetIdx === -1) return s;
      const msgs = s.messages.map((m, idx) => {
        if (idx === targetIdx) return fn(m);
        return m;
      });
      return { ...s, messages: msgs };
    });
  };

  const flushPending = () => {
    flushTimer = null;
    const chunk = pendingText;
    const chunkTurn = pendingTextTurn;
    const step = thoughtDirty ? currentThought : null;
    const textBlock = textDirty ? currentTextBlock : null;
    if (!chunk && !step && !textBlock) return;
    pendingText = '';
    thoughtDirty = false;
    textDirty = false;
    updateLastAgentMsg(m => {
      const next = { ...m };
      if (chunk) {
        next.content = (next.content || '') + chunk;
        const turnTexts = { ...(next.turnTexts || {}) };
        turnTexts[chunkTurn] = (turnTexts[chunkTurn] || '') + chunk;
        next.turnTexts = turnTexts;
        next.isThinking = false;
      }
      if (textBlock) {
        const blocks = next.textBlocks || [];
        const exists = blocks.some(b => b.id === textBlock.id);
        next.textBlocks = exists
          ? blocks.map(b => (b.id === textBlock.id ? textBlock : b))
          : [...blocks, textBlock];
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

  return {
    onThinkingDelta: (delta: string, turn?: number) => {
      const turnNum = turn || 1;
      currentTextBlock = null;
      if (!currentThought || currentThought.turn !== turnNum) {
        itemSeq++;
        currentThought = newThoughtStep(delta, turnNum);
        currentThought.seq = itemSeq;
        currentThought.createdAtMs = Date.now();
      } else {
        currentThought = {
          ...currentThought,
          thoughtText: currentThought.thoughtText + delta,
          durationSeconds: Math.max(
            1,
            Math.round((Date.now() - (currentThought.createdAtMs || Date.now())) / 1000)
          )
        };
      }
      thoughtDirty = true;
      scheduleFlush();
    },

    onTextDelta: (delta: string, turn?: number) => {
      currentThought = null;
      const turnNum = turn || 1;
      pendingTextTurn = turnNum;
      pendingText += delta;

      if (!currentTextBlock || currentTextBlock.turn !== turnNum) {
        itemSeq++;
        currentTextBlock = {
          id: `tb-${Date.now()}-${itemSeq}`,
          text: delta,
          turn: turnNum,
          createdAtMs: Date.now(),
          seq: itemSeq
        };
      } else {
        currentTextBlock = {
          ...currentTextBlock,
          text: currentTextBlock.text + delta
        };
      }
      textDirty = true;
      scheduleFlush();
    },

    onToolUpsert: (tool: ToolExecution) => {
      currentThought = null;
      currentTextBlock = null;
      updateLastAgentMsg(m => {
        const tools = m.toolExecutions || [];
        const existing = tools.find(t => t.id === tool.id);
        let updated: ToolExecution[];
        if (existing) {
          const merged: ToolExecution = {
            ...existing,
            ...tool,
            toolName:
              tool.toolName && tool.toolName !== 'tool'
                ? tool.toolName
                : existing.toolName || tool.toolName,
            command:
              tool.command && tool.command !== '{}' && tool.command !== '{ }' && tool.command !== ''
                ? tool.command
                : existing.command || tool.command,
            output: tool.output !== undefined ? tool.output : existing.output,
            createdAtMs: existing.createdAtMs || tool.createdAtMs || Date.now(),
            seq: existing.seq ?? tool.seq
          };
          updated = tools.map(t => (t.id === tool.id ? merged : t));
        } else {
          itemSeq++;
          const newTool: ToolExecution = {
            ...tool,
            seq: tool.seq ?? itemSeq,
            createdAtMs: tool.createdAtMs || Date.now()
          };
          updated = [...tools, newTool];
        }
        return { ...m, toolExecutions: updated, isThinking: false };
      });
    },

    onToolRemove: (toolId: string) => {
      updateLastAgentMsg(m => ({
        ...m,
        toolExecutions: (m.toolExecutions || []).filter(t => t.id !== toolId)
      }));
    },

    onDiffCreated: (diff: FileDiff) => {
      addDiff(diff);
      updateSession(storeSessionId, s => {
        const kept = (s.diffs || []).filter(d => d.filePath !== diff.filePath);
        return { ...s, diffs: [...kept, diff].slice(-10) };
      });
    },

    onStateChange: (snap: any) => {
      const state = snap.state || snap.State || '';
      updateSession(storeSessionId, s => {
        if (s.goSessionId !== goSessionId) return s;
        const paused = state === 'awaiting_approval' || state === 'awaiting_input';
        const msgs = paused
          ? s.messages.map((m, idx) =>
              idx === s.messages.length - 1 && m.role === 'agent' ? { ...m, isThinking: false } : m
            )
          : s.messages;
        return {
          ...s,
          goState: state,
          model: (snap as any).model || (snap as any).Model || s.model,
          pendingTools: Array.isArray(snap.pending_tools)
            ? snap.pending_tools
            : Array.isArray(snap.PendingTools)
            ? snap.PendingTools
            : [],
          pendingQuestions: Array.isArray(snap.pending_questions)
            ? snap.pending_questions
            : Array.isArray(snap.PendingQuestions)
            ? snap.PendingQuestions
            : [],
          contextTokens:
            (snap as any).contextTokens ?? (snap as any).ContextTokens ?? s.contextTokens,
          cacheHitRate:
            (snap as any).cacheHitRate ?? (snap as any).CacheHitRate ?? s.cacheHitRate,
          tasks: (Array.isArray(snap.tasks)
            ? snap.tasks
            : Array.isArray(snap.Tasks)
            ? snap.Tasks
            : s.tasks || []
          ).map((t: any, i: number) => ({
            ...t,
            id: t.id || t.ID || `t${i + 1}`,
            title: t.title || t.Title || '',
            status: t.status || t.Status || (t.completed || t.Completed ? 'completed' : 'pending'),
            completed: !!(
              t.completed ||
              t.Completed ||
              t.status === 'completed' ||
              t.Status === 'completed'
            )
          })),
          progress: snap.progress ?? snap.Progress ?? s.progress,
          parentSessionId: snap.parent_session_id ?? snap.ParentSessionID ?? s.parentSessionId,
          subagentIds: Array.isArray(snap.subagent_ids)
            ? snap.subagent_ids
            : Array.isArray(snap.SubagentIDs)
            ? snap.SubagentIDs
            : s.subagentIds || [],
          messages: msgs
        };
      });
    },

    onTurnEnd: (error?: string) => {
      if (flushTimer !== null) {
        window.clearTimeout(flushTimer);
        flushTimer = null;
      }
      flushPending();
      currentThought = null;
      const cancelled = !!error && /context canceled/i.test(error);

      updateSession(storeSessionId, s => {
        let targetIdx = -1;
        for (let i = s.messages.length - 1; i >= 0; i--) {
          if (s.messages[i].role === 'agent') {
            targetIdx = i;
            break;
          }
        }
        const msgs = s.messages.map((m, idx) => {
          if (idx === targetIdx) {
            const cleanedContent = cleanRawToolTags(m.content || '');
            const cleanedTurnTexts: Record<number, string> = {};
            if (m.turnTexts) {
              for (const [k, v] of Object.entries(m.turnTexts)) {
                cleanedTurnTexts[Number(k)] = cleanRawToolTags(v || '');
              }
            }
            if (cancelled) {
              const base = cleanedContent
                .replace(/\s*\(\*?stopped by user\*?\)/gi, '')
                .replace(/\s*\(Execution stopped by user\)/gi, '')
                .trim();
              return {
                ...m,
                content: base ? `${base}\n\n*(stopped by user)*` : '*(stopped by user)*',
                turnTexts: cleanedTurnTexts,
                isThinking: false
              };
            }
            if (error) {
              return {
                ...m,
                content: `${cleanedContent}${cleanedContent ? '\n\n' : ''}⚠️ **Error**: ${error}`,
                turnTexts: cleanedTurnTexts,
                isThinking: false
              };
            }
            return {
              ...m,
              content: cleanedContent,
              turnTexts: cleanedTurnTexts,
              isThinking: false
            };
          }
          return m;
        });
        const finishedSession: AgentSession = {
          ...s,
          status: 'completed' as const,
          goState: 'idle',
          pendingTools: [],
          pendingQuestions: [],
          messages: msgs
        };
        ApiBridge.saveSessionJsonl(
          finishedSession,
          finishedSession.workspacePath ?? activeWorkspacePath
        );
        return finishedSession;
      });
    },

    onSteerPreempt: () => {
      if (flushTimer !== null) {
        window.clearTimeout(flushTimer);
        flushTimer = null;
      }
      flushPending();
      currentThought = null;
    }
  };
}
