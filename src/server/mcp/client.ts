import { spawn, type ChildProcess, execFileSync } from "node:child_process";
import fs from "node:fs";
import path from "node:path";
import os from "node:os";
// Real MCP stdio client: JSON-RPC 2.0 over newline-delimited frames on a
// subprocess's stdin/stdout (mirrors internal/mcp/stdio.go semantics).

export interface McpToolDef {
  name: string;
  description?: string | undefined;
  inputSchema?: Record<string, unknown> | undefined;
}

export interface McpCallResult {
  content: string;
  isError: boolean;
}

interface JsonRpcErrorShape {
  code: number;
  message: string;
}

interface PendingEntry {
  resolve: (value: unknown) => void;
  reject: (err: Error) => void;
  timer: NodeJS.Timeout;
}

const DEFAULT_REQUEST_TIMEOUT_MS = 10_000;
const DEFAULT_INIT_TIMEOUT_MS = 15_000;
const STDERR_TAIL_CHARS = 4096;
const KILL_GRACE_MS = 1500;
const PROTOCOL_VERSION = "2024-11-05";

function isRecord(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

function findCachedNpxBinary(pkgName: string): string | null {
  const npxRoot = path.join(os.homedir(), ".npm", "_npx");
  if (!fs.existsSync(npxRoot)) return null;
  const binName = pkgName.split("/").pop() ?? pkgName;
  try {
    const hashes = fs.readdirSync(npxRoot);
    for (const h of hashes) {
      const candidate = path.join(npxRoot, h, "node_modules", ".bin", binName);
      if (fs.existsSync(candidate)) return candidate;
    }
  } catch {}
  return null;
}

function asJsonRpcError(value: unknown): JsonRpcErrorShape | null {
  if (!isRecord(value)) return null;
  if (typeof value.code !== "number" || typeof value.message !== "string") return null;
  return { code: value.code, message: value.message };
}

export class McpStdioClient {
  readonly serverName: string;

  private readonly child: ChildProcess;
  private readonly requestTimeoutMs: number;
  private readonly pending = new Map<number, PendingEntry>();
  private nextId = 1;
  private alive = true;
  private stdoutBuffer = "";
  private stderrTail = "";

  private stderrTailFor(_name: string): string {
    return this.stderrTail.trim().slice(-400);
  }

  private constructor(
    child: ChildProcess,
    opts: { serverName: string; requestTimeoutMs: number },
  ) {
    this.child = child;
    this.serverName = opts.serverName;
    this.requestTimeoutMs = opts.requestTimeoutMs;

    child.stdout?.setEncoding("utf8");
    child.stdout?.on("data", (chunk: string) => this.consumeStdout(chunk));
    child.stderr?.setEncoding("utf8");
    child.stderr?.on("data", (chunk: string) => {
      this.stderrTail = (this.stderrTail + chunk).slice(-STDERR_TAIL_CHARS);
    });
    // Writes can race process exit (EPIPE); keep the stream's own 'error'
    // event from surfacing as an unhandled error.
    child.stdin?.on("error", () => {});
    child.on("error", () => this.handleChildDown());
    child.on("close", () => this.handleChildDown());
  }

  /** Spawns the server process and performs the initialize handshake; rejects on failure/timeout. */
  static async spawn(opts: {
    command: string;
    args?: string[] | undefined;
    env?: Record<string, string> | undefined;
    cwd?: string | undefined;
    requestTimeoutMs?: number | undefined;
    initTimeoutMs?: number | undefined;
    serverName: string;
  }): Promise<McpStdioClient> {
    let command = opts.command;
    let args = opts.args ? [...opts.args] : [];

    // If running via npx, check if the package binary is already installed locally in ~/.npm/_npx.
    // Executing the binary directly completely bypasses npx's network version-checks and lock contention.
    if (command === "npx" || command.endsWith("/npx")) {
      const pkgIndex = args.findIndex((a) => !a.startsWith("-"));
      if (pkgIndex !== -1) {
        const pkgName = args[pkgIndex];
        const cachedBin = findCachedNpxBinary(pkgName);
        if (cachedBin) {
          command = cachedBin;
          args = args.slice(pkgIndex + 1);
        }
      }
    }

    // Resolve bare commands: check homebrew / user bin before shelling out.
    if (!command.includes("/")) {
      const candidates = [
        path.join(os.homedir(), "homebrew", "bin", command),
        path.join("/opt", "homebrew", "bin", command),
        path.join("/usr", "local", "bin", command),
      ];
      const found = candidates.find((c) => fs.existsSync(c));
      if (found) {
        command = found;
      } else {
        try {
          const shell = process.env.SHELL || "/bin/bash";
          const resolved = execFileSync(shell, ["-ilc", `command -v -- ${command}`], {
            encoding: "utf-8",
            timeout: 10_000,
          })
            .trim()
            .split("\n")
            .pop();
          if (resolved) command = resolved.trim();
        } catch {}
      }
    }
    const env: Record<string, string> = {
      CI: "1",
      TERM: "dumb",
      NO_COLOR: "1",
      FORCE_COLOR: "0",
      NPM_CONFIG_YES: "true",
    };
    for (const [key, value] of Object.entries(process.env)) {
      if (value !== undefined) env[key] = value;
    }
    if (opts.env !== undefined) Object.assign(env, opts.env);
    // Re-assert non-interactive mode so tools like npx never probe /dev/tty
    env.CI = "1";
    env.TERM = "dumb";
    env.NO_COLOR = "1";
    env.FORCE_COLOR = "0";
    env.NPM_CONFIG_YES = "true";

    // Ensure the directory of the resolved executable (e.g. ~/homebrew/bin)
    // is at the front of PATH so child scripts (like npx's node shebang)
    // resolve the matching node runtime.
    const binDir = path.dirname(command);
    const extraPaths = [binDir, path.join(os.homedir(), "homebrew", "bin"), "/opt/homebrew/bin", "/usr/local/bin"].filter(
      (p) => fs.existsSync(p),
    );
    env.PATH = [...new Set([...extraPaths, ...(env.PATH ? env.PATH.split(":") : [])])].join(":");
    console.log(`[mcp-client] ${opts.serverName}: spawn "${command}" (cwd=${opts.cwd ?? process.cwd()})`);
    const child = spawn(command, args, {
      stdio: ["pipe", "pipe", "pipe"],
      env,
      ...(opts.cwd !== undefined ? { cwd: opts.cwd } : {}),
    });

    const client = new McpStdioClient(child, {
      serverName: opts.serverName,
      requestTimeoutMs: opts.requestTimeoutMs ?? DEFAULT_REQUEST_TIMEOUT_MS,
    });

    try {
      await client.request("initialize", {
        protocolVersion: PROTOCOL_VERSION,
        capabilities: {},
        clientInfo: { name: "forge-ade", version: "1.0.0" },
      }, opts.initTimeoutMs ?? DEFAULT_INIT_TIMEOUT_MS);
    } catch (err) {
      client.close();
      const tail = client.stderrTail;
      throw err instanceof Error
        ? new Error(`${err.message}${tail ? ` | stderr: ${tail}` : ""}`)
        : new Error(`mcp initialize ${opts.serverName}: ${String(err)}${tail ? ` | stderr: ${tail}` : ""}`);
    }

    client.notify("notifications/initialized");
    return client;
  }

  get connected(): boolean {
    return this.alive && this.child.exitCode === null;
  }

  async listTools(): Promise<McpToolDef[]> {
    const tools: McpToolDef[] = [];
    let cursor: string | undefined;
    for (;;) {
      const params: Record<string, unknown> = cursor === undefined ? {} : { cursor };
      const res = await this.request("tools/list", params);
      if (!isRecord(res)) throw this.error(`mcp ${this.serverName}: tools/list returned a non-object result`);
      const rawTools = Array.isArray(res.tools) ? res.tools : [];
      for (const raw of rawTools) {
        if (!isRecord(raw) || typeof raw.name !== "string") continue;
        const def: McpToolDef = { name: raw.name };
        if (typeof raw.description === "string") def.description = raw.description;
        if (isRecord(raw.inputSchema)) def.inputSchema = raw.inputSchema;
        tools.push(def);
      }
      const next = res.nextCursor;
      if (typeof next !== "string" || next === "") break;
      cursor = next;
    }
    return tools;
  }

  async callTool(name: string, args: Record<string, unknown>): Promise<McpCallResult> {
    const res = await this.request("tools/call", { name, arguments: args });
    if (!isRecord(res)) throw this.error(`mcp ${this.serverName}: tools/call returned a non-object result`);

    let content = "";
    const rawContent = res.content;
    if (typeof rawContent === "string") {
      content = rawContent;
    } else if (Array.isArray(rawContent)) {
      const parts: string[] = [];
      for (const block of rawContent) {
        if (isRecord(block) && block.type === "text" && typeof block.text === "string") {
          parts.push(block.text);
        }
      }
      content = parts.join("\n");
    }

    return { content, isError: res.isError === true };
  }

  close(): void {
    if (!this.alive) return;
    this.alive = false;

    for (const entry of this.pending.values()) {
      clearTimeout(entry.timer);
      entry.reject(this.error(`mcp ${this.serverName}: transport closed with requests in flight`));
    }
    this.pending.clear();

    this.child.removeAllListeners("error");
    this.child.removeAllListeners("close");
    this.child.stdout?.removeAllListeners();
    this.child.stderr?.removeAllListeners();

    this.child.stdin?.end();
    this.child.kill("SIGTERM");
    setTimeout(() => this.child.kill("SIGKILL"), KILL_GRACE_MS).unref();
  }

  // -------------------------------------------------------------------------

  private request(method: string, params: unknown, timeoutMs?: number): Promise<unknown> {
    const effectiveTimeout = timeoutMs ?? this.requestTimeoutMs;
    if (!this.alive) return Promise.reject(this.error(`mcp ${this.serverName}: transport not connected`));

    const id = this.nextId++;
    const { promise, resolve, reject } = Promise.withResolvers<unknown>();
    const entry: PendingEntry = {
      resolve,
      reject,
      timer: setTimeout(() => {
        this.pending.delete(id);
        reject(this.error(`mcp ${this.serverName}: request "${method}" timed out after ${effectiveTimeout}ms`));
      }, effectiveTimeout),
    };
    this.pending.set(id, entry);

    const frame = JSON.stringify({ jsonrpc: "2.0", id, method, params }) + "\n";
    this.child.stdin?.write(frame);
    return promise;
  }

  private notify(method: string, params: unknown = null): void {
    if (!this.alive) return;
    this.child.stdin?.write(JSON.stringify({ jsonrpc: "2.0", method, params }) + "\n");
  }

  private send(message: Record<string, unknown>): void {
    if (process.env.FORGE_MCP_DEBUG) {
      console.log(`[mcp-wire:${this.serverName}] send:`, JSON.stringify(message).slice(0, 200));
    }
    if (!this.alive) return;
    this.child.stdin?.write(JSON.stringify(message) + "\n");
  }

  private consumeStdout(chunk: string): void {
    if (process.env.FORGE_MCP_DEBUG) {
      console.log(`[mcp-wire:${this.serverName}] recv:`, JSON.stringify(chunk).slice(0, 300));
    }
    this.stdoutBuffer += chunk;
    for (;;) {
      const nl = this.stdoutBuffer.indexOf("\n");
      if (nl < 0) break;
      const line = this.stdoutBuffer.slice(0, nl).trim();
      this.stdoutBuffer = this.stdoutBuffer.slice(nl + 1);
      if (line !== "") this.handleFrame(line);
    }
  }

  private handleFrame(line: string): void {
    let msg: unknown;
    try {
      msg = JSON.parse(line);
    } catch {
      return; // skip malformed frames
    }
    if (!isRecord(msg)) return;

    const method = typeof msg.method === "string" ? msg.method : undefined;
    if (method !== undefined) {
      // Message FROM the server: request when it carries an id, notification otherwise.
      if (msg.id !== undefined && msg.id !== null) {
        if (method === "ping") {
          this.send({ jsonrpc: "2.0", id: msg.id, result: {} });
        } else {
          this.send({
            jsonrpc: "2.0",
            id: msg.id,
            error: { code: -32601, message: `method not found: ${method}` },
          });
        }
      }
      return;
    }

    // Response to one of our requests.
    if (typeof msg.id !== "number") return;
    const entry = this.pending.get(msg.id);
    if (entry === undefined) return;
    this.pending.delete(msg.id);
    clearTimeout(entry.timer);

    const rpcError = asJsonRpcError(msg.error);
    if (rpcError !== null) {
      entry.reject(this.error(`mcp ${this.serverName}: error ${rpcError.code}: ${rpcError.message}`));
    } else {
      entry.resolve(msg.result);
    }
  }

  private handleChildDown(): void {
    if (!this.alive) return;
    this.alive = false;
    for (const [id, entry] of this.pending) {
      clearTimeout(entry.timer);
      entry.reject(this.error(`mcp ${this.serverName}: server exited with pending request id ${id}`));
    }
    this.pending.clear();
  }

  private error(message: string): Error {
    const tail = this.stderrTail.trim();
    return new Error(tail === "" ? message : `${message} (server "${this.serverName}" stderr: ${tail})`);
  }
}
