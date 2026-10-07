/**
 * Live `fetch` implementations for every `api` method.
 *
 * One function per endpoint; 401 always bounces to `#/login`. No mock
 * state, no latency simulation - pure HTTP against the Go server.
 */

import { handle401, loginRequest } from './live';
import type {
  ApiMeta,
  BrowseResult,
  FileContentResult,
  PackPayload,
  PackResult,
  RecentProject,
  SettingsData,
  SmartSelectResult,
  TreeResult,
} from './types';

export async function getMeta(): Promise<ApiMeta> {
    const res = await fetch('/api/meta');
    if (res.status === 401) handle401();
    if (!res.ok) throw new Error(`HTTP ${res.status}: ${res.statusText}`);
    return res.json();
}

export async function login(password: string): Promise<void> {
    await loginRequest({ password }, 'password');
    return;
}

export async function loginWithToken(token: string): Promise<void> {
    await loginRequest({ token }, 'token');
    return;
}

export async function logout(): Promise<void> {
    await fetch('/api/logout', { method: 'POST', headers: { 'Content-Type': 'application/json' } }).catch(() => {});
    return;
}

export async function getRecents(): Promise<RecentProject[]> {
    const res = await fetch('/api/recents');
    if (res.status === 401) handle401();
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    return res.json();
}

export async function recordRecent(root: string): Promise<void> {
    const res = await fetch('/api/recents', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ root }),
    });
    if (res.status === 401) handle401();
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    return;
}

export async function deleteRecent(root: string): Promise<void> {
    const res = await fetch(`/api/recents?root=${encodeURIComponent(root)}`, {
      method: 'DELETE',
    });
    if (res.status === 401) handle401();
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    return;
}

export async function browse(pathQuery: string, hidden = false): Promise<BrowseResult> {
    const res = await fetch(
      `/api/browse?path=${encodeURIComponent(pathQuery)}${hidden ? '&hidden=1' : ''}`,
    );
    if (res.status === 401) handle401();
    if (res.status === 403) throw new Error("Couldn't read that folder: outside allowed roots");
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    return res.json();
}

export async function getTree(root: string): Promise<TreeResult> {
    const res = await fetch(`/api/tree?root=${encodeURIComponent(root)}`);
    if (res.status === 401) handle401();
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    return res.json();
}

export async function getFile(root: string, path: string, mode: 'full' | 'sigs'): Promise<FileContentResult> {
    const res = await fetch(
      `/api/file?root=${encodeURIComponent(root)}&path=${encodeURIComponent(path)}&mode=${mode}`,
    );
    if (res.status === 401) handle401();
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    return res.json();
}

export async function smartSelect(root: string, budget: number): Promise<SmartSelectResult> {
    const res = await fetch('/api/smart-select', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ root, budget }),
    });
    if (res.status === 401) handle401();
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    return res.json();
}

export async function pack(payload: PackPayload): Promise<PackResult> {
    const res = await fetch('/api/pack', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
    if (res.status === 401) handle401();
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    return res.json();
}

export async function getSettings(): Promise<SettingsData> {
    const res = await fetch('/api/settings');
    if (res.status === 401) handle401();
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    return res.json();
}

export async function saveSettings(settings: SettingsData): Promise<void> {
    const res = await fetch('/api/settings', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(settings),
    });
    if (res.status === 401) handle401();
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    return;
}

/**
 * Exchange a one-time app-mode launch token for a session cookie. The token
 * travels in the URL fragment, so it is submitted once here and then cleared.
 */
export async function launchSession(token: string): Promise<{ app: boolean }> {
    const res = await fetch('/api/session/launch', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify({ token }),
    });
    if (!res.ok) throw new Error(`HTTP ${res.status}`);
    return res.json();
}


