import { svelte } from '@sveltejs/vite-plugin-svelte';
import { defineConfig } from 'vite';

export default defineConfig({
  base: './',
  plugins: [
    svelte({
      compilerOptions: {
        // Keep styles in an external CSS file: no inline <style> injection,
        // which keeps every page compatible with the serve CSP.
        css: 'external',
      },
    }),
  ],
  build: {
    outDir: 'dist',
    // Never delete dist/.keep: the file must exist in git so the Go embed
    // pattern always has at least one match.
    emptyOutDir: false,
    modulePreload: { polyfill: false },
  },
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://127.0.0.1:7777',
        // Rewrite both Host and Origin so the Go Host/Origin checks see the
        // proxied request as same-origin. Never weaken the server checks.
        changeOrigin: true,
        configure: (proxy) => {
          proxy.on('proxyReq', (proxyReq) => {
            proxyReq.setHeader('Origin', 'http://127.0.0.1:7777');
          });
        },
      },
    },
  },
});
