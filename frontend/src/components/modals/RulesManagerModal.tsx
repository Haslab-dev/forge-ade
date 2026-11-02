import React, { useState, useEffect, useCallback } from 'react';
import { FileText, Check, ArrowRightLeft, Save, X } from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';
import { GetWorkspaceRules, SaveWorkspaceRule, SyncWorkspaceRule, WorkspaceRuleFile } from '../../lib/wails';
import { showToast } from '../../lib/toast';

interface RulesManagerModalProps {
  isOpen: boolean;
  onClose: () => void;
}

export const RulesManagerModal: React.FC<RulesManagerModalProps> = ({ isOpen, onClose }) => {
  const { activeWorkspacePath } = useWorkspace();
  const [rules, setRules] = useState<WorkspaceRuleFile[]>([]);
  const [selectedFile, setSelectedFile] = useState<string>('AGENTS.md');
  const [content, setContent] = useState<string>('');
  const [isSaving, setIsSaving] = useState(false);

  const loadRules = useCallback(async () => {
    try {
      const targetPath = activeWorkspacePath || '.';
      const res = await GetWorkspaceRules(targetPath);
      setRules(res?.activeRules || []);
      const current = (res?.activeRules || []).find(r => r.path === selectedFile);
      if (current) {
        setContent(current.content || '');
      }
    } catch {
      // ignore
    }
  }, [activeWorkspacePath, selectedFile]);

  useEffect(() => {
    if (isOpen) {
      void loadRules();
    }
  }, [isOpen, loadRules]);

  const handleSelectRule = (path: string) => {
    setSelectedFile(path);
    const found = rules.find(r => r.path === path);
    setContent(found?.content || '');
  };

  const handleSave = async () => {
    if (!activeWorkspacePath || !selectedFile) return;
    setIsSaving(true);
    try {
      await SaveWorkspaceRule(activeWorkspacePath, selectedFile, content);
      showToast(`Saved ${selectedFile}`, 'success');
      await loadRules();
    } catch (err: any) {
      showToast(err?.message || `Failed to save ${selectedFile}`, 'error');
    } finally {
      setIsSaving(false);
    }
  };

  const handleSync = async (sourcePath: string, targetPath: string) => {
    if (!activeWorkspacePath) return;
    try {
      await SyncWorkspaceRule(activeWorkspacePath, sourcePath, targetPath);
      showToast(`Synced ${sourcePath} -> ${targetPath}`, 'success');
      await loadRules();
      if (selectedFile === targetPath) {
        const found = rules.find(r => r.path === targetPath);
        setContent(found?.content || '');
      }
    } catch (err: any) {
      showToast(err?.message || `Failed to sync rules`, 'error');
    }
  };

  if (!isOpen) return null;

  const currentRule = rules.find(r => r.path === selectedFile);

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-labelledby="rules-modal-title"
      className="fixed inset-0 z-50 flex items-center justify-center bg-background/80 backdrop-blur-xs p-4"
    >
      <div className="flex h-[80vh] w-full max-w-4xl flex-col rounded-xl border border-border bg-card shadow-2xl overflow-hidden animate-in fade-in zoom-in-95 duration-150">
        {/* Header */}
        <div className="flex items-center justify-between border-b border-border px-5 py-3.5 bg-sidebar/50">
          <div className="flex items-center gap-2.5">
            <div className="flex size-8 items-center justify-center rounded-lg bg-primary/10 text-primary">
              <FileText className="size-4" />
            </div>
            <div>
              <h2 id="rules-modal-title" className="text-ui-base font-semibold text-foreground">
                Unified Project Rules Manager
              </h2>
              <p className="text-ui-xs text-foreground-subtle">
                Manage and sync instruction standards (AGENTS.md, .cursorrules, CLAUDE.md)
              </p>
            </div>
          </div>
          <button
            type="button"
            onClick={onClose}
            className="rounded-md p-1.5 text-foreground-subtle hover:bg-surface-hover hover:text-foreground transition-colors cursor-pointer"
          >
            <X className="size-4" />
          </button>
        </div>

        {/* Content Body */}
        <div className="grid flex-1 grid-cols-1 md:grid-cols-[260px_minmax(0,1fr)] overflow-hidden">
          {/* Left: Rule Files List */}
          <div className="flex flex-col gap-1 border-r border-border p-3 bg-surface/30 overflow-y-auto">
            <span className="px-2 pb-1 text-ui-xs font-medium text-foreground-subtle uppercase tracking-wider">
              Detected Standards
            </span>
            {rules.map((rule) => {
              const isSelected = selectedFile === rule.path;
              return (
                <button
                  key={rule.path}
                  type="button"
                  onClick={() => handleSelectRule(rule.path)}
                  className={`flex w-full items-center justify-between rounded-lg px-2.5 py-2 text-left text-ui-xs transition-colors cursor-pointer ${
                    isSelected
                      ? 'bg-primary/10 text-primary font-medium border border-primary/20'
                      : 'text-foreground hover:bg-surface-hover border border-transparent'
                  }`}
                >
                  <div className="flex items-center gap-2 truncate">
                    <FileText className="size-3.5 shrink-0 text-foreground-subtle" />
                    <span className="truncate">{rule.path}</span>
                  </div>
                  {rule.exists ? (
                    <span className="shrink-0 flex items-center gap-1 rounded bg-success/10 px-1.5 py-0.5 text-[10px] text-success font-medium">
                      <Check className="size-2.5" />
                      Active
                    </span>
                  ) : (
                    <span className="shrink-0 text-[10px] text-foreground-subtlest">
                      Missing
                    </span>
                  )}
                </button>
              );
            })}

            {/* Quick sync options */}
            <div className="mt-4 pt-3 border-t border-border">
              <span className="px-2 pb-1 text-ui-xs font-medium text-foreground-subtle uppercase tracking-wider block">
                Quick Sync
              </span>
              <button
                type="button"
                onClick={() => handleSync('AGENTS.md', '.cursorrules')}
                className="mt-1 flex w-full items-center gap-1.5 rounded px-2 py-1.5 text-left text-ui-xs text-foreground hover:bg-surface-hover cursor-pointer"
                title="Sync AGENTS.md to .cursorrules"
              >
                <ArrowRightLeft className="size-3 text-foreground-subtle" />
                <span>AGENTS.md ➔ .cursorrules</span>
              </button>
              <button
                type="button"
                onClick={() => handleSync('AGENTS.md', 'CLAUDE.md')}
                className="flex w-full items-center gap-1.5 rounded px-2 py-1.5 text-left text-ui-xs text-foreground hover:bg-surface-hover cursor-pointer"
                title="Sync AGENTS.md to CLAUDE.md"
              >
                <ArrowRightLeft className="size-3 text-foreground-subtle" />
                <span>AGENTS.md ➔ CLAUDE.md</span>
              </button>
            </div>
          </div>

          {/* Right: Rule Editor */}
          <div className="flex flex-col h-full overflow-hidden bg-background">
            <div className="flex items-center justify-between border-b border-border px-4 py-2 bg-surface/40">
              <span className="font-mono text-ui-xs text-foreground font-medium">
                {selectedFile} {!currentRule?.exists && <span className="text-foreground-subtlest">(New)</span>}
              </span>
              <button
                type="button"
                disabled={isSaving}
                onClick={handleSave}
                className="flex items-center gap-1.5 rounded-md bg-primary px-3 py-1 text-ui-xs font-medium text-primary-foreground hover:bg-primary/90 transition-colors disabled:opacity-50 cursor-pointer shadow-xs"
              >
                <Save className="size-3" />
                <span>{isSaving ? 'Saving...' : 'Save Rule'}</span>
              </button>
            </div>

            <textarea
              value={content}
              onChange={(e) => setContent(e.target.value)}
              placeholder={`# Rules for ${selectedFile}\n\nEnter guidelines, tech stack constraints, and coding standards here...`}
              className="flex-1 w-full resize-none p-4 font-mono text-ui-xs text-foreground bg-transparent outline-none focus:ring-0 leading-relaxed"
            />
          </div>
        </div>
      </div>
    </div>
  );
};

export default RulesManagerModal;
