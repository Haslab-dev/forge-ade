import React, { useState, useRef, useEffect } from 'react';
import { ChevronDown, Check, Sparkles, Cpu, Bot } from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';
import { SetActiveModel, AcpGetAgentModels, AcpSetSessionModel } from '../../lib/wails';

interface ModelGroup {
  providerId: string;
  providerName: string;
  models: string[];
}

export const ModelSwitcherDropdown: React.FC<{ compact?: boolean }> = ({ compact = false }) => {
  const { currentModel, setCurrentModel, activeAgent, providers, sessions, activeSessionId } = useWorkspace();
  const [isOpen, setIsOpen] = useState(false);
  const [acpModels, setAcpModels] = useState<string[]>([]);
  const dropdownRef = useRef<HTMLDivElement>(null);

  const isInternal = !activeAgent || activeAgent.type === 'internal';
  const agentId = activeAgent?.id || '';

  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
        setIsOpen(false);
      }
    };
    // Close when keyboard focus moves into the composer input or anywhere
    // outside the dropdown (covers Tab/programmatic focus, not just clicks).
    const handleFocusIn = (e: FocusEvent) => {
      if (!isOpen) return;
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
        setIsOpen(false);
      }
    };
    const handleEscape = (e: KeyboardEvent) => {
      if (e.key === 'Escape' && isOpen) {
        setIsOpen(false);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    document.addEventListener('focusin', handleFocusIn);
    document.addEventListener('keydown', handleEscape);
    return () => {
      document.removeEventListener('mousedown', handleClickOutside);
      document.removeEventListener('focusin', handleFocusIn);
      document.removeEventListener('keydown', handleEscape);
    };
  }, [isOpen]);

  // External (ACP) agents list the models their binary reports; fall back to
  // the models advertised on the agent record.
  useEffect(() => {
    if (isInternal || !agentId) {
      setAcpModels([]);
      return;
    }
    let cancelled = false;
    AcpGetAgentModels(agentId)
      .then((models) => {
        if (!cancelled && Array.isArray(models) && models.length > 0) setAcpModels(models);
      })
      .catch(() => { /* binary unreachable — supportedModels fallback */ });
    return () => { cancelled = true; };
  }, [isInternal, agentId, isOpen]);

  // Internal agents list models grouped by the enabled providers they come
  // from, so picking a model also picks the provider (and its API key).
  const modelGroups = React.useMemo<ModelGroup[]>(() => {
    if (!isInternal) return [];
    const groups: ModelGroup[] = [];
    const seen = new Set<string>();
    (providers || []).filter(p => p.enabled).forEach(p => {
      const list = (p.selectedModels && p.selectedModels.length > 0 ? p.selectedModels : p.models) || [];
      const models: string[] = [];
      list.forEach(m => {
        if (!seen.has(m)) {
          seen.add(m);
          models.push(m);
        }
      });
      if (models.length > 0) {
        groups.push({ providerId: p.id, providerName: p.name, models });
      }
    });
    return groups;
  }, [isInternal, providers]);

  const externalModels = React.useMemo(() => {
    if (isInternal) return [];
    const base = acpModels.length > 0 ? acpModels : (activeAgent?.supportedModels || []);
    return Array.from(new Set(base));
  }, [isInternal, acpModels, activeAgent?.supportedModels]);

  const availableModels = React.useMemo(() => {
    if (!isInternal) return externalModels;
    return modelGroups.flatMap(g => g.models);
  }, [isInternal, externalModels, modelGroups]);

  const findGroupFor = (model: string): ModelGroup | undefined =>
    modelGroups.find(g => g.models.includes(model));

  const selectModel = async (modelName: string) => {
    setCurrentModel(modelName);
    setIsOpen(false);
    if (isInternal) {
      const group = findGroupFor(modelName);
      if (group) {
        try { await SetActiveModel(group.providerId, modelName); } catch { /* ignore */ }
      }
      return;
    }
    // External agent: apply to the live ACP session if one exists; new
    // sessions pick it up from currentModel at prompt time.
    const session = sessions?.find(s => s.id === activeSessionId);
    const acpSessionId = session?.acpSessionId || (activeAgent as any)?.sessionId;
    if (acpSessionId) {
      try { await AcpSetSessionModel(acpSessionId, modelName); } catch { /* ignore */ }
    }
  };

  const renderGroups = () => {
    if (availableModels.length === 0) {
      return (
        <div className="px-3 py-3 text-center text-xs text-foreground-subtle dark:text-foreground-subtle">
          No models fetched or selected.<br/>
          <span className="text-ui-xs text-primary dark:text-info mt-1 block">
            Configure in Settings &gt; Providers
          </span>
        </div>
      );
    }

    if (isInternal) {
      return modelGroups.map(group => (
        <div key={group.providerId} className="mb-1">
          <div className="px-3 py-1 text-[10px] uppercase tracking-wider font-semibold text-foreground-subtlest dark:text-foreground-subtle">
            {group.providerName}
          </div>
          {group.models.map(modelName => {
            const isSelected = modelName === currentModel;
            return (
              <button
                key={`${group.providerId}-${modelName}`}
                type="button"
                onClick={() => void selectModel(modelName)}
                className={`w-full text-left px-3 py-2 text-xs flex items-center justify-between transition-colors cursor-pointer ${
                  isSelected
                    ? 'bg-primary/10 dark:bg-card/60 text-[#1e40af] dark:text-[#93c5fd] font-semibold'
                    : 'text-foreground-subtle dark:text-foreground-secondary hover:bg-surface dark:hover:bg-surface-hover'
                }`}
              >
                <div className="flex items-center gap-2">
                  <Sparkles className="w-3.5 h-3.5 text-info" />
                  <span className="font-mono">{modelName}</span>
                </div>
                {isSelected && (
                  <Check className="w-4 h-4 text-primary dark:text-info shrink-0" />
                )}
              </button>
            );
          })}
        </div>
      ));
    }

    return (
      <div className="mb-1">
        <div className="px-3 py-1 text-[10px] uppercase tracking-wider font-semibold text-foreground-subtlest dark:text-foreground-subtle">
          {activeAgent.name} models
        </div>
        {externalModels.map(modelName => {
          const isSelected = modelName === currentModel;
          return (
            <button
              key={modelName}
              type="button"
              onClick={() => void selectModel(modelName)}
              className={`w-full text-left px-3 py-2 text-xs flex items-center justify-between transition-colors cursor-pointer ${
                isSelected
                  ? 'bg-primary/10 dark:bg-card/60 text-[#1e40af] dark:text-[#93c5fd] font-semibold'
                  : 'text-foreground-subtle dark:text-foreground-secondary hover:bg-surface dark:hover:bg-surface-hover'
              }`}
            >
              <div className="flex items-center gap-2">
                <Bot className="w-3.5 h-3.5 text-success" />
                <span className="font-mono">{modelName}</span>
              </div>
              {isSelected && (
                <Check className="w-4 h-4 text-primary dark:text-info shrink-0" />
              )}
            </button>
          );
        })}
      </div>
    );
  };

  return (
    <div className="relative inline-block text-left" ref={dropdownRef}>
      <button
        type="button"
        onClick={() => setIsOpen(prev => !prev)}
        aria-haspopup="menu"
        aria-expanded={isOpen}
        aria-label={`Model: ${currentModel || (availableModels.length > 0 ? availableModels[0] : 'none')}`}
        className="flex items-center gap-1.5 px-2.5 py-1 rounded-lg border border-border dark:border-border bg-white dark:bg-[#252526] hover:bg-surface dark:hover:bg-[#2e2e2e] text-xs text-foreground-subtle dark:text-foreground-secondary font-medium transition-all shadow-2xs group cursor-pointer"
      >
        <Cpu className="w-3.5 h-3.5 text-primary dark:text-info" aria-hidden="true" />
        <span className="text-foreground-subtle dark:text-foreground-subtle">Model:</span>
        <span className="font-semibold text-foreground dark:text-white truncate max-w-[120px]">
          {currentModel || (availableModels.length > 0 ? availableModels[0] : 'No model')}
        </span>
        <ChevronDown className="w-3 h-3 text-foreground-subtle group-hover:text-foreground-subtle transition-transform duration-150" />
      </button>

      {isOpen && (
        <div role="menu" aria-label="Provider models" className="absolute left-0 bottom-full mb-2 w-72 rounded-2xl bg-white dark:bg-[#222224] shadow-2xl border border-border dark:border-border py-2 z-50 animate-in fade-in zoom-in-95 duration-100 font-sans">
          <div className="px-3.5 py-1.5 border-b border-border dark:border-border flex items-center justify-between">
            <span className="text-ui-xs font-semibold text-foreground-subtlest dark:text-foreground-subtle uppercase tracking-wider">
              {isInternal ? `Provider Models (${availableModels.length})` : `Agent Models (${availableModels.length})`}
            </span>
            <span className="text-[10px] px-1.5 py-0.5 rounded bg-primary/10 text-primary dark:bg-card dark:text-[#93c5fd] font-mono truncate max-w-[110px]">
              {activeAgent?.name || 'Agent'}
            </span>
          </div>

          <div className="py-1 max-h-64 overflow-y-auto space-y-0.5">
            {renderGroups()}
          </div>
        </div>
      )}
    </div>
  );
};
