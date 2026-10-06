/**
 * Fuzzy path matching for the file-tree filter.
 *
 * Subsequence matching (fzf-style) with basename-aware scoring: a query like
 * `dmp` finds `internal/app/dump.go`, and matches in the file name outrank
 * matches buried in parent directories.
 */

import type { FuzzyHit } from './types';

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
