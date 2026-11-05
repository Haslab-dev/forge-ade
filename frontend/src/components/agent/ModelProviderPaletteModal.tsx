import React, { useState, useMemo, useEffect, useLayoutEffect, useRef } from 'react';
import { createPortal } from 'react-dom';
import {
  Search,
  Check,
  Plus,
  X,
  RefreshCw,
  Settings2,
  KeyRound
} from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';
import { useProviderStore } from '../../stores/providerStore';
import { ProviderIcon } from './ProviderIcon';
import { parseModelMetadata, type ModelMeta } from '../../features/agent/modelMetadata';
import { SetActiveModel, SetActiveReasoningEffort } from '../../lib/wails';
import { cn } from '../../lib/utils';
import { showToast } from '../../lib/toast';

import type { AgentReasoningLevel } from '../../types';

interface ModelProviderPaletteModalProps {
  isOpen: boolean;
  onClose: () => void;
  anchorRef?: React.RefObject<HTMLElement | null>;
}

interface HarnessGroup {
  id: string;
  name: string;
  icon: string;
  type: 'internal' | 'acp' | 'provider';
  description?: string;
  isCurrent: boolean;
  models: ModelMeta[];
}

const EFFORT_OPTIONS: { id: AgentReasoningLevel; label: string; desc: string }[] = [
  { id: 'off', label: 'Off', desc: 'Thinking disabled (standard non-reasoning)' },
  { id: 'low', label: 'Low', desc: 'Fast responses with minimal thinking budget' },
  { id: 'medium', label: 'Medium', desc: 'Balanced reasoning for general code tasks' },
  { id: 'high', label: 'High', desc: 'Deep multi-step reasoning with maximum thinking tokens' },
];

export const ModelProviderPaletteModal: React.FC<ModelProviderPaletteModalProps> = ({
  isOpen,
  onClose,
  anchorRef
}) => {
  const {
    currentModel,
    setCurrentModel,
    providers,
    agents,
    activeAgentId,
    setActiveAgentId,
    reasoningLevel,
    setReasoningLevel,
    acpEnabled,
    checkAgentsAvailability,
    openSettingsTab,
    setMode,
    setSettingsActiveSection
  } = useWorkspace();

  const localServices = useProviderStore(s => s.localServices);
  const isScanningLocal = useProviderStore(s => s.isScanningLocal);
  const discoverLocalProviders = useProviderStore(s => s.discoverLocalProviders);
  const [isScanningAgents, setIsScanningAgents] = useState(false);

  const handleScanAll = async () => {
    setIsScanningAgents(true);
    try {
      await Promise.all([
        checkAgentsAvailability ? checkAgentsAvailability() : Promise.resolve(),
        discoverLocalProviders()
      ]);
      showToast('Scanned installed ACP agents & local LLMs', 'info');
    } finally {
      setIsScanningAgents(false);
    }
  };

  const localOnlineCount = useMemo(() => {
    return (localServices || []).filter(s => s.running).length;
  }, [localServices]);

  const [search, setSearch] = useState('');
  const [selectedHarnessId, setSelectedHarnessId] = useState<string>('');
  const [selectedProviderFilter, setSelectedProviderFilter] = useState<string>('all');
  const [highlightedIdx, setHighlightedIdx] = useState<number>(0);
  const inputRef = useRef<HTMLInputElement>(null);
  const popoverRef = useRef<HTMLDivElement>(null);

  const [coords, setCoords] = useState<{ top?: number; bottom?: number; left: number; width: number; maxHeight: number }>({
    left: 20,
    width: 540,
    maxHeight: 420
  });

  // Calculate anchored popover coordinates with dynamic viewport-safe maxHeight
  useLayoutEffect(() => {
    if (!isOpen) return;
    if (anchorRef?.current) {
      const rect = anchorRef.current.getBoundingClientRect();
      const composerSurface = anchorRef.current.closest('.chat-composer-surface') || anchorRef.current.closest('[data-file-drop-target="true"]');
      const composerRect = composerSurface?.getBoundingClientRect();

      const maxAvailableWidth = composerRect ? Math.max(320, composerRect.width) : (window.innerWidth - 24);
      const width = Math.min(540, window.innerWidth - 24, maxAvailableWidth);

      const minLeft = composerRect ? Math.max(12, composerRect.left) : 12;
      const maxRight = composerRect ? Math.min(window.innerWidth - 12, composerRect.right) : (window.innerWidth - 12);
      const idealLeft = Math.max(minLeft, Math.min(rect.left, maxRight - width));
      const left = Math.max(12, Math.min(idealLeft, window.innerWidth - width - 12));
      const gap = 8;
      const idealHeight = 420;

      const roomAbove = rect.top - 12;
      const roomBelow = window.innerHeight - rect.bottom - 12;
      if (roomAbove >= idealHeight || roomAbove >= roomBelow) {
        setCoords({
          bottom: window.innerHeight - rect.top + gap,
          left,
          width,
          maxHeight: Math.min(idealHeight, Math.max(240, roomAbove - gap))
        });
      } else {
        setCoords({
          top: rect.bottom + gap,
          left,
          width,
          maxHeight: Math.min(idealHeight, Math.max(240, roomBelow - gap))
        });
      }
    } else {
      const width = Math.min(540, window.innerWidth - 24);
      const maxHeight = Math.min(420, Math.max(240, window.innerHeight - 40));
      setCoords({
        top: Math.max(20, (window.innerHeight - maxHeight) / 2),
        left: Math.max(12, (window.innerWidth - width) / 2),
        width,
        maxHeight
      });
    }
  }, [isOpen, anchorRef]);

  // Build Harnesses & Models hierarchy matching PandaCode
  const harnessGroups = useMemo<HarnessGroup[]>(() => {
    const list: HarnessGroup[] = [];

    // 1. Internal Agent Engine
    const internalAgent = (agents || []).find(a => a.type === 'internal') || {
      id: 'agent-internal',
      name: 'ForgeADE Internal',
      type: 'internal',
      icon: 'forge-ade',
      description: 'Internal autonomous agent engine'
    };

    const internalModels: ModelMeta[] = [];
    (providers || []).forEach(p => {
      if (!p.enabled) return;
      const models = (Array.isArray(p.selectedModels) ? p.selectedModels : p.models) || [];
      models.forEach(m => {
        internalModels.push(parseModelMetadata(m, p.name, 'agent-internal'));
      });
    });

    list.push({
      id: internalAgent.id,
      name: internalAgent.name,
      icon: internalAgent.icon || 'forge-ade',
      type: 'internal',
      description: internalAgent.description,
      isCurrent: activeAgentId === internalAgent.id || (!activeAgentId && internalAgent.id === 'agent-internal'),
      models: internalModels
    });

    // 2. ACP External Agents (only included if ACP is enabled, agent is enabled, AND agent is available/installed)
    if (acpEnabled !== false) {
      (agents || []).forEach(ag => {
        if (ag.type === 'internal') return;
        if (ag.enabled === false) return;
        if (ag.available !== true) return;
        const supported = ag.supportedModels || (ag.model ? [ag.model] : []);
        const agModels: ModelMeta[] = supported.map(m => {
          const meta = parseModelMetadata(m, ag.name, ag.id);
          meta.agentName = ag.name;
          return meta;
        });

        list.push({
          id: ag.id,
          name: ag.name,
          icon: ag.icon || ag.provider || 'custom',
          type: 'acp',
          description: ag.description,
          isCurrent: activeAgentId === ag.id,
          models: agModels
        });
      });
    }

    return list;
  }, [agents, providers, activeAgentId, acpEnabled]);

  // Trigger availability scan when opening
  useEffect(() => {
    if (isOpen) {
      void checkAgentsAvailability?.();
    }
  }, [isOpen, checkAgentsAvailability]);

  // Set default selected harness when opening
  useEffect(() => {
    if (isOpen) {
      const current = harnessGroups.find(h => h.isCurrent);
      setSelectedHarnessId(current ? current.id : (harnessGroups[0]?.id || 'agent-internal'));
      setSelectedProviderFilter('all');
      setHighlightedIdx(0);
      setTimeout(() => inputRef.current?.focus(), 40);
    } else {
      setSearch(prev => (prev ? '' : prev));
    }
  }, [isOpen, harnessGroups]);

  const activeHarness = useMemo(() => {
    return harnessGroups.find(h => h.id === selectedHarnessId) || harnessGroups[0];
  }, [harnessGroups, selectedHarnessId]);

  const providerChoices = useMemo(() => {
    if (!activeHarness || activeHarness.type !== 'internal') return [];
    const seen = new Map<string, { name: string; key: string; count: number; hasApiKey: boolean }>();
    for (const m of activeHarness.models) {
      const pName = m.provider || 'Custom';
      const existing = seen.get(pName);
      if (existing) {
        existing.count++;
      } else {
        const provCfg = (providers || []).find(p => p.name === pName);
        const isLocalProv = m.isLocal || pName.toLowerCase().includes('ollama') || pName.toLowerCase().includes('lm studio');
        const hasKey = Boolean(provCfg?.apiKey?.trim()) || isLocalProv;
        seen.set(pName, {
          name: pName,
          key: m.providerKey || pName,
          count: 1,
          hasApiKey: hasKey
        });
      }
    }
    return Array.from(seen.values());
  }, [activeHarness, providers]);

  const filteredHarnessModels = useMemo(() => {
    const models = activeHarness?.models || [];
    if (selectedProviderFilter === 'all' || activeHarness?.type !== 'internal') return models;
    return models.filter(m => m.provider === selectedProviderFilter);
  }, [activeHarness, selectedProviderFilter]);

  // Search filter across ALL models and harnesses
  const searchResults = useMemo<ModelMeta[]>(() => {
    const q = search.trim().toLowerCase();
    if (!q) return [];

    const all: ModelMeta[] = [];
    const seen = new Set<string>();

    harnessGroups.forEach(h => {
      h.models.forEach(m => {
        const key = `${h.id}:${m.id}`;
        if (!seen.has(key)) {
          seen.add(key);
          const matches =
            m.displayName.toLowerCase().includes(q) ||
            m.id.toLowerCase().includes(q) ||
            m.provider.toLowerCase().includes(q) ||
            h.name.toLowerCase().includes(q);

          if (matches) {
            all.push({
              ...m,
              agentId: h.id,
              agentName: h.name
            });
          }
        }
      });
    });

    return all;
  }, [harnessGroups, search]);

  useEffect(() => {
    setHighlightedIdx(0);
  }, [search, selectedHarnessId, selectedProviderFilter]);

  const handleSelectModel = async (model: ModelMeta, harnessId?: string) => {
    const targetAgentId = harnessId || model.agentId || 'agent-internal';

    if (targetAgentId !== activeAgentId) {
      setActiveAgentId(targetAgentId);
    }

    setCurrentModel(model.id);

    try {
      const p = (providers || []).find(prov => (prov.models || []).includes(model.id));
      await SetActiveModel(p?.id || 'default', model.id);
    } catch {}

    showToast(`Model set to ${model.displayName}`, 'info');
    onClose();
  };

  const handleSelectDefault = async (harness: HarnessGroup) => {
    if (harness.id !== activeAgentId) {
      setActiveAgentId(harness.id);
    }
    const defaultModel = harness.models[0]?.id || 'default';
    setCurrentModel(defaultModel);

    try {
      await SetActiveModel('default', defaultModel);
    } catch {}

    showToast(`Using default model for ${harness.name}`, 'info');
    onClose();
  };

  const handleCustomSubmit = (e?: React.FormEvent) => {
    e?.preventDefault();
    const val = search.trim();
    if (!val) return;
    setCurrentModel(val);
    showToast(`Custom model set to ${val}`, 'info');
    onClose();
  };

  const handleOpenProviderSettings = () => {
    onClose();
    if (openSettingsTab) {
      openSettingsTab('model');
    } else {
      setSettingsActiveSection?.('model');
      setMode?.('settings');
    }
  };

  // Handle outside click & keyboard navigation
  useEffect(() => {
    if (!isOpen) return;

    const handlePointerDown = (e: MouseEvent) => {
      const target = e.target as Node | null;
      if (popoverRef.current?.contains(target)) return;
      if (anchorRef?.current?.contains(target)) return;
      onClose();
    };

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        onClose();
      }
    };

    document.addEventListener('mousedown', handlePointerDown);
    window.addEventListener('keydown', handleKeyDown);
    return () => {
      document.removeEventListener('mousedown', handlePointerDown);
      window.removeEventListener('keydown', handleKeyDown);
    };
  }, [isOpen, onClose, anchorRef]);

  const handleSetEffort = (effort: AgentReasoningLevel) => {
    setReasoningLevel(effort);
    SetActiveReasoningEffort(effort).catch(() => {});
    showToast(`Reasoning effort: ${effort}`, 'info');
  };

  if (!isOpen) return null;

  const isSearching = search.trim().length > 0;
  const activeList = isSearching ? searchResults : filteredHarnessModels;

  const handleSearchKeyDown = (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      if (activeList.length > 0) {
        setHighlightedIdx(prev => (prev + 1) % activeList.length);
      }
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      if (activeList.length > 0) {
        setHighlightedIdx(prev => (prev - 1 + activeList.length) % activeList.length);
      }
    } else if (e.key === 'Enter') {
      e.preventDefault();
      const target = activeList[highlightedIdx];
      if (target) {
        void handleSelectModel(target, isSearching ? target.agentId : activeHarness?.id);
      } else if (isSearching) {
        handleCustomSubmit();
      }
    }
  };

  const content = (
    <div
      ref={popoverRef}
      role="dialog"
      data-portal-modal="true"
      aria-label="Harness and model"
      style={{
        position: 'fixed',
        left: coords.left,
        top: coords.top,
        bottom: coords.bottom,
        width: coords.width,
        maxHeight: `${coords.maxHeight}px`,
        zIndex: 9999
      }}
      className="flex flex-col rounded-2xl border border-border/80 bg-panel/95 backdrop-blur-md shadow-2xl overflow-hidden font-sans text-foreground animate-in fade-in zoom-in-95 duration-100 select-none"
    >
      {/* Top Search Bar + Configure Providers Action */}
      <div className="flex items-center gap-2 h-10 px-3 border-b border-border/60 bg-surface/30 shrink-0">
        <Search className="size-4 text-foreground-subtle shrink-0" />
        <input
          ref={inputRef}
          type="search"
          role="combobox"
          aria-expanded="true"
          value={search}
          onChange={e => setSearch(e.target.value)}
          onKeyDown={handleSearchKeyDown}
          placeholder="Search models, providers, or harnesses..."
          className="flex-1 min-w-0 bg-transparent text-ui-sm text-foreground placeholder:text-foreground-subtle/60 outline-none border-0 p-0"
        />
        {search && (
          <button
            type="button"
            onClick={() => setSearch('')}
            className="p-1 rounded text-foreground-subtle hover:text-foreground cursor-pointer"
          >
            <X className="size-3.5" />
          </button>
        )}
        <button
          type="button"
          onClick={handleOpenProviderSettings}
          title="Configure API keys & model providers"
          className="flex items-center gap-1 rounded-md border border-border/70 bg-surface px-2 py-1 text-2xs font-medium text-foreground-subtle hover:bg-surface-hover hover:text-foreground transition-colors cursor-pointer shrink-0"
        >
          <Settings2 className="size-3" />
          <span className="hidden sm:inline">Providers</span>
        </button>
      </div>

      {/* Main 2-Column Body (PandaCode Split Layout) */}
      <div className="grid grid-cols-[140px_minmax(0,1fr)] sm:grid-cols-[172px_minmax(0,1fr)] flex-1 min-h-0 overflow-hidden">
        {/* Left Rail: Harnesses List */}
        <div className="bg-surface/40 overflow-y-auto p-1.5 border-r border-border/60 scrollbar-thin flex flex-col gap-0.5">
          <div className="px-2 py-1 text-2xs font-semibold text-foreground-subtle/75 uppercase tracking-wider">
            Harnesses
          </div>

          {harnessGroups.map(h => {
            const isTabActive = h.id === selectedHarnessId;
            return (
              <button
                key={h.id}
                type="button"
                onClick={() => {
                  setSelectedHarnessId(h.id);
                  setSelectedProviderFilter('all');
                  if (isSearching) setSearch('');
                }}
                className={cn(
                  'flex items-center gap-2 w-full px-2 py-1.5 rounded-lg text-ui-xs transition-colors text-left truncate cursor-pointer group',
                  isTabActive
                    ? 'bg-primary/10 text-primary font-semibold'
                    : 'text-foreground-subtle hover:text-foreground hover:bg-surface-hover'
                )}
                title={h.name}
              >
                <div className="size-4 rounded flex items-center justify-center shrink-0">
                  <ProviderIcon provider={h.icon} size={14} />
                </div>
                <span className="truncate flex-1 leading-tight">{h.name}</span>
                {h.isCurrent && (
                  <span
                    className="size-1.5 rounded-full bg-primary shrink-0"
                    title="Active harness in session"
                  />
                )}
              </button>
            );
          })}
        </div>

        {/* Right List: Models for Selected Harness (or Search Results) */}
        <div className="overflow-y-auto p-2 scrollbar-thin space-y-1 min-h-0 flex flex-col">
          {isSearching ? (
            /* Search Results Mode */
            searchResults.length === 0 ? (
              <div className="py-8 text-center text-foreground-subtle space-y-2">
                <p className="text-ui-xs">No matching models found.</p>
                <button
                  type="button"
                  onClick={handleCustomSubmit}
                  className="inline-flex items-center gap-1.5 px-3 py-1.5 rounded-lg bg-primary text-primary-foreground text-ui-xs font-semibold hover:bg-primary/90 cursor-pointer shadow-xs"
                >
                  <Plus className="size-3.5" />
                  <span>Use custom "{search}"</span>
                </button>
              </div>
            ) : (
              searchResults.map((item, idx) => {
                const isSelected = item.id === currentModel;
                const isHighlighted = idx === highlightedIdx;
                return (
                  <button
                    key={`${item.agentId}-${item.id}`}
                    type="button"
                    onMouseEnter={() => setHighlightedIdx(idx)}
                    onClick={() => handleSelectModel(item, item.agentId)}
                    className={cn(
                      'flex items-center justify-between gap-2 w-full px-2.5 py-1.5 rounded-lg text-ui-xs text-left cursor-pointer transition-colors select-none group',
                      isSelected
                        ? 'bg-primary/15 text-primary font-semibold border border-primary/20'
                        : isHighlighted
                          ? 'bg-surface-hover text-foreground'
                          : 'text-foreground-subtle hover:text-foreground hover:bg-surface-hover'
                    )}
                  >
                    <div className="flex items-center gap-2 min-w-0">
                      <ProviderIcon provider={item.providerKey} size={14} className="shrink-0" />
                      <span className="truncate font-medium text-foreground group-hover:text-foreground">
                        {item.displayName}
                      </span>
                      {item.isLocal && (
                        <span className="text-3xs px-1.5 py-0.2 rounded font-semibold bg-success/15 text-success border border-success/25 shrink-0">
                          Local
                        </span>
                      )}
                    </div>

                    <div className="flex items-center gap-1.5 shrink-0 ml-auto">
                      <span className="text-2xs text-foreground-subtle font-mono">
                        {item.contextWindow || item.provider || item.agentName}
                      </span>
                      {isSelected && (
                        <div className="size-4 rounded-full bg-primary text-primary-foreground flex items-center justify-center shrink-0">
                          <Check className="size-2.5 stroke-[3]" />
                        </div>
                      )}
                    </div>
                  </button>
                );
              })
            )
          ) : (
            /* Standard Harness Catalog Mode */
            <>
              {/* Harness Group Header */}
              {activeHarness && (
                <div className="flex items-center gap-2 px-2 py-1 mb-0.5 border-b border-border/40 pb-1.5 shrink-0">
                  <ProviderIcon provider={activeHarness.icon} size={15} />
                  <span className="text-ui-xs font-bold text-foreground truncate">
                    {activeHarness.name}
                  </span>
                  <span className="text-2xs text-foreground-subtle ml-auto shrink-0">
                    {filteredHarnessModels.length} {filteredHarnessModels.length === 1 ? 'model' : 'models'}
                  </span>
                </div>
              )}

              {/* Provider Filter Pills when viewing multi-provider Internal Engine */}
              {providerChoices.length > 1 && (
                <div className="flex items-center gap-1 overflow-x-auto pb-1 pt-0.5 px-1 scrollbar-none shrink-0">
                  <button
                    type="button"
                    onClick={() => setSelectedProviderFilter('all')}
                    className={cn(
                      'px-2 py-0.5 rounded-md text-2xs font-medium shrink-0 transition-colors cursor-pointer',
                      selectedProviderFilter === 'all'
                        ? 'bg-foreground text-background'
                        : 'bg-surface border border-border/60 text-foreground-subtle hover:text-foreground'
                    )}
                  >
                    All
                  </button>
                  {providerChoices.map(prov => {
                    const active = selectedProviderFilter === prov.name;
                    return (
                      <button
                        key={prov.name}
                        type="button"
                        onClick={() => setSelectedProviderFilter(prov.name)}
                        title={prov.hasApiKey ? `${prov.name} (${prov.count} models)` : `${prov.name} — API key not configured`}
                        className={cn(
                          'flex items-center gap-1 px-2 py-0.5 rounded-md text-2xs font-medium shrink-0 transition-colors cursor-pointer',
                          active
                            ? 'bg-foreground text-background'
                            : 'bg-surface border border-border/60 text-foreground-subtle hover:text-foreground'
                        )}
                      >
                        <ProviderIcon provider={prov.key} size={11} />
                        <span className="truncate max-w-[90px]">{prov.name.replace(/\s+Claude$/i, '')}</span>
                        <span
                          className={cn(
                            'size-1.5 rounded-full shrink-0',
                            prov.hasApiKey ? 'bg-success' : 'bg-warning'
                          )}
                        />
                      </button>
                    );
                  })}
                </div>
              )}

              {/* Empty state when Internal Engine has no enabled custom providers/models */}
              {activeHarness?.type === 'internal' && filteredHarnessModels.length === 0 && (
                <div className="flex flex-col items-center justify-center py-8 px-3 text-center space-y-2.5">
                  <p className="text-ui-xs text-foreground-subtle">
                    No custom OpenAI or Anthropic provider configured.
                  </p>
                  <button
                    type="button"
                    onClick={handleOpenProviderSettings}
                    className="inline-flex items-center gap-1.5 rounded-lg bg-foreground px-3 py-1.5 text-ui-xs font-semibold text-background hover:opacity-90 transition-opacity cursor-pointer shadow-2xs"
                  >
                    <Plus className="size-3.5" />
                    <span>Add Custom Provider</span>
                  </button>
                </div>
              )}

              {/* Option 1: Default (PandaCode signature entry — shown when models exist or on ACP harness) */}
              {activeHarness && selectedProviderFilter === 'all' && (activeHarness.type === 'acp' || filteredHarnessModels.length > 0) && (
                <button
                  type="button"
                  onClick={() => handleSelectDefault(activeHarness)}
                  className={cn(
                    'flex items-center justify-between gap-2 w-full px-2.5 py-1.5 rounded-lg text-ui-xs text-left cursor-pointer transition-colors select-none group shrink-0',
                    !currentModel || currentModel === 'default'
                      ? 'bg-primary/15 text-primary font-semibold border border-primary/20'
                      : 'text-foreground-subtle hover:text-foreground hover:bg-surface-hover'
                  )}
                >
                  <div className="flex items-center gap-2 min-w-0">
                    <ProviderIcon provider={activeHarness.icon} size={14} className="shrink-0" />
                    <span className="font-semibold text-foreground">Default</span>
                  </div>
                  <span className="text-2xs text-foreground-subtle truncate max-w-[140px]">
                    Uses harness default
                  </span>
                  {(!currentModel || currentModel === 'default') && (
                    <div className="size-4 rounded-full bg-primary text-primary-foreground flex items-center justify-center shrink-0 ml-auto">
                      <Check className="size-2.5 stroke-[3]" />
                    </div>
                  )}
                </button>
              )}

              {/* Option 2..N: Harness Models */}
              {filteredHarnessModels.map((item, idx) => {
                const isSelected = item.id === currentModel;
                const isHighlighted = idx === highlightedIdx;
                const prevProvider = idx > 0 ? filteredHarnessModels[idx - 1]?.provider : null;
                const showProviderHeader =
                  activeHarness?.type === 'internal' &&
                  selectedProviderFilter === 'all' &&
                  providerChoices.length > 1 &&
                  item.provider &&
                  item.provider !== prevProvider;
                const provInfo = providerChoices.find(p => p.name === item.provider);

                return (
                  <React.Fragment key={`${activeHarness?.id}-${item.id}`}>
                    {showProviderHeader && (
                      <div className="flex items-center justify-between px-2 pt-2 pb-0.5 text-3xs font-semibold uppercase tracking-wider text-foreground-subtlest">
                        <span className="truncate">{item.provider}</span>
                        {provInfo && !provInfo.hasApiKey && (
                          <button
                            type="button"
                            onClick={(e) => {
                              e.stopPropagation();
                              handleOpenProviderSettings();
                            }}
                            className="inline-flex items-center gap-1 font-sans normal-case text-warning hover:underline cursor-pointer"
                            title="Add API key in Settings → Model settings"
                          >
                            <KeyRound className="size-2.5" />
                            <span>Set API key</span>
                          </button>
                        )}
                      </div>
                    )}
                    <button
                      type="button"
                      onMouseEnter={() => setHighlightedIdx(idx)}
                      onClick={() => handleSelectModel(item, activeHarness?.id)}
                      className={cn(
                        'flex items-center justify-between gap-2 w-full px-2.5 py-1.5 rounded-lg text-ui-xs text-left cursor-pointer transition-colors select-none group shrink-0',
                        isSelected
                          ? 'bg-primary/15 text-primary font-semibold border border-primary/20'
                          : isHighlighted
                            ? 'bg-surface-hover text-foreground'
                            : 'text-foreground-subtle hover:text-foreground hover:bg-surface-hover'
                      )}
                    >
                      <div className="flex items-center gap-2 min-w-0">
                        <ProviderIcon provider={item.providerKey} size={14} className="shrink-0" />
                        <span className="truncate font-medium text-foreground">
                          {item.displayName}
                        </span>
                        {item.isLocal && (
                          <span className="text-3xs px-1.5 py-0.2 rounded font-semibold bg-success/15 text-success border border-success/25 shrink-0">
                            Local
                          </span>
                        )}
                      </div>

                      <div className="flex items-center gap-1.5 shrink-0 ml-auto">
                        {item.contextWindow && (
                          <span className="text-2xs text-foreground-subtle font-mono">
                            {item.contextWindow}
                          </span>
                        )}
                        {isSelected && (
                          <div className="size-4 rounded-full bg-primary text-primary-foreground flex items-center justify-center shrink-0">
                            <Check className="size-2.5 stroke-[3]" />
                          </div>
                        )}
                      </div>
                    </button>
                  </React.Fragment>
                );
              })}
            </>
          )}
        </div>
      </div>

      {/* Bottom Footer: Local LLM Status & PandaCode Segmented Reasoning Effort Control */}
      <div className="border-t border-border/60 p-2 bg-surface/30 shrink-0 space-y-1.5">
        {/* Local LLM & Agent Discovery Row */}
        <div className="flex items-center justify-between text-2xs px-1 text-foreground-subtle">
          <div className="flex items-center gap-1.5 min-w-0">
            <span
              className={cn(
                "size-2 rounded-full shrink-0",
                localOnlineCount > 0 ? "bg-success" : "bg-foreground-subtle/40"
              )}
            />
            <span className="truncate">
              {harnessGroups.length - 1} ACP agent(s) · {localOnlineCount > 0 ? `${localOnlineCount} local LLM(s) online` : 'Local LLMs offline'}
            </span>
          </div>
          <button
            type="button"
            onClick={() => void handleScanAll()}
            disabled={isScanningLocal || isScanningAgents}
            className="flex items-center gap-1 px-2 py-0.5 rounded-md border border-border/70 bg-surface hover:bg-surface-hover text-foreground-subtle hover:text-foreground cursor-pointer transition-colors text-2xs shrink-0"
            title="Scan PATH for installed ACP/CLI agents and probe Ollama/LM Studio"
          >
            <RefreshCw className={cn("size-3", (isScanningLocal || isScanningAgents) && "animate-spin text-primary")} />
            <span>{isScanningLocal || isScanningAgents ? 'Scanning...' : 'Scan Agents & Local'}</span>
          </button>
        </div>

        <div className="flex items-center gap-2">
          <span className="text-2xs font-medium text-foreground-subtle w-11 shrink-0 px-1">
            Effort
          </span>

          <div className="flex flex-1 items-center bg-surface p-0.5 rounded-lg border border-border/60">
            {EFFORT_OPTIONS.map(opt => {
              const active = (reasoningLevel || '') === opt.id;
              return (
                <button
                  key={opt.id}
                  type="button"
                  onClick={() => handleSetEffort(opt.id)}
                  className={cn(
                    'flex-1 py-0.5 text-2xs rounded-md transition-all text-center cursor-pointer select-none',
                    active
                      ? 'bg-panel text-foreground font-semibold shadow-xs'
                      : 'text-foreground-subtle hover:text-foreground'
                  )}
                  title={opt.desc}
                >
                  {opt.label}
                </button>
              );
            })}
          </div>
        </div>
      </div>
    </div>
  );

  return typeof document !== 'undefined' ? createPortal(content, document.body) : null;
};
