import React, { useState, useEffect, useMemo, useCallback } from 'react';
import {
  Bot,
  Sparkles,
  Plus,
  Trash2,
  RotateCcw,
  Check,
  Terminal,
  Shield,
  BookOpen,
  Cpu,
  HelpCircle,
  FileText
} from 'lucide-react';
import { cn } from '../../lib/utils';
import { useToast } from '../../lib/toast';
import {
  ListAgentDefinitions,
  SaveAgentDefinition,
  DeleteAgentDefinition
} from '../../lib/wails';
import { AgentDefinition, LLMProviderConfig } from '../../types';
import {
  SectionTitle,
  SettingsGroupCard,
  SettingsRow,
  Switch,
  settingsButtonClasses as btn
} from './settingsPrimitives';

interface SubagentsSettingsSectionProps {
  providers: LLMProviderConfig[];
}

export const SubagentsSettingsSection: React.FC<SubagentsSettingsSectionProps> = ({ providers }) => {
  const { toast } = useToast();
  const [definitions, setDefinitions] = useState<AgentDefinition[]>([]);
  const [selectedId, setSelectedId] = useState<string>('research');
  const [isLoading, setIsLoading] = useState<boolean>(true);
  const [isSaving, setIsSaving] = useState<boolean>(false);
  const [isCreating, setIsCreating] = useState<boolean>(false);

  // Active definition editing state
  const [currentDef, setCurrentDef] = useState<AgentDefinition | null>(null);
  const [isCustomModelInput, setIsCustomModelInput] = useState<boolean>(false);

  // Load agent definitions from Go backend
  const loadDefinitions = useCallback(async () => {
    try {
      setIsLoading(true);
      const list = await ListAgentDefinitions();
      if (Array.isArray(list)) {
        setDefinitions(list);
        // Ensure selected definition exists or default to first
        if (list.length > 0) {
          const match = list.find((d) => d.id === selectedId) || list[0];
          setSelectedId(match.id);
          setCurrentDef({ ...match });
        }
      }
    } catch (err) {
      console.error('Failed to load agent definitions:', err);
    } finally {
      setIsLoading(false);
    }
  }, [selectedId]);

  useEffect(() => {
    loadDefinitions();
  }, [loadDefinitions]);

  // When selection changes, update currentDef
  const handleSelect = (def: AgentDefinition) => {
    setSelectedId(def.id);
    setCurrentDef({ ...def });
    setIsCreating(false);
    // Check if model is not in standard provider list
    if (def.model && !flatModels.some((m) => m.value === def.model)) {
      setIsCustomModelInput(true);
    } else {
      setIsCustomModelInput(false);
    }
  };

  // Build flattened list of available models from configured providers
  const flatModels = useMemo(() => {
    const list: Array<{ label: string; value: string; providerName: string }> = [];
    providers.forEach((p) => {
      const pModels = (p.selectedModels && p.selectedModels.length > 0) ? p.selectedModels : p.models;
      if (Array.isArray(pModels)) {
        pModels.forEach((m) => {
          list.push({
            label: m,
            value: m,
            providerName: p.name || p.id
          });
        });
      }
    });
    return list;
  }, [providers]);

  // Group models by provider
  const modelsByProvider = useMemo(() => {
    const map = new Map<string, Array<{ label: string; value: string }>>();
    flatModels.forEach((item) => {
      const existing = map.get(item.providerName) || [];
      existing.push({ label: item.label, value: item.value });
      map.set(item.providerName, existing);
    });
    return map;
  }, [flatModels]);

  const handleSave = async () => {
    if (!currentDef || !currentDef.name.trim()) {
      toast('Agent role name is required.', 'error');
      return;
    }

    try {
      setIsSaving(true);
      const saved = await SaveAgentDefinition(currentDef);
      toast(`Sub-agent role "${currentDef.name}" has been updated.`, 'success');
      setIsCreating(false);
      setSelectedId(saved.id || currentDef.id);
      await loadDefinitions();
    } catch (err) {
      toast(`Error saving role: ${String(err)}`, 'error');
    } finally {
      setIsSaving(false);
    }
  };

  const handleDelete = async (id: string, name: string) => {
    try {
      await DeleteAgentDefinition(id);
      toast(`Role "${name}" removed or reset to built-in default.`, 'success');
      setSelectedId('research');
      await loadDefinitions();
    } catch (err) {
      toast(`Delete failed: ${String(err)}`, 'error');
    }
  };

  const handleStartCreate = (rolePreset: 'research' | 'planning' | 'coding' | 'custom' = 'research') => {
    const newRole: AgentDefinition = {
      id: '',
      name: rolePreset === 'research' ? 'Deep Researcher' : rolePreset === 'planning' ? 'Strategy Planner' : 'Custom Subagent',
      description: rolePreset === 'research' ? 'Autonomous codebase analysis and literature research.' : 'Specialized child agent.',
      role_filter: rolePreset,
      model: '',
      prompt: '',
      rules: '',
      allow_bash: rolePreset !== 'planning',
      read_only: rolePreset === 'research' || rolePreset === 'planning'
    };
    setCurrentDef(newRole);
    setSelectedId('');
    setIsCreating(true);
    setIsCustomModelInput(false);
  };

  const isBuiltin = (id: string) => ['coding', 'planning', 'research', 'custom'].includes(id);

  return (
    <div className="space-y-6">
      <div className="flex flex-wrap items-center justify-between gap-3">
        <SectionTitle
          title="Sub-Agents & Roles"
          subtitle="Configure sub-agent roles, assign dedicated models per role (e.g. Research → gpt-6-luna, Planning → deepseek-v4-flash), customize system prompts, and grant bash or tool execution permissions."
        />
        <button
          type="button"
          onClick={() => handleStartCreate('research')}
          className={btn.primary}
        >
          <Plus className="size-4" />
          <span>New Sub-Agent Role</span>
        </button>
      </div>

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-12">
        {/* Left column: Role Selector Cards */}
        <div className="space-y-3 lg:col-span-4">
          <div className="text-ui-xs font-semibold uppercase tracking-wider text-foreground-subtle">
            Configured Roles ({definitions.length})
          </div>

          <div className="space-y-2">
            {definitions.map((def) => {
              const isSelected = selectedId === def.id && !isCreating;
              return (
                <div
                  key={def.id}
                  onClick={() => handleSelect(def)}
                  className={cn(
                    'group relative flex cursor-pointer flex-col gap-1 rounded-xl border p-3.5 transition-all',
                    isSelected
                      ? 'border-brand bg-brand/5 shadow-xs'
                      : 'border-border bg-card hover:border-brand/40 hover:bg-surface-hover'
                  )}
                >
                  <div className="flex items-center justify-between gap-2">
                    <div className="flex items-center gap-2">
                      <div
                        className={cn(
                          'flex size-7 items-center justify-center rounded-lg',
                          def.role_filter === 'research'
                            ? 'bg-primary/10 text-primary'
                            : def.role_filter === 'planning'
                            ? 'bg-amber-500/10 text-amber-500'
                            : 'bg-emerald-500/10 text-emerald-500'
                        )}
                      >
                        {def.role_filter === 'research' ? (
                          <BookOpen className="size-4" />
                        ) : def.role_filter === 'planning' ? (
                          <Shield className="size-4" />
                        ) : (
                          <Bot className="size-4" />
                        )}
                      </div>
                      <div className="text-ui-base font-medium text-foreground">{def.name}</div>
                    </div>

                    <span className="rounded-md bg-surface px-2 py-0.5 text-ui-xs font-medium text-foreground-subtle uppercase">
                      {def.role_filter || 'coding'}
                    </span>
                  </div>

                  {def.description && (
                    <div className="line-clamp-2 text-ui-xs text-foreground-subtle">
                      {def.description}
                    </div>
                  )}

                  <div className="mt-2 flex flex-wrap items-center gap-1.5 pt-1 border-t border-border/50 text-ui-xs">
                    {def.model ? (
                      <span className="inline-flex items-center gap-1 rounded bg-surface px-1.5 py-0.5 text-foreground-subtle font-mono">
                        <Cpu className="size-3 text-brand" />
                        {def.model}
                      </span>
                    ) : (
                      <span className="text-foreground-subtlest">Default model</span>
                    )}

                    {def.allow_bash ? (
                      <span className="rounded bg-success/10 px-1.5 py-0.5 text-success font-medium">
                        Bash: ON
                      </span>
                    ) : (
                      <span className="rounded bg-surface px-1.5 py-0.5 text-foreground-subtlest">
                        Bash: OFF
                      </span>
                    )}

                    {def.read_only && (
                      <span className="rounded bg-amber-500/10 px-1.5 py-0.5 text-amber-500 font-medium">
                        Read-only
                      </span>
                    )}
                  </div>
                </div>
              );
            })}
          </div>
        </div>

        {/* Right column: Role Editor */}
        <div className="lg:col-span-8">
          {currentDef ? (
            <div className="space-y-5 rounded-2xl border border-border bg-card p-5 shadow-xs">
              {/* Header */}
              <div className="flex flex-wrap items-center justify-between gap-3 border-b border-border pb-4">
                <div className="flex items-center gap-3">
                  <div className="flex size-9 items-center justify-center rounded-xl bg-brand/10 text-brand">
                    <Sparkles className="size-5" />
                  </div>
                  <div>
                    <h3 className="text-ui-lg font-semibold text-foreground">
                      {isCreating ? 'Create New Sub-Agent Role' : `Configure "${currentDef.name}"`}
                    </h3>
                    <p className="text-ui-xs text-foreground-subtle">
                      {isBuiltin(currentDef.id)
                        ? 'Built-in system role. You can customize model, prompt, and tool permissions.'
                        : 'Custom user-defined sub-agent role.'}
                    </p>
                  </div>
                </div>

                <div className="flex items-center gap-2">
                  {!isCreating && (
                    <button
                      type="button"
                      onClick={() => handleDelete(currentDef.id, currentDef.name)}
                      className={cn(btn.outline, 'text-destructive hover:bg-destructive/10 hover:border-destructive/30')}
                      title={isBuiltin(currentDef.id) ? 'Reset to factory defaults' : 'Delete this role'}
                    >
                      {isBuiltin(currentDef.id) ? (
                        <>
                          <RotateCcw className="size-3.5" />
                          <span>Reset</span>
                        </>
                      ) : (
                        <>
                          <Trash2 className="size-3.5" />
                          <span>Delete</span>
                        </>
                      )}
                    </button>
                  )}

                  <button
                    type="button"
                    onClick={handleSave}
                    disabled={isSaving}
                    className={btn.primary}
                  >
                    <Check className="size-4" />
                    <span>{isSaving ? 'Saving...' : 'Save Role'}</span>
                  </button>
                </div>
              </div>

              {/* General Info */}
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <div className="space-y-1.5">
                  <label className="text-ui-sm font-medium text-foreground">Role Name</label>
                  <input
                    type="text"
                    value={currentDef.name}
                    onChange={(e) => setCurrentDef({ ...currentDef, name: e.target.value })}
                    placeholder="e.g. Research Agent"
                    className={btn.input}
                  />
                </div>

                <div className="space-y-1.5">
                  <label className="text-ui-sm font-medium text-foreground">Base Role Type</label>
                  <select
                    value={currentDef.role_filter || 'custom'}
                    onChange={(e) => setCurrentDef({ ...currentDef, role_filter: e.target.value })}
                    className={cn(btn.input, 'cursor-pointer')}
                  >
                    <option value="research">Research (analysis, codebase exploration)</option>
                    <option value="planning">Planning (architecture breakdown, plan synthesis)</option>
                    <option value="coding">Coding (implementation, refactoring, tests)</option>
                    <option value="custom">Custom (general purpose)</option>
                  </select>
                </div>

                <div className="space-y-1.5 sm:col-span-2">
                  <label className="text-ui-sm font-medium text-foreground">Description</label>
                  <input
                    type="text"
                    value={currentDef.description || ''}
                    onChange={(e) => setCurrentDef({ ...currentDef, description: e.target.value })}
                    placeholder="Brief description of this agent's specialization..."
                    className={btn.input}
                  />
                </div>
              </div>

              {/* Model Selection */}
              <div className="space-y-2 rounded-xl border border-border bg-surface/50 p-4">
                <div className="flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <Cpu className="size-4 text-brand" />
                    <span className="text-ui-sm font-semibold text-foreground">Role Model</span>
                  </div>
                  <button
                    type="button"
                    onClick={() => setIsCustomModelInput(!isCustomModelInput)}
                    className="text-ui-xs text-brand hover:underline cursor-pointer"
                  >
                    {isCustomModelInput ? 'Select from active providers' : 'Enter custom model ID'}
                  </button>
                </div>
                <p className="text-ui-xs text-foreground-subtle">
                  Sub-agents spawned with this role will invoke this specific model (e.g. <code>gpt-6-luna</code>, <code>deepseek-v4-flash</code>, <code>claude-3-7-sonnet</code>).
                </p>

                {isCustomModelInput ? (
                  <div className="mt-2">
                    <input
                      type="text"
                      value={currentDef.model || ''}
                      onChange={(e) => setCurrentDef({ ...currentDef, model: e.target.value })}
                      placeholder="e.g. gpt-6-luna or openrouter/deepseek/deepseek-chat"
                      className={cn(btn.input, 'font-mono')}
                    />
                    <div className="mt-1 text-ui-xs text-foreground-subtle">
                      Tip: You can specify bare model names or provider-prefixed IDs (e.g. <code>provider/model</code>).
                    </div>
                  </div>
                ) : (
                  <div className="mt-2">
                    <select
                      value={currentDef.model || ''}
                      onChange={(e) => setCurrentDef({ ...currentDef, model: e.target.value })}
                      className={cn(btn.input, 'cursor-pointer font-mono')}
                    >
                      <option value="">(Inherit from Session / Default Model)</option>
                      {Array.from(modelsByProvider.entries()).map(([provName, list]) => (
                        <optgroup key={provName} label={`Provider: ${provName}`}>
                          {list.map((m) => (
                            <option key={m.value} value={m.value}>
                              {m.label}
                            </option>
                          ))}
                        </optgroup>
                      ))}
                    </select>
                  </div>
                )}
              </div>

              {/* Tool Execution & Permissions */}
              <div className="space-y-3">
                <div className="text-ui-sm font-semibold text-foreground">
                  Tool Permissions & Safety Controls
                </div>

                <SettingsGroupCard>
                  <SettingsRow
                    label={
                      <div className="flex items-center gap-2">
                        <Terminal className="size-4 text-primary" />
                        <span>Allow Bash / Terminal Execution</span>
                      </div>
                    }
                    description="Permit this sub-agent to run shell commands, grep/find tools, git log, and verification tests. Enable this to unblock sub-agents that need bash access."
                    control={
                      <Switch
                        checked={Boolean(currentDef.allow_bash)}
                        onCheckedChange={(checked) => setCurrentDef({ ...currentDef, allow_bash: checked })}
                      />
                    }
                  />

                  <SettingsRow
                    label={
                      <div className="flex items-center gap-2">
                        <Shield className="size-4 text-amber-500" />
                        <span>Read-Only Mode</span>
                      </div>
                    }
                    description="Disallow file mutation tools (write, edit). Safe for read-only research, exploration, and architecture inspection."
                    control={
                      <Switch
                        checked={Boolean(currentDef.read_only)}
                        onCheckedChange={(checked) => setCurrentDef({ ...currentDef, read_only: checked })}
                      />
                    }
                  />
                </SettingsGroupCard>
              </div>

              {/* System Prompt / Additional Instructions */}
              <div className="space-y-1.5">
                <div className="flex items-center justify-between">
                  <label className="text-ui-sm font-medium text-foreground flex items-center gap-1.5">
                    <FileText className="size-4 text-brand" />
                    <span>System Prompt / Additional Instructions</span>
                  </label>
                  <span className="text-ui-xs text-foreground-subtle">Injected into system context</span>
                </div>
                <textarea
                  rows={4}
                  value={currentDef.prompt || ''}
                  onChange={(e) => setCurrentDef({ ...currentDef, prompt: e.target.value })}
                  placeholder="e.g. You are a research agent specialized in deep code audits. Trace symbols, analyze call-chains, and synthesize clear architectural summaries..."
                  className={cn(btn.input, 'font-mono text-ui-xs resize-y')}
                />
              </div>

              {/* Custom Rules */}
              <div className="space-y-1.5">
                <div className="flex items-center justify-between">
                  <label className="text-ui-sm font-medium text-foreground flex items-center gap-1.5">
                    <BookOpen className="size-4 text-brand" />
                    <span>Rules (always follow)</span>
                  </label>
                  <span className="text-ui-xs text-foreground-subtle">Hard constraints</span>
                </div>
                <textarea
                  rows={3}
                  value={currentDef.rules || ''}
                  onChange={(e) => setCurrentDef({ ...currentDef, rules: e.target.value })}
                  placeholder="e.g. Never execute git commit or push. Always provide file paths and line numbers when referencing code..."
                  className={cn(btn.input, 'font-mono text-ui-xs resize-y')}
                />
              </div>

              {/* Footer Save Button */}
              <div className="flex items-center justify-end gap-2 pt-3 border-t border-border">
                <button
                  type="button"
                  onClick={handleSave}
                  disabled={isSaving}
                  className={btn.primary}
                >
                  <Check className="size-4" />
                  <span>{isSaving ? 'Saving...' : 'Save Role Configuration'}</span>
                </button>
              </div>
            </div>
          ) : (
            <div className="flex h-64 flex-col items-center justify-center rounded-2xl border border-dashed border-border p-8 text-center">
              <Bot className="size-10 text-foreground-subtlest" />
              <div className="mt-2 text-ui-base font-medium text-foreground">No Role Selected</div>
              <p className="mt-1 text-ui-xs text-foreground-subtle">
                Select a sub-agent role from the left column or create a new one.
              </p>
            </div>
          )}
        </div>
      </div>
    </div>
  );
};
