import { type ClassValue, clsx } from "clsx";
import { PureComponent } from "react";
import { twMerge } from "tailwind-merge";

// App version — injected at build time from frontend/package.json (vite.config.ts).
// Bump via `make patch-version` / `minor-version` / `major-version`.
export const APP_VERSION = __APP_VERSION__;

export function cn(...inputs: ClassValue[]) {
  return twMerge(clsx(inputs));
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

