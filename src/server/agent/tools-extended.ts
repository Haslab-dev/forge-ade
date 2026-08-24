import fs from "fs";
import os from "os";
import path from "path";
import { exec } from "child_process";
import { readCheckpoints } from "./checkpoints";
import { isGitIgnored, loadGitignores } from "../explorer";
import {
  fail,
  isTextFile,
  ok,
  resolveIn,
  truncate,
  walk,
  MAX_OUTPUT_CHARS,
  type ToolContext,
  type ToolHandler,
  type WalkAccumulator,
} from "./tools";
// Everything here is real implementation against local infrastructure — no
// stubs. Tools needing external services forge does not ship (browser,
// computer, hub) are intentionally absent.

export interface ExtendedToolOptions {
  dataDir: string;
  /** Lazily resolved LSP manager for the lsp tool; null disables it. */
  getLsp?: () => {
    getDiagnostics(filePath?: string): Record<
      string,
      { errors: number; warnings: number; diagnostics: unknown[] }
    >;
    searchIndexSymbols(query: string, folders: string[]): unknown[];
  } | null;
  getWorkspaceFolders?: () => string[];
}

// ---------------------------------------------------------------------------
// Memory store (~/.forge-ade/memory.jsonl)
// ---------------------------------------------------------------------------

interface MemoryEntry {
  ts: number;
  kind: string;
  text: string;
  tags?: string[];
}

function memoryFile(dataDir: string): string {
  return path.join(dataDir, "memory.jsonl");
}

function appendMemory(dataDir: string, entry: MemoryEntry): void {
  fs.mkdirSync(path.dirname(memoryFile(dataDir)), { recursive: true });
  fs.appendFileSync(memoryFile(dataDir), JSON.stringify(entry) + "\n", "utf-8");
}

function readMemory(dataDir: string): MemoryEntry[] {
  try {
    return fs
      .readFileSync(memoryFile(dataDir), "utf-8")
      .split("\n")
      .filter(Boolean)
      .map((l) => JSON.parse(l) as MemoryEntry);
  } catch {
    return [];
  }
}

// ---------------------------------------------------------------------------
// Skill roots (mirrors discovery/skills.ts user+project conventions)
// ---------------------------------------------------------------------------

function skillRoots(ctx: ToolContext): string[] {
  const home = os.homedir();
  const roots = [
    path.join(ctx.projectFolder, ".agents", "skills"),
    path.join(ctx.projectFolder, ".claude", "skills"),
    path.join(ctx.projectFolder, "skills"),
    path.join(home, ".forge-ade", "skills"),
    path.join(home, ".agents", "skills"),
    path.join(home, ".claude", "skills"),
  ];
  return [...new Set(roots)];
}

function findSkillDir(ctx: ToolContext, name: string): string | null {
  for (const root of skillRoots(ctx)) {
    const candidate = path.join(root, name);
    if (fs.existsSync(path.join(candidate, "SKILL.md"))) return candidate;
    // Also allow <root>/<name>.md single-file skills.
    if (fs.existsSync(path.join(root, `${name}.md`))) return path.join(root, `${name}.md`);
  }
  return null;
}

// ---------------------------------------------------------------------------
// Image header parsing (inspect_image metadata)
// ---------------------------------------------------------------------------

function imageMeta(buf: Buffer): { format: string; width?: number; height?: number } {
  if (buf.length >= 24 && buf.toString("ascii", 1, 4) === "PNG") {
    return { format: "png", width: buf.readUInt32BE(16), height: buf.readUInt32BE(20) };
  }
  if (buf.length > 10 && buf.toString("ascii", 0, 3) === "GIF") {
    return { format: "gif", width: buf.readUInt16LE(6), height: buf.readUInt16LE(8) };
  }
  if (buf.length > 12 && buf.toString("ascii", 0, 4) === "RIFF" && buf.toString("ascii", 8, 12) === "WEBP") {
    return { format: "webp" };
  }
  if (buf.length > 3 && buf[0] === 0xff && buf[1] === 0xd8) {
    // JPEG: scan SOF markers for dimensions.
    let off = 2;
    while (off + 9 < buf.length) {
      if (buf[off] !== 0xff) break;
      const marker = buf[off + 1];
      const len = buf.readUInt16BE(off + 2);
      if (marker >= 0xc0 && marker <= 0xcf && marker !== 0xc4 && marker !== 0xc8 && marker !== 0xcc) {
        return { format: "jpeg", height: buf.readUInt16BE(off + 5), width: buf.readUInt16BE(off + 7) };
      }
      off += 2 + len;
    }
    return { format: "jpeg" };
  }
  return { format: "unknown" };
}

// ---------------------------------------------------------------------------
// HTML → text stripping for web_fetch
// ---------------------------------------------------------------------------

function htmlToText(html: string): string {
  return html
    .replace(/<script[\s\S]*?<\/script>/gi, "")
    .replace(/<style[\s\S]*?<\/style>/gi, "")
    .replace(/<!--[\s\S]*?-->/g, "")
    .replace(/<\/(p|div|section|article|li|h[1-6]|tr)>/gi, "\n")
    .replace(/<(br|hr)\s*\/?>/gi, "\n")
    .replace(/<[^>]+>/g, "")
    .replace(/&nbsp;/g, " ")
    .replace(/&amp;/g, "&")
    .replace(/&lt;/g, "<")
    .replace(/&gt;/g, ">")
    .replace(/&quot;/g, '"')
    .replace(/&#39;/g, "'")
    .split("\n")
    .map((l) => l.trim())
    .filter((l) => l.length > 0)
    .join("\n");
}

// ---------------------------------------------------------------------------
// Registration
// ---------------------------------------------------------------------------

function def(name: string, description: string, parameters: Record<string, unknown>): ToolHandler["definition"] {
  return { type: "function", function: { name, description, parameters } };
}

function str(args: Record<string, unknown>, key: string): string {
  const v = args[key];
  return typeof v === "string" ? v : "";
}

function num(args: Record<string, unknown>, key: string): number | undefined {
  const v = args[key];
  return typeof v === "number" ? v : undefined;
}

export function registerExtendedTools(registry: Map<string, ToolHandler>, opts: ExtendedToolOptions): void {
  // -------------------------------------------------------------------------
  // web_fetch — URL → readable text (omp fetch parity: 300-line cap)
  // -------------------------------------------------------------------------
  registry.set("web_fetch", {
    definition: def(
      "web_fetch",
      "Fetch a URL and return its content as plain text (HTML stripped). Caps output at ~300 lines.",
      {
        type: "object",
        properties: {
          url: { type: "string", description: "Absolute http(s) URL" },
          max_lines: { type: "number", description: "Default 300" },
        },
        required: ["url"],
      }
    ),
    cost: "medium",
    mutating: false,
    async run(args) {
      const url = str(args, "url");
      if (!/^https?:\/\//i.test(url)) return fail("web_fetch failed: url must start with http:// or https://");
      const maxLines = num(args, "max_lines") ?? 300;
      try {
        const res = await fetch(url, { signal: AbortSignal.timeout(20_000), redirect: "follow" });
        if (!res.ok) return fail(`web_fetch failed: HTTP ${res.status} for ${url}`);
        const ctype = res.headers.get("content-type") ?? "";
        const raw = await res.text();
        let text = ctype.includes("html") ? htmlToText(raw) : raw;
        const lines = text.split("\n");
        if (lines.length > maxLines) text = lines.slice(0, maxLines).join("\n") + `\n... [${lines.length - maxLines} more lines]`;
        return ok(truncate(`[${res.status} ${ctype.split(";")[0]}] ${url}\n\n${text}`));
      } catch (err) {
        return fail(`web_fetch failed: ${(err as Error).message}`);
      }
    },
  });

  // -------------------------------------------------------------------------
  // eval — run a JS/TS snippet under Bun (omp eval parity: 30s cap)
  // -------------------------------------------------------------------------
  registry.set("eval", {
    definition: def(
      "eval",
      "Run a JavaScript/TypeScript snippet under Bun and capture stdout/stderr. Use console.log to emit results. 30s timeout.",
      {
        type: "object",
        properties: {
          code: { type: "string" },
          timeout_ms: { type: "number", description: "Default 30000, max 60000" },
        },
        required: ["code"],
      }
    ),
    cost: "medium",
    mutating: false,
    async run(args, ctx) {
      const code = str(args, "code");
      if (!code.trim()) return fail("eval failed: empty code");
      const timeoutMs = Math.min(num(args, "timeout_ms") ?? 30_000, 60_000);
      return new Promise((resolve) => {
        exec(
          `bun --eval ${JSON.stringify(code)}`,
          { cwd: ctx.projectFolder, timeout: timeoutMs, maxBuffer: 2_000_000, env: process.env },
          (error, stdout, stderr) => {
            const parts: string[] = [];
            if (stdout) parts.push(String(stdout));
            if (stderr) parts.push(`stderr:\n${String(stderr)}`);
            if (error && (error as { code?: number }).code !== 0) {
              parts.push(`exit: ${(error as { code?: number }).code ?? 1}`);
            }
            resolve(ok(truncate(parts.join("\n").trim() || "(no output)")));
          }
        );
      });
    },
  });

  // -------------------------------------------------------------------------
  // lsp — diagnostics and workspace symbol search via the LSP manager
  // -------------------------------------------------------------------------
  registry.set("lsp", {
    definition: def(
      "lsp",
      "Language-server intelligence. Actions: diagnostics {file?}, symbols {query}. Requires the file's language server to be running.",
      {
        type: "object",
        properties: {
          action: { type: "string", enum: ["diagnostics", "symbols"] },
          file: { type: "string", description: "For diagnostics: scope to one file" },
          query: { type: "string", description: "For symbols: name substring" },
        },
        required: ["action"],
      }
    ),
    cost: "cheap",
    mutating: false,
    async run(args, ctx) {
      const lsp = opts.getLsp?.() ?? null;
      if (!lsp) return fail("lsp failed: language server manager unavailable");
      const action = str(args, "action");
      if (action === "diagnostics") {
        const file = str(args, "file");
        const all = lsp.getDiagnostics(file ? resolveIn(ctx.projectFolder, file) : undefined);
        const entries = Object.entries(all);
        if (entries.length === 0) return ok("no diagnostics reported");
        const out = entries.map(([f, d]) => `${path.relative(ctx.projectFolder, f)}: ${d.errors}E/${d.warnings}W`);
        return ok(truncate(out.join("\n")));
      }
      if (action === "symbols") {
        const query = str(args, "query");
        if (!query) return fail("lsp failed: symbols requires query");
        const syms = lsp.searchIndexSymbols(query, [ctx.projectFolder]);
        return ok(truncate(JSON.stringify(syms, null, 1).slice(0, MAX_OUTPUT_CHARS)));
      }
      return fail(`lsp failed: unknown action "${action}"`);
    },
  });

  // -------------------------------------------------------------------------
  // memory — retain / recall / reflect over ~/.forge-ade/memory.jsonl
  // -------------------------------------------------------------------------
  registry.set("memory", {
    definition: def(
      "memory",
      "Persistent cross-session memory. Actions: retain {text, tags?}, recall {query, limit?}, reflect {} (summarize recent memories).",
      {
        type: "object",
        properties: {
          action: { type: "string", enum: ["retain", "recall", "reflect"] },
          text: { type: "string" },
          tags: { type: "array", items: { type: "string" } },
          query: { type: "string" },
          limit: { type: "number" },
        },
        required: ["action"],
      }
    ),
    cost: "cheap",
    mutating: true,
    async run(args, ctx) {
      const dataDir = ctx.dataDir ?? opts.dataDir;
      const action = str(args, "action");
      if (action === "retain") {
        const text = str(args, "text");
        if (!text.trim()) return fail("memory failed: retain requires text");
        const tags = Array.isArray(args.tags) ? args.tags.map(String) : undefined;
        appendMemory(dataDir, { ts: Date.now(), kind: "fact", text: text.slice(0, 2000), tags });
        return ok("retained.");
      }
      if (action === "recall") {
        const query = str(args, "query").toLowerCase();
        const limit = num(args, "limit") ?? 10;
        const all = readMemory(dataDir);
        const hits = (query ? all.filter((m) => m.text.toLowerCase().includes(query)) : all)
          .slice(-limit)
          .reverse()
          .map((m) => `${new Date(m.ts).toISOString()} [${m.kind}]${m.tags?.length ? ` #${m.tags.join(" #")}` : ""}: ${m.text}`);
        return ok(hits.length ? hits.join("\n") : "no matching memories");
      }
      if (action === "reflect") {
        const all = readMemory(dataDir);
        const recent = all.slice(-20).map((m) => `- [${new Date(m.ts).toISOString()}] ${m.text}`);
        return ok(recent.length ? `memory has ${all.length} entries; most recent 20:\n${recent.join("\n")}` : "memory is empty");
      }
      return fail(`memory failed: unknown action "${action}"`);
    },
  });

  // -------------------------------------------------------------------------
  // manage_skill — list/read/create/delete skills across standard roots
  // -------------------------------------------------------------------------
  registry.set("manage_skill", {
    definition: def(
      "manage_skill",
      "Author and inspect skills. Actions: list {}, read {name, file?}, create {name, description, body}, delete {name}. Skills live in SKILL.md under standard skill roots.",
      {
        type: "object",
        properties: {
          action: { type: "string", enum: ["list", "read", "create", "delete"] },
          name: { type: "string" },
          file: { type: "string", description: "Relative file inside the skill bundle (default SKILL.md)" },
          description: { type: "string" },
          body: { type: "string", description: "Markdown body for create" },
        },
        required: ["action"],
      }
    ),
    cost: "cheap",
    mutating: true,
    async run(args, ctx) {
      const action = str(args, "action");
      if (action === "list") {
        const out: string[] = [];
        for (const root of skillRoots(ctx)) {
          if (!fs.existsSync(root)) continue;
          for (const dir of fs.readdirSync(root, { withFileTypes: true })) {
            if (!dir.isDirectory()) continue;
            const skillMd = path.join(root, dir.name, "SKILL.md");
            if (!fs.existsSync(skillMd)) continue;
            const head = fs.readFileSync(skillMd, "utf-8").split("\n").slice(0, 8).find((l) => l.startsWith("description:"));
            out.push(`${dir.name} (${path.relative(ctx.projectFolder, root).startsWith("..") ? "user" : "project"}${head ? "" : ", no frontmatter"})`);
          }
        }
        return ok(out.length ? truncate(out.join("\n")) : "no skills found in standard roots");
      }
      const name = str(args, "name").replace(/[^a-zA-Z0-9._-]/g, "");
      if (!name) return fail("manage_skill failed: invalid name");
      if (action === "read") {
        const dir = findSkillDir(ctx, name);
        if (!dir) return fail(`manage_skill failed: skill "${name}" not found`);
        const relFile = str(args, "file") || (fs.statSync(dir).isDirectory() ? "SKILL.md" : "");
        const target = fs.statSync(dir).isDirectory() ? path.join(dir, relFile) : dir;
        if (!fs.existsSync(target)) return fail(`manage_skill failed: ${relFile} not found in skill`);
        return ok(truncate(fs.readFileSync(target, "utf-8")));
      }
      if (action === "create") {
        const description = str(args, "description");
        const body = str(args, "body");
        if (!body.trim()) return fail("manage_skill failed: create requires body");
        const root = path.join(ctx.projectFolder, ".agents", "skills", name);
        fs.mkdirSync(root, { recursive: true });
        const frontmatter = `---\nname: ${name}\ndescription: ${description || name}\n---\n\n`;
        fs.writeFileSync(path.join(root, "SKILL.md"), frontmatter + body + "\n", "utf-8");
        return ok(`created skill "${name}" at ${root}/SKILL.md`);
      }
      if (action === "delete") {
        const dir = findSkillDir(ctx, name);
        if (!dir) return fail(`manage_skill failed: skill "${name}" not found`);
        fs.rmSync(dir, { recursive: true, force: true });
        return ok(`deleted ${dir}`);
      }
      return fail(`manage_skill failed: unknown action "${action}"`);
    },
  });

  // -------------------------------------------------------------------------
  // security_scan — heuristic secret/dangerous-pattern scanner
  // -------------------------------------------------------------------------
  const SECRET_PATTERNS: Array<[string, RegExp]> = [
    ["private-key-block", /-----BEGIN [A-Z ]*PRIVATE KEY-----/],
    ["aws-access-key", /AKIA[0-9A-Z]{16}/],
    ["github-token", /gh[pousr]_[A-Za-z0-9]{36,}/],
    ["slack-token", /xox[baprs]-[A-Za-z0-9-]{10,}/],
    ["google-api-key", /AIza[0-9A-Za-z_-]{35}/],
    ["generic-api-key", /\b(api[_-]?key|apikey|secret|password|passwd|token)\b\s*[:=]\s*['"][^'"\s]{12,}['"]/i],
    ["connection-string", /\b(mongodb(?:\+srv)?|postgres(?:ql)?|mysql|redis):\/\/[^\s'"]+:[^\s'"]+@/],
  ];

  registry.set("security_scan", {
    definition: def(
      "security_scan",
      "Scan a file or directory tree for likely hardcoded secrets (API keys, tokens, private keys, credentials). Read-only.",
      {
        type: "object",
        properties: {
          target: { type: "string", description: "File or directory (default workspace root)" },
          max_findings: { type: "number", description: "Default 50" },
        },
      }
    ),
    cost: "medium",
    mutating: false,
    async run(args, ctx) {
      const target = resolveIn(ctx.projectFolder, str(args, "target") || ".");
      const maxFindings = num(args, "max_findings") ?? 50;
      if (!fs.existsSync(target)) return fail(`security_scan failed: ${target} not found`);
      const findings: string[] = [];
      const scanFile = (abs: string, rel: string): boolean => {
        if (!isTextFile(abs)) return true;
        let content: string;
        try {
          if (fs.statSync(abs).size > 500_000) return true;
          content = fs.readFileSync(abs, "utf-8");
        } catch {
          return true;
        }
        const lines = content.split("\n");
        for (let i = 0; i < lines.length && findings.length < maxFindings; i++) {
          for (const [kind, re] of SECRET_PATTERNS) {
            if (re.test(lines[i])) {
              findings.push(`${kind} · ${rel}:${i + 1}: ${lines[i].trim().slice(0, 120)}`);
              break;
            }
          }
        }
        return findings.length < maxFindings;
      };
      const stat = fs.statSync(target);
      if (stat.isFile()) {
        scanFile(target, path.basename(target));
      } else {
        const gi = loadGitignores(ctx.projectFolder);
        const acc: WalkAccumulator = { files: [], budget: 20_000 };
        walk(target, "", acc, gi);
        for (const rel of acc.files) {
          if (!scanFile(path.join(target, rel), rel)) break;
        }
      }
      return ok(findings.length ? truncate(findings.join("\n")) : "no secret-pattern findings");
    },
  });

  // -------------------------------------------------------------------------
  // inspect_image — structural metadata (dimensions/format); visual
  // inspection needs a multimodal model surface forge's text pipeline lacks.
  // -------------------------------------------------------------------------
  registry.set("inspect_image", {
    definition: def(
      "inspect_image",
      "Return structural metadata about an image (format, dimensions, size). Does not describe visual content.",
      {
        type: "object",
        properties: { path: { type: "string" } },
        required: ["path"],
      }
    ),
    cost: "cheap",
    mutating: false,
    async run(args, ctx) {
      const p = resolveIn(ctx.projectFolder, str(args, "path"));
      try {
        const buf = fs.readFileSync(p);
        const meta = imageMeta(buf);
        const dims = meta.width ? ` ${meta.width}x${meta.height}` : "";
        return ok(`${path.basename(p)}: ${meta.format}${dims}, ${buf.length} bytes. Visual content description is unavailable in this text-only pipeline.`);
      } catch (err) {
        return fail(`inspect_image failed: ${(err as Error).message}`);
      }
    },
  });

  // -------------------------------------------------------------------------
  // github — gh CLI passthrough (present only when gh is installed)
  // -------------------------------------------------------------------------
  registry.set("github", {
    definition: def(
      "github",
      "Interact with GitHub through the installed `gh` CLI. Pass subcommand arguments, e.g. {args: [\"pr\", \"list\"]}.",
      {
        type: "object",
        properties: {
          args: { type: "array", items: { type: "string" }, description: "Arguments after `gh`" },
          timeout_ms: { type: "number", description: "Default 60000" },
        },
        required: ["args"],
      }
    ),
    cost: "medium",
    mutating: true,
    async run(args, ctx) {
      const argv = Array.isArray(args.args) ? args.args.map(String) : [];
      if (argv.length === 0) return fail("github failed: args required (e.g. ['pr','list'])");
      const timeoutMs = Math.min(num(args, "timeout_ms") ?? 60_000, 300_000);
      return new Promise((resolve) => {
        exec(
          `gh ${argv.map((a) => JSON.stringify(a)).join(" ")}`,
          { cwd: ctx.projectFolder, timeout: timeoutMs, maxBuffer: 4_000_000, env: process.env },
          (error, stdout, stderr) => {
            if (error && !stdout && !stderr) {
              resolve(fail(`github failed: gh CLI not available or errored: ${(error as Error).message}`));
              return;
            }
            const parts = [`exit: ${error ? ((error as { code?: number }).code ?? 1) : 0}`];
            if (stdout) parts.push(String(stdout));
            if (stderr) parts.push(`stderr:\n${String(stderr)}`);
            resolve(ok(truncate(parts.join("\n"))));
          }
        );
      });
    },
  });

  // -------------------------------------------------------------------------
  // checkpoint / rewind — snapshot listing & restore
  // -------------------------------------------------------------------------
  registry.set("checkpoint", {
    definition: def(
      "checkpoint",
      "Save a named restore point label for the files mutated so far this session. Snapshots are captured automatically before every write/edit.",
      { type: "object", properties: { label: { type: "string" } } }
    ),
    cost: "cheap",
    mutating: false,
    async run(_args, ctx) {
      if (!ctx.sessionId || !ctx.dataDir) return fail("checkpoint failed: no session context");
      const cps = readCheckpoints(ctx.dataDir, ctx.sessionId);
      const byPath = new Map<string, { id: string; ts: number }>();
      for (const cp of cps) byPath.set(cp.path, { id: cp.id, ts: cp.ts });
      const lines = [...byPath.entries()].map(
        ([p, v]) => `${path.relative(ctx.projectFolder, p)} → checkpoint ${v.id} (${new Date(v.ts).toISOString()})`
      );
      return ok(lines.length ? `${byPath.size} restorable file(s):\n${lines.join("\n")}` : "no checkpoints yet — write/edit a file first");
    },
  });

  registry.set("rewind", {
    definition: def(
      "rewind",
      "Restore files to their state before mutations. Actions: list {}, restore {id?, path?} — latest snapshot when neither given.",
      {
        type: "object",
        properties: {
          action: { type: "string", enum: ["list", "restore"] },
          id: { type: "string" },
          path: { type: "string" },
        },
        required: ["action"],
      }
    ),
    cost: "high",
    mutating: true,
    async run(args, ctx) {
      if (!ctx.sessionId || !ctx.dataDir) return fail("rewind failed: no session context");
      const action = str(args, "action") || "list";
      const cps = readCheckpoints(ctx.dataDir, ctx.sessionId);
      if (cps.length === 0) return ok("no checkpoints recorded");
      if (action === "list") {
        return ok(truncate(cps.slice(-30).reverse().map((c) => `${c.id} ${new Date(c.ts).toISOString()} ${c.path}`).join("\n")));
      }
      const id = str(args, "id");
      const targetPath = str(args, "path") ? resolveIn(ctx.projectFolder, str(args, "path")) : undefined;
      let candidates = cps;
      if (id) candidates = candidates.filter((c) => c.id === id);
      if (targetPath) candidates = candidates.filter((c) => c.path === targetPath);
      if (candidates.length === 0) return fail("rewind failed: no matching checkpoint");
      // Latest snapshot per path wins; restoring multiple paths uses each path's newest.
      const newestByPath = new Map<string, (typeof candidates)[number]>();
      for (const c of candidates) {
        const existing = newestByPath.get(c.path);
        if (!existing || existing.ts < c.ts) newestByPath.set(c.path, c);
      }
      const restored: string[] = [];
      for (const c of newestByPath.values()) {
        try {
          fs.mkdirSync(path.dirname(c.path), { recursive: true });
          fs.writeFileSync(c.path, c.before, "utf-8");
          restored.push(path.relative(ctx.projectFolder, c.path));
        } catch (err) {
          restored.push(`FAILED ${c.path}: ${(err as Error).message}`);
        }
      }
      return ok(`restored ${restored.length} file(s):\n${restored.join("\n")}`);
    },
  });
}

// Local lite walker reuse ---------------------------------------------------
