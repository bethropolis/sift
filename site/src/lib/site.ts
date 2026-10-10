export const base = import.meta.env.BASE_URL.replace(/\/+$/, '');

export function toBase(path: string): string {
  if (!path) return base || '/';
  if (
    path.startsWith('http://') ||
    path.startsWith('https://') ||
    path.startsWith('mailto:') ||
    path.startsWith('#')
  ) {
    return path;
  }
  const clean = path.startsWith('/') ? path : `/${path}`;
  return `${base}${clean}`;
}

export const repoUrl = 'https://github.com/bethropolis/sift';
export const curlInstall = 'curl -fsSL https://bethropolis.github.io/sift/install.sh | sh';
