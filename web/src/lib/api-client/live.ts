/**
 * Live HTTP client for the Go serve backend (`/api/*`).
 *
 * Stateless: the browser holds selections and sends them per request; the
 * server keeps only auth + small state-dir files. All non-GET bodies are
 * JSON; a 401 always bounces to `#/login` via `handle401`.
 */

import { navigate } from '../router.svelte';
import type { LoginError } from './types';

export function loginError(message: string, retryAfter?: number): LoginError {
  const err = new Error(message) as LoginError;
  if (retryAfter !== undefined) err.retryAfter = retryAfter;
  return err;
}

/** Bounce to the login route on 401. Never returns. */
export function handle401(): never {
  navigate('/login');
  throw new Error('401 Unauthorized: please log in');
}

/** POST /api/login with `{password}` or `{token}`. */
export async function loginRequest(body: Record<string, string>, kind: 'token' | 'password'): Promise<void> {
  const res = await fetch('/api/login', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(body),
  });
  // Tokens rotate on every server restart, so a stale login URL is the most
  // common 401 here: say exactly that instead of "wrong password".
  if (res.status === 401) {
    throw loginError(
      kind === 'token'
        ? 'Invalid or expired token — copy the fresh login URL from the `sift serve` terminal and reopen it.'
        : 'Wrong password',
    );
  }
  if (res.status === 429) {
    const data = await res.json().catch(() => ({ retryAfter: 15 }));
    throw loginError(`Too many attempts, try again in ${data.retryAfter || 15}s`, data.retryAfter || 15);
  }
  if (!res.ok) throw new Error(`Login failed with HTTP ${res.status}`);
}

/** Throw unless `res.ok`, mapping 401 → login bounce. */
export async function requireOk(res: Response): Promise<void> {
  if (res.status === 401) handle401();
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
}
