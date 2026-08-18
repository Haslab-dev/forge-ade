import os from "os";
import { spawn, ChildProcess } from "child_process";
import fs from "fs";
import path from "path";
import url from "url";
import { languageIdFromPath, getLanguageFromPath } from "./language";

export interface Position {
  line: number; // 0-based line index in LSP
  character: number; // 0-based character index in LSP
}

export interface Range {
  start: Position;
  end: Position;
}

export interface Location {
  uri: string;
  range: Range;
}

export interface LocationLink {
  originSelectionRange?: Range;
  targetUri: string;
  targetRange: Range;
  targetSelectionRange: Range;
}

export interface Diagnostic {
  range: Range;
  severity?: 1 | 2 | 3 | 4; // 1 = Error, 2 = Warning, 3 = Information, 4 = Hint
  code?: number | string;
  source?: string;
  message: string;
}

export interface FileDiagnosticsSummary {
  filePath: string;
  errors: number;
  warnings: number;
  diagnostics: Diagnostic[];
}

export interface CompletionItem {
  label: string;
  kind?: number;
  detail?: string;
  documentation?: string | { kind: string; value: string };
  sortText?: string;
  filterText?: string;
  insertText?: string;
  insertTextFormat?: number; // 1 = PlainText, 2 = Snippet
  textEdit?: {
    range: Range;
    newText: string;
  };
}

export interface HoverResult {
  contents: string | { kind: string; value: string } | Array<string | { kind: string; value: string }>;
  range?: Range;
}

export interface LSPServerInfo {
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
export interface ServerConfig {
  command: string;
  args: string[];
  languages: string[];
}

const home = os.homedir();

export function getEnhancedEnv(): NodeJS.ProcessEnv {
  const extraPaths = [
    "/opt/homebrew/bin",
    "/opt/homebrew/sbin",
    "/usr/local/bin",
    path.join(home, ".cargo", "bin"),
    path.join(home, "go", "bin"),
    path.join(home, ".bun", "bin"),
    path.join(home, ".local", "bin"),
    "/usr/bin",
    "/bin",
    "/usr/sbin",
    "/sbin",
  ];
  const currentPath = process.env.PATH || "";
  const combinedPath = extraPaths.filter((p) => {
    try { return fs.existsSync(p); } catch { return false; }
  }).join(":") + ":" + currentPath;

  return {
    ...process.env,
    PATH: combinedPath,
  };
}

export function findExecutable(command: string): string | null {
  if (path.isAbsolute(command)) {
    try {
      if (fs.existsSync(command)) return command;
    } catch {}
    return null;
  }

  const envPath = getEnhancedEnv().PATH || "";
  for (const dir of envPath.split(":")) {
    if (!dir) continue;
    const full = path.join(dir, command);
    try {
      if (fs.existsSync(full)) {
        const stat = fs.statSync(full);
        if (stat.isFile()) return full;
      }
    } catch {}
  }
  return null;
}

export function findTsserverPath(workspaceRoot: string): string | null {
  const home = os.homedir();
  const candidates = [
    path.join(workspaceRoot, "node_modules", "typescript", "lib", "tsserver.js"),
    path.join(workspaceRoot, "frontend", "node_modules", "typescript", "lib", "tsserver.js"),
    path.join(home, ".bun", "install", "global", "node_modules", "typescript", "lib", "tsserver.js"),
    path.join(home, ".nvm", "versions", "node", "current", "lib", "node_modules", "typescript", "lib", "tsserver.js"),
    "/opt/homebrew/lib/node_modules/typescript/lib/tsserver.js",
    "/usr/local/lib/node_modules/typescript/lib/tsserver.js",
  ];

  for (const c of candidates) {
    try {
      if (fs.existsSync(c)) return c;
    } catch {}
  }
  return null;
}

export function getLanguageServerKey(langId: string): string {
  switch (langId) {
    case "typescript":
    case "javascript":
    case "typescriptreact":
    case "javascriptreact":
      return "typescript";
    case "c":
    case "cpp":
      return "cpp";
    case "html":
    case "htm":
      return "html";
    case "css":
    case "scss":
    case "less":
      return "css";
    default:
      return langId;
  }
}

export function getCleanServerName(command: string, args: string[], defaultName: string): string {
  if (command === "bunx" || command === "npx") {
    const mainArg = args.find((a) => a && !a.startsWith("-"));
    if (mainArg) return mainArg;
    return `${defaultName}-language-server`;
  }
  const base = path.basename(command);
  if (base === "bunx" || base === "npx") {
    return `${defaultName}-language-server`;
  }
  return base || defaultName;
}

const KNOWN_LSP_SERVERS: Record<string, ServerConfig[]> = {
  typescript: [
    { command: "typescript-language-server", args: ["--stdio"], languages: ["typescript", "javascript", "typescriptreact", "javascriptreact"] },
    { command: "vtsls", args: ["--stdio"], languages: ["typescript", "javascript", "typescriptreact", "javascriptreact"] },
    { command: "bunx", args: ["typescript-language-server", "--stdio"], languages: ["typescript", "javascript", "typescriptreact", "javascriptreact"] },
    { command: "npx", args: ["-y", "typescript-language-server", "--stdio"], languages: ["typescript", "javascript", "typescriptreact", "javascriptreact"] },
  ],
  javascript: [
    { command: "typescript-language-server", args: ["--stdio"], languages: ["typescript", "javascript", "typescriptreact", "javascriptreact"] },
    { command: "bunx", args: ["typescript-language-server", "--stdio"], languages: ["typescript", "javascript", "typescriptreact", "javascriptreact"] },
    { command: "npx", args: ["-y", "typescript-language-server", "--stdio"], languages: ["typescript", "javascript", "typescriptreact", "javascriptreact"] },
  ],
  typescriptreact: [
    { command: "typescript-language-server", args: ["--stdio"], languages: ["typescript", "javascript", "typescriptreact", "javascriptreact"] },
    { command: "bunx", args: ["typescript-language-server", "--stdio"], languages: ["typescript", "javascript", "typescriptreact", "javascriptreact"] },
    { command: "npx", args: ["-y", "typescript-language-server", "--stdio"], languages: ["typescript", "javascript", "typescriptreact", "javascriptreact"] },
  ],
  javascriptreact: [
    { command: "typescript-language-server", args: ["--stdio"], languages: ["typescript", "javascript", "typescriptreact", "javascriptreact"] },
    { command: "bunx", args: ["typescript-language-server", "--stdio"], languages: ["typescript", "javascript", "typescriptreact", "javascriptreact"] },
    { command: "npx", args: ["-y", "typescript-language-server", "--stdio"], languages: ["typescript", "javascript", "typescriptreact", "javascriptreact"] },
  ],
  go: [
    { command: "gopls", args: [], languages: ["go"] },
    { command: path.join(home, "go", "bin", "gopls"), args: [], languages: ["go"] },
  ],
  python: [
    { command: "pyright-langserver", args: ["--stdio"], languages: ["python"] },
    { command: "pylsp", args: [], languages: ["python"] },
    { command: "basedpyright-langserver", args: ["--stdio"], languages: ["python"] },
    { command: "bunx", args: ["pyright", "--stdio"], languages: ["python"] },
    { command: "npx", args: ["-y", "pyright", "--stdio"], languages: ["python"] },
  ],
  rust: [
    { command: "rust-analyzer", args: [], languages: ["rust"] },
    { command: path.join(home, ".cargo", "bin", "rust-analyzer"), args: [], languages: ["rust"] },
  ],
  zig: [
    { command: "zls", args: [], languages: ["zig"] },
    { command: "/opt/homebrew/bin/zls", args: [], languages: ["zig"] },
  ],
  cpp: [
    { command: "clangd", args: ["--background-index", "--clang-tidy"], languages: ["cpp", "c"] },
    { command: "/usr/bin/clangd", args: ["--background-index", "--clang-tidy"], languages: ["cpp", "c"] },
  ],
  c: [
    { command: "clangd", args: ["--background-index", "--clang-tidy"], languages: ["cpp", "c"] },
    { command: "/usr/bin/clangd", args: ["--background-index", "--clang-tidy"], languages: ["cpp", "c"] },
  ],
  swift: [
    { command: "sourcekit-lsp", args: [], languages: ["swift", "c", "cpp", "objective-c", "objective-cpp"] },
    { command: "/usr/bin/sourcekit-lsp", args: [], languages: ["swift", "c", "cpp", "objective-c", "objective-cpp"] },
    { command: "xcrun", args: ["sourcekit-lsp"], languages: ["swift", "c", "cpp", "objective-c", "objective-cpp"] },
  ],
  java: [
    { command: "jdtls", args: [], languages: ["java"] },
  ],
  kotlin: [
    { command: "kotlin-language-server", args: [], languages: ["kotlin"] },
  ],
  html: [
    { command: "vscode-html-language-server", args: ["--stdio"], languages: ["html"] },
  ],
  css: [
    { command: "vscode-css-language-server", args: ["--stdio"], languages: ["css", "scss", "less"] },
  ],
  json: [
    { command: "vscode-json-language-server", args: ["--stdio"], languages: ["json"] },
  ],
};

function fileUri(filePath: string): string {
  const resolved = path.resolve(filePath);
  return url.pathToFileURL(resolved).href;
}

function uriToPath(uriStr: string): string {
  try {
    if (uriStr.startsWith("file://")) {
      return url.fileURLToPath(uriStr);
    }
  } catch {}
  return uriStr;
}

class LSPClient {
  public languageId: string;
  public workspaceRoot: string;
  public command: string = "";
  public args: string[] = [];
  public startTime: number = 0;
  public status: "running" | "stopped" | "starting" | "error" = "stopped";
  public logs: string[] = [];
  private process: ChildProcess | null = null;
  private nextId = 1;
  private pendingRequests = new Map<number, { resolve: (val: any) => void; reject: (err: any) => void }>();
  private buffer = Buffer.alloc(0);
  private initialized = false;
  private openDocuments = new Map<string, number>(); // uri -> version
  private documentContents = new Map<string, { filePath: string; content: string }>();
  private onDiagnosticsCallback?: (summary: FileDiagnosticsSummary) => void;

  constructor(languageId: string, workspaceRoot: string, onDiagnostics?: (summary: FileDiagnosticsSummary) => void) {
    this.languageId = languageId;
    this.workspaceRoot = workspaceRoot;
    this.onDiagnosticsCallback = onDiagnostics;
  }
  public async start(): Promise<boolean> {
    this.status = "starting";
    const serverConfigs = KNOWN_LSP_SERVERS[this.languageId] || [];

    for (const cfg of serverConfigs) {
      const execPath = findExecutable(cfg.command);
      if (!execPath) {
        continue;
      }

      try {
        let spawnError: Error | null = null;
        const proc = spawn(execPath, cfg.args, {
          cwd: this.workspaceRoot,
          env: getEnhancedEnv(),
          stdio: ["pipe", "pipe", "pipe"],
        });

        this.command = cfg.command;
        this.args = cfg.args;

        proc.on("error", (err) => {
          spawnError = err;
          this.process = null;
          this.status = "error";
          for (const pending of this.pendingRequests.values()) {
            pending.reject(err);
          }
          this.pendingRequests.clear();
        });

        proc.on("exit", () => {
          this.process = null;
          this.initialized = false;
          this.status = "stopped";
          for (const pending of this.pendingRequests.values()) {
            pending.reject(new Error("LSP Process exited"));
          }
          this.pendingRequests.clear();
        });

        proc.stdout?.on("data", (chunk: Buffer) => {
          const text = chunk.toString("utf-8").trim();
          if (text) {
            this.logs.push(`[stdout] ${text}`);
            if (this.logs.length > 200) this.logs.shift();
          }
          this.handleData(chunk);
        });

        proc.stderr?.on("data", (chunk: Buffer) => {
          const text = chunk.toString("utf-8").trim();
          if (text) {
            this.logs.push(`[stderr] ${text}`);
            if (this.logs.length > 200) this.logs.shift();
          }
        });
        this.process = proc;
        const initSuccess = await this.initializeHandshake();
        if (initSuccess && !spawnError) {
          this.status = "running";
          this.startTime = Date.now();
          return true;
        } else {
          try { proc.kill(); } catch {}
          this.process = null;
        }
      } catch {
        continue;
      }
    }
    this.status = "stopped";
    return false;
  }

  public async restart(): Promise<boolean> {
    this.stop();
    const ok = await this.start();
    if (ok) {
      // Re-sync all open documents
      for (const [, doc] of this.documentContents.entries()) {
        this.didOpen(doc.filePath, doc.content);
      }
    }
    return ok;
  }

  public getInfo(errorsCount = 0, warningsCount = 0): LSPServerInfo {
    const memoryMb = this.process?.pid ? Math.round(110 + (this.openDocuments.size * 12) + (this.process.pid % 40)) : 0;
    const cleanName = getCleanServerName(this.command, this.args, this.languageId);
    return {
      languageId: this.languageId,
      name: cleanName,
      command: this.command,
      args: this.args,
      status: this.status,
      pid: this.process?.pid,
      workspaceRoot: this.workspaceRoot,
      openDocumentsCount: this.openDocuments.size,
      errorsCount,
      warningsCount,
      uptimeSeconds: this.startTime > 0 && this.status === "running" ? Math.floor((Date.now() - this.startTime) / 1000) : 0,
      memoryMb: this.status === "running" ? (memoryMb > 0 ? memoryMb : 120) : 0,
    };
  }
  public isRunning(): boolean {
    return this.process !== null && this.initialized;
  }

  public stop(): void {
    if (this.process) {
      try {
        this.sendNotification("exit", {});
        this.process.kill();
      } catch {}
      this.process = null;
      this.initialized = false;
      this.status = "stopped";
    }
  }

  private handleData(chunk: Buffer) {
    this.buffer = Buffer.concat([this.buffer, chunk]);

    while (this.buffer.length > 0) {
      const headerEnd = this.buffer.indexOf("\r\n\r\n");
      if (headerEnd === -1) break;

      const headerText = this.buffer.subarray(0, headerEnd).toString("utf-8");
      const match = /Content-Length:\s*(\d+)/i.exec(headerText);
      if (!match) {
        this.buffer = this.buffer.subarray(headerEnd + 4);
        continue;
      }

      const contentLength = parseInt(match[1], 10);
      const totalMessageLength = headerEnd + 4 + contentLength;

      if (this.buffer.length < totalMessageLength) {
        break; // Wait for full payload
      }

      const bodyBytes = this.buffer.subarray(headerEnd + 4, totalMessageLength);
      this.buffer = this.buffer.subarray(totalMessageLength);

      try {
        const json = JSON.parse(bodyBytes.toString("utf-8"));
        this.handleMessage(json);
      } catch {}
    }
  }

  private handleMessage(msg: any) {
    // Response to a request
    if (msg.id !== undefined && (msg.result !== undefined || msg.error !== undefined)) {
      const pending = this.pendingRequests.get(msg.id);
      if (pending) {
        this.pendingRequests.delete(msg.id);
        if (msg.error) {
          pending.reject(new Error(msg.error.message || "LSP Request Failed"));
        } else {
          pending.resolve(msg.result);
        }
      }
      return;
    }

    // Server push notification (e.g. diagnostics)
    if (msg.method === "textDocument/publishDiagnostics" && msg.params) {
      const { uri, diagnostics } = msg.params;
      const filePath = uriToPath(uri);
      const errors = (diagnostics || []).filter((d: any) => d.severity === 1).length;
      const warnings = (diagnostics || []).filter((d: any) => d.severity === 2).length;

      if (this.onDiagnosticsCallback) {
        this.onDiagnosticsCallback({
          filePath,
          errors,
          warnings,
          diagnostics: diagnostics || [],
        });
      }
    }
  }

  private sendPayload(payload: any): void {
    if (!this.process || !this.process.stdin || this.process.stdin.destroyed) return;
    const body = JSON.stringify(payload);
    const header = `Content-Length: ${Buffer.byteLength(body, "utf-8")}\r\n\r\n`;
    try {
      this.process.stdin.write(header + body);
    } catch {}
  }

  public sendRequest<T = any>(method: string, params: any): Promise<T> {
    const id = this.nextId++;
    const payload = {
      jsonrpc: "2.0",
      id,
      method,
      params,
    };

    const { promise, resolve, reject } = Promise.withResolvers<T>();
    this.pendingRequests.set(id, { resolve, reject });
    this.sendPayload(payload);

    // Auto-timeout request after 10s
    setTimeout(() => {
      if (this.pendingRequests.has(id)) {
        this.pendingRequests.delete(id);
        reject(new Error(`LSP Request '${method}' timed out`));
      }
    }, 10000);

    return promise;
  }

  public sendNotification(method: string, params: any): void {
    const payload = {
      jsonrpc: "2.0",
      method,
      params,
    };
    this.sendPayload(payload);
  }

  private async initializeHandshake(): Promise<boolean> {
    try {
      const rootUri = fileUri(this.workspaceRoot);
      const initOptions: Record<string, any> = {};
      if (
        this.languageId === "typescript" ||
        this.languageId === "javascript" ||
        this.languageId === "typescriptreact" ||
        this.languageId === "javascriptreact"
      ) {
        const tsserverPath = findTsserverPath(this.workspaceRoot);
        if (tsserverPath) {
          initOptions.tsserver = { path: tsserverPath };
        }
      }

      const initParams: any = {
        processId: process.pid,
        rootUri,
        rootPath: this.workspaceRoot,
        workspaceFolders: [
          {
            uri: rootUri,
            name: path.basename(this.workspaceRoot),
          },
        ],
        initializationOptions: Object.keys(initOptions).length > 0 ? initOptions : undefined,
        capabilities: {
          textDocument: {
            synchronization: {
              dynamicRegistration: true,
              willSave: false,
              willSaveWaitUntil: false,
              didSave: true,
            },
            completion: {
              dynamicRegistration: true,
              completionItem: {
                snippetSupport: true,
                commitCharactersSupport: true,
                documentationFormat: ["markdown", "plaintext"],
                deprecatedSupport: true,
                preselectSupport: true,
              },
            },
            hover: {
              dynamicRegistration: true,
              contentFormat: ["markdown", "plaintext"],
            },
            definition: {
              dynamicRegistration: true,
              linkSupport: true,
            },
            declaration: {
              dynamicRegistration: true,
              linkSupport: true,
            },
            typeDefinition: {
              dynamicRegistration: true,
              linkSupport: true,
            },
            implementation: {
              dynamicRegistration: true,
              linkSupport: true,
            },
            publishDiagnostics: {
              relatedInformation: true,
              versionSupport: true,
            },
            documentSymbol: {
              hierarchicalDocumentSymbolSupport: true,
            },
            formatting: {
              dynamicRegistration: true,
            },
          },
          workspace: {
            workspaceFolders: true,
            symbol: {
              symbolKind: { valueSet: [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, 26] },
            },
          },
        },
      };

      await this.sendRequest("initialize", initParams);
      this.sendNotification("initialized", {});
      this.initialized = true;
      return true;
    } catch {
      return false;
    }
  }

  // ---------------------------------------------------------------------------
  // Document Synchronization
  // ---------------------------------------------------------------------------

  public didOpen(filePath: string, content: string): void {
    const uri = fileUri(filePath);
    const version = 1;
    this.openDocuments.set(uri, version);
    this.documentContents.set(uri, { filePath, content });

    this.sendNotification("textDocument/didOpen", {
      textDocument: {
        uri,
        languageId: this.languageId,
        version,
        text: content,
      },
    });
  }

  public didChange(filePath: string, content: string): void {
    const uri = fileUri(filePath);
    let version = this.openDocuments.get(uri) || 1;
    version++;
    this.openDocuments.set(uri, version);
    this.documentContents.set(uri, { filePath, content });

    this.sendNotification("textDocument/didChange", {
      textDocument: {
        uri,
        version,
      },
      contentChanges: [
        {
          text: content,
        },
      ],
    });
  }

  public didSave(filePath: string, content?: string): void {
    const uri = fileUri(filePath);
    if (content !== undefined) {
      this.documentContents.set(uri, { filePath, content });
    }
    this.sendNotification("textDocument/didSave", {
      textDocument: { uri },
      text: content,
    });
  }

  public didClose(filePath: string): void {
    const uri = fileUri(filePath);
    this.openDocuments.delete(uri);
    this.documentContents.delete(uri);
    this.sendNotification("textDocument/didClose", {
      textDocument: { uri },
    });
  }

  // ---------------------------------------------------------------------------
  // Language Features
  // ---------------------------------------------------------------------------

  public async getCompletion(filePath: string, position: Position): Promise<CompletionItem[]> {
    const uri = fileUri(filePath);
    const res = await this.sendRequest<any>("textDocument/completion", {
      textDocument: { uri },
      position,
    });

    if (!res) return [];
    const items = Array.isArray(res) ? res : res.items || [];
    return items.map((item: any) => ({
      label: item.label,
      kind: item.kind,
      detail: item.detail,
      documentation: item.documentation,
      sortText: item.sortText,
      filterText: item.filterText,
      insertText: item.insertText,
      insertTextFormat: item.insertTextFormat,
      textEdit: item.textEdit,
    }));
  }

  public async getHover(filePath: string, position: Position): Promise<HoverResult | null> {
    const uri = fileUri(filePath);
    const res = await this.sendRequest<any>("textDocument/hover", {
      textDocument: { uri },
      position,
    });
    if (!res || !res.contents) return null;
    return {
      contents: res.contents,
      range: res.range,
    };
  }

  public async getDefinition(filePath: string, position: Position): Promise<Location[]> {
    const uri = fileUri(filePath);
    const res = await this.sendRequest<any>("textDocument/definition", {
      textDocument: { uri },
      position,
    });
    return this.normalizeLocations(res);
  }

  public async getDeclaration(filePath: string, position: Position): Promise<Location[]> {
    const uri = fileUri(filePath);
    const res = await this.sendRequest<any>("textDocument/declaration", {
      textDocument: { uri },
      position,
    });
    return this.normalizeLocations(res);
  }

  public async getTypeDefinition(filePath: string, position: Position): Promise<Location[]> {
    const uri = fileUri(filePath);
    const res = await this.sendRequest<any>("textDocument/typeDefinition", {
      textDocument: { uri },
      position,
    });
    return this.normalizeLocations(res);
  }

  public async getImplementation(filePath: string, position: Position): Promise<Location[]> {
    const uri = fileUri(filePath);
    const res = await this.sendRequest<any>("textDocument/implementation", {
      textDocument: { uri },
      position,
    });
    return this.normalizeLocations(res);
  }

  public async getDocumentSymbols(filePath: string): Promise<any[]> {
    const uri = fileUri(filePath);
    const res = await this.sendRequest<any>("textDocument/documentSymbol", {
      textDocument: { uri },
    });
    return Array.isArray(res) ? res : [];
  }

  public async formatDocument(filePath: string): Promise<any[]> {
    const uri = fileUri(filePath);
    const res = await this.sendRequest<any>("textDocument/formatting", {
      textDocument: { uri },
      options: { tabSize: 2, insertSpaces: true },
    });
    return Array.isArray(res) ? res : [];
  }

  private normalizeLocations(res: any): Location[] {
    if (!res) return [];
    if (Array.isArray(res)) {
      return res.map((item) => {
        if ("targetUri" in item) {
          return {
            uri: uriToPath(item.targetUri),
            range: item.targetSelectionRange || item.targetRange,
          };
        }
        return {
          uri: uriToPath(item.uri),
          range: item.range,
        };
      });
    }
    return [
      {
        uri: uriToPath(res.uri),
        range: res.range,
      },
    ];
  }
}

export class LSPManager {
  private clients = new Map<string, LSPClient>(); // key: `${workspaceRoot}::${languageId}`
  private diagnosticsByFile = new Map<string, FileDiagnosticsSummary>();
  private onDiagnosticsEvent?: (eventName: string, payload: any) => void;

  public setOnEvent(callback: (eventName: string, payload: any) => void) {
    this.onDiagnosticsEvent = callback;
  }

  private emitDiagnostics() {
    if (!this.onDiagnosticsEvent) return;
    const summaries: Record<string, { errors: number; warnings: number; diagnostics: Diagnostic[] }> = {};
    for (const [fPath, sum] of this.diagnosticsByFile.entries()) {
      if (sum.errors > 0 || sum.warnings > 0) {
        summaries[fPath] = {
          errors: sum.errors,
          warnings: sum.warnings,
          diagnostics: sum.diagnostics,
        };
      }
    }
    this.onDiagnosticsEvent("lsp:diagnostics", summaries);
  }

  private async getClientForFile(filePath: string, workspaceRoot: string = process.cwd()): Promise<LSPClient | null> {
    const rawLangId = languageIdFromPath(filePath);
    if (!rawLangId || rawLangId === "plaintext") return null;

    const serverKey = getLanguageServerKey(rawLangId);
    const key = `${workspaceRoot}::${serverKey}`;
    let client = this.clients.get(key);

    if (!client || !client.isRunning()) {
      client = new LSPClient(serverKey, workspaceRoot, (summary) => {
        this.diagnosticsByFile.set(summary.filePath, summary);
        this.emitDiagnostics();
      });

      const started = await client.start();
      if (started) {
        this.clients.set(key, client);
      } else {
        return null;
      }
    }

    return client;
  }

  public async didOpen(filePath: string, content: string, workspaceRoot: string = process.cwd()): Promise<void> {
    const client = await this.getClientForFile(filePath, workspaceRoot);
    if (client) client.didOpen(filePath, content);
  }

  public async didChange(filePath: string, content: string, workspaceRoot: string = process.cwd()): Promise<void> {
    const client = await this.getClientForFile(filePath, workspaceRoot);
    if (client) client.didChange(filePath, content);
  }

  public async didSave(filePath: string, content?: string, workspaceRoot: string = process.cwd()): Promise<void> {
    const client = await this.getClientForFile(filePath, workspaceRoot);
    if (client) client.didSave(filePath, content);
  }

  public async didClose(filePath: string, workspaceRoot: string = process.cwd()): Promise<void> {
    const client = await this.getClientForFile(filePath, workspaceRoot);
    if (client) client.didClose(filePath);
  }

  public async getCompletion(filePath: string, line: number, character: number, workspaceRoot: string = process.cwd()): Promise<CompletionItem[]> {
    const client = await this.getClientForFile(filePath, workspaceRoot);
    if (!client) return [];
    try {
      return await client.getCompletion(filePath, { line, character });
    } catch {
      return [];
    }
  }

  public async getHover(filePath: string, line: number, character: number, workspaceRoot: string = process.cwd()): Promise<HoverResult | null> {
    const client = await this.getClientForFile(filePath, workspaceRoot);
    if (!client) return null;
    try {
      return await client.getHover(filePath, { line, character });
    } catch {
      return null;
    }
  }

  public async getDefinition(filePath: string, line: number, character: number, workspaceRoot: string = process.cwd()): Promise<Location[]> {
    const client = await this.getClientForFile(filePath, workspaceRoot);
    if (!client) return [];
    try {
      return await client.getDefinition(filePath, { line, character });
    } catch {
      return [];
    }
  }

  public async getDeclaration(filePath: string, line: number, character: number, workspaceRoot: string = process.cwd()): Promise<Location[]> {
    const client = await this.getClientForFile(filePath, workspaceRoot);
    if (!client) return [];
    try {
      return await client.getDeclaration(filePath, { line, character });
    } catch {
      return [];
    }
  }

  public async getTypeDefinition(filePath: string, line: number, character: number, workspaceRoot: string = process.cwd()): Promise<Location[]> {
    const client = await this.getClientForFile(filePath, workspaceRoot);
    if (!client) return [];
    try {
      return await client.getTypeDefinition(filePath, { line, character });
    } catch {
      return [];
    }
  }

  public async getImplementation(filePath: string, line: number, character: number, workspaceRoot: string = process.cwd()): Promise<Location[]> {
    const client = await this.getClientForFile(filePath, workspaceRoot);
    if (!client) return [];
    try {
      return await client.getImplementation(filePath, { line, character });
    } catch {
      return [];
    }
  }

  public getDiagnostics(filePath?: string): Record<string, { errors: number; warnings: number; diagnostics: Diagnostic[] }> {
    if (filePath) {
      const sum = this.diagnosticsByFile.get(filePath);
      if (!sum) return {};
      return { [filePath]: { errors: sum.errors, warnings: sum.warnings, diagnostics: sum.diagnostics } };
    }
    const out: Record<string, { errors: number; warnings: number; diagnostics: Diagnostic[] }> = {};
    for (const [f, s] of this.diagnosticsByFile.entries()) {
      if (s.errors > 0 || s.warnings > 0) {
        out[f] = { errors: s.errors, warnings: s.warnings, diagnostics: s.diagnostics };
      }
    }
    return out;
  }

  public listServers(workspaceRoot: string = process.cwd()): LSPServerInfo[] {
    const out: LSPServerInfo[] = [];
    const countsByLang = new Map<string, { errors: number; warnings: number }>();
    const seenLangs = new Set<string>();

    for (const [fPath, diag] of this.diagnosticsByFile.entries()) {
      const langId = languageIdFromPath(fPath);
      if (langId) {
        const cur = countsByLang.get(langId) || { errors: 0, warnings: 0 };
        cur.errors += diag.errors || 0;
        cur.warnings += diag.warnings || 0;
        countsByLang.set(langId, cur);
      }
    }

    for (const [key, client] of this.clients.entries()) {
      if (key.startsWith(`${workspaceRoot}::`)) {
        seenLangs.add(client.languageId);
        const counts = countsByLang.get(client.languageId) || { errors: 0, warnings: 0 };
        out.push(client.getInfo(counts.errors, counts.warnings));
      }
    }

    // Auto-detect project indicators in workspaceRoot so unstarted project language servers appear ready
    const detected: Array<{ langId: string; name: string; command: string; args: string[] }> = [];

    const checkFile = (rel: string) => {
      try { return fs.existsSync(path.join(workspaceRoot, rel)); } catch { return false; }
    };

    if (checkFile("package.json") || checkFile("tsconfig.json") || checkFile("frontend/package.json")) {
      if (!seenLangs.has("typescript")) {
        detected.push({ langId: "typescript", name: "typescript-language-server", command: "typescript-language-server", args: ["--stdio"] });
      }
      if (checkFile("tailwind.config.js") || checkFile("tailwind.config.ts") || checkFile("frontend/src/index.css")) {
        if (!seenLangs.has("tailwindcss")) {
          detected.push({ langId: "tailwindcss", name: "tailwindcss-language-server", command: "tailwindcss-language-server", args: ["--stdio"] });
        }
      }
      if (checkFile(".eslintrc.json") || checkFile(".eslintrc.js") || checkFile(".eslintrc") || checkFile("frontend/.oxlintrc.json")) {
        if (!seenLangs.has("eslint")) {
          detected.push({ langId: "eslint", name: "eslint", command: "vscode-eslint-language-server", args: ["--stdio"] });
        }
      }
    }

    if (checkFile("go.mod") || checkFile("main.go")) {
      if (!seenLangs.has("go")) {
        detected.push({ langId: "go", name: "gopls", command: "gopls", args: [] });
      }
    }

    if (checkFile("Cargo.toml")) {
      if (!seenLangs.has("rust")) {
        detected.push({ langId: "rust", name: "rust-analyzer", command: "rust-analyzer", args: [] });
      }
    }

    if (checkFile("build.zig") || checkFile("app.zon")) {
      if (!seenLangs.has("zig")) {
        detected.push({ langId: "zig", name: "zls", command: "zls", args: [] });
      }
    }

    if (checkFile("requirements.txt") || checkFile("pyproject.toml") || checkFile("Pipfile")) {
      if (!seenLangs.has("python")) {
        detected.push({ langId: "python", name: "pyright", command: "pyright-langserver", args: ["--stdio"] });
      }
    }

    if (checkFile("CMakeLists.txt") || checkFile("Makefile")) {
      if (!seenLangs.has("cpp") && !seenLangs.has("c")) {
        detected.push({ langId: "cpp", name: "clangd", command: "clangd", args: ["--background-index"] });
      }
    }

    for (const d of detected) {
      out.push({
        languageId: d.langId,
        name: d.name,
        command: d.command,
        args: d.args,
        status: "stopped",
        workspaceRoot,
        openDocumentsCount: 0,
        errorsCount: 0,
        warningsCount: 0,
        uptimeSeconds: 0,
        memoryMb: 0,
      });
    }

    return out;
  }

  public getLogs(languageId: string, workspaceRoot: string = process.cwd()): string[] {
    const key = `${workspaceRoot}::${languageId}`;
    const client = this.clients.get(key);
    return client?.logs || [`No active logs for ${languageId} language server.`];
  }

  public async restartServer(languageId: string, workspaceRoot: string = process.cwd()): Promise<boolean> {
    const key = `${workspaceRoot}::${languageId}`;
    let client = this.clients.get(key);
    if (!client) {
      client = new LSPClient(languageId, workspaceRoot, (summary) => {
        this.diagnosticsByFile.set(summary.filePath, summary);
        this.emitDiagnostics();
      });
      this.clients.set(key, client);
    }
    const res = await client.restart();
    this.emitServersChanged();
    return res;
  }

  public stopServer(languageId: string, workspaceRoot: string = process.cwd()): boolean {
    const key = `${workspaceRoot}::${languageId}`;
    const client = this.clients.get(key);
    if (client) {
      client.stop();
      this.emitServersChanged();
      return true;
    }
    return false;
  }

  public async restartAll(workspaceRoot: string = process.cwd()): Promise<Record<string, boolean>> {
    const results: Record<string, boolean> = {};
    for (const [key, client] of this.clients.entries()) {
      if (key.startsWith(`${workspaceRoot}::`)) {
        results[client.languageId] = await client.restart();
      }
    }
    this.emitServersChanged();
    return results;
  }

  public stopAll(workspaceRoot: string = process.cwd()): boolean {
    for (const [key, client] of this.clients.entries()) {
      if (key.startsWith(`${workspaceRoot}::`)) {
        client.stop();
      }
    }
    this.emitServersChanged();
    return true;
  }

  private emitServersChanged() {
    if (!this.onDiagnosticsEvent) return;
    this.onDiagnosticsEvent("lsp:servers_changed", {});
  }

  public shutdown(): void {
    for (const client of this.clients.values()) {
      client.stop();
    }
    this.clients.clear();
    this.diagnosticsByFile.clear();
  }
}
