/**
 * In-memory mock server for isolated UI work (VITE_MOCK=true).
 *
 * Only imported when USE_MOCK is true. Mock data intentionally mirrors a
 * generic Go project; production always uses the live client.
 */

import type { RecentProject, SettingsData, TreeFile } from './types';

// In-memory mock server state
export let mockLoginAttempts = 0;
export let mockLockoutUntil = 0;
export let mockAuthenticated = true;

export let mockRecents: RecentProject[] = [
  {
    root: '/Users/developer/code/sift',
    name: 'sift',
    lastOpened: '10 minutes ago',
    branch: 'main',
  },
  {
    root: '/Users/developer/code/astral-uv',
    name: 'astral-uv',
    lastOpened: '2 hours ago',
    branch: 'feat/cache-cleanup',
  },
  {
    root: '/Users/developer/code/conc',
    name: 'conc',
    lastOpened: '3 days ago',
    branch: 'master',
  },
];

export let mockSettings: SettingsData = {
  defaultStyle: 'xml',
  defaultBudget: 64000,
  theme: 'system',
  showHidden: false,
  fileSort: 'name',
};

// Mock browse entries get plausible staggered mtimes for sort-by-updated UI work.
export function stamp<T extends { name: string }>(entries: T[]): (T & { modTime: number })[] {
  const base = Date.now();
  return entries.map((e, i) => ({ ...e, modTime: base - i * 3600_000 }));
}

// Generate realistic mock files for a Go project (~150 files)
export const mockTreeFiles: Array<Omit<TreeFile, 'followable'>> = [
  // Root files
  { path: 'go.mod', size: 1420, tokens: 340, language: 'go', score: 0.94 },
  { path: 'go.sum', size: 18450, tokens: 4200, language: 'text', score: 0.25 },
  { path: 'main.go', size: 2180, tokens: 520, language: 'go', score: 0.98 },
  { path: 'README.md', size: 4890, tokens: 1120, language: 'markdown', score: 0.78 },
  { path: 'LICENSE', size: 1060, tokens: 280, language: 'text', score: 0.05 },
  { path: 'Makefile', size: 1840, tokens: 440, language: 'makefile', score: 0.65 },
  { path: '.gitignore', size: 450, tokens: 110, language: 'text', score: 0.15 },
  { path: 'Dockerfile', size: 980, tokens: 240, language: 'dockerfile', score: 0.42 },

  // cmd/sift
  { path: 'cmd/sift/main.go', size: 2800, tokens: 680, language: 'go', score: 0.96 },
  { path: 'cmd/sift/serve.go', size: 5400, tokens: 1350, language: 'go', score: 0.97 },
  { path: 'cmd/sift/pack.go', size: 4100, tokens: 980, language: 'go', score: 0.91 },
  { path: 'cmd/sift/scan.go', size: 3600, tokens: 840, language: 'go', score: 0.88 },
  { path: 'cmd/sift/flags.go', size: 2900, tokens: 690, language: 'go', score: 0.72 },
  { path: 'cmd/sift/version.go', size: 850, tokens: 190, language: 'go', score: 0.35 },

  // pkg/scanner
  { path: 'pkg/scanner/scanner.go', size: 6800, tokens: 1680, language: 'go', score: 0.95 },
  { path: 'pkg/scanner/ignore.go', size: 4200, tokens: 1020, language: 'go', score: 0.84 },
  { path: 'pkg/scanner/walker.go', size: 5900, tokens: 1450, language: 'go', score: 0.9 },
  { path: 'pkg/scanner/filter.go', size: 3100, tokens: 760, language: 'go', score: 0.77 },
  { path: 'pkg/scanner/binary.go', size: 2400, tokens: 590, language: 'go', score: 0.61 },
  { path: 'pkg/scanner/stats.go', size: 1950, tokens: 480, language: 'go', score: 0.55 },
  { path: 'pkg/scanner/types.go', size: 1600, tokens: 390, language: 'go', score: 0.73 },
  { path: 'pkg/scanner/scanner_test.go', size: 8400, tokens: 2050, language: 'go', score: 0.68 },
  { path: 'pkg/scanner/ignore_test.go', size: 4800, tokens: 1150, language: 'go', score: 0.52 },
  { path: 'pkg/scanner/walker_test.go', size: 6100, tokens: 1490, language: 'go', score: 0.58 },

  // pkg/git
  { path: 'pkg/git/repo.go', size: 5600, tokens: 1380, language: 'go', score: 0.92 },
  { path: 'pkg/git/log.go', size: 7200, tokens: 1750, language: 'go', score: 0.94 },
  { path: 'pkg/git/blame.go', size: 4900, tokens: 1200, language: 'go', score: 0.82 },
  { path: 'pkg/git/diff.go', size: 6300, tokens: 1540, language: 'go', score: 0.89 },
  { path: 'pkg/git/commit.go', size: 3400, tokens: 820, language: 'go', score: 0.71 },
  { path: 'pkg/git/churn.go', size: 4500, tokens: 1110, language: 'go', score: 0.86 },
  { path: 'pkg/git/author.go', size: 2100, tokens: 510, language: 'go', score: 0.44 },
  { path: 'pkg/git/git_test.go', size: 9200, tokens: 2240, language: 'go', score: 0.62 },
  { path: 'pkg/git/diff_test.go', size: 5400, tokens: 1310, language: 'go', score: 0.53 },

  // pkg/rank
  { path: 'pkg/rank/scorer.go', size: 7800, tokens: 1920, language: 'go', score: 0.97 },
  { path: 'pkg/rank/tfidf.go', size: 4600, tokens: 1140, language: 'go', score: 0.85 },
  { path: 'pkg/rank/bm25.go', size: 5200, tokens: 1280, language: 'go', score: 0.89 },
  { path: 'pkg/rank/recency.go', size: 3800, tokens: 920, language: 'go', score: 0.81 },
  { path: 'pkg/rank/decay.go', size: 2900, tokens: 710, language: 'go', score: 0.76 },
  { path: 'pkg/rank/pagerank.go', size: 6400, tokens: 1570, language: 'go', score: 0.79 },
  { path: 'pkg/rank/heuristic.go', size: 3900, tokens: 950, language: 'go', score: 0.74 },
  { path: 'pkg/rank/rank_test.go', size: 7600, tokens: 1850, language: 'go', score: 0.66 },
  { path: 'pkg/rank/bm25_test.go', size: 4900, tokens: 1190, language: 'go', score: 0.48 },

  // pkg/packer
  { path: 'pkg/packer/packer.go', size: 8900, tokens: 2180, language: 'go', score: 0.98 },
  { path: 'pkg/packer/xml.go', size: 3800, tokens: 930, language: 'go', score: 0.87 },
  { path: 'pkg/packer/markdown.go', size: 3500, tokens: 860, language: 'go', score: 0.86 },
  { path: 'pkg/packer/plain.go', size: 2700, tokens: 660, language: 'go', score: 0.75 },
  { path: 'pkg/packer/tokens.go', size: 4400, tokens: 1080, language: 'go', score: 0.93 },
  { path: 'pkg/packer/counter.go', size: 3200, tokens: 790, language: 'go', score: 0.83 },
  { path: 'pkg/packer/tree_fmt.go', size: 3600, tokens: 880, language: 'go', score: 0.79 },
  { path: 'pkg/packer/chunk.go', size: 4800, tokens: 1180, language: 'go', score: 0.71 },
  { path: 'pkg/packer/packer_test.go', size: 10200, tokens: 2490, language: 'go', score: 0.69 },

  // pkg/redact
  { path: 'pkg/redact/redact.go', size: 6900, tokens: 1710, language: 'go', score: 0.96 },
  { path: 'pkg/redact/patterns.go', size: 8200, tokens: 2010, language: 'go', score: 0.9 },
  { path: 'pkg/redact/entropy.go', size: 4500, tokens: 1110, language: 'go', score: 0.88 },
  { path: 'pkg/redact/mask.go', size: 2800, tokens: 690, language: 'go', score: 0.82 },
  { path: 'pkg/redact/api_keys.go', size: 5400, tokens: 1330, language: 'go', score: 0.91 },
  { path: 'pkg/redact/rules.go', size: 4100, tokens: 1010, language: 'go', score: 0.79 },
  { path: 'pkg/redact/redact_test.go', size: 8800, tokens: 2150, language: 'go', score: 0.64 },
  { path: 'pkg/redact/entropy_test.go', size: 4600, tokens: 1120, language: 'go', score: 0.51 },

  // pkg/signatures
  { path: 'pkg/signatures/extract.go', size: 7100, tokens: 1740, language: 'go', score: 0.93 },
  { path: 'pkg/signatures/golang.go', size: 6800, tokens: 1670, language: 'go', score: 0.91 },
  { path: 'pkg/signatures/typescript.go', size: 5900, tokens: 1450, language: 'go', score: 0.85 },
  { path: 'pkg/signatures/python.go', size: 5400, tokens: 1320, language: 'go', score: 0.83 },
  { path: 'pkg/signatures/rust.go', size: 6200, tokens: 1520, language: 'go', score: 0.86 },
  { path: 'pkg/signatures/csharp.go', size: 4800, tokens: 1180, language: 'go', score: 0.67 },
  { path: 'pkg/signatures/java.go', size: 5100, tokens: 1250, language: 'go', score: 0.69 },
  { path: 'pkg/signatures/ruby.go', size: 3900, tokens: 960, language: 'go', score: 0.62 },
  { path: 'pkg/signatures/c_cpp.go', size: 5800, tokens: 1420, language: 'go', score: 0.7 },
  { path: 'pkg/signatures/extract_test.go', size: 9400, tokens: 2310, language: 'go', score: 0.63 },

  // pkg/server
  { path: 'pkg/server/server.go', size: 6500, tokens: 1600, language: 'go', score: 0.96 },
  { path: 'pkg/server/routes.go', size: 4800, tokens: 1180, language: 'go', score: 0.95 },
  { path: 'pkg/server/handlers.go', size: 8400, tokens: 2060, language: 'go', score: 0.97 },
  { path: 'pkg/server/middleware.go', size: 4200, tokens: 1030, language: 'go', score: 0.87 },
  { path: 'pkg/server/auth.go', size: 3600, tokens: 880, language: 'go', score: 0.89 },
  { path: 'pkg/server/tls.go', size: 3100, tokens: 760, language: 'go', score: 0.73 },
  { path: 'pkg/server/ratelimit.go', size: 3400, tokens: 830, language: 'go', score: 0.84 },
  { path: 'pkg/server/static.go', size: 2600, tokens: 640, language: 'go', score: 0.68 },
  { path: 'pkg/server/server_test.go', size: 9800, tokens: 2400, language: 'go', score: 0.65 },

  // pkg/config
  { path: 'pkg/config/config.go', size: 4900, tokens: 1200, language: 'go', score: 0.88 },
  { path: 'pkg/config/defaults.go', size: 2800, tokens: 690, language: 'go', score: 0.74 },
  { path: 'pkg/config/validate.go', size: 3500, tokens: 860, language: 'go', score: 0.8 },
  { path: 'pkg/config/config_test.go', size: 5200, tokens: 1270, language: 'go', score: 0.56 },

  // pkg/cache
  { path: 'pkg/cache/lru.go', size: 4100, tokens: 1010, language: 'go', score: 0.78 },
  { path: 'pkg/cache/disk.go', size: 4700, tokens: 1150, language: 'go', score: 0.75 },
  { path: 'pkg/cache/mmap.go', size: 3800, tokens: 930, language: 'go', score: 0.68 },
  { path: 'pkg/cache/hasher.go', size: 2200, tokens: 540, language: 'go', score: 0.62 },
  { path: 'pkg/cache/cache_test.go', size: 5800, tokens: 1420, language: 'go', score: 0.49 },

  // internal/testutil
  { path: 'internal/testutil/fixtures.go', size: 6400, tokens: 1570, language: 'go', score: 0.59 },
  { path: 'internal/testutil/mock_git.go', size: 5800, tokens: 1420, language: 'go', score: 0.61 },
  { path: 'internal/testutil/mock_fs.go', size: 4900, tokens: 1200, language: 'go', score: 0.57 },
  { path: 'internal/testutil/assertions.go', size: 3200, tokens: 780, language: 'go', score: 0.45 },
  { path: 'internal/testutil/temp.go', size: 2100, tokens: 510, language: 'go', score: 0.41 },

  // internal/tokens (including large token files)
  { path: 'internal/tokens/bpe.go', size: 7600, tokens: 1860, language: 'go', score: 0.82 },
  { path: 'internal/tokens/cl100k.go', size: 9400, tokens: 2300, language: 'go', score: 0.8 },
  { path: 'internal/tokens/o200k.go', size: 11200, tokens: 2750, language: 'go', score: 0.83 },
  { path: 'internal/tokens/encoder.go', size: 6200, tokens: 1520, language: 'go', score: 0.81 },
  { path: 'internal/tokens/tables.go', size: 165000, tokens: 41200, language: 'go', score: 0.22 },

  // web/src and web app files
  { path: 'web/package.json', size: 1850, tokens: 460, language: 'json', score: 0.91 },
  { path: 'web/tsconfig.json', size: 890, tokens: 220, language: 'json', score: 0.65 },
  { path: 'web/vite.config.ts', size: 640, tokens: 160, language: 'typescript', score: 0.72 },
  { path: 'web/index.html', size: 780, tokens: 190, language: 'html', score: 0.7 },
  { path: 'web/src/App.tsx', size: 3400, tokens: 830, language: 'typescript', score: 0.95 },
  { path: 'web/src/main.tsx', size: 450, tokens: 110, language: 'typescript', score: 0.6 },
  { path: 'web/src/index.css', size: 5200, tokens: 1280, language: 'css', score: 0.89 },
  { path: 'web/src/lib/api.ts', size: 7800, tokens: 1910, language: 'typescript', score: 0.96 },
  { path: 'web/src/lib/router.ts', size: 2100, tokens: 510, language: 'typescript', score: 0.88 },
  { path: 'web/src/lib/types.ts', size: 3100, tokens: 760, language: 'typescript', score: 0.84 },
  { path: 'web/src/lib/format.ts', size: 2400, tokens: 590, language: 'typescript', score: 0.78 },
  { path: 'web/src/components/FileTree.tsx', size: 9200, tokens: 2250, language: 'typescript', score: 0.94 },
  { path: 'web/src/components/Preview.tsx', size: 5400, tokens: 1320, language: 'typescript', score: 0.92 },
  { path: 'web/src/components/OutputView.tsx', size: 6800, tokens: 1660, language: 'typescript', score: 0.93 },
  { path: 'web/src/components/BudgetMeter.tsx', size: 3800, tokens: 930, language: 'typescript', score: 0.9 },
  { path: 'web/src/components/ProjectSwitcher.tsx', size: 4200, tokens: 1030, language: 'typescript', score: 0.87 },
  { path: 'web/src/components/ShortcutsModal.tsx', size: 3600, tokens: 880, language: 'typescript', score: 0.81 },
  { path: 'web/src/routes/WorkspaceRoute.tsx', size: 10500, tokens: 2570, language: 'typescript', score: 0.97 },
  { path: 'web/src/routes/ProjectsRoute.tsx', size: 8200, tokens: 2010, language: 'typescript', score: 0.93 },
  { path: 'web/src/routes/LoginRoute.tsx', size: 5100, tokens: 1250, language: 'typescript', score: 0.89 },
  { path: 'web/src/routes/SettingsRoute.tsx', size: 4900, tokens: 1200, language: 'typescript', score: 0.86 },
  { path: 'web/public/favicon.svg', size: 620, tokens: 150, language: 'svg', score: 0.4 },
  { path: 'web/public/robots.txt', size: 80, tokens: 20, language: 'text', score: 0.1 },

  // docs
  { path: 'docs/architecture.md', size: 9800, tokens: 2400, language: 'markdown', score: 0.72 },
  { path: 'docs/cli-reference.md', size: 7600, tokens: 1860, language: 'markdown', score: 0.69 },
  { path: 'docs/serve-protocol.md', size: 6200, tokens: 1520, language: 'markdown', score: 0.75 },
  { path: 'docs/benchmarks.md', size: 5100, tokens: 1250, language: 'markdown', score: 0.58 },
  { path: 'docs/contributing.md', size: 3400, tokens: 830, language: 'markdown', score: 0.45 },
  { path: 'docs/token-budgets.md', size: 4200, tokens: 1030, language: 'markdown', score: 0.67 },

  // scripts
  { path: 'scripts/build.sh', size: 1400, tokens: 340, language: 'bash', score: 0.52 },
  { path: 'scripts/release.sh', size: 2100, tokens: 510, language: 'bash', score: 0.48 },
  { path: 'scripts/test_coverage.sh', size: 980, tokens: 240, language: 'bash', score: 0.41 },
  { path: 'scripts/lint.sh', size: 850, tokens: 210, language: 'bash', score: 0.44 },
  { path: 'scripts/generate_fixtures.go', size: 4800, tokens: 1180, language: 'go', score: 0.39 },

  // additional modules for depth (~150 files total)
  { path: 'pkg/scanner/symlinks.go', size: 3100, tokens: 760, language: 'go', score: 0.64 },
  { path: 'pkg/scanner/device.go', size: 1900, tokens: 460, language: 'go', score: 0.47 },
  { path: 'pkg/scanner/fifo.go', size: 1700, tokens: 420, language: 'go', score: 0.43 },
  { path: 'pkg/scanner/submodule.go', size: 3800, tokens: 930, language: 'go', score: 0.73 },
  { path: 'pkg/scanner/submodule_test.go', size: 4200, tokens: 1030, language: 'go', score: 0.51 },

  { path: 'pkg/git/tags.go', size: 2400, tokens: 590, language: 'go', score: 0.49 },
  { path: 'pkg/git/remotes.go', size: 2900, tokens: 710, language: 'go', score: 0.53 },
  { path: 'pkg/git/status.go', size: 4100, tokens: 1010, language: 'go', score: 0.84 },
  { path: 'pkg/git/stash.go', size: 2300, tokens: 560, language: 'go', score: 0.42 },
  { path: 'pkg/git/tree.go', size: 4800, tokens: 1180, language: 'go', score: 0.78 },

  { path: 'pkg/rank/frequency.go', size: 3200, tokens: 780, language: 'go', score: 0.71 },
  { path: 'pkg/rank/proximity.go', size: 3900, tokens: 950, language: 'go', score: 0.75 },
  { path: 'pkg/rank/import_graph.go', size: 6800, tokens: 1670, language: 'go', score: 0.87 },
  { path: 'pkg/rank/import_graph_test.go', size: 5400, tokens: 1320, language: 'go', score: 0.59 },

  { path: 'pkg/packer/json.go', size: 3200, tokens: 780, language: 'go', score: 0.68 },
  { path: 'pkg/packer/header.go', size: 2100, tokens: 510, language: 'go', score: 0.77 },
  { path: 'pkg/packer/summary.go', size: 2800, tokens: 680, language: 'go', score: 0.81 },
  { path: 'pkg/packer/minify.go', size: 3700, tokens: 910, language: 'go', score: 0.7 },

  { path: 'pkg/redact/credit_cards.go', size: 3100, tokens: 760, language: 'go', score: 0.65 },
  { path: 'pkg/redact/passwords.go', size: 3900, tokens: 960, language: 'go', score: 0.83 },
  { path: 'pkg/redact/jwt.go', size: 2800, tokens: 690, language: 'go', score: 0.79 },
  { path: 'pkg/redact/ssh_keys.go', size: 3400, tokens: 830, language: 'go', score: 0.81 },

  { path: 'pkg/signatures/sql.go', size: 2900, tokens: 710, language: 'go', score: 0.58 },
  { path: 'pkg/signatures/shell.go', size: 2600, tokens: 640, language: 'go', score: 0.54 },
  { path: 'pkg/signatures/proto.go', size: 3300, tokens: 810, language: 'go', score: 0.66 },
  { path: 'pkg/signatures/graphql.go', size: 3100, tokens: 760, language: 'go', score: 0.61 },

  { path: 'pkg/server/cors.go', size: 1900, tokens: 460, language: 'go', score: 0.6 },
  { path: 'pkg/server/health.go', size: 1200, tokens: 290, language: 'go', score: 0.4 },
  { path: 'pkg/server/context.go', size: 1800, tokens: 440, language: 'go', score: 0.63 },
  { path: 'pkg/server/response.go', size: 2400, tokens: 590, language: 'go', score: 0.72 },

  { path: 'internal/tokens/vocab.json', size: 84000, tokens: 21000, language: 'json', score: 0.28 },
  { path: 'internal/tokens/splits.txt', size: 12000, tokens: 3000, language: 'text', score: 0.2 },
  { path: 'internal/testutil/corpus/small.go', size: 1400, tokens: 340, language: 'go', score: 0.35 },
  { path: 'internal/testutil/corpus/medium.go', size: 5200, tokens: 1270, language: 'go', score: 0.38 },
  { path: 'internal/testutil/corpus/secrets.env', size: 890, tokens: 220, language: 'text', score: 0.31 },

  { path: '.github/workflows/ci.yml', size: 2400, tokens: 590, language: 'yaml', score: 0.63 },
  { path: '.github/workflows/release.yml', size: 3100, tokens: 760, language: 'yaml', score: 0.58 },
  { path: '.github/dependabot.yml', size: 450, tokens: 110, language: 'yaml', score: 0.25 },
  { path: '.golangci.yml', size: 3800, tokens: 930, language: 'yaml', score: 0.54 },
];

/** Reset mock auth counters (used by the mock login flow). */
export function resetMockAuth(): void {
  mockLoginAttempts = 0;
  mockLockoutUntil = 0;
}

export function setMockAuthenticated(v: boolean): void {
  mockAuthenticated = v;
}

export function setMockRecents(v: RecentProject[]): void {
  mockRecents = v;
}

export function setMockSettings(v: SettingsData): void {
  mockSettings = v;
}

export function setMockLockout(until: number, attempts = 0): void {
  mockLockoutUntil = until;
  mockLoginAttempts = attempts;
}

export function recordMockAttempt(): number {
  mockLoginAttempts++;
  return mockLoginAttempts;
}
