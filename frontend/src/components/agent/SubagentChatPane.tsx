import React, { useState, useRef, useEffect, useMemo } from 'react';
import {
  Bot,
  Loader2,
  Brain,
  ChevronDown,
  ChevronRight,
  Send,
  Sparkles,
  Copy,
  Check,
  CheckCircle2,
  AlertCircle,
  Square,
} from 'lucide-react';
import { cn } from '../../lib/utils';
import { AgentSession, AgentMessage, ThoughtStep, ToolExecution } from '../../types';
import { ToolExecutionCard } from './ToolBlockViews';
import { MarkdownRenderer } from './MarkdownRenderer';
import { RespondAgentApproval, StopAgentTurn } from '../../lib/wails';

interface SubagentChatPaneProps {
  subagent: AgentSession;
  isRunning: boolean;
  messages: AgentMessage[];
  workspacePath: string;
  onSendMessage: (text: string) => Promise<void> | void;
}

interface WorkItem {
  id: string;
  kind: 'thought' | 'tool';
  thought?: ThoughtStep;
  tool?: ToolExecution;
  time: number;
  order: number;
}

export const SubagentChatPane: React.FC<SubagentChatPaneProps> = ({
  subagent,
  isRunning,
  messages,
  workspacePath,
  onSendMessage,
}) => {
  const [inputText, setInputText] = useState('');
  const [isSending, setIsSending] = useState(false);
  const [copiedMsgId, setCopiedMsgId] = useState<string | null>(null);
  const [expandedWork, setExpandedWork] = useState<Record<string, boolean>>({});
  const [expandedThoughts, setExpandedThoughts] = useState<Record<string, boolean>>({});
  const [expandedTools, setExpandedTools] = useState<Record<string, boolean>>({});

  const chatBottomRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    chatBottomRef.current?.scrollIntoView({ behavior: 'smooth' });
  }, [messages.length, isRunning]);

  const handleCopy = (text: string, id: string) => {
    navigator.clipboard.writeText(text);
    setCopiedMsgId(id);
    setTimeout(() => setCopiedMsgId(null), 2000);
  };

  const handleSend = async (e: React.FormEvent) => {
    e.preventDefault();
    const text = inputText.trim();
    if (!text || isSending) return;
    setInputText('');
    setIsSending(true);
    try {
      await onSendMessage(text);
    } finally {
      setIsSending(false);
    }
  };

  const getWorkItems = (msg: AgentMessage): WorkItem[] => {
    const items: WorkItem[] = [];
    (msg.thoughts || []).forEach((th, idx) => {
      items.push({
        id: th.id || `th-${idx}`,
        kind: 'thought',
        thought: th,
        time: th.createdAtMs || 0,
        order: (th.turn || 1) * 1000 + idx,
      });
    });
    (msg.toolExecutions || []).forEach((t, idx) => {
      items.push({
        id: t.id || `tool-${idx}`,
        kind: 'tool',
        tool: t,
        time: t.createdAtMs || 0,
        order: (t.turn || 1) * 1000 + idx,
      });
    });
    items.sort((a, b) => {
      if (a.time > 0 && b.time > 0 && a.time !== b.time) return a.time - b.time;
      return a.order - b.order;
    });
    return items;
  };

  return (
    <div className="flex-1 min-h-0 flex flex-col overflow-hidden bg-background">
      {/* Subagent Header Bar */}
      <div className="px-3.5 py-2.5 border-b border-border flex items-center justify-between bg-panel shrink-0 select-none">
        <div className="flex items-center gap-2 min-w-0">
          <div className="size-6 rounded-md bg-background border border-border flex items-center justify-center shrink-0">
            {isRunning ? (
              <Loader2 className="size-3.5 text-primary animate-spin" />
            ) : (
              <Bot className="size-3.5 text-primary" />
            )}
          </div>
          <span className="font-semibold text-ui-xs text-foreground truncate">
            {subagent.title || 'Sub-agent'}
          </span>
          {subagent.model && (
            <span className="text-[10px] font-mono text-foreground-subtle px-1.5 py-0.2 rounded bg-border/60 shrink-0">
              {subagent.model}
            </span>
          )}
        </div>

        <div className="flex items-center gap-1.5 shrink-0">
          {isRunning && (
            <button
              type="button"
              onClick={() => {
                void StopAgentTurn(subagent.goSessionId || subagent.id);
              }}
              className="flex items-center gap-1 px-2 py-0.5 rounded-full text-[10px] font-medium bg-destructive/15 text-destructive hover:bg-destructive/25 transition-colors cursor-pointer border border-destructive/30"
              title="Stop sub-agent execution"
            >
              <Square className="size-2 fill-current" />
              Stop
            </button>
          )}
          <span
            className={cn(
              'text-[10px] font-mono px-2 py-0.5 rounded-full uppercase tracking-wider font-medium flex items-center gap-1',
              isRunning
                ? 'bg-primary/20 text-primary'
                : 'bg-border/60 text-foreground-subtle'
            )}
          >
            {isRunning && <span className="size-1.5 rounded-full bg-primary animate-ping" />}
            {isRunning ? 'running' : subagent.goState || subagent.status || 'idle'}
          </span>
        </div>
      </div>

      {/* Message Stream */}
      <div className="flex-1 overflow-y-auto p-4 space-y-4 select-text">
        {messages.length === 0 ? (
          <div className="py-12 text-center text-foreground-subtlest space-y-2">
            <Sparkles className="size-7 mx-auto opacity-60 text-foreground-subtle" />
            <p className="text-ui-xs font-medium text-foreground-subtle">
              Subagent initialized
            </p>
            <p className="text-[11px] text-foreground-subtlest max-w-xs mx-auto">
              Autonomous execution transcript will stream below.
            </p>
          </div>
        ) : (
          messages.map((msg, idx) => {
            const isUser = msg.role === 'user';
            const workItems = getWorkItems(msg);
            const hasWork = workItems.length > 0;
            const wbId = `wb-${msg.id || idx}`;
            const workOpen = expandedWork[wbId] !== undefined ? expandedWork[wbId] : true;
            const hasText = !!(msg.content && msg.content.trim());

            if (isUser) {
              return (
                <div key={msg.id || idx} className="group/user flex flex-col items-end">
                  <div className="max-w-[85%] rounded-[var(--radius-user-message,1rem)] bg-[var(--app-user-message-background)] px-3 py-2 text-ui-xs text-foreground select-text border border-transparent">
                    <p className="leading-relaxed whitespace-pre-wrap break-words [overflow-wrap:anywhere]">
                      {msg.content}
                    </p>
                  </div>
                  <div className="mt-1 flex items-center pr-1 opacity-0 group-hover/user:opacity-100 transition-opacity">
                    <button
                      type="button"
                      onClick={() => handleCopy(msg.content, msg.id || `u-${idx}`)}
                      className="p-1 rounded text-foreground-subtle hover:text-foreground transition-colors cursor-pointer"
                      title="Copy message"
                    >
                      {copiedMsgId === (msg.id || `u-${idx}`) ? (
                        <Check className="size-3 text-success" />
                      ) : (
                        <Copy className="size-3" />
                      )}
                    </button>
                  </div>
                </div>
              );
            }

            // Agent Message
            return (
              <div key={msg.id || idx} className="flex flex-col items-start space-y-2.5 w-full">
                {/* Collapsible Worked-for Header */}
                {hasWork && (
                  <div className="w-full space-y-2">
                    <button
                      type="button"
                      onClick={() => setExpandedWork((p) => ({ ...p, [wbId]: !workOpen }))}
                      className="flex items-center gap-1.5 py-0.5 text-ui-xs text-foreground-subtle hover:text-foreground cursor-pointer select-none group"
                    >
                      <span className="font-medium">
                        {msg.isThinking ? 'Working…' : `Worked (${workItems.length} steps)`}
                      </span>
                      {workOpen ? (
                        <ChevronDown className="size-3 opacity-60 group-hover:opacity-100" />
                      ) : (
                        <ChevronRight className="size-3 opacity-60 group-hover:opacity-100" />
                      )}
                    </button>

                    {workOpen && (
                      <div className="w-full space-y-2 pl-1">
                        {workItems.map((item) => {
                          if (item.kind === 'thought') {
                            const th = item.thought!;
                            const isOpen =
                              expandedThoughts[th.id] !== undefined
                                ? expandedThoughts[th.id]
                                : false;

                            return (
                              <div key={th.id} className="w-full">
                                <button
                                  type="button"
                                  onClick={() =>
                                    setExpandedThoughts((p) => ({ ...p, [th.id]: !isOpen }))
                                  }
                                  className="flex items-center gap-1.5 text-ui-xs text-foreground-subtle hover:text-foreground transition-colors cursor-pointer py-0.5 w-full"
                                >
                                  <Brain className="size-3.5 text-foreground-subtle shrink-0" />
                                  <span className="font-medium text-foreground-secondary">
                                    Thought
                                  </span>
                                  <span className="ml-auto">
                                    {isOpen ? (
                                      <ChevronDown className="size-3" />
                                    ) : (
                                      <ChevronRight className="size-3" />
                                    )}
                                  </span>
                                </button>
                                {isOpen && (
                                  <div className="border-l-2 border-border/70 bg-surface/30 rounded-r-md pl-3 pr-2 py-1.5 my-1 text-ui-xs text-foreground-subtle select-text max-h-48 overflow-y-auto scrollbar-thin leading-relaxed">
                                    <MarkdownRenderer content={th.thoughtText} muted />
                                  </div>
                                )}
                              </div>
                            );
                          }

                          // Tool call
                          const t = item.tool!;
                          const tNameLower = (t.toolName || "").toLowerCase();
                          const defaultOpen = tNameLower.includes("write") || tNameLower.includes("edit") || tNameLower.includes("bash") || tNameLower.includes("skill") || t.status === "failed";
                          const isToolOpen =
                            expandedTools[t.id] !== undefined ? expandedTools[t.id] : defaultOpen;

                          return (
                            <ToolExecutionCard
                              key={t.id}
                              tool={t}
                              isOpen={isToolOpen}
                              onToggle={() =>
                                setExpandedTools((p) => ({ ...p, [t.id]: !isToolOpen }))
                              }
                              workspacePath={workspacePath}
                            />
                          );
                        })}
                      </div>
                    )}
                  </div>
                )}

                {/* Agent Response Text */}
                {hasText && (
                  <div className="text-ui-xs leading-relaxed text-foreground select-text py-1 w-full break-words [overflow-wrap:anywhere]">
                    <MarkdownRenderer content={msg.content} />
                  </div>
                )}

                {/* Live Thinking Status */}
                {msg.isThinking && (
                  <div className="flex items-center gap-2 text-warning py-1 font-mono text-ui-xs">
                    <Loader2 className="size-3.5 animate-spin" />
                    <span>Subagent is reasoning…</span>
                  </div>
                )}
              </div>
            );
          })
        )}
        <div ref={chatBottomRef} />
      </div>

      {/* Permission Approval Gate */}
      {(subagent.goState === 'awaiting_approval' || (subagent.pendingTools && subagent.pendingTools.length > 0)) && (
        <div className="shrink-0 p-2.5 border-t border-warning/30 bg-background">
          <SubagentPendingApprovalCard
            goSessionId={subagent.goSessionId || subagent.id}
            tools={subagent.pendingTools || []}
          />
        </div>
      )}

      {/* Composer Input Bar */}
      <form
        onSubmit={handleSend}
        className="shrink-0 p-2.5 border-t border-border bg-background flex items-center gap-2 select-none"
      >
        <input
          type="text"
          value={inputText}
          onChange={(e) => setInputText(e.target.value)}
          placeholder={`Instruct ${subagent.title || 'subagent'}...`}
          disabled={isSending}
          className="flex-1 bg-surface border border-border rounded-lg px-3 py-1.5 text-ui-xs text-foreground placeholder-foreground-subtlest focus:outline-hidden"
        />
        <button
          type="submit"
          disabled={!inputText.trim() || isSending}
          className="size-7 rounded-lg bg-surface-hover hover:bg-border text-foreground flex items-center justify-center disabled:opacity-40 transition-colors cursor-pointer shrink-0"
          title="Send instruction"
        >
          {isSending ? (
            <Loader2 className="size-3.5 animate-spin" />
          ) : (
            <Send className="size-3.5" />
          )}
        </button>
      </form>
    </div>
  );
};

function SubagentPendingApprovalCard({
  goSessionId,
  tools,
}: {
  goSessionId: string;
  tools: any[];
}) {
  const [busy, setBusy] = useState(false);
  const respond = async (approve: boolean, autoAll: boolean) => {
    if (busy) return;
    setBusy(true);
    try {
      await RespondAgentApproval(goSessionId, approve, autoAll);
    } catch (err) {
      console.error('subagent approval failed:', err);
    } finally {
      setBusy(false);
    }
  };

  const toolList = Array.isArray(tools) ? tools : [];

  return (
    <div className="w-full flex flex-col rounded-xl border border-warning/40 bg-warning/5 p-3 shadow-md">
      <div className="flex items-center justify-between pb-1.5 border-b border-warning/20">
        <div className="flex items-center gap-1.5 text-ui-xs font-semibold text-warning">
          <AlertCircle className="size-3.5 shrink-0" />
          <span>Permission required for sub-agent:</span>
        </div>
        <span className="text-ui-xs font-mono text-warning/80">
          {toolList.length} {toolList.length === 1 ? 'action' : 'actions'}
        </span>
      </div>
      <div className="max-h-36 overflow-y-auto space-y-1.5 py-2 scrollbar-thin">
        {toolList.length === 0 ? (
          <div className="text-ui-xs text-foreground-subtle italic">Action requires user approval to proceed.</div>
        ) : (
          toolList.map((tc: any, i: number) => {
            const name = tc?.name || 'tool';
            let argsText = tc?.arguments ?? '{}';
            if (typeof argsText !== 'string') argsText = JSON.stringify(argsText, null, 2);
            return (
              <div
                key={tc?.tool_call_id || i}
                className="text-ui-xs font-mono bg-surface border border-border rounded p-2 overflow-x-auto whitespace-pre-wrap break-words"
              >
                <div className="font-semibold text-warning mb-0.5">{name}</div>
                <div className="text-foreground-subtle text-[11px]">{argsText}</div>
              </div>
            );
          })
        )}
      </div>
      <div className="flex items-center justify-end gap-2 pt-1.5 border-t border-warning/20">
        <button
          type="button"
          onClick={() => respond(false, false)}
          disabled={busy}
          className="px-2 py-0.5 text-ui-xs text-foreground-subtle border border-border rounded hover:bg-surface-hover cursor-pointer disabled:opacity-50"
        >
          Deny
        </button>
        <button
          type="button"
          onClick={() => respond(true, true)}
          disabled={busy}
          title="Approve this and all future tool calls for this subagent"
          className="px-2 py-0.5 text-ui-xs text-foreground-subtle border border-border rounded hover:bg-surface-hover cursor-pointer disabled:opacity-50"
        >
          Always allow
        </button>
        <button
          type="button"
          onClick={() => respond(true, false)}
          disabled={busy}
          className="px-2.5 py-0.5 text-ui-xs font-semibold bg-warning text-black rounded cursor-pointer disabled:opacity-50"
        >
          Approve
        </button>
      </div>
    </div>
  );
}
