import React, { useEffect, useMemo, useState } from 'react';
import { Search, RefreshCw, Plus, MoreHorizontal, Download } from 'lucide-react';
import { ApiBridge } from '../../services/apiBridge';
import { PluginInfo } from '../../types';
import { useWorkspace } from '../../stores/workspaceStore';

/**
 * Plugin marketplace page: page header, search, installed strip,
 * Public/Personal tabs, and category groups of plugin cards
 * (`grid gap-x-6 gap-y-1 sm:grid-cols-2`, flat hover rows).
 */
export const PluginMarketplaceScreen: React.FC = () => {
  const { setMode } = useWorkspace();
  const [plugins, setPlugins] = useState<PluginInfo[]>([]);
  const [loading, setLoading] = useState(true);
  const [query, setQuery] = useState('');
  const [tab, setTab] = useState<'public' | 'personal'>('public');

  const load = async () => {
    setLoading(true);
    try {
      const list = await ApiBridge.listPlugins();
      setPlugins((list || []).filter(Boolean) as PluginInfo[]);
    } catch {
      setPlugins([]);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => { void load(); }, []);

  const installed = useMemo(() => plugins.filter(p => p.enabled), [plugins]);
  const filtered = useMemo(() => {
    const q = query.trim().toLowerCase();
    return plugins.filter(p =>
      !q || p.name.toLowerCase().includes(q) || (p.description || '').toLowerCase().includes(q)
    );
  }, [plugins, query]);

  const categories = useMemo(() => {
    const groups = new Map<string, PluginInfo[]>();
    for (const p of filtered) {
      const cat = p.source === 'builtin' ? 'Built-in' : p.enabled ? 'Active' : 'Community';
      if (!groups.has(cat)) groups.set(cat, []);
      groups.get(cat)!.push(p);
    }
    return Array.from(groups.entries());
  }, [filtered]);

  return (
    <div className="dark flex-1 h-full overflow-hidden bg-background text-foreground">
      <div className="mx-auto flex h-full max-w-3xl flex-col overflow-y-auto px-8 py-10">
        {/* Page header */}
        <div className="mb-6 flex items-start justify-between gap-4">
          <div>
            <h1 className="text-ui-xl font-semibold tracking-tight text-foreground">Plugin Marketplace</h1>
            <p className="mt-1 text-ui-base text-foreground-subtle">
              Extend ForgeADE with skills, commands, and MCP servers from plugins
            </p>
          </div>
          <div className="flex shrink-0 items-center gap-1.5">
            <button
              type="button"
              onClick={() => void load()}
              className="flex h-8 w-8 items-center justify-center rounded-lg border border-border text-foreground-subtle hover:bg-surface-hover hover:text-foreground transition-colors cursor-pointer"
              title="Refresh"
            >
              <RefreshCw className={`size-3.5 ${loading ? 'animate-spin' : ''}`} />
            </button>
            <button
              type="button"
              onClick={() => setMode('settings')}
              className="flex h-8 items-center gap-1 rounded-lg bg-foreground px-3 text-ui-sm font-medium text-background hover:opacity-90 transition-opacity cursor-pointer"
            >
              <Plus className="size-3.5" />
              New
            </button>
          </div>
        </div>

        {/* Search */}
        <div className="relative mb-5">
          <Search className="absolute left-3 top-1/2 size-4 -translate-y-1/2 text-foreground-subtlest" />
          <input
            value={query}
            onChange={(e) => setQuery(e.target.value)}
            placeholder="Search plugins"
            className="h-9 w-full rounded-lg border border-border bg-background pl-9 pr-3 text-ui-sm text-foreground outline-none placeholder:text-foreground-subtlest focus:border-input-border-focused"
          />
        </div>

        {/* Installed strip */}
        <div className="mb-6">
          <div className="mb-2 flex items-center justify-between">
            <h2 className="text-ui-base font-semibold text-foreground">Installed</h2>
            <button
              type="button"
              onClick={() => setMode('settings')}
              className="flex h-7 w-7 items-center justify-center rounded-lg border border-border text-foreground-subtle hover:bg-surface-hover hover:text-foreground transition-colors cursor-pointer"
              title="Manage plugins"
            >
              <MoreHorizontal className="size-4" />
            </button>
          </div>
          <div className="flex flex-wrap items-center gap-2">
            {installed.map(p => (
              <div
                key={p.id}
                className="flex size-10 items-center justify-center rounded-xl border border-border bg-surface text-ui-sm font-semibold text-foreground"
                title={`${p.name} v${p.version}`}
              >
                {(p.name || '?').slice(0, 1).toUpperCase()}
              </div>
            ))}
            {installed.length === 0 && (
              <p className="text-ui-sm text-foreground-subtlest">No plugins installed yet.</p>
            )}
          </div>
        </div>

        {/* Tabs */}
        <div className="mb-4 flex items-center gap-1">
          {(['public', 'personal'] as const).map(t => (
            <button
              key={t}
              type="button"
              onClick={() => setTab(t)}
              className={`rounded-full px-3 py-1 text-ui-sm capitalize transition-colors cursor-pointer ${
                tab === t ? 'bg-selected text-foreground font-medium' : 'text-foreground-subtle hover:text-foreground'
              }`}
            >
              {t}
            </button>
          ))}
        </div>

        {/* Category groups */}
        {tab === 'public' ? (
          <div className="flex flex-col gap-6">
            {categories.map(([cat, list]) => (
              <div key={cat}>
                <h2 className="mb-2 text-ui-base font-semibold text-foreground">{cat}</h2>
                <div className="grid gap-x-6 gap-y-1 sm:grid-cols-2">
                  {list.map(p => (
                    <div
                      key={p.id}
                      className="group/card flex min-w-0 cursor-pointer items-center gap-3 rounded-xl px-2 py-2.5 transition-colors hover:bg-hover"
                    >
                      <div className="flex size-10 shrink-0 items-center justify-center rounded-lg bg-surface text-ui-sm font-semibold text-foreground">
                        {(p.name || '?').slice(0, 1).toUpperCase()}
                      </div>
                      <div className="min-w-0 flex-1">
                        <div className="flex min-w-0 items-center gap-1.5">
                          <span className="min-w-0 truncate text-ui-base font-semibold text-foreground">{p.name}</span>
                          {p.enabled ? (
                            <span className="inline-flex shrink-0 items-center gap-1 rounded-full bg-success/14 px-1.5 py-0.5 text-ui-sm leading-none text-success">
                              <Download className="size-3" />
                            </span>
                          ) : null}
                        </div>
                        {p.description ? (
                          <div className="mt-0.5 truncate text-ui-sm text-foreground-subtle">{p.description}</div>
                        ) : null}
                      </div>
                      <div className="flex shrink-0 items-center gap-1.5">
                        <button
                          type="button"
                          onClick={async () => {
                            setPlugins(prev => prev.map(x => x.id === p.id ? { ...x, enabled: !x.enabled } : x));
                            try { await ApiBridge.togglePlugin(p.id, !p.enabled); } catch { void load(); }
                          }}
                          className={`h-7 shrink-0 rounded-full border px-3 text-ui-sm font-medium transition-colors cursor-pointer ${
                            p.enabled
                              ? 'border-border text-foreground-subtle hover:bg-surface-hover'
                              : 'border-border text-foreground hover:bg-surface-hover'
                          }`}
                        >
                          {p.enabled ? 'Active' : 'Install'}
                        </button>
                      </div>
                    </div>
                  ))}
                </div>
              </div>
            ))}
            {categories.length === 0 && !loading && (
              <p className="text-ui-sm text-foreground-subtlest">No plugins match your search.</p>
            )}
          </div>
        ) : (
          <p className="text-ui-sm text-foreground-subtlest">
            Personal plugins live in your workspace <span className="font-mono">.forge/plugins</span> directory.
          </p>
        )}
      </div>
    </div>
  );
};
