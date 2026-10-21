import React, { useState, useRef, useEffect } from 'react';
import { Palette, Sun, Moon, Laptop, Check, ChevronDown, Type, Minus, Plus } from 'lucide-react';
import { useWorkspace } from '../../stores/workspaceStore';
import { BuiltInThemeId, BUILT_IN_THEMES, ThemeColors, getThemeColorsForAppearance } from '../../lib/themePalettes';
import {
  AppearanceFontPreferences,
  DIFF_COLOR_SCHEMES,
  DiffColorScheme,
  ChatWidth,
  CHAT_WIDTHS,
  DEFAULT_CHAT_WIDTH,
  clampCodeFontSize,
  clampInterfaceFontSize,
  getStoredAppearanceContrast,
  getStoredChatWidth,
  getStoredDiffColorScheme,
  getStoredFontPreferences,
  saveAppearanceContrast,
  saveChatWidth,
  saveDiffColorScheme,
  saveFontPreferences,
} from '../../lib/themeEngine';
import {
  isNotificationSoundEnabled,
  setNotificationSoundEnabled,
} from '../../lib/notificationSounds';
import { cn } from '../../lib/utils';

// Swatch previews per theme use the dark variant colors so every option is
// recognizable at a glance regardless of the current appearance.
const PREVIEW_APPEARANCE: 'light' | 'dark' = 'dark';

function themeSwatches(colors: ThemeColors): string[] {
  return [colors.canvas, colors.sidebar, colors.messageSurface, colors.messageAction, colors.accent];
}

function Stepper({
  value,
  min,
  max,
  onChange,
  label,
}: {
  value: number;
  min: number;
  max: number;
  onChange: (next: number) => void;
  label: string;
}) {
  return (
    <div className="flex items-center justify-between gap-2">
      <span className="text-ui-xs text-fg-secondary">{label}</span>
      <div className="flex items-center gap-1 rounded-md border border-border-default bg-surface">
        <button
          type="button"
          aria-label={`Decrease ${label}`}
          disabled={value <= min}
          onClick={() => onChange(Math.max(min, value - 1))}
          className="rounded p-1 text-fg-tertiary hover:bg-surface-hover hover:text-fg-primary disabled:opacity-40"
        >
          <Minus className="size-3" />
        </button>
        <span className="min-w-7 text-center font-mono text-ui-xs text-fg-primary tabular-nums">{value}</span>
        <button
          type="button"
          aria-label={`Increase ${label}`}
          disabled={value >= max}
          onClick={() => onChange(Math.min(max, value + 1))}
          className="rounded p-1 text-fg-tertiary hover:bg-surface-hover hover:text-fg-primary disabled:opacity-40"
        >
          <Plus className="size-3" />
        </button>
      </div>
    </div>
  );
}

const COMMON_SANS_FONTS = [
  'System',
  'SF Pro',
  'Helvetica Neue',
  'Arial',
  'Segoe UI',
  'Roboto',
  'Inter',
  'Verdana',
];
const COMMON_CODE_FONTS = [
  'SF Mono',
  'Menlo',
  'Monaco',
  'Consolas',
  'Cascadia Code',
  'JetBrains Mono',
  'Fira Code',
  'IBM Plex Mono',
];

function FontFamilyField({
  label,
  value,
  options,
  onChange,
}: {
  label: string;
  value: string;
  options: string[];
  onChange: (next: string) => void;
}) {
  return (
    <label className="block space-y-1">
      <span className="text-ui-xs text-fg-secondary">{label}</span>
      <input
        type="text"
        list={`forge-font-options-${label.replace(/\s+/g, '-')}`}
        value={value}
        placeholder="System default"
        onChange={(e) => onChange(e.target.value)}
        className="w-full rounded-md border border-border-default bg-surface px-2 py-1 text-ui-xs text-fg-primary outline-none placeholder:text-fg-tertiary"
      />
      <datalist id={`forge-font-options-${label.replace(/\s+/g, '-')}`}>
        {options.map((f) => (
          <option key={f} value={f === 'System' ? '' : f}>
            {f}
          </option>
        ))}
      </datalist>
    </label>
  );
}

/**
 * Appearance panel (ported from t3code Settings → Appearance): theme gallery
 * with light/dark/system control, contrast slider, interface & code font
 * families and sizes, and the chat lane width. Lives in the workspace header
 * popover; state persists through the theme engine (localStorage).
 */
export const ThemePicker: React.FC<{ compact?: boolean }> = ({ compact = false }) => {
  const { theme, setTheme, themePalette, setThemePalette } = useWorkspace();
  const [open, setOpen] = useState(false);
  const menuRef = useRef<HTMLDivElement>(null);
  const [fonts, setFonts] = useState<AppearanceFontPreferences>(() => getStoredFontPreferences());
  const [contrast, setContrast] = useState<number>(() => getStoredAppearanceContrast());
  const [chatWidth, setChatWidth] = useState<ChatWidth>(() => getStoredChatWidth());
  const [soundEnabled, setSoundEnabled] = useState<boolean>(() => isNotificationSoundEnabled());
  const [diffScheme, setDiffScheme] = useState<DiffColorScheme>(() => getStoredDiffColorScheme());

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

  const updateFonts = (patch: Partial<AppearanceFontPreferences>) => {
    const next = { ...fonts, ...patch };
    setFonts(next);
    saveFontPreferences(next);
  };

  const currentTheme = BUILT_IN_THEMES[themePalette] ?? BUILT_IN_THEMES['t3-chat'];
  const currentColors = getThemeColorsForAppearance(currentTheme, PREVIEW_APPEARANCE);

  const modes: { id: 'light' | 'dark' | 'system'; label: string; icon: React.ReactNode }[] = [
    { id: 'light', label: 'Light', icon: <Sun className="size-3.5" /> },
    { id: 'dark', label: 'Dark', icon: <Moon className="size-3.5" /> },
    { id: 'system', label: 'Auto', icon: <Laptop className="size-3.5" /> },
  ];

  return (
    <div className="relative inline-block text-left" ref={menuRef}>
      <button
        type="button"
        onClick={() => setOpen((prev) => !prev)}
        className={cn(
          'inline-flex items-center gap-1.5 rounded-md border border-border-default px-2.5 py-1 text-ui-xs font-medium text-fg-secondary transition-colors hover:bg-surface-hover hover:text-fg-primary',
          open && 'border-border bg-surface-active text-fg-primary',
        )}
        title="Appearance"
      >
        <Palette className="size-3.5 shrink-0 text-accent-primary" />
        {!compact && <span>{currentTheme.label}</span>}
        <span
          className="size-2 rounded-full border border-border/40"
          style={{ backgroundColor: currentColors.messageAction }}
        />
        <ChevronDown className={cn('size-3 text-fg-muted transition-transform', open && 'rotate-180')} />
      </button>

      {open && (
        <div className="dropdown-glass absolute right-0 z-50 mt-1.5 w-72 animate-in rounded-xl p-2 shadow-xl fade-in zoom-in-95 duration-100">
          {/* Appearance mode */}
          <div className="px-2 pb-1 pt-0.5 text-2xs font-semibold uppercase tracking-wider text-fg-muted">
            Appearance
          </div>
          <div className="grid grid-cols-3 gap-1">
            {modes.map((m) => (
              <button
                key={m.id}
                type="button"
                onClick={() => setTheme(m.id)}
                className={cn(
                  'flex items-center justify-center gap-1.5 rounded-md border px-2 py-1.5 text-ui-xs font-medium transition-colors',
                  theme === m.id
                    ? 'border-accent-primary bg-accent-primary/10 text-fg-primary'
                    : 'border-border-default text-fg-secondary hover:bg-surface-hover',
                )}
              >
                {m.icon}
                <span>{m.label}</span>
              </button>
            ))}
          </div>

          <div className="my-2 h-px bg-border-subtle" />

          {/* Theme gallery */}
          <div className="px-2 pb-1 text-2xs font-semibold uppercase tracking-wider text-fg-muted">
            Theme
          </div>
          <div className="space-y-0.5">
            {(Object.keys(BUILT_IN_THEMES) as BuiltInThemeId[]).map((id) => {
              const t = BUILT_IN_THEMES[id];
              const swatches = themeSwatches(getThemeColorsForAppearance(t, PREVIEW_APPEARANCE));
              const isSelected = themePalette === id;
              return (
                <button
                  key={id}
                  type="button"
                  onClick={() => setThemePalette(id)}
                  className={cn(
                    'flex w-full items-center justify-between rounded-lg px-2.5 py-1.5 text-left text-ui-xs transition-colors',
                    isSelected
                      ? 'bg-accent-primary/15 font-medium text-fg-primary'
                      : 'text-fg-secondary hover:bg-surface-hover hover:text-fg-primary',
                  )}
                >
                  <span className="flex items-center gap-2">
                    <span className="flex shrink-0 overflow-hidden rounded-full border border-border/40">
                      {swatches.map((c, i) => (
                        <span key={i} className="block size-3" style={{ backgroundColor: c }} />
                      ))}
                    </span>
                    <span>{t.label}</span>
                  </span>
                  {isSelected && <Check className="size-3.5 text-accent-primary" />}
                </button>
              );
            })}
          </div>

          <div className="my-2 h-px bg-border-subtle" />

          {/* Contrast */}
          <div className="space-y-1 px-1">
            <div className="flex items-center justify-between">
              <span className="text-ui-xs text-fg-secondary">Contrast</span>
              <span className="font-mono text-2xs text-fg-tertiary tabular-nums">{contrast}%</span>
            </div>
            <input
              type="range"
              min={70}
              max={130}
              step={5}
              value={contrast}
              onChange={(e) => {
                const next = Number(e.target.value);
                setContrast(next);
                saveAppearanceContrast(next);
              }}
              className="settings-slider w-full"
              style={{ ['--settings-slider-progress' as string]: `${((contrast - 70) / 60) * 100}%` }}
            />
          </div>

          <div className="my-2 h-px bg-border-subtle" />

          {/* Typography */}
          <div className="flex items-center gap-1.5 px-2 pb-1 text-2xs font-semibold uppercase tracking-wider text-fg-muted">
            <Type className="size-3" />
            Typography
          </div>
          <div className="space-y-2 px-1">
            <Stepper
              label="Interface size"
              value={clampInterfaceFontSize(fonts.sizeInterface)}
              min={12}
              max={20}
              onChange={(next) => updateFonts({ sizeInterface: next })}
            />
            <Stepper
              label="Code size"
              value={clampCodeFontSize(fonts.sizeCode)}
              min={10}
              max={18}
              onChange={(next) => updateFonts({ sizeCode: next })}
            />
            <FontFamilyField
              label="Interface font"
              value={fonts.sans}
              options={COMMON_SANS_FONTS}
              onChange={(next) => updateFonts({ sans: next })}
            />
            <FontFamilyField
              label="Code font"
              value={fonts.code}
              options={COMMON_CODE_FONTS}
              onChange={(next) => updateFonts({ code: next })}
            />
            <label className="flex items-center justify-between">
              <span className="text-ui-xs text-fg-secondary">Font smoothing</span>
              <input
                type="checkbox"
                checked={fonts.smoothing}
                onChange={(e) => updateFonts({ smoothing: e.target.checked })}
                className="size-3.5 accent-[var(--focus)]"
              />
            </label>
          </div>

          <div className="my-2 h-px bg-border-subtle" />

          {/* Chat width */}
          <div className="px-2 pb-1 text-2xs font-semibold uppercase tracking-wider text-fg-muted">
            Chat width
          </div>
          <div className="grid grid-cols-3 gap-1">
            {CHAT_WIDTHS.map((w) => (
              <button
                key={w}
                type="button"
                onClick={() => {
                  setChatWidth(w);
                  saveChatWidth(w);
                }}
                className={cn(
                  'rounded-md border px-2 py-1.5 text-ui-xs font-medium capitalize transition-colors',
                  chatWidth === w
                    ? 'border-accent-primary bg-accent-primary/10 text-fg-primary'
                    : 'border-border-default text-fg-secondary hover:bg-surface-hover',
                )}
              >
                {w}
              </button>
            ))}
          </div>
          {/* Width comparator indicator */}
          <div className="px-2 pt-2 pb-0.5 space-y-1">
            <div className="h-1.5 w-full bg-surface-hover rounded-full overflow-hidden flex items-center">
              <div
                className="h-full bg-accent-primary rounded-full transition-all duration-200"
                style={{
                  width: chatWidth === 'comfortable' ? '50%' : chatWidth === 'wide' ? '75%' : '100%',
                }}
              />
            </div>
            <div className="flex items-center justify-between text-3xs text-fg-tertiary">
              <span>
                {chatWidth === 'comfortable' && 'Comfortable (736px)'}
                {chatWidth === 'wide' && 'Wide layout (928px)'}
                {chatWidth === 'full' && 'Full width (100%)'}
              </span>
              <span className="font-mono text-fg-muted">
                {chatWidth === 'comfortable' ? '46rem' : chatWidth === 'wide' ? '58rem' : '100%'}
              </span>
            </div>
          </div>

          <div className="my-2 h-px bg-border-subtle" />

          {/* Diff colors */}
          <div className="px-2 pb-1 text-2xs font-semibold uppercase tracking-wider text-fg-muted">
            Diff colors
          </div>
          <div className="grid grid-cols-2 gap-1">
            {DIFF_COLOR_SCHEMES.map((s) => (
              <button
                key={s}
                type="button"
                onClick={() => {
                  setDiffScheme(s);
                  saveDiffColorScheme(s);
                }}
                className={cn(
                  'rounded-md border px-2 py-1.5 text-ui-xs font-medium capitalize transition-colors',
                  diffScheme === s
                    ? 'border-accent-primary bg-accent-primary/10 text-fg-primary'
                    : 'border-border-default text-fg-secondary hover:bg-surface-hover',
                )}
              >
                {s === 'classic' ? 'Green / Red' : 'Blue / Orange'}
              </button>
            ))}
          </div>

          <div className="my-2 h-px bg-border-subtle" />

          {/* Notifications */}
          <label className="flex items-center justify-between px-1">
            <span className="text-ui-xs text-fg-secondary">Turn completion sound</span>
            <input
              type="checkbox"
              checked={soundEnabled}
              onChange={(e) => {
                setSoundEnabled(e.target.checked);
                setNotificationSoundEnabled(e.target.checked);
              }}
              className="size-3.5 accent-[var(--focus)]"
            />
          </label>
        </div>
      )}
    </div>
  );
};
