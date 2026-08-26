import React, { useState, useEffect, useCallback, useRef } from "react";
import {
  IconGitBranch,
  IconCheck,
  IconSparkles,
  IconRefresh,
  IconChevronDown,
  IconFileDiff,
  IconAlertTriangle,
  IconX,
  IconTrash,
  IconArrowBackUp,
  IconMaximize,
  IconMinimize,
  IconDots,
  IconGitCommit,
  IconCloudDownload,
  IconCloudUpload,
  IconSquare,
  IconSquareCheck,
  IconSquarePlus,
  IconSquareMinus,
  IconSquareDot,
  IconFile,
  IconFileText,
  IconPlusMinus,
  IconAdjustmentsHorizontal,
  IconUser,
} from "@tabler/icons-react";
import { cn } from "../lib/utils";
import {
  GetGitStatus,
  GetGitCommitGraph,
  GetGitCommitDiff,
  GetGitFileDiff,
  GitStage,
  GitUnstage,
  GitDiscard,
  GitCommit,
  GitPush,
  GitFetch,
  GenerateAICommitMessage,
  GetProviderProfiles,
} from "../lib/native";
import { globalOpenFile, globalOpenDiff, globalOpenConflict } from "../panels/editor";
import { useToast } from "../lib/toast";

function formatRelativeTime(timestamp: string | number): string {
  const ts = typeof timestamp === "string" ? new Date(timestamp).getTime() : timestamp;
  if (!ts || isNaN(ts)) return "";
  const now = Date.now();
  const diffSec = Math.floor((now - ts) / 1000);
  if (diffSec < 60) return "just now";
  const diffMin = Math.floor(diffSec / 60);
  if (diffMin < 60) return `${diffMin}m ago`;
  const diffHours = Math.floor(diffMin / 60);
  if (diffHours < 24) return `${diffHours}h ago`;
  const diffDays = Math.floor(diffHours / 24);
  if (diffDays === 1) return "yesterday";
  if (diffDays < 7) return `${diffDays} days ago`;
  const diffWeeks = Math.floor(diffDays / 7);
  if (diffWeeks < 5) return `${diffWeeks} week${diffWeeks > 1 ? "s" : ""} ago`;
  const diffMonths = Math.floor(diffDays / 30);
  if (diffMonths < 12) return `${diffMonths} month${diffMonths > 1 ? "s" : ""} ago`;
  const diffYears = Math.floor(diffDays / 365);
  return `${diffYears} year${diffYears > 1 ? "s" : ""} ago`;
}

// Status icon rendered on the left of each file row
function FileStatusIcon({ status }: { status: string }) {
  switch (status) {
    case "M":
      return (
        <span className="text-amber-500 font-bold shrink-0" title="Modified">
          <IconSquareDot className="size-3.5" />
        </span>
      );
    case "A":
    case "U":
    case "?":
      return (
        <span className="text-emerald-400 font-bold shrink-0" title="Added / Untracked">
          <IconSquarePlus className="size-3.5" />
        </span>
      );
    case "D":
      return (
        <span className="text-rose-500 font-bold shrink-0" title="Deleted">
          <IconSquareMinus className="size-3.5" />
        </span>
      );
    case "R":
      return (
        <span className="text-sky-400 font-bold shrink-0" title="Renamed">
          <IconSquareDot className="size-3.5" />
        </span>
      );
    default:
      return (
        <span className="text-amber-500 font-bold shrink-0">
          <IconSquareDot className="size-3.5" />
        </span>
      );
  }
}

export function GitPanel() {
  const { toast } = useToast();
  const [activeTab, setActiveTab] = useState<"changes" | "history">("changes");
  const [status, setStatus] = useState<any>(null);
  const [discardConfirmOpen, setDiscardConfirmOpen] = useState(false);
  const [loading, setLoading] = useState(false);
  const [commitMessage, setCommitMessage] = useState("");
  const [generatingAI, setGeneratingAI] = useState(false);
  const [committing, setCommitting] = useState(false);
  const [fetching, setFetching] = useState(false);
  const [pushing, setPushing] = useState(false);
  // History tab commits
  const [commits, setCommits] = useState<any[]>([]);
  const [loadingHistory, setLoadingHistory] = useState(false);

  // Selected file for highlight and context menu
  const [selectedFilePath, setSelectedFilePath] = useState<string | null>(null);
  const [modalFile, setModalFile] = useState<any | null>(null);

  // Split button dropdowns
  const [stageMenuOpen, setStageMenuOpen] = useState(false);
  const [commitMenuOpen, setCommitMenuOpen] = useState(false);
  const [commitMode, setCommitMode] = useState<"staged" | "tracked" | "amend">("staged");
  const [isCommitExpanded, setIsCommitExpanded] = useState(false);

  // LLM profiles for commit message generation
  const [profiles, setProfiles] = useState<any[]>([]);

  const refreshStatus = useCallback(async () => {
    setLoading(true);
    try {
      const res = await GetGitStatus("");
      setStatus(res || null);
    } catch (err) {
      console.error("Failed to load git status:", err);
    } finally {
      setLoading(false);
    }
  }, []);

  const refreshHistory = useCallback(async () => {
    setLoadingHistory(true);
    try {
      const res = await GetGitCommitGraph("", 0, 50, "");
      setCommits(res?.commits || []);
    } catch (err) {
      console.error("Failed to load git history:", err);
    } finally {
      setLoadingHistory(false);
    }
  }, []);

  const loadProfiles = useCallback(async () => {
    try {
      const list = await GetProviderProfiles();
      setProfiles((Array.isArray(list) ? list : []).filter((p) => p.enabled));
    } catch {}
  }, []);

  useEffect(() => {
    refreshStatus();
    loadProfiles();
  }, [refreshStatus, loadProfiles]);

  useEffect(() => {
    if (activeTab === "history") {
      refreshHistory();
    }
  }, [activeTab, refreshHistory]);

  useEffect(() => {
    let timer: ReturnType<typeof setTimeout> | null = null;
    const handler = () => {
      if (timer) clearTimeout(timer);
      timer = setTimeout(() => {
        timer = null;
        refreshStatus();
        if (activeTab === "history") refreshHistory();
      }, 250);
    };
    window.addEventListener("forge:git-status-changed", handler);
    return () => {
      if (timer) clearTimeout(timer);
      window.removeEventListener("forge:git-status-changed", handler);
    };
  }, [refreshStatus, refreshHistory, activeTab]);

  // Stage toggle for a single file
  async function handleToggleStage(item: any, isCurrentlyStaged: boolean) {
    try {
      if (isCurrentlyStaged) {
        await GitUnstage("", [item.path]);
      } else {
        await GitStage("", [item.path]);
      }
      refreshStatus();
    } catch (err) {
      console.error("Stage toggle error:", err);
    }
  }

  // Open file diff
  async function handleOpenDiff(path: string) {
    try {
      setSelectedFilePath(path);
      const diff = await GetGitFileDiff("", path);
      const fileName = path.split(/[/\\]/).pop() || path;
      globalOpenDiff(path, diff || "", { label: `${fileName} (diff)` });
    } catch (err: any) {
      toast("Failed to load diff: " + err, "danger");
    }
  }

  // Open entire workspace diff (± View Diff)
  async function handleOpenFullDiff() {
    try {
      const diff = await GetGitFileDiff("", "");
      globalOpenDiff("All Changes", diff || "", { label: "Working Tree Diff" });
    } catch (err: any) {
      toast("Failed to load diff: " + err, "danger");
    }
  }

  // Open commit diff from history
  async function handleOpenCommitDiff(commit: any) {
    try {
      const diff = await GetGitCommitDiff("", commit.hash || commit.short_hash);
      const shortHash = commit.short_hash || commit.hash?.slice(0, 7) || "";
      globalOpenDiff(commit.hash, diff || "", { label: `Commit ${shortHash}` });
    } catch (err: any) {
      toast("Failed to load commit diff: " + err, "danger");
    }
  }

  // Stage all changes
  async function handleStageAll() {
    setStageMenuOpen(false);
    const allUnstaged = [
      ...(status?.unstaged || []),
      ...(status?.untracked || []),
    ].map((f) => f.path);
    if (allUnstaged.length === 0) return;
    try {
      await GitStage("", allUnstaged);
      refreshStatus();
      toast(`Staged ${allUnstaged.length} files`, "success");
    } catch (err) {
      console.error(err);
    }
  }

  // Unstage all changes
  async function handleUnstageAll() {
    setStageMenuOpen(false);
    const allStaged = (status?.staged || []).map((f: any) => f.path);
    if (allStaged.length === 0) return;
    try {
      await GitUnstage("", allStaged);
      refreshStatus();
      toast(`Unstaged ${allStaged.length} files`, "success");
    } catch (err) {
      console.error(err);
    }
  }

  // Discard all changes
  async function handleDiscardAll() {
    setStageMenuOpen(false);
    const all = [
      ...(status?.unstaged || []),
      ...(status?.untracked || []),
    ].map((f: any) => f.path);
    if (all.length === 0) return;
    setDiscardConfirmOpen(true);
  }

  async function executeDiscardAll() {
    setDiscardConfirmOpen(false);
    const all = [
      ...(status?.unstaged || []),
      ...(status?.untracked || []),
    ].map((f: any) => f.path);
    if (all.length === 0) return;
    try {
      await GitDiscard("", all);
      refreshStatus();
      toast("Discarded all changes", "success");
    } catch (err: any) {
      toast("Failed to discard: " + err, "danger");
    }
  }
  // Generate AI commit message
  async function handleGenerateAICommit() {
    setGeneratingAI(true);
    try {
      let providerId: string | undefined = undefined;
      let model: string | undefined = undefined;
      let customPrompt: string | undefined = undefined;
      try {
        const raw = localStorage.getItem("forge-ade-ai-commit-config");
        if (raw) {
          const cfg = JSON.parse(raw);
          if (cfg.provider) providerId = cfg.provider;
          if (cfg.model) model = cfg.model;
          if (cfg.prompt && cfg.prompt.trim().length > 10 && !/^(hai|hello|hi|hey)$/i.test(cfg.prompt.trim())) {
            customPrompt = cfg.prompt.trim();
          }
        }
      } catch {}

      if (providerId) {
        const matchingProfile = profiles.find((p: any) => p.id === providerId || p.provider === providerId);
        if (!matchingProfile || matchingProfile.enabled === false) {
          providerId = undefined;
        }
      }

      const res = await GenerateAICommitMessage(
        "",
        providerId,
        model,
        customPrompt,
      );
      if (res && res.trim()) {
        setCommitMessage(res.trim());
        toast("Generated commit message", "success");
      } else {
        toast("AI returned an empty message. Try again or check the LLM provider is reachable.", "warn");
      }
    } catch (err: any) {
      console.error("[git-panel] AI commit generation failed:", err);
      toast("AI commit failed: " + (err?.message || String(err)), "danger");
    } finally {
      setGeneratingAI(false);
    }
  }

  // Commit changes
  async function handleCommit(pushAfter: boolean = false) {
    if (!commitMessage.trim() && commitMode !== "amend") {
      toast("Please enter a commit message", "warn");
      return;
    }

    const stagedList = status?.staged || [];
    const unstagedList = status?.unstaged || [];

    if (commitMode === "staged") {
      if (stagedList.length === 0) {
        toast("No staged changes to commit. Click '+' to stage files, or choose 'Commit All'.", "warn");
        return;
      }
      // Strictly commit only staged changes. Never stage unstaged files.
    } else if (commitMode === "tracked") {
      if (stagedList.length === 0 && unstagedList.length === 0) {
        toast("No changes to commit.", "warn");
        return;
      }
      // Stage unstaged files when user explicitly chose 'Commit All (Tracked)'
      const unstagedPaths = unstagedList.map((f: any) => f.path);
      if (unstagedPaths.length > 0) {
        await GitStage("", unstagedPaths);
      }
    }

    setCommitting(true);
    try {
      await GitCommit("", commitMessage.trim(), commitMode === "amend");
      setCommitMessage("");
      toast(
        commitMode === "amend"
          ? "Amended commit successfully"
          : commitMode === "tracked"
          ? "Committed all tracked changes"
          : "Committed staged changes",
        "success"
      );

      if (pushAfter) {
        await GitPush("");
        toast("Pushed to remote", "success");
      }

      await refreshStatus();
      if (activeTab === "history") await refreshHistory();
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      toast("Commit failed: " + msg, "danger");
    } finally {
      setCommitting(false);
    }
  }

  // Fetch from remote
  async function handleFetch() {
    setFetching(true);
    try {
      await GitFetch("");
      toast("Fetched latest remote changes", "success");
      refreshStatus();
      if (activeTab === "history") refreshHistory();
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      toast("Fetch failed: " + msg, "danger");
    } finally {
      setFetching(false);
    }
  }

  // Push to remote
  async function handlePush() {
    setPushing(true);
    try {
      await GitPush("");
      toast("Pushed commits to remote", "success");
      refreshStatus();
      if (activeTab === "history") refreshHistory();
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      toast("Push failed: " + msg, "danger");
    } finally {
      setPushing(false);
    }
  }
  const stagedFiles: any[] = status?.staged || [];
  const unstagedFiles: any[] = status?.unstaged || [];
  const untrackedFiles: any[] = status?.untracked || [];
  const allUnstaged = [...unstagedFiles, ...untrackedFiles];
  const totalChanges = stagedFiles.length + allUnstaged.length;
  const totalAdditions = status?.totalAdditions || 0;
  const totalDeletions = status?.totalDeletions || 0;

  return (
    <div className="flex flex-col h-full bg-[#18181b] text-[var(--fg-primary)] select-none font-sans text-xs">
      {/* ------------------------------------------------------------------- */}
      {/* Top Segmented Header Tabs (Changes vs History)                      */}
      {/* ------------------------------------------------------------------- */}
      <div className="flex border-b border-[#27272a] bg-[#141416] p-1 gap-1 shrink-0">
        <button
          onClick={() => setActiveTab("changes")}
          className={cn(
            "flex-1 py-1.5 px-3 rounded text-center text-xs font-semibold transition-all cursor-pointer",
            activeTab === "changes"
              ? "bg-[#27272a] text-white shadow-xs"
              : "text-[#71717a] hover:text-[#a1a1aa]"
          )}
        >
          Changes {totalChanges > 0 ? `(${totalChanges})` : ""}
        </button>
        <button
          onClick={() => setActiveTab("history")}
          className={cn(
            "flex-1 py-1.5 px-3 rounded text-center text-xs font-semibold transition-all cursor-pointer",
            activeTab === "history"
              ? "bg-[#27272a] text-white shadow-xs"
              : "text-[#71717a] hover:text-[#a1a1aa]"
          )}
        >
          History
        </button>
      </div>

      {/* ------------------------------------------------------------------- */}
      {/* TAB 1: CHANGES VIEW (Image #1)                                      */}
      {/* ------------------------------------------------------------------- */}
      {activeTab === "changes" && (
        <div className="flex flex-col flex-1 min-h-0">
          {/* Subheader bar: ± View Diff +111 -3,173 | [Stage All ⌄] */}
          <div className="flex items-center justify-between px-3 py-2 border-b border-[#27272a] bg-[#18181b] shrink-0">
            <button
              onClick={handleOpenFullDiff}
              className="flex items-center gap-2 hover:opacity-80 transition-opacity cursor-pointer text-left"
              title="View all repository changes"
            >
              <div className="flex items-center gap-1 text-[#a1a1aa] font-medium text-xs">
                <IconPlusMinus className="size-3.5 text-[#a1a1aa]" />
                <span>View Diff</span>
              </div>
              <div className="flex items-center gap-1 font-mono text-xs font-semibold">
                {totalAdditions > 0 && <span className="text-[#4ade80]">+{totalAdditions.toLocaleString()}</span>}
                {totalDeletions > 0 && <span className="text-[#fb7185]">-{totalDeletions.toLocaleString()}</span>}
              </div>
            </button>

            {/* Stage All split button */}
            <div className="flex items-center gap-1.5">
              {/* Fetch button at top, left of Stage All dropdown */}
              <button
                onClick={handleFetch}
                disabled={fetching}
                className="flex items-center gap-1 px-2.5 py-1 text-xs font-medium text-[#e4e4e7] bg-[#27272a] hover:bg-[#3f3f46] border border-[#3f3f46] rounded cursor-pointer disabled:opacity-50 transition-colors"
                title="Fetch latest remote changes"
              >
                <IconRefresh className={cn("size-3 text-[#a1a1aa]", fetching && "animate-spin")} />
                <span>Fetch</span>
              </button>

              {/* Stage All split button */}
              <div className="relative">
                <div className="flex items-center border border-[#3f3f46] bg-[#27272a] rounded overflow-hidden">
                  <button
                    onClick={handleStageAll}
                    className="flex items-center gap-1.5 px-2.5 py-1 text-xs font-medium text-[#e4e4e7] hover:bg-[#3f3f46] cursor-pointer"
                  >
                    <IconAdjustmentsHorizontal className="size-3 text-[#a1a1aa]" />
                    <span>Stage All</span>
                  </button>
                  <button
                    onClick={() => setStageMenuOpen(!stageMenuOpen)}
                    className="px-1.5 py-1 border-l border-[#3f3f46] hover:bg-[#3f3f46] text-[#a1a1aa] hover:text-white cursor-pointer"
                  >
                    <IconChevronDown className="size-3" />
                  </button>
                </div>

                {stageMenuOpen && (
                  <div className="absolute right-0 top-full mt-1 w-44 bg-[#1f1f23] border border-[#3f3f46] rounded shadow-xl py-1 z-30 font-medium">
                    <button
                      onClick={handleStageAll}
                      className="w-full text-left px-3 py-1.5 hover:bg-[#27272a] text-[#e4e4e7] cursor-pointer"
                    >
                      Stage All
                    </button>
                    <button
                      onClick={handleUnstageAll}
                      className="w-full text-left px-3 py-1.5 hover:bg-[#27272a] text-[#e4e4e7] cursor-pointer"
                    >
                      Unstage All
                    </button>
                    <button
                      onClick={handleDiscardAll}
                      className="w-full text-left px-3 py-1.5 hover:bg-[#27272a] text-[#fb7185] cursor-pointer"
                    >
                      Discard All Changes
                    </button>
                  </div>
                )}
              </div>
            </div>
          </div>

          {/* File Lists Scroll Area */}
          <div className="flex-1 overflow-y-auto px-2 py-2 space-y-4 font-sans">
            {/* SECTION 1: STAGED CHANGES */}
            <div>
              <div className="flex items-center justify-between px-1 mb-1 text-[11px] font-semibold text-[#71717a]">
                <span>Staged</span>
                {stagedFiles.length > 0 && (
                  <button
                    onClick={handleUnstageAll}
                    className="hover:text-white cursor-pointer"
                    title="Unstage all staged files"
                  >
                    <IconSquareCheck className="size-3.5 text-[#38bdf8]" />
                  </button>
                )}
              </div>

              {stagedFiles.length === 0 ? (
                <div className="text-[#52525b] text-[11px] px-2 py-1 italic">
                  No staged changes yet
                </div>
              ) : (
                <div className="space-y-0.5">
                  {stagedFiles.map((file) => {
                    const isSelected = selectedFilePath === file.path;
                    const fileName = file.path.split(/[/\\]/).pop() || file.path;
                    const dir = file.dir || (file.path.includes("/") ? file.path.slice(0, file.path.lastIndexOf("/")) : "");

                    return (
                      <div
                        key={file.path}
                        onClick={() => handleOpenDiff(file.path)}
                        onContextMenu={(e) => {
                          e.preventDefault();
                          setModalFile({ ...file, isStaged: true });
                        }}
                        className={cn(
                          "group flex items-center justify-between px-2 py-1.5 rounded transition-all cursor-pointer select-none",
                          isSelected
                            ? "border border-[#38bdf8] bg-[#38bdf8]/10"
                            : "hover:bg-[#27272a] border border-transparent"
                        )}
                      >
                        <div className="flex items-center gap-2 min-w-0 flex-1">
                          <FileStatusIcon status={file.status || "A"} />
                          <span className="font-semibold text-xs text-[#f4f4f5] truncate">{fileName}</span>
                          {dir && <span className="text-[11px] text-[#71717a] truncate">{dir}</span>}
                        </div>

                        <div className="flex items-center gap-2 shrink-0 ml-2">
                          {(file.additions > 0 || file.deletions > 0) && (
                            <div className="flex items-center gap-1 font-mono text-[11px]">
                              {file.additions > 0 && <span className="text-[#4ade80]">+{file.additions}</span>}
                              {file.deletions > 0 && <span className="text-[#fb7185]">-{file.deletions}</span>}
                            </div>
                          )}
                          <button
                            onClick={(e) => {
                              e.stopPropagation();
                              handleToggleStage(file, true);
                            }}
                            className="text-[#38bdf8] hover:opacity-80 cursor-pointer"
                            title="Unstage file"
                          >
                            <IconSquareCheck className="size-4" />
                          </button>
                        </div>
                      </div>
                    );
                  })}
                </div>
              )}
            </div>

            {/* SECTION 2: UNSTAGED CHANGES */}
            <div>
              <div className="flex items-center justify-between px-1 mb-1 text-[11px] font-semibold text-[#71717a]">
                <span>Unstaged</span>
                {allUnstaged.length > 0 && (
                  <button
                    onClick={handleStageAll}
                    className="hover:text-white cursor-pointer"
                    title="Stage all unstaged files"
                  >
                    <IconSquare className="size-3.5 text-[#71717a]" />
                  </button>
                )}
              </div>

              {allUnstaged.length === 0 && stagedFiles.length === 0 ? (
                <div className="text-[#52525b] text-[11px] px-2 py-3 text-center italic">
                  Working tree clean
                </div>
              ) : (
                <div className="space-y-0.5">
                  {allUnstaged.map((file) => {
                    const isSelected = selectedFilePath === file.path;
                    const fileName = file.path.split(/[/\\]/).pop() || file.path;
                    const dir = file.dir || (file.path.includes("/") ? file.path.slice(0, file.path.lastIndexOf("/")) : "");

                    return (
                      <div
                        key={file.path}
                        onClick={() => handleOpenDiff(file.path)}
                        onContextMenu={(e) => {
                          e.preventDefault();
                          setModalFile({ ...file, isStaged: false });
                        }}
                        className={cn(
                          "group flex items-center justify-between px-2 py-1.5 rounded transition-all cursor-pointer select-none",
                          isSelected
                            ? "border border-[#38bdf8] bg-[#38bdf8]/10"
                            : "hover:bg-[#27272a] border border-transparent"
                        )}
                      >
                        <div className="flex items-center gap-2 min-w-0 flex-1">
                          <FileStatusIcon status={file.status || "M"} />
                          <span className="font-semibold text-xs text-[#f4f4f5] truncate">{fileName}</span>
                          {dir && <span className="text-[11px] text-[#71717a] truncate">{dir}</span>}
                        </div>

                        <div className="flex items-center gap-2 shrink-0 ml-2">
                          {(file.additions > 0 || file.deletions > 0) && (
                            <div className="flex items-center gap-1 font-mono text-[11px]">
                              {file.additions > 0 && <span className="text-[#4ade80]">+{file.additions}</span>}
                              {file.deletions > 0 && <span className="text-[#fb7185]">-{file.deletions}</span>}
                            </div>
                          )}
                          <button
                            onClick={(e) => {
                              e.stopPropagation();
                              handleToggleStage(file, false);
                            }}
                            className="text-[#52525b] hover:text-[#e4e4e7] cursor-pointer"
                            title="Stage file"
                          >
                            <IconSquare className="size-4" />
                          </button>
                        </div>
                      </div>
                    );
                  })}
                </div>
              )}
            </div>
          </div>

          {/* --------------------------------------------------------------- */}
          {/* Bottom Bar: Branch ⑂ | [ ⤒ Push (N) ]                           */}
          {/* --------------------------------------------------------------- */}
          <div className="flex items-center justify-between px-3 py-1.5 border-t border-[#27272a] bg-[#18181b] text-xs shrink-0">
            <div className="flex items-center gap-1.5 text-[#a1a1aa] font-mono font-medium truncate">
              <IconGitBranch className="size-3.5 text-[#a1a1aa]" />
              <span className="truncate">{status?.branch || "main"}</span>
              {status?.behind ? (
                <span className="text-[10px] text-amber-400 font-mono ml-1">
                  ({status.behind} behind)
                </span>
              ) : null}
            </div>

            <div>
              <button
                onClick={handlePush}
                disabled={pushing || (status?.ahead ?? 0) === 0}
                className={cn(
                  "flex items-center gap-1.5 px-3 py-1 text-xs font-semibold rounded cursor-pointer transition-colors",
                  (status?.ahead ?? 0) > 0
                    ? "bg-[var(--accent-primary)] hover:bg-[var(--accent-hover)] text-black shadow-sm"
                    : "bg-[#27272a] text-[#71717a] border border-[#3f3f46] cursor-not-allowed opacity-60"
                )}
                title={
                  (status?.ahead ?? 0) > 0
                    ? `Push ${status?.ahead} unpushed commit${(status?.ahead ?? 0) > 1 ? "s" : ""} to remote`
                    : "No unpushed local commits"
                }
              >
                <IconCloudUpload className={cn("size-3.5", pushing && "animate-bounce")} />
                <span>{pushing ? "Pushing..." : "Push"}</span>
                {(status?.ahead ?? 0) > 0 && (
                  <span className="ml-0.5 px-1.5 py-0.2 bg-black/25 rounded-full text-[10px] font-mono font-bold">
                    {status?.ahead}
                  </span>
                )}
              </button>
            </div>
          </div>

          {/* --------------------------------------------------------------- */}
          {/* Commit Message Box                                              */}
          {/* --------------------------------------------------------------- */}
          <div className="p-3 border-t border-[#27272a] bg-[#141416] space-y-2 shrink-0">
            <div className="relative">
              <textarea
                value={commitMessage}
                onChange={(e) => setCommitMessage(e.target.value)}
                placeholder="Enter commit message"
                rows={isCommitExpanded ? 5 : 2}
                className="w-full bg-[#1c1c20] border border-[#27272a] focus:border-[#38bdf8] rounded-lg p-2.5 pr-8 text-xs text-[#f4f4f5] placeholder-[#52525b] focus:outline-none resize-none font-sans leading-relaxed"
                onKeyDown={(e) => {
                  if ((e.metaKey || e.ctrlKey) && e.key === "Enter") {
                    handleCommit();
                  }
                }}
              />
              <button
                onClick={() => setIsCommitExpanded(!isCommitExpanded)}
                className="absolute top-2 right-2 text-[#71717a] hover:text-white cursor-pointer p-0.5"
                title={isCommitExpanded ? "Collapse" : "Expand"}
              >
                {isCommitExpanded ? <IconMinimize className="size-3.5" /> : <IconMaximize className="size-3.5" />}
              </button>
            </div>

            <div className="flex items-center justify-between">
              <button
                onClick={handleGenerateAICommit}
                disabled={generatingAI}
                className="flex items-center gap-1 px-2 py-1 text-xs text-[#a1a1aa] hover:text-white hover:bg-[#27272a] border border-[#27272a] rounded cursor-pointer transition-colors disabled:opacity-50"
                title="Generate AI commit message"
              >
                <IconSparkles className={cn("size-3.5 text-purple-400", generatingAI && "animate-spin")} />
              </button>

              {/* Split Commit Button */}
              <div className="relative">
                <div className="flex items-center border border-[#3f3f46] bg-[#27272a] rounded-md overflow-hidden">
                  <button
                    onClick={() => handleCommit(false)}
                    disabled={committing || !commitMessage.trim()}
                    className="px-3 py-1 text-xs font-semibold text-white hover:bg-[#3f3f46] cursor-pointer disabled:opacity-40 disabled:cursor-not-allowed"
                  >
                    {commitMode === "tracked" ? "Commit Tracked" : commitMode === "staged" ? "Commit Staged" : "Commit (Amend)"}
                  </button>
                  <button
                    onClick={() => setCommitMenuOpen(!commitMenuOpen)}
                    className="px-1.5 py-1 border-l border-[#3f3f46] hover:bg-[#3f3f46] text-[#a1a1aa] hover:text-white cursor-pointer"
                  >
                    <IconChevronDown className="size-3" />
                  </button>
                </div>

                {commitMenuOpen && (
                  <div className="absolute right-0 bottom-full mb-1 w-48 bg-[#1f1f23] border border-[#3f3f46] rounded shadow-xl py-1 z-30 font-medium">
                    <button
                      onClick={() => {
                        setCommitMode("staged");
                        setCommitMenuOpen(false);
                      }}
                      className={cn(
                        "w-full text-left px-3 py-1.5 hover:bg-[#27272a] text-[#e4e4e7] cursor-pointer text-xs flex items-center justify-between",
                        commitMode === "staged" && "text-cyan-300 font-semibold"
                      )}
                    >
                      <span>Commit Staged</span>
                      {commitMode === "staged" && <span className="text-[10px]">✓</span>}
                    </button>
                    <button
                      onClick={() => {
                        setCommitMode("tracked");
                        setCommitMenuOpen(false);
                      }}
                      className={cn(
                        "w-full text-left px-3 py-1.5 hover:bg-[#27272a] text-[#e4e4e7] cursor-pointer text-xs flex items-center justify-between",
                        commitMode === "tracked" && "text-cyan-300 font-semibold"
                      )}
                    >
                      <span>Commit All (Tracked)</span>
                      {commitMode === "tracked" && <span className="text-[10px]">✓</span>}
                    </button>
                    <button
                      onClick={() => {
                        setCommitMode("amend");
                        setCommitMenuOpen(false);
                      }}
                      className={cn(
                        "w-full text-left px-3 py-1.5 hover:bg-[#27272a] text-[#e4e4e7] cursor-pointer text-xs flex items-center justify-between",
                        commitMode === "amend" && "text-cyan-300 font-semibold"
                      )}
                    >
                      <span>Commit (Amend)</span>
                      {commitMode === "amend" && <span className="text-[10px]">✓</span>}
                    </button>
                    <div className="border-t border-[#3f3f46] my-1" />
                    <button
                      onClick={() => {
                        setCommitMenuOpen(false);
                        handleCommit(true);
                      }}
                      className="w-full text-left px-3 py-1.5 hover:bg-[#27272a] text-[#e4e4e7] cursor-pointer text-xs"
                    >
                      Commit & Push
                    </button>
                  </div>
                )}
              </div>
            </div>
          </div>

          {/* --------------------------------------------------------------- */}
          {/* Latest Commit Footer Line (Image #1 bottom)                     */}
          {/* --------------------------------------------------------------- */}
          <div className="flex items-center justify-between px-3 py-2 border-t border-[#27272a] bg-[#141416] text-[11px] text-[#a1a1aa] font-medium shrink-0">
            <span className="truncate pr-2">{status?.latestCommit || "No commits yet"}</span>
            <div className="flex items-center gap-1 shrink-0">
              <button
                onClick={() => {
                  GitUnstage("", []).then(() => {
                    toast("Soft undo performed", "info");
                    refreshStatus();
                  });
                }}
                className="p-1 hover:bg-[#27272a] rounded text-[#71717a] hover:text-white cursor-pointer"
                title="Undo commit"
              >
                <IconArrowBackUp className="size-3.5" />
              </button>
              <button className="p-1 hover:bg-[#27272a] rounded text-[#71717a] hover:text-white cursor-pointer">
                <IconDots className="size-3.5" />
              </button>
            </div>
          </div>
        </div>
      )}

      {/* ------------------------------------------------------------------- */}
      {/* TAB 2: HISTORY VIEW (Image #2)                                      */}
      {/* ------------------------------------------------------------------- */}
      {activeTab === "history" && (
        <div className="flex flex-col flex-1 min-h-0 bg-[#18181b]">
          <div className="flex-1 overflow-y-auto divide-y divide-[#27272a] p-1">
            {loadingHistory && commits.length === 0 ? (
              <div className="p-4 text-center text-[#71717a] text-xs">Loading commit history…</div>
            ) : commits.length === 0 ? (
              <div className="p-4 text-center text-[#71717a] text-xs">No commit history found</div>
            ) : (
              commits.map((commit) => {
                const shortHash = commit.short_hash || commit.hash?.slice(0, 7) || "";
                const authorName = commit.author_name || commit.author || "User";
                const relTime = formatRelativeTime(commit.timestamp || commit.date);

                return (
                  <div
                    key={commit.hash || shortHash}
                    onClick={() => handleOpenCommitDiff(commit)}
                    className="group px-3 py-2.5 hover:bg-[#27272a] transition-colors cursor-pointer select-none"
                    title={`View Commit Diff ${shortHash}`}
                  >
                    {/* Top line: commit subject */}
                    <div className="text-xs font-semibold text-[#f4f4f5] leading-snug truncate group-hover:text-white">
                      {commit.message}
                    </div>

                    {/* Bottom line: author avatar • author • time • hash */}
                    <div className="flex items-center gap-1.5 text-[11px] text-[#71717a] mt-1 font-mono">
                      <div className="size-3.5 rounded-full bg-[#3f3f46] flex items-center justify-center text-[8px] text-white font-bold shrink-0">
                        {authorName.slice(0, 1).toUpperCase()}
                      </div>
                      <span className="text-[#a1a1aa] font-sans truncate max-w-28">{authorName}</span>
                      <span>•</span>
                      <span className="font-sans shrink-0">{relTime}</span>
                      <span>•</span>
                      <span className="text-[#38bdf8] font-mono shrink-0">{shortHash}</span>
                    </div>
                  </div>
                );
              })
            )}
          </div>

          <div className="flex items-center justify-between px-3 py-2 border-t border-[#27272a] bg-[#141416] text-[11px] text-[#71717a] shrink-0">
            <span>{commits.length} commits</span>
            <button
              onClick={refreshHistory}
              className="flex items-center gap-1 hover:text-white cursor-pointer"
            >
              <IconRefresh className={cn("size-3", loadingHistory && "animate-spin")} />
              <span>Refresh</span>
            </button>
          </div>
        </div>
      )}

      {/* ------------------------------------------------------------------- */}
      {/* File Action Modal / Select (Right-Click Context Menu)               */}
      {/* ------------------------------------------------------------------- */}
      {modalFile && (
        <div
          className="fixed inset-0 bg-black/60 backdrop-blur-xs flex items-center justify-center z-50 p-4"
          onClick={() => setModalFile(null)}
        >
          <div
            className="bg-[#1f1f23] border border-[#3f3f46] rounded-xl shadow-2xl w-72 overflow-hidden py-1 text-xs font-medium animate-in fade-in zoom-in-95 duration-100"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="px-3 py-2 border-b border-[#27272a] text-[#71717a] font-mono text-[11px] truncate">
              {modalFile.path}
            </div>

            <div className="py-1">
              <button
                onClick={() => {
                  handleToggleStage(modalFile, modalFile.isStaged);
                  setModalFile(null);
                }}
                className="w-full text-left px-3 py-2 hover:bg-[#27272a] text-[#f4f4f5] flex items-center justify-between cursor-pointer"
              >
                <span>{modalFile.isStaged ? "Unstage File" : "Stage File"}</span>
                <span className="text-[10px] text-[#71717a] font-mono">{modalFile.isStaged ? "Unstage" : "Stage"}</span>
              </button>

              <button
                onClick={() => {
                  GitDiscard("", [modalFile.path]).then(() => {
                    refreshStatus();
                    toast(`Discarded ${modalFile.path.split("/").pop()}`, "success");
                  });
                  setModalFile(null);
                }}
                className="w-full text-left px-3 py-2 hover:bg-[#27272a] text-[#fb7185] flex items-center justify-between cursor-pointer"
              >
                <span>Discard Changes</span>
                <span className="text-[10px] text-rose-400 font-mono">Revert</span>
              </button>

              <button
                onClick={() => {
                  handleOpenDiff(modalFile.path);
                  setModalFile(null);
                }}
                className="w-full text-left px-3 py-2 hover:bg-[#27272a] text-[#f4f4f5] flex items-center justify-between cursor-pointer"
              >
                <span>Open Diff</span>
                <IconFileDiff className="size-3.5 text-[#38bdf8]" />
              </button>

              <button
                onClick={() => {
                  globalOpenFile(modalFile.path);
                  setModalFile(null);
                }}
                className="w-full text-left px-3 py-2 hover:bg-[#27272a] text-[#f4f4f5] flex items-center justify-between cursor-pointer"
              >
                <span>View File</span>
                <IconFileText className="size-3.5 text-[#a1a1aa]" />
              </button>
            </div>
          </div>
        </div>
      )}
      {/* Discard Confirmation Modal */}
      {discardConfirmOpen && (
        <div
          className="fixed inset-0 bg-black/60 backdrop-blur-xs z-[9999] flex items-center justify-center p-4 select-none font-sans"
          onClick={() => setDiscardConfirmOpen(false)}
        >
          <div
            className="bg-[var(--bg-elevated)] border border-[var(--border-default)] w-full max-w-sm rounded-xl shadow-2xl p-4 flex flex-col gap-3.5"
            onClick={(e) => e.stopPropagation()}
          >
            <div className="space-y-1">
              <h3 className="text-sm font-semibold text-[var(--fg-primary)]">Discard All Changes?</h3>
              <p className="text-xs text-[var(--fg-secondary)] leading-relaxed">
                This will permanently discard all unstaged and untracked changes in your workspace. This action cannot be undone.
              </p>
            </div>
            <div className="flex items-center justify-end gap-2 pt-1 border-t border-[var(--border-default)]/60">
              <button
                type="button"
                onClick={() => setDiscardConfirmOpen(false)}
                className="px-3 py-1.5 rounded text-xs text-[var(--fg-secondary)] hover:text-[var(--fg-primary)] hover:bg-[var(--bg-surface-hover)] transition-colors cursor-pointer"
              >
                Cancel
              </button>
              <button
                type="button"
                onClick={executeDiscardAll}
                className="px-3.5 py-1.5 bg-red-600 hover:bg-red-500 text-white rounded text-xs font-semibold shadow-sm transition-colors cursor-pointer"
              >
                Discard All
              </button>
            </div>
          </div>
        </div>
      )}
    </div>
  );
}
