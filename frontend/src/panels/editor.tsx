import { useState, useEffect, useCallback, useRef, useMemo, memo } from "react";
import { File, X, Save, Eye, EyeOff, Copy, GitCompare, Plus } from "lucide-react";
import { ReadFile, WriteFile, ReadFileBase64, RenameSession } from "../../wailsjs/go/main/App";
import { CodeEditor } from "../components/code-editor";
import { TerminalView } from "../components/terminal-view";
import { EventsOn } from "../../wailsjs/runtime";
import { cn } from "../lib/utils";
import { terminal } from "../../wailsjs/go/models";
import { marked } from "marked";

const IMAGE_EXTS = new Set(["png", "jpg", "jpeg", "gif", "svg", "webp", "ico"]);

function getFileExt(path: string) {
  return path.split(".").pop()?.toLowerCase() ?? "";
}

function FilePreview({ path }: { path: string }) {
  const ext = getFileExt(path);
  const [data, setData] = useState<string | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    setData(null);
    setError("");
    if (IMAGE_EXTS.has(ext)) {
      ReadFileBase64(path)
        .then((b64) => setData(b64))
        .catch((e) => setError(String(e)));
    } else if (ext === "pdf") {
      ReadFileBase64(path)
        .then((b64) => {
          const bytes = atob(b64);
          const arr = new Uint8Array(bytes.length);
          for (let i = 0; i < bytes.length; i++) arr[i] = bytes.charCodeAt(i);
          const blob = new Blob([arr], { type: "application/pdf" });
          setData(URL.createObjectURL(blob));
        })
        .catch((e) => setError(String(e)));
    } else if (ext === "html" || ext === "htm") {
      ReadFile(path)
        .then((html) => setData(html))
        .catch((e) => setError(String(e)));
    }
    return () => {
      if (data && ext === "pdf") URL.revokeObjectURL(data);
    };
  }, [path]);

  if (error) {
    return <div className="flex items-center justify-center h-full text-sm text-red-400">{error}</div>;
  }
  if (!data) {
    return <div className="flex items-center justify-center h-full text-sm text-muted-foreground">Loading...</div>;
  }
  if (IMAGE_EXTS.has(ext)) {
    return (
      <div className="flex items-center justify-center h-full overflow-auto bg-[#1a1a1a]">
        <img src={`data:image/${ext === "svg" ? "svg+xml" : ext};base64,${data}`} className="max-w-full max-h-full object-contain" alt={path.split("/").pop()} />
      </div>
    );
  }
  if (ext === "pdf") {
    return <iframe src={data} className="w-full h-full border-0" title={path.split("/").pop()} />;
  }
  if (ext === "html" || ext === "htm") {
    return <iframe srcDoc={data} className="w-full h-full border-0" title={path.split("/").pop()} sandbox="allow-scripts" />;
  }
  return null;
}

interface OpenFile {
  path: string;
  name: string;
  content: string;
  modified: boolean;
  targetLine?: number;
}

const EditorBody = memo(function EditorBody({
  path,
  content,
  targetLine,
  onChange,
  onSave,
}: {
  path: string;
  content: string;
  targetLine?: number;
  onChange: (v: string) => void;
  onSave: () => void;
}) {
  const ext = getFileExt(path);
  if (IMAGE_EXTS.has(ext) || ext === "pdf") {
    return <FilePreview path={path} />;
  }
  return (
    <CodeEditor
      key={path}
      value={content}
      path={path}
      scrollToLine={targetLine}
      onChange={onChange}
      onSave={onSave}
    />
  );
});

interface EditorProps {
  sessionTabs: terminal.Session[];
  activeSessionId: string | null;
  onSelectSession: (id: string | null) => void;
  onCloseSession: (id: string) => void;
  onRenameSession: (id: string, name: string) => void;
  onCreateShell?: () => void;
}

export function Editor({
  sessionTabs,
  activeSessionId,
  onSelectSession,
  onCloseSession,
  onRenameSession,
  onCreateShell,
}: EditorProps) {
  const [files, setFiles] = useState<OpenFile[]>([]);
  const [activeFileIndex, setActiveFileIndex] = useState(-1);
  const [previewFile, setPreviewFile] = useState<string | null>(null);
  const [diffFile, setDiffFile] = useState<string | null>(null);
  const [fileGitStatus, setFileGitStatus] = useState<Map<string, string>>(new Map());
  const filesRef = useRef<OpenFile[]>([]);

  useEffect(() => {
    filesRef.current = files;
  }, [files]);

  // Git status polling — removed, unused without git backend
  useEffect(() => {
    setFileGitStatus(new Map());
  }, []);

  useEffect(() => {
    setGlobalOpenFile(async (filePath: string, line?: number) => {
      setDiffFile(null);
      let cleanPath = filePath;
      let targetLine = line;
      const match = filePath.match(/^(.*?):(\d+)$/);
      if (match) {
        cleanPath = match[1];
        targetLine = parseInt(match[2], 10);
      }

      const currentFiles = filesRef.current;
      const existing = currentFiles.findIndex((f) => f.path === cleanPath);
      if (existing >= 0) {
        setFiles((prev) => {
          const next = [...prev];
          next[existing] = { ...next[existing], targetLine };
          return next;
        });
        setActiveFileIndex(existing);
        onSelectSession(null);
        return;
      }
      try {
        const content = await ReadFile(cleanPath);
        const name = cleanPath.split("/").pop() || cleanPath;
        setFiles((prev) => [...prev, { path: cleanPath, name, content, modified: false, targetLine }]);
        setActiveFileIndex(currentFiles.length);
        onSelectSession(null);
      } catch (err) {
        console.error("Failed to open file:", err);
      }
    });
    return () => clearGlobalOpenFile();
  }, [onSelectSession]);

  useEffect(() => {
    const dispose = EventsOn("fs:changed", async (data: any) => {
      if (!data || !data.path) return;
      setFiles((prev) => {
        const idx = prev.findIndex((f) => f.path === data.path);
        if (idx < 0 || prev[idx].modified) return prev;
        ReadFile(data.path)
          .then((content) => {
            setFiles((p) => {
              const next = [...p];
              next[idx] = { ...next[idx], content };
              return next;
            });
          })
          .catch(() => { });
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
  const isMarkdown = activeFile?.path.endsWith(".md") ?? false;
  const isHtml = !!(activeFile?.path.endsWith(".html") || activeFile?.path.endsWith(".htm"));
  const showPreview = previewFile === activeFile?.path;
  const showDiff = diffFile !== null;

  const markdownHtml = useMemo(() => {
    if (!isMarkdown || !activeFile) return "";
    try {
      return marked.parse(activeFile.content, { async: false }) as string;
    } catch {
      return activeFile.content;
    }
  }, [activeFile?.content, isMarkdown]);

  return (
    <div className="flex flex-col h-full">
      {/* Unified Tab Bar */}
      <div className="flex items-center border-b bg-muted/20 shrink-0 overflow-x-auto">
        {/* File tabs */}
        {files.map((file, i) => {
          const gitBadge = fileGitStatus.get(file.path);
          return (
            <div
              key={file.path}
              className={cn(
                "flex items-center gap-1 px-3 py-1.5 text-xs border-r cursor-pointer whitespace-nowrap shrink-0 group",
                !isSessionActive && i === activeFileIndex
                  ? "bg-background border-b-2 border-b-primary"
                  : "hover:bg-accent/50"
              )}
              onClick={() => { setActiveFileIndex(i); onSelectSession(null); setDiffFile(null); }}
              onContextMenu={(e) => {
                e.preventDefault();
                const newName = prompt("Rename file tab:", file.name);
                if (newName && newName !== file.name) {
                  const dir = file.path.split("/").slice(0, -1).join("/");
                  const newPath = dir + "/" + newName;
                  import("../../wailsjs/go/main/App").then(({ RenameFile }) => {
                    RenameFile(file.path, newPath).then(() => {
                      setFiles((prev) => {
                        const next = [...prev];
                        next[i] = { ...next[i], path: newPath, name: newName };
                        return next;
                      });
                    });
                  });
                }
              }}
              title={file.path}
            >
              <File className="size-3 text-blue-400 shrink-0" />
              <span className="truncate max-w-28">{file.name}</span>
              {gitBadge && (
                <span className={cn(
                  "text-[10px] font-bold",
                  gitBadge === "M" && "text-blue-400",
                  gitBadge === "A" && "text-green-400",
                  gitBadge === "D" && "text-red-400",
                )}>
                  {gitBadge === "A" ? "+" : gitBadge === "D" ? "-" : "●"}
                </span>
              )}
              {file.modified && <span className="text-yellow-500 text-[10px]">●</span>}
              <div className="hidden group-hover:flex items-center gap-0.5 ml-1">
                <button
                  className="p-0.5 hover:bg-accent rounded"
                  onClick={(e) => { e.stopPropagation(); setDiffFile(file.path); }}
                  title="Open Changes"
                >
                  <GitCompare className="size-3" />
                </button>
                <button
                  className="p-0.5 hover:bg-accent rounded"
                  onClick={(e) => { e.stopPropagation(); closeFile(i); }}
                >
                  <X className="size-3" />
                </button>
              </div>
            </div>
          );
        })}

        {files.length === 0 && sessionTabs.length === 0 && (
          <span className="px-3 py-1.5 text-xs text-muted-foreground">
            No files or sessions open
          </span>
        )}
      </div>

      {/* Content Area */}
      <div className="flex-1 overflow-hidden">
        {isSessionActive && activeSessionId ? (
          <TerminalView sessionId={activeSessionId} isActive={isSessionActive} />
        ) : activeFile ? (
          <div className="flex flex-col h-full">
            <div className="flex items-center justify-between px-3 py-1 border-b bg-muted/10 text-xs text-muted-foreground shrink-0">
              <span
                className="truncate cursor-pointer hover:text-foreground inline-flex items-center gap-1"
                onClick={() => navigator.clipboard.writeText(activeFile.path)}
                title="Click to copy path"
              >
                <Copy className="size-3 shrink-0 opacity-50" />
                {activeFile.path}
              </span>
              <div className="flex items-center gap-1">
                <button
                  className="flex items-center gap-1 px-2 py-0.5 rounded transition-colors text-muted-foreground hover:text-foreground hover:bg-accent/50"
                  onClick={() => setDiffFile(activeFile.path)}
                  title="Open Changes"
                >
                  <GitCompare className="size-3" /> Changes
                </button>
                {isMarkdown && (
                  <button
                    className="flex items-center gap-1 px-2 py-0.5 rounded transition-colors text-muted-foreground hover:text-foreground hover:bg-accent/50"
                    onClick={() => setPreviewFile(showPreview ? null : activeFile.path)}
                  >
                    {showPreview ? <EyeOff className="size-3" /> : <Eye className="size-3" />}
                    {showPreview ? "Edit" : "Preview"}
                  </button>
                )}
                {isHtml && (
                  <button
                    className="flex items-center gap-1 px-2 py-0.5 rounded transition-colors text-muted-foreground hover:text-foreground hover:bg-accent/50"
                    onClick={() => setPreviewFile(showPreview ? null : activeFile.path)}
                  >
                    {showPreview ? <EyeOff className="size-3" /> : <Eye className="size-3" />}
                    {showPreview ? "Code" : "Preview"}
                  </button>
                )}
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
            </div>
            <div className="flex-1 overflow-hidden">
              {showPreview && isHtml ? (
                <div
                  className="h-full overflow-auto p-0"
                  dangerouslySetInnerHTML={{ __html: activeFile.content }}
                />
              ) : showPreview ? (
                <div
                  className="h-full overflow-auto p-6 text-sm leading-relaxed"
                  style={{ lineHeight: 1.7 }}
                  dangerouslySetInnerHTML={{ __html: markdownHtml }}
                />
              ) : (
                <EditorBody
                  path={activeFile.path}
                  content={activeFile.content}
                  targetLine={activeFile.targetLine}
                  onChange={handleEditorChange}
                  onSave={handleSave}
                />
              )}
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

let globalOpenFn: (path: string, line?: number) => void = () => {};
let globalOpenFnActive = false;
let beforeOpenFileFn: ((path: string) => void) | null = null;
let pendingOpenPath: { path: string; line?: number } | null = null;

export function setGlobalOpenFile(fn: (path: string, line?: number) => void) {
  globalOpenFn = fn;
  globalOpenFnActive = true;
  // Flush any path buffered while Editor was unmounted
  if (pendingOpenPath) {
    const p = pendingOpenPath;
    pendingOpenPath = null;
    fn(p.path, p.line);
  }
}

export function clearGlobalOpenFile() {
  globalOpenFn = () => {};
  globalOpenFnActive = false;
}

export function setOnBeforeOpenFile(fn: (path: string) => void) {
  beforeOpenFileFn = fn;
}

export function globalOpenFile(path: string, line?: number) {
  beforeOpenFileFn?.(path);
  if (globalOpenFnActive) {
    globalOpenFn(path, line);
  } else {
    pendingOpenPath = { path, line };
  }
}
