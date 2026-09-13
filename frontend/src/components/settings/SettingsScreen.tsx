import React, { useState, useMemo } from 'react';
import {
  ArrowLeft,
  Settings2,
  Palette,
  Package,
  Globe,
  Monitor,
  BrainCircuit,
  Bot,
  Puzzle,
  Cable,
  Sparkles,
  Terminal,
  Anchor,
  ShieldCheck,
  ChartColumn,
  Rocket,
  RefreshCw,
  Eye,
  EyeOff,
  Plus,
  Trash2,
  KeyRound,
  Check,
  Sun,
  Moon,
  Search,
  ChevronDown,
  ChevronRight,
  Code2,
  Wrench,
  Layers,
  Compass
} from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';
import { LLMProviderConfig } from '../../types';

export const SettingsScreen: React.FC = () => {
  const {
    theme,
    setTheme,
    currentModel,
    setCurrentModel,
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
    toggleSkill,
    deleteSkill,
    createBackendSkill,
    deleteBackendSkill,
    reloadSkills,
    discoveredSkills,
    importDiscoveredSkill,
    runDiscovery,
    isDiscovering,
    plugins,
    createPlugin,
    togglePlugin,
    deletePlugin,
    reloadPlugins,
    agents,
    toggleAgentEnabled,
    activeWorkspacePath,
    settingsActiveSection,
    setSettingsActiveSection,
    goBackToWorkspace,
    privacySettings,
    setPrivacySettings
  } = useWorkspace();

  // Selected Provider in Model settings
  const [selectedProviderId, setSelectedProviderId] = useState<string>(() => {
    return providers[0]?.id || 'custom-myairouter';
  });

  const selectedProvider = useMemo(() => {
    return providers.find(p => p.id === selectedProviderId) || providers[0];
  }, [providers, selectedProviderId]);

  // Form states
  const [showApiKey, setShowApiKey] = useState(false);
  const [newModelName, setNewModelName] = useState('');
  const [isAddingModel, setIsAddingModel] = useState(false);
  const [isAddingProvider, setIsAddingProvider] = useState(false);
  const [newProviderName, setNewProviderName] = useState('');
  const [newProviderBaseUrl, setNewProviderBaseUrl] = useState('');
  const [newProviderApiKey, setNewProviderApiKey] = useState('');
  const [isRefreshing, setIsRefreshing] = useState(false);
  const [modelSearch, setModelSearch] = useState('');

  // New MCP form
  const [newMcpName, setNewMcpName] = useState('');
  const [newMcpCommand, setNewMcpCommand] = useState('');
  const [isAddingMcp, setIsAddingMcp] = useState(false);

  // Plugins state
  const [pluginSearch, setPluginSearch] = useState('');
  const [isAddingPlugin, setIsAddingPlugin] = useState(false);
  const [newPluginId, setNewPluginId] = useState('');
  const [newPluginName, setNewPluginName] = useState('');
  const [newPluginDesc, setNewPluginDesc] = useState('');
  const [newPluginScope, setNewPluginScope] = useState<'workspace' | 'global'>('workspace');
  const [newPluginPrompt, setNewPluginPrompt] = useState('');
  const [newPluginToolName, setNewPluginToolName] = useState('');
  const [newPluginToolDesc, setNewPluginToolDesc] = useState('');
  const [newPluginToolCmd, setNewPluginToolCmd] = useState('');
  const [isCreatingPlugin, setIsCreatingPlugin] = useState(false);
  const [pluginError, setPluginError] = useState('');
  const [isReloadingPlugins, setIsReloadingPlugins] = useState(false);

  // Skills state
  const [skillSearch, setSkillSearch] = useState('');
  const [isAddingSkill, setIsAddingSkill] = useState(false);
  const [newSkillName, setNewSkillName] = useState('');
  const [newSkillDesc, setNewSkillDesc] = useState('');
  const [newSkillScope, setNewSkillScope] = useState<'workspace' | 'global'>('workspace');
  const [newSkillPrompt, setNewSkillPrompt] = useState('');
  const [expandedSkillId, setExpandedSkillId] = useState<string | null>(null);
  const [isCreatingSkill, setIsCreatingSkill] = useState(false);
  const [skillError, setSkillError] = useState('');
  const [isReloadingSkills, setIsReloadingSkills] = useState(false);

  // Navigation items matching ref/settings.html
  const navSections = [
    {
      group: 'Basics',
      items: [
        { id: 'general', label: 'General', icon: Settings2 },
        { id: 'appearance', label: 'Appearance', icon: Palette },
        { id: 'model', label: 'Model settings', icon: Package },
        { id: 'browser', label: 'Browser Use', icon: Globe },
        { id: 'computer', label: 'Computer Use', icon: Monitor }
      ]
    },
    {
      group: 'Agent capabilities',
      items: [
        { id: 'memory', label: 'Memory', icon: BrainCircuit },
        { id: 'subagents', label: 'Subagents', icon: Bot },
        { id: 'plugins', label: 'Plugins', icon: Puzzle },
        { id: 'mcps', label: 'MCP Servers', icon: Cable },
        { id: 'skills', label: 'Skills', icon: Sparkles },
        { id: 'commands', label: 'Commands', icon: Terminal },
        { id: 'hooks', label: 'Hooks', icon: Anchor }
      ]
    },
    {
      group: 'Data and statistics',
      items: [
        { id: 'indexing', label: 'Indexing', icon: ShieldCheck },
        { id: 'usage', label: 'Usage stats', icon: ChartColumn }
      ]
    }
  ];

  const handleRefreshModels = async () => {
    if (!selectedProvider) return;
    setIsRefreshing(true);
    try {
      await fetchProviderModels(selectedProvider.id);
    } finally {
      setIsRefreshing(false);
    }
  };

  const handleCreateProvider = (e: React.FormEvent) => {
    e.preventDefault();
    if (!newProviderName.trim()) return;
    const newId = `provider-${Date.now()}`;
    const newP: LLMProviderConfig = {
      id: newId,
      name: newProviderName.trim(),
      baseUrl: newProviderBaseUrl.trim() || undefined,
      apiKey: newProviderApiKey.trim() || undefined,
      enabled: true,
      models: ['default-model'],
      selectedModels: ['default-model']
    };
    addProvider(newP);
    setSelectedProviderId(newId);
    setIsAddingProvider(false);
    setNewProviderName('');
    setNewProviderBaseUrl('');
    setNewProviderApiKey('');
  };

  const handleAddModel = (e: React.FormEvent) => {
    e.preventDefault();
    if (!newModelName.trim() || !selectedProvider) return;
    addModelToProvider(selectedProvider.id, newModelName.trim());
    setNewModelName('');
    setIsAddingModel(false);
  };

  const handleReloadPlugins = async () => {
    setIsReloadingPlugins(true);
    try {
      await reloadPlugins();
    } finally {
      setIsReloadingPlugins(false);
    }
  };

  const handleCreatePlugin = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newPluginId.trim() || !newPluginName.trim()) return;
    setIsCreatingPlugin(true);
    setPluginError('');
    try {
      const tools = newPluginToolName.trim() ? [{
        name: newPluginToolName.trim(),
        description: newPluginToolDesc.trim() || 'Custom plugin tool',
        handler_type: 'command' as const,
        command: newPluginToolCmd.trim() || 'echo "Plugin tool executed"'
      }] : [];

      await createPlugin({
        id: newPluginId.trim().toLowerCase().replace(/[^a-z0-9_-]/g, '-'),
        name: newPluginName.trim(),
        description: newPluginDesc.trim(),
        scope: newPluginScope,
        system_prompt: newPluginPrompt.trim() || undefined,
        tools: tools.length > 0 ? tools : undefined
      });

      setIsAddingPlugin(false);
      setNewPluginId('');
      setNewPluginName('');
      setNewPluginDesc('');
      setNewPluginPrompt('');
      setNewPluginToolName('');
      setNewPluginToolDesc('');
      setNewPluginToolCmd('');
    } catch (err: any) {
      setPluginError(err.message || 'Failed to create plugin');
    } finally {
      setIsCreatingPlugin(false);
    }
  };

  const handleReloadSkills = async () => {
    setIsReloadingSkills(true);
    try {
      await reloadSkills();
    } finally {
      setIsReloadingSkills(false);
    }
  };

  const handleCreateSkill = async (e: React.FormEvent) => {
    e.preventDefault();
    if (!newSkillName.trim() || !newSkillPrompt.trim()) return;
    setIsCreatingSkill(true);
    setSkillError('');
    try {
      await createBackendSkill({
        name: newSkillName.trim().toLowerCase().replace(/[^a-z0-9_-]/g, '-'),
        description: newSkillDesc.trim(),
        scope: newSkillScope,
        body: newSkillPrompt.trim()
      });

      setIsAddingSkill(false);
      setNewSkillName('');
      setNewSkillDesc('');
      setNewSkillPrompt('');
    } catch (err: any) {
      setSkillError(err.message || 'Failed to create skill');
    } finally {
      setIsCreatingSkill(false);
    }
  };

  // Provider logo rendering
  const renderProviderLogo = (name: string, size = 'sm') => {
    const letter = name.charAt(0).toUpperCase();
    const isZai = name.toLowerCase().includes('z.ai') || name.toLowerCase().includes('zai');
    const bg = isZai
      ? 'bg-[#111827] text-white dark:bg-[#F2F2F2] dark:text-[#111111]'
      : 'bg-[#E5E7EB] text-[#111827] dark:bg-[#2A2A2D] dark:text-[#F2F2F2]';

    if (size === 'lg') {
      return (
        <div className={`w-8 h-8 rounded-md ${bg} font-bold flex items-center justify-center text-lg shrink-0`}>
          {letter}
        </div>
      );
    }

    return (
      <div className={`w-[22px] h-[22px] rounded-[4px] ${bg} font-bold flex items-center justify-center text-[13px] shrink-0`}>
        {letter}
      </div>
    );
  };

  // Filtered models
  const visibleModels = useMemo(() => {
    if (!selectedProvider) return [];
    const models = selectedProvider.models || [];
    if (!modelSearch.trim()) return models;
    return models.filter(m => m.toLowerCase().includes(modelSearch.toLowerCase()));
  }, [selectedProvider, modelSearch]);

  return (
    <div className="flex-1 h-full w-full flex bg-[#F8F9FA] dark:bg-[#161617] text-[#111827] dark:text-[#F2F2F2] select-none font-sans overflow-hidden transition-colors">
      
      {/* LEFT SIDEBAR (Width 340px, matching ref/settings.html) */}
      <aside className="w-[340px] shrink-0 h-full flex flex-col justify-between p-[16px_20px] border-r border-[#E5E7EB] dark:border-[#333336] bg-[#FFFFFF] dark:bg-[#161617] overflow-y-auto transition-colors">
        <div className="flex flex-col w-full">
          
          {/* Back to workspace button */}
          <button
            type="button"
            onClick={goBackToWorkspace}
            className="w-full flex items-center gap-2.5 p-[8px_0px_24px_0px] text-[#4B5563] dark:text-[#9B9B9F] hover:text-[#111827] dark:hover:text-[#F2F2F2] transition-colors cursor-pointer group"
          >
            <ArrowLeft className="w-4 h-4 text-[#4B5563] dark:text-[#9B9B9F] group-hover:text-[#111827] dark:group-hover:text-[#F2F2F2] transition-colors" />
            <span className="text-[15px] font-normal">Back to workspace</span>
          </button>

          {/* Navigation Sections */}
          {navSections.map(sec => (
            <div key={sec.group} className="w-full flex flex-col mb-4">
              <div className="text-[13px] text-[#6B7280] dark:text-[#6B6B70] px-0 py-2 font-normal">
                {sec.group}
              </div>
              <div className="flex flex-col gap-1 w-full">
                {sec.items.map(item => {
                  const Icon = item.icon;
                  const isActive = settingsActiveSection === item.id;

                  return (
                    <button
                      key={item.id}
                      type="button"
                      onClick={() => setSettingsActiveSection(item.id)}
                      className={`w-full flex items-center gap-3 p-[10px_12px] rounded-[8px] text-[15px] transition-colors cursor-pointer ${
                        isActive
                          ? 'bg-[#F3F4F6] dark:bg-[#2A2A2D] text-[#111827] dark:text-[#F2F2F2] font-medium shadow-2xs'
                          : 'text-[#4B5563] dark:text-[#9B9B9F] hover:bg-[#F3F4F6] dark:hover:bg-[#1E1E20] hover:text-[#111827] dark:hover:text-[#F2F2F2]'
                      }`}
                    >
                      <Icon className="w-[17px] h-[17px] shrink-0" />
                      <span className="truncate">{item.label}</span>
                    </button>
                  );
                })}
              </div>
            </div>
          ))}
        </div>

        {/* Bottom Onboard Button */}
        <button
          type="button"
          onClick={() => setSettingsActiveSection('general')}
          className="w-full flex items-center gap-3 p-3 rounded-[10px] border border-[#E5E7EB] dark:border-[#333336] hover:bg-[#F3F4F6] dark:hover:bg-[#1E1E20] text-[#111827] dark:text-[#F2F2F2] transition-colors cursor-pointer mt-4"
        >
          <Rocket className="w-[17px] h-[17px] text-[#111827] dark:text-[#F2F2F2] shrink-0" />
          <span className="text-[15px]">Onboard</span>
        </button>
      </aside>

      {/* MAIN CONTENT AREA */}
      <main className="flex-1 h-full overflow-y-auto p-[40px_60px] lg:p-[56px_100px] flex flex-col gap-[28px] bg-[#F8F9FA] dark:bg-[#161617] transition-colors">
        
        {/* =========================================================================
            SECTION: MODEL SETTINGS (Ref: ref/settings.html)
            ========================================================================= */}
        {(settingsActiveSection === 'model' || settingsActiveSection === 'agents') && (
          <>
            <div className="text-[34px] font-bold text-[#111827] dark:text-[#F2F2F2]">
              Model settings
            </div>

            <div className="w-full flex items-center justify-between">
              <div className="text-[15px] text-[#4B5563] dark:text-[#9B9B9F]">
                Manage custom model providers. Once configured, they can be selected during chat.
              </div>
              <button
                type="button"
                onClick={handleRefreshModels}
                disabled={isRefreshing}
                className="p-2 rounded-lg text-[#4B5563] dark:text-[#9B9B9F] hover:text-[#111827] dark:hover:text-[#F2F2F2] hover:bg-[#E5E7EB] dark:hover:bg-[#1E1E20] transition-colors cursor-pointer"
                title="Refresh model list"
              >
                <RefreshCw className={`w-4 h-4 ${isRefreshing ? 'animate-spin text-[#16A34A] dark:text-[#4ADE80]' : ''}`} />
              </button>
            </div>

            {/* Inner Panel Split */}
            <div className="w-full border border-[#E5E7EB] dark:border-[#333336] rounded-[12px] overflow-hidden flex bg-[#FFFFFF] dark:bg-[#161617] shadow-xs">
              
              {/* Left Column: Providers List (Width 290px) */}
              <div className="w-[290px] shrink-0 border-r border-[#E5E7EB] dark:border-[#333336] p-[20px_16px] flex flex-col gap-2 bg-[#FAFAFA] dark:bg-[#161617]">
                <div className="text-[13px] text-[#6B7280] dark:text-[#6B6B70] mb-1">
                  Providers
                </div>

                {providers.map(p => {
                  const isSelected = selectedProvider?.id === p.id;

                  return (
                    <button
                      key={p.id}
                      type="button"
                      onClick={() => {
                        setSelectedProviderId(p.id);
                        setIsAddingProvider(false);
                      }}
                      className={`w-full flex items-center gap-2.5 p-[10px_12px] rounded-[8px] transition-colors cursor-pointer text-left ${
                        isSelected
                          ? 'border border-[#D1D5DB] dark:border-[#333336] bg-[#FFFFFF] dark:bg-[#1E1E20] shadow-xs'
                          : 'hover:bg-[#F3F4F6] dark:hover:bg-[#1E1E20]'
                      }`}
                    >
                      {renderProviderLogo(p.name)}
                      <span className="text-[15px] text-[#111827] dark:text-[#F2F2F2] flex-1 truncate">{p.name}</span>
                      {p.enabled && (
                        <div className="w-2 h-2 rounded-full bg-[#16A34A] dark:bg-[#4ADE80] shrink-0" />
                      )}
                    </button>
                  );
                })}

                {/* Add provider trigger */}
                <div className="text-[13px] text-[#6B7280] dark:text-[#6B6B70] mt-4 mb-1">
                  Custom providers
                </div>

                <button
                  type="button"
                  onClick={() => setIsAddingProvider(true)}
                  className="w-full flex items-center gap-2.5 p-[10px_12px] rounded-[8px] hover:bg-[#F3F4F6] dark:hover:bg-[#1E1E20] text-[#111827] dark:text-[#F2F2F2] transition-colors cursor-pointer text-left"
                >
                  <Plus className="w-[18px] h-[18px] text-[#6B7280] dark:text-[#9B9B9F] shrink-0" />
                  <span className="text-[15px]">Add provider</span>
                </button>
              </div>

              {/* Right Column: Provider Detail */}
              <div className="flex-1 p-[28px_36px] flex flex-col gap-5 overflow-y-auto bg-[#FFFFFF] dark:bg-[#161617]">
                {isAddingProvider ? (
                  /* Form to Add New Provider */
                  <form onSubmit={handleCreateProvider} className="flex flex-col gap-4">
                    <div className="text-[20px] font-bold text-[#111827] dark:text-[#F2F2F2]">
                      Add New Model Provider
                    </div>
                    <div className="flex flex-col gap-1.5">
                      <label className="text-[15px] text-[#4B5563] dark:text-[#9B9B9F]">Provider Name</label>
                      <input
                        type="text"
                        value={newProviderName}
                        onChange={e => setNewProviderName(e.target.value)}
                        placeholder="e.g. Ollama, OpenRouter, DeepSeek"
                        className="p-[12px_16px] bg-[#F9FAFB] dark:bg-[#1E1E20] border border-[#E5E7EB] dark:border-[#333336] rounded-[8px] text-[15px] text-[#111827] dark:text-[#F2F2F2] focus:outline-hidden"
                        required
                        autoFocus
                      />
                    </div>
                    <div className="flex flex-col gap-1.5">
                      <label className="text-[15px] text-[#4B5563] dark:text-[#9B9B9F]">Base URL</label>
                      <input
                        type="text"
                        value={newProviderBaseUrl}
                        onChange={e => setNewProviderBaseUrl(e.target.value)}
                        placeholder="https://api.openai.com/v1 or http://localhost:11434"
                        className="p-[12px_16px] bg-[#F9FAFB] dark:bg-[#1E1E20] border border-[#E5E7EB] dark:border-[#333336] rounded-[8px] text-[15px] text-[#111827] dark:text-[#F2F2F2] focus:outline-hidden font-mono"
                      />
                    </div>
                    <div className="flex flex-col gap-1.5">
                      <label className="text-[15px] text-[#4B5563] dark:text-[#9B9B9F]">API Key</label>
                      <input
                        type="password"
                        value={newProviderApiKey}
                        onChange={e => setNewProviderApiKey(e.target.value)}
                        placeholder="sk-..."
                        className="p-[12px_16px] bg-[#F9FAFB] dark:bg-[#1E1E20] border border-[#E5E7EB] dark:border-[#333336] rounded-[8px] text-[15px] text-[#111827] dark:text-[#F2F2F2] focus:outline-hidden"
                      />
                    </div>
                    <div className="flex items-center gap-3 mt-2">
                      <button
                        type="submit"
                        className="px-5 py-2 rounded-[8px] bg-[#16A34A] dark:bg-[#4ADE80] text-white dark:text-[#0E2A18] font-semibold text-[14px] hover:bg-[#15803D] dark:hover:bg-[#3ec472] transition-colors cursor-pointer"
                      >
                        Save Provider
                      </button>
                      <button
                        type="button"
                        onClick={() => setIsAddingProvider(false)}
                        className="px-4 py-2 rounded-[8px] border border-[#E5E7EB] dark:border-[#333336] text-[#4B5563] dark:text-[#9B9B9F] hover:text-[#111827] dark:hover:text-[#F2F2F2] hover:bg-[#F3F4F6] dark:hover:bg-[#1E1E20] text-[14px] cursor-pointer"
                      >
                        Cancel
                      </button>
                    </div>
                  </form>
                ) : selectedProvider ? (
                  /* Configured Provider Details */
                  <>
                    {/* Header with Provider Logo, Name, Enabled Badge, Connection Mode */}
                    <div className="w-full flex items-center justify-between pb-2">
                      <div className="flex items-center gap-3">
                        {renderProviderLogo(selectedProvider.name, 'lg')}
                        <div className="text-[20px] font-bold text-[#111827] dark:text-[#F2F2F2]">
                          {selectedProvider.name}
                        </div>
                        <button
                          type="button"
                          onClick={() => updateProvider(selectedProvider.id, { enabled: !selectedProvider.enabled })}
                          className={`px-3 py-1 rounded-full text-[14px] font-bold cursor-pointer transition-colors ${
                            selectedProvider.enabled
                              ? 'bg-[#DCFCE7] text-[#15803D] dark:bg-[#4ADE80] dark:text-[#0E2A18]'
                              : 'bg-[#F3F4F6] text-[#6B7280] dark:bg-[#2A2A2D] dark:text-[#9B9B9F]'
                          }`}
                        >
                          {selectedProvider.enabled ? 'Enabled' : 'Disabled'}
                        </button>
                      </div>

                      <div className="flex items-center gap-3.5">
                        <span className="text-[15px] text-[#4B5563] dark:text-[#9B9B9F]">Connection mode</span>
                        <div className="flex items-center gap-3 px-4 py-2 border border-[#E5E7EB] dark:border-[#333336] rounded-[8px] text-[14px] text-[#111827] dark:text-[#F2F2F2] bg-[#F9FAFB] dark:bg-[#1E1E20]">
                          <span>API key</span>
                        </div>
                      </div>
                    </div>

                    {/* Base URL Field */}
                    <div className="flex flex-col gap-1.5 w-full">
                      <label className="text-[15px] text-[#4B5563] dark:text-[#9B9B9F]">Base URL</label>
                      <input
                        type="text"
                        value={selectedProvider.baseUrl || ''}
                        onChange={e => updateProvider(selectedProvider.id, { baseUrl: e.target.value })}
                        placeholder="https://api.openai.com/v1"
                        className="w-full p-[12px_16px] bg-[#F9FAFB] dark:bg-[#1E1E20] border border-[#E5E7EB] dark:border-[#333336] rounded-[8px] text-[15px] text-[#111827] dark:text-[#F2F2F2] font-mono focus:outline-hidden"
                      />
                    </div>

                    {/* API Key Field */}
                    <div className="flex flex-col gap-1.5 w-full">
                      <label className="text-[15px] text-[#4B5563] dark:text-[#9B9B9F]">API key</label>
                      <div className="w-full flex items-center bg-[#F9FAFB] dark:bg-[#1E1E20] border border-[#E5E7EB] dark:border-[#333336] rounded-[8px] p-[6px_16px]">
                        <input
                          type={showApiKey ? 'text' : 'password'}
                          value={selectedProvider.apiKey || ''}
                          onChange={e => updateProvider(selectedProvider.id, { apiKey: e.target.value })}
                          placeholder="Enter API Key..."
                          className="flex-1 bg-transparent border-0 text-[15px] text-[#111827] dark:text-[#F2F2F2] py-2 focus:outline-hidden"
                        />
                        <button
                          type="button"
                          onClick={() => setShowApiKey(!showApiKey)}
                          className="p-1 text-[#6B7280] dark:text-[#9B9B9F] hover:text-[#111827] dark:hover:text-[#F2F2F2] cursor-pointer"
                        >
                          {showApiKey ? <EyeOff className="w-4 h-4" /> : <Eye className="w-4 h-4" />}
                        </button>
                      </div>
                    </div>

                    {/* Models List Section */}
                    <div className="flex flex-col gap-3 w-full pt-2">
                      <div className="flex items-center justify-between">
                        <span className="text-[15px] text-[#4B5563] dark:text-[#9B9B9F]">Model list</span>
                        <div className="flex items-center gap-2">
                          <button
                            type="button"
                            onClick={handleRefreshModels}
                            className="text-xs text-[#16A34A] dark:text-[#4ADE80] hover:underline cursor-pointer flex items-center gap-1 font-medium"
                          >
                            <RefreshCw className="w-3 h-3" />
                            <span>Fetch models</span>
                          </button>
                        </div>
                      </div>

                      {/* Model search if more than 4 models */}
                      {(selectedProvider.models || []).length > 4 && (
                        <div className="flex items-center gap-2 px-3 py-1.5 bg-[#F9FAFB] dark:bg-[#1E1E20] border border-[#E5E7EB] dark:border-[#333336] rounded-lg text-xs">
                          <Search className="w-3.5 h-3.5 text-[#6B7280] dark:text-[#6B6B70]" />
                          <input
                            type="text"
                            value={modelSearch}
                            onChange={e => setModelSearch(e.target.value)}
                            placeholder="Filter models..."
                            className="bg-transparent text-[#111827] dark:text-[#F2F2F2] text-xs focus:outline-hidden w-full"
                          />
                        </div>
                      )}

                      {/* Model Rows (matching ref/settings.html) */}
                      <div className="flex flex-col gap-2 max-h-[320px] overflow-y-auto pr-1">
                        {visibleModels.map(modelName => {
                          const isSelected = (selectedProvider.selectedModels || selectedProvider.models).includes(modelName);
                          const isCurrentActive = currentModel === modelName;

                          return (
                            <div
                              key={modelName}
                              className="w-full flex items-center gap-3 p-[12px_14px] bg-[#FFFFFF] dark:bg-[#1E1E20] border border-[#E5E7EB] dark:border-[#333336] rounded-[10px] group shadow-2xs"
                            >
                              <button
                                type="button"
                                onClick={() => toggleModelSelection(selectedProvider.id, modelName)}
                                className={`w-4 h-4 rounded-[4px] border flex items-center justify-center cursor-pointer transition-colors ${
                                  isSelected
                                    ? 'bg-[#16A34A] border-[#16A34A] dark:bg-[#4ADE80] dark:border-[#4ADE80] text-white dark:text-[#0E2A18]'
                                    : 'border-[#9CA3AF] dark:border-[#6B6B70] hover:border-[#111827] dark:hover:border-[#F2F2F2]'
                                }`}
                              >
                                {isSelected && <Check className="w-3 h-3 stroke-[3]" />}
                              </button>

                              <button
                                type="button"
                                onClick={() => setCurrentModel(modelName)}
                                className={`text-[15px] font-mono flex-1 text-left cursor-pointer truncate ${
                                  isCurrentActive ? 'text-[#16A34A] dark:text-[#4ADE80] font-semibold' : 'text-[#111827] dark:text-[#F2F2F2]'
                                }`}
                                title="Click to make primary active chat model"
                              >
                                {modelName}
                                {isCurrentActive && <span className="text-xs ml-2 font-sans text-[#16A34A] dark:text-[#4ADE80]">(Active)</span>}
                              </button>

                              {/* Tags */}
                              <div className="flex items-center gap-1.5 shrink-0">
                                {modelName.toLowerCase().includes('vision') && (
                                  <span className="text-[12px] text-[#4B5563] dark:text-[#9B9B9F] bg-[#F3F4F6] dark:bg-[#2A2A2D] px-2 py-0.5 rounded-full">
                                    Vision
                                  </span>
                                )}
                                <span className="text-[12px] text-[#4B5563] dark:text-[#9B9B9F] bg-[#F3F4F6] dark:bg-[#2A2A2D] px-2 py-0.5 rounded-full">
                                  1M
                                </span>
                              </div>

                              <KeyRound className="w-4 h-4 text-[#9CA3AF] dark:text-[#6B6B70] shrink-0" />

                              <button
                                type="button"
                                onClick={() => deleteModelFromProvider(selectedProvider.id, modelName)}
                                className="p-1 text-[#9CA3AF] dark:text-[#6B6B70] hover:text-[#DC2626] dark:hover:text-[#EF4444] transition-colors cursor-pointer"
                                title="Remove model"
                              >
                                <Trash2 className="w-4 h-4" />
                              </button>
                            </div>
                          );
                        })}
                      </div>

                      {/* Add Model Form */}
                      {isAddingModel ? (
                        <form onSubmit={handleAddModel} className="flex items-center gap-2 mt-2">
                          <input
                            type="text"
                            value={newModelName}
                            onChange={e => setNewModelName(e.target.value)}
                            placeholder="Model identifier (e.g. gpt-4o, claude-3-5-sonnet)"
                            className="flex-1 p-[10px_14px] bg-[#F9FAFB] dark:bg-[#1E1E20] border border-[#E5E7EB] dark:border-[#333336] rounded-[8px] text-[14px] text-[#111827] dark:text-[#F2F2F2] font-mono focus:outline-hidden"
                            autoFocus
                          />
                          <button
                            type="submit"
                            className="px-4 py-2 bg-[#16A34A] dark:bg-[#4ADE80] text-white dark:text-[#0E2A18] rounded-[8px] text-xs font-bold hover:bg-[#15803D] dark:hover:bg-[#3ec472] cursor-pointer"
                          >
                            Add
                          </button>
                          <button
                            type="button"
                            onClick={() => setIsAddingModel(false)}
                            className="px-3 py-2 border border-[#E5E7EB] dark:border-[#333336] text-[#4B5563] dark:text-[#9B9B9F] rounded-[8px] text-xs hover:bg-[#F3F4F6] dark:hover:bg-[#1E1E20] cursor-pointer"
                          >
                            Cancel
                          </button>
                        </form>
                      ) : (
                        <button
                          type="button"
                          onClick={() => setIsAddingModel(true)}
                          className="w-fit flex items-center gap-2.5 p-[10px_16px] border border-[#E5E7EB] dark:border-[#333336] rounded-[8px] hover:bg-[#F3F4F6] dark:hover:bg-[#1E1E20] text-[#111827] dark:text-[#F2F2F2] transition-colors cursor-pointer mt-1"
                        >
                          <Plus className="w-4 h-4 text-[#6B7280] dark:text-[#9B9B9F]" />
                          <span className="text-[15px]">Add model</span>
                        </button>
                      )}

                      {/* Delete Provider Button */}
                      <div className="pt-4 border-t border-[#E5E7EB] dark:border-[#333336] mt-4 flex items-center justify-between">
                        <span className="text-xs text-[#6B7280] dark:text-[#6B6B70]">Provider ID: {selectedProvider.id}</span>
                        <button
                          type="button"
                          onClick={() => {
                            if (confirm(`Delete provider ${selectedProvider.name}?`)) {
                              deleteProvider(selectedProvider.id);
                            }
                          }}
                          className="text-xs text-[#DC2626] dark:text-[#EF4444] hover:underline cursor-pointer flex items-center gap-1"
                        >
                          <Trash2 className="w-3.5 h-3.5" />
                          <span>Delete provider</span>
                        </button>
                      </div>
                    </div>
                  </>
                ) : (
                  <div className="text-center py-20 text-[#6B7280] dark:text-[#6B6B70]">
                    Select or add a model provider from the left column.
                  </div>
                )}
              </div>
            </div>
          </>
        )}

        {/* =========================================================================
            SECTION: APPEARANCE
            ========================================================================= */}
        {settingsActiveSection === 'appearance' && (
          <div className="flex flex-col gap-6 max-w-2xl">
            <div className="text-[34px] font-bold text-[#111827] dark:text-[#F2F2F2]">Appearance</div>
            <p className="text-[15px] text-[#4B5563] dark:text-[#9B9B9F]">Customize themes, visual scaling, and editor font appearance.</p>

            <div className="border border-[#E5E7EB] dark:border-[#333336] rounded-[12px] p-6 bg-[#FFFFFF] dark:bg-[#161617] flex flex-col gap-5 shadow-xs">
              <span className="text-[15px] font-medium text-[#111827] dark:text-[#F2F2F2]">Color Theme</span>
              <div className="grid grid-cols-2 gap-4">
                <button
                  type="button"
                  onClick={() => setTheme('dark')}
                  className={`p-4 rounded-xl border flex flex-col items-center gap-3 cursor-pointer transition-all ${
                    theme === 'dark'
                      ? 'border-[#16A34A] dark:border-[#4ADE80] bg-[#F3F4F6] dark:bg-[#1E1E20] ring-2 ring-[#16A34A]/20 dark:ring-[#4ADE80]/20'
                      : 'border-[#E5E7EB] dark:border-[#333336] hover:bg-[#F3F4F6] dark:hover:bg-[#1E1E20]'
                  }`}
                >
                  <Moon className="w-6 h-6 text-[#111827] dark:text-[#F2F2F2]" />
                  <span className="text-sm font-medium text-[#111827] dark:text-[#F2F2F2]">Dark Theme (Standard)</span>
                  <span className="text-xs text-[#6B7280] dark:text-[#6B6B70]">Zinc & Jet Black</span>
                </button>

                <button
                  type="button"
                  onClick={() => setTheme('light')}
                  className={`p-4 rounded-xl border flex flex-col items-center gap-3 cursor-pointer transition-all ${
                    theme === 'light'
                      ? 'border-[#16A34A] dark:border-[#4ADE80] bg-[#F3F4F6] dark:bg-[#1E1E20] ring-2 ring-[#16A34A]/20 dark:ring-[#4ADE80]/20'
                      : 'border-[#E5E7EB] dark:border-[#333336] hover:bg-[#F3F4F6] dark:hover:bg-[#1E1E20]'
                  }`}
                >
                  <Sun className="w-6 h-6 text-[#111827] dark:text-[#F2F2F2]" />
                  <span className="text-sm font-medium text-[#111827] dark:text-[#F2F2F2]">Light Theme</span>
                  <span className="text-xs text-[#6B7280] dark:text-[#6B6B70]">Paper & Slate</span>
                </button>
              </div>
            </div>
          </div>
        )}

        {/* =========================================================================
            SECTION: GENERAL
            ========================================================================= */}
        {settingsActiveSection === 'general' && (
          <div className="flex flex-col gap-6 max-w-2xl">
            <div className="text-[34px] font-bold text-[#111827] dark:text-[#F2F2F2]">General Settings</div>
            <p className="text-[15px] text-[#4B5563] dark:text-[#9B9B9F]">Configure core environment, workspace, and telemetry behavior.</p>

            <div className="border border-[#E5E7EB] dark:border-[#333336] rounded-[12px] p-6 bg-[#FFFFFF] dark:bg-[#161617] flex flex-col gap-5 shadow-xs">
              <div className="flex flex-col gap-1.5">
                <label className="text-[15px] text-[#4B5563] dark:text-[#9B9B9F]">Active Workspace Directory</label>
                <div className="p-3 rounded-lg bg-[#F3F4F6] dark:bg-[#1E1E20] border border-[#E5E7EB] dark:border-[#333336] text-[14px] text-[#111827] dark:text-[#F2F2F2] font-mono truncate">
                  {activeWorkspacePath || 'No workspace opened'}
                </div>
              </div>

              <div className="flex items-center justify-between pt-2 border-t border-[#E5E7EB] dark:border-[#333336]">
                <div>
                  <div className="text-[15px] text-[#111827] dark:text-[#F2F2F2] font-medium">Share Terminal Activity</div>
                  <div className="text-xs text-[#6B7280] dark:text-[#6B6B70]">Allow background agents to capture command outcomes</div>
                </div>
                <input
                  type="checkbox"
                  checked={privacySettings.shareTerminalActivity}
                  onChange={e => setPrivacySettings(prev => ({ ...prev, shareTerminalActivity: e.target.checked }))}
                  className="w-4 h-4 accent-[#16A34A] dark:accent-[#4ADE80] cursor-pointer"
                />
              </div>

              <div className="flex items-center justify-between pt-2 border-t border-[#E5E7EB] dark:border-[#333336]">
                <div>
                  <div className="text-[15px] text-[#111827] dark:text-[#F2F2F2] font-medium">Share User Edits</div>
                  <div className="text-xs text-[#6B7280] dark:text-[#6B6B70]">Include manual editor buffer diffs in context window</div>
                </div>
                <input
                  type="checkbox"
                  checked={privacySettings.shareUserEdits}
                  onChange={e => setPrivacySettings(prev => ({ ...prev, shareUserEdits: e.target.checked }))}
                  className="w-4 h-4 accent-[#16A34A] dark:accent-[#4ADE80] cursor-pointer"
                />
              </div>
            </div>
          </div>
        )}

        {/* =========================================================================
            SECTION: MCP SERVERS
            ========================================================================= */}
        {settingsActiveSection === 'mcps' && (
          <div className="flex flex-col gap-6 max-w-3xl">
            <div className="text-[34px] font-bold text-[#111827] dark:text-[#F2F2F2]">MCP Servers</div>
            <p className="text-[15px] text-[#4B5563] dark:text-[#9B9B9F]">Model Context Protocol servers provide standard tool execution interfaces to Forge agents.</p>

            <div className="flex flex-col gap-3">
              {mcps.map(mcp => (
                <div key={mcp.id} className="p-4 rounded-xl border border-[#E5E7EB] dark:border-[#333336] bg-[#FFFFFF] dark:bg-[#1E1E20] flex items-center justify-between shadow-xs">
                  <div className="flex items-center gap-3">
                    <Cable className="w-5 h-5 text-[#16A34A] dark:text-[#4ADE80]" />
                    <div>
                      <div className="text-[15px] font-semibold text-[#111827] dark:text-[#F2F2F2]">{mcp.name}</div>
                      <div className="text-xs font-mono text-[#6B7280] dark:text-[#6B6B70] mt-0.5">{mcp.command}</div>
                      <div className="flex items-center gap-1.5 mt-2">
                        {(mcp.tools || []).map(t => (
                          <span key={t} className="text-[11px] bg-[#F3F4F6] dark:bg-[#2A2A2D] text-[#4B5563] dark:text-[#9B9B9F] px-2 py-0.5 rounded-full font-mono">
                            {t}
                          </span>
                        ))}
                      </div>
                    </div>
                  </div>

                  <div className="flex items-center gap-3">
                    <button
                      type="button"
                      onClick={() => toggleMcp(mcp.id)}
                      className={`px-3 py-1 rounded-full text-xs font-bold cursor-pointer transition-colors ${
                        mcp.enabled ? 'bg-[#DCFCE7] text-[#15803D] dark:bg-[#4ADE80] dark:text-[#0E2A18]' : 'bg-[#F3F4F6] text-[#6B7280] dark:bg-[#2A2A2D] dark:text-[#9B9B9F]'
                      }`}
                    >
                      {mcp.enabled ? 'Enabled' : 'Disabled'}
                    </button>
                    <button
                      type="button"
                      onClick={() => deleteMcp(mcp.id)}
                      className="p-1 text-[#9CA3AF] dark:text-[#6B6B70] hover:text-[#DC2626] dark:hover:text-[#EF4444] cursor-pointer"
                    >
                      <Trash2 className="w-4 h-4" />
                    </button>
                  </div>
                </div>
              ))}

              {isAddingMcp ? (
                <form
                  onSubmit={e => {
                    e.preventDefault();
                    if (!newMcpName.trim() || !newMcpCommand.trim()) return;
                    addMcp({
                      id: `mcp-${Date.now()}`,
                      name: newMcpName.trim(),
                      command: newMcpCommand.trim(),
                      status: 'connected',
                      enabled: true,
                      tools: ['tool_call']
                    });
                    setNewMcpName('');
                    setNewMcpCommand('');
                    setIsAddingMcp(false);
                  }}
                  className="p-4 rounded-xl border border-[#E5E7EB] dark:border-[#333336] bg-[#FFFFFF] dark:bg-[#1E1E20] flex flex-col gap-3 shadow-xs"
                >
                  <input
                    type="text"
                    value={newMcpName}
                    onChange={e => setNewMcpName(e.target.value)}
                    placeholder="MCP Server Name"
                    className="p-2.5 bg-[#F9FAFB] dark:bg-[#161617] border border-[#E5E7EB] dark:border-[#333336] rounded-lg text-sm text-[#111827] dark:text-[#F2F2F2] focus:outline-hidden"
                    required
                  />
                  <input
                    type="text"
                    value={newMcpCommand}
                    onChange={e => setNewMcpCommand(e.target.value)}
                    placeholder="Command (e.g. npx -y @modelcontextprotocol/server-git)"
                    className="p-2.5 bg-[#F9FAFB] dark:bg-[#161617] border border-[#E5E7EB] dark:border-[#333336] rounded-lg text-sm font-mono text-[#111827] dark:text-[#F2F2F2] focus:outline-hidden"
                    required
                  />
                  <div className="flex gap-2">
                    <button type="submit" className="px-4 py-1.5 rounded-lg bg-[#16A34A] dark:bg-[#4ADE80] text-white dark:text-[#0E2A18] text-xs font-bold cursor-pointer">
                      Save MCP Server
                    </button>
                    <button type="button" onClick={() => setIsAddingMcp(false)} className="px-3 py-1.5 rounded-lg border border-[#E5E7EB] dark:border-[#333336] text-xs text-[#4B5563] dark:text-[#9B9B9F] hover:bg-[#F3F4F6] dark:hover:bg-[#1E1E20] cursor-pointer">
                      Cancel
                    </button>
                  </div>
                </form>
              ) : (
                <button
                  type="button"
                  onClick={() => setIsAddingMcp(true)}
                  className="w-fit flex items-center gap-2 px-4 py-2 rounded-lg border border-[#E5E7EB] dark:border-[#333336] hover:bg-[#F3F4F6] dark:hover:bg-[#1E1E20] text-sm text-[#111827] dark:text-[#F2F2F2] cursor-pointer"
                >
                  <Plus className="w-4 h-4 text-[#6B7280] dark:text-[#9B9B9F]" />
                  <span>Add MCP Server</span>
                </button>
              )}
            </div>
          </div>
        )}

        {/* =========================================================================
            SECTION: SKILLS
            ========================================================================= */}
        {/* =========================================================================
            SECTION: PLUGINS
            ========================================================================= */}
        {settingsActiveSection === 'plugins' && (
          <div className="flex flex-col gap-6 max-w-4xl">
            <div className="flex items-start justify-between">
              <div>
                <div className="text-[34px] font-bold text-[#111827] dark:text-[#F2F2F2]">Agent Plugins</div>
                <p className="text-[15px] text-[#4B5563] dark:text-[#9B9B9F] mt-1">
                  Plugins extend agent capabilities with dynamic shell tools, scripts, and custom system prompts.
                </p>
              </div>
              <div className="flex items-center gap-2">
                <button
                  type="button"
                  onClick={handleReloadPlugins}
                  disabled={isReloadingPlugins}
                  className="flex items-center gap-1.5 px-3 py-1.5 border border-[#E5E7EB] dark:border-[#333336] rounded-lg text-xs font-medium text-[#4B5563] dark:text-[#9B9B9F] hover:bg-[#F3F4F6] dark:hover:bg-[#1E1E20] cursor-pointer"
                  title="Reload plugins from disk"
                >
                  <RefreshCw className={`w-3.5 h-3.5 ${isReloadingPlugins ? 'animate-spin' : ''}`} />
                  <span>Reload</span>
                </button>
                <button
                  type="button"
                  onClick={() => setIsAddingPlugin(prev => !prev)}
                  className="flex items-center gap-1.5 px-3.5 py-1.5 rounded-lg bg-[#16A34A] dark:bg-[#4ADE80] text-white dark:text-[#0E2A18] text-xs font-bold hover:bg-[#15803D] dark:hover:bg-[#3ec472] cursor-pointer shadow-xs"
                >
                  <Plus className="w-3.5 h-3.5" />
                  <span>Create Plugin</span>
                </button>
              </div>
            </div>

            {/* Filter & Stats bar */}
            <div className="flex items-center gap-3">
              <div className="relative flex-1">
                <Search className="w-4 h-4 text-[#9CA3AF] dark:text-[#6B6B70] absolute left-3 top-1/2 -translate-y-1/2 pointer-events-none" />
                <input
                  type="text"
                  value={pluginSearch}
                  onChange={e => setPluginSearch(e.target.value)}
                  placeholder="Filter plugins by name, ID, or tool..."
                  className="w-full pl-9 pr-4 py-2 bg-[#FFFFFF] dark:bg-[#161617] border border-[#E5E7EB] dark:border-[#333336] rounded-lg text-xs text-[#111827] dark:text-[#F2F2F2] focus:outline-hidden"
                />
              </div>
              <div className="px-3 py-2 border border-[#E5E7EB] dark:border-[#333336] rounded-lg text-xs text-[#6B7280] dark:text-[#9B9B9F] bg-[#FFFFFF] dark:bg-[#161617] shrink-0 font-medium">
                {plugins.filter(p => p.enabled).length} of {plugins.length} active
              </div>
            </div>

            {/* Add Plugin Modal / Form */}
            {isAddingPlugin && (
              <form
                onSubmit={handleCreatePlugin}
                className="p-5 rounded-xl border border-[#16A34A]/30 dark:border-[#4ADE80]/30 bg-[#FFFFFF] dark:bg-[#1E1E20] flex flex-col gap-4 shadow-sm"
              >
                <div className="flex items-center justify-between pb-3 border-b border-[#E5E7EB] dark:border-[#333336]">
                  <div className="flex items-center gap-2">
                    <Puzzle className="w-4 h-4 text-[#16A34A] dark:text-[#4ADE80]" />
                    <span className="text-sm font-bold text-[#111827] dark:text-[#F2F2F2]">Create New Plugin</span>
                  </div>
                  <button
                    type="button"
                    onClick={() => setIsAddingPlugin(false)}
                    className="text-xs text-[#6B7280] hover:text-[#111827] dark:hover:text-[#F2F2F2]"
                  >
                    Close
                  </button>
                </div>

                {pluginError && (
                  <div className="p-2.5 rounded-lg bg-red-50 dark:bg-red-950/40 border border-red-200 dark:border-red-900/50 text-xs text-red-600 dark:text-red-400">
                    {pluginError}
                  </div>
                )}

                <div className="grid grid-cols-3 gap-3">
                  <div>
                    <label className="text-xs font-medium text-[#4B5563] dark:text-[#9B9B9F] mb-1 block">Plugin ID</label>
                    <input
                      type="text"
                      value={newPluginId}
                      onChange={e => setNewPluginId(e.target.value)}
                      placeholder="e.g. docker-tools"
                      required
                      className="w-full p-2 bg-[#F9FAFB] dark:bg-[#161617] border border-[#E5E7EB] dark:border-[#333336] rounded-lg text-xs font-mono text-[#111827] dark:text-[#F2F2F2] focus:outline-hidden"
                    />
                  </div>
                  <div>
                    <label className="text-xs font-medium text-[#4B5563] dark:text-[#9B9B9F] mb-1 block">Name</label>
                    <input
                      type="text"
                      value={newPluginName}
                      onChange={e => setNewPluginName(e.target.value)}
                      placeholder="e.g. Docker Tools"
                      required
                      className="w-full p-2 bg-[#F9FAFB] dark:bg-[#161617] border border-[#E5E7EB] dark:border-[#333336] rounded-lg text-xs text-[#111827] dark:text-[#F2F2F2] focus:outline-hidden"
                    />
                  </div>
                  <div>
                    <label className="text-xs font-medium text-[#4B5563] dark:text-[#9B9B9F] mb-1 block">Scope</label>
                    <select
                      value={newPluginScope}
                      onChange={e => setNewPluginScope(e.target.value as any)}
                      className="w-full p-2 bg-[#F9FAFB] dark:bg-[#161617] border border-[#E5E7EB] dark:border-[#333336] rounded-lg text-xs text-[#111827] dark:text-[#F2F2F2] focus:outline-hidden"
                    >
                      <option value="workspace">Workspace (.forge/plugins)</option>
                      <option value="global">Global (~/.forge-ade/plugins)</option>
                    </select>
                  </div>
                </div>

                <div>
                  <label className="text-xs font-medium text-[#4B5563] dark:text-[#9B9B9F] mb-1 block">Description</label>
                  <input
                    type="text"
                    value={newPluginDesc}
                    onChange={e => setNewPluginDesc(e.target.value)}
                    placeholder="Short description of capabilities provided"
                    className="w-full p-2 bg-[#F9FAFB] dark:bg-[#161617] border border-[#E5E7EB] dark:border-[#333336] rounded-lg text-xs text-[#111827] dark:text-[#F2F2F2] focus:outline-hidden"
                  />
                </div>

                <div>
                  <label className="text-xs font-medium text-[#4B5563] dark:text-[#9B9B9F] mb-1 block">System Prompt (Optional)</label>
                  <textarea
                    rows={2}
                    value={newPluginPrompt}
                    onChange={e => setNewPluginPrompt(e.target.value)}
                    placeholder="Instructions injected into agent prompt when plugin is active..."
                    className="w-full p-2 bg-[#F9FAFB] dark:bg-[#161617] border border-[#E5E7EB] dark:border-[#333336] rounded-lg text-xs font-mono text-[#111827] dark:text-[#F2F2F2] focus:outline-hidden resize-y"
                  />
                </div>

                <div className="p-3.5 rounded-lg border border-[#E5E7EB] dark:border-[#333336] bg-[#F9FAFB] dark:bg-[#161617] flex flex-col gap-2.5">
                  <span className="text-xs font-semibold text-[#111827] dark:text-[#F2F2F2] flex items-center gap-1.5">
                    <Wrench className="w-3.5 h-3.5 text-[#16A34A] dark:text-[#4ADE80]" />
                    Initial Tool Definition (Optional)
                  </span>
                  <div className="grid grid-cols-2 gap-2.5">
                    <input
                      type="text"
                      value={newPluginToolName}
                      onChange={e => setNewPluginToolName(e.target.value)}
                      placeholder="Tool name (e.g. docker_ps)"
                      className="p-2 bg-white dark:bg-[#1E1E20] border border-[#E5E7EB] dark:border-[#333336] rounded-lg text-xs font-mono text-[#111827] dark:text-[#F2F2F2] focus:outline-hidden"
                    />
                    <input
                      type="text"
                      value={newPluginToolDesc}
                      onChange={e => setNewPluginToolDesc(e.target.value)}
                      placeholder="Tool description"
                      className="p-2 bg-white dark:bg-[#1E1E20] border border-[#E5E7EB] dark:border-[#333336] rounded-lg text-xs text-[#111827] dark:text-[#F2F2F2] focus:outline-hidden"
                    />
                  </div>
                  <input
                    type="text"
                    value={newPluginToolCmd}
                    onChange={e => setNewPluginToolCmd(e.target.value)}
                    placeholder="Execution command (e.g. docker ps --format json)"
                    className="p-2 bg-white dark:bg-[#1E1E20] border border-[#E5E7EB] dark:border-[#333336] rounded-lg text-xs font-mono text-[#111827] dark:text-[#F2F2F2] focus:outline-hidden"
                  />
                </div>

                <div className="flex items-center justify-end gap-2 pt-1">
                  <button
                    type="button"
                    onClick={() => setIsAddingPlugin(false)}
                    className="px-3.5 py-1.5 border border-[#E5E7EB] dark:border-[#333336] rounded-lg text-xs text-[#4B5563] dark:text-[#9B9B9F] hover:bg-[#F3F4F6] dark:hover:bg-[#1E1E20] cursor-pointer"
                  >
                    Cancel
                  </button>
                  <button
                    type="submit"
                    disabled={isCreatingPlugin}
                    className="px-4 py-1.5 rounded-lg bg-[#16A34A] dark:bg-[#4ADE80] text-white dark:text-[#0E2A18] text-xs font-bold hover:bg-[#15803D] dark:hover:bg-[#3ec472] cursor-pointer shadow-xs disabled:opacity-50"
                  >
                    {isCreatingPlugin ? 'Creating...' : 'Create Plugin'}
                  </button>
                </div>
              </form>
            )}

            {/* Plugins List */}
            <div className="flex flex-col gap-3">
              {plugins
                .filter(p => {
                  if (!pluginSearch.trim()) return true;
                  const q = pluginSearch.toLowerCase();
                  return (
                    p.id.toLowerCase().includes(q) ||
                    p.name.toLowerCase().includes(q) ||
                    (p.description && p.description.toLowerCase().includes(q)) ||
                    (p.tools && p.tools.some(t => t.name.toLowerCase().includes(q)))
                  );
                })
                .map(p => {
                  const scope = p.source || 'workspace';
                  const scopeBadgeClass =
                    scope === 'builtin'
                      ? 'bg-blue-500/10 text-blue-600 dark:text-blue-400 border-blue-500/20'
                      : scope === 'workspace'
                      ? 'bg-purple-500/10 text-purple-600 dark:text-purple-400 border-purple-500/20'
                      : scope === 'global'
                      ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20'
                      : 'bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/20';

                  return (
                    <div
                      key={p.id}
                      className={`p-4 rounded-xl border transition-all ${
                        p.enabled
                          ? 'border-[#E5E7EB] dark:border-[#333336] bg-[#FFFFFF] dark:bg-[#1E1E20] shadow-xs'
                          : 'border-[#E5E7EB]/60 dark:border-[#333336]/60 bg-[#F9FAFB]/50 dark:bg-[#161617]/50 opacity-70'
                      }`}
                    >
                      <div className="flex items-start justify-between gap-4">
                        <div className="flex items-start gap-3.5 flex-1 min-w-0">
                          <div className="p-2 rounded-lg bg-[#F3F4F6] dark:bg-[#2A2A2D] text-[#16A34A] dark:text-[#4ADE80] shrink-0 mt-0.5">
                            <Puzzle className="w-5 h-5" />
                          </div>
                          <div className="flex-1 min-w-0">
                            <div className="flex items-center gap-2 flex-wrap">
                              <span className="text-[15px] font-semibold text-[#111827] dark:text-[#F2F2F2]">{p.name}</span>
                              <span className="text-xs font-mono text-[#6B7280] dark:text-[#6B6B70]">({p.id})</span>
                              <span className={`text-[11px] font-medium px-2 py-0.5 rounded-full border ${scopeBadgeClass}`}>
                                {scope}
                              </span>
                              {p.version && (
                                <span className="text-[11px] font-mono text-[#6B7280] dark:text-[#6B6B70] bg-[#F3F4F6] dark:bg-[#2A2A2D] px-1.5 py-0.5 rounded">
                                  v{p.version}
                                </span>
                              )}
                            </div>

                            {p.description && (
                              <div className="text-xs text-[#4B5563] dark:text-[#9B9B9F] mt-1 leading-relaxed">
                                {p.description}
                              </div>
                            )}

                            {/* Registered Tools */}
                            {p.tools && p.tools.length > 0 && (
                              <div className="mt-3 flex flex-col gap-1.5">
                                <span className="text-[11px] font-medium text-[#6B7280] dark:text-[#6B6B70]">
                                  Tools ({p.tools.length}):
                                </span>
                                <div className="flex items-center gap-1.5 flex-wrap">
                                  {p.tools.map(t => (
                                    <div
                                      key={t.name}
                                      className="flex items-center gap-1 px-2 py-0.5 rounded-md bg-[#F3F4F6] dark:bg-[#2A2A2D] border border-[#E5E7EB] dark:border-[#333336] text-[11px] font-mono text-[#111827] dark:text-[#F2F2F2]"
                                      title={t.description || (t.command ? `Command: ${t.command}` : t.name)}
                                    >
                                      <Wrench className="w-3 h-3 text-[#16A34A] dark:text-[#4ADE80]" />
                                      <span>{t.name}</span>
                                    </div>
                                  ))}
                                </div>
                              </div>
                            )}

                            {/* Plugin Skills */}
                            {p.skills && p.skills.length > 0 && (
                              <div className="mt-2 flex items-center gap-1.5 flex-wrap">
                                <span className="text-[11px] font-medium text-[#6B7280] dark:text-[#6B6B70]">Skills:</span>
                                {p.skills.map(s => (
                                  <span
                                    key={s.name}
                                    className="px-2 py-0.5 rounded-md bg-[#F3F4F6] dark:bg-[#2A2A2D] border border-[#E5E7EB] dark:border-[#333336] text-[11px] text-[#4B5563] dark:text-[#9B9B9F]"
                                  >
                                    {s.name}
                                  </span>
                                ))}
                              </div>
                            )}
                          </div>
                        </div>

                        {/* Actions */}
                        <div className="flex items-center gap-2 shrink-0">
                          <button
                            type="button"
                            onClick={() => togglePlugin(p.id)}
                            className={`px-3 py-1 rounded-full text-xs font-bold cursor-pointer transition-colors ${
                              p.enabled
                                ? 'bg-[#DCFCE7] text-[#15803D] dark:bg-[#4ADE80] dark:text-[#0E2A18]'
                                : 'bg-[#F3F4F6] text-[#6B7280] dark:bg-[#2A2A2D] dark:text-[#9B9B9F]'
                            }`}
                          >
                            {p.enabled ? 'Active' : 'Disabled'}
                          </button>

                          {scope !== 'builtin' && (
                            <button
                              type="button"
                              onClick={() => {
                                if (confirm(`Delete plugin ${p.name}?`)) {
                                  deletePlugin(p.id);
                                }
                              }}
                              className="p-1.5 text-[#9CA3AF] dark:text-[#6B6B70] hover:text-[#DC2626] dark:hover:text-[#EF4444] transition-colors cursor-pointer rounded-md hover:bg-red-50 dark:hover:bg-red-950/30"
                              title="Delete plugin"
                            >
                              <Trash2 className="w-4 h-4" />
                            </button>
                          )}
                        </div>
                      </div>
                    </div>
                  );
                })}

              {plugins.length === 0 && (
                <div className="p-8 rounded-xl border border-[#E5E7EB] dark:border-[#333336] bg-[#FFFFFF] dark:bg-[#1E1E20] text-center text-xs text-[#6B7280] dark:text-[#6B6B70]">
                  No plugins found. Click &quot;Create Plugin&quot; or place plugins in <code className="font-mono bg-black/5 dark:bg-white/5 px-1 rounded">.forge/plugins</code>.
                </div>
              )}
            </div>
          </div>
        )}

        {/* =========================================================================
            SECTION: SKILLS (UPGRADED)
            ========================================================================= */}
        {settingsActiveSection === 'skills' && (
          <div className="flex flex-col gap-6 max-w-4xl">
            <div className="flex items-start justify-between">
              <div>
                <div className="text-[34px] font-bold text-[#111827] dark:text-[#F2F2F2]">Agent Skills</div>
                <p className="text-[15px] text-[#4B5563] dark:text-[#9B9B9F] mt-1">
                  Skills inject specialized domain workflows and instructions into coding sessions on demand.
                </p>
              </div>
              <div className="flex items-center gap-2">
                <button
                  type="button"
                  onClick={handleReloadSkills}
                  disabled={isReloadingSkills}
                  className="flex items-center gap-1.5 px-3 py-1.5 border border-[#E5E7EB] dark:border-[#333336] rounded-lg text-xs font-medium text-[#4B5563] dark:text-[#9B9B9F] hover:bg-[#F3F4F6] dark:hover:bg-[#1E1E20] cursor-pointer"
                  title="Reload skills from disk"
                >
                  <RefreshCw className={`w-3.5 h-3.5 ${isReloadingSkills ? 'animate-spin' : ''}`} />
                  <span>Reload</span>
                </button>
                <button
                  type="button"
                  onClick={runDiscovery}
                  disabled={isDiscovering}
                  className="flex items-center gap-1.5 px-3 py-1.5 border border-[#E5E7EB] dark:border-[#333336] rounded-lg text-xs font-medium text-[#4B5563] dark:text-[#9B9B9F] hover:bg-[#F3F4F6] dark:hover:bg-[#1E1E20] cursor-pointer"
                  title="Discover skills from Claude, Antigravity, OpenCode"
                >
                  <Compass className={`w-3.5 h-3.5 ${isDiscovering ? 'animate-spin' : ''}`} />
                  <span>Discover</span>
                </button>
                <button
                  type="button"
                  onClick={() => setIsAddingSkill(prev => !prev)}
                  className="flex items-center gap-1.5 px-3.5 py-1.5 rounded-lg bg-[#16A34A] dark:bg-[#4ADE80] text-white dark:text-[#0E2A18] text-xs font-bold hover:bg-[#15803D] dark:hover:bg-[#3ec472] cursor-pointer shadow-xs"
                >
                  <Plus className="w-3.5 h-3.5" />
                  <span>Create Skill</span>
                </button>
              </div>
            </div>

            {/* Filter & Stats bar */}
            <div className="flex items-center gap-3">
              <div className="relative flex-1">
                <Search className="w-4 h-4 text-[#9CA3AF] dark:text-[#6B6B70] absolute left-3 top-1/2 -translate-y-1/2 pointer-events-none" />
                <input
                  type="text"
                  value={skillSearch}
                  onChange={e => setSkillSearch(e.target.value)}
                  placeholder="Filter skills by name or description..."
                  className="w-full pl-9 pr-4 py-2 bg-[#FFFFFF] dark:bg-[#161617] border border-[#E5E7EB] dark:border-[#333336] rounded-lg text-xs text-[#111827] dark:text-[#F2F2F2] focus:outline-hidden"
                />
              </div>
              <div className="px-3 py-2 border border-[#E5E7EB] dark:border-[#333336] rounded-lg text-xs text-[#6B7280] dark:text-[#9B9B9F] bg-[#FFFFFF] dark:bg-[#161617] shrink-0 font-medium">
                {skills.filter(s => s.enabled).length} of {skills.length} active
              </div>
            </div>

            {/* Add Skill Form */}
            {isAddingSkill && (
              <form
                onSubmit={handleCreateSkill}
                className="p-5 rounded-xl border border-[#16A34A]/30 dark:border-[#4ADE80]/30 bg-[#FFFFFF] dark:bg-[#1E1E20] flex flex-col gap-4 shadow-sm"
              >
                <div className="flex items-center justify-between pb-3 border-b border-[#E5E7EB] dark:border-[#333336]">
                  <div className="flex items-center gap-2">
                    <Sparkles className="w-4 h-4 text-[#16A34A] dark:text-[#4ADE80]" />
                    <span className="text-sm font-bold text-[#111827] dark:text-[#F2F2F2]">Create New Skill</span>
                  </div>
                  <button
                    type="button"
                    onClick={() => setIsAddingSkill(false)}
                    className="text-xs text-[#6B7280] hover:text-[#111827] dark:hover:text-[#F2F2F2]"
                  >
                    Close
                  </button>
                </div>

                {skillError && (
                  <div className="p-2.5 rounded-lg bg-red-50 dark:bg-red-950/40 border border-red-200 dark:border-red-900/50 text-xs text-red-600 dark:text-red-400">
                    {skillError}
                  </div>
                )}

                <div className="grid grid-cols-2 gap-3">
                  <div>
                    <label className="text-xs font-medium text-[#4B5563] dark:text-[#9B9B9F] mb-1 block">Skill Name</label>
                    <input
                      type="text"
                      value={newSkillName}
                      onChange={e => setNewSkillName(e.target.value)}
                      placeholder="e.g. clean-architecture"
                      required
                      className="w-full p-2 bg-[#F9FAFB] dark:bg-[#161617] border border-[#E5E7EB] dark:border-[#333336] rounded-lg text-xs font-mono text-[#111827] dark:text-[#F2F2F2] focus:outline-hidden"
                    />
                  </div>
                  <div>
                    <label className="text-xs font-medium text-[#4B5563] dark:text-[#9B9B9F] mb-1 block">Scope</label>
                    <select
                      value={newSkillScope}
                      onChange={e => setNewSkillScope(e.target.value as any)}
                      className="w-full p-2 bg-[#F9FAFB] dark:bg-[#161617] border border-[#E5E7EB] dark:border-[#333336] rounded-lg text-xs text-[#111827] dark:text-[#F2F2F2] focus:outline-hidden"
                    >
                      <option value="workspace">Workspace (.forge/skills)</option>
                      <option value="global">Global (~/.forge-ade/skills)</option>
                    </select>
                  </div>
                </div>

                <div>
                  <label className="text-xs font-medium text-[#4B5563] dark:text-[#9B9B9F] mb-1 block">Description</label>
                  <input
                    type="text"
                    value={newSkillDesc}
                    onChange={e => setNewSkillDesc(e.target.value)}
                    placeholder="Short summary of what this skill does and when to use it"
                    required
                    className="w-full p-2 bg-[#F9FAFB] dark:bg-[#161617] border border-[#E5E7EB] dark:border-[#333336] rounded-lg text-xs text-[#111827] dark:text-[#F2F2F2] focus:outline-hidden"
                  />
                </div>

                <div>
                  <label className="text-xs font-medium text-[#4B5563] dark:text-[#9B9B9F] mb-1 block">
                    Instructions / Prompt Body (Markdown)
                  </label>
                  <textarea
                    rows={6}
                    value={newSkillPrompt}
                    onChange={e => setNewSkillPrompt(e.target.value)}
                    placeholder="Detailed instructions, coding guidelines, or prompt rules injected into the agent when this skill is invoked..."
                    required
                    className="w-full p-2.5 bg-[#F9FAFB] dark:bg-[#161617] border border-[#E5E7EB] dark:border-[#333336] rounded-lg text-xs font-mono text-[#111827] dark:text-[#F2F2F2] focus:outline-hidden resize-y"
                  />
                </div>

                <div className="flex items-center justify-end gap-2 pt-1">
                  <button
                    type="button"
                    onClick={() => setIsAddingSkill(false)}
                    className="px-3.5 py-1.5 border border-[#E5E7EB] dark:border-[#333336] rounded-lg text-xs text-[#4B5563] dark:text-[#9B9B9F] hover:bg-[#F3F4F6] dark:hover:bg-[#1E1E20] cursor-pointer"
                  >
                    Cancel
                  </button>
                  <button
                    type="submit"
                    disabled={isCreatingSkill}
                    className="px-4 py-1.5 rounded-lg bg-[#16A34A] dark:bg-[#4ADE80] text-white dark:text-[#0E2A18] text-xs font-bold hover:bg-[#15803D] dark:hover:bg-[#3ec472] cursor-pointer shadow-xs disabled:opacity-50"
                  >
                    {isCreatingSkill ? 'Creating...' : 'Create Skill'}
                  </button>
                </div>
              </form>
            )}

            {/* Skills List */}
            <div className="flex flex-col gap-3">
              {skills
                .filter(sk => {
                  if (!skillSearch.trim()) return true;
                  const q = skillSearch.toLowerCase();
                  return (
                    sk.name.toLowerCase().includes(q) ||
                    (sk.description && sk.description.toLowerCase().includes(q)) ||
                    (sk.trigger && sk.trigger.toLowerCase().includes(q))
                  );
                })
                .map(sk => {
                  const scope = sk.origin || 'workspace';
                  const isExpanded = expandedSkillId === sk.id;
                  const scopeBadgeClass =
                    scope === 'plugin'
                      ? 'bg-blue-500/10 text-blue-600 dark:text-blue-400 border-blue-500/20'
                      : scope === 'workspace'
                      ? 'bg-purple-500/10 text-purple-600 dark:text-purple-400 border-purple-500/20'
                      : scope === 'global'
                      ? 'bg-emerald-500/10 text-emerald-600 dark:text-emerald-400 border-emerald-500/20'
                      : 'bg-amber-500/10 text-amber-600 dark:text-amber-400 border-amber-500/20';

                  return (
                    <div
                      key={sk.id}
                      className={`p-4 rounded-xl border transition-all ${
                        sk.enabled
                          ? 'border-[#E5E7EB] dark:border-[#333336] bg-[#FFFFFF] dark:bg-[#1E1E20] shadow-xs'
                          : 'border-[#E5E7EB]/60 dark:border-[#333336]/60 bg-[#F9FAFB]/50 dark:bg-[#161617]/50 opacity-70'
                      }`}
                    >
                      <div className="flex items-start justify-between gap-4">
                        <div className="flex items-start gap-3.5 flex-1 min-w-0">
                          <div className="p-2 rounded-lg bg-[#F3F4F6] dark:bg-[#2A2A2D] text-[#16A34A] dark:text-[#4ADE80] shrink-0 mt-0.5">
                            <Sparkles className="w-5 h-5" />
                          </div>
                          <div className="flex-1 min-w-0">
                            <div className="flex items-center gap-2 flex-wrap">
                              <span className="text-[15px] font-semibold text-[#111827] dark:text-[#F2F2F2]">{sk.name}</span>
                              <span className={`text-[11px] font-medium px-2 py-0.5 rounded-full border ${scopeBadgeClass}`}>
                                {scope}
                              </span>
                              {sk.category && sk.category !== scope && (
                                <span className="text-[11px] text-[#6B7280] dark:text-[#6B6B70] bg-[#F3F4F6] dark:bg-[#2A2A2D] px-2 py-0.5 rounded-full">
                                  {sk.category}
                                </span>
                              )}
                            </div>

                            <div className="text-xs text-[#4B5563] dark:text-[#9B9B9F] mt-1 leading-relaxed">
                              {sk.description}
                            </div>

                            {sk.trigger && (
                              <div className="text-[11px] font-mono text-[#6B7280] dark:text-[#6B6B70] mt-1.5 flex items-center gap-1">
                                <span>Trigger:</span>
                                <code className="bg-[#F3F4F6] dark:bg-[#2A2A2D] px-1.5 py-0.5 rounded text-[#111827] dark:text-[#F2F2F2]">
                                  {sk.trigger}
                                </code>
                              </div>
                            )}

                            {sk.instructions && (
                              <div className="mt-2">
                                <button
                                  type="button"
                                  onClick={() => setExpandedSkillId(isExpanded ? null : sk.id)}
                                  className="text-[11px] text-[#16A34A] dark:text-[#4ADE80] hover:underline flex items-center gap-1 cursor-pointer"
                                >
                                  {isExpanded ? <ChevronDown className="w-3 h-3" /> : <ChevronRight className="w-3 h-3" />}
                                  <span>{isExpanded ? 'Hide Instructions' : 'View Instructions'}</span>
                                </button>
                                {isExpanded && (
                                  <pre className="mt-2 p-3 rounded-lg bg-[#F9FAFB] dark:bg-[#161617] border border-[#E5E7EB] dark:border-[#333336] text-[11px] font-mono text-[#111827] dark:text-[#F2F2F2] overflow-x-auto whitespace-pre-wrap leading-relaxed max-h-60 overflow-y-auto">
                                    {sk.instructions}
                                  </pre>
                                )}
                              </div>
                            )}
                          </div>
                        </div>

                        {/* Actions */}
                        <div className="flex items-center gap-2 shrink-0">
                          <button
                            type="button"
                            onClick={() => toggleSkill(sk.id)}
                            className={`px-3 py-1 rounded-full text-xs font-bold cursor-pointer transition-colors ${
                              sk.enabled
                                ? 'bg-[#DCFCE7] text-[#15803D] dark:bg-[#4ADE80] dark:text-[#0E2A18]'
                                : 'bg-[#F3F4F6] text-[#6B7280] dark:bg-[#2A2A2D] dark:text-[#9B9B9F]'
                            }`}
                          >
                            {sk.enabled ? 'Active' : 'Disabled'}
                          </button>

                          <button
                            type="button"
                            onClick={async () => {
                              if (confirm(`Delete skill ${sk.name}?`)) {
                                if (deleteBackendSkill) {
                                  try {
                                    await deleteBackendSkill(sk.name);
                                  } catch {
                                    deleteSkill(sk.id);
                                  }
                                } else {
                                  deleteSkill(sk.id);
                                }
                              }
                            }}
                            className="p-1.5 text-[#9CA3AF] dark:text-[#6B6B70] hover:text-[#DC2626] dark:hover:text-[#EF4444] transition-colors cursor-pointer rounded-md hover:bg-red-50 dark:hover:bg-red-950/30"
                            title="Delete skill"
                          >
                            <Trash2 className="w-4 h-4" />
                          </button>
                        </div>
                      </div>
                    </div>
                  );
                })}
            </div>
          </div>
        )}

        {/* =========================================================================
            SECTION: SUBAGENTS
            ========================================================================= */}
        {settingsActiveSection === 'subagents' && (
          <div className="flex flex-col gap-6 max-w-3xl">
            <div className="text-[34px] font-bold text-[#111827] dark:text-[#F2F2F2]">Subagents & ACP Protocol</div>
            <p className="text-[15px] text-[#4B5563] dark:text-[#9B9B9F]">Configure multi-agent protocol endpoints and specialized subagent executors.</p>

            <div className="flex flex-col gap-3">
              {agents.map(ag => (
                <div key={ag.id} className="p-4 rounded-xl border border-[#E5E7EB] dark:border-[#333336] bg-[#FFFFFF] dark:bg-[#1E1E20] flex items-center justify-between shadow-xs">
                  <div className="flex items-center gap-3">
                    <Bot className="w-5 h-5 text-[#16A34A] dark:text-[#4ADE80]" />
                    <div>
                      <div className="text-[15px] font-semibold text-[#111827] dark:text-[#F2F2F2]">{ag.name}</div>
                      <div className="text-xs text-[#4B5563] dark:text-[#9B9B9F] mt-0.5">{ag.description}</div>
                      <div className="text-[11px] font-mono text-[#6B7280] dark:text-[#6B6B70] mt-1">
                        Endpoint: {ag.endpoint || 'Internal Loop'}
                      </div>
                    </div>
                  </div>

                  <button
                    type="button"
                    onClick={() => toggleAgentEnabled(ag.id)}
                    className={`px-3 py-1 rounded-full text-xs font-bold cursor-pointer transition-colors ${
                      ag.enabled ? 'bg-[#DCFCE7] text-[#15803D] dark:bg-[#4ADE80] dark:text-[#0E2A18]' : 'bg-[#F3F4F6] text-[#6B7280] dark:bg-[#2A2A2D] dark:text-[#9B9B9F]'
                    }`}
                  >
                    {ag.enabled ? 'Active' : 'Disabled'}
                  </button>
                </div>
              ))}
            </div>
          </div>
        )}

        {/* =========================================================================
            OTHER SECTIONS FALLBACK
            ========================================================================= */}
        {!['model', 'agents', 'appearance', 'general', 'mcps', 'skills', 'subagents', 'plugins'].includes(settingsActiveSection) && (
          <div className="flex flex-col gap-6 max-w-2xl">
            <div className="text-[34px] font-bold text-[#111827] dark:text-[#F2F2F2]">
              {settingsActiveSection.charAt(0).toUpperCase() + settingsActiveSection.slice(1)}
            </div>
            <p className="text-[15px] text-[#4B5563] dark:text-[#9B9B9F]">Configuration panel for {settingsActiveSection}.</p>
            <div className="p-8 rounded-xl border border-[#E5E7EB] dark:border-[#333336] bg-[#FFFFFF] dark:bg-[#1E1E20] text-center text-[#6B7280] dark:text-[#6B6B70] shadow-xs">
              Active configuration for {settingsActiveSection} is operational and synchronized with workspace state.
            </div>
          </div>
        )}

      </main>
    </div>
  );
};

export default SettingsScreen;
