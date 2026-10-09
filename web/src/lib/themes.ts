export interface ThemeDefinition {
  id: string;
  name: string;
  category: 'dark' | 'light';
  bg: string;
  accent: string;
  surface: string;
  description: string;
}

export const THEMES: ThemeDefinition[] = [
  {
    id: 'catppuccin-mocha',
    name: 'Catppuccin Mocha',
    category: 'dark',
    bg: '#1e1e2e',
    accent: '#cba6f7',
    surface: '#181825',
    description: 'Soothing pastel theme for high-spirited developers',
  },
  {
    id: 'tokyo-night',
    name: 'Tokyo Night',
    category: 'dark',
    bg: '#1a1b26',
    accent: '#7aa2f7',
    surface: '#16161e',
    description: 'A clean dark theme celebrating Tokyo night lights',
  },
  {
    id: 'nord',
    name: 'Nord',
    category: 'dark',
    bg: '#2e3440',
    accent: '#88c0d0',
    surface: '#242933',
    description: 'An arctic, north-bluish clean and elegant palette',
  },
  {
    id: 'gruvbox-dark',
    name: 'Gruvbox Dark',
    category: 'dark',
    bg: '#282828',
    accent: '#fe8019',
    surface: '#1d2021',
    description: 'Retro groove warm and earthy dark colors',
  },
  {
    id: 'gruvbox-light',
    name: 'Gruvbox Light',
    category: 'light',
    bg: '#fbf1c7',
    accent: '#d65d0e',
    surface: '#f2e5bc',
    description: 'Warm paper retro groove light colors',
  },
  {
    id: 'dracula',
    name: 'Dracula',
    category: 'dark',
    bg: '#282a36',
    accent: '#bd93f9',
    surface: '#21222c',
    description: 'Famous gothic high-contrast dark theme',
  },
  {
    id: 'rose-pine',
    name: 'Rose Pine',
    category: 'dark',
    bg: '#191724',
    accent: '#ebbcba',
    surface: '#1f1d2e',
    description: 'Minimal, muted warmth and pine undertones',
  },
  {
    id: 'one-dark',
    name: 'One Dark',
    category: 'dark',
    bg: '#21252b',
    accent: '#61afef',
    surface: '#1d1f23',
    description: 'The iconic Atom and modern editor classic',
  },
  {
    id: 'github-dark',
    name: 'GitHub Dark',
    category: 'dark',
    bg: '#0d1117',
    accent: '#58a6ff',
    surface: '#161b22',
    description: 'GitHub default dark code workspace',
  },
  {
    id: 'github-light',
    name: 'GitHub Light',
    category: 'light',
    bg: '#ffffff',
    accent: '#0969da',
    surface: '#f6f8fa',
    description: 'Crisp GitHub default light workspace',
  },
  {
    id: 'monokai',
    name: 'Monokai',
    category: 'dark',
    bg: '#272822',
    accent: '#a6e22e',
    surface: '#1e1f1c',
    description: 'Classic vibrant high-contrast hacker palette',
  },
  {
    id: 'classic-dark',
    name: 'Classic Zinc (Dark)',
    category: 'dark',
    bg: '#0b0d12',
    accent: '#d4a359',
    surface: '#131720',
    description: 'Quiet zinc and slate with warm brass accent',
  },
  {
    id: 'classic-light',
    name: 'Classic Zinc (Light)',
    category: 'light',
    bg: '#ffffff',
    accent: '#b58532',
    surface: '#f6f8fa',
    description: 'Clean paper and ink with subtle brass accent',
  },
  {
    id: 'everforest-dark',
    name: 'Everforest Dark',
    category: 'dark',
    bg: '#2b3339',
    accent: '#a7c080',
    surface: '#232a2e',
    description: 'Earthy greens and warm paper tones, easy on the eyes',
  },
  {
    id: 'kanagawa-wave',
    name: 'Kanagawa Wave',
    category: 'dark',
    bg: '#1f1f28',
    accent: '#7e9cd8',
    surface: '#16161d',
    description: 'Great wave paints muted ink and water tones',
  },
  {
    id: 'night-owl',
    name: 'Night Owl',
    category: 'dark',
    bg: '#011627',
    accent: '#82aaff',
    surface: '#010e1a',
    description: 'Deep blue night for late sessions',
  },
  {
    id: 'catppuccin-latte',
    name: 'Catppuccin Latte',
    category: 'light',
    bg: '#eff1f5',
    accent: '#8839ef',
    surface: '#e6e9ef',
    description: 'Soothing pastel light theme for daytime coding',
  },
  {
    id: 'tokyo-day',
    name: 'Tokyo Night Day',
    category: 'light',
    bg: '#e1e2e7',
    accent: '#2e7de9',
    surface: '#d0d5e3',
    description: 'Tokyo night lights inverted for bright rooms',
  },
  {
    id: 'rose-pine-dawn',
    name: 'Rosé Pine Dawn',
    category: 'light',
    bg: '#faf4ed',
    accent: '#907aa9',
    surface: '#fffaf3',
    description: 'Warm morning light with pine undertones',
  },
];

export const DEFAULT_THEME_ID = 'catppuccin-mocha';

export function getThemeById(id: string): ThemeDefinition {
  return THEMES.find((t) => t.id === id) || THEMES[0];
}
