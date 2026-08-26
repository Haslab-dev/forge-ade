import { FormatCode } from "./native";
// Editor behavior settings (settings modal → Editor tab), persisted in
// localStorage. "auto" values defer to the project's prettier/editorconfig;
// explicit values are sent as overrides on top of project config.

export interface EditorSettings {
  formatOnSave: boolean;
  /** "auto" follows project config; number forces indent width. */
  tabWidth: number | "auto";
  /** "auto" follows project config; boolean forces spaces vs tabs. */
  useTabs: boolean | "auto";
}

const STORAGE_KEY = "forge-ade-editor-settings";

export const DEFAULT_EDITOR_SETTINGS: EditorSettings = {
  formatOnSave: true,
  tabWidth: "auto",
  useTabs: "auto",
};

export function loadEditorSettings(): EditorSettings {
  try {
    const raw = localStorage.getItem(STORAGE_KEY);
    if (!raw) return { ...DEFAULT_EDITOR_SETTINGS };
    const parsed = JSON.parse(raw);
    return {
      formatOnSave: typeof parsed.formatOnSave === "boolean" ? parsed.formatOnSave : DEFAULT_EDITOR_SETTINGS.formatOnSave,
      tabWidth: typeof parsed.tabWidth === "number" ? parsed.tabWidth : "auto",
      useTabs: typeof parsed.useTabs === "boolean" ? parsed.useTabs : "auto",
    };
  } catch {
    return { ...DEFAULT_EDITOR_SETTINGS };
  }
}

export function saveEditorSettings(settings: EditorSettings): void {
  localStorage.setItem(STORAGE_KEY, JSON.stringify(settings));
}

/** Overrides payload for FormatCode; omits "auto" so project config wins. */
export function formatOverrides(settings: EditorSettings): { tabWidth?: number; useTabs?: boolean } {
  return {
    ...(typeof settings.tabWidth === "number" ? { tabWidth: settings.tabWidth } : {}),
    ...(typeof settings.useTabs === "boolean" ? { useTabs: settings.useTabs } : {}),
  };
}

/**
 * Formats code with the user's editor settings layered over the project's
 * prettier/editorconfig. Reads settings per call so changes in the settings
 * modal apply without a reload.
 */
export function formatWithSettings(path: string, content: string): Promise<string> {
  return FormatCode(path, content, formatOverrides(loadEditorSettings()));
}
