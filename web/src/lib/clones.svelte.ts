/**
 * Temporary clones: the session list plus the URL helpers shared by the
 * Projects page, the clone modal, and the workspace. Server-owned state
 * only (nothing here is persisted; see lib/persist.ts for the storage rule).
 */

import { api, type TempClone } from './api';
import { toast } from './toast.svelte';

export const clones = $state<{ items: TempClone[] }>({ items: [] });

export async function loadClones(): Promise<void> {
  try {
    clones.items = await api.getClones();
  } catch (err) {
    console.error('Failed to load temporary clones', err);
  }
}

/** Add a freshly finished clone to the list (newest first, no duplicates). */
export function addClone(clone: TempClone): void {
  clones.items = [clone, ...clones.items.filter((c) => c.root !== clone.root)];
}

export async function removeClone(clone: TempClone): Promise<void> {
  try {
    await api.deleteClone(clone.root);
    clones.items = clones.items.filter((c) => c.root !== clone.root);
    toast({ id: 'clone-removed', kind: 'info', message: `Removed ${clone.name}`, duration: 2200 });
  } catch (err) {
    console.error('Failed to remove temporary clone', err);
    toast({ id: 'clone-removed', kind: 'error', message: `Couldn’t remove ${clone.name}` });
  }
}

/**
 * Whether the text in the search box is a remote repository URL we should
 * offer to clone. Loose on purpose: the server validates for real.
 */
export function looksLikeRepoUrl(text: string): boolean {
  const q = text.trim();
  if (/\s/.test(q)) return false;
  if (/^(https?|ssh|git):\/\/[^/]+\/./i.test(q)) return true;
  // scp-like: [user@]host:owner/repo (a colon before any slash).
  return /^(?:[\w.-]+@)?[\w.-]+\.[\w.-]+:[^/\s][^\s]*$/.test(q);
}

/** "https://host/owner/repo.git" → "repo" (display only). */
export function repoNameFromUrl(url: string): string {
  const path = url.trim().replace(/^[a-z]+:\/\/[^/]+/i, '').replace(/^[^:/]+:/, '');
  const last = path.replace(/\/+$/, '').split('/').pop() || '';
  return last.replace(/\.git$/, '') || 'repo';
}

/**
 * Whether a project root is one of the server's temporary clones. Matches
 * the on-disk shape (<tmp>/sift-clone-XXXX/<name>) rather than the session
 * list, so it is right even before that list has loaded (e.g. on reload).
 */
export function isTempCloneRoot(root: string): boolean {
  return /\/sift-clone-[^/]+\/[^/]+\/?$/.test(root);
}
