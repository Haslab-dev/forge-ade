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
  IconFileDiff,
  IconAlertTriangle,
  IconX,
} from "@tabler/icons-react";
import { cn } from "../lib/utils";
import {
  GetGitStatus,
  GetGitFileDiff,
  GitStage,
  GitUnstage,
  GitDiscard,
  GitCommit,
  GitPush,
  GenerateAICommitMessage,
  GetProviderProfiles,
} from "../lib/wails";
import { globalOpenFile, globalOpenDiff, globalOpenConflict } from "../panels/editor";
import { useToast } from "../lib/toast";

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

function getConflictLabel(status: string) {
  switch (status) {
    case "UU":
      return "Both Modified";
    case "AA":
      return "Both Added";
    case "DD":
      return "Both Deleted";
    case "AU":
      return "Added by Them";
    case "UA":
      return "Added by Us";
    case "DU":
      return "Deleted by Them";
    case "UD":
      return "Deleted by Us";
    default:
      return "Conflict";
  }
}

// Render a file row's title like VS Code's source control: the filename in
// the status color, then the parent directory path in a subtle style.
// e.g. "test.js  (normal)   src/lib (subtle)"
function FileTitle({ item }: { item: any }) {
  const path: string = item?.path ?? "";
  const name = path.split("/").pop() || path || "(untitled)";
  const dir: string = item?.dir ?? "";
  return (
    <span className="flex items-baseline gap-1 min-w-0">
      <span className={cn("truncate font-mono text-[11px]", getStatusColorClass(item?.status))}>{name}</span>
      {dir && (
        <span className="truncate text-[10px] font-mono text-[var(--fg-tertiary)]">— {dir}</span>
      )}
    </span>
  );
}

export function GitPanel() {
  const { toast } = useToast();
  const [status, setStatus] = useState<any>(null);
  const [loading, setLoading] = useState(false);
  const [commitMessage, setCommitMessage] = useState("");
  const [generatingAI, setGeneratingAI] = useState(false);
  const [committing, setCommitting] = useState(false);
  const [pushing, setPushing] = useState(false);

  const [profiles, setProfiles] = useState<any[]>([]);
  const [selectedProvider, setSelectedProvider] = useState<string>("");
  const [selectedModel, setSelectedModel] = useState<string>("");
  const [commitInstruction, setCommitInstruction] = useState<string>("");

  const [expandStaged, setExpandStaged] = useState(true);
  const [expandUnstaged, setExpandUnstaged] = useState(true);
  const [expandUntracked, setExpandUntracked] = useState(true);
  const [expandConflicts, setExpandConflicts] = useState(true);

  const [discardConfirm, setDiscardConfirm] = useState<{ path: string } | null>(null);

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

  useEffect(() => {
    const handler = () => refreshStatus();
    window.addEventListener("forge:git-status-changed", handler);
    return () => window.removeEventListener("forge:git-status-changed", handler);
  }, [refreshStatus]);

  async function handleStage(path: string) {
    try {
      await GitStage("", [path]);
      refreshStatus();
    } catch (err) {
      console.error(err);
    }
  }

  async function handleOpenDiff(path: string) {
    try {
      const diff = await GetGitFileDiff("", path);
      globalOpenDiff(path, diff || "", { label: `${path.split("/").pop()} (diff)` });
    } catch (err: any) {
      toast("Failed to load diff: " + err, "danger");
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

  async function requestDiscard(path: string) {
    setDiscardConfirm({ path });
  }

  async function confirmDiscard() {
    if (!discardConfirm) return;
    const path = discardConfirm.path;
    setDiscardConfirm(null);
    try {
      await GitDiscard("", [path]);
      refreshStatus();
      toast(`Discarded changes in ${path.split("/").pop()}`, "success");
    } catch (err: any) {
      console.error(err);
      toast("Failed to discard: " + err, "danger");
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
      toast("Stage files (+) first before generating AI commit message.", "info");
      return;
    }
    setGeneratingAI(true);
    try {
      // Config (provider/model/prompt) comes from Global Settings → AI Commit.
      let provider = selectedProvider;
      let model = selectedModel;
      let instruction = commitInstruction;
      try {
        const raw = localStorage.getItem("forge-ade-ai-commit-config");
        if (raw) {
          const cfg = JSON.parse(raw);
          if (cfg.provider) provider = cfg.provider;
          if (cfg.model) model = cfg.model;
          if (cfg.prompt) instruction = cfg.prompt;
        }
      } catch { /* ignore */ }
      const msg = await GenerateAICommitMessage("", provider, model, instruction);
      if (msg) {
        setCommitMessage(msg);
        toast("AI commit message generated", "success");
      }
    } catch (err: any) {
      toast("AI Commit failed: " + err, "danger");
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
      toast("Committed", "success");
    } catch (err: any) {
      toast("Commit failed: " + err, "danger");
    } finally {
      setCommitting(false);
    }
  }

  async function handlePush() {
    setPushing(true);
    try {
      await GitPush("");
      refreshStatus();
      toast("Pushed", "success");
    } catch (err: any) {
      toast("Push failed: " + err, "danger");
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
        {/* Conflicts */}
        {status?.conflicts?.length > 0 && (
          <div className="space-y-1">
            <div
              onClick={() => setExpandConflicts(!expandConflicts)}
              className="flex items-center justify-between px-1.5 py-1 hover:bg-[var(--bg-surface-hover)] cursor-pointer text-[10px] font-bold uppercase tracking-wide text-red-400"
            >
              <div className="flex items-center gap-1">
                {expandConflicts ? <IconChevronDown className="size-3" /> : <IconChevronRight className="size-3" />}
                <IconAlertTriangle className="size-3" />
                <span>Conflicts ({status.conflicts.length})</span>
              </div>
            </div>
            {expandConflicts && (
              <div className="space-y-0.5 pl-1.5">
                {status.conflicts.map((item: any) => (
                  <div
                    key={item.path}
                    onClick={() => globalOpenConflict(item.path, item.status)}
                    className="flex items-center justify-between p-1 bg-red-500/10 border border-red-500/30 hover:bg-red-500/20 group rounded cursor-pointer"
                  >
                    <span className="truncate font-mono text-[11px] text-red-300">{item.path.split("/").pop()}</span>
                    <div className="flex items-center space-x-1.5 shrink-0">
                      <span className="text-[10px] font-mono font-bold select-none text-red-400">{getConflictLabel(item.status)}</span>
                      <button
                        onClick={(e) => {
                          e.stopPropagation();
                          globalOpenConflict(item.path, item.status);
                        }}
                        className="px-1.5 py-0.5 bg-red-500/20 hover:bg-red-500/40 text-red-300 rounded text-[9px] font-bold cursor-pointer"
                        title="Resolve conflict"
                      >
                        Resolve
                      </button>
                      <button
                        onClick={(e) => {
                          e.stopPropagation();
                          globalOpenFile(item.path);
                        }}
                        className="p-0.5 hover:bg-[var(--bg-surface-hover)] text-red-300 rounded opacity-0 group-hover:opacity-100 transition-opacity"
                        title="Open file"
                      >
                        <IconFileDiff className="size-3" />
                      </button>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>
        )}

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
                  <FileTitle item={item} />
                  <div className="flex items-center space-x-1.5">
                  <span className={cn("text-[10px] font-mono font-bold select-none", getStatusColorClass(item.status))}>{item.status}</span>
                  <div className="flex items-center space-x-1 opacity-0 group-hover:opacity-100 transition-opacity">
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        handleOpenDiff(item.path);
                      }}
                      className="p-0.5 hover:bg-[var(--bg-surface-hover)] text-blue-500 rounded"
                      title="Open diff"
                    >
                      <IconFileDiff className="size-3" />
                    </button>
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        handleUnstage(item.path);
                      }}
                      className="p-0.5 hover:bg-[var(--bg-surface-hover)] text-amber-500 rounded"
                      title="Unstage"
                    >
                      <IconMinus className="size-3" />
                    </button>
                  </div>
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
                  <FileTitle item={item} />
                  <div className="flex items-center space-x-1.5">
                    <span className={cn("text-[10px] font-mono font-bold select-none", getStatusColorClass(item.status))}>{item.status}</span>
                    <div className="flex items-center space-x-1 opacity-0 group-hover:opacity-100 transition-opacity">
                      <button
                        onClick={(e) => {
                          e.stopPropagation();
                          handleOpenDiff(item.path);
                        }}
                        className="p-0.5 hover:bg-[var(--bg-surface-hover)] text-blue-500 rounded"
                        title="Open diff"
                      >
                        <IconFileDiff className="size-3" />
                      </button>
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
                          requestDiscard(item.path);
                        }}
                        className="p-0.5 hover:bg-[var(--bg-surface-hover)] text-red-500 rounded"
                        title="Discard changes"
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
                  <FileTitle item={item} />
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
                    <button
                      onClick={(e) => {
                        e.stopPropagation();
                        requestDiscard(item.path);
                      }}
                      className="p-0.5 hover:bg-[var(--bg-surface-hover)] text-red-500 rounded opacity-0 group-hover:opacity-100 transition-opacity"
                      title="Discard changes"
                    >
                      <IconTrash className="size-3" />
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

        {profiles.length > 0 && (
          <div className="text-[10px] text-[var(--fg-tertiary)] italic">
            AI commit pakai config dari Global Settings → AI Commit (provider, model, prompt).
          </div>
        )}

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

      {/* Discard confirmation modal (window.confirm is unreliable in the webview) */}
      {discardConfirm && (
        <div className="fixed inset-0 bg-black/60 backdrop-blur-sm z-50 flex items-center justify-center p-4">
          <div className="bg-[var(--bg-sidebar)] border border-[var(--border-default)] w-full max-w-sm p-4 space-y-4 shadow-2xl">
            <div className="flex items-center justify-between pb-2 border-b border-[var(--border-default)]">
              <span className="font-bold text-xs text-[var(--fg-primary)] uppercase tracking-wider text-red-400">
                Discard Changes
              </span>
              <button
                onClick={() => setDiscardConfirm(null)}
                className="text-[var(--fg-tertiary)] hover:text-white cursor-pointer"
              >
                <IconX className="size-4" />
              </button>
            </div>

            <div className="text-xs text-[var(--fg-secondary)] break-all">
              Discard all changes to <span className="font-mono text-[var(--fg-primary)]">{discardConfirm.path}</span>? This cannot be undone.
            </div>

            <div className="flex items-center justify-end space-x-2 pt-2 border-t border-[var(--border-default)]">
              <button
                onClick={() => setDiscardConfirm(null)}
                className="px-3 py-1.5 text-xs text-[var(--fg-secondary)] hover:text-white cursor-pointer"
              >
                Cancel
              </button>
              <button
                onClick={confirmDiscard}
                className="px-4 py-1.5 bg-red-600 hover:bg-red-700 text-white text-xs font-semibold cursor-pointer"
              >
                Discard
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
