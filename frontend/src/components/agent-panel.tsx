import React, { useEffect, useMemo, useRef, useState } from "react";
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
  IconSquare,
  IconTrashX,
  IconPointFilled,
  IconCircle,
  IconSearch,
  IconTerminal,
  IconSparkles,
  IconCheck,
} from "@tabler/icons-react";
import { cn } from "../lib/utils";
import {
  ListAgentDefinitions,
  ListAgentSessions,
  ApplyAgentDefinitionToSession,
  SendAgentMessage,
  RespondAgentApproval,
  SetAgentAutoApprove,
  StopAgentTurn,
  ClearAgentSession,
  SearchFilename,
  GetProviderProfiles,
  GetLLMConfig,
  SetActiveModel,
  GenerateAICommitMessage,
  EventsOn,
} from "../lib/native";
import { AgentChatBody } from "./agent-chat";

// ---------------------------------------------------------------------------
// Token Usage Metric Badge
// ---------------------------------------------------------------------------
function TokenUsageBadge({ usage }: { usage: any }) {
  const inTok = usage?.prompt_tokens ?? usage?.PromptTokens ?? 0;
  const outTok = usage?.completion_tokens ?? usage?.CompletionTokens ?? 0;
  const hit = usage?.prompt_cache_hit_tokens ?? usage?.PromptCacheHitTokens ?? 0;
  const miss = usage?.prompt_cache_miss_tokens ?? usage?.PromptCacheMissTokens ?? 0;
  const cached = hit > 0 ? hit : usage?.cached_tokens ?? usage?.CachedTokens ?? 0;
  const hitPct = hit > 0 ? Math.round((hit / (hit + miss)) * 100) : null;
  if (inTok + outTok + cached === 0) return null;
  return (
    <span
      className="flex items-center gap-2 px-1.5 py-0.5 bg-[var(--bg-panel)] border border-[var(--border-default)] text-[10px] font-mono text-[var(--fg-tertiary)] rounded"
      title="Token usage: input / output / cached"
    >
      <span className="flex items-center gap-0.5">
        <IconArrowDown className="size-2.5 text-sky-400" />
        {inTok.toLocaleString()}
      </span>
      <span className="flex items-center gap-0.5">
        <IconArrowUp className="size-2.5 text-emerald-400" />
        {outTok.toLocaleString()}
      </span>
      {cached > 0 && (
        <span className="flex items-center gap-0.5" title={hitPct !== null ? `Cache: ${hit.toLocaleString()} hit / ${miss.toLocaleString()} miss` : "Cached tokens"}>
          <IconBolt className="size-2.5 text-amber-400" />
          {cached.toLocaleString()}
          {hitPct !== null ? ` (${hitPct}%)` : ""}
        </span>
      )}
    </span>
  );
}

// ---------------------------------------------------------------------------
// Live Activity Indicator
// ---------------------------------------------------------------------------
const STATUS_WORDS = [
  "Executing tools", "Inspecting code", "Analyzing AST", "Formulating plan", "Refactoring",
  "Resolving references", "Synthesizing", "Deliberating", "Evaluating",
];

function AgentStatusBar({ running }: { running: boolean }) {
  const [wordIdx, setWordIdx] = useState(0);
  const [elapsed, setElapsed] = useState(0);

  useEffect(() => {
    const iv = setInterval(() => setWordIdx((i) => (i + 1) % STATUS_WORDS.length), 2000);
    return () => clearInterval(iv);
  }, []);

  useEffect(() => {
    if (!running) {
      setElapsed(0);
      return;
    }
    const start = Date.now();
    const iv = setInterval(() => setElapsed(Date.now() - start), 1000);
    return () => clearInterval(iv);
  }, [running]);

  const totalSec = Math.floor(elapsed / 1000);
  const h = Math.floor(totalSec / 3600);
  const m = Math.floor((totalSec % 3600) / 60);
  const s = totalSec % 60;
  const timer = h > 0 ? `${h}m ${m}s` : m > 0 ? `${m}m ${s}s` : `${s}s`;

  return (
    <div className="flex items-center gap-2 px-1 pb-1.5 text-[10px] font-mono text-[var(--fg-tertiary)] select-none">
      <span className="text-cyan-400 font-bold animate-spin">⠋</span>
      <span className="text-[var(--fg-secondary)] font-medium">{STATUS_WORDS[wordIdx]}…</span>
      <span className="text-[var(--fg-tertiary)]/50">•</span>
      <span className="text-[var(--fg-tertiary)]">{timer}</span>
      <span className="text-[var(--fg-tertiary)]/50">•</span>
      <span className="text-[var(--fg-tertiary)]">esc to stop</span>
    </div>
  );
}

// ---------------------------------------------------------------------------
// Agent Terminal Panel
// ---------------------------------------------------------------------------
export function AgentChatPanel({
  session,
  onClose,
  onAgentLaunched,
}: {
  session: any;
  onClose: () => void;
  onAgentLaunched?: (sessionId: string) => void;
}) {
  const [liveSession, setLiveSession] = useState<any>(session);
  const [inputText, setInputText] = useState("");
  const [showMentionMenu, setShowMentionMenu] = useState(false);
  const [mentionResults, setMentionResults] = useState<string[]>([]);
  const [profiles, setProfiles] = useState<any[]>([]);
  const [activeModel, setActiveModelName] = useState<string>("");
  const [activeProviderId, setActiveProviderId] = useState<string>("");
  const [showModelPicker, setShowModelPicker] = useState(false);
  const [modelSearchQuery, setModelSearchQuery] = useState("");
  const [agentDefs, setAgentDefs] = useState<any[]>([]);
  const [showAgentPicker, setShowAgentPicker] = useState(false);

  const modelPickerRef = useRef<HTMLDivElement>(null);
  const agentPickerRef = useRef<HTMLDivElement>(null);
  const textareaRef = useRef<HTMLTextAreaElement>(null);
  const mentionTimerRef = useRef<number | NodeJS.Timeout | null>(null);

  useEffect(() => {
    setLiveSession(session);
  }, [session]);

  useEffect(() => {
    const unsubs = [
      EventsOn("agent:updated", (payload: any) => {
        if (!payload?.id || payload.id === session?.id) {
          ListAgentSessions().then((list) => {
            const found = list.find((s: any) => s.id === session?.id);
            if (found) {
              setLiveSession({ ...found, messages: [...found.messages] });
            }
          });
        }
      }),
      EventsOn("agent:turn_start", (payload: any) => {
        if (!payload?.id || payload.id === session?.id) {
          setLiveSession((prev: any) => ({ ...prev, state: "thinking" }));
        }
      }),
      EventsOn("agent:turn_end", (payload: any) => {
        if (!payload?.id || payload.id === session?.id) {
          ListAgentSessions().then((list) => {
            const found = list.find((s: any) => s.id === session?.id);
            if (found) {
              setLiveSession({ ...found, state: "idle", messages: [...found.messages] });
            }
          });
        }
      }),
      EventsOn("agent:config:changed", () => {
        loadProfiles();
        loadAgentDefs();
      }),
    ];

    loadProfiles();
    loadAgentDefs();

    return () => {
      clearTimeout(mentionTimerRef.current as any);
      unsubs.forEach((u) => typeof u === "function" && u());
    };
  }, [session?.id]);

  // Close dropdowns on outside click
  useEffect(() => {
    const handleOutsideClick = (e: MouseEvent) => {
      if (modelPickerRef.current && !modelPickerRef.current.contains(e.target as Node)) {
        setShowModelPicker(false);
      }
      if (agentPickerRef.current && !agentPickerRef.current.contains(e.target as Node)) {
        setShowAgentPicker(false);
      }
    };
    window.addEventListener("mousedown", handleOutsideClick);
    return () => window.removeEventListener("mousedown", handleOutsideClick);
  }, []);

  async function loadProfiles() {
    try {
      const list = await GetProviderProfiles();
      setProfiles(Array.isArray(list) ? list : []);
      const cfg = await GetLLMConfig();
      if (cfg) {
        if (cfg.model) setActiveModelName(cfg.model);
        if (cfg.provider_id) setActiveProviderId(cfg.provider_id);
      }
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
    setActiveProviderId(providerId);
    setShowModelPicker(false);
    setModelSearchQuery("");
    try {
      await SetActiveModel(providerId, model);
    } catch { /* ignore */ }
  }

  async function handleLaunchAgentDef(def: any) {
    setShowAgentPicker(false);
    try {
      await ApplyAgentDefinitionToSession(session.id, def.id || def.ID);
      onAgentLaunched?.(session.id);
    } catch (err) {
      console.error("Failed to apply agent:", err);
    }
  }

  async function handleSendMessage(customText?: string) {
    const text = customText || inputText;
    if (!text.trim()) return;

    // Handle slash commands
    if (text.startsWith("/commit")) {
      setInputText("");
      try {
        const commitMsg = await GenerateAICommitMessage("", activeProviderId, activeModel);
        await SendAgentMessage(session.id, `/commit\n\nProposed commit message:\n\`${commitMsg}\``, []);
      } catch {
        await SendAgentMessage(session.id, text, []);
      }
      return;
    }

    if (text.startsWith("/clear")) {
      setInputText("");
      ClearAgentSession(session.id);
      return;
    }

    if (text.startsWith("/model")) {
      setInputText("");
      setShowModelPicker(true);
      return;
    }

    if (text.startsWith("/help")) {
      setInputText("");
      await SendAgentMessage(
        session.id,
        "Help: Available Slash Commands & Tools\n- `/commit`: Analyze git diff and draft a conventional commit message\n- `/clear`: Clear conversation history\n- `/model`: Open AI model picker\n- `@filename`: Mention a file for workspace context\n\nAutonomous tools enabled: `read_file`, `write_file`, `edit_file`, `bash`, `grep_code`, `search_files`, `list_dir`.",
        []
      );
      return;
    }

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

  const handleStop = () => {
    try {
      StopAgentTurn(session.id);
    } catch (err) {
      console.error(err);
    }
  };

  useEffect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === "Escape") {
        const st = session?.state;
        if (st === "thinking" || st === "executing") {
          e.preventDefault();
          e.stopPropagation();
          handleStop();
        }
      }
    };
    window.addEventListener("keydown", onKey, true);
    return () => window.removeEventListener("keydown", onKey, true);
  }, [session?.id, session?.state]);

  async function handleInputChange(e: React.ChangeEvent<HTMLTextAreaElement>) {
    const val = e.target.value;
    setInputText(val);
    const atIndex = val.lastIndexOf("@");
    if (atIndex !== -1 && atIndex >= val.length - 25) {
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
      }, 150);
    } else {
      setShowMentionMenu(false);
    }
  }

  // Filtered models for dropdown
  const filteredProfiles = useMemo(() => {
    if (!modelSearchQuery.trim()) return profiles;
    const q = modelSearchQuery.toLowerCase();
    return profiles.map((p) => {
      const allModels = p.selected_models || p.available_models || [];
      const matched = allModels.filter((m: string) => m.toLowerCase().includes(q));
      return { ...p, filteredModels: matched };
    }).filter((p) => p.filteredModels && p.filteredModels.length > 0);
  }, [profiles, modelSearchQuery]);

  const isRunning = (liveSession?.state ?? session?.state) === "thinking" || (liveSession?.state ?? session?.state) === "executing";

  return (
    <div className="flex flex-col h-full w-full bg-[var(--bg-app)] relative overflow-hidden font-sans text-xs">
      {/* Terminal Top Control Bar */}
      <div className="flex items-center justify-between px-3 py-1.5 bg-[var(--bg-sidebar)] border-b border-[var(--border-default)] select-none shrink-0 font-mono">
        <div className="flex items-center space-x-2 min-w-0">
          <IconTerminal className="size-3.5 text-cyan-400 shrink-0" />
          <span className="font-bold text-[var(--fg-primary)] truncate max-w-44 text-[11px]">{liveSession?.name || session.name}</span>

          {/* Role Picker */}
          <div className="relative" ref={agentPickerRef}>
            <button
              onClick={() => setShowAgentPicker(!showAgentPicker)}
              className="px-1.5 py-0.5 bg-[var(--accent)]/15 border border-[var(--accent)]/40 text-[var(--accent)] hover:bg-[var(--accent)]/25 rounded text-[10px] uppercase tracking-wide flex items-center gap-1 cursor-pointer transition-colors"
            >
              <span>{liveSession?.role_filter || session.role_filter || "coding"}</span>
              <IconChevronDown className="size-2.5 opacity-70" />
            </button>
            {showAgentPicker && (
              <div className="absolute top-full left-0 mt-1 z-40 w-56 bg-[var(--bg-sidebar)] border border-[var(--border-default)] shadow-2xl p-1 rounded font-sans text-xs max-h-60 overflow-y-auto">
                <div className="text-[10px] uppercase font-bold text-[var(--fg-tertiary)] px-2 py-1 tracking-wider">Agent Role</div>
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
        </div>

        {/* Right Tools: Model Picker, Token Usage, Stop, Clear */}
        <div className="flex items-center space-x-1.5">
          {/* Model Selector Chip */}
          <div className="relative" ref={modelPickerRef}>
            <button
              onClick={() => setShowModelPicker(!showModelPicker)}
              className="px-2 py-0.5 bg-[var(--bg-panel)] hover:bg-[var(--bg-surface-hover)] border border-[var(--border-default)] text-[var(--fg-secondary)] hover:text-[var(--fg-primary)] rounded flex items-center space-x-1.5 cursor-pointer font-mono text-[11px] transition-colors"
              title="Change active AI model"
            >
              <IconCpu className="size-3 text-purple-400 shrink-0" />
              <span className="truncate max-w-40">{activeModel || "Model"}</span>
              <IconChevronDown className="size-2.5 opacity-60 shrink-0" />
            </button>

            {showModelPicker && (
              <div className="absolute top-full right-0 mt-1 z-50 w-72 bg-[var(--bg-sidebar)] border border-[var(--border-default)] shadow-2xl p-2 rounded text-xs font-sans max-h-80 flex flex-col">
                <div className="relative mb-2">
                  <IconSearch className="size-3 absolute left-2 top-2 text-[var(--fg-tertiary)]" />
                  <input
                    autoFocus
                    value={modelSearchQuery}
                    onChange={(e) => setModelSearchQuery(e.target.value)}
                    placeholder="Search models..."
                    className="w-full bg-[var(--bg-panel)] border border-[var(--border-default)] pl-7 pr-2 py-1 text-[11px] text-[var(--fg-primary)] rounded focus:outline-none focus:border-[var(--accent-primary)] font-mono"
                  />
                </div>

                <div className="overflow-y-auto flex-1 space-y-2 pr-0.5">
                  {filteredProfiles.map((p) => {
                    const pid = p.id || p.Id || p.name || p.Name;
                    const models = (p as any).filteredModels || p.selected_models || p.available_models || [];
                    if (models.length === 0) return null;
                    return (
                      <div key={pid}>
                        <div className="px-2 py-0.5 text-[9px] uppercase tracking-wider text-[var(--fg-tertiary)] font-bold">
                          {p.name || p.Name}
                        </div>
                        {models.map((m: string) => {
                          const isActive = activeModel === m;
                          return (
                            <button
                              key={m}
                              onClick={() => handleSelectModel(pid, m)}
                              className={cn(
                                "w-full text-left px-2 py-1 hover:bg-[var(--bg-panel)] rounded text-[var(--fg-primary)] truncate font-mono text-[11px] flex items-center justify-between cursor-pointer group",
                                isActive && "bg-[var(--bg-surface-active)] text-[var(--fg-on-active)]"
                              )}
                            >
                              <span className="truncate">{m}</span>
                              {isActive && <IconCheck className="size-3 text-emerald-400 shrink-0 ml-1" />}
                            </button>
                          );
                        })}
                      </div>
                    );
                  })}
                </div>
              </div>
            )}
          </div>

          {/* Running Indicator & Stop Button */}
          {isRunning && (
            <button
              onClick={() => handleStop()}
              className="flex items-center gap-1 px-2 py-0.5 text-[10px] font-bold text-white bg-red-600 hover:bg-red-500 rounded cursor-pointer transition-colors"
              title="Stop execution (Esc)"
            >
              <IconSquare className="size-2.5" />
              <span>Stop</span>
            </button>
          )}
          {/* Token usage badge */}
          {((liveSession?.token_usage?.total_tokens ?? liveSession?.token_usage?.TotalTokens ?? session.token_usage?.total_tokens ?? 0) > 0) && (
            <TokenUsageBadge usage={liveSession?.token_usage || session.token_usage} />
          )}

          {/* Clear Button */}
          <button
            onClick={() => {
              if (window.confirm("Clear conversation history for this session?")) {
                ClearAgentSession(session.id);
              }
            }}
            className="p-1 text-[var(--fg-secondary)] hover:text-white hover:bg-[var(--bg-surface-hover)] border border-[var(--border-default)] rounded cursor-pointer transition-colors"
            title="Clear transcript"
          >
            <IconTrashX className="size-3" />
          </button>

          {onClose && (
            <button onClick={onClose} className="hover:text-white cursor-pointer rounded p-1 text-[var(--fg-tertiary)]">
              <IconX className="size-3.5" />
            </button>
          )}
        </div>
      </div>

      {/* Terminal Message Stream (No cards, no bubbles) */}
      <AgentChatBody
        messages={liveSession?.messages || session.messages || []}
        session={liveSession || session}
        onApprove={(approve, autoAll) => handleApproval(approve, autoAll)}
        onDeny={() => handleApproval(false)}
      />
      {/* Terminal Input Bar */}
      <div className="p-3 border-t border-[var(--border-default)] bg-[var(--bg-sidebar)] shrink-0 relative">
        {isRunning && <AgentStatusBar running={true} />}

        {/* Quick Slash Commands */}
        <div className="flex items-center gap-1.5 pb-1.5 overflow-x-auto text-[10px] font-mono select-none">
          <button
            onClick={() => handleSendMessage("/commit")}
            className="px-2 py-0.5 bg-[var(--bg-panel)] hover:bg-[var(--bg-surface-hover)] border border-[var(--border-default)] rounded text-[var(--fg-secondary)] hover:text-cyan-300 cursor-pointer flex items-center gap-1"
            title="Generate and review AI commit"
          >
            <IconSparkles className="size-2.5 text-cyan-400" />
            /commit
          </button>
          <button
            onClick={() => handleSendMessage("Create a step-by-step implementation plan for the requested task.")}
            className="px-2 py-0.5 bg-[var(--bg-panel)] hover:bg-[var(--bg-surface-hover)] border border-[var(--border-default)] rounded text-[var(--fg-secondary)] hover:text-cyan-300 cursor-pointer"
          >
            /plan
          </button>
          <button
            onClick={() => handleSendMessage("Run a security and code quality review on the modified files.")}
            className="px-2 py-0.5 bg-[var(--bg-panel)] hover:bg-[var(--bg-surface-hover)] border border-[var(--border-default)] rounded text-[var(--fg-secondary)] hover:text-cyan-300 cursor-pointer"
          >
            /review
          </button>
          <button
            onClick={() => handleSendMessage("/help")}
            className="px-2 py-0.5 bg-[var(--bg-panel)] hover:bg-[var(--bg-surface-hover)] border border-[var(--border-default)] rounded text-[var(--fg-secondary)] hover:text-cyan-300 cursor-pointer"
          >
            /help
          </button>
        </div>

        {/* Input prompt line */}
        <div className="relative flex items-start gap-2 bg-[var(--bg-panel)] border border-[var(--border-default)] focus-within:border-[var(--accent-primary)] p-2 rounded transition-colors">
          <span className="text-cyan-400 font-mono font-bold text-sm select-none pt-0.5">❯</span>
          <textarea
            ref={textareaRef}
            value={inputText}
            onChange={handleInputChange}
            onKeyDown={(e) => {
              if (e.key === "Enter" && !e.shiftKey) {
                e.preventDefault();
                handleSendMessage();
              }
            }}
            placeholder="Ask agent, type /commands or @file..."
            rows={2}
            className="flex-1 bg-transparent text-xs text-[var(--fg-primary)] focus:outline-none resize-none font-mono leading-relaxed"
          />

          <div className="flex items-center gap-1 shrink-0 pt-0.5">
            {/* YOLO auto-approve toggle */}
            <button
              onClick={() => {
                const next = !Boolean(liveSession?.auto_approve ?? session?.auto_approve);
                SetAgentAutoApprove(session.id, next);
                setLiveSession((prev: any) => ({ ...prev, auto_approve: next }));
              }}
              title={Boolean(liveSession?.auto_approve ?? session?.auto_approve) ? "YOLO Mode ACTIVE: All tool actions auto-approved" : "YOLO Mode OFF: Click to auto-approve tools"}
              className={cn(
                "px-1.5 py-1 text-[10px] font-mono font-bold rounded border cursor-pointer flex items-center gap-0.5 transition-colors",
                Boolean(liveSession?.auto_approve ?? session?.auto_approve)
                  ? "bg-red-500/20 border-red-500 text-red-400"
                  : "bg-[var(--bg-app)] border-[var(--border-default)] text-[var(--fg-tertiary)] hover:text-[var(--fg-secondary)]"
              )}
            >
              <IconBolt className="size-3" />
              <span>YOLO</span>
            </button>

            <button
              onClick={() => handleSendMessage()}
              disabled={!inputText.trim() || isRunning}
              className="px-2.5 py-1 bg-[var(--accent-primary)] hover:bg-[var(--accent-hover)] text-black text-xs font-bold rounded flex items-center gap-1 cursor-pointer disabled:opacity-40 transition-colors"
            >
              <IconSend className="size-3" />
              <span>Run</span>
            </button>
          </div>
        </div>

        {/* File mentions autocomplete menu */}
        {showMentionMenu && mentionResults.length > 0 && (
          <div className="absolute bottom-20 left-3 right-3 z-50 bg-[var(--bg-sidebar)] border border-[var(--border-default)] shadow-2xl max-h-36 overflow-y-auto p-1 space-y-0.5 rounded font-mono text-xs">
            <div className="text-[9px] uppercase tracking-wider text-[var(--fg-tertiary)] font-bold px-2 py-0.5">Workspace Files</div>
            {mentionResults.map((p) => (
              <button
                key={p}
                onClick={() => {
                  const atIndex = inputText.lastIndexOf("@");
                  setInputText(inputText.slice(0, atIndex) + "@" + p + " ");
                  setShowMentionMenu(false);
                  textareaRef.current?.focus();
                }}
                className="w-full text-left px-2 py-1.5 hover:bg-[var(--bg-panel)] rounded text-[var(--fg-secondary)] hover:text-[var(--fg-primary)] text-[11px] font-mono truncate cursor-pointer"
              >
                {p}
              </button>
            ))}
          </div>
        )}
      </div>
    </div>
  );
}
