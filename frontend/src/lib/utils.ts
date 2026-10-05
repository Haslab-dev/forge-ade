import { type ClassValue, clsx } from "clsx";
import { PureComponent } from "react";
import { extendTailwindMerge } from "tailwind-merge";

// App version — injected at build time from frontend/package.json (vite.config.ts).
// Bump via `make patch-version` / `minor-version` / `major-version`.
export const APP_VERSION = __APP_VERSION__;

const customTwMerge = extendTailwindMerge({
  extend: {
    classGroups: {
      'font-size': [
        'text-ui-xs',
        'text-ui-sm',
        'text-ui-base',
        'text-ui-lg',
        'text-ui-xl'
      ]
    }
  }
});

export function cn(...inputs: ClassValue[]) {
  return customTwMerge(clsx(inputs));
}

export function cleanPiBanner(text: string): string {
  if (!text) return '';
  if (/pi\s+v[\d.]+/i.test(text) && (text.includes('Skills') || text.includes('Extensions'))) {
    const lines = text.split('\n');
    let inBanner = true;
    const kept: string[] = [];
    for (const line of lines) {
      const l = line.trim();
      if (inBanner) {
        if (
          /^pi\s+v[\d.]+/i.test(l) ||
          l === '---' ||
          /^(##\s*)?Skills/i.test(l) ||
          /^(##\s*)?Extensions/i.test(l) ||
          /^(?:-\s*)?\/.*(?:\.md|\.js|\.ts|\.mjs)$/i.test(l) ||
          l === ''
        ) {
          continue;
        }
        inBanner = false;
      }
      kept.push(line);
    }
    return kept.join('\n').trim();
  }
  return text;
}

export function formatDisplayTitle(title: string | undefined | null): string {
  if (!title) return '';
  let clean = title.trim().replace(/^['"]|['"]$/g, '');
  if (clean.includes('/var/folders/') || clean.includes('/TemporaryItems/') || clean.includes('/tmp/')) {
    const parts = clean.split('/');
    const last = parts[parts.length - 1];
    if (last) {
      clean = last.replace(/\.[a-zA-Z0-9]+$/, ''); // remove file extension
    }
  }
  return clean;
}

export interface ParsedFilePath {
  raw: string;
  cleanPath: string; // Absolute or resolved path
  displayName: string; // Base name for UI chip/tab
  line?: number;
  column?: number;
  isLikelyFilePath: boolean;
}

export function parseFilePath(raw: string, workspaceRoot?: string): ParsedFilePath {
  if (!raw || typeof raw !== 'string') {
    return { raw: '', cleanPath: '', displayName: '', isLikelyFilePath: false };
  }

  let text = raw.trim();

  // Strip wrapping quotes/brackets/backticks
  text = text.replace(/^[`'"]+|[`'"]+$/g, '');
  text = text.replace(/^\((.*)\)$/, '$1');

  // Strip file:// protocol if present
  if (text.startsWith('file://')) {
    text = text.replace(/^file:\/\//, '');
  }

  // Parse hash anchors like #L10 or #L10-L20
  let line: number | undefined;
  const hashMatch = /#L(\d+)(?:-L\d+)?$/.exec(text);
  if (hashMatch) {
    line = parseInt(hashMatch[1], 10);
    text = text.slice(0, hashMatch.index);
  }

  // Parse :line:col or :line suffix
  const colonMatch = /:(\d+)(?::(\d+))?$/.exec(text);
  let column: number | undefined;
  if (colonMatch) {
    if (line === undefined) {
      line = parseInt(colonMatch[1], 10);
    }
    if (colonMatch[2]) {
      column = parseInt(colonMatch[2], 10);
    }
    text = text.slice(0, colonMatch.index);
  }

  // Strip trailing punctuation like comma or period from prose
  text = text.replace(/[,;.)]+$/, '');

  const commonExtensions = /\.(ts|tsx|js|jsx|json|mjs|cjs|go|py|rs|css|scss|html|md|mdx|toml|yaml|yml|sh|bash|zsh|c|h|cpp|hpp|sql|txt|zon|zig|swift|kt|java|vue|svelte|xml|env|png|jpg|jpeg|svg|gif|webp)$/i;

  const hasSlash = text.includes('/') || text.includes('\\');
  const hasExt = commonExtensions.test(text);

  // Check if it's likely a file path:
  const isLikelyFilePath = (hasSlash && hasExt) ||
    hasExt ||
    (/^\.?\.?\//.test(text) && text.length > 2) ||
    (/^~?\//.test(text) && text.length > 2);

  // Resolve relative paths against workspaceRoot
  let cleanPath = text.replace(/\\/g, '/');
  if (workspaceRoot && !cleanPath.startsWith('/') && !cleanPath.startsWith('~')) {
    const rel = cleanPath.replace(/^\.\//, '');
    cleanPath = `${workspaceRoot.replace(/\/+$/, '')}/${rel}`;
  }

  const displayName = cleanPath.split('/').filter(Boolean).pop() || cleanPath;

  return {
    raw,
    cleanPath,
    displayName,
    line,
    column,
    isLikelyFilePath
  };
}


