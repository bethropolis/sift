/**
 * File-tree filter module.
 *
 * `pattern.ts` is the entry point (`parsePattern` once per query,
 * `matchPattern` per path); `fuzzy.ts` / `glob.ts` are the matchers,
 * `highlight.ts` maps hits onto display names for the row renderer.
 */

export type { FuzzyHit, HighlightSeg, ParsedPattern, PathHit, PatternMode } from './types';
export { fuzzyMatch } from './fuzzy';
export { extToRegExpSource, globToRegExpSource } from './glob';
export { matchPattern, parsePattern } from './pattern';
export { nameMatchIndices, splitHighlight } from './highlight';
