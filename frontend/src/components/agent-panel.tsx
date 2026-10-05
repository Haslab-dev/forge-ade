import React, { useEffect, useRef, useState } from "react";
import {
  IconRobot,
  IconCpu,
  IconChevronDown,
  IconChevronUp,
  IconArrowDown,
  IconArrowUp,
  IconBolt,
  IconSend,
  IconX,
} from "@tabler/icons-react";
import { cn } from "../lib/utils";
import {
  ListAgentDefinitions,
  ApplyAgentDefinitionToSession,
  SendAgentMessage,
  RespondAgentApproval,
  SetAgentAutoApprove,
  StopAgentTurn,
  SearchFilename,
  GetProviderProfiles,
  GetLLMConfig,
  SetActiveModel,
  EventsOn,
} from "../lib/wails";
import { AgentChatBody } from "./agent-chat";

// Token usage breakdown: ↓ input, ↑ output, ⚡ cached.
function TokenUsageBadge({ usage }: { usage: any }) {
  const inTok = usage?.prompt_tokens ?? usage?.PromptTokens ?? 0;
  const outTok = usage?.completion_tokens ?? usage?.CompletionTokens ?? 0;
  // DeepSeek reports hit/miss; other providers only report cached_tokens.
  const hit = usage?.prompt_cache_hit_tokens ?? usage?.PromptCacheHitTokens ?? 0;
  const miss = usage?.prompt_cache_miss_tokens ?? usage?.PromptCacheMissTokens ?? 0;
  const cached = hit > 0 ? hit : usage?.cached_tokens ?? usage?.CachedTokens ?? 0;
  const hitPct = hit > 0 ? Math.round((hit / (hit + miss)) * 100) : null;
  if (inTok + outTok + cached === 0) return null;
  return (
    <span
      className="flex items-center gap-2 px-1.5 py-0.5 bg-[var(--bg-panel)] border border-[var(--border-default)] text-[10px] font-mono text-[var(--fg-tertiary)] rounded"
      title="Token usage: input / output / cached (cache hit %)"
    >
      <span className="flex items-center gap-0.5">
        <IconArrowDown className="size-2.5" />
        {inTok.toLocaleString()}
      </span>
      <span className="flex items-center gap-0.5">
        <IconArrowUp className="size-2.5" />
        {outTok.toLocaleString()}
      </span>
      <span className="flex items-center gap-0.5" title={hitPct !== null ? `Cache: ${hit.toLocaleString()} hit / ${miss.toLocaleString()} miss` : "Cached tokens"}>
        <IconBolt className="size-2.5" />
        {cached.toLocaleString()}
        {hitPct !== null ? ` (${hitPct}%)` : ""}
      </span>
    </span>
  );
}

// Shared agent chat panel — the single view used by BOTH the Session panel and
// the Workspace editor tab, so they always look and behave identically.
export function AgentChatPanel({
  session,
  onClose,
  onAgentLaunched,
}: {
  session: any;
  onClose: () => void;
  onAgentLaunched?: (sessionId: string) => void;
}) {
  const [inputText, setInputText] = useState("");
  const [showMentionMenu, setShowMentionMenu] = useState(false);
  const [mentionResults, setMentionResults] = useState<string[]>([]);
  const [profiles, setProfiles] = useState<any[]>([]);
  const [activeModel, setActiveModelName] = useState<string>("");
  const [showModelPicker, setShowModelPicker] = useState(false);
  const [agentDefs, setAgentDefs] = useState<any[]>([]);
  const [showAgentPicker, setShowAgentPicker] = useState(false);

  useEffect(() => {
    loadProfiles();
    loadAgentDefs();
    const unsubscribe = EventsOn("agent:config:changed", () => {
      loadProfiles();
      loadAgentDefs();
    });
    return () => {
      if (mentionTimerRef.current) clearTimeout(mentionTimerRef.current);
      if (typeof unsubscribe === "function") unsubscribe();
    };
  }, []);

  async function loadProfiles() {
    try {
      const list = await GetProviderProfiles();
      setProfiles(Array.isArray(list) ? list : []);
      const cfg = await GetLLMConfig();
      if (cfg && cfg.model) setActiveModelName(cfg.model);
    } catch { /* ignore */ }
  }

  async function loadAgentDefs() {
    try {
      const list = await ListAgentDefinitions();
      setAgentDefs(Array.isArray(list) ? list : []);
    } catch { /* ignore */ }
  }

  async function handleSelectModel(providerId: string, model: string) {
    setActiveModelName(model);
    setShowModelPicker(false);
    try {
      await SetActiveModel(providerId, model);
    } catch { /* ignore */ }
  }

  async function handleLaunchAgentDef(def: any) {
    setShowAgentPicker(false);
    try {
      // Re-selecting an agent mode re-configures the CURRENT session's context
      // (role, prompt, rules, model) instead of spawning a new agent tab.
      await ApplyAgentDefinitionToSession(session.id, def.id || def.ID);
      onAgentLaunched?.(session.id);
    } catch (err) {
      console.error("Failed to apply agent:", err);
    }
  }

  async function handleSendMessage() {
    if (!inputText.trim()) return;
    const text = inputText;
    setInputText("");
    try {
      await SendAgentMessage(session.id, text, []);
    } catch (err) {
      console.error(err);
    }
  }

  async function handleApproval(approve: boolean, autoAll = false) {
    try {
      await RespondAgentApproval(session.id, approve, autoAll);
    } catch (err) {
      console.error(err);
    }
  }

  // Debounce mention lookup: SearchFilename is a backend RPC — firing it on
  // every keystroke while typing near an @ is wasted work.
  const mentionTimerRef = useRef<ReturnType<typeof setTimeout> | null>(null);
  async function handleInputChange(e: React.ChangeEvent<HTMLTextAreaElement>) {
    const val = e.target.value;
    setInputText(val);
    const atIndex = val.lastIndexOf("@");
    if (atIndex !== -1 && atIndex >= val.length - 20) {
      const query = val.slice(atIndex + 1);
      if (mentionTimerRef.current) clearTimeout(mentionTimerRef.current);
      mentionTimerRef.current = setTimeout(async () => {
        try {
          const results = await SearchFilename(query, 8);
          setMentionResults(results.map((r: any) => r.path ?? r.Path));
          setShowMentionMenu(true);
        } catch {
          setMentionResults([]);
        }
      }, 200);
    } else {
      setShowMentionMenu(false);
    }
  }

  return (
    <div className="flex flex-col h-full w-full bg-[var(--bg-app)] relative overflow-hidden">
      {/* Header bar — identical on both surfaces */}
      <div className="flex items-center justify-between px-3 py-1.5 bg-[var(--bg-sidebar)] border-b border-[var(--border-default)] text-xs text-[var(--fg-secondary)] select-none shrink-0">
        <div className="flex items-center space-x-2 font-semibold text-[var(--fg-primary)]">
          <IconRobot className="size-3.5 text-blue-400" />
          <span className="truncate max-w-48">{session.name}</span>
          <span className="text-[10px] bg-[var(--accent)]/15 border border-[var(--accent)]/40 text-[var(--accent)] px-1.5 py-0.5 rounded font-mono uppercase">
            {session.role_filter || "coding"}
          </span>
        </div>

        <div className="flex items-center space-x-1">
          {(session.state === "thinking" || session.state === "executing") && (
            <>
              <span className="flex items-center gap-1 text-[10px] text-purple-400 font-mono">
                <span className="inline-block size-2 rounded-full bg-purple-400 animate-pulse" />
                {session.state === "executing" ? "running tools…" : "thinking…"}
              </span>
              <button
                onClick={() => StopAgentTurn(session.id)}
                className="px-1.5 py-0.5 text-[10px] text-red-400 hover:text-red-300 border border-red-900/60 hover:border-red-700 rounded cursor-pointer"
                title="Stop the current turn"
              >
                stop
              </button>
            </>
          )}
          {(session.token_usage?.total_tokens ?? session.token_usage?.TotalTokens ?? 0) > 0 && (
            <TokenUsageBadge usage={session.token_usage} />
          )}

          {onClose && (
            <button onClick={onClose} className="hover:text-white cursor-pointer rounded p-0.5">
              <IconX className="size-3.5" />
            </button>
          )}
        </div>
      </div>

      {/* Messages */}
      <AgentChatBody
        messages={session.messages || []}
        session={session}
        onApprove={(approve, autoAll) => handleApproval(approve, autoAll)}
        onDeny={() => handleApproval(false)}
      />

      {/* Input container — styled after MyADE floating card container */}
      <div className="p-3 bg-[var(--bg-app)] border-t border-[var(--border-default)] shrink-0 relative">
        <div className="rounded-xl border border-[var(--border-default)] bg-[var(--bg-panel)] shadow-sm hover:shadow-md transition-shadow overflow-visible relative">
          {/* Tip Header */}
          <div className="px-3 pt-2 pb-0.5 text-[11px] text-[var(--fg-tertiary)] flex items-center justify-between select-none">
            <div>
              Tip: Type <span className="text-[var(--accent-primary)] font-mono font-medium">@</span> to bring in files or symbols
            </div>
            {session.auto_approve && (
              <span className="text-[10px] font-mono text-red-400 bg-red-500/10 px-1.5 py-0.2 rounded border border-red-500/30">
                ⚡ Auto-approve ON
              </span>
            )}
          </div>

          <textarea
            value={inputText}
            onChange={handleInputChange}
            onKeyDown={(e) => {
              if (e.key === "Tab" && e.shiftKey) {
                if (session.pending_tools && session.pending_tools.length > 0) {
                  e.preventDefault();
                  handleApproval(true);
                }
              } else if (e.key === "Enter" && !e.shiftKey) {
                e.preventDefault();
                handleSendMessage();
              }
            }}
            placeholder="Ask anything, type @ to mention files..."
            rows={2}
            className="w-full bg-transparent px-3 py-1.5 text-xs text-[var(--fg-primary)] placeholder:text-[var(--fg-tertiary)] focus:outline-none resize-none font-sans leading-relaxed"
          />

          {/* File mentions autocomplete box */}
          {showMentionMenu && mentionResults.length > 0 && (
            <div className="absolute bottom-full left-0 right-0 mb-2 z-50 bg-[var(--bg-sidebar)] border border-[var(--border-default)] rounded-lg shadow-2xl max-h-40 overflow-y-auto p-1 space-y-0.5">
              {mentionResults.map((p) => (
                <button
                  key={p}
                  onClick={() => {
                    const atIndex = inputText.lastIndexOf("@");
                    setInputText(inputText.slice(0, atIndex) + "@" + p + " ");
                    setShowMentionMenu(false);
                  }}
                  className="w-full text-left px-2.5 py-1.5 hover:bg-[var(--bg-panel)] rounded text-[var(--fg-secondary)] hover:text-[var(--fg-primary)] text-xs font-mono truncate flex items-center gap-1.5 cursor-pointer"
                >
                  <span className="text-blue-400">@</span>
                  <span className="truncate">{p}</span>
                </button>
              ))}
            </div>
          )}

          {/* Bottom Control Bar inside card */}
          <div className="px-2.5 pb-2 pt-1 flex items-center justify-between border-t border-[var(--border-default)]/40">
            <div className="flex items-center space-x-1.5">
              {agentDefs.length > 0 && (
                <div className="relative">
                  <button
                    onClick={() => setShowAgentPicker(!showAgentPicker)}
                    className="px-2 py-0.5 bg-[var(--bg-app)] hover:bg-[var(--bg-surface-hover)] border border-[var(--border-default)] text-[var(--fg-secondary)] rounded-md flex items-center space-x-1 cursor-pointer text-xs"
                    title="Switch pre-configured agent persona"
                  >
                    <IconRobot className="size-3 text-blue-400" />
                    <span className="text-[11px] font-medium">{session.role_filter || "Agent"}</span>
                    {showAgentPicker ? <IconChevronUp className="size-3" /> : <IconChevronDown className="size-3" />}
                  </button>
                  {showAgentPicker && (
                    <div className="absolute bottom-full left-0 mb-1 z-50 w-56 bg-[var(--bg-sidebar)] border border-[var(--border-default)] shadow-2xl rounded-lg p-1 text-xs max-h-56 overflow-y-auto">
                      {agentDefs.map((def) => (
                        <button
                          key={def.id || def.ID}
                          onClick={() => handleLaunchAgentDef(def)}
                          className="w-full text-left px-2 py-1.5 hover:bg-[var(--bg-panel)] rounded text-[var(--fg-primary)] cursor-pointer"
                        >
                          <div className="font-semibold text-[11px]">{def.name || def.Name}</div>
                          {def.description && (
                            <div className="text-[9px] text-[var(--fg-tertiary)] truncate">{def.description}</div>
                          )}
                        </button>
                      ))}
                    </div>
                  )}
                </div>
              )}

              <div className="relative">
                <button
                  onClick={() => setShowModelPicker(!showModelPicker)}
                  className="px-2 py-0.5 bg-[var(--bg-app)] hover:bg-[var(--bg-surface-hover)] border border-[var(--border-default)] text-[var(--fg-secondary)] rounded-md flex items-center space-x-1 cursor-pointer font-mono text-xs"
                  title="Switch active model"
                >
                  <IconCpu className="size-3 text-purple-400" />
                  <span className="text-[11px] truncate max-w-28">{activeModel || "Model"}</span>
                  {showModelPicker ? <IconChevronUp className="size-3" /> : <IconChevronDown className="size-3" />}
                </button>
                {showModelPicker && (
                  <div className="absolute bottom-full left-0 mb-1 z-50 w-64 bg-[var(--bg-sidebar)] border border-[var(--border-default)] shadow-2xl rounded-lg p-2 text-xs max-h-72 overflow-y-auto">
                    <div className="font-bold text-[10px] text-[var(--fg-tertiary)] uppercase tracking-wider mb-1.5 px-2">Models</div>
                    {profiles.map((p) => {
                      const pid = p.id || p.Id || p.name || p.Name;
                      const models = p.selected_models || p.SelectedModels || p.available_models || p.AvailableModels || [];
                      if (models.length === 0) {
                        return (
                          <div key={pid} className="px-2 py-1 text-[var(--fg-tertiary)] font-mono text-[11px]">
                            {p.name || p.Name} — no models
                          </div>
                        );
                      }
                      return (
                        <div key={pid} className="mb-1">
                          <div className="px-2 py-0.5 text-[9px] uppercase tracking-wider text-[var(--fg-tertiary)] font-semibold">
                            {p.name || p.Name}
                          </div>
                          {models.map((m: string) => (
                            <button
                              key={m}
                              onClick={() => handleSelectModel(pid, m)}
                              className={cn(
                                "w-full text-left px-2 py-1 hover:bg-[var(--bg-panel)] rounded text-[var(--fg-primary)] truncate font-mono text-[11px] cursor-pointer",
                                activeModel === m && "bg-[var(--bg-surface-active)] text-[var(--fg-on-active)]"
                              )}
                            >
                              <span className="mr-1.5">{activeModel === m ? "●" : "○"}</span>
                              {m}
                            </button>
                          ))}
                        </div>
                      );
                    })}
                  </div>
                )}
              </div>
            </div>

            <div className="flex items-center space-x-1.5">
              {/* YOLO toggle */}
              <button
                onClick={() => {
                  const next = !session.auto_approve;
                  SetAgentAutoApprove(session.id, next);
                }}
                title={session.auto_approve ? "YOLO mode ON — all tool calls auto-approved" : "YOLO mode OFF — click to auto-approve tool calls"}
                className={cn(
                  "px-2 py-0.5 text-xs font-semibold rounded-md flex items-center space-x-1 cursor-pointer border transition-colors",
                  session.auto_approve
                    ? "bg-red-500/20 border-red-500 text-red-400"
                    : "bg-[var(--bg-app)] border-[var(--border-default)] text-[var(--fg-tertiary)] hover:text-[var(--fg-secondary)]"
                )}
              >
                <IconBolt className="size-3" />
                <span className="text-[10px]">YOLO</span>
              </button>

              <button
                onClick={handleSendMessage}
                disabled={!inputText.trim()}
                className="px-3 py-1 bg-[var(--accent-primary)] hover:bg-[var(--accent-hover)] text-black text-xs font-semibold rounded-md flex items-center space-x-1.5 cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed shadow-xs transition-opacity"
              >
                <IconSend className="size-3.5" />
                <span>Send</span>
              </button>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
