import { DiffHunkLine } from './inlineAiTypes';

/**
 * Computes line-by-line diff comparing originalText and generatedText.
 * Uses LCS (Longest Common Subsequence) with fast prefix/suffix trimming.
 */
export function computeLineDiff(originalText: string, generatedText: string): DiffHunkLine[] {
  const origLines = originalText ? originalText.split('\n') : [];
  const genLines = generatedText ? generatedText.split('\n') : [];

  let pre = 0;
  while (pre < origLines.length && pre < genLines.length && origLines[pre] === genLines[pre]) {
    pre++;
  }

  let sEnd = origLines.length;
  let gEnd = genLines.length;
  while (sEnd > pre && gEnd > pre && origLines[sEnd - 1] === genLines[gEnd - 1]) {
    sEnd--;
    gEnd--;
  }

  const result: DiffHunkLine[] = [];

  // Prefix (unchanged)
  for (let i = 0; i < pre; i++) {
    result.push({ type: 'unchanged', text: origLines[i] });
  }

  const a = origLines.slice(pre, sEnd);
  const b = genLines.slice(pre, gEnd);

  if (a.length === 0 && b.length > 0) {
    for (const line of b) {
      result.push({ type: 'added', text: line });
    }
  } else if (b.length === 0 && a.length > 0) {
    for (const line of a) {
      result.push({ type: 'deleted', text: line });
    }
  } else if (a.length > 0 && b.length > 0) {
    // DP LCS table for middle segment
    const n = a.length;
    const m = b.length;
    const dp = Array.from({ length: n + 1 }, () => new Int32Array(m + 1));

    for (let i = n - 1; i >= 0; i--) {
      for (let j = m - 1; j >= 0; j--) {
        if (a[i] === b[j]) {
          dp[i][j] = dp[i + 1][j + 1] + 1;
        } else {
          dp[i][j] = Math.max(dp[i + 1][j], dp[i][j + 1]);
        }
      }
    }

    let i = 0;
    let j = 0;
    while (i < n || j < m) {
      if (i < n && j < m && a[i] === b[j]) {
        result.push({ type: 'unchanged', text: a[i] });
        i++;
        j++;
      } else if (j < m && (i >= n || dp[i][j + 1] >= dp[i + 1][j])) {
        result.push({ type: 'added', text: b[j] });
        j++;
      } else if (i < n) {
        result.push({ type: 'deleted', text: a[i] });
        i++;
      }
    }
  }

  // Suffix (unchanged)
  for (let i = sEnd; i < origLines.length; i++) {
    result.push({ type: 'unchanged', text: origLines[i] });
  }

  return result;
}
