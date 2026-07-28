import { useState, useEffect, useCallback, useRef } from "react";
import { File, X, Save, Shell, Bot } from "lucide-react";
import { ReadFile, WriteFile, RenameSession } from "../../wailsjs/go/main/App";
import { CodeEditor } from "../components/code-editor";
import { TerminalView } from "../components/terminal-view";
import { EventsOn } from "../../wailsjs/runtime";
import { cn } from "../lib/utils";
import { terminal } from "../../wailsjs/go/models";

interface OpenFile {
  path: string;
  name: string;
  content: string;
  modified: boolean;
}

interface EditorProps {
  sessionTabs: terminal.Session[];
  activeSessionId: string | null;
  onSelectSession: (id: string | null) => void;
  onCloseSession: (id: string) => void;
  onRenameSession: (id: string, name: string) => void;
}

export function Editor({
  sessionTabs,
  activeSessionId,
  onSelectSession,
  onCloseSession,
  onRenameSession,
}: EditorProps) {
  const [files, setFiles] = useState<OpenFile[]>([]);
  const [activeFileIndex, setActiveFileIndex] = useState(-1);
  const filesRef = useRef<OpenFile[]>([]);

  useEffect(() => {
    filesRef.current = files;
  }, [files]);

  // Register global open handler once
  useEffect(() => {
    setGlobalOpenFile(async (path: string) => {
      const currentFiles = filesRef.current;
      const existing = currentFiles.findIndex((f) => f.path === path);
      if (existing >= 0) {
        setActiveFileIndex(existing);
        onSelectSession(null);
        return;
      }
      try {
        const content = await ReadFile(path);
        const name = path.split("/").pop() || path;
        setFiles((prev) => [...prev, { path, name, content, modified: false }]);
        setActiveFileIndex(currentFiles.length);
        onSelectSession(null);
      } catch (err) {
        console.error("Failed to open file:", err);
      }
    });
  }, [onSelectSession]);

  // Listen for external file changes → reload open files
  useEffect(() => {
    const dispose = EventsOn("fs:changed", async (data: any) => {
      if (!data || !data.path) return;
      setFiles((prev) => {
        const idx = prev.findIndex((f) => f.path === data.path);
        if (idx < 0 || prev[idx].modified) return prev; // skip if modified
        // Re-read file content
        ReadFile(data.path)
          .then((content) => {
            setFiles((p) => {
              const next = [...p];
              next[idx] = { ...next[idx], content };
              return next;
            });
          })
          .catch(() => {}); // file might have been deleted
        return prev;
      });
    });
    return () => { if (dispose) dispose(); };
  }, []);

  const closeFile = useCallback((index: number) => {
    setFiles((prev) => {
      const next = [...prev];
      next.splice(index, 1);
      return next;
    });
    setActiveFileIndex((prev) => {
      if (prev === index) return Math.max(0, prev - 1);
      if (prev > index) return prev - 1;
      return prev;
    });
  }, []);

  const handleEditorChange = useCallback(
    (content: string) => {
      setFiles((prev) => {
        if (!prev[activeFileIndex]) return prev;
        const next = [...prev];
        next[activeFileIndex] = { ...next[activeFileIndex], content, modified: true };
        return next;
      });
    },
    [activeFileIndex]
  );

  const handleSave = useCallback(async () => {
    const file = filesRef.current[activeFileIndex];
    if (!file) return;
    try {
      await WriteFile(file.path, file.content);
      setFiles((prev) => {
        const next = [...prev];
        next[activeFileIndex] = { ...next[activeFileIndex], modified: false };
        return next;
      });
    } catch (err) {
      console.error("Failed to save file:", err);
    }
  }, [activeFileIndex]);

  // Right-click → rename session directly
  const handleRename = useCallback(
    async (s: terminal.Session) => {
      const newName = prompt("Rename session:", s.name);
      if (newName && newName !== s.name) {
        try {
          await RenameSession(s.id, newName);
          onRenameSession(s.id, newName);
        } catch (err) {
          console.error(err);
        }
      }
    },
    [onRenameSession]
  );

  const activeFile = files[activeFileIndex];
  const isSessionActive = activeSessionId !== null;

  return (
    <div className="flex flex-col h-full">
      {/* Unified Tab Bar */}
      <div className="flex items-center border-b bg-muted/20 shrink-0 overflow-x-auto">
        {/* File tabs */}
        {files.map((file, i) => (
          <div
            key={file.path}
            className={cn(
              "flex items-center gap-1 px-3 py-1.5 text-xs border-r cursor-pointer whitespace-nowrap shrink-0",
              !isSessionActive && i === activeFileIndex
                ? "bg-background border-b-2 border-b-primary"
                : "hover:bg-accent/50"
            )}
            onClick={() => { setActiveFileIndex(i); onSelectSession(null); }}
          >
            <File className="size-3 text-blue-400 shrink-0" />
            <span className="truncate max-w-28">{file.name}</span>
            {file.modified && <span className="text-yellow-500 text-[10px]">●</span>}
            <button
              className="p-0.5 hover:bg-accent rounded ml-1"
              onClick={(e) => { e.stopPropagation(); closeFile(i); }}
            >
              <X className="size-3" />
            </button>
          </div>
        ))}

        {/* Session tabs — only open ones */}
        {sessionTabs.map((s) => (
          <div
            key={s.id}
            className={cn(
              "flex items-center gap-1 px-3 py-1.5 text-xs border-r whitespace-nowrap shrink-0 select-none",
              isSessionActive && activeSessionId === s.id
                ? "bg-background border-b-2 border-b-cyan-500"
                : "hover:bg-accent/50 cursor-pointer"
            )}
            onClick={() => onSelectSession(s.id)}
            onContextMenu={(e) => {
              e.preventDefault();
              handleRename(s);
            }}
            title="Right-click to rename"
          >
            {s.type === "shell" ? (
              <Shell className="size-3 text-green-500 shrink-0" />
            ) : (
              <Bot className="size-3 text-cyan-500 shrink-0" />
            )}
            <span className="truncate max-w-28">{s.name}</span>
            <span
              className={cn(
                "w-1.5 h-1.5 rounded-full",
                s.status === "running" ? "bg-green-500" : "bg-muted-foreground"
              )}
            />
            <button
              className="p-0.5 hover:bg-accent rounded ml-1"
              onClick={(e) => {
                e.stopPropagation();
                onCloseSession(s.id);
              }}
              title="Close tab"
            >
              <X className="size-3" />
            </button>
          </div>
        ))}

        {files.length === 0 && sessionTabs.length === 0 && (
          <span className="px-3 py-1.5 text-xs text-muted-foreground">
            No files or sessions open
          </span>
        )}
      </div>

      {/* Content Area */}
      <div className="flex-1 overflow-hidden">
        {isSessionActive && activeSessionId ? (
          <TerminalView sessionId={activeSessionId} />
        ) : activeFile ? (
          <div className="flex flex-col h-full">
            <div className="flex items-center justify-between px-3 py-1 border-b bg-muted/10 text-xs text-muted-foreground shrink-0">
              <span className="truncate">{activeFile.path}</span>
              <button
                className={cn(
                  "flex items-center gap-1 px-2 py-0.5 rounded transition-colors",
                  activeFile.modified
                    ? "text-yellow-500 hover:bg-yellow-500/10"
                    : "text-muted-foreground"
                )}
                onClick={handleSave}
                disabled={!activeFile.modified}
              >
                <Save className="size-3" /> Save
              </button>
            </div>
            <div className="flex-1 overflow-hidden">
              <CodeEditor
                key={activeFile.path}
                value={activeFile.content}
                path={activeFile.path}
                onChange={handleEditorChange}
                onSave={handleSave}
              />
            </div>
          </div>
        ) : (
          <div className="flex items-center justify-center h-full text-sm text-muted-foreground">
            <div className="text-center">
              <File className="size-8 mx-auto mb-2 opacity-30" />
              <p>Click a file in the Explorer or start a Session</p>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}

let globalOpenFn: (path: string) => void = () => {};

export function setGlobalOpenFile(fn: (path: string) => void) {
  globalOpenFn = fn;
}

export function globalOpenFile(path: string) {
  globalOpenFn(path);
}
