import React, { useState, useMemo, useEffect, useRef } from 'react';
import {
  X,
  Search,
  BookMarked,
  History,
  Play,
  ClipboardCopy,
  Check,
  Star,
  Trash2,
  Plus,
  Edit2,
  Terminal,
  Server,
  Container,
  GitBranch,
  Cpu,
  Code
} from 'lucide-react';
import { useTerminalSnippetStore, TerminalSnippet } from '../../stores/terminalSnippetStore';
import { cn } from '../../lib/utils';
import { showToast } from '../../lib/toast';

const CATEGORY_META: Record<string, { label: string; icon: React.FC<{ className?: string }> }> = {
  all: { label: 'All', icon: Terminal },
  remote: { label: 'Remote / SSH', icon: Server },
  docker: { label: 'Docker', icon: Container },
  system: { label: 'System', icon: Cpu },
  git: { label: 'Git', icon: GitBranch },
  dev: { label: 'Dev', icon: Code },
  custom: { label: 'Custom', icon: BookMarked }
};

export const TerminalSnippetModal: React.FC = () => {
  const {
    isSnippetModalOpen,
    snippetModalTargetSessionId,
    snippetDraftCommand,
    snippetDraftTitle,
    snippets,
    history,
    addSnippet,
    updateSnippet,
    deleteSnippet,
    toggleFavoriteHistory,
    deleteHistoryItem,
    clearHistory,
    closeSnippetModal,
    pasteToTerminal
  } = useTerminalSnippetStore();

  const [activeTab, setActiveTab] = useState<'snippets' | 'history'>('snippets');
  const [searchQuery, setSearchQuery] = useState('');
  const [selectedCategory, setSelectedCategory] = useState<string>('all');
  const [copiedId, setCopiedId] = useState<string | null>(null);

  // Form state for creating/editing snippets
  const [isFormOpen, setIsFormOpen] = useState(false);
  const [editingId, setEditingId] = useState<string | null>(null);
  const [formTitle, setFormTitle] = useState('');
  const [formCommand, setFormCommand] = useState('');
  const [formCategory, setFormCategory] = useState('custom');
  const [formDescription, setFormDescription] = useState('');

  const searchInputRef = useRef<HTMLInputElement>(null);

  // When opened with a draft command from terminal selection
  useEffect(() => {
    if (isSnippetModalOpen) {
      if (snippetDraftCommand) {
        setIsFormOpen(true);
        setEditingId(null);
        setFormTitle(snippetDraftTitle || 'New Command Snippet');
        setFormCommand(snippetDraftCommand);
        setFormCategory('custom');
        setFormDescription('Saved from terminal selection');
      } else {
        setIsFormOpen(false);
      }
      setTimeout(() => {
        searchInputRef.current?.focus();
      }, 50);
    } else {
      setSearchQuery('');
      setIsFormOpen(false);
      setEditingId(null);
    }
  }, [isSnippetModalOpen, snippetDraftCommand, snippetDraftTitle]);

  const handleCopy = async (id: string, text: string) => {
    try {
      await navigator.clipboard.writeText(text);
      setCopiedId(id);
      setTimeout(() => setCopiedId(null), 1500);
      showToast('Copied to clipboard', 'info');
    } catch {
      showToast('Failed to copy', 'error');
    }
  };

  const handlePaste = async (command: string, runImmediately: boolean) => {
    const success = await pasteToTerminal(command, runImmediately, snippetModalTargetSessionId);
    if (success) {
      closeSnippetModal();
    }
  };

  const handleStartEdit = (s: TerminalSnippet) => {
    setEditingId(s.id);
    setFormTitle(s.title);
    setFormCommand(s.command);
    setFormCategory(s.category);
    setFormDescription(s.description || '');
    setIsFormOpen(true);
  };

  const handleSaveForm = (e: React.FormEvent) => {
    e.preventDefault();
    if (!formTitle.trim() || !formCommand.trim()) {
      showToast('Title and Command are required', 'warn');
      return;
    }

    if (editingId) {
      updateSnippet(editingId, {
        title: formTitle.trim(),
        command: formCommand.trim(),
        category: formCategory.trim() || 'custom',
        description: formDescription.trim() || undefined
      });
    } else {
      addSnippet({
        title: formTitle.trim(),
        command: formCommand.trim(),
        category: formCategory.trim() || 'custom',
        description: formDescription.trim() || undefined
      });
    }

    setIsFormOpen(false);
    setEditingId(null);
  };

  const filteredSnippets = useMemo(() => {
    return snippets.filter(s => {
      const matchesCategory = selectedCategory === 'all' || s.category === selectedCategory;
      if (!matchesCategory) return false;
      if (!searchQuery.trim()) return true;
      const q = searchQuery.toLowerCase();
      return (
        s.title.toLowerCase().includes(q) ||
        s.command.toLowerCase().includes(q) ||
        (s.description && s.description.toLowerCase().includes(q)) ||
        s.category.toLowerCase().includes(q)
      );
    });
  }, [snippets, selectedCategory, searchQuery]);

  const filteredHistory = useMemo(() => {
    return history
      .filter(item => {
        if (!searchQuery.trim()) return true;
        return item.command.toLowerCase().includes(searchQuery.toLowerCase());
      })
      .sort((a, b) => {
        if (a.isFavorite && !b.isFavorite) return -1;
        if (!a.isFavorite && b.isFavorite) return 1;
        return b.timestamp - a.timestamp;
      });
  }, [history, searchQuery]);

  if (!isSnippetModalOpen) return null;

  return (
    <div
      role="dialog"
      aria-modal="true"
      aria-label="Terminal Snippets and History"
      className="fixed inset-0 z-50 flex items-center justify-center bg-black/60 backdrop-blur-xs p-4 animate-in fade-in duration-150"
      onClick={(e) => {
        if (e.target === e.currentTarget) {
          closeSnippetModal();
        }
      }}
      onKeyDown={(e) => {
        if (e.key === 'Escape') {
          e.preventDefault();
          closeSnippetModal();
        }
      }}
    >
      <div
        className="relative flex flex-col w-full max-w-2xl max-h-[85vh] rounded-xl border border-border bg-panel shadow-2xl overflow-hidden select-none"
        onClick={(e) => e.stopPropagation()}
      >
        {/* Modal Header */}
        <div className="flex shrink-0 items-center justify-between border-b border-border/60 px-4 py-3 bg-chrome/60">
          <div className="flex items-center gap-2.5">
            <div className="flex size-7 items-center justify-center rounded-lg bg-primary/10 text-primary">
              <Terminal className="size-4" />
            </div>
            <div>
              <h2 className="text-ui-base font-semibold text-fg-primary tracking-tight">
                Terminal Snippets & Command History
              </h2>
              <p className="text-ui-xs text-fg-secondary">
                Remember, paste, and run commands in your local or remote SSH terminal
              </p>
            </div>
          </div>
          <button
            type="button"
            onClick={closeSnippetModal}
            className="flex size-7 items-center justify-center rounded-md text-fg-tertiary hover:bg-surface-hover hover:text-fg-primary transition-colors cursor-pointer"
            title="Close (Esc)"
          >
            <X className="size-4" />
          </button>
        </div>

        {/* Top Control Bar: Tabs + Search + Add Button */}
        <div className="flex shrink-0 flex-col gap-2.5 border-b border-border/60 p-3 bg-surface/30">
          <div className="flex items-center justify-between gap-3">
            {/* Primary Mode Tabs */}
            <div className="flex items-center gap-1 rounded-lg bg-surface p-0.5 border border-border/40 text-ui-xs">
              <button
                type="button"
                onClick={() => { setActiveTab('snippets'); setIsFormOpen(false); }}
                className={cn(
                  'flex items-center gap-1.5 px-3 py-1 rounded-md font-medium transition-colors cursor-pointer',
                  activeTab === 'snippets'
                    ? 'bg-panel text-fg-primary shadow-xs'
                    : 'text-fg-secondary hover:text-fg-primary'
                )}
              >
                <BookMarked className="size-3.5" />
                <span>Snippets ({snippets.length})</span>
              </button>
              <button
                type="button"
                onClick={() => { setActiveTab('history'); setIsFormOpen(false); }}
                className={cn(
                  'flex items-center gap-1.5 px-3 py-1 rounded-md font-medium transition-colors cursor-pointer',
                  activeTab === 'history'
                    ? 'bg-panel text-fg-primary shadow-xs'
                    : 'text-fg-secondary hover:text-fg-primary'
                )}
              >
                <History className="size-3.5" />
                <span>Remembered History ({history.length})</span>
              </button>
            </div>

            {/* Action on right */}
            {activeTab === 'snippets' ? (
              <button
                type="button"
                onClick={() => {
                  setEditingId(null);
                  setFormTitle('');
                  setFormCommand('');
                  setFormCategory('custom');
                  setFormDescription('');
                  setIsFormOpen(prev => !prev);
                }}
                className="flex items-center gap-1.5 rounded-md bg-accent-primary px-2.5 py-1 text-ui-xs font-medium text-white hover:brightness-110 transition-colors cursor-pointer"
              >
                <Plus className="size-3.5" />
                <span>{isFormOpen ? 'Cancel' : 'New Snippet'}</span>
              </button>
            ) : history.length > 0 ? (
              <button
                type="button"
                onClick={clearHistory}
                className="flex items-center gap-1 text-ui-xs text-fg-tertiary hover:text-destructive transition-colors cursor-pointer"
                title="Clear all remembered history"
              >
                <Trash2 className="size-3.5" />
                <span>Clear History</span>
              </button>
            ) : null}
          </div>

          {/* Search Bar */}
          <div className="relative flex items-center">
            <Search className="absolute left-2.5 size-3.5 text-fg-tertiary pointer-events-none" />
            <input
              ref={searchInputRef}
              type="text"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
              placeholder={activeTab === 'snippets' ? 'Search snippets by title, command, or tag...' : 'Search recent commands...'}
              className="w-full h-8 rounded-lg bg-surface border border-border/60 pl-8 pr-7 text-ui-xs text-fg-primary placeholder:text-fg-tertiary focus:outline-hidden focus:border-primary/60"
            />
            {searchQuery && (
              <button
                type="button"
                onClick={() => setSearchQuery('')}
                className="absolute right-2 size-4 text-fg-tertiary hover:text-fg-primary cursor-pointer"
              >
                <X className="size-3" />
              </button>
            )}
          </div>

          {/* Category Filter Pills (Snippets tab only) */}
          {activeTab === 'snippets' && !isFormOpen && (
            <div className="flex items-center gap-1 overflow-x-auto scrollbar-none text-ui-xs pt-0.5">
              {Object.entries(CATEGORY_META).map(([key, meta]) => {
                const isSelected = selectedCategory === key;
                const IconComponent = meta.icon;
                return (
                  <button
                    key={key}
                    type="button"
                    onClick={() => setSelectedCategory(key)}
                    className={cn(
                      'flex items-center gap-1 px-2.5 py-0.5 rounded-full transition-colors cursor-pointer shrink-0',
                      isSelected
                        ? 'bg-foreground/15 text-foreground font-medium'
                        : 'text-fg-secondary hover:bg-foreground/5 hover:text-fg-primary'
                    )}
                  >
                    <IconComponent className="size-3" />
                    <span>{meta.label}</span>
                  </button>
                );
              })}
            </div>
          )}
        </div>

        {/* Modal Body */}
        <div className="flex-1 overflow-y-auto p-3 min-h-72 max-h-[55vh]">
          {/* Add / Edit Form */}
          {isFormOpen ? (
            <form onSubmit={handleSaveForm} className="flex flex-col gap-3 rounded-lg border border-border/80 bg-surface/50 p-4">
              <div className="flex items-center justify-between pb-1 border-b border-border/40">
                <span className="font-semibold text-ui-sm text-fg-primary">
                  {editingId ? 'Edit Snippet' : 'Create New Snippet'}
                </span>
                <span className="text-ui-xs text-fg-tertiary">Saves to local workspace storage</span>
              </div>

              <div className="grid grid-cols-1 sm:grid-cols-2 gap-3">
                <div className="flex flex-col gap-1">
                  <label className="text-ui-xs font-medium text-fg-secondary">Title / Label *</label>
                  <input
                    type="text"
                    required
                    value={formTitle}
                    onChange={(e) => setFormTitle(e.target.value)}
                    placeholder="e.g. Restart API Server"
                    className="h-8 rounded-md bg-panel border border-border/60 px-2.5 text-ui-xs text-fg-primary focus:outline-hidden focus:border-primary/60"
                  />
                </div>
                <div className="flex flex-col gap-1">
                  <label className="text-ui-xs font-medium text-fg-secondary">Category</label>
                  <select
                    value={formCategory}
                    onChange={(e) => setFormCategory(e.target.value)}
                    className="h-8 rounded-md bg-panel border border-border/60 px-2 text-ui-xs text-fg-primary focus:outline-hidden focus:border-primary/60 cursor-pointer"
                  >
                    <option value="remote">Remote / SSH</option>
                    <option value="docker">Docker</option>
                    <option value="system">System</option>
                    <option value="git">Git</option>
                    <option value="dev">Development</option>
                    <option value="custom">Custom</option>
                  </select>
                </div>
              </div>

              <div className="flex flex-col gap-1">
                <label className="text-ui-xs font-medium text-fg-secondary">Command / Script *</label>
                <textarea
                  required
                  rows={3}
                  value={formCommand}
                  onChange={(e) => setFormCommand(e.target.value)}
                  placeholder="e.g. systemctl restart api && journalctl -u api -f"
                  className="rounded-md bg-panel border border-border/60 p-2 font-mono text-ui-xs text-fg-primary focus:outline-hidden focus:border-primary/60 resize-y"
                />
              </div>

              <div className="flex flex-col gap-1">
                <label className="text-ui-xs font-medium text-fg-secondary">Description (Optional)</label>
                <input
                  type="text"
                  value={formDescription}
                  onChange={(e) => setFormDescription(e.target.value)}
                  placeholder="Brief note on what this command does..."
                  className="h-8 rounded-md bg-panel border border-border/60 px-2.5 text-ui-xs text-fg-primary focus:outline-hidden focus:border-primary/60"
                />
              </div>

              <div className="flex items-center justify-end gap-2 pt-2 border-t border-border/40">
                <button
                  type="button"
                  onClick={() => setIsFormOpen(false)}
                  className="px-3 py-1.5 rounded-md border border-border/60 text-ui-xs text-fg-secondary hover:bg-surface-hover hover:text-fg-primary transition-colors cursor-pointer"
                >
                  Cancel
                </button>
                <button
                  type="submit"
                  className="px-3 py-1.5 rounded-md bg-accent-primary text-ui-xs font-medium text-white hover:brightness-110 transition-colors cursor-pointer"
                >
                  {editingId ? 'Save Changes' : 'Create Snippet'}
                </button>
              </div>
            </form>
          ) : activeTab === 'snippets' ? (
            /* Snippets List */
            <div className="flex flex-col gap-2">
              {filteredSnippets.length === 0 ? (
                <div className="flex flex-col items-center justify-center p-8 text-center text-fg-tertiary">
                  <BookMarked className="size-8 opacity-40 mb-2" />
                  <p className="text-ui-sm font-medium">No snippets match your filter</p>
                  <p className="text-ui-xs mt-1">Try another search term or create a new snippet</p>
                </div>
              ) : (
                filteredSnippets.map(snippet => (
                  <div
                    key={snippet.id}
                    className="group relative flex flex-col gap-1.5 rounded-lg border border-border/60 bg-surface/40 p-2.5 hover:border-border transition-all hover:bg-surface/70"
                  >
                    <div className="flex items-center justify-between gap-2">
                      <div className="flex items-center gap-2 min-w-0">
                        <span className="font-semibold text-ui-xs text-fg-primary truncate">
                          {snippet.title}
                        </span>
                        <span className="shrink-0 rounded-full bg-surface-hover px-1.5 py-0.2 text-[10px] font-medium text-fg-tertiary uppercase border border-border/40">
                          {snippet.category}
                        </span>
                      </div>

                      {/* Action buttons on card header */}
                      <div className="flex items-center gap-1 shrink-0">
                        <button
                          type="button"
                          onClick={() => handleCopy(snippet.id, snippet.command)}
                          className="flex size-6 items-center justify-center rounded text-fg-tertiary hover:bg-surface hover:text-fg-primary transition-colors cursor-pointer"
                          title="Copy command to clipboard"
                        >
                          {copiedId === snippet.id ? (
                            <Check className="size-3 text-success" />
                          ) : (
                            <ClipboardCopy className="size-3" />
                          )}
                        </button>

                        {!snippet.isBuiltin && (
                          <>
                            <button
                              type="button"
                              onClick={() => handleStartEdit(snippet)}
                              className="flex size-6 items-center justify-center rounded text-fg-tertiary hover:bg-surface hover:text-fg-primary transition-colors cursor-pointer"
                              title="Edit snippet"
                            >
                              <Edit2 className="size-3" />
                            </button>
                            <button
                              type="button"
                              onClick={() => deleteSnippet(snippet.id)}
                              className="flex size-6 items-center justify-center rounded text-fg-tertiary hover:bg-destructive/10 hover:text-destructive transition-colors cursor-pointer"
                              title="Delete snippet"
                            >
                              <Trash2 className="size-3" />
                            </button>
                          </>
                        )}

                        {/* Paste without Enter */}
                        <button
                          type="button"
                          onClick={() => handlePaste(snippet.command, false)}
                          className="flex items-center gap-1 rounded bg-surface hover:bg-surface-hover border border-border/60 px-2 py-0.5 text-ui-xs font-medium text-fg-primary transition-colors cursor-pointer"
                          title="Paste into terminal without running"
                        >
                          <span>Paste</span>
                        </button>

                        {/* Paste and Run with Enter */}
                        <button
                          type="button"
                          onClick={() => handlePaste(snippet.command, true)}
                          className="flex items-center gap-1 rounded bg-accent-primary hover:brightness-110 px-2 py-0.5 text-ui-xs font-medium text-white transition-all cursor-pointer shadow-2xs"
                          title="Paste and immediately execute (↵ Enter)"
                        >
                          <Play className="size-2.5 fill-current" />
                          <span>Run ↵</span>
                        </button>
                      </div>
                    </div>

                    {snippet.description && (
                      <p className="text-ui-xs text-fg-tertiary leading-normal line-clamp-2">
                        {snippet.description}
                      </p>
                    )}

                    {/* Command Code Preview */}
                    <div className="mt-0.5 rounded bg-app border border-border/40 p-2 font-mono text-ui-xs text-fg-secondary break-all select-text">
                      {snippet.command}
                    </div>
                  </div>
                ))
              )}
            </div>
          ) : (
            /* History List */
            <div className="flex flex-col gap-1.5">
              {filteredHistory.length === 0 ? (
                <div className="flex flex-col items-center justify-center p-8 text-center text-fg-tertiary">
                  <History className="size-8 opacity-40 mb-2" />
                  <p className="text-ui-sm font-medium">No remembered commands yet</p>
                  <p className="text-ui-xs mt-1">Commands pasted or run in terminal are recorded here</p>
                </div>
              ) : (
                filteredHistory.map(item => (
                  <div
                    key={item.id}
                    className="group flex items-center justify-between gap-3 rounded-lg border border-border/40 bg-surface/30 px-3 py-2 hover:bg-surface/70 hover:border-border transition-colors"
                  >
                    <div className="flex items-center gap-2 min-w-0 flex-1">
                      <button
                        type="button"
                        onClick={() => toggleFavoriteHistory(item.id)}
                        className={cn(
                          'flex size-5 shrink-0 items-center justify-center rounded cursor-pointer transition-colors',
                          item.isFavorite
                            ? 'text-amber-500 hover:text-amber-400'
                            : 'text-fg-disabled hover:text-fg-tertiary'
                        )}
                        title={item.isFavorite ? 'Remove from favorites' : 'Star favorite command'}
                      >
                        <Star className={cn('size-3.5', item.isFavorite && 'fill-current')} />
                      </button>

                      <div className="min-w-0 flex-1">
                        <div className="font-mono text-ui-xs text-fg-primary truncate select-text">
                          {item.command}
                        </div>
                        <div className="text-[10px] text-fg-tertiary flex items-center gap-2 mt-0.5">
                          <span>{new Date(item.timestamp).toLocaleTimeString([], { hour: '2-digit', minute: '2-digit' })}</span>
                          {item.source && (
                            <>
                              <span>•</span>
                              <span className="capitalize">{item.source}</span>
                            </>
                          )}
                        </div>
                      </div>
                    </div>

                    <div className="flex items-center gap-1 shrink-0">
                      <button
                        type="button"
                        onClick={() => handleCopy(item.id, item.command)}
                        className="flex size-6 items-center justify-center rounded text-fg-tertiary hover:bg-surface hover:text-fg-primary transition-colors cursor-pointer"
                        title="Copy command"
                      >
                        {copiedId === item.id ? (
                          <Check className="size-3 text-success" />
                        ) : (
                          <ClipboardCopy className="size-3" />
                        )}
                      </button>

                      <button
                        type="button"
                        onClick={() => deleteHistoryItem(item.id)}
                        className="flex size-6 items-center justify-center rounded text-fg-tertiary hover:bg-destructive/10 hover:text-destructive transition-colors cursor-pointer"
                        title="Remove from history"
                      >
                        <Trash2 className="size-3" />
                      </button>

                      <button
                        type="button"
                        onClick={() => handlePaste(item.command, false)}
                        className="flex items-center gap-1 rounded bg-surface hover:bg-surface-hover border border-border/60 px-2 py-0.5 text-ui-xs font-medium text-fg-primary transition-colors cursor-pointer"
                        title="Paste into terminal"
                      >
                        <span>Paste</span>
                      </button>

                      <button
                        type="button"
                        onClick={() => handlePaste(item.command, true)}
                        className="flex items-center gap-1 rounded bg-accent-primary hover:brightness-110 px-2 py-0.5 text-ui-xs font-medium text-white transition-all cursor-pointer shadow-2xs"
                        title="Paste and immediately execute (↵ Enter)"
                      >
                        <Play className="size-2.5 fill-current" />
                        <span>Run ↵</span>
                      </button>
                    </div>
                  </div>
                ))
              )}
            </div>
          )}
        </div>

        {/* Modal Footer */}
        <div className="flex shrink-0 items-center justify-between border-t border-border/60 px-4 py-2 bg-chrome/40 text-ui-xs text-fg-tertiary">
          <div className="flex items-center gap-3">
            <span><kbd className="px-1 rounded bg-surface border border-border/40 font-mono">↵</kbd> Run immediately</span>
            <span><kbd className="px-1 rounded bg-surface border border-border/40 font-mono">Paste</kbd> Insert at cursor</span>
            <span><kbd className="px-1 rounded bg-surface border border-border/40 font-mono">Esc</kbd> Close</span>
          </div>
          <div>
            <span>Target session: <code className="font-mono text-fg-secondary">{snippetModalTargetSessionId || 'Active terminal'}</code></span>
          </div>
        </div>
      </div>
    </div>
  );
};

export default TerminalSnippetModal;
