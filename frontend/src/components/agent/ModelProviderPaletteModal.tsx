import React, { useState, useMemo, useEffect, useLayoutEffect, useRef } from 'react';
import { createPortal } from 'react-dom';
import {
  Search,
  Check,
  Brain,
  Plus,
  X,
  History,
  Workflow,
  Sparkles,
  RefreshCw,
  Cpu
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
    activeAgent,
    activeAgentId,
    setActiveAgentId,
    reasoningLevel,
    setReasoningLevel,
    activeSession,
    acpEnabled
  } = useWorkspace();

  const localServices = useProviderStore(s => s.localServices);
  const isScanningLocal = useProviderStore(s => s.isScanningLocal);
  const discoverLocalProviders = useProviderStore(s => s.discoverLocalProviders);

  const localOnlineCount = useMemo(() => {
    return (localServices || []).filter(s => s.running).length;
  }, [localServices]);

  const [search, setSearch] = useState('');
  const [selectedHarnessId, setSelectedHarnessId] = useState<string>('');
  const inputRef = useRef<HTMLInputElement>(null);
  const popoverRef = useRef<HTMLDivElement>(null);

  const [coords, setCoords] = useState<{ top?: number; bottom?: number; left: number; width: number }>({
    left: 20,
    width: 520
  });

  // Calculate anchored popover coordinates
  useLayoutEffect(() => {
    if (!isOpen) return;
    if (anchorRef?.current) {
      const rect = anchorRef.current.getBoundingClientRect();
      const composerSurface = anchorRef.current.closest('.chat-composer-surface') || anchorRef.current.closest('[data-file-drop-target="true"]');
      const composerRect = composerSurface?.getBoundingClientRect();

      const maxAvailableWidth = composerRect ? Math.max(300, composerRect.width) : (window.innerWidth - 24);
      const width = Math.min(520, window.innerWidth - 24, maxAvailableWidth);

      const minLeft = composerRect ? Math.max(12, composerRect.left) : 12;
      const maxRight = composerRect ? Math.min(window.innerWidth - 12, composerRect.right) : (window.innerWidth - 12);
      const idealLeft = Math.max(minLeft, Math.min(rect.left, maxRight - width));
      const left = Math.max(12, Math.min(idealLeft, window.innerWidth - width - 12));
      const gap = 8;
      const height = 440;

      const roomAbove = rect.top;
      if (roomAbove >= height + gap || roomAbove > window.innerHeight - rect.bottom) {
        setCoords({
          bottom: window.innerHeight - rect.top + gap,
          left,
          width
        });
      } else {
        setCoords({
          top: rect.bottom + gap,
          left,
          width
        });
      }
    } else {
      // Fallback: center in viewport
      const width = Math.min(520, window.innerWidth - 24);
      setCoords({
        top: Math.max(20, (window.innerHeight - 440) / 2),
        left: Math.max(12, (window.innerWidth - width) / 2),
        width
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
      const models = (p.selectedModels && p.selectedModels.length > 0 ? p.selectedModels : p.models) || [];
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

    // 2. ACP External Agents (only included if ACP is enabled and agent is enabled)
    if (acpEnabled !== false) {
      (agents || []).forEach(ag => {
        if (ag.type === 'internal') return;
        if (ag.enabled === false) return;
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

  // Set default selected harness when opening
  useEffect(() => {
    if (isOpen) {
      const current = harnessGroups.find(h => h.isCurrent);
      setSelectedHarnessId(current ? current.id : (harnessGroups[0]?.id || 'agent-internal'));
      setTimeout(() => inputRef.current?.focus(), 40);
    } else {
      setSearch(prev => (prev ? '' : prev));
    }
  }, [isOpen, harnessGroups]);

  // Handle outside click & escape key
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

  const activeHarness = useMemo(() => {
    return harnessGroups.find(h => h.id === selectedHarnessId) || harnessGroups[0];
  }, [harnessGroups, selectedHarnessId]);

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

  const handleCustomSubmit = (e: React.FormEvent) => {
    e.preventDefault();
    const val = search.trim();
    if (!val) return;
    setCurrentModel(val);
    showToast(`Custom model set to ${val}`, 'info');
    onClose();
  };

  const handleSetEffort = (effort: AgentReasoningLevel) => {
    setReasoningLevel(effort);
    SetActiveReasoningEffort(effort).catch(() => {});
    showToast(`Reasoning effort: ${effort}`, 'info');
  };

  if (!isOpen) return null;

  const isSearching = search.trim().length > 0;

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
        maxHeight: '440px',
        zIndex: 9999
      }}
      className="flex flex-col rounded-2xl border border-border/80 bg-panel/95 backdrop-blur-md shadow-2xl overflow-hidden font-sans text-foreground animate-in fade-in zoom-in-95 duration-100 select-none"
    >
      {/* Top Search Bar */}
      <div className="flex items-center gap-2 h-10 px-3 border-b border-border/60 bg-surface/30 shrink-0">
        <Search className="size-4 text-foreground-subtle shrink-0" />
        <input
          ref={inputRef}
          type="search"
          role="combobox"
          aria-expanded="true"
          value={search}
          onChange={e => setSearch(e.target.value)}
          placeholder="Search models and harnesses..."
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
      </div>

      {/* Main 2-Column Body (PandaCode Split Layout) */}
      <div className="grid grid-cols-[140px_minmax(0,1fr)] sm:grid-cols-[180px_minmax(0,1fr)] flex-1 min-h-0 overflow-hidden">
        {/* Left Rail: Harnesses List */}
        <div className="bg-surface/40 overflow-y-auto p-1 border-r border-border/60 scrollbar-thin flex flex-col gap-0.5">
          <div className="px-2 py-1 text-[10px] font-bold text-foreground-subtle/70 uppercase tracking-wider">
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
        <div className="overflow-y-auto p-2 scrollbar-thin space-y-1 min-h-0">
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
              searchResults.map(item => {
                const isSelected = item.id === currentModel;
                return (
                  <button
                    key={`${item.agentId}-${item.id}`}
                    type="button"
                    onClick={() => handleSelectModel(item, item.agentId)}
                    className={cn(
                      'flex items-center justify-between gap-2 w-full px-2.5 py-1.5 rounded-lg text-ui-xs text-left cursor-pointer transition-colors select-none group',
                      isSelected
                        ? 'bg-primary/15 text-primary font-semibold border border-primary/20'
                        : 'text-foreground-subtle hover:text-foreground hover:bg-surface-hover'
                    )}
                  >
                    <div className="flex items-center gap-2 min-w-0">
                      <ProviderIcon provider={item.providerKey} size={14} className="shrink-0" />
                      <span className="truncate font-medium text-foreground group-hover:text-foreground">
                        {item.displayName}
                      </span>
                      {item.isLocal && (
                        <span className="text-[9px] px-1.5 py-0.2 rounded font-semibold bg-emerald-500/15 text-emerald-600 dark:text-emerald-400 border border-emerald-500/25 shrink-0">
                          Local
                        </span>
                      )}
                    </div>

                    <div className="flex items-center gap-1.5 shrink-0 ml-auto">
                      <span className="text-[10px] text-foreground-subtle font-mono">
                        {item.contextWindow || item.agentName}
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
            /* Standard Harness Catalog Mode (PandaCode Default) */
            <>
              {/* Harness Group Header */}
              {activeHarness && (
                <div className="flex items-center gap-2 px-2 py-1 mb-1 border-b border-border/40 pb-1.5">
                  <ProviderIcon provider={activeHarness.icon} size={15} />
                  <span className="text-ui-xs font-bold text-foreground">
                    {activeHarness.name}
                  </span>
                  <span className="text-[10px] text-foreground-subtle ml-auto">
                    {activeHarness.models.length} models
                  </span>
                </div>
              )}

              {/* Option 1: Default (PandaCode signature entry) */}
              {activeHarness && (
                <button
                  type="button"
                  onClick={() => handleSelectDefault(activeHarness)}
                  className={cn(
                    'flex items-center justify-between gap-2 w-full px-2.5 py-1.5 rounded-lg text-ui-xs text-left cursor-pointer transition-colors select-none group',
                    !currentModel || currentModel === 'default'
                      ? 'bg-primary/15 text-primary font-semibold border border-primary/20'
                      : 'text-foreground-subtle hover:text-foreground hover:bg-surface-hover'
                  )}
                >
                  <div className="flex items-center gap-2 min-w-0">
                    <ProviderIcon provider={activeHarness.icon} size={14} className="shrink-0" />
                    <span className="font-semibold text-foreground">Default</span>
                  </div>
                  <span className="text-[10px] text-foreground-subtle truncate max-w-[140px]">
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
              {(activeHarness?.models || []).map(item => {
                const isSelected = item.id === currentModel;
                return (
                  <button
                    key={`${activeHarness?.id}-${item.id}`}
                    type="button"
                    onClick={() => handleSelectModel(item, activeHarness?.id)}
                    className={cn(
                      'flex items-center justify-between gap-2 w-full px-2.5 py-1.5 rounded-lg text-ui-xs text-left cursor-pointer transition-colors select-none group',
                      isSelected
                        ? 'bg-primary/15 text-primary font-semibold border border-primary/20'
                        : 'text-foreground-subtle hover:text-foreground hover:bg-surface-hover'
                    )}
                  >
                    <div className="flex items-center gap-2 min-w-0">
                      <ProviderIcon provider={item.providerKey} size={14} className="shrink-0" />
                      <span className="truncate font-medium text-foreground">
                        {item.displayName}
                      </span>
                      {item.isLocal && (
                        <span className="text-[9px] px-1.5 py-0.2 rounded font-semibold bg-emerald-500/15 text-emerald-600 dark:text-emerald-400 border border-emerald-500/25 shrink-0">
                          Local
                        </span>
                      )}
                    </div>

                    <div className="flex items-center gap-1.5 shrink-0 ml-auto">
                      {item.contextWindow && (
                        <span className="text-[10px] text-foreground-subtle font-mono">
                          {item.contextWindow} context
                        </span>
                      )}
                      {isSelected && (
                        <div className="size-4 rounded-full bg-primary text-primary-foreground flex items-center justify-center shrink-0">
                          <Check className="size-2.5 stroke-[3]" />
                        </div>
                      )}
                    </div>
                  </button>
                );
              })}
            </>
          )}
        </div>
      </div>

      {/* Bottom Footer: Local LLM Status & PandaCode Segmented Reasoning Effort Control */}
      <div className="border-t border-border/60 p-2 bg-surface/30 shrink-0 space-y-2">
        {/* Local LLM Discovery Row */}
        <div className="flex items-center justify-between text-[11px] px-1 text-foreground-subtle">
          <div className="flex items-center gap-1.5">
            <span
              className={cn(
                "size-2 rounded-full shrink-0",
                localOnlineCount > 0 ? "bg-emerald-500 shadow-[0_0_6px_rgba(16,185,129,0.5)]" : "bg-foreground-subtle/40"
              )}
            />
            <span className="truncate">
              Local LLMs: {localOnlineCount > 0 ? `${localOnlineCount} local server(s) detected` : 'Ollama & LM Studio offline'}
            </span>
          </div>
          <button
            type="button"
            onClick={() => void discoverLocalProviders()}
            disabled={isScanningLocal}
            className="flex items-center gap-1 px-1.5 py-0.5 rounded border border-border/70 hover:bg-surface-hover text-foreground-subtle hover:text-foreground cursor-pointer transition-colors text-[10px]"
            title="Probe 127.0.0.1:11434 (Ollama) and 127.0.0.1:1234 (LM Studio)"
          >
            <RefreshCw className={cn("size-3", isScanningLocal && "animate-spin text-primary")} />
            <span>{isScanningLocal ? 'Scanning...' : 'Scan Local'}</span>
          </button>
        </div>

        <div className="flex items-center gap-2">
          <span className="text-[11px] font-medium text-foreground-subtle w-11 shrink-0">
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
                    'flex-1 py-0.5 text-[11px] rounded-md transition-all text-center cursor-pointer select-none',
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

        {/* Effort Hint Text */}
        <p className="text-[10px] text-foreground-subtle/80 pl-13 truncate">
          {EFFORT_OPTIONS.find(o => o.id === (reasoningLevel || ''))?.desc ||
            'Standard model reasoning configuration'}
        </p>
      </div>
    </div>
  );

  return typeof document !== 'undefined' ? createPortal(content, document.body) : null;
};
