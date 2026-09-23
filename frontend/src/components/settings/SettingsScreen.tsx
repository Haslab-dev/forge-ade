import React, { useState, useMemo, useEffect } from 'react';
import {
  ArrowLeft,
  Settings2,
  Palette,
  Package,
  Globe2,
  Keyboard as KeyboardIcon,
  Brain,
  Bot,
  Blocks,
  Cable,
  WandSparkles,
  Terminal,
  Anchor,
  BarChart3,
  Rocket,
  RefreshCw,
  Plus,
  Trash2,
  Check,
  Search,
  ChevronDown,
  ChevronRight,
  Copy,
  Cpu,
  Eye,
  EyeOff,
  FileText,
  Database,
  Wrench,
  Compass,
  Activity
} from 'lucide-react';
import { cn } from '../../lib/utils';
import { useToast } from '../../lib/toast';
import { useWorkspace } from '../../stores/workspaceStore';
import { LLMProviderConfig, AgentMemoryEntry } from '../../types';
import {
  BetaBadge,
  SETTINGS_FRAME_CONTENT_CLASSNAME,
  SectionTitle,
  SettingsBadge,
  SettingsEmptyState,
  SettingsGroupCard,
  SettingsResourceList,
  SettingsResourceSeparator,
  SettingsRow,
  SettingsScopeBadge,
  SettingsSearchInput,
  SettingsSelect,
  Switch,
  settingsButtonClasses as btn
} from './settingsPrimitives';

/**
 * Settings page shell ported 1:1 from ZCode SettingsPage:
 * icon rail (68px) / labeled sidebar (268px at lg), framed content panel with
 * h-12 breadcrumb header, and a centered max-w-4xl section column.
 * Nav sections and groups mirror settingsPageConfig.ts (desktop, en-US).
 */

type SettingsSectionId =
  | 'general'
  | 'appearance'
  | 'model'
  | 'browser'
  | 'shortcuts'
  | 'memory'
  | 'subagents'
  | 'plugins'
  | 'mcps'
  | 'skills'
  | 'commands'
  | 'hooks'
  | 'usage';

interface NavItem {
  id: SettingsSectionId;
  label: string;
  icon: React.ComponentType<{ className?: string }>;
  badge?: string;
}

const NAV_GROUPS: Array<{ id: string; label: string; items: NavItem[] }> = [
  {
    id: 'basics',
    label: 'Basics',
    items: [
      { id: 'general', label: 'General', icon: Settings2 },
      { id: 'appearance', label: 'Appearance', icon: Palette },
      { id: 'model', label: 'Model settings', icon: Package },
      { id: 'browser', label: 'Browser Use', icon: Globe2 },
      { id: 'shortcuts', label: 'Keyboard Shortcuts', icon: KeyboardIcon }
    ]
  },
  {
    id: 'agentCapabilities',
    label: 'Agent capabilities',
    items: [
      { id: 'memory', label: 'Memory', icon: Brain },
      { id: 'subagents', label: 'Subagents', icon: Bot },
      { id: 'plugins', label: 'Plugins', icon: Blocks },
      { id: 'mcps', label: 'MCP Servers', icon: Cable },
      { id: 'skills', label: 'Skills', icon: WandSparkles },
      { id: 'commands', label: 'Commands', icon: Terminal },
      { id: 'hooks', label: 'Hooks', icon: Anchor }
    ]
  },
  {
    id: 'dataAndStats',
    label: 'Data and statistics',
    items: [{ id: 'usage', label: 'Usage stats', icon: BarChart3 }]
  }
];

const SECTION_TITLES: Record<SettingsSectionId, string> = {
  general: 'General',
  appearance: 'Appearance',
  model: 'Model settings',
  browser: 'Browser Use',
  shortcuts: 'Keyboard Shortcuts',
  memory: 'Memory',
  subagents: 'Subagents',
  plugins: 'Plugins',
  mcps: 'MCP Servers',
  skills: 'Skills',
  commands: 'Commands',
  hooks: 'Hooks',
  usage: 'Usage stats'
};

/** ForgeADE's real keybindings (workspaceStore global key handler). */
const SHORTCUT_ROWS: Array<{ command: string; binding: string; scope: string }> = [
  { command: 'Command center', binding: '⌘K', scope: 'Global' },
  { command: 'Quick open file', binding: '⌘P', scope: 'Global' },
  { command: 'New terminal tab', binding: '⌘T', scope: 'Editor' },
  { command: 'Toggle terminal focus', binding: 'Alt T', scope: 'Editor' },
  { command: 'Open agent workspace', binding: '⌘1', scope: 'Global' },
  { command: 'Open editor workspace', binding: '⌘2', scope: 'Global' },
  { command: 'Toggle review panel', binding: '⌘/', scope: 'Editor' },
  { command: 'Open settings', binding: '⌘,', scope: 'Global' },
  { command: 'Open workspace', binding: '⌘O', scope: 'Global' }
];

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
    memories,
    saveMemory,
    deleteMemory,
    reloadMemories,
    customCommands,
    addCustomCommand,
    updateCustomCommand,
    deleteCustomCommand,
    toggleCustomCommand,
    hooks,
    updateHook,
    toggleHook,
    addHook,
    deleteHook,
    browserSettings,
    updateBrowserSettings,
    indexStatus,
    fetchIndexStatus,
    reindexWorkspace,
    isReindexing,
    contextUsage,
    agents,
    toggleAgentEnabled,
    activeWorkspacePath,
    settingsActiveSection,
    setSettingsActiveSection,
    goBackToWorkspace,
    privacySettings,
    setPrivacySettings
  } = useWorkspace();

  const activeSection = (settingsActiveSection || 'general') as SettingsSectionId;
  const setActiveSection = (id: string) => setSettingsActiveSection(id);

  // Keep legacy persisted ids inside the visible nav.
  useEffect(() => {
    const visible = NAV_GROUPS.flatMap((group) => group.items.map((item) => item.id));
    if (!visible.includes(activeSection)) setSettingsActiveSection('general');
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  useEffect(() => {
    void fetchIndexStatus?.();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  return (
    <div
      data-testid="settings-page"
      className="relative grid h-full min-h-full w-full grid-cols-[68px_minmax(0,1fr)] grid-rows-[minmax(0,1fr)] overflow-hidden bg-background-alt lg:grid-cols-[268px_minmax(0,1fr)]"
    >
      <aside className="flex min-h-0 flex-col overflow-y-auto pb-4">
        <div className="flex w-full flex-col px-3 pb-2 pt-3">
          {/* Back to workspace */}
          <button
            type="button"
            onClick={goBackToWorkspace}
            data-testid="settings-back-button"
            className="group flex h-8 w-full items-center gap-2 rounded-xl px-2.5 text-foreground-subtle transition-colors hover:bg-surface-hover hover:text-foreground max-lg:justify-center max-lg:px-0"
          >
            <ArrowLeft className="size-4 shrink-0" />
            <span className="text-ui-base max-lg:hidden">Back</span>
          </button>
        </div>

        <nav aria-label="Sections" data-testid="settings-section-nav" className="flex flex-1 flex-col gap-6 px-3">
          {NAV_GROUPS.map((group) => (
            <div key={group.id} className="w-full space-y-1">
              <div className="px-2.5 pb-1 text-ui-sm font-medium text-foreground-subtlest max-lg:hidden">
                {group.label}
              </div>
              <div className="flex flex-col gap-1">
                {group.items.map((item) => {
                  const Icon = item.icon;
                  const isActive = activeSection === item.id;
                  return (
                    <button
                      key={item.id}
                      type="button"
                      onClick={() => setActiveSection(item.id)}
                      aria-current={isActive ? 'page' : undefined}
                      className={cn(
                        'flex h-8 w-full items-center gap-2 rounded-xl px-2.5 text-left transition-colors max-lg:justify-center max-lg:px-0',
                        isActive
                          ? 'bg-surface-hover text-foreground'
                          : 'text-foreground-subtle hover:bg-surface-hover hover:text-foreground'
                      )}
                    >
                      <Icon className="size-4 shrink-0" />
                      <span className="min-w-0 flex-1 truncate text-ui-base max-lg:hidden">
                        {item.label}
                      </span>
                      {item.badge ? (
                        <span className="shrink-0 max-lg:hidden">
                          <BetaBadge label={item.badge} />
                        </span>
                      ) : null}
                    </button>
                  );
                })}
              </div>
            </div>
          ))}
        </nav>

        <div className="px-3 pt-4">
          <button
            type="button"
            onClick={() => setActiveSection('general')}
            className="flex h-8 w-full items-center gap-2 rounded-xl border border-dashed border-border px-2.5 text-left text-foreground transition-colors hover:border-border-hover hover:bg-surface-hover max-lg:justify-center max-lg:px-0"
          >
            <Rocket className="size-4 shrink-0" />
            <span className="truncate text-ui-base max-lg:hidden">Onboard</span>
          </button>
        </div>
      </aside>

      {/* Framed content panel */}
      <section className="relative flex min-h-0 flex-col p-2 pl-0 pt-2 max-lg:pl-0">
        <div className="relative flex h-full min-h-0 flex-col rounded-xl border border-border bg-background">
          {/* Header drag row with section breadcrumb */}
          <div className="h-12 shrink-0">
            <div className="flex h-full items-center px-4">
              <span className="truncate text-ui-base text-foreground-subtle">
                {SECTION_TITLES[activeSection]}
              </span>
            </div>
          </div>
          <main className="min-h-0 flex-1 overflow-y-auto [scrollbar-gutter:stable]">
            <div className={cn(SETTINGS_FRAME_CONTENT_CLASSNAME, 'flex flex-col gap-8')}>
              <SectionTitle title={SECTION_TITLES[activeSection]} />
              <div className="space-y-8">
                {activeSection === 'general' && <GeneralSection />}

                {activeSection === 'appearance' && (
                  <AppearanceSection theme={theme} setTheme={(t) => setTheme(t as any)} />
                )}

                {activeSection === 'model' && (
                  <ModelSettingsSection
                    providers={providers}
                    selectedProviderId={undefined}
                    currentModel={currentModel}
                    setCurrentModel={setCurrentModel}
                    updateProvider={updateProvider}
                    addProvider={addProvider}
                    deleteProvider={deleteProvider}
                    addModelToProvider={addModelToProvider}
                    deleteModelFromProvider={deleteModelFromProvider}
                    toggleModelSelection={toggleModelSelection}
                    fetchProviderModels={fetchProviderModels}
                  />
                )}

                {activeSection === 'browser' && (
                  <BrowserUseSection
                    browserSettings={browserSettings}
                    updateBrowserSettings={updateBrowserSettings}
                  />
                )}

                {activeSection === 'shortcuts' && <ShortcutsSection />}

                {activeSection === 'memory' && (
                  <MemorySection
                    memories={memories}
                    saveMemory={saveMemory}
                    deleteMemory={deleteMemory}
                    reloadMemories={reloadMemories}
                  />
                )}

                {activeSection === 'subagents' && (
                  <SubagentsSection agents={agents} toggleAgentEnabled={toggleAgentEnabled} />
                )}

                {activeSection === 'plugins' && (
                  <PluginsSection
                    plugins={plugins}
                    createPlugin={createPlugin}
                    togglePlugin={togglePlugin}
                    deletePlugin={deletePlugin}
                    reloadPlugins={reloadPlugins}
                  />
                )}

                {activeSection === 'mcps' && (
                  <McpSection mcps={mcps} addMcp={addMcp} toggleMcp={toggleMcp} deleteMcp={deleteMcp} />
                )}

                {activeSection === 'skills' && (
                  <SkillsSection
                    skills={skills}
                    toggleSkill={toggleSkill}
                    deleteSkill={deleteSkill}
                    createBackendSkill={createBackendSkill}
                    deleteBackendSkill={deleteBackendSkill}
                    reloadSkills={reloadSkills}
                    discoveredSkills={discoveredSkills}
                    importDiscoveredSkill={importDiscoveredSkill}
                    runDiscovery={runDiscovery}
                    isDiscovering={isDiscovering}
                  />
                )}

                {activeSection === 'commands' && (
                  <CommandsSection
                    customCommands={customCommands}
                    addCustomCommand={addCustomCommand}
                    updateCustomCommand={updateCustomCommand}
                    deleteCustomCommand={deleteCustomCommand}
                    toggleCustomCommand={toggleCustomCommand}
                  />
                )}

                {activeSection === 'hooks' && (
                  <HooksSection
                    hooks={hooks}
                    updateHook={updateHook}
                    toggleHook={toggleHook}
                    addHook={addHook}
                    deleteHook={deleteHook}
                  />
                )}

                {activeSection === 'usage' && (
                  <UsageSection
                    providers={providers}
                    contextUsage={contextUsage}
                    indexStatus={indexStatus}
                    fetchIndexStatus={fetchIndexStatus}
                    reindexWorkspace={reindexWorkspace}
                    isReindexing={isReindexing}
                  />
                )}
              </div>
            </div>
          </main>
        </div>
      </section>
    </div>
  );
};

// ── General ─────────────────────────────────────────────────────────────────

const GeneralSection: React.FC = () => {
  const {
    activeWorkspacePath,
    privacySettings,
    setPrivacySettings,
    indexStatus,
    fetchIndexStatus,
    reindexWorkspace,
    isReindexing
  } = useWorkspace();
  const { toast } = useToast();

  return (
    <>
      <SettingsGroupCard>
        <SettingsRow
          label="Workspace directory"
          description="Folder the agent, editor and search operate on."
          control={
            <span className="max-w-full truncate font-mono text-ui-sm text-foreground">
              {activeWorkspacePath || 'No workspace opened'}
            </span>
          }
        />
        <SettingsRow
          label="Share terminal activity"
          description="Allow background agents to capture command outcomes."
          control={
            <Switch
              checked={privacySettings.shareTerminalActivity}
              onCheckedChange={(checked) =>
                setPrivacySettings((prev: PrivacySettingsDraft) => ({ ...prev, shareTerminalActivity: checked }))
              }
            />
          }
        />
        <SettingsRow
          label="Share user edits"
          description="Include manual editor buffer diffs in the context window."
          control={
            <Switch
              checked={privacySettings.shareUserEdits}
              onCheckedChange={(checked) =>
                setPrivacySettings((prev: PrivacySettingsDraft) => ({ ...prev, shareUserEdits: checked }))
              }
            />
          }
        />
      </SettingsGroupCard>

      <SettingsGroupCard>
        <SettingsRow
          label="Workspace index"
          description={
            indexStatus?.built
              ? `Built · ${indexStatus.symbols ?? 0} symbols indexed`
              : 'Not built yet. Indexing powers symbol search and completions.'
          }
          control={
            <button
              type="button"
              className={btn.outline}
              disabled={isReindexing}
              onClick={async () => {
                try {
                  await reindexWorkspace();
                  toast('Workspace reindexed');
                } finally {
                  void fetchIndexStatus?.();
                }
              }}
            >
              <RefreshCw className={cn('size-3.5', isReindexing && 'animate-spin')} />
              {isReindexing ? 'Reindexing…' : 'Reindex'}
            </button>
          }
        />
      </SettingsGroupCard>
    </>
  );
};

type PrivacySettingsDraft = { shareTerminalActivity: boolean; shareUserEdits: boolean };

// ── Appearance ──────────────────────────────────────────────────────────────

const AppearanceSection: React.FC<{ theme: string; setTheme: (t: string) => void }> = ({
  theme,
  setTheme
}) => (
  <>
    <SettingsGroupCard>
      <SettingsRow
        label="Theme"
        description="Choose the interface color scheme."
        control={
          <SettingsSelect
            value={theme}
            onChange={(value) => setTheme(value)}
            options={[
              { value: 'dark', label: 'Dark' },
              { value: 'light', label: 'Light' }
            ]}
            className="w-40"
          />
        }
      />
    </SettingsGroupCard>

    <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
      <ThemePreviewCard
        mode="light"
        title="Light Preview"
        themeName="Paper & Slate"
        isActive={theme === 'light'}
        onApply={() => setTheme('light')}
      />
      <ThemePreviewCard
        mode="dark"
        title="Dark Preview"
        themeName="Zinc & Jet Black"
        isActive={theme === 'dark'}
        onApply={() => setTheme('dark')}
      />
    </div>
  </>
);

const ThemePreviewCard: React.FC<{
  mode: 'light' | 'dark';
  title: string;
  themeName: string;
  isActive: boolean;
  onApply: () => void;
}> = ({ mode, title, themeName, isActive, onApply }) => (
  <button
    type="button"
    onClick={onApply}
    className={cn(
      'overflow-hidden rounded-xl border bg-card text-left transition-colors',
      isActive ? 'border-input-border-focused' : 'border-border hover:border-input-border-hover'
    )}
  >
    <div className="flex items-center justify-between gap-3 border-b border-border px-4 py-3">
      <div>
        <div className="text-ui-base font-semibold text-foreground">{title}</div>
        <div className="text-ui-base text-foreground-subtle">{themeName}</div>
      </div>
      <span
        className={cn(
          'rounded-md px-2.5 py-1 text-ui-xs font-medium',
          isActive ? 'bg-selected text-foreground' : 'bg-surface text-foreground-subtle'
        )}
      >
        {isActive ? 'Active' : mode}
      </span>
    </div>
    <div className="p-2">
      <pre
        className="overflow-hidden rounded-lg border-0 p-3 font-mono text-ui-xs leading-relaxed"
        style={
          mode === 'light'
            ? { background: '#f8f8f8', color: '#0d0d0d' }
            : { background: '#161616', color: '#ffffff' }
        }
      >
{`const agent = createAgent({
  model: "forge-1",
  tools: [readFile, editFile],
})

await agent.run("Refactor the auth module")`}
      </pre>
    </div>
  </button>
);

// ── Model settings ──────────────────────────────────────────────────────────

interface ModelSettingsSectionProps {
  providers: LLMProviderConfig[];
  selectedProviderId?: string;
  currentModel: string;
  setCurrentModel: (model: string) => void;
  updateProvider: (id: string, patch: Partial<LLMProviderConfig>) => void;
  addProvider: (provider: LLMProviderConfig) => void;
  deleteProvider: (id: string) => void;
  addModelToProvider: (id: string, model: string) => void;
  deleteModelFromProvider: (id: string, model: string) => void;
  toggleModelSelection: (id: string, model: string) => void;
  fetchProviderModels: (id: string) => Promise<unknown>;
}

const ModelSettingsSection: React.FC<ModelSettingsSectionProps> = ({
  providers,
  currentModel,
  setCurrentModel,
  updateProvider,
  addProvider,
  deleteProvider,
  addModelToProvider,
  deleteModelFromProvider,
  toggleModelSelection,
  fetchProviderModels
}) => {
  const [selectedProviderId, setSelectedProviderId] = useState<string>(providers[0]?.id || '');
  const selectedProvider = useMemo(
    () => providers.find((p) => p.id === selectedProviderId) || providers[0],
    [providers, selectedProviderId]
  );

  const [showApiKey, setShowApiKey] = useState(false);
  const [isAddingModel, setIsAddingModel] = useState(false);
  const [newModelName, setNewModelName] = useState('');
  const [isAddingProvider, setIsAddingProvider] = useState(false);
  const [newProviderName, setNewProviderName] = useState('');
  const [newProviderBaseUrl, setNewProviderBaseUrl] = useState('');
  const [newProviderApiKey, setNewProviderApiKey] = useState('');
  const [isRefreshing, setIsRefreshing] = useState(false);

  const handleRefreshModels = async () => {
    if (!selectedProvider) return;
    setIsRefreshing(true);
    try {
      await fetchProviderModels(selectedProvider.id);
    } finally {
      setIsRefreshing(false);
    }
  };

  return (
    <div className="w-full overflow-hidden rounded-xl border border-border">
      <div className="grid grid-cols-1 md:grid-cols-[280px_minmax(0,1fr)]">
        {/* Provider list */}
        <div className="flex flex-col gap-1 border-b border-border p-4 md:border-b-0 md:border-r">
          <div className="pb-1 text-ui-sm text-foreground-subtle">Providers</div>
          {providers.map((p) => {
            const isSelected = selectedProvider?.id === p.id;
            return (
              <button
                key={p.id}
                type="button"
                onClick={() => {
                  setSelectedProviderId(p.id);
                  setIsAddingProvider(false);
                }}
                className={cn(
                  'flex w-full items-center gap-2.5 rounded-lg px-3 py-2.5 text-left transition-colors',
                  isSelected
                    ? 'border border-border bg-card shadow-xs'
                    : 'border border-transparent hover:bg-surface-hover'
                )}
              >
                <ProviderLogo name={p.name} />
                <span className="min-w-0 flex-1 truncate text-ui-base text-foreground">{p.name}</span>
                {p.enabled && <div className="size-2 shrink-0 rounded-full bg-success" />}
              </button>
            );
          })}
          <div className="pb-1 pt-4 text-ui-sm text-foreground-subtle">Custom providers</div>
          <button
            type="button"
            onClick={() => setIsAddingProvider(true)}
            className="flex w-full items-center gap-2.5 rounded-lg px-3 py-2.5 text-left transition-colors hover:bg-surface-hover"
          >
            <Plus className="size-4 text-foreground-subtle" />
            <span className="text-ui-base text-foreground">Add provider</span>
          </button>
        </div>

        {/* Detail */}
        <div className="flex flex-col gap-5 p-6">
          {isAddingProvider ? (
            <form
              className="flex flex-col gap-4"
              onSubmit={(e) => {
                e.preventDefault();
                if (!newProviderName.trim()) return;
                const newId = `provider-${Date.now()}`;
                addProvider({
                  id: newId,
                  name: newProviderName.trim(),
                  baseUrl: newProviderBaseUrl.trim() || undefined,
                  apiKey: newProviderApiKey.trim() || undefined,
                  enabled: true,
                  models: ['default-model'],
                  selectedModels: ['default-model']
                });
                setSelectedProviderId(newId);
                setIsAddingProvider(false);
                setNewProviderName('');
                setNewProviderBaseUrl('');
                setNewProviderApiKey('');
              }}
            >
              <div className="text-ui-lg font-semibold text-foreground">Add New Model Provider</div>
              <FormField label="Provider Name">
                <input
                  type="text"
                  value={newProviderName}
                  onChange={(e) => setNewProviderName(e.target.value)}
                  placeholder="e.g. Ollama, OpenRouter, DeepSeek"
                  className={btn.input}
                  required
                  autoFocus
                />
              </FormField>
              <FormField label="Base URL">
                <input
                  type="text"
                  value={newProviderBaseUrl}
                  onChange={(e) => setNewProviderBaseUrl(e.target.value)}
                  placeholder="https://api.openai.com/v1 or http://localhost:11434"
                  className={cn(btn.input, 'font-mono')}
                />
              </FormField>
              <FormField label="API Key">
                <input
                  type="password"
                  value={newProviderApiKey}
                  onChange={(e) => setNewProviderApiKey(e.target.value)}
                  placeholder="sk-..."
                  className={btn.input}
                />
              </FormField>
              <div className="mt-2 flex items-center gap-3">
                <button type="submit" className={btn.primary}>
                  Save Provider
                </button>
                <button type="button" className={btn.outline} onClick={() => setIsAddingProvider(false)}>
                  Cancel
                </button>
              </div>
            </form>
          ) : selectedProvider ? (
            <>
              <div className="flex w-full items-center justify-between pb-2">
                <div className="flex items-center gap-3">
                  <ProviderLogo name={selectedProvider.name} size="lg" />
                  <div className="text-ui-lg font-semibold text-foreground">{selectedProvider.name}</div>
                  <button
                    type="button"
                    onClick={() => updateProvider(selectedProvider.id, { enabled: !selectedProvider.enabled })}
                    className={cn(
                      'rounded-full px-3 py-1 text-ui-base font-semibold transition-colors',
                      selectedProvider.enabled
                        ? 'bg-success/10 text-success'
                        : 'bg-surface-hover text-foreground-subtle'
                    )}
                  >
                    {selectedProvider.enabled ? 'Enabled' : 'Disabled'}
                  </button>
                </div>
                <div className="hidden items-center gap-3 sm:flex">
                  <span className="text-ui-base text-foreground-subtle">Connection mode</span>
                  <div className="flex items-center gap-3 rounded-lg border border-border bg-surface px-4 py-2 text-ui-base text-foreground">
                    <span>API key</span>
                  </div>
                </div>
              </div>

              <FormField label="Base URL">
                <input
                  type="text"
                  value={selectedProvider.baseUrl || ''}
                  onChange={(e) => updateProvider(selectedProvider.id, { baseUrl: e.target.value })}
                  placeholder="https://api.openai.com/v1"
                  className={cn(btn.input, 'w-full font-mono')}
                />
              </FormField>

              <FormField label="API key">
                <div className="flex w-full items-center rounded-lg border border-border bg-background px-3 py-1.5">
                  <input
                    type={showApiKey ? 'text' : 'password'}
                    value={selectedProvider.apiKey || ''}
                    onChange={(e) => updateProvider(selectedProvider.id, { apiKey: e.target.value })}
                    placeholder="Enter API Key..."
                    className="flex-1 border-0 bg-transparent py-1.5 text-ui-sm text-foreground outline-none"
                  />
                  <button
                    type="button"
                    onClick={() => setShowApiKey(!showApiKey)}
                    className="p-1 text-foreground-subtle transition-colors hover:text-foreground"
                  >
                    {showApiKey ? <EyeOff className="size-4" /> : <Eye className="size-4" />}
                  </button>
                </div>
              </FormField>

              <div className="flex w-full flex-col gap-3 pt-2">
                <div className="flex items-center justify-between">
                  <span className="text-ui-base text-foreground-subtle">Model list</span>
                  <button
                    type="button"
                    onClick={handleRefreshModels}
                    className="flex items-center gap-1 text-xs font-medium text-success hover:underline"
                  >
                    <RefreshCw className={cn('size-3', isRefreshing && 'animate-spin')} />
                    <span>Fetch models</span>
                  </button>
                </div>

                <div className="flex max-h-[320px] flex-col gap-2 overflow-y-auto pr-1">
                  {(selectedProvider.models || []).map((modelName) => {
                    const isSelected = (selectedProvider.selectedModels || selectedProvider.models).includes(
                      modelName
                    );
                    const isCurrentActive = currentModel === modelName;
                    return (
                      <div
                        key={modelName}
                        className="group flex w-full items-center gap-3 rounded-[10px] border border-border bg-card p-3 shadow-2xs"
                      >
                        <button
                          type="button"
                          onClick={() => toggleModelSelection(selectedProvider.id, modelName)}
                          className={cn(
                            'flex size-4 items-center justify-center rounded border transition-colors',
                            isSelected
                              ? 'border-success bg-success text-white'
                              : 'border-foreground-subtlest hover:border-foreground'
                          )}
                        >
                          {isSelected && <Check className="size-3 stroke-[3]" />}
                        </button>
                        <button
                          type="button"
                          onClick={() => setCurrentModel(modelName)}
                          className={cn(
                            'flex-1 truncate text-left font-mono text-ui-base',
                            isCurrentActive ? 'font-semibold text-success' : 'text-foreground'
                          )}
                          title="Click to make primary active chat model"
                        >
                          {modelName}
                          {isCurrentActive && <span className="ml-2 font-sans text-xs text-success">(Active)</span>}
                        </button>
                        <KeyRoundIcon />
                        <button
                          type="button"
                          onClick={() => deleteModelFromProvider(selectedProvider.id, modelName)}
                          className="p-1 text-foreground-subtlest transition-colors hover:text-destructive"
                          title="Remove model"
                        >
                          <Trash2 className="size-4" />
                        </button>
                      </div>
                    );
                  })}
                </div>

                {isAddingModel ? (
                  <form
                    className="mt-2 flex items-center gap-2"
                    onSubmit={(e) => {
                      e.preventDefault();
                      if (!newModelName.trim() || !selectedProvider) return;
                      addModelToProvider(selectedProvider.id, newModelName.trim());
                      setNewModelName('');
                      setIsAddingModel(false);
                    }}
                  >
                    <input
                      type="text"
                      value={newModelName}
                      onChange={(e) => setNewModelName(e.target.value)}
                      placeholder="Model identifier (e.g. gpt-4o, claude-3-5-sonnet)"
                      className={cn(btn.input, 'flex-1 font-mono')}
                      autoFocus
                    />
                    <button type="submit" className={btn.primary}>
                      Add
                    </button>
                    <button type="button" className={btn.outline} onClick={() => setIsAddingModel(false)}>
                      Cancel
                    </button>
                  </form>
                ) : (
                  <button
                    type="button"
                    onClick={() => setIsAddingModel(true)}
                    className="mt-1 flex w-fit items-center gap-2.5 rounded-lg border border-border px-4 py-2.5 text-foreground transition-colors hover:bg-surface-hover"
                  >
                    <Plus className="size-4 text-foreground-subtle" />
                    <span className="text-ui-base">Add model</span>
                  </button>
                )}

                <div className="mt-4 flex items-center justify-between border-t border-border pt-4">
                  <span className="text-xs text-foreground-subtle">
                    Provider ID: {selectedProvider.id}
                  </span>
                  <button
                    type="button"
                    onClick={() => {
                      if (confirm(`Delete provider ${selectedProvider.name}?`)) {
                        deleteProvider(selectedProvider.id);
                        setSelectedProviderId(providers[0]?.id || '');
                      }
                    }}
                    className="flex items-center gap-1 text-xs text-destructive hover:underline"
                  >
                    <Trash2 className="size-3.5" />
                    <span>Delete provider</span>
                  </button>
                </div>
              </div>
            </>
          ) : (
            <div className="py-20 text-center text-foreground-subtle">
              Select or add a model provider from the left column.
            </div>
          )}
        </div>
      </div>
    </div>
  );
};

const FormField: React.FC<{ label: string; children: React.ReactNode }> = ({ label, children }) => (
  <div className="flex flex-col gap-1.5">
    <label className="text-ui-base text-foreground-subtle">{label}</label>
    {children}
  </div>
);

const ProviderLogo: React.FC<{ name: string; size?: 'sm' | 'lg' }> = ({ name, size = 'sm' }) => {
  const letter = name.charAt(0).toUpperCase();
  const isZai = name.toLowerCase().includes('z.ai') || name.toLowerCase().includes('zai');
  return (
    <div
      className={cn(
        'flex shrink-0 items-center justify-center rounded-md font-bold',
        size === 'lg' ? 'size-8 text-lg' : 'size-[22px] text-ui-sm',
        isZai ? 'bg-[#111827] text-white dark:bg-[#F2F2F2] dark:text-foreground' : 'bg-surface-hover text-foreground'
      )}
    >
      {letter}
    </div>
  );
};

const KeyRoundIcon: React.FC = () => (
  <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="2" className="size-4 shrink-0 text-foreground-subtlest">
    <path d="M21 2l-2 2m-7.61 7.61a5.5 5.5 0 1 1-7.778 7.778 5.5 5.5 0 0 1 7.777-7.777zm0 0L15.5 7.5m0 0l3 3L22 7l-3-3m-3.5 3.5L19 4" />
  </svg>
);

// ── Browser Use ─────────────────────────────────────────────────────────────

const BrowserUseSection: React.FC<{
  browserSettings: any;
  updateBrowserSettings: (patch: any) => void;
}> = ({ browserSettings, updateBrowserSettings }) => (
  <SettingsGroupCard>
    <SettingsRow
      label="Enable Browser Use"
      description="Let the agent open, navigate and extract from embedded browser sessions."
      control={
        <Switch
          checked={browserSettings.enabled}
          onCheckedChange={(checked) => updateBrowserSettings({ enabled: checked })}
        />
      }
    />
    <SettingsRow
      label="Run headless"
      description="Hide the browser window while the agent works."
      control={
        <Switch
          checked={browserSettings.headless}
          onCheckedChange={(checked) => updateBrowserSettings({ headless: checked })}
        />
      }
    />
    <SettingsRow
      label="Allow JavaScript"
      description="Permit page scripting during automated browsing."
      control={
        <Switch
          checked={browserSettings.allowJavaScript}
          onCheckedChange={(checked) => updateBrowserSettings({ allowJavaScript: checked })}
        />
      }
    />
    <SettingsRow
      label="Search provider"
      description="Engine used for web search tooling."
      control={
        <SettingsSelect
          value={browserSettings.searchProvider}
          onChange={(value) => updateBrowserSettings({ searchProvider: value })}
          options={[
            { value: 'google', label: 'Google' },
            { value: 'duckduckgo', label: 'DuckDuckGo' },
            { value: 'bing', label: 'Bing' },
            { value: 'tavily', label: 'Tavily' },
            { value: 'searxng', label: 'SearXNG' }
          ]}
        />
      }
    />
    <SettingsRow
      label="Viewport"
      description="Default browser viewport size."
      control={
        <SettingsSelect
          value={browserSettings.viewport}
          onChange={(value) => updateBrowserSettings({ viewport: value })}
          options={[
            { value: '1280x800', label: '1280 × 800' },
            { value: '1920x1080', label: '1920 × 1080' },
            { value: '390x844', label: '390 × 844' }
          ]}
        />
      }
    />
  </SettingsGroupCard>
);

// ── Keyboard Shortcuts ──────────────────────────────────────────────────────

const ShortcutsSection: React.FC = () => {
  const [query, setQuery] = useState('');
  const [keyQuery, setKeyQuery] = useState('');
  const [recording, setRecording] = useState(false);
  const keyInputRef = React.useRef<HTMLInputElement | null>(null);

  const rows = SHORTCUT_ROWS.filter((row) => {
    const matchText =
      !query.trim() || row.command.toLowerCase().includes(query.trim().toLowerCase());
    const matchKey = !keyQuery.trim() || row.binding.toLowerCase().includes(keyQuery.trim().toLowerCase());
    return matchText && matchKey;
  });

  return (
    <>
      <div className="flex flex-col gap-3">
        <SettingsSearchInput
          value={query}
          onChange={setQuery}
          placeholder="Search shortcuts"
          testId="settings-shortcut-search"
        />
        <div className="relative">
          <KeyboardIcon
            className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-foreground-subtle"
            aria-hidden="true"
          />
          <input
            ref={keyInputRef}
            value={recording ? '' : keyQuery}
            readOnly={!recording}
            onKeyDown={(event) => {
              if (!recording) return;
              event.preventDefault();
              const parts: string[] = [];
              if (event.metaKey) parts.push('⌘');
              if (event.ctrlKey) parts.push('Ctrl');
              if (event.altKey) parts.push('Alt');
              if (event.shiftKey) parts.push('Shift');
              if (event.key.length === 1 && event.key !== ' ') parts.push(event.key.toUpperCase());
              if (parts.length > 0) {
                setKeyQuery(parts.join(''));
                setRecording(false);
              }
              if (event.key === 'Escape') {
                setRecording(false);
              }
            }}
            placeholder={recording ? 'Press a key combination to search…' : 'Search by key combination'}
            className="h-9 w-full rounded-xl border border-border bg-background pl-9 pr-3 text-ui-sm text-foreground outline-none transition-colors placeholder:text-foreground-subtlest hover:border-input-border-hover focus:border-input-border-focused"
          />
        </div>
      </div>

      <SettingsGroupCard>
        <div className="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)_80px_72px] gap-2 border-b border-border px-4 py-2 text-ui-xs font-medium text-foreground-subtlest">
          <span>Command</span>
          <span>Keybinding</span>
          <span className="text-right">Scope</span>
          <span className="text-right">Actions</span>
        </div>
        {rows.map((row) => (
          <div
            key={row.command}
            className="grid grid-cols-[minmax(0,1fr)_minmax(0,1fr)_80px_72px] items-center gap-2 border-t border-border px-4 py-2 text-ui-base first:border-t-0"
          >
            <span className="min-w-0 truncate text-foreground">{row.command}</span>
            <span className="min-w-0">
              <kbd className="rounded-md border border-border bg-surface px-2 py-0.5 font-mono text-ui-sm text-foreground">
                {row.binding}
              </kbd>
            </span>
            <span className="text-right text-foreground-subtle">{row.scope}</span>
            <span className="flex justify-end">
              <button
                type="button"
                className="flex size-7 items-center justify-center rounded-md text-foreground-subtle transition-colors hover:bg-surface-hover hover:text-foreground"
                title="Record new binding (not yet configurable)"
                onClick={() => {
                  setRecording(true);
                  keyInputRef.current?.focus();
                }}
              >
                <KeyboardIcon className="size-3.5" />
              </button>
            </span>
          </div>
        ))}
        {rows.length === 0 ? (
          <div className="px-4 py-6 text-center text-foreground-subtle">No matching commands</div>
        ) : null}
      </SettingsGroupCard>
    </>
  );
};

// ── Memory ──────────────────────────────────────────────────────────────────

const MemorySection: React.FC<{
  memories: AgentMemoryEntry[];
  saveMemory: (entry: AgentMemoryEntry) => Promise<unknown>;
  deleteMemory: (id: string) => void;
  reloadMemories: () => Promise<unknown>;
}> = ({ memories, saveMemory, deleteMemory, reloadMemories }) => {
  const [isReloading, setIsReloading] = useState(false);
  const [isAdding, setIsAdding] = useState(false);
  const [newKey, setNewKey] = useState('');
  const [newContent, setNewContent] = useState('');
  const [newCategory, setNewCategory] = useState('project');
  const [newScope, setNewScope] = useState<'workspace' | 'global'>('workspace');
  const [isSaving, setIsSaving] = useState(false);
  const [search, setSearch] = useState('');
  const [categoryFilter, setCategoryFilter] = useState('all');

  const filtered = memories.filter((m) => {
    const matchSearch =
      !search ||
      m.key.toLowerCase().includes(search.toLowerCase()) ||
      m.content.toLowerCase().includes(search.toLowerCase());
    const matchCat = categoryFilter === 'all' || m.category === categoryFilter;
    return matchSearch && matchCat;
  });

  return (
    <>
      <div className="flex items-start justify-between gap-3">
        <p className="min-w-0 flex-1 text-ui-base leading-6 text-foreground-subtle">
          Persistent rules, preferences, and project patterns remembered across all turns.
        </p>
        <div className="flex shrink-0 items-center gap-2">
          <button
            type="button"
            className={btn.ghost}
            disabled={isReloading}
            onClick={async () => {
              setIsReloading(true);
              try {
                await reloadMemories();
              } finally {
                setIsReloading(false);
              }
            }}
          >
            <RefreshCw className={cn('size-3.5', isReloading && 'animate-spin')} />
            Refresh
          </button>
          <button type="button" className={btn.primary} onClick={() => setIsAdding((prev) => !prev)}>
            <Plus className="size-3.5" />
            Add Memory
          </button>
        </div>
      </div>

      {isAdding ? (
        <SettingsGroupCard className="p-4">
          <form
            className="flex flex-col gap-4"
            onSubmit={async (e) => {
              e.preventDefault();
              if (!newKey.trim() || !newContent.trim()) return;
              setIsSaving(true);
              try {
                await saveMemory({
                  id: `mem-${Date.now()}`,
                  key: newKey.trim().toLowerCase().replace(/\s+/g, '-'),
                  content: newContent.trim(),
                  category: newCategory,
                  scope: newScope
                } as AgentMemoryEntry);
                setIsAdding(false);
                setNewKey('');
                setNewContent('');
              } catch (err: any) {
                alert(`Failed to save memory: ${err.message}`);
              } finally {
                setIsSaving(false);
              }
            }}
          >
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
              <FormField label="Memory Key">
                <input
                  type="text"
                  value={newKey}
                  onChange={(e) => setNewKey(e.target.value)}
                  placeholder="e.g. auth-flow, design-system"
                  className={cn(btn.input, 'font-mono')}
                  required
                />
              </FormField>
              <FormField label="Category">
                <SettingsSelect
                  value={newCategory}
                  onChange={setNewCategory}
                  className="w-full"
                  options={[
                    { value: 'project', label: 'Project (Context)' },
                    { value: 'architecture', label: 'Architecture' },
                    { value: 'rule', label: 'Rule / Standard' },
                    { value: 'preference', label: 'User Preference' }
                  ]}
                />
              </FormField>
              <FormField label="Scope">
                <SettingsSelect
                  value={newScope}
                  onChange={(value) => setNewScope(value as 'workspace' | 'global')}
                  className="w-full"
                  options={[
                    { value: 'workspace', label: 'Workspace (.forge/)' },
                    { value: 'global', label: 'Global (~/.forge-ade/)' }
                  ]}
                />
              </FormField>
            </div>
            <FormField label="Memory Content">
              <textarea
                rows={3}
                value={newContent}
                onChange={(e) => setNewContent(e.target.value)}
                placeholder="Describe the architectural rule, decision, or project knowledge..."
                className={btn.input}
                required
              />
            </FormField>
            <div className="flex items-center justify-end gap-2">
              <button type="button" className={btn.outline} onClick={() => setIsAdding(false)}>
                Cancel
              </button>
              <button type="submit" className={btn.primary} disabled={isSaving}>
                {isSaving ? 'Saving…' : 'Save to Memory'}
              </button>
            </div>
          </form>
        </SettingsGroupCard>
      ) : null}

      <div className="flex flex-col gap-3 sm:flex-row sm:items-center">
        <SettingsSearchInput
          value={search}
          onChange={setSearch}
          placeholder="Search memories by key or content..."
          className="flex-1"
        />
        <div className="flex items-center gap-1.5">
          {['all', 'project', 'architecture', 'rule', 'preference'].map((cat) => (
            <button
              key={cat}
              type="button"
              onClick={() => setCategoryFilter(cat)}
              className={cn(
                'rounded-full px-3 py-1 text-ui-sm font-medium capitalize transition-colors',
                categoryFilter === cat
                  ? 'bg-selected text-foreground'
                  : 'text-foreground-subtle hover:bg-hover hover:text-foreground'
              )}
            >
              {cat}
            </button>
          ))}
        </div>
      </div>

      {filtered.length === 0 ? (
        <SettingsEmptyState>
          No memories saved yet. Click "Add Memory" to create your first persistent rule.
        </SettingsEmptyState>
      ) : (
        <SettingsResourceList>
          {filtered.map((m, index) => (
            <React.Fragment key={m.id || m.key}>
              {index > 0 && <SettingsResourceSeparator />}
              <div className="flex flex-col gap-2.5 px-4 py-3 transition-colors hover:bg-hover">
                <div className="flex items-center justify-between">
                  <div className="flex min-w-0 items-center gap-2">
                    <span className="truncate font-mono text-ui-sm font-semibold text-foreground">
                      {m.key}
                    </span>
                    <SettingsScopeBadge scope={m.scope || 'workspace'} />
                    <SettingsBadge>{m.category || 'project'}</SettingsBadge>
                  </div>
                  <div className="flex items-center gap-1">
                    <button
                      type="button"
                      onClick={() => navigator.clipboard.writeText(m.content)}
                      className="rounded p-1 text-foreground-subtle transition-colors hover:bg-surface-hover hover:text-foreground"
                      title="Copy content"
                    >
                      <Copy className="size-3.5" />
                    </button>
                    <button
                      type="button"
                      onClick={() => {
                        if (confirm(`Delete memory "${m.key}"?`)) deleteMemory(m.id);
                      }}
                      className="rounded p-1 text-foreground-subtle transition-colors hover:bg-destructive/10 hover:text-destructive"
                      title="Delete memory"
                    >
                      <Trash2 className="size-3.5" />
                    </button>
                  </div>
                </div>
                <div className="whitespace-pre-wrap rounded-lg border border-border bg-background px-3 py-2 font-mono text-xs leading-relaxed text-foreground-subtle">
                  {m.content}
                </div>
              </div>
            </React.Fragment>
          ))}
        </SettingsResourceList>
      )}
    </>
  );
};

// ── Subagents ───────────────────────────────────────────────────────────────

const SubagentsSection: React.FC<{
  agents: Array<{ id: string; name: string; description?: string; endpoint?: string; enabled: boolean }>;
  toggleAgentEnabled: (id: string) => void;
}> = ({ agents, toggleAgentEnabled }) => (
  <>
    <p className="text-ui-base leading-6 text-foreground-subtle">
      Specialized subagent executors and protocol endpoints available to the agent.
    </p>
    {agents.length === 0 ? (
      <SettingsEmptyState>No subagents configured yet.</SettingsEmptyState>
    ) : (
      <SettingsResourceList>
        {agents.map((ag, index) => (
          <React.Fragment key={ag.id}>
            {index > 0 && <SettingsResourceSeparator />}
            <div className="flex items-center gap-3 px-4 py-3 transition-colors hover:bg-hover">
              <span className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-surface text-foreground-subtle">
                <Bot className="size-4" />
              </span>
              <div className="min-w-0 flex-1">
                <div className="text-ui-base font-semibold text-foreground">{ag.name}</div>
                <div className="truncate text-ui-sm text-foreground-subtle">
                  {ag.endpoint || 'Internal Loop'}{ag.description ? ` · ${ag.description}` : ''}
                </div>
              </div>
              <Switch checked={ag.enabled} onCheckedChange={() => toggleAgentEnabled(ag.id)} />
            </div>
          </React.Fragment>
        ))}
      </SettingsResourceList>
    )}
  </>
);

// ── Plugins ─────────────────────────────────────────────────────────────────

const PluginsSection: React.FC<{
  plugins: any[];
  createPlugin: (req: any) => Promise<unknown>;
  togglePlugin: (id: string, enabled?: boolean) => void;
  deletePlugin: (id: string) => void;
  reloadPlugins: () => Promise<unknown>;
}> = ({ plugins, createPlugin, togglePlugin, deletePlugin, reloadPlugins }) => {
  const [search, setSearch] = useState('');
  const [isReloading, setIsReloading] = useState(false);
  const [isCreating, setIsCreating] = useState(false);
  const [newId, setNewId] = useState('');
  const [newName, setNewName] = useState('');
  const [newDesc, setNewDesc] = useState('');
  const [newScope, setNewScope] = useState<'workspace' | 'global'>('workspace');
  const [error, setError] = useState('');

  const filtered = plugins.filter((p) => {
    if (!search.trim()) return true;
    const q = search.toLowerCase();
    return (
      p.id.toLowerCase().includes(q) ||
      p.name.toLowerCase().includes(q) ||
      (p.description && p.description.toLowerCase().includes(q))
    );
  });

  return (
    <>
      <div className="flex items-start justify-between gap-3">
        <p className="min-w-0 flex-1 text-ui-base leading-6 text-foreground-subtle">
          Plugins extend agent capabilities with dynamic shell tools, scripts, and custom system
          prompts. Browse and install more from the Plugin Marketplace.
        </p>
        <div className="flex shrink-0 items-center gap-2">
          <button
            type="button"
            className={btn.ghost}
            disabled={isReloading}
            onClick={async () => {
              setIsReloading(true);
              try {
                await reloadPlugins();
              } finally {
                setIsReloading(false);
              }
            }}
          >
            <RefreshCw className={cn('size-3.5', isReloading && 'animate-spin')} />
            Refresh
          </button>
          <button type="button" className={btn.primary} onClick={() => setIsCreating((prev) => !prev)}>
            <Plus className="size-3.5" />
            Create Plugin
          </button>
        </div>
      </div>

      <SettingsSearchInput
        value={search}
        onChange={setSearch}
        placeholder="Filter plugins by name, ID, or tool..."
      />

      {isCreating ? (
        <SettingsGroupCard className="p-4">
          <form
            className="flex flex-col gap-4"
            onSubmit={async (e) => {
              e.preventDefault();
              if (!newId.trim() || !newName.trim()) return;
              setError('');
              try {
                await createPlugin({
                  id: newId.trim().toLowerCase().replace(/[^a-z0-9_-]/g, '-'),
                  name: newName.trim(),
                  description: newDesc.trim(),
                  scope: newScope
                });
                setIsCreating(false);
                setNewId('');
                setNewName('');
                setNewDesc('');
              } catch (err: any) {
                setError(err.message || 'Failed to create plugin');
              }
            }}
          >
            {error ? (
              <div className="rounded-lg border border-destructive/30 bg-destructive/10 px-3 py-2 text-xs text-destructive">
                {error}
              </div>
            ) : null}
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
              <FormField label="Plugin ID">
                <input
                  type="text"
                  value={newId}
                  onChange={(e) => setNewId(e.target.value)}
                  placeholder="e.g. docker-tools"
                  className={cn(btn.input, 'font-mono')}
                  required
                />
              </FormField>
              <FormField label="Name">
                <input
                  type="text"
                  value={newName}
                  onChange={(e) => setNewName(e.target.value)}
                  placeholder="e.g. Docker Tools"
                  className={btn.input}
                  required
                />
              </FormField>
              <FormField label="Scope">
                <SettingsSelect
                  value={newScope}
                  onChange={(value) => setNewScope(value as 'workspace' | 'global')}
                  className="w-full"
                  options={[
                    { value: 'workspace', label: 'Workspace (.forge/plugins)' },
                    { value: 'global', label: 'Global (~/.forge-ade/plugins)' }
                  ]}
                />
              </FormField>
            </div>
            <FormField label="Description">
              <input
                type="text"
                value={newDesc}
                onChange={(e) => setNewDesc(e.target.value)}
                placeholder="Short description of capabilities provided"
                className={btn.input}
              />
            </FormField>
            <div className="flex items-center justify-end gap-2">
              <button type="button" className={btn.outline} onClick={() => setIsCreating(false)}>
                Cancel
              </button>
              <button type="submit" className={btn.primary}>
                Create Plugin
              </button>
            </div>
          </form>
        </SettingsGroupCard>
      ) : null}

      {filtered.length === 0 ? (
        <SettingsEmptyState>
          No plugins found. Create one above or place plugins in{' '}
          <code className="rounded bg-surface px-1 font-mono">.forge/plugins</code>.
        </SettingsEmptyState>
      ) : (
        <SettingsResourceList>
          {filtered.map((p, index) => (
            <React.Fragment key={p.id}>
              {index > 0 && <SettingsResourceSeparator />}
              <div className={cn('flex items-start gap-3 px-4 py-3 transition-colors hover:bg-hover', !p.enabled && 'opacity-70')}>
                <span className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-surface text-foreground-subtle">
                  <Blocks className="size-4" />
                </span>
                <div className="min-w-0 flex-1">
                  <div className="flex min-w-0 flex-wrap items-center gap-2">
                    <span className="text-ui-base font-semibold text-foreground">{p.name}</span>
                    <span className="font-mono text-xs text-foreground-subtle">({p.id})</span>
                    <SettingsScopeBadge scope={p.source || 'workspace'} />
                    {p.version ? <SettingsBadge>v{p.version}</SettingsBadge> : null}
                  </div>
                  {p.description ? (
                    <div className="mt-1 text-xs leading-relaxed text-foreground-subtle">
                      {p.description}
                    </div>
                  ) : null}
                  {p.tools && p.tools.length > 0 ? (
                    <div className="mt-2 flex flex-wrap items-center gap-1.5">
                      {p.tools.map((t: any) => (
                        <span
                          key={t.name}
                          title={t.description || t.command || t.name}
                          className="flex items-center gap-1 rounded-md border border-border bg-background px-2 py-0.5 font-mono text-ui-xs text-foreground"
                        >
                          <Wrench className="size-3" />
                          {t.name}
                        </span>
                      ))}
                    </div>
                  ) : null}
                </div>
                <div className="flex shrink-0 items-center gap-2">
                  <Switch
                    checked={p.enabled}
                    onCheckedChange={(checked) => togglePlugin(p.id, checked)}
                  />
                  {p.source !== 'builtin' ? (
                    <button
                      type="button"
                      onClick={() => {
                        if (confirm(`Delete plugin ${p.name}?`)) deletePlugin(p.id);
                      }}
                      className="rounded-md p-1.5 text-foreground-subtlest transition-colors hover:bg-destructive/10 hover:text-destructive"
                      title="Delete plugin"
                    >
                      <Trash2 className="size-4" />
                    </button>
                  ) : null}
                </div>
              </div>
            </React.Fragment>
          ))}
        </SettingsResourceList>
      )}
    </>
  );
};

// ── MCP Servers ─────────────────────────────────────────────────────────────

const McpSection: React.FC<{
  mcps: any[];
  addMcp: (mcp: any) => void;
  toggleMcp: (id: string) => void;
  deleteMcp: (id: string) => void;
}> = ({ mcps, addMcp, toggleMcp, deleteMcp }) => {
  const [isAdding, setIsAdding] = useState(false);
  const [newName, setNewName] = useState('');
  const [newCommand, setNewCommand] = useState('');

  return (
    <>
      <p className="text-ui-base leading-6 text-foreground-subtle">
        Model Context Protocol servers provide standard tool execution interfaces to Forge agents.
      </p>
      {isAdding ? (
        <SettingsGroupCard className="p-4">
          <form
            className="flex flex-col gap-3"
            onSubmit={(e) => {
              e.preventDefault();
              if (!newName.trim() || !newCommand.trim()) return;
              addMcp({
                id: `mcp-${Date.now()}`,
                name: newName.trim(),
                command: newCommand.trim(),
                status: 'connected',
                enabled: true,
                tools: ['tool_call']
              });
              setNewName('');
              setNewCommand('');
              setIsAdding(false);
            }}
          >
            <FormField label="MCP Server Name">
              <input
                type="text"
                value={newName}
                onChange={(e) => setNewName(e.target.value)}
                className={btn.input}
                required
              />
            </FormField>
            <FormField label="Command">
              <input
                type="text"
                value={newCommand}
                onChange={(e) => setNewCommand(e.target.value)}
                placeholder="e.g. npx -y @modelcontextprotocol/server-git"
                className={cn(btn.input, 'font-mono')}
                required
              />
            </FormField>
            <div className="flex justify-end gap-2">
              <button type="button" className={btn.outline} onClick={() => setIsAdding(false)}>
                Cancel
              </button>
              <button type="submit" className={btn.primary}>
                Save MCP Server
              </button>
            </div>
          </form>
        </SettingsGroupCard>
      ) : (
        <button type="button" className={cn(btn.outline, 'w-fit')} onClick={() => setIsAdding(true)}>
          <Plus className="size-4 text-foreground-subtle" />
          Add MCP Server
        </button>
      )}

      {mcps.length === 0 ? (
        <SettingsEmptyState>No MCP servers configured yet.</SettingsEmptyState>
      ) : (
        <SettingsResourceList>
          {mcps.map((mcp, index) => (
            <React.Fragment key={mcp.id}>
              {index > 0 && <SettingsResourceSeparator />}
              <div className="flex items-center gap-3 px-4 py-3 transition-colors hover:bg-hover">
                <span className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-surface text-success">
                  <Cable className="size-4" />
                </span>
                <div className="min-w-0 flex-1">
                  <div className="text-ui-base font-semibold text-foreground">{mcp.name}</div>
                  <div className="truncate font-mono text-xs text-foreground-subtle">{mcp.command}</div>
                  {(mcp.tools || []).length > 0 ? (
                    <div className="mt-1.5 flex flex-wrap gap-1.5">
                      {mcp.tools.map((t: string) => (
                        <span
                          key={t}
                          className="rounded-full bg-background px-2 py-0.5 font-mono text-ui-xs text-foreground-subtle"
                        >
                          {t}
                        </span>
                      ))}
                    </div>
                  ) : null}
                </div>
                <Switch checked={mcp.enabled} onCheckedChange={() => toggleMcp(mcp.id)} />
                <button
                  type="button"
                  onClick={() => deleteMcp(mcp.id)}
                  className="rounded-md p-1.5 text-foreground-subtlest transition-colors hover:bg-destructive/10 hover:text-destructive"
                >
                  <Trash2 className="size-4" />
                </button>
              </div>
            </React.Fragment>
          ))}
        </SettingsResourceList>
      )}
    </>
  );
};

// ── Skills ──────────────────────────────────────────────────────────────────

const SkillsSection: React.FC<{
  skills: any[];
  toggleSkill: (id: string) => void;
  deleteSkill: (id: string) => void;
  createBackendSkill: (req: any) => Promise<unknown>;
  deleteBackendSkill?: (name: string) => Promise<unknown>;
  reloadSkills: () => Promise<unknown>;
  discoveredSkills: any[];
  importDiscoveredSkill: (name: string) => any;
  runDiscovery: () => Promise<unknown>;
  isDiscovering: boolean;
}> = ({
  skills,
  toggleSkill,
  deleteSkill,
  createBackendSkill,
  deleteBackendSkill,
  reloadSkills,
  discoveredSkills,
  importDiscoveredSkill,
  runDiscovery,
  isDiscovering
}) => {
  const [search, setSearch] = useState('');
  const [isReloading, setIsReloading] = useState(false);
  const [isCreating, setIsCreating] = useState(false);
  const [newName, setNewName] = useState('');
  const [newDesc, setNewDesc] = useState('');
  const [newScope, setNewScope] = useState<'workspace' | 'global'>('workspace');
  const [newBody, setNewBody] = useState('');
  const [error, setError] = useState('');
  const [expandedId, setExpandedId] = useState<string | null>(null);

  const filtered = skills.filter((sk) => {
    if (!search.trim()) return true;
    const q = search.toLowerCase();
    return (
      sk.name.toLowerCase().includes(q) ||
      (sk.description && sk.description.toLowerCase().includes(q))
    );
  });

  return (
    <>
      <div className="flex items-start justify-between gap-3">
        <p className="min-w-0 flex-1 text-ui-base leading-6 text-foreground-subtle">
          Skills inject specialized domain workflows and instructions into coding sessions on demand.
        </p>
        <div className="flex shrink-0 items-center gap-2">
          <button
            type="button"
            className={btn.ghost}
            disabled={isReloading}
            onClick={async () => {
              setIsReloading(true);
              try {
                await reloadSkills();
              } finally {
                setIsReloading(false);
              }
            }}
          >
            <RefreshCw className={cn('size-3.5', isReloading && 'animate-spin')} />
            Refresh
          </button>
          <button type="button" className={btn.ghost} disabled={isDiscovering} onClick={() => void runDiscovery()}>
            <Compass className={cn('size-3.5', isDiscovering && 'animate-spin')} />
            Discover
          </button>
          <button type="button" className={btn.primary} onClick={() => setIsCreating((prev) => !prev)}>
            <Plus className="size-3.5" />
            Create Skill
          </button>
        </div>
      </div>

      <SettingsSearchInput
        value={search}
        onChange={setSearch}
        placeholder="Filter skills by name or description..."
      />

      {isCreating ? (
        <SettingsGroupCard className="p-4">
          <form
            className="flex flex-col gap-4"
            onSubmit={async (e) => {
              e.preventDefault();
              if (!newName.trim() || !newBody.trim()) return;
              setError('');
              try {
                await createBackendSkill({
                  name: newName.trim().toLowerCase().replace(/[^a-z0-9_-]/g, '-'),
                  description: newDesc.trim(),
                  scope: newScope,
                  body: newBody.trim()
                });
                setIsCreating(false);
                setNewName('');
                setNewDesc('');
                setNewBody('');
              } catch (err: any) {
                setError(err.message || 'Failed to create skill');
              }
            }}
          >
            {error ? (
              <div className="rounded-lg border border-destructive/30 bg-destructive/10 px-3 py-2 text-xs text-destructive">
                {error}
              </div>
            ) : null}
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
              <FormField label="Skill Name">
                <input
                  type="text"
                  value={newName}
                  onChange={(e) => setNewName(e.target.value)}
                  placeholder="e.g. clean-architecture"
                  className={btn.input}
                  required
                />
              </FormField>
              <FormField label="Scope">
                <SettingsSelect
                  value={newScope}
                  onChange={(value) => setNewScope(value as 'workspace' | 'global')}
                  className="w-full"
                  options={[
                    { value: 'workspace', label: 'Workspace (.forge/skills)' },
                    { value: 'global', label: 'Global (~/.forge-ade/skills)' }
                  ]}
                />
              </FormField>
            </div>
            <FormField label="Description">
              <input
                type="text"
                value={newDesc}
                onChange={(e) => setNewDesc(e.target.value)}
                placeholder="Short summary of what this skill does and when to use it"
                className={btn.input}
                required
              />
            </FormField>
            <FormField label="Instructions / Prompt Body (Markdown)">
              <textarea
                rows={5}
                value={newBody}
                onChange={(e) => setNewBody(e.target.value)}
                placeholder="Detailed instructions injected into the agent when this skill is invoked..."
                className={btn.input}
                required
              />
            </FormField>
            <div className="flex justify-end gap-2">
              <button type="button" className={btn.outline} onClick={() => setIsCreating(false)}>
                Cancel
              </button>
              <button type="submit" className={btn.primary}>
                Create Skill
              </button>
            </div>
          </form>
        </SettingsGroupCard>
      ) : null}

      {discoveredSkills.length > 0 ? (
        <SettingsGroupCard className="p-4">
          <div className="mb-2 text-ui-sm font-medium text-foreground">Discovered skills</div>
          <div className="flex flex-wrap gap-2">
            {discoveredSkills.map((ds: any) => (
              <button
                key={ds.name}
                type="button"
                onClick={() => void importDiscoveredSkill(ds.name)}
                className="flex items-center gap-1.5 rounded-full border border-border px-3 py-1 text-ui-sm text-foreground transition-colors hover:bg-surface-hover"
                title={ds.description}
              >
                <Plus className="size-3" />
                {ds.name}
              </button>
            ))}
          </div>
        </SettingsGroupCard>
      ) : null}

      {filtered.length === 0 ? (
        <SettingsEmptyState>No skills found. Create one or discover skills from other agents.</SettingsEmptyState>
      ) : (
        <SettingsResourceList>
          {filtered.map((sk, index) => (
            <React.Fragment key={sk.id}>
              {index > 0 && <SettingsResourceSeparator />}
              <div className={cn('flex items-start gap-3 px-4 py-3 transition-colors hover:bg-hover', !sk.enabled && 'opacity-70')}>
                <span className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-surface text-foreground-subtle">
                  <WandSparkles className="size-4" />
                </span>
                <div className="min-w-0 flex-1">
                  <div className="flex min-w-0 flex-wrap items-center gap-2">
                    <span className="text-ui-base font-semibold text-foreground">{sk.name}</span>
                    <SettingsScopeBadge scope={sk.origin || 'workspace'} />
                    {sk.category && sk.category !== sk.origin ? (
                      <SettingsBadge>{sk.category}</SettingsBadge>
                    ) : null}
                  </div>
                  <div className="mt-1 text-xs leading-relaxed text-foreground-subtle">
                    {sk.description}
                  </div>
                  {sk.trigger ? (
                    <div className="mt-1 font-mono text-ui-xs text-foreground-subtle">
                      Trigger: <code className="rounded bg-background px-1.5 py-0.5 text-foreground">{sk.trigger}</code>
                    </div>
                  ) : null}
                  {sk.instructions ? (
                    <div className="mt-2">
                      <button
                        type="button"
                        onClick={() => setExpandedId(expandedId === sk.id ? null : sk.id)}
                        className="flex items-center gap-1 text-ui-xs text-foreground-subtle transition-colors hover:text-foreground"
                      >
                        {expandedId === sk.id ? (
                          <ChevronDown className="size-3" />
                        ) : (
                          <ChevronRight className="size-3" />
                        )}
                        {expandedId === sk.id ? 'Hide Instructions' : 'View Instructions'}
                      </button>
                      {expandedId === sk.id ? (
                        <pre className="mt-2 max-h-60 overflow-auto whitespace-pre-wrap rounded-lg border border-border bg-background p-3 font-mono text-ui-xs leading-relaxed text-foreground">
                          {sk.instructions}
                        </pre>
                      ) : null}
                    </div>
                  ) : null}
                </div>
                <div className="flex shrink-0 items-center gap-2">
                  <Switch checked={sk.enabled} onCheckedChange={() => toggleSkill(sk.id)} />
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
                    className="rounded-md p-1.5 text-foreground-subtlest transition-colors hover:bg-destructive/10 hover:text-destructive"
                    title="Delete skill"
                  >
                    <Trash2 className="size-4" />
                  </button>
                </div>
              </div>
            </React.Fragment>
          ))}
        </SettingsResourceList>
      )}
    </>
  );
};

// ── Commands ────────────────────────────────────────────────────────────────

const CommandsSection: React.FC<{
  customCommands: any[];
  addCustomCommand: (cmd: any) => void;
  updateCustomCommand: (id: string, patch: any) => void;
  deleteCustomCommand: (id: string) => void;
  toggleCustomCommand: (id: string) => void;
}> = ({ customCommands, addCustomCommand, deleteCustomCommand, toggleCustomCommand }) => {
  const [isAdding, setIsAdding] = useState(false);
  const [newName, setNewName] = useState('');
  const [newDesc, setNewDesc] = useState('');
  const [newPrompt, setNewPrompt] = useState('');
  const [newScope, setNewScope] = useState<'workspace' | 'global'>('workspace');

  return (
    <>
      <div className="flex items-start justify-between gap-3">
        <p className="min-w-0 flex-1 text-ui-base leading-6 text-foreground-subtle">
          Define slash-command shortcuts that preload proven system prompts for your workflow.
        </p>
        <button type="button" className={cn(btn.primary, 'shrink-0')} onClick={() => setIsAdding((prev) => !prev)}>
          <Plus className="size-3.5" />
          New Command
        </button>
      </div>

      {isAdding ? (
        <SettingsGroupCard className="p-4">
          <form
            className="flex flex-col gap-3"
            onSubmit={(e) => {
              e.preventDefault();
              if (!newName.trim() || !newPrompt.trim()) return;
              addCustomCommand({
                id: `cmd-${Date.now()}`,
                name: newName.trim().replace(/^\//, ''),
                description: newDesc.trim(),
                prompt: newPrompt.trim(),
                scope: newScope,
                enabled: true
              });
              setNewName('');
              setNewDesc('');
              setNewPrompt('');
              setIsAdding(false);
            }}
          >
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-2">
              <FormField label="Command Name">
                <input
                  type="text"
                  value={newName}
                  onChange={(e) => setNewName(e.target.value)}
                  placeholder="e.g. review-security"
                  className={cn(btn.input, 'font-mono')}
                  required
                />
              </FormField>
              <FormField label="Scope">
                <SettingsSelect
                  value={newScope}
                  onChange={(value) => setNewScope(value as 'workspace' | 'global')}
                  className="w-full"
                  options={[
                    { value: 'workspace', label: 'Workspace' },
                    { value: 'global', label: 'Global' }
                  ]}
                />
              </FormField>
            </div>
            <FormField label="Description">
              <input
                type="text"
                value={newDesc}
                onChange={(e) => setNewDesc(e.target.value)}
                className={btn.input}
              />
            </FormField>
            <FormField label="Prompt Template">
              <textarea
                rows={4}
                value={newPrompt}
                onChange={(e) => setNewPrompt(e.target.value)}
                className={btn.input}
                required
              />
            </FormField>
            <div className="flex justify-end gap-2">
              <button type="button" className={btn.outline} onClick={() => setIsAdding(false)}>
                Cancel
              </button>
              <button type="submit" className={btn.primary}>
                Save Command
              </button>
            </div>
          </form>
        </SettingsGroupCard>
      ) : null}

      {customCommands.length === 0 ? (
        <SettingsEmptyState>No custom commands yet.</SettingsEmptyState>
      ) : (
        <SettingsResourceList>
          {customCommands.map((cmd, index) => (
            <React.Fragment key={cmd.id}>
              {index > 0 && <SettingsResourceSeparator />}
              <div className="flex items-center gap-3 px-4 py-3 transition-colors hover:bg-hover">
                <span className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-surface font-mono text-foreground-subtle">
                  /
                </span>
                <div className="min-w-0 flex-1">
                  <div className="flex min-w-0 items-center gap-2">
                    <span className="truncate font-mono text-ui-base font-semibold text-foreground">
                      /{cmd.name}
                    </span>
                    <SettingsScopeBadge scope={cmd.scope || 'workspace'} />
                  </div>
                  {cmd.description ? (
                    <div className="truncate text-ui-sm text-foreground-subtle">{cmd.description}</div>
                  ) : null}
                </div>
                <Switch checked={cmd.enabled} onCheckedChange={() => toggleCustomCommand(cmd.id)} />
                <button
                  type="button"
                  onClick={() => deleteCustomCommand(cmd.id)}
                  className="rounded-md p-1.5 text-foreground-subtlest transition-colors hover:bg-destructive/10 hover:text-destructive"
                >
                  <Trash2 className="size-4" />
                </button>
              </div>
            </React.Fragment>
          ))}
        </SettingsResourceList>
      )}
    </>
  );
};

// ── Hooks ───────────────────────────────────────────────────────────────────

const HooksSection: React.FC<{
  hooks: any[];
  updateHook: (id: string, patch: any) => void;
  toggleHook: (id: string) => void;
  addHook: (hook: any) => void;
  deleteHook: (id: string) => void;
}> = ({ hooks, addHook, deleteHook, toggleHook }) => {
  const [isAdding, setIsAdding] = useState(false);
  const [newEvent, setNewEvent] = useState<'pre_turn' | 'post_turn' | 'post_file_write' | 'pre_commit'>(
    'post_file_write'
  );
  const [newCmd, setNewCmd] = useState('');
  const [newTimeout, setNewTimeout] = useState(10);

  return (
    <>
      <div className="flex items-start justify-between gap-3">
        <p className="min-w-0 flex-1 text-ui-base leading-6 text-foreground-subtle">
          Hooks run shell commands at lifecycle points: before/after turns, file writes, and commits.
        </p>
        <button type="button" className={cn(btn.primary, 'shrink-0')} onClick={() => setIsAdding((prev) => !prev)}>
          <Plus className="size-3.5" />
          New Hook
        </button>
      </div>

      {isAdding ? (
        <SettingsGroupCard className="p-4">
          <form
            className="flex flex-col gap-3"
            onSubmit={(e) => {
              e.preventDefault();
              if (!newCmd.trim()) return;
              addHook({
                id: `hook-${Date.now()}`,
                event: newEvent,
                command: newCmd.trim(),
                timeoutSeconds: newTimeout,
                enabled: true
              });
              setNewCmd('');
              setIsAdding(false);
            }}
          >
            <div className="grid grid-cols-1 gap-3 sm:grid-cols-3">
              <FormField label="Event">
                <SettingsSelect
                  value={newEvent}
                  onChange={(value) => setNewEvent(value as typeof newEvent)}
                  className="w-full"
                  options={[
                    { value: 'pre_turn', label: 'Pre Turn' },
                    { value: 'post_turn', label: 'Post Turn' },
                    { value: 'post_file_write', label: 'Post File Write' },
                    { value: 'pre_commit', label: 'Pre Commit' }
                  ]}
                />
              </FormField>
              <FormField label="Command">
                <input
                  type="text"
                  value={newCmd}
                  onChange={(e) => setNewCmd(e.target.value)}
                  placeholder="e.g. npx biome check --staged"
                  className={cn(btn.input, 'font-mono')}
                  required
                />
              </FormField>
              <FormField label="Timeout (s)">
                <input
                  type="number"
                  min={1}
                  value={newTimeout}
                  onChange={(e) => setNewTimeout(Number(e.target.value))}
                  className={btn.input}
                />
              </FormField>
            </div>
            <div className="flex justify-end gap-2">
              <button type="button" className={btn.outline} onClick={() => setIsAdding(false)}>
                Cancel
              </button>
              <button type="submit" className={btn.primary}>
                Save Hook
              </button>
            </div>
          </form>
        </SettingsGroupCard>
      ) : null}

      {hooks.length === 0 ? (
        <SettingsEmptyState>No hooks configured yet.</SettingsEmptyState>
      ) : (
        <SettingsResourceList>
          {hooks.map((hook, index) => (
            <React.Fragment key={hook.id}>
              {index > 0 && <SettingsResourceSeparator />}
              <div className="flex items-center gap-3 px-4 py-3 transition-colors hover:bg-hover">
                <span className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-surface text-foreground-subtle">
                  <Anchor className="size-4" />
                </span>
                <div className="min-w-0 flex-1">
                  <div className="flex min-w-0 items-center gap-2">
                    <SettingsBadge>{hook.event}</SettingsBadge>
                  </div>
                  <div className="truncate font-mono text-xs text-foreground-subtle">{hook.command}</div>
                </div>
                <Switch checked={hook.enabled} onCheckedChange={() => toggleHook(hook.id)} />
                <button
                  type="button"
                  onClick={() => deleteHook(hook.id)}
                  className="rounded-md p-1.5 text-foreground-subtlest transition-colors hover:bg-destructive/10 hover:text-destructive"
                >
                  <Trash2 className="size-4" />
                </button>
              </div>
            </React.Fragment>
          ))}
        </SettingsResourceList>
      )}
    </>
  );
};

// ── Usage stats ─────────────────────────────────────────────────────────────

const UsageSection: React.FC<{
  providers: LLMProviderConfig[];
  contextUsage: any;
  indexStatus: any;
  fetchIndexStatus: () => void;
  reindexWorkspace: () => Promise<unknown>;
  isReindexing: boolean;
}> = ({ providers, contextUsage, indexStatus }) => {
  const categories = contextUsage?.categories ?? {};
  const legend = [
    { label: 'Messages', color: 'bg-blue-500', pct: categories.messages?.percent || 0 },
    { label: 'System Tools', color: 'bg-amber-500', pct: categories.systemTools?.percent || 0 },
    { label: 'MCP Tools', color: 'bg-cyan-500', pct: categories.mcpTools?.percent || 0 },
    { label: 'Skills', color: 'bg-emerald-500', pct: categories.skills?.percent || 0 },
    { label: 'System Prompt', color: 'bg-purple-500', pct: categories.systemPrompt?.percent || 0 },
    { label: 'Meta Context', color: 'bg-gray-400', pct: categories.metaContext?.percent || 0 }
  ];

  return (
    <>
      <SettingsGroupCard>
        <div className="px-4 py-3">
          <div className="mb-3 flex items-center gap-2">
            <Activity className="size-4 text-foreground-subtle" />
            <span className="text-ui-base font-medium text-foreground">Context window usage</span>
          </div>
          <div className="flex h-3 w-full overflow-hidden rounded-full bg-surface">
            {legend.map((leg) => (
              <div
                key={leg.label}
                style={{ width: `${Math.max(leg.pct, 1)}%` }}
                className={leg.color}
                title={`${leg.label}: ${leg.pct.toFixed(1)}%`}
              />
            ))}
          </div>
          <div className="grid grid-cols-2 gap-3 pt-3 sm:grid-cols-3">
            {legend.map((leg) => (
              <div key={leg.label} className="flex items-center gap-2">
                <span className={cn('size-3 shrink-0 rounded-full', leg.color)} />
                <span className="text-ui-sm text-foreground-subtle">{leg.label}:</span>
                <span className="text-ui-sm font-semibold text-foreground">{leg.pct.toFixed(1)}%</span>
              </div>
            ))}
          </div>
        </div>
      </SettingsGroupCard>

      <SettingsGroupCard>
        <div className="px-4 py-3">
          <div className="mb-2 flex items-center gap-2">
            <Cpu className="size-4 text-foreground-subtle" />
            <span className="text-ui-base font-medium text-foreground">Configured models</span>
          </div>
          <div className="flex flex-col divide-y divide-border">
            {providers
              .filter((p) => p.enabled)
              .map((p) => (
                <div key={p.id} className="flex items-center justify-between py-2.5">
                  <div>
                    <div className="text-ui-sm font-semibold text-foreground">{p.name}</div>
                    <div className="text-xs text-foreground-subtle">
                      {p.selectedModels?.length || p.models.length} active models configured
                    </div>
                  </div>
                  <span className="rounded bg-surface px-2.5 py-1 font-mono text-xs text-foreground-subtle">
                    {p.baseUrl || 'Standard Endpoint'}
                  </span>
                </div>
              ))}
            {providers.filter((p) => p.enabled).length === 0 ? (
              <div className="py-3 text-ui-sm text-foreground-subtle">No providers enabled.</div>
            ) : null}
          </div>
        </div>
      </SettingsGroupCard>

      <SettingsGroupCard>
        <div className="flex items-center gap-3 px-4 py-3">
          <FileText className="size-4 text-foreground-subtle" />
          <div className="min-w-0 flex-1">
            <div className="text-ui-base font-medium text-foreground">Workspace index</div>
            <div className="text-ui-sm text-foreground-subtle">
              {indexStatus?.built ? `${indexStatus.symbols ?? 0} symbols indexed` : 'Not built yet'}
            </div>
          </div>
          <Database className="size-4 text-foreground-subtlest" />
        </div>
      </SettingsGroupCard>
    </>
  );
};

export default SettingsScreen;
