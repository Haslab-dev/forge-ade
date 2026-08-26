// MCP manager — discovers server configs from every supported tool's config
// files (native/Claude/Codex/Cursor/Windsurf/Gemini), connects stdio servers,
// and exposes their tools to the agent engine as mcp_<server>_<tool>.

import fs from "fs";
import path from "path";
import os from "os";
import { discoverMcpServers } from "./discovery/mcp-config";
import type { McpServerConfig } from "./discovery/mcp-config";
import { McpStdioClient } from "./mcp/client";
import { McpHttpClient } from "./mcp/http-client";
import type { McpCallResult } from "./mcp/http-client";
import type { McpToolDef } from "./mcp/client";

/** Shape shared by the stdio and HTTP transports. */
interface CommonClient {
  listTools(): Promise<McpToolDef[]>;
  callTool(name: string, args: Record<string, unknown>): Promise<McpCallResult>;
  close(): void;
}

export interface MCPServerConfig {
  name: string;
  command: string;
  args: string[];
  env?: Record<string, string> | undefined;
  enabled: boolean;
  source: string;
}

export interface MCPTool {
  /** Fully-qualified tool name used by the agent: mcp_<server>_<tool>. */
  name: string;
  description: string;
  server: string;
  parameters?: Record<string, unknown> | undefined;
}

export interface MCPServerStatus extends MCPServerConfig {
  connected: boolean;
  error?: string | undefined;
}

export interface McpReconnectResult {
  connected: string[];
  failed: string[];
}

export class MCPManager {
  private dataDir: string;
  private ownedConfigFile: string;
  private servers = new Map<string, McpServerConfig>();
  private clients = new Map<string, CommonClient>();
  private connecting = new Map<string, Promise<boolean>>();
  private toolsByServer = new Map<string, McpToolDef[]>();
  private connectErrors = new Map<string, string>();
  constructor(dataDir?: string) {
    this.dataDir = dataDir || path.join(os.homedir(), ".forge-ade");
    this.ownedConfigFile = path.join(this.dataDir, "mcp.json");
    void this.connectAll();
  }

  // ---------------------------------------------------------------------------
  // Config
  // ---------------------------------------------------------------------------

  /** Discovers configs from all known locations; earlier discovery order wins per name. */
  public refreshConfigs(projectFolder?: string): McpServerConfig[] {
    const { servers, warnings } = discoverMcpServers(projectFolder || process.cwd());
    for (const w of warnings) console.warn(`[mcp] ${w}`);
    this.servers.clear();
    for (const s of servers) this.servers.set(s.name, s);
    return [...this.servers.values()];
  }

  /** Lists discovered servers with connection state. */
  public listServers(): MCPServerStatus[] {
    return [...this.servers.values()].map((s) => ({
      name: s.name,
      command: s.command ?? "",
      args: s.args ?? [],
      env: s.env,
      enabled: s.enabled,
      source: s.source,
      connected: this.clients.has(s.name),
      ...(this.connectErrors.has(s.name) ? { error: this.connectErrors.get(s.name) } : {}),
    }));
  }

  /** Re-scans all discovery locations without touching live connections. */
  public refresh(projectFolder?: string): MCPServerStatus[] {
    this.refreshConfigs(projectFolder);
    return this.listServers();
  }

  /**
   * Persists a server into the app-owned config (~/.forge-ade/mcp.json).
   * Foreign tool configs are never mutated.
   */
  public saveServer(server: MCPServerConfig): MCPServerConfig {
    const config = readOwnedConfig(this.ownedConfigFile);
    config.mcpServers[server.name] = {
      command: server.command,
      args: server.args,
      ...(server.env ? { env: server.env } : {}),
      enabled: server.enabled,
    };
    writeOwnedConfig(this.ownedConfigFile, config);
    this.refreshConfigs();
    void this.connect(server.name);
    return server;
  }

  public deleteServer(name: string): void {
    const config = readOwnedConfig(this.ownedConfigFile);
    if (config.mcpServers[name]) {
      delete config.mcpServers[name];
      writeOwnedConfig(this.ownedConfigFile, config);
    }
    this.clients.get(name)?.close();
    this.clients.delete(name);
    this.toolsByServer.delete(name);
    this.refreshConfigs();
  }

  // ---------------------------------------------------------------------------
  // Connection + tools
  // ---------------------------------------------------------------------------

  /** Connects every enabled stdio server that isn't already connected. */
  public async connectAll(projectFolder?: string): Promise<void> {
    this.refreshConfigs(projectFolder);
    await Promise.allSettled(
      [...this.servers.values()].filter((s) => s.enabled && (s.command || s.url)).map((s) => this.connect(s.name)),
    );
  }

  private async connect(name: string): Promise<boolean> {
    const inFlight = this.connecting.get(name);
    if (inFlight) return inFlight;

    const promise = this.doConnect(name).finally(() => {
      this.connecting.delete(name);
    });
    this.connecting.set(name, promise);
    return promise;
  }

  private async doConnect(name: string): Promise<boolean> {
    const cfg = this.servers.get(name);
    if (!cfg || this.clients.has(name)) return this.clients.has(name);
    // HTTP transport (remote server)
    if (cfg.url) {
      try {
        const client = await McpHttpClient.connect({
          url: cfg.url,
          headers: cfg.headers,
          serverName: name,
          requestTimeoutMs: 30_000,
        });
        this.clients.set(name, client);
        this.toolsByServer.set(name, await client.listTools());
        this.connectErrors.delete(name);
        console.log(`[mcp] connected "${name}" over HTTP (${this.toolsByServer.get(name)?.length ?? 0} tools)`);
        return true;
      } catch (err) {
        const message = err instanceof Error ? err.message : String(err);
        this.connectErrors.set(name, message);
        console.error(`[mcp] connect failed for "${name}": ${message}`);
        return false;
      }
    }

    if (!cfg.command) return false;
    try {
      const client = await McpStdioClient.spawn({
        command: cfg.command,
        args: cfg.args,
        env: cfg.env,
        cwd: cfg.cwd,
        serverName: name,
        initTimeoutMs: 25_000,
      });
      this.clients.set(name, client);
      this.toolsByServer.set(name, await client.listTools());
      this.connectErrors.delete(name);
      console.log(`[mcp] connected "${name}" (${this.toolsByServer.get(name)?.length ?? 0} tools)`);
      return true;
    } catch (err) {
      const message = err instanceof Error ? err.message : String(err);
      this.connectErrors.set(name, message);
      console.error(`[mcp] connect failed for "${name}": ${message}`);
      return false;
    }
  }

  public async reconnect(): Promise<McpReconnectResult> {
    for (const client of this.clients.values()) client.close();
    this.clients.clear();
    this.toolsByServer.clear();
    this.connectErrors.clear();
    await this.connectAll();
    return {
      connected: [...this.clients.keys()],
      failed: [...this.connectErrors.keys()],
    };
  }

  /** Tools across all connected servers, namespaced mcp_<server>_<tool>. */
  public listConnectedTools(): MCPTool[] {
    const out: MCPTool[] = [];
    for (const [serverName, defs] of this.toolsByServer) {
      for (const def of defs) {
        out.push({
          name: qualifyTool(serverName, def.name),
          description: def.description ?? "",
          server: serverName,
          parameters: def.inputSchema,
        });
      }
    }
    return out;
  }

  /** Legacy stub surface kept for the settings UI. */
  public listTools(): MCPTool[] {
    return this.listConnectedTools();
  }

  /** Routes a qualified agent tool call to its server. */
  public async callQualifiedTool(qualifiedName: string, args: Record<string, unknown>): Promise<string> {
    const parsed = splitQualified(qualifiedName);
    if (!parsed) throw new Error(`invalid MCP tool name: ${qualifiedName}`);
    const [serverName, toolName] = parsed;
    let client = this.clients.get(serverName);
    if (!client) {
      const ok = await this.connect(serverName);
      if (!ok) throw new Error(`MCP server "${serverName}" is not connected`);
      client = this.clients.get(serverName)!;
    }
    const result = await client.callTool(toolName, args);
    return result.content;
  }

  /** True when the name was produced by qualifyTool. */
  public static isQualifiedToolName(name: string): boolean {
    if (!name.startsWith("mcp_")) return false;
    const rest = name.slice("mcp_".length);
    const sep = rest.indexOf("_");
    return sep > 0 && sep < rest.length - 1;
  }
}

function qualifyTool(server: string, tool: string): string {
  return `mcp_${server}_${tool}`;
}

function splitQualified(qualified: string): [string, string] | null {
  if (!qualified.startsWith("mcp_")) return null;
  const rest = qualified.slice("mcp_".length);
  const sep = rest.indexOf("_");
  if (sep <= 0 || sep === rest.length - 1) return null;
  return [rest.slice(0, sep), rest.slice(sep + 1)];
}

interface OwnedConfig {
  mcpServers: Record<string, unknown>;
}

function readOwnedConfig(file: string): OwnedConfig {
  try {
    const parsed: unknown = JSON.parse(fs.readFileSync(file, "utf-8"));
    if (parsed && typeof parsed === "object" && "mcpServers" in parsed) {
      const record = parsed as { mcpServers?: unknown };
      if (record.mcpServers && typeof record.mcpServers === "object") {
        return { mcpServers: record.mcpServers as Record<string, unknown> };
      }
    }
  } catch {}
  return { mcpServers: {} };
}

function writeOwnedConfig(file: string, config: OwnedConfig): void {
  fs.mkdirSync(path.dirname(file), { recursive: true });
  fs.writeFileSync(file, JSON.stringify({ mcpServers: config.mcpServers }, null, 2), "utf-8");
}
