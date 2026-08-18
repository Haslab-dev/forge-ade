import React, { useState, useEffect, useRef, useMemo, useCallback } from "react";
import {
  IconSearch,
  IconTerminal2,
  IconRobot,
  IconFileText,
  IconFolder,
  IconFolderOpen,
  IconDeviceFloppy,
  IconX,
  IconCopy,
  IconTrash,
  IconSettings,
  IconCode,
  IconGitBranch,
  IconGitCommit,
  IconGitMerge,
  IconCloudUpload,
  IconCloudDownload,
  IconSparkles,
  IconActivity,
  IconWorld,
  IconLayoutSidebar,
  IconColumns,
  IconLayoutGrid,
  IconSquare,
  IconChevronRight,
  IconArrowUp,
  IconArrowDown,
  IconPlus,
  IconCursorText,
  IconSelect,
  IconArrowBackUp,
  IconPalette,
  IconCommand,
  IconKeyboard,
  IconRefresh,
  IconCheck,
  IconHelp,
  IconListNumbers,
  IconFunction,
  IconVariable,
  IconFiles,
  IconLocation,
  IconBinaryTree,
  IconHierarchy,
  IconTarget,
  IconServer,
  IconPlayerStop,
} from "@tabler/icons-react";
import { cn } from "../lib/utils";
import type { EditorView } from "@codemirror/view";
import {
  useEditorStore,
  useWorkspaceStore,
  useUIStore,
  useShortcutsStore,
  useWorkspaceTabStore,
} from "../hooks/store";
import { useLSPStore } from "../lib/lsp-store";
import {
  addCursorAbove,
  addCursorBelow,
  insertCursorsAtEndOfLine,
  selectCurrentLine,
  selectAllOccurrences,
  selectNextOccurrence,
  selectSkipOccurrence,
  undoCursor,
} from "../lib/multi-cursor";
import {
  executeLSPGoToDefinition,
  executeLSPGoToDeclaration,
  executeLSPGoToTypeDefinition,
  executeLSPGoToImplementation,
} from "../lib/lsp-extension";

export type PaletteMode = "command" | "file" | "line" | "doc-symbol" | "workspace-symbol" | "help";

export interface CommandItem {
  id: string;
  title: string;
  category: string;
  description?: string;
  shortcut?: string;
  icon: React.ReactNode;
  keywords?: string[];
  action: () => void | Promise<void>;
}

export interface FileItem {
  id: string;
  name: string;
  path: string;
  isDir?: boolean;
}

export interface SymbolItem {
  id: string;
  name: string;
  kind: string;
  line: number;
  container?: string;
}

export interface PaletteDisplayItem {
  id: string;
  title: string;
  category?: string;
  description?: string;
  shortcut?: string;
  icon: React.ReactNode;
  score?: number;
  matches?: boolean;
  action: () => void | Promise<void>;
}

interface CommandPaletteProps {
  open: boolean;
  initialMode?: PaletteMode;
  initialQuery?: string;
  onClose: () => void;
  onOpenFolder?: () => void;
  onOpenWorkspace?: () => void;
  onSaveFile?: () => void;
  onCloseFile?: () => void;
  onCreateShell?: () => void;
  onCreateAgent?: () => void;
  onOpenSettings?: () => void;
  onOpenUsage?: () => void;
  onSwitchScreen?: (screen: "editor" | "git-graph" | "usage") => void;
  onFormatCode?: () => void;
  onToggleSidebar?: () => void;
  onOpenFileByPath?: (path: string) => void;
  globalEditorView?: EditorView | null;
  onOpenLSPModal?: () => void;
}

// ---------------------------------------------------------------------------
// Fuzzy score algorithm
// ---------------------------------------------------------------------------
function fuzzyScore(pattern: string, text: string): { matches: boolean; score: number; indices: number[] } {
  if (!pattern) return { matches: true, score: 0, indices: [] };
  const pat = pattern.toLowerCase();
  const str = text.toLowerCase();
  const indices: number[] = [];

  let patIdx = 0;
  let score = 0;
  let prevMatchIdx = -2;

  for (let strIdx = 0; strIdx < str.length; strIdx++) {
    if (patIdx < pat.length && str[strIdx] === pat[patIdx]) {
      indices.push(strIdx);
      // Consecutive character bonus
      if (strIdx === prevMatchIdx + 1) {
        score += 8;
      }
      // Word boundary bonus
      if (strIdx === 0 || /[\s/\\._-]/.test(str[strIdx - 1])) {
        score += 10;
      }
      // Exact case match bonus
      if (text[strIdx] === pattern[patIdx]) {
        score += 2;
      }
      score += 1;
      patIdx++;
      prevMatchIdx = strIdx;
    }
  }

  const matches = patIdx === pat.length;
  if (!matches) return { matches: false, score: -1, indices: [] };

  // Penalize distance from start of string
  if (indices.length > 0) {
    score -= indices[0] * 0.5;
  }
  return { matches: true, score, indices };
}

export function CommandPalette({
  open,
  initialMode = "command",
  initialQuery = "",
  onClose,
  onOpenFolder,
  onOpenWorkspace,
  onSaveFile,
  onCloseFile,
  onCreateShell,
  onCreateAgent,
  onOpenSettings,
  onOpenUsage,
  onSwitchScreen,
  onFormatCode,
  onToggleSidebar,
  onOpenFileByPath,
  onOpenLSPModal,
  globalEditorView,
}: CommandPaletteProps) {
  const { files, activeFileIndex, setActiveFileIndex, setFiles } = useEditorStore();
  const { workspace, recentProjects } = useWorkspaceStore();
  const { theme, setTheme } = useUIStore();
  const { setWorkspaceLayoutMode } = useWorkspaceTabStore();

  const [query, setQuery] = useState("");
  const [selectedIndex, setSelectedIndex] = useState(0);
  const inputRef = useRef<HTMLInputElement>(null);
  const listRef = useRef<HTMLDivElement>(null);

  // Initialize input when modal opens
  useEffect(() => {
    if (open) {
      let q = initialQuery;
      if (!q) {
        if (initialMode === "command") q = ">";
        else if (initialMode === "doc-symbol") q = "@";
        else if (initialMode === "workspace-symbol") q = "#";
        else if (initialMode === "line") q = ":";
        else if (initialMode === "help") q = "?";
        else q = "";
      }
      setQuery(q);
      setSelectedIndex(0);
      requestAnimationFrame(() => {
        if (inputRef.current) {
          inputRef.current.focus();
          inputRef.current.select();
        }
      });
    }
  }, [open, initialMode, initialQuery]);

  // Determine current mode and active search term from prefix
  const { mode, cleanQuery } = useMemo<{ mode: PaletteMode; cleanQuery: string }>(() => {
    const trimmed = query.trimStart();
    if (trimmed.startsWith(">") || trimmed.startsWith(":>")) {
      return { mode: "command", cleanQuery: trimmed.slice(1).trim() };
    }
    if (trimmed.startsWith("@")) {
      return { mode: "doc-symbol", cleanQuery: trimmed.slice(1).trim() };
    }
    if (trimmed.startsWith("#")) {
      return { mode: "workspace-symbol", cleanQuery: trimmed.slice(1).trim() };
    }
    if (trimmed.startsWith(":") || trimmed.startsWith("/")) {
      const lineStr = trimmed.slice(1).trim();
      if (/^\d+/.test(lineStr) || trimmed.startsWith("/")) {
        return { mode: "line", cleanQuery: lineStr };
      }
      return { mode: "command", cleanQuery: lineStr };
    }
    if (trimmed.startsWith("?")) {
      return { mode: "help", cleanQuery: trimmed.slice(1).trim() };
    }
    // Default: if no prefix, show both commands and files matching query
    return { mode: "command", cleanQuery: trimmed };
  }, [query]);

  const activeFile = activeFileIndex >= 0 && activeFileIndex < files.length ? files[activeFileIndex] : null;

  // Jump to line in editor
  const handleJumpToLine = useCallback(
    (lineNum: number) => {
      if (!globalEditorView || !activeFile) return;
      const view = globalEditorView;
      const doc = view.state.doc;
      const targetLineNum = Math.max(1, Math.min(lineNum, doc.lines));
      const line = doc.line(targetLineNum);
      view.dispatch({
        selection: { anchor: line.from, head: line.from },
        scrollIntoView: true,
      });
      view.focus();
      onClose();
    },
    [globalEditorView, activeFile, onClose]
  );

  // ---------------------------------------------------------------------------
  // Build Full Command Catalog
  // ---------------------------------------------------------------------------
  const allCommands = useMemo<CommandItem[]>(() => {
    const cmds: CommandItem[] = [
      // File Operations
      {
        id: "file.save",
        title: "File: Save",
        category: "File",
        description: "Save active file to disk",
        shortcut: "⌘S",
        icon: <IconDeviceFloppy className="size-4 text-blue-400" />,
        keywords: ["save", "write", "disk"],
        action: () => onSaveFile?.(),
      },
      {
        id: "file.close",
        title: "File: Close Active Tab",
        category: "File",
        description: "Close the currently open tab",
        shortcut: "⌘W",
        icon: <IconX className="size-4 text-red-400" />,
        keywords: ["close", "tab", "file"],
        action: () => onCloseFile?.(),
      },
      {
        id: "file.close-all",
        title: "File: Close All Tabs",
        category: "File",
        description: "Close all open files, terminals, and agent tabs",
        icon: <IconTrash className="size-4 text-red-400" />,
        keywords: ["close", "all", "tabs", "clean"],
        action: () => {
          setFiles([]);
          setActiveFileIndex(-1);
        },
      },
      {
        id: "file.open-folder",
        title: "File: Open Folder",
        category: "File",
        description: "Open directory in workspace",
        shortcut: "⌘O",
        icon: <IconFolderOpen className="size-4 text-yellow-400" />,
        keywords: ["open", "folder", "directory", "project"],
        action: () => onOpenFolder?.(),
      },
      {
        id: "file.open-workspace",
        title: "File: Open Workspace",
        category: "File",
        description: "Open saved .workspace file",
        icon: <IconFolder className="size-4 text-yellow-400" />,
        keywords: ["workspace", "open"],
        action: () => onOpenWorkspace?.(),
      },
      {
        id: "file.copy-path",
        title: "File: Copy Path",
        category: "File",
        description: "Copy active file absolute path to clipboard",
        icon: <IconCopy className="size-4 text-gray-300" />,
        keywords: ["copy", "path", "clipboard"],
        action: () => {
          if (activeFile?.path) {
            navigator.clipboard.writeText(activeFile.path);
          }
        },
      },

      // Multi-Cursor and Selection Commands (RFC Compliant)
      {
        id: "cursor.add-above",
        title: "Multi-Cursor: Add Cursor Above",
        category: "Selection",
        description: "Insert a cursor directly on the line above (RFC)",
        shortcut: "⌥⌘↑ / Ctrl+Alt+↑",
        icon: <IconArrowUp className="size-4 text-emerald-400" />,
        keywords: ["cursor", "above", "multi", "line", "add"],
        action: () => {
          if (globalEditorView) {
            addCursorAbove(globalEditorView);
            globalEditorView.focus();
          }
        },
      },
      {
        id: "cursor.add-below",
        title: "Multi-Cursor: Add Cursor Below",
        category: "Selection",
        description: "Insert a cursor directly on the line below (RFC)",
        shortcut: "⌥⌘↓ / Ctrl+Alt+↓",
        icon: <IconArrowDown className="size-4 text-emerald-400" />,
        keywords: ["cursor", "below", "multi", "line", "add"],
        action: () => {
          if (globalEditorView) {
            addCursorBelow(globalEditorView);
            globalEditorView.focus();
          }
        },
      },
      {
        id: "cursor.insert-end-of-lines",
        title: "Multi-Cursor: Insert Cursors at End of Line",
        category: "Selection",
        description: "Place cursors at the end of each selected line (RFC)",
        shortcut: "⌥⇧I / Alt+Shift+I",
        icon: <IconCursorText className="size-4 text-emerald-400" />,
        keywords: ["cursor", "end", "line", "split", "multi"],
        action: () => {
          if (globalEditorView) {
            insertCursorsAtEndOfLine(globalEditorView);
            globalEditorView.focus();
          }
        },
      },
      {
        id: "cursor.select-current-line",
        title: "Multi-Cursor: Select Current Line",
        category: "Selection",
        description: "Select the entire line at cursor (RFC)",
        shortcut: "⌘L / Ctrl+L",
        icon: <IconSelect className="size-4 text-blue-400" />,
        keywords: ["select", "line", "expand"],
        action: () => {
          if (globalEditorView) {
            selectCurrentLine(globalEditorView);
            globalEditorView.focus();
          }
        },
      },
      {
        id: "cursor.select-all-occurrences",
        title: "Multi-Cursor: Select All Occurrences",
        category: "Selection",
        description: "Select all occurrences of the word/selection in file (RFC)",
        shortcut: "⇧⌘L / Ctrl+Shift+L",
        icon: <IconSparkles className="size-4 text-yellow-400" />,
        keywords: ["select", "all", "occurrences", "match"],
        action: () => {
          if (globalEditorView) {
            selectAllOccurrences(globalEditorView);
            globalEditorView.focus();
          }
        },
      },
      {
        id: "cursor.add-next-occurrence",
        title: "Multi-Cursor: Add Next Occurrence",
        category: "Selection",
        description: "Find next occurrence of word and add a cursor (RFC)",
        shortcut: "⌘D / Ctrl+D",
        icon: <IconPlus className="size-4 text-cyan-400" />,
        keywords: ["next", "occurrence", "cursor", "match"],
        action: () => {
          if (globalEditorView) {
            selectNextOccurrence(globalEditorView);
            globalEditorView.focus();
          }
        },
      },
      {
        id: "cursor.skip-next-occurrence",
        title: "Multi-Cursor: Skip Next Occurrence",
        category: "Selection",
        description: "Move current cursor to next occurrence without adding (RFC)",
        shortcut: "⌘K ⌘D / Ctrl+K Ctrl+D",
        icon: <IconChevronRight className="size-4 text-orange-400" />,
        keywords: ["skip", "occurrence", "next"],
        action: () => {
          if (globalEditorView) {
            selectSkipOccurrence(globalEditorView);
            globalEditorView.focus();
          }
        },
      },
      {
        id: "cursor.undo-cursor",
        title: "Multi-Cursor: Undo Cursor",
        category: "Selection",
        description: "Undo the last cursor or selection addition (RFC)",
        shortcut: "⌘U / Ctrl+U",
        icon: <IconArrowBackUp className="size-4 text-purple-400" />,
        keywords: ["undo", "cursor", "selection", "revert"],
        action: () => {
          if (globalEditorView) {
            undoCursor(globalEditorView);
            globalEditorView.focus();
          }
        },
      },

      // Code & Editing
      {
        id: "edit.format",
        title: "Editor: Format Document",
        category: "Editor",
        description: "Format active document using language formatter",
        shortcut: "⇧⌥F / ⇧⌘F",
        icon: <IconCode className="size-4 text-purple-400" />,
        keywords: ["format", "prettier", "biome", "indent"],
        action: () => onFormatCode?.(),
      },
      {
        id: "edit.goto-line",
        title: "Go to Line...",
        category: "Navigation",
        description: "Jump to a specific line number in active file",
        shortcut: "Ctrl+G",
        icon: <IconListNumbers className="size-4 text-cyan-400" />,
        keywords: ["line", "goto", "jump", "row"],
        action: () => {
          setQuery(":");
          if (inputRef.current) inputRef.current.focus();
        },
      },
      {
        id: "lsp.goto-definition",
        title: "Navigation: Go to Definition",
        category: "Navigation",
        description: "Jump to symbol definition in source (LSP)",
        shortcut: "F12",
        icon: <IconLocation className="size-4 text-cyan-400" />,
        keywords: ["definition", "jump", "goto", "symbol", "lsp"],
        action: () => {
          if (activeFile && globalEditorView && onOpenFileByPath) {
            executeLSPGoToDefinition(activeFile.path, globalEditorView, onOpenFileByPath);
          }
        },
      },
      {
        id: "lsp.goto-declaration",
        title: "Navigation: Go to Declaration",
        category: "Navigation",
        description: "Jump to symbol declaration in header or interface (LSP)",
        icon: <IconTarget className="size-4 text-blue-400" />,
        keywords: ["declaration", "jump", "goto", "symbol", "lsp"],
        action: () => {
          if (activeFile && globalEditorView && onOpenFileByPath) {
            executeLSPGoToDeclaration(activeFile.path, globalEditorView, onOpenFileByPath);
          }
        },
      },
      {
        id: "lsp.goto-type-definition",
        title: "Navigation: Go to Type Definition",
        category: "Navigation",
        description: "Jump to type definition of symbol (LSP)",
        icon: <IconHierarchy className="size-4 text-purple-400" />,
        keywords: ["type", "definition", "jump", "goto", "struct", "interface", "lsp"],
        action: () => {
          if (activeFile && globalEditorView && onOpenFileByPath) {
            executeLSPGoToTypeDefinition(activeFile.path, globalEditorView, onOpenFileByPath);
          }
        },
      },
      {
        id: "lsp.goto-implementation",
        title: "Navigation: Go to Implementation",
        category: "Navigation",
        description: "Jump to concrete implementation of interface/trait/method (LSP)",
        shortcut: "⌘F12 / Ctrl+F12",
        icon: <IconBinaryTree className="size-4 text-emerald-400" />,
        keywords: ["implementation", "impl", "jump", "goto", "method", "lsp"],
        action: () => {
          if (activeFile && globalEditorView && onOpenFileByPath) {
            executeLSPGoToImplementation(activeFile.path, globalEditorView, onOpenFileByPath);
          }
        },
      },
      {
        id: "lsp.show-servers",
        title: "LSP: Manage Language Servers",
        category: "LSP",
        description: "View active language server processes, diagnostics, and status",
        icon: <IconServer className="size-4 text-cyan-400" />,
        keywords: ["lsp", "server", "language", "status", "processes"],
        action: () => onOpenLSPModal?.(),
      },
      {
        id: "lsp.restart-active",
        title: "LSP: Restart Server for Active File",
        category: "LSP",
        description: "Restart language server associated with current active file",
        icon: <IconRefresh className="size-4 text-emerald-400" />,
        keywords: ["lsp", "restart", "server", "file", "reload"],
        action: () => {
          if (activeFile) {
            const ext = activeFile.path.split(".").pop() || "";
            useLSPStore.getState().restartServer(ext);
          }
        },
      },
      {
        id: "lsp.restart-all",
        title: "LSP: Restart All Language Servers",
        category: "LSP",
        description: "Restart all currently running language server processes",
        icon: <IconRefresh className="size-4 text-cyan-400" />,
        keywords: ["lsp", "restart", "all", "servers", "reload"],
        action: () => {
          useLSPStore.getState().restartAllServers();
        },
      },
      {
        id: "lsp.stop-all",
        title: "LSP: Stop All Language Servers",
        category: "LSP",
        description: "Terminate all active language server background processes",
        icon: <IconPlayerStop className="size-4 text-red-400" />,
        keywords: ["lsp", "stop", "all", "servers", "kill", "terminate"],
        action: () => {
          useLSPStore.getState().stopAllServers();
        },
      },

      // View & Navigation
      {
        id: "view.toggle-sidebar",
        title: "View: Toggle Sidebar Explorer",
        category: "View",
        description: "Show or hide the project explorer sidebar",
        shortcut: "⌘B",
        icon: <IconLayoutSidebar className="size-4 text-gray-300" />,
        keywords: ["sidebar", "explorer", "toggle", "files"],
        action: () => onToggleSidebar?.(),
      },
      {
        id: "view.screen-editor",
        title: "View: Switch to Editor Screen",
        category: "View",
        description: "Show main code editor surface",
        icon: <IconFileText className="size-4 text-blue-400" />,
        keywords: ["editor", "code", "screen"],
        action: () => onSwitchScreen?.("editor"),
      },
      {
        id: "view.screen-git-graph",
        title: "View: Switch to Git Graph Screen",
        category: "View",
        description: "Open interactive visual Git commit graph",
        icon: <IconGitMerge className="size-4 text-purple-400" />,
        keywords: ["git", "graph", "commit", "history", "branches"],
        action: () => onSwitchScreen?.("git-graph"),
      },
      {
        id: "view.screen-usage",
        title: "View: Switch to AI Usage & Analytics",
        category: "View",
        description: "Inspect LLM token usage and cost metrics",
        icon: <IconActivity className="size-4 text-emerald-400" />,
        keywords: ["usage", "tokens", "cost", "analytics"],
        action: () => onSwitchScreen?.("usage"),
      },
      {
        id: "view.layout-single",
        title: "View: Single Pane Layout",
        category: "Layout",
        description: "Show one full-screen active tab",
        icon: <IconSquare className="size-4 text-gray-300" />,
        keywords: ["layout", "single", "one"],
        action: () => setWorkspaceLayoutMode("single"),
      },
      {
        id: "view.layout-split",
        title: "View: Side-by-Side Split Layout",
        category: "Layout",
        description: "Split workspace horizontally into 2 columns",
        icon: <IconColumns className="size-4 text-gray-300" />,
        keywords: ["layout", "split", "columns", "side-by-side"],
        action: () => setWorkspaceLayoutMode("horizontal"),
      },
      {
        id: "view.layout-grid",
        title: "View: Grid Layout",
        category: "Layout",
        description: "Arrange tabs in a 2x2 grid layout",
        icon: <IconLayoutGrid className="size-4 text-gray-300" />,
        keywords: ["layout", "grid", "quad"],
        action: () => setWorkspaceLayoutMode("grid"),
      },

      // Terminal & AI Agents
      {
        id: "terminal.new",
        title: "Terminal: Launch New Shell Session",
        category: "Terminal",
        description: "Create a new interactive PTY terminal tab",
        shortcut: "⌃⇧T",
        icon: <IconTerminal2 className="size-4 text-emerald-400" />,
        keywords: ["terminal", "shell", "pty", "bash", "zsh"],
        action: () => onCreateShell?.(),
      },
      {
        id: "agent.new",
        title: "AI Agent: Launch Autonomous Agent Session",
        category: "AI Agent",
        description: "Create autonomous coding & reasoning agent",
        shortcut: "⌘I",
        icon: <IconRobot className="size-4 text-cyan-400" />,
        keywords: ["agent", "ai", "llm", "chat", "assistant"],
        action: () => onCreateAgent?.(),
      },

      // Settings & Preferences
      {
        id: "settings.open",
        title: "Preferences: Open Global Settings",
        category: "Settings",
        description: "Configure shortcuts, themes, LLMs, and MCP tools",
        shortcut: "⌘,",
        icon: <IconSettings className="size-4 text-gray-300" />,
        keywords: ["settings", "preferences", "config", "theme", "keys"],
        action: () => onOpenSettings?.(),
      },
      {
        id: "theme.dark-plus",
        title: "Preferences: Theme — Dark Plus (Default)",
        category: "Theme",
        description: "Switch to Dark Plus Theme",
        icon: <IconPalette className="size-4 text-blue-400" />,
        keywords: ["theme", "dark", "blue", "dark-plus"],
        action: () => setTheme("dark-plus"),
      },
      {
        id: "theme.midnight",
        title: "Preferences: Theme — Midnight",
        category: "Theme",
        description: "Switch to Midnight Theme",
        icon: <IconPalette className="size-4 text-indigo-400" />,
        keywords: ["theme", "midnight", "dark"],
        action: () => setTheme("midnight"),
      },
      {
        id: "theme.catppuccin",
        title: "Preferences: Theme — Catppuccin Mocha",
        category: "Theme",
        description: "Switch to Catppuccin Mocha Theme",
        icon: <IconPalette className="size-4 text-pink-400" />,
        keywords: ["theme", "catppuccin", "mocha"],
        action: () => setTheme("catppuccin"),
      },
      {
        id: "theme.nord",
        title: "Preferences: Theme — Nord",
        category: "Theme",
        description: "Switch to Nord Arctic Theme",
        icon: <IconPalette className="size-4 text-cyan-300" />,
        keywords: ["theme", "nord", "arctic"],
        action: () => setTheme("nord"),
      },
      {
        id: "theme.dracula",
        title: "Preferences: Theme — Dracula",
        category: "Theme",
        description: "Switch to Dracula Theme",
        icon: <IconPalette className="size-4 text-purple-400" />,
        keywords: ["theme", "dracula"],
        action: () => setTheme("dracula"),
      },
    ];

    return cmds;
  }, [
    onSaveFile,
    onCloseFile,
    setFiles,
    setActiveFileIndex,
    onOpenFolder,
    onOpenWorkspace,
    activeFile,
    globalEditorView,
    onFormatCode,
    onToggleSidebar,
    onSwitchScreen,
    setWorkspaceLayoutMode,
    onCreateShell,
    onCreateAgent,
    onOpenSettings,
    setTheme,
  ]);

  // ---------------------------------------------------------------------------
  // Document Symbols (Outline extracted from active file)
  // ---------------------------------------------------------------------------
  const docSymbols = useMemo<SymbolItem[]>(() => {
    if (!activeFile?.content) return [];
    const lines = activeFile.content.split("\n");
    const symbols: SymbolItem[] = [];

    const patterns = [
      { regex: /(?:function|func|def)\s+([A-Za-z0-9_$]+)/, kind: "function" },
      { regex: /(?:class|struct|enum|interface|type)\s+([A-Za-z0-9_$]+)/, kind: "class" },
      { regex: /(?:const|let|var|val)\s+([A-Za-z0-9_$]+)\s*[:=]/, kind: "variable" },
      { regex: /^\s*([A-Za-z0-9_$]+)\s*\([^)]*\)\s*[{:]/, kind: "method" },
    ];

    lines.forEach((line, idx) => {
      const lineNum = idx + 1;
      for (const p of patterns) {
        const match = line.match(p.regex);
        if (match && match[1]) {
          symbols.push({
            id: `sym-${lineNum}-${match[1]}`,
            name: match[1],
            kind: p.kind,
            line: lineNum,
          });
          break;
        }
      }
    });

    return symbols.slice(0, 100);
  }, [activeFile?.content]);

  // ---------------------------------------------------------------------------
  // Filter items based on active mode
  // ---------------------------------------------------------------------------
  const filteredItems = useMemo<PaletteDisplayItem[]>(() => {
    if (mode === "help") {
      return [
        {
          id: "help-cmd",
          title: "> or : Commands",
          category: "Palette Prefix",
          description: "Search and run all workbench and editor commands",
          icon: <IconCommand className="size-4 text-blue-400" />,
          action: () => setQuery(">"),
        },
        {
          id: "help-files",
          title: "(No Prefix) Files",
          category: "Palette Prefix",
          description: "Search and jump to any file in workspace or recent projects",
          icon: <IconFiles className="size-4 text-yellow-400" />,
          action: () => setQuery(""),
        },
        {
          id: "help-line",
          title: ": or / Go to Line",
          category: "Palette Prefix",
          description: "Jump directly to line number (e.g. :42 or /42)",
          icon: <IconListNumbers className="size-4 text-cyan-400" />,
          action: () => setQuery(":"),
        },
        {
          id: "help-symbol",
          title: "@ Document Symbols",
          category: "Palette Prefix",
          description: "Search functions, classes, and types in current document",
          icon: <IconFunction className="size-4 text-purple-400" />,
          action: () => setQuery("@"),
        },
        {
          id: "help-ws-symbol",
          title: "# Workspace Symbols",
          category: "Palette Prefix",
          description: "Search all symbols indexed across entire workspace",
          icon: <IconSparkles className="size-4 text-emerald-400" />,
          action: () => setQuery("#"),
        },
      ];
    }

    if (mode === "doc-symbol") {
      return docSymbols
        .map((s) => {
          const scoreResult = fuzzyScore(cleanQuery, s.name);
          return {
            ...s,
            title: s.name,
            category: s.kind,
            description: `Line ${s.line} · in ${activeFile?.name || "current file"}`,
            icon: <IconFunction className="size-4 text-purple-400" />,
            score: scoreResult.score,
            matches: scoreResult.matches,
            action: () => handleJumpToLine(s.line),
          };
        })
        .filter((i) => i.matches)
        .sort((a, b) => b.score - a.score);
    }

    if (mode === "line") {
      const lineNum = parseInt(cleanQuery, 10);
      const isValid = !isNaN(lineNum) && lineNum > 0;
      return [
        {
          id: "goto-line-target",
          title: isValid ? `Jump to Line ${lineNum}` : "Type a line number to jump...",
          category: "Go to Line",
          description: activeFile
            ? `Target in ${activeFile.name} (1 - ${activeFile.content?.split("\n").length || 1})`
            : "No active file",
          icon: <IconListNumbers className="size-4 text-cyan-400" />,
          action: () => {
            if (isValid) handleJumpToLine(lineNum);
          },
        },
      ];
    }

    // Command + File Mode
    const scoredCommands = allCommands
      .map((cmd) => {
        const titleScore = fuzzyScore(cleanQuery, cmd.title);
        const catScore = fuzzyScore(cleanQuery, cmd.category);
        const descScore = cmd.description ? fuzzyScore(cleanQuery, cmd.description) : { matches: false, score: -1 };
        const keyScore = cmd.keywords?.some((k) => k.toLowerCase().includes(cleanQuery.toLowerCase())) ? 15 : 0;

        const maxScore = Math.max(
          titleScore.score,
          catScore.score > 0 ? catScore.score * 0.7 : -1,
          descScore.score > 0 ? descScore.score * 0.5 : -1
        ) + keyScore;

        const matches = !cleanQuery || titleScore.matches || catScore.matches || descScore.matches || keyScore > 0;
        return {
          ...cmd,
          score: maxScore,
          matches,
        };
      })
      .filter((c) => c.matches)
      .sort((a, b) => b.score - a.score);

    // Also include open tabs and recent files if query is not strictly prefixed with '>'
    if (!query.startsWith(">") && cleanQuery) {
      const scoredFiles = files
        .filter((f) => f.type === "file")
        .map((f) => {
          const scoreResult = fuzzyScore(cleanQuery, f.name);
          return {
            id: `file-tab-${f.id}`,
            title: f.name,
            category: "Open Files",
            description: f.path,
            icon: <IconFileText className="size-4 text-blue-400" />,
            score: scoreResult.score + 5,
            matches: scoreResult.matches,
            action: () => {
              const idx = files.findIndex((item) => item.id === f.id);
              if (idx >= 0) setActiveFileIndex(idx);
            },
          };
        })
        .filter((f) => f.matches);

      return [...scoredCommands, ...scoredFiles].sort((a, b) => b.score - a.score);
    }

    return scoredCommands;
  }, [
    mode,
    cleanQuery,
    query,
    docSymbols,
    activeFile,
    handleJumpToLine,
    allCommands,
    files,
    setActiveFileIndex,
  ]);

  // Keep selection clamped within bounds
  useEffect(() => {
    setSelectedIndex(0);
  }, [filteredItems.length]);

  // Scroll selected item into view
  useEffect(() => {
    if (!listRef.current) return;
    const activeEl = listRef.current.children[selectedIndex] as HTMLElement;
    if (activeEl) {
      activeEl.scrollIntoView({ block: "nearest" });
    }
  }, [selectedIndex]);

  // Execute currently selected item
  const handleExecute = useCallback(
    (item?: { action: () => void | Promise<void> }) => {
      const target = item || filteredItems[selectedIndex];
      if (!target) return;
      onClose();
      try {
        target.action();
      } catch (err) {
        console.error("Error executing command:", err);
      }
    },
    [filteredItems, selectedIndex, onClose]
  );

  // Keyboard navigation inside palette
  const handleKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === "ArrowDown") {
      e.preventDefault();
      setSelectedIndex((prev) => (prev + 1) % Math.max(1, filteredItems.length));
    } else if (e.key === "ArrowUp") {
      e.preventDefault();
      setSelectedIndex((prev) => (prev - 1 + filteredItems.length) % Math.max(1, filteredItems.length));
    } else if (e.key === "Enter") {
      e.preventDefault();
      handleExecute();
    } else if (e.key === "Escape") {
      e.preventDefault();
      onClose();
    } else if (e.key === "Tab") {
      e.preventDefault();
      const first = filteredItems[0];
      if (first && mode === "help") {
        first.action();
      }
    }
  };

  if (!open) return null;

  return (
    <div
      className="fixed inset-0 z-50 flex items-start justify-center pt-[10vh] px-4 bg-black/60 backdrop-blur-sm animate-in fade-in duration-100"
      onClick={onClose}
    >
      <div
        className="w-full max-w-2xl bg-[var(--bg-sidebar)] border border-[var(--border-default)] shadow-2xl rounded-lg overflow-hidden flex flex-col font-sans select-none text-[var(--fg-primary)]"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Input Bar */}
        <div className="flex items-center px-3 py-2.5 border-b border-[var(--border-default)] bg-[var(--bg-app)]/50 gap-2.5">
          <input
            ref={inputRef}
            type="text"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            onKeyDown={handleKeyDown}
            placeholder={
              mode === "command"
                ? "Type a command name or shortcut..."
                : mode === "doc-symbol"
                ? "Type symbol name in document..."
                : mode === "workspace-symbol"
                ? "Type symbol name across workspace..."
                : mode === "line"
                ? "Type line number to jump..."
                : "Search files, commands, or type ? for help..."
            }
            className="flex-1 bg-transparent text-sm text-[var(--fg-primary)] placeholder:text-[var(--fg-tertiary)] focus:outline-none font-mono"
            autoFocus
          />
          {/* Mode Badge */}
          <div className="flex items-center gap-1 text-[10px] uppercase font-mono px-2 py-0.5 rounded bg-[var(--accent-primary)]/10 text-[var(--accent-primary)] border border-[var(--accent-primary)]/20">
            {mode === "command"
              ? "Commands"
              : mode === "doc-symbol"
              ? "Symbols"
              : mode === "line"
              ? "Go to Line"
              : mode === "help"
              ? "Help"
              : "Palette"}
          </div>
        </div>

        {/* Results List */}
        <div ref={listRef} className="max-h-[50vh] overflow-y-auto divide-y divide-[var(--border-default)]/30 p-1">
          {filteredItems.length === 0 ? (
            <div className="py-8 text-center text-xs text-[var(--fg-tertiary)] flex flex-col items-center gap-1.5">
              <IconHelp className="size-5 text-[var(--fg-tertiary)]" />
              <span>No matching commands or files found</span>
            </div>
          ) : (
            filteredItems.map((item, idx: number) => {
              const isSelected = idx === selectedIndex;
              return (
                <div
                  key={item.id || idx}
                  onClick={() => handleExecute(item)}
                  onMouseEnter={() => setSelectedIndex(idx)}
                  className={cn(
                    "flex items-center justify-between px-3 py-2 rounded text-xs cursor-pointer transition-colors gap-3",
                    isSelected
                      ? "bg-[var(--accent-primary)]/15 text-[var(--fg-primary)]"
                      : "hover:bg-[var(--bg-app)] text-[var(--fg-secondary)]"
                  )}
                >
                  <div className="flex items-center gap-2.5 min-w-0 flex-1">
                    <span className="shrink-0">{item.icon}</span>
                    <div className="flex flex-col min-w-0 flex-1">
                      <div className="flex items-center gap-2 truncate">
                        <span className={cn("font-medium", isSelected ? "text-[var(--fg-primary)]" : "")}>
                          {item.title}
                        </span>
                        {item.category && (
                          <span className="text-[10px] text-[var(--fg-tertiary)] px-1.5 py-0.2 rounded bg-black/20 border border-[var(--border-default)]/40 shrink-0">
                            {item.category}
                          </span>
                        )}
                      </div>
                      {item.description && (
                        <span className="text-[11px] text-[var(--fg-tertiary)] truncate mt-0.5">
                          {item.description}
                        </span>
                      )}
                    </div>
                  </div>

                  {/* Shortcut Key Badge */}
                  {item.shortcut && (
                    <span className="font-mono text-[10px] text-[var(--fg-tertiary)] bg-black/30 border border-[var(--border-default)] px-1.5 py-0.5 rounded shrink-0">
                      {item.shortcut}
                    </span>
                  )}
                </div>
              );
            })
          )}
        </div>

        {/* Footer info */}
        <div className="px-3 py-1.5 bg-[var(--bg-app)]/80 border-t border-[var(--border-default)] flex items-center justify-between text-[10px] text-[var(--fg-tertiary)] font-mono">
          <div className="flex items-center gap-3">
            <span>↑↓ Navigate</span>
            <span>↵ Select</span>
            <span>Esc Close</span>
          </div>
          <div className="flex items-center gap-2">
            <span>Prefixes: &gt; Cmd · @ Sym · : Line · ? Help</span>
          </div>
        </div>
      </div>
    </div>
  );
}
