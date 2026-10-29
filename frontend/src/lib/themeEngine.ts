import {
  BUILT_IN_THEME_IDS,
  BuiltInThemeId,
  ThemeAppearance,
  THEME_COLOR_ROLES,
  ThemeColors,
  ThemeColorRole,
  getThemeColorsForAppearance,
  getThemeDefinition,
} from "./themePalettes";

export type ThemePreferenceMode = ThemeAppearance | "system";

const THEME_PALETTE_KEY = "forge:theme-palette";
const THEME_MODE_KEY = "forge:theme-mode";
const APPEARANCE_FONTS_KEY = "forge:appearance-fonts";
const APPEARANCE_CONTRAST_KEY = "forge:appearance-contrast";
const CHAT_WIDTH_KEY = "forge:chat-width";

// ForgeADE runtime typography controls: clamp ranges for the runtime
// typography controls.
export const MIN_INTERFACE_FONT_SIZE = 12;
export const MAX_INTERFACE_FONT_SIZE = 20;
export const DEFAULT_INTERFACE_FONT_SIZE = 16;
export const MIN_PROMPT_FONT_SIZE = 12;
export const MAX_PROMPT_FONT_SIZE = 20;
export const DEFAULT_PROMPT_FONT_SIZE = 14;
export const MIN_CODE_FONT_SIZE = 10;
export const MAX_CODE_FONT_SIZE = 18;
export const DEFAULT_CODE_FONT_SIZE = 13;

export const DEFAULT_SANS_FONT_STACK =
  '"Geist", -apple-system, BlinkMacSystemFont, "Segoe UI", Roboto, system-ui, sans-serif';
// Concrete names first: some engines alias `ui-monospace` to the proportional
// system UI font, which would break every code surface.
export const DEFAULT_CODE_FONT_STACK =
  '"Geist Mono", "JetBrains Mono", "SF Mono", "SFMono-Regular", Menlo, Consolas, "Liberation Mono", monospace';

export const CHAT_WIDTHS = ["comfortable", "wide", "full"] as const;
export type ChatWidth = (typeof CHAT_WIDTHS)[number];
export const DEFAULT_CHAT_WIDTH: ChatWidth = "comfortable";

export interface AppearanceFontPreferences {
  /** Custom interface font family ("" = system default). */
  sans: string;
  /** Custom code/terminal font family ("" = system default). */
  code: string;
  sizeInterface: number;
  sizePrompt: number;
  sizeCode: number;
  smoothing: boolean;
}

export const DEFAULT_FONT_PREFERENCES: AppearanceFontPreferences = {
  sans: "",
  code: "",
  sizeInterface: DEFAULT_INTERFACE_FONT_SIZE,
  sizePrompt: DEFAULT_PROMPT_FONT_SIZE,
  sizeCode: DEFAULT_CODE_FONT_SIZE,
  smoothing: true,
};

export function getStoredThemePalette(): BuiltInThemeId {
  try {
    const val = localStorage.getItem(THEME_PALETTE_KEY);
    if (val === 't3-chat') return 'forge-default';
    if (val && (BUILT_IN_THEME_IDS as readonly string[]).includes(val)) {
      return val as BuiltInThemeId;
    }
  } catch {
    // ignore
  }
  return "forge-default";
}

export function getStoredAppearanceMode(): ThemePreferenceMode {
  try {
    const val = localStorage.getItem(THEME_MODE_KEY);
    if (val === "light" || val === "dark" || val === "system") {
      return val;
    }
  } catch {
    // ignore
  }
  return "system";
}

function resolveEffectiveAppearance(mode: ThemePreferenceMode): ThemeAppearance {
  if (mode === "system") {
    if (typeof window !== "undefined" && window.matchMedia) {
      return window.matchMedia("(prefers-color-scheme: dark)").matches ? "dark" : "light";
    }
    return "dark";
  }
  return mode;
}

function clampFontSize(value: number, minimum: number, maximum: number, fallback: number): number {
  if (!Number.isFinite(value)) return fallback;
  return Math.min(maximum, Math.max(minimum, Math.round(value)));
}

export function clampInterfaceFontSize(value: number): number {
  return clampFontSize(value, MIN_INTERFACE_FONT_SIZE, MAX_INTERFACE_FONT_SIZE, DEFAULT_INTERFACE_FONT_SIZE);
}

export function clampPromptFontSize(value: number): number {
  return clampFontSize(value, MIN_PROMPT_FONT_SIZE, MAX_PROMPT_FONT_SIZE, DEFAULT_PROMPT_FONT_SIZE);
}

export function clampCodeFontSize(value: number): number {
  return clampFontSize(value, MIN_CODE_FONT_SIZE, MAX_CODE_FONT_SIZE, DEFAULT_CODE_FONT_SIZE);
}

function quoteFontFamilyName(name: string): string {
  const bare = name.trim();
  if (bare.length === 0) return "";
  if (/^(['"]).*\1$/.test(bare)) return bare;
  if (/^[a-zA-Z][a-zA-Z0-9-]*$/.test(bare)) return bare;
  return `"${bare.replaceAll('"', "")}"`;
}

/** Normalize a user-entered font list; null when nothing usable was entered. */
export function cssFontFamilies(input: string): string | null {
  const families = input
    .split(",")
    .map(quoteFontFamilyName)
    .filter((name) => name.length > 0);
  return families.length > 0 ? families.join(", ") : null;
}

export function appearanceFontStack(custom: string, defaultStack: string): string {
  const families = cssFontFamilies(custom);
  return families === null ? defaultStack : `${families}, ${defaultStack}`;
}

/**
 * Sizes are always written: the interface size drives the root font size (and
 * with it every rem-based dimension), while the prompt and code sizes stay in
 * absolute pixels so they do not scale twice.
 */
export function applyAppearanceFontVariables(
  root: HTMLElement,
  preferences: AppearanceFontPreferences,
): void {
  const families: ReadonlyArray<readonly [variable: string, custom: string, fallback: string]> = [
    ["--font-sans", preferences.sans, DEFAULT_SANS_FONT_STACK],
    ["--font-mono", preferences.code, DEFAULT_CODE_FONT_STACK],
  ];
  for (const [variable, custom, fallback] of families) {
    const list = cssFontFamilies(custom);
    if (list === null) {
      root.style.removeProperty(variable);
    } else {
      root.style.setProperty(variable, `${list}, ${fallback}`);
    }
  }

  root.style.fontSize = `${clampInterfaceFontSize(preferences.sizeInterface)}px`;
  root.style.setProperty("--font-size-prompt", `${clampPromptFontSize(preferences.sizePrompt)}px`);
  const code = clampCodeFontSize(preferences.sizeCode);
  root.style.setProperty("--font-size-code", `${code}px`);

  if (preferences.smoothing) {
    root.style.setProperty("-webkit-font-smoothing", "antialiased");
  } else {
    root.style.removeProperty("-webkit-font-smoothing");
  }
}

/** Appearance contrast calculations. */
export function applyAppearanceContrast(root: HTMLElement, contrast: number): void {
  root.style.setProperty("--appearance-contrast-base", `${Math.min(contrast, 100)}%`);
  root.style.setProperty("--appearance-contrast-boost", `${Math.max(contrast - 100, 0)}%`);
  root.style.setProperty(
    "--appearance-contrast-border-boost",
    `${Math.max(contrast - 100, 0) / 4}%`,
  );
}

export function getStoredFontPreferences(): AppearanceFontPreferences {
  try {
    const raw = localStorage.getItem(APPEARANCE_FONTS_KEY);
    if (raw) {
      const parsed = JSON.parse(raw) as Partial<AppearanceFontPreferences>;
      return {
        sans: typeof parsed.sans === "string" ? parsed.sans : "",
        code: typeof parsed.code === "string" ? parsed.code : "",
        sizeInterface: clampInterfaceFontSize(Number(parsed.sizeInterface) || DEFAULT_INTERFACE_FONT_SIZE),
        sizePrompt: clampPromptFontSize(Number(parsed.sizePrompt) || DEFAULT_PROMPT_FONT_SIZE),
        sizeCode: clampCodeFontSize(Number(parsed.sizeCode) || DEFAULT_CODE_FONT_SIZE),
        smoothing: parsed.smoothing !== false,
      };
    }
  } catch {
    // ignore
  }
  return { ...DEFAULT_FONT_PREFERENCES };
}

export function saveFontPreferences(preferences: AppearanceFontPreferences) {
  try {
    localStorage.setItem(APPEARANCE_FONTS_KEY, JSON.stringify(preferences));
  } catch {
    // ignore
  }
  if (typeof document !== "undefined") {
    applyAppearanceFontVariables(document.documentElement, preferences);
  }
}

export function getStoredAppearanceContrast(): number {
  try {
    const val = Number(localStorage.getItem(APPEARANCE_CONTRAST_KEY));
    if (Number.isFinite(val) && val >= 70 && val <= 130) return val;
  } catch {
    // ignore
  }
  return 100;
}

export function saveAppearanceContrast(contrast: number) {
  try {
    localStorage.setItem(APPEARANCE_CONTRAST_KEY, String(contrast));
  } catch {
    // ignore
  }
  if (typeof document !== "undefined") {
    applyAppearanceContrast(document.documentElement, contrast);
  }
}

export const DIFF_COLOR_SCHEMES = ["classic", "blue-orange"] as const;
export type DiffColorScheme = (typeof DIFF_COLOR_SCHEMES)[number];
const DIFF_SCHEME_KEY = "forge:diff-color-scheme";

export function getStoredDiffColorScheme(): DiffColorScheme {
  try {
    const val = localStorage.getItem(DIFF_SCHEME_KEY);
    if (val && (DIFF_COLOR_SCHEMES as readonly string[]).includes(val)) return val as DiffColorScheme;
  } catch {
    // ignore
  }
  return "classic";
}

export function saveDiffColorScheme(scheme: DiffColorScheme) {
  try {
    localStorage.setItem(DIFF_SCHEME_KEY, scheme);
  } catch {
    // ignore
  }
  if (typeof document !== "undefined") {
    if (scheme === "classic") {
      document.documentElement.removeAttribute("data-diff-color-scheme");
    } else {
      document.documentElement.setAttribute("data-diff-color-scheme", scheme);
    }
  }
}

export function getStoredChatWidth(): ChatWidth {
  try {
    const val = localStorage.getItem(CHAT_WIDTH_KEY);
    if (val && (CHAT_WIDTHS as readonly string[]).includes(val)) return val as ChatWidth;
  } catch {
    // ignore
  }
  return DEFAULT_CHAT_WIDTH;
}

export function saveChatWidth(width: ChatWidth) {
  try {
    localStorage.setItem(CHAT_WIDTH_KEY, width);
  } catch {
    // ignore
  }
  if (typeof document !== "undefined") {
    if (width === DEFAULT_CHAT_WIDTH) {
      document.documentElement.removeAttribute("data-chat-width");
    } else {
      document.documentElement.setAttribute("data-chat-width", width);
    }
  }
}

export function applyTheme(paletteId: BuiltInThemeId, mode: ThemePreferenceMode): ThemeAppearance {
  const effective = resolveEffectiveAppearance(mode);
  const themeDef = getThemeDefinition(paletteId);
  const colors: ThemeColors = getThemeColorsForAppearance(themeDef, effective);

  const root = document.documentElement;
  const body = document.body;

  // Toggle dark/light class
  if (effective === "dark") {
    root.classList.add("dark");
    root.classList.remove("light");
    body.classList.add("dark");
    body.classList.remove("light");
  } else {
    root.classList.add("light");
    root.classList.remove("dark");
    body.classList.add("light");
    body.classList.remove("dark");
  }

  // Mark the active pack so the CSS mapping block (index.css) routes every
  // semantic token through the --app-theme-* variables.
  root.setAttribute("data-theme-id", paletteId);

  const style = root.style;
  for (const role of THEME_COLOR_ROLES) {
    style.setProperty(`--app-theme-${kebabCase(role)}`, colors[role as ThemeColorRole]);
  }

  if (typeof window !== "undefined") {
    window.dispatchEvent(new CustomEvent("forge:theme-changed", {
      detail: { paletteId, appearance: effective }
    }));
  }

  return effective;
}

function kebabCase(name: string): string {
  return name.replace(/([a-z0-9])([A-Z])/g, "$1-$2").toLowerCase();
}

export function getActiveHtmlRenderTheme(): {
  appearance: ThemeAppearance;
  variables: Record<string, string>;
} {
  const palette = getStoredThemePalette();
  const mode = getStoredAppearanceMode();
  const effective = resolveEffectiveAppearance(mode);
  const themeDef = getThemeDefinition(palette);
  const colors = getThemeColorsForAppearance(themeDef, effective);

  return {
    appearance: effective,
    variables: {
      "--background": colors.canvas,
      "--foreground": colors.text,
      "--muted": colors.muted,
      "--muted-foreground": colors.mutedForeground,
      "--card": colors.surface,
      "--card-foreground": colors.text,
      "--popover": colors.surfaceOverlay,
      "--popover-foreground": colors.text,
      "--border": colors.border,
      "--input": colors.input,
      "--ring": colors.focus,
      "--primary": colors.messageAction,
      "--primary-foreground": colors.messageActionForeground,
      "--accent": colors.accentSurface,
      "--accent-foreground": colors.accentSurfaceForeground,
      "--destructive": colors.error,
      "--destructive-foreground": colors.errorForeground,
      "--warning": colors.warning,
      "--warning-foreground": colors.warningForeground,
      "--success": effective === "dark" ? "#10b981" : "#059669",
      "--success-foreground": effective === "dark" ? "#34d399" : "#047857",
      "--info": effective === "dark" ? "#3b82f6" : "#2563eb",
      "--info-foreground": effective === "dark" ? "#60a5fa" : "#1d4ed8",
      "--code-background": colors.codeBackground,
      "--code-foreground": colors.codeForeground,
      "--chart-1": colors.accent,
      "--chart-2": effective === "dark" ? "#2dd4bf" : "#0d9488",
      "--chart-3": effective === "dark" ? "#fbbf24" : "#d97706",
      "--chart-4": effective === "dark" ? "#c084fc" : "#9333ea",
      "--chart-5": effective === "dark" ? "#fb7185" : "#e11d48",
      "--chart-6": effective === "dark" ? "#a3e635" : "#65a30d",
      "--radius": "0.625rem",
      "--font-sans": DEFAULT_SANS_FONT_STACK,
      "--font-mono": DEFAULT_CODE_FONT_STACK,
    }
  };
}

export function saveThemePreference(paletteId: BuiltInThemeId, mode: ThemePreferenceMode) {
  try {
    localStorage.setItem(THEME_PALETTE_KEY, paletteId);
    localStorage.setItem(THEME_MODE_KEY, mode);
  } catch {
    // ignore
  }
  return applyTheme(paletteId, mode);
}

let systemListenerInitialized = false;

export function initThemeEngine() {
  const root = typeof document !== "undefined" ? document.documentElement : null;
  if (root) {
    // Add the suppression class so the initial application does not animate
    // every surface from the pre-paint defaults.
    root.classList.add("no-transitions");
  }
  applyTheme(getStoredThemePalette(), getStoredAppearanceMode());
  if (root) {
    applyAppearanceFontVariables(root, getStoredFontPreferences());
    applyAppearanceContrast(root, getStoredAppearanceContrast());
    saveChatWidth(getStoredChatWidth());
    saveDiffColorScheme(getStoredDiffColorScheme());
    requestAnimationFrame(() => {
      root.classList.remove("no-transitions");
    });
  }

  if (!systemListenerInitialized && typeof window !== "undefined" && window.matchMedia) {
    systemListenerInitialized = true;
    const media = window.matchMedia("(prefers-color-scheme: dark)");
    const handler = () => {
      const currentMode = getStoredAppearanceMode();
      if (currentMode === "system") {
        applyTheme(getStoredThemePalette(), "system");
      }
    };
    if (media.addEventListener) {
      media.addEventListener("change", handler);
    } else {
      media.addListener(handler);
    }
  }
}
