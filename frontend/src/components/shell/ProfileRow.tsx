import React from 'react';
import { Moon, Sun, Settings } from 'lucide-react';
import { cn } from '../../lib/utils';

/**
 * Sidebar footer controls (ZCode parity): theme toggle and settings shortcut.
 */
export const ProfileRow: React.FC<{
  theme: 'dark' | 'light';
  onToggleTheme: () => void;
  onOpenSettings: () => void;
  className?: string;
}> = ({ theme, onToggleTheme, onOpenSettings, className }) => {
  return (
    <div className={cn('flex shrink-0 items-center justify-end gap-2 border-t border-border/40 px-3 py-2.5', className)}>
      <button
        type="button"
        aria-label={theme === 'dark' ? 'Switch to light theme' : 'Switch to dark theme'}
        title={theme === 'dark' ? 'Light theme (⌘/)' : 'Dark theme (⌘/)'}
        onClick={onToggleTheme}
        className="flex size-7 shrink-0 items-center justify-center rounded-md text-foreground-subtle hover:bg-surface-hover hover:text-foreground transition-colors cursor-pointer"
      >
        {theme === 'dark' ? <Sun className="size-4" /> : <Moon className="size-4" />}
      </button>
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
  );
};
