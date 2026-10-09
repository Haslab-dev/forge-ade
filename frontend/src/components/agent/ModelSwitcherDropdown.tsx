import React, { useState, useRef, useEffect } from 'react';
import { ChevronDown, Check, Sparkles, Cpu } from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';
import { SetActiveModel } from '../../lib/wails';

interface ModelGroup {
  providerId: string;
  providerName: string;
  models: string[];
}

/**
 * Model picker for the internal ForgeADE agent. Models are grouped by the
 * enabled providers configured in Settings — picking a model also picks the
 * provider (and its API key). The old external (ACP) agent branch is gone:
 * the Agent UI is internal-only.
 */
export const ModelSwitcherDropdown: React.FC<{ compact?: boolean }> = ({ compact = false }) => {
  const { currentModel, setCurrentModel, providers, activeAgent } = useWorkspace();
  const [isOpen, setIsOpen] = useState(false);
  const dropdownRef = useRef<HTMLDivElement>(null);

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

  // Internal agents list models grouped by the enabled providers they come
  // from, so picking a model also picks the provider (and its API key).
  const modelGroups = React.useMemo<ModelGroup[]>(() => {
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
  }, [providers]);

  const availableModels = React.useMemo(
    () => modelGroups.flatMap(g => g.models),
    [modelGroups]
  );

  const findGroupFor = (model: string): ModelGroup | undefined =>
    modelGroups.find(g => g.models.includes(model));

  const selectModel = async (modelName: string) => {
    setCurrentModel(modelName);
    setIsOpen(false);
    const group = findGroupFor(modelName);
    if (group) {
      try { await SetActiveModel(group.providerId, modelName); } catch { /* ignore */ }
    }
  };

  const renderGroups = () => {
    if (availableModels.length === 0) {
      return (
        <div className="px-3 py-3 text-center text-xs text-foreground-subtle">
          No models fetched or selected.<br/>
          <span className="text-ui-xs text-primary dark:text-info mt-1 block">
            Configure in Settings &gt; Providers
          </span>
        </div>
      );
    }

    return modelGroups.map(group => (
      <div key={group.providerId} className="mb-1">
        <div className="px-3 py-1 text-ui-xs uppercase tracking-wider font-semibold text-foreground-subtlest dark:text-foreground-subtle">
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
                  ? 'bg-primary/10 dark:bg-card/60 text-link font-semibold'
                  : 'text-foreground-subtle hover:bg-surface dark:hover:bg-surface-hover'
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
  };

  return (
    <div className="relative inline-block text-left" ref={dropdownRef}>
      <button
        type="button"
        onClick={() => setIsOpen(prev => !prev)}
        aria-haspopup="menu"
        aria-expanded={isOpen}
        aria-label={`Model: ${currentModel || (availableModels.length > 0 ? availableModels[0] : 'none')}`}
        className="flex items-center gap-1.5 px-2.5 py-1 rounded-lg border border-border bg-surface hover:bg-surface-hover text-xs text-foreground-subtle font-medium transition-all shadow-2xs group cursor-pointer"
      >
        <Cpu className="w-3.5 h-3.5 text-primary dark:text-info" aria-hidden="true" />
        <span className="text-foreground-subtle">Model:</span>
        <span className="font-semibold text-foreground truncate max-w-[120px]">
          {currentModel || (availableModels.length > 0 ? availableModels[0] : 'No model')}
        </span>
        <ChevronDown className="w-3 h-3 text-foreground-subtle group-hover:text-foreground-subtle transition-transform duration-150" />
      </button>

      {isOpen && (
        <div role="menu" aria-label="Provider models" className="absolute left-0 bottom-full mb-2 w-72 rounded-2xl bg-popover shadow-2xl border border-border py-2 z-50 animate-in fade-in zoom-in-95 duration-100 font-sans">
          <div className="px-3.5 py-1.5 border-b border-border flex items-center justify-between">
            <span className="text-ui-xs font-semibold text-foreground-subtlest dark:text-foreground-subtle uppercase tracking-wider">
              Provider Models ({availableModels.length})
            </span>
            <span className="text-ui-xs px-1.5 py-0.5 rounded bg-primary/10 text-primary dark:bg-card text-link font-mono truncate max-w-[110px]">
              {activeAgent?.name || 'ForgeADE'}
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

export default ModelSwitcherDropdown;
