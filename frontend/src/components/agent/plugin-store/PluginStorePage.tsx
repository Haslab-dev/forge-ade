import React, { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { Loader2, Plus, RefreshCw, Settings as SettingsIcon, Sparkles, ChevronDown } from 'lucide-react';
import { ApiBridge } from '../../../services/apiBridge';
import { cn } from '../../../lib/utils';
import { useToast } from '../../../lib/toast';
import type {
  PluginDescribeResult,
  PluginMarketplaceSummary,
  PluginStoreOverview
} from './types';
import { OFFICIAL_MARKETPLACE_ID } from './types';
import type { StorePluginItem } from './pluginStoreListing';
import {
  buildStoreItems,
  canUpdatePluginItem,
  resolveItemDisplayName
} from './pluginStoreListing';
import { PluginStoreListView, type PluginStoreSegment } from './PluginStoreListView';
import {
  PluginStoreAdvancedSection,
  PluginStoreDetailView
} from './PluginStoreDetailView';
import {
  AddMarketplaceSourceDialog,
  PluginStoreSourcesDialog,
  PluginUninstallConfirmDialog
} from './PluginStoreDialogs';
import type { PluginStoreActions } from './PluginStoreCard';

interface DescribeEntry {
  status: 'loading' | 'loaded' | 'error';
  data?: PluginDescribeResult;
}

type PluginStoreView = 'store' | 'detail';

/**
 * Plugin Marketplace page (ported 1:1 from ZCode PluginStorePage):
 * title + actions row, list/detail routing, marketplace source dialogs,
 * install/uninstall/update orchestration, describe-on-demand.
 */
export function PluginStorePage({
  onManageInstalled,
  onBreadcrumbChange
}: {
  onManageInstalled: () => void;
  onBreadcrumbChange?: (items: Array<{ label: string }>) => void;
}) {
  const { toast } = useToast();
  const [overview, setOverview] = useState<PluginStoreOverview | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [operationId, setOperationId] = useState<string | null>(null);

  const [view, setView] = useState<PluginStoreView>('store');
  const [detailPluginId, setDetailPluginId] = useState<string | null>(null);
  const [segment, setSegment] = useState<PluginStoreSegment>('public');
  const [query, setQuery] = useState('');
  const [refreshing, setRefreshing] = useState(false);
  const [sourcesOpen, setSourcesOpen] = useState(false);
  const [addSourceOpen, setAddSourceOpen] = useState(false);
  const [addMarketplaceError, setAddMarketplaceError] = useState<string | null>(null);
  const [describeCache, setDescribeCache] = useState<Record<string, DescribeEntry>>({});
  const [uninstallTarget, setUninstallTarget] = useState<{ id: string; name: string } | null>(null);
  const [uninstalling, setUninstalling] = useState(false);
  const rootRef = useRef<HTMLDivElement | null>(null);
  const storeScrollTopRef = useRef(0);

  const loadOverview = useCallback(async () => {
    const data = await ApiBridge.pluginStoreOverview();
    setOverview(data);
    setLoading(false);
    return data;
  }, []);

  useEffect(() => {
    void loadOverview();
  }, [loadOverview]);

  // Refresh the official catalog on entry, throttled to a 10-minute window
  // (module-level state — the page remounts on every open).
  useEffect(() => {
    const official = overview?.marketplaces.find((m) => m.id === OFFICIAL_MARKETPLACE_ID);
    if (!official) return;
    if (!claimMarketplaceAutoRefresh(official.lastUpdated)) return;
    void (async () => {
      try {
        await ApiBridge.pluginStoreUpdateMarketplace(OFFICIAL_MARKETPLACE_ID);
      } catch {
        // auto-refresh is best effort; manual refresh surfaces errors
      }
      await loadOverview();
    })();
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [overview?.marketplaces]);

  const items = useMemo(() => {
    if (!overview) return [] as StorePluginItem[];
    return buildStoreItems({
      marketplaces: overview.marketplaces,
      marketplaceAvailabilityKnown: overview.marketplaceAvailabilityKnown,
      availablePlugins: overview.availablePlugins,
      installedPlugins: overview.installedPlugins,
      plugins: overview.plugins
    });
  }, [overview]);

  const itemById = useMemo(() => new Map(items.map((item) => [item.id, item])), [items]);
  const detailItem = detailPluginId ? (itemById.get(detailPluginId) ?? null) : null;

  // Project detail state to the parent breadcrumb row (Settings breadcrumb contract).
  useEffect(() => {
    onBreadcrumbChange?.(view === 'detail' && detailItem ? [{ label: resolveItemDisplayName(detailItem) }] : []);
  }, [detailItem, onBreadcrumbChange, view]);

  // Detail describe-on-demand: installed entries describe from their root dir,
  // candidates describe from their marketplace without installing.
  useEffect(() => {
    if (view !== 'detail' || !detailItem) return;
    if (describeCache[detailItem.id]) return;
    setDescribeCache((prev) => ({ ...prev, [detailItem.id]: { status: 'loading' } }));
    void (async () => {
      try {
        let data: PluginDescribeResult | null;
        if (detailItem.info?.dir) {
          data = await ApiBridge.pluginStoreDescribeInstalled(detailItem.info.dir);
        } else {
          data = await ApiBridge.pluginStoreDescribePlugin(detailItem.name, detailItem.marketplace);
        }
        setDescribeCache((prev) => ({
          ...prev,
          [detailItem.id]: data ? { status: 'loaded', data } : { status: 'error' }
        }));
      } catch {
        setDescribeCache((prev) => ({ ...prev, [detailItem.id]: { status: 'error' } }));
      }
    })();
  }, [detailItem, describeCache, view]);

  // Detail entry vanished (uninstalled etc.) → back to the list.
  useEffect(() => {
    if (view === 'detail' && detailPluginId && !itemById.has(detailPluginId)) {
      setView('store');
      setDetailPluginId(null);
    }
  }, [detailPluginId, itemById, view]);

  const scrollContainer = (): HTMLElement | null => rootRef.current?.closest('main') ?? null;

  const openDetail = useCallback((pluginId: string) => {
    storeScrollTopRef.current =
      (rootRef.current?.closest('main') as HTMLElement | null)?.scrollTop ?? 0;
    setDetailPluginId(pluginId);
    setView('detail');
    requestAnimationFrame(() => {
      const main = rootRef.current?.closest('main');
      if (main) main.scrollTop = 0;
    });
  }, []);

  const backToStore = useCallback(() => {
    setView('store');
    setDetailPluginId(null);
    requestAnimationFrame(() => {
      const main = scrollContainer();
      if (main) main.scrollTop = storeScrollTopRef.current;
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, []);

  const runOperation = useCallback(
    async (id: string, run: () => Promise<void>, toastMessage?: string) => {
      setOperationId(id);
      setError(null);
      try {
        await run();
        if (toastMessage) toast(toastMessage);
      } catch (e: any) {
        setError(e?.message || String(e));
      } finally {
        setOperationId(null);
      }
      await loadOverview();
    },
    [loadOverview, toast]
  );

  const handleInstall = useCallback(
    (item: StorePluginItem) => {
      void runOperation(
        `plugin:install:${item.name}@${item.marketplace}`,
        () => ApiBridge.pluginStoreInstallPlugin(item.name, item.marketplace)
      );
    },
    [runOperation]
  );

  const handleUpdatePlugin = useCallback(
    (pluginId: string) => {
      void runOperation(`plugin:update:${pluginId}`, () =>
        ApiBridge.pluginStoreUpdatePlugin(pluginId)
      );
    },
    [runOperation]
  );

  const requestUninstall = useCallback((pluginId: string) => {
    const item = itemById.get(pluginId);
    setUninstallTarget({
      id: pluginId,
      name: item ? resolveItemDisplayName(item) : pluginId.split('@')[0]
    });
  }, [itemById]);

  const confirmUninstall = useCallback(() => {
    if (!uninstallTarget) return;
    setUninstalling(true);
    void runOperation(`plugin:uninstall:${uninstallTarget.id}`, async () => {
      await ApiBridge.pluginStoreUninstallPlugin(uninstallTarget.id);
      setUninstallTarget(null);
    }).finally(() => setUninstalling(false));
  }, [runOperation, uninstallTarget]);

  const handleRefresh = async () => {
    setRefreshing(true);
    try {
      const marketplaces = overview?.marketplaces ?? [];
      let failures = 0;
      for (const marketplace of marketplaces) {
        try {
          await ApiBridge.pluginStoreUpdateMarketplace(marketplace.id);
        } catch {
          failures += 1;
        }
      }
      const data = await loadOverview();
      const pending = data.installedPlugins.filter((p) => p.updateStatus === 'update-available');
      if (pending.length > 0) {
        toast(`Found ${pending.length} plugin update(s) available`);
      } else if (failures === 0) {
        toast('All installed plugins are up to date');
      }
    } finally {
      setRefreshing(false);
    }
  };

  const handleAddMarketplace = useCallback(
    async (source: string): Promise<boolean> => {
      const opId = `marketplace:add:${source}`;
      setOperationId(opId);
      setAddMarketplaceError(null);
      try {
        await ApiBridge.pluginStoreAddMarketplace(source);
        // Custom marketplaces are personal-segment; jump there so the new
        // source is immediately visible (matches reference behavior).
        setSegment('personal');
        await loadOverview();
        return true;
      } catch (e: any) {
        setAddMarketplaceError(e?.message || String(e));
        return false;
      } finally {
        setOperationId(null);
      }
    },
    [loadOverview, toast]
  );

  const handleUpdateMarketplace = useCallback(
    (marketplace: string) => {
      void runOperation(`marketplace:update:${marketplace}`, () =>
        ApiBridge.pluginStoreUpdateMarketplace(marketplace)
      );
    },
    [runOperation]
  );

  const handleRemoveMarketplace = useCallback(
    (marketplace: string) => {
      void runOperation(`marketplace:remove:${marketplace}`, () =>
        ApiBridge.pluginStoreRemoveMarketplace(marketplace)
      );
    },
    [runOperation]
  );

  const actions: PluginStoreActions = useMemo(
    () => ({
      onOpenDetail: openDetail,
      onInstall: handleInstall,
      onUninstall: requestUninstall,
      onUpdate: handleUpdatePlugin,
      onSetEnabled: (pluginId, enabled) => {
        void runOperation(`plugin:toggle:${pluginId}`, async () => {
          await ApiBridge.togglePlugin(pluginId, enabled);
        });
      },
      operationId
    }),
    [handleInstall, handleUpdatePlugin, openDetail, operationId, requestUninstall, runOperation]
  );

  const detailUpdatePending = canUpdatePluginItem(detailItem);
  const detailDescribe = detailItem ? describeCache[detailItem.id] : undefined;

  return (
    <div ref={rootRef} className="space-y-5" data-testid="plugin-store-root" data-view={view}>
      {view === 'store' ? (
        <h1
          data-testid="plugin-store-title"
          className="text-2xl font-semibold tracking-tight text-foreground lg:text-3xl"
        >
          Plugin Marketplace
        </h1>
      ) : null}
      {view === 'store' ? (
        <div className="flex flex-wrap items-start justify-between gap-3">
          <p className="min-w-0 flex-1 text-ui-base leading-6 text-foreground-subtle">
            Extend ForgeADE with skills, commands, and MCP servers from plugins
          </p>
          <div className="flex shrink-0 items-center gap-2">
            <button
              type="button"
              data-testid="plugin-store-refresh"
              aria-label={refreshing ? 'Refreshing...' : 'Refresh'}
              disabled={refreshing}
              className="inline-flex size-9 items-center justify-center gap-1.5 rounded-lg border border-border text-foreground transition-colors hover:bg-surface-hover disabled:opacity-60"
              onClick={() => void handleRefresh()}
            >
              <RefreshCw
                className={refreshing ? 'size-3.5 animate-spin' : 'size-3.5'}
                aria-hidden="true"
              />
            </button>
            <button
              type="button"
              data-testid="plugin-store-sources-open"
              aria-label="Marketplace sources"
              title="Marketplace sources"
              className="inline-flex size-9 items-center justify-center gap-1.5 rounded-lg border border-border text-foreground transition-colors hover:bg-surface-hover"
              onClick={() => setSourcesOpen(true)}
            >
              <SettingsIcon className="size-3.5" aria-hidden="true" />
            </button>
            <PluginAddMenu
              onAddMarketplace={() => {
                setAddMarketplaceError(null);
                setAddSourceOpen(true);
              }}
            />
          </div>
        </div>
      ) : null}

      {error && !addSourceOpen ? (
        <div
          className="rounded-lg border border-destructive/30 bg-destructive/10 px-3 py-2 text-ui-base text-destructive"
          data-testid="plugin-store-error"
        >
          {error}
        </div>
      ) : null}

      {view === 'detail' && detailItem ? (
        <PluginStoreDetailView
          item={detailItem}
          actions={actions}
          describeEntry={detailDescribe}
          onRetryDescribe={() => {
            if (!detailItem) return;
            setDescribeCache((prev) => {
              const next = { ...prev };
              delete next[detailItem.id];
              return next;
            });
          }}
          advanced={
            detailItem.installed ? (
              <PluginStoreAdvancedSection>
                {detailUpdatePending ? (
                  <div className="flex items-center justify-between gap-2 rounded-xl border border-border bg-card px-3 py-2">
                    <div className="min-w-0">
                      <p className="text-ui-base font-medium text-foreground">
                        {`Version ${detailItem.installedMeta?.latestVersion ?? ''} is available`}
                      </p>
                      <p className="text-ui-xs text-foreground-subtle">
                        Updates apply to new sessions; existing sessions keep the current version.
                      </p>
                    </div>
                    <button
                      type="button"
                      data-testid="plugin-store-detail-update"
                      disabled={operationId === `plugin:update:${detailItem.id}`}
                      className="inline-flex h-8 shrink-0 items-center justify-center rounded-lg border border-border px-3 text-ui-sm font-medium text-foreground transition-colors hover:bg-surface-hover disabled:opacity-60"
                      onClick={() => handleUpdatePlugin(detailItem.id)}
                    >
                      Update
                    </button>
                  </div>
                ) : null}
                {detailItem.info?.rootPath ? (
                  <DetailRow label="Plugin path" value={detailItem.info.rootPath} />
                ) : null}
              </PluginStoreAdvancedSection>
            ) : undefined
          }
        />
      ) : (
        <PluginStoreListView
          items={items}
          marketplaces={overview?.marketplaces ?? []}
          actions={actions}
          loading={loading || (operationId?.startsWith('marketplace:update:') ?? false)}
          query={query}
          onQueryChange={setQuery}
          segment={segment}
          onSegmentChange={setSegment}
          onOpenManage={onManageInstalled}
        />
      )}

      <PluginUninstallConfirmDialog
        open={uninstallTarget !== null}
        pluginName={uninstallTarget?.name ?? ''}
        pending={uninstalling}
        onCancel={() => setUninstallTarget(null)}
        onConfirm={confirmUninstall}
      />
      <PluginStoreSourcesDialog
        open={sourcesOpen}
        onOpenChange={setSourcesOpen}
        marketplaces={overview?.marketplaces ?? []}
        onUpdateMarketplace={handleUpdateMarketplace}
        onRemoveMarketplace={handleRemoveMarketplace}
        operationId={operationId}
      />
      <AddMarketplaceSourceDialog
        open={addSourceOpen}
        onOpenChange={setAddSourceOpen}
        onAddMarketplace={handleAddMarketplace}
        operationId={operationId}
        error={addMarketplaceError}
      />
    </div>
  );
}

function DetailRow({ label, value }: { label: string; value: string }) {
  return (
    <div className="grid grid-cols-[10rem_minmax(0,1fr)] items-baseline gap-3">
      <div className="text-ui-sm font-medium text-foreground-subtle">{label}</div>
      <div className="min-w-0 truncate rounded-lg border border-border bg-surface px-2 py-1 font-mono text-ui-sm text-foreground">
        {value}
      </div>
    </div>
  );
}

/** Top-right "Add" dropdown: create plugin + add marketplace source. */
function PluginAddMenu({ onAddMarketplace }: { onAddMarketplace: () => void }) {
  const [open, setOpen] = useState(false);
  const ref = useRef<HTMLDivElement | null>(null);
  useEffect(() => {
    if (!open) return;
    const onPointerDown = (event: MouseEvent) => {
      if (ref.current && !ref.current.contains(event.target as Node)) setOpen(false);
    };
    const onKey = (event: KeyboardEvent) => {
      if (event.key === 'Escape') setOpen(false);
    };
    document.addEventListener('mousedown', onPointerDown);
    document.addEventListener('keydown', onKey);
    return () => {
      document.removeEventListener('mousedown', onPointerDown);
      document.removeEventListener('keydown', onKey);
    };
  }, [open]);

  return (
    <div className="relative" ref={ref}>
      <button
        type="button"
        data-testid="plugin-store-create"
        className={cn(
          'inline-flex h-9 items-center justify-center gap-1.5 whitespace-nowrap rounded-lg bg-brand px-3.5 text-ui-sm font-medium text-foreground-inverse transition-colors hover:bg-brand/80'
        )}
        onClick={() => setOpen((prev) => !prev)}
      >
        <Sparkles className="size-3.5" aria-hidden="true" />
        Add
        <ChevronDown className="size-3.5" aria-hidden="true" />
      </button>
      {open ? (
        <div
          role="menu"
          data-testid="plugin-add-menu"
          className="absolute right-0 top-full z-50 mt-1 min-w-52 overflow-hidden rounded-xl border border-border bg-popover p-1 shadow-lg"
        >
          <button
            type="button"
            role="menuitem"
            data-testid="plugin-store-add-source-menu-item"
            className="flex w-full items-center gap-2 rounded-lg px-2 py-1.5 text-left text-ui-sm text-foreground transition-colors hover:bg-surface-hover"
            onClick={() => {
              setOpen(false);
              onAddMarketplace();
            }}
          >
            <Plus className="size-4" aria-hidden="true" />
            Add plugin marketplace
          </button>
        </div>
      ) : null}
    </div>
  );
}

// ── module-level helpers ─────────────────────────────────────────────────────

const AUTO_REFRESH_INTERVAL_MS = 10 * 60 * 1000;
const moduleAutoRefreshState = new Map<string, number>();

function claimMarketplaceAutoRefresh(lastUpdated?: string): boolean {
  const now = Date.now();
  const last = Math.max(
    lastUpdated ? Date.parse(lastUpdated) || 0 : 0,
    moduleAutoRefreshState.get(OFFICIAL_MARKETPLACE_ID) ?? 0
  );
  if (now - last < AUTO_REFRESH_INTERVAL_MS) return false;
  moduleAutoRefreshState.set(OFFICIAL_MARKETPLACE_ID, now);
  return true;
}

