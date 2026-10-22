import React from 'react';
import {
  FileCode,
  FileText,
  Braces,
  Atom,
  Code2,
  Binary
} from 'lucide-react';

export const CHUNK_SIZE = 250;

/** Return file-type specific icon matching developer expectations and screenshot */
export function getFileIcon(filename: string) {
  const ext = filename.split('.').pop()?.toLowerCase();
  if (ext === 'json') {
    return <Braces className="size-4 text-amber-400 shrink-0" />;
  }
  if (ext === 'tsx' || ext === 'jsx') {
    return <Atom className="size-4 text-cyan-400 shrink-0" />;
  }
  if (ext === 'ts' || ext === 'js' || ext === 'mjs' || ext === 'cjs') {
    return <Code2 className="size-4 text-blue-400 shrink-0" />;
  }
  if (ext === 'css' || ext === 'scss' || ext === 'sass' || ext === 'less') {
    return <FileCode className="size-4 text-purple-400 shrink-0" />;
  }
  if (ext === 'bin' || ext === 'exe' || ext === 'wasm' || ext === 'dat' || ext === 'png' || ext === 'jpg' || ext === 'ico') {
    return <Binary className="size-4 text-blue-300 shrink-0" />;
  }
  return <FileText className="size-4 text-foreground-subtle shrink-0" />;
}

/** Check if file is likely binary based on extension or diff content */
export function isBinaryFile(filename: string, diffText?: string): boolean {
  const ext = filename.split('.').pop()?.toLowerCase();
  const binaryExts = [
    'bin', 'dat', 'exe', 'dll', 'so', 'dylib', 'wasm',
    'png', 'jpg', 'jpeg', 'gif', 'ico', 'webp', 'pdf',
    'zip', 'gz', 'tar', 'woff', 'woff2', 'ttf', 'eot'
  ];
  if (ext && binaryExts.includes(ext)) return true;
  if (diffText && diffText.includes('Binary files') && diffText.includes('differ')) return true;
  return false;
}

/** Fast common prefix & suffix extraction for intra-line highlighting */
export function computeIntraLineSegments(oldText: string, newText: string) {
  if (oldText === newText) {
    return {
      oldSegments: [{ text: oldText, changed: false }],
      newSegments: [{ text: newText, changed: false }]
    };
  }

  let prefixLen = 0;
  const minLen = Math.min(oldText.length, newText.length);
  while (prefixLen < minLen && oldText[prefixLen] === newText[prefixLen]) {
    prefixLen++;
  }

  let oldSuffixLen = 0;
  let newSuffixLen = 0;
  while (
    oldSuffixLen < oldText.length - prefixLen &&
    newSuffixLen < newText.length - prefixLen &&
    oldText[oldText.length - 1 - oldSuffixLen] === newText[newText.length - 1 - newSuffixLen]
  ) {
    oldSuffixLen++;
    newSuffixLen++;
  }

  const oldPrefix = oldText.substring(0, prefixLen);
  const oldMid = oldText.substring(prefixLen, oldText.length - oldSuffixLen);
  const oldSuffix = oldText.substring(oldText.length - oldSuffixLen);

  const newPrefix = newText.substring(0, prefixLen);
  const newMid = newText.substring(prefixLen, newText.length - newSuffixLen);
  const newSuffix = newText.substring(newText.length - newSuffixLen);

  return {
    oldSegments: [
      { text: oldPrefix, changed: false },
      { text: oldMid, changed: true },
      { text: oldSuffix, changed: false }
    ].filter(s => s.text.length > 0),
    newSegments: [
      { text: newPrefix, changed: false },
      { text: newMid, changed: true },
      { text: newSuffix, changed: false }
    ].filter(s => s.text.length > 0)
  };
}

/** Format human-readable relative timestamp */
export function timeAgo(ts?: string | number): string {
  if (!ts) return '';
  const t = typeof ts === 'number' ? (ts < 1e11 ? ts * 1000 : ts) : new Date(ts).getTime();
  if (isNaN(t) || t <= 0) return '';
  const diff = Date.now() - t;
  if (diff < 0) return 'just now';
  const mins = Math.floor(diff / 60000);
  if (mins < 1) return 'just now';
  if (mins < 60) return `${mins}m ago`;
  const hours = Math.floor(mins / 60);
  if (hours < 24) return `${hours}h ago`;
  const days = Math.floor(hours / 24);
  if (days < 30) return `${days}d ago`;
  const months = Math.floor(days / 30);
  if (months < 12) return `${months}mo ago`;
  const years = Math.floor(days / 365);
  return `${years}y ago`;
}
