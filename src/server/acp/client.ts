// Minimal ACP (Agent Client Protocol) client connection.
//
// Speaks newline-delimited JSON-RPC 2.0 over a spawned agent process's stdio:
//   client → agent : initialize, session/new, session/prompt, session/cancel
//   agent → client : session/update (notification), session/request_permission,
//                    fs/read_text_file, fs/write_text_file
//
// Protocol reference: https://agentclientprotocol.com/protocol/v1/schema

import { spawn, type ChildProcess } from "child_process";
import path from "path";
import os from "os";
import fs from "fs";
import type { ExternalAgentDef } from "./registry";

const PROTOCOL_VERSION = 1;
const REQUEST_TIMEOUT_MS = 30_000;
const INITIALIZE_TIMEOUT_MS = 20_000;

interface JsonRpcMsg {
  jsonrpc?: "2.0";
  id?: number | string;
  method?: string;
  params?: unknown;
  result?: unknown;
  error?: { code: number; message: string; data?: unknown };
}

type ClientRequestHandler = (params: any) => Promise<unknown>;

export interface AcpConnectionCallbacks {
  /** session/update notification for an ACP session. */
  onUpdate?: (acpSessionId: string, update: any) => void;
  /** Emitted whenever config options or available commands change. */
  onSessionState?: (acpSessionId: string, state: { configOptions: any[]; availableCommands: any[] }) => void;
  onExit?: (code: number | null) => void;
  onError?: (message: string) => void;
}

export class AcpConnection {
  readonly acpSessionIds = new Set<string>();
  private proc: ChildProcess | null = null;
  private nextId = 1;
  private pending = new Map<number, { resolve: (v: unknown) => void; reject: (e: Error) => void }>();
  private handlers = new Map<string, ClientRequestHandler>();
  private buffer = "";
  private exited = false;
  agentInfo?: { name: string; version?: string };

  constructor(
    private def: ExternalAgentDef,
    private cwd: string,
    private callbacks: AcpConnectionCallbacks = {},
  ) {}


  /** Spawns the process and performs the initialize handshake. */
  async start(): Promise<void> {
    // GUI-launched daemons often carry a minimal PATH that misses user
    // installs (nvm, ~/.local/bin, homebrew). Augment before spawning.
    const extraDirs = [
      "/opt/homebrew/bin",
      "/usr/local/bin",
      path.join(os.homedir(), ".local/bin"),
      path.join(os.homedir(), ".bun/bin"),
      path.dirname(process.execPath),
    ];
    const pathEnv = [process.env.PATH || "/usr/bin:/bin:/usr/sbin:/sbin", ...extraDirs]
      .flatMap((d) => d.split(":"))
      .filter((d, i, all) => d && all.indexOf(d) === i)
      .join(":");
    const proc = spawn(this.def.command, this.def.args, {
      cwd: this.cwd || undefined,
      env: { ...process.env, PATH: pathEnv },
      stdio: ["pipe", "pipe", "pipe"],
    });
    this.proc = proc;
    proc.on("error", (err: Error) => {
      this.callbacks.onError?.(`${this.def.command} failed to launch: ${err.message}`);
      this.rejectAll(new Error(`agent process error: ${err.message}`));
    });
    proc.on("exit", (code) => {
      this.exited = true;
      this.callbacks.onExit?.(code);
      this.rejectAll(new Error(`agent process exited (code ${code ?? "?"})`));
    });
    proc.stdout!.on("data", (chunk: Buffer) => this.feed(chunk.toString("utf-8")));
    proc.stderr!.on("data", (chunk: Buffer) => {
      const text = chunk.toString("utf-8").trim();
      if (text) console.error(`[acp:${this.def.id}] ${text}`);
    });

    // Default client-side handlers.
    this.handle("fs/read_text_file", async (params) => {
      const abs = path.isAbsolute(params.path) ? params.path : path.join(this.cwd, params.path);
      return { content: fs.readFileSync(abs, "utf-8") };
    });
    this.handle("fs/write_text_file", async (params) => {
      const abs = path.isAbsolute(params.path) ? params.path : path.join(this.cwd, params.path);
      fs.mkdirSync(path.dirname(abs), { recursive: true });
      fs.writeFileSync(abs, String(params.content ?? ""), "utf-8");
      return {};
    });
    // Permission requests: v1 policy is to allow the first non-reject option
    // once. A dedicated approval UI can replace this handler later.
    this.handle("session/request_permission", async (params) => {
      const options: any[] = Array.isArray(params.options) ? params.options : [];
      const allow = options.find((o) => o.kind === "allow_once")
        || options.find((o) => o.kind === "allow_always");
      const chosen = allow || options.find((o) => o.kind !== "reject_once" && o.kind !== "reject_always");
      if (chosen?.optionId != null) {
        return { outcome: { outcome: "selected", optionId: chosen.optionId } };
      }
      return { outcome: { outcome: "cancelled" } };
    });
    this.handle("session/set_mode", async () => ({}));

    const res = (await this.request(
      "initialize",
      {
        protocolVersion: PROTOCOL_VERSION,
        clientCapabilities: {
          fs: { readTextFile: true, writeTextFile: true },
          terminal: false,
          session: { configOptions: { boolean: {} } },
        },
        clientInfo: { name: "ForgeADE", version: "0.1.0" },
      },
      INITIALIZE_TIMEOUT_MS,
    )) as any;
    if (typeof res?.protocolVersion === "number" && res.protocolVersion > PROTOCOL_VERSION) {
      throw new Error(`agent requires unsupported ACP protocol version ${res.protocolVersion}`);
    }
    if (res?.agentInfo?.name) this.agentInfo = res.agentInfo;
  }

  /** Registers a handler for a client-side (agent → client) request method. */
  handle(method: string, fn: ClientRequestHandler): void {
    this.handlers.set(method, fn);
  }

  /** Normalized per-session UI state: config options (model/mode/thinking/…)
   *  and advertised slash commands. */
  private sessionState = new Map<string, { configOptions: any[]; availableCommands: any[] }>();

  getState(acpSessionId: string): { configOptions: any[]; availableCommands: any[] } {
    return this.sessionState.get(acpSessionId) ?? { configOptions: [], availableCommands: [] };
  }

  async newSession(cwd: string): Promise<string> {
    const res = (await this.request("session/new", { cwd: cwd || this.cwd, mcpServers: [] })) as any;
    const sessionId = res?.sessionId;
    if (!sessionId) throw new Error("agent returned no sessionId from session/new");
    this.acpSessionIds.add(sessionId);
    // Normalize: config options are preferred; legacy `modes` become a
    // synthetic `mode` select when the agent didn't send config options.
    let configOptions: any[] = Array.isArray(res.configOptions) ? res.configOptions : [];
    if (!configOptions.some((c) => c.category === "mode") && res.modes?.availableModes?.length) {
      configOptions = [
        {
          id: "mode",
          name: "Mode",
          category: "mode",
          type: "select",
          currentValue: res.modes.currentModeId,
          options: res.modes.availableModes.map((m: any) => ({
            value: m.id,
            name: m.name,
            description: m.description,
          })),
        },
        ...configOptions,
      ];
      this.legacyModes.set(sessionId, true);
    }
    this.sessionState.set(sessionId, { configOptions, availableCommands: [] });
    return sessionId;
  }

  /**
   * Sets a config option value. Falls back to legacy session/set_mode for
   * agents that only expose modes. Resolves with the refreshed full state.
   */
  async setConfigOption(acpSessionId: string, configId: string, value: string | boolean): Promise<{ configOptions: any[]; availableCommands: any[] }> {
    const useLegacy = this.legacyModes.get(acpSessionId) === true;
    if (useLegacy) {
      await this.request("session/set_mode", { sessionId: acpSessionId, modeId: String(value) });
      const state = this.getState(acpSessionId);
      const modeOpt = state.configOptions.find((c) => c.category === "mode" || c.id === "mode");
      if (modeOpt) modeOpt.currentValue = String(value);
      this.callbacks.onSessionState?.(acpSessionId, state);
      return state;
    }
    const opt = this.getState(acpSessionId).configOptions.find((c) => c.id === configId);
    const params: Record<string, unknown> = { sessionId: acpSessionId, configId, value };
    if (opt?.type === "boolean") params.type = "boolean";
    const res = (await this.request("session/set_config_option", params)) as any;
    if (Array.isArray(res?.configOptions)) {
      const state = this.getState(acpSessionId);
      state.configOptions = res.configOptions;
      this.callbacks.onSessionState?.(acpSessionId, state);
    }
    return this.getState(acpSessionId);
  }

  /** Runs one prompt turn; resolves with stopReason + token usage when the
   *  turn ends (usage present when the agent reports it). */
  async prompt(
    acpSessionId: string,
    text: string,
  ): Promise<{
    stopReason?: string;
    usage?: { inputTokens: number; outputTokens: number; totalTokens: number; cachedReadTokens: number };
  }> {
    const res = (await this.request(
      "session/prompt",
      { sessionId: acpSessionId, prompt: [{ type: "text", text }] },
      0,
    )) as any;
    return {
      stopReason: res?.stopReason,
      usage: res?.usage && typeof res.usage === "object" ? res.usage : undefined,
    };
  }

  cancel(acpSessionId: string): void {
    this.notify("session/cancel", { sessionId: acpSessionId });
  }

  /** True for agents whose "mode"-like options only exist via legacy modes. */
  private legacyModes = new Map<string, boolean>();

  close(): void {
    if (!this.proc || this.exited) return;
    try {
      this.proc.stdin?.end();
      this.proc.kill();
      // Hard fallback so hung agents don't leak.
      const p = this.proc;
      const t = setTimeout(() => p.kill("SIGKILL"), 3000);
      t.unref?.();
    } catch {}
  }

  // -- transport --------------------------------------------------------------

  private feed(text: string): void {
    this.buffer += text;
    let idx: number;
    while ((idx = this.buffer.indexOf("\n")) >= 0) {
      const line = this.buffer.slice(0, idx).trim();
      this.buffer = this.buffer.slice(idx + 1);
      if (!line) continue;
      let msg: JsonRpcMsg;
      try {
        msg = JSON.parse(line);
      } catch {
        console.error(`[acp:${this.def.id}] unparseable line: ${line.slice(0, 200)}`);
        continue;
      }
      this.dispatch(msg);
    }
  }

  private dispatch(msg: JsonRpcMsg): void {
    if (msg.id != null && msg.method == null) {
      const entry = this.pending.get(Number(msg.id));
      if (!entry) return;
      this.pending.delete(Number(msg.id));
      if (msg.error) entry.reject(new Error(msg.error.message || JSON.stringify(msg.error)));
      else entry.resolve(msg.result);
      return;
    }
    if (msg.method != null && msg.id != null) {
      // Agent → client request: must be answered.
      const handler = this.handlers.get(msg.method);
      Promise.resolve()
        .then(() => (handler ? handler(msg.params) : Promise.reject(new Error(`method not supported: ${msg.method}`))))
        .then(
          (result) => this.send({ jsonrpc: "2.0", id: msg.id, result: result ?? {} }),
          (err) =>
            this.send({
              jsonrpc: "2.0",
              id: msg.id,
              error: { code: -32603, message: err instanceof Error ? err.message : String(err) },
            }),
        );
      return;
    }
    // Notification.
    if (msg.method === "session/update") {
      const p = msg.params as any;
      if (!p?.sessionId || !p.update) return;
      const update = p.update;
      switch (update.sessionUpdate) {
        case "available_commands_update":
          this.applyState(p.sessionId, (s) => {
            s.availableCommands = Array.isArray(update.availableCommands) ? update.availableCommands : [];
          });
          break;
        case "config_option_update":
          this.applyState(p.sessionId, (s) => {
            s.configOptions = Array.isArray(update.configOptions) ? update.configOptions : [];
          });
          break;
        case "current_mode_update":
          this.applyState(p.sessionId, (s) => {
            const modeOpt = s.configOptions.find((c) => c.category === "mode" || c.id === "mode");
            if (modeOpt) modeOpt.currentValue = String(update.modeId ?? "");
          });
          break;
      }
      this.callbacks.onUpdate?.(p.sessionId, update);
    }
  }

  private applyState(acpSessionId: string, mutate: (s: { configOptions: any[]; availableCommands: any[] }) => void): void {
    const state = this.sessionState.get(acpSessionId);
    if (!state) return;
    mutate(state);
    this.callbacks.onSessionState?.(acpSessionId, state);
  }

  private send(msg: object): void {
    if (!this.proc?.stdin?.writable) return;
    this.proc.stdin.write(JSON.stringify(msg) + "\n");
  }

  private notify(method: string, params: unknown): void {
    this.send({ jsonrpc: "2.0", method, params });
  }

  private request(method: string, params: unknown, timeoutMs = REQUEST_TIMEOUT_MS): Promise<unknown> {
    if (!this.proc || this.exited) return Promise.reject(new Error("agent process is not running"));
    const id = this.nextId++;
    const { promise, resolve, reject } = Promise.withResolvers<unknown>();
    let timer: ReturnType<typeof setTimeout> | null = null;
    if (timeoutMs > 0) {
      timer = setTimeout(() => {
        this.pending.delete(id);
        reject(new Error(`${method} timed out after ${timeoutMs}ms`));
      }, timeoutMs);
      timer.unref?.();
    }
    this.pending.set(id, {
      resolve: (v) => {
        clearTimeout(timer!);
        resolve(v);
      },
      reject: (e) => {
        clearTimeout(timer!);
        reject(e);
      },
    });
    this.send({ jsonrpc: "2.0", id, method, params });
    return promise;
  }

  private rejectAll(err: Error): void {
    for (const [, entry] of this.pending) entry.reject(err);
    this.pending.clear();
  }
}
