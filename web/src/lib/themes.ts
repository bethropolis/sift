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
