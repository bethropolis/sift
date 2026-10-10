import { defineConfig } from 'astro/config';
import sitemap from '@astrojs/sitemap';

export default defineConfig({
  site: 'https://bethropolis.github.io/sift',
  base: '/sift',
  trailingSlash: 'always',
  integrations: [sitemap()],
});
