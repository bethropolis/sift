#!/usr/bin/env bun
// Sync repo docs/*.md into the Astro docs content collection.
//
//   bun scripts/sync-docs.mjs
//
// For each slug in SLUGS it writes site/src/content/docs/<slug>.md with
// `title` + `description` front matter (Astro content collections read both
// natively) and rewrites repo-relative links so they resolve under /sift/.
// Generated files are gitignored; run before `bun dev` and `bun build`.
import { mkdirSync, rmSync, cpSync, existsSync, readFileSync, writeFileSync, readdirSync } from 'node:fs';
import { join, dirname } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = join(dirname(fileURLToPath(import.meta.url)), '..', '..');
const docsSrc = join(root, 'docs');
const destDir = join(root, 'site', 'src', 'content', 'docs');
const assetsDest = join(root, 'site', 'public', 'assets', 'docs');
const base = '/sift';
const repoUrl = 'https://github.com/bethropolis/sift';

// Canonical nav order (matches docs/README.md grouping).
const SLUGS = [
  'index', 'installation', 'usage', 'picker', 'configuration', 'output',
  'delta', 'dependencies', 'follow', 'mcp', 'serve', 'serve-protocol',
];
const DOC_NAMES = SLUGS.filter((s) => s !== 'index');

rmSync(destDir, { recursive: true, force: true });
mkdirSync(destDir, { recursive: true });

if (existsSync(join(docsSrc, 'assets'))) {
  rmSync(assetsDest, { recursive: true, force: true });
  mkdirSync(assetsDest, { recursive: true });
  for (const f of readdirSync(join(docsSrc, 'assets'))) {
    cpSync(join(docsSrc, 'assets', f), join(assetsDest, f));
  }
}

function quote(s) {
  return '"' + s.replace(/\\/g, '\\\\').replace(/"/g, '\\"') + '"';
}

for (const slug of SLUGS) {
  const srcFile = slug === 'index' ? join(docsSrc, 'README.md') : join(docsSrc, `${slug}.md`);
  let text;
  try {
    text = readFileSync(srcFile, 'utf8');
  } catch {
    console.warn(`sync-docs: missing ${srcFile}, skipping`);
    continue;
  }
  const lines = text.split('\n');

  let title = slug;
  let bodyStart = 0;
  for (let i = 0; i < lines.length; i++) {
    const m = /^#\s+(.*)/.exec(lines[i]);
    if (m) {
      title = m[1].trim();
      bodyStart = i + 1;
      break;
    }
  }

  // Description: first non-empty body paragraph that is not a heading,
  // blockquote, list item, table row, HTML, or fenced block.
  let description = '';
  let para = [];
  let inFence = false;
  for (const line of lines.slice(bodyStart)) {
    const stripped = line.trim();
    if (stripped.startsWith('```')) {
      inFence = !inFence;
      continue;
    }
    if (inFence) continue;
    if (!stripped) {
      if (para.length) break;
      continue;
    }
    if (/^(#{1,6}\s|>\s*|[-*+]\s|\d+\.\s|\||<)/.test(stripped)) {
      if (para.length) break;
      continue;
    }
    para.push(stripped);
  }
  description = para
    .join(' ')
    .replace(/!\[([^\]]*)\]\([^)]*\)/g, '$1')
    .replace(/\[([^\]]*)\]\([^)]*\)/g, '$1')
    .replace(/`/g, '')
    .replace(/\s+/g, ' ')
    .trim();
  if (description.length > 150) {
    const cut = description.slice(0, 147);
    description = cut.slice(0, cut.lastIndexOf(' ')) + '...';
  }

  // The index page becomes link cards, so drop README's own link lists and
  // keep the intro + cheat-sheet + closer.
  let bodyLines = lines;
  if (slug === 'index') {
    const gs = lines.findIndex((l) => l.trim() === '## Getting started');
    const cs = lines.findIndex((l) => l.trim() === '## Command cheat-sheet');
    if (gs !== -1 && cs !== -1) bodyLines = [...lines.slice(0, gs), ...lines.slice(cs)];
  }

  let body = bodyLines.slice(bodyStart).join('\n');

  // Rewrite doc-to-doc links: ](name.md) / ](name.md#frag) -> site paths.
  for (const name of DOC_NAMES) {
    body = body.replaceAll(
      new RegExp(`\\]\\(${name}\\.md(#[^)]*)?\\)`, 'g'),
      `](${base}/docs/${name}/$1)`,
    );
  }
  body = body.replaceAll('](../justfile)', `](${repoUrl}/blob/main/justfile)`);
  body = body.replaceAll('](../README.md)', `](${base}/)`);
  body = body.replaceAll('](assets/', `](${base}/assets/docs/`);

  const front = `---\ntitle: ${quote(title)}\ndescription: ${quote(description)}\n---\n`;
  writeFileSync(join(destDir, `${slug}.md`), front + body);
  console.log(`synced docs/${slug === 'index' ? 'README.md' : slug + '.md'} -> content/docs/${slug}.md (${title})`);
}
