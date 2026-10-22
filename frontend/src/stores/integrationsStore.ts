import { create } from 'zustand';
import {
  MCPEntry,
  SkillEntry,
  CreateSkillRequest,
  PluginInfo,
  CreatePluginRequest,
  AgentMemoryEntry,
  CustomSlashCommand,
  AgentHookConfig,
  BrowserUseSettings,
  ComputerUseSettings
} from '../types';
import { ApiBridge } from '../services/apiBridge';
import {
  SaveMCPServer,
  DeleteMCPServer,
  ListMCPServers,
  ListConnectedMCPTools,
  EventsOn,
  type Automation as ZAutomation,
  type AutomationSaveInput as ZAutomationSaveInput
} from '../lib/wails';
import {
  DEFAULT_SLASH_COMMANDS,
  DEFAULT_HOOKS,
  DEFAULT_BROWSER_SETTINGS,
  DEFAULT_COMPUTER_SETTINGS
} from './workspaceConstants';

export interface IntegrationsState {
  // MCP
  mcps: MCPEntry[];
  setMcps: (mcps: MCPEntry[] | ((prev: MCPEntry[]) => MCPEntry[])) => void;
  fetchBackendMCPServers: () => Promise<void>;
  addMcp: (mcp: MCPEntry) => void;
  toggleMcp: (id: string) => void;
  deleteMcp: (id: string) => void;

  // Skills
  skills: SkillEntry[];
  setSkills: (skills: SkillEntry[] | ((prev: SkillEntry[]) => SkillEntry[])) => void;
  addSkill: (skill: SkillEntry) => void;
  toggleSkill: (id: string) => void;
  deleteSkill: (id: string) => void;
  fetchBackendSkills: () => Promise<void>;
  createBackendSkill: (req: CreateSkillRequest) => Promise<any>;
  deleteBackendSkill: (name: string) => Promise<void>;
  reloadSkills: () => Promise<void>;

  // Plugins
  plugins: PluginInfo[];
  fetchPlugins: () => Promise<void>;
  createPlugin: (req: CreatePluginRequest) => Promise<PluginInfo | null>;
  togglePlugin: (id: string, enabled?: boolean) => Promise<void>;
  deletePlugin: (id: string) => Promise<void>;
  reloadPlugins: () => Promise<void>;

  // Discovery
  discoveredMcps: any[];
  discoveredSkills: any[];
  isDiscovering: boolean;
  runDiscovery: () => Promise<void>;
  importDiscoveredMcp: (disc: any) => void;
  importDiscoveredSkill: (disc: any) => void;

  // Memory
  memories: AgentMemoryEntry[];
  fetchMemories: () => Promise<void>;
  saveMemory: (entry: AgentMemoryEntry) => Promise<void>;
  deleteMemory: (id: string) => Promise<void>;
  reloadMemories: () => Promise<void>;

  // Automations
  automations: ZAutomation[];
  automationsLoading: boolean;
  fetchAutomations: () => Promise<void>;
  saveAutomation: (automation: ZAutomationSaveInput) => Promise<ZAutomation>;
  deleteAutomation: (id: string) => Promise<void>;
  setAutomationEnabled: (id: string, enabled: boolean) => Promise<void>;

  // Custom Slash Commands
  customCommands: CustomSlashCommand[];
  addCustomCommand: (cmd: CustomSlashCommand) => void;
  updateCustomCommand: (id: string, updates: Partial<CustomSlashCommand>) => void;
  deleteCustomCommand: (id: string) => void;
  toggleCustomCommand: (id: string) => void;

  // Hooks
  hooks: AgentHookConfig[];
  updateHook: (id: string, updates: Partial<AgentHookConfig>) => void;
  toggleHook: (id: string) => void;
  addHook: (hook: AgentHookConfig) => void;
  deleteHook: (id: string) => void;

  // Browser & Computer Use Settings
  browserSettings: BrowserUseSettings;
  updateBrowserSettings: (updates: Partial<BrowserUseSettings>) => void;
  computerSettings: ComputerUseSettings;
  updateComputerSettings: (updates: Partial<ComputerUseSettings>) => void;

  // Lifecycle subscription
  initIntegrationsListeners: () => () => void;
}

export const useIntegrationsStore = create<IntegrationsState>((set, get) => ({
  mcps: [],
  setMcps: mcpsOrFn =>
    set(state => ({
      mcps: typeof mcpsOrFn === 'function' ? mcpsOrFn(state.mcps) : mcpsOrFn
    })),

  fetchBackendMCPServers: async () => {
    try {
      const [servers, tools] = await Promise.all([
        ListMCPServers().catch(() => []),
        ListConnectedMCPTools().catch(() => [])
      ]);
      const toolMap = new Map<string, string[]>();
      for (const t of tools) {
        const sName = t.server_name || (t as any).serverName || '';
        if (sName) {
          const arr = toolMap.get(sName) || [];
          arr.push(t.name);
          toolMap.set(sName, arr);
        }
      }
      if (Array.isArray(servers)) {
        set({
          mcps: servers.map((s: any) => {
            const sTools = toolMap.get(s.name) || [];
            const cmdStr =
              s.args && s.args.length > 0 ? `${s.command} ${s.args.join(' ')}` : s.command;
            return {
              id: s.name,
              name: s.name,
              command: cmdStr,
              args: s.args,
              env: s.env,
              url: s.url,
              type: s.type,
              status:
                s.status === 'connected' || sTools.length > 0
                  ? 'connected'
                  : s.status === 'error'
                  ? 'error'
                  : s.enabled
                  ? 'running'
                  : 'standby',
              enabled: s.enabled !== false,
              tools: sTools.length > 0 ? sTools : ['tools'],
              lastError: s.last_error || undefined
            };
          })
        });
      }
    } catch (e) {
      console.warn('fetchBackendMCPServers failed', e);
    }
  },

  addMcp: mcp => {
    void (async () => {
      try {
        await SaveMCPServer({
          name: mcp.name,
          command: mcp.command,
          args: mcp.args,
          env: mcp.env,
          url: mcp.url,
          enabled: mcp.enabled !== false,
          type: mcp.type || (mcp.url ? 'remote' : 'local')
        });
        await get().fetchBackendMCPServers();
      } catch (err) {
        console.error('addMcp failed', err);
      }
    })();
    set(state => ({ mcps: [...state.mcps.filter(m => m.name !== mcp.name), mcp] }));
  },

  toggleMcp: id => {
    const target = get().mcps.find(m => m.id === id || m.name === id);
    if (!target) return;
    const nextEnabled = !target.enabled;
    void SaveMCPServer({
      name: target.name,
      command: target.command,
      args: target.args,
      env: target.env,
      url: target.url,
      enabled: nextEnabled,
      type: target.type || (target.url ? 'remote' : 'local')
    })
      .then(() => get().fetchBackendMCPServers())
      .catch(err => console.error('toggleMcp failed', err));
    set(state => ({
      mcps: state.mcps.map(m => (m.id === id || m.name === id ? { ...m, enabled: nextEnabled } : m))
    }));
  },

  deleteMcp: id => {
    const target = get().mcps.find(m => m.id === id || m.name === id);
    const name = target ? target.name : id;
    void DeleteMCPServer(name)
      .then(() => get().fetchBackendMCPServers())
      .catch(err => console.error('deleteMcp failed', err));
    set(state => ({
      mcps: state.mcps.filter(m => m.id !== id && m.name !== id)
    }));
  },

  // Skills
  skills: [
    {
      id: 'sk-native',
      name: 'native-sdk',
      category: 'Native SDK',
      description:
        'Discovery skill for building native desktop applications with TypeScript cores, Native markup, and Zig engine.',
      enabled: true,
      trigger: 'native-sdk'
    },
    {
      id: 'sk-forge',
      name: 'forge-ade',
      category: 'Workspace',
      description:
        'High performance editor primitives, unified diff engine, Git graph, LSP diagnostics, and ACP subagent bridges.',
      enabled: true,
      trigger: 'forge-ade'
    },
    {
      id: 'sk-1',
      name: 'clean-architecture',
      category: 'Code Quality',
      description: 'Auto-detects code smells and optimizes design patterns.',
      enabled: true,
      trigger: 'clean-architecture'
    },
    {
      id: 'sk-2',
      name: 'test-runner',
      category: 'Testing',
      description: 'Runs automated test suites in background sandbox.',
      enabled: true,
      trigger: 'test-runner'
    }
  ],

  setSkills: skillsOrFn =>
    set(state => ({
      skills: typeof skillsOrFn === 'function' ? skillsOrFn(state.skills) : skillsOrFn
    })),

  addSkill: skill => set(state => ({ skills: [...state.skills, skill] })),

  toggleSkill: id =>
    set(state => ({
      skills: state.skills.map(s => (s && s.id === id ? { ...s, enabled: !s.enabled } : s))
    })),

  deleteSkill: id =>
    set(state => ({
      skills: state.skills.filter(s => s && s.id !== id && s.name !== id)
    })),

  fetchBackendSkills: async () => {
    try {
      const backendSkills = await ApiBridge.listSkills();
      if (Array.isArray(backendSkills) && backendSkills.length > 0) {
        set(state => {
          const map = new Map<string, SkillEntry>();
          for (const s of state.skills || []) {
            if (!s) continue;
            const sName = String(s.name || s.trigger || s.id || '').trim();
            if (!sName) continue;
            const key = sName.toLowerCase();
            map.set(key, { ...s, name: key.replace(/\s+/g, '-') });
          }
          for (const b of backendSkills) {
            if (!b) continue;
            const rawName = String(b.name || (b as any).Name || b.trigger || b.id || '').trim();
            if (!rawName) continue;
            const bKey = rawName.toLowerCase();
            const cleanName = bKey.replace(/\s+/g, '-');
            const existing = map.get(bKey);
            map.set(bKey, {
              id: existing?.id || b.id || b.name || cleanName,
              name: cleanName,
              category: b.category || b.scope || 'Custom',
              description: b.description || '',
              enabled: existing ? existing.enabled : true,
              trigger: b.trigger || cleanName,
              instructions: b.body || b.instructions || (b as any).prompt || existing?.instructions,
              origin: b.scope || 'workspace',
              path: b.path || existing?.path
            });
          }
          return { skills: Array.from(map.values()) };
        });
      }
    } catch (e) {
      console.warn('fetchBackendSkills failed', e);
    }
  },

  createBackendSkill: async req => {
    const sk = await ApiBridge.createSkill(req);
    await get().fetchBackendSkills();
    return sk;
  },

  deleteBackendSkill: async name => {
    await ApiBridge.deleteSkill(name);
    set(state => ({ skills: state.skills.filter(s => s.name !== name && s.id !== name) }));
    await get().fetchBackendSkills();
  },

  reloadSkills: async () => {
    await ApiBridge.reloadSkills();
    await get().fetchBackendSkills();
  },

  // Plugins
  plugins: [],
  fetchPlugins: async () => {
    try {
      const list = await ApiBridge.listPlugins();
      set({ plugins: list });
    } catch (e) {
      console.warn('fetchPlugins failed', e);
    }
  },

  createPlugin: async req => {
    const p = await ApiBridge.createPlugin(req);
    await get().fetchPlugins();
    return p;
  },

  togglePlugin: async (id, enabled) => {
    set(state => ({
      plugins: state.plugins.map(p =>
        p.id === id ? { ...p, enabled: enabled !== undefined ? enabled : !p.enabled } : p
      )
    }));
    const target = get().plugins.find(p => p.id === id);
    const newEnabled = enabled !== undefined ? enabled : target ? !target.enabled : true;
    await ApiBridge.togglePlugin(id, newEnabled);
    await get().fetchPlugins();
  },

  deletePlugin: async id => {
    set(state => ({ plugins: state.plugins.filter(p => p.id !== id) }));
    await ApiBridge.deletePlugin(id);
    await get().fetchPlugins();
  },

  reloadPlugins: async () => {
    const list = await ApiBridge.reloadPlugins();
    set({ plugins: list });
  },

  // Discovery
  discoveredMcps: [],
  discoveredSkills: [],
  isDiscovering: false,

  runDiscovery: async () => {
    set({ isDiscovering: true });
    try {
      const [mcpsRes, skillsRes] = await Promise.all([
        ApiBridge.discoverMcps(),
        ApiBridge.discoverSkills()
      ]);
      set({ discoveredMcps: mcpsRes, discoveredSkills: skillsRes });
    } finally {
      set({ isDiscovering: false });
    }
  },

  importDiscoveredMcp: disc => {
    void (async () => {
      try {
        await SaveMCPServer({
          name: disc.name,
          command: disc.command,
          args: disc.args,
          enabled: true,
          type: disc.type || 'local'
        });
        await get().fetchBackendMCPServers();
      } catch (e) {
        console.warn('importDiscoveredMcp failed', e);
      }
    })();
  },

  importDiscoveredSkill: disc => {
    set(state => {
      const exists = state.skills.some(s => s.trigger === disc.trigger || s.name === disc.name);
      if (exists) return state;
      return {
        skills: [
          ...state.skills,
          {
            id: `skill-${Date.now()}`,
            name: disc.name,
            category: disc.category || 'Discovered',
            description: disc.description,
            trigger: disc.trigger,
            instructions: disc.instructions,
            enabled: true,
            origin: disc.origin
          }
        ]
      };
    });
  },

  // Memories
  memories: [],
  fetchMemories: async () => {
    try {
      const list = await ApiBridge.listMemories();
      set({ memories: list });
    } catch (e) {
      console.warn('fetchMemories failed', e);
    }
  },

  saveMemory: async entry => {
    await ApiBridge.saveMemory(entry);
    await get().fetchMemories();
  },

  deleteMemory: async id => {
    await ApiBridge.deleteMemory(id);
    set(state => ({ memories: state.memories.filter(m => m.id !== id) }));
    await get().fetchMemories();
  },

  reloadMemories: async () => {
    const list = await ApiBridge.reloadMemories();
    set({ memories: list });
  },

  // Automations
  automations: [],
  automationsLoading: false,
  fetchAutomations: async () => {
    set({ automationsLoading: true });
    try {
      const list = await ApiBridge.listAutomations();
      set({ automations: list });
    } catch (e) {
      console.warn('fetchAutomations failed', e);
    } finally {
      set({ automationsLoading: false });
    }
  },

  saveAutomation: async automation => {
    const saved = await ApiBridge.saveAutomation(automation);
    await get().fetchAutomations();
    return saved;
  },

  deleteAutomation: async id => {
    await ApiBridge.deleteAutomation(id);
    await get().fetchAutomations();
  },

  setAutomationEnabled: async (id, enabled) => {
    await ApiBridge.setAutomationEnabled(id, enabled);
    await get().fetchAutomations();
  },

  // Custom Slash Commands
  customCommands: DEFAULT_SLASH_COMMANDS,
  addCustomCommand: cmd => set(state => ({ customCommands: [...state.customCommands, cmd] })),
  updateCustomCommand: (id, updates) =>
    set(state => ({
      customCommands: state.customCommands.map(c => (c.id === id ? { ...c, ...updates } : c))
    })),
  deleteCustomCommand: id =>
    set(state => ({
      customCommands: state.customCommands.filter(c => c.id !== id)
    })),
  toggleCustomCommand: id =>
    set(state => ({
      customCommands: state.customCommands.map(c =>
        c.id === id ? { ...c, enabled: !c.enabled } : c
      )
    })),

  // Hooks
  hooks: DEFAULT_HOOKS,
  updateHook: (id, updates) =>
    set(state => ({
      hooks: state.hooks.map(h => (h.id === id ? { ...h, ...updates } : h))
    })),
  toggleHook: id =>
    set(state => ({
      hooks: state.hooks.map(h => (h.id === id ? { ...h, enabled: !h.enabled } : h))
    })),
  addHook: hook => set(state => ({ hooks: [...state.hooks, hook] })),
  deleteHook: id => set(state => ({ hooks: state.hooks.filter(h => h.id !== id) })),

  // Browser & Computer Use Settings
  browserSettings: DEFAULT_BROWSER_SETTINGS,
  updateBrowserSettings: updates =>
    set(state => ({ browserSettings: { ...state.browserSettings, ...updates } })),
  computerSettings: DEFAULT_COMPUTER_SETTINGS,
  updateComputerSettings: updates =>
    set(state => ({ computerSettings: { ...state.computerSettings, ...updates } })),

  initIntegrationsListeners: () => {
    const unsubPlugins = EventsOn('plugins:changed', () => {
      void get().fetchPlugins();
      void get().fetchBackendSkills();
      void get().fetchBackendMCPServers();
    });
    const unsubSkills = EventsOn('skills:changed', () => {
      void get().fetchBackendSkills();
    });
    const unsubConfig = EventsOn('agent:config:changed', () => {
      void get().fetchBackendSkills();
      void get().fetchBackendMCPServers();
    });
    const unsubMemory = EventsOn('memory:changed', () => {
      void get().fetchMemories();
    });
    const unsubAuto = EventsOn('automations:changed', () => {
      void get().fetchAutomations();
    });

    return () => {
      unsubPlugins?.();
      unsubSkills?.();
      unsubConfig?.();
      unsubMemory?.();
      unsubAuto?.();
    };
  }
}));
