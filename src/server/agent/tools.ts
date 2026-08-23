// Server-side tool registry for the agent engine.
// Ported from the reference core tool surface: read, write, edit, bash, search,
// find, glob, git_status plus session-coupled todo/ask handled by the engine.

import fs from "fs";
import path from "path";
import { exec } from "child_process";
import type { ToolDefinition, ToolCall } from "./types";

export interface ToolResult {
  content: string;
  isError: boolean;
}

export type ToolCost = "cheap" | "medium" | "high";

export interface ToolHandler {
  definition: ToolDefinition;
  cost: ToolCost;
  /** Mutating tools require approval unless YOLO mode is on. */
  mutating: boolean;
  run(args: Record<string, unknown>, ctx: ToolContext): Promise<ToolResult>;
}

export interface ToolContext {
  projectFolder: string;
}

const MAX_OUTPUT_CHARS = 24_000;
const MAX_FILE_BYTES = 2_000_000;
const DEFAULT_IGNORED = new Set([
  "node_modules", ".git", ".zig-cache", "zig-out", "dist", "build",
  ".next", ".venv", "__pycache__", "target", ".turbo", "coverage",
]);

function truncate(text: string): string {
  if (text.length <= MAX_OUTPUT_CHARS) return text;
  const head = text.slice(0, MAX_OUTPUT_CHARS * 0.7);
  const tail = text.slice(-MAX_OUTPUT_CHARS * 0.3);
  return `${head}\n... [truncated ${text.length - MAX_OUTPUT_CHARS} chars] ...\n${tail}`;
}

function resolveIn(folder: string, target: unknown): string {
  const raw = typeof target === "string" ? target : "";
  if (!raw) return folder;
  if (path.isAbsolute(raw)) return path.normalize(raw);
  return path.resolve(folder, raw);
}

function ok(content: string): ToolResult {
  return { content, isError: false };
}

function fail(content: string): ToolResult {
  return { content, isError: true };
}

// ---------------------------------------------------------------------------
// Unified diff (minimal, for write/edit feedback)
// ---------------------------------------------------------------------------

export function unifiedDiff(before: string, after: string, filePath: string): string {
  const oldLines = before.split("\n");
  const newLines = after.split("\n");
  // Trim common prefix/suffix, then emit a simple -/+ body (good enough for
  // change feedback; not a minimal-edit diff).
  let start = 0;
  while (start < oldLines.length && start < newLines.length && oldLines[start] === newLines[start]) start++;
  let endOld = oldLines.length;
  let endNew = newLines.length;
  while (endOld > start && endNew > start && oldLines[endOld - 1] === newLines[endNew - 1]) {
    endOld--;
    endNew--;
  }
  const context = 2;
  const from = Math.max(0, start - context);
  const toOld = Math.min(oldLines.length, endOld + context);
  const toNew = Math.min(newLines.length, endNew + context);
  const lines: string[] = [`--- a/${filePath}`, `+++ b/${filePath}`];
  lines.push(`@@ -${from + 1},${toOld - from} +${from + 1},${Math.min(newLines.length, endNew + context) - from} @@`);
  for (let i = from; i < Math.max(toOld, start); i++) lines.push(` ${oldLines[i]}`);
  for (let i = start; i < endOld; i++) lines.push(`-${oldLines[i]}`);
  for (let i = start; i < endNew; i++) lines.push(`+${newLines[i]}`);
  for (let i = endOld; i < toOld && i >= endOld; i++) lines.push(` ${oldLines[i]}`);
  return truncate(lines.join("\n"));
}

// ---------------------------------------------------------------------------
// File walker (respects defaults + .gitignore-lite)
// ---------------------------------------------------------------------------

interface WalkAccumulator {
  files: string[];
  budget: number;
}

function walk(root: string, rel: string, acc: WalkAccumulator): void {
  if (acc.budget <= 0) return;
  const abs = rel ? path.join(root, rel) : root;
  let entries: fs.Dirent[];
  try {
    entries = fs.readdirSync(abs, { withFileTypes: true });
  } catch {
    return;
  }
  let ignores: string[] = [];
  try {
    ignores = fs.readFileSync(path.join(abs, ".gitignore"), "utf-8")
      .split("\n").map((l) => l.trim()).filter((l) => l && !l.startsWith("#"));
  } catch {}
  for (const entry of entries) {
    if (DEFAULT_IGNORED.has(entry.name)) continue;
    if (ignores.includes(entry.name)) continue;
    const childRel = rel ? `${rel}/${entry.name}` : entry.name;
    if (entry.isDirectory()) {
      walk(root, childRel, acc);
    } else if (entry.isFile()) {
      acc.files.push(childRel);
      acc.budget -= 1;
      if (acc.budget <= 0) return;
    }
  }
}

const TEXT_EXTENSIONS = new Set([
  ".ts", ".tsx", ".js", ".jsx", ".mjs", ".cjs", ".json", ".css", ".html", ".md",
  ".go", ".py", ".rs", ".java", ".kt", ".swift", ".dart", ".rb", ".php", ".c",
  ".h", ".cpp", ".hpp", ".zig", ".sql", ".sh", ".yaml", ".yml", ".toml", ".zon",
  ".txt", ".xml", ".vue", ".svelte",
]);

function isTextFile(p: string): boolean {
  return TEXT_EXTENSIONS.has(path.extname(p).toLowerCase());
}

/** Simple glob matcher supporting **, * and ?. */
export function globToRegExp(pattern: string): RegExp {
  let re = "";
  for (let i = 0; i < pattern.length; i++) {
    const c = pattern[i];
    if (c === "*") {
      if (pattern[i + 1] === "*") {
        re += ".*";
        i++;
        if (pattern[i + 1] === "/") i++; // `**/` may also match zero segments
      } else {
        re += "[^/]*";
      }
    } else if (c === "?") {
      re += "[^/]";
    } else if ("\\^$.|+()[{".includes(c)) {
      re += `\\${c}`;
    } else {
      re += c;
    }
  }
  return new RegExp(`^${re}$`);
}

// ---------------------------------------------------------------------------
// Handlers
// ---------------------------------------------------------------------------

function def(name: string, description: string, parameters: Record<string, unknown>): ToolDefinition {
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

const READ_DEF = def("read", "Read a file. Use start_line/end_line for a range; omit both for the whole file.", {
  type: "object",
  properties: {
    path: { type: "string", description: "File path (absolute or workspace-relative)" },
    start_line: { type: "number" },
    end_line: { type: "number" },
  },
  required: ["path"],
});

const WRITE_DEF = def("write", "Create or overwrite a file. Returns a unified diff of what changed.", {
  type: "object",
  properties: {
    path: { type: "string" },
    content: { type: "string" },
  },
  required: ["path", "content"],
});

const EDIT_DEF = def("edit", "Replace the exact `old` string with `new` in a file. Fails when old does not appear exactly once.", {
  type: "object",
  properties: {
    path: { type: "string" },
    old: { type: "string" },
    new: { type: "string" },
    replace_all: { type: "boolean" },
  },
  required: ["path", "old", "new"],
});

const BASH_DEF = def("bash", "Run a shell command in the workspace and capture stdout/stderr/exit code.", {
  type: "object",
  properties: {
    command: { type: "string" },
    timeout_ms: { type: "number", description: "Default 60000" },
  },
  required: ["command"],
});

const SEARCH_DEF = def("search", "Regex-search file contents across the workspace, returning file:line matches.", {
  type: "object",
  properties: {
    pattern: { type: "string" },
    max_results: { type: "number" },
  },
  required: ["pattern"],
});

const FIND_DEF = def("find", "Find paths whose name matches a substring (case-insensitive).", {
  type: "object",
  properties: {
    pattern: { type: "string" },
    max_results: { type: "number" },
  },
  required: ["pattern"],
});

const GLOB_DEF = def("glob", "Find files matching a glob pattern (e.g. src/**/*.ts). Supports ** wildcards.", {
  type: "object",
  properties: {
    pattern: { type: "string" },
  },
  required: ["pattern"],
});

const GIT_STATUS_DEF = def("git_status", "Git status (porcelain) plus current branch for the workspace.", {
  type: "object",
  properties: {},
});

export function buildCoreTools(): Map<string, ToolHandler> {
  const registry = new Map<string, ToolHandler>();

  registry.set("read", {
    definition: READ_DEF,
    cost: "cheap",
    mutating: false,
    async run(args, ctx) {
      const p = resolveIn(ctx.projectFolder, str(args, "path"));
      try {
        const stat = fs.statSync(p);
        if (stat.isDirectory()) {
          const entries = fs.readdirSync(p, { withFileTypes: true })
            .slice(0, 500)
            .map((e) => `${e.isDirectory() ? "d" : "-"} ${e.name}`);
          return ok(entries.join("\n"));
        }
        const content = fs.readFileSync(p, "utf-8");
        const start = num(args, "start_line");
        const end = num(args, "end_line");
        const all = content.split("\n");
        const lo = Math.max(1, start ?? 1);
        const hi = Math.min(all.length, end ?? all.length);
        const numbered = all.slice(lo - 1, hi).map((l, i) => `${lo + i}: ${l}`);
        return ok(truncate(numbered.join("\n")));
      } catch (err) {
        return fail(`read failed: ${(err as Error).message}`);
      }
    },
  });

  registry.set("write", {
    definition: WRITE_DEF,
    cost: "medium",
    mutating: true,
    async run(args, ctx) {
      const p = resolveIn(ctx.projectFolder, str(args, "path"));
      const content = str(args, "content");
      try {
        let before = "";
        if (fs.existsSync(p)) before = fs.readFileSync(p, "utf-8");
        fs.mkdirSync(path.dirname(p), { recursive: true });
        fs.writeFileSync(p, content, "utf-8");
        return ok(unifiedDiff(before, content, p));
      } catch (err) {
        return fail(`write failed: ${(err as Error).message}`);
      }
    },
  });

  registry.set("edit", {
    definition: EDIT_DEF,
    cost: "medium",
    mutating: true,
    async run(args, ctx) {
      const p = resolveIn(ctx.projectFolder, str(args, "path"));
      const oldStr = str(args, "old");
      const newStr = str(args, "new");
      if (!oldStr) return fail("edit failed: empty `old` string");
      try {
        const before = fs.readFileSync(p, "utf-8");
        const occurrences = before.split(oldStr).length - 1;
        if (occurrences === 0) return fail("edit failed: `old` not found in file");
        const replaceAll = args.replace_all === true;
        if (occurrences > 1 && !replaceAll) {
          return fail(`edit failed: \`old\` appears ${occurrences} times; provide more context or set replace_all`);
        }
        const after = replaceAll
          ? before.split(oldStr).join(newStr)
          : before.replace(oldStr, () => newStr);
        fs.writeFileSync(p, after, "utf-8");
        return ok(unifiedDiff(before, after, p));
      } catch (err) {
        return fail(`edit failed: ${(err as Error).message}`);
      }
    },
  });

  registry.set("bash", {
    definition: BASH_DEF,
    cost: "high",
    mutating: false, // commands vary; approval policy decided by engine config below
    async run(args, ctx) {
      const command = str(args, "command");
      const timeoutMs = num(args, "timeout_ms") ?? 60_000;
      return new Promise<ToolResult>((resolve) => {
        exec(
          command,
          { cwd: ctx.projectFolder, timeout: timeoutMs, maxBuffer: 4_000_000, env: process.env },
          (error, stdout, stderr) => {
            const code = error ? ((error as unknown as { code?: number }).code ?? 1) : 0;
            const parts = [`exit: ${code}`];
            if (stdout) parts.push(truncate(String(stdout)));
            if (stderr) parts.push(`stderr:\n${truncate(String(stderr))}`);
            resolve(ok(truncate(parts.join("\n"))));
          },
        );
      });
    },
  });

  registry.set("search", {
    definition: SEARCH_DEF,
    cost: "cheap",
    mutating: false,
    async run(args, ctx) {
      const pattern = str(args, "pattern");
      const maxResults = num(args, "max_results") ?? 200;
      if (!pattern) return fail("search failed: empty pattern");
      let regex: RegExp;
      try {
        regex = new RegExp(pattern, "i");
      } catch {
        return fail(`search failed: invalid regex: ${pattern}`);
      }
      const acc: WalkAccumulator = { files: [], budget: 8000 };
      walk(ctx.projectFolder, "", acc);
      const out: string[] = [];
      for (const rel of acc.files) {
        if (!isTextFile(rel)) continue;
        const abs = path.join(ctx.projectFolder, rel);
        try {
          if (fs.statSync(abs).size > MAX_FILE_BYTES) continue;
          const lines = fs.readFileSync(abs, "utf-8").split("\n");
          for (let i = 0; i < lines.length && out.length < maxResults; i++) {
            if (regex.test(lines[i])) out.push(`${rel}:${i + 1}: ${lines[i].trim().slice(0, 300)}`);
          }
        } catch {}
        if (out.length >= maxResults) break;
      }
      return ok(out.length > 0 ? truncate(out.join("\n")) : "no matches");
    },
  });

  registry.set("find", {
    definition: FIND_DEF,
    cost: "cheap",
    mutating: false,
    async run(args, ctx) {
      const pattern = str(args, "pattern").toLowerCase();
      const maxResults = num(args, "max_results") ?? 100;
      if (!pattern) return fail("find failed: empty pattern");
      const acc: WalkAccumulator = { files: [], budget: 20_000 };
      walk(ctx.projectFolder, "", acc);
      const hits = acc.files.filter((f) => f.toLowerCase().includes(pattern)).slice(0, maxResults);
      return ok(hits.length > 0 ? hits.join("\n") : "no matches");
    },
  });

  registry.set("glob", {
    definition: GLOB_DEF,
    cost: "cheap",
    mutating: false,
    async run(args, ctx) {
      const pattern = str(args, "pattern");
      if (!pattern) return fail("glob failed: empty pattern");
      let regex: RegExp;
      try {
        regex = globToRegExp(pattern);
      } catch {
        return fail(`glob failed: invalid pattern: ${pattern}`);
      }
      const acc: WalkAccumulator = { files: [], budget: 20_000 };
      walk(ctx.projectFolder, "", acc);
      // `**/x` style patterns should match bare `x` too.
      const altPattern = pattern.startsWith("**/") ? pattern.slice(3) : null;
      const alt = altPattern ? globToRegExp(altPattern) : null;
      const hits = acc.files.filter((f) => regex.test(f) || (alt?.test(f) ?? false)).slice(0, 500);
      return ok(hits.length > 0 ? hits.join("\n") : "no matches");
    },
  });

  registry.set("git_status", {
    definition: GIT_STATUS_DEF,
    cost: "medium",
    mutating: false,
    async run(_args, ctx) {
      const run = (gitArgs: string[]) =>
        new Promise<string>((resolve) => {
          exec(`git ${gitArgs.join(" ")}`, { cwd: ctx.projectFolder, timeout: 15_000 }, (err, stdout) => {
            resolve(err ? "" : String(stdout));
          });
        });
      const branch = (await run(["rev-parse", "--abbrev-ref", "HEAD"])).trim() || "(none)";
      const status = await run(["status", "--porcelain=v1", "-uall"]);
      return ok(`branch: ${branch}\n\n${status || "(clean)"}`);
    },
  });

  return registry;
}
