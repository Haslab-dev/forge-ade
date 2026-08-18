import { create } from "zustand";
import {
  EventsOn,
  LSPGetDiagnostics,
  LSPListServers,
  LSPRestartServer,
  LSPStopServer,
  LSPRestartAll,
  LSPStopAll,
} from "./wails";

export interface LSPServerItem {
  languageId: string;
  name: string;
  command: string;
  args: string[];
  status: "running" | "stopped" | "starting" | "error";
  pid?: number;
  workspaceRoot: string;
  openDocumentsCount: number;
  errorsCount: number;
  warningsCount: number;
  uptimeSeconds?: number;
  memoryMb?: number;
}
export interface DiagnosticItem {
  range: {
    start: { line: number; character: number };
    end: { line: number; character: number };
  };
  severity?: 1 | 2 | 3 | 4; // 1 = Error, 2 = Warning, 3 = Info, 4 = Hint
  code?: number | string;
  source?: string;
  message: string;
}

export interface FileDiagnosticData {
  errors: number;
  warnings: number;
  diagnostics: DiagnosticItem[];
}

interface LSPState {
  diagnostics: Record<string, FileDiagnosticData>;
  servers: LSPServerItem[];
  loading: boolean;
  setDiagnostics: (diags: Record<string, FileDiagnosticData>) => void;
  setServers: (servers: LSPServerItem[]) => void;
  updateFileDiagnostics: (filePath: string, data: FileDiagnosticData) => void;
  refreshDiagnostics: () => Promise<void>;
  fetchServers: () => Promise<void>;
  restartServer: (languageId: string) => Promise<boolean>;
  stopServer: (languageId: string) => Promise<boolean>;
  restartAllServers: () => Promise<void>;
  stopAllServers: () => Promise<void>;
}

export const useLSPStore = create<LSPState>((set, get) => ({
  diagnostics: {},
  servers: [],
  loading: false,
  setDiagnostics: (diagnostics) => set({ diagnostics }),
  setServers: (servers) => set({ servers }),
  updateFileDiagnostics: (filePath, data) =>
    set((state) => ({
      diagnostics: {
        ...state.diagnostics,
        [filePath]: data,
      },
    })),
  refreshDiagnostics: async () => {
    try {
      const res = await LSPGetDiagnostics();
      if (res && typeof res === "object") {
        set({ diagnostics: res as Record<string, FileDiagnosticData> });
      }
    } catch {}
  },
  fetchServers: async () => {
    try {
      const res = await LSPListServers();
      if (Array.isArray(res)) {
        set({ servers: res as LSPServerItem[] });
      }
    } catch {}
  },
  restartServer: async (languageId: string) => {
    const ok = await LSPRestartServer(languageId);
    await get().fetchServers();
    await get().refreshDiagnostics();
    return ok;
  },
  stopServer: async (languageId: string) => {
    const ok = await LSPStopServer(languageId);
    await get().fetchServers();
    return ok;
  },
  restartAllServers: async () => {
    await LSPRestartAll();
    await get().fetchServers();
    await get().refreshDiagnostics();
  },
  stopAllServers: async () => {
    await LSPStopAll();
    await get().fetchServers();
  },
}));

// Initialize real-time diagnostics event listener
if (typeof window !== "undefined") {
  EventsOn("lsp:diagnostics", (payload: any) => {
    if (payload && typeof payload === "object") {
      useLSPStore.getState().setDiagnostics(payload);
    }
  });
  EventsOn("lsp:servers_changed", () => {
    useLSPStore.getState().fetchServers();
  });

  // Initial load of servers
  useLSPStore.getState().fetchServers();

  // Initial load
  useLSPStore.getState().refreshDiagnostics();
}

/**
 * Returns diagnostic counts for a single file.
 */
export function getFileDiagnostics(filePath: string): { errors: number; warnings: number; diagnostics: DiagnosticItem[] } {
  const diags = useLSPStore.getState().diagnostics;
  const match = diags[filePath];
  if (match) {
    return {
      errors: match.errors || 0,
      warnings: match.warnings || 0,
      diagnostics: match.diagnostics || [],
    };
  }
  return { errors: 0, warnings: 0, diagnostics: [] };
}

/**
 * Calculates aggregated error and warning count for a folder directory by inspecting all descendant files.
 */
export function getFolderDiagnostics(folderPath: string, diagsMap: Record<string, FileDiagnosticData>): { errors: number; warnings: number } {
  let errors = 0;
  let warnings = 0;
  const normalized = folderPath.endsWith("/") ? folderPath : folderPath + "/";

  for (const [filePath, data] of Object.entries(diagsMap)) {
    if (filePath.startsWith(normalized)) {
      errors += data.errors || 0;
      warnings += data.warnings || 0;
    }
  }

  return { errors, warnings };
}
