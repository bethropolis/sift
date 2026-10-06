/**
 * Pattern dispatch for the file-tree filter.
 *
 * One pattern per query (no AND/OR): a leading `!` negates, then the body is
 * fuzzy by default, a glob when it contains `* ? [`, a regex for `/.../` or
 * `re:...`, or an extension filter for `ext:go,ts`. Invalid regex/glob falls
 * back to fuzzy on the body so a typo never blanks the tree.
 */

import { fuzzyMatch } from './fuzzy';
import { extToRegExpSource, globToRegExpSource } from './glob';
import type { ParsedPattern, PathHit } from './types';

function hasGlobChars(body: string): boolean {
  return body.includes('*') || body.includes('?') || body.includes('[');
}

/**
 * Parse a raw query. Returns null for empty queries (no filtering).
 * Never throws; uncompilable patterns come back with `valid: false`.
 */
export function parsePattern(rawQuery: string): ParsedPattern | null {
  const raw = rawQuery.trim();
  if (raw === '') return null;

  let rest = raw;
  let negate = false;
  if (rest.startsWith('!')) {
    negate = true;
    rest = rest.slice(1).trim();
    if (rest === '') return null;
  }

  const lower = rest.toLowerCase();
  let mode: ParsedPattern['mode'] = 'fuzzy';
  let body = rest;
  if (lower.startsWith('ext:')) {
    mode = 'ext';
    body = rest.slice(4).trim();
  } else if (rest.startsWith('/')) {
    mode = 'regex';
    body = rest.endsWith('/') && rest.length > 1 ? rest.slice(1, -1) : rest.slice(1);
  } else if (lower.startsWith('re:')) {
    mode = 'regex';
    body = rest.slice(3);
  } else if (hasGlobChars(rest)) {
    mode = 'glob';
  }
  if (body === '') return null;

  if (mode === 'fuzzy') {
    return { raw, negate, mode, body, regex: null, valid: true };
  }
  try {
    const source = mode === 'ext' ? extToRegExpSource(body) : mode === 'glob' ? globToRegExpSource(body) : body;
    return { raw, negate, mode, body, regex: new RegExp(source, 'i'), valid: true };
  } catch {
    return { raw, negate, mode, body, regex: null, valid: false };
  }
}

/** Expand a regex exec match into ascending char offsets. */
function rangeIndices(start: number, length: number): number[] | null {
  if (length <= 0) return null;
  const out: number[] = [];
  for (let i = start; i < start + length; i++) out.push(i);
  return out;
}

/** Test one path against an already-parsed pattern. */
export function matchPattern(path: string, pat: ParsedPattern): PathHit {
  let hit = false;
  let indices: number[] | null = null;

  if (pat.mode === 'fuzzy' || !pat.valid) {
    // Invalid regex/glob degrades to fuzzy on the body.
    const m = fuzzyMatch(path, pat.body);
    hit = m !== null;
    indices = m?.indices ?? null;
  } else if (pat.regex) {
    const m = pat.regex.exec(path);
    hit = m !== null;
    if (m) indices = rangeIndices(m.index, m[0].length);
  }

  if (pat.negate) return { hit: !hit, indices: null };
  return { hit, indices };
}
