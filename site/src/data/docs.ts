export interface DocNavPage {
  title: string;
  slug: string;
  desc: string;
}

export interface DocNavGroup {
  group: string;
  pages: DocNavPage[];
}

// Sidebar order + docs landing cards. Mirrors the grouping in
// docs/README.md (the canonical nav order).
export const DOC_NAV: DocNavGroup[] = [
  {
    group: 'Getting started',
    pages: [
      {
        title: 'Installation',
        slug: 'installation',
        desc: 'Install on macOS, Linux, Windows, or FreeBSD, or build from source.',
      },
      {
        title: 'Usage',
        slug: 'usage',
        desc: 'Commands, output destinations, filtering, and examples.',
      },
    ],
  },
  {
    group: 'Features in depth',
    pages: [
      {
        title: 'Interactive picker',
        slug: 'picker',
        desc: 'The dual-pane TUI — navigation, per-file modes, and task prompts.',
      },
      {
        title: 'Configuration and profiles',
        slug: 'configuration',
        desc: '.sift.toml keys, profiles, targets, and precedence.',
      },
      {
        title: 'Output formats and prompts',
        slug: 'output',
        desc: 'The four styles, signature mode, and clipboard output.',
      },
      {
        title: 'Incremental deltas',
        slug: 'delta',
        desc: 'Feed an LLM only what changed since the last dump.',
      },
      {
        title: 'Dependency expansion',
        slug: 'dependencies',
        desc: 'Auto-include imports of selected files as compressed context.',
      },
      {
        title: 'Following imports',
        slug: 'follow',
        desc: 'Render a file plus its import-graph neighbors, transitively.',
      },
      {
        title: 'MCP server',
        slug: 'mcp',
        desc: 'Expose sift to coding agents over stdio.',
      },
      {
        title: 'Browser picker',
        slug: 'serve',
        desc: 'sift serve — the embedded web UI and its protocol.',
      },
      {
        title: 'Serve protocol',
        slug: 'serve-protocol',
        desc: 'Endpoints, auth, and the threat model for sift serve.',
      },
    ],
  },
];

export const DOC_ORDER: string[] = DOC_NAV.flatMap((g) => g.pages.map((p) => p.slug));

export function docNeighbors(slug: string): { prev?: DocNavPage; next?: DocNavPage } {
  const i = DOC_ORDER.indexOf(slug);
  if (i === -1) return {};
  const flat = DOC_NAV.flatMap((g) => g.pages);
  return {
    prev: i > 0 ? flat[i - 1] : undefined,
    next: i < flat.length - 1 ? flat[i + 1] : undefined,
  };
}
