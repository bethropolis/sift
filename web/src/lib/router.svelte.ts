// Hash router state shared across the app. Hash routing (no history mode)
// means the Go server needs no SPA fallback and subpath proxies keep working.
export type RouteType = 'login' | 'projects' | 'workspace';

export interface RouteState {
  path: string;
  route: RouteType;
  projectRoot: string | null;
  /** Raw login token from #/login?token= (fragment-only, never sent). */
  loginToken: string | null;
  /** One-time app-mode launch token from #launch= (fragment-only, never sent). */
  launchToken: string | null;
  /** One-time mobile pairing token from #pair= (fragment-only, never sent). */
  pairToken: string | null;
}

export function parseHash(hash: string): RouteState {
  // #launch=<token> arrives as a bare fragment (no '#/' path), so it is peeled
  // off before path parsing and the app resolves it into a session.
  let rest = hash;
  let launchToken: string | null = null;
  let pairToken: string | null = null;
  const bare = /^(?:#|\/?)launch=([^&]+)(?:&(.*))?$/.exec(rest);
  if (bare) {
    launchToken = bare[1];
    rest = bare[2] ? `#/${bare[2]}` : '';
  } else {
    const pair = /^(?:#|\/?)pair=([^&]+)(?:&(.*))?$/.exec(rest);
    if (pair) {
      pairToken = pair[1];
      rest = pair[2] ? `#/${pair[2]}` : '';
    }
  }
  const [hashPath, query] = rest.split('?');
  const clean = hashPath.replace(/^#\/?/, '').replace(/^\//, '');
  if (clean === 'login') {
    let loginToken: string | null = null;
    try {
      loginToken = new URLSearchParams(query ?? '').get('token');
    } catch {
      loginToken = null;
    }
    return { path: '/login', route: 'login', projectRoot: null, loginToken, launchToken, pairToken };
  }
  if (!clean || clean === 'projects') {
    return { path: '/projects', route: 'projects', projectRoot: null, loginToken: null, launchToken, pairToken };
  }
  // Former settings page: settings now float as a modal, so a stale
  // bookmark lands on projects instead of a dead route.
  if (clean === 'settings') {
    return { path: '/projects', route: 'projects', projectRoot: null, loginToken: null, launchToken, pairToken };
  }
  if (clean.startsWith('p/')) {
    const rawRoot = clean.slice(2);
    try {
      return { path: `/p/${rawRoot}`, route: 'workspace', projectRoot: decodeURIComponent(rawRoot), loginToken: null, launchToken, pairToken };
    } catch {
      return { path: `/p/${rawRoot}`, route: 'workspace', projectRoot: rawRoot, loginToken: null, launchToken, pairToken };
    }
  }
  return { path: '/projects', route: 'projects', projectRoot: null, loginToken: null, launchToken, pairToken };
}

export function navigate(path: string) {
  const targetHash = path.startsWith('#') ? path : `#${path.startsWith('/') ? path : '/' + path}`;
  if (window.location.hash !== targetHash) {
    window.location.hash = targetHash;
  }
}

/** Remove the token fragment after login so it never lingers in the URL. */
export function clearHashQuery() {
  const h = window.location.hash;
  // A launch URL keeps no path: land on projects rather than re-parsing it.
  if (h === '#launch' || h.startsWith('#launch=') || h === '#pair' || h.startsWith('#pair=')) {
    history.replaceState(null, '', '#/projects');
    return;
  }
  const hashPath = h.split('?')[0];
  history.replaceState(null, '', hashPath || '#/projects');
}
