// Prettier-backed code formatting for the editor surface.
//
// Resolution order per file:
//   1. The project's own prettier (nearest node_modules/prettier walking up
//      from the file) so the workspace's pinned version and plugins apply.
//   2. The app-owned prettier from the daemon's own node_modules.
// Config comes from the project's .prettierrc / package.json "prettier" /
// .editorconfig via prettier.resolveConfig — user settings only override
// when explicitly set (they arrive as `overrides`).
//
// NOTE: prettier is deliberately loaded via await import(), never a static
// import — the module path is selected at runtime from the workspace being
// edited (plugin-loading-style registry), and eager loading at daemon boot
// would pay prettier's startup cost for every non-formatting session.

import fs from "fs";
import path from "path";
import { pathToFileURL } from "url";

export interface FormatOverrides {
  tabWidth?: number;
  useTabs?: boolean;
}

const FORMATTABLE_EXTS = new Set([
  "ts", "tsx", "js", "jsx", "mjs", "cjs", "mts", "cts",
  "json", "jsonc", "css", "scss", "less", "html", "htm",
  "md", "markdown", "mdx", "yaml", "yml", "vue",
]);

function findPrettierRoot(dir: string): string | null {
  let cur = path.resolve(dir);
  while (true) {
    if (fs.existsSync(path.join(cur, "node_modules", "prettier"))) return cur;
    const parent = path.dirname(cur);
    if (parent === cur) return null;
    cur = parent;
  }
}

// Dynamic import results cached per project root: loading prettier is not
// free, and searches fire on every format request.
const prettierCache = new Map<string, Promise<any> | null>();

async function importPrettier(fromDir: string): Promise<any | null> {
  const root = findPrettierRoot(fromDir);
  if (root) {
    if (!prettierCache.has(root)) {
      prettierCache.set(
        root,
        import(pathToFileURL(path.join(root, "node_modules", "prettier")).href).catch(() => null),
      );
    }
    const mod = await prettierCache.get(root)!;
    if (mod) return mod.default ?? mod;
  }
  // App-owned fallback resolved from the daemon's own dependency graph.
  try {
    const mod = await import("prettier");
    return mod.default ?? mod;
  } catch {
    return null;
  }
}

/**
 * Formats content with the project's prettier + its config files
 * (.prettierrc*, prettier.config.*, package.json "prettier", .editorconfig).
 * Returns null when no prettier is resolvable or formatting fails — callers
 * treat null as "leave content unchanged".
 */
export async function formatCode(filePath: string, content: string, overrides?: FormatOverrides): Promise<string | null> {
  const ext = filePath.split(".").pop()?.toLowerCase() || "";
  if (!FORMATTABLE_EXTS.has(ext)) return null;

  const prettier = await importPrettier(path.dirname(filePath));
  if (!prettier || typeof prettier.format !== "function") return null;

  try {
    // Project config files win over defaults; explicit user settings win over
    // both (undefined keys here mean "auto" and are skipped).
    const config = (await prettier.resolveConfig(filePath, { editorconfig: true })) ?? {};
    const options = { ...config, ...overrides, filepath: filePath };
    return await prettier.format(content, options);
  } catch (err) {
    // Parse errors etc.: report but never destroy the user's buffer.
    console.warn(`[formatter] ${filePath}:`, err instanceof Error ? err.message : err);
    return null;
  }
}
