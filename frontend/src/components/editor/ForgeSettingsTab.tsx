import React, { useState, useMemo } from 'react';
import {
  Search,
  Sparkles,
  Star,
  MoreHorizontal,
  Sun,
  Moon,
  Key,
  Check,
  ExternalLink,
  Shield,
  Terminal,
  FileCode,
  RefreshCw,
  Cpu,
  Bot,
  Zap,
  Layers,
  Settings,
  Globe,
  Code2,
  Eye,
  Lock,
  Plus,
  Trash2,
  Sliders,
  FileText,
  SlidersHorizontal,
  Folder
} from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';
import { LLMProviderConfig, MCPEntry, SkillEntry, RuleEntry } from '../../types';
import { ApiBridge } from '../../services/apiBridge';

export const ForgeSettingsTab: React.FC = () => {
  const {
    theme,
    setTheme,
    currentModel,
    setCurrentModel,
    privacySettings,
    setPrivacySettings,
    providers,
    updateProvider,
    addProvider,
    deleteProvider,
    addModelToProvider,
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
    runDiscovery,
    isDiscovering,
    activeWorkspacePath
  } = useWorkspace();

  const [activeSection, setActiveSection] = useState<string>('general');
  const [navSearchQuery, setNavSearchQuery] = useState('');
  const [acpEnabled, setAcpEnabled] = useState(true);

  // New Provider Form
  const [isAddingProvider, setIsAddingProvider] = useState(false);
  const [newProviderName, setNewProviderName] = useState('');
  const [newProviderBaseUrl, setNewProviderBaseUrl] = useState('');
  const [newProviderApiKey, setNewProviderApiKey] = useState('');
  const [fetchingProviderId, setFetchingProviderId] = useState<string | null>(null);

  // New MCP Form
  const [isAddingMcp, setIsAddingMcp] = useState(false);
  const [newMcpName, setNewMcpName] = useState('');
  const [newMcpCommand, setNewMcpCommand] = useState('');

  // New Skill Form
  const [isAddingSkill, setIsAddingSkill] = useState(false);
  const [newSkillName, setNewSkillName] = useState('');
  const [newSkillTrigger, setNewSkillTrigger] = useState('');
  const [newSkillDesc, setNewSkillDesc] = useState('');
  const [newSkillInstructions, setNewSkillInstructions] = useState('');

  // Rules state
  const [rules] = useState<RuleEntry[]>([
    { id: 'rule-1', title: 'Always write TypeScript strict mode code', content: 'Ensure all new interfaces and types avoid any.', enabled: true, category: 'Code Quality' },
    { id: 'rule-2', title: 'Follow Clean Architecture modularization', content: 'Separate UI components, state stores, and backend bridges.', enabled: true, category: 'Architecture' },
    { id: 'rule-3', title: 'Preserve user instructions and code integrity', content: 'Never delete user files or existing tests without explicit instruction.', enabled: true, category: 'Safety' }
  ]);

  // Agents list state
  const [agentsList, setAgentsList] = useState([
    {
      id: 'forge-local',
      name: 'Forge Local',
      description: 'Forge-ADE coding agent via Forge CLI',
      iconType: 'badge',
      badgeText: 'FA',
      enabled: true,
      hasStar: false
    },
    {
      id: 'forge-cloud',
      name: 'Forge Cloud',
      description: 'Forge-ADE Cloud Environment',
      iconType: 'badge',
      badgeText: 'FA',
      enabled: true,
      hasStar: true
    },
    {
      id: 'claude-agent',
      name: 'Claude Agent',
      description: "ACP wrapper for Anthropic's Claude",
      iconType: 'claude',
      enabled: false,
      hasStar: false
    },
    {
      id: 'codex',
      name: 'Codex',
      description: "ACP adapter for OpenAI's coding assistant",
      iconType: 'codex',
      enabled: false,
      hasStar: false
    },
    {
      id: 'agoragentic',
      name: 'Agoragentic',
      description: 'Agent marketplace with 174+ AI capabilities. Browse, invoke, and pay for ag...',
      iconType: 'agoragentic',
      enabled: false,
      hasStar: false
    },
    {
      id: 'amp',
      name: 'Amp',
      description: 'ACP wrapper for Amp - the frontier coding agent',
      iconType: 'amp',
      enabled: false,
      hasStar: false
    },
    {
      id: 'google-antigravity',
      name: 'Google Antigravity',
      description: "Google's AI coding agent",
      iconType: 'antigravity',
      enabled: false,
      hasStar: false
    }
  ]);

  const toggleAgent = (id: string) => {
    setAgentsList(prev => prev.map(a => a.id === id ? { ...a, enabled: !a.enabled } : a));
  };

  // Nav Items definition from before with simplicity layout
  const navItems = [
    { id: 'general', label: 'General', icon: SlidersHorizontal },
    { id: 'providers', label: 'Providers & Models', icon: Cpu },
    { id: 'mcps', label: 'MCP Servers', icon: Layers },
    { id: 'skills', label: 'Skills', icon: Sparkles },
    { id: 'agents', label: 'Agents', icon: Bot },
    { id: 'rules', label: 'Rules', icon: FileText },
    { id: 'privacy', label: 'Privacy & Sharing', icon: Shield },
    { id: 'shortcuts', label: 'Shortcuts', icon: Key }
  ];

  const filteredNavItems = useMemo(() => {
    if (!navSearchQuery.trim()) return navItems;
    const q = navSearchQuery.toLowerCase();
    return navItems.filter(item => item.label.toLowerCase().includes(q));
  }, [navItems, navSearchQuery]);

  const handleCreateProvider = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newProviderName.trim()) return;
    const newProvId = `prov-${Date.now()}`;
    const newProv: LLMProviderConfig = {
      id: newProvId,
      name: newProviderName.trim(),
      baseUrl: newProviderBaseUrl.trim() || undefined,
      apiKey: newProviderApiKey.trim() || undefined,
      enabled: true,
      models: [],
      selectedModels: []
    };
    addProvider(newProv);
    setNewProviderName('');
    setNewProviderBaseUrl('');
    setNewProviderApiKey('');
    setIsAddingProvider(false);
  };

  const handleCreateMcp = (e: React.FormEvent) => {
    e.preventDefault();
    if (!newMcpName.trim() || !newMcpCommand.trim()) return;
    addMcp({
      id: `mcp-${Date.now()}`,
      name: newMcpName.trim(),
      command: newMcpCommand.trim(),
      status: 'connected',
      tools: [],
      enabled: true
    });
    setNewMcpName('');
    setNewMcpCommand('');
    setIsAddingMcp(false);
  };

  const handleCreateSkill = (e: React.FormEvent) => {
    e.preventDefault();
    if (!newSkillName.trim() || !newSkillTrigger.trim()) return;
    addSkill({
      id: `skill-${Date.now()}`,
      name: newSkillName.trim(),
      category: 'Custom',
      trigger: newSkillTrigger.trim().replace(/^@/, ''),
      description: newSkillDesc.trim(),
      instructions: newSkillInstructions.trim(),
      enabled: true
    });
    setNewSkillName('');
    setNewSkillTrigger('');
    setNewSkillDesc('');
    setNewSkillInstructions('');
    setIsAddingSkill(false);
  };

  const handleFetchModels = async (provId: string) => {
    setFetchingProviderId(provId);
    try {
      await fetchProviderModels(provId);
    } finally {
      setFetchingProviderId(null);
    }
  };

  return (
    <div className="flex-1 flex h-full overflow-hidden bg-white dark:bg-[#181818] text-[#1e293b] dark:text-[#e2e8f0] select-none font-sans">

      {/* ── LEFT SIDEBAR NAVIGATION ── */}
      <div className="w-64 min-w-[220px] max-w-[280px] p-5 border-r border-[#e5e7eb] dark:border-[#262626] flex flex-col justify-between overflow-y-auto">

        <div className="space-y-4">
          {/* Search Settings Input */}
          <div className="relative">
            <Search className="absolute left-2.5 top-2.5 w-3.5 h-3.5 text-[#9ca3af]" />
            <input
              type="text"
              placeholder="Search settings"
              value={navSearchQuery}
              onChange={e => setNavSearchQuery(e.target.value)}
              className="w-full pl-8 pr-3 py-1.5 bg-[#f8fafc] dark:bg-[#202022] border border-[#e2e8f0] dark:border-[#333336] rounded-lg text-xs text-[#1e293b] dark:text-white placeholder-[#9ca3af] focus:outline-none focus:border-[#2563eb]"
            />
          </div>

          {/* Navigation Items */}
          <div className="space-y-1">
            {filteredNavItems.map(item => {
              const Icon = item.icon;
              const isActive = activeSection === item.id;
              return (
                <button
                  key={item.id}
                  type="button"
                  onClick={() => setActiveSection(item.id)}
                  className={`w-full px-3 py-2 rounded-xl text-xs font-medium text-left transition-colors flex items-center gap-2.5 cursor-pointer ${
                    isActive
                      ? 'bg-[#f1f5f9] dark:bg-[#262626] text-[#0f172a] dark:text-white font-semibold shadow-2xs'
                      : 'text-[#475569] dark:text-[#9ca3af] hover:bg-[#f8fafc] dark:hover:bg-[#1f1f22] hover:text-[#0f172a] dark:hover:text-white'
                  }`}
                >
                  <Icon className={`w-4 h-4 ${isActive ? 'text-[#2563eb] dark:text-[#60a5fa]' : 'text-[#9ca3af]'}`} />
                  <span>{item.label}</span>
                </button>
              );
            })}
          </div>
        </div>

        {/* Workspace path pill */}
        <div className="pt-4 border-t border-[#f1f5f9] dark:border-[#262626]">
          <div className="p-2.5 rounded-xl bg-[#f8fafc] dark:bg-[#202022] border border-[#e5e7eb] dark:border-[#333336] text-[11px] text-[#64748b] dark:text-[#9ca3af]">
            <p className="font-semibold text-[#0f172a] dark:text-white mb-0.5">Active Workspace</p>
            <p className="font-mono text-[10px] truncate">{activeWorkspacePath || 'No folder open'}</p>
          </div>
        </div>

      </div>

      {/* ── RIGHT MAIN CONTENT AREA ── */}
      <div className="flex-1 overflow-y-auto p-8 max-w-4xl space-y-6">

        {/* SECTION: GENERAL */}
        {activeSection === 'general' && (
          <div className="space-y-6 animate-in fade-in duration-150">
            <div>
              <h1 className="text-2xl font-bold text-[#0f172a] dark:text-white">General Preferences</h1>
              <p className="text-xs text-[#64748b] dark:text-[#94a3b8] mt-1">
                Customize appearance, themes, and editor behaviors.
              </p>
            </div>

            {/* Theme Card */}
            <div className="rounded-2xl border border-[#e5e7eb] dark:border-[#2a2a2e] bg-white dark:bg-[#1c1c1f] p-5 shadow-2xs flex items-center justify-between">
              <div>
                <h3 className="text-sm font-bold text-[#0f172a] dark:text-white">Color Theme</h3>
                <p className="text-xs text-[#64748b] dark:text-[#94a3b8] mt-0.5">
                  Toggle between Light Mode and Dark Mode.
                </p>
              </div>

              <button
                type="button"
                onClick={() => setTheme(theme === 'dark' ? 'light' : 'dark')}
                className="px-3.5 py-1.5 rounded-xl border border-[#cbd5e1] dark:border-[#3f3f46] hover:bg-[#f8fafc] dark:hover:bg-[#27272a] text-xs font-semibold flex items-center gap-2 transition-colors cursor-pointer"
              >
                {theme === 'dark' ? <Moon className="w-3.5 h-3.5 text-[#a855f7]" /> : <Sun className="w-3.5 h-3.5 text-[#eab308]" />}
                <span>{theme === 'dark' ? 'Dark Mode' : 'Light Mode'}</span>
              </button>
            </div>

            {/* Font Size Card */}
            <div className="rounded-2xl border border-[#e5e7eb] dark:border-[#2a2a2e] bg-white dark:bg-[#1c1c1f] p-5 shadow-2xs flex items-center justify-between">
              <div>
                <h3 className="text-sm font-bold text-[#0f172a] dark:text-white">Editor Font Size</h3>
                <p className="text-xs text-[#64748b] dark:text-[#94a3b8] mt-0.5">
                  Font size for editor panes and line gutter numbers.
                </p>
              </div>
              <span className="text-xs font-mono font-bold px-2.5 py-1 rounded-lg bg-[#f1f5f9] dark:bg-[#262629]">
                13px
              </span>
            </div>
          </div>
        )}

        {/* SECTION: PROVIDERS & MODELS */}
        {activeSection === 'providers' && (
          <div className="space-y-6 animate-in fade-in duration-150">
            <div className="flex items-center justify-between">
              <div>
                <h1 className="text-2xl font-bold text-[#0f172a] dark:text-white">Providers & Models</h1>
                <p className="text-xs text-[#64748b] dark:text-[#94a3b8] mt-1">
                  Manage AI model providers (OpenAI, Anthropic, Ollama, OpenRouter, Google) and API keys.
                </p>
              </div>
              <button
                type="button"
                onClick={() => setIsAddingProvider(true)}
                className="px-3 py-1.5 rounded-xl bg-[#2563eb] text-white text-xs font-semibold flex items-center gap-1.5 shadow-2xs hover:bg-[#1d4ed8] transition-colors cursor-pointer"
              >
                <Plus className="w-3.5 h-3.5" />
                <span>Add Provider</span>
              </button>
            </div>

            {/* New Provider Modal / Box */}
            {isAddingProvider && (
              <form onSubmit={handleCreateProvider} className="rounded-2xl border border-[#2563eb] bg-[#f8fafc] dark:bg-[#202023] p-5 shadow-xs space-y-3">
                <h3 className="text-xs font-bold text-[#0f172a] dark:text-white uppercase tracking-wider">Add Custom LLM Provider</h3>
                <div className="grid grid-cols-1 md:grid-cols-3 gap-3">
                  <input
                    type="text"
                    required
                    placeholder="Provider Name (e.g. Ollama, OpenRouter)"
                    value={newProviderName}
                    onChange={e => setNewProviderName(e.target.value)}
                    className="px-3 py-1.5 bg-white dark:bg-[#181818] border border-[#cbd5e1] dark:border-[#383838] rounded-lg text-xs"
                  />
                  <input
                    type="text"
                    placeholder="Base URL (e.g. http://localhost:11434)"
                    value={newProviderBaseUrl}
                    onChange={e => setNewProviderBaseUrl(e.target.value)}
                    className="px-3 py-1.5 bg-white dark:bg-[#181818] border border-[#cbd5e1] dark:border-[#383838] rounded-lg text-xs"
                  />
                  <input
                    type="password"
                    placeholder="API Key"
                    value={newProviderApiKey}
                    onChange={e => setNewProviderApiKey(e.target.value)}
                    className="px-3 py-1.5 bg-white dark:bg-[#181818] border border-[#cbd5e1] dark:border-[#383838] rounded-lg text-xs"
                  />
                </div>
                <div className="flex items-center justify-end gap-2 pt-2">
                  <button
                    type="button"
                    onClick={() => setIsAddingProvider(false)}
                    className="px-3 py-1 rounded-lg hover:bg-[#e2e8f0] dark:hover:bg-[#333] text-xs"
                  >
                    Cancel
                  </button>
                  <button
                    type="submit"
                    className="px-3 py-1 rounded-lg bg-[#2563eb] text-white text-xs font-semibold"
                  >
                    Save Provider
                  </button>
                </div>
              </form>
            )}

            {/* Providers List */}
            <div className="space-y-4">
              {providers.map(prov => (
                <div key={prov.id} className="rounded-2xl border border-[#e5e7eb] dark:border-[#2a2a2e] bg-white dark:bg-[#1c1c1f] p-5 shadow-2xs space-y-4">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2.5">
                      <Cpu className="w-4 h-4 text-[#2563eb]" />
                      <span className="font-bold text-sm text-[#0f172a] dark:text-white">{prov.name}</span>
                      <span className="text-[10px] font-mono font-bold px-2 py-0.5 rounded-full bg-[#ecfdf5] text-[#059669] dark:bg-[#064e3b] dark:text-[#a7f3d0]">
                        Active
                      </span>
                    </div>

                    <div className="flex items-center gap-2">
                      <button
                        type="button"
                        onClick={() => handleFetchModels(prov.id)}
                        disabled={fetchingProviderId === prov.id}
                        className="px-2.5 py-1 rounded-lg border border-[#cbd5e1] dark:border-[#3f3f46] hover:bg-[#f8fafc] dark:hover:bg-[#27272a] text-xs font-medium flex items-center gap-1 cursor-pointer"
                      >
                        <RefreshCw className={`w-3 h-3 ${fetchingProviderId === prov.id ? 'animate-spin' : ''}`} />
                        <span>Fetch Models</span>
                      </button>
                      <button
                        type="button"
                        onClick={() => deleteProvider(prov.id)}
                        className="p-1 rounded-lg hover:bg-[#fee2e2] dark:hover:bg-[#450a0a] text-[#dc2626] cursor-pointer"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </div>
                  </div>

                  <div className="grid grid-cols-1 md:grid-cols-2 gap-3 text-xs">
                    <div>
                      <label className="block text-[11px] text-[#64748b] dark:text-[#94a3b8] mb-1">Base URL</label>
                      <input
                        type="text"
                        value={prov.baseUrl || ''}
                        placeholder="https://api.openai.com/v1"
                        onChange={e => updateProvider(prov.id, { baseUrl: e.target.value })}
                        className="w-full px-3 py-1.5 bg-[#f8fafc] dark:bg-[#141416] border border-[#e2e8f0] dark:border-[#333336] rounded-lg text-xs font-mono"
                      />
                    </div>
                    <div>
                      <label className="block text-[11px] text-[#64748b] dark:text-[#94a3b8] mb-1">API Key</label>
                      <input
                        type="password"
                        value={prov.apiKey || ''}
                        placeholder="••••••••••••••••"
                        onChange={e => updateProvider(prov.id, { apiKey: e.target.value })}
                        className="w-full px-3 py-1.5 bg-[#f8fafc] dark:bg-[#141416] border border-[#e2e8f0] dark:border-[#333336] rounded-lg text-xs font-mono"
                      />
                    </div>
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* SECTION: MCP SERVERS */}
        {activeSection === 'mcps' && (
          <div className="space-y-6 animate-in fade-in duration-150">
            <div className="flex items-center justify-between">
              <div>
                <h1 className="text-2xl font-bold text-[#0f172a] dark:text-white">Model Context Protocol (MCP)</h1>
                <p className="text-xs text-[#64748b] dark:text-[#94a3b8] mt-1">
                  Connect MCP servers to equip Forge-ADE with custom tools and databases.
                </p>
              </div>
              <div className="flex items-center gap-2">
                <button
                  type="button"
                  onClick={runDiscovery}
                  disabled={isDiscovering}
                  className="px-3 py-1.5 rounded-xl border border-[#cbd5e1] dark:border-[#3f3f46] hover:bg-[#f8fafc] dark:hover:bg-[#27272a] text-xs font-semibold flex items-center gap-1.5 cursor-pointer"
                >
                  <RefreshCw className={`w-3.5 h-3.5 ${isDiscovering ? 'animate-spin' : ''}`} />
                  <span>Auto-Discover</span>
                </button>
                <button
                  type="button"
                  onClick={() => setIsAddingMcp(true)}
                  className="px-3 py-1.5 rounded-xl bg-[#2563eb] text-white text-xs font-semibold flex items-center gap-1.5 shadow-2xs hover:bg-[#1d4ed8] cursor-pointer"
                >
                  <Plus className="w-3.5 h-3.5" />
                  <span>Add MCP</span>
                </button>
              </div>
            </div>

            {/* New MCP Box */}
            {isAddingMcp && (
              <form onSubmit={handleCreateMcp} className="rounded-2xl border border-[#2563eb] bg-[#f8fafc] dark:bg-[#202023] p-5 shadow-xs space-y-3">
                <h3 className="text-xs font-bold text-[#0f172a] dark:text-white uppercase tracking-wider">Register MCP Server</h3>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                  <input
                    type="text"
                    required
                    placeholder="Server Name (e.g. SQLite DB)"
                    value={newMcpName}
                    onChange={e => setNewMcpName(e.target.value)}
                    className="px-3 py-1.5 bg-white dark:bg-[#181818] border border-[#cbd5e1] dark:border-[#383838] rounded-lg text-xs"
                  />
                  <input
                    type="text"
                    required
                    placeholder="Command (e.g. npx -y @modelcontextprotocol/server-sqlite)"
                    value={newMcpCommand}
                    onChange={e => setNewMcpCommand(e.target.value)}
                    className="px-3 py-1.5 bg-white dark:bg-[#181818] border border-[#cbd5e1] dark:border-[#383838] rounded-lg text-xs font-mono"
                  />
                </div>
                <div className="flex items-center justify-end gap-2 pt-2">
                  <button type="button" onClick={() => setIsAddingMcp(false)} className="px-3 py-1 hover:bg-[#e2e8f0] dark:hover:bg-[#333] text-xs">
                    Cancel
                  </button>
                  <button type="submit" className="px-3 py-1 rounded-lg bg-[#2563eb] text-white text-xs font-semibold">
                    Save Server
                  </button>
                </div>
              </form>
            )}

            <div className="space-y-3">
              {mcps.map(mcp => (
                <div key={mcp.id} className="rounded-2xl border border-[#e5e7eb] dark:border-[#2a2a2e] bg-white dark:bg-[#1c1c1f] p-4 shadow-2xs flex items-center justify-between">
                  <div className="min-w-0 flex-1 pr-4">
                    <div className="flex items-center gap-2">
                      <span className="font-bold text-xs text-[#0f172a] dark:text-white">{mcp.name}</span>
                      <span className={`text-[10px] font-mono px-2 py-0.5 rounded-full ${mcp.enabled ? 'bg-[#ecfdf5] text-[#059669]' : 'bg-[#f1f5f9] text-[#64748b]'}`}>
                        {mcp.enabled ? 'Enabled' : 'Disabled'}
                      </span>
                    </div>
                    <p className="text-[11px] font-mono text-[#64748b] dark:text-[#94a3b8] truncate mt-1">
                      {mcp.command}
                    </p>
                  </div>

                  <div className="flex items-center gap-2 shrink-0">
                    <button
                      type="button"
                      onClick={() => toggleMcp(mcp.id)}
                      className={`px-3 py-1 rounded-lg text-xs font-semibold cursor-pointer ${
                        mcp.enabled ? 'bg-[#ecfdf5] text-[#059669] hover:bg-[#d1fae5]' : 'bg-[#f1f5f9] text-[#64748b] hover:bg-[#e2e8f0]'
                      }`}
                    >
                      {mcp.enabled ? 'Connected' : 'Enable'}
                    </button>
                    <button
                      type="button"
                      onClick={() => deleteMcp(mcp.id)}
                      className="p-1 rounded-lg hover:bg-[#fee2e2] text-[#9ca3af] hover:text-[#dc2626] cursor-pointer"
                    >
                      <Trash2 className="w-4 h-4" />
                    </button>
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* SECTION: SKILLS */}
        {activeSection === 'skills' && (
          <div className="space-y-6 animate-in fade-in duration-150">
            <div className="flex items-center justify-between">
              <div>
                <h1 className="text-2xl font-bold text-[#0f172a] dark:text-white">Agent Skills</h1>
                <p className="text-xs text-[#64748b] dark:text-[#94a3b8] mt-1">
                  Custom specialized skill instructions invoked via @trigger tags.
                </p>
              </div>
              <button
                type="button"
                onClick={() => setIsAddingSkill(true)}
                className="px-3 py-1.5 rounded-xl bg-[#2563eb] text-white text-xs font-semibold flex items-center gap-1.5 shadow-2xs hover:bg-[#1d4ed8] cursor-pointer"
              >
                <Plus className="w-3.5 h-3.5" />
                <span>Add Skill</span>
              </button>
            </div>

            {/* New Skill Box */}
            {isAddingSkill && (
              <form onSubmit={handleCreateSkill} className="rounded-2xl border border-[#2563eb] bg-[#f8fafc] dark:bg-[#202023] p-5 shadow-xs space-y-3">
                <h3 className="text-xs font-bold text-[#0f172a] dark:text-white uppercase tracking-wider">Create Custom Skill</h3>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                  <input
                    type="text"
                    required
                    placeholder="Skill Name (e.g. Frontend UI)"
                    value={newSkillName}
                    onChange={e => setNewSkillName(e.target.value)}
                    className="px-3 py-1.5 bg-white dark:bg-[#181818] border border-[#cbd5e1] dark:border-[#383838] rounded-lg text-xs"
                  />
                  <input
                    type="text"
                    required
                    placeholder="Trigger (e.g. ui or git-flow)"
                    value={newSkillTrigger}
                    onChange={e => setNewSkillTrigger(e.target.value)}
                    className="px-3 py-1.5 bg-white dark:bg-[#181818] border border-[#cbd5e1] dark:border-[#383838] rounded-lg text-xs font-mono"
                  />
                </div>
                <textarea
                  rows={2}
                  placeholder="Instructions for the agent..."
                  value={newSkillInstructions}
                  onChange={e => setNewSkillInstructions(e.target.value)}
                  className="w-full px-3 py-1.5 bg-white dark:bg-[#181818] border border-[#cbd5e1] dark:border-[#383838] rounded-lg text-xs font-mono resize-none"
                />
                <div className="flex items-center justify-end gap-2 pt-2">
                  <button type="button" onClick={() => setIsAddingSkill(false)} className="px-3 py-1 hover:bg-[#e2e8f0] text-xs">
                    Cancel
                  </button>
                  <button type="submit" className="px-3 py-1 rounded-lg bg-[#2563eb] text-white text-xs font-semibold">
                    Save Skill
                  </button>
                </div>
              </form>
            )}

            <div className="space-y-3">
              {skills.map(sk => (
                <div key={sk.id} className="rounded-2xl border border-[#e5e7eb] dark:border-[#2a2a2e] bg-white dark:bg-[#1c1c1f] p-4 shadow-2xs space-y-2">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <span className="font-bold text-xs text-[#0f172a] dark:text-white">{sk.name}</span>
                      <span className="text-[10px] px-2 py-0.5 rounded-md bg-[#eff6ff] text-[#2563eb] dark:bg-[#1e293b] dark:text-[#38bdf8] font-mono">
                        @{sk.trigger}
                      </span>
                    </div>
                    <div className="flex items-center gap-2">
                      <button
                        type="button"
                        onClick={() => toggleSkill(sk.id)}
                        className={`px-3 py-1 rounded-lg text-xs font-semibold cursor-pointer ${
                          sk.enabled ? 'bg-[#ecfdf5] text-[#059669]' : 'bg-[#f1f5f9] text-[#64748b]'
                        }`}
                      >
                        {sk.enabled ? 'Enabled' : 'Disabled'}
                      </button>
                      <button
                        type="button"
                        onClick={() => deleteSkill(sk.id)}
                        className="p-1 rounded-lg text-[#9ca3af] hover:text-[#dc2626] cursor-pointer"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </div>
                  </div>
                  <p className="text-xs text-[#64748b] dark:text-[#94a3b8]">{sk.description || sk.instructions}</p>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* SECTION: AGENTS */}
        {activeSection === 'agents' && (
          <div className="space-y-6 animate-in fade-in duration-150">
            <div>
              <h1 className="text-2xl font-bold text-[#0f172a] dark:text-white">Agents & ACP</h1>
              <p className="text-xs text-[#64748b] dark:text-[#94a3b8] mt-1">
                Configure Agent Client Protocol (ACP) agents and runtime adapters.
              </p>
            </div>

            <div className="rounded-2xl border border-[#e5e7eb] dark:border-[#2a2a2e] bg-white dark:bg-[#1c1c1f] p-5 shadow-2xs space-y-4">
              <div className="flex items-center justify-between">
                <div>
                  <h2 className="text-sm font-bold text-[#0f172a] dark:text-white">Enable ACP</h2>
                  <p className="text-xs text-[#64748b] dark:text-[#94a3b8] mt-0.5">
                    Agents can be instantiated through the Agent Client Protocol
                  </p>
                </div>

                <button
                  type="button"
                  onClick={() => setAcpEnabled(prev => !prev)}
                  className={`w-11 h-6 rounded-full transition-colors relative cursor-pointer ${
                    acpEnabled ? 'bg-[#2563eb]' : 'bg-[#cbd5e1] dark:bg-[#475569]'
                  }`}
                >
                  <span
                    className={`block w-4 h-4 rounded-full bg-white transition-transform ${
                      acpEnabled ? 'translate-x-6' : 'translate-x-1'
                    }`}
                  />
                </button>
              </div>

              <div className="divide-y divide-[#f1f5f9] dark:divide-[#262629]">
                {agentsList.map(agent => (
                  <div key={agent.id} className="py-3 flex items-center justify-between gap-3">
                    <div className="flex items-center gap-3 min-w-0">
                      <div className="w-8 h-8 rounded-lg bg-[#f1f5f9] dark:bg-[#2a2a2e] border border-[#e2e8f0] dark:border-[#383838] flex items-center justify-center font-bold text-xs text-[#334155] dark:text-[#cbd5e1] font-mono shrink-0">
                        {agent.badgeText || 'AG'}
                      </div>
                      <div className="min-w-0">
                        <div className="flex items-center gap-1.5">
                          <span className="font-semibold text-xs text-[#0f172a] dark:text-white">{agent.name}</span>
                          {agent.hasStar && <Star className="w-3.5 h-3.5 fill-[#2563eb] text-[#2563eb]" />}
                        </div>
                        <p className="text-[11px] text-[#64748b] dark:text-[#94a3b8] truncate mt-0.5">{agent.description}</p>
                      </div>
                    </div>

                    <div className="flex items-center gap-2 shrink-0">
                      <button
                        type="button"
                        onClick={() => toggleAgent(agent.id)}
                        className={`px-3 py-1 rounded-lg text-xs font-semibold cursor-pointer ${
                          agent.enabled ? 'text-[#16a34a] hover:underline' : 'border border-[#cbd5e1] dark:border-[#3f3f46] text-[#0f172a] dark:text-white hover:bg-[#f8fafc]'
                        }`}
                      >
                        {agent.enabled ? 'Enabled' : 'Enable'}
                      </button>
                    </div>
                  </div>
                ))}
              </div>
            </div>
          </div>
        )}

        {/* SECTION: RULES */}
        {activeSection === 'rules' && (
          <div className="space-y-6 animate-in fade-in duration-150">
            <div>
              <h1 className="text-2xl font-bold text-[#0f172a] dark:text-white">Coding Rules (~/.forge-ade/rules)</h1>
              <p className="text-xs text-[#64748b] dark:text-[#94a3b8] mt-1">
                Project conventions and instructions injected into agent prompts.
              </p>
            </div>

            <div className="space-y-3">
              {rules.map(rule => (
                <div key={rule.id} className="p-4 rounded-2xl bg-white dark:bg-[#1e1e1e] border border-[#e2e8f0] dark:border-[#2b2b2b] shadow-2xs space-y-1">
                  <h3 className="text-sm font-bold text-[#0f172a] dark:text-white">{rule.title}</h3>
                  <p className="text-xs text-[#64748b] dark:text-[#94a3b8]">{rule.content}</p>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* SECTION: PRIVACY */}
        {activeSection === 'privacy' && (
          <div className="space-y-6 animate-in fade-in duration-150">
            <div>
              <h1 className="text-2xl font-bold text-[#0f172a] dark:text-white">Privacy & Data Sharing</h1>
              <p className="text-xs text-[#64748b] dark:text-[#94a3b8] mt-1">
                Control terminal activity and user edit telemetry shared with connected agents.
              </p>
            </div>

            <div className="space-y-4">
              <div className="p-5 rounded-2xl bg-white dark:bg-[#1e1e1e] border border-[#e2e8f0] dark:border-[#2b2b2b] shadow-2xs flex items-center justify-between">
                <div>
                  <h3 className="text-sm font-bold text-[#0f172a] dark:text-white">Share Terminal Activity</h3>
                  <p className="text-xs text-[#64748b] dark:text-[#94a3b8] mt-0.5">
                    Let Forge-ADE see commands and output from your integrated terminal.
                  </p>
                </div>
                <button
                  type="button"
                  onClick={() => setPrivacySettings(prev => ({ ...prev, shareTerminalActivity: !prev.shareTerminalActivity }))}
                  className={`w-11 h-6 rounded-full transition-colors relative cursor-pointer ${
                    privacySettings.shareTerminalActivity ? 'bg-[#2563eb]' : 'bg-[#cbd5e1] dark:bg-[#475569]'
                  }`}
                >
                  <span
                    className={`block w-4 h-4 rounded-full bg-white transition-transform ${
                      privacySettings.shareTerminalActivity ? 'translate-x-6' : 'translate-x-1'
                    }`}
                  />
                </button>
              </div>

              <div className="p-5 rounded-2xl bg-white dark:bg-[#1e1e1e] border border-[#e2e8f0] dark:border-[#2b2b2b] shadow-2xs flex items-center justify-between">
                <div>
                  <h3 className="text-sm font-bold text-[#0f172a] dark:text-white">Share User Edits</h3>
                  <p className="text-xs text-[#64748b] dark:text-[#94a3b8] mt-0.5">
                    Let Forge-ADE see which files you edit and create in your workspace.
                  </p>
                </div>
                <button
                  type="button"
                  onClick={() => setPrivacySettings(prev => ({ ...prev, shareUserEdits: !prev.shareUserEdits }))}
                  className={`w-11 h-6 rounded-full transition-colors relative cursor-pointer ${
                    privacySettings.shareUserEdits ? 'bg-[#2563eb]' : 'bg-[#cbd5e1] dark:bg-[#475569]'
                  }`}
                >
                  <span
                    className={`block w-4 h-4 rounded-full bg-white transition-transform ${
                      privacySettings.shareUserEdits ? 'translate-x-6' : 'translate-x-1'
                    }`}
                  />
                </button>
              </div>
            </div>
          </div>
        )}

        {/* SECTION: SHORTCUTS */}
        {activeSection === 'shortcuts' && (
          <div className="space-y-6 animate-in fade-in duration-150">
            <div>
              <h1 className="text-2xl font-bold text-[#0f172a] dark:text-white">Keyboard Shortcuts</h1>
              <p className="text-xs text-[#64748b] dark:text-[#94a3b8] mt-1">
                Essential shortcuts for fast development in the editor.
              </p>
            </div>

            <div className="rounded-2xl border border-[#e5e7eb] dark:border-[#2a2a2e] bg-white dark:bg-[#1c1c1f] p-5 shadow-2xs divide-y divide-[#f1f5f9] dark:divide-[#262629] text-xs">
              {[
                { label: 'Toggle Command Search', keys: ['⌘', 'P'] },
                { label: 'Find in File', keys: ['⌘', 'F'] },
                { label: 'Replace in File', keys: ['⌘', 'H'] },
                { label: 'Global Search', keys: ['⇧', '⌘', 'F'] },
                { label: 'Commit Changes', keys: ['⌘', 'Enter'] },
                { label: 'Open Settings', keys: ['⌘', ','] },
                { label: 'Toggle Bottom Terminal Panel', keys: ['⌃', '`'] },
                { label: 'Split Editor Right', keys: ['⌘', '\\'] }
              ].map((s, idx) => (
                <div key={idx} className="py-2.5 flex items-center justify-between">
                  <span className="text-[#334155] dark:text-[#e2e8f0] font-medium">{s.label}</span>
                  <div className="flex items-center gap-1">
                    {s.keys.map((k, ki) => (
                      <kbd key={ki} className="px-2 py-0.5 rounded bg-[#f1f5f9] dark:bg-[#28282b] border border-[#cbd5e1] dark:border-[#383838] font-mono text-[11px] font-semibold">
                        {k}
                      </kbd>
                    ))}
                  </div>
                </div>
              ))}
            </div>
          </div>
        )}

      </div>

    </div>
  );
};

export default ForgeSettingsTab;
