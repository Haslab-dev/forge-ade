import React, { useState, useRef } from 'react';
import * as DialogPrimitive from '@radix-ui/react-dialog';
import { AlertTriangle, Loader2, Plus, RefreshCw, Trash2 } from 'lucide-react';
import type { PluginMarketplaceSummary } from './types';
import { sortMarketplaceSources } from './pluginStoreListing';

function StoreDialog({
  open,
  onOpenChange,
  widthClassName,
  children
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  widthClassName: string;
  children: React.ReactNode;
}) {
  return (
    <DialogPrimitive.Root open={open} onOpenChange={onOpenChange}>
      <DialogPrimitive.Portal>
        <DialogPrimitive.Overlay className="fixed inset-0 z-50 bg-black/50 backdrop-blur-[2px] data-[state=open]:animate-in data-[state=open]:fade-in-0" />
        <DialogPrimitive.Content
          className={`fixed left-1/2 top-1/2 z-50 -translate-x-1/2 -translate-y-1/2 rounded-2xl border border-border bg-popover p-4 shadow-xl outline-none ${widthClassName}`}
        >
          {children}
        </DialogPrimitive.Content>
      </DialogPrimitive.Portal>
    </DialogPrimitive.Root>
  );
}

function formatSourceTime(value: string): string {
  const time = new Date(value);
  if (Number.isNaN(time.getTime())) return value;
  return time.toLocaleString();
}

/** Gear dialog: registered marketplaces with refresh + remove (official pinned). */
export function PluginStoreSourcesDialog({
  open,
  onOpenChange,
  marketplaces,
  onUpdateMarketplace,
  onRemoveMarketplace,
  operationId
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  marketplaces: PluginMarketplaceSummary[];
  onUpdateMarketplace: (marketplace: string) => void;
  onRemoveMarketplace: (marketplace: string) => void;
  operationId: string | null;
}) {
  const sortedMarketplaces = sortMarketplaceSources(marketplaces);
  return (
    <StoreDialog open={open} onOpenChange={onOpenChange} widthClassName="w-[min(480px,calc(100vw-2rem))]">
      <DialogPrimitive.Title className="text-ui-lg font-medium text-foreground">
        Marketplace sources
      </DialogPrimitive.Title>
      <div className="max-h-[min(420px,60vh)] space-y-1 overflow-y-auto pt-3">
        {sortedMarketplaces.length === 0 ? (
          <p className="px-1 py-2 text-ui-base text-foreground-subtle">
            No marketplace sources registered yet
          </p>
        ) : (
          sortedMarketplaces.map((marketplace) => {
            const updating = operationId === `marketplace:update:${marketplace.id}`;
            const removing = operationId === `marketplace:remove:${marketplace.id}`;
            const removable = marketplace.id !== 'zcode-plugins-official';
            return (
              <div
                key={marketplace.id}
                data-testid="plugin-store-source-row"
                data-marketplace-id={marketplace.id}
                className="flex items-center gap-3 rounded-lg px-2 py-2 transition-colors hover:bg-hover"
              >
                <div className="min-w-0 flex-1">
                  <div className="flex min-w-0 items-center gap-1.5">
                    <span className="truncate text-ui-base font-medium text-foreground">
                      {marketplace.name || marketplace.id}
                    </span>
                    {marketplace.isOfficial ? (
                      <span className="shrink-0 rounded-full border border-border px-1.5 py-0.5 text-ui-xs text-foreground-subtle">
                        Official
                      </span>
                    ) : null}
                  </div>
                  <div className="mt-0.5 truncate text-ui-base text-foreground-subtle">
                    {`${marketplace.pluginCount} plugins`}
                    {marketplace.lastUpdated
                      ? ` · Updated ${formatSourceTime(marketplace.lastUpdated)}`
                      : ''}
                  </div>
                  {marketplace.refreshFailure ? (
                    <div
                      className="mt-1 flex min-w-0 items-start gap-1 text-ui-base text-destructive"
                      data-testid="plugin-store-source-refresh-failure"
                    >
                      <AlertTriangle className="mt-0.5 size-3.5 shrink-0" aria-hidden="true" />
                      <span className="min-w-0 break-words">
                        {`Refresh failed at ${formatSourceTime(marketplace.refreshFailure.failedAt)}: ${marketplace.refreshFailure.message}`}
                      </span>
                    </div>
                  ) : null}
                </div>
                <button
                  type="button"
                  data-testid="plugin-store-source-update"
                  data-marketplace-id={marketplace.id}
                  aria-label="Refresh this marketplace"
                  disabled={updating}
                  className="flex size-8 items-center justify-center rounded-lg text-foreground-subtle transition-colors hover:bg-surface-hover hover:text-foreground disabled:opacity-60"
                  onClick={() => onUpdateMarketplace(marketplace.id)}
                >
                  <RefreshCw
                    className={updating ? 'size-3.5 animate-spin' : 'size-3.5'}
                    aria-hidden="true"
                  />
                </button>
                {removable ? (
                  <button
                    type="button"
                    data-testid="plugin-store-source-remove"
                    data-marketplace-id={marketplace.id}
                    aria-label="Remove this marketplace"
                    disabled={removing}
                    className="flex size-8 items-center justify-center rounded-lg text-destructive transition-colors hover:bg-destructive/10 hover:text-destructive disabled:opacity-60"
                    onClick={() => onRemoveMarketplace(marketplace.id)}
                  >
                    {removing ? (
                      <Loader2 className="size-3.5 animate-spin" aria-hidden="true" />
                    ) : (
                      <Trash2 className="size-3.5" aria-hidden="true" />
                    )}
                  </button>
                ) : null}
              </div>
            );
          })
        )}
      </div>
    </StoreDialog>
  );
}

/** New → Add marketplace source: URL / git / local path (+ directory picker). */
export function AddMarketplaceSourceDialog({
  open,
  onOpenChange,
  onAddMarketplace,
  operationId,
  error
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  onAddMarketplace: (source: string) => Promise<boolean>;
  operationId: string | null;
  error?: string | null;
}) {
  const [source, setSource] = useState('');
  const [dragActive, setDragActive] = useState(false);
  const trimmedSource = source.trim();
  const adding = operationId === `marketplace:add:${trimmedSource}`;
  const pendingRef = useRef(false);

  const handleAdd = async () => {
    if (trimmedSource.length === 0 || pendingRef.current) return;
    pendingRef.current = true;
    try {
      const added = await onAddMarketplace(trimmedSource);
      if (added) {
        setSource('');
        onOpenChange(false);
      }
    } finally {
      pendingRef.current = false;
    }
  };

  const handleChooseDirectory = async () => {
    // Wails dialog binding returns "" when cancelled; apiBridge may be mocked
    // in browser dev where OpenFolderDialog falls back to "".
    try {
      const mod = await import('../../../lib/wails');
      const dir = await mod.OpenFolderDialog();
      if (dir) setSource(dir);
    } catch {
      // cancelled or unavailable — keep current input
    }
  };

  const handleDrop = (event: React.DragEvent<HTMLDivElement>) => {
    event.preventDefault();
    setDragActive(false);
    const file = event.dataTransfer.files[0];
    // Electron-style path access doesn't exist in the webview; use the name
    // only as a hint when the runtime exposes a path.
    const anyFile = file as File & { path?: string };
    const path = anyFile?.path?.trim();
    if (path) setSource(path);
  };

  return (
    <StoreDialog
      open={open}
      onOpenChange={onOpenChange}
      widthClassName="w-[min(420px,calc(100vw-2rem))]"
    >
      <div
        data-testid="plugin-store-add-source-dialog"
        className={
          dragActive ? 'rounded-2xl border-border bg-surface-hover transition-colors' : ''
        }
        onDragOver={(event) => {
          event.preventDefault();
          setDragActive(true);
        }}
        onDragLeave={(event) => {
          if (!event.currentTarget.contains(event.relatedTarget as Node | null)) setDragActive(false);
        }}
        onDrop={handleDrop}
      >
        <DialogPrimitive.Title className="text-ui-lg font-medium text-foreground">
          Add marketplace
        </DialogPrimitive.Title>
        {error ? (
          <div
            className="mt-3 max-h-[min(240px,40vh)] min-w-0 overflow-y-auto whitespace-pre-wrap break-words rounded-lg border border-destructive/30 bg-destructive/10 px-3 py-2 text-ui-base text-destructive"
            data-testid="plugin-store-add-source-error"
            role="alert"
          >
            {error}
          </div>
        ) : null}
        <DialogPrimitive.Description className="sr-only">
          Register a plugin marketplace from a GitHub repo, git URL, or local directory.
        </DialogPrimitive.Description>
        <input
          type="text"
          data-testid="plugin-store-add-source-input"
          autoFocus
          aria-label="GitHub repo, git URL, file, or directory"
          value={source}
          onChange={(event) => setSource(event.target.value)}
          onKeyDown={(event) => {
            if (event.key === 'Enter') {
              event.preventDefault();
              void handleAdd();
            }
          }}
          placeholder="GitHub repo, git URL, file, or directory"
          className="mt-3 h-9 w-full rounded-xl border border-border bg-background px-3 text-ui-sm text-foreground outline-none transition-colors placeholder:text-foreground-subtlest focus:border-input-border-focused"
        />
        <p className="mt-2 text-ui-base text-foreground-subtle">
          Drag a file or folder here, or choose a directory.
        </p>
        <div className="mt-4 flex justify-end gap-2">
          <button
            type="button"
            className="inline-flex h-9 items-center justify-center gap-1.5 whitespace-nowrap rounded-lg border border-border px-4 text-ui-sm font-medium text-foreground transition-colors hover:bg-surface-hover"
            onClick={() => void handleChooseDirectory()}
          >
            Choose directory
          </button>
          <button
            type="button"
            data-testid="plugin-store-add-source-submit"
            disabled={trimmedSource.length === 0 || adding}
            className="inline-flex h-9 items-center justify-center gap-1.5 whitespace-nowrap rounded-lg bg-brand px-4 text-ui-sm font-medium text-foreground-inverse transition-colors hover:bg-brand/80 disabled:opacity-60"
            onClick={() => void handleAdd()}
          >
            {adding ? (
              <Loader2 className="size-3.5 animate-spin" aria-hidden="true" />
            ) : (
              <Plus className="size-3.5" aria-hidden="true" />
            )}
            Add marketplace
          </button>
        </div>
      </div>
    </StoreDialog>
  );
}

/** Uninstall confirmation dialog. */
export function PluginUninstallConfirmDialog({
  open,
  pluginName,
  pending,
  onCancel,
  onConfirm
}: {
  open: boolean;
  pluginName: string;
  pending: boolean;
  onCancel: () => void;
  onConfirm: () => void;
}) {
  return (
    <StoreDialog open={open} onOpenChange={(next) => (!next ? onCancel() : undefined)} widthClassName="w-[min(420px,calc(100vw-2rem))]">
      <DialogPrimitive.Title className="text-ui-lg font-medium text-foreground">
        Uninstall {pluginName}?
      </DialogPrimitive.Title>
      <DialogPrimitive.Description className="mt-2 text-ui-base text-foreground-subtle">
        This removes the plugin package from your user plugins directory. Skills, commands, and
        servers it provides stop being available in new sessions.
      </DialogPrimitive.Description>
      <div className="mt-4 flex justify-end gap-2">
        <button
          type="button"
          className="inline-flex h-9 items-center justify-center rounded-lg border border-border px-4 text-ui-sm font-medium text-foreground transition-colors hover:bg-surface-hover"
          onClick={onCancel}
        >
          Cancel
        </button>
        <button
          type="button"
          data-testid="plugin-store-uninstall-confirm"
          disabled={pending}
          className="inline-flex h-9 items-center justify-center gap-1.5 rounded-lg bg-destructive px-4 text-ui-sm font-medium text-white transition-colors hover:bg-destructive/90 disabled:opacity-60"
          onClick={onConfirm}
        >
          {pending ? <Loader2 className="size-3.5 animate-spin" aria-hidden="true" /> : null}
          Uninstall
        </button>
      </div>
    </StoreDialog>
  );
}
