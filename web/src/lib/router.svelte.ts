// Hash router state shared across the app. Hash routing (no history mode)
// means the Go server needs no SPA fallback and subpath proxies keep working.
export type RouteType = 'login' | 'projects' | 'workspace' | 'settings';

export interface RouteState {
  path: string;
  route: RouteType;
  projectRoot: string | null;
  /** Raw login token from #/login?token= (fragment-only, never sent). */
  loginToken: string | null;
}

export function parseHash(hash: string): RouteState {
  const [hashPath, query] = hash.split('?');
  const clean = hashPath.replace(/^#\/?/, '').replace(/^\//, '');
  if (clean === 'login') {
    let loginToken: string | null = null;
    try {
      loginToken = new URLSearchParams(query ?? '').get('token');
    } catch {
      loginToken = null;
    }
    return { path: '/login', route: 'login', projectRoot: null, loginToken };
  }
  if (!clean || clean === 'projects') {
    return { path: '/projects', route: 'projects', projectRoot: null, loginToken: null };
  }
  if (clean === 'settings') {
    return { path: '/settings', route: 'settings', projectRoot: null, loginToken: null };
  }
  if (clean.startsWith('p/')) {
    const rawRoot = clean.slice(2);
    try {
      return { path: `/p/${rawRoot}`, route: 'workspace', projectRoot: decodeURIComponent(rawRoot), loginToken: null };
    } catch {
      return { path: `/p/${rawRoot}`, route: 'workspace', projectRoot: rawRoot, loginToken: null };
    }
  }
  return { path: '/projects', route: 'projects', projectRoot: null, loginToken: null };
}

export function navigate(path: string) {
  const targetHash = path.startsWith('#') ? path : `#${path.startsWith('/') ? path : '/' + path}`;
  if (window.location.hash !== targetHash) {
    window.location.hash = targetHash;
  }
}

/** Remove the token fragment after login so it never lingers in the URL. */
export function clearHashQuery() {
  const hashPath = window.location.hash.split('?')[0];
  history.replaceState(null, '', hashPath || '#/projects');
}
