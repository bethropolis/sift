// CI size gate for the sift serve web UI (run with bun).
// Initial (entry) JS must stay under 60 KB gzipped, the whole UI under 150 KB.
// Usage: bun scripts/check-budget.ts
import { readdirSync, statSync } from 'node:fs';
import { join } from 'node:path';

const dist = new URL('../dist/', import.meta.url).pathname;

function walk(dir: string): string[] {
  const out: string[] = [];
  for (const entry of readdirSync(dir)) {
    if (entry === '.keep') continue;
    const full = join(dir, entry);
    if (statSync(full).isDirectory()) out.push(...walk(full));
    else out.push(full);
  }
  return out;
}

const files = walk(dist);
const gz = files.filter((f) => f.endsWith('.gz'));
if (gz.length === 0) {
  console.error('check-budget: no .gz files in web/dist (run `just web` first)');
  process.exit(1);
}

const size = (f: string) => statSync(f).size;
const total = gz.reduce((n, f) => n + size(f), 0);
// The entry chunk is assets/index-*.js.gz. Every other JS chunk (the Settings
// and Clone dialogs, ...) is lazy-loaded and excluded from the initial budget;
// matching the entry by name means new lazy chunks never need a script edit.
const initialFiles = gz.filter((f) => /\/index-[^/]*\.js\.gz$/.test(f));
const initial = initialFiles.reduce((n, f) => n + size(f), 0);

console.log(`initial JS: ${(initial / 1024).toFixed(1)} KiB gz (${initialFiles.length} files)`);
console.log(`total UI:   ${(total / 1024).toFixed(1)} KiB gz (${gz.length} files)`);

let failed = false;
if (initial > 60 * 1024) {
  console.error(`OVER BUDGET: initial JS ${initial} bytes > 60 KiB`);
  failed = true;
}
if (total > 150 * 1024) {
  console.error(`OVER BUDGET: total UI ${total} bytes > 150 KiB`);
  failed = true;
}
process.exit(failed ? 1 : 0);
