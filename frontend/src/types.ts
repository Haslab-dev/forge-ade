export interface Workspace {
  name: string;
  folders: string[];
  isTemporary: boolean;
  filePath: string;
  theme: string;
}

export interface RecentEntry {
  path: string;
  name: string;
  isWorkspace: boolean;
  lastOpened: number;
  pinned: boolean;
  favorite: boolean;
}

export interface EditorFile {
  id:  string;
  name: string;
  path: string; // absolute path for files, session ID for shells/agents
  type: "file" | "shell" | "agent" | "diff" | "conflict";
  content: string; // file content or diff text for "diff" tabs
  modified: boolean;
  // Diff tab metadata
  diffPath?: string; // the file the diff belongs to (relative to repo)
  diffHash?: string; // commit hash for commit diffs
  // Conflict resolve tab metadata
  conflictPath?: string; // the conflicted file (relative to repo)
  conflictStatus?: string; // e.g. "UU", "AU", "DU"
}

export interface ShortcutKeybinding {
  id: string;
  name: string;
  key: string;
}
