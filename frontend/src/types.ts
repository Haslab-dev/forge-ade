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
  type: "file" | "shell" | "agent";
  content: string; // file content
  modified: boolean;
}

export interface ShortcutKeybinding {
  id: string;
  name: string;
  key: string;
}
