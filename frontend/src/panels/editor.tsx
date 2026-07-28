import { useState, useEffect, useCallback, useRef } from "react";
import { File, X, Save } from "lucide-react";
import { ReadFile, WriteFile } from "../../wailsjs/go/main/App";
import { CodeEditor } from "../components/code-editor";
import { cn } from "../lib/utils";

interface OpenFile {
  path: string;
  name: string;
  content: string;
  modified: boolean;
}

export function Editor() {
  const [files, setFiles] = useState<OpenFile[]>([]);
  const [activeIndex, setActiveIndex] = useState(-1);
  const filesRef = useRef<OpenFile[]>([]);

  // Keep ref in sync
  useEffect(() => {
    filesRef.current = files;
  }, [files]);

  // Register global open handler once
  useEffect(() => {
    setGlobalOpenFile(async (path: string) => {
      const currentFiles = filesRef.current;
      const existing = currentFiles.findIndex((f) => f.path === path);
      if (existing >= 0) {
        setActiveIndex(existing);
        return;
      }

      try {
        const content = await ReadFile(path);
        const name = path.split("/").pop() || path;
        setFiles((prev) => [
          ...prev,
          { path, name, content, modified: false },
        ]);
        setActiveIndex(currentFiles.length);
      } catch (err) {
        console.error("Failed to open file:", err);
      }
    });
  }, []);

  const closeFile = useCallback(
    (index: number) => {
      setFiles((prev) => {
        const next = [...prev];
        next.splice(index, 1);
        return next;
      });
      setActiveIndex((prev) => {
        if (prev === index) return Math.max(0, index - 1);
        if (prev > index) return prev - 1;
        return prev;
      });
    },
    []
  );

  const handleEditorChange = useCallback(
    (content: string) => {
      setFiles((prev) => {
        if (!prev[activeIndex]) return prev;
        const next = [...prev];
        next[activeIndex] = { ...next[activeIndex], content, modified: true };
        return next;
      });
    },
    [activeIndex]
  );

  const handleSave = useCallback(async () => {
    const file = filesRef.current[activeIndex];
    if (!file) return;
    try {
      await WriteFile(file.path, file.content);
      setFiles((prev) => {
        if (prev[activeIndex]) {
          const next = [...prev];
          next[activeIndex] = { ...next[activeIndex], modified: false };
          return next;
        }
        return prev;
      });
    } catch (err) {
      console.error("Failed to save file:", err);
    }
  }, [activeIndex]);

  const activeFile = files[activeIndex];

  if (files.length === 0) {
    return (
      <div className="flex-1 flex items-center justify-center text-sm text-muted-foreground">
        <div className="text-center">
          <File className="size-8 mx-auto mb-2 opacity-30" />
          <p>Click a file in the Explorer to open it</p>
        </div>
      </div>
    );
  }

  return (
    <div className="flex flex-col h-full">
      {/* Tabs bar */}
      <div className="flex items-center border-b bg-muted/20 shrink-0 overflow-x-auto">
        {files.map((file, i) => (
          <div
            key={file.path}
            className={cn(
              "flex items-center gap-1 px-3 py-1.5 text-xs border-r cursor-pointer whitespace-nowrap",
              i === activeIndex
                ? "bg-background border-b-2 border-b-primary"
                : "hover:bg-accent/50"
            )}
            onClick={() => setActiveIndex(i)}
          >
            <File className="size-3 text-blue-400 shrink-0" />
            <span className="truncate max-w-36">{file.name}</span>
            {file.modified && (
              <span className="text-yellow-500 text-[10px]">●</span>
            )}
            <button
              className="p-0.5 hover:bg-accent rounded ml-1"
              onClick={(e) => {
                e.stopPropagation();
                closeFile(i);
              }}
            >
              <X className="size-3" />
            </button>
          </div>
        ))}
      </div>

      {/* CodeMirror Editor */}
      {activeFile && (
        <div className="flex-1 flex flex-col overflow-hidden">
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
              <Save className="size-3" />
              Save
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
      )}
    </div>
  );
}

// Module-level global open handler
let globalOpenFn: (path: string) => void = () => {};

export function setGlobalOpenFile(fn: (path: string) => void) {
  globalOpenFn = fn;
}

export function globalOpenFile(path: string) {
  globalOpenFn(path);
}
