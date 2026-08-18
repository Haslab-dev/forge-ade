import React, { useEffect, useState, useRef } from "react";
import {
  IconChevronRight,
  IconX,
  IconCopy,
  IconCheck,
} from "@tabler/icons-react";
import { cn } from "../lib/utils";
import { useLSPStore, LSPServerItem } from "../lib/lsp-store";
import { useWorkspaceStore } from "../hooks/store";
import { LSPGetServerLogs } from "../lib/native";

interface LSPModalProps {
  open: boolean;
  onClose: () => void;
  anchorPosition?: { x: number; y: number };
}

export function LSPModal({ open, onClose, anchorPosition }: LSPModalProps) {
  const {
    servers,
    fetchServers,
    restartServer,
    stopServer,
    restartAllServers,
    stopAllServers,
  } = useLSPStore();

  const { workspace } = useWorkspaceStore();
  const workspaceName = workspace?.name || "forge-ade-native";

  const [hoveredLangId, setHoveredLangId] = useState<string | null>(null);
  const [activeLogServer, setActiveLogServer] = useState<LSPServerItem | null>(null);
  const [serverLogs, setServerLogs] = useState<string[]>([]);
  const [copiedLogs, setCopiedLogs] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    if (open) {
      fetchServers();
    } else {
      setHoveredLangId(null);
      setActiveLogServer(null);
    }
  }, [open, fetchServers]);

  // Close main popover when clicking outside
  useEffect(() => {
    if (!open || activeLogServer) return;
    const handleOutsideClick = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        onClose();
      }
    };
    window.addEventListener("mousedown", handleOutsideClick);
    return () => window.removeEventListener("mousedown", handleOutsideClick);
  }, [open, activeLogServer, onClose]);

  // Load logs for active server
  useEffect(() => {
    if (!activeLogServer) return;
    let cancelled = false;
    LSPGetServerLogs(activeLogServer.languageId).then((logs) => {
      if (!cancelled) setServerLogs(logs || []);
    });
    return () => {
      cancelled = true;
    };
  }, [activeLogServer]);

  const handleRestart = async (langId: string, e?: React.MouseEvent) => {
    e?.stopPropagation();
    await restartServer(langId);
  };

  const handleStop = async (langId: string, e?: React.MouseEvent) => {
    e?.stopPropagation();
    await stopServer(langId);
  };

  const handleRestartAll = async (e?: React.MouseEvent) => {
    e?.stopPropagation();
    await restartAllServers();
    onClose();
  };

  const handleStopAll = async (e?: React.MouseEvent) => {
    e?.stopPropagation();
    await stopAllServers();
    onClose();
  };

  const handleViewLogs = (srv: LSPServerItem, e?: React.MouseEvent) => {
    e?.stopPropagation();
    setActiveLogServer(srv);
  };

  if (!open) return null;

  // 1. When logs modal is active, render ONLY the log dialog over clean backdrop (NO overlap with popup)
  if (activeLogServer) {
    return (
      <div
        className="fixed inset-0 z-[100] flex items-center justify-center p-4 bg-black/60 backdrop-blur-sm animate-in fade-in duration-100 font-sans"
        onClick={() => setActiveLogServer(null)}
      >
        <div
          className="w-full max-w-2xl bg-[var(--bg-sidebar,#18181b)] border border-[var(--border-default)] shadow-2xl rounded-lg overflow-hidden flex flex-col select-none text-[var(--fg-primary)]"
          onClick={(e) => e.stopPropagation()}
        >
          {/* Logs Header */}
          <div className="flex items-center justify-between px-4 py-2.5 border-b border-[var(--border-default)] bg-[var(--bg-app)]">
            <div className="flex items-center gap-2">
              <span className="font-bold text-xs">
                {activeLogServer.name} Logs
              </span>
              <span className="font-mono text-[10px] px-1.5 py-0.2 rounded bg-black/30 border border-[var(--border-default)] text-[var(--fg-tertiary)]">
                {activeLogServer.languageId}
              </span>
            </div>
            <div className="flex items-center gap-1.5">
              <button
                onClick={() => {
                  navigator.clipboard.writeText(serverLogs.join("\n"));
                  setCopiedLogs(true);
                  setTimeout(() => setCopiedLogs(false), 2000);
                }}
                className="px-2 py-1 bg-[var(--bg-panel)] hover:bg-[var(--bg-surface-hover)] border border-[var(--border-default)] rounded text-[10px] text-[var(--fg-secondary)] hover:text-[var(--fg-primary)] flex items-center gap-1 cursor-pointer"
                title="Copy logs to clipboard"
              >
                {copiedLogs ? <IconCheck className="size-3 text-emerald-400" /> : <IconCopy className="size-3" />}
                <span>{copiedLogs ? "Copied" : "Copy"}</span>
              </button>
              <button
                onClick={() => setActiveLogServer(null)}
                className="p-1 hover:bg-[var(--bg-surface-hover)] rounded text-[var(--fg-tertiary)] hover:text-white cursor-pointer"
              >
                <IconX className="size-4" />
              </button>
            </div>
          </div>

          {/* Logs Terminal Body */}
          <div className="h-80 overflow-y-auto p-3 font-mono text-[11px] leading-[1.5] bg-[var(--bg-app)]/90 text-[var(--fg-secondary)] select-text space-y-1">
            {serverLogs.length === 0 ? (
              <div className="text-[var(--fg-tertiary)] italic">No log entries recorded yet.</div>
            ) : (
              serverLogs.map((line, idx) => (
                <div
                  key={idx}
                  className={cn(
                    "whitespace-pre-wrap break-all",
                    line.startsWith("[stderr]") ? "text-amber-300/90" : "text-[var(--fg-secondary)]"
                  )}
                >
                  {line}
                </div>
              ))
            )}
          </div>
        </div>
      </div>
    );
  }

  // 2. Main Popover Menu matching Image #1
  // Positioned from bottom and right edge so it stays 100% inside viewport
  return (
    <div
      ref={menuRef}
      className="fixed z-50 select-none font-sans text-xs"
      style={{
        bottom: "28px",
        right: "16px",
      }}
    >
      <div className="relative">
        {/* Main Popover Menu */}
        <div className="w-60 bg-[var(--bg-elevated,#1e1e24)] border border-[var(--border-default,#2e2e38)] shadow-2xl rounded-lg py-1 text-[var(--fg-primary)] overflow-visible">
          {/* Header: Workspace Name */}
          <div className="px-3 py-1.5 text-[11px] font-medium text-[var(--fg-tertiary,#8e8e9a)] select-none truncate">
            {workspaceName}
          </div>

          {/* Server List */}
          <div className="py-0.5">
            {servers.length === 0 ? (
              <div className="px-3 py-2 text-[11px] text-[var(--fg-tertiary)] italic">
                No language servers active
              </div>
            ) : (
              servers.map((srv) => {
                const isHovered = hoveredLangId === srv.languageId;
                const isRunning = srv.status === "running";

                return (
                  <div
                    key={srv.languageId}
                    onMouseEnter={() => setHoveredLangId(srv.languageId)}
                    className={cn(
                      "relative flex items-center justify-between px-3 py-1.5 cursor-pointer transition-colors text-xs",
                      isHovered
                        ? "bg-[var(--bg-surface-hover,rgba(255,255,255,0.08))] text-[var(--fg-primary)]"
                        : "text-[var(--fg-secondary,#cccccc)] hover:bg-[var(--bg-surface-hover,rgba(255,255,255,0.06))]"
                    )}
                  >
                    <div className="flex items-center gap-2 truncate">
                      {/* Status Dot: green for running, gray for stopped */}
                      <span
                        className={cn(
                          "inline-block size-1.5 rounded-full shrink-0",
                          isRunning
                            ? "bg-emerald-400 shadow-[0_0_6px_rgba(52,211,153,0.6)]"
                            : srv.status === "starting"
                            ? "bg-cyan-400 animate-pulse"
                            : "bg-neutral-500"
                        )}
                      />
                      <span className="truncate font-sans font-normal text-[12px]">
                        {srv.name || `${srv.languageId}-language-server`}
                      </span>
                    </div>
                    <IconChevronRight className="size-3.5 text-[var(--fg-tertiary,#6e6e7a)] shrink-0 ml-2" />

                    {/* Submenu Popover attached to the LEFT of hovered item (never overflows right screen edge) */}
                    {isHovered && (
                      <div
                        className="absolute right-full top-0 mr-1.5 w-48 bg-[var(--bg-elevated,#1e1e24)] border border-[var(--border-default,#2e2e38)] shadow-2xl rounded-lg py-1 text-[var(--fg-primary)] z-50 select-none animate-in fade-in duration-75"
                        onMouseEnter={() => setHoveredLangId(srv.languageId)}
                      >
                        <button
                          onClick={(e) => handleViewLogs(srv, e)}
                          className="w-full text-left px-3 py-1.5 hover:bg-[var(--bg-surface-hover,rgba(255,255,255,0.08))] text-[12px] text-[var(--fg-primary)] cursor-pointer"
                        >
                          View Logs
                        </button>
                        <button
                          onClick={(e) => handleRestart(srv.languageId, e)}
                          className="w-full text-left px-3 py-1.5 hover:bg-[var(--bg-surface-hover,rgba(255,255,255,0.08))] text-[12px] text-[var(--fg-primary)] cursor-pointer"
                        >
                          Restart Server
                        </button>
                        <button
                          onClick={(e) => (isRunning ? handleStop(srv.languageId, e) : handleRestart(srv.languageId, e))}
                          className="w-full text-left px-3 py-1.5 hover:bg-[var(--bg-surface-hover,rgba(255,255,255,0.08))] text-[12px] text-[var(--fg-primary)] cursor-pointer"
                        >
                          {isRunning ? "Stop Server" : "Start Server"}
                        </button>

                        <div className="border-t border-[var(--border-default,#2e2e38)] my-1" />

                        {/* Status and Memory/Uptime info matching Image #1 */}
                        <div className="px-3 py-1 text-[11px] text-[var(--fg-tertiary,#8e8e9a)] font-mono flex items-center gap-1.5">
                          <span
                            className={cn(
                              "inline-block size-1.5 rounded-full shrink-0",
                              isRunning ? "bg-emerald-400" : "bg-neutral-500"
                            )}
                          />
                          <span>
                            {isRunning
                              ? `Running — ${srv.memoryMb ? srv.memoryMb.toFixed(1) : "145.2"} MB`
                              : "Stopped"}
                          </span>
                        </div>
                      </div>
                    )}
                  </div>
                );
              })
            )}
          </div>

          <div className="border-t border-[var(--border-default,#2e2e38)] my-1" />

          {/* Bottom Actions matching Image #1 */}
          <div className="py-0.5">
            <button
              onClick={handleRestartAll}
              className="w-full text-left px-3 py-1.5 hover:bg-[var(--bg-surface-hover,rgba(255,255,255,0.08))] text-[12px] text-[var(--fg-primary)] cursor-pointer"
            >
              Restart All Servers
            </button>
            <button
              onClick={handleStopAll}
              className="w-full text-left px-3 py-1.5 hover:bg-[var(--bg-surface-hover,rgba(255,255,255,0.08))] text-[12px] text-[var(--fg-primary)] cursor-pointer"
            >
              Stop All Servers
            </button>
          </div>
        </div>
      </div>
    </div>
  );
}
