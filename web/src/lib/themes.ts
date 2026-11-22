export interface ThemeDefinition {
  id: string;
  name: string;
  category: 'dark' | 'light';
  bg: string;
  accent: string;
  surface: string;
  description: string;
}

import { GENERATED_THEMES } from './themes-generated';

export const THEMES: ThemeDefinition[] = GENERATED_THEMES;

export const DEFAULT_THEME_ID = 'catppuccin-mocha';

export function getThemeById(id: string): ThemeDefinition {
  return THEMES.find((t) => t.id === id) || THEMES[0];
}

/** Keep the browser/PWA chrome color in step with the active theme. */
export function applyThemeColor(id: string): void {
  if (typeof document === 'undefined') return;
  const meta = document.querySelector<HTMLMetaElement>('meta[name="theme-color"]');
  if (!meta) return;
  meta.content = getThemeById(id).bg;
}
