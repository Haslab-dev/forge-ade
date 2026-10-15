import React, { useState, useEffect, useMemo, useCallback } from 'react';
import {
  Bot,
  Plus,
  Trash2,
  RotateCcw,
  Pencil,
  Sparkles,
  RefreshCw,
  X,
  Wrench,
  Server,
  BookOpen,
  Check,
  Search
} from 'lucide-react';
import { cn } from '../../lib/utils';
import { useToast } from '../../lib/toast';
import {
  ListAgentDefinitions,
  SaveAgentDefinition,
  DeleteAgentDefinition,
  ListMCPServers,
  ListSkills
} from '../../lib/wails';
import { AgentDefinition, LLMProviderConfig } from '../../types';
import {
  SettingsRow,
  SettingsSearchInput,
  SettingsResourceList,
  SettingsResourceSeparator,
  SettingsEmptyState,
  SettingsBadge,
  SettingsScopeBadge,
  SettingsSelect,
  ConfirmActionButton,
  Switch,
  settingsButtonClasses as btn
} from './settingsPrimitives';

interface SubagentsSettingsSectionProps {
  providers: LLMProviderConfig[];
}

export const TOOL_CATEGORIES = [
  {
    id: 'filesystem',
    name: 'Filesystem & Search',
    description: 'Read, write, edit files, workspace grep, symbol search'
  },
  {
    id: 'terminal',
    name: 'Terminal & Execution',
    description: 'Execute shell commands, run builds/tests, background processes'
  },
  {
    id: 'git',
    name: 'Git Operations',
    description: 'Git status, diff checking, branch inspection'
  },
  {
    id: 'web',
    name: 'Fast Web & Search',
    description: 'Lightweight HTTP fetch to markdown and search queries'
  },
  {
    id: 'browser',
    name: 'Browser Automation',
    description: 'Full browser session (Playwright) for interactive UI testing'
  }
] as const;

const FormField: React.FC<{
  label: string;
  children: React.ReactNode;
  description?: string;
}> = ({ label, children, description }) => (
  <div className="flex flex-col gap-1.5">
    <div className="flex items-center justify-between">
      <label className="text-ui-base text-foreground-subtle">{label}</label>
      {description ? (
        <span className="text-ui-xs text-foreground-subtlest">{description}</span>
      ) : null}
    </div>
    {children}
  </div>
);

export const SubagentsSettingsSection: React.FC<SubagentsSettingsSectionProps> = ({
  providers
}) => {
  const { toast } = useToast();
  const [definitions, setDefinitions] = useState<AgentDefinition[]>([]);
  const [availableMcpServers, setAvailableMcpServers] = useState<string[]>([]);
  const [availableSkills, setAvailableSkills] = useState<
    Array<{ name: string; description: string }>
  >([]);
  const [skillSearch, setSkillSearch] = useState<string>('');
  const [search, setSearch] = useState<string>('');
  const [isReloading, setIsReloading] = useState<boolean>(false);
  const [isSaving, setIsSaving] = useState<boolean>(false);

  // Modal configuration state
  const [isModalOpen, setIsModalOpen] = useState<boolean>(false);
  const [isCreating, setIsCreating] = useState<boolean>(false);
  const [isCustomModelInput, setIsCustomModelInput] = useState<boolean>(false);

  const [editForm, setEditForm] = useState<AgentDefinition>({
    id: '',
    name: '',
    description: '',
    role_filter: 'custom',
    model: '',
    prompt: '',
    rules: '',
    allow_bash: true,
    read_only: false,
    tools: ['filesystem', 'terminal', 'web'],
    mcp_servers: [],
    skills: []
  });

  const isBuiltin = (id: string) =>
    ['coding', 'planning', 'research', 'custom'].includes(id);

  // Close modal on Escape key
  useEffect(() => {
    if (!isModalOpen) return;
    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        setIsModalOpen(false);
      }
    };
    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [isModalOpen]);

  // Load agent definitions, MCP servers, and skills from backend
  const loadResources = useCallback(async () => {
    try {
      setIsReloading(true);
      const [list, mcpList, skillList] = await Promise.all([
        ListAgentDefinitions().catch(() => []),
        ListMCPServers().catch(() => []),
        ListSkills().catch(() => [])
      ]);
      if (Array.isArray(list)) {
        setDefinitions(list);
      }
      if (Array.isArray(mcpList)) {
        const names = mcpList
          .map((s: any) => s.name || s.id)
          .filter((name: any): name is string => typeof name === 'string' && Boolean(name.trim()));
        setAvailableMcpServers(names);
      }
      if (Array.isArray(skillList)) {
        setAvailableSkills(
          skillList
            .map((s: any) => ({
              name: s.name || '',
              description: s.description || ''
            }))
            .filter((s) => Boolean(s.name))
        );
      }
    } catch (err) {
      console.error('Failed to load sub-agent resources:', err);
    } finally {
      setIsReloading(false);
    }
  }, []);

  useEffect(() => {
    loadResources();
  }, [loadResources]);

  // Flattened list of available models from configured providers
  const flatModels = useMemo(() => {
    const list: Array<{ label: string; value: string }> = [];
    providers.forEach((p) => {
      const pModels =
        p.selectedModels && p.selectedModels.length > 0
          ? p.selectedModels
          : p.models;
      if (Array.isArray(pModels)) {
        pModels.forEach((m) => {
          list.push({
            label: `${m} (${p.name || p.id})`,
            value: m
          });
        });
      }
    });
    return list;
  }, [providers]);

  // Model select options
  const modelSelectOptions = useMemo(() => {
    const opts = [
      { value: '', label: 'Workspace Default Model' },
      ...flatModels.map((m) => ({ value: m.value, label: m.label })),
      { value: '__custom__', label: 'Custom Model ID...' }
    ];
    return opts;
  }, [flatModels]);

  const handleStartCreate = () => {
    setEditForm({
      id: '',
      name: '',
      description: '',
      role_filter: 'custom',
      model: '',
      prompt: '',
      rules: '',
      allow_bash: true,
      read_only: false,
      tools: ['filesystem', 'terminal', 'git', 'web'],
      mcp_servers: [],
      skills: []
    });
    setSkillSearch('');
    setIsCustomModelInput(false);
    setIsCreating(true);
    setIsModalOpen(true);
  };

  const handleStartEdit = (def: AgentDefinition) => {
    setEditForm({
      ...def,
      tools: def.tools ? [...def.tools] : ['filesystem', 'terminal', 'web'],
      mcp_servers: def.mcp_servers ? [...def.mcp_servers] : [],
      skills: def.skills ? [...def.skills] : []
    });
    setSkillSearch('');
    const isCustomModel = Boolean(
      def.model && !flatModels.some((m) => m.value === def.model)
    );
    setIsCustomModelInput(isCustomModel);
    setIsCreating(false);
    setIsModalOpen(true);
  };

  const handleCloseModal = () => {
    setIsModalOpen(false);
  };

  const toggleToolCategory = (id: string) => {
    setEditForm((prev) => {
      const current = prev.tools || [];
      const next = current.includes(id)
        ? current.filter((t) => t !== id)
        : [...current, id];
      return { ...prev, tools: next };
    });
  };

  const toggleMcpServer = (name: string) => {
    setEditForm((prev) => {
      const current = prev.mcp_servers || [];
      const next = current.includes(name)
        ? current.filter((s) => s !== name)
        : [...current, name];
      return { ...prev, mcp_servers: next };
    });
  };

  const selectAllMcpServers = () => {
    setEditForm((prev) => ({
      ...prev,
      mcp_servers: [...availableMcpServers]
    }));
  };

  const clearMcpServers = () => {
    setEditForm((prev) => ({
      ...prev,
      mcp_servers: []
    }));
  };

  const toggleSkill = (name: string) => {
    setEditForm((prev) => {
      const current = prev.skills || [];
      const next = current.includes(name)
        ? current.filter((s) => s !== name)
        : [...current, name];
      return { ...prev, skills: next };
    });
  };

  const selectAllSkills = (skillsToSelect: string[]) => {
    setEditForm((prev) => {
      const set = new Set([...(prev.skills || []), ...skillsToSelect]);
      return { ...prev, skills: Array.from(set) };
    });
  };

  const clearSkills = () => {
    setEditForm((prev) => ({
      ...prev,
      skills: []
    }));
  };

  const filteredAvailableSkills = useMemo(() => {
    if (!skillSearch.trim()) return availableSkills;
    const q = skillSearch.toLowerCase();
    return availableSkills.filter(
      (s) =>
        (s?.name || '').toLowerCase().includes(q) ||
        (s?.description || '').toLowerCase().includes(q)
    );
  }, [availableSkills, skillSearch]);

  const handleSave = async () => {
    if (!editForm.name.trim()) {
      toast('Sub-agent name is required.', 'error');
      return;
    }

    // Auto-generate ID for new custom agent if blank
    let targetDef = { ...editForm };
    if (isCreating && !targetDef.id.trim()) {
      targetDef.id = targetDef.name
        .trim()
        .toLowerCase()
        .replace(/[^a-z0-9_-]/g, '-');
    }

    try {
      setIsSaving(true);
      await SaveAgentDefinition(targetDef);
      toast(`Sub-agent "${targetDef.name}" saved successfully.`, 'success');
      setIsModalOpen(false);
      await loadResources();
    } catch (err) {
      toast(`Failed to save sub-agent: ${String(err)}`, 'error');
    } finally {
      setIsSaving(false);
    }
  };

  const handleDelete = async (id: string, name: string) => {
    try {
      await DeleteAgentDefinition(id);
      toast(
        `Sub-agent "${name}" ${
          isBuiltin(id) ? 'reset to factory default' : 'deleted'
        }.`,
        'success'
      );
      if (editForm.id === id) {
        setIsModalOpen(false);
      }
      await loadResources();
    } catch (err) {
      toast(`Operation failed: ${String(err)}`, 'error');
    }
  };

  const filteredDefinitions = useMemo(() => {
    if (!search.trim()) return definitions;
    const q = search.toLowerCase();
    return definitions.filter(
      (d) =>
        (d?.name || '').toLowerCase().includes(q) ||
        (d?.id || '').toLowerCase().includes(q) ||
        (d?.description && d.description.toLowerCase().includes(q)) ||
        (d?.role_filter && d.role_filter.toLowerCase().includes(q))
    );
  }, [definitions, search]);

  return (
    <div className="space-y-6">
      {/* Top Header Bar */}
      <div className="flex items-start justify-between gap-3">
        <p className="min-w-0 flex-1 text-ui-base leading-6 text-foreground-subtle">
          Configure sub-agents and specialized roles for autonomous delegation.
          When the primary agent delegates work, Jev evaluates intent in real-time
          to dynamically pick the best-matching sub-agent.
        </p>
        <div className="flex shrink-0 items-center gap-2">
          <button
            type="button"
            className={btn.ghost}
            disabled={isReloading}
            onClick={loadResources}
          >
            <RefreshCw
              className={cn('size-3.5', isReloading && 'animate-spin')}
            />
            Refresh
          </button>
          <button
            type="button"
            className={btn.primary}
            onClick={handleStartCreate}
          >
            <Plus className="size-3.5" />
            Create Sub-Agent
          </button>
        </div>
      </div>

      {/* Search Bar */}
      <SettingsSearchInput
        value={search}
        onChange={setSearch}
        placeholder="Filter sub-agents by name, role, or description..."
      />

      {/* Sub-Agents Resource List */}
      {filteredDefinitions.length === 0 ? (
        <SettingsEmptyState>
          No sub-agents found matching your search. Create one above or clear the
          filter.
        </SettingsEmptyState>
      ) : (
        <SettingsResourceList>
          {filteredDefinitions.map((def, index) => {
            const builtin = isBuiltin(def.id);
            return (
              <React.Fragment key={def.id}>
                {index > 0 && <SettingsResourceSeparator />}
                <div className="flex items-start gap-3.5 px-4 py-3.5 transition-colors hover:bg-surface-hover">
                  <span className="flex size-9 shrink-0 items-center justify-center rounded-lg bg-surface text-foreground-subtle mt-0.5">
                    <Bot className="size-4" />
                  </span>

                  <div className="min-w-0 flex-1">
                    <div className="flex min-w-0 flex-wrap items-center gap-2">
                      <span className="text-ui-base font-semibold text-foreground">
                        {def.name}
                      </span>
                      <span className="font-mono text-ui-xs text-foreground-subtle">
                        ({def.id})
                      </span>
                      <SettingsScopeBadge
                        scope={builtin ? 'built-in' : 'custom'}
                      />
                      <SettingsBadge>
                        {def.role_filter || 'custom'}
                      </SettingsBadge>
                      <SettingsBadge>
                        {def.model ? `Model: ${def.model}` : 'Default Model'}
                      </SettingsBadge>
                      {def.read_only ? (
                        <SettingsBadge>Read-Only</SettingsBadge>
                      ) : null}
                      {!def.allow_bash ? (
                        <SettingsBadge>No Bash</SettingsBadge>
                      ) : null}
                    </div>

                    <div className="mt-1 text-ui-sm text-foreground-subtle leading-relaxed">
                      {def.description || 'No description provided.'}
                    </div>

                    {/* Tool category badges, MCP badges, Skill badges */}
                    <div className="mt-2 flex flex-wrap items-center gap-2">
                      {def.tools && def.tools.length > 0 ? (
                        <div className="flex flex-wrap items-center gap-1">
                          <span className="text-ui-xs text-foreground-subtlest font-medium">Tools:</span>
                          {def.tools.map((t) => (
                            <span
                              key={t}
                              className="rounded border border-border/70 bg-surface/80 px-1.5 py-0.5 font-mono text-ui-xs text-foreground-subtle capitalize"
                            >
                              {t}
                            </span>
                          ))}
                        </div>
                      ) : null}

                      {def.mcp_servers && def.mcp_servers.length > 0 ? (
                        <div className="flex flex-wrap items-center gap-1">
                          <span className="text-ui-xs text-foreground-subtlest font-medium">MCP:</span>
                          {def.mcp_servers.map((s) => (
                            <span
                              key={s}
                              className="rounded border border-brand/30 bg-brand/10 px-1.5 py-0.5 font-mono text-ui-xs text-foreground font-medium"
                            >
                              {s}
                            </span>
                          ))}
                        </div>
                      ) : null}

                      {def.skills && def.skills.length > 0 ? (
                        <div className="flex flex-wrap items-center gap-1">
                          <span className="text-ui-xs text-foreground-subtlest font-medium">Skills:</span>
                          <span className="rounded border border-border bg-surface px-1.5 py-0.5 text-ui-xs font-medium text-foreground-subtle">
                            {def.skills.length} assigned
                          </span>
                        </div>
                      ) : null}
                    </div>

                    {def.prompt ? (
                      <div className="mt-1.5 text-ui-xs text-foreground-subtlest font-mono truncate max-w-2xl">
                        Prompt: {def.prompt}
                      </div>
                    ) : null}
                  </div>

                  <div className="flex shrink-0 items-center gap-2">
                    <button
                      type="button"
                      className={btn.outline}
                      onClick={() => handleStartEdit(def)}
                    >
                      <Pencil className="size-3.5" />
                      Configure
                    </button>
                    {!builtin ? (
                      <ConfirmActionButton
                        label=""
                        confirmLabel="Delete?"
                        onConfirm={() => handleDelete(def.id, def.name)}
                        className="rounded-md p-2 text-foreground-subtle transition-colors hover:bg-destructive/10 hover:text-destructive"
                      >
                        <Trash2 className="size-4" />
                      </ConfirmActionButton>
                    ) : null}
                  </div>
                </div>
              </React.Fragment>
            );
          })}
        </SettingsResourceList>
      )}

      {/* Jev Autonomous Delegation Callout */}
      <div className="flex items-start gap-3 rounded-xl border border-border bg-surface/40 p-4">
        <Sparkles className="size-4 text-foreground-subtle shrink-0 mt-0.5" />
        <div className="min-w-0 flex-1 text-ui-sm text-foreground-subtle leading-relaxed">
          <span className="font-medium text-foreground">
            Autonomous Delegation via Jev:{' '}
          </span>
          When the primary agent delegates work using{' '}
          <code className="rounded bg-background px-1 py-0.5 font-mono text-ui-xs text-foreground">
            invoke_subagent
          </code>
          , Jev evaluates the task intent against these sub-agent definitions to
          dynamically route to the most suitable specialist. Any custom
          sub-agent created here is automatically available for autonomous
          matching.
        </div>
      </div>

      {/* ── Configure / Create Sub-Agent Modal ────────────────────────────── */}
      {isModalOpen ? (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-xs p-4 animate-in fade-in duration-150"
          onClick={handleCloseModal}
        >
          <div
            className="w-full max-w-3xl bg-card border border-border rounded-xl shadow-2xl flex flex-col overflow-hidden max-h-[90vh]"
            onClick={(e) => e.stopPropagation()}
          >
            {/* Modal Header */}
            <div className="px-5 py-4 border-b border-border flex items-center justify-between bg-surface/50">
              <div className="flex items-center gap-3">
                <div className="size-8 rounded-lg bg-surface text-foreground-subtle flex items-center justify-center">
                  <Bot className="size-4" />
                </div>
                <div>
                  <h3 className="text-ui-base font-semibold text-foreground">
                    {isCreating
                      ? 'Create New Sub-Agent'
                      : `Configure "${editForm.name}"`}
                  </h3>
                  <p className="text-ui-xs text-foreground-subtle">
                    {isCreating
                      ? 'Define a new domain specialist with custom instructions, tool capabilities, and MCP/skills.'
                      : 'Adjust model assignment, tool capabilities, MCP servers, assigned skills, and operating rules.'}
                  </p>
                </div>
              </div>
              <button
                type="button"
                onClick={handleCloseModal}
                className="p-1 rounded-lg text-foreground-subtle hover:bg-surface-hover hover:text-foreground transition-colors cursor-pointer"
                aria-label="Close dialog"
              >
                <X className="size-4" />
              </button>
            </div>

            {/* Modal Body */}
            <div className="flex-1 overflow-y-auto p-5 space-y-4">
              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <FormField label="Sub-Agent Name">
                  <input
                    type="text"
                    value={editForm.name}
                    onChange={(e) =>
                      setEditForm({ ...editForm, name: e.target.value })
                    }
                    placeholder="e.g. Database Specialist"
                    className={btn.input}
                    required
                  />
                </FormField>

                <FormField
                  label="Role Identifier / ID"
                  description={
                    isBuiltin(editForm.id) && !isCreating
                      ? 'Built-in ID cannot be changed'
                      : 'Unique slug ID'
                  }
                >
                  <input
                    type="text"
                    value={editForm.id}
                    disabled={isBuiltin(editForm.id) && !isCreating}
                    onChange={(e) =>
                      setEditForm({
                        ...editForm,
                        id: e.target.value
                          .toLowerCase()
                          .replace(/[^a-z0-9_-]/g, '-')
                      })
                    }
                    placeholder="e.g. db-specialist"
                    className={cn(
                      btn.input,
                      'font-mono',
                      isBuiltin(editForm.id) &&
                        !isCreating &&
                        'cursor-not-allowed opacity-60'
                    )}
                  />
                </FormField>
              </div>

              <div className="grid grid-cols-1 gap-4 sm:grid-cols-2">
                <FormField label="Base Role Type">
                  <SettingsSelect
                    value={editForm.role_filter || 'custom'}
                    onChange={(val) => {
                      const role = val as
                        | 'coding'
                        | 'planning'
                        | 'research'
                        | 'custom';
                      setEditForm((prev) => ({
                        ...prev,
                        role_filter: role,
                        ...(isCreating
                          ? {
                              read_only:
                                role === 'research' || role === 'planning',
                              allow_bash: role !== 'planning',
                              tools:
                                role === 'coding'
                                  ? ['filesystem', 'terminal', 'git', 'web']
                                  : role === 'planning'
                                  ? ['filesystem', 'git']
                                  : role === 'research'
                                  ? ['filesystem', 'terminal', 'web']
                                  : ['filesystem', 'terminal', 'git', 'web', 'browser']
                            }
                          : {})
                      }));
                    }}
                    className="w-full"
                    options={[
                      {
                        value: 'coding',
                        label: 'Coding (Edits, refactoring, building & testing)'
                      },
                      {
                        value: 'research',
                        label:
                          'Research (Codebase exploration & symbol auditing)'
                      },
                      {
                        value: 'planning',
                        label: 'Planning (Architecture & task decomposition)'
                      },
                      {
                        value: 'custom',
                        label: 'Custom Specialist (Dedicated domain agent)'
                      }
                    ]}
                  />
                </FormField>

                <FormField label="Assigned Model">
                  {isCustomModelInput ? (
                    <div className="flex items-center gap-2">
                      <input
                        type="text"
                        value={editForm.model || ''}
                        onChange={(e) =>
                          setEditForm({ ...editForm, model: e.target.value })
                        }
                        placeholder="e.g. gpt-4o, claude-3-7-sonnet"
                        className={cn(btn.input, 'font-mono')}
                      />
                      <button
                        type="button"
                        className={btn.outline}
                        onClick={() => {
                          setIsCustomModelInput(false);
                          setEditForm({ ...editForm, model: '' });
                        }}
                        title="Select from list"
                      >
                        List
                      </button>
                    </div>
                  ) : (
                    <SettingsSelect
                      value={editForm.model || ''}
                      onChange={(val) => {
                        if (val === '__custom__') {
                          setIsCustomModelInput(true);
                          setEditForm({ ...editForm, model: '' });
                        } else {
                          setEditForm({ ...editForm, model: val });
                        }
                      }}
                      className="w-full"
                      options={modelSelectOptions}
                    />
                  )}
                </FormField>
              </div>

              <FormField
                label="Description & Intent Matching"
                description="Used by Jev to match task instructions during autonomous delegation"
              >
                <input
                  type="text"
                  value={editForm.description || ''}
                  onChange={(e) =>
                    setEditForm({ ...editForm, description: e.target.value })
                  }
                  placeholder="e.g. Inspects SQL queries, explains database execution plans, and tunes schema indexes."
                  className={btn.input}
                />
              </FormField>

              {/* Permissions Toggles */}
              <div className="overflow-hidden rounded-xl border border-border bg-surface/30">
                <SettingsRow
                  label="Allow Bash / Terminal Execution"
                  description="Permits running shell commands, builds, git queries, and test suites."
                  control={
                    <Switch
                      checked={Boolean(editForm.allow_bash)}
                      onCheckedChange={(checked) =>
                        setEditForm((prev) => ({
                          ...prev,
                          allow_bash: checked
                        }))
                      }
                    />
                  }
                />
                <SettingsRow
                  label="Read-Only Mode"
                  description="Restricts sub-agent from making file edits or mutating files in the workspace."
                  control={
                    <Switch
                      checked={Boolean(editForm.read_only)}
                      onCheckedChange={(checked) =>
                        setEditForm((prev) => ({
                          ...prev,
                          read_only: checked
                        }))
                      }
                    />
                  }
                />
              </div>

              {/* ── Tool Capabilities & Access ────────────────────── */}
              <div className="overflow-hidden rounded-xl border border-border bg-surface/30">
                <div className="px-4 py-3 border-b border-border bg-surface/50 flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <Wrench className="size-4 text-foreground-subtle" />
                    <span className="text-ui-base font-semibold text-foreground">
                      Tool Capabilities &amp; Access
                    </span>
                  </div>
                  <span className="text-ui-xs text-foreground-subtlest">
                    {(editForm.tools || []).length} of {TOOL_CATEGORIES.length} active
                  </span>
                </div>
                <div className="divide-y divide-border">
                  {TOOL_CATEGORIES.map((cat) => {
                    const isChecked = (editForm.tools || []).includes(cat.id);
                    return (
                      <div
                        key={cat.id}
                        className="px-4 py-3 flex items-center justify-between gap-4 transition-colors hover:bg-surface/50"
                      >
                        <div className="min-w-0 flex-1">
                          <div className="text-ui-base font-medium text-foreground">
                            {cat.name}
                          </div>
                          <div className="text-ui-xs text-foreground-subtle">
                            {cat.description}
                          </div>
                        </div>
                        <Switch
                          checked={isChecked}
                          onCheckedChange={() => toggleToolCategory(cat.id)}
                        />
                      </div>
                    );
                  })}
                </div>
              </div>

              {/* ── MCP Servers Integration ───────────────────────── */}
              <div className="overflow-hidden rounded-xl border border-border bg-surface/30">
                <div className="px-4 py-3 border-b border-border bg-surface/50 flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <Server className="size-4 text-foreground-subtle" />
                    <span className="text-ui-base font-semibold text-foreground">
                      MCP Servers Integration
                    </span>
                  </div>
                  {availableMcpServers.length > 0 ? (
                    <div className="flex items-center gap-2">
                      <button
                        type="button"
                        className="text-ui-xs text-foreground-subtle hover:text-foreground cursor-pointer"
                        onClick={selectAllMcpServers}
                      >
                        Select All
                      </button>
                      <span className="text-foreground-subtlest">•</span>
                      <button
                        type="button"
                        className="text-ui-xs text-foreground-subtle hover:text-foreground cursor-pointer"
                        onClick={clearMcpServers}
                      >
                        Clear
                      </button>
                    </div>
                  ) : null}
                </div>
                <div className="p-4">
                  {availableMcpServers.length === 0 ? (
                    <p className="text-ui-xs text-foreground-subtle">
                      No MCP servers currently configured. You can configure and connect MCP servers in{' '}
                      <span className="font-medium text-foreground">Settings &gt; MCP</span>.
                    </p>
                  ) : (
                    <div className="flex flex-wrap gap-2">
                      {availableMcpServers.map((name) => {
                        const isSelected = (editForm.mcp_servers || []).includes(name);
                        return (
                          <button
                            key={name}
                            type="button"
                            onClick={() => toggleMcpServer(name)}
                            className={cn(
                              'inline-flex items-center gap-1.5 rounded-lg border px-3 py-1.5 text-ui-sm font-medium transition-colors cursor-pointer',
                              isSelected
                                ? 'border-brand bg-brand/10 text-foreground font-semibold shadow-xs'
                                : 'border-border bg-surface text-foreground-subtle hover:bg-surface-hover hover:text-foreground'
                            )}
                          >
                            <div
                              className={cn(
                                'size-3.5 rounded-xs border flex items-center justify-center transition-colors',
                                isSelected
                                  ? 'border-brand bg-brand text-foreground-inverse'
                                  : 'border-border bg-background'
                              )}
                            >
                              {isSelected && <Check className="size-2.5 stroke-[3]" />}
                            </div>
                            <span>{name}</span>
                          </button>
                        );
                      })}
                    </div>
                  )}
                </div>
              </div>

              {/* ── Assigned Skills ───────────────────────────────── */}
              <div className="overflow-hidden rounded-xl border border-border bg-surface/30">
                <div className="px-4 py-3 border-b border-border bg-surface/50 flex items-center justify-between">
                  <div className="flex items-center gap-2">
                    <BookOpen className="size-4 text-foreground-subtle" />
                    <span className="text-ui-base font-semibold text-foreground">
                      Assigned Skills (SKILL.md)
                    </span>
                  </div>
                  <div className="flex items-center gap-2">
                    <span className="text-ui-xs text-foreground-subtlest">
                      {(editForm.skills || []).length} assigned
                    </span>
                    {filteredAvailableSkills.length > 0 ? (
                      <>
                        <span className="text-foreground-subtlest">•</span>
                        <button
                          type="button"
                          className="text-ui-xs text-foreground-subtle hover:text-foreground cursor-pointer"
                          onClick={() =>
                            selectAllSkills(filteredAvailableSkills.map((s) => s.name))
                          }
                        >
                          Select All
                        </button>
                        <span className="text-foreground-subtlest">•</span>
                        <button
                          type="button"
                          className="text-ui-xs text-foreground-subtle hover:text-foreground cursor-pointer"
                          onClick={clearSkills}
                        >
                          Clear
                        </button>
                      </>
                    ) : null}
                  </div>
                </div>
                <div className="p-4 space-y-3">
                  {availableSkills.length > 5 ? (
                    <div className="relative">
                      <Search className="pointer-events-none absolute left-3 top-1/2 size-3.5 -translate-y-1/2 text-foreground-subtle" />
                      <input
                        type="text"
                        value={skillSearch}
                        onChange={(e) => setSkillSearch(e.target.value)}
                        placeholder="Search skills by name or keyword..."
                        className={cn(btn.input, 'pl-8 text-ui-xs h-8')}
                      />
                    </div>
                  ) : null}

                  {availableSkills.length === 0 ? (
                    <p className="text-ui-xs text-foreground-subtle">
                      No skills found in <code className="font-mono text-foreground">~/.agents/skills</code> or{' '}
                      <code className="font-mono text-foreground">~/.forge-ade/skills</code>. Add SKILL.md bundles to activate them.
                    </p>
                  ) : filteredAvailableSkills.length === 0 ? (
                    <p className="text-ui-xs text-foreground-subtle">
                      No skills match &quot;{skillSearch}&quot;.
                    </p>
                  ) : (
                    <div className="flex flex-wrap gap-2 max-h-48 overflow-y-auto pr-1">
                      {filteredAvailableSkills.map((sk) => {
                        const isSelected = (editForm.skills || []).includes(sk.name);
                        return (
                          <button
                            key={sk.name}
                            type="button"
                            title={sk.description || sk.name}
                            onClick={() => toggleSkill(sk.name)}
                            className={cn(
                              'inline-flex items-center gap-1.5 rounded-lg border px-2.5 py-1 text-ui-xs font-mono transition-colors cursor-pointer',
                              isSelected
                                ? 'border-brand bg-brand/10 text-foreground font-semibold shadow-xs'
                                : 'border-border bg-surface text-foreground-subtle hover:bg-surface-hover hover:text-foreground'
                            )}
                          >
                            <div
                              className={cn(
                                'size-3 rounded-xs border flex items-center justify-center transition-colors',
                                isSelected
                                  ? 'border-brand bg-brand text-foreground-inverse'
                                  : 'border-border bg-background'
                              )}
                            >
                              {isSelected && <Check className="size-2 stroke-[3]" />}
                            </div>
                            <span>{sk.name}</span>
                          </button>
                        );
                      })}
                    </div>
                  )}
                </div>
              </div>

              <FormField
                label="Specialized Instructions / System Prompt"
                description="Injected into the sub-agent prompt context"
              >
                <textarea
                  rows={4}
                  value={editForm.prompt || ''}
                  onChange={(e) =>
                    setEditForm({ ...editForm, prompt: e.target.value })
                  }
                  placeholder="e.g. You are a specialized database tuning engineer. Analyze query plans, verify index usage, and never execute destructive schema commands..."
                  className={cn(btn.input, 'font-mono text-ui-xs resize-y')}
                />
              </FormField>

              <FormField
                label="Operating Rules & Constraints"
                description="Hard operational guardrails"
              >
                <textarea
                  rows={3}
                  value={editForm.rules || ''}
                  onChange={(e) =>
                    setEditForm({ ...editForm, rules: e.target.value })
                  }
                  placeholder="e.g. Never run destructive DROP TABLE or TRUNCATE commands. Always report execution cost metrics."
                  className={cn(btn.input, 'font-mono text-ui-xs resize-y')}
                />
              </FormField>
            </div>

            {/* Modal Footer */}
            <div className="px-5 py-3.5 border-t border-border bg-surface/50 flex items-center justify-between">
              <div>
                {!isCreating && !isBuiltin(editForm.id) ? (
                  <ConfirmActionButton
                    label="Delete Sub-Agent"
                    confirmLabel="Confirm Delete"
                    className={cn(
                      btn.ghost,
                      'text-destructive hover:bg-destructive/10 hover:text-destructive'
                    )}
                    onConfirm={() => handleDelete(editForm.id, editForm.name)}
                  />
                ) : !isCreating && isBuiltin(editForm.id) ? (
                  <button
                    type="button"
                    className={btn.ghost}
                    onClick={() => handleDelete(editForm.id, editForm.name)}
                  >
                    <RotateCcw className="size-3.5" />
                    Reset to Default
                  </button>
                ) : null}
              </div>
              <div className="flex items-center gap-2">
                <button
                  type="button"
                  className={btn.outline}
                  onClick={handleCloseModal}
                >
                  Cancel
                </button>
                <button
                  type="button"
                  className={btn.primary}
                  disabled={isSaving}
                  onClick={handleSave}
                >
                  {isSaving
                    ? 'Saving...'
                    : isCreating
                    ? 'Create Sub-Agent'
                    : 'Save Changes'}
                </button>
              </div>
            </div>
          </div>
        </div>
      ) : null}
    </div>
  );
};

export default SubagentsSettingsSection;
