import { useState, useEffect, useCallback, useRef } from "react";
import { File, X, Save } from "lucide-react";
import { ReadFile, WriteFile } from "../../wailsjs/go/main/App";
import { cn } from "../lib/utils";
interface OpenFile {
  path: string;
  name: string;
  content: string;
  modified: boolean;
}

interface EditorProps {
  initialPath?: string;
  onClose?: () => void;
}

export function Editor({ initialPath, onClose }: EditorProps) {
  const [files, setFiles] = useState<OpenFile[]>([]);
  const [activeIndex, setActiveIndex] = useState(0);
  const textareaRef = useRef<HTMLTextAreaElement>(null);

  // Open initial file
  useEffect(() => {
    if (initialPath) {
      openFile(initialPath);
    }
  }, [initialPath]);

  const openFile = useCallback(async (path: string) => {
    // Check if already open
    const existing = files.findIndex((f) => f.path === path);
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
      setActiveIndex(files.length); // will be the new last index
    } catch (err) {
      console.error("Failed to open file:", err);
    }
  }, [files]);

  const closeFile = useCallback((index: number) => {
    setFiles((prev) => {
      const next = [...prev];
      next.splice(index, 1);
      return next;
    });
    if (activeIndex >= index) {
      setActiveIndex(Math.max(0, activeIndex - 1));
    }
  }, [activeIndex]);

  const handleEdit = useCallback((content: string) => {
    setFiles((prev) => {
      const next = [...prev];
      if (next[activeIndex]) {
        next[activeIndex] = {
          ...next[activeIndex],
          content,
          modified: true,
        };
      }
      return next;
    });
  }, [activeIndex]);

  const handleSave = useCallback(async () => {
    const file = files[activeIndex];
    if (!file) return;
    try {
      await WriteFile(file.path, file.content);
      setFiles((prev) => {
        const next = [...prev];
        if (next[activeIndex]) {
          next[activeIndex] = { ...next[activeIndex], modified: false };
        }
        return next;
      });
    } catch (err) {
      console.error("Failed to save file:", err);
    }
  }, [files, activeIndex]);

  const activeFile = files[activeIndex];

  return (
    <div className="flex flex-col h-full">
      {/* Tabs bar */}
      {files.length > 0 && (
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
              <span className="truncate max-w-24">{file.name}</span>
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
      )}

      {/* Editor area */}
      {activeFile ? (
        <div className="flex-1 flex flex-col">
          <div className="flex items-center justify-between px-3 py-1 border-b bg-muted/10 text-xs text-muted-foreground shrink-0">
            <span>{activeFile.path}</span>
            <button
              className={cn(
                "flex items-center gap-1 px-2 py-0.5 rounded",
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
          <textarea
            ref={textareaRef}
            className="flex-1 bg-background text-sm font-mono p-4 resize-none outline-none"
            value={activeFile.content}
            onChange={(e) => handleEdit(e.target.value)}
            spellCheck={false}
          />
        </div>
      ) : (
        <div className="flex-1 flex items-center justify-center text-sm text-muted-foreground">
          <div className="text-center">
            <File className="size-8 mx-auto mb-2 opacity-30" />
            <p>Click a file in the Explorer to open it</p>
          </div>
        </div>
      )}
    </div>
  );
}

// Expose openFile for external use (from explorer)
export let globalOpenFile: (path: string) => void = () => { };

export function setGlobalOpenFile(fn: (path: string) => void) {
  globalOpenFile = fn;
}
