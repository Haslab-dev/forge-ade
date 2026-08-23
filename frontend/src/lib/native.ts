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

const STORAGE_KEY_AGENTS = "forge_agent_sessions";
const STORAGE_KEY_DEFS = "forge_agent_definitions";
const STORAGE_KEY_PROFILES = "forge_llm_profiles";

export async function GetProviderProfiles(): Promise<any[]> {
  const home = await GetHomeDir();
  const diskPath = `${home}/.forge-ade/providers_config.json`;
  const activeConfigPath = `${home}/.forge-ade/llm_config.json`;

  let activeConfig: { provider_id?: string; model?: string } | null = null;
  try {
    const rawActive = await ReadFile(activeConfigPath);
    if (rawActive) activeConfig = JSON.parse(rawActive);
  } catch {}

  if (!activeConfig) {
    try {
      const raw = localStorage.getItem("forge_active_llm_config");
      if (raw) activeConfig = JSON.parse(raw);
    } catch {}
  }

  // 1. Try reading disk config from ~/.forge-ade/providers_config.json
  try {
    const content = await ReadFile(diskPath);
    if (content) {
      const parsed = JSON.parse(content);
      if (Array.isArray(parsed) && parsed.length > 0) {
        const normalized = parsed.map((p: any) => {
          const isCurrentActive = activeConfig && (activeConfig.provider_id === p.id || activeConfig.provider_id === p.name);
          const activeModel = isCurrentActive && activeConfig?.model
            ? activeConfig.model
            : p.activeModel || (p.selected_models && p.selected_models[0]) || (p.available_models && p.available_models[0]) || "";

          return {
            id: p.id || p.name,
            name: p.name || p.id,
            provider: p.provider || p.id,
            apiKey: p.api_key || p.apiKey || p.ApiKey || "",
            api_key: p.api_key || p.apiKey || p.ApiKey || "",
            baseURL: p.base_url || p.baseURL || p.BaseURL || "",
            base_url: p.base_url || p.baseURL || p.BaseURL || "",
            activeModel,
            selected_models: p.selected_models || p.available_models || [],
            available_models: p.available_models || p.selected_models || [],
            enabled: p.enabled !== false,
          };
        });
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
    if (raw) {
      const parsed = JSON.parse(raw);
      if (Array.isArray(parsed) && parsed.length > 0) return parsed;
    }
  } catch {}

  // 3. Defaults
  const defaults = [
    {
      id: "myairouter",
      name: "MyAiRouter-local",
      provider: "openai",
      apiKey: "sk-558cd478fc5a8631532a21fd5b105a3112b668446b429fd4",
      baseURL: "http://localhost:20128/v1",
      activeModel: "db/deepseek-v4-flash",
      selected_models: ["db/deepseek-v4-flash", "kc/kilo-auto/free", "openrouter/openrouter/free"],
      available_models: ["db/deepseek-v4-flash", "kc/kilo-auto/free", "openrouter/openrouter/free"],
      enabled: true,
    },
    {
      id: "openrouter",
      name: "OpenRouter",
      provider: "openrouter",
      apiKey: "",
      baseURL: "https://openrouter.ai/api/v1",
      activeModel: "anthropic/claude-3.7-sonnet",
      selected_models: ["anthropic/claude-3.7-sonnet", "openai/gpt-4o", "deepseek/deepseek-r1"],
      available_models: ["anthropic/claude-3.7-sonnet", "openai/gpt-4o", "deepseek/deepseek-r1"],
      enabled: true,
    },
    {
      id: "openai",
      name: "OpenAI",
      provider: "openai",
      apiKey: "",
      baseURL: "https://api.openai.com/v1",
      activeModel: "gpt-4o",
      selected_models: ["gpt-4o", "gpt-4o-mini", "o3-mini"],
      available_models: ["gpt-4o", "gpt-4o-mini", "o3-mini"],
      enabled: true,
    },
    {
      id: "anthropic",
      name: "Anthropic",
      provider: "anthropic",
      apiKey: "",
      baseURL: "https://api.anthropic.com/v1",
      activeModel: "claude-3-7-sonnet-20250219",
      selected_models: ["claude-3-7-sonnet-20250219", "claude-3-5-sonnet-20241022", "claude-3-5-haiku-20241022"],
      available_models: ["claude-3-7-sonnet-20250219", "claude-3-5-sonnet-20241022", "claude-3-5-haiku-20241022"],
      enabled: true,
    },
    {
      id: "ollama",
      name: "Ollama",
      provider: "ollama",
      apiKey: "",
      baseURL: "http://127.0.0.1:11434/v1",
      activeModel: "qwen2.5-coder",
      selected_models: ["qwen2.5-coder", "llama3.3", "deepseek-r1"],
      available_models: ["qwen2.5-coder", "llama3.3", "deepseek-r1"],
      enabled: true,
    },
  ];

  return defaults;
}

export async function SaveProviderProfiles(profiles: any[]): Promise<void> {
  localStorage.setItem(STORAGE_KEY_PROFILES, JSON.stringify(profiles));
  try {
    const home = await GetHomeDir();
    const dirPath = `${home}/.forge-ade`;
    const diskPath = `${dirPath}/providers_config.json`;
    await CreateFolder(dirPath).catch(() => {});
    await WriteFile(diskPath, JSON.stringify(profiles, null, 2));
  } catch (err) {
    console.warn("Save providers_config.json to disk error:", err);
  }
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
  const p = profiles.find((x) => x.id === providerId || x.name === providerId);
  if (p) {
    p.activeModel = model;
    if (!p.selected_models.includes(model)) {
      p.selected_models.unshift(model);
    }
    await SaveProviderProfiles(profiles);
  }
  try {
    const home = await GetHomeDir();
    const dirPath = `${home}/.forge-ade`;
    const activeConfigPath = `${dirPath}/llm_config.json`;
    const payload = { provider_id: providerId, model };
    localStorage.setItem("forge_active_llm_config", JSON.stringify(payload));
    await CreateFolder(dirPath).catch(() => {});
    await WriteFile(activeConfigPath, JSON.stringify(payload, null, 2));
  } catch {}
  emitEvent("agent:config:changed", {});
}

export async function SaveLLMProfile(
  providerId: string,
  apiKey: string,
  baseURL: string,
  model: string
): Promise<void> {
  const profiles = await GetProviderProfiles();
  let p = profiles.find((x) => x.id === providerId || x.name === providerId);
  if (p) {
    p.apiKey = apiKey;
    p.api_key = apiKey;
    p.baseURL = baseURL;
    p.base_url = baseURL;
    if (model) p.activeModel = model;
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
      selected_models: model ? [model] : [],
      available_models: model ? [model] : [],
      enabled: true,
    });
  }
  await SaveProviderProfiles(profiles);
  if (model) {
    await SetActiveModel(providerId, model);
  }
}

export async function GetLLMConfig(): Promise<any> {
  const profiles = await GetProviderProfiles();
  let activeConfig: { provider_id?: string; model?: string } | null = null;
  try {
    const home = await GetHomeDir();
    const diskPath = `${home}/.forge-ade/llm_config.json`;
    const content = await ReadFile(diskPath);
    if (content) activeConfig = JSON.parse(content);
  } catch {}

  if (!activeConfig) {
    try {
      const raw = localStorage.getItem("forge_active_llm_config");
      if (raw) activeConfig = JSON.parse(raw);
    } catch {}
  }

  const active = (activeConfig?.provider_id && profiles.find((p) => p.id === activeConfig?.provider_id || p.name === activeConfig?.provider_id))
    || profiles.find((p) => p.enabled && (p.apiKey || p.api_key || p.baseURL || p.base_url))
    || profiles.find((p) => p.enabled)
    || profiles[0];

  const model = activeConfig?.model || active?.activeModel || (active?.selected_models && active.selected_models[0]) || "";

  return {
    provider_id: active?.id,
    api_key: active?.apiKey || active?.api_key || "",
    base_url: active?.baseURL || active?.base_url || "https://api.openai.com/v1",
    model,
    activeProfile: active ? { ...active, activeModel: model } : null,
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

export const AGENT_TOOLS = [
  {
    type: "function",
    function: {
      name: "read_file",
      description: "Read the contents of a file in the workspace",
      parameters: {
        type: "object",
        properties: {
          path: { type: "string", description: "Relative or absolute path to the file" },
          offset: { type: "number", description: "Optional 1-based start line" },
          limit: { type: "number", description: "Optional max lines to read" },
        },
        required: ["path"],
      },
    },
  },
  {
    type: "function",
    function: {
      name: "write_file",
      description: "Create or overwrite a file with new content",
      parameters: {
        type: "object",
        properties: {
          path: { type: "string", description: "Path to the file to create or overwrite" },
          content: { type: "string", description: "Full content of the file" },
        },
        required: ["path", "content"],
      },
    },
  },
  {
    type: "function",
    function: {
      name: "edit_file",
      description: "Edit an existing file by replacing old text with new text",
      parameters: {
        type: "object",
        properties: {
          path: { type: "string", description: "Path to the file to edit" },
          old_text: { type: "string", description: "Exact old text to find and replace" },
          new_text: { type: "string", description: "New replacement text" },
        },
        required: ["path", "old_text", "new_text"],
      },
    },
  },
  {
    type: "function",
    function: {
      name: "bash",
      description: "Execute a shell command or git command in the workspace folder",
      parameters: {
        type: "object",
        properties: {
          command: { type: "string", description: "Shell command line to execute" },
        },
        required: ["command"],
      },
    },
  },
  {
    type: "function",
    function: {
      name: "search_files",
      description: "Find files by filename or pattern",
      parameters: {
        type: "object",
        properties: {
          query: { type: "string", description: "Filename search query or glob" },
        },
        required: ["query"],
      },
    },
  },
  {
    type: "function",
    function: {
      name: "grep_code",
      description: "Search for text or regex pattern across workspace files",
      parameters: {
        type: "object",
        properties: {
          pattern: { type: "string", description: "Search pattern or regex" },
          path: { type: "string", description: "Optional path or subdirectory to search in" },
        },
        required: ["pattern"],
      },
    },
  },
  {
    type: "function",
    function: {
      name: "list_dir",
      description: "List files and directories in a folder",
      parameters: {
        type: "object",
        properties: {
          path: { type: "string", description: "Folder path to list" },
        },
        required: ["path"],
      },
    },
  },
];

const activeTurnAborts = new Map<string, AbortController>();

export function StopAgentTurn(id: string): void {
  const controller = activeTurnAborts.get(id);
  if (controller) {
    controller.abort();
    activeTurnAborts.delete(id);
  }
  emitEvent("agent:turn_end", { id });
}

export async function executeAgentTool(
  name: string,
  args: Record<string, any>,
  workspaceFolder: string
): Promise<{ result: string; is_error?: boolean }> {
  const root = workspaceFolder || process.cwd?.() || "";
  const resolvePath = (p: string) => {
    if (!p) return root;
    if (p.startsWith("/") || p.startsWith("~")) return p;
    return `${root.replace(/\/+$/, "")}/${p.replace(/^\/+/, "")}`;
  };

  try {
    switch (name) {
      case "read":
      case "read_file": {
        const fPath = resolvePath(args.path);
        const content = await ReadFile(fPath);
        if (args.offset || args.limit) {
          const lines = content.split("\n");
          const start = Math.max(0, (args.offset || 1) - 1);
          const end = args.limit ? start + args.limit : lines.length;
          const sliced = lines.slice(start, end).join("\n");
          return { result: JSON.stringify({ path: args.path, lines: `${start + 1}-${Math.min(lines.length, end)}`, content: sliced }) };
        }
        return { result: JSON.stringify({ path: args.path, content }) };
      }

      case "write":
      case "write_file": {
        const fPath = resolvePath(args.path);
        await WriteFile(fPath, args.content || "");
        emitEvent("fs:changed", { type: "modify", path: fPath });
        return { result: JSON.stringify({ path: args.path, status: "written", size: (args.content || "").length }) };
      }

      case "edit":
      case "edit_file": {
        const fPath = resolvePath(args.path);
        const content = await ReadFile(fPath);
        if (!args.old_text) {
          return { result: "Error: old_text parameter is required", is_error: true };
        }
        if (!content.includes(args.old_text)) {
          return { result: `Error: old_text was not found in ${args.path}`, is_error: true };
        }
        const updated = content.replace(args.old_text, args.new_text || "");
        await WriteFile(fPath, updated);
        emitEvent("fs:changed", { type: "modify", path: fPath });
        return { result: JSON.stringify({ path: args.path, status: "edited", replacements: 1 }) };
      }

      case "bash":
      case "exec":
      case "run_command": {
        const cmd = args.command || "";
        if (!cmd) return { result: "Error: command parameter is required", is_error: true };
        const res = await execCommand(cmd, root);
        return {
          result: JSON.stringify({
            command: cmd,
            stdout: res.output || "(no output)",
            exit_code: res.exitCode,
          }),
          is_error: res.exitCode !== 0,
        };
      }

      case "search_files":
      case "find":
      case "glob": {
        const query = args.query || args.pattern || "";
        const matches = await SearchFilename(query, 40);
        return { result: JSON.stringify({ count: matches.length, entries: matches }) };
      }

      case "grep_code":
      case "search":
      case "grep": {
        const pattern = args.pattern || args.query || "";
        const escaped = pattern.replace(/"/g, '\\"');
        const subPath = args.path ? ` -- "${args.path}"` : "";
        const res = await execCommand(`git grep -n -I "${escaped}"${subPath}`, root);
        return { result: JSON.stringify({ pattern, stdout: res.output || "No matches found" }) };
      }

      case "list_dir":
      case "ls": {
        const dPath = resolvePath(args.path || ".");
        const res = await ListDirectory(dPath);
        return { result: JSON.stringify({ path: args.path, entries: JSON.parse(res) }) };
      }

      default:
        return { result: `Unknown tool: ${name}`, is_error: true };
    }
  } catch (err: any) {
    return { result: `Tool execution failed: ${err.message || err}`, is_error: true };
  }
}

export async function SendAgentMessage(id: string, message: string, files: string[] = []): Promise<void> {
  const list = await ListAgentSessions();
  const session = list.find((s) => s.id === id);
  if (!session) return;

  const abortController = new AbortController();
  activeTurnAborts.set(id, abortController);

  const userMsg = {
    id: `msg-${Date.now()}`,
    role: "user" as const,
    content: [{ type: "text" as const, text: message }],
    timestamp: new Date().toISOString(),
  };
  session.messages.push(userMsg);
  session.state = "thinking";
  session.updatedAt = new Date().toISOString();
  localStorage.setItem(STORAGE_KEY_AGENTS, JSON.stringify(list));
  emitEvent("agent:updated", { id });
  emitEvent("agent:turn_start", { id });

  const llmConfig = await GetLLMConfig();
  const profiles = await GetProviderProfiles();
  const activeProfile = (llmConfig?.provider_id && profiles.find((p) => p.id === llmConfig?.provider_id || p.name === llmConfig?.provider_id))
    || profiles.find((p) => p.enabled && (p.apiKey || p.api_key || p.baseURL || p.base_url))
    || profiles.find((p) => p.enabled)
    || profiles[0];

  const apiKey = activeProfile?.apiKey || activeProfile?.api_key || llmConfig?.api_key || "";
  let baseURL = activeProfile?.baseURL || activeProfile?.base_url || llmConfig?.base_url || "https://api.openai.com/v1";
  const model = activeProfile?.activeModel || llmConfig?.model || (activeProfile?.selected_models && activeProfile.selected_models[0]) || "gpt-4o";

  baseURL = baseURL.trim().replace(/\/+$/, "");

  // Auto-generate session title if first turn or placeholder name
  const userMessages = session.messages.filter((m: any) => m.role === "user");
  const isDefaultName = !session.name || session.name === "New Session" || session.name === "AI Assistant" || session.name === "Agent Session";
  if (userMessages.length === 1 || isDefaultName) {
    generateSessionTitle(message, apiKey, baseURL, model).then((title) => {
      if (title && title.length > 1) {
        session.name = title;
        ListAgentSessions().then((all) => {
          const idx = all.findIndex((s: any) => s.id === id);
          if (idx >= 0) all[idx].name = title;
          localStorage.setItem(STORAGE_KEY_AGENTS, JSON.stringify(all));
          emitEvent("agent:updated", { id });
        });
      }
    }).catch(() => {});
  }

  const systemPrompt =
    session.customPrompt ||
    `You are Forge AI, an elite software engineering assistant running inside ForgeADE native development workspace.\n` +
    `Workspace Folder: ${session.projectFolder}\n\n` +
    `You have full autonomous tool access to inspect files, edit code, run terminal commands, and search the repository.\n` +
    `When asked to perform tasks, directly call the appropriate tools (read_file, write_file, edit_file, bash, grep_code, search_files).\n` +
    `Always verify changes after editing and keep explanations crisp, direct, and evidence-first.`;

  let iteration = 0;
  const maxIterations = 12;
  const startTime = Date.now();
  let totalOutputTokens = 0;

  try {
    while (iteration < maxIterations && !abortController.signal.aborted) {
      iteration++;

      const assistantMsgId = `msg-${Date.now()}-${iteration}`;
      const assistantMsg = {
        id: assistantMsgId,
        role: "assistant" as const,
        content: [] as any[],
        timestamp: new Date().toISOString(),
      };
      session.messages.push(assistantMsg);

      // Prepare conversation history
      const apiMessages: any[] = [{ role: "system", content: systemPrompt }];
      for (const m of session.messages.slice(0, -1)) {
        if (m.role === "user") {
          const text = Array.isArray(m.content) ? m.content.map((c: any) => c.text || "").join("\n") : String(m.content || "");
          apiMessages.push({ role: "user", content: text });
        } else if (m.role === "assistant") {
          const textBlocks = Array.isArray(m.content) ? m.content.filter((c: any) => c.type === "text").map((c: any) => c.text).join("\n") : "";
          const toolCalls = Array.isArray(m.content)
            ? m.content
                .filter((c: any) => c.type === "tool_call")
                .map((c: any) => ({
                  id: c.tool_call_id,
                  type: "function",
                  function: { name: c.name, arguments: JSON.stringify(c.arguments || {}) },
                }))
            : [];

          const msgPayload: any = { role: "assistant" };
          if (textBlocks) msgPayload.content = textBlocks;
          if (toolCalls.length > 0) msgPayload.tool_calls = toolCalls;
          if (msgPayload.content || msgPayload.tool_calls) apiMessages.push(msgPayload);
        } else if (m.role === "tool") {
          for (const b of (Array.isArray(m.content) ? m.content : [])) {
            if (b.type === "tool_result") {
              apiMessages.push({
                role: "tool",
                tool_call_id: b.tool_call_id,
                content: b.text || "",
              });
            }
          }
        }
      }

      let endpoint = baseURL;
      if (!endpoint.endsWith("/v1") && !endpoint.includes("/v1/")) {
        endpoint = `${endpoint}/v1`;
      }
      endpoint = `${endpoint}/chat/completions`;

      const headers: Record<string, string> = { "Content-Type": "application/json" };
      if (apiKey) headers["Authorization"] = `Bearer ${apiKey}`;

      session.state = "thinking";
      emitEvent("agent:updated", { id });

      const res = await fetch(endpoint, {
        method: "POST",
        headers,
        signal: abortController.signal,
        body: JSON.stringify({
          model,
          messages: apiMessages,
          tools: AGENT_TOOLS,
          stream: true,
        }),
      });

      if (!res.ok || !res.body) {
        const errText = await res.text().catch(() => res.statusText);
        assistantMsg.content.push({
          type: "text",
          text: `**API Error (${res.status}):** ${errText || "Request failed"}`,
        });
        break;
      }

      const reader = res.body.getReader();
      let thinkingText = "";
      let answerText = "";
      let buffer = "";
      let inThinkTag = false;
      const pendingToolCalls: Map<number, { id: string; name: string; argsText: string }> = new Map();

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

            // 1. Thinking / Reasoning delta
            const reasoning = delta.reasoning_content || delta.reasoning || "";
            if (reasoning) {
              thinkingText += reasoning;
              let thinkBlock = assistantMsg.content.find((c: any) => c.type === "thinking");
              if (!thinkBlock) {
                thinkBlock = { type: "thinking", text: "", startTs: Date.now() };
                assistantMsg.content.unshift(thinkBlock);
              }
              thinkBlock.text = thinkingText;
            }

            // 2. Prose content delta
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
                let thinkBlock = assistantMsg.content.find((c: any) => c.type === "thinking");
                if (!thinkBlock) {
                  thinkBlock = { type: "thinking", text: "", startTs: Date.now() };
                  assistantMsg.content.unshift(thinkBlock);
                }
                thinkBlock.text = thinkingText;
              } else {
                answerText += content;
                let textBlock = assistantMsg.content.find((c: any) => c.type === "text");
                if (!textBlock) {
                  textBlock = { type: "text", text: "", startTs: Date.now() };
                  assistantMsg.content.push(textBlock);
                }
                textBlock.text = answerText;
              }
            }

            // 3. Tool calls delta
            if (Array.isArray(delta.tool_calls)) {
              for (const tc of delta.tool_calls) {
                const idx = tc.index ?? 0;
                let curr = pendingToolCalls.get(idx);
                if (!curr) {
                  curr = { id: tc.id || `call-${Date.now()}-${idx}`, name: tc.function?.name || "", argsText: "" };
                  pendingToolCalls.set(idx, curr);
                }
                if (tc.id) curr.id = tc.id;
                if (tc.function?.name) curr.name = tc.function.name;
                if (tc.function?.arguments) curr.argsText += tc.function.arguments;
              }
            }

            session.updatedAt = new Date().toISOString();
            emitEvent("agent:updated", { id });
          } catch {}
        }
      }

      totalOutputTokens += Math.round((thinkingText.length + answerText.length) / 3.8);

      // Finalize tool calls
      if (pendingToolCalls.size > 0) {
        session.state = "executing";
        emitEvent("agent:updated", { id });

        for (const [, tc] of pendingToolCalls.entries()) {
          let parsedArgs = {};
          try {
            parsedArgs = tc.argsText ? JSON.parse(tc.argsText) : {};
          } catch {
            parsedArgs = { raw: tc.argsText };
          }

          assistantMsg.content.push({
            type: "tool_call",
            tool_call_id: tc.id,
            name: tc.name,
            arguments: parsedArgs,
            startTs: Date.now(),
          });

          emitEvent("agent:updated", { id });

          // Execute tool on workspace
          const toolExecResult = await executeAgentTool(tc.name, parsedArgs, session.projectFolder);

          const toolMsg = {
            id: `tool-${Date.now()}-${tc.id}`,
            role: "tool" as const,
            content: [
              {
                type: "tool_result" as const,
                tool_call_id: tc.id,
                text: toolExecResult.result,
                is_error: toolExecResult.is_error,
              },
            ],
            timestamp: new Date().toISOString(),
          };
          session.messages.push(toolMsg);
          emitEvent("agent:updated", { id });
        }

        continue;
      }

      break;
    }
  } catch (err: any) {
    if (!abortController.signal.aborted) {
      console.error("Agent execution error:", err);
      const errId = `msg-${Date.now()}-err`;
      session.messages.push({
        id: errId,
        role: "assistant",
        content: [{ type: "text", text: `**Error:** ${err.message || err}` }],
        timestamp: new Date().toISOString(),
      });
    }
  } finally {
    const latencyMs = Math.max(1, Date.now() - startTime);
    const inTokensEst = Math.round((systemPrompt.length + message.length) / 3.8);
    const outTokensEst = Math.max(totalOutputTokens, Math.round(message.length / 3.8));
    const speedTps = latencyMs > 0 ? Number(((outTokensEst / (latencyMs / 1000))).toFixed(1)) : 0;

    session.token_usage = {
      prompt_tokens: inTokensEst,
      completion_tokens: outTokensEst,
      cached_tokens: 0,
      total_tokens: inTokensEst + outTokensEst,
      speed_tps: speedTps,
    };

    const ws = await GetCurrentWorkspace();
    await RecordLLMUsage({
      provider: activeProfile?.provider || activeProfile?.name || "openai",
      model,
      workspace: ws?.name || "forge-ade-native",
      agentRole: session.role_filter || session.role || "coding",
      inputTokens: inTokensEst,
      outputTokens: outTokensEst,
      cachedTokens: 0,
      latencyMs,
      status: "success",
      toolCalls: iteration - 1,
    });

    session.state = "idle";
    session.updatedAt = new Date().toISOString();
    activeTurnAborts.delete(id);
    localStorage.setItem(STORAGE_KEY_AGENTS, JSON.stringify(list));
    emitEvent("agent:updated", { id });
    emitEvent("agent:turn_end", { id });
  }
}

async function generateSessionTitle(userPrompt: string, apiKey: string, baseURL: string, model: string): Promise<string> {
  try {
    let cleanBase = baseURL.trim().replace(/\/+$/, "");
    if (!cleanBase.endsWith("/v1") && !cleanBase.includes("/v1/")) cleanBase = `${cleanBase}/v1`;
    const res = await fetch(`${cleanBase}/chat/completions`, {
      method: "POST",
      headers: { "Content-Type": "application/json", ...(apiKey ? { Authorization: `Bearer ${apiKey}` } : {}) },
      body: JSON.stringify({
        model,
        temperature: 0.3,
        max_tokens: 25,
        messages: [
          {
            role: "system",
            content: "You are a title generator. Output ONLY a concise 3 to 5 word title summarizing the user prompt. No quotes, no markdown, no punctuation at the end.",
          },
          { role: "user", content: userPrompt.slice(0, 300) },
        ],
      }),
    });
    if (res.ok) {
      const data = await res.json();
      const title = data.choices?.[0]?.message?.content?.trim();
      if (title) {
        return title.replace(/^["']|["']$/g, "").replace(/\.$/, "").trim();
      }
    }
  } catch {}
  return "";
}

export async function RespondAgentApproval(id: string, approve: boolean, autoAll: boolean): Promise<void> {
  if (autoAll) await SetAgentAutoApprove(id, true);
}

export async function RespondAgentAsk(id: string, answers: any): Promise<void> {}

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
    // Preserve existing session name — only update role filter, system prompt, and rules
    await UpdateAgentSession(id, "", def.role_filter || "coding", def.prompt, def.rules);
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
  const ws = await GetCurrentWorkspace();
  const targetPath = repoPath || ws?.folders?.[0] || "";
  if (!targetPath) return "chore: update project files";

  // 1. Gather git diffs (staged first, then unstaged, then status)
  const cachedDiff = await execCommand("git diff --cached", targetPath);
  const unstagedDiff = await execCommand("git diff", targetPath);
  const statusRes = await execCommand("git status --short", targetPath);
  const statRes = await execCommand("git diff --stat", targetPath);

  let rawDiff = cachedDiff.output.trim() || unstagedDiff.output.trim();
  if (!rawDiff) {
    rawDiff = statusRes.output.trim();
  }
  if (!rawDiff) {
    return "chore: update project files";
  }

  // Bound diff length to ~7000 chars to avoid token exhaustion
  let diffContext = rawDiff;
  if (diffContext.length > 7000) {
    diffContext = `${statRes.output.trim()}\n\nDiff snippet:\n${diffContext.slice(0, 6000)}\n\n[...truncated]`;
  }

  // 2. Load custom AI Commit config from localStorage or active profile
  let customCfg: { provider?: string; model?: string; prompt?: string } = {};
  try {
    const raw = localStorage.getItem("forge-ade-ai-commit-config");
    if (raw) customCfg = JSON.parse(raw);
  } catch {}

  const profiles = await GetProviderProfiles();
  const targetProviderId = customCfg.provider || providerId;
  const activeProfile = (targetProviderId && profiles.find((p) => p.id === targetProviderId || p.name === targetProviderId))
    || profiles.find((p) => p.enabled && (p.apiKey || p.api_key || p.baseURL || p.base_url))
    || profiles.find((p) => p.enabled)
    || profiles[0];

  const activeModel = customCfg.model || model || activeProfile?.activeModel || (activeProfile?.selected_models && activeProfile.selected_models[0]) || "gpt-4o";
  const apiKey = activeProfile?.apiKey || activeProfile?.api_key || "";
  let baseURL = activeProfile?.baseURL || activeProfile?.base_url || "https://api.openai.com/v1";
  baseURL = baseURL.trim().replace(/\/+$/, "");

  const defaultPrompt =
    "You are an expert Git commit message generator. Your output MUST be ONLY a concise 1-2 line Git commit message following Conventional Commits format (e.g. 'feat(explorer): dim gitignored files and highlight modified status'). DO NOT include any analysis, explanations, markdown fences, or preamble. Output ONLY the raw commit message text.";

  const systemPrompt = customCfg.prompt || instruction || defaultPrompt;

  try {
    // Anthropic direct API
    if (baseURL.includes("anthropic.com")) {
      const endpoint = `${baseURL}/messages`;
      const res = await fetch(endpoint, {
        method: "POST",
        headers: {
          "Content-Type": "application/json",
          "x-api-key": apiKey,
          "anthropic-version": "2023-06-01",
        },
        body: JSON.stringify({
          model: activeModel,
          max_tokens: 300,
          system: systemPrompt,
          messages: [
            {
              role: "user",
              content: `Git Status:\n${statusRes.output.trim()}\n\nGit Diff:\n${diffContext}`,
            },
          ],
        }),
      });
      if (res.ok) {
        const data = await res.json();
        const text = data.content?.[0]?.text?.trim() || "";
        if (text) return cleanCommitText(text);
      }
    }

    // OpenAI-compatible endpoint
    if (!baseURL.endsWith("/v1") && !baseURL.includes("/v1/")) {
      baseURL = `${baseURL}/v1`;
    }
    const endpoint = `${baseURL}/chat/completions`;
    const headers: Record<string, string> = { "Content-Type": "application/json" };
    if (apiKey) headers["Authorization"] = `Bearer ${apiKey}`;

    const res = await fetch(endpoint, {
      method: "POST",
      headers,
      body: JSON.stringify({
        model: activeModel,
        temperature: 0.2,
        messages: [
          { role: "system", content: systemPrompt },
          { role: "user", content: `Git Status:\n${statusRes.output.trim()}\n\nGit Diff:\n${diffContext}` },
        ],
      }),
    });

    if (res.ok) {
      const data = await res.json();
      const text = data.choices?.[0]?.message?.content?.trim() || "";
      if (text) return cleanCommitText(text);
    }
  } catch (err) {
    console.warn("AI Commit generation request failed:", err);
  }

  // Fallback: smart heuristic from git status
  const firstLine = statusRes.output.split("\n")[0]?.trim() || "";
  return `feat: update project files (${firstLine.slice(3) || "changes"})`;
}

function cleanCommitText(text: string): string {
  let clean = text.trim();
  // Strip markdown code fences
  if (clean.startsWith("```")) {
    clean = clean.replace(/^```[a-z]*\n?/i, "").replace(/\n?```$/i, "").trim();
  }
  // Strip surrounding quotes
  if ((clean.startsWith('"') && clean.endsWith('"')) || (clean.startsWith("'") && clean.endsWith("'"))) {
    clean = clean.slice(1, -1).trim();
  }
  // Remove preamble like "Here is the commit message:"
  clean = clean.replace(/^(here is|commit message:?|here's a commit message:?)\s*/i, "").trim();
  return clean;
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
  try {
    const raw = localStorage.getItem(STORAGE_KEY_USAGE);
    if (raw) {
      const parsed = JSON.parse(raw);
      if (Array.isArray(parsed)) return parsed;
    }
  } catch {}
  return [];
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
  const map = new Map<string, { key: string; label: string; total_tokens: number; cost_usd: number; requests: number }>();

  for (const r of records) {
    let key = "";
    if (dimension === "model") key = r.model || "unknown";
    else if (dimension === "provider") key = r.provider || "unknown";
    else if (dimension === "workspace") key = r.workspace || "unknown";
    else if (dimension === "agent") key = r.agentRole || "coding";
    else key = r.model || "unknown";

    let b = map.get(key);
    if (!b) {
      b = { key, label: key, total_tokens: 0, cost_usd: 0, requests: 0 };
      map.set(key, b);
    }
    b.total_tokens += (r.inputTokens || 0) + (r.outputTokens || 0) + (r.cachedTokens || 0);
    b.cost_usd += r.cost || 0;
    b.requests += 1;
  }

  const buckets = Array.from(map.values());
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
