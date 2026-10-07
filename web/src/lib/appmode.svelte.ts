/**
 * App-mode state (`sift serve --app`).
 *
 * The session cookie is the source of truth, not a URL hint, so `appMode`
 * survives reloads inside one server run. Everything else in the UI reads this
 * module rather than sniffing for app mode itself.
 */

import { api, type ApiMeta } from './api';

export const appState = $state({
  /** True when the window was opened by `sift serve --app`. */
  appMode: false,
  /** Set once the server is gone while the window is still open. */
  stopped: false,
  /** Set while the heartbeat stream is reconnecting (laptop sleep, etc). */
  reconnecting: false,
});

/** Read the app flag out of the boot metadata. */
export function applyMeta(meta: ApiMeta | null): void {
  if (!meta) return;
  appState.appMode = meta.app === true;
}

/**
 * Exchange a one-time launch token for a session. The token arrives in the URL
 * fragment, so it never reaches the server logs.
 */
export async function exchangeLaunchToken(token: string): Promise<boolean> {
  try {
    await api.launchSession(token);
    return true;
  } catch {
    // Expired or already used: fall through to the normal password screen.
    return false;
  }
}

/**
 * Keep the liveness stream open while the window exists. The server counts
 * connections and exits shortly after the last one goes away, so this must not
 * stop until the window closes.
 *
 * The server sends nothing on a timer, so there is no polling here beyond the
 * browser's own EventSource retry.
 */
export function startHeartbeat(): void {
  let attempts = 0;
  let wasOpen = false;
  const MAX_ATTEMPTS = 5;

  const connect = () => {
    const source = new EventSource('/api/app/heartbeat');
    source.onopen = () => {
      attempts = 0;
      wasOpen = true;
      appState.reconnecting = false;
    };
    const giveUp = (ended: boolean) => {
      appState.reconnecting = false;
      // The stream lived and then died: the server is gone while the window
      // is still open, so show the ended screen. A stream that never opened
      // is just app mode being off — stay quiet.
      if (ended && wasOpen) appState.stopped = true;
    };
    source.onerror = () => {
      source.close();
      // A 401/404 is an answer, not a hiccup: app mode is off or the session
      // is gone. Retrying would spam the console and show a misleading
      // "reconnecting" banner, so give up and clear the notice.
      if (source.status === 401 || source.status === 404) {
        giveUp(false);
        return;
      }
      // A closed port fails instantly, so a few quick retries settle it.
      if (attempts++ >= MAX_ATTEMPTS) {
        giveUp(true);
        return;
      }
      appState.reconnecting = true;
      setTimeout(connect, 2000);
    };
  };
  connect();
}