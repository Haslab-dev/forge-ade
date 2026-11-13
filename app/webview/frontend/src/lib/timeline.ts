import type { AgentMessage, ThoughtStep, ToolExecution } from '../types';
import { cleanPiBanner } from './utils';

export function chatText(content: unknown): string {
  if (typeof content === 'string') return content;
  if (Array.isArray(content)) {
    return content
      .map(part => {
        if (typeof part === 'string') return part;
        if (part && typeof part === 'object') {
          const text = (part as { text?: unknown }).text;
          if (typeof text === 'string') return text;
          const contentField = (part as { content?: unknown }).content;
          if (typeof contentField === 'string') return contentField;
        }
        return '';
      })
      .filter(Boolean)
      .join('');
  }
  return '';
}

export interface TimelineItem {
  id: string;
  turn: number;
  kind: 'thought' | 'text' | 'tool';
  thought?: ThoughtStep;
  text?: string;
  tool?: ToolExecution;
  order: number;
}

export function getChronologicalTimeline(msg: AgentMessage): TimelineItem[] {
  const items: TimelineItem[] = [];
  const thoughts = msg.thoughts || [];
  const tools = msg.toolExecutions || [];
  const textBlocks = msg.textBlocks || [];
  const turnTexts = msg.turnTexts || {};

  const hasSeq = thoughts.some(th => th.seq !== undefined) ||
                 tools.some(t => t.seq !== undefined) ||
                 textBlocks.some(tb => tb.seq !== undefined);

  // 1. Thoughts
  thoughts.forEach((th, idx) => {
    const turn = th.turn || 1;
    const order = hasSeq
      ? (th.seq ?? (th.createdAtMs ?? (turn * 1000 + 10 + idx)))
      : (th.createdAtMs && th.createdAtMs > 0 ? th.createdAtMs : (turn * 1000 + 10 + idx));
    items.push({
      id: th.id || `th-${turn}-${idx}`,
      turn,
      kind: 'thought',
      thought: th,
      order,
    });
  });

  // 2. Text (Use interleaved textBlocks if available, otherwise turnTexts or msg.content)
  if (textBlocks.length > 0) {
    textBlocks.forEach((tb, idx) => {
      const turn = tb.turn || 1;
      const cleaned = cleanPiBanner(chatText(tb.text || ''));
      if (cleaned && cleaned.trim()) {
        const order = hasSeq
          ? (tb.seq ?? (tb.createdAtMs ?? (turn * 1000 + 20 + idx)))
          : (tb.createdAtMs && tb.createdAtMs > 0 ? tb.createdAtMs : (turn * 1000 + 20 + idx));
        items.push({
          id: tb.id || `tb-${turn}-${idx}`,
          turn,
          kind: 'text',
          text: cleaned,
          order,
        });
      }
    });
  } else {
    const turnKeys = Object.keys(turnTexts).map(Number).filter(n => !isNaN(n)).sort((a, b) => a - b);
    if (turnKeys.length > 0) {
      turnKeys.forEach(turn => {
        const raw = turnTexts[turn];
        const cleaned = cleanPiBanner(chatText(raw || ''));
        if (cleaned && cleaned.trim()) {
          items.push({
            id: `text-${turn}`,
            turn,
            kind: 'text',
            text: cleaned,
            order: turn * 1000 + 20,
          });
        }
      });
    } else {
      const cleaned = cleanPiBanner(chatText(msg.content || ''));
      if (cleaned && cleaned.trim()) {
        const maxTurn = Math.max(1, ...tools.map(t => t.turn || 1));
        const textTurn = tools.length > 0 ? maxTurn : 1;
        items.push({
          id: `text-fallback`,
          turn: textTurn,
          kind: 'text',
          text: cleaned,
          order: textTurn * 1000 + (tools.length > 0 ? 40 : 20),
        });
      }
    }
  }

  // 3. Tools
  tools.forEach((t, idx) => {
    const turn = t.turn || 1;
    const order = hasSeq
      ? (t.seq ?? (t.createdAtMs ?? (turn * 1000 + 30 + idx)))
      : (t.createdAtMs && t.createdAtMs > 0 ? t.createdAtMs : (turn * 1000 + 30 + idx));
    items.push({
      id: t.id || `tool-${turn}-${idx}`,
      turn,
      kind: 'tool',
      tool: t,
      order,
    });
  });

  // Append error or cancellation notice if not already present in text items
  const cleanedFull = cleanPiBanner(chatText(msg.content || ''));
  if (cleanedFull.includes('*(stopped by user)*') || cleanedFull.includes('⚠️ **Error**:')) {
    const errorNotice = cleanedFull.includes('*(stopped by user)*')
      ? '*(stopped by user)*'
      : cleanedFull.slice(cleanedFull.indexOf('⚠️ **Error**:'));
    const textItems = items.filter(i => i.kind === 'text');
    const lastTextItem = textItems[textItems.length - 1];
    if (lastTextItem && !lastTextItem.text?.includes(errorNotice)) {
      lastTextItem.text = `${lastTextItem.text}\n\n${errorNotice}`;
    } else if (!lastTextItem) {
      items.push({
        id: 'text-error-notice',
        turn: 9999,
        kind: 'text',
        text: errorNotice,
        order: 999999999,
      });
    }
  }

  items.sort((a, b) => a.order - b.order);
  return items;
}
