/** Match-highlight helpers for the file-tree row renderer. */

import type { HighlightSeg } from './types';

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
