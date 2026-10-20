import React, { useState, useRef, useEffect } from 'react';
import { Palette, Sun, Moon, Laptop, Check, ChevronDown } from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';
import { BuiltInThemeId, BUILT_IN_THEMES } from '../../lib/themePalettes';
import { cn } from '../../lib/utils';

export const ThemePicker: React.FC<{ compact?: boolean }> = ({ compact = false }) => {
  const { theme, setTheme, themePalette, setThemePalette } = useWorkspace();
  const [open, setOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const handleOutside = (e: MouseEvent) => {
      if (menuRef.current && !menuRef.current.contains(e.target as Node)) {
        setOpen(false);
      }
    };
    if (open) {
      document.addEventListener('mousedown', handleOutside);
    }
    return () => document.removeEventListener('mousedown', handleOutside);
  }, [open]);

  const themesList: { id: BuiltInThemeId; label: string; accentColor: string; surfaceColor: string }[] = [
    { id: 't3-chat', label: 'T3 Chat', accentColor: '#346bf1', surfaceColor: '#171717' },
    { id: 'grove', label: 'Grove', accentColor: '#10b981', surfaceColor: '#16221c' },
    { id: 'ocean', label: 'Ocean', accentColor: '#0ea5e9', surfaceColor: '#131e29' },
    { id: 'ember', label: 'Ember', accentColor: '#f97316', surfaceColor: '#221915' },
    { id: 'iris', label: 'Iris', accentColor: '#8b5cf6', surfaceColor: '#1d1729' }
  ];

  const currentTheme = BUILT_IN_THEMES[themePalette] || BUILT_IN_THEMES['t3-chat'];

  return (
    <div className="relative inline-block text-left" ref={menuRef}>
      <button
        type="button"
        onClick={() => setOpen(prev => !prev)}
        className={cn(
          "inline-flex items-center gap-1.5 px-2.5 py-1 text-ui-xs font-medium rounded-md border border-border/60 hover:bg-surface-hover transition-colors text-fg-secondary hover:text-fg-primary",
          open && "bg-surface-active text-fg-primary border-border"
        )}
        title="Theme and Palettes"
      >
        <Palette className="size-3.5 shrink-0 text-accent-primary" />
        {!compact && <span>{currentTheme.label}</span>}
        <span className="size-2 rounded-full border border-border/40" style={{ backgroundColor: themesList.find(t => t.id === themePalette)?.accentColor }} />
        <ChevronDown className={cn("size-3 transition-transform text-fg-muted", open && "rotate-180")} />
      </button>

      {open && (
        <div className="absolute right-0 mt-1.5 w-56 rounded-xl border border-border bg-panel p-2 shadow-xl z-50 animate-in fade-in zoom-in-95 duration-100">
          <div className="px-2 py-1 text-2xs font-semibold text-fg-muted uppercase tracking-wider">
            Palette
          </div>
          <div className="space-y-0.5 mb-2">
            {themesList.map(t => {
              const isSelected = themePalette === t.id;
              return (
                <button
                  key={t.id}
                  type="button"
                  onClick={() => {
                    setThemePalette(t.id);
                  }}
                  className={cn(
                    "w-full flex items-center justify-between px-2.5 py-1.5 rounded-lg text-ui-xs text-left transition-colors",
                    isSelected ? "bg-accent-primary/15 text-fg-primary font-medium" : "text-fg-secondary hover:bg-surface-hover hover:text-fg-primary"
                  )}
                >
                  <div className="flex items-center gap-2">
                    <span
                      className="size-3.5 rounded-full border border-border/40 shrink-0"
                      style={{ backgroundColor: t.accentColor }}
                    />
                    <span>{t.label}</span>
                  </div>
                  {isSelected && <Check className="size-3.5 text-accent-primary" />}
                </button>
              );
            })}
          </div>

          <div className="h-px bg-border/60 my-1.5" />

          <div className="px-2 py-1 text-2xs font-semibold text-fg-muted uppercase tracking-wider">
            Appearance
          </div>
          <div className="grid grid-cols-2 gap-1 mt-1">
            <button
              type="button"
              onClick={() => setTheme('dark')}
              className={cn(
                "flex items-center justify-center gap-1.5 px-2 py-1.5 rounded-md text-ui-xs font-medium border transition-colors",
                theme === 'dark' ? "border-accent-primary bg-accent-primary/10 text-fg-primary" : "border-border/60 text-fg-secondary hover:bg-surface-hover"
              )}
            >
              <Moon className="size-3.5" />
              <span>Dark</span>
            </button>
            <button
              type="button"
              onClick={() => setTheme('light')}
              className={cn(
                "flex items-center justify-center gap-1.5 px-2 py-1.5 rounded-md text-ui-xs font-medium border transition-colors",
                theme === 'light' ? "border-accent-primary bg-accent-primary/10 text-fg-primary" : "border-border/60 text-fg-secondary hover:bg-surface-hover"
              )}
            >
              <Sun className="size-3.5" />
              <span>Light</span>
            </button>
          </div>
        </div>
      )}
    </div>
  );
};
