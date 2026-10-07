/**
 * Themed favicon.
 *
 * The favicon is an inline SVG data URI (no asset files, per design). Browsers
 * render favicons in an isolated document with no access to our CSS or the
 * `data-theme` attribute, so theming means rewriting the data URI whenever the
 * effective theme changes. Best-effort by nature: some browsers cache tab
 * icons aggressively and repaint only after a reload.
 */

import { getThemeById } from './themes';

/** Default mark colors: zinc dark + brass, matching the shipped static icon. */
const DEFAULT_BG = '#14171c';
const DEFAULT_ACCENT = '#e3a63f';

function hex(c: string): string {
  return c.replace('#', '');
}

/**
 * Build the favicon data URI for a theme id. 'system' resolves to the
 * concrete OS-matched theme the caller passes in (the app never keeps
 * 'system' on the document after boot).
 */
export function faviconDataUri(themeId: string): string {
  const t = getThemeById(themeId);
  const bg = hex(t.bg || DEFAULT_BG);
  const accent = hex(t.accent || DEFAULT_ACCENT);
  const svg =
    `<svg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 32 32'>` +
    `<rect width='32' height='32' rx='6' fill='%23${bg}'/>` +
    `<rect x='6' y='11' width='14' height='3' rx='1.5' fill='%23${accent}'/>` +
    `<rect x='12' y='18' width='14' height='3' rx='1.5' fill='%23${accent}'/>` +
    `</svg>`;
  return `data:image/svg+xml,${svg}`;
}

/**
 * Point every <link rel="icon"> at the themed data URI. Rewrites `href` in
 * place so the existing link keeps its position and attributes. No-op before
 * the document head exists.
 */
export function applyFavicon(themeId: string): void {
  if (typeof document === 'undefined') return;
  const href = faviconDataUri(themeId);
  const links = document.querySelectorAll<HTMLLinkElement>('link[rel~="icon"]');
  if (links.length === 0) return;
  for (const link of links) {
    if (link.href !== href) link.href = href;
  }
}