import './app.css';
import './themes-generated.css';
import App from './App.svelte';
import { mount } from 'svelte';
import { applyFavicon } from './lib/favicon';

// Resolve "system" to a concrete theme before first paint so every render
// has a complete variable set. The server-stored theme replaces it once
// known. (No inline script: CSP forbids it; the module runs pre-paint.)
const rootEl = document.documentElement;
if (!rootEl.hasAttribute('data-theme')) {
  const dark =
    typeof window.matchMedia === 'function' &&
    window.matchMedia('(prefers-color-scheme: dark)').matches;
  rootEl.setAttribute('data-theme', dark ? 'classic-dark' : 'classic-light');
}

// Tint the favicon to the boot theme before first paint, so the tab never
// shows the stock icon next to a themed page.
applyFavicon(rootEl.getAttribute('data-theme') ?? 'classic-dark');

mount(App, { target: document.getElementById('root')! });
