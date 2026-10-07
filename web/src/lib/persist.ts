/**
 * Local UI preferences (the only localStorage use in the app).
 *
 * Scoped to pure layout/view state that carries no security meaning and
 * cannot cause cache-invalidation problems, because nothing here is
 * server data:
 *   - sidebar open/closed, output tab, file-outline open/closed
 *   - last opened project (restores the workspace on reload)
 *   - output format (mirrors the server default so first load matches)
 *
 * Rules: one namespaced key, JSON, every value validated on read, and any
 * read/parse failure silently falls back to the default. Server-persisted
 * state (recents, settings, theme, budget) keeps using the API, not this.
 * The CI hygiene gate keeps `localStorage` confined to this file.
 */

const KEY = 'sift.ui.v1';

export interface UIPrefs {
  /** Files sidebar expanded. */
  sidebarOpen: boolean;
  /** Workspace tab: 'preview' | 'output'. */
  tab: 'preview' | 'output';
  /** Output view file outline visible. */
  outlineOpen: boolean;
  /** Last opened project root (workspace restore). */
  lastProject: string;
  /** Output document style (xml | markdown | plain). */
  style: string;
}

const DEFAULTS: UIPrefs = {
  sidebarOpen: true,
  tab: 'preview',
  outlineOpen: true,
  lastProject: '',
  style: 'xml',
};

const STYLES = new Set(['xml', 'markdown', 'plain']);
const TABS = new Set(['preview', 'output']);

function read(): Partial<UIPrefs> {
  if (typeof localStorage === 'undefined') return {};
  try {
    const raw = localStorage.getItem(KEY);
    if (!raw) return {};
    const v = JSON.parse(raw);
    return v && typeof v === 'object' ? (v as Partial<UIPrefs>) : {};
  } catch {
    return {};
  }
}

export function getUIPrefs(): UIPrefs {
  const v = read();
  return {
    sidebarOpen: typeof v.sidebarOpen === 'boolean' ? v.sidebarOpen : DEFAULTS.sidebarOpen,
    tab: typeof v.tab === 'string' && TABS.has(v.tab) ? (v.tab as UIPrefs['tab']) : DEFAULTS.tab,
    outlineOpen: typeof v.outlineOpen === 'boolean' ? v.outlineOpen : DEFAULTS.outlineOpen,
    lastProject: typeof v.lastProject === 'string' ? v.lastProject : DEFAULTS.lastProject,
    style: typeof v.style === 'string' && STYLES.has(v.style) ? v.style : DEFAULTS.style,
  };
}

/** Merge a partial update into the stored prefs. No-op when storage fails. */
export function setUIPrefs(patch: Partial<UIPrefs>): void {
  if (typeof localStorage === 'undefined') return;
  try {
    localStorage.setItem(KEY, JSON.stringify({ ...getUIPrefs(), ...patch }));
  } catch {
    // Private mode / quota: preferences are best-effort.
  }
}