import { create } from 'zustand';
import type { AgentCLIConfig, AppSettings, TerminalSessionRecord } from '../types';
import {
  CreateAgentTerminalSession,
  DeleteAgentTerminalSession,
  EventsOn,
  GetAppSettings,
  ListAgentCLIConfigs,
  ListAgentTerminalSessions,
  ResetAgentCLIConfig,
  RestartAgentTerminalSession,
  SaveAgentCLIConfig,
  SaveAppSettings,
  StopSession
} from '../lib/wails';
import { DEFAULT_TERMINAL_SETTINGS, setTerminalSettings } from '../lib/terminalSettings';
import { ResetTerminalView } from '../components/terminal-view';

/**
 * Terminal Session mode store: agent CLI sessions (one process + PTY each),
 * their launch configs, and the application settings (default mode +
 * terminal rendering). The agent experience itself lives inside the CLI —
 * this store only manages session lifecycle, selection, and metadata.
 */

export const DEFAULT_APP_SETTINGS: AppSettings = {
  defaultMode: 'terminal',
  terminal: DEFAULT_TERMINAL_SETTINGS
};

interface SessionState {
  sessions: TerminalSessionRecord[];
  activeSessionId: string | null;
  loaded: boolean;
  /** New Task flow (choose agent → choose workspace → create session). */
  isNewTaskOpen: boolean;
  newTaskWorkspacePath?: string;
  /** Backend reason from the last failed createSession (shown in the modal). */
  lastCreateError: string;
  appSettings: AppSettings;
  agentConfigs: AgentCLIConfig[];

  load: () => Promise<void>;
  loadSettings: () => Promise<AppSettings>;
  saveSettings: (patch: Partial<AppSettings>) => Promise<AppSettings>;
  loadAgentConfigs: () => Promise<void>;
  saveAgentConfig: (cfg: AgentCLIConfig) => Promise<void>;
  resetAgentConfig: (id: string) => Promise<void>;

  selectSession: (id: string | null) => void;
  createSession: (agentId: string, workspacePath: string, title?: string) => Promise<TerminalSessionRecord | null>;
  stopSession: (id: string) => Promise<void>;
  restartSession: (id: string) => Promise<void>;
  deleteSession: (id: string) => Promise<void>;

  openNewTask: (workspacePath?: string) => void;
  closeNewTask: () => void;
}

function normalizeSession(raw: any): TerminalSessionRecord {
  return {
    id: String(raw?.id ?? ''),
    agentId: String(raw?.agentId ?? raw?.AgentId ?? ''),
    agentName: String(raw?.agentName ?? raw?.AgentName ?? 'Agent'),
    title: String(raw?.title ?? raw?.Title ?? ''),
    titleAuto: Boolean(raw?.titleAuto ?? raw?.TitleAuto),
    workspacePath: raw?.workspacePath ?? raw?.WorkspacePath ?? '',
    workingDirectory: String(raw?.workingDirectory ?? raw?.WorkingDirectory ?? ''),
    executable: String(raw?.executable ?? raw?.Executable ?? ''),
    args: Array.isArray(raw?.args) ? raw.args : raw?.Args,
    pid: raw?.pid ?? raw?.Pid,
    createdAt: Number(raw?.createdAt ?? raw?.CreatedAt ?? 0),
    startedAt: raw?.startedAt ?? raw?.StartedAt,
    endedAt: raw?.endedAt ?? raw?.EndedAt,
    status: (raw?.status ?? raw?.Status ?? 'exited') as TerminalSessionRecord['status'],
    exitCode: raw?.exitCode ?? raw?.ExitCode,
    error: raw?.error ?? raw?.Error
  };
}

/** "Running" / "Exited · 12m" — compact sidebar/status metadata. */
export function sessionStatusLabel(s: TerminalSessionRecord): string {
  if (s.status === 'running' || s.status === 'starting') return 'Running';
  if (s.status === 'failed') return 'Failed';
  const base = s.status === 'terminated' ? 'Terminated' : 'Exited';
  if (s.endedAt) {
    const mins = Math.max(0, Math.round((Date.now() - s.endedAt) / 60000));
    if (mins < 1) return `${base} · now`;
    if (mins < 60) return `${base} · ${mins}m`;
    const hours = Math.floor(mins / 60);
    if (hours < 24) return `${base} · ${hours}h`;
    return `${base} · ${Math.floor(hours / 24)}d`;
  }
  return base;
}

export const useSessionStore = create<SessionState>((set, get) => ({
  sessions: [],
  activeSessionId: (() => {
    try {
      return localStorage.getItem('forge_ade_active_terminal_session') || null;
    } catch {
      return null;
    }
  })(),
  loaded: false,
  isNewTaskOpen: false,
  newTaskWorkspacePath: undefined,
  lastCreateError: '',
  appSettings: DEFAULT_APP_SETTINGS,
  agentConfigs: [],

  load: async () => {
    try {
      const raw = await ListAgentTerminalSessions();
      const sessions = (Array.isArray(raw) ? raw : []).map(normalizeSession).filter(s => s.id);
      set(prev => {
        // Keep the selected session if it still exists; otherwise fall back
        // to the newest running session, else none (empty state).
        let active = prev.activeSessionId;
        if (!active || !sessions.some(s => s.id === active)) {
          active = sessions.find(s => s.status === 'running')?.id ?? null;
        }
        if (active) {
          try { localStorage.setItem('forge_ade_active_terminal_session', active); } catch {}
        }
        return { sessions, activeSessionId: active, loaded: true };
      });
    } catch {
      set({ loaded: true });
    }

    // Live updates from the Go session manager.
    EventsOn('agentsession:updated', (payload: any) => {
      const incoming = normalizeSession(payload?.session ?? payload);
      if (!incoming.id) return;
      set(prev => {
        const exists = prev.sessions.some(s => s.id === incoming.id);
        const sessions = exists
          ? prev.sessions.map(s => (s.id === incoming.id ? incoming : s))
          : [incoming, ...prev.sessions];
        return { sessions };
      });
    });
    EventsOn('agentsession:deleted', (payload: any) => {
      const id = String(payload?.id ?? payload ?? '');
      if (!id) return;
      set(prev => {
        const sessions = prev.sessions.filter(s => s.id !== id);
        const activeSessionId = prev.activeSessionId === id ? null : prev.activeSessionId;
        return { sessions, activeSessionId };
      });
    });
  },

  loadSettings: async () => {
    try {
      const raw = await GetAppSettings();
      const settings: AppSettings = {
        defaultMode: raw?.defaultMode === 'agent-ui' ? 'agent-ui' : 'terminal',
        terminal: { ...DEFAULT_TERMINAL_SETTINGS, ...(raw?.terminal ?? {}) }
      };
      set({ appSettings: settings });
      setTerminalSettings(settings.terminal);
      return settings;
    } catch {
      // Browser dev mode without the Go backend — defaults already apply.
      return get().appSettings;
    }
  },

  saveSettings: async patch => {
    const next: AppSettings = { ...get().appSettings, ...patch };
    set({ appSettings: next });
    setTerminalSettings(next.terminal);
    try {
      const saved = await SaveAppSettings(next);
      const merged: AppSettings = {
        defaultMode: saved?.defaultMode === 'agent-ui' ? 'agent-ui' : 'terminal',
        terminal: { ...DEFAULT_TERMINAL_SETTINGS, ...(saved?.terminal ?? next.terminal) }
      };
      set({ appSettings: merged });
      setTerminalSettings(merged.terminal);
      return merged;
    } catch (e) {
      console.warn('saveSettings failed', e);
      return next;
    }
  },

  loadAgentConfigs: async () => {
    try {
      const raw = await ListAgentCLIConfigs();
      const agentConfigs = (Array.isArray(raw) ? raw : []).map((c: any) => ({
        id: String(c?.id ?? ''),
        name: String(c?.name ?? c?.id ?? 'Agent'),
        executable: String(c?.executable ?? ''),
        args: Array.isArray(c?.args) ? c.args : [],
        resumeArgs: Array.isArray(c?.resumeArgs) ? c.resumeArgs : Array.isArray(c?.ResumeArgs) ? c.ResumeArgs : undefined,
        workingDirectory: c?.workingDirectory || '',
        environment: c?.environment && typeof c.environment === 'object' ? c.environment : {},
        enabled: c?.enabled !== false
      })) as AgentCLIConfig[];
      set({ agentConfigs });
    } catch {
      // ignore — New Task falls back to the static defaults at call sites
    }
  },

  saveAgentConfig: async cfg => {
    // Optimistic update; the backend merge is authoritative for defaults.
    set(prev => ({
      agentConfigs: prev.agentConfigs.some(c => c.id === cfg.id)
        ? prev.agentConfigs.map(c => (c.id === cfg.id ? cfg : c))
        : [...prev.agentConfigs, cfg]
    }));
    try {
      const saved = await SaveAgentCLIConfig(cfg);
      if (saved?.id) {
        set(prev => ({
          agentConfigs: prev.agentConfigs.map(c => (c.id === saved.id ? { ...c, ...saved } : c))
        }));
      }
    } catch (e) {
      console.warn('saveAgentConfig failed', e);
    }
  },

  resetAgentConfig: async id => {
    try {
      const saved = await ResetAgentCLIConfig(id);
      if (saved?.id) {
        set(prev => ({
          agentConfigs: prev.agentConfigs.map(c => (c.id === id ? { ...c, ...saved } : c))
        }));
      }
    } catch (e) {
      console.warn('resetAgentConfig failed', e);
    }
  },

  selectSession: id => {
    set({ activeSessionId: id });
    try {
      if (id) localStorage.setItem('forge_ade_active_terminal_session', id);
      else localStorage.removeItem('forge_ade_active_terminal_session');
    } catch {
      // ignore
    }
  },

  createSession: async (agentId, workspacePath, title) => {
    try {
      const created = await CreateAgentTerminalSession(agentId, workspacePath, title || '');
      if (!created?.id) return null;
      const session = normalizeSession(created);
      set(prev => {
        const exists = prev.sessions.some(s => s.id === session.id);
        return {
          sessions: exists ? prev.sessions.map(s => (s.id === session.id ? session : s)) : [session, ...prev.sessions],
          activeSessionId: session.id
        };
      });
      try { localStorage.setItem('forge_ade_active_terminal_session', session.id); } catch {}
      return session;
    } catch (e: any) {
      // Surface the backend reason (disabled CLI, remote workspace, missing
      // binary) — the New Task modal shows it instead of a generic failure.
      const message = e?.message || String(e || 'session failed');
      console.error('createSession failed', e);
      set({ lastCreateError: message });
      return null;
    }
  },

  stopSession: async id => {
    try {
      await StopSession(id);
    } catch (e) {
      console.warn('stopSession failed', e);
    }
  },

  restartSession: async id => {
    try {
      const restarted = await RestartAgentTerminalSession(id);
      if (restarted?.id) {
        // The new PTY spawns at the 80x24 default and the old process's
        // screen is stale — wipe the terminal, refit, and re-push the size.
        ResetTerminalView(id);
        const session = normalizeSession(restarted);
        set(prev => ({
          sessions: prev.sessions.map(s => (s.id === session.id ? session : s)),
          activeSessionId: session.id
        }));
      }
    } catch (e) {
      console.warn('restartSession failed', e);
    }
  },

  deleteSession: async id => {
    try {
      await DeleteAgentTerminalSession(id);
      set(prev => {
        const sessions = prev.sessions.filter(s => s.id !== id);
        const activeSessionId = prev.activeSessionId === id ? null : prev.activeSessionId;
        return { sessions, activeSessionId };
      });
    } catch (e) {
      console.warn('deleteSession failed', e);
    }
  },

  openNewTask: (workspacePath?: string) => set({ isNewTaskOpen: true, newTaskWorkspacePath: workspacePath }),
  closeNewTask: () => set({ isNewTaskOpen: false, newTaskWorkspacePath: undefined })
}));
