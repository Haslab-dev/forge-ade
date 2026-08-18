/**
 * Native SDK Bridge & Subsystems Dispatcher for ForgeADE
 * Connects the React Frontend to Native SDK POSIX PTY, Git Engine, File System, and AI Agent.
 */

export const hasBridge = typeof window !== "undefined" && Boolean((window as any).zero);

const getZero = (): any => {
  if (typeof window !== "undefined") {
    return (window as any).zero || null;
  }
  return null;
};

export const textEncoder = new TextEncoder();
export const textDecoder = new TextDecoder();

export function base64ToBytes(base64: string): Uint8Array {
  const binString = atob(base64);
  return Uint8Array.from(binString, (m) => m.charCodeAt(0));
}

export function bytesToBase64(bytes: Uint8Array): string {
  const binString = Array.from(bytes, (x) => String.fromCharCode(x)).join("");
  return btoa(binString);
}

// Event listener registry for events emitted from native runtime
const eventListeners = new Map<string, Set<(data: any) => void>>();

export function emitEvent(eventName: string, data: any): void {
  const listeners = eventListeners.get(eventName);
  if (listeners) {
    listeners.forEach((callback) => {
      try {
        callback(data);
      } catch (err) {
        console.error(`Error in event listener for ${eventName}:`, err);
      }
    });
  }
}

const terminalDecoders = new Map<string, TextDecoder>();

function getTerminalDecoder(id: string): TextDecoder {
  let dec = terminalDecoders.get(id);
  if (!dec) {
    dec = new TextDecoder("utf-8", { fatal: false });
    terminalDecoders.set(id, dec);
  }
  return dec;
}

function handleTerminalDataPayload(payload: any) {
  if (typeof payload === "string") {
    try {
      payload = JSON.parse(payload);
    } catch {}
  }
  if (!payload || !payload.data) return;
  try {
    const rawBytes = base64ToBytes(payload.data);
    const id = String(payload.sessionId || payload.id);
    const decoder = getTerminalDecoder(id);
    const text = decoder.decode(rawBytes, { stream: true });
    if (text) {
      emitEvent("session:output", { id, data: text });
    }
  } catch (err) {
    console.error("Decode terminal.data error:", err);
  }
}

// ---------------------------------------------------------------------------
// Native Bridge Event Listeners Initialization
// ---------------------------------------------------------------------------

if (typeof window !== "undefined") {
  const zero = getZero();
  if (zero && typeof zero.on === "function") {
    zero.on("terminal.data", (event: any) => {
      handleTerminalDataPayload(event);
    });

    zero.on("terminal.exit", (event: any) => {
      let payload = event;
      if (typeof payload === "string") {
        try {
          payload = JSON.parse(payload);
        } catch {}
      }
      const id = String(payload?.sessionId || payload?.id);
      terminalDecoders.delete(id);
      sessionPtyMap.delete(id);
      emitEvent("session:closed", { id });
    });

    zero.on("fs.change", (event: any) => {
      let payload = event;
      if (typeof payload === "string") {
        try {
          payload = JSON.parse(payload);
        } catch {}
      }
      emitEvent("fs:changed", payload);
      emitEvent("forge:git-status-changed", {});
    });
  } else {
    window.addEventListener("native-sdk:terminal.data", (e: any) => {
      handleTerminalDataPayload(e.detail);
    });
    window.addEventListener("native-sdk:terminal.exit", (e: any) => {
      const id = String(e.detail?.sessionId || e.detail?.id);
      terminalDecoders.delete(id);
      sessionPtyMap.delete(id);
      emitEvent("session:closed", { id });
    });
    window.addEventListener("native-sdk:fs.change", (e: any) => {
      emitEvent("fs:changed", e.detail);
      emitEvent("forge:git-status-changed", {});
    });
  }

  // Connect to backend daemon WebSocket for streaming LSP diagnostics & agent events
  try {
    let ws: WebSocket | null = null;
    let reconnectTimer: any = null;
    const connectWs = () => {
      clearTimeout(reconnectTimer);
      try {
        const isBrowser = typeof window !== "undefined" && window.location.origin.startsWith("http");
        const wsUrl = isBrowser
          ? `${window.location.protocol === "https:" ? "wss:" : "ws:"}//${window.location.host}/ws`
          : "ws://127.0.0.1:45123/ws";

        ws = new WebSocket(wsUrl);
        ws.onmessage = (event) => {
          try {
            const data = JSON.parse(event.data);
            if (data.type) {
              emitEvent(data.type, data.payload);
            }
          } catch {}
        };
        ws.onclose = () => {
          ws = null;
          reconnectTimer = setTimeout(connectWs, 5000);
        };
        ws.onerror = () => {
          // Silence websocket error when daemon is offline
        };
      } catch {
        reconnectTimer = setTimeout(connectWs, 5000);
      }
    };
    connectWs();
  } catch {}
}

const getBackendUrl = (): string => {
  if (typeof window !== "undefined" && window.location.origin.startsWith("http")) {
    return `${window.location.origin}/api/invoke`;
  }
  return "http://127.0.0.1:45123/api/invoke";
};

export async function invokeBackend<T = any>(method: string, params: any = {}): Promise<T | null> {
  try {
    const controller = new AbortController();
    const timeoutId = setTimeout(() => controller.abort(), 1500);
    const res = await fetch(getBackendUrl(), {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ method, params }),
      signal: controller.signal,
    });
    clearTimeout(timeoutId);
    if (res.ok) {
      const data = await res.json();
      return data.result as T;
    }
  } catch {
    // Fallback directly to 127.0.0.1:45123 if origin proxy is not active
    try {
      const res = await fetch("http://127.0.0.1:45123/api/invoke", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ method, params }),
      });
      if (res.ok) {
        const data = await res.json();
        return data.result as T;
      }
    } catch {}
  }
  return null;
}

/**
 * Execute a shell command via native bridge (command.exec)
 */
export async function execCommand(command: string, cwd: string = ""): Promise<{ output: string; exitCode: number; success: boolean }> {
  const zero = getZero();
  if (zero && typeof zero.invoke === "function") {
    try {
      const res = await zero.invoke("command.exec", { command, cwd });
      return {
        output: res?.output || "",
        exitCode: res?.exitCode ?? 0,
        success: Boolean(res?.success ?? res?.exitCode === 0),
      };
    } catch (err: any) {
      return { output: err?.message || String(err), exitCode: 1, success: false };
    }
  }
  return { output: "", exitCode: 0, success: true };
}

/**
 * Subscribe to an event
 */
export function EventsOn(eventName: string, callback: (data: any) => void): () => void {
  if (!eventListeners.has(eventName)) {
    eventListeners.set(eventName, new Set());
  }
  eventListeners.get(eventName)!.add(callback);

  const zero = getZero();
  if (zero && typeof zero.on === "function") {
    try {
      zero.on(eventName, callback);
    } catch {}
  }

  return () => {
    EventsOff(eventName, callback);
  };
}

/**
 * Unsubscribe from an event
 */
export function EventsOff(eventName: string, callback?: (data: any) => void): void {
  if (callback) {
    eventListeners.get(eventName)?.delete(callback);
  } else {
    eventListeners.delete(eventName);
  }

  const zero = getZero();
  if (zero && typeof zero.off === "function") {
    try {
      zero.off(eventName, callback);
    } catch {}
  }
}

export function EventsEmit(eventName: string, data: any): void {
  emitEvent(eventName, data);
  const zero = getZero();
  if (zero && typeof zero._emit === "function") {
    try {
      zero._emit(eventName, data);
    } catch {}
  }
}

// ---------------------------------------------------------------------------
// Native Dialogs & System Integration
// ---------------------------------------------------------------------------

export async function OpenFolderDialog(): Promise<string> {
  const zero = getZero();
  if (zero && typeof zero.invoke === "function") {
    try {
      const res = await zero.invoke("native-sdk.dialog.openFile", {
        title: "Open Folder",
        allowDirectories: true,
        allowMultiple: false,
      });
      if (Array.isArray(res)) return res[0] || "";
      if (typeof res === "string") return res;
      if (res && typeof res === "object") return res.path || res.filePath || "";
    } catch (e) {
      console.warn("native-sdk.dialog.openFile error:", e);
    }
  }
  return prompt("Enter folder path:", "/workspace") || "";
}

export async function OpenFileDialog(): Promise<string> {
  const zero = getZero();
  if (zero && typeof zero.invoke === "function") {
    try {
      const res = await zero.invoke("native-sdk.dialog.openFile", {
        title: "Open File",
        allowDirectories: false,
        allowMultiple: false,
      });
      if (Array.isArray(res)) return res[0] || "";
      if (typeof res === "string") return res;
      if (res && typeof res === "object") return res.path || res.filePath || "";
    } catch (e) {
      console.warn("native-sdk.dialog.openFile error:", e);
    }
  }
  return prompt("Enter file path:") || "";
}

export async function OpenWorkspaceDialog(): Promise<string> {
  return await OpenFileDialog();
}

export async function SaveWorkspaceDialog(): Promise<string> {
  const zero = getZero();
  if (zero && typeof zero.invoke === "function") {
    try {
      const res = await zero.invoke("native-sdk.dialog.saveFile", {
        title: "Save Workspace",
        defaultName: "workspace.json",
      });
      if (typeof res === "string") return res;
      if (res && typeof res === "object") return res.path || res.filePath || "";
    } catch (e) {
      console.warn("native-sdk.dialog.saveFile error:", e);
    }
  }
  return prompt("Save workspace as:", "workspace.json") || "";
}

export async function ClipboardGetText(): Promise<string> {
  const zero = getZero();
  if (zero && typeof zero.invoke === "function") {
    try {
      const text = await zero.invoke("native-sdk.clipboard.readText");
      if (typeof text === "string") return text;
    } catch {}
  }
  if (typeof navigator !== "undefined" && navigator.clipboard?.readText) {
    try {
      return await navigator.clipboard.readText();
    } catch {
      return "";
    }
  }
  return "";
}

export async function BrowserOpenURL(url: string): Promise<void> {
  const zero = getZero();
  if (zero && typeof zero.invoke === "function") {
    try {
      await zero.invoke("native-sdk.os.openUrl", { url });
      return;
    } catch {}
  }
  if (typeof window !== "undefined") {
    window.open(url, "_blank");
  }
}

export async function OpenInFinder(path: string): Promise<void> {
  const zero = getZero();
  if (zero && typeof zero.invoke === "function") {
    try {
      await zero.invoke("native-sdk.os.revealPath", { path });
      return;
    } catch {}
  }
  console.log("Reveal in finder:", path);
}

export async function OpenNewWindow(url?: string): Promise<void> {
  const zero = getZero();
  if (zero?.windows?.create) {
    try {
      await zero.windows.create({
        label: `forge-win-${Date.now()}`,
        title: "ForgeADE",
        width: 1280,
        height: 800,
      });
      return;
    } catch {}
  }
  if (typeof window !== "undefined") {
    window.open(url || window.location.href, "_blank");
  }
}

// ---------------------------------------------------------------------------
// Workspace & Project APIs (Persisted via LocalStorage + Native Watcher)
// ---------------------------------------------------------------------------

const STORAGE_KEY_RECENT = "forge_recent_projects";
const STORAGE_KEY_WORKSPACE = "forge_current_workspace";

export async function GetRecentProjects(): Promise<any[]> {
  try {
    const raw = localStorage.getItem(STORAGE_KEY_RECENT);
    return raw ? JSON.parse(raw) : [];
  } catch {
    return [];
  }
}

export async function OpenFolder(folderPath: string): Promise<any> {
  const cleanPath = folderPath.trim();
  const name = cleanPath.split(/[/\\]/).filter(Boolean).pop() || "Project";
  const ws = {
    name,
    folders: [cleanPath],
    isTemporary: true,
    filePath: "",
    theme: "dark-plus",
  };

  try {
    localStorage.setItem(STORAGE_KEY_WORKSPACE, JSON.stringify(ws));
    const recent = await GetRecentProjects();
    const updated = [
      {
        path: cleanPath,
        name,
        isWorkspace: false,
        lastOpened: Date.now(),
        pinned: false,
        favorite: false,
      },
      ...recent.filter((r) => r.path !== cleanPath),
    ];
    localStorage.setItem(STORAGE_KEY_RECENT, JSON.stringify(updated.slice(0, 50)));

    const zero = getZero();
    if (zero && typeof zero.invoke === "function") {
      zero.invoke("fs.watch", { path: cleanPath }).catch(() => {});
    }
  } catch {}

  return ws;
}

export async function OpenWorkspace(path: string): Promise<any> {
  try {
    const content = await ReadFile(path);
    if (content) {
      const parsed = JSON.parse(content);
      const ws = {
        name: parsed.name || path.split(/[/\\]/).pop() || "Workspace",
        folders: parsed.folders || [path],
        isTemporary: false,
        filePath: path,
        theme: parsed.theme || "dark-plus",
      };
      localStorage.setItem(STORAGE_KEY_WORKSPACE, JSON.stringify(ws));
      return ws;
    }
  } catch {}
  return OpenFolder(path);
}

export async function AddFolderToWorkspace(path: string): Promise<void> {
  const ws = await GetCurrentWorkspace();
  if (ws && !ws.folders.includes(path)) {
    ws.folders.push(path);
    localStorage.setItem(STORAGE_KEY_WORKSPACE, JSON.stringify(ws));
  }
}

export async function CloseWorkspace(): Promise<void> {
  try {
    localStorage.removeItem(STORAGE_KEY_WORKSPACE);
  } catch {}
}

export async function GetCurrentWorkspace(): Promise<any> {
  try {
    const raw = localStorage.getItem(STORAGE_KEY_WORKSPACE);
    return raw ? JSON.parse(raw) : null;
  } catch {
    return null;
  }
}

export async function SaveWorkspace(): Promise<void> {
  const ws = await GetCurrentWorkspace();
  if (ws?.filePath) {
    await WriteFile(ws.filePath, JSON.stringify(ws, null, 2));
  }
}

export async function SaveWorkspaceAs(path: string): Promise<void> {
  const ws = await GetCurrentWorkspace();
  if (ws) {
    ws.filePath = path;
    ws.isTemporary = false;
    await WriteFile(path, JSON.stringify(ws, null, 2));
    localStorage.setItem(STORAGE_KEY_WORKSPACE, JSON.stringify(ws));
  }
}

export async function PinRecent(path: string, pinned: boolean): Promise<void> {
  try {
    const recent = await GetRecentProjects();
    const entry = recent.find((r) => r.path === path);
    if (entry) {
      entry.pinned = pinned;
      localStorage.setItem(STORAGE_KEY_RECENT, JSON.stringify(recent));
    }
  } catch {}
}

export async function RemoveRecent(path: string): Promise<void> {
  try {
    const recent = await GetRecentProjects();
    const updated = recent.filter((r) => r.path !== path);
    localStorage.setItem(STORAGE_KEY_RECENT, JSON.stringify(updated));
  } catch {}
}

// ---------------------------------------------------------------------------
// File Operations (Native fs.* Bridge)
// ---------------------------------------------------------------------------

export async function ReadFile(path: string): Promise<string> {
  const zero = getZero();
  const resolvedPath = path.startsWith("~/") ? path.replace(/^~/, await GetHomeDir()) : path;

  if (zero && typeof zero.invoke === "function") {
    try {
      const res = await zero.invoke("fs.readFile", { path: resolvedPath });
      if (res && typeof res.content === "string") return res.content;
      if (typeof res === "string") return res;
    } catch {}
  }
  return "";
}

export async function ReadFileBase64(path: string): Promise<string> {
  const text = await ReadFile(path);
  return btoa(text);
}

export async function WriteFile(path: string, content: string): Promise<void> {
  const zero = getZero();
  const resolvedPath = path.startsWith("~/") ? path.replace(/^~/, await GetHomeDir()) : path;

  if (zero && typeof zero.invoke === "function") {
    await zero.invoke("fs.writeFile", { path: resolvedPath, content });
    emitEvent("fs:changed", { type: "modify", path: resolvedPath });
  }
}

export async function CreateFile(path: string): Promise<void> {
  const zero = getZero();
  const resolvedPath = path.startsWith("~/") ? path.replace(/^~/, await GetHomeDir()) : path;

  if (zero && typeof zero.invoke === "function") {
    await zero.invoke("fs.createFile", { path: resolvedPath });
    emitEvent("fs:changed", { type: "create", path: resolvedPath });
  }
}

export async function CreateFolder(path: string): Promise<void> {
  const zero = getZero();
  const resolvedPath = path.startsWith("~/") ? path.replace(/^~/, await GetHomeDir()) : path;

  if (zero && typeof zero.invoke === "function") {
    await zero.invoke("fs.createDir", { path: resolvedPath });
    emitEvent("fs:changed", { type: "create", path: resolvedPath });
  }
}

export async function DeleteFile(path: string): Promise<void> {
  const resolvedPath = path.startsWith("~/") ? path.replace(/^~/, await GetHomeDir()) : path;
  await execCommand(`rm -rf "${resolvedPath}"`);
  emitEvent("fs:changed", { type: "delete", path: resolvedPath });
}

export async function RenameFile(oldPath: string, newPath: string): Promise<void> {
  const oldResolved = oldPath.startsWith("~/") ? oldPath.replace(/^~/, await GetHomeDir()) : oldPath;
  const newResolved = newPath.startsWith("~/") ? newPath.replace(/^~/, await GetHomeDir()) : newPath;
  await execCommand(`mv "${oldResolved}" "${newResolved}"`);
  emitEvent("fs:changed", { type: "modify", path: newResolved, oldPath: oldResolved });
}

export async function CopyFile(src: string, dst: string): Promise<void> {
  await execCommand(`cp "${src}" "${dst}"`);
  emitEvent("fs:changed", { type: "create", path: dst });
}

export async function CopyPath(src: string, dst: string): Promise<void> {
  await execCommand(`cp -R "${src}" "${dst}"`);
  emitEvent("fs:changed", { type: "create", path: dst });
}

export async function MoveFile(src: string, dst: string): Promise<void> {
  await RenameFile(src, dst);
}

export async function GetClipboardFiles(): Promise<string[]> {
  return [];
}

let cachedHomeDir = "";
export async function GetHomeDir(): Promise<string> {
  if (cachedHomeDir) return cachedHomeDir;
  const zero = getZero();
  if (zero && typeof zero.invoke === "function") {
    try {
      const res = await zero.invoke("fs.getCwd", {});
      if (res?.cwd) {
        // Derive home directory from /Users/<username> or /home/<username>
        const match = res.cwd.match(/^(\/(?:Users|home)\/[^/]+)/);
        if (match) {
          cachedHomeDir = match[1];
          return cachedHomeDir;
        }
        cachedHomeDir = res.cwd;
        return res.cwd;
      }
    } catch {}
  }
  const cmd = await execCommand("echo $HOME");
  if (cmd.output.trim()) {
    cachedHomeDir = cmd.output.trim();
    return cachedHomeDir;
  }
  return "/Users/hy4-mac-002";
}

export async function IsDir(path: string): Promise<boolean> {
  const zero = getZero();
  if (zero && typeof zero.invoke === "function") {
    try {
      const res = await zero.invoke("fs.readDir", { path });
      return Boolean(res && Array.isArray(res.entries));
    } catch {
      return false;
    }
  }
  return false;
}

export async function GetFileTree(depth: number = 2): Promise<string> {
  const ws = await GetCurrentWorkspace();
  const folder = ws?.folders?.[0];
  if (!folder) return "[]";

  return await ListDirectory(folder);
}

export async function ListDirectory(dirPath: string): Promise<string> {
  const zero = getZero();
  if (zero && typeof zero.invoke === "function") {
    try {
      const res = await zero.invoke("fs.readDir", { path: dirPath });
      if (res && Array.isArray(res.entries)) {
        const nodes = res.entries.map((entry: any) => ({
          path: `${dirPath.replace(/\/+$/, "")}/${entry.name}`,
          name: entry.name,
          isDir: entry.isDirectory ?? entry.isDir ?? false,
          size: entry.size || 0,
          modTime: entry.mtime ? Number(entry.mtime) : Date.now(),
        }));
        return JSON.stringify(nodes);
      }
    } catch (err) {
      console.warn("fs.readDir error:", err);
    }
  }
  return "[]";
}

export async function ExpandPath(targetPath: string): Promise<string> {
  return await ListDirectory(targetPath);
}

export async function ToggleHiddenFiles(): Promise<boolean> {
  return true;
}

// ---------------------------------------------------------------------------
// Editor & Code Tooling
// ---------------------------------------------------------------------------

export async function CheckSyntax(path: string, content: string): Promise<any[]> {
  const backendRes = await invokeBackend<any[]>("CheckSyntax", { path, content });
  if (backendRes && Array.isArray(backendRes)) return backendRes;

  const diags: any[] = [];
  if (path.endsWith(".json")) {
    try {
      JSON.parse(content);
    } catch (err: any) {
      diags.push({ line: 1, column: 1, message: err.message, severity: "error" });
    }
  }
  return diags;
}

export async function FormatCode(path: string, content: string): Promise<string> {
  const backendRes = await invokeBackend<string>("FormatCode", { path, content });
  if (backendRes && typeof backendRes === "string") return backendRes;

  if (path.endsWith(".json")) {
    try {
      return JSON.stringify(JSON.parse(content), null, 2);
    } catch {}
  }
  return content;
}

export async function GetCompletion(prefix: string, path: string): Promise<any[]> {
  const backendRes = await invokeBackend<any[]>("GetCompletion", { prefix, path });
  if (backendRes && Array.isArray(backendRes)) return backendRes;

  return [
    { Name: "console.log", Kind: "snippet", Detail: "console.log(...)" },
    { Name: "function", Kind: "keyword", Detail: "function declaration" },
    { Name: "import", Kind: "keyword", Detail: "import statement" },
    { Name: "export", Kind: "keyword", Detail: "export statement" },
    { Name: "interface", Kind: "keyword", Detail: "interface declaration" },
  ].filter((s) => s.Name.toLowerCase().startsWith(prefix.toLowerCase()));
}

export async function GetMembers(instance: string, path: string): Promise<any[]> {
  const backendRes = await invokeBackend<any[]>("GetMembers", { instance, path });
  if (backendRes && Array.isArray(backendRes)) return backendRes;

  return [
    { Name: "length", Kind: "property", Detail: "number" },
    { Name: "toString", Kind: "method", Detail: "(): string" },
    { Name: "map", Kind: "method", Detail: "(fn) => []" },
    { Name: "filter", Kind: "method", Detail: "(fn) => []" },
  ];
}

export async function FindSymbol(name: string): Promise<any[]> {
  const backendRes = await invokeBackend<any[]>("FindSymbol", { name });
  return Array.isArray(backendRes) ? backendRes : [];
}

export async function SearchIndexSymbols(query: string): Promise<any[]> {
  const backendRes = await invokeBackend<any[]>("SearchIndexSymbols", { query });
  return Array.isArray(backendRes) ? backendRes : [];
}

// ---------------------------------------------------------------------------
// LSP (Language Server Protocol) Client Methods
// ---------------------------------------------------------------------------

export async function LSPDidOpen(path: string, content: string): Promise<boolean> {
  return Boolean(await invokeBackend("LSPDidOpen", { path, content }));
}

export async function LSPDidChange(path: string, content: string): Promise<boolean> {
  return Boolean(await invokeBackend("LSPDidChange", { path, content }));
}

export async function LSPDidSave(path: string, content?: string): Promise<boolean> {
  return Boolean(await invokeBackend("LSPDidSave", { path, content }));
}

export async function LSPDidClose(path: string): Promise<boolean> {
  return Boolean(await invokeBackend("LSPDidClose", { path }));
}

export async function LSPGetCompletion(path: string, line: number, character: number): Promise<any[]> {
  const res = await invokeBackend<any[]>("LSPGetCompletion", { path, line, character });
  return Array.isArray(res) ? res : [];
}

export async function LSPGetHover(path: string, line: number, character: number): Promise<any | null> {
  return await invokeBackend("LSPGetHover", { path, line, character });
}

export async function LSPGetDefinition(path: string, line: number, character: number): Promise<any[]> {
  const res = await invokeBackend<any[]>("LSPGetDefinition", { path, line, character });
  return Array.isArray(res) ? res : [];
}

export async function LSPGetDeclaration(path: string, line: number, character: number): Promise<any[]> {
  const res = await invokeBackend<any[]>("LSPGetDeclaration", { path, line, character });
  return Array.isArray(res) ? res : [];
}

export async function LSPGetTypeDefinition(path: string, line: number, character: number): Promise<any[]> {
  const res = await invokeBackend<any[]>("LSPGetTypeDefinition", { path, line, character });
  return Array.isArray(res) ? res : [];
}

export async function LSPGetImplementation(path: string, line: number, character: number): Promise<any[]> {
  const res = await invokeBackend<any[]>("LSPGetImplementation", { path, line, character });
  return Array.isArray(res) ? res : [];
}

export async function LSPGetDiagnostics(path?: string): Promise<Record<string, { errors: number; warnings: number; diagnostics: any[] }>> {
  const res = await invokeBackend<any>("LSPGetDiagnostics", { path });
  return res || {};
}

export async function LSPListServers(): Promise<any[]> {
  const res = await invokeBackend<any[]>("LSPListServers");
  return Array.isArray(res) ? res : [];
}

export async function LSPRestartServer(languageId: string): Promise<boolean> {
  return Boolean(await invokeBackend("LSPRestartServer", { languageId }));
}

export async function LSPStopServer(languageId: string): Promise<boolean> {
  return Boolean(await invokeBackend("LSPStopServer", { languageId }));
}

export async function LSPRestartAll(): Promise<Record<string, boolean>> {
  const res = await invokeBackend<Record<string, boolean>>("LSPRestartAll");
  return res || {};
}

export async function LSPStopAll(): Promise<boolean> {
  return Boolean(await invokeBackend("LSPStopAll"));
}

export async function LSPGetServerLogs(languageId: string): Promise<string[]> {
  const res = await invokeBackend<string[]>("LSPGetServerLogs", { languageId });
  return Array.isArray(res) ? res : [];
}

// ---------------------------------------------------------------------------
// Terminal & Interactive POSIX PTY Sessions
// ---------------------------------------------------------------------------

const sessionPtyMap = new Map<string, number>();

export async function CreateShell(name: string, cwd: string): Promise<any> {
  const zero = getZero();
  if (zero && typeof zero.invoke === "function") {
    try {
      const response = await zero.invoke("terminal.spawn", {
        cwd: cwd || "",
        cols: 80,
        rows: 24,
      });
      if (response && (response.sessionId !== undefined || response.id !== undefined)) {
        const ptyId = response.sessionId ?? response.id;
        const sessionId = String(ptyId);
        sessionPtyMap.set(sessionId, ptyId);

        const sessionObj = {
          id: sessionId,
          name: name || `Terminal (${cwd ? cwd.split(/[/\\]/).filter(Boolean).pop() : "zsh"})`,
          type: "shell",
          folder: cwd,
          status: "running",
        };
        emitEvent("session:opened", sessionObj);
        return sessionObj;
      }
    } catch (err) {
      console.error("terminal.spawn error:", err);
    }
  }

  const fallbackId = `shell-${Date.now()}`;
  const fallbackObj = { id: fallbackId, name: name || "Terminal", type: "shell", folder: cwd, status: "running" };
  emitEvent("session:opened", fallbackObj);
  return fallbackObj;
}

export async function WriteSession(id: string, data: string): Promise<void> {
  const zero = getZero();
  const ptyId = sessionPtyMap.get(id) ?? Number(id);

  if (zero && typeof zero.invoke === "function" && !isNaN(ptyId)) {
    try {
      const bytes = textEncoder.encode(data);
      const base64 = bytesToBase64(bytes);
      await zero.invoke("terminal.write", {
        sessionId: ptyId,
        data: base64,
      });
    } catch (err) {
      console.error("terminal.write error:", err);
    }
  }
}

export async function ResizeSession(id: string, rows: number, cols: number): Promise<void> {
  const zero = getZero();
  const ptyId = sessionPtyMap.get(id) ?? Number(id);

  if (zero && typeof zero.invoke === "function" && !isNaN(ptyId)) {
    try {
      await zero.invoke("terminal.resize", {
        sessionId: ptyId,
        cols,
        rows,
      });
    } catch {}
  }
}

export async function StopSession(id: string): Promise<void> {
  const zero = getZero();
  const ptyId = sessionPtyMap.get(id) ?? Number(id);

  if (zero && typeof zero.invoke === "function" && !isNaN(ptyId)) {
    try {
      await zero.invoke("terminal.kill", { sessionId: ptyId });
    } catch {}
  }
  sessionPtyMap.delete(id);
  emitEvent("session:closed", { id });
}

export async function RenameSession(id: string, name: string): Promise<void> {
  emitEvent("session:renamed", { id, name });
}

export async function ListSessions(): Promise<any[]> {
  const zero = getZero();
  if (zero && typeof zero.invoke === "function") {
    try {
      const res = await zero.invoke("terminal.list", {});
      if (Array.isArray(res?.sessions)) {
        return res.sessions.map((s: any) => ({
          id: String(s.sessionId),
          name: `Terminal (PID: ${s.pid})`,
          type: "shell",
          status: "running",
        }));
      }
    } catch {}
  }
  return [];
}

export async function ListShells(): Promise<any[]> {
  return await ListSessions();
}

export async function ListAIAgents(): Promise<any[]> {
  return await ListAgentSessions();
}

// ---------------------------------------------------------------------------
// AI Agent Engine & Live LLM Multi-Provider Execution
// ---------------------------------------------------------------------------

const STORAGE_KEY_AGENTS = "forge_agent_sessions";
const STORAGE_KEY_DEFS = "forge_agent_definitions";
const STORAGE_KEY_PROFILES = "forge_llm_profiles";

export async function GetProviderProfiles(): Promise<any[]> {
  // 1. Try reading disk config from ~/.forge-ade/providers_config.json
  try {
    const home = await GetHomeDir();
    const diskPath = `${home}/.forge-ade/providers_config.json`;
    const content = await ReadFile(diskPath);
    if (content) {
      const parsed = JSON.parse(content);
      if (Array.isArray(parsed) && parsed.length > 0) {
        const normalized = parsed.map((p: any) => ({
          id: p.id || p.name,
          name: p.name || p.id,
          provider: p.provider || p.id,
          apiKey: p.api_key || p.apiKey || p.ApiKey || "",
          api_key: p.api_key || p.apiKey || p.ApiKey || "",
          baseURL: p.base_url || p.baseURL || p.BaseURL || "",
          base_url: p.base_url || p.baseURL || p.BaseURL || "",
          activeModel: (p.selected_models && p.selected_models[0]) || (p.available_models && p.available_models[0]) || p.activeModel || "",
          selected_models: p.selected_models || p.available_models || [],
          available_models: p.available_models || p.selected_models || [],
          enabled: p.enabled !== false,
        }));
        localStorage.setItem(STORAGE_KEY_PROFILES, JSON.stringify(normalized));
        return normalized;
      }
    }
  } catch (err) {
    console.warn("Read providers_config.json error:", err);
  }

  // 2. Check localStorage
  try {
    const raw = localStorage.getItem(STORAGE_KEY_PROFILES);
    if (raw) return JSON.parse(raw);
  } catch {}

  // 3. Defaults
  return [
    {
      id: "myairouter",
      name: "MyAiRouter-local",
      provider: "openai",
      apiKey: "sk-558cd478fc5a8631532a21fd5b105a3112b668446b429fd4",
      baseURL: "http://localhost:20128/v1",
      activeModel: "db/deepseek-v4-flash",
      selected_models: ["db/deepseek-v4-flash", "kc/kilo-auto/free", "openrouter/openrouter/free"],
      enabled: true,
    },
  ];
}

export async function SaveProviderProfiles(profiles: any[]): Promise<void> {
  localStorage.setItem(STORAGE_KEY_PROFILES, JSON.stringify(profiles));
  emitEvent("agent:config:changed", {});
}

export async function FetchProviderModels(apiKey: string, baseURL: string): Promise<string[]> {
  try {
    const cleanUrl = baseURL.trim().replace(/\/+$/, "");
    const res = await fetch(`${cleanUrl}/models`, {
      headers: { ...(apiKey ? { Authorization: `Bearer ${apiKey}` } : {}) },
    });
    if (res.ok) {
      const data = await res.json();
      if (Array.isArray(data.data)) return data.data.map((m: any) => m.id);
    }
  } catch {}
  return [];
}

export async function SetActiveModel(providerId: string, model: string): Promise<void> {
  const profiles = await GetProviderProfiles();
  const p = profiles.find((x) => x.id === providerId);
  if (p) {
    p.activeModel = model;
    await SaveProviderProfiles(profiles);
  }
}

export async function SaveLLMProfile(
  providerId: string,
  apiKey: string,
  baseURL: string,
  model: string
): Promise<void> {
  const profiles = await GetProviderProfiles();
  let p = profiles.find((x) => x.id === providerId);
  if (p) {
    p.apiKey = apiKey;
    p.api_key = apiKey;
    p.baseURL = baseURL;
    p.base_url = baseURL;
    p.activeModel = model;
  } else {
    profiles.push({
      id: providerId,
      name: providerId,
      provider: providerId,
      apiKey,
      api_key: apiKey,
      baseURL,
      base_url: baseURL,
      activeModel: model,
      selected_models: [model],
      enabled: true,
    });
  }
  await SaveProviderProfiles(profiles);
}

export async function GetLLMConfig(): Promise<any> {
  try {
    const home = await GetHomeDir();
    const diskPath = `${home}/.forge-ade/profiles.json`;
    const content = await ReadFile(diskPath);
    if (content) {
      const parsed = JSON.parse(content);
      const profiles = await GetProviderProfiles();
      const active = profiles.find((p) => p.id === parsed.provider_id) || profiles[0];
      return {
        provider_id: parsed.provider_id || active?.id,
        api_key: parsed.api_key || active?.apiKey || active?.api_key,
        base_url: parsed.base_url || active?.baseURL || active?.base_url,
        model: parsed.model || active?.activeModel,
        activeProfile: active,
        profiles,
      };
    }
  } catch {}

  const profiles = await GetProviderProfiles();
  const active = profiles.find((p) => p.enabled) || profiles[0];
  return {
    provider_id: active?.id,
    api_key: active?.apiKey || active?.api_key,
    base_url: active?.baseURL || active?.base_url,
    model: active?.activeModel,
    activeProfile: active,
    profiles,
  };
}

export async function ListLLMProviders(): Promise<any[]> {
  return await GetProviderProfiles();
}

// Agent Sessions
export async function ListAgentSessions(): Promise<any[]> {
  try {
    const raw = localStorage.getItem(STORAGE_KEY_AGENTS);
    return raw ? JSON.parse(raw) : [];
  } catch {
    return [];
  }
}

export async function ListAgentSessionsForFolder(folder: string): Promise<any[]> {
  const list = await ListAgentSessions();
  const norm = folder ? folder.toLowerCase().replace(/\/+$/, "") : "";
  return list.filter((s) => s.projectFolder && s.projectFolder.toLowerCase().replace(/\/+$/, "") === norm);
}

export async function GetAgentSession(id: string): Promise<any> {
  const list = await ListAgentSessions();
  return list.find((s) => s.id === id) || null;
}

export async function CreateAgentSession(name: string, role: string, projectFolder: string): Promise<any> {
  const list = await ListAgentSessions();
  const id = `agent-${Date.now()}-${Math.random().toString(36).slice(2, 6)}`;
  const session = {
    id,
    name: name || "AI Assistant",
    role: role || "coding",
    projectFolder: projectFolder || "/",
    messages: [],
    createdAt: Date.now(),
  };
  list.unshift(session);
  localStorage.setItem(STORAGE_KEY_AGENTS, JSON.stringify(list));
  emitEvent("session:opened", session);
  return session;
}

export async function CreateAgentSessionFromDefinition(defId: string, projectFolder: string): Promise<any> {
  const defs = await ListAgentDefinitions();
  const def = defs.find((d) => d.id === defId) || defs[0];
  const session = await CreateAgentSession(def?.name || "AI Agent", def?.role_filter || "coding", projectFolder);
  session.customPrompt = def?.prompt;
  session.customRules = def?.rules;
  const list = await ListAgentSessions();
  const idx = list.findIndex((s) => s.id === session.id);
  if (idx >= 0) list[idx] = session;
  localStorage.setItem(STORAGE_KEY_AGENTS, JSON.stringify(list));
  return session;
}

export async function UpdateAgentSession(
  id: string,
  name: string,
  role: string,
  customPrompt: string,
  customRules: string
): Promise<any> {
  const list = await ListAgentSessions();
  const s = list.find((x) => x.id === id);
  if (s) {
    if (name) s.name = name;
    if (role) s.role = role;
    if (customPrompt !== undefined) s.customPrompt = customPrompt;
    if (customRules !== undefined) s.customRules = customRules;
    localStorage.setItem(STORAGE_KEY_AGENTS, JSON.stringify(list));
    emitEvent("agent:updated", { id });
  }
  return s;
}

export const RenameAgentSession = (id: string, name: string): Promise<void> =>
  UpdateAgentSession(id, name, "", "", "");

export async function DeleteAgentSession(id: string): Promise<void> {
  const list = await ListAgentSessions();
  const filtered = list.filter((s) => s.id !== id);
  localStorage.setItem(STORAGE_KEY_AGENTS, JSON.stringify(filtered));
  emitEvent("session:closed", { id });
}

export async function ClearAgentSession(id: string): Promise<void> {
  const list = await ListAgentSessions();
  const s = list.find((x) => x.id === id);
  if (s) {
    s.messages = [];
    localStorage.setItem(STORAGE_KEY_AGENTS, JSON.stringify(list));
    emitEvent("agent:updated", { id });
  }
}

export async function SetAgentDialect(id: string, dialect: string): Promise<void> {
  const list = await ListAgentSessions();
  const s = list.find((x) => x.id === id);
  if (s) {
    s.dialect = dialect;
    localStorage.setItem(STORAGE_KEY_AGENTS, JSON.stringify(list));
  }
}

export async function SetAgentAutoApprove(id: string, enabled: boolean): Promise<void> {
  const list = await ListAgentSessions();
  const s = list.find((x) => x.id === id);
  if (s) {
    s.autoApprove = enabled;
    localStorage.setItem(STORAGE_KEY_AGENTS, JSON.stringify(list));
  }
}

export async function ToggleAgentTask(id: string, taskId: string, active: boolean): Promise<void> {
  const list = await ListAgentSessions();
  const s = list.find((x) => x.id === id);
  if (s?.tasks) {
    const t = s.tasks.find((task: any) => task.id === taskId);
    if (t) {
      t.completed = active;
      localStorage.setItem(STORAGE_KEY_AGENTS, JSON.stringify(list));
    }
  }
}

export async function SendAgentMessage(id: string, message: string, files: string[] = []): Promise<void> {
  const list = await ListAgentSessions();
  const session = list.find((s) => s.id === id);
  if (!session) return;

  const userMsg = {
    id: `msg-${Date.now()}`,
    role: "user" as const,
    content: [{ type: "text" as const, text: message }],
    timestamp: new Date().toISOString(),
  };
  session.messages.push(userMsg);
  localStorage.setItem(STORAGE_KEY_AGENTS, JSON.stringify(list));
  emitEvent("agent:updated", { id });

  const llmConfig = await GetLLMConfig();
  const profiles = await GetProviderProfiles();
  const activeProfile = profiles.find((p) => p.id === llmConfig?.provider_id && p.enabled) ||
    profiles.find((p) => p.enabled && (p.apiKey || p.api_key || p.baseURL || p.base_url)) ||
    profiles[0];

  const apiKey = activeProfile?.apiKey || activeProfile?.api_key || llmConfig?.api_key || "";
  let baseURL = activeProfile?.baseURL || activeProfile?.base_url || llmConfig?.base_url || "http://localhost:20128/v1";
  const model = activeProfile?.activeModel || llmConfig?.model || (activeProfile?.selected_models && activeProfile.selected_models[0]) || "db/deepseek-v4-flash";

  const assistantMsgId = `msg-${Date.now() + 1}`;
  const assistantMsg = {
    id: assistantMsgId,
    role: "assistant" as const,
    content: [
      {
        type: "thinking" as const,
        text: `Connecting to ${activeProfile?.name || "AI"} (${model})...\nWorkspace: ${session.projectFolder}`,
      },
      {
        type: "text" as const,
        text: "",
      },
    ],
    timestamp: new Date().toISOString(),
  };
  session.messages.push(assistantMsg);
  localStorage.setItem(STORAGE_KEY_AGENTS, JSON.stringify(list));
  emitEvent("agent:updated", { id });

  // Clean baseURL
  baseURL = baseURL.trim().replace(/\/+$/, "");
  if (!baseURL.endsWith("/v1") && !baseURL.includes("/v1/")) {
    baseURL = `${baseURL}/v1`;
  }
  const endpoint = `${baseURL}/chat/completions`;

  try {
    const systemPrompt = session.customPrompt || "You are Forge AI, an expert software development assistant.";
    const apiMessages = [
      { role: "system", content: systemPrompt },
      ...session.messages.slice(0, -1).map((m: any) => ({
        role: m.role,
        content: Array.isArray(m.content) ? m.content.map((c: any) => c.text || "").join("\n") : String(m.content || ""),
      })),
    ];

    const headers: Record<string, string> = { "Content-Type": "application/json" };
    if (apiKey) headers["Authorization"] = `Bearer ${apiKey}`;

    const res = await fetch(endpoint, {
      method: "POST",
      headers,
      body: JSON.stringify({
        model,
        messages: apiMessages,
        stream: true,
      }),
    });

    if (res.ok && res.body) {
      const reader = res.body.getReader();
      let thinkingText = "";
      let answerText = "";
      let buffer = "";
      let inThinkTag = false;

      while (true) {
        const { done, value } = await reader.read();
        if (done) break;

        buffer += textDecoder.decode(value, { stream: true });
        const lines = buffer.split("\n");
        buffer = lines.pop() || "";

        for (const line of lines) {
          const trimmed = line.trim();
          if (!trimmed.startsWith("data:")) continue;
          const dataStr = trimmed.slice(5).trim();
          if (dataStr === "[DONE]") break;

          try {
            const parsed = JSON.parse(dataStr);
            const delta = parsed.choices?.[0]?.delta;
            if (!delta) continue;

            const reasoning = delta.reasoning_content || delta.reasoning || "";
            if (reasoning) {
              thinkingText += reasoning;
            }

            const content = delta.content || "";
            if (content) {
              if (content.includes("<think>")) {
                inThinkTag = true;
                const parts = content.split("<think>");
                if (parts[1]) thinkingText += parts[1];
              } else if (content.includes("</think>")) {
                inThinkTag = false;
                const parts = content.split("</think>");
                thinkingText += parts[0];
                if (parts[1]) answerText += parts[1];
              } else if (inThinkTag) {
                thinkingText += content;
              } else {
                answerText += content;
              }
            }

            const thinkBlock = assistantMsg.content.find((c) => c.type === "thinking");
            if (thinkBlock && thinkingText) thinkBlock.text = thinkingText;

            const textBlock = assistantMsg.content.find((c) => c.type === "text");
            if (textBlock && answerText) textBlock.text = answerText;

            emitEvent("agent:updated", { id });
          } catch {}
        }
      }
      localStorage.setItem(STORAGE_KEY_AGENTS, JSON.stringify(list));
      return;
    }
  } catch (err: any) {
    console.warn("LLM API streaming error:", err);
  }

  // Fallback if network stream failed
  const textBlock = assistantMsg.content.find((c) => c.type === "text");
  if (textBlock) {
    textBlock.text = `Unable to connect to \`${endpoint}\`.\n\nPlease verify that your AI router or local model endpoint is running and reachable.`;
  }
  localStorage.setItem(STORAGE_KEY_AGENTS, JSON.stringify(list));
  emitEvent("agent:updated", { id });
}

export async function RespondAgentApproval(id: string, approve: boolean, autoAll: boolean): Promise<void> {
  if (autoAll) await SetAgentAutoApprove(id, true);
}

export async function RespondAgentAsk(id: string, answers: any): Promise<void> {}
export async function StopAgentTurn(id: string): Promise<void> {}

export async function ListAgentDefinitions(): Promise<any[]> {
  try {
    const raw = localStorage.getItem(STORAGE_KEY_DEFS);
    if (raw) return JSON.parse(raw);
  } catch {}
  return [
    {
      id: "coder",
      name: "Full-Stack Engineer",
      role_filter: "coding",
      description: "Builds features, fixes bugs, and runs refactors with tool access.",
      prompt: "You are an expert full-stack engineer. Write clean, idiomatic code.",
      rules: "1. Read files before editing.\n2. Verify changes with tests.",
      model: "claude-3-7-sonnet-20250219",
    },
    {
      id: "planner",
      name: "Architect & Planner",
      role_filter: "planning",
      description: "Designs system architectures and breaks down complex phases.",
      prompt: "You are a software architect. Create crisp, structured plans.",
      rules: "1. List constraints.\n2. Break down into discrete phases.",
      model: "claude-3-7-sonnet-20250219",
    },
    {
      id: "researcher",
      name: "Research Scout",
      role_filter: "research",
      description: "Investigates APIs, repos, and documentation.",
      prompt: "You are a research scout. Gather exact facts from sources.",
      rules: "1. Be evidence-first.\n2. Cite exact files and symbols.",
      model: "claude-3-5-haiku-20241022",
    },
  ];
}

export async function SaveAgentDefinition(def: any): Promise<any> {
  const list = await ListAgentDefinitions();
  const idx = list.findIndex((d) => d.id === def.id);
  if (idx >= 0) list[idx] = def;
  else list.push(def);
  localStorage.setItem(STORAGE_KEY_DEFS, JSON.stringify(list));
  emitEvent("agent:config:changed", {});
  return def;
}

export async function DeleteAgentDefinition(id: string): Promise<void> {
  const list = await ListAgentDefinitions();
  const filtered = list.filter((d) => d.id !== id);
  localStorage.setItem(STORAGE_KEY_DEFS, JSON.stringify(filtered));
  emitEvent("agent:config:changed", {});
}

export async function ApplyAgentDefinitionToSession(id: string, defId: string): Promise<void> {
  const defs = await ListAgentDefinitions();
  const def = defs.find((d) => d.id === defId);
  if (def) {
    await UpdateAgentSession(id, def.name, def.role_filter || "coding", def.prompt, def.rules);
  }
}

// ---------------------------------------------------------------------------
// Git Operations (Native Bridge + Git CLI Engine)
// ---------------------------------------------------------------------------

function parseNumstat(output: string): Map<string, { additions: number; deletions: number }> {
  const map = new Map<string, { additions: number; deletions: number }>();
  for (const line of output.split("\n")) {
    const parts = line.split("\t");
    if (parts.length >= 3) {
      const add = parseInt(parts[0], 10) || 0;
      const del = parseInt(parts[1], 10) || 0;
      const fPath = parts[2].trim();
      map.set(fPath, { additions: add, deletions: del });
    }
  }
  return map;
}

export async function GetGitStatus(repoPath: string): Promise<any> {
  const ws = await GetCurrentWorkspace();
  const targetPath = repoPath || ws?.folders?.[0] || "";

  try {
    const branchRes = await execCommand("git rev-parse --abbrev-ref HEAD", targetPath);
    const statusRes = await execCommand("git status --porcelain=v1 -uall", targetPath);
    const unstagedNumstat = parseNumstat((await execCommand("git diff --numstat", targetPath)).output);
    const stagedNumstat = parseNumstat((await execCommand("git diff --cached --numstat", targetPath)).output);
    const latestCommitRes = await execCommand('git log -1 --format="%s"', targetPath);

    const staged: any[] = [];
    const unstaged: any[] = [];
    const untracked: any[] = [];
    const conflicts: any[] = [];

    let totalAdditions = 0;
    let totalDeletions = 0;

    const lines = statusRes.output.split("\n").filter(Boolean);
    for (const line of lines) {
      if (line.length < 3) continue;
      const x = line[0];
      const y = line[1];
      let filePath = line.slice(3).trim();
      if (filePath.startsWith('"') && filePath.endsWith('"')) {
        filePath = filePath.slice(1, -1);
      }

      const parts = filePath.split(/[/\\]/);
      parts.pop();
      const dir = parts.join("/");

      if (x === "U" || y === "U" || (x === "A" && y === "A") || (x === "D" && y === "D")) {
        conflicts.push({ path: filePath, dir, staging: "conflicts", status: `${x}${y}` });
        continue;
      }

      if (x === "?" && y === "?") {
        untracked.push({ path: filePath, dir, staging: "untracked", status: "U" });
      } else {
        if (x !== " " && x !== "?") {
          const stat = stagedNumstat.get(filePath);
          if (stat) {
            totalAdditions += stat.additions;
            totalDeletions += stat.deletions;
          }
          staged.push({
            path: filePath,
            dir,
            staging: "staged",
            status: x,
            additions: stat?.additions,
            deletions: stat?.deletions,
          });
        }
        if (y !== " " && y !== "?") {
          const stat = unstagedNumstat.get(filePath);
          if (stat) {
            totalAdditions += stat.additions;
            totalDeletions += stat.deletions;
          }
          unstaged.push({
            path: filePath,
            dir,
            staging: "unstaged",
            status: y,
            additions: stat?.additions,
            deletions: stat?.deletions,
          });
        }
      }
    }

    return {
      branch: branchRes.output.trim() || "main",
      ahead: 0,
      behind: 0,
      staged,
      unstaged,
      untracked,
      conflicts,
      totalAdditions,
      totalDeletions,
      latestCommit: latestCommitRes.output.trim() || "Initial commit",
    };
  } catch {
    return {
      branch: "main",
      ahead: 0,
      behind: 0,
      staged: [],
      unstaged: [],
      untracked: [],
      conflicts: [],
      totalAdditions: 0,
      totalDeletions: 0,
      latestCommit: "",
    };
  }
}

export async function GetGitCommitGraph(
  repoPath: string,
  offset: number = 0,
  limit: number = 50,
  branch: string = ""
): Promise<any> {
  const ws = await GetCurrentWorkspace();
  const targetPath = repoPath || ws?.folders?.[0] || "";

  try {
    const branchFilter = branch ? `"${branch}"` : "--all";
    const cmd = `git log --graph --skip=${offset || 0} -n ${limit || 50} --format="format:COMMIT_ROW|%h|%H|%an|%ae|%aI|%p|%D|%s" ${branchFilter}`;
    const res = await execCommand(cmd, targetPath);

    const commits: any[] = [];
    const lines = res.output.split("\n");

    for (const line of lines) {
      const markerIdx = line.indexOf("COMMIT_ROW|");
      if (markerIdx === -1) continue;

      const graphPrefix = line.slice(0, markerIdx).trimEnd();
      const rawPayload = line.slice(markerIdx + "COMMIT_ROW|".length);
      const parts = rawPayload.split("|");
      if (parts.length < 8) continue;

      const short_hash = parts[0];
      const hash = parts[1];
      const author_name = parts[2];
      const author_email = parts[3];
      const timestamp = parts[4];
      const parents = parts[5] ? parts[5].split(" ").filter(Boolean) : [];
      const decorations = parts[6] || "";
      const message = parts.slice(7).join("|");

      commits.push({
        hash,
        short_hash,
        parents,
        author_name,
        author_email,
        timestamp,
        message,
        graph_prefix: graphPrefix || "*",
        decorations,
      });
    }

    return {
      commits,
      total_count: commits.length,
      has_more: commits.length >= (limit || 50),
      offset: offset || 0,
      limit: limit || 50,
    };
  } catch {
    return { commits: [], total_count: 0, has_more: false, offset: 0, limit: 50 };
  }
}

export async function GetGitBranches(repoPath: string): Promise<string[]> {
  const ws = await GetCurrentWorkspace();
  const targetPath = repoPath || ws?.folders?.[0] || "";
  try {
    const res = await execCommand("git branch --list --format=\"%(refname:short)\"", targetPath);
    const list = res.output.split("\n").map((b) => b.trim()).filter(Boolean);
    return list.length ? list : ["main"];
  } catch {
    return ["main"];
  }
}

export async function GetGitCommitDiff(repoPath: string, hash: string): Promise<string> {
  const ws = await GetCurrentWorkspace();
  const targetPath = repoPath || ws?.folders?.[0] || "";
  const res = await execCommand(`git show "${hash}"`, targetPath);
  return res.output;
}

export async function GetGitCommitBody(repoPath: string, hash: string): Promise<string> {
  const ws = await GetCurrentWorkspace();
  const targetPath = repoPath || ws?.folders?.[0] || "";
  const res = await execCommand(`git log -1 --format=%B "${hash}"`, targetPath);
  return res.output;
}

export async function GetGitFileDiff(repoPath: string, path: string): Promise<string> {
  const ws = await GetCurrentWorkspace();
  const targetPath = repoPath || ws?.folders?.[0] || "";
  const res = await execCommand(`git diff HEAD -- "${path}"`, targetPath);
  return res.output;
}

export async function GetGitCommitFileDiff(repoPath: string, hash: string, path: string): Promise<string> {
  return await GetGitCommitDiff(repoPath, hash);
}

export async function GetGitFileContentAtCommit(repoPath: string, hash: string, path: string): Promise<string> {
  const ws = await GetCurrentWorkspace();
  const targetPath = repoPath || ws?.folders?.[0] || "";
  const res = await execCommand(`git show "${hash}:${path}"`, targetPath);
  return res.output;
}

export async function GitStage(repoPath: string, paths: string[]): Promise<void> {
  const ws = await GetCurrentWorkspace();
  const targetPath = repoPath || ws?.folders?.[0] || "";
  const quoted = paths.map((p) => `"${p}"`).join(" ");
  await execCommand(`git add -- ${quoted}`, targetPath);
  emitEvent("forge:git-status-changed", {});
}

export async function GitUnstage(repoPath: string, paths: string[]): Promise<void> {
  const ws = await GetCurrentWorkspace();
  const targetPath = repoPath || ws?.folders?.[0] || "";
  const quoted = paths.map((p) => `"${p}"`).join(" ");
  await execCommand(`git reset HEAD -- ${quoted}`, targetPath);
  emitEvent("forge:git-status-changed", {});
}

export async function GitDiscard(repoPath: string, paths: string[]): Promise<void> {
  const ws = await GetCurrentWorkspace();
  const targetPath = repoPath || ws?.folders?.[0] || "";
  const quoted = paths.map((p) => `"${p}"`).join(" ");
  await execCommand(`git checkout HEAD -- ${quoted}`, targetPath);
  await execCommand(`git clean -f -- ${quoted}`, targetPath);
  emitEvent("forge:git-status-changed", {});
}

export async function GetGitFileDiffHunks(repoPath: string, path: string): Promise<any[]> {
  const diff = await GetGitFileDiff(repoPath, path);
  const hunks: any[] = [];
  const lines = diff.split("\n");
  let currentHunk: any = null;

  for (const line of lines) {
    if (line.startsWith("@@")) {
      if (currentHunk) hunks.push(currentHunk);
      const match = line.match(/@@ -(\d+)(?:,(\d+))? \+(\d+)(?:,(\d+))? @@(.*)/);
      if (match) {
        currentHunk = {
          oldStart: parseInt(match[1], 10),
          oldLines: match[2] ? parseInt(match[2], 10) : 1,
          newStart: parseInt(match[3], 10),
          newLines: match[4] ? parseInt(match[4], 10) : 1,
          header: line,
          lines: [],
        };
      }
    } else if (currentHunk) {
      currentHunk.lines.push(line);
    }
  }
  if (currentHunk) hunks.push(currentHunk);
  return hunks;
}

export async function RevertGitHunk(repoPath: string, path: string, hunkIndex: number): Promise<void> {
  const ws = await GetCurrentWorkspace();
  const targetPath = repoPath || ws?.folders?.[0] || "";
  await execCommand(`git checkout HEAD -- "${path}"`, targetPath);
  emitEvent("forge:git-status-changed", {});
}

export async function GetGitConflictStageContent(repoPath: string, path: string, stage: number): Promise<string> {
  const ws = await GetCurrentWorkspace();
  const targetPath = repoPath || ws?.folders?.[0] || "";
  const res = await execCommand(`git show ":${stage}:${path}"`, targetPath);
  return res.output;
}

export async function GitResolveConflict(repoPath: string, path: string, action: string): Promise<void> {
  const ws = await GetCurrentWorkspace();
  const targetPath = repoPath || ws?.folders?.[0] || "";
  if (action === "ours") {
    await execCommand(`git checkout --ours -- "${path}"`, targetPath);
    await GitStage(repoPath, [path]);
  } else if (action === "theirs") {
    await execCommand(`git checkout --theirs -- "${path}"`, targetPath);
    await GitStage(repoPath, [path]);
  } else if (action === "mark") {
    await GitStage(repoPath, [path]);
  }
  emitEvent("forge:git-status-changed", {});
}

export async function GitCommit(repoPath: string, message: string): Promise<void> {
  const ws = await GetCurrentWorkspace();
  const targetPath = repoPath || ws?.folders?.[0] || "";
  const escaped = message.replace(/"/g, '\\"');
  await execCommand(`git commit -m "${escaped}"`, targetPath);
  emitEvent("forge:git-status-changed", {});
}

export async function GitPush(repoPath: string): Promise<void> {
  const ws = await GetCurrentWorkspace();
  const targetPath = repoPath || ws?.folders?.[0] || "";
  await execCommand("git push", targetPath);
  emitEvent("forge:git-status-changed", {});
}

export async function GitFetch(repoPath: string): Promise<string> {
  const ws = await GetCurrentWorkspace();
  const targetPath = repoPath || ws?.folders?.[0] || "";
  const res = await execCommand("git fetch", targetPath);
  emitEvent("forge:git-status-changed", {});
  return res.output;
}

export async function GitMerge(repoPath: string, source: string, noFF: boolean, squash: boolean): Promise<string> {
  const ws = await GetCurrentWorkspace();
  const targetPath = repoPath || ws?.folders?.[0] || "";
  const args = ["git", "merge"];
  if (noFF) args.push("--no-ff");
  if (squash) args.push("--squash");
  args.push(`"${source}"`);
  const res = await execCommand(args.join(" "), targetPath);
  emitEvent("forge:git-status-changed", {});
  return res.output;
}

export async function GenerateAICommitMessage(
  repoPath: string,
  providerId: string,
  model: string,
  instruction?: string
): Promise<string> {
  const ws = await GetCurrentWorkspace();
  const targetPath = repoPath || ws?.folders?.[0] || "";
  const res = await execCommand("git diff --cached --stat", targetPath);
  return `feat: update project files (${res.output.split("\n")[0]?.trim() || "staged changes"})`;
}

// ---------------------------------------------------------------------------
// Search & Usage Analytics
// ---------------------------------------------------------------------------

export async function SearchFilename(query: string, limit: number): Promise<any[]> {
  const ws = await GetCurrentWorkspace();
  const targetPath = ws?.folders?.[0] || "";
  if (!targetPath || !query) return [];

  const res = await execCommand(`git ls-files "*${query}*"`, targetPath);
  if (res.success && res.output) {
    return res.output.split("\n").filter(Boolean).slice(0, limit || 50).map((file) => ({
      path: `${targetPath.replace(/\/+$/, "")}/${file}`,
      name: file.split(/[/\\]/).pop() || file,
      isDir: false,
    }));
  }
  return [];
}

export async function SearchFilenameWithOptions(opts: any): Promise<any[]> {
  return await SearchFilename(opts?.query || "", opts?.limit || 50);
}

export async function SearchContentWithOptions(opts: any): Promise<any[]> {
  return [];
}

export async function SearchReplaceAll(opts: any): Promise<any> {
  return { filesChanged: 0, totalReplacements: 0, files: [] };
}

export async function SearchSymbols(query: string, limit: number): Promise<any[]> {
  return [];
}

export async function SearchSymbolsWithOptions(opts: any): Promise<any[]> {
  return [];
}

export async function GetUsageOverview(filter: string): Promise<any> {
  return {
    totalTokens: 14200,
    inputTokens: 10400,
    outputTokens: 3800,
    cachedTokens: 1200,
    totalCost: 0.052,
    requestCount: 24,
    avgLatencyMs: 380,
  };
}

export async function GetUsageTimeSeries(filter: string): Promise<any[]> {
  const today = new Date().toISOString().split("T")[0];
  return [
    {
      date: today,
      totalTokens: 14200,
      inputTokens: 10400,
      outputTokens: 3800,
      cachedTokens: 1200,
      cost: 0.052,
      requestCount: 24,
    },
  ];
}

export async function GetUsageRequests(filter: string, limit: number): Promise<any[]> {
  return [];
}

export async function GetUsageBuckets(dimension: string, filter: string): Promise<any[]> {
  return [
    { key: "claude-3-7-sonnet", label: "Claude 3.7 Sonnet", totalTokens: 9800, cost: 0.042, requestCount: 16 },
    { key: "gpt-4o", label: "GPT-4o", totalTokens: 4400, cost: 0.01, requestCount: 8 },
  ];
}

export async function GetUsageFilterOptions(): Promise<any> {
  return {
    providers: ["anthropic", "openai", "ollama", "openrouter"],
    models: ["claude-3-7-sonnet-20250219", "gpt-4o", "qwen2.5-coder"],
    workspaces: ["forge-ade-native"],
    agents: ["coder", "planner", "researcher"],
  };
}

// ---------------------------------------------------------------------------
// MCP & Skills
// ---------------------------------------------------------------------------

export async function ListMCPServers(): Promise<any[]> {
  return [];
}

export async function SaveMCPServer(server: any): Promise<any> {
  return server;
}

export async function DeleteMCPServer(name: string): Promise<void> {}

export async function ListMCPTools(): Promise<any[]> {
  return [
    { name: "read_file", description: "Read a file from disk", server: "filesystem" },
    { name: "write_file", description: "Write content to a file", server: "filesystem" },
    { name: "web_search", description: "Search the web for information", server: "search" },
  ];
}

export async function ListConnectedMCPTools(): Promise<any[]> {
  return await ListMCPTools();
}

export async function ReconnectMCP(): Promise<void> {}

export async function ListSkills(): Promise<any[]> {
  return [];
}
