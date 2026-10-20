import {
  BuiltInThemeId,
  ThemeAppearance,
  getThemeDefinition,
  ThemeColors
} from "./themePalettes";

export type ThemePreferenceMode = ThemeAppearance | "system";

const THEME_PALETTE_KEY = "forge:theme-palette";
const THEME_MODE_KEY = "forge:theme-mode";

export function getStoredThemePalette(): BuiltInThemeId {
  try {
    const val = localStorage.getItem(THEME_PALETTE_KEY);
    if (val === "t3-chat" || val === "grove" || val === "ocean" || val === "ember" || val === "iris") {
      return val;
    }
  } catch {
    // ignore
  }
  return "t3-chat";
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

export function applyTheme(paletteId: BuiltInThemeId, mode: ThemePreferenceMode): ThemeAppearance {
  const effective = resolveEffectiveAppearance(mode);
  const themeDef = getThemeDefinition(paletteId);
  const colors: ThemeColors = effective === "dark" ? themeDef.variants.dark : themeDef.colors;

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

  // Set modern T3 token variables
  const style = root.style;
  style.setProperty("--canvas", colors.canvas);
  style.setProperty("--chrome", colors.chrome);
  style.setProperty("--toolbar", colors.toolbar);
  style.setProperty("--toolbar-foreground", colors.toolbarForeground);
  style.setProperty("--toolbar-border", colors.toolbarBorder);
  style.setProperty("--toolbar-control", colors.toolbarControl);
  style.setProperty("--toolbar-control-foreground", colors.toolbarControlForeground);
  style.setProperty("--surface", colors.surface);
  style.setProperty("--surface-raised", colors.surfaceRaised);
  style.setProperty("--surface-overlay", colors.surfaceOverlay);
  style.setProperty("--text", colors.text);
  style.setProperty("--text-muted", colors.textMuted);
  style.setProperty("--secondary-label", colors.secondaryLabel);
  style.setProperty("--icon-muted", colors.iconMuted);
  style.setProperty("--border", colors.border);
  style.setProperty("--input", colors.input);
  style.setProperty("--focus", colors.focus);
  style.setProperty("--accent", colors.accent);
  style.setProperty("--accent-foreground", colors.accentForeground);
  style.setProperty("--accent-surface", colors.accentSurface);
  style.setProperty("--accent-surface-foreground", colors.accentSurfaceForeground);
  style.setProperty("--message-surface", colors.messageSurface);
  style.setProperty("--message-foreground", colors.messageForeground);
  style.setProperty("--code-background", colors.codeBackground);
  style.setProperty("--code-foreground", colors.codeForeground);
  style.setProperty("--sidebar", colors.sidebar);
  style.setProperty("--sidebar-foreground", colors.sidebarForeground);
  style.setProperty("--sidebar-muted-foreground", colors.sidebarMutedForeground);
  style.setProperty("--sidebar-control-surface", colors.sidebarControlSurface);
  style.setProperty("--sidebar-row-hover", colors.sidebarRowHover);
  style.setProperty("--sidebar-row-active", colors.sidebarRowActive);
  style.setProperty("--sidebar-row-selected", colors.sidebarRowSelected);
  style.setProperty("--sidebar-border", colors.sidebarBorder);
  style.setProperty("--terminal-background", colors.terminalBackground);
  style.setProperty("--terminal-foreground", colors.terminalForeground);
  style.setProperty("--terminal-cursor", colors.terminalCursor);
  style.setProperty("--terminal-selection", colors.terminalSelection);
  style.setProperty("--error", colors.error);
  style.setProperty("--error-foreground", colors.errorForeground);
  style.setProperty("--error-surface", colors.errorSurface);
  style.setProperty("--warning", colors.warning);
  style.setProperty("--warning-foreground", colors.warningForeground);
  style.setProperty("--warning-surface", colors.warningSurface);
  style.setProperty("--update", colors.update);
  style.setProperty("--update-foreground", colors.updateForeground);

  // Map to legacy ForgeADE tokens for 100% backward compatibility
  style.setProperty("--bg-app", colors.canvas);
  style.setProperty("--bg-sidebar", colors.sidebar);
  style.setProperty("--bg-panel", colors.surface);
  style.setProperty("--bg-elevated", colors.surfaceRaised);
  style.setProperty("--bg-surface", colors.surface);
  style.setProperty("--bg-surface-hover", colors.sidebarRowHover);
  style.setProperty("--bg-surface-active", colors.sidebarRowActive);
  style.setProperty("--fg-primary", colors.text);
  style.setProperty("--fg-secondary", colors.textMuted);
  style.setProperty("--fg-tertiary", colors.secondaryLabel);
  style.setProperty("--fg-muted", colors.textMuted);
  style.setProperty("--border-default", colors.border);
  style.setProperty("--accent-primary", colors.accent);

  if (typeof window !== "undefined") {
    window.dispatchEvent(new CustomEvent("forge:theme-changed", {
      detail: { paletteId, appearance: effective }
    }));
  }

  return effective;
}

export function getActiveHtmlRenderTheme(): {
  appearance: ThemeAppearance;
  variables: Record<string, string>;
} {
  const palette = getStoredThemePalette();
  const mode = getStoredAppearanceMode();
  const effective = resolveEffectiveAppearance(mode);
  const themeDef = getThemeDefinition(palette);
  const colors = effective === "dark" ? themeDef.variants.dark : themeDef.colors;

  return {
    appearance: effective,
    variables: {
      "--background": colors.canvas,
      "--foreground": colors.text,
      "--muted": colors.textMuted,
      "--muted-foreground": colors.secondaryLabel,
      "--card": colors.surface,
      "--card-foreground": colors.text,
      "--popover": colors.surfaceOverlay,
      "--popover-foreground": colors.text,
      "--border": colors.border,
      "--input": colors.input,
      "--ring": colors.focus,
      "--primary": colors.accent,
      "--primary-foreground": colors.accentForeground,
      "--accent": colors.accent,
      "--accent-foreground": colors.accentForeground,
      "--accent-surface": colors.accentSurface,
      "--accent-surface-foreground": colors.accentSurfaceForeground,
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
      "--radius": "0.5rem",
      "--font-sans": '-apple-system, BlinkMacSystemFont, "Segoe UI", system-ui, sans-serif',
      "--font-mono": '"SF Mono", Menlo, Monaco, Consolas, monospace',
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
  const palette = getStoredThemePalette();
  const mode = getStoredAppearanceMode();
  applyTheme(palette, mode);

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
