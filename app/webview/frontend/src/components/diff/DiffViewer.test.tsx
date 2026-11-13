import { describe, expect, it } from 'vitest';
import { parseUnifiedDiff, parseUnifiedDiffByFiles, parseTwoFilesDiff } from './DiffViewer';

describe('DiffViewer parsers', () => {
  it('parses unified git diff with additions, deletions and context lines', () => {
    const rawDiff = `--- a/file.ts
+++ b/file.ts
@@ -1,4 +1,5 @@
 const a = 1;
-const b = 2;
+const b = 20;
+const c = 30;
 const d = 4;`;

    const { lines, splitRows } = parseUnifiedDiff(rawDiff);
    expect(lines.filter(l => l.type === 'add')).toHaveLength(2);
    expect(lines.filter(l => l.type === 'del')).toHaveLength(1);
    expect(lines.filter(l => l.type === 'normal')).toHaveLength(2);
    expect(splitRows.length).toBeGreaterThan(0);
  });

  it('parses unified diff across multiple files with parseUnifiedDiffByFiles', () => {
    const rawMultiDiff = `diff --git a/src/a.ts b/src/a.ts
index 111..222 100644
--- a/src/a.ts
+++ b/src/a.ts
@@ -1,2 +1,3 @@
 let x = 1;
+let y = 2;
 let z = 3;
diff --git a/src/b.ts b/src/b.ts
index 333..444 100644
--- a/src/b.ts
+++ b/src/b.ts
@@ -1,2 +1,1 @@
-let old = true;
 let keep = true;`;

    const sections = parseUnifiedDiffByFiles(rawMultiDiff);
    expect(sections).toHaveLength(2);
    expect(sections[0].filePath).toBe('src/a.ts');
    expect(sections[0].additions).toBe(1);
    expect(sections[0].deletions).toBe(0);

    expect(sections[1].filePath).toBe('src/b.ts');
    expect(sections[1].additions).toBe(0);
    expect(sections[1].deletions).toBe(1);
  });

  it('computes accurate LCS additions and deletions in parseTwoFilesDiff', () => {
    const orig = `line 1\nline 2\nline 3`;
    const mod = `line 1\nline 2 modified\nline 3\nline 4`;

    const { lines } = parseTwoFilesDiff(orig, mod);
    const adds = lines.filter(l => l.type === 'add');
    const dels = lines.filter(l => l.type === 'del');
    const normals = lines.filter(l => l.type === 'normal');

    expect(adds).toHaveLength(2);
    expect(dels).toHaveLength(1);
    expect(normals).toHaveLength(2);
  });
});
