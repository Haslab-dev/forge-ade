import React, { useState, useRef, useEffect } from 'react';
import {
  Crown,
  Download,
  Loader2,
  MoreHorizontal,
  Power,
  TriangleAlert,
  Trash2
} from 'lucide-react';
import { cn } from '../../../lib/utils';
import type { StorePluginItem } from './pluginStoreListing';
import { canUpdatePluginItem, resolveItemDescription, resolveItemDisplayName } from './pluginStoreListing';
import { PluginStoreAvatar } from './PluginStoreAvatar';

/** Shared action set for list cards and the detail page. */
export interface PluginStoreActions {
  onOpenDetail: (pluginId: string) => void;
  onInstall: (item: StorePluginItem) => void;
  onUninstall: (pluginId: string) => void;
  onUpdate: (pluginId: string) => void;
  onSetEnabled?: (pluginId: string, enabled: boolean) => void;
  operationId: string | null;
}

function isItemBusy(item: StorePluginItem, actions: PluginStoreActions): boolean {
  return (
    actions.operationId === `plugin:install:${item.name}@${item.marketplace}` ||
    actions.operationId === `plugin:uninstall:${item.id}` ||
    actions.operationId === `plugin:update:${item.id}`
  );
}

/** "…" menu shared by cards and the detail header (ZCode PluginStoreItemMenu). */
export function PluginStoreItemMenu({
  item,
  actions,
  triggerClassName,
  triggerVariant = 'ghost'
}: {
  item: StorePluginItem;
  actions: PluginStoreActions;
  triggerClassName?: string;
  triggerVariant?: 'ghost' | 'outline';
}) {
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

  const enabled = item.info?.enabled ?? false;
  const updatePending = canUpdatePluginItem(item);
  const busy = isItemBusy(item, actions);
  const hasActionsBeforeUninstall = updatePending;

  const triggerClasses = cn(
    'flex h-8 w-8 items-center justify-center rounded-lg transition-colors',
    triggerVariant === 'outline'
      ? 'border border-border text-foreground hover:bg-surface-hover'
      : 'text-foreground-subtle hover:bg-surface-hover hover:text-foreground',
    triggerClassName
  );

  return (
    <div className="relative" ref={ref}>
      <button
        type="button"
        data-testid="plugin-store-item-menu"
        data-plugin-id={item.id}
        aria-label="More actions"
        className={triggerClasses}
        onClick={(event) => {
          event.stopPropagation();
          setOpen((prev) => !prev);
        }}
      >
        {busy ? (
          <Loader2 className="size-4 animate-spin" aria-hidden="true" />
        ) : (
          <MoreHorizontal className="size-4" aria-hidden="true" />
        )}
      </button>
      {open ? (
        <div
          role="menu"
          className="absolute right-0 top-full z-50 mt-1 min-w-44 overflow-hidden rounded-xl border border-border bg-popover p-1 shadow-lg"
          onClick={(event) => event.stopPropagation()}
        >
          {updatePending ? (
            <MenuRow
              testId="plugin-store-menu-update"
              disabled={busy}
              icon={<Download className="size-4" aria-hidden="true" />}
              label="Update"
              onSelect={() => {
                setOpen(false);
                actions.onUpdate(item.id);
              }}
            />
          ) : null}
          {item.info && actions.onSetEnabled ? (
            <MenuRow
              testId="plugin-store-menu-enabled"
              disabled={busy}
              icon={<Power className="size-4" aria-hidden="true" />}
              label={enabled ? 'Disable' : 'Enable'}
              onSelect={() => {
                setOpen(false);
                actions.onSetEnabled?.(item.info!.id, !enabled);
              }}
            />
          ) : null}
          {item.installed ? (
            <>
              {hasActionsBeforeUninstall ? <div className="mx-1 my-1 h-px bg-border" /> : null}
              <MenuRow
                testId="plugin-store-menu-uninstall"
                disabled={busy}
                destructive
                icon={<Trash2 className="size-4" aria-hidden="true" />}
                label="Uninstall"
                onSelect={() => {
                  setOpen(false);
                  actions.onUninstall(item.id);
                }}
              />
            </>
          ) : null}
        </div>
      ) : null}
    </div>
  );
}

function MenuRow({
  label,
  icon,
  onSelect,
  disabled,
  destructive,
  testId
}: {
  label: string;
  icon: React.ReactNode;
  onSelect: () => void;
  disabled?: boolean;
  destructive?: boolean;
  testId?: string;
}) {
  return (
    <button
      type="button"
      role="menuitem"
      data-testid={testId}
      disabled={disabled}
      className={cn(
        'flex w-full items-center gap-2 rounded-lg px-2 py-1.5 text-left text-ui-sm text-foreground transition-colors hover:bg-surface-hover disabled:opacity-50',
        destructive && 'text-destructive hover:bg-destructive/10'
      )}
      onClick={onSelect}
    >
      {icon}
      {label}
    </button>
  );
}

/** Install pill for not-installed entries (shared by card + detail). */
export function PluginStoreInstallButton({
  item,
  actions,
  size = 'sm'
}: {
  item: StorePluginItem;
  actions: PluginStoreActions;
  size?: 'sm' | 'lg';
}) {
  const installing =
    actions.operationId === `plugin:install:${item.name}@${item.marketplace}`;
  return (
    <button
      type="button"
      data-testid="plugin-store-install"
      data-plugin-id={item.id}
      disabled={installing}
      className={cn(
        'inline-flex shrink-0 items-center justify-center gap-1.5 whitespace-nowrap rounded-full bg-surface-hover font-medium text-foreground transition-colors hover:bg-selected disabled:opacity-60',
        size === 'lg' ? 'h-9 px-4 text-ui-sm' : 'h-7 px-3 text-ui-sm'
      )}
      onClick={(event) => {
        event.stopPropagation();
        actions.onInstall(item);
      }}
    >
      {installing ? <Loader2 className="size-3.5 animate-spin" aria-hidden="true" /> : null}
      {installing ? 'Installing…' : 'Install'}
    </button>
  );
}

/** "Update" corner/inline badge for entries with pending updates. */
export function PluginStoreUpdateBadge({
  item
}: {
  item: Pick<StorePluginItem, 'id' | 'installedMeta' | 'orphaned'> | null | undefined;
}) {
  if (!canUpdatePluginItem(item)) return null;
  return (
    <span
      data-testid="plugin-store-update-badge"
      data-plugin-id={item?.id}
      className="inline-flex shrink-0 items-center gap-1 whitespace-nowrap rounded-full bg-success/14 px-1.5 py-0.5 text-ui-sm leading-none text-success dark:bg-success/18"
      aria-label="Update"
    >
      <Download className="size-3" aria-hidden="true" />
      Update
    </span>
  );
}

/** Inline "Update" pill on installed+updatable cards. */
export function PluginStoreUpdateButton({
  item,
  actions,
  size = 'sm'
}: {
  item: StorePluginItem | null | undefined;
  actions: PluginStoreActions;
  size?: 'sm' | 'lg';
}) {
  if (!item || !canUpdatePluginItem(item)) return null;
  const updating = actions.operationId === `plugin:update:${item.id}`;
  return (
    <button
      type="button"
      data-testid="plugin-store-card-update"
      data-plugin-id={item.id}
      disabled={actions.operationId !== null}
      className={cn(
        'inline-flex shrink-0 items-center justify-center gap-1.5 whitespace-nowrap rounded-full bg-surface-hover font-medium text-foreground transition-colors hover:bg-selected disabled:opacity-60',
        size === 'lg' ? 'h-9 px-4 text-ui-sm' : 'h-7 px-3 text-ui-sm'
      )}
      onClick={(event) => {
        event.stopPropagation();
        actions.onUpdate(item.id);
      }}
    >
      {updating ? (
        <Loader2 className="size-3.5 animate-spin" aria-hidden="true" />
      ) : (
        <Download className="size-3.5" aria-hidden="true" />
      )}
      Update
    </button>
  );
}

/** Gradient "Coding Plan" badge when listing.requiresPaidPlan is set. */
export function PluginStorePaidPlanBadge({
  item
}: {
  item: Pick<StorePluginItem, 'id' | 'listing'>;
}) {
  if (!item.listing?.requiresPaidPlan) return null;
  return (
    <span
      title="This plugin works better with a Coding Plan"
      aria-label="This plugin works better with a Coding Plan"
      data-testid="plugin-store-paid-plan-badge"
      data-plugin-id={item.id}
      className="inline-flex shrink-0 items-center gap-1 whitespace-nowrap rounded-full bg-[var(--color-plugin-paid-plan-badge,linear-gradient(90deg,#8b5cf6,#ec4899))] px-1.5 py-0.5 text-ui-sm leading-none text-white"
    >
      <Crown className="size-3" aria-hidden="true" />
      Coding Plan
    </span>
  );
}

/**
 * Store card (two-column grid cell): 40px avatar + display name + one-line
 * truncated description; installed → update pill + "…" menu, otherwise Install.
 */
export function PluginStoreCard({
  item,
  actions
}: {
  item: StorePluginItem;
  actions: PluginStoreActions;
}) {
  const displayName = resolveItemDisplayName(item);
  const description = resolveItemDescription(item);
  return (
    <div
      role="button"
      tabIndex={0}
      data-testid="plugin-store-card"
      data-plugin-id={item.id}
      className="group/card flex min-w-0 cursor-pointer items-center gap-3 rounded-xl px-2 py-2.5 transition-colors hover:bg-hover focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-input-border-focused"
      onClick={() => actions.onOpenDetail(item.id)}
      onKeyDown={(event) => {
        if (event.key === 'Enter' || event.key === ' ') {
          event.preventDefault();
          actions.onOpenDetail(item.id);
        }
      }}
    >
      <PluginStoreAvatar src={item.listing?.icon} pluginId={item.id} className="size-10" />
      <div className="min-w-0 flex-1">
        <div className="flex min-w-0 items-center gap-1.5">
          <span className="min-w-0 truncate text-ui-base font-semibold text-foreground">
            {displayName}
          </span>
          <PluginStorePaidPlanBadge item={item} />
          <PluginStoreUpdateBadge item={item} />
        </div>
        {item.orphaned ? (
          <div
            data-testid="plugin-store-source-degraded"
            data-plugin-id={item.id}
            className="mt-0.5 flex items-center gap-1 truncate text-ui-sm text-warning"
          >
            <TriangleAlert className="size-3 shrink-0" aria-hidden="true" />
            <span className="truncate">
              Marketplace source is missing. This plugin remains usable, but updates are unavailable.
            </span>
          </div>
        ) : null}
        {description ? (
          <div className="mt-0.5 truncate text-ui-sm text-foreground-subtle">{description}</div>
        ) : null}
      </div>
      <div className="flex shrink-0 items-center gap-1.5">
        {item.installed ? (
          <>
            <PluginStoreUpdateButton item={item} actions={actions} />
            <PluginStoreItemMenu item={item} actions={actions} />
          </>
        ) : (
          <PluginStoreInstallButton item={item} actions={actions} />
        )}
      </div>
    </div>
  );
}
