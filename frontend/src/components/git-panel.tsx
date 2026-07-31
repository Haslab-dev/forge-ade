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
} from "@tabler/icons-react";
import { cn } from "../lib/utils";
import {
  GetGitStatus,
  GitStage,
  GitUnstage,
  GitDiscard,
  GitCommit,
  GitPush,
  GenerateAICommitMessage,
  GetProviderProfiles,
} from "../lib/wails";
import { globalOpenFile } from "../panels/editor";

function getStatusColorClass(status: string) {
  switch (status) {
    case "M":
      return "text-amber-400";
    case "A":
      return "text-emerald-400";
    case "D":
      return "text-rose-400";
    case "R":
      return "text-cyan-400";
    case "?":
    default:
      return "text-gray-450";
  }
}

export function GitPanel() {
  const [status, setStatus] = useState<any>(null);
  const [loading, setLoading] = useState(false);
  const [commitMessage, setCommitMessage] = useState("");
  const [generatingAI, setGeneratingAI] = useState(false);
  const [committing, setCommitting] = useState(false);
  const [pushing, setPushing] = useState(false);

  const [profiles, setProfiles] = useState<any[]>([]);
  const [selectedProvider, setSelectedProvider] = useState<string>("");
  const [selectedModel, setSelectedModel] = useState<string>("");

  const [expandStaged, setExpandStaged] = useState(true);
  const [expandUnstaged, setExpandUnstaged] = useState(true);
  const [expandUntracked, setExpandUntracked] = useState(true);

  const refreshStatus = useCallback(async () => {
    setLoading(true);
    try {
      const resStr = await GetGitStatus("");
      if (resStr) {
        setStatus(resStr);
      } else {
        setStatus(null);
      }
    } catch (err) {
      console.error("Failed to load git status:", err);
    } finally {
      setLoading(false);
    }
  }, []);

  const loadProfiles = useCallback(async () => {
    try {
      const list = await GetProviderProfiles();
      const enabled = (Array.isArray(list) ? list : []).filter((p) => p.enabled || p.Enabled);
      setProfiles(enabled);
      if (enabled.length > 0) {
        const id = enabled[0].id || enabled[0].Id;
        setSelectedProvider(id);
        const models = enabled[0].selected_models || enabled[0].SelectedModels || [];
        if (models.length > 0) {
          setSelectedModel(models[0]);
        }
      }
    } catch { /* ignore */ }
  }, []);

  useEffect(() => {
    refreshStatus();
    loadProfiles();
  }, [refreshStatus, loadProfiles]);

  async function handleStage(path: string) {
    try {
      await GitStage("", [path]);
      refreshStatus();
    } catch (err) {
      console.error(err);
    }
  }

  async function handleUnstage(path: string) {
    try {
      await GitUnstage("", [path]);
      refreshStatus();
    } catch (err) {
      console.error(err);
    }
  }

  async function handleDiscard(path: string) {
    if (!confirm(`Discard changes in ${path}? This cannot be undone.`)) return;
    try {
      await GitDiscard("", [path]);
      refreshStatus();
    } catch (err) {
      console.error(err);
    }
  }

  async function handleStageAll(paths: string[]) {
    if (paths.length === 0) return;
    try {
      await GitStage("", paths);
      refreshStatus();
    } catch (err) {
      console.error(err);
    }
  }

  async function handleUnstageAll(paths: string[]) {
    if (paths.length === 0) return;
    try {
      await GitUnstage("", paths);
      refreshStatus();
    } catch (err) {
      console.error(err);
    }
  }

  async function handleGenerateAICommit() {
    if (!status?.staged || status.staged.length === 0) {
      alert("Stage files (+) first before generating AI commit message.");
      return;
    }
    setGeneratingAI(true);
    try {
      const msg = await GenerateAICommitMessage("", selectedProvider, selectedModel);
      if (msg) setCommitMessage(msg);
    } catch (err: any) {
      alert("AI Commit failed: " + err);
    } finally {
      setGeneratingAI(false);
    }
  }

  async function handleCommit() {
    if (!commitMessage.trim()) return;
    setCommitting(true);
    try {
      await GitCommit("", commitMessage);
      setCommitMessage("");
      refreshStatus();
    } catch (err: any) {
      alert("Commit failed: " + err);
    } finally {
      setCommitting(false);
    }
  }

  async function handlePush() {
    setPushing(true);
    try {
      await GitPush("");
      alert("Pushed successfully!");
      refreshStatus();
    } catch (err: any) {
      alert("Push failed: " + err);
    } finally {
      setPushing(false);
    }
  }

  return (
    <div className="flex flex-col h-full bg-[var(--bg-sidebar)] select-none text-xs font-sans">
      {/* Header */}
      <div className="flex items-center justify-between px-3 py-2 border-b border-[var(--border-default)] shrink-0 bg-[var(--bg-panel)]">
        <div className="flex items-center gap-1.5 font-bold text-[10px] uppercase tracking-wider text-[var(--fg-tertiary)]">
          <IconGitBranch className="size-3.5 text-purple-400" />
          <span>Source Control</span>
        </div>
        <button
          onClick={refreshStatus}
          disabled={loading}
          className="p-1 hover:bg-[var(--bg-surface-hover)] rounded text-[var(--fg-secondary)] hover:text-white cursor-pointer disabled:opacity-50"
        >
          <IconRefresh className={cn("size-3.5", loading && "animate-spin")} />
        </button>
      </div>

      {/* Changes list */}
      <div className="flex-1 overflow-y-auto p-2 space-y-3">
        {/* Staged Changes */}
        <div className="space-y-1">
          <div
            onClick={() => setExpandStaged(!expandStaged)}
            className="flex items-center justify-between px-1.5 py-1 hover:bg-[var(--bg-surface-hover)] cursor-pointer text-[10px] font-bold uppercase tracking-wide text-[var(--fg-tertiary)]"
          >
            <div className="flex items-center gap-1">
              {expandStaged ? <IconChevronDown className="size-3" /> : <IconChevronRight className="size-3" />}
              <span>Staged ({status?.staged?.length ?? 0})</span>
            </div>
            {status?.staged?.length > 0 && (
              <button
                onClick={(e) => {
                  e.stopPropagation();
                  handleUnstageAll(status.staged.map((s: any) => s.path));
                }}
                className="p-0.5 hover:bg-[var(--bg-surface-hover)] text-amber-500 rounded"
                title="Unstage all"
              >
                <IconMinus className="size-3" />
              </button>
            )}
          </div>
          {expandStaged && (
            <div className="space-y-0.5 pl-1.5">
              {status?.staged?.map((item: any) => (
                <div
                  key={item.path}
                  onClick={() => globalOpenFile(item.path)}
                  className="flex items-center justify-between p-1 hover:bg-[var(--bg-panel)] group rounded cursor-pointer"
                >
                  <span className={cn("truncate font-mono text-[11px]", getStatusColorClass(item.status))}>{item.path.split("/").pop()}</span>
                  <div className="flex items-center space-x-1.5">
                    <span className={cn("text-[10px] font-mono font-bold select-none", getStatusColorClass(item.status))}>{item.status}</span>
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        handleUnstage(item.path);
                      }}
                      className="p-0.5 hover:bg-[var(--bg-surface-hover)] text-amber-500 rounded opacity-0 group-hover:opacity-100 transition-opacity"
                      title="Unstage"
                    >
                      <IconMinus className="size-3" />
                    </button>
                  </div>
                </div>
              ))}
              {(!status?.staged || status.staged.length === 0) && (
                <div className="text-[10px] text-[var(--fg-tertiary)] italic pl-3">No staged changes</div>
              )}
            </div>
          )}
        </div>

        {/* Unstaged Changes */}
        <div className="space-y-1">
          <div
            onClick={() => setExpandUnstaged(!expandUnstaged)}
            className="flex items-center justify-between px-1.5 py-1 hover:bg-[var(--bg-surface-hover)] cursor-pointer text-[10px] font-bold uppercase tracking-wide text-[var(--fg-tertiary)]"
          >
            <div className="flex items-center gap-1">
              {expandUnstaged ? <IconChevronDown className="size-3" /> : <IconChevronRight className="size-3" />}
              <span>Unstaged ({status?.unstaged?.length ?? 0})</span>
            </div>
            {status?.unstaged?.length > 0 && (
              <button
                onClick={(e) => {
                  e.stopPropagation();
                  handleStageAll(status.unstaged.map((s: any) => s.path));
                }}
                className="p-0.5 hover:bg-[var(--bg-surface-hover)] text-green-500 rounded"
                title="Stage all"
              >
                <IconPlus className="size-3" />
              </button>
            )}
          </div>
          {expandUnstaged && (
            <div className="space-y-0.5 pl-1.5">
              {status?.unstaged?.map((item: any) => (
                <div
                  key={item.path}
                  onClick={() => globalOpenFile(item.path)}
                  className="flex items-center justify-between p-1 hover:bg-[var(--bg-panel)] group rounded cursor-pointer"
                >
                  <span className={cn("truncate font-mono text-[11px]", getStatusColorClass(item.status))}>{item.path.split("/").pop()}</span>
                  <div className="flex items-center space-x-1.5">
                    <span className={cn("text-[10px] font-mono font-bold select-none", getStatusColorClass(item.status))}>{item.status}</span>
                    <div className="flex items-center space-x-1 opacity-0 group-hover:opacity-100 transition-opacity">
                      <button
                        onClick={(e) => {
                          e.stopPropagation();
                          handleStage(item.path);
                        }}
                        className="p-0.5 hover:bg-[var(--bg-surface-hover)] text-green-500 rounded"
                        title="Stage"
                      >
                        <IconPlus className="size-3" />
                      </button>
                      <button
                        onClick={(e) => {
                          e.stopPropagation();
                          handleDiscard(item.path);
                        }}
                        className="p-0.5 hover:bg-[var(--bg-surface-hover)] text-red-500 rounded"
                        title="Discard"
                      >
                        <IconTrash className="size-3" />
                      </button>
                    </div>
                  </div>
                </div>
              ))}
              {(!status?.unstaged || status.unstaged.length === 0) && (
                <div className="text-[10px] text-[var(--fg-tertiary)] italic pl-3">No unstaged changes</div>
              )}
            </div>
          )}
        </div>

        {/* Untracked Files */}
        <div className="space-y-1">
          <div
            onClick={() => setExpandUntracked(!expandUntracked)}
            className="flex items-center justify-between px-1.5 py-1 hover:bg-[var(--bg-surface-hover)] cursor-pointer text-[10px] font-bold uppercase tracking-wide text-[var(--fg-tertiary)]"
          >
            <div className="flex items-center gap-1">
              {expandUntracked ? <IconChevronDown className="size-3" /> : <IconChevronRight className="size-3" />}
              <span>Untracked ({status?.untracked?.length ?? 0})</span>
            </div>
            {status?.untracked?.length > 0 && (
              <button
                onClick={(e) => {
                  e.stopPropagation();
                  handleStageAll(status.untracked.map((s: any) => s.path));
                }}
                className="p-0.5 hover:bg-[var(--bg-surface-hover)] text-green-500 rounded"
                title="Stage all"
              >
                <IconPlus className="size-3" />
              </button>
            )}
          </div>
          {expandUntracked && (
            <div className="space-y-0.5 pl-1.5">
              {status?.untracked?.map((item: any) => (
                <div
                  key={item.path}
                  onClick={() => globalOpenFile(item.path)}
                  className="flex items-center justify-between p-1 hover:bg-[var(--bg-panel)] group rounded cursor-pointer"
                >
                  <span className={cn("truncate font-mono text-[11px]", getStatusColorClass(item.status))}>{item.path.split("/").pop()}</span>
                  <div className="flex items-center space-x-1.5">
                    <span className={cn("text-[10px] font-mono font-bold select-none", getStatusColorClass(item.status))}>{item.status}</span>
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        handleStage(item.path);
                      }}
                      className="p-0.5 hover:bg-[var(--bg-surface-hover)] text-green-500 rounded opacity-0 group-hover:opacity-100 transition-opacity"
                      title="Stage"
                    >
                      <IconPlus className="size-3" />
                    </button>
                  </div>
                </div>
              ))}
              {(!status?.untracked || status.untracked.length === 0) && (
                <div className="text-[10px] text-[var(--fg-tertiary)] italic pl-3">No untracked files</div>
              )}
            </div>
          )}
        </div>
      </div>

      {/* Action Drawer */}
      <div className="p-3 border-t border-[var(--border-default)] bg-[var(--bg-panel)] shrink-0 space-y-2">
        <textarea
          value={commitMessage}
          onChange={(e) => setCommitMessage(e.target.value)}
          placeholder="Commit message..."
          rows={2}
          className="w-full bg-[var(--bg-app)] border border-[var(--border-default)] p-2 text-xs text-[var(--fg-primary)] focus:outline-none focus:border-[var(--accent-primary)] resize-none"
        />

        <div className="flex items-center gap-1.5">
          {profiles.length > 0 && (
            <button
              onClick={handleGenerateAICommit}
              disabled={generatingAI}
              className="flex-1 py-1.5 bg-purple-600 hover:bg-purple-700 text-white text-xs font-semibold flex items-center justify-center gap-1 cursor-pointer disabled:opacity-50"
              title="Generate AI Commit Message"
            >
              <IconSparkles className="size-3.5" />
              <span>{generatingAI ? "AI..." : "AI Msg"}</span>
            </button>
          )}

          <button
            onClick={handleCommit}
            disabled={committing || !commitMessage.trim()}
            className="flex-1 py-1.5 bg-[var(--accent-primary)] hover:bg-[var(--accent-hover)] text-black text-xs font-semibold flex items-center justify-center gap-1 cursor-pointer disabled:opacity-50"
          >
            <IconCheck className="size-3.5" />
            <span>{committing ? "Saving..." : "Commit"}</span>
          </button>

          <button
            onClick={handlePush}
            disabled={pushing}
            className="p-1.5 bg-[var(--bg-surface-hover)] border border-[var(--border-default)] hover:bg-[var(--bg-surface-active)] text-[var(--fg-primary)] rounded cursor-pointer disabled:opacity-50"
            title="Push to remote"
          >
            <IconUpload className="size-3.5" />
          </button>
        </div>
      </div>
    </div>
  );
}
