// Multi-source MCP server config discovery — ported from the reference
// discovery layer. Aggregates server maps from every major agent tool's
// conventional config location (including opencode's "mcp" format with
// command arrays), expands env vars, rebases relative paths against the
// declaring config's directory, and dedupes by server name.

import fs from "fs";
import path from "path";
import os from "os";

export interface McpServerConfig {
  name: string;
  /** stdio transport */
  command?: string | undefined;
  args?: string[] | undefined;
  env?: Record<string, string> | undefined;
  cwd?: string | undefined;
  /** http/sse transport */
  url?: string | undefined;
  enabled: boolean;
  source: string;
}

export interface McpConfigResult {
  servers: McpServerConfig[];
  warnings: string[];
}

interface RawServerRecord {
  command?: unknown;
  args?: unknown;
  env?: unknown;
  environment?: unknown;
  cwd?: unknown;
  url?: unknown;
  type?: unknown;
  enabled?: unknown;
  disabled?: unknown;
}

// ---------------------------------------------------------------------------
// Shared helpers
// ---------------------------------------------------------------------------

/** ${VAR} and $VAR expansion against process.env; missing vars become "". */
export function expandEnvVarsDeep(value: unknown): unknown {
  if (typeof value === "string") {
    return value.replace(/\$\{([A-Z0-9_]+)\}|\$([A-Z0-9_]+)/g, (_all, braced: string | undefined, bare: string | undefined) => {
      const name = braced ?? bare ?? "";
      return process.env[name] ?? "";
    });
  }
  if (Array.isArray(value)) return value.map((v) => expandEnvVarsDeep(v));
  if (value && typeof value === "object") {
    const out: Record<string, unknown> = {};
    for (const [k, v] of Object.entries(value as Record<string, unknown>)) {
      out[k] = expandEnvVarsDeep(v);
    }
    return out;
  }
  return value;
}

function readJson(filePath: string): Record<string, unknown> | null {
  try {
    const parsed: unknown = JSON.parse(fs.readFileSync(filePath, "utf-8"));
    return parsed && typeof parsed === "object" ? (parsed as Record<string, unknown>) : null;
  } catch {
    return null;
  }
}

/** Reads a JSONC file: strips // and block comments plus trailing commas. */
function readJsonc(filePath: string): Record<string, unknown> | null {
  let text: string;
  try {
    text = fs.readFileSync(filePath, "utf-8");
  } catch {
    return null;
  }
  let out = "";
  let inStr = false;
  let esc = false;
  for (let i = 0; i < text.length; i++) {
    const c = text[i];
    if (inStr) {
      out += c;
      if (esc) esc = false;
      else if (c === "\\") esc = true;
      else if (c === '"') inStr = false;
      continue;
    }
    if (c === '"') {
      inStr = true;
      out += c;
      continue;
    }
    if (c === "/" && text[i + 1] === "/") {
      while (i < text.length && text[i] !== "\n") i++;
      out += "\n";
      continue;
    }
    if (c === "/" && text[i + 1] === "*") {
      i += 2;
      while (i < text.length && !(text[i] === "*" && text[i + 1] === "/")) i++;
      i += 1;
      continue;
    }
    out += c;
  }
  try {
    const parsed: unknown = JSON.parse(out.replace(/,\s*([}\]])/g, "$1"));
    return parsed && typeof parsed === "object" ? (parsed as Record<string, unknown>) : null;
  } catch {
    return null;
  }
}

function isHttpServer(rec: RawServerRecord): boolean {
  return typeof rec.url === "string" && rec.url.length > 0;
}

/** Extracts a normalized config from one raw mcpServers entry. */
function normalizeServer(
  name: string,
  rawRaw: unknown,
  source: string,
  configDir: string,
): McpServerConfig | null {
  if (!rawRaw || typeof rawRaw !== "object") return null;
  const raw = rawRaw as RawServerRecord;
  let enabled = true;
  if (typeof raw.enabled === "boolean") enabled = raw.enabled;
  if (raw.enabled === undefined && typeof raw.disabled === "boolean") enabled = !raw.disabled;

  const env: Record<string, string> = {};
  for (const envSource of [raw.env, raw.environment]) {
    if (envSource && typeof envSource === "object") {
      for (const [k, v] of Object.entries(envSource as Record<string, unknown>)) {
        if (typeof v === "string") env[k] = v;
      }
    }
  }
  const args = Array.isArray(raw.args)
    ? raw.args.filter((a): a is string => typeof a === "string")
    : [];

  const base: McpServerConfig = { name, enabled, source };
  if (typeof raw.url === "string" && raw.url) {
    return { ...base, url: raw.url };
  }
  if (typeof raw.command !== "string" || raw.command.length === 0) return null;
  const command = path.isAbsolute(raw.command) ? raw.command : path.resolve(configDir, raw.command);
  const cwd = typeof raw.cwd === "string" ? raw.cwd : undefined;
  const resolvedCwd = cwd ? (path.isAbsolute(cwd) ? cwd : path.resolve(configDir, cwd)) : configDir;
  return { ...base, command, args, env, cwd: resolvedCwd };
}

function collectFromJson(
  filePath: string,
  source: string,
  servers: Map<string, McpServerConfig>,
  warnings: string[],
): void {
  const parsed = readJson(filePath);
  if (!parsed) return;
  const dir = path.dirname(filePath);
  let map: unknown = parsed.mcpServers;
  if (map === undefined && !("mcpServers" in parsed)) map = parsed; // flat shape
  if (!map || typeof map !== "object" || Array.isArray(map)) return;
  const expanded = expandEnvVarsDeep(map) as Record<string, unknown>;
  for (const [name, cfg] of Object.entries(expanded)) {
    const normalized = normalizeServer(name, cfg, source, dir);
    if (!normalized) {
      warnings.push(`${source}: "${name}" has no command or url, skipped`);
      continue;
    }
    if (!servers.has(name)) servers.set(name, normalized);
  }
}

// ---------------------------------------------------------------------------
// Codex TOML subset: [mcp_servers.<name>] with command/args/env.
// ---------------------------------------------------------------------------

export function parseCodexMcpServers(toml: string): Record<string, RawServerRecord> {
  const result: Record<string, RawServerRecord> = {};
  let current: string | null = null;

  for (const rawLine of toml.split("\n")) {
    const line = rawLine.trim();
    if (line.length === 0 || line.startsWith("#")) continue;

    const sectionMatch =
      /^\[mcp_servers\.?"?([^"\]]+)"?\]$/.exec(line) ?? /^\[mcp_servers\."([^"]+)"\]$/.exec(line);
    if (sectionMatch) {
      current = sectionMatch[1];
      result[current] = {};
      continue;
    }
    if (/^\[/.test(line)) {
      current = null; // some other section
      continue;
    }
    if (!current) continue;
    const kv = /^([A-Za-z_]+)\s*=\s*(.+)$/.exec(line);
    if (!kv) continue;
    const key = kv[1];
    const record = result[current];
    if (key === "env") {
      // env is an inline table on one line: { KEY = "val", ... }
      if (record) record.env = parseInlineTable(kv[2].trim());
      continue;
    }
    const value = parseTomlValue(kv[2].trim());
    if (record) {
      record[key as keyof RawServerRecord] = value;
    }
  }
  return result;
}

function parseTomlValue(text: string): unknown {
  const t = text.trim();
  if (t.startsWith("{")) return parseInlineTable(t);
  if (t.startsWith("[")) {
    const inner = t.slice(1, t.lastIndexOf("]"));
    return splitTopLevel(inner).map((item) => parseTomlValue(item));
  }
  if ((t.startsWith('"') && t.endsWith('"')) || (t.startsWith("'") && t.endsWith("'"))) {
    return t.slice(1, -1);
  }
  if (t === "true") return true;
  if (t === "false") return false;
  return t;
}

function splitTopLevel(input: string): string[] {
  const parts: string[] = [];
  let depth = 0;
  let quote: string | null = null;
  let current = "";
  for (const ch of input) {
    if (quote) {
      current += ch;
      if (ch === quote) quote = null;
      continue;
    }
    if (ch === '"' || ch === "'") {
      quote = ch;
      current += ch;
      continue;
    }
    if (ch === "[" || ch === "{") depth++;
    if (ch === "]" || ch === "}") depth--;
    if (ch === "," && depth === 0) {
      parts.push(current.trim());
      current = "";
      continue;
    }
    current += ch;
  }
  if (current.trim()) parts.push(current.trim());
  return parts.filter((p) => p.length > 0);
}

function parseInlineTable(text: string): Record<string, string> {
  const inner = text.replace(/^\{/, "").replace(/\}$/, "").trim();
  const out: Record<string, string> = {};
  for (const pair of splitTopLevel(inner)) {
    const idx = pair.indexOf("=");
    if (idx <= 0) continue;
    const key = pair.slice(0, idx).trim().replace(/^"|"$/g, "");
    const value = parseTomlValue(pair.slice(idx + 1).trim());
    if (typeof value === "string" || typeof value === "number" || typeof value === "boolean") {
      out[key] = String(value);
    }
  }
  return out;
}

function collectFromCodexToml(
  filePath: string,
  source: string,
  servers: Map<string, McpServerConfig>,
): void {
  let toml: string;
  try {
    toml = fs.readFileSync(filePath, "utf-8");
  } catch {
    return;
  }
  const dir = path.dirname(filePath);
  const expanded = expandEnvVarsDeep(parseCodexMcpServers(toml)) as Record<string, RawServerRecord>;
  for (const [name, cfg] of Object.entries(expanded)) {
    const normalized = normalizeServer(name, cfg, source, dir);
    if (normalized && !servers.has(name)) servers.set(name, normalized);
  }
}

// ---------------------------------------------------------------------------
// opencode format: opencode(.jsonc|.json) → "mcp": { name: {...} }
//   local:  { type: "local", command: ["npx", "-y", "pkg"], environment: {} }
//   remote: { type: "remote", url: "https://..." }
// ---------------------------------------------------------------------------

function collectFromOpencode(
  filePath: string,
  source: string,
  servers: Map<string, McpServerConfig>,
): void {
  const parsed = readJsonc(filePath);
  if (!parsed) return;
  const map = parsed.mcp;
  if (!map || typeof map !== "object" || Array.isArray(map)) return;
  for (const [name, rawRaw] of Object.entries(map as Record<string, unknown>)) {
    if (!rawRaw || typeof rawRaw !== "object") continue;
    const rec = rawRaw as RawServerRecord;
    let enabled = true;
    if (typeof rec.enabled === "boolean") enabled = rec.enabled;
    const env: Record<string, string> = {};
    for (const envSource of [rec.environment, rec.env]) {
      if (envSource && typeof envSource === "object") {
        for (const [k, v] of Object.entries(envSource as Record<string, unknown>)) {
          if (typeof v === "string") env[k] = v;
        }
      }
    }
    const base: McpServerConfig = { name, enabled, source };
    if (typeof rec.url === "string" && rec.url) {
      if (!servers.has(name)) servers.set(name, { ...base, url: rec.url });
      continue;
    }
    if (Array.isArray(rec.command)) {
      const parts = rec.command.filter((c): c is string => typeof c === "string");
      if (parts.length === 0) continue;
      if (!servers.has(name)) {
        servers.set(name, { ...base, command: parts[0], args: parts.slice(1), env });
      }
    }
  }
}

// ---------------------------------------------------------------------------
// Source registry + discovery
// ---------------------------------------------------------------------------

type McpSource =
  | { kind: "json"; source: string; path: string }
  | { kind: "toml"; source: string; path: string }
  | { kind: "opencode"; source: string; path: string };

function jsonSource(source: string, filePath: () => string): McpSource {
  return { kind: "json", source, path: filePath() };
}

function tomlSource(source: string, filePath: () => string): McpSource {
  return { kind: "toml", source, path: filePath() };
}

function opencodeSource(source: string, filePath: () => string): McpSource {
  return { kind: "opencode", source, path: filePath() };
}

/** Every known MCP config location, priority order (first wins per name). */
export function mcpConfigSources(cwd: string): McpSource[] {
  const h = os.homedir();
  const projectFiles = ["mcp.json", ".mcp.json"];
  return [
    // Native (owned)
    ...projectFiles.map((f) => jsonSource("native:project", () => path.join(cwd, f))),
    jsonSource("native:user", () => path.join(h, ".forge-ade", "mcp.json")),
    jsonSource("native:project-omp", () => path.join(cwd, ".omp", "mcp.json")),
    jsonSource("native:user-omp", () => path.join(h, ".omp", "agent", "mcp.json")),
    // Claude Code
    jsonSource("claude:user", () => path.join(h, ".claude.json")),
    jsonSource("claude:user-dir", () => path.join(h, ".claude", "mcp.json")),
    jsonSource("claude:project", () => path.join(cwd, ".claude", "mcp.json")),
    jsonSource("claude:project-alt", () => path.join(cwd, ".claude", ".mcp.json")),
    // Codex CLI (TOML)
    tomlSource("codex:user", () => path.join(h, ".codex", "config.toml")),
    tomlSource("codex:project", () => path.join(cwd, ".codex", "config.toml")),
    // Cursor
    jsonSource("cursor:user", () => path.join(h, ".cursor", "mcp.json")),
    jsonSource("cursor:project", () => path.join(cwd, ".cursor", "mcp.json")),
    // Windsurf / Codeium
    jsonSource("windsurf:user", () => path.join(h, ".codeium", "windsurf", "mcp_config.json")),
    // Gemini CLI
    jsonSource("gemini:user", () => path.join(h, ".gemini", "settings.json")),
    jsonSource("gemini:project", () => path.join(cwd, ".gemini", "settings.json")),
    // opencode ("mcp" key, command arrays, JSONC-tolerant)
    opencodeSource("opencode:user", () => path.join(h, ".config", "opencode", "opencode.jsonc")),
    opencodeSource("opencode:user-json", () => path.join(h, ".config", "opencode", "opencode.json")),
    opencodeSource("opencode:project", () => path.join(cwd, ".opencode", "opencode.json")),
  ];
}

/** Discovers MCP server configs from every known location. */
export function discoverMcpServers(cwd: string): McpConfigResult {
  const warnings: string[] = [];
  const servers = new Map<string, McpServerConfig>();
  for (const src of mcpConfigSources(cwd)) {
    if (!fs.existsSync(src.path)) continue;
    if (src.kind === "json") {
      collectFromJson(src.path, src.source, servers, warnings);
    } else if (src.kind === "opencode") {
      collectFromOpencode(src.path, src.source, servers);
    } else {
      collectFromCodexToml(src.path, src.source, servers);
    }
  }
  return { servers: [...servers.values()], warnings };
}
