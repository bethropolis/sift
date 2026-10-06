/**
 * Fuzzy path matching for the file-tree filter.
 *
 * Subsequence matching (fzf-style) with basename-aware scoring: a query like
 * `dmp` finds `internal/app/dump.go`, and matches in the file name outrank
 * matches buried in parent directories. Scores currently only decide
 * inclusion (the tree keeps hierarchy order); indices drive match
 * highlighting in the row renderer.
 */

export interface FuzzyHit {
  /** Higher is better. Only meaningful for comparing hits of one query. */
  score: number;
  /** Character offsets into the haystack, ascending. */
  indices: number[];
}

export interface HighlightSeg {
  text: string;
  hit: boolean;
}

const BOUNDARY_CHARS = new Set(['/', '_', '-', '.', ' ', '+']);

/** Subsequence-match `haystack` against `query` (case-insensitive). */
export function fuzzyMatch(haystack: string, query: string): FuzzyHit | null {
  const q = query.trim().toLowerCase();
  if (q === '') return null;
  const h = haystack.toLowerCase();
  const baseStart = h.lastIndexOf('/') + 1;

  const indices: number[] = [];
  let score = 0;
  let hi = 0;
  let prevFound = -2;

  for (let qi = 0; qi < q.length; qi++) {
    const found = h.indexOf(q[qi], hi);
    if (found === -1) return null;
    // Prime real estate first: file-name start, then word boundaries.
    if (found === baseStart) score += 8;
    else if (found === 0 || BOUNDARY_CHARS.has(h[found - 1])) score += 6;
    else if (found === prevFound + 1) score += 6;
    // Basename matches beat directory-prefix matches.
    if (found >= baseStart) score += 3;
    // Skipped characters cost a little (capped so long paths stay viable).
    score -= Math.min(found - hi, 8);
    indices.push(found);
    prevFound = found;
    hi = found + 1;
  }
  score += q.length * 10;
  return { score, indices };
}

/**
 * Rebase full-path match indices onto the trailing display name. Returns
 * null when nothing matched inside the name itself (the hit lives in a
 * parent directory, which the expanded tree already reveals).
 */
export function nameMatchIndices(path: string, name: string, indices: number[]): number[] | null {
  const off = path.length - name.length;
  const rebased = indices.filter((i) => i >= off).map((i) => i - off);
  return rebased.length > 0 ? rebased : null;
}

/** Split a display name into hit/plain segments for the row renderer. */
export function splitHighlight(name: string, indices: number[] | null): HighlightSeg[] {
  if (!indices || indices.length === 0) return [{ text: name, hit: false }];
  const hits = new Set(indices);
  const segs: HighlightSeg[] = [];
  let i = 0;
  while (i < name.length) {
    const hit = hits.has(i);
    let j = i + 1;
    while (j < name.length && hits.has(j) === hit) j++;
    segs.push({ text: name.slice(i, j), hit });
    i = j;
  }
  return segs;
}
