import React, { createContext, useContext, useState, useEffect, useCallback, useRef, useMemo } from 'react';
import { 
  WorkspaceMode, 
  ThemeMode, 
  AgentSession, 
  AgentMessage, 
  FileItem, 
  EditorTab, 
  TerminalTab, 
  ActivityBarItem, 
  LSPDiagnostic,
  AgentExecutionMode,
  AgentEngineType,
  ACPAgent,
  PrivacySettings,
  FileDiff,
  LLMProviderConfig,
  MCPEntry,
  SkillEntry
} from '../types';
import { DEFAULT_AGENTS, DEFAULT_PRIVACY, DEFAULT_PROVIDERS } from './agentRegistryStore';
import { AgentEngine } from '../services/agentEngine';
import { ApiBridge } from '../services/apiBridge';
import { useUIStore } from '../hooks/store';

interface WorkspaceContextType {
  mode: WorkspaceMode;
  setMode: (mode: WorkspaceMode) => void;
  theme: ThemeMode;
  setTheme: (theme: ThemeMode) => void;
  toggleTheme: () => void;
  
  // Recent Workspaces
  recentWorkspaces: string[];
  addRecentWorkspace: (path: string) => void;
  removeRecentWorkspace: (path: string) => void;
  clearRecentWorkspaces: () => void;
  closeWorkspace: () => void;
  
  // Agent Sessions
  activeSessionId: string | null;
  setActiveSessionId: (id: string | null) => void;
  sessions: AgentSession[];
  activeSession: AgentSession | undefined;

  // Agent Registry & ACP
  agents: ACPAgent[];
  activeAgentId: string;
  setActiveAgentId: (id: string) => void;
  activeAgent: ACPAgent;
  toggleAgentEnabled: (id: string) => void;
  updateAgentConfig: (id: string, updates: Partial<ACPAgent>) => void;
  addAgent: (agent: ACPAgent) => void;
  acpEnabled: boolean;
  setAcpEnabled: (enabled: boolean) => void;
  privacySettings: PrivacySettings;
  setPrivacySettings: React.Dispatch<React.SetStateAction<PrivacySettings>>;

  // LLM Providers & Models Config
  providers: LLMProviderConfig[];
  updateProvider: (id: string, updates: Partial<LLMProviderConfig>) => void;
  addProvider: (provider: LLMProviderConfig) => void;
  deleteProvider: (id: string) => void;
  addModelToProvider: (providerId: string, modelName: string) => void;
  deleteModelFromProvider: (providerId: string, modelName: string) => void;
  toggleModelSelection: (providerId: string, modelName: string) => void;
  fetchProviderModels: (providerId: string) => Promise<void>;

  // MCPs & Skills Discovery
  mcps: MCPEntry[];
  addMcp: (mcp: MCPEntry) => void;
  toggleMcp: (id: string) => void;
  deleteMcp: (id: string) => void;
  skills: SkillEntry[];
  addSkill: (skill: SkillEntry) => void;
  toggleSkill: (id: string) => void;
  deleteSkill: (id: string) => void;
  discoveredMcps: any[];
  discoveredSkills: any[];
  isDiscovering: boolean;
  runDiscovery: () => Promise<void>;
  importDiscoveredMcp: (disc: any) => void;
  importDiscoveredSkill: (disc: any) => void;

  // Agent input configuration
  agentExecutionMode: AgentExecutionMode;
  setAgentExecutionMode: (mode: AgentExecutionMode) => void;
  agentEngine: AgentEngineType;
  setAgentEngine: (engine: AgentEngineType) => void;
  contextUsage: { usedTokens: number; maxTokens: number; percent: number };

  // Diffs & Review System
  diffs: FileDiff[];
  activeDiff: FileDiff | null;
  setActiveDiff: (diff: FileDiff | null) => void;
  acceptDiff: (diffId: string) => void;
  rejectDiff: (diffId: string) => void;
  addDiff: (diff: FileDiff) => void;
  openDiffInEditor: (diff: FileDiff) => void;
  openGitGraphPane: () => void;

  // Git State
  gitBranch: string;
  gitFiles: Array<{ path: string; status: string }>;
  gitCommits: any[];
  refreshGitStatus: () => Promise<void>;
  refreshGitLog: () => Promise<void>;

  // Workspace / Files
  files: FileItem[];
  openTabs: EditorTab[];
  activeTabId: string | null;
  selectedFile: FileItem | null;
  isSplitEditor: boolean;
  setIsSplitEditor: (val: boolean | ((prev: boolean) => boolean)) => void;
  createFile: (filePath: string, content?: string) => Promise<void>;
  createFolder: (folderPath: string) => Promise<void>;
  deleteFile: (filePath: string) => Promise<void>;
  renameFile: (oldPath: string, newPath: string) => Promise<void>;
  openFolder: (path?: string) => Promise<void>;
  refreshFiles: () => Promise<void>;

  // Folder modal
  isFolderModalOpen: boolean;
  setIsFolderModalOpen: (val: boolean) => void;

  // Navigation & Activities
  activeActivity: ActivityBarItem;
  setActiveActivity: (activity: ActivityBarItem) => void;
  
  // Ghostty Terminal
  terminalTabs: TerminalTab[];
  activeTerminalTabId: string;
  setActiveTerminalTabId: (id: string) => void;
  isTerminalOpen: boolean;
  setIsTerminalOpen: (val: boolean) => void;

  // Right quick drawer
  isRightActionDrawerOpen: boolean;
  setIsRightActionDrawerOpen: (val: boolean | ((prev: boolean) => boolean)) => void;

  // Models & Modals
  currentModel: string;
  setCurrentModel: (model: string) => void;
  isCommandPaletteOpen: boolean;
  setIsCommandPaletteOpen: (val: boolean) => void;
  isCustomizationsModalOpen: boolean;
  setIsCustomizationsModalOpen: (val: boolean) => void;
  isSettingsModalOpen: boolean;
  setIsSettingsModalOpen: (val: boolean) => void;

  // Unified Settings Pane
  settingsActiveSection: string;
  setSettingsActiveSection: (sec: string) => void;

  // Actions
  openFileInEditor: (filePath: string, line?: number, column?: number) => Promise<void>;
  updateFolderChildren: (folderPath: string, children: FileItem[]) => void;
  openSettingsTab: (section?: string) => void;
  closeTab: (tabId: string) => void;
  setActiveTabId: (tabId: string) => void;
  updateFileContent: (fileId: string, newContent: string) => Promise<void>;
  createNewSession: (initialPrompt?: string, agentId?: string) => void;
  sendAgentPrompt: (promptText: string) => void;
  stopAgentExecution: () => void;
  deleteSession: (id: string) => void;

  // Persistent Sessions & History
  savedSessions: AgentSession[];
  openSessionFromHistory: (session: AgentSession) => void;
  deleteSessionPermanently: (id: string) => Promise<void>;
  reloadSavedSessions: () => Promise<void>;

  // Path info
  activeWorkspacePath: string;
  setActiveWorkspacePath: (path: string) => void;

  // LSP
  diagnostics: LSPDiagnostic[];
}

const WorkspaceContext = createContext<WorkspaceContextType | undefined>(undefined);

export const WorkspaceProvider: React.FC<{ children: React.ReactNode }> = ({ children }) => {
  const [mode, setMode] = useState<WorkspaceMode>('agent');
  const [theme, setTheme] = useState<ThemeMode>(() => {
    return (localStorage.getItem('forge_ade_theme') as ThemeMode) || (localStorage.getItem('my_ade_theme') as ThemeMode) || 'dark';
  });

  const [activeWorkspacePath, setActiveWorkspacePathState] = useState<string>(() => {
    return localStorage.getItem('forge_ade_workspace_path') || localStorage.getItem('my_ade_workspace_path') || '';
  });

  const [isFolderModalOpen, setIsFolderModalOpen] = useState<boolean>(false);

  // Recent Workspaces
  const [recentWorkspaces, setRecentWorkspaces] = useState<string[]>(() => {
    try {
      const saved = localStorage.getItem('forge_ade_recent_workspaces') || localStorage.getItem('my_ade_recent_workspaces');
      if (saved) return JSON.parse(saved);
    } catch {}
    return [];
  });

  const addRecentWorkspace = useCallback((path: string) => {
    if (!path || !path.trim()) return;
    setRecentWorkspaces(prev => {
      const filtered = prev.filter(p => p !== path);
      const next = [path, ...filtered].slice(0, 15);
      try { localStorage.setItem('forge_ade_recent_workspaces', JSON.stringify(next)); } catch {}
      return next;
    });
  }, []);

  const removeRecentWorkspace = useCallback((pathToRemove: string) => {
    setRecentWorkspaces(prev => {
      const next = prev.filter(p => p !== pathToRemove);
      try { localStorage.setItem('forge_ade_recent_workspaces', JSON.stringify(next)); } catch {}
      return next;
    });
  }, []);

  const clearRecentWorkspaces = useCallback(() => {
    setRecentWorkspaces([]);
    try { localStorage.setItem('forge_ade_recent_workspaces', JSON.stringify([])); } catch {}
  }, []);

  const closeWorkspace = useCallback(() => {
    setActiveWorkspacePathState('');
    localStorage.removeItem('forge_ade_workspace_path');
    localStorage.removeItem('my_ade_workspace_path');
    setFiles([]);
    setOpenTabs([]);
  }, []);

  const setActiveWorkspacePath = useCallback((newPath: string) => {
    setActiveWorkspacePathState(newPath);
    if (newPath) {
      localStorage.setItem('forge_ade_workspace_path', newPath);
      addRecentWorkspace(newPath);
    } else {
      localStorage.removeItem('forge_ade_workspace_path');
    }
  }, [addRecentWorkspace]);

  // Smart auto-renaming session helper (20-30 chars, ChatGPT/Gemini style)
  const formatSmartSessionTitle = (prompt: string): string => {
    if (!prompt || !prompt.trim()) return 'New Session';
    
    // Remove attachments or system markers
    let clean = prompt
      .replace(/\[Attached File:[\s\S]*?\]/g, '')
      .replace(/\[TOOL EXECUTION RESULTS\][\s\S]*$/g, '')
      .replace(/^["'`]|["'`]$/g, '')
      .trim();

    // Strip leading conversational filler words
    clean = clean
      .replace(/^(please|can you|could you|help me|i want to|how to|how do i|tell me about|explain to me|what is|create a|build a|write a|give me)\s+/i, '')
      .replace(/[`*_#~]/g, '')
      .trim();

    if (!clean) return 'New Session';

    // Capitalize first character
    clean = clean.charAt(0).toUpperCase() + clean.slice(1);

    // Limit to 20-28 characters max without cutting words awkwardly
    if (clean.length > 28) {
      const trimmed = clean.slice(0, 26);
      const lastSpace = trimmed.lastIndexOf(' ');
      if (lastSpace > 12) {
        clean = trimmed.slice(0, lastSpace).trim();
      } else {
        clean = trimmed.trim();
      }
      return `${clean}...`;
    }

    return clean;
  };

  // Persistent Real Sessions
  const [sessions, setSessions] = useState<AgentSession[]>(() => {
    try {
      const saved = localStorage.getItem('forge_ade_sessions') || localStorage.getItem('my_ade_sessions');
      if (saved) return JSON.parse(saved);
    } catch {
      // ignore
    }
    return [];
  });

  const [savedSessions, setSavedSessions] = useState<AgentSession[]>([]);

  const [activeSessionId, setActiveSessionId] = useState<string | null>(() => {
    return sessions.length > 0 ? sessions[0].id : null;
  });

  // Load all sessions from disk (~/.forge-ade/sessions/ and workspace/.forge-ade/sessions/)
  const reloadSavedSessions = useCallback(async () => {
    try {
      const diskSessions = await ApiBridge.loadSessionsJsonl(activeWorkspacePath);
      if (diskSessions && diskSessions.length > 0) {
        setSavedSessions(diskSessions);
        setSessions(prev => {
          if (prev.length === 0 && diskSessions.length > 0) {
            return [diskSessions[0]];
          }
          return prev;
        });
      }
    } catch {
      // ignore
    }
  }, [activeWorkspacePath]);

  useEffect(() => {
    reloadSavedSessions();
  }, [reloadSavedSessions]);

  // Sync session changes to localStorage and disk
  useEffect(() => {
    try {
      localStorage.setItem('forge_ade_sessions', JSON.stringify(sessions));
      for (const sess of sessions) {
        if (sess && sess.id) {
          ApiBridge.saveSessionJsonl(sess, activeWorkspacePath);
        }
      }
    } catch {
      // ignore
    }
  }, [sessions, activeWorkspacePath]);

  // ACP & Agent Registry (ForgeADE Internal + Pi, OhMyPi/OMP, OpenCode)
  const [agents, setAgents] = useState<ACPAgent[]>(() => {
    try {
      const saved = localStorage.getItem('forge_ade_agents') || localStorage.getItem('my_ade_agents');
      if (saved) {
        const parsed: ACPAgent[] = JSON.parse(saved);
        const valid = parsed.filter(a => ['agent-internal', 'agent-pi', 'agent-ohmypi', 'agent-opencode'].includes(a.id));
        if (valid.length === 4) {
          return valid.map(a => ({
            ...a,
            name: a.id === 'agent-internal' ? 'ForgeADE Internal' : a.name,
            provider: a.type === 'internal' ? 'custom' : a.provider,
            model: undefined
          }));
        }
      }
    } catch {
      // ignore
    }
    return DEFAULT_AGENTS;
  });

  const [activeAgentId, setActiveAgentIdState] = useState<string>(() => {
    const saved = localStorage.getItem('forge_ade_active_agent_id') || localStorage.getItem('my_ade_active_agent_id');
    if (saved && ['agent-internal', 'agent-pi', 'agent-ohmypi', 'agent-opencode'].includes(saved)) {
      return saved;
    }
    return 'agent-internal';
  });

  const setActiveAgentId = useCallback((id: string) => {
    if (['agent-internal', 'agent-pi', 'agent-ohmypi', 'agent-opencode'].includes(id)) {
      setActiveAgentIdState(id);
      localStorage.setItem('forge_ade_active_agent_id', id);
    }
  }, []);

  const [acpEnabled, setAcpEnabled] = useState<boolean>(true);
  const [privacySettings, setPrivacySettings] = useState<PrivacySettings>(() => {
    try {
      const saved = localStorage.getItem('forge_ade_privacy') || localStorage.getItem('my_ade_privacy');
      if (saved) return JSON.parse(saved);
    } catch {
      // ignore
    }
    return DEFAULT_PRIVACY;
  });

  useEffect(() => {
    localStorage.setItem('forge_ade_privacy', JSON.stringify(privacySettings));
  }, [privacySettings]);

  useEffect(() => {
    localStorage.setItem('forge_ade_agents', JSON.stringify(agents));
  }, [agents]);

  // ACP Handshake Verification
  const verifyAcpConnections = useCallback(async () => {
    try {
      const results = await Promise.all(
        agents.map(async (ag) => {
          const res = await ApiBridge.handshakeACP(ag);
          return {
            id: ag.id,
            status: res.connected ? ('connected' as const) : ('disconnected' as const),
            handshakeError: res.error,
            endpoint: res.endpoint || ag.endpoint
          };
        })
      );
      setAgents(prev => prev.map(a => {
        const match = results.find(r => r.id === a.id);
        if (match) {
          return { ...a, status: match.status, handshakeError: match.handshakeError, endpoint: match.endpoint };
        }
        return a;
      }));
    } catch {
      // ignore
    }
  }, [agents]);

  useEffect(() => {
    verifyAcpConnections();
  }, []);

  // Providers state & Active Model Persistence
  const [providers, setProviders] = useState<LLMProviderConfig[]>(() => {
    try {
      const saved = localStorage.getItem('forge_ade_providers') || localStorage.getItem('my_ade_providers');
      if (saved) return JSON.parse(saved);
    } catch {
      // ignore
    }
    return DEFAULT_PROVIDERS;
  });

  const [currentModel, setCurrentModelState] = useState<string>(() => {
    try {
      return localStorage.getItem('forge_ade_current_model') || localStorage.getItem('my_ade_current_model') || '';
    } catch {
      return '';
    }
  });

  const setCurrentModel = useCallback((model: string) => {
    setCurrentModelState(model);
    try {
      if (model) {
        localStorage.setItem('forge_ade_current_model', model);
      } else {
        localStorage.removeItem('forge_ade_current_model');
      }
    } catch {}
  }, []);

  // Sync currentModel with enabled providers
  useEffect(() => {
    const enabled = providers.filter(p => p.enabled);
    const validModels: string[] = Array.from(new Set(
      enabled.flatMap(p => (p.selectedModels && p.selectedModels.length > 0 ? p.selectedModels : p.models || []))
    ));

    if (validModels.length > 0) {
      if (!currentModel || !validModels.includes(currentModel)) {
        const defaultModel = validModels[0];
        setCurrentModelState(defaultModel);
        try { localStorage.setItem('my_ade_current_model', defaultModel); } catch {}
      }
    }
  }, [providers, currentModel]);

  useEffect(() => {
    localStorage.setItem('my_ade_providers', JSON.stringify(providers));
  }, [providers]);

  const updateProvider = useCallback((id: string, updates: Partial<LLMProviderConfig>) => {
    setProviders(prev => prev.map(p => p.id === id ? { ...p, ...updates } : p));
  }, []);

  const addProvider = useCallback((provider: LLMProviderConfig) => {
    setProviders(prev => [...prev, provider]);
  }, []);

  const deleteProvider = useCallback((id: string) => {
    setProviders(prev => prev.filter(p => p.id !== id));
  }, []);

  const addModelToProvider = useCallback((providerId: string, modelName: string) => {
    setProviders(prev => prev.map(p => {
      if (p.id === providerId && !p.models.includes(modelName)) {
        const nextModels = [...p.models, modelName];
        const nextSelected = [...(p.selectedModels || p.models), modelName];
        return { ...p, models: nextModels, selectedModels: nextSelected };
      }
      return p;
    }));
  }, []);

  const deleteModelFromProvider = useCallback((providerId: string, modelName: string) => {
    setProviders(prev => prev.map(p => {
      if (p.id === providerId) {
        const nextModels = p.models.filter(m => m !== modelName);
        const nextSelected = (p.selectedModels || p.models).filter(m => m !== modelName);
        return { ...p, models: nextModels, selectedModels: nextSelected };
      }
      return p;
    }));
  }, []);

  const toggleModelSelection = useCallback((providerId: string, modelName: string) => {
    setProviders(prev => prev.map(p => {
      if (p.id === providerId) {
        const currentSelected = p.selectedModels || p.models;
        const isSelected = currentSelected.includes(modelName);
        const nextSelected = isSelected
          ? currentSelected.filter(m => m !== modelName)
          : [...currentSelected, modelName];
        return { ...p, selectedModels: nextSelected };
      }
      return p;
    }));
  }, []);

  const fetchProviderModels = useCallback(async (providerId: string) => {
    const prov = providers.find(p => p.id === providerId);
    if (!prov) return;
    const fetched = await ApiBridge.fetchModels(prov.id, prov.baseUrl, prov.apiKey);
    if (fetched.length > 0) {
      setProviders(prev => prev.map(p => {
        if (p.id === providerId) {
          const merged = Array.from(new Set([...p.models, ...fetched]));
          return { ...p, models: merged, selectedModels: merged };
        }
        return p;
      }));
      // Immediately activate the first fetched model
      setCurrentModel(fetched[0]);
    }
  }, [providers, setCurrentModel]);

  // MCPs state
  const [mcps, setMcps] = useState<MCPEntry[]>(() => {
    try {
      const saved = localStorage.getItem('forge_ade_mcps') || localStorage.getItem('my_ade_mcps');
      if (saved) return JSON.parse(saved);
    } catch {}
    return [
      {
        id: 'mcp-1',
        name: 'Filesystem MCP Server',
        command: 'npx -y @modelcontextprotocol/server-filesystem .',
        status: 'running',
        enabled: true,
        version: 'v0.8.2',
        tools: ['read_file', 'batch_edit', 'grep_search', 'directory_tree']
      },
      {
        id: 'mcp-2',
        name: 'GitHub & Git Tools MCP',
        command: 'npx -y @modelcontextprotocol/server-github',
        status: 'running',
        enabled: true,
        version: 'v1.1.0',
        tools: ['create_pr', 'list_issues', 'get_commit_history', 'view_diff']
      }
    ];
  });

  useEffect(() => {
    localStorage.setItem('forge_ade_mcps', JSON.stringify(mcps));
  }, [mcps]);

  const addMcp = useCallback((mcp: MCPEntry) => {
    setMcps(prev => [...prev, mcp]);
  }, []);

  const toggleMcp = useCallback((id: string) => {
    setMcps(prev => prev.map(m => m.id === id ? { ...m, enabled: !m.enabled } : m));
  }, []);

  const deleteMcp = useCallback((id: string) => {
    setMcps(prev => prev.filter(m => m.id !== id));
  }, []);

  // Skills state
  const [skills, setSkills] = useState<SkillEntry[]>(() => {
    try {
      const saved = localStorage.getItem('forge_ade_skills') || localStorage.getItem('my_ade_skills');
      if (saved) return JSON.parse(saved);
    } catch {}
    return [
      {
        id: 'sk-native',
        name: 'Native SDK Discovery & Authoring',
        category: 'Native SDK',
        description: 'Discovery skill for building native desktop applications with TypeScript cores, Native markup, and Zig engine.',
        enabled: true,
        trigger: 'native-sdk'
      },
      {
        id: 'sk-forge',
        name: 'Forge ADE Architecture',
        category: 'Workspace',
        description: 'High performance editor primitives, unified diff engine, Git graph, LSP diagnostics, and ACP subagent bridges.',
        enabled: true,
        trigger: 'forge-ade'
      },
      {
        id: 'sk-1',
        name: 'Clean Architecture & Refactoring',
        category: 'Code Quality',
        description: 'Auto-detects code smells and optimizes design patterns.',
        enabled: true,
        trigger: 'on_refactor'
      },
      {
        id: 'sk-2',
        name: 'Automated Test Runner',
        category: 'Testing',
        description: 'Runs automated test suites in background sandbox.',
        enabled: true,
        trigger: 'on_test_fail'
      }
    ];
  });

  useEffect(() => {
    localStorage.setItem('my_ade_skills', JSON.stringify(skills));
  }, [skills]);

  const addSkill = useCallback((skill: SkillEntry) => {
    setSkills(prev => [...prev, skill]);
  }, []);

  const toggleSkill = useCallback((id: string) => {
    setSkills(prev => prev.map(s => s.id === id ? { ...s, enabled: !s.enabled } : s));
  }, []);

  const deleteSkill = useCallback((id: string) => {
    setSkills(prev => prev.filter(s => s.id !== id));
  }, []);

  // Discovery State (Antigravity, OpenCode, Pi, Claude Code, Codex)
  const [discoveredMcps, setDiscoveredMcps] = useState<any[]>([]);
  const [discoveredSkills, setDiscoveredSkills] = useState<any[]>([]);
  const [isDiscovering, setIsDiscovering] = useState<boolean>(false);

  const runDiscovery = useCallback(async () => {
    setIsDiscovering(true);
    try {
      const [mcpsRes, skillsRes] = await Promise.all([
        ApiBridge.discoverMcps(),
        ApiBridge.discoverSkills()
      ]);
      setDiscoveredMcps(mcpsRes);
      setDiscoveredSkills(skillsRes);
    } finally {
      setIsDiscovering(false);
    }
  }, []);

  // Run discovery on mount
  useEffect(() => {
    runDiscovery();
  }, [runDiscovery]);

  const importDiscoveredMcp = useCallback((disc: any) => {
    setMcps(prev => {
      const exists = prev.some(m => m.name === disc.name || m.command === disc.command);
      if (exists) return prev;
      return [...prev, {
        id: `mcp-${Date.now()}`,
        name: disc.name,
        command: disc.command,
        status: 'connected',
        enabled: true,
        origin: disc.origin,
        tools: disc.tools || ['tools']
      }];
    });
  }, []);

  const importDiscoveredSkill = useCallback((disc: any) => {
    setSkills(prev => {
      const exists = prev.some(s => s.trigger === disc.trigger || s.name === disc.name);
      if (exists) return prev;
      return [...prev, {
        id: `skill-${Date.now()}`,
        name: disc.name,
        category: disc.category || 'Discovered',
        description: disc.description,
        trigger: disc.trigger,
        instructions: disc.instructions,
        enabled: true,
        origin: disc.origin
      }];
    });
  }, []);

  const activeAgent = agents.find(a => a.id === activeAgentId) || agents[0];

  const toggleAgentEnabled = useCallback((id: string) => {
    setAgents(prev => prev.map(a => a.id === id ? { ...a, enabled: !a.enabled } : a));
  }, []);

  const updateAgentConfig = useCallback((id: string, updates: Partial<ACPAgent>) => {
    setAgents(prev => prev.map(a => a.id === id ? { ...a, ...updates } : a));
  }, []);

  const addAgent = useCallback((agent: ACPAgent) => {
    setAgents(prev => [...prev, agent]);
  }, []);

  // Diffs & Review System
  const [diffs, setDiffs] = useState<FileDiff[]>([]);
  const [activeDiff, setActiveDiff] = useState<FileDiff | null>(null);

  const addDiff = useCallback((diff: FileDiff) => {
    setDiffs(prev => [diff, ...prev.filter(d => d.filePath !== diff.filePath)]);
  }, []);

  // Git State
  const [gitBranch, setGitBranch] = useState<string>('main');
  const [gitFiles, setGitFiles] = useState<Array<{ path: string; status: string }>>([]);
  const [gitCommits, setGitCommits] = useState<any[]>([]);

  const refreshGitStatus = useCallback(async () => {
    const result = await ApiBridge.gitStatus(activeWorkspacePath);
    setGitBranch(result.branch);
    setGitFiles(result.files);
  }, [activeWorkspacePath]);

  const refreshGitLog = useCallback(async () => {
    const commits = await ApiBridge.gitLog(activeWorkspacePath);
    setGitCommits(commits);
  }, [activeWorkspacePath]);

  // File tree & editor tabs
  const [files, setFiles] = useState<FileItem[]>([]);
  const [openTabs, setOpenTabs] = useState<EditorTab[]>([]);
  const [activeTabId, setActiveTabId] = useState<string | null>(null);
  const [selectedFile, setSelectedFile] = useState<FileItem | null>(null);
  const [isSplitEditor, setIsSplitEditor] = useState<boolean>(false);

  // Navigation & Activities
  const [activeActivity, setActiveActivity] = useState<ActivityBarItem>('explorer');

  // Terminal state
  const [terminalTabs] = useState<TerminalTab[]>([
    { id: 'term-problems', title: 'Problems', type: 'problems' },
    { id: 'term-output', title: 'Output', type: 'output' },
    { id: 'term-debug', title: 'Debug Console', type: 'debug' },
    { id: 'term-terminal-1', title: 'Terminal', type: 'terminal' },
    { id: 'term-ports', title: 'Ports', type: 'ports' }
  ]);
  const [activeTerminalTabId, setActiveTerminalTabId] = useState<string>('term-terminal-1');
  const [isTerminalOpen, setIsTerminalOpen] = useState<boolean>(false);

  // Quick Action Drawer
  const [isRightActionDrawerOpen, setIsRightActionDrawerOpen] = useState<boolean>(true);

  // Modals & Palette
  const [isCommandPaletteOpen, setIsCommandPaletteOpen] = useState<boolean>(false);
  const [isCustomizationsModalOpen, setIsCustomizationsModalOpen] = useState<boolean>(false);
  const [isSettingsModalOpen, setIsSettingsModalOpen] = useState<boolean>(false);

  // Single Pane Settings Section
  const [settingsActiveSection, setSettingsActiveSection] = useState<string>('agents');

  // Agent execution mode & Engine
  const [agentExecutionMode, setAgentExecutionMode] = useState<AgentExecutionMode>('code');
  const [agentEngine, setAgentEngine] = useState<AgentEngineType>('pi');

  // Real Dynamic Context Usage calculation
  const activeSession = sessions.find(s => s.id === activeSessionId) || sessions[0];
  const contextUsage = useMemo(() => {
    let chatCharCount = 0;
    if (activeSession?.messages) {
      chatCharCount = activeSession.messages.reduce((acc, m) => acc + (m.content?.length || 0), 0);
    }
    let openFilesCharCount = 0;
    if (openTabs) {
      openFilesCharCount = openTabs.reduce((acc, t) => acc + (t.content?.length || 0), 0);
    }
    const chatTokens = Math.round(chatCharCount / 4);
    const fileTokens = Math.round(openFilesCharCount / 4);
    const usedTokens = chatTokens + fileTokens;
    const maxTokens = 128000;
    const percent = usedTokens === 0 ? 0 : Math.min(100, Math.max(1, Math.round((usedTokens / maxTokens) * 100)));
    return { usedTokens, maxTokens, percent };
  }, [activeSession, openTabs]);

  // Diagnostics (LSP)
  const [diagnostics] = useState<LSPDiagnostic[]>([]);

  const engineRef = useRef<AgentEngine>(new AgentEngine());

  // Sync theme
  useEffect(() => {
    localStorage.setItem('forge-ade-theme', theme);
    useUIStore.getState().setTheme(theme);
    if (theme === 'dark') {
      document.documentElement.classList.add('dark');
      document.documentElement.classList.remove('light');
      document.body.classList.add('dark');
      document.body.classList.remove('light');
    } else {
      document.documentElement.classList.remove('dark');
      document.documentElement.classList.add('light');
      document.body.classList.remove('dark');
      document.body.classList.add('light');
    }
  }, [theme]);

  const toggleTheme = useCallback(() => {
    setTheme(prev => (prev === 'dark' ? 'light' : 'dark'));
  }, []);

  // Refresh files from disk
  const refreshFiles = useCallback(async () => {
    const currentPath = activeWorkspacePath || (await ApiBridge.getWorkspaceInfo()).cwd;
    if (currentPath) {
      const realTree = await ApiBridge.readDirectoryTree(currentPath);
      setFiles(realTree);
    }
  }, [activeWorkspacePath]);

  // Initial load
  useEffect(() => {
    const initWorkspace = async () => {
      const targetPath = activeWorkspacePath;
      if (targetPath) {
        const realTree = await ApiBridge.readDirectoryTree(targetPath);
        setFiles(realTree);
        refreshGitStatus();
        refreshGitLog();
      }
    };
    initWorkspace();
  }, [activeWorkspacePath]);

  // Helper to find file by path in tree
  const findFileInTree = useCallback((items: FileItem[], path: string): FileItem | null => {
    const clean = path.replace(/^\/+/, '');
    for (const item of items) {
      const itemClean = item.path.replace(/^\/+/, '');
      if (itemClean === clean || item.path === path || item.name === path || item.path.endsWith(clean)) {
        return item;
      }
      if (item.children) {
        const found = findFileInTree(item.children, path);
        if (found) return found;
      }
    }
    return null;
  }, []);

  // Open / Pick folder
  const openFolder = useCallback(async (customPath?: string) => {
    if (customPath) {
      setActiveWorkspacePath(customPath);
      const realTree = await ApiBridge.readDirectoryTree(customPath);
      setFiles(realTree);
      setOpenTabs([]);
      return;
    }
    setIsFolderModalOpen(true);
  }, [setActiveWorkspacePath]);

  const openFileInEditor = useCallback(async (filePath: string, line?: number, column?: number) => {
    let file = findFileInTree(files, filePath);
    let content = file?.content || '';

    if (!content) {
      content = await ApiBridge.readFile(filePath);
    }

    const fileName = filePath.split('/').pop() || filePath;
    const fileItem: FileItem = file || {
      id: `file-${Date.now()}`,
      name: fileName,
      path: filePath,
      type: 'file',
      content,
      isModified: false
    };

    setSelectedFile(fileItem);

    const existingTab = openTabs.find(t => t.filePath === filePath && t.type === 'code');
    if (existingTab) {
      if (line !== undefined) {
        setOpenTabs(prev => prev.map(t => t.id === existingTab.id ? { ...t, line, column } : t));
      }
      setActiveTabId(existingTab.id);
    } else {
      const newTab: EditorTab = {
        id: `tab-${fileItem.id}-${Date.now()}`,
        fileId: fileItem.id,
        fileName: fileItem.name,
        filePath: fileItem.path,
        content,
        type: 'code',
        line,
        column
      };
      setOpenTabs(prev => [...prev, newTab]);
      setActiveTabId(newTab.id);
    }

    setMode('editor');
  }, [files, findFileInTree, openTabs]);

  const updateFolderChildren = useCallback((folderPath: string, children: FileItem[]) => {
    setFiles(prevFiles => {
      const updateNode = (items: FileItem[]): FileItem[] => {
        return items.map(item => {
          if (item.path === folderPath || item.id === folderPath || item.name === folderPath) {
            return { ...item, children };
          }
          if (item.children) {
            return { ...item, children: updateNode(item.children) };
          }
          return item;
        });
      };
      return updateNode(prevFiles);
    });
  }, []);

  const openSettingsTab = useCallback((section?: string) => {
    if (section) {
      setSettingsActiveSection(section);
    }
    const settingsTabId = 'tab-forge-settings';
    const existing = openTabs.find(t => t.id === settingsTabId);
    if (existing) {
      setActiveTabId(existing.id);
    } else {
      const newTab: EditorTab = {
        id: settingsTabId,
        fileId: 'settings-virtual-file',
        fileName: 'Forge Settings',
        filePath: 'Settings/Preferences',
        type: 'settings',
        content: ''
      };
      setOpenTabs(prev => [...prev, newTab]);
      setActiveTabId(settingsTabId);
    }
    setMode('editor');
  }, [openTabs, setMode]);

  const openDiffInEditor = useCallback((diff: FileDiff) => {
    setActiveDiff(diff);
    // Register in diffs so EditorView's `diffs.find(d => d.id === tab.diffId)`
    // resolves — without this, git-sourced diff tabs silently fall through to
    // the code editor (agent diffs only worked because they were pre-registered).
    setDiffs(prev => {
      const idx = prev.findIndex(d => d.id === diff.id);
      if (idx === -1) return [...prev, diff];
      const next = [...prev];
      next[idx] = diff;
      return next;
    });
    const diffTabId = `tab-diff-${diff.id}`;
    const existing = openTabs.find(t => t.id === diffTabId);
    if (existing) {
      setActiveTabId(diffTabId);
    } else {
      const newTab: EditorTab = {
        id: diffTabId,
        fileId: `file-diff-${diff.id}`,
        fileName: `Diff: ${diff.fileName}`,
        filePath: diff.filePath,
        type: 'diff',
        diffId: diff.id,
        content: diff.modifiedContent
      };
      setOpenTabs(prev => [...prev, newTab]);
      setActiveTabId(diffTabId);
    }
    setMode('editor');
  }, [openTabs]);

  // Full git graph pane in the editor area (lanes + commit detail).
  const openGitGraphPane = useCallback(() => {
    const graphTabId = 'tab-git-graph';
    const existing = openTabs.find(t => t.id === graphTabId);
    if (existing) {
      setActiveTabId(graphTabId);
    } else {
      const newTab: EditorTab = {
        id: graphTabId,
        fileId: 'file-git-graph',
        fileName: 'Git Graph',
        filePath: '',
        type: 'git-graph',
        content: ''
      };
      setOpenTabs(prev => [...prev, newTab]);
      setActiveTabId(graphTabId);
    }
    setMode('editor');
  }, [openTabs]);

  const closeTab = useCallback((tabId: string) => {
    setOpenTabs(prev => {
      const nextTabs = prev.filter(t => t.id !== tabId);
      if (activeTabId === tabId && nextTabs.length > 0) {
        setActiveTabId(nextTabs[nextTabs.length - 1].id);
      }
      return nextTabs;
    });
  }, [activeTabId]);

  const updateFileContent = useCallback(async (fileId: string, newContent: string) => {
    const targetTab = openTabs.find(t => t.fileId === fileId || t.filePath === fileId);
    const targetPath = targetTab ? targetTab.filePath : fileId;

    await ApiBridge.writeFile(targetPath, newContent);

    const updateRecursive = (items: FileItem[]): FileItem[] => {
      return items.map(item => {
        if (item.id === fileId || item.path === fileId || item.path === targetPath) {
          return { ...item, content: newContent, isModified: true };
        }
        if (item.children) {
          return { ...item, children: updateRecursive(item.children) };
        }
        return item;
      });
    };

    setFiles(prev => updateRecursive(prev));
    setOpenTabs(prev => prev.map(t => {
      if (t.fileId === fileId || t.filePath === fileId || t.filePath === targetPath) {
        return { ...t, content: newContent };
      }
      return t;
    }));
  }, [openTabs]);

  const createFile = useCallback(async (filePath: string, content = '') => {
    const fullPath = filePath.startsWith('/') || filePath.includes(':') 
      ? filePath 
      : `${activeWorkspacePath}/${filePath.replace(/^\/+/, '')}`;

    await ApiBridge.createFile(fullPath, content);
    await refreshFiles();
    await openFileInEditor(fullPath);
  }, [activeWorkspacePath, refreshFiles, openFileInEditor]);

  const createFolder = useCallback(async (folderPath: string) => {
    const fullPath = folderPath.startsWith('/') || folderPath.includes(':') 
      ? folderPath 
      : `${activeWorkspacePath}/${folderPath.replace(/^\/+/, '')}`;

    await ApiBridge.createFile(fullPath, '');
    await refreshFiles();
  }, [activeWorkspacePath, refreshFiles]);

  const deleteFile = useCallback(async (filePath: string) => {
    await ApiBridge.deleteFile(filePath);
    await refreshFiles();
    setOpenTabs(prev => prev.filter(t => t.filePath !== filePath));
  }, [refreshFiles]);

  const renameFile = useCallback(async (oldPath: string, newPath: string) => {
    await ApiBridge.renameFile(oldPath, newPath);
    await refreshFiles();
    const newName = newPath.split('/').pop() || newPath;
    setOpenTabs(prev => prev.map(t => {
      if (t.filePath === oldPath) {
        return { ...t, filePath: newPath, fileName: newName };
      }
      return t;
    }));
  }, [refreshFiles]);

  const acceptDiff = useCallback(async (diffId: string) => {
    const targetDiff = diffs.find(d => d.id === diffId);
    // Git review diffs carry no proposed content (they show what is already on
    // disk) — writing their empty modifiedContent would blank the file.
    if (!targetDiff || !targetDiff.modifiedContent) return;

    await ApiBridge.writeFile(targetDiff.filePath, targetDiff.modifiedContent);
    await refreshFiles();

    setDiffs(prev => prev.map(d => d.id === diffId ? { ...d, status: 'accepted' } : d));
  }, [diffs, refreshFiles]);

  const rejectDiff = useCallback((diffId: string) => {
    setDiffs(prev => prev.map(d => d.id === diffId ? { ...d, status: 'rejected' } : d));
  }, []);

  const createNewSession = useCallback((initialPrompt?: string, selectedAgentId?: string) => {
    const newId = `session-${Date.now()}`;
    const targetAgentId = selectedAgentId || activeAgentId;
    const agentObj = agents.find(a => a.id === targetAgentId) || activeAgent;

    const newSession: AgentSession = {
      id: newId,
      title: initialPrompt ? formatSmartSessionTitle(initialPrompt) : 'New Session',
      status: initialPrompt ? 'running' : 'idle',
      createdAt: 'Just now',
      updatedAt: 'Just now',
      model: agentObj.model || currentModel,
      agentId: agentObj.id,
      workspacePath: activeWorkspacePath,
      messages: initialPrompt
        ? [
            {
              id: `msg-user-${Date.now()}`,
              role: 'user',
              content: initialPrompt,
              timestamp: 'Just now'
            },
            {
              id: `msg-agent-${Date.now()}`,
              role: 'agent',
              agentId: agentObj.id,
              agentName: agentObj.name,
              content: '',
              timestamp: 'Just now',
              isThinking: true,
              thoughts: []
            }
          ]
        : [],
      diffs: []
    };

    setSessions(prev => [newSession, ...prev]);
    setActiveSessionId(newId);
    setMode('agent');

    if (initialPrompt) {
      engineRef.current.runSession(
        initialPrompt,
        agentObj,
        {
          files,
          workspacePath: activeWorkspacePath,
          updateFileContent,
          createFile,
          activeModel: currentModel,
          messages: newSession.messages
        },
        {
          onThought: (thought) => {
            setSessions(prev => prev.map(s => {
              if (s.id !== newId) return s;
              const msgs = s.messages.map((m, idx) => {
                if (idx === s.messages.length - 1 && m.role === 'agent') {
                  const thoughts = m.thoughts || [];
                  const existingIdx = thoughts.findIndex(t => t.id === thought.id);
                  const updatedThoughts = existingIdx >= 0
                    ? thoughts.map((t, i) => i === existingIdx ? { ...t, ...thought } : t)
                    : [...thoughts, thought];
                  return { ...m, thoughts: updatedThoughts };
                }
                return m;
              });
              return { ...s, messages: msgs };
            }));
          },
          onToolStart: (tool) => {
            setSessions(prev => prev.map(s => {
              if (s.id !== newId) return s;
              const msgs = s.messages.map((m, idx) => {
                if (idx === s.messages.length - 1 && m.role === 'agent') {
                  const tools = m.toolExecutions || [];
                  const existingIdx = tools.findIndex(t => t.id === tool.id);
                  const updatedTools = existingIdx >= 0
                    ? tools.map((t, i) => i === existingIdx ? { ...t, ...tool } : t)
                    : [...tools, tool];
                  return { ...m, toolExecutions: updatedTools };
                }
                return m;
              });
              return { ...s, messages: msgs };
            }));
          },
          onToolComplete: (tool) => {
            setSessions(prev => prev.map(s => {
              if (s.id !== newId) return s;
              const msgs = s.messages.map((m, idx) => {
                if (idx === s.messages.length - 1 && m.role === 'agent') {
                  const tools = m.toolExecutions || [];
                  const updated = tools.some(t => t.id === tool.id)
                    ? tools.map(t => t.id === tool.id ? { ...t, ...tool } : t)
                    : [...tools, tool];
                  return { ...m, toolExecutions: updated };
                }
                return m;
              });
              return { ...s, messages: msgs };
            }));
          },
          onContentChunk: (chunk) => {
            setSessions(prev => prev.map(s => {
              if (s.id !== newId) return s;
              const msgs = s.messages.map((m, idx) => {
                if (idx === s.messages.length - 1 && m.role === 'agent') {
                  return { ...m, content: (m.content || '') + chunk, isThinking: false };
                }
                return m;
              });
              return { ...s, messages: msgs };
            }));
          },
          onDiffCreated: (diff) => {
            addDiff(diff);
            setSessions(prev => prev.map(s => {
              if (s.id !== newId) return s;
              return { ...s, diffs: [...(s.diffs || []), diff] };
            }));
          },
          onFinish: (finalContent) => {
            setSessions(prev => {
              const updated = prev.map(s => {
                if (s.id !== newId) return s;
                const msgs = s.messages.map((m, idx) => {
                  if (idx === s.messages.length - 1 && m.role === 'agent') {
                    return { ...m, content: finalContent, isThinking: false };
                  }
                  return m;
                });
                const finishedSession = { ...s, status: 'completed' as const, messages: msgs };
                // Persist session to JSONL file
                ApiBridge.saveSessionJsonl(finishedSession, activeWorkspacePath);
                return finishedSession;
              });
              return updated;
            });
          },
          onError: (err) => {
            setSessions(prev => prev.map(s => {
              if (s.id !== newId) return s;
              const msgs = s.messages.map((m, idx) => {
                if (idx === s.messages.length - 1 && m.role === 'agent') {
                  return { ...m, content: `⚠️ **Error**: ${err}`, isThinking: false };
                }
                return m;
              });
              return { ...s, status: 'idle' as const, messages: msgs };
            }));
          }
        }
      );
    }
  }, [activeAgentId, agents, activeAgent, currentModel, activeWorkspacePath, files, updateFileContent, createFile, addDiff]);

  const sendAgentPrompt = useCallback((promptText: string) => {
    if (!promptText.trim()) return;

    if (!activeSessionId) {
      createNewSession(promptText);
      return;
    }

    const agentObj = agents.find(a => a.id === activeAgentId) || activeAgent;

    const userMsg: AgentMessage = {
      id: `msg-${Date.now()}`,
      role: 'user',
      content: promptText,
      timestamp: 'Just now'
    };

    const agentPlaceholder: AgentMessage = {
      id: `msg-${Date.now() + 1}`,
      role: 'agent',
      agentId: agentObj.id,
      agentName: agentObj.name,
      content: '',
      timestamp: 'Just now',
      isThinking: true,
      thoughts: []
    };

    const currentSession = sessions.find(s => s.id === activeSessionId);
    const sessionHistory = currentSession ? [...currentSession.messages, userMsg] : [userMsg];

    setSessions(prev => prev.map(s => {
      if (s.id !== activeSessionId) return s;
      const isDefaultTitle = !s.title || s.title === 'New Session' || s.title.startsWith('session-');
      const updatedTitle = isDefaultTitle ? formatSmartSessionTitle(promptText) : s.title;
      return {
        ...s,
        title: updatedTitle,
        status: 'running',
        updatedAt: 'Just now',
        messages: [...s.messages, userMsg, agentPlaceholder]
      };
    }));

    engineRef.current.runSession(
      promptText,
      agentObj,
      {
        files,
        workspacePath: activeWorkspacePath,
        updateFileContent,
        createFile,
        activeModel: currentModel,
        messages: sessionHistory
      },
      {
        onThought: (thought) => {
          setSessions(prev => prev.map(s => {
            if (s.id !== activeSessionId) return s;
            const msgs = s.messages.map((m, idx) => {
              if (idx === s.messages.length - 1 && m.role === 'agent') {
                const thoughts = m.thoughts || [];
                const existingIdx = thoughts.findIndex(t => t.id === thought.id);
                const updatedThoughts = existingIdx >= 0
                  ? thoughts.map((t, i) => i === existingIdx ? { ...t, ...thought } : t)
                  : [...thoughts, thought];
                return { ...m, thoughts: updatedThoughts };
              }
              return m;
            });
            return { ...s, messages: msgs };
          }));
        },
        onToolStart: (tool) => {
          setSessions(prev => prev.map(s => {
            if (s.id !== activeSessionId) return s;
            const msgs = s.messages.map((m, idx) => {
              if (idx === s.messages.length - 1 && m.role === 'agent') {
                const tools = m.toolExecutions || [];
                const existingIdx = tools.findIndex(t => t.id === tool.id);
                const updatedTools = existingIdx >= 0
                  ? tools.map((t, i) => i === existingIdx ? { ...t, ...tool } : t)
                  : [...tools, tool];
                return { ...m, toolExecutions: updatedTools };
              }
              return m;
            });
            return { ...s, messages: msgs };
          }));
        },
        onToolComplete: (tool) => {
          setSessions(prev => prev.map(s => {
            if (s.id !== activeSessionId) return s;
            const msgs = s.messages.map((m, idx) => {
              if (idx === s.messages.length - 1 && m.role === 'agent') {
                const tools = m.toolExecutions || [];
                const updated = tools.some(t => t.id === tool.id)
                  ? tools.map(t => t.id === tool.id ? { ...t, ...tool } : t)
                  : [...tools, tool];
                return { ...m, toolExecutions: updated };
              }
              return m;
            });
            return { ...s, messages: msgs };
          }));
        },
        onContentChunk: (chunk) => {
          setSessions(prev => prev.map(s => {
            if (s.id !== activeSessionId) return s;
            const msgs = s.messages.map((m, idx) => {
              if (idx === s.messages.length - 1 && m.role === 'agent') {
                return { ...m, content: (m.content || '') + chunk, isThinking: false };
              }
              return m;
            });
            return { ...s, messages: msgs };
          }));
        },
        onDiffCreated: (diff) => {
          addDiff(diff);
          setSessions(prev => prev.map(s => {
            if (s.id !== activeSessionId) return s;
            return { ...s, diffs: [...(s.diffs || []), diff] };
          }));
        },
        onFinish: (finalContent) => {
          setSessions(prev => {
            const updated = prev.map(s => {
              if (s.id !== activeSessionId) return s;
              const msgs = s.messages.map((m, idx) => {
                if (idx === s.messages.length - 1 && m.role === 'agent') {
                  return { ...m, content: finalContent, isThinking: false };
                }
                return m;
              });
              const finishedSession = { ...s, status: 'completed' as const, messages: msgs };
              // Persist session to JSONL file
              ApiBridge.saveSessionJsonl(finishedSession, activeWorkspacePath);
              return finishedSession;
            });
            return updated;
          });
        },
        onError: (err) => {
          setSessions(prev => prev.map(s => {
            if (s.id !== activeSessionId) return s;
            const msgs = s.messages.map((m, idx) => {
              if (idx === s.messages.length - 1 && m.role === 'agent') {
                return { ...m, content: `⚠️ **Error**: ${err}`, isThinking: false };
              }
              return m;
            });
            return { ...s, status: 'idle', messages: msgs };
          }));
        }
      }
    );
  }, [activeSessionId, activeAgentId, agents, activeAgent, files, activeWorkspacePath, updateFileContent, createFile, addDiff, createNewSession]);

  const stopAgentExecution = useCallback(() => {
    engineRef.current.abort();
    if (!activeSessionId) return;
    setSessions(prev => prev.map(s => {
      if (s.id !== activeSessionId) return s;
      const msgs = [...s.messages];
      const last = msgs[msgs.length - 1];
      if (last && last.role === 'agent') {
        last.isThinking = false;
        last.content += ' (Execution stopped by user)';
      }
      return { ...s, status: 'idle', messages: msgs };
    }));
  }, [activeSessionId]);

  const openSessionFromHistory = useCallback((sessionToOpen: AgentSession) => {
    setSessions(prev => {
      const exists = prev.some(s => s.id === sessionToOpen.id);
      if (exists) return prev;
      return [sessionToOpen, ...prev];
    });
    setActiveSessionId(sessionToOpen.id);
    setMode('agent');
  }, []);

  const deleteSessionPermanently = useCallback(async (id: string) => {
    await ApiBridge.deleteSessionJsonl(id, activeWorkspacePath);
    setSavedSessions(prev => prev.filter(s => s.id !== id));
    setSessions(prev => {
      const next = prev.filter(s => s.id !== id);
      if (activeSessionId === id) {
        setActiveSessionId(next.length > 0 ? next[0].id : null);
      }
      return next;
    });
  }, [activeWorkspacePath, activeSessionId]);

  const deleteSession = useCallback((id: string) => {
    setSessions(prev => {
      const next = prev.filter(s => s.id !== id);
      if (activeSessionId === id) {
        setActiveSessionId(next.length > 0 ? next[0].id : null);
      }
      return next;
    });
  }, [activeSessionId]);

  // Global Keyboard Shortcuts
  useEffect(() => {
    const handleKeyDown = (e: KeyboardEvent) => {
      if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault();
        setIsCommandPaletteOpen(true);
      } else if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'p') {
        e.preventDefault();
        setIsCommandPaletteOpen(true);
      } else if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 't') {
        e.preventDefault();
        createNewSession();
      } else if (e.altKey && e.key.toLowerCase() === 't') {
        e.preventDefault();
        createNewSession();
      } else if ((e.metaKey || e.ctrlKey) && e.key === '1') {
        e.preventDefault();
        setMode('agent');
      } else if ((e.metaKey || e.ctrlKey) && e.key === '2') {
        e.preventDefault();
        setMode('editor');
      } else if ((e.metaKey || e.ctrlKey) && e.key === '/') {
        e.preventDefault();
        toggleTheme();
      } else if ((e.metaKey || e.ctrlKey) && e.key === ',') {
        e.preventDefault();
        openSettingsTab('agents');
      } else if ((e.metaKey || e.ctrlKey) && e.key.toLowerCase() === 'o') {
        e.preventDefault();
        openFolder();
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [createNewSession, toggleTheme, openSettingsTab, openFolder]);

  return (
    <WorkspaceContext.Provider
      value={{
        mode,
        setMode,
        theme,
        setTheme,
        toggleTheme,
        recentWorkspaces,
        addRecentWorkspace,
        removeRecentWorkspace,
        clearRecentWorkspaces,
        closeWorkspace,
        activeSessionId,
        setActiveSessionId,
        sessions,
        activeSession,
        agents,
        activeAgentId,
        setActiveAgentId,
        activeAgent,
        toggleAgentEnabled,
        updateAgentConfig,
        addAgent,
        acpEnabled,
        setAcpEnabled,
        privacySettings,
        setPrivacySettings,
        providers,
        updateProvider,
        addProvider,
        deleteProvider,
        addModelToProvider,
        deleteModelFromProvider,
        toggleModelSelection,
        fetchProviderModels,
        mcps,
        addMcp,
        toggleMcp,
        deleteMcp,
        skills,
        addSkill,
        toggleSkill,
        deleteSkill,
        discoveredMcps,
        discoveredSkills,
        isDiscovering,
        runDiscovery,
        importDiscoveredMcp,
        importDiscoveredSkill,
        diffs,
        activeDiff,
        setActiveDiff,
        acceptDiff,
        rejectDiff,
        addDiff,
        openDiffInEditor,
        openGitGraphPane,
        gitBranch,
        gitFiles,
        gitCommits,
        refreshGitStatus,
        refreshGitLog,
        files,
        openTabs,
        activeTabId,
        selectedFile,
        isSplitEditor,
        setIsSplitEditor,
        createFile,
        createFolder,
        deleteFile,
        renameFile,
        openFolder,
        refreshFiles,
        isFolderModalOpen,
        setIsFolderModalOpen,
        activeActivity,
        setActiveActivity,
        terminalTabs,
        activeTerminalTabId,
        setActiveTerminalTabId,
        isTerminalOpen,
        setIsTerminalOpen,
        isRightActionDrawerOpen,
        setIsRightActionDrawerOpen,
        agentExecutionMode,
        setAgentExecutionMode,
        agentEngine,
        setAgentEngine,
        contextUsage,
        currentModel,
        setCurrentModel,
        isCommandPaletteOpen,
        setIsCommandPaletteOpen,
        isCustomizationsModalOpen,
        setIsCustomizationsModalOpen,
        isSettingsModalOpen,
        setIsSettingsModalOpen,
        settingsActiveSection,
        setSettingsActiveSection,
        openFileInEditor,
        updateFolderChildren,
        openSettingsTab,
        closeTab,
        setActiveTabId,
        updateFileContent,
        createNewSession,
        sendAgentPrompt,
        stopAgentExecution,
        deleteSession,
        savedSessions,
        openSessionFromHistory,
        deleteSessionPermanently,
        reloadSavedSessions,
        activeWorkspacePath,
        setActiveWorkspacePath,
        diagnostics
      }}
    >
      {children}
    </WorkspaceContext.Provider>
  );
};

export const useWorkspace = () => {
  const context = useContext(WorkspaceContext);
  if (!context) {
    throw new Error('useWorkspace must be used within a WorkspaceProvider');
  }
  return context;
};
