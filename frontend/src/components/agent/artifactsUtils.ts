/**
 * Utilities for Claude Desktop-grade Artifacts Canvas:
 * Versioning, multi-slide carousels, responsive viewports, and token estimation.
 */

export interface ArtifactVersion {
  version: number;
  content: string;
  timestamp: number;
  label?: string;
}

export type ViewportMode = 'responsive' | 'desktop' | 'tablet' | 'mobile';

export const VIEWPORT_WIDTHS: Record<ViewportMode, string> = {
  responsive: '100%',
  desktop: '100%',
  tablet: '768px',
  mobile: '375px'
};

/**
 * Splits HTML content into individual slides if carousel syntax or slide classes are present.
 * Supports:
 * - `<!-- slide -->` comment markers
 * - `<section class="slide">...</section>`
 * - `<div class="slide">...</div>`
 */
export function detectSlides(html: string): string[] {
  if (!html) return [];

  // 1. Check for `<!-- slide -->` comments
  if (html.includes('<!-- slide -->')) {
    const rawSlides = html.split(/<!--\s*slide\s*-->/i);
    return rawSlides.map(s => s.trim()).filter(Boolean);
  }

  // 2. Check for <section class="slide"> or <div class="slide">
  const slideRegex = /<(section|div)[^>]*class=["'][^"']*\bslide\b[^"']*["'][^>]*>([\s\S]*?)<\/\1>/gi;
  const matches: string[] = [];
  let match: RegExpExecArray | null;

  while ((match = slideRegex.exec(html)) !== null) {
    if (match[0]) {
      matches.push(match[0].trim());
    }
  }

  if (matches.length > 1) {
    return matches;
  }

  return [html];
}

/**
 * Estimates token count based on the standard 4 characters per token heuristic.
 */
export function estimateTokenCount(text: string): number {
  if (!text) return 0;
  return Math.max(1, Math.round(text.length / 4));
}

/**
 * Formats elapsed duration into human-readable stopwatch string (e.g. "12.4s" or "2m 15s").
 */
export function formatElapsedSeconds(seconds: number): string {
  if (seconds < 60) {
    return `${seconds % 1 === 0 ? seconds : seconds.toFixed(1)}s`;
  }
  const mins = Math.floor(seconds / 60);
  const remSecs = Math.round(seconds % 60);
  return `${mins}m ${remSecs}s`;
}

/**
 * Generates an export filename for an artifact based on title and version.
 */
export function generateArtifactFilename(title = 'artifact', version = 1, ext = 'html'): string {
  const cleanTitle = title
    .toLowerCase()
    .replace(/[^a-z0-9_-]+/g, '-')
    .replace(/^-+|-+$/g, '') || 'artifact';
  return `${cleanTitle}-v${version}.${ext}`;
}
