import React, { useState, useEffect, useRef } from "react";
import {
  IconRobot,
  IconSend,
  IconCheckbox,
  IconSquare,
  IconChevronDown,
  IconChevronRight,
  IconBrain,
  IconShield,
  IconSparkles,
  IconPaperclip,
  IconPlus,
  IconTrash,
  IconCpu,
  IconFileText,
  IconStack,
  IconCode,
  IconCompass,
  IconSettings,
  IconServer,
  IconCheck,
} from "@tabler/icons-react";
import { cn } from "../lib/utils";
import {
  CreateAgentSession,
  ListAgentSessions,
  SendAgentMessage,
  RespondAgentApproval,
  ToggleAgentTask,
  SearchFilename,
  GetLLMConfig,
  GetProviderProfiles,
  SetActiveModel,
} from "../../wailsjs/go/main/App";
import { EventsOn } from "../../wailsjs/runtime";
import { ProviderModal, ProviderProfile } from "../components/provider-modal";

interface AgentMessage {
  id: string;
  role: string;
  content: string;
  reasoning?: string;
  tool_calls?: any[];
  timestamp: string;
}

interface TaskItem {
  id: string;
  title: string;
  completed: boolean;
}

interface TokenUsage {
  prompt_tokens: number;
  completion_tokens: number;
  cached_tokens: number;
  total_tokens: number;
}

interface AgentSession {
  id: string;
  name: string;
  role_filter: string;
  state: string;
  messages: AgentMessage[];
  tasks: TaskItem[];
  token_usage: TokenUsage;
  auto_approve: boolean;
  pending_tool?: any;
}

export function AgentScreen() {
  const [sessions, setSessions] = useState<AgentSession[]>([]);
  const [activeSessionId, setActiveSessionId] = useState<string | null>(null);
  const [inputText, setInputText] = useState("");
  const [roleFilter, setRoleFilter] = useState<"coding" | "planning" | "research" | "custom">("coding");
  
  // @ mention autocomplete state
  const [showMentionMenu, setShowMentionMenu] = useState(false);
  const [mentionQuery, setMentionQuery] = useState("");
  const [mentionResults, setMentionResults] = useState<string[]>([]);
  const [attachedFiles, setAttachedFiles] = useState<string[]>([]);

  // UI Toggles
  const [showReasoning, setShowReasoning] = useState<Record<string, boolean>>({});
  const [showToDos, setShowToDos] = useState(true);

  // Model & Provider Accordion State
  const [llmConfig, setLlmConfig] = useState<any>(null);
  const [profiles, setProfiles] = useState<ProviderProfile[]>([]);
  const [showModelDropdown, setShowModelDropdown] = useState(false);
  const [expandedAccordion, setExpandedAccordion] = useState<Record<string, boolean>>({});
  const [showProviderModal, setShowProviderModal] = useState(false);

  const chatEndRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    loadSessions();
    loadLLM();

    const unsub = EventsOn("agent:updated", () => {
      loadSessions();
    });

    return () => {
      if (typeof unsub === "function") unsub();
    };
  }, []);

  useEffect(() => {
    chatEndRef.current?.scrollIntoView({ behavior: "smooth" });
  }, [sessions, activeSessionId]);

  async function loadSessions() {
    try {
      const list: AgentSession[] = await ListAgentSessions();
      setSessions(Array.isArray(list) ? list : []);
      if (list && list.length > 0 && !activeSessionId) {
        setActiveSessionId(list[0].id);
      }
    } catch { /* ignore */ }
  }

  async function loadLLM() {
    try {
      const cfg = await GetLLMConfig();
      const profs = await GetProviderProfiles();
      setLlmConfig(cfg);
      setProfiles(Array.isArray(profs) ? profs : []);
    } catch { /* ignore */ }
  }

  const activeSession = sessions.find((s) => s.id === activeSessionId) || null;

  async function handleCreateSession() {
    try {
      const sess: AgentSession = await CreateAgentSession("", roleFilter, "");
      setSessions((prev) => [...prev, sess]);
      setActiveSessionId(sess.id);
    } catch (err) {
      console.error("Failed to create agent session:", err);
    }
  }

  async function handleSendMessage() {
    if (!activeSessionId || (!inputText.trim() && attachedFiles.length === 0)) return;
    const textToSend = inputText;
    const filesToSend = [...attachedFiles];
    setInputText("");
    setAttachedFiles([]);
    setShowMentionMenu(false);

    try {
      await SendAgentMessage(activeSessionId, textToSend, filesToSend);
      loadSessions();
    } catch (err) {
      console.error("Failed to send agent message:", err);
    }
  }

  async function handleApproval(approve: boolean, autoApproveAll: boolean) {
    if (!activeSessionId) return;
    try {
      await RespondAgentApproval(activeSessionId, approve, autoApproveAll);
      loadSessions();
    } catch (err) {
      console.error("Failed approval response:", err);
    }
  }

  async function handleSelectModel(providerID: string, model: string) {
    try {
      await SetActiveModel(providerID, model);
      await loadLLM();
      setShowModelDropdown(false);
    } catch (err) {
      console.error("Failed to set active model:", err);
    }
  }

  async function handleInputChange(e: React.ChangeEvent<HTMLTextAreaElement>) {
    const val = e.target.value;
    setInputText(val);

    const atIndex = val.lastIndexOf("@");
    if (atIndex !== -1 && atIndex >= val.length - 20) {
      const query = val.slice(atIndex + 1);
      setMentionQuery(query);
      setShowMentionMenu(true);
      try {
        const results = await SearchFilename(query, 8);
        setMentionResults(results.map((r: any) => r.path ?? r.Path));
      } catch {
        setMentionResults([]);
      }
    } else {
      setShowMentionMenu(false);
    }
  }

  function attachFile(path: string) {
    if (!attachedFiles.includes(path)) {
      setAttachedFiles((prev) => [...prev, path]);
    }
    const atIndex = inputText.lastIndexOf("@");
    if (atIndex !== -1) {
      setInputText(inputText.slice(0, atIndex));
    }
    setShowMentionMenu(false);
  }

  return (
    <div className="flex flex-col h-full bg-[var(--color-bg-primary)] text-[var(--color-fg-primary)]">
      {/* Session Top Navigation & Tokens Meter */}
      <div className="flex items-center justify-between border-b border-[var(--color-border)] px-4 py-2 bg-[var(--color-bg-secondary)]">
        <div className="flex items-center space-x-2 overflow-x-auto">
          {sessions.map((s) => (
            <button
              key={s.id}
              onClick={() => setActiveSessionId(s.id)}
              className={cn(
                "px-3 py-1.5 rounded-md text-xs font-medium flex items-center space-x-2 transition-all",
                activeSessionId === s.id
                  ? "bg-blue-600 text-white shadow-sm"
                  : "bg-[var(--color-bg-tertiary)] hover:bg-[var(--color-border)] text-gray-300"
              )}
            >
              <IconRobot className="w-3.5 h-3.5" />
              <span className="truncate max-w-[120px]">{s.name}</span>
              <span className="text-[10px] opacity-75 uppercase">({s.role_filter})</span>
            </button>
          ))}
          <button
            onClick={handleCreateSession}
            className="p-1.5 rounded-md bg-[var(--color-bg-tertiary)] hover:bg-[var(--color-border)] text-gray-300 flex items-center space-x-1 text-xs"
            title="New Agent Session"
          >
            <IconPlus className="w-3.5 h-3.5" />
            <span>New Session</span>
          </button>
        </div>

        {/* Live Token Usage Bar */}
        {activeSession && (
          <div className="flex items-center space-x-4 text-xs font-mono bg-black/20 px-3 py-1 rounded-md border border-[var(--color-border)]">
            <div className="flex items-center space-x-1 text-blue-400">
              <span>Tokens ⬆️</span>
              <span className="font-bold">{((activeSession.token_usage?.prompt_tokens || 0) / 1000).toFixed(1)}k</span>
            </div>
            <div className="flex items-center space-x-1 text-emerald-400">
              <span>cache ⬆️</span>
              <span className="font-bold">{((activeSession.token_usage?.cached_tokens || 0) / 1000).toFixed(1)}k</span>
            </div>
            <div className="flex items-center space-x-1 text-purple-400">
              <span>out ⬇️</span>
              <span className="font-bold">{((activeSession.token_usage?.completion_tokens || 0) / 1000).toFixed(1)}k</span>
            </div>
          </div>
        )}
      </div>

      {/* Main Conversation Body */}
      <div className="flex-1 overflow-y-auto p-4 space-y-4">
        {!activeSession ? (
          <div className="flex flex-col items-center justify-center h-full text-center space-y-4 text-gray-400">
            <IconRobot className="w-16 h-16 text-blue-500 animate-pulse" />
            <h2 className="text-xl font-semibold text-white">ForgeADE AI Agent</h2>
            <p className="max-w-md text-sm">
              Launch an autonomous agent session for coding, planning, or codebase research.
            </p>
            <button
              onClick={handleCreateSession}
              className="px-4 py-2 bg-blue-600 hover:bg-blue-500 text-white rounded-lg font-medium text-sm flex items-center space-x-2 shadow-lg"
            >
              <IconPlus className="w-4 h-4" />
              <span>Start Agent Session</span>
            </button>
          </div>
        ) : (
          <>
            {/* Task Checklist Panel */}
            {activeSession.tasks && activeSession.tasks.length > 0 && (
              <div className="border border-[var(--color-border)] rounded-lg bg-[var(--color-bg-secondary)] p-3 space-y-2">
                <button
                  onClick={() => setShowToDos(!showToDos)}
                  className="flex items-center justify-between w-full text-xs font-semibold text-gray-300 uppercase tracking-wider"
                >
                  <span className="flex items-center space-x-1.5">
                    <IconCheckbox className="w-4 h-4 text-blue-400" />
                    <span>To-Dos ({activeSession.tasks.filter((t) => t.completed).length}/{activeSession.tasks.length})</span>
                  </span>
                  {showToDos ? <IconChevronDown className="w-4 h-4" /> : <IconChevronRight className="w-4 h-4" />}
                </button>
                {showToDos && (
                  <div className="space-y-1.5 pt-1">
                    {activeSession.tasks.map((task) => (
                      <div
                        key={task.id}
                        onClick={() => ToggleAgentTask(activeSession.id, task.id, !task.completed)}
                        className="flex items-center space-x-2 text-sm cursor-pointer hover:bg-[var(--color-bg-tertiary)] p-1 rounded transition-all"
                      >
                        {task.completed ? (
                          <IconCheckbox className="w-4 h-4 text-emerald-400" />
                        ) : (
                          <IconSquare className="w-4 h-4 text-gray-500" />
                        )}
                        <span className={cn(task.completed && "line-through text-gray-500")}>
                          {task.title}
                        </span>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            )}

            {/* Messages Log */}
            {activeSession.messages.map((msg) => (
              <div
                key={msg.id}
                className={cn(
                  "flex flex-col space-y-2 rounded-lg p-3 max-w-3xl",
                  msg.role === "user"
                    ? "ml-auto bg-blue-600/20 border border-blue-500/30 text-white"
                    : msg.role === "tool"
                    ? "bg-zinc-900/80 border border-zinc-700/50 font-mono text-xs text-emerald-300"
                    : "bg-[var(--color-bg-secondary)] border border-[var(--color-border)]"
                )}
              >
                <div className="flex items-center justify-between text-xs text-gray-400">
                  <span className="font-semibold uppercase flex items-center space-x-1">
                    {msg.role === "user" ? (
                      "User"
                    ) : msg.role === "tool" ? (
                      "Tool Result"
                    ) : (
                      <>
                        <IconSparkles className="w-3.5 h-3.5 text-blue-400" />
                        <span>Agent ({activeSession.role_filter})</span>
                      </>
                    )}
                  </span>
                  <span>{new Date(msg.timestamp).toLocaleTimeString()}</span>
                </div>

                {/* Collapsible Reasoning Block */}
                {msg.reasoning && (
                  <div className="border border-purple-500/30 bg-purple-950/20 rounded p-2 text-xs">
                    <button
                      onClick={() =>
                        setShowReasoning((prev) => ({ ...prev, [msg.id]: !prev[msg.id] }))
                      }
                      className="flex items-center space-x-1 text-purple-300 font-medium"
                    >
                      <IconBrain className="w-3.5 h-3.5" />
                      <span>Reasoning</span>
                      {showReasoning[msg.id] ? (
                        <IconChevronDown className="w-3.5 h-3.5" />
                      ) : (
                        <IconChevronRight className="w-3.5 h-3.5" />
                      )}
                    </button>
                    {showReasoning[msg.id] && (
                      <p className="mt-2 text-purple-200/80 whitespace-pre-wrap italic">
                        {msg.reasoning}
                      </p>
                    )}
                  </div>
                )}

                {/* Message Content */}
                <div className="text-sm whitespace-pre-wrap leading-relaxed">{msg.content}</div>

                {/* Tool Calls Execution List */}
                {msg.tool_calls && msg.tool_calls.length > 0 && (
                  <div className="space-y-1.5 pt-2">
                    {msg.tool_calls.map((tc, idx) => (
                      <div
                        key={idx}
                        className="bg-black/40 border border-zinc-700 rounded p-2 text-xs font-mono space-y-1"
                      >
                        <div className="text-amber-400 font-bold">
                          ⚡ Call Tool: {tc.function?.name}
                        </div>
                        <div className="text-gray-300 text-[11px] truncate">
                          Args: {tc.function?.arguments}
                        </div>
                      </div>
                    ))}
                  </div>
                )}
              </div>
            ))}

            {/* Pending Tool Approval Banner */}
            {activeSession.state === "awaiting_approval" && activeSession.pending_tool && (
              <div className="border border-amber-500/50 bg-amber-950/30 rounded-lg p-4 space-y-3">
                <div className="flex items-center space-x-2 text-amber-400 font-semibold">
                  <IconShield className="w-5 h-5" />
                  <span>Human-in-the-Loop Approval Required</span>
                </div>
                <div className="text-xs font-mono bg-black/50 p-2 rounded text-gray-200">
                  Tool: <span className="text-amber-300 font-bold">{activeSession.pending_tool.function?.name}</span>
                  <br />
                  Arguments: {activeSession.pending_tool.function?.arguments}
                </div>
                <div className="flex items-center space-x-2 pt-1">
                  <button
                    onClick={() => handleApproval(true, false)}
                    className="px-3 py-1.5 bg-emerald-600 hover:bg-emerald-500 text-white rounded text-xs font-medium"
                  >
                    Approve (y)
                  </button>
                  <button
                    onClick={() => handleApproval(false, false)}
                    className="px-3 py-1.5 bg-rose-600 hover:bg-rose-500 text-white rounded text-xs font-medium"
                  >
                    Reject (n)
                  </button>
                  <button
                    onClick={() => handleApproval(true, true)}
                    className="px-3 py-1.5 bg-blue-600 hover:bg-blue-500 text-white rounded text-xs font-medium"
                  >
                    Always Approve (yy)
                  </button>
                </div>
              </div>
            )}

            <div ref={chatEndRef} />
          </>
        )}
      </div>

      {/* Input Section & Autocomplete */}
      {activeSession && (
        <div className="border-t border-[var(--color-border)] p-3 bg-[var(--color-bg-secondary)] space-y-2 relative">
          {/* Autocomplete Menu for @ Mentions */}
          {showMentionMenu && mentionResults.length > 0 && (
            <div className="absolute bottom-full mb-2 left-3 right-3 max-h-48 overflow-y-auto bg-[var(--color-bg-tertiary)] border border-[var(--color-border)] rounded-lg shadow-2xl z-50 p-1 space-y-1">
              <div className="text-[10px] font-semibold text-gray-400 uppercase px-2 py-1">
                Mention File / Folder Context (@)
              </div>
              {mentionResults.map((path) => (
                <button
                  key={path}
                  onClick={() => attachFile(path)}
                  className="w-full text-left px-2 py-1 rounded text-xs hover:bg-blue-600 hover:text-white flex items-center space-x-2 truncate"
                >
                  <IconFileText className="w-3.5 h-3.5 shrink-0" />
                  <span className="truncate">{path}</span>
                </button>
              ))}
            </div>
          )}

          {/* Provider & Model Accordion Popover Menu */}
          {showModelDropdown && (
            <div className="absolute bottom-full mb-2 left-3 w-80 max-h-72 overflow-y-auto bg-[var(--color-bg-tertiary)] border border-[var(--color-border)] rounded-lg shadow-2xl z-50 p-2 space-y-1 text-xs">
              <div className="flex items-center justify-between px-2 py-1 border-b border-[var(--color-border)] text-gray-400 font-semibold uppercase text-[10px]">
                <span>Select LLM Model</span>
                <button
                  onClick={() => {
                    setShowModelDropdown(false);
                    setShowProviderModal(true);
                  }}
                  className="text-blue-400 hover:text-blue-300 flex items-center space-x-1"
                >
                  <IconSettings className="w-3 h-3" />
                  <span>Config</span>
                </button>
              </div>

              {profiles
                .filter((p) => p.enabled)
                .map((prof) => (
                  <div key={prof.id} className="border border-[var(--color-border)]/50 rounded overflow-hidden">
                    <button
                      onClick={() => setExpandedAccordion((prev) => ({ ...prev, [prof.id]: !prev[prof.id] }))}
                      className="w-full flex items-center justify-between px-2.5 py-1.5 bg-black/30 hover:bg-white/5 font-semibold text-gray-200"
                    >
                      <span className="flex items-center space-x-1.5">
                        <IconServer className="w-3.5 h-3.5 text-blue-400" />
                        <span>{prof.name}</span>
                      </span>
                      {expandedAccordion[prof.id] ? <IconChevronDown className="w-3.5 h-3.5" /> : <IconChevronRight className="w-3.5 h-3.5" />}
                    </button>

                    {expandedAccordion[prof.id] && (
                      <div className="p-1 space-y-0.5 bg-black/10 font-mono">
                        {(prof.selected_models && prof.selected_models.length > 0
                          ? prof.selected_models
                          : prof.available_models || []
                        ).map((model) => {
                          const isActive = llmConfig?.provider_id === prof.id && llmConfig?.model === model;
                          return (
                            <button
                              key={model}
                              onClick={() => handleSelectModel(prof.id, model)}
                              className={cn(
                                "w-full text-left px-3 py-1 rounded flex items-center justify-between transition-colors",
                                isActive
                                  ? "bg-blue-600 text-white font-bold"
                                  : "hover:bg-blue-600/30 text-gray-300"
                              )}
                            >
                              <span className="truncate">{model}</span>
                              {isActive && <IconCheck className="w-3.5 h-3.5 shrink-0" />}
                            </button>
                          );
                        })}
                      </div>
                    )}
                  </div>
                ))}
            </div>
          )}

          {/* Context Badges */}
          {attachedFiles.length > 0 && (
            <div className="flex flex-wrap gap-1.5">
              {attachedFiles.map((path) => (
                <span
                  key={path}
                  className="bg-blue-600/30 border border-blue-500/40 text-blue-200 text-xs px-2 py-0.5 rounded-md flex items-center space-x-1"
                >
                  <IconPaperclip className="w-3 h-3" />
                  <span className="truncate max-w-[180px]">{path}</span>
                  <button
                    onClick={() => setAttachedFiles(attachedFiles.filter((p) => p !== path))}
                    className="hover:text-white"
                  >
                    ×
                  </button>
                </span>
              ))}
            </div>
          )}

          {/* Text Area Input */}
          <textarea
            value={inputText}
            onChange={handleInputChange}
            onKeyDown={(e) => {
              if (e.key === "Enter" && !e.shiftKey) {
                e.preventDefault();
                handleSendMessage();
              }
            }}
            placeholder="Type a message, @ to mention files... (Enter to send, Shift+Enter for new line)"
            rows={2}
            className="w-full bg-[var(--color-bg-primary)] border border-[var(--color-border)] rounded-lg p-2.5 text-sm text-white focus:outline-none focus:border-blue-500 resize-none"
          />

          {/* Action Bar (Filters & Accordion Model Selection) */}
          <div className="flex items-center justify-between">
            <div className="flex items-center space-x-2">
              {/* Agent Role Filter Select */}
              <select
                value={roleFilter}
                onChange={(e: any) => setRoleFilter(e.target.value)}
                className="bg-[var(--color-bg-tertiary)] border border-[var(--color-border)] rounded px-2.5 py-1 text-xs text-gray-200 focus:outline-none"
              >
                <option value="coding">Code</option>
                <option value="planning">Planning</option>
                <option value="research">Research</option>
                <option value="custom">Custom</option>
              </select>

              {/* Accordion Model Selector Button */}
              {llmConfig && (
                <button
                  onClick={() => setShowModelDropdown(!showModelDropdown)}
                  className="text-xs bg-black/40 hover:bg-black/60 border border-[var(--color-border)] px-2.5 py-1 rounded text-gray-200 flex items-center space-x-1.5 transition-all"
                >
                  <IconCpu className="w-3.5 h-3.5 text-purple-400" />
                  <span className="font-mono">{llmConfig.provider_id} / {llmConfig.model}</span>
                  <IconChevronDown className="w-3 h-3 opacity-70" />
                </button>
              )}

              {/* Provider Config Modal Launcher */}
              <button
                onClick={() => setShowProviderModal(true)}
                className="p-1 rounded bg-[var(--color-bg-tertiary)] hover:bg-[var(--color-border)] text-gray-300"
                title="Configure Providers & Fetch Models"
              >
                <IconSettings className="w-3.5 h-3.5" />
              </button>
            </div>

            <button
              onClick={handleSendMessage}
              disabled={activeSession.state === "thinking" || activeSession.state === "executing"}
              className="px-4 py-1.5 bg-blue-600 hover:bg-blue-500 disabled:opacity-50 text-white rounded-lg text-xs font-semibold flex items-center space-x-1.5 shadow"
            >
              <IconSend className="w-3.5 h-3.5" />
              <span>Send</span>
            </button>
          </div>
        </div>
      )}

      {/* Provider Config Modal */}
      <ProviderModal
        open={showProviderModal}
        onClose={() => setShowProviderModal(false)}
        onSaved={loadLLM}
      />
    </div>
  );
}
