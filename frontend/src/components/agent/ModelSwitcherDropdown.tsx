import React, { useState, useRef, useEffect } from 'react';
import { ChevronDown, Check, Sparkles, Cpu, Plus } from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';

export const ModelSwitcherDropdown: React.FC<{ compact?: boolean }> = ({ compact = false }) => {
  const { currentModel, setCurrentModel, activeAgent, providers } = useWorkspace();
  const [isOpen, setIsOpen] = useState(false);
  const dropdownRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handleClickOutside = (e: MouseEvent) => {
      if (dropdownRef.current && !dropdownRef.current.contains(e.target as Node)) {
        setIsOpen(false);
      }
    };
    document.addEventListener('mousedown', handleClickOutside);
    return () => document.removeEventListener('mousedown', handleClickOutside);
  }, []);

  // Strictly collect models from enabled providers (selectedModels)
  const availableModels = React.useMemo(() => {
    const list = new Set<string>();
    if (providers) {
      providers.filter(p => p.enabled).forEach(p => {
        const activeList = p.selectedModels && p.selectedModels.length > 0 ? p.selectedModels : p.models;
        (activeList || []).forEach(m => list.add(m));
      });
    }
    return Array.from(list);
  }, [providers]);

  return (
    <div className="relative inline-block text-left" ref={dropdownRef}>
      <button
        type="button"
        onClick={() => setIsOpen(prev => !prev)}
        className="flex items-center gap-1.5 px-2.5 py-1 rounded-lg border border-[#e5e7eb] dark:border-[#383838] bg-white dark:bg-[#252526] hover:bg-[#f9fafb] dark:hover:bg-[#2e2e2e] text-xs text-[#374151] dark:text-[#d1d5db] font-medium transition-all shadow-2xs group cursor-pointer"
      >
        <Cpu className="w-3.5 h-3.5 text-[#2563eb] dark:text-[#38bdf8]" />
        <span className="text-[#6b7280] dark:text-[#9ca3af]">Model:</span>
        <span className="font-semibold text-[#111827] dark:text-white truncate max-w-[120px]">
          {currentModel || (availableModels.length > 0 ? availableModels[0] : 'No model')}
        </span>
        <ChevronDown className="w-3 h-3 text-[#9ca3af] group-hover:text-[#4b5563] transition-transform duration-150" />
      </button>

      {isOpen && (
        <div className="absolute left-0 bottom-full mb-2 w-72 rounded-2xl bg-white dark:bg-[#222224] shadow-2xl border border-[#e5e7eb] dark:border-[#383838] py-2 z-50 animate-in fade-in zoom-in-95 duration-100 font-sans">
          <div className="px-3.5 py-1.5 border-b border-[#f3f4f6] dark:border-[#2f2f31] flex items-center justify-between">
            <span className="text-[11px] font-semibold text-[#64748b] dark:text-[#9ca3af] uppercase tracking-wider">
              Provider Models ({availableModels.length})
            </span>
            <span className="text-[10px] px-1.5 py-0.5 rounded bg-[#eff6ff] text-[#2563eb] dark:bg-[#1e293b] dark:text-[#93c5fd] font-mono">
              {activeAgent.name}
            </span>
          </div>

          <div className="py-1 max-h-64 overflow-y-auto space-y-0.5">
            {availableModels.length === 0 ? (
              <div className="px-3 py-3 text-center text-xs text-[#6b7280] dark:text-[#9ca3af]">
                No models fetched or selected.<br/>
                <span className="text-[11px] text-[#2563eb] dark:text-[#60a5fa] mt-1 block">
                  Configure in Settings &gt; Providers
                </span>
              </div>
            ) : (
              availableModels.map(modelName => {
                const isSelected = modelName === currentModel;
                return (
                  <button
                    key={modelName}
                    type="button"
                    onClick={() => {
                      setCurrentModel(modelName);
                      setIsOpen(false);
                    }}
                    className={`w-full text-left px-3 py-2 text-xs flex items-center justify-between transition-colors cursor-pointer ${
                      isSelected
                        ? 'bg-[#eff6ff] dark:bg-[#1e293b]/60 text-[#1e40af] dark:text-[#93c5fd] font-semibold'
                        : 'text-[#374151] dark:text-[#d1d5db] hover:bg-[#f9fafb] dark:hover:bg-[#2a2a2c]'
                    }`}
                  >
                    <div className="flex items-center gap-2">
                      <Sparkles className="w-3.5 h-3.5 text-[#3b82f6]" />
                      <span className="font-mono">{modelName}</span>
                    </div>
                    {isSelected && (
                      <Check className="w-4 h-4 text-[#2563eb] dark:text-[#60a5fa] shrink-0" />
                    )}
                  </button>
                );
              })
            )}
          </div>
        </div>
      )}
    </div>
  );
};
