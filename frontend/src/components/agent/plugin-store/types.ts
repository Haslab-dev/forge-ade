/**
 * Types mirroring the Go `internal/marketplace` payloads (Wails bindings).
 */

export interface PluginStoreListing {
  displayName?: string;
  icon?: string;
  category?: string;
  author?: string;
  authorUrl?: string;
  homepage?: string;
  privacyPolicy?: string;
  termsOfService?: string;
  heroImage?: string;
  examplePrompts?: string[];
  requiresPaidPlan?: boolean;
  descriptionI18n?: Record<string, string>;
}

export interface PluginMarketplaceSummary {
  id: string;
  name: string;
  source: string;
  description?: string;
  lastUpdated?: string;
  pluginCount: number;
  isOfficial?: boolean;
  featured?: string[];
  refreshFailure?: {
    code: string;
    failedAt: string;
    message: string;
  };
  available?: boolean;
}

export interface PluginAvailableSummary {
  id: string; // name@marketplace
  name: string;
  marketplace: string;
  description?: string;
  version?: string;
  installed: boolean;
  componentTypes?: string[];
  listing?: PluginStoreListing;
}

export interface PluginInstalledMeta {
  id: string; // name@marketplace
  name: string;
  marketplace: string;
  version?: string;
  description?: string;
  installedAt?: string;
  updateStatus?: 'none' | 'update-available';
  latestVersion?: string;
}

export interface PluginRuntimeInfo {
  id: string;
  name: string;
  description?: string;
  version?: string;
  author?: string;
  enabled: boolean;
  source: string; // builtin | global | workspace | runtime
  marketplace?: string;
  rootPath?: string;
  dir?: string;
}

export interface PluginStoreOverview {
  marketplaces: PluginMarketplaceSummary[];
  availablePlugins: PluginAvailableSummary[];
  installedPlugins: PluginInstalledMeta[];
  plugins: PluginRuntimeInfo[];
  marketplaceAvailabilityKnown: boolean;
  diagnostics?: string[];
}

export type PluginComponentKind = 'mcp' | 'skill' | 'command' | 'agent' | 'hook';

export interface PluginComponentItem {
  name: string;
  description?: string;
}

export interface PluginComponentGroup {
  kind: PluginComponentKind;
  items: PluginComponentItem[];
}

export interface PluginDescribeMetadata {
  name?: string;
  description?: string;
  version?: string;
  author?: string;
  homepage?: string;
}

export interface PluginDescribeResult {
  metadata: PluginDescribeMetadata;
  components: PluginComponentGroup[];
}

/** The single public-store marketplace id (mirrors Go OfficialMarketplaceID). */
export const OFFICIAL_MARKETPLACE_ID = 'zcode-plugins-official';

export function isPublicStoreMarketplaceId(id: string): boolean {
  return id === OFFICIAL_MARKETPLACE_ID;
}
