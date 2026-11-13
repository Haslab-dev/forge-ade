import { create } from 'zustand';
import { WriteSession } from '../lib/wails';
import { showToast } from '../lib/toast';
import { useTerminalStore } from './terminalStore';
import { useSessionStore } from './sessionStore';

export interface TerminalSnippet {
  id: string;
  title: string;
  command: string;
  category: 'remote' | 'docker' | 'git' | 'system' | 'dev' | 'custom' | string;
  description?: string;
  tags?: string[];
  isBuiltin?: boolean;
  createdAt: number;
}

export interface TerminalHistoryItem {
  id: string;
  command: string;
  timestamp: number;
  isFavorite: boolean;
  source?: 'manual' | 'snippet' | 'remote';
}

const BUILTIN_SNIPPETS: Omit<TerminalSnippet, 'id' | 'createdAt'>[] = [
  // Remote & SSH
  {
    title: 'Check Service Status',
    command: 'systemctl status <service>',
    category: 'remote',
    description: 'Check active/running status of a systemd unit on remote server',
    isBuiltin: true
  },
  {
    title: 'Tail Service Logs',
    command: 'journalctl -u <service> -n 100 -f',
    category: 'remote',
    description: 'Follow live stdout/stderr logs from a systemd service',
    isBuiltin: true
  },
  {
    title: 'Tail Syslog',
    command: 'tail -f /var/log/syslog',
    category: 'remote',
    description: 'Follow general system event log stream',
    isBuiltin: true
  },
  {
    title: 'Tail Nginx Error Log',
    command: 'tail -f /var/log/nginx/error.log',
    category: 'remote',
    description: 'Follow web server HTTP errors and connection drops',
    isBuiltin: true
  },
  {
    title: 'SSH Local Port Forward',
    command: 'ssh -N -L 8080:localhost:80 user@remote-host',
    category: 'remote',
    description: 'Forward remote port 80 to your local machine at port 8080',
    isBuiltin: true
  },
  {
    title: 'Rsync Fast Upload',
    command: "rsync -avzP --exclude 'node_modules' --exclude '.git' ./ user@remote-host:/var/www/app/",
    category: 'remote',
    description: 'Efficiently sync local directory to remote server over SSH',
    isBuiltin: true
  },
  {
    title: 'Inspect HTTP Response',
    command: 'curl -I -sS https://localhost',
    category: 'remote',
    description: 'Fetch headers from local or remote web server',
    isBuiltin: true
  },

  // Docker
  {
    title: 'Docker PS Formatted',
    command: 'docker ps -a --format "table {{.ID}}\\t{{.Names}}\\t{{.Status}}\\t{{.Ports}}"',
    category: 'docker',
    description: 'Clean readable list of running and stopped containers',
    isBuiltin: true
  },
  {
    title: 'Docker Compose Up',
    command: 'docker compose up -d',
    category: 'docker',
    description: 'Start compose service containers in detached background mode',
    isBuiltin: true
  },
  {
    title: 'Docker Compose Down',
    command: 'docker compose down',
    category: 'docker',
    description: 'Stop and remove containers, networks, and internal mounts',
    isBuiltin: true
  },
  {
    title: 'Docker Compose Logs',
    command: 'docker compose logs -f --tail=100',
    category: 'docker',
    description: 'Follow live interleaved logs from all compose services',
    isBuiltin: true
  },
  {
    title: 'Shell into Container',
    command: 'docker exec -it <container-name> /bin/sh',
    category: 'docker',
    description: 'Open interactive shell inside a running container',
    isBuiltin: true
  },
  {
    title: 'Prune Unused Docker Data',
    command: 'docker system prune -f --volumes',
    category: 'docker',
    description: 'Free up disk space by removing dangling images and unused volumes',
    isBuiltin: true
  },

  // System & Monitoring
  {
    title: 'Process Monitor',
    command: 'htop || top',
    category: 'system',
    description: 'Interactive CPU, memory, and task manager',
    isBuiltin: true
  },
  {
    title: 'Disk Space Usage',
    command: 'df -h',
    category: 'system',
    description: 'Human-readable filesystem disk capacity and usage',
    isBuiltin: true
  },
  {
    title: 'Memory Free / Used',
    command: 'free -h -m',
    category: 'system',
    description: 'RAM and swap usage statistics in megabytes/gigabytes',
    isBuiltin: true
  },
  {
    title: 'Listening Ports & PIDs',
    command: 'lsof -i -P -n | grep LISTEN',
    category: 'system',
    description: 'List all open server ports with bound process IDs',
    isBuiltin: true
  },
  {
    title: 'Top Memory Processes',
    command: 'ps aux --sort=-%mem | head -n 11',
    category: 'system',
    description: 'Display top 10 memory-consuming processes',
    isBuiltin: true
  },
  {
    title: 'Create Tarball Archive',
    command: 'tar -czvf backup-$(date +%F).tar.gz <folder>',
    category: 'system',
    description: 'Compress directory into date-stamped gzip tar archive',
    isBuiltin: true
  },

  // Git
  {
    title: 'Git Status Short',
    command: 'git status -sb',
    category: 'git',
    description: 'Compact git status showing branch tracking and dirty files',
    isBuiltin: true
  },
  {
    title: 'Git Log One-line Graph',
    command: 'git log --oneline -n 15 --graph',
    category: 'git',
    description: 'Compact ASCII tree of the last 15 commits',
    isBuiltin: true
  },
  {
    title: 'Git Pull Rebase',
    command: 'git pull --rebase origin $(git branch --show-current)',
    category: 'git',
    description: 'Rebase current branch on top of upstream changes without merge commit',
    isBuiltin: true
  },
  {
    title: 'Git Diff Stats',
    command: 'git diff --stat',
    category: 'git',
    description: 'Summary of file modifications and line change counts',
    isBuiltin: true
  },

  // Dev & Processes
  {
    title: 'Kill Hanging Node Processes',
    command: 'pkill -f node || true',
    category: 'dev',
    description: 'Cleanly terminate lingering node dev servers and watchers',
    isBuiltin: true
  },
  {
    title: 'Run Dev Server',
    command: 'bun run dev || npm run dev',
    category: 'dev',
    description: 'Start application development server with hot reload',
    isBuiltin: true
  }
];

const SNIPPETS_STORAGE_KEY = 'forge:terminal-snippets-v1';
const HISTORY_STORAGE_KEY = 'forge:terminal-history-v1';
const MAX_HISTORY_ITEMS = 60;

function loadStoredSnippets(): TerminalSnippet[] {
  try {
    const raw = localStorage.getItem(SNIPPETS_STORAGE_KEY);
    if (raw) {
      const parsed = JSON.parse(raw);
      if (Array.isArray(parsed) && parsed.length > 0) {
        return parsed;
      }
    }
  } catch {}
  // Default fallback
  const now = Date.now();
  return BUILTIN_SNIPPETS.map((b, i) => ({
    ...b,
    id: `snippet-builtin-${i + 1}`,
    createdAt: now - (BUILTIN_SNIPPETS.length - i) * 60000
  }));
}

function loadStoredHistory(): TerminalHistoryItem[] {
  try {
    const raw = localStorage.getItem(HISTORY_STORAGE_KEY);
    if (raw) {
      const parsed = JSON.parse(raw);
      if (Array.isArray(parsed)) {
        return parsed;
      }
    }
  } catch {}
  return [];
}

function saveSnippetsToStorage(snippets: TerminalSnippet[]) {
  try {
    localStorage.setItem(SNIPPETS_STORAGE_KEY, JSON.stringify(snippets));
  } catch {}
}

function saveHistoryToStorage(history: TerminalHistoryItem[]) {
  try {
    localStorage.setItem(HISTORY_STORAGE_KEY, JSON.stringify(history));
  } catch {}
}

export interface TerminalSnippetStoreState {
  snippets: TerminalSnippet[];
  history: TerminalHistoryItem[];
  isSnippetModalOpen: boolean;
  snippetModalTargetSessionId: string | null;
  snippetDraftCommand: string | null;
  snippetDraftTitle: string | null;

  // Snippets CRUD
  addSnippet: (snippet: Omit<TerminalSnippet, 'id' | 'createdAt'>) => string;
  updateSnippet: (id: string, updates: Partial<Omit<TerminalSnippet, 'id' | 'createdAt'>>) => void;
  deleteSnippet: (id: string) => void;
  resetDefaultSnippets: () => void;

  // History & Remember Command
  recordCommand: (command: string, source?: 'manual' | 'snippet' | 'remote') => void;
  toggleFavoriteHistory: (id: string) => void;
  deleteHistoryItem: (id: string) => void;
  clearHistory: () => void;

  // Modal controls
  openSnippetModal: (targetSessionId?: string | null, draftCommand?: string, draftTitle?: string) => void;
  closeSnippetModal: () => void;

  // Terminal Execution / Paste Bridge
  pasteToTerminal: (command: string, runImmediately: boolean, targetSessionId?: string | null) => Promise<boolean>;
}

export const useTerminalSnippetStore = create<TerminalSnippetStoreState>((set, get) => ({
  snippets: loadStoredSnippets(),
  history: loadStoredHistory(),
  isSnippetModalOpen: false,
  snippetModalTargetSessionId: null,
  snippetDraftCommand: null,
  snippetDraftTitle: null,

  addSnippet: (item) => {
    const id = `snippet-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`;
    const newSnippet: TerminalSnippet = {
      ...item,
      id,
      createdAt: Date.now()
    };
    const next = [newSnippet, ...get().snippets];
    set({ snippets: next });
    saveSnippetsToStorage(next);
    showToast(`Snippet "${item.title}" saved`, 'success');
    return id;
  },

  updateSnippet: (id, updates) => {
    const next = get().snippets.map(s => (s.id === id ? { ...s, ...updates } : s));
    set({ snippets: next });
    saveSnippetsToStorage(next);
    showToast('Snippet updated', 'info');
  },

  deleteSnippet: (id) => {
    const next = get().snippets.filter(s => s.id !== id);
    set({ snippets: next });
    saveSnippetsToStorage(next);
    showToast('Snippet removed', 'info');
  },

  resetDefaultSnippets: () => {
    const now = Date.now();
    const defaults: TerminalSnippet[] = BUILTIN_SNIPPETS.map((b, i) => ({
      ...b,
      id: `snippet-builtin-${i + 1}`,
      createdAt: now - (BUILTIN_SNIPPETS.length - i) * 60000
    }));
    set({ snippets: defaults });
    saveSnippetsToStorage(defaults);
    showToast('Reset to default snippets', 'info');
  },

  recordCommand: (command, source = 'manual') => {
    const trimmed = (command || '').trim();
    if (!trimmed) return;

    const prev = get().history;
    // Don't duplicate if identical to top item
    if (prev.length > 0 && prev[0].command === trimmed) {
      return;
    }

    const existingIdx = prev.findIndex(item => item.command === trimmed);
    let isFav = false;
    let nextList = [...prev];
    if (existingIdx !== -1) {
      isFav = nextList[existingIdx].isFavorite;
      nextList.splice(existingIdx, 1);
    }

    const newItem: TerminalHistoryItem = {
      id: `hist-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`,
      command: trimmed,
      timestamp: Date.now(),
      isFavorite: isFav,
      source
    };

    nextList = [newItem, ...nextList].slice(0, MAX_HISTORY_ITEMS);
    set({ history: nextList });
    saveHistoryToStorage(nextList);
  },

  toggleFavoriteHistory: (id) => {
    const next = get().history.map(item => (item.id === id ? { ...item, isFavorite: !item.isFavorite } : item));
    set({ history: next });
    saveHistoryToStorage(next);
  },

  deleteHistoryItem: (id) => {
    const next = get().history.filter(item => item.id !== id);
    set({ history: next });
    saveHistoryToStorage(next);
  },

  clearHistory: () => {
    set({ history: [] });
    saveHistoryToStorage([]);
    showToast('Command history cleared', 'info');
  },

  openSnippetModal: (targetSessionId, draftCommand, draftTitle) => {
    set({
      isSnippetModalOpen: true,
      snippetModalTargetSessionId: targetSessionId || null,
      snippetDraftCommand: draftCommand || null,
      snippetDraftTitle: draftTitle || null
    });
  },

  closeSnippetModal: () => {
    set({
      isSnippetModalOpen: false,
      snippetModalTargetSessionId: null,
      snippetDraftCommand: null,
      snippetDraftTitle: null
    });
  },

  pasteToTerminal: async (command, runImmediately, targetSessionId) => {
    const cleanCmd = (command || '').replace(/\r?\n/g, '\n');
    if (!cleanCmd.trim()) return false;

    // Resolve target session ID
    const target =
      targetSessionId ||
      get().snippetModalTargetSessionId ||
      useTerminalStore.getState().activeTerminalId ||
      useSessionStore.getState().activeSessionId;

    if (!target) {
      showToast('No active terminal session to paste into', 'warn');
      return false;
    }

    const payload = runImmediately
      ? (cleanCmd.endsWith('\r') || cleanCmd.endsWith('\n') ? cleanCmd : cleanCmd + '\r')
      : cleanCmd;

    try {
      await WriteSession(target, payload);
      get().recordCommand(cleanCmd, 'snippet');
      showToast(runImmediately ? 'Command executed' : 'Command inserted', 'info');
      // Dispatch refocus event
      window.dispatchEvent(new CustomEvent('forge:terminal-refocus', { detail: { sessionId: target } }));
      return true;
    } catch (err: any) {
      console.warn('Failed to paste to terminal session:', err);
      showToast(err?.message || 'Failed to paste into terminal', 'error');
      return false;
    }
  }
}));
