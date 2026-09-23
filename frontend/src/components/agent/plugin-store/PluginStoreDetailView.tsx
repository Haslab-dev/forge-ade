import React, { useState } from 'react';
import {
  Anchor,
  ArrowRight,
  Bot,
  ChevronRight,
  ExternalLink,
  Loader2,
  MessagesSquare,
  RefreshCw,
  Server,
  Terminal,
  TriangleAlert,
  WandSparkles
} from 'lucide-react';
import { cn } from '../../../lib/utils';
import type { PluginComponentKind, PluginComponentGroup, PluginDescribeResult } from './types';
import type { StorePluginItem } from './pluginStoreListing';
import {
  canUpdatePluginItem,
  isTrustedImageUrl,
  KNOWN_CATEGORY_LABELS,
  resolveItemDescription,
  resolveItemDisplayName,
  resolveLocalizedList,
  resolveStoreCategory
} from './pluginStoreListing';
import { PluginStoreAvatar } from './PluginStoreAvatar';
import {
  PluginStoreInstallButton,
  PluginStoreItemMenu,
  PluginStorePaidPlanBadge,
  type PluginStoreActions
} from './PluginStoreCard';

// Detail section order matches the reference: MCP servers → Skills → Commands → Subagents → Hooks.
const SECTION_ORDER: PluginComponentKind[] = ['mcp', 'skill', 'command', 'agent', 'hook'];

const SECTION_TITLES: Record<PluginComponentKind, string> = {
  mcp: 'MCP servers',
  skill: 'Skills',
  command: 'Commands',
  agent: 'Subagents',
  hook: 'Hooks'
};

const SECTION_ICONS: Record<PluginComponentKind, typeof Server> = {
  mcp: Server,
  skill: WandSparkles,
  command: Terminal,
  agent: Bot,
  hook: Anchor
};

type ExamplePromptDocumentIcon = 'documents' | 'pdf' | 'spreadsheets';

function resolveExamplePromptDocumentIcon(prompt: string): ExamplePromptDocumentIcon {
  if (/\bpdf\b/iu.test(prompt)) return 'pdf';
  if (/\b(?:csv|xlsx|excel)\b/iu.test(prompt)) return 'spreadsheets';
  return 'documents';
}

function DocumentIcon({ kind }: { kind: ExamplePromptDocumentIcon }) {
  if (kind === 'pdf') {
    return (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" className="size-7 shrink-0 text-white/80">
        <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
        <path d="M14 2v6h6" />
        <path d="M9 15h6" />
        <path d="M12 12v6" />
      </svg>
    );
  }
  if (kind === 'spreadsheets') {
    return (
      <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" className="size-7 shrink-0 text-white/80">
        <rect x="3" y="3" width="18" height="18" rx="2" />
        <path d="M3 9h18" />
        <path d="M3 15h18" />
        <path d="M9 3v18" />
        <path d="M15 3v18" />
      </svg>
    );
  }
  return (
    <svg viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth="1.5" className="size-7 shrink-0 text-white/80">
      <path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8z" />
      <path d="M14 2v6h6" />
    </svg>
  );
}

export function PluginStoreDetailView({
  item,
  actions,
  describeEntry,
  onRetryDescribe,
  advanced
}: {
  item: StorePluginItem;
  actions: PluginStoreActions;
  describeEntry?: { status: 'loading' | 'loaded' | 'error'; data?: PluginDescribeResult };
  onRetryDescribe: () => void;
  advanced?: React.ReactNode;
}) {
  const displayName = resolveItemDisplayName(item);
  const description = resolveItemDescription(item);
  const heroImage = item.listing?.heroImage;
  const locale = 'en-US';
  const examplePrompts =
    resolveLocalizedList(locale, item.listing?.examplePrompts, undefined) ?? [];
  const describeMetadata = describeEntry?.status === 'loaded' ? describeEntry.data?.metadata : undefined;

  // Installed and candidate entries both render from describe data; the page
  // describes installed plugins via their root dir on first open.
  const componentGroups: PluginComponentGroup[] =
    describeEntry?.status === 'loaded' && describeEntry.data ? describeEntry.data.components : [];
  const orderedGroups = SECTION_ORDER.map((kind) =>
    componentGroups.find((group) => group.kind === kind)
  ).filter((group): group is PluginComponentGroup => group !== undefined);
  const componentsLoading = (describeEntry?.status ?? 'loading') === 'loading';
  const componentsFailed = describeEntry?.status === 'error';

  return (
    <div className="space-y-8" data-testid="plugin-store-detail" data-plugin-id={item.id}>
      {/* Header: large icon above, name + menu/action row, description below. */}
      <div className="space-y-3">
        <PluginStoreAvatar
          src={item.listing?.icon}
          pluginId={item.id}
          className="size-16 rounded-2xl"
          iconClassName="size-6"
        />
        <div
          className="flex min-w-0 items-center justify-between gap-3"
          data-testid="plugin-store-title-actions"
        >
          <div className="flex min-w-0 items-center gap-2">
            <h1 className="min-w-0 truncate text-2xl font-semibold tracking-tight text-foreground">
              {displayName}
            </h1>
            <PluginStorePaidPlanBadge item={item} />
          </div>
          <div className="flex shrink-0 items-center gap-1.5">
            {item.installed ? (
              <>
                <PluginStoreItemMenu item={item} actions={actions} triggerVariant="outline" />
                <button
                  type="button"
                  data-testid="plugin-store-try-now"
                  data-plugin-id={item.id}
                  className="inline-flex h-9 items-center justify-center gap-1.5 whitespace-nowrap rounded-lg bg-brand px-4 text-ui-sm font-medium text-foreground-inverse transition-colors hover:bg-brand/80"
                >
                  <MessagesSquare className="size-3.5" aria-hidden="true" />
                  Try now
                </button>
              </>
            ) : (
              <PluginStoreInstallButton item={item} actions={actions} size="lg" />
            )}
          </div>
        </div>
        {description ? (
          <p data-testid="plugin-store-description" className="text-ui-base text-foreground-subtle">
            {description}
          </p>
        ) : null}
        {item.orphaned ? (
          <p
            data-testid="plugin-store-source-degraded"
            data-plugin-id={item.id}
            className="flex items-center gap-1.5 text-ui-base text-warning"
          >
            <TriangleAlert className="size-3.5 shrink-0" aria-hidden="true" />
            Marketplace source is missing. This plugin remains usable, but updates are unavailable.
          </p>
        ) : null}
      </div>

      {/* Hero: banner image with stacked example-prompt pills; prompts-only or nothing. */}
      {isTrustedImageUrl(heroImage) || examplePrompts.length > 0 ? (
        <HeroSection
          item={item}
          displayName={displayName}
          examplePrompts={examplePrompts}
          heroImage={heroImage}
        />
      ) : null}

      {/* Component sections. */}
      {componentsLoading ? (
        <div
          className="flex items-center gap-2 py-2 text-ui-base text-foreground-subtle"
          data-testid="plugin-store-components-loading"
        >
          <Loader2 className="size-3.5 animate-spin" aria-hidden="true" />
          <span>Loading components…</span>
        </div>
      ) : componentsFailed ? (
        <div
          className="flex items-center justify-between gap-2 rounded-xl border border-warning/30 bg-warning/10 px-3 py-2 text-ui-base text-foreground-subtle"
          data-testid="plugin-store-components-error"
        >
          <span>Couldn't load the component list. Showing available info only.</span>
          <button
            type="button"
            className="inline-flex h-8 items-center justify-center gap-1.5 rounded-lg border border-border px-3 text-ui-sm font-medium text-foreground transition-colors hover:bg-surface-hover"
            data-testid="plugin-store-components-retry"
            onClick={onRetryDescribe}
          >
            <RefreshCw className="size-3" aria-hidden="true" />
            Retry
          </button>
        </div>
      ) : (
        orderedGroups.map((group) => <ComponentSection key={group.kind} group={group} />)
      )}

      <InfoSection item={item} describeMetadata={describeMetadata} />

      {advanced}
    </div>
  );
}

function HeroSection({
  item,
  displayName,
  examplePrompts,
  heroImage
}: {
  item: StorePluginItem;
  displayName: string;
  examplePrompts: string[];
  heroImage?: string;
}) {
  const [heroFailed, setHeroFailed] = useState(false);
  const showHeroImage = isTrustedImageUrl(heroImage) && !heroFailed;
  const prompts = examplePrompts.map((prompt) => {
    const documentIcon = resolveExamplePromptDocumentIcon(prompt);
    return (
      <button
        key={prompt}
        type="button"
        data-testid="plugin-store-example-prompt"
        data-plugin-id={item.id}
        className="group/prompt flex w-fit max-w-2xl items-center gap-2.5 rounded-3xl bg-black/75 px-3.5 py-2.5 text-left text-ui-base text-white shadow-lg backdrop-blur-md transition-colors hover:bg-black/85 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-white/70"
      >
        <span className="shrink-0">
          <DocumentIcon kind={documentIcon} />
        </span>
        <span className="shrink-0 font-medium text-white/70">{displayName}</span>
        <span className="min-w-0 whitespace-normal">{prompt}</span>
        <span
          aria-hidden="true"
          className="flex size-9 shrink-0 items-center justify-center rounded-full bg-white/15 transition-colors group-hover/prompt:bg-white/25"
        >
          <ArrowRight className="size-4" />
        </span>
      </button>
    );
  });

  if (!showHeroImage && prompts.length === 0) return null;

  return (
    <div
      className="relative min-h-72 overflow-hidden rounded-3xl sm:aspect-[3/1] sm:min-h-0"
      data-testid="plugin-store-hero"
    >
      <div
        aria-hidden="true"
        className="pointer-events-none absolute inset-0 bg-[radial-gradient(120%_140%_at_20%_0%,color-mix(in_oklab,var(--color-brand)_28%,transparent),transparent_60%)]"
      />
      {showHeroImage ? (
        <img
          src={heroImage}
          alt=""
          aria-hidden="true"
          draggable={false}
          className="absolute inset-0 size-full select-none object-cover opacity-80"
          onError={() => setHeroFailed(true)}
        />
      ) : null}
      <div className="absolute inset-0 bg-black/15" data-testid="plugin-store-hero-overlay" aria-hidden="true" />
      {prompts.length > 0 ? (
        <div className="absolute inset-0 flex flex-col items-center justify-center gap-4 p-5 sm:p-8">
          {prompts}
        </div>
      ) : null}
    </div>
  );
}

function ComponentSection({ group }: { group: PluginComponentGroup }) {
  const Icon = SECTION_ICONS[group.kind];
  return (
    <section data-testid="plugin-store-component-section" data-component-kind={group.kind}>
      <div className="flex items-baseline gap-2 border-b border-border pb-2">
        <h2 className="text-ui-lg font-semibold text-foreground">{SECTION_TITLES[group.kind]}</h2>
        <span className="text-ui-base text-foreground-subtle">{group.items.length}</span>
      </div>
      <ul className="mt-1 divide-y divide-border/60">
        {group.items.map((componentItem) => (
          <li key={componentItem.name} className="flex min-w-0 items-center gap-3 py-2.5">
            <span
              aria-hidden="true"
              className="flex size-7 shrink-0 items-center justify-center rounded-lg bg-surface text-foreground-subtle"
            >
              <Icon className="size-4" />
            </span>
            <div className="min-w-0 flex-1">
              <div className="truncate text-ui-base font-medium text-foreground">
                {componentItem.name}
              </div>
              {componentItem.description ? (
                <div className="mt-0.5 line-clamp-1 text-ui-base text-foreground-subtle">
                  {componentItem.description}
                </div>
              ) : null}
            </div>
          </li>
        ))}
      </ul>
    </section>
  );
}

function InfoSection({
  item,
  describeMetadata
}: {
  item: StorePluginItem;
  describeMetadata?: { name?: string; description?: string; version?: string; author?: string; homepage?: string };
}) {
  const listing = item.listing;
  const developer = listing?.author ?? item.info?.author ?? describeMetadata?.author;
  const category = resolveStoreCategory(listing?.category);
  const categoryLabel = category ? (KNOWN_CATEGORY_LABELS[category] ?? category) : undefined;
  const version =
    item.info?.version ?? item.installedMeta?.version ?? item.summary?.version ?? describeMetadata?.version;
  const website = pickHttpsUrl(listing?.homepage, item.info ? undefined : undefined, describeMetadata?.homepage);

  const rows: Array<{ key: string; label: string; value: React.ReactNode }> = [];
  if (developer) rows.push({ key: 'developer', label: 'Developer', value: developer });
  if (categoryLabel) rows.push({ key: 'category', label: 'Category', value: categoryLabel });
  if (version) rows.push({ key: 'version', label: 'Version', value: version });

  const linkRows: Array<{ key: string; label: string; url: string }> = [];
  if (website) linkRows.push({ key: 'website', label: 'Website', url: website });
  if (listing?.privacyPolicy && listing.privacyPolicy.startsWith('https://')) {
    linkRows.push({ key: 'privacyPolicy', label: 'Privacy policy', url: listing.privacyPolicy });
  }
  if (listing?.termsOfService && listing.termsOfService.startsWith('https://')) {
    linkRows.push({ key: 'termsOfService', label: 'Terms of service', url: listing.termsOfService });
  }
  if (rows.length === 0 && linkRows.length === 0) return null;

  return (
    <section>
      <div className="border-b border-border pb-2">
        <h2 className="text-ui-lg font-semibold text-foreground">Information</h2>
      </div>
      <dl className="mt-3 space-y-2.5">
        {rows.map((row) => (
          <div key={row.key} className="grid grid-cols-[8rem_minmax(0,1fr)] items-baseline gap-3">
            <dt className="text-ui-base text-foreground-subtle">{row.label}</dt>
            <dd className="min-w-0 truncate text-ui-base text-foreground">{row.value}</dd>
          </div>
        ))}
        {linkRows.map((row) => (
          <div key={row.key} className="grid grid-cols-[8rem_minmax(0,1fr)] items-baseline gap-3">
            <dt className="text-ui-base text-foreground-subtle">{row.label}</dt>
            <dd className="min-w-0">
              <button
                type="button"
                title={row.url}
                aria-label={`${row.label}: ${row.url}`}
                className="inline-flex items-center gap-1 rounded-md text-ui-base text-foreground transition-colors hover:text-brand focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-input-border-focused"
                onClick={() => void window.open(row.url, '_blank', 'noopener')}
              >
                <ExternalLink className="size-3.5" aria-hidden="true" />
              </button>
            </dd>
          </div>
        ))}
      </dl>
    </section>
  );
}

function pickHttpsUrl(...candidates: Array<string | undefined>): string | undefined {
  return candidates.find(
    (candidate): candidate is string => typeof candidate === 'string' && candidate.startsWith('https://')
  );
}

/** Advanced fold at the bottom of the detail page (parent injects content). */
export function PluginStoreAdvancedSection({ children }: { children: React.ReactNode }) {
  return (
    <details className="group/advanced border-t border-border pt-4" data-testid="plugin-store-advanced">
      <summary className="flex cursor-pointer list-none items-center gap-2 text-ui-base font-medium text-foreground-subtle transition-colors hover:text-foreground">
        <ChevronRight
          className={cn('size-3.5 transition-transform', 'group-open/advanced:rotate-90')}
          aria-hidden="true"
        />
        More details
      </summary>
      <div className="mt-3 space-y-3">{children}</div>
    </details>
  );
}
