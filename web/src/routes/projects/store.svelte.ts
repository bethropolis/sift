/**
 * Shared state + async actions for the Projects route.
 *
 * A `.svelte.ts` rune module (runes are legal here per the Svelte docs) so the
 * shell, `RecentsList`, and `FolderBrowser` share one reactive object without
 * prop-drilling. `browseReq` is module-private for the rapid-click race guard.
 */

import { api, type ApiMeta, type BrowseResult, type RecentProject } from '../../lib/api';

export const projects = $state({
  // Recents view
  recents: [] as RecentProject[],
  loadingRecents: true,
  // Folder browser view
  browsePath: '',
  browseData: null as BrowseResult | null,
  browseLoading: false,
  browseError: null as string | null,
  // Folder-browser prefs (shared settings)
  showHidden: false,
  fileSort: 'name' as 'name' | 'updated',
  // Shared shell state
  searchQuery: '',
  activeView: 'recents' as 'recents' | 'browse',
  selectedIndex: 0,
  hoveredRoot: null as string | null,
});

// Race guard: rapid clicks must never show a stale directory.
let browseReq = 0;

export async function loadRecents(): Promise<void> {
  try {
    projects.loadingRecents = true;
    projects.recents = await api.getRecents();
  } catch (err) {
    console.error('Failed to load recent projects', err);
  } finally {
    projects.loadingRecents = false;
  }
}

export async function loadBrowse(path: string, hidden = projects.showHidden): Promise<void> {
  const id = ++browseReq;
  try {
    projects.browseLoading = true;
    projects.browseError = null;
    const res = await api.browse(path, hidden);
    if (id !== browseReq) return;
    projects.browseData = res;
    projects.browsePath = res.path;
  } catch (err) {
    if (id !== browseReq) return;
    projects.browseError =
      err instanceof Error ? err.message : "Couldn't read that folder: outside allowed roots";
  } finally {
    if (id === browseReq) projects.browseLoading = false;
  }
}

export async function removeRecent(root: string): Promise<void> {
  try {
    await api.deleteRecent(root);
    projects.recents = projects.recents.filter((r) => r.root !== root);
  } catch (err) {
    console.error('Failed to delete recent project', err);
  }
}

/** Start dir for a fresh browse: current path, else meta hint, else first root. */
export function defaultStartDir(meta: ApiMeta | null): string {
  if (projects.browsePath) return projects.browsePath;
  if (meta?.defaultBrowse) return meta.defaultBrowse;
  if (meta?.roots?.[0]) return meta.roots[0];
  return '';
}

/** Flip to the Folder Browser tab, loading the default dir when we have one. */
export function switchToBrowse(meta: ApiMeta | null): void {
  projects.activeView = 'browse';
  const start = defaultStartDir(meta);
  if (start) void loadBrowse(start);
}

/** Pull showHidden/fileSort from the shared settings store. */
export async function loadBrowsePrefs(): Promise<void> {
  try {
    const s = await api.getSettings();
    projects.showHidden = s.showHidden;
    projects.fileSort = s.fileSort === 'updated' ? 'updated' : 'name';
  } catch {
    // prefs are cosmetic; defaults are fine
  }
}

/** Recents matching the search query (name, root, or branch). */
export function filterRecents(): RecentProject[] {
  const q = projects.searchQuery.trim().toLowerCase();
  if (!q) return projects.recents;
  return projects.recents.filter(
    (r) =>
      r.name.toLowerCase().includes(q) ||
      r.root.toLowerCase().includes(q) ||
      r.branch.toLowerCase().includes(q),
  );
}

/** Dir entries matching the query, sorted by the FileSort setting. */
export function filterEntries(): BrowseResult['entries'] {
  const q = projects.searchQuery.trim().toLowerCase();
  const dirs = (projects.browseData?.entries ?? []).filter((e) => e.isDir);
  const list = q ? dirs.filter((e) => e.name.toLowerCase().includes(q)) : [...dirs];
  if (projects.fileSort === 'updated') {
    list.sort((a, b) => (b.modTime || 0) - (a.modTime || 0));
  } else {
    list.sort((a, b) => a.name.localeCompare(b.name));
  }
  return list;
}

/** Breadcrumb segments of the current browse path. */
export function pathParts(): string[] {
  return projects.browsePath.split('/').filter(Boolean);
}

/** Last segment of the parent dir (the ".." row label). */
export function parentName(): string {
  const p = projects.browseData?.parent;
  return p ? p.split('/').filter(Boolean).pop() || '/' : '';
}
