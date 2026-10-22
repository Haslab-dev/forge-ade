import { AgentSession, AgentMessage, ThoughtStep, ToolExecution, TextBlock } from '../../types';

// The Go agent harness (internal agents) persists block-based sessions into
// the same ~/.forge/sessions/<project>/ directory the frontend JSONL store
// uses: message content is an array of typed blocks and roles are
// assistant/tool. Those files are the harness transcript, not frontend chat
// sessions — detect them so they are never loaded into the chat UI.
export function isGoHarnessSession(s: any): boolean {
  if (!s || !Array.isArray(s.messages)) return false;
  if (s.role_filter !== undefined || s.roleFilter !== undefined) return true;
  return s.messages.some((m: any) => m != null && Array.isArray(m.content));
}

// Defensive normalization: coerce any non-string message content (e.g. a
// legacy block array that slipped through) into plain text so the
// string-based chat renderer can never crash on it. Also cleans up
// orphaned isThinking flags and unclosed running states from prior runs.
export function normalizeLoadedSession(s: any): AgentSession | null {
  if (!s || typeof s !== 'object' || !Array.isArray(s.messages)) return null;
  const messages = s.messages.map((m: any) => {
    let text = '';
    if (m == null) return m;
    if (typeof m.content === 'string') {
      text = m.content;
    } else if (Array.isArray(m.content)) {
      text = m.content
        .filter((b: any) => b && b.type === 'text' && typeof b.text === 'string')
        .map((b: any) => b.text)
        .join('');
    } else if (m.content !== undefined && m.content !== null) {
      text = String(m.content);
    }
    // Startup sanitization: clear orphaned isThinking flags from prior runs
    let isThinking = !!m.isThinking;
    if (isThinking) {
      isThinking = false;
      if (!text || text.trim() === '') {
        text = '*(turn interrupted by restart)*';
      }
    }
    return { ...m, content: text, isThinking };
  });

  const status = s.status === 'running' ? 'idle' : (s.status || 'idle');
  const goState = s.goState === 'thinking' || s.goState === 'executing' ? 'idle' : (s.goState || 'idle');

  return {
    ...s,
    status,
    goState,
    messages,
    tasks: (Array.isArray(s.tasks) ? s.tasks : (Array.isArray(s.Tasks) ? s.Tasks : [])).map((t: any, i: number) => ({
      ...t,
      id: t.id || t.ID || `t${i + 1}`,
      title: t.title || t.Title || '',
      status: t.status || t.Status || (t.completed || t.Completed ? 'completed' : 'pending'),
      completed: !!(t.completed || t.Completed || t.status === 'completed' || t.Status === 'completed'),
    })),
    progress: s.progress || s.Progress,
    parentSessionId: s.parentSessionId || s.parent_session_id || s.ParentSessionID,
    subagentIds: Array.isArray(s.subagentIds) ? s.subagentIds : (Array.isArray(s.subagent_ids) ? s.subagent_ids : (Array.isArray(s.SubagentIDs) ? s.SubagentIDs : [])),
    pendingTools: Array.isArray(s.pendingTools) ? s.pendingTools : (Array.isArray(s.pending_tools) ? s.pending_tools : (Array.isArray(s.PendingTools) ? s.PendingTools : [])),
  } as AgentSession;
}

export function convertGoSessionToAgentSession(snap: any): AgentSession {
  const msgs: AgentMessage[] = [];
  if (Array.isArray(snap.messages)) {
    // Map tool_call_id -> result block for matching outputs
    const toolResults = new Map<string, { output: string; isError: boolean }>();
    for (const m of snap.messages) {
      if (!m) continue;
      if ((m.role === 'tool' || m.role === 'system') && Array.isArray(m.content)) {
        for (const block of m.content) {
          if (block && (block.type === 'tool_result' || block.ToolCallID) && (block.tool_call_id || block.ToolCallID)) {
            const id = block.tool_call_id || block.ToolCallID;
            toolResults.set(id, {
              output: typeof block.text === 'string' ? block.text : (typeof block.Text === 'string' ? block.Text : JSON.stringify(block.text ?? block.Text ?? '')),
              isError: !!(block.is_error ?? block.IsError),
            });
          }
        }
      }
    }

    let currentAgentMsg: AgentMessage | null = null;
    let agentTurnCount = 0;

    for (let i = 0; i < snap.messages.length; i++) {
      const m = snap.messages[i];
      if (!m) continue;

      if (m.role === 'tool') {
        // Handled via toolResults map
        continue;
      }

      if (m.role === 'user') {
        currentAgentMsg = null;
        agentTurnCount = 0;
        let text = '';
        if (typeof m.content === 'string') {
          text = m.content;
        } else if (Array.isArray(m.content)) {
          for (const b of m.content) {
            if (b && (b.type === 'text' || b.Type === 'text')) {
              text += (b.text || b.Text || '');
            }
          }
        }
        msgs.push({
          id: m.id || m.ID || `msg-${i}`,
          role: 'user',
          content: text,
          timestamp: m.timestamp || m.Timestamp ? new Date(m.timestamp || m.Timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) : new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
        });
        continue;
      }

      // If already a persisted AgentMessage with turnTexts or toolExecutions
      if (m.turnTexts || (Array.isArray(m.toolExecutions) && (m.role === 'agent' || m.role === 'assistant'))) {
        currentAgentMsg = null;
        agentTurnCount = 0;
        msgs.push({
          ...m,
          role: 'agent',
        });
        continue;
      }

      // Assistant message from Go
      if (m.role === 'assistant' || m.role === 'agent') {
        agentTurnCount++;
        const turnNum = agentTurnCount;
        let iterText = '';
        const iterThoughts: ThoughtStep[] = [];
        const iterTools: ToolExecution[] = [];
        const iterTextBlocks: TextBlock[] = [];
        let seqCounter = 0;

        if (typeof m.content === 'string') {
          iterText = m.content;
          if (iterText.trim()) {
            seqCounter++;
            iterTextBlocks.push({
              id: `tb-${i}-${seqCounter}`,
              text: iterText,
              turn: turnNum,
              seq: seqCounter,
              createdAtMs: Date.now() + seqCounter,
            });
          }
        } else if (Array.isArray(m.content)) {
          for (const block of m.content) {
            if (!block) continue;
            const bType = block.type || block.Type;
            const bText = block.text || block.Text;
            if (bType === 'text' && typeof bText === 'string') {
              iterText += bText;
              seqCounter++;
              iterTextBlocks.push({
                id: `tb-${i}-${seqCounter}`,
                text: bText,
                turn: turnNum,
                seq: seqCounter,
                createdAtMs: Date.now() + seqCounter,
              });
            } else if (bType === 'thinking' && typeof bText === 'string') {
              seqCounter++;
              iterThoughts.push({
                id: `th-${i}-${iterThoughts.length}`,
                thoughtText: bText,
                durationSeconds: 2,
                timestamp: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
                turn: turnNum,
                seq: seqCounter,
                createdAtMs: Date.now() + seqCounter,
              });
            } else if (bType === 'tool_call') {
              seqCounter++;
              const callId = block.tool_call_id || block.ToolCallID || block.id || block.ID || `tool-${i}-${iterTools.length}`;
              const res = toolResults.get(callId);
              const args = block.arguments || block.Arguments;
              iterTools.push({
                id: callId,
                toolName: block.name || block.Name || 'tool',
                command: typeof args === 'string' ? args : JSON.stringify(args || {}),
                output: res?.output || '',
                status: res ? (res.isError ? 'failed' : 'completed') : 'completed',
                turn: turnNum,
                seq: seqCounter,
                createdAtMs: Date.now() + seqCounter,
              });
            }
          }
        }

        if (currentAgentMsg) {
          // Append to existing agent message for this user query
          currentAgentMsg.content = (currentAgentMsg.content || '') + (iterText ? '\n\n' + iterText : '');
          currentAgentMsg.thoughts = [...(currentAgentMsg.thoughts || []), ...iterThoughts];
          currentAgentMsg.toolExecutions = [...(currentAgentMsg.toolExecutions || []), ...iterTools];
          currentAgentMsg.textBlocks = [...(currentAgentMsg.textBlocks || []), ...iterTextBlocks];
          if (iterText) {
            currentAgentMsg.turnTexts = {
              ...(currentAgentMsg.turnTexts || {}),
              [turnNum]: iterText,
            };
          }
        } else {
          currentAgentMsg = {
            id: m.id || m.ID || `msg-${i}`,
            role: 'agent',
            content: iterText,
            timestamp: m.timestamp || m.Timestamp ? new Date(m.timestamp || m.Timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }) : new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
            thoughts: iterThoughts,
            toolExecutions: iterTools,
            textBlocks: iterTextBlocks,
            turnTexts: iterText ? { [turnNum]: iterText } : {},
          };
          msgs.push(currentAgentMsg);
        }
      }
    }
  }

  return {
    id: snap.id || snap.ID,
    title: snap.name || snap.Name || 'Agent Session',
    createdAt: snap.created_at || snap.CreatedAt || new Date().toISOString(),
    updatedAt: snap.updated_at || snap.UpdatedAt || new Date().toISOString(),
    status: snap.state === 'executing' || snap.state === 'thinking' ? 'running' : 'idle',
    model: snap.model || snap.Model || '',
    agentId: snap.id || snap.ID,
    workspacePath: snap.folder || snap.Folder || '',
    messages: msgs,
    goSessionId: snap.id || snap.ID,
    goState: snap.state || snap.State || 'idle',
    pendingTools: snap.pending_tools || snap.PendingTools || [],
    tasks: (Array.isArray(snap.tasks) ? snap.tasks : (Array.isArray(snap.Tasks) ? snap.Tasks : [])).map((t: any, i: number) => ({
      ...t,
      id: t.id || t.ID || `t${i + 1}`,
      title: t.title || t.Title || '',
      status: t.status || t.Status || (t.completed || t.Completed ? 'completed' : 'pending'),
      completed: !!(t.completed || t.Completed || t.status === 'completed' || t.Status === 'completed'),
    })),
    progress: snap.progress || snap.Progress,
    parentSessionId: snap.parent_session_id || snap.ParentSessionID,
    subagentIds: Array.isArray(snap.subagent_ids) ? snap.subagent_ids : (Array.isArray(snap.SubagentIDs) ? snap.SubagentIDs : []),
    contextTokens: snap.contextTokens || snap.ContextTokens || 0,
    cacheHitRate: snap.cacheHitRate || snap.CacheHitRate || 0,
    tokenUsage: snap.token_usage || snap.TokenUsage,
  };
}
