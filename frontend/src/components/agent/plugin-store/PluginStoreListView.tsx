import React, { useMemo, useState } from 'react';
import { Download, Loader2, Search, Settings2, X } from 'lucide-react';
import { cn } from '../../../lib/utils';
import { OFFICIAL_MARKETPLACE_ID } from './types';
import type { PluginMarketplaceSummary } from './types';
import type { PluginStoreActions } from './PluginStoreCard';
import { PluginStoreCard } from './PluginStoreCard';
import { PluginStoreAvatar } from './PluginStoreAvatar';
import {
  FALLBACK_CATEGORY,
  KNOWN_CATEGORY_LABELS,
  canUpdatePluginItem,
  groupItemsByCategory,
  isPublicStoreMarketplaceId,
  resolveItemDisplayName,
  resolveMarketplaceDisplayName,
  selectFeaturedItems,
  sortInstalledStripItems,
  sortPersonalMarketplaceGroups,
  storeItemMatches,
  type PersonalMarketplaceGroup,
  type StorePluginItem
} from './pluginStoreListing';

// Manual group collapse shows this many cards before the "see more" row.
const CATEGORY_VISIBLE_LIMIT = 6;

export type PluginStoreSegment = 'public' | 'personal';

export function PluginStoreListView({
  items,
  marketplaces,
  actions,
  loading,
  query,
  onQueryChange,
  segment,
  onSegmentChange,
  onOpenManage
}: {
  items: StorePluginItem[];
  marketplaces: PluginMarketplaceSummary[];
  actions: PluginStoreActions;
  loading: boolean;
  query: string;
  onQueryChange: (query: string) => void;
  segment: PluginStoreSegment;
  onSegmentChange: (segment: PluginStoreSegment) => void;
  onOpenManage: () => void;
}) {
  const keyword = query.trim().toLowerCase();
  const [expandedGroups, setExpandedGroups] = useState<Record<string, boolean>>({});

  const installedItems = useMemo(
    () => sortInstalledStripItems(items.filter((item) => item.installed)),
    [items]
  );
  const publicItems = useMemo(
    () => items.filter((item) => isPublicStoreMarketplaceId(item.marketplace)),
    [items]
  );
  const personalItems = useMemo(
    () => items.filter((item) => !isPublicStoreMarketplaceId(item.marketplace)),
    [items]
  );

  const featuredItems = useMemo(
    () => selectFeaturedItems(publicItems, marketplaces),
    [marketplaces, publicItems]
  );
  const categoryGroups = useMemo(() => groupItemsByCategory(publicItems), [publicItems]);

  // Personal segment: one group per marketplace, most recently refreshed first.
  const personalGroups = useMemo(() => {
    const groups = new Map<string, StorePluginItem[]>();
    for (const item of personalItems) {
      const group = groups.get(item.marketplace) ?? [];
      group.push(item);
      groups.set(item.marketplace, group);
    }
    const titled: PersonalMarketplaceGroup[] = [...groups.entries()].map(
      ([marketplace, groupItems]) => ({
        marketplace,
        title: resolveMarketplaceDisplayName(marketplace, marketplaces),
        items: [...groupItems].sort((left, right) =>
          resolveItemDisplayName(left).localeCompare(resolveItemDisplayName(right))
        )
      })
    );
    return sortPersonalMarketplaceGroups(titled, marketplaces);
  }, [marketplaces, personalItems]);

  const searchResults = useMemo(() => {
    if (!keyword) return [];
    return items
      .filter((item) => storeItemMatches(item, keyword))
      .sort((left, right) =>
        resolveItemDisplayName(left).localeCompare(resolveItemDisplayName(right))
      );
  }, [items, keyword]);

  const toggleGroup = (key: string) => {
    setExpandedGroups((current) => ({ ...current, [key]: !(current[key] ?? true) }));
  };

  const canClear = query.length > 0;

  return (
    <div className="space-y-8" data-testid="plugin-store-list">
      {/* Search spans public+personal; typing replaces the segmented layout. */}
      <div className="relative" data-testid="plugin-store-search">
        <Search
          className="pointer-events-none absolute left-3 top-1/2 size-4 -translate-y-1/2 text-foreground-subtle"
          aria-hidden="true"
        />
        <input
          type="search"
          value={query}
          onChange={(event) => onQueryChange(event.target.value)}
          placeholder="Search plugins"
          className="h-9 w-full rounded-xl border border-border bg-background pl-9 pr-9 text-ui-sm text-foreground outline-none transition-colors placeholder:text-foreground-subtlest hover:border-input-border-hover focus:border-input-border-focused [&::-webkit-search-cancel-button]:appearance-none"
        />
        {canClear ? (
          <button
            type="button"
            aria-label="Clear search"
            className="absolute right-1 top-1/2 flex size-7 -translate-y-1/2 items-center justify-center rounded-full text-foreground-subtle transition-colors hover:bg-surface-hover hover:text-foreground"
            onClick={() => onQueryChange('')}
          >
            <X className="size-3.5" aria-hidden="true" />
          </button>
        ) : null}
      </div>

      {/* Installed strip: avatar opens detail, gear opens manage-installed. */}
      {installedItems.length > 0 ? (
        <section data-testid="plugin-store-installed-strip">
          <div className="flex items-center justify-between border-b border-border pb-2">
            <h2 className="text-ui-lg font-semibold text-foreground">Installed</h2>
            <button
              type="button"
              data-testid="plugin-store-manage-open"
              aria-label="Manage installed"
              title="Manage installed"
              className="flex size-8 items-center justify-center rounded-lg border border-border text-foreground transition-colors hover:bg-surface-hover"
              onClick={onOpenManage}
            >
              <Settings2 className="size-4" aria-hidden="true" />
            </button>
          </div>
          <div className="mt-1 flex items-center gap-3 overflow-x-auto px-2 pb-1 pt-2 sm:-mx-2">
            {installedItems.map((item) => {
              const displayName = resolveItemDisplayName(item);
              const updating = actions.operationId === `plugin:update:${item.id}`;
              return (
                <div key={item.id} className="relative shrink-0" title={displayName}>
                  <button
                    type="button"
                    data-testid="plugin-store-installed-item"
                    data-plugin-id={item.id}
                    aria-label={displayName}
                    className="shrink-0 rounded-xl transition-transform hover:scale-105 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-input-border-focused"
                    onClick={() => actions.onOpenDetail(item.id)}
                  >
                    <PluginStoreAvatar
                      src={item.listing?.icon}
                      pluginId={item.id}
                      className="size-10"
                    />
                  </button>
                  {canUpdatePluginItem(item) ? (
                    <button
                      type="button"
                      data-testid="plugin-store-installed-item-update"
                      data-plugin-id={item.id}
                      aria-label="Update"
                      className="absolute -right-1 -top-1 flex size-4 items-center justify-center rounded-full bg-success text-white shadow-sm transition-transform hover:scale-110 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-input-border-focused disabled:opacity-60"
                      disabled={actions.operationId !== null}
                      onClick={(event) => {
                        event.stopPropagation();
                        actions.onUpdate(item.id);
                      }}
                    >
                      {updating ? (
                        <Loader2 className="size-2.5 animate-spin" aria-hidden="true" />
                      ) : (
                        <Download className="size-2.5" aria-hidden="true" />
                      )}
                    </button>
                  ) : null}
                </div>
              );
            })}
          </div>
        </section>
      ) : null}

      {/* Public / Personal segment pills. */}
      <div className="flex items-center gap-1.5">
        <SegmentPill
          active={segment === 'public'}
          testId="plugin-store-segment-public"
          label="Public"
          onClick={() => onSegmentChange('public')}
        />
        <SegmentPill
          active={segment === 'personal'}
          testId="plugin-store-segment-personal"
          label="Personal"
          onClick={() => onSegmentChange('personal')}
        />
      </div>

      {keyword ? (
        <StoreSection key="search" title={`Search results (${searchResults.length})`}>
          {searchResults.length === 0 ? (
            <p className="rounded-xl border border-dashed border-border px-4 py-3 text-ui-base text-foreground-subtle">
              No plugins match your search
            </p>
          ) : (
            <CardGrid items={searchResults} actions={actions} />
          )}
        </StoreSection>
      ) : segment === 'public' ? (
        <PublicSegment
          actions={actions}
          categoryGroups={categoryGroups}
          expandedGroups={expandedGroups}
          featuredItems={featuredItems}
          loading={loading}
          onToggleGroup={toggleGroup}
        />
      ) : (
        <PersonalSegment
          actions={actions}
          expandedGroups={expandedGroups}
          groups={personalGroups}
          onToggleGroup={toggleGroup}
        />
      )}
    </div>
  );
}

export function SegmentPill({
  active,
  label,
  onClick,
  testId
}: {
  active: boolean;
  label: string;
  onClick: () => void;
  testId: string;
}) {
  return (
    <button
      type="button"
      data-testid={testId}
      aria-pressed={active}
      className={cn(
        'rounded-full px-3 py-1 text-ui-base font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-input-border-focused',
        active ? 'bg-selected text-foreground' : 'text-foreground-subtle hover:bg-hover hover:text-foreground'
      )}
      onClick={onClick}
    >
      {label}
    </button>
  );
}

function StoreSection({
  title,
  children,
  className
}: {
  title: string;
  children: React.ReactNode;
  className?: string;
}) {
  return (
    <section className={className}>
      <div className="pb-2">
        <h2 className="text-ui-lg font-semibold text-foreground">{title}</h2>
      </div>
      <div aria-hidden="true" className="h-px bg-surface" />
      <div className="mt-2">{children}</div>
    </section>
  );
}

function CardGrid({ items, actions }: { items: StorePluginItem[]; actions: PluginStoreActions }) {
  return (
    <div className="grid gap-x-6 gap-y-1 sm:grid-cols-2">
      {items.map((item) => (
        <PluginStoreCard key={item.id} item={item} actions={actions} />
      ))}
    </div>
  );
}

/** Category/marketplace group: fully expanded by default, collapse shows 6 + teaser row. */
function CollapsibleCardGroup({
  groupKey,
  items,
  expanded,
  onToggle,
  actions
}: {
  groupKey: string;
  items: StorePluginItem[];
  expanded: boolean;
  onToggle: (key: string) => void;
  actions: PluginStoreActions;
}) {
  const visible = expanded ? items : items.slice(0, CATEGORY_VISIBLE_LIMIT);
  const hidden = expanded ? [] : items.slice(CATEGORY_VISIBLE_LIMIT);
  const hiddenNames = hidden.slice(0, 2).map((item) => resolveItemDisplayName(item));
  return (
    <div>
      <CardGrid items={visible} actions={actions} />
      {hidden.length > 0 ? (
        <button
          type="button"
          data-testid="plugin-store-group-toggle"
          data-group-key={groupKey}
          className="mt-2 flex w-full min-w-0 items-center gap-2 rounded-xl px-2 py-2 text-left text-ui-base text-foreground-subtle transition-colors hover:bg-hover hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-input-border-focused"
          onClick={() => onToggle(groupKey)}
        >
          <span className="flex shrink-0 items-center gap-1.5" aria-hidden="true">
            {hidden.slice(0, 3).map((item) => (
              <PluginStoreAvatar
                key={item.id}
                src={item.listing?.icon}
                pluginId={item.id}
                className="size-5 rounded-md"
                iconClassName="size-2.5"
              />
            ))}
          </span>
          <span className="min-w-0 truncate">
            {hidden.length > hiddenNames.length
              ? `See ${hiddenNames.join(', ')}, and ${hidden.length - hiddenNames.length} more`
              : `See ${hiddenNames.join(', ')}`}
          </span>
        </button>
      ) : expanded && items.length > CATEGORY_VISIBLE_LIMIT ? (
        <button
          type="button"
          data-testid="plugin-store-group-toggle"
          data-group-key={groupKey}
          className="mt-2 rounded-xl px-2 py-2 text-ui-base text-foreground-subtle transition-colors hover:bg-hover hover:text-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-input-border-focused"
          onClick={() => onToggle(groupKey)}
        >
          Show less
        </button>
      ) : null}
    </div>
  );
}

function PublicSegment({
  actions,
  categoryGroups,
  expandedGroups,
  featuredItems,
  loading,
  onToggleGroup
}: {
  actions: PluginStoreActions;
  categoryGroups: ReturnType<typeof groupItemsByCategory>;
  expandedGroups: Record<string, boolean>;
  featuredItems: StorePluginItem[];
  loading: boolean;
  onToggleGroup: (key: string) => void;
}) {
  const isEmpty = featuredItems.length === 0 && categoryGroups.length === 0;
  if (isEmpty) {
    return (
      <p className="rounded-xl border border-dashed border-border px-4 py-3 text-ui-base text-foreground-subtle">
        {loading ? 'Loading plugins…' : 'No marketplace plugins found'}
      </p>
    );
  }
  return (
    <div>
      {featuredItems.length > 0 ? (
        <StoreSection title="Featured" className="py-4 first:pt-0 last:pb-0">
          <CardGrid items={featuredItems} actions={actions} />
        </StoreSection>
      ) : null}
      {categoryGroups.map((group) => (
        <StoreSection
          key={group.category}
          className="py-4 first:pt-0 last:pb-0"
          title={
            group.category === FALLBACK_CATEGORY
              ? 'Other'
              : KNOWN_CATEGORY_LABELS[group.category] ?? group.category
          }
        >
          <CollapsibleCardGroup
            groupKey={`category:${group.category}`}
            items={group.items}
            expanded={expandedGroups[`category:${group.category}`] ?? true}
            onToggle={onToggleGroup}
            actions={actions}
          />
        </StoreSection>
      ))}
    </div>
  );
}

function PersonalSegment({
  actions,
  expandedGroups,
  groups,
  onToggleGroup
}: {
  actions: PluginStoreActions;
  expandedGroups: Record<string, boolean>;
  groups: PersonalMarketplaceGroup[];
  onToggleGroup: (key: string) => void;
}) {
  if (groups.length === 0) {
    return (
      <p className="rounded-xl border border-dashed border-border px-4 py-3 text-ui-base text-foreground-subtle">
        No plugins from personal sources yet. Use Add to add a marketplace source.
      </p>
    );
  }
  return (
    <div>
      {groups.map((group) => (
        <StoreSection key={group.marketplace} title={group.title} className="py-4 first:pt-0 last:pb-0">
          <CollapsibleCardGroup
            groupKey={`marketplace:${group.marketplace}`}
            items={group.items}
            expanded={expandedGroups[`marketplace:${group.marketplace}`] ?? true}
            onToggle={onToggleGroup}
            actions={actions}
          />
        </StoreSection>
      ))}
    </div>
  );
}
