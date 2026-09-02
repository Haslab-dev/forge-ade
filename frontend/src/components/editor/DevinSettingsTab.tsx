import React, { useState } from 'react';
import { 
  Bot, 
  Shield, 
  Code2, 
  Layers, 
  Cpu, 
  FileText, 
  Settings, 
  Key, 
  Plus, 
  Trash2, 
  Check, 
  Copy, 
  Sun, 
  Moon, 
  Sparkles,
  RefreshCw,
  ExternalLink,
  Download,
  CheckSquare,
  Square
} from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';
import { LLMProviderConfig, MCPEntry, SkillEntry, RuleEntry, SubAgentEntry } from '../../types';

export const MyAdeSettingsTab: React.FC = () => {
  const { 
    theme, 
    setTheme, 
    currentModel, 
    setCurrentModel,
    settingsActiveSection,
    setSettingsActiveSection,
    agents,
    setActiveAgentId,
    activeAgentId,
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
    discoveredMcps,
    discoveredSkills,
    isDiscovering,
    runDiscovery,
    importDiscoveredMcp,
    importDiscoveredSkill
  } = useWorkspace();

  const activeNav = settingsActiveSection || 'agents';
  const setActiveNav = (sec: string) => setSettingsActiveSection(sec);

  // Provider Form State
  const [isAddingProvider, setIsAddingProvider] = useState(false);
  const [newProviderName, setNewProviderName] = useState('');
  const [newProviderBaseUrl, setNewProviderBaseUrl] = useState('');
  const [newProviderApiKey, setNewProviderApiKey] = useState('');
  const [fetchingProviderId, setFetchingProviderId] = useState<string | null>(null);

  // MCP Form State
  const [isAddingMcp, setIsAddingMcp] = useState(false);
  const [newMcpName, setNewMcpName] = useState('');
  const [newMcpCommand, setNewMcpCommand] = useState('');

  // Skill Form State
  const [isAddingSkill, setIsAddingSkill] = useState(false);
  const [newSkillName, setNewSkillName] = useState('');
  const [newSkillTrigger, setNewSkillTrigger] = useState('');
  const [newSkillDesc, setNewSkillDesc] = useState('');
  const [newSkillInstructions, setNewSkillInstructions] = useState('');

  // Rules state
  const [rules] = useState<RuleEntry[]>([
    { id: 'rule-1', title: 'Always write TypeScript strict mode code', content: 'Ensure all new interfaces and types avoid any.', enabled: true, category: 'Code Quality' },
    { id: 'rule-2', title: 'Follow Clean Architecture modularization', content: 'Separate UI components, state stores, and backend bridges.', enabled: true, category: 'Architecture' }
  ]);

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

    // Automatically fetch models from newly added provider
    setFetchingProviderId(newProvId);
    try {
      await fetchProviderModels(newProvId);
    } finally {
      setFetchingProviderId(null);
    }
  };

  const handleFetchModels = async (provId: string) => {
    setFetchingProviderId(provId);
    try {
      await fetchProviderModels(provId);
    } finally {
      setFetchingProviderId(null);
    }
  };

  const handleCreateMcp = (e: React.FormEvent) => {
    e.preventDefault();
    if (!newMcpName.trim() || !newMcpCommand.trim()) return;
    const newEntry: MCPEntry = {
      id: `mcp-${Date.now()}`,
      name: newMcpName.trim(),
      command: newMcpCommand.trim(),
      status: 'connected',
      enabled: true,
      version: 'v1.0.0',
      origin: 'custom',
      tools: ['execute', 'query']
    };
    addMcp(newEntry);
    setNewMcpName('');
    setNewMcpCommand('');
    setIsAddingMcp(false);
  };

  const handleCreateSkill = (e: React.FormEvent) => {
    e.preventDefault();
    if (!newSkillName.trim() || !newSkillDesc.trim()) return;
    const newEntry: SkillEntry = {
      id: `skill-${Date.now()}`,
      name: newSkillName.trim(),
      category: 'Custom',
      description: newSkillDesc.trim(),
      instructions: newSkillInstructions.trim() || undefined,
      trigger: newSkillTrigger.trim() || newSkillName.toLowerCase().replace(/\s+/g, '-'),
      enabled: true,
      origin: 'custom'
    };
    addSkill(newEntry);
    setNewSkillName('');
    setNewSkillTrigger('');
    setNewSkillDesc('');
    setNewSkillInstructions('');
    setIsAddingSkill(false);
  };

  return (
    <div className="h-full w-full bg-[#f8fafc] dark:bg-[#181818] text-[#111827] dark:text-[#e5e7eb] flex flex-col md:flex-row overflow-hidden select-none font-sans transition-colors duration-150">
      
      {/* Left Sidebar Navigation */}
      <div className="w-full md:w-64 min-w-[240px] bg-[#f1f5f9] dark:bg-[#1e1e1e] border-b md:border-b-0 md:border-r border-[#e2e8f0] dark:border-[#2b2b2b] p-3 flex flex-col justify-between">
        <div className="space-y-1">
          <div className="px-3 py-2 mb-2">
            <h2 className="text-sm font-bold text-[#0f172a] dark:text-white flex items-center gap-2">
              <Settings className="w-4 h-4 text-[#3b82f6]" />
              Settings & Config
            </h2>
            <p className="text-[11px] text-[#64748b] dark:text-[#94a3b8] mt-0.5">
              ACP agents, providers, MCPs & skills
            </p>
          </div>

          {/* Nav Items */}
          {[
            { id: 'agents', label: 'ACP Agents (Pi, OMP, OpenCode)', icon: Bot, badge: '3 Available' },
            { id: 'providers', label: 'Providers & API Keys', icon: Key, badge: `${providers.length}` },
            { id: 'models', label: 'Models & Selection', icon: Cpu, badge: currentModel },
            { id: 'mcps', label: 'MCP Discovery & Servers', icon: Layers, badge: `${mcps.filter(m => m.enabled).length} Active` },
            { id: 'skills', label: 'Skills & Discovery', icon: Sparkles, badge: `${skills.filter(s => s.enabled).length}` },
            { id: 'privacy', label: 'Privacy & Sharing', icon: Shield },
            { id: 'rules', label: 'Rules & Instructions', icon: FileText },
            { id: 'editor', label: 'Editor & Appearance', icon: Code2 }
          ].map(item => {
            const Icon = item.icon;
            const isActive = activeNav === item.id;
            return (
              <button
                key={item.id}
                type="button"
                onClick={() => setActiveNav(item.id)}
                className={`w-full text-left px-3 py-2 rounded-xl text-xs font-medium flex items-center justify-between transition-all cursor-pointer ${
                  isActive
                    ? 'bg-white dark:bg-[#282828] text-[#2563eb] dark:text-white shadow-xs font-semibold'
                    : 'text-[#475569] dark:text-[#a1a1aa] hover:bg-[#e2e8f0] dark:hover:bg-[#252525]'
                }`}
              >
                <div className="flex items-center gap-2.5 min-w-0">
                  <Icon className="w-4 h-4 text-[#3b82f6] shrink-0" />
                  <span className="truncate">{item.label}</span>
                </div>
                {item.badge && (
                  <span className="px-1.5 py-0.5 rounded text-[10px] font-bold bg-[#eff6ff] text-[#2563eb] dark:bg-[#1e293b] dark:text-[#60a5fa] truncate max-w-[80px]">
                    {item.badge}
                  </span>
                )}
              </button>
            );
          })}
        </div>

        {/* Footer info */}
        <div className="p-3 rounded-xl bg-white dark:bg-[#252526] border border-[#e2e8f0] dark:border-[#333333] text-[11px] space-y-1.5 mt-4">
          <div className="flex items-center justify-between">
            <span className="text-[#64748b] dark:text-[#a1a1aa]">ACP Protocol</span>
            <span className="text-[#10b981] font-semibold flex items-center gap-1">
              <span className="w-2 h-2 rounded-full bg-[#10b981] animate-pulse" />
              Connected
            </span>
          </div>
          <div className="flex items-center justify-between">
            <span className="text-[#64748b] dark:text-[#a1a1aa]">Active Agent</span>
            <span className="font-semibold text-[#0f172a] dark:text-white truncate max-w-[110px]">
              {agents.find(a => a.id === activeAgentId)?.name || 'Pi Agent'}
            </span>
          </div>
        </div>
      </div>

      {/* Main Content Area */}
      <div className="flex-1 overflow-y-auto p-4 sm:p-6 md:p-8">
        
        {/* SECTION: ACP AGENTS (Only Pi, OhMyPi, OpenCode) */}
        {activeNav === 'agents' && (
          <div className="max-w-3xl space-y-6 animate-in fade-in duration-200">
            <div>
              <h1 className="text-xl font-bold text-[#0f172a] dark:text-white tracking-tight flex items-center gap-2">
                <Bot className="w-5 h-5 text-[#3b82f6]" />
                Available ACP Agents
              </h1>
              <p className="text-xs text-[#64748b] dark:text-[#94a3b8] mt-1">
                Only available Agent Client Protocol (ACP) agents: <strong>Pi</strong>, <strong>OhMyPi (omp)</strong>, and <strong>OpenCode</strong>.
              </p>
            </div>

            {/* Agent Cards */}
            <div className="space-y-3">
              {agents.map(agent => {
                const isActive = agent.id === activeAgentId;
                return (
                  <div
                    key={agent.id}
                    className={`p-4 rounded-2xl border transition-all ${
                      isActive
                        ? 'bg-white dark:bg-[#1e1e1e] border-[#2563eb] dark:border-[#38bdf8] shadow-md ring-1 ring-[#2563eb]/20'
                        : 'bg-white dark:bg-[#1e1e1e] border-[#e2e8f0] dark:border-[#2b2b2b]'
                    }`}
                  >
                    <div className="flex items-start justify-between gap-4">
                      <div className="flex items-start gap-3">
                        <div className="w-10 h-10 rounded-xl bg-[#eff6ff] dark:bg-[#1e293b] flex items-center justify-center text-[#2563eb] dark:text-[#38bdf8] shrink-0 font-bold">
                          {agent.name.slice(0, 2).toUpperCase()}
                        </div>
                        <div className="space-y-1">
                          <div className="flex items-center gap-2">
                            <h3 className="text-sm font-bold text-[#0f172a] dark:text-white">{agent.name}</h3>
                            <span className="text-[10px] px-2 py-0.5 rounded-full bg-[#ecfdf5] text-[#059669] dark:bg-[#064e3b] dark:text-[#a7f3d0] font-semibold">
                              ACP Ready
                            </span>
                            {agent.isStarred && (
                              <span className="text-[10px] px-1.5 py-0.2 rounded bg-[#fef3c7] text-[#b45309] dark:bg-[#3d2c14] dark:text-[#fde68a]">
                                ★ Starred
                              </span>
                            )}
                          </div>
                          <p className="text-xs text-[#64748b] dark:text-[#94a3b8] leading-relaxed">
                            {agent.description}
                          </p>
                          <div className="flex items-center gap-3 pt-1 text-[11px] text-[#64748b] dark:text-[#94a3b8] font-mono flex-wrap">
                            <span>Provider: <strong className="text-[#0f172a] dark:text-white">{agent.type === 'internal' ? (providers.find(p => p.enabled)?.name || 'Custom (Direct LLM)') : `${agent.name} Engine`}</strong></span>
                            <span>Endpoint: <strong className="text-[#0f172a] dark:text-white">{agent.type === 'internal' ? (providers.find(p => p.enabled)?.baseUrl || 'Direct API') : (agent.endpoint || 'stdio')}</strong></span>
                            <span>Model: <strong className="text-[#0f172a] dark:text-white">{currentModel || 'Auto-detected'}</strong></span>
                          </div>
                        </div>
                      </div>

                      <div className="flex items-center gap-2 shrink-0">
                        <button
                          type="button"
                          onClick={() => setActiveAgentId(agent.id)}
                          className={`px-3 py-1.5 rounded-xl text-xs font-semibold transition-colors cursor-pointer ${
                            isActive
                              ? 'bg-[#2563eb] text-white shadow-xs'
                              : 'bg-[#f1f5f9] dark:bg-[#2a2a2c] text-[#334155] dark:text-[#d1d5db] hover:bg-[#e2e8f0]'
                          }`}
                        >
                          {isActive ? 'Active Agent' : 'Set as Active'}
                        </button>
                      </div>
                    </div>
                  </div>
                );
              })}
            </div>
          </div>
        )}

        {/* SECTION: PROVIDERS & KEYS (With Model Fetching) */}
        {activeNav === 'providers' && (
          <div className="max-w-3xl space-y-6 animate-in fade-in duration-200">
            <div className="flex items-center justify-between">
              <div>
                <h1 className="text-xl font-bold text-[#0f172a] dark:text-white tracking-tight flex items-center gap-2">
                  <Key className="w-5 h-5 text-[#3b82f6]" />
                  LLM Providers & API Keys
                </h1>
                <p className="text-xs text-[#64748b] dark:text-[#94a3b8] mt-1">
                  Configure Google Gemini, OpenAI / Ollama, Anthropic Claude, and fetch model lists automatically.
                </p>
              </div>
              <button
                type="button"
                onClick={() => setIsAddingProvider(true)}
                className="px-3 py-1.5 rounded-xl bg-[#2563eb] text-white text-xs font-semibold flex items-center gap-1.5 hover:bg-[#1d4ed8] transition-colors cursor-pointer shadow-xs"
              >
                <Plus className="w-4 h-4" />
                <span>Add Provider</span>
              </button>
            </div>

            {/* Add Provider Modal Form */}
            {isAddingProvider && (
              <form onSubmit={handleCreateProvider} className="p-4 rounded-2xl bg-white dark:bg-[#1e1e1e] border border-[#2563eb] shadow-lg space-y-3">
                <h3 className="text-xs font-bold text-[#0f172a] dark:text-white uppercase tracking-wider">New Provider</h3>
                <div className="grid grid-cols-1 md:grid-cols-2 gap-3">
                  <div>
                    <label className="text-[11px] font-semibold text-[#64748b] dark:text-[#94a3b8]">Provider Name</label>
                    <input
                      type="text"
                      placeholder="e.g. Local Ollama or DeepSeek"
                      value={newProviderName}
                      onChange={e => setNewProviderName(e.target.value)}
                      className="w-full px-3 py-1.5 rounded-xl bg-[#f8fafc] dark:bg-[#252526] border border-[#cbd5e1] dark:border-[#383838] text-xs text-[#0f172a] dark:text-white focus:outline-hidden focus:ring-2 focus:ring-[#2563eb]"
                      required
                    />
                  </div>
                  <div>
                    <label className="text-[11px] font-semibold text-[#64748b] dark:text-[#94a3b8]">Base URL (Optional)</label>
                    <input
                      type="text"
                      placeholder="http://localhost:11434/v1"
                      value={newProviderBaseUrl}
                      onChange={e => setNewProviderBaseUrl(e.target.value)}
                      className="w-full px-3 py-1.5 rounded-xl bg-[#f8fafc] dark:bg-[#252526] border border-[#cbd5e1] dark:border-[#383838] text-xs text-[#0f172a] dark:text-white focus:outline-hidden focus:ring-2 focus:ring-[#2563eb]"
                    />
                  </div>
                  <div>
                    <label className="text-[11px] font-semibold text-[#64748b] dark:text-[#94a3b8]">API Key (Optional for local)</label>
                    <input
                      type="password"
                      placeholder="sk-... or AIza..."
                      value={newProviderApiKey}
                      onChange={e => setNewProviderApiKey(e.target.value)}
                      className="w-full px-3 py-1.5 rounded-xl bg-[#f8fafc] dark:bg-[#252526] border border-[#cbd5e1] dark:border-[#383838] text-xs text-[#0f172a] dark:text-white focus:outline-hidden focus:ring-2 focus:ring-[#2563eb]"
                    />
                  </div>
                </div>
                <div className="flex justify-end gap-2 pt-2">
                  <button
                    type="button"
                    onClick={() => setIsAddingProvider(false)}
                    className="px-3 py-1.5 rounded-xl hover:bg-[#f1f5f9] dark:hover:bg-[#2a2a2c] text-xs text-[#64748b]"
                  >
                    Cancel
                  </button>
                  <button
                    type="submit"
                    className="px-4 py-1.5 rounded-xl bg-[#2563eb] text-white text-xs font-semibold hover:bg-[#1d4ed8]"
                  >
                    Save Provider
                  </button>
                </div>
              </form>
            )}

            {/* Provider List */}
            <div className="space-y-4">
              {providers.length === 0 ? (
                <div className="p-8 text-center rounded-2xl bg-white dark:bg-[#1e1e1e] border border-dashed border-[#e2e8f0] dark:border-[#2b2b2b] space-y-2">
                  <p className="text-sm font-semibold text-[#0f172a] dark:text-white">No LLM Providers Configured</p>
                  <p className="text-xs text-[#64748b] dark:text-[#94a3b8]">
                    Click "+ Add Custom Provider" above to add your Ollama, OpenRouter, Gemini, or OpenAI-compatible server.
                  </p>
                </div>
              ) : (
                providers.map(prov => (
                  <div
                    key={prov.id}
                    className="p-4 rounded-2xl bg-white dark:bg-[#1e1e1e] border border-[#e2e8f0] dark:border-[#2b2b2b] shadow-xs space-y-3"
                  >
                    <div className="flex items-center justify-between">
                      <div className="flex items-center gap-2">
                        <span className="w-2.5 h-2.5 rounded-full bg-[#10b981]" />
                        <h3 className="text-sm font-bold text-[#0f172a] dark:text-white">{prov.name}</h3>
                      </div>
                      <div className="flex items-center gap-2">
                        <button
                          type="button"
                          onClick={() => handleFetchModels(prov.id)}
                          disabled={fetchingProviderId === prov.id}
                          className="px-3 py-1 rounded-xl bg-[#eff6ff] dark:bg-[#1e293b] text-[#2563eb] dark:text-[#38bdf8] text-xs font-semibold flex items-center gap-1.5 hover:bg-[#dbeafe] transition-colors cursor-pointer"
                        >
                          <RefreshCw className={`w-3.5 h-3.5 ${fetchingProviderId === prov.id ? 'animate-spin' : ''}`} />
                          <span>{fetchingProviderId === prov.id ? 'Fetching...' : 'Fetch Models'}</span>
                        </button>
                        <button
                          type="button"
                          onClick={() => deleteProvider(prov.id)}
                          title="Delete Provider"
                          className="p-1.5 rounded-xl hover:bg-[#fee2e2] dark:hover:bg-[#451a1a] text-[#ef4444] transition-colors cursor-pointer"
                        >
                          <Trash2 className="w-4 h-4" />
                        </button>
                        <label className="relative inline-flex items-center cursor-pointer ml-1">
                          <input
                            type="checkbox"
                            checked={prov.enabled}
                            onChange={e => updateProvider(prov.id, { enabled: e.target.checked })}
                            className="sr-only peer"
                          />
                          <div className="w-9 h-5 bg-[#cbd5e1] peer-focus:outline-hidden rounded-full peer dark:bg-[#383838] peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-[#cbd5e1] after:border after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:bg-[#2563eb]" />
                        </label>
                      </div>
                    </div>

                    <div className="grid grid-cols-1 md:grid-cols-2 gap-3 text-xs">
                      <div>
                        <label className="text-[11px] text-[#64748b] dark:text-[#94a3b8] font-medium">Base URL</label>
                        <input
                          type="text"
                          value={prov.baseUrl || ''}
                          onChange={e => updateProvider(prov.id, { baseUrl: e.target.value })}
                          placeholder="https://..."
                          className="w-full px-3 py-1.5 mt-0.5 rounded-xl bg-[#f8fafc] dark:bg-[#252526] border border-[#cbd5e1] dark:border-[#383838] text-xs font-mono text-[#0f172a] dark:text-white"
                        />
                      </div>
                      <div>
                        <label className="text-[11px] text-[#64748b] dark:text-[#94a3b8] font-medium">API Key</label>
                        <input
                          type="password"
                          value={prov.apiKey || ''}
                          onChange={e => updateProvider(prov.id, { apiKey: e.target.value })}
                          placeholder="Enter API Key"
                          className="w-full px-3 py-1.5 mt-0.5 rounded-xl bg-[#f8fafc] dark:bg-[#252526] border border-[#cbd5e1] dark:border-[#383838] text-xs font-mono text-[#0f172a] dark:text-white"
                        />
                      </div>
                    </div>
                  </div>
                ))
              )}
            </div>
          </div>
        )}

        {/* SECTION: MODELS & SELECTION (Checkboxes + Default Selection) */}
        {activeNav === 'models' && (
          <div className="max-w-3xl space-y-6 animate-in fade-in duration-200">
            <div>
              <h1 className="text-xl font-bold text-[#0f172a] dark:text-white tracking-tight flex items-center gap-2">
                <Cpu className="w-5 h-5 text-[#3b82f6]" />
                Models & Selection (Checkboxes)
              </h1>
              <p className="text-xs text-[#64748b] dark:text-[#94a3b8] mt-1">
                Use checkboxes to enable or disable models available in the prompt bar and agent reasoning dropdown.
              </p>
            </div>

            {/* Current Active Default Model */}
            <div className="p-4 rounded-2xl bg-white dark:bg-[#1e1e1e] border border-[#2563eb] shadow-xs flex items-center justify-between">
              <div>
                <span className="text-[10px] font-semibold text-[#2563eb] uppercase tracking-wider">Active Default Reasoning Model</span>
                <h3 className="text-base font-bold text-[#0f172a] dark:text-white font-mono mt-0.5">{currentModel}</h3>
              </div>
              <span className="px-3 py-1 rounded-xl bg-[#eff6ff] text-[#2563eb] dark:bg-[#1e293b] dark:text-[#38bdf8] text-xs font-semibold">
                Current Active
              </span>
            </div>



            {/* Model Selection Groups with Checkboxes */}
            <div className="space-y-4">
              {providers.length === 0 ? (
                <div className="p-8 text-center rounded-2xl bg-white dark:bg-[#1e1e1e] border border-dashed border-[#e2e8f0] dark:border-[#2b2b2b] space-y-2">
                  <p className="text-sm font-semibold text-[#0f172a] dark:text-white">No Providers Configured</p>
                  <p className="text-xs text-[#64748b] dark:text-[#94a3b8]">
                    Add a provider in the "Providers & API Keys" tab to start managing models.
                  </p>
                </div>
              ) : (
                providers.map(prov => {
                const selectedList = prov.selectedModels || prov.models;
                return (
                  <div key={prov.id} className="p-4 rounded-2xl bg-white dark:bg-[#1e1e1e] border border-[#e2e8f0] dark:border-[#2b2b2b] shadow-xs space-y-3">
                    <div className="flex items-center justify-between border-b border-[#f3f4f6] dark:border-[#2f2f31] pb-2">
                      <div className="flex items-center gap-2">
                        <span className="w-2.5 h-2.5 rounded-full bg-[#3b82f6]" />
                        <h3 className="text-xs font-bold text-[#0f172a] dark:text-white uppercase tracking-wider">{prov.name} Models</h3>
                      </div>
                      <span className="text-[11px] text-[#64748b] dark:text-[#94a3b8] font-mono">
                        {selectedList.length} of {prov.models.length} Enabled
                      </span>
                    </div>

                    <div className="grid grid-cols-1 sm:grid-cols-2 gap-2">
                      {prov.models.map(m => {
                        const isChecked = selectedList.includes(m);
                        const isDefault = currentModel === m;
                        return (
                          <div
                            key={m}
                            className={`p-2.5 rounded-xl border flex items-center justify-between transition-colors ${
                              isChecked
                                ? 'bg-[#f8fafc] dark:bg-[#252526] border-[#cbd5e1] dark:border-[#383838]'
                                : 'bg-transparent border-dashed border-[#e2e8f0] dark:border-[#2f2f31] opacity-50'
                            }`}
                          >
                            <label className="flex items-center gap-2.5 cursor-pointer select-none flex-1 min-w-0">
                              <input
                                type="checkbox"
                                checked={isChecked}
                                onChange={() => toggleModelSelection(prov.id, m)}
                                className="w-4 h-4 text-[#2563eb] rounded border-[#cbd5e1] focus:ring-0 cursor-pointer"
                              />
                              <span className="text-xs font-mono font-medium text-[#0f172a] dark:text-white truncate">{m}</span>
                            </label>

                            {isChecked && (
                              <button
                                type="button"
                                onClick={() => setCurrentModel(m)}
                                className={`text-[10px] px-2 py-0.5 rounded font-semibold transition-colors cursor-pointer shrink-0 ${
                                  isDefault
                                    ? 'bg-[#2563eb] text-white'
                                    : 'bg-[#f1f5f9] dark:bg-[#333336] text-[#64748b] dark:text-[#9ca3af] hover:text-[#0f172a] dark:hover:text-white'
                                }`}
                              >
                                {isDefault ? 'Default' : 'Set Default'}
                              </button>
                            )}
                          </div>
                        );
                      })}
                    </div>
                  </div>
                );
              }))}
            </div>
          </div>
        )}

        {/* SECTION: MCP DISCOVERY & SERVERS */}
        {activeNav === 'mcps' && (
          <div className="max-w-3xl space-y-6 animate-in fade-in duration-200">
            <div className="flex items-center justify-between">
              <div>
                <h1 className="text-xl font-bold text-[#0f172a] dark:text-white tracking-tight flex items-center gap-2">
                  <Layers className="w-5 h-5 text-[#3b82f6]" />
                  MCP Discovery & Servers
                </h1>
                <p className="text-xs text-[#64748b] dark:text-[#94a3b8] mt-1">
                  Discover and attach MCP servers from other agent ecosystems (Antigravity, OpenCode, Pi, Claude Code, Codex).
                </p>
              </div>
              <div className="flex items-center gap-2">
                <button
                  type="button"
                  onClick={runDiscovery}
                  disabled={isDiscovering}
                  className="px-3 py-1.5 rounded-xl bg-[#eff6ff] dark:bg-[#1e293b] text-[#2563eb] dark:text-[#38bdf8] text-xs font-semibold flex items-center gap-1.5 hover:bg-[#dbeafe] transition-colors cursor-pointer"
                >
                  <RefreshCw className={`w-3.5 h-3.5 ${isDiscovering ? 'animate-spin' : ''}`} />
                  <span>Scan Other Agents</span>
                </button>
                <button
                  type="button"
                  onClick={() => setIsAddingMcp(true)}
                  className="px-3 py-1.5 rounded-xl bg-[#2563eb] text-white text-xs font-semibold flex items-center gap-1.5 hover:bg-[#1d4ed8] transition-colors cursor-pointer shadow-xs"
                >
                  <Plus className="w-4 h-4" />
                  <span>Add MCP</span>
                </button>
              </div>
            </div>

            {/* Discovered MCPs Box */}
            {discoveredMcps.length > 0 && (
              <div className="p-4 rounded-2xl bg-gradient-to-r from-[#eff6ff]/60 to-[#f5f3ff]/60 dark:from-[#1e293b]/40 dark:to-[#2e1065]/20 border border-[#bfdbfe] dark:border-[#3b82f6]/30 shadow-xs space-y-3">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <Sparkles className="w-4 h-4 text-[#2563eb] dark:text-[#38bdf8]" />
                    <h3 className="text-xs font-bold text-[#0f172a] dark:text-white uppercase tracking-wider">
                      Discovered MCPs from Other Agents ({discoveredMcps.length})
                    </h3>
                  </div>
                  <span className="text-[10px] text-[#2563eb] dark:text-[#93c5fd] font-medium">1-Click Import</span>
                </div>

                <div className="grid grid-cols-1 sm:grid-cols-2 gap-2.5">
                  {discoveredMcps.map(disc => {
                    const isImported = mcps.some(m => m.name === disc.name || m.command === disc.command);
                    return (
                      <div key={disc.id} className="p-3 rounded-xl bg-white/90 dark:bg-[#1e1e1e]/90 border border-[#e2e8f0] dark:border-[#383838] flex items-center justify-between gap-2 shadow-xs">
                        <div className="min-w-0 flex-1">
                          <div className="flex items-center gap-1.5">
                            <span className="font-semibold text-xs text-[#0f172a] dark:text-white truncate">{disc.name}</span>
                          </div>
                          <p className="text-[10px] font-mono text-[#64748b] dark:text-[#94a3b8] truncate mt-0.5">{disc.command}</p>
                        </div>
                        <button
                          type="button"
                          disabled={isImported}
                          onClick={() => importDiscoveredMcp(disc)}
                          className={`px-2.5 py-1 rounded-lg text-xs font-semibold shrink-0 transition-colors cursor-pointer ${
                            isImported
                              ? 'bg-[#ecfdf5] text-[#059669] dark:bg-[#064e3b] dark:text-[#a7f3d0]'
                              : 'bg-[#2563eb] text-white hover:bg-[#1d4ed8]'
                          }`}
                        >
                          {isImported ? 'Imported' : 'Import'}
                        </button>
                      </div>
                    );
                  })}
                </div>
              </div>
            )}

            {/* Active MCP List */}
            <div className="space-y-3">
              <h3 className="text-xs font-bold text-[#0f172a] dark:text-white uppercase tracking-wider">Active MCP Servers</h3>
              {mcps.map(mcp => (
                <div
                  key={mcp.id}
                  className="p-4 rounded-2xl bg-white dark:bg-[#1e1e1e] border border-[#e2e8f0] dark:border-[#2b2b2b] shadow-xs space-y-2"
                >
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <span className="w-2.5 h-2.5 rounded-full bg-[#10b981]" />
                      <h3 className="text-sm font-bold text-[#0f172a] dark:text-white">{mcp.name}</h3>
                      {mcp.origin && (
                        <span className="text-[10px] px-1.5 py-0.2 rounded bg-[#eff6ff] text-[#2563eb] dark:bg-[#1e293b] dark:text-[#60a5fa] font-mono">
                          {mcp.origin}
                        </span>
                      )}
                    </div>
                    <div className="flex items-center gap-2">
                      <button
                        type="button"
                        onClick={() => toggleMcp(mcp.id)}
                        className={`px-2.5 py-1 rounded-lg text-xs font-semibold cursor-pointer ${
                          mcp.enabled ? 'bg-[#ecfdf5] text-[#059669]' : 'bg-[#f1f5f9] text-[#64748b]'
                        }`}
                      >
                        {mcp.enabled ? 'Enabled' : 'Disabled'}
                      </button>
                      <button
                        type="button"
                        onClick={() => deleteMcp(mcp.id)}
                        className="p-1 text-[#9ca3af] hover:text-[#ef4444] rounded-lg cursor-pointer"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </div>
                  </div>
                  <p className="text-xs font-mono text-[#64748b] dark:text-[#94a3b8] bg-[#f8fafc] dark:bg-[#252526] p-2 rounded-xl">
                    {mcp.command}
                  </p>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* SECTION: SKILLS & DISCOVERY */}
        {activeNav === 'skills' && (
          <div className="max-w-3xl space-y-6 animate-in fade-in duration-200">
            <div className="flex items-center justify-between">
              <div>
                <h1 className="text-xl font-bold text-[#0f172a] dark:text-white tracking-tight flex items-center gap-2">
                  <Sparkles className="w-5 h-5 text-[#8b5cf6]" />
                  Skills & Discovery
                </h1>
                <p className="text-xs text-[#64748b] dark:text-[#94a3b8] mt-1">
                  Discover and import skills from Google Antigravity, Pi Agent Core, OpenCode, and Claude Code.
                </p>
              </div>
              <div className="flex items-center gap-2">
                <button
                  type="button"
                  onClick={runDiscovery}
                  disabled={isDiscovering}
                  className="px-3 py-1.5 rounded-xl bg-[#eff6ff] dark:bg-[#1e293b] text-[#2563eb] dark:text-[#38bdf8] text-xs font-semibold flex items-center gap-1.5 hover:bg-[#dbeafe] transition-colors cursor-pointer"
                >
                  <RefreshCw className={`w-3.5 h-3.5 ${isDiscovering ? 'animate-spin' : ''}`} />
                  <span>Scan Skills</span>
                </button>
                <button
                  type="button"
                  onClick={() => setIsAddingSkill(true)}
                  className="px-3 py-1.5 rounded-xl bg-[#2563eb] text-white text-xs font-semibold flex items-center gap-1.5 hover:bg-[#1d4ed8] transition-colors cursor-pointer shadow-xs"
                >
                  <Plus className="w-4 h-4" />
                  <span>Add Skill</span>
                </button>
              </div>
            </div>

            {/* Discovered Skills Box */}
            {discoveredSkills.length > 0 && (
              <div className="p-4 rounded-2xl bg-gradient-to-r from-[#faf5ff] to-[#f5f3ff] dark:from-[#2e1065]/20 dark:to-[#1e1b4b]/20 border border-[#e9d5ff] dark:border-[#8b5cf6]/30 shadow-xs space-y-3">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <Sparkles className="w-4 h-4 text-[#8b5cf6]" />
                    <h3 className="text-xs font-bold text-[#0f172a] dark:text-white uppercase tracking-wider">
                      Discovered Skills from Other Agents ({discoveredSkills.length})
                    </h3>
                  </div>
                  <span className="text-[10px] text-[#8b5cf6] font-medium">1-Click Add</span>
                </div>

                <div className="grid grid-cols-1 sm:grid-cols-2 gap-2.5">
                  {discoveredSkills.map(disc => {
                    const isImported = skills.some(s => s.trigger === disc.trigger || s.name === disc.name);
                    return (
                      <div key={disc.id} className="p-3 rounded-xl bg-white/90 dark:bg-[#1e1e1e]/90 border border-[#e2e8f0] dark:border-[#383838] flex items-center justify-between gap-2 shadow-xs">
                        <div className="min-w-0 flex-1">
                          <div className="flex items-center gap-1.5">
                            <span className="font-semibold text-xs text-[#0f172a] dark:text-white truncate">{disc.name}</span>
                          </div>
                          <p className="text-[11px] text-[#64748b] dark:text-[#94a3b8] truncate mt-0.5">{disc.description}</p>
                        </div>
                        <button
                          type="button"
                          disabled={isImported}
                          onClick={() => importDiscoveredSkill(disc)}
                          className={`px-2.5 py-1 rounded-lg text-xs font-semibold shrink-0 transition-colors cursor-pointer ${
                            isImported
                              ? 'bg-[#ecfdf5] text-[#059669] dark:bg-[#064e3b] dark:text-[#a7f3d0]'
                              : 'bg-[#8b5cf6] text-white hover:bg-[#7c3aed]'
                          }`}
                        >
                          {isImported ? 'Added' : 'Add Skill'}
                        </button>
                      </div>
                    );
                  })}
                </div>
              </div>
            )}

            {/* Active Skills List */}
            <div className="space-y-3">
              <h3 className="text-xs font-bold text-[#0f172a] dark:text-white uppercase tracking-wider">Installed Skills</h3>
              {skills.map(sk => (
                <div
                  key={sk.id}
                  className="p-4 rounded-2xl bg-white dark:bg-[#1e1e1e] border border-[#e2e8f0] dark:border-[#2b2b2b] shadow-xs space-y-2"
                >
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <h3 className="text-sm font-bold text-[#0f172a] dark:text-white">{sk.name}</h3>
                      <span className="text-[10px] px-2 py-0.5 rounded-md bg-[#eff6ff] text-[#2563eb] dark:bg-[#1e293b] dark:text-[#38bdf8] font-mono">
                        @{sk.trigger}
                      </span>
                    </div>
                    <div className="flex items-center gap-2">
                      <button
                        type="button"
                        onClick={() => toggleSkill(sk.id)}
                        className={`px-2.5 py-1 rounded-lg text-xs font-semibold cursor-pointer ${
                          sk.enabled ? 'bg-[#ecfdf5] text-[#059669]' : 'bg-[#f1f5f9] text-[#64748b]'
                        }`}
                      >
                        {sk.enabled ? 'Enabled' : 'Disabled'}
                      </button>
                      <button
                        type="button"
                        onClick={() => deleteSkill(sk.id)}
                        className="p-1 text-[#9ca3af] hover:text-[#ef4444] rounded-lg cursor-pointer"
                      >
                        <Trash2 className="w-4 h-4" />
                      </button>
                    </div>
                  </div>
                  <p className="text-xs text-[#64748b] dark:text-[#94a3b8]">
                    {sk.description}
                  </p>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* SECTION: PRIVACY & SHARING */}
        {activeNav === 'privacy' && (
          <div className="max-w-3xl space-y-6 animate-in fade-in duration-200">
            <div>
              <h1 className="text-xl font-bold text-[#0f172a] dark:text-white tracking-tight flex items-center gap-2">
                <Shield className="w-5 h-5 text-[#10b981]" />
                Privacy & Data Sharing
              </h1>
              <p className="text-xs text-[#64748b] dark:text-[#94a3b8] mt-1">
                Control terminal activity and user edit telemetry shared with connected agents.
              </p>
            </div>

            <div className="space-y-4">
              <div className="p-4 rounded-2xl bg-white dark:bg-[#1e1e1e] border border-[#e2e8f0] dark:border-[#2b2b2b] shadow-xs flex items-center justify-between">
                <div>
                  <h3 className="text-sm font-bold text-[#0f172a] dark:text-white">Share Terminal Activity</h3>
                  <p className="text-xs text-[#64748b] dark:text-[#94a3b8] mt-0.5">
                    Allow connected ACP agent to inspect stdout/stderr from Ghostty terminal executions.
                  </p>
                </div>
                <label className="relative inline-flex items-center cursor-pointer">
                  <input
                    type="checkbox"
                    checked={privacySettings.shareTerminalActivity}
                    onChange={e => setPrivacySettings(prev => ({ ...prev, shareTerminalActivity: e.target.checked }))}
                    className="sr-only peer"
                  />
                  <div className="w-9 h-5 bg-[#cbd5e1] peer-focus:outline-hidden rounded-full peer dark:bg-[#383838] peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-[#cbd5e1] after:border after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:bg-[#10b981]" />
                </label>
              </div>

              <div className="p-4 rounded-2xl bg-white dark:bg-[#1e1e1e] border border-[#e2e8f0] dark:border-[#2b2b2b] shadow-xs flex items-center justify-between">
                <div>
                  <h3 className="text-sm font-bold text-[#0f172a] dark:text-white">Share User Edits</h3>
                  <p className="text-xs text-[#64748b] dark:text-[#94a3b8] mt-0.5">
                    Stream local file edits directly into active agent reasoning memory.
                  </p>
                </div>
                <label className="relative inline-flex items-center cursor-pointer">
                  <input
                    type="checkbox"
                    checked={privacySettings.shareUserEdits}
                    onChange={e => setPrivacySettings(prev => ({ ...prev, shareUserEdits: e.target.checked }))}
                    className="sr-only peer"
                  />
                  <div className="w-9 h-5 bg-[#cbd5e1] peer-focus:outline-hidden rounded-full peer dark:bg-[#383838] peer-checked:after:translate-x-full peer-checked:after:border-white after:content-[''] after:absolute after:top-[2px] after:left-[2px] after:bg-white after:border-[#cbd5e1] after:border after:rounded-full after:h-4 after:w-4 after:transition-all peer-checked:bg-[#10b981]" />
                </label>
              </div>
            </div>
          </div>
        )}

        {/* SECTION: RULES */}
        {activeNav === 'rules' && (
          <div className="max-w-3xl space-y-6 animate-in fade-in duration-200">
            <div>
              <h1 className="text-xl font-bold text-[#0f172a] dark:text-white tracking-tight flex items-center gap-2">
                <FileText className="w-5 h-5 text-[#0284c7]" />
                Coding Rules (~/.my-ade/rules)
              </h1>
              <p className="text-xs text-[#64748b] dark:text-[#94a3b8] mt-1">
                Project-level instructions and architectural conventions injected into agent system prompt.
              </p>
            </div>

            <div className="space-y-3">
              {rules.map(rule => (
                <div key={rule.id} className="p-4 rounded-2xl bg-white dark:bg-[#1e1e1e] border border-[#e2e8f0] dark:border-[#2b2b2b] shadow-xs space-y-1">
                  <h3 className="text-sm font-bold text-[#0f172a] dark:text-white">{rule.title}</h3>
                  <p className="text-xs text-[#64748b] dark:text-[#94a3b8]">{rule.content}</p>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* SECTION: EDITOR & APPEARANCE */}
        {activeNav === 'editor' && (
          <div className="max-w-3xl space-y-6 animate-in fade-in duration-200">
            <div>
              <h1 className="text-xl font-bold text-[#0f172a] dark:text-white tracking-tight flex items-center gap-2">
                <Code2 className="w-5 h-5 text-[#3b82f6]" />
                Editor & Appearance
              </h1>
              <p className="text-xs text-[#64748b] dark:text-[#94a3b8] mt-1">
                Customize code font size, indent spacing, and color theme.
              </p>
            </div>

            <div className="space-y-4">
              <div className="p-4 rounded-2xl bg-white dark:bg-[#1e1e1e] border border-[#e2e8f0] dark:border-[#2b2b2b] shadow-xs flex items-center justify-between">
                <div>
                  <h3 className="text-sm font-bold text-[#0f172a] dark:text-white">Color Theme</h3>
                  <p className="text-xs text-[#64748b] dark:text-[#94a3b8] mt-0.5">Toggle light and dark interface</p>
                </div>
                <button
                  type="button"
                  onClick={() => setTheme(theme === 'dark' ? 'light' : 'dark')}
                  className="px-3 py-1.5 rounded-xl border border-[#cbd5e1] dark:border-[#444444] flex items-center gap-2 text-xs font-semibold cursor-pointer hover:bg-[#f1f5f9] dark:hover:bg-[#2a2a2c]"
                >
                  {theme === 'dark' ? <Moon className="w-4 h-4 text-[#a855f7]" /> : <Sun className="w-4 h-4 text-[#eab308]" />}
                  <span>{theme === 'dark' ? 'Dark Mode' : 'Light Mode'}</span>
                </button>
              </div>
            </div>
          </div>
        )}

      </div>
    </div>
  );
};

export const DevinSettingsTab = MyAdeSettingsTab;
