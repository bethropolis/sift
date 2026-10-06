/** Shared filter-module types. */

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

/** How a parsed query matches (after negation stripping). */
export type PatternMode = 'fuzzy' | 'glob' | 'regex' | 'ext';

export interface ParsedPattern {
  /** Original trimmed query (for debugging / UI). */
  raw: string;
  /** Leading `!` — the sense of the match is inverted. */
  negate: boolean;
  mode: PatternMode;
  /** The match body with prefix/slashes stripped. */
  body: string;
  /** Compiled matcher for glob/regex/ext modes; null when invalid. */
  regex: RegExp | null;
  /** False when a regex/glob failed to compile (falls back to fuzzy). */
  valid: boolean;
}

/** Result of testing one path against a parsed pattern. */
export interface PathHit {
  hit: boolean;
  /** Character offsets into the path for highlighting; null when none. */
  indices: number[] | null;
}
