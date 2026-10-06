/**
 * Glob → RegExp translation for the file-tree filter.
 *
 * `*` crosses `/` (so `*.go` matches at any depth and `src/*.ts` scopes by
 * directory), `?` matches one char, `[...]` classes pass through. Everything
 * else is regex-escaped. Matching is case-insensitive; callers add anchors.
 */

const REGEX_SPECIAL = new Set(['\\', '^', '$', '.', '|', '+', '(', ')', '{', '}']);

/** Translate a glob body to a regex source. Throws on unbalanced `[`. */
export function globToRegExpSource(glob: string): string {
  let out = '';
  let i = 0;
  while (i < glob.length) {
    const c = glob[i];
    if (c === '*') {
      out += '.*';
      i++;
    } else if (c === '?') {
      out += '.';
      i++;
    } else if (c === '[') {
      const close = glob.indexOf(']', i + 1);
      if (close === -1) throw new Error('unbalanced [ in glob');
      out += glob.slice(i, close + 1);
      i = close + 1;
    } else {
      out += REGEX_SPECIAL.has(c) ? `\\${c}` : c;
      i++;
    }
  }
  return out;
}

/** `ext:go,ts` → matches a `.go` / `.ts` suffix (case-insensitive). */
export function extToRegExpSource(spec: string): string {
  const exts = spec
    .split(',')
    .map((e) => e.trim().replace(/^\./, '').toLowerCase())
    .filter((e) => e !== '');
  if (exts.length === 0) throw new Error('empty ext: list');
  const escaped = exts.map((e) => e.replace(/[^a-z0-9]/g, (c) => `\\${c}`));
  return `\\.(?:${escaped.join('|')})$`;
}
