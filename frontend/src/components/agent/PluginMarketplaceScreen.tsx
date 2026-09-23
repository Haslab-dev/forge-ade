import React, { useState } from 'react';
import { ChevronRight } from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';
import { PluginStorePage } from './plugin-store/PluginStorePage';

interface BreadcrumbState {
  items: Array<{ label: string }>;
}

/**
 * Plugin Marketplace main view. Mirrors the ZCode workspace main-view frame:
 * breadcrumb drag row + scrollable `max-w-4xl` content column hosting
 * PluginStorePage (list ↔ detail, sources dialogs, install flows).
 */
export const PluginMarketplaceScreen: React.FC = () => {
  const { setMode, setSettingsActiveSection } = useWorkspace();
  const [breadcrumb, setBreadcrumb] = useState<BreadcrumbState>({ items: [] });
  const visible = breadcrumb.items.length > 0;

  return (
    <main className="flex h-full min-h-0 flex-1 flex-col bg-background">
      <div className="h-12 shrink-0" data-testid="plugin-store-main-drag-region">
        {visible ? (
          <nav aria-label="Settings" className="flex h-full min-w-0 items-center overflow-hidden px-2.5">
            <ol className="flex min-w-0 items-center gap-0 overflow-hidden">
              {breadcrumb.items.map((item, index) => {
                const current = index === breadcrumb.items.length - 1;
                return (
                  <li key={`${index}:${item.label}`} className="flex min-w-0 items-center gap-0">
                    {index > 0 ? (
                      <ChevronRight
                        aria-hidden="true"
                        className="size-3.5 shrink-0 text-foreground-subtlest"
                      />
                    ) : null}
                    <span
                      aria-current={current ? 'page' : undefined}
                      className={`truncate ${current ? 'px-2 text-foreground' : 'text-foreground-subtle'}`}
                    >
                      {item.label}
                    </span>
                  </li>
                );
              })}
            </ol>
          </nav>
        ) : null}
      </div>
      <div className="min-h-0 flex-1 overflow-y-auto [scrollbar-gutter:stable]">
        <div className="mx-auto flex w-full max-w-4xl flex-col px-4 py-4 md:px-6 md:py-6">
          <PluginStorePage
            onBreadcrumbChange={(items) => setBreadcrumb({ items })}
            onManageInstalled={() => {
              setSettingsActiveSection('plugins');
              setMode('settings');
            }}
          />
        </div>
      </div>
    </main>
  );
};
