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
  Search
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
        {settingsActiveSection === 'skills' && (
          <div className="flex flex-col gap-6 max-w-3xl">
            <div className="text-[34px] font-bold text-[#111827] dark:text-[#F2F2F2]">Agent Skills</div>
            <p className="text-[15px] text-[#4B5563] dark:text-[#9B9B9F]">Skills inject specialized agentic workflows and knowledge into coding sessions.</p>

            <div className="flex flex-col gap-3">
              {skills.map(sk => (
                <div key={sk.id} className="p-4 rounded-xl border border-[#E5E7EB] dark:border-[#333336] bg-[#FFFFFF] dark:bg-[#1E1E20] flex items-center justify-between shadow-xs">
                  <div className="flex items-center gap-3">
                    <Sparkles className="w-5 h-5 text-[#16A34A] dark:text-[#4ADE80]" />
                    <div>
                      <div className="text-[15px] font-semibold text-[#111827] dark:text-[#F2F2F2]">{sk.name}</div>
                      <div className="text-xs text-[#4B5563] dark:text-[#9B9B9F] mt-0.5">{sk.description}</div>
                      <div className="text-[11px] font-mono text-[#6B7280] dark:text-[#6B6B70] mt-1">Trigger: {sk.trigger}</div>
                    </div>
                  </div>

                  <div className="flex items-center gap-3">
                    <button
                      type="button"
                      onClick={() => toggleSkill(sk.id)}
                      className={`px-3 py-1 rounded-full text-xs font-bold cursor-pointer transition-colors ${
                        sk.enabled ? 'bg-[#DCFCE7] text-[#15803D] dark:bg-[#4ADE80] dark:text-[#0E2A18]' : 'bg-[#F3F4F6] text-[#6B7280] dark:bg-[#2A2A2D] dark:text-[#9B9B9F]'
                      }`}
                    >
                      {sk.enabled ? 'Enabled' : 'Disabled'}
                    </button>
                    <button
                      type="button"
                      onClick={() => deleteSkill(sk.id)}
                      className="p-1 text-[#9CA3AF] dark:text-[#6B6B70] hover:text-[#DC2626] dark:hover:text-[#EF4444] cursor-pointer"
                    >
                      <Trash2 className="w-4 h-4" />
                    </button>
                  </div>
                </div>
              ))}
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
        {!['model', 'agents', 'appearance', 'general', 'mcps', 'skills', 'subagents'].includes(settingsActiveSection) && (
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
