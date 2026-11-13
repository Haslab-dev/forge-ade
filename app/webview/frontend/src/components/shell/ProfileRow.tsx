import React from 'react';
import { Laptop, Moon, Sun, Settings } from 'lucide-react';
import { cn, APP_VERSION, APP_BUILD } from '../../lib/utils';
import type { ThemeMode } from '../../types';

const NEXT_MODE: Record<ThemeMode, ThemeMode> = { light: 'dark', dark: 'system', system: 'light' };

/**
 * Sidebar footer controls: appearance toggle (light → dark → system), version
 * badge, and settings shortcut.
 */
export const ProfileRow: React.FC<{
  theme: ThemeMode;
  onToggleTheme: (next: ThemeMode) => void;
  onOpenSettings: () => void;
  className?: string;
}> = ({ theme, onToggleTheme, onOpenSettings, className }) => {
  const label = theme === 'dark' ? 'Dark theme' : theme === 'light' ? 'Light theme' : 'Follow system';
  return (
    <div className={cn('flex shrink-0 items-center justify-between gap-2 border-t border-border/40 px-3 py-2', className)}>
      <button
        type="button"
        aria-label={`Appearance: ${label}`}
        title={`${label} — click for ${NEXT_MODE[theme]}`}
        onClick={() => onToggleTheme(NEXT_MODE[theme])}
        className="flex size-7 shrink-0 items-center justify-center rounded-md text-foreground-subtle hover:bg-surface-hover hover:text-foreground transition-colors cursor-pointer"
      >
        {theme === 'dark' ? <Moon className="size-4" /> : theme === 'light' ? <Sun className="size-4" /> : <Laptop className="size-4" />}
      </button>

      <div className="flex items-center gap-1.5">
        <span
          title={`ForgeADE v${APP_VERSION} (Build ${APP_BUILD})`}
          className="font-mono text-ui-xs text-foreground-subtle hover:text-foreground transition-colors select-none cursor-default font-medium px-1.5 py-0.5 rounded bg-surface border border-border/50"
        >
          v{APP_VERSION}
          <span className="text-[10px] text-foreground-subtle/70 font-normal ml-1">#{APP_BUILD}</span>
        </span>
        <button
          type="button"
          aria-label="Settings"
          title="Settings (⌘,)"
          onClick={onOpenSettings}
          className="flex size-7 shrink-0 items-center justify-center rounded-md text-foreground-subtle hover:bg-surface-hover hover:text-foreground transition-colors cursor-pointer"
        >
          <Settings className="size-4" />
        </button>
      </div>
    </div>
  );
};
