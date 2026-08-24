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
      let res = await zero.invoke("command.exec", { command, cwd });
      if (typeof res === "string") {
        try {
          res = JSON.parse(res);
        } catch {}
      }
      const output = typeof res?.output === "string" ? res.output : (typeof res === "string" ? res : "");
      const exitCode = res?.exitCode ?? (res?.exit_code ?? (res?.success === false ? 1 : 0));
      const success = Boolean(res?.success ?? (exitCode === 0));
      return { output, exitCode, success };
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

// Provider profiles live in ~/.forge-ade/config.json (server-owned).
// The frontend is a thin view: read/write only through the bridge — never
// touch config files or localStorage directly, that caused config tug-of-war.

export async function GetProviderProfiles(): Promise<any[]> {
  const profiles = await invokeBackend<any[]>("ListProviderProfiles");
  const list = Array.isArray(profiles) ? profiles : [];
  // Dual key shapes kept for settings-modal compatibility. Model entries may
  // arrive as metadata objects ({id, name, ...}) — the UI renders plain ids.
  const toIdList = (value: unknown): string[] =>
    Array.isArray(value)
      ? value
          .map((m) => {
            if (typeof m === "string") return m;
            if (m && typeof m === "object" && typeof (m as { id?: unknown }).id === "string") {
              return (m as { id: string }).id;
            }
            return "";
          })
          .filter((s): s is string => s.length > 0)
      : [];
  return list.map((p) => {
    const catalogIds = toIdList(p.models);
    const selected = toIdList(p.selected_models);
    const allIds = Array.from(new Set([...catalogIds, ...selected]));
    return {
      ...p,
      api_key: p.apiKey ?? p.api_key ?? "",
      base_url: p.baseURL ?? p.base_url ?? "",
      models: allIds,
      selected_models: selected.length > 0 ? selected : allIds,
      available_models: allIds,
      enabled: p.enabled !== false,
    };
  });
}

export async function SaveProviderProfiles(profiles: any[]): Promise<void> {
  await invokeBackend("SaveProviderProfiles", { profiles });
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
  await invokeBackend("SetActiveModel", { providerId, model });
  emitEvent("agent:config:changed", {});
}

export async function SaveLLMProfile(
  providerId: string,
  apiKey: string,
  baseURL: string,
  model: string
): Promise<void> {
  await invokeBackend("SaveLLMProfile", { providerId, apiKey, baseURL, model });
  emitEvent("agent:config:changed", {});
}

/**
 * Active LLM configuration — resolved by the daemon from ~/.forge-ade/models.json.
 * Returned in the legacy shape (provider_id/model/...) that UI consumers expect.
 */
export async function GetLLMConfig(): Promise<any> {
  const cfg = await invokeBackend<any>("GetLLMConfig");
  const profiles = await GetProviderProfiles();
  const active = cfg?.activeProfile ?? null;
  const chosen = active ? profiles.find((p) => p.id === active.id) ?? { ...active } : null;
  return {
    provider_id: chosen?.id,
    api_key: chosen?.apiKey || chosen?.api_key || "",
    base_url: chosen?.baseURL || chosen?.base_url || "",
    model: active?.activeModel || chosen?.activeModel || "",
    activeProfile: chosen ? { ...chosen, activeModel: active.activeModel || chosen.activeModel } : null,
    profiles,
  };
}

export async function ListLLMProviders(): Promise<any[]> {
  const list = await invokeBackend<any[]>("ListLLMProviders");
  return Array.isArray(list) ? list : [];
}

// Agent Sessions — all state lives in the backend daemon. The frontend only
// invokes wire-API methods (contract §2) and renders WS events (§4); no local
// persistence and no client-side LLM traffic.

export type ContentBlockType = "text" | "thinking" | "tool_call" | "tool_result";

export interface ContentBlock {
  type: ContentBlockType;
  text?: string;
  tool_call_id?: string;
  name?: string;
  arguments?: string; // raw JSON string, streamed incrementally
  is_error?: boolean;
}

export interface AgentMessage {
  id: string;
  role: "user" | "assistant" | "system" | "tool";
  content: ContentBlock[];
  timestamp: string;
  state?: "running" | "done";
  usage?: {
    at: number;
    promptTokens: number;
    completionTokens: number;
    cachedTokens: number;
    durationMs: number;
  };
}

export type SessionState = "idle" | "running" | "awaiting_approval" | "awaiting_input";

export interface SessionMeta {
  id: string;
  name: string;
  role: string;
  projectFolder: string;
  dialect?: string;
  autoApprove?: boolean;
  auto_approve?: boolean;
  createdAt: string | number;
  updatedAt: string | number;
  messageCount: number;
  lastMessagePreview?: string;
  state: SessionState;
  contextWindow?: number;
  lastUsage?: TurnUsage;
}

/** Usage snapshot of the most recent LLM call (status line). */
export interface TurnUsage {
  at: number;
  promptTokens: number;
  completionTokens: number;
  cachedTokens: number;
  durationMs: number;
}

export interface FullAgentSession extends SessionMeta {
  messages: AgentMessage[];
  summary?: string;
  observations?: string[];
}

export async function ListAgentSessions(): Promise<SessionMeta[]> {
  const list = await invokeBackend<SessionMeta[]>("ListAgentSessions");
  return Array.isArray(list) ? list : [];
}

export async function ListAgentSessionsForFolder(folder: string): Promise<SessionMeta[]> {
  const list = await invokeBackend<SessionMeta[]>("ListAgentSessionsForFolder", { folder });
  return Array.isArray(list) ? list : [];
}

export async function GetAgentSession(id: string): Promise<FullAgentSession | null> {
  return await invokeBackend("GetAgentSession", { id });
}

export async function CreateAgentSession(name: string, role: string, projectFolder: string): Promise<FullAgentSession | null> {
  return await invokeBackend("CreateAgentSession", { name, role, projectFolder });
}

export async function CreateAgentSessionFromDefinition(defId: string, projectFolder: string): Promise<FullAgentSession | null> {
  return await invokeBackend("CreateAgentSessionFromDefinition", { defId, projectFolder });
}

export async function UpdateAgentSession(
  id: string,
  name: string,
  role: string,
  customPrompt: string,
  customRules: string
): Promise<SessionMeta | null> {
  return await invokeBackend("UpdateAgentSession", { id, name, role, customPrompt, customRules });
}

export async function RenameAgentSession(id: string, name: string): Promise<void> {
  await UpdateAgentSession(id, name, "", "", "");
}

export async function DeleteAgentSession(id: string): Promise<void> {
  await invokeBackend("DeleteAgentSession", { id });
}

export async function ClearAgentSession(id: string): Promise<void> {
  await invokeBackend("ClearAgentSession", { id });
}

export async function SetAgentDialect(id: string, dialect: string): Promise<void> {
  await invokeBackend("SetAgentDialect", { id, dialect });
}

export async function SetAgentAutoApprove(id: string, enabled: boolean): Promise<void> {
  await invokeBackend("SetAgentAutoApprove", { id, enabled });
}

export async function ToggleAgentTask(id: string, taskId: string, active: boolean): Promise<void> {
  await invokeBackend("ToggleAgentTask", { id, taskId, active });
}

export async function SendAgentMessage(id: string, message: string, files: string[] = []): Promise<void> {
  await invokeBackend("SendAgentMessage", { id, message, files });
}

export async function StopAgentTurn(id: string): Promise<void> {
  await invokeBackend("StopAgentTurn", { id });
}

export async function RespondAgentApproval(id: string, approve: boolean, autoAll: boolean): Promise<void> {
  await invokeBackend("RespondAgentApproval", { id, approve, autoAll });
}

export async function RespondAgentAsk(id: string, answers: Record<string, unknown>): Promise<void> {
  await invokeBackend("RespondAgentAsk", { id, answers });
}

export async function ListAgentDefinitions(): Promise<Record<string, unknown>[]> {
  const defs = await invokeBackend<Record<string, unknown>[]>("ListAgentDefinitions");
  return Array.isArray(defs) ? defs : [];
}

export async function SaveAgentDefinition(def: Record<string, unknown>): Promise<Record<string, unknown> | null> {
  return await invokeBackend("SaveAgentDefinition", { def });
}

export async function DeleteAgentDefinition(id: string): Promise<void> {
  await invokeBackend("DeleteAgentDefinition", { id });
}

export async function ApplyAgentDefinitionToSession(id: string, defId: string): Promise<void> {
  await invokeBackend("ApplyAgentDefinitionToSession", { id, defId });
}

// ---------------------------------------------------------------------------
// Slash Commands (daemon bridge)
// ---------------------------------------------------------------------------

export interface SlashCommand {
  name: string;
  description: string;
  kind: string;
}

export interface SlashCommandResult {
  handled: boolean;
  message?: string;
}

/** List known slash commands; empty query returns []. Server filters by prefix. */
export async function ListSlashCommands(query?: string): Promise<SlashCommand[]> {
  const res = await invokeBackend<SlashCommand[]>("ListSlashCommands", { query: query ?? "" });
  return Array.isArray(res) ? res : [];
}

/**
 * Execute a slash command. Returns {handled:true,message} when the daemon ran
 * it locally; {handled:false} means it was forwarded to the agent as a normal
 * message and turn events arrive over WS.
 */
export async function ExecuteSlashCommand(sessionId: string | undefined, text: string): Promise<SlashCommandResult> {
  const res = await invokeBackend<SlashCommandResult | null>("ExecuteSlashCommand", { sessionId, text });
  if (!res || typeof res !== "object") return { handled: false };
  return { handled: Boolean(res.handled), message: res.message };
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

export interface GitDecorations {
  statusMap: Map<string, string>;
  folderStatusMap: Map<string, string>;
  ignoredSet: Set<string>;
}

let cachedDecorations: { repoPath: string; ts: number; data: GitDecorations } | null = null;

export function isGitPathIgnored(
  targetPath: string,
  rootFolder: string,
  ignoredSet: Set<string>
): boolean {
  if (!targetPath || !rootFolder || !ignoredSet || ignoredSet.size === 0) return false;
  const rootNorm = rootFolder.replace(/\\/g, "/").replace(/\/+$/, "");
  const pathNorm = targetPath.replace(/\\/g, "/").replace(/\/+$/, "");
  let relPath = pathNorm;
  if (pathNorm.startsWith(rootNorm + "/")) {
    relPath = pathNorm.slice(rootNorm.length + 1);
  } else if (pathNorm === rootNorm) {
    return false;
  }

  if (ignoredSet.has(relPath)) return true;

  // Check all parent folder segments
  const parts = relPath.split("/");
  let acc = "";
  for (let i = 0; i < parts.length; i++) {
    acc = acc ? `${acc}/${parts[i]}` : parts[i];
    if (ignoredSet.has(acc)) return true;
  }

  // Common ignorable root names if present
  const lastPart = parts[parts.length - 1];
  if (lastPart === ".DS_Store" || lastPart === ".zig-cache" || lastPart === "zig-out") {
    return true;
  }

  return false;
}

export async function GetGitDecorations(repoPath?: string): Promise<GitDecorations> {
  const ws = await GetCurrentWorkspace();
  const targetPath = repoPath || ws?.folders?.[0] || "";
  if (!targetPath) {
    return { statusMap: new Map(), folderStatusMap: new Map(), ignoredSet: new Set() };
  }

  const now = Date.now();
  if (cachedDecorations && cachedDecorations.repoPath === targetPath && now - cachedDecorations.ts < 1200) {
    return cachedDecorations.data;
  }

  try {
    const statusRes = await execCommand("git status --porcelain=v1 -uall --ignored=traditional", targetPath);
    const statusMap = new Map<string, string>();
    const folderStatusMap = new Map<string, string>();
    const ignoredSet = new Set<string>();

    const rootNorm = targetPath.replace(/\\/g, "/").replace(/\/+$/, "");
    const lines = statusRes.output.split("\n").filter(Boolean);

    for (const line of lines) {
      if (line.length < 3) continue;
      const x = line[0];
      const y = line[1];
      let filePath = line.slice(3).trim();
      if (filePath.startsWith('"') && filePath.endsWith('"')) {
        filePath = filePath.slice(1, -1);
      }
      filePath = filePath.replace(/\\/g, "/").replace(/\/+$/, "");

      if (x === "!" && y === "!") {
        ignoredSet.add(filePath);
        continue;
      }

      let st = "M";
      if (x === "?" && y === "?") {
        st = "U";
      } else if (x === "U" || y === "U" || (x === "A" && y === "A") || (x === "D" && y === "D")) {
        st = "C";
      } else if (x === "A" || y === "A") {
        st = "A";
      } else if (x === "D" || y === "D") {
        st = "D";
      } else if (x === "R" || y === "R") {
        st = "R";
      } else if (x !== " " || y !== " ") {
        st = "M";
      }

      statusMap.set(filePath, st);
      statusMap.set(`${rootNorm}/${filePath}`, st);

      // Propagate status to all parent directories
      const parts = filePath.split("/");
      let dirAcc = "";
      for (let i = 0; i < parts.length - 1; i++) {
        dirAcc = dirAcc ? `${dirAcc}/${parts[i]}` : parts[i];
        const existing = folderStatusMap.get(dirAcc);
        // Higher priority statuses take precedence: Conflict > Modified > Added > Untracked > Deleted
        if (!existing || (existing === "U" && st === "M") || (existing === "D" && (st === "M" || st === "A"))) {
          folderStatusMap.set(dirAcc, st);
          folderStatusMap.set(`${rootNorm}/${dirAcc}`, st);
        }
      }
    }

    const result: GitDecorations = { statusMap, folderStatusMap, ignoredSet };
    cachedDecorations = { repoPath: targetPath, ts: now, data: result };
    return result;
  } catch {
    return { statusMap: new Map(), folderStatusMap: new Map(), ignoredSet: new Set() };
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
  providerId?: string,
  model?: string,
  instruction?: string
): Promise<string> {
  const msg = await invokeBackend<string>("GenerateAICommitMessage", {
    repoPath,
    providerId,
    model,
    instruction,
  });
  return typeof msg === "string" && msg ? msg : "chore: update project files";
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

export interface UsageRecord {
  id: string;
  timestamp: number;
  date: string;
  provider: string;
  model: string;
  workspace: string;
  agentRole: string;
  inputTokens: number;
  outputTokens: number;
  cachedTokens: number;
  cost: number;
  latencyMs: number;
  speedTps: number;
  status: "success" | "error";
  toolCalls?: number;
}

const STORAGE_KEY_USAGE = "forge_usage_records";

const MODEL_PRICES: Record<string, { inPrice: number; outPrice: number; cachePrice: number }> = {
  "claude-3-7-sonnet": { inPrice: 3.0, outPrice: 15.0, cachePrice: 0.3 },
  "claude-3-5-sonnet": { inPrice: 3.0, outPrice: 15.0, cachePrice: 0.3 },
  "claude-3-5-haiku": { inPrice: 0.8, outPrice: 4.0, cachePrice: 0.08 },
  "gpt-4o": { inPrice: 2.5, outPrice: 10.0, cachePrice: 1.25 },
  "gpt-4o-mini": { inPrice: 0.15, outPrice: 0.6, cachePrice: 0.075 },
  "o1": { inPrice: 15.0, outPrice: 60.0, cachePrice: 7.5 },
  "o3-mini": { inPrice: 1.1, outPrice: 4.4, cachePrice: 0.55 },
  "deepseek-chat": { inPrice: 0.14, outPrice: 0.28, cachePrice: 0.014 },
  "deepseek-v4-flash": { inPrice: 0.14, outPrice: 0.28, cachePrice: 0.014 },
  "deepseek-reasoner": { inPrice: 0.55, outPrice: 2.19, cachePrice: 0.14 },
  "deepseek-r1": { inPrice: 0.55, outPrice: 2.19, cachePrice: 0.14 },
  "qwen2.5-coder": { inPrice: 0.0, outPrice: 0.0, cachePrice: 0.0 },
  "llama3.3": { inPrice: 0.0, outPrice: 0.0, cachePrice: 0.0 },
};

function calculateCost(model: string, inTok: number, outTok: number, cacheTok: number): number {
  const norm = (model || "").toLowerCase();
  let pricing = { inPrice: 1.0, outPrice: 3.0, cachePrice: 0.2 };
  for (const [key, p] of Object.entries(MODEL_PRICES)) {
    if (norm.includes(key)) {
      pricing = p;
      break;
    }
  }
  return Number(((inTok * pricing.inPrice + outTok * pricing.outPrice + cacheTok * pricing.cachePrice) / 1_000_000).toFixed(6));
}

export async function GetAllUsageRecords(): Promise<UsageRecord[]> {
  // Single source of truth: the daemon's usage journal (all projects).
  const rows = await invokeBackend<any[]>("GetAllUsageRecords");
  const rawList = Array.isArray(rows) ? rows : [];
  return rawList.map((r, i) => {
    const speedTps =
      r.latencyMs > 0 ? Number(((r.outputTokens || 0) / (r.latencyMs / 1000)).toFixed(1)) : 0;
    return {
      id: `${r.sessionId}-${i}`,
      timestamp: r.ts,
      date: new Date(r.ts).toISOString().split("T")[0],
      provider: r.provider,
      model: r.model,
      workspace: r.workspace,
      agentRole: r.sessionId,
      inputTokens: r.inputTokens || 0,
      outputTokens: r.outputTokens || 0,
      cachedTokens: r.cachedTokens || 0,
      cost: 0,
      latencyMs: r.latencyMs || 0,
      speedTps,
      status: "success" as const,
      toolCalls: 0,
    };
  });
}

export async function RecordLLMUsage(data: {
  provider: string;
  model: string;
  workspace: string;
  agentRole: string;
  inputTokens: number;
  outputTokens: number;
  cachedTokens?: number;
  latencyMs: number;
  status?: "success" | "error";
  toolCalls?: number;
}): Promise<void> {
  const now = Date.now();
  const dateStr = new Date(now).toISOString().split("T")[0];
  const inTok = data.inputTokens || 0;
  const outTok = data.outputTokens || 0;
  const cacheTok = data.cachedTokens || 0;
  const latency = Math.max(1, data.latencyMs || 1);
  const speed = latency > 0 ? Number(((outTok / (latency / 1000))).toFixed(1)) : 0;
  const cost = calculateCost(data.model, inTok, outTok, cacheTok);

  const record: UsageRecord = {
    id: `req-${now}-${Math.random().toString(36).slice(2, 6)}`,
    timestamp: now,
    date: dateStr,
    provider: data.provider,
    model: data.model,
    workspace: data.workspace,
    agentRole: data.agentRole,
    inputTokens: inTok,
    outputTokens: outTok,
    cachedTokens: cacheTok,
    cost,
    latencyMs: latency,
    speedTps: speed,
    status: data.status || "success",
    toolCalls: data.toolCalls || 0,
  };

  const records = await GetAllUsageRecords();
  records.unshift(record);
  const capped = records.slice(0, 1000);
  localStorage.setItem(STORAGE_KEY_USAGE, JSON.stringify(capped));

  try {
    const home = await GetHomeDir();
    const diskPath = `${home}/.forge-ade/usage.json`;
    await WriteFile(diskPath, JSON.stringify(capped, null, 2)).catch(() => {});
  } catch {}

  emitEvent("usage:updated", record);
}

function filterRecordsByRange(records: UsageRecord[], filter: string): UsageRecord[] {
  if (!filter || filter === "all") return records;
  const now = Date.now();
  const todayStr = new Date(now).toISOString().split("T")[0];

  if (filter === "today") {
    return records.filter((r) => r.date === todayStr);
  }
  if (filter === "7d") {
    const cutoff = now - 7 * 86400 * 1000;
    return records.filter((r) => r.timestamp >= cutoff);
  }
  if (filter === "30d" || filter === "month") {
    const cutoff = now - 30 * 86400 * 1000;
    return records.filter((r) => r.timestamp >= cutoff);
  }
  if (filter === "year") {
    const cutoff = now - 365 * 86400 * 1000;
    return records.filter((r) => r.timestamp >= cutoff);
  }
  return records;
}

export async function GetUsageOverview(filter: string): Promise<any> {
  const records = filterRecordsByRange(await GetAllUsageRecords(), filter);
  if (records.length === 0) {
    return {
      requests: 0,
      input_tokens: 0,
      output_tokens: 0,
      cached_tokens: 0,
      cache_hit_rate: 0,
      latency_p95_ms: 0,
      avg_tool_calls: 0,
      cost_usd: 0,
      avg_speed_tps: 0,
    };
  }

  let inTok = 0;
  let outTok = 0;
  let cacheTok = 0;
  let totalCost = 0;
  let totalTools = 0;
  let totalSpeed = 0;
  const latencies: number[] = [];

  for (const r of records) {
    inTok += r.inputTokens || 0;
    outTok += r.outputTokens || 0;
    cacheTok += r.cachedTokens || 0;
    totalCost += r.cost || 0;
    totalTools += r.toolCalls || 0;
    totalSpeed += r.speedTps || 0;
    latencies.push(r.latencyMs || 0);
  }

  latencies.sort((a, b) => a - b);
  const p95Idx = Math.floor(latencies.length * 0.95);
  const latencyP95 = latencies[p95Idx] || latencies[latencies.length - 1] || 0;
  const cacheHitRate = inTok + cacheTok > 0 ? Number(((cacheTok / (inTok + cacheTok)) * 100).toFixed(1)) : 0;

  return {
    requests: records.length,
    input_tokens: inTok,
    output_tokens: outTok,
    cached_tokens: cacheTok,
    cache_hit_rate: cacheHitRate,
    latency_p95_ms: latencyP95,
    avg_tool_calls: Number((totalTools / records.length).toFixed(1)),
    cost_usd: Number(totalCost.toFixed(4)),
    avg_speed_tps: Number((totalSpeed / records.length).toFixed(1)),
  };
}

export async function GetUsageTimeSeries(filter: string): Promise<any[]> {
  const records = filterRecordsByRange(await GetAllUsageRecords(), filter);
  const dayMap = new Map<string, { date: string; input_tokens: number; output_tokens: number; cached_tokens: number; cost_usd: number; requests: number }>();

  for (const r of records) {
    let point = dayMap.get(r.date);
    if (!point) {
      point = { date: r.date, input_tokens: 0, output_tokens: 0, cached_tokens: 0, cost_usd: 0, requests: 0 };
      dayMap.set(r.date, point);
    }
    point.input_tokens += r.inputTokens || 0;
    point.output_tokens += r.outputTokens || 0;
    point.cached_tokens += r.cachedTokens || 0;
    point.cost_usd += r.cost || 0;
    point.requests += 1;
  }

  const points = Array.from(dayMap.values());
  points.sort((a, b) => a.date.localeCompare(b.date));
  if (points.length === 0) {
    const today = new Date().toISOString().split("T")[0];
    return [{ date: today, input_tokens: 0, output_tokens: 0, cached_tokens: 0, cost_usd: 0, requests: 0 }];
  }
  return points;
}

export async function GetUsageRequests(filter: string, limit: number = 100): Promise<any[]> {
  const records = filterRecordsByRange(await GetAllUsageRecords(), filter);
  return records.slice(0, limit).map((r) => ({
    id: r.id,
    timestamp: r.timestamp,
    workspace: r.workspace,
    provider: r.provider,
    model: r.model,
    input_tokens: r.inputTokens,
    output_tokens: r.outputTokens,
    cached_tokens: r.cachedTokens,
    latency_ms: r.latencyMs,
    tool_calls: r.toolCalls || 0,
    cost_usd: r.cost,
    success: r.status === "success",
    speed_tps: r.speedTps,
  }));
}

export async function GetUsageBuckets(dimension: string, filter: string): Promise<any[]> {
  const records = filterRecordsByRange(await GetAllUsageRecords(), filter);
  const map = new Map<
    string,
    {
      key: string;
      label: string;
      total_tokens: number;
      input_tokens: number;
      output_tokens: number;
      cached_tokens: number;
      cost_usd: number;
      requests: number;
    }
  >();

  for (const r of records) {
    let key = "";
    if (dimension === "model") key = r.model || "unknown";
    else if (dimension === "provider") key = r.provider || "unknown";
    else if (dimension === "workspace") key = r.workspace || "unknown";
    else if (dimension === "agent") key = r.agentRole || "coding";
    else key = r.model || "unknown";

    let b = map.get(key);
    if (!b) {
      b = { key, label: key, total_tokens: 0, input_tokens: 0, output_tokens: 0, cached_tokens: 0, cost_usd: 0, requests: 0 };
      map.set(key, b);
    }
    const inTok = r.inputTokens || 0;
    const outTok = r.outputTokens || 0;
    const cacheTok = r.cachedTokens || 0;
    b.input_tokens += inTok;
    b.output_tokens += outTok;
    b.cached_tokens += cacheTok;
    b.total_tokens += inTok + outTok + cacheTok;
    b.cost_usd += r.cost || 0;
    b.requests += 1;
  }

  const buckets = Array.from(map.values()).map((b) => ({
    ...b,
    cache_hit_rate:
      b.input_tokens + b.cached_tokens > 0
        ? Number(((b.cached_tokens / (b.input_tokens + b.cached_tokens)) * 100).toFixed(1))
        : 0,
  }));
  buckets.sort((a, b) => b.total_tokens - a.total_tokens);
  return buckets;
}

export async function GetUsageFilterOptions(): Promise<any> {
  const records = await GetAllUsageRecords();
  const providers = new Set<string>(["openai", "openrouter", "anthropic", "ollama", "deepseek", "groq"]);
  const models = new Set<string>(["gpt-4o", "claude-3-7-sonnet-20250219", "deepseek-v4-flash", "qwen2.5-coder"]);
  const workspaces = new Set<string>(["forge-ade-native"]);
  const agents = new Set<string>(["coding", "planning", "research"]);

  for (const r of records) {
    if (r.provider) providers.add(r.provider);
    if (r.model) models.add(r.model);
    if (r.workspace) workspaces.add(r.workspace);
    if (r.agentRole) agents.add(r.agentRole);
  }

  return {
    providers: Array.from(providers),
    models: Array.from(models),
    workspaces: Array.from(workspaces),
    agents: Array.from(agents),
  };
}

// ---------------------------------------------------------------------------
// MCP & Skills
// ---------------------------------------------------------------------------

export interface MCPToolInfo {
  name: string;
  description: string;
  server: string;
  parameters?: Record<string, unknown>;
}

export interface MCPServerInfo {
  name: string;
  command?: string;
  args?: string[];
  env?: Record<string, string>;
  url?: string;
  headers?: Record<string, string>;
  enabled: boolean;
  source: string;
  connected: boolean;
  error?: string;
}

export async function ListMCPServers(): Promise<MCPServerInfo[]> {
  const list = await invokeBackend<MCPServerInfo[]>("ListMCPServers");
  return Array.isArray(list) ? list : [];
}

export async function SaveMCPServer(server: Partial<MCPServerInfo>): Promise<any> {
  const res = await invokeBackend("SaveMCPServer", { server });
  emitEvent("agent:config:changed", {});
  return res;
}

export async function DeleteMCPServer(name: string): Promise<void> {
  await invokeBackend("DeleteMCPServer", { name });
  emitEvent("agent:config:changed", {});
}

export async function ListConnectedMCPTools(): Promise<MCPToolInfo[]> {
  const list = await invokeBackend<MCPToolInfo[]>("ListConnectedMCPTools");
  return Array.isArray(list) ? list : [];
}

export async function ListMCPTools(): Promise<MCPToolInfo[]> {
  return await ListConnectedMCPTools();
}

export async function ReconnectMCP(): Promise<{ connected: string[]; failed: string[] }> {
  const res = await invokeBackend<{ connected: string[]; failed: string[] }>("ReconnectMCP");
  emitEvent("agent:config:changed", {});
  return res || { connected: [], failed: [] };
}

export interface SkillInfo {
  name: string;
  description: string;
  path: string;
  source: string;
  enabled: boolean;
}

export async function ListSkills(): Promise<SkillInfo[]> {
  const list = await invokeBackend<SkillInfo[]>("ListSkills");
  return Array.isArray(list) ? list : [];
}

export async function ListAllSkills(): Promise<SkillInfo[]> {
  const list = await invokeBackend<SkillInfo[]>("ListAllSkills");
  return Array.isArray(list) ? list : [];
}

export async function SetSkillEnabled(name: string, enabled: boolean): Promise<boolean> {
  const res = await invokeBackend<boolean>("SetSkillEnabled", { name, enabled });
  emitEvent("agent:config:changed", {});
  return Boolean(res);
}

export async function SetAllSkillsEnabled(enabled: boolean): Promise<boolean> {
  const res = await invokeBackend<boolean>("SetAllSkillsEnabled", { enabled });
  emitEvent("agent:config:changed", {});
  return Boolean(res);
}
