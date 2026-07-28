export interface Workspace {
  name: string;
  folders: string[];
  isTemporary: boolean;
  filePath?: string;
  theme: string;
}

export interface RecentEntry {
  path: string;
  name: string;
  isWorkspace: boolean;
  lastOpened: string;
  pinned: boolean;
  favorite: boolean;
}

export interface FileInfo {
  name: string;
  path: string;
  isDir: boolean;
  size: number;
  mode: string;
  modTime: string;
  symlink: boolean;
  symlinkTarget?: string;
  children?: FileInfo[];
  hidden: boolean;
  gitIgnored: boolean;
}

export interface TerminalSession {
  id: string;
  name: string;
  shell: string;
  cwd: string;
  createdAt: string;
  pid: number;
}

export interface SearchResult {
  path: string;
  filename: string;
  line: number;
  content: string;
  score: number;
}

export interface GitStatusEntry {
  path: string;
  staging: string;
  worktree: string;
}

export interface GitBranch {
  name: string;
  isHead: boolean;
  isRemote: boolean;
  isActive: boolean;
  commitHash: string;
}

export interface GitCommit {
  hash: string;
  author: string;
  email: string;
  message: string;
  timestamp: number;
  parents: number;
}

export interface Agent {
  id: string;
  name: string;
  provider: string;
  status: string;
  pid: number;
  createdAt: string;
  workspace: string;
}
