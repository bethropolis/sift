// Build script for the sift serve web UI (run with bun, cross-platform).
//
// Steps: clear dist except .keep, run `vite build`, then gzip -9 every
// emitted file. The Go server embeds and serves only the .gz files.
//
// Usage: bun scripts/build.ts
import { gzipSync } from 'node:zlib';
import { readdirSync, readFileSync, rmSync, statSync, writeFileSync } from 'node:fs';
import { join } from 'node:path';
import { spawnSync } from 'node:child_process';

const dist = new URL('../dist/', import.meta.url).pathname;

// 1. Clear everything except .keep so stale chunks never survive a rebuild.
for (const entry of readdirSync(dist)) {
  if (entry === '.keep') continue;
  rmSync(join(dist, entry), { recursive: true, force: true });
}

// 2. Build.
const build = spawnSync('bunx', ['vite', 'build'], { stdio: 'inherit' });
if (build.status !== 0) process.exit(build.status ?? 1);

// 3. Precompress every file at max level. The .gz files are what embed.go
// serves; the uncompressed originals are removed afterwards.
function walk(dir: string): string[] {
  const out: string[] = [];
  for (const entry of readdirSync(dir)) {
    if (entry === '.keep' || entry.endsWith('.gz')) continue;
    const full = join(dir, entry);
    if (statSync(full).isDirectory()) out.push(...walk(full));
    else out.push(full);
  }
  return out;
}

const targets = walk(dist);
for (const file of targets) {
  const data = readFileSync(file);
  writeFileSync(file + '.gz', gzipSync(data, { level: 9 }));
  rmSync(file);
}
console.log(`precompressed ${targets.length} .gz files`);
