import fs from "fs";
import path from "path";
import os from "os";

export interface MCPServerConfig {
  name: string;
  command: string;
  args: string[];
  env?: Record<string, string>;
  enabled: boolean;
}

export interface MCPTool {
  name: string;
  description: string;
  server: string;
  parameters?: any;
}

export class MCPManager {
  private dataDir: string;
  private configFile: string;
  private servers: MCPServerConfig[] = [];

  constructor(dataDir?: string) {
    this.dataDir = dataDir || path.join(os.homedir(), ".forge-ade");
    this.configFile = path.join(this.dataDir, "mcp_servers.json");
    this.loadServers();
  }

  private loadServers(): void {
    try {
      if (fs.existsSync(this.configFile)) {
        this.servers = JSON.parse(fs.readFileSync(this.configFile, "utf-8"));
      }
    } catch {
      this.servers = [];
    }
  }

  private saveServers(): void {
    try {
      fs.writeFileSync(this.configFile, JSON.stringify(this.servers, null, 2), "utf-8");
    } catch (err) {
      console.error("Failed to save MCP servers:", err);
    }
  }

  public listServers(): MCPServerConfig[] {
    return [...this.servers];
  }

  public saveServer(server: MCPServerConfig): MCPServerConfig {
    const existingIndex = this.servers.findIndex((s) => s.name === server.name);
    if (existingIndex >= 0) {
      this.servers[existingIndex] = server;
    } else {
      this.servers.push(server);
    }
    this.saveServers();
    return server;
  }

  public deleteServer(name: string): void {
    this.servers = this.servers.filter((s) => s.name !== name);
    this.saveServers();
  }

  public listTools(): MCPTool[] {
    return [
      { name: "read_file", description: "Read a file from disk", server: "filesystem" },
      { name: "write_file", description: "Write content to a file", server: "filesystem" },
      { name: "web_search", description: "Search the web for information", server: "search" },
    ];
  }

  public listConnectedTools(): MCPTool[] {
    return this.listTools();
  }

  public reconnect(): void {}
}
