import { type ClassValue, clsx } from "clsx";
import { PureComponent } from "react";
import { extendTailwindMerge } from "tailwind-merge";

// App version & build — injected at build time from frontend/package.json (vite.config.ts).
// Bump via `make patch-version` / `minor-version` / `major-version`.
export const APP_VERSION = typeof __APP_VERSION__ !== 'undefined' ? __APP_VERSION__ : '0.0.0';
export const APP_BUILD = typeof __APP_BUILD__ !== 'undefined' ? __APP_BUILD__ : 'dev';

// The theme's extra font sizes (index.css). Unregistered, tailwind-merge reads
// text-2xs / text-ui-sm as colours and drops them next to text-* foregrounds.
const twMerge = extendTailwindMerge({
  extend: {
    theme: {
      text: ["2xs", "3xs", "4xs", "5xs", "ui-xs", "ui-sm", "ui-base", "ui-lg", "ui-xl"],
    },
  },
});

// text-ui-* classes are real Tailwind theme utilities (@theme --text-ui-*
// in index.css), so tailwind-merge resolves them as font sizes natively.
export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
}

export function cleanRawToolTags(text: string): string {
  if (!text) return '';
  let s = text;
  // 1. Remove complete in-band tool call and container blocks
  s = s.replace(/<toolcall>[\s\S]*?<\/toolcall>/gi, '');
  s = s.replace(/<tool_call>[\s\S]*?<\/tool_call>/gi, '');
  s = s.replace(/<[|\s\uFF5C]*(?:DSML[|\s\uFF5C]*)?calls>[\s\S]*?<\/[|\s\uFF5C]*(?:DSML[|\s\uFF5C]*)?calls>/gi, '');
  s = s.replace(/<[|\s\uFF5C]*(?:DSML[|\s\uFF5C]*)?invoke[\s\S]*?<\/[|\s\uFF5C]*(?:DSML[|\s\uFF5C]*)?invoke>/gi, '');
  s = s.replace(/<function_call[\s\S]*?<\/function_call>/gi, '');
  // 2. Remove any UNCLOSED in-band tool block at the end of text (e.g. while streaming parameters/code)
  s = s.replace(/<[|\s\uFF5C]*(?:DSML[|\s\uFF5C]*)?(?:calls|invoke)[\s\S]*$/gi, '');
  s = s.replace(/<(?:toolcall|tool_call|function_call)[\s\S]*$/gi, '');
  // 3. Remove standalone or unclosed parameter blocks
  s = s.replace(/<[|\s\uFF5C]*(?:DSML[|\s\uFF5C]*)?parameter[\s\S]*?<\/[|\s\uFF5C]*(?:DSML[|\s\uFF5C]*)?parameter>/gi, '');
  s = s.replace(/<[|\s\uFF5C]*(?:DSML[|\s\uFF5C]*)?parameter[\s\S]*$/gi, '');
  // 4. Remove any residual tool tags
  s = s.replace(/<\/?(?:toolcall|tool_call|function_call|[|\s\uFF5C]*(?:DSML[|\s\uFF5C]*)?(?:invoke|parameter|calls|call_begin|call_end))(?:=[^>]+|[^>]*)>/gi, '');
  return s.trim();
}

export function cleanPiBanner(text: string): string {
  if (!text) return '';
  const cleaned = cleanRawToolTags(text);
  if (/pi\s+v[\d.]+/i.test(cleaned) && (cleaned.includes('Skills') || cleaned.includes('Extensions'))) {
    const lines = cleaned.split('\n');
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
  return cleaned;
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
  relativePath: string; // Path relative to workspaceRoot (or cleanPath if outside)
  displayName: string; // Base name for UI chip/tab
  line?: number;
  column?: number;
  isLikelyFilePath: boolean;
}

export function getRelativePath(fullPath: string, workspaceRoot?: string): string {
  if (!fullPath) return '';
  let clean = fullPath.replace(/^file:\/\//, '').replace(/\\/g, '/');
  if (workspaceRoot) {
    const ws = workspaceRoot.replace(/\\/g, '/').replace(/\/+$/, '');
    if (clean.startsWith(ws + '/')) {
      return clean.slice(ws.length + 1);
    }
  }
  return clean;
}

export function parseFilePath(raw: string, workspaceRoot?: string): ParsedFilePath {
  if (!raw || typeof raw !== 'string') {
    return { raw: '', cleanPath: '', relativePath: '', displayName: '', isLikelyFilePath: false };
  }

  let text = raw.trim();

  // Strip wrapping quotes/brackets/backticks
  text = text.replace(/^[`'"]+|[`'"]+$/g, '');
  text = text.replace(/^\((.*)\)$/, '$1');

  // Strip file:// protocol if present
  if (text.startsWith('file://')) {
    text = text.replace(/^file:\/\//, '');
  }

  // Strip any accidental JSON or property key wrappers (e.g. Path":", path: , etc.)
  text = text.replace(/^(?:\{?\s*"?path"?\s*:\s*"?|\{?\s*"?filepath"?\s*:\s*"?|\{?\s*"?file"?\s*:\s*"?)/i, '');
  text = text.replace(/^(?:["']?(?:path|filepath|file|filename|target|targetfile)["']?\s*[:=]\s*["']?)+/i, '');
  text = text.replace(/^[`'"]+|[`'"]+$/g, '');
  text = text.replace(/["}\s]+$/, '');

  // If text contains an embedded absolute path preceded by malformed JSON remnants, extract it cleanly
  const absMatch = text.match(/(?:^|["'\s:=])(\/(?:Users|home|data|tmp|private|var|opt|etc|usr|[a-zA-Z0-9_.-]+)\/[^"'\s\t\r\n}]+)/);
  if (absMatch && (text.includes('":') || text.includes('Path"') || text.startsWith('{'))) {
    text = absMatch[1];
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

  const relativePath = getRelativePath(cleanPath, workspaceRoot);
  const displayName = cleanPath.split('/').filter(Boolean).pop() || cleanPath;

  return {
    raw,
    cleanPath,
    relativePath,
    displayName,
    line,
    column,
    isLikelyFilePath
  };
}

export const isMac = typeof navigator !== 'undefined' && (
  /Mac|iPod|iPhone|iPad/i.test(navigator.platform || '') ||
  /Mac|iPod|iPhone|iPad/i.test(navigator.userAgent || '')
);

export const isWindows = typeof navigator !== 'undefined' && (
  /Win/i.test(navigator.platform || '') ||
  /Windows/i.test(navigator.userAgent || '')
);

/** Determines if an event target is an active input, textarea, or content-editable code editor. */
export function isEditableElement(target: EventTarget | null): boolean {
  if (!target || !(target instanceof HTMLElement)) return false;
  const tagName = target.tagName.toLowerCase();
  if (tagName === 'input' || tagName === 'textarea' || tagName === 'select') return true;
  let curr: HTMLElement | null = target;
  while (curr) {
    if (curr.isContentEditable || curr.contentEditable === 'true' || curr.getAttribute?.('contenteditable') === 'true') {
      return true;
    }
    if (curr.classList?.contains('cm-editor') || curr.classList?.contains('xterm')) {
      return true;
    }
    curr = curr.parentElement;
  }
  return false;
}


