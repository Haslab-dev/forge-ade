/**
 * VS Code-style fuzzy matcher for the command palette / quick open.
 *
 * A query matches when its characters appear in the target in order
 * (subsequence match). The score rewards what a human expects: exact
 * substrings, matches at word boundaries (after `/`, `_`, `-`, `.`, or a
 * camelCase hump), consecutive runs, and short targets. For file paths,
 * basename matches beat path matches so "palette" ranks
 * `CommandPaletteModal.tsx` above a same-named directory.
 */

export interface FuzzyMatch {
  score: number;
  /** Indices into the target where query characters matched, ascending. */
  positions: number[];
}

const SEPARATOR_CHARS = new Set(['/', '\\', '_', '-', '.', ' ', '@', ':', '$', '#']);

// Match against the basename wins ties outright — this is what makes quick
// open feel like VS Code when many files share directory prefixes.
const BASENAME_BONUS = 60;
const CONSECUTIVE_BONUS = 8;
const BOUNDARY_BONUS = 12;
const LEADING_GAP_PENALTY = 1.2;
const LENGTH_PENALTY = 0.05;
const BASE_SCORE = 16;

function isCamelHump(target: string, idx: number): boolean {
  if (idx === 0) return false;
  const prev = target[idx - 1];
  const cur = target[idx];
  return (
    prev === prev.toLowerCase() &&
    cur === cur.toUpperCase() &&
    cur !== cur.toLowerCase()
  );
}

/** Greedy in-order subsequence match of `query` against `target`. */
function matchIn(query: string, target: string): FuzzyMatch | null {
  const positions: number[] = [];
  let score = BASE_SCORE;
  let prevIdx = -2;
  let searchFrom = 0;

  for (let qi = 0; qi < query.length; qi++) {
    const ch = query[qi].toLowerCase();
    let idx = -1;
    for (let i = searchFrom; i < target.length; i++) {
      if (target[i].toLowerCase() === ch) {
        idx = i;
        break;
      }
    }
    if (idx === -1) return null;

    if (idx === prevIdx + 1) {
      score += CONSECUTIVE_BONUS;
    } else if (prevIdx >= 0) {
      score -= Math.min(10, idx - prevIdx - 1) * LEADING_GAP_PENALTY;
    }
    if (idx === 0 || SEPARATOR_CHARS.has(target[idx - 1]) || isCamelHump(target, idx)) {
      score += BOUNDARY_BONUS;
    }

    positions.push(idx);
    prevIdx = idx;
    searchFrom = idx + 1;
  }

  return { score: score - target.length * LENGTH_PENALTY, positions };
}

/**
 * Fuzzy-match `query` against a file path, basename-aware. Returns null when
 * the query is not a subsequence of the path. An empty query matches
 * everything with a neutral score so callers can list files unranked.
 */
export function scoreFilePath(query: string, filePath: string): FuzzyMatch | null {
  const q = query.trim();
  if (!q) return { score: 0, positions: [] };

  const sep = Math.max(filePath.lastIndexOf('/'), filePath.lastIndexOf('\\'));
  if (sep >= 0) {
    const base = matchIn(q, filePath.slice(sep + 1));
    if (base) {
      return {
        score: base.score + BASENAME_BONUS,
        positions: base.positions.map(p => p + sep + 1)
      };
    }
  }
  return matchIn(q, filePath);
}

/** Plain fuzzy match for labels (commands, symbols). */
export function scoreText(query: string, text: string): FuzzyMatch | null {
  const q = query.trim();
  if (!q) return { score: 0, positions: [] };
  return matchIn(q, text);
}
