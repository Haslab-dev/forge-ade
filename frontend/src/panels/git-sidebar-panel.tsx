import React, { useState, useEffect, useCallback } from "react";
import {
  IconGitBranch,
  IconPlus,
  IconMinus,
  IconTrash,
  IconCheck,
  IconUpload,
  IconSparkles,
  IconRefresh,
  IconChevronDown,
  IconChevronRight,
  IconFileText,
  IconLoader,
  IconCpu,
} from "@tabler/icons-react";
import { cn } from "../lib/utils";
import { EventsOn } from "../../wailsjs/runtime";
import {
  GetGitStatus,
  GitStage,
  GitUnstage,
  GitDiscard,
  GitCommit,
  GitPush,
  GenerateAICommitMessage,
  GetProviderProfiles,
} from "../../wailsjs/go/main/App";
import { git, llm } from "../../wailsjs/go/models";

export function GitSidebarPanel() {
  const [status, setStatus] = useState<git.GitStatusResult | null>(null);
  const [loading, setLoading] = useState(false);
  const [commitMessage, setCommitMessage] = useState("");
  const [generatingAI, setGeneratingAI] = useState(false);
  const [committing, setCommitting] = useState(false);
  const [pushing, setPushing] = useState(false);

  const [profiles, setProfiles] = useState<llm.ProviderProfile[]>([]);
  const [selectedProvider, setSelectedProvider] = useState<string>("");
  const [selectedModel, setSelectedModel] = useState<string>("");
  const [showAIConfig, setShowAIConfig] = useState<boolean>(false);

  const [expandStaged, setExpandStaged] = useState(true);
  const [expandUnstaged, setExpandUnstaged] = useState(true);
  const [expandUntracked, setExpandUntracked] = useState(true);

  const refreshStatus = useCallback(async () => {
    setLoading(true);
    try {
      const res = await GetGitStatus("");
      setStatus(res);
    } catch (err) {
      console.error("Failed to load git status:", err);
    } finally {
      setLoading(false);
    }
  }, []);

  const loadProfiles = useCallback(async () => {
    try {
      const list = await GetProviderProfiles();
      const enabled = (Array.isArray(list) ? list : []).filter((p) => p.enabled);
      setProfiles(enabled);
      if (enabled.length > 0) {
        setSelectedProvider(enabled[0].id);
        if (enabled[0].selected_models && enabled[0].selected_models.length > 0) {
          setSelectedModel(enabled[0].selected_models[0]);
        }
      }
    } catch { /* ignore */ }
  }, []);

  useEffect(() => {
    refreshStatus();
    loadProfiles();
  }, [refreshStatus, loadProfiles]);

  useEffect(() => {
    let timer: any = null;
    const dispose = EventsOn("fs:changed", () => {
      if (timer) clearTimeout(timer);
      timer = setTimeout(() => {
        refreshStatus();
      }, 500);
    });
    return () => {
      if (timer) clearTimeout(timer);
      if (typeof dispose === "function") dispose();
    };
  }, [refreshStatus]);

  // Update selected model when provider changes
  function handleProviderChange(provId: string) {
    setSelectedProvider(provId);
    const prof = profiles.find((p) => p.id === provId);
    if (prof && prof.selected_models && prof.selected_models.length > 0) {
      setSelectedModel(prof.selected_models[0]);
    } else if (prof && prof.available_models && prof.available_models.length > 0) {
      setSelectedModel(prof.available_models[0]);
    } else {
      setSelectedModel("");
    }
  }

  async function handleStage(paths: string[]) {
    try {
      await GitStage("", paths);
      refreshStatus();
    } catch (err: any) {
      alert("Stage error: " + err);
    }
  }

  async function handleUnstage(paths: string[]) {
    try {
      await GitUnstage("", paths);
      refreshStatus();
    } catch (err: any) {
      alert("Unstage error: " + err);
    }
  }

  async function handleDiscard(paths: string[]) {
    if (!confirm("Are you sure you want to discard changes for these files?")) return;
    try {
      await GitDiscard("", paths);
      refreshStatus();
    } catch (err: any) {
      alert("Discard error: " + err);
    }
  }

  async function handleGenerateAICommit() {
    if (!status?.staged || status.staged.length === 0) {
      alert("Please stage files (+) before generating AI commit message.");
      return;
    }
    setGeneratingAI(true);
    try {
      const msg = await GenerateAICommitMessage("", selectedProvider, selectedModel);
      if (msg) setCommitMessage(msg);
    } catch (err: any) {
      alert("AI Generation failed: " + (err.message || err));
    } finally {
      setGeneratingAI(false);
    }
  }

  async function handleCommit() {
    if (!commitMessage.trim()) {
      alert("Please enter a commit message.");
      return;
    }
    setCommitting(true);
    try {
      await GitCommit("", commitMessage);
      setCommitMessage("");
      refreshStatus();
    } catch (err: any) {
      alert("Commit failed: " + (err.message || err));
    } finally {
      setCommitting(false);
    }
  }

  async function handlePush() {
    setPushing(true);
    try {
      await GitPush("");
      alert("Pushed successfully to remote branch!");
      refreshStatus();
    } catch (err: any) {
      alert("Push failed: " + (err.message || err));
    } finally {
      setPushing(false);
    }
  }

  const stagedCount = status?.staged?.length || 0;
  const unstagedCount = status?.unstaged?.length || 0;
  const untrackedCount = status?.untracked?.length || 0;

  const currentProf = profiles.find((p) => p.id === selectedProvider);
  const availableModels = currentProf?.selected_models?.length
    ? currentProf.selected_models
    : currentProf?.available_models || [];

  return (
    <div className="flex flex-col h-full bg-[var(--color-bg-primary)] text-xs text-[var(--color-fg-primary)] p-2 gap-2 overflow-hidden">
      {/* Header with branch badge */}
      <div className="flex items-center justify-between border-b border-[var(--color-border)] pb-2 shrink-0">
        <div className="flex items-center space-x-1.5 font-bold text-white">
          <IconGitBranch className="w-4 h-4 text-purple-400" />
          <span className="font-mono text-purple-300">{status?.branch || "main"}</span>
        </div>
        <div className="flex items-center space-x-1">
          <button
            onClick={refreshStatus}
            disabled={loading}
            className="p-1 rounded hover:bg-[var(--color-bg-tertiary)] text-gray-300 transition-all cursor-pointer"
            title="Refresh Git Status"
          >
            <IconRefresh className={cn("w-3.5 h-3.5", loading && "animate-spin")} />
          </button>
        </div>
      </div>

      {/* Commit message & AI generation area */}
      <div className="space-y-2 shrink-0 bg-[var(--color-bg-secondary)] p-2.5 rounded-lg border border-[var(--color-border)]">
        <div className="flex items-center justify-between">
          <span className="font-semibold text-gray-300 text-[11px] uppercase tracking-wider">Commit Message</span>
          <div className="flex items-center space-x-1">
            <button
              onClick={() => setShowAIConfig(!showAIConfig)}
              className="p-1 rounded hover:bg-white/10 text-gray-400 hover:text-purple-300 cursor-pointer"
              title="Configure Provider & Model for AI Commit Generator"
            >
              <IconCpu className="w-3.5 h-3.5" />
            </button>
            <button
              onClick={handleGenerateAICommit}
              disabled={generatingAI || stagedCount === 0}
              className="px-2 py-0.5 bg-purple-600/30 hover:bg-purple-600 disabled:opacity-40 text-purple-200 hover:text-white rounded text-[10px] font-semibold flex items-center space-x-1 transition-all cursor-pointer"
              title="Generate commit message using AI from staged changes"
            >
              {generatingAI ? (
                <IconLoader className="w-3 h-3 animate-spin text-purple-300" />
              ) : (
                <IconSparkles className="w-3 h-3 text-purple-400" />
              )}
              <span>Generate AI</span>
            </button>
          </div>
        </div>

        {/* AI Provider & Model Picker Drawer */}
        {showAIConfig && (
          <div className="p-2 rounded bg-[var(--color-bg-tertiary)] border border-[var(--color-border)] space-y-1.5 text-[11px]">
            <div className="font-semibold text-purple-300 text-[10px] uppercase">AI Commit Generator Provider & Model</div>
            <div className="grid grid-cols-2 gap-1.5">
              <div>
                <label className="text-gray-400 block mb-0.5">Provider</label>
                <select
                  value={selectedProvider}
                  onChange={(e) => handleProviderChange(e.target.value)}
                  className="w-full bg-[var(--color-bg-primary)] border border-[var(--color-border)] rounded px-1.5 py-1 text-gray-200 focus:outline-none"
                >
                  {profiles.map((p) => (
                    <option key={p.id} value={p.id}>
                      {p.name}
                    </option>
                  ))}
                </select>
              </div>
              <div>
                <label className="text-gray-400 block mb-0.5">Model</label>
                <select
                  value={selectedModel}
                  onChange={(e) => setSelectedModel(e.target.value)}
                  className="w-full bg-[var(--color-bg-primary)] border border-[var(--color-border)] rounded px-1.5 py-1 text-gray-200 focus:outline-none"
                >
                  {availableModels.map((m) => (
                    <option key={m} value={m}>
                      {m}
                    </option>
                  ))}
                </select>
              </div>
            </div>
          </div>
        )}

        <textarea
          value={commitMessage}
          onChange={(e) => setCommitMessage(e.target.value)}
          placeholder="Message (Cmd+Enter to commit)..."
          rows={2}
          onKeyDown={(e) => {
            if (e.key === "Enter" && (e.metaKey || e.ctrlKey)) {
              e.preventDefault();
              handleCommit();
            }
          }}
          className="w-full bg-[var(--color-bg-primary)] border border-[var(--color-border)] rounded p-2 text-xs text-white focus:outline-none focus:border-blue-500 resize-none font-mono"
        />

        <div className="flex items-center space-x-1.5 pt-0.5">
          <button
            onClick={handleCommit}
            disabled={committing || stagedCount === 0 || !commitMessage.trim()}
            className="flex-1 py-1.5 bg-[var(--bg-surface-active)] hover:bg-[var(--bg-surface-hover)] border border-[var(--border-default)] disabled:opacity-40 text-white rounded font-semibold flex items-center justify-center space-x-1 transition-all cursor-pointer"
          >
            <IconCheck className="w-3.5 h-3.5" />
            <span>Commit ({stagedCount})</span>
          </button>
          <button
            onClick={handlePush}
            disabled={pushing}
            className="px-3 py-1.5 bg-[var(--color-bg-tertiary)] hover:bg-[var(--color-border)] disabled:opacity-40 text-gray-200 rounded font-semibold flex items-center space-x-1 transition-all cursor-pointer"
            title="Push commits to remote branch"
          >
            <IconUpload className={cn("w-3.5 h-3.5", pushing && "animate-bounce")} />
            <span>Push</span>
          </button>
        </div>
      </div>

      {/* Changes Accordion Lists */}
      <div className="flex-1 overflow-y-auto space-y-2 pr-0.5">
        {/* Staged Changes Section */}
        <div className="border border-[var(--color-border)] rounded-lg bg-[var(--color-bg-secondary)] overflow-hidden">
          <div
            onClick={() => setExpandStaged(!expandStaged)}
            className="flex items-center justify-between px-2.5 py-1.5 bg-[var(--color-bg-tertiary)] cursor-pointer select-none font-semibold text-gray-200"
          >
            <span className="flex items-center space-x-1">
              {expandStaged ? <IconChevronDown className="w-3.5 h-3.5" /> : <IconChevronRight className="w-3.5 h-3.5" />}
              <span>Staged Changes ({stagedCount})</span>
            </span>
            {stagedCount > 0 && (
              <button
                onClick={(e) => {
                  e.stopPropagation();
                  handleUnstage([]);
                }}
                className="p-1 hover:bg-white/10 rounded text-gray-400 hover:text-white cursor-pointer"
                title="Unstage All"
              >
                <IconMinus className="w-3.5 h-3.5" />
              </button>
            )}
          </div>

          {expandStaged && (
            <div className="p-1 space-y-0.5">
              {stagedCount === 0 ? (
                <div className="text-[11px] text-gray-500 italic p-2 text-center">No staged changes</div>
              ) : (
                status?.staged?.map((file) => (
                  <div key={file.path} className="flex items-center justify-between p-1 rounded hover:bg-white/5 group">
                    <div className="flex items-center space-x-1.5 min-w-0 font-mono">
                      <span className="text-emerald-400 font-bold text-[10px] w-3">{file.status}</span>
                      <span className="truncate text-gray-200">{file.path}</span>
                    </div>
                    <button
                      onClick={() => handleUnstage([file.path])}
                      className="p-1 hover:bg-white/10 rounded text-gray-400 hover:text-white cursor-pointer"
                      title="Unstage File"
                    >
                      <IconMinus className="w-3.5 h-3.5" />
                    </button>
                  </div>
                ))
              )}
            </div>
          )}
        </div>

        {/* Unstaged / Modified Changes Section */}
        <div className="border border-[var(--color-border)] rounded-lg bg-[var(--color-bg-secondary)] overflow-hidden">
          <div
            onClick={() => setExpandUnstaged(!expandUnstaged)}
            className="flex items-center justify-between px-2.5 py-1.5 bg-[var(--color-bg-tertiary)] cursor-pointer select-none font-semibold text-gray-200"
          >
            <span className="flex items-center space-x-1">
              {expandUnstaged ? <IconChevronDown className="w-3.5 h-3.5" /> : <IconChevronRight className="w-3.5 h-3.5" />}
              <span>Changes ({unstagedCount})</span>
            </span>
            {unstagedCount > 0 && (
              <button
                onClick={(e) => {
                  e.stopPropagation();
                  handleStage([]);
                }}
                className="p-1 hover:bg-white/10 rounded text-gray-400 hover:text-white cursor-pointer"
                title="Stage All"
              >
                <IconPlus className="w-3.5 h-3.5" />
              </button>
            )}
          </div>

          {expandUnstaged && (
            <div className="p-1 space-y-0.5">
              {unstagedCount === 0 ? (
                <div className="text-[11px] text-gray-500 italic p-2 text-center">No modified changes</div>
              ) : (
                status?.unstaged?.map((file) => (
                  <div key={file.path} className="flex items-center justify-between p-1 rounded hover:bg-white/5 group">
                    <div className="flex items-center space-x-1.5 min-w-0 font-mono">
                      <span className="text-amber-400 font-bold text-[10px] w-3">{file.status}</span>
                      <span className="truncate text-gray-200">{file.path}</span>
                    </div>
                    <div className="flex items-center space-x-1">
                      <button
                        onClick={() => handleStage([file.path])}
                        className="p-1 hover:bg-white/10 rounded text-gray-400 hover:text-white cursor-pointer"
                        title="Stage File"
                      >
                        <IconPlus className="w-3.5 h-3.5" />
                      </button>
                      <button
                        onClick={() => handleDiscard([file.path])}
                        className="p-1 hover:bg-rose-500/20 rounded text-gray-400 hover:text-rose-400 cursor-pointer"
                        title="Discard Changes"
                      >
                        <IconTrash className="w-3.5 h-3.5" />
                      </button>
                    </div>
                  </div>
                ))
              )}
            </div>
          )}
        </div>

        {/* Untracked Files Section */}
        <div className="border border-[var(--color-border)] rounded-lg bg-[var(--color-bg-secondary)] overflow-hidden">
          <div
            onClick={() => setExpandUntracked(!expandUntracked)}
            className="flex items-center justify-between px-2.5 py-1.5 bg-[var(--color-bg-tertiary)] cursor-pointer select-none font-semibold text-gray-200"
          >
            <span className="flex items-center space-x-1">
              {expandUntracked ? <IconChevronDown className="w-3.5 h-3.5" /> : <IconChevronRight className="w-3.5 h-3.5" />}
              <span>Untracked ({untrackedCount})</span>
            </span>
            {untrackedCount > 0 && (
              <button
                onClick={(e) => {
                  e.stopPropagation();
                  handleStage([]);
                }}
                className="p-1 hover:bg-white/10 rounded text-gray-400 hover:text-white cursor-pointer"
                title="Stage All Untracked"
              >
                <IconPlus className="w-3.5 h-3.5" />
              </button>
            )}
          </div>

          {expandUntracked && (
            <div className="p-1 space-y-0.5">
              {untrackedCount === 0 ? (
                <div className="text-[11px] text-gray-500 italic p-2 text-center">No untracked files</div>
              ) : (
                status?.untracked?.map((file) => (
                  <div key={file.path} className="flex items-center justify-between p-1 rounded hover:bg-white/5 group">
                    <div className="flex items-center space-x-1.5 min-w-0 font-mono">
                      <span className="text-purple-400 font-bold text-[10px] w-3">?</span>
                      <span className="truncate text-gray-200">{file.path}</span>
                    </div>
                    <div className="flex items-center space-x-1">
                      <button
                        onClick={() => handleStage([file.path])}
                        className="p-1 hover:bg-white/10 rounded text-gray-400 hover:text-white cursor-pointer"
                        title="Stage File"
                      >
                        <IconPlus className="w-3.5 h-3.5" />
                      </button>
                      <button
                        onClick={() => handleDiscard([file.path])}
                        className="p-1 hover:bg-rose-500/20 rounded text-gray-400 hover:text-rose-400 cursor-pointer"
                        title="Discard File"
                      >
                        <IconTrash className="w-3.5 h-3.5" />
                      </button>
                    </div>
                  </div>
                ))
              )}
            </div>
          )}
        </div>
      </div>
    </div>
  );
}
