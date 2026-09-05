// Parses git decorations like " (HEAD -> main, origin/main)" into chip labels.
export function parseGitDecorations(dec?: string): string[] {
  if (!dec) return [];
  return dec
    .replace(/^\(|\)$/g, '')
    .split(',')
    .map(s => s.trim())
    .filter(Boolean);
}
