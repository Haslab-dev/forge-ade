import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import {
  Search,
  Replace,
  ChevronDown,
  ChevronRight,
  X,
  RefreshCw,
  FolderTree,
  ChevronsDownUp,
  Ban,
  ArrowRight,
  Filter,
  FileCode,
  Loader2,
  ExternalLink,
} from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';
import { ApiBridge } from '../../services/apiBridge';
import { getFileIcon } from '../../lib/file-icons';
import { isMac, isWindows } from '../../lib/utils';
import { showToast } from '../../lib/toast';

export interface SearchMatchItem {
  id: string;
  line: number;
  text: string;
  match: string;
}

export interface SearchFileGroup {
  file: string;
  filePath: string;
  count: number;
  matches: SearchMatchItem[];
}

/** Matches glob patterns like *.ts or src/** */
function matchesGlob(path: string, pattern: string): boolean {
  const p = pattern.trim();
  if (!p) return true;
  // Convert basic glob into regex
  const regexStr = '^' + p
    .replace(/[.+^${}()|[\]\\]/g, '\\$&')
    .replace(/\*\*/g, '.*')
    .replace(/\*/g, '[^/]*')
    .replace(/\?/g, '.') + '$';
  try {
    const re = new RegExp(regexStr, 'i');
    return re.test(path) || re.test(path.split('/').pop() || '');
  } catch {
    return path.toLowerCase().includes(p.toLowerCase());
  }
}

export const ProjectSearchModal: React.FC = () => {
  const {
    mode,
    switchSurface,
    isProjectSearchOpen,
    setIsProjectSearchOpen,
    projectSearchInitialQuery,
    openFileInEditor,
    activeWorkspacePath,
    refreshFiles,
  } = useWorkspace();

  const [query, setQuery] = useState('');
  const [replaceQuery, setReplaceQuery] = useState('');
  const [showReplace, setShowReplace] = useState(false);
  const [showFilters, setShowFilters] = useState(false);
  const [includeFilter, setIncludeFilter] = useState('');
  const [excludeFilter, setExcludeFilter] = useState('');

  const [matchCase, setMatchCase] = useState(false);
  const [matchWholeWord, setMatchWholeWord] = useState(false);
  const [useRegex, setUseRegex] = useState(false);
  const [preserveCase, setPreserveCase] = useState(false);

  const [isSearching, setIsSearching] = useState(false);
  const [searchResults, setSearchResults] = useState<SearchFileGroup[]>([]);
  const [expandedFiles, setExpandedFiles] = useState<Record<string, boolean>>({});
  const [searchDurationMs, setSearchDurationMs] = useState<number | null>(null);
  const [searchError, setSearchError] = useState<string | null>(null);

  // Keyboard navigation through flat list of matches
  const [selectedFlatIndex, setSelectedFlatIndex] = useState(0);

  const searchInputRef = useRef<HTMLInputElement>(null);
  const resultsContainerRef = useRef<HTMLDivElement>(null);

  // Sync initial query when opened
  useEffect(() => {
    if (isProjectSearchOpen) {
      if (projectSearchInitialQuery !== undefined && projectSearchInitialQuery !== '') {
        setQuery(projectSearchInitialQuery);
      }
      setSelectedFlatIndex(0);
      requestAnimationFrame(() => {
        searchInputRef.current?.focus();
        searchInputRef.current?.select();
      });
    }
  }, [isProjectSearchOpen, projectSearchInitialQuery]);

  // Execute project search
  const executeSearch = useCallback(async (searchQuery: string) => {
    const q = searchQuery.trim();
    if (!q) {
      setSearchResults([]);
      setSearchError(null);
      setSearchDurationMs(null);
      return;
    }

    setIsSearching(true);
    setSearchError(null);
    const startTime = performance.now();

    try {
      const [contentHits, nameHits] = await Promise.all([
        ApiBridge.searchContent({
          query: q,
          caseSensitive: matchCase,
          wholeWord: matchWholeWord,
          isRegex: useRegex,
          maxResults: 1500,
        }),
        ApiBridge.searchFilename(q, 300).catch(() => [] as string[]),
      ]);

      const grouped: Record<string, SearchMatchItem[]> = {};
      for (const res of contentHits) {
        const filePath = res.path;
        if (!filePath) continue;
        if (!grouped[filePath]) grouped[filePath] = [];
        grouped[filePath].push({
          id: `${filePath}:${res.line}:${grouped[filePath].length}`,
          line: res.line,
          text: res.text,
          match: q,
        });
      }

      // Add filename-only matches
      for (const filePath of nameHits) {
        if (!filePath || grouped[filePath]) continue;
        const base = filePath.split('/').pop() || filePath;
        const hit = matchCase ? base.includes(q) : base.toLowerCase().includes(q.toLowerCase());
        if (hit) {
          grouped[filePath] = [];
        }
      }

      // Apply include / exclude glob filters if specified
      let filteredEntries = Object.entries(grouped);
      if (includeFilter.trim()) {
        const patterns = includeFilter.split(',').map(s => s.trim()).filter(Boolean);
        filteredEntries = filteredEntries.filter(([p]) =>
          patterns.some(pattern => matchesGlob(p, pattern))
        );
      }
      if (excludeFilter.trim()) {
        const patterns = excludeFilter.split(',').map(s => s.trim()).filter(Boolean);
        filteredEntries = filteredEntries.filter(([p]) =>
          !patterns.some(pattern => matchesGlob(p, pattern))
        );
      }

      const groups: SearchFileGroup[] = filteredEntries.map(([filePath, matches]) => ({
        file: filePath.split('/').pop() || filePath,
        filePath,
        count: matches.length || 1,
        matches,
      }));

      // Sort: files with most content hits first, then alphabetically
      groups.sort((a, b) => b.matches.length - a.matches.length || a.filePath.localeCompare(b.filePath));

      const autoExpanded: Record<string, boolean> = {};
      groups.forEach(g => { autoExpanded[g.filePath] = true; });

      setSearchResults(groups);
      setExpandedFiles(autoExpanded);
      setSelectedFlatIndex(0);
      setSearchDurationMs(Math.round(performance.now() - startTime));
    } catch (err: unknown) {
      setSearchResults([]);
      const msg = err instanceof Error ? err.message : String(err);
      setSearchError(msg ? `Search failed: ${msg}` : 'Search failed — backend unavailable');
    } finally {
      setIsSearching(false);
    }
  }, [matchCase, matchWholeWord, useRegex, includeFilter, excludeFilter]);

  // Debounced search on query / option changes
  useEffect(() => {
    if (!isProjectSearchOpen) return;
    const timer = setTimeout(() => {
      void executeSearch(query);
    }, 200);
    return () => clearTimeout(timer);
  }, [query, isProjectSearchOpen, executeSearch]);

  // Flattened matches for keyboard navigation
  const flatMatches = useMemo(() => {
    const list: Array<{ filePath: string; line: number; matchId: string }> = [];
    searchResults.forEach(group => {
      if (expandedFiles[group.filePath] !== false) {
        if (group.matches.length > 0) {
          group.matches.forEach(m => {
            list.push({ filePath: group.filePath, line: m.line, matchId: m.id });
          });
        } else {
          list.push({ filePath: group.filePath, line: 1, matchId: group.filePath });
        }
      }
    });
    return list;
  }, [searchResults, expandedFiles]);

  const totalMatchesCount = useMemo(() => {
    return searchResults.reduce((acc, curr) => acc + curr.matches.length, 0);
  }, [searchResults]);

  const handleOpenResult = useCallback((filePath: string, line: number, forceEditor = false) => {
    setIsProjectSearchOpen(false);
    if (forceEditor && mode === 'agent') {
      switchSurface('editor');
    }
    void openFileInEditor(filePath, line);
  }, [setIsProjectSearchOpen, openFileInEditor, mode, switchSurface]);

  // Replace Single Match
  const handleReplaceSingleMatch = async (filePath: string, matchId: string, line: number) => {
    try {
      const fileContent = await ApiBridge.readFile(filePath);
      if (!fileContent) return;
      const lines = fileContent.split('\n');
      if (lines.length >= line) {
        lines[line - 1] = lines[line - 1].replace(query, replaceQuery);
        const newContent = lines.join('\n');
        await ApiBridge.writeFile(filePath, newContent);
        setSearchResults(prev => prev.map(group => {
          if (group.filePath === filePath) {
            const filtered = group.matches.filter(m => m.id !== matchId);
            return { ...group, matches: filtered, count: filtered.length };
          }
          return group;
        }).filter(g => g.count > 0));
        showToast(`Replaced 1 occurrence in ${filePath.split('/').pop()}`, 'success');
      }
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      showToast(`Replace failed: ${msg}`, 'error');
    }
  };

  // Replace All in Project
  const handleReplaceAll = async () => {
    if (!query) return;
    try {
      const res = await ApiBridge.searchReplaceAll({
        query,
        replaceText: replaceQuery,
        caseSensitive: matchCase,
        wholeWord: matchWholeWord,
        isRegex: useRegex,
        preserveCase,
      });

      if (res.totalReplacements > 0) {
        showToast(
          `Replaced ${res.totalReplacements} occurrence${res.totalReplacements === 1 ? '' : 's'} across ${res.filesChanged} file${res.filesChanged === 1 ? '' : 's'}`,
          'success'
        );
      } else {
        showToast('No matching text found to replace', 'info');
      }

      await refreshFiles();
      void executeSearch(query);
    } catch (err: unknown) {
      const msg = err instanceof Error ? err.message : String(err);
      showToast(`Replace All failed: ${msg}`, 'error');
    }
  };

  // Keyboard navigation within the modal
  useEffect(() => {
    if (!isProjectSearchOpen) return;

    const handleKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.preventDefault();
        setIsProjectSearchOpen(false);
        return;
      }

      // Mod+Shift+F or Home while open can close or refocus
      const mod = e.metaKey || e.ctrlKey;
      if (mod && e.shiftKey && (e.key === 'f' || e.key === 'F')) {
        e.preventDefault();
        searchInputRef.current?.focus();
        searchInputRef.current?.select();
        return;
      }

      // Option toggles via Alt+ shortcuts (matching Zed)
      if (e.altKey && (e.key === 'c' || e.key === 'C')) {
        e.preventDefault();
        setMatchCase(prev => !prev);
        return;
      }
      if (e.altKey && (e.key === 'w' || e.key === 'W')) {
        e.preventDefault();
        setMatchWholeWord(prev => !prev);
        return;
      }
      if (e.altKey && (e.key === 'r' || e.key === 'R')) {
        e.preventDefault();
        setUseRegex(prev => !prev);
        return;
      }
      if (e.altKey && (e.key === 'h' || e.key === 'H')) {
        e.preventDefault();
        setShowReplace(prev => !prev);
        return;
      }

      // Arrow navigation
      if (e.key === 'ArrowDown') {
        if (flatMatches.length > 0) {
          e.preventDefault();
          setSelectedFlatIndex(prev => (prev + 1) % flatMatches.length);
        }
      } else if (e.key === 'ArrowUp') {
        if (flatMatches.length > 0) {
          e.preventDefault();
          setSelectedFlatIndex(prev => (prev - 1 + flatMatches.length) % flatMatches.length);
        }
      } else if (e.key === 'Enter') {
        // If Enter is pressed and we have selected match, jump to it
        if (flatMatches.length > 0 && selectedFlatIndex >= 0 && selectedFlatIndex < flatMatches.length) {
          e.preventDefault();
          const target = flatMatches[selectedFlatIndex];
          handleOpenResult(target.filePath, target.line, e.shiftKey);
        }
      }
    };

    window.addEventListener('keydown', handleKeyDown);
    return () => window.removeEventListener('keydown', handleKeyDown);
  }, [isProjectSearchOpen, flatMatches, selectedFlatIndex, handleOpenResult, setIsProjectSearchOpen]);

  // Keep selected item scrolled into view
  useEffect(() => {
    if (selectedFlatIndex >= 0 && resultsContainerRef.current) {
      const activeEl = resultsContainerRef.current.querySelector(`[data-flat-index="${selectedFlatIndex}"]`);
      if (activeEl) {
        activeEl.scrollIntoView({ block: 'nearest' });
      }
    }
  }, [selectedFlatIndex]);

  if (!isProjectSearchOpen) return null;

  const shortcutHint = isMac ? '⇧⌘F' : isWindows ? 'Home / ⌃⇧F' : '⌃⇧F';

  return (
    <div
      className="fixed inset-0 z-[75] bg-black/60 backdrop-blur-sm flex items-start justify-center pt-[5vh] p-4 md:p-6 select-none animate-in fade-in duration-100"
      onClick={() => setIsProjectSearchOpen(false)}
    >
      <div
        role="dialog"
        aria-modal="true"
        aria-label="Project Search"
        className="w-full max-w-4xl h-[82vh] bg-popover border border-border rounded-2xl shadow-2xl overflow-hidden flex flex-col animate-in zoom-in-95 duration-100 font-sans"
        onClick={e => e.stopPropagation()}
      >
        {/* Header (Zed-inspired compact bar) */}
        <div className="h-11 px-4 flex items-center justify-between border-b border-border bg-surface shrink-0">
          <div className="flex items-center gap-2.5">
            <Search className="size-4 text-accent-primary" />
            <span className="text-ui-sm font-semibold text-foreground tracking-tight">Project Search</span>
            <kbd className="px-1.5 py-0.5 rounded border border-border bg-background text-ui-xs font-mono text-foreground-subtle">
              {shortcutHint}
            </kbd>
          </div>

          <div className="flex items-center gap-1.5 text-foreground-subtle">
            <button
              type="button"
              onClick={() => void executeSearch(query)}
              className="p-1 rounded hover:bg-surface-hover text-foreground-subtle hover:text-foreground transition-colors cursor-pointer"
              title="Refresh Search (Enter)"
            >
              <RefreshCw className={`size-3.5 ${isSearching ? 'animate-spin' : ''}`} />
            </button>
            <button
              type="button"
              onClick={() => {
                const allExpanded = Object.values(expandedFiles).every(Boolean);
                const next: Record<string, boolean> = {};
                searchResults.forEach(g => { next[g.filePath] = !allExpanded; });
                setExpandedFiles(next);
              }}
              className="p-1 rounded hover:bg-surface-hover text-foreground-subtle hover:text-foreground transition-colors cursor-pointer"
              title="Toggle Collapse / Expand All"
            >
              <ChevronsDownUp className="size-3.5" />
            </button>
            <button
              type="button"
              onClick={() => { setQuery(''); setSearchResults([]); setSearchDurationMs(null); }}
              className="p-1 rounded hover:bg-surface-hover text-foreground-subtle hover:text-foreground transition-colors cursor-pointer"
              title="Clear Search"
            >
              <Ban className="size-3.5" />
            </button>
            <div className="w-px h-3.5 bg-border mx-1" />
            <button
              type="button"
              onClick={() => setIsProjectSearchOpen(false)}
              className="p-1 rounded hover:bg-surface-hover text-foreground-subtle hover:text-foreground transition-colors cursor-pointer"
              title="Close (Esc)"
            >
              <X className="size-4" />
            </button>
          </div>
        </div>

        {/* Search & Options Controls Bar */}
        <div className="p-3 border-b border-border bg-card/60 flex flex-col gap-2 shrink-0">
          {/* Main Search Input Row */}
          <div className="flex items-center gap-2">
            <div className="relative flex-1 flex items-center">
              <Search className="absolute left-3 size-4 text-foreground-subtler pointer-events-none" />
              <input
                ref={searchInputRef}
                type="text"
                value={query}
                onChange={e => setQuery(e.target.value)}
                onKeyDown={e => {
                  if (e.key === 'Enter' && (!flatMatches.length || e.shiftKey)) {
                    void executeSearch(query);
                  }
                }}
                placeholder="Search across project (regex, case sensitive, whole word)..."
                className="w-full h-9 pl-9 pr-28 rounded-lg border border-border bg-background text-ui-sm font-mono text-foreground outline-none transition-colors placeholder:text-foreground-subtlest focus:border-accent-primary"
              />

              {/* Inside-input match option buttons (Zed style) */}
              <div className="absolute right-1.5 flex items-center gap-0.5">
                <button
                  type="button"
                  onClick={() => setMatchCase(prev => !prev)}
                  aria-pressed={matchCase}
                  title="Match Case (Alt+C)"
                  className={`size-6 rounded flex items-center justify-center text-ui-xs font-mono font-bold transition-colors cursor-pointer ${
                    matchCase ? 'bg-primary text-primary-foreground font-semibold' : 'text-foreground-subtle hover:bg-surface-hover'
                  }`}
                >
                  Aa
                </button>
                <button
                  type="button"
                  onClick={() => setMatchWholeWord(prev => !prev)}
                  aria-pressed={matchWholeWord}
                  title="Match Whole Word (Alt+W)"
                  className={`size-6 rounded flex items-center justify-center text-ui-xs font-mono font-bold transition-colors cursor-pointer ${
                    matchWholeWord ? 'bg-primary text-primary-foreground font-semibold' : 'text-foreground-subtle hover:bg-surface-hover'
                  }`}
                >
                  \b
                </button>
                <button
                  type="button"
                  onClick={() => setUseRegex(prev => !prev)}
                  aria-pressed={useRegex}
                  title="Use Regular Expression (Alt+R)"
                  className={`size-6 rounded flex items-center justify-center text-ui-xs font-mono font-bold transition-colors cursor-pointer ${
                    useRegex ? 'bg-primary text-primary-foreground font-semibold' : 'text-foreground-subtle hover:bg-surface-hover'
                  }`}
                >
                  .*
                </button>
                {query && (
                  <button
                    type="button"
                    onClick={() => { setQuery(''); searchInputRef.current?.focus(); }}
                    className="size-6 rounded flex items-center justify-center text-foreground-subtle hover:text-foreground transition-colors cursor-pointer"
                    title="Clear query"
                  >
                    <X className="size-3.5" />
                  </button>
                )}
              </div>
            </div>

            {/* Quick action toggles */}
            <button
              type="button"
              onClick={() => setShowReplace(prev => !prev)}
              aria-pressed={showReplace}
              className={`h-9 px-2.5 rounded-lg border text-ui-xs font-medium flex items-center gap-1.5 transition-colors cursor-pointer ${
                showReplace
                  ? 'border-accent-primary bg-accent-primary/10 text-accent-primary'
                  : 'border-border bg-background text-foreground-subtle hover:text-foreground hover:bg-surface-hover'
              }`}
              title="Toggle Replace (Alt+H)"
            >
              <Replace className="size-3.5" />
              <span>Replace</span>
            </button>

            <button
              type="button"
              onClick={() => setShowFilters(prev => !prev)}
              aria-pressed={showFilters}
              className={`h-9 px-2.5 rounded-lg border text-ui-xs font-medium flex items-center gap-1.5 transition-colors cursor-pointer ${
                showFilters
                  ? 'border-accent-primary bg-accent-primary/10 text-accent-primary'
                  : 'border-border bg-background text-foreground-subtle hover:text-foreground hover:bg-surface-hover'
              }`}
              title="Toggle File Filter Globs"
            >
              <Filter className="size-3.5" />
              <span>Filter</span>
            </button>
          </div>

          {/* Replace Row (expandable) */}
          {showReplace && (
            <div className="flex items-center gap-2 pt-1 animate-in slide-in-from-top-1 duration-100">
              <div className="relative flex-1 flex items-center">
                <Replace className="absolute left-3 size-4 text-foreground-subtler pointer-events-none" />
                <input
                  type="text"
                  value={replaceQuery}
                  onChange={e => setReplaceQuery(e.target.value)}
                  placeholder="Replace with..."
                  className="w-full h-8 pl-9 pr-14 rounded-lg border border-border bg-background text-ui-sm font-mono text-foreground outline-none transition-colors placeholder:text-foreground-subtlest focus:border-accent-primary"
                />
                <div className="absolute right-1.5 flex items-center gap-0.5">
                  <button
                    type="button"
                    onClick={() => setPreserveCase(prev => !prev)}
                    aria-pressed={preserveCase}
                    title="Preserve Case"
                    className={`size-6 rounded flex items-center justify-center text-ui-xs font-mono font-bold transition-colors cursor-pointer ${
                      preserveCase ? 'bg-primary text-primary-foreground font-semibold' : 'text-foreground-subtle hover:bg-surface-hover'
                    }`}
                  >
                    AB
                  </button>
                </div>
              </div>

              <button
                type="button"
                onClick={handleReplaceAll}
                disabled={!query || totalMatchesCount === 0}
                className="h-8 px-3 rounded-lg bg-primary text-primary-foreground text-ui-xs font-semibold flex items-center gap-1.5 disabled:opacity-40 transition-opacity cursor-pointer disabled:cursor-not-allowed shadow-xs"
                title="Replace All Occurrences in Project"
              >
                <span>Replace All ({totalMatchesCount})</span>
              </button>
            </div>
          )}

          {/* Path Filters Row (expandable) */}
          {showFilters && (
            <div className="grid grid-cols-2 gap-2 pt-1 animate-in slide-in-from-top-1 duration-100 text-ui-xs">
              <div className="flex items-center gap-1.5">
                <span className="text-foreground-subtler shrink-0 font-mono">include:</span>
                <input
                  type="text"
                  value={includeFilter}
                  onChange={e => setIncludeFilter(e.target.value)}
                  placeholder="e.g. *.ts, src/**"
                  className="w-full h-7 px-2 rounded-md border border-border bg-background text-ui-xs font-mono text-foreground outline-none focus:border-accent-primary placeholder:text-foreground-subtlest"
                />
              </div>
              <div className="flex items-center gap-1.5">
                <span className="text-foreground-subtler shrink-0 font-mono">exclude:</span>
                <input
                  type="text"
                  value={excludeFilter}
                  onChange={e => setExcludeFilter(e.target.value)}
                  placeholder="e.g. node_modules, dist, *.min.js"
                  className="w-full h-7 px-2 rounded-md border border-border bg-background text-ui-xs font-mono text-foreground outline-none focus:border-accent-primary placeholder:text-foreground-subtlest"
                />
              </div>
            </div>
          )}
        </div>

        {/* Results Stats Bar */}
        <div className="h-7 px-4 border-b border-border bg-surface flex items-center justify-between text-ui-xs text-foreground-subtle shrink-0">
          <div className="flex items-center gap-2">
            {isSearching ? (
              <span className="flex items-center gap-1.5 text-accent-primary">
                <Loader2 className="size-3 animate-spin" />
                <span>Searching project…</span>
              </span>
            ) : query.trim() ? (
              <span>
                <strong className="text-foreground">{totalMatchesCount}</strong> result{totalMatchesCount === 1 ? '' : 's'} in{' '}
                <strong className="text-foreground">{searchResults.length}</strong> file{searchResults.length === 1 ? '' : 's'}
                {searchDurationMs !== null ? ` (${searchDurationMs}ms)` : ''}
              </span>
            ) : (
              <span>Type to search across the entire codebase</span>
            )}
            {searchError && (
              <span className="text-destructive font-medium ml-2">{searchError}</span>
            )}
          </div>

          <div className="flex items-center gap-3 text-foreground-subtler text-ui-xs font-mono">
            <span>↑↓ navigate</span>
            <span>↵ open in {mode === 'agent' ? 'sidebar' : 'editor pane'}</span>
            {mode === 'agent' && <span className="opacity-75">⇧↵ editor pane</span>}
            <span>esc close</span>
          </div>
        </div>

        {/* Multibuffer Results Area (Zed-style code excerpts) */}
        <div
          ref={resultsContainerRef}
          className="flex-1 overflow-y-auto divide-y divide-border/60 bg-background"
        >
          {searchResults.length === 0 ? (
            <div className="h-full flex flex-col items-center justify-center text-foreground-subtle p-8 text-center">
              {isSearching ? (
                <div className="flex flex-col items-center gap-2">
                  <Loader2 className="size-6 text-accent-primary animate-spin" />
                  <span className="text-ui-sm font-medium text-foreground">Searching files…</span>
                </div>
              ) : query.trim() ? (
                <div className="flex flex-col items-center gap-2 max-w-sm">
                  <Ban className="size-8 text-foreground-subtler" />
                  <span className="text-ui-base font-semibold text-foreground">No matches found</span>
                  <p className="text-ui-xs text-foreground-subtle">
                    No results for &ldquo;{query}&rdquo;. Try adjusting match case, regex, or file filters.
                  </p>
                </div>
              ) : (
                <div className="flex flex-col items-center gap-2 max-w-sm">
                  <Search className="size-8 text-foreground-subtler" />
                  <span className="text-ui-base font-semibold text-foreground">Global Project Search</span>
                  <p className="text-ui-xs text-foreground-subtle">
                    Search text patterns and symbols across the entire workspace with live multibuffer excerpts.
                  </p>
                </div>
              )}
            </div>
          ) : (
            searchResults.map(group => {
              const isExpanded = expandedFiles[group.filePath] !== false;
              const relPath = activeWorkspacePath && group.filePath.startsWith(activeWorkspacePath)
                ? group.filePath.slice(activeWorkspacePath.length).replace(/^\/+/, '')
                : group.filePath;

              return (
                <div key={group.filePath} className="flex flex-col">
                  {/* File Header Bar */}
                  <div
                    onClick={() => {
                      setExpandedFiles(prev => ({ ...prev, [group.filePath]: !isExpanded }));
                    }}
                    className="h-8 px-3 flex items-center justify-between bg-surface/50 hover:bg-surface-hover/80 border-b border-border/40 cursor-pointer transition-colors"
                  >
                    <div className="flex items-center gap-2 min-w-0">
                      <button
                        type="button"
                        aria-label={isExpanded ? 'Collapse file' : 'Expand file'}
                        className="text-foreground-subtler hover:text-foreground"
                      >
                        {isExpanded ? <ChevronDown className="size-3.5" /> : <ChevronRight className="size-3.5" />}
                      </button>
                      <span className="shrink-0">{getFileIcon(group.file)}</span>
                      <span className="text-ui-xs font-mono font-semibold text-foreground truncate">
                        {group.file}
                      </span>
                      <span className="text-ui-xs font-mono text-foreground-subtle truncate text-foreground-subtler">
                        {relPath !== group.file ? relPath : ''}
                      </span>
                    </div>

                    <div className="flex items-center gap-2 shrink-0">
                      <span className="px-1.5 py-0.2 rounded-full bg-surface border border-border text-ui-xs font-mono text-foreground-subtle">
                        {group.matches.length}
                      </span>
                      <button
                        type="button"
                        onClick={e => {
                          e.stopPropagation();
                          handleOpenResult(group.filePath, group.matches[0]?.line || 1, e.shiftKey);
                        }}
                        className="p-1 rounded text-foreground-subtler hover:text-foreground hover:bg-surface transition-colors cursor-pointer"
                        title={mode === 'agent' ? "Open in sidebar (Shift+Click for editor pane)" : "Open in editor pane"}
                      >
                        <ExternalLink className="size-3.5" />
                      </button>
                    </div>
                  </div>

                  {/* Multibuffer Lines View */}
                  {isExpanded && group.matches.length > 0 && (
                    <div className="flex flex-col font-mono text-ui-sm">
                      {group.matches.map(m => {
                        const flatIdx = flatMatches.findIndex(fm => fm.matchId === m.id);
                        const isSelected = flatIdx === selectedFlatIndex;

                        return (
                          <div
                            key={m.id}
                            data-flat-index={flatIdx}
                            onClick={e => handleOpenResult(group.filePath, m.line, e.shiftKey)}
                            title={mode === 'agent' ? `Open line ${m.line} in sidebar (Shift+Click for editor pane)` : `Open line ${m.line} in editor pane`}
                            className={`group flex items-center min-h-[1.75rem] px-2 py-0.5 border-l-2 transition-colors cursor-pointer ${
                              isSelected
                                ? 'bg-accent-primary/10 border-accent-primary text-foreground'
                                : 'border-transparent hover:bg-surface-hover/60 text-foreground'
                            }`}
                          >
                            {/* Line Number Gutter */}
                            <span className="min-w-[3.5rem] pr-3 select-none text-right text-ui-xs font-mono text-foreground-subtlest group-hover:text-foreground-subtle shrink-0">
                              {m.line}
                            </span>

                            {/* Code snippet with highlighted match */}
                            <div className="flex-1 min-w-0 overflow-x-auto whitespace-pre truncate font-mono text-ui-xs text-foreground">
                              {renderHighlightedText(m.text, query, matchCase, useRegex)}
                            </div>

                            {/* Inline Replace button for this single match */}
                            {showReplace && replaceQuery && (
                              <button
                                type="button"
                                onClick={e => {
                                  e.stopPropagation();
                                  void handleReplaceSingleMatch(group.filePath, m.id, m.line);
                                }}
                                className="opacity-0 group-hover:opacity-100 ml-2 px-1.5 py-0.5 rounded border border-border bg-surface text-ui-xs font-mono text-foreground-subtle hover:text-foreground hover:bg-surface-hover transition-opacity cursor-pointer shrink-0"
                                title="Replace this occurrence"
                              >
                                Replace
                              </button>
                            )}
                          </div>
                        );
                      })}
                    </div>
                  )}
                </div>
              );
            })
          )}
        </div>
      </div>
    </div>
  );
};

/** Highlights matched text pattern within a line */
function renderHighlightedText(
  lineText: string,
  searchQuery: string,
  matchCase: boolean,
  useRegex: boolean
): React.ReactNode {
  if (!searchQuery) return lineText;

  try {
    const flags = matchCase ? 'g' : 'gi';
    const regex = useRegex
      ? new RegExp(`(${searchQuery})`, flags)
      : new RegExp(`(${searchQuery.replace(/[.+^${}()|[\]\\]/g, '\\$&')})`, flags);

    const parts = lineText.split(regex);
    if (parts.length <= 1) return lineText;

    return (
      <>
        {parts.map((part, idx) => {
          const isMatch = regex.test(part);
          // Reset regex state after test
          regex.lastIndex = 0;
          return isMatch ? (
            <mark
              key={idx}
              className="bg-accent-primary/25 text-accent-primary font-semibold rounded-xs px-0.5"
            >
              {part}
            </mark>
          ) : (
            <span key={idx}>{part}</span>
          );
        })}
      </>
    );
  } catch {
    return lineText;
  }
}
