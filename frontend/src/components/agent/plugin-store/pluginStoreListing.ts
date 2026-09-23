/**
 * Store view-model layer: joins catalog entries, installed records and runtime
 * plugin info into a single StorePluginItem per plugin, plus grouping/sorting/
 * search helpers (ported 1:1 from ZCode settings/pluginStoreListing.ts).
 */
import type {
  PluginAvailableSummary,
  PluginInstalledMeta,
  PluginMarketplaceSummary,
  PluginRuntimeInfo,
  PluginStoreListing
} from './types';
import { isPublicStoreMarketplaceId } from './types';

export { isPublicStoreMarketplaceId };

export interface StorePluginItem {
  id: string; // name@marketplace
  name: string;
  marketplace: string;
  installed: boolean;
  restorable: boolean;
  /** installed but its marketplace source is gone */
  orphaned: boolean;
  listing?: PluginStoreListing;
  summary?: PluginAvailableSummary;
  info?: PluginRuntimeInfo;
  installedMeta?: PluginInstalledMeta;
}

export type PluginUpdateStatus = 'none' | 'update-available';

export function isPluginUpdatePending(
  updateStatus: PluginUpdateStatus | undefined
): updateStatus is Exclude<PluginUpdateStatus, 'none'> {
  return updateStatus === 'update-available';
}

export function canUpdatePluginItem(
  item: Pick<StorePluginItem, 'installedMeta' | 'orphaned'> | null | undefined
): boolean {
  return Boolean(item && !item.orphaned && isPluginUpdatePending(item.installedMeta?.updateStatus));
}

export function resolveItemDisplayName(item: StorePluginItem): string {
  return item.listing?.displayName || item.info?.name || item.name;
}

export function resolveItemDescription(item: StorePluginItem): string | undefined {
  return (
    item.summary?.description ??
    item.info?.description ??
    item.installedMeta?.description ??
    item.listing?.descriptionI18n?.en
  );
}

export function resolveLocalizedList(
  locale: string,
  base: string[] | undefined,
  i18n: Record<string, string[]> | undefined
): string[] | undefined {
  if (i18n) {
    const exact = i18n[locale];
    if (exact && exact.length > 0) return exact;
    const language = locale.split('-')[0];
    if (language) {
      const match = Object.entries(i18n).find(([key]) => key.split('-')[0] === language);
      if (match?.[1] && match[1].length > 0) return match[1];
    }
  }
  return base && base.length > 0 ? base : undefined;
}

export const KNOWN_CATEGORY_LABELS: Record<string, string> = {
  'developer-tools': 'Developer Tools',
  productivity: 'Productivity',
  utilities: 'Utilities',
  legal: 'Legal',
  template: 'Templates',
  finance: 'Finance',
  other: 'Other'
};

export const FALLBACK_CATEGORY = 'other';

const CATEGORY_ORDER = [
  'developer-tools',
  'productivity',
  'utilities',
  'legal',
  'template',
  'finance',
  FALLBACK_CATEGORY
];

export function resolveStoreCategory(category?: string): string | undefined {
  const trimmed = category?.trim();
  return trimmed ? trimmed : undefined;
}

export function resolveMarketplaceDisplayName(
  marketplace: string,
  marketplaces: PluginMarketplaceSummary[]
): string {
  const found = marketplaces.find((m) => m.id === marketplace);
  if (!found) return marketplace;
  return found.name || marketplace;
}

export interface StoreCategoryGroup {
  category: string;
  items: StorePluginItem[];
}

export interface PersonalMarketplaceGroup {
  marketplace: string;
  title: string;
  items: StorePluginItem[];
}

const OFFICIAL_MARKETPLACE_ORDER: readonly string[] = ['zcode-plugins-official'];

/** Sources dialog order: official pinned first, then most-recently-updated. */
export function sortMarketplaceSources(
  marketplaces: PluginMarketplaceSummary[]
): PluginMarketplaceSummary[] {
  return [...marketplaces].sort((left, right) => {
    const leftRank = OFFICIAL_MARKETPLACE_ORDER.indexOf(left.id);
    const rightRank = OFFICIAL_MARKETPLACE_ORDER.indexOf(right.id);
    if (leftRank !== -1 || rightRank !== -1) {
      if (leftRank === -1) return 1;
      if (rightRank === -1) return -1;
      return leftRank - rightRank;
    }
    const leftAt = left.lastUpdated ?? '';
    const rightAt = right.lastUpdated ?? '';
    if (leftAt !== rightAt) return rightAt.localeCompare(leftAt);
    return left.name.localeCompare(right.name);
  });
}

/** Personal segment groups: most recently refreshed marketplace first. */
export function sortPersonalMarketplaceGroups(
  groups: PersonalMarketplaceGroup[],
  marketplaces: PluginMarketplaceSummary[]
): PersonalMarketplaceGroup[] {
  const lastUpdatedById = new Map(
    marketplaces.flatMap((m) => (m.lastUpdated ? [[m.id, m.lastUpdated] as const] : []))
  );
  return [...groups].sort((left, right) => {
    const leftAt = lastUpdatedById.get(left.marketplace) ?? '';
    const rightAt = lastUpdatedById.get(right.marketplace) ?? '';
    if (leftAt !== rightAt) return rightAt.localeCompare(leftAt);
    return left.title.localeCompare(right.title);
  });
}

/**
 * Join overview payloads into store items. Universe =
 * available catalog entries ∪ installed records ∪ discovered runtime plugins.
 */
export function buildStoreItems(input: {
  marketplaces: PluginMarketplaceSummary[];
  marketplaceAvailabilityKnown: boolean;
  availablePlugins: PluginAvailableSummary[];
  installedPlugins: PluginInstalledMeta[];
  plugins: PluginRuntimeInfo[];
}): StorePluginItem[] {
  const infoById = new Map(input.plugins.map((p) => [p.id, p]));
  const infoByStableId = new Map(
    input.plugins.map((p) => [`${p.name}@${p.marketplace || ''}`, p])
  );
  const metaById = new Map(input.installedPlugins.map((m) => [m.id, m]));
  const marketplaceIds = new Set(input.marketplaces.map((m) => m.id));
  const items = new Map<string, StorePluginItem>();

  for (const summary of input.availablePlugins) {
    const info = infoById.get(summary.id) ?? infoByStableId.get(summary.id);
    items.set(summary.id, {
      id: summary.id,
      name: summary.name,
      marketplace: summary.marketplace,
      installed: summary.installed || info !== undefined,
      restorable: false,
      orphaned: false,
      ...(summary.listing ? { listing: summary.listing } : {}),
      summary,
      ...(info ? { info } : {}),
      ...(metaById.get(summary.id) ? { installedMeta: metaById.get(summary.id) } : {})
    });
  }

  // Runtime-discovered marketplace-installed plugins missing from catalogs
  // (inline or removed-source installs) remain visible and searchable.
  for (const info of input.plugins) {
    if (!info.marketplace) continue;
    const id = `${info.name}@${info.marketplace}`;
    const existing = items.get(id);
    if (existing) {
      if (!existing.info) existing.info = info;
      continue;
    }
    items.set(id, {
      id,
      name: info.name,
      marketplace: info.marketplace,
      installed: true,
      restorable: false,
      orphaned:
        input.marketplaceAvailabilityKnown &&
        info.source === 'global' &&
        !marketplaceIds.has(info.marketplace),
      info,
      ...(metaById.get(id) ? { installedMeta: metaById.get(id) } : {})
    });
  }

  return [...items.values()];
}

/** Public segment: featured (catalog curated order) then category groups. */
export function selectFeaturedItems(
  publicItems: StorePluginItem[],
  marketplaces: PluginMarketplaceSummary[]
): StorePluginItem[] {
  const byName = new Map<string, StorePluginItem>();
  for (const item of publicItems) {
    if (!byName.has(item.name)) byName.set(item.name, item);
  }
  const featured: StorePluginItem[] = [];
  const seen = new Set<string>();
  for (const marketplace of marketplaces) {
    if (!isPublicStoreMarketplaceId(marketplace.id)) continue;
    for (const name of marketplace.featured ?? []) {
      const item = byName.get(name);
      if (item && !seen.has(item.id)) {
        seen.add(item.id);
        featured.push(item);
      }
    }
  }
  return featured;
}

export function groupItemsByCategory(items: StorePluginItem[]): StoreCategoryGroup[] {
  const groups = new Map<string, StorePluginItem[]>();
  const sorted = [...items].sort((a, b) =>
    resolveItemDisplayName(a).localeCompare(resolveItemDisplayName(b))
  );
  for (const item of sorted) {
    const category = resolveStoreCategory(item.listing?.category) ?? FALLBACK_CATEGORY;
    const group = groups.get(category) ?? [];
    group.push(item);
    groups.set(category, group);
  }
  return [...groups.entries()]
    .map(([category, grouped]) => ({ category, items: grouped }))
    .sort((a, b) => {
      const ai = CATEGORY_ORDER.indexOf(a.category);
      const bi = CATEGORY_ORDER.indexOf(b.category);
      const aRank = ai === -1 ? CATEGORY_ORDER.length - 1 : ai;
      const bRank = bi === -1 ? CATEGORY_ORDER.length - 1 : bi;
      return aRank - bRank;
    });
}

export function storeItemMatches(item: StorePluginItem, keyword: string): boolean {
  const haystacks = [
    item.name,
    item.id,
    item.marketplace,
    resolveItemDisplayName(item),
    resolveItemDescription(item) ?? '',
    item.listing?.category ?? '',
    item.listing?.author ?? ''
  ];
  return haystacks.some((value) => value.toLowerCase().includes(keyword));
}

/** Installed strip order: most recently installed first, then by name. */
export function sortInstalledStripItems(items: StorePluginItem[]): StorePluginItem[] {
  return [...items].sort((left, right) => {
    const leftAt = left.installedMeta?.installedAt ?? '';
    const rightAt = right.installedMeta?.installedAt ?? '';
    if (leftAt !== rightAt) return rightAt.localeCompare(leftAt);
    return resolveItemDisplayName(left).localeCompare(resolveItemDisplayName(right));
  });
}

/** Only allow https image sources in icon/hero slots. */
export function isTrustedImageUrl(url: string | undefined): url is string {
  return typeof url === 'string' && url.startsWith('https://');
}
