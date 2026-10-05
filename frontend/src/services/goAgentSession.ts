// Go agent harness chat client — drives internal agents through the Go
// backend (native tool calling, read/edit/write tools applied on disk,
// approval gate) instead of the webview LLM loop. Mirrors the harness chat
// behavior: streaming text/thinking deltas, granular tool events, approval
// pauses, and turn boundaries.
import { CreateAgentSession, SendAgentMessage, GetAgentSession, EventsOn } from '../lib/wails';
import { ApiBridge } from './apiBridge';
import type { FileDiff, ThoughtStep, ToolExecution } from '../types';

export interface GoAgentCallbacks {
  onThinkingDelta: (delta: string) => void;
  onTextDelta: (delta: string) => void;
  onToolUpsert: (tool: ToolExecution) => void;
  onToolRemove: (toolId: string) => void;
  onDiffCreated: (diff: FileDiff) => void;
  onStateChange: (goSession: any) => void;
  onTurnEnd: (error?: string) => void;
}

// ---------------------------------------------------------------------------
// Unified-diff reverse patching — reconstructs the pre-edit file content from
// the post-edit content plus the tool's unified diff, so the review pane can
// show a true before/after for harness edits (which apply immediately, like
// the harness reference: nothing is "staged").
// ---------------------------------------------------------------------------

interface Hunk {
  oldStart: number; // 1-based
  oldLines: number;
  newStart: number; // 1-based
  newLines: number;
  oldText: string[];
  newText: string[];
}

function parseHunks(patch: string): Hunk[] {
  const hunks: Hunk[] = [];
  const lines = patch.split('\n');
  let cur: Hunk | null = null;
  for (const line of lines) {
    const m = line.match(/^@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@/);
    if (m) {
      cur = {
        oldStart: parseInt(m[1], 10),
        oldLines: m[2] !== undefined ? parseInt(m[2], 10) : 1,
        newStart: parseInt(m[3], 10),
        newLines: m[4] !== undefined ? parseInt(m[4], 10) : 1,
        oldText: [],
        newText: [],
      };
      hunks.push(cur);
      continue;
    }
    if (!cur) continue;
    if (line.startsWith('---') || line.startsWith('+++')) continue;
    if (line.startsWith('\\')) continue; // "\ No newline at end of file"
    if (line.startsWith('+')) {
      cur.newText.push(line.slice(1));
    } else if (line.startsWith('-')) {
      cur.oldText.push(line.slice(1));
    } else if (line.startsWith(' ') || line === '') {
      cur.oldText.push(line.slice(1));
      cur.newText.push(line.slice(1));
    }
  }
  return hunks;
}

// reversePatch applies each hunk backwards onto the modified content.
// New-side line numbers locate the position exactly, so hunks are applied
// bottom-up to keep earlier offsets valid.
export function reversePatch(modified: string, patch: string): string {
  let hunks: Hunk[];
  try {
    hunks = parseHunks(patch);
  } catch {
    return modified;
  }
  if (hunks.length === 0) return modified;
  const modLines = modified.split('\n');
  for (let i = hunks.length - 1; i >= 0; i--) {
    const h = hunks[i];
    const start = h.newStart - 1;
    if (start < 0 || start > modLines.length) continue;
    modLines.splice(start, h.newLines, ...h.oldText);
  }
  return modLines.join('\n');
}

// ---------------------------------------------------------------------------
// Manager — one instance, global event subscriptions, per-session callbacks.
// ---------------------------------------------------------------------------

class GoAgentSessionManager {
  private callbacks = new Map<string, GoAgentCallbacks>();
  private subscribed = false;
  // Streaming tool-call previews by provider tool-call index, per go session.
  private streamPreviews = new Map<string, Map<number, ToolExecution>>();
  private syncTimers = new Map<string, ReturnType<typeof setTimeout>>();

  private ensureSubscribed() {
    if (this.subscribed) return;
    this.subscribed = true;

    EventsOn('agent:thinking_delta', (data: any) => {
      const cb = this.callbacks.get(data?.session_id);
      if (cb && data.delta) cb.onThinkingDelta(data.delta);
    });

    EventsOn('agent:message_delta', (data: any) => {
      const cb = this.callbacks.get(data?.session_id);
      if (cb && data.delta && data.kind === 'text') cb.onTextDelta(data.delta);
    });

    EventsOn('agent:tool_delta', (data: any) => {
      const cb = this.callbacks.get(data?.session_id);
      if (!cb) return;
      // Live preview row for the streaming tool call (args may be partial).
      const idx = typeof data.index === 'number' ? data.index : 0;
      let previews = this.streamPreviews.get(data.session_id);
      if (!previews) {
        previews = new Map();
        this.streamPreviews.set(data.session_id, previews);
      }
      const id = `go-stream-${idx}`;
      const existing = previews.get(idx);
      const argsText = typeof data.args === 'string' ? data.args : JSON.stringify(data.args ?? {});
      const tool: ToolExecution = {
        id,
        toolName: data.name || existing?.toolName || 'tool',
        command: argsText,
        status: 'running',
        createdAtMs: existing?.createdAtMs || Date.now(),
      };
      previews.set(idx, tool);
      cb.onToolUpsert(tool);
    });

    EventsOn('agent:tool_end', (data: any) => {
      const cb = this.callbacks.get(data?.session_id);
      if (!cb) return;
      // Drop the streaming preview row: tool_end has no batch index, so match
      // the oldest preview with the same tool name.
      const previews = this.streamPreviews.get(data.session_id);
      if (previews) {
        for (const [idx, preview] of previews) {
          if (preview.toolName === (data.name || 'tool')) {
            cb.onToolRemove(preview.id);
            previews.delete(idx);
            break;
          }
        }
      }
      const display = data.display || {};
      const args = data.args ?? {};
      const toolId = data.tool_call_id || `go-tool-${Date.now()}`;
      const name = data.name || 'tool';

      const tool: ToolExecution = {
        id: toolId,
        toolName: name,
        command: typeof args === 'string' ? args : JSON.stringify(args),
        output: data.result || '',
        status: data.is_error ? 'failed' : 'completed',
        createdAtMs: Date.now(),
      };

      // Attach a review diff for file tools (applied immediately by the
      // harness — status 'accepted' so no Accept/Reject buttons appear).
      if (display.diff && display.path) {
        void this.buildDiffForTool(display).then(diff => {
          if (diff) {
            tool.diff = diff;
            cb.onDiffCreated(diff);
          }
          cb.onToolUpsert(tool);
        });
        return;
      }
      cb.onToolUpsert(tool);
    });

    EventsOn('agent:turn_end', (data: any) => {
      const cb = this.callbacks.get(data?.session_id);
      // Flush any streaming preview rows left over from the turn.
      const previews = this.streamPreviews.get(data?.session_id);
      if (cb && previews) {
        for (const preview of previews.values()) cb.onToolRemove(preview.id);
        this.streamPreviews.delete(data?.session_id);
      }
      if (cb) cb.onTurnEnd(data?.error || undefined);
    });

    EventsOn('agent:updated', (data: any) => {
      const id = data?.session_id;
      if (!id || !this.callbacks.has(id)) return;
      // Coalesce bursts of session updates into one snapshot fetch.
      const pending = this.syncTimers.get(id);
      if (pending) clearTimeout(pending);
      this.syncTimers.set(id, setTimeout(async () => {
        this.syncTimers.delete(id);
        try {
          const snap = await GetAgentSession(id);
          const cb = this.callbacks.get(id);
          if (snap && cb) cb.onStateChange(snap);
        } catch { /* session gone */ }
      }, 300));
    });
  }

  // Reconstructs a FileDiff for an applied write/edit: the on-disk file is the
  // modified side; reverse-applying the tool's unified diff yields the original.
  private async buildDiffForTool(display: any): Promise<FileDiff | null> {
    try {
      const path = String(display.path);
      const modified = await ApiBridge.readFile(path);
      const patch = String(display.diff);
      const original = display.status === 'written'
        ? ''
        : reversePatch(modified, patch);
      const fileName = path.split('/').pop() || path;
      return {
        id: `go-diff-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`,
        filePath: path,
        fileName,
        originalContent: original,
        modifiedContent: modified,
        additions: typeof display.additions === 'number' ? display.additions : 0,
        deletions: typeof display.deletions === 'number' ? display.deletions : 0,
        status: 'accepted',
        timestamp: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
        kind: 'agent',
      };
    } catch {
      return null;
    }
  }

  register(goSessionId: string, callbacks: GoAgentCallbacks) {
    this.ensureSubscribed();
    this.callbacks.set(goSessionId, callbacks);
  }

  unregister(goSessionId: string) {
    this.callbacks.delete(goSessionId);
  }

  // createSession provisions the Go-side session for a store session.
  async createSession(title: string, workspacePath: string): Promise<string | null> {
    try {
      const created = await CreateAgentSession((title || 'Agent').slice(0, 40), 'coding', workspacePath || '');
      return created?.id || created?.ID || null;
    } catch {
      return null;
    }
  }

  async send(goSessionId: string, prompt: string, mentionedPaths: string[]): Promise<void> {
    await SendAgentMessage(goSessionId, prompt, mentionedPaths || []);
  }
}

export const goAgentSessions = new GoAgentSessionManager();

// Extract @-mentioned workspace paths so the harness can inline them.
export function extractMentionedPaths(prompt: string, workspacePath: string): string[] {
  const out: string[] = [];
  for (const m of prompt.matchAll(/(?:^|\s)@([^\s]+)/g)) {
    const rel = m[1];
    if (!rel || rel.startsWith('/')) continue;
    const abs = workspacePath ? `${workspacePath}/${rel}` : rel;
    if (!out.includes(abs)) out.push(abs);
  }
  return out;
}

// Helper for stores: builds a ThoughtStep-shaped accumulator for one turn.
export function newThoughtStep(delta: string): ThoughtStep {
  const now = Date.now();
  return {
    id: `go-thought-${now}`,
    thoughtText: delta,
    durationSeconds: 1,
    timestamp: new Date().toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' }),
    createdAtMs: now,
  };
}
