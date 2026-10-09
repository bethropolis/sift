<script lang="ts">
  import { api, type ApiMeta, type SettingsData } from './lib/api';
  import { parseHash, navigate, type RouteState } from './lib/router.svelte';
  import { buildContextMenuItems } from './lib/contextmenu';
  import { closeCtxMenu, ctxMenuState, openCtxMenu } from './lib/ctxmenu.svelte';
import { applyFavicon } from './lib/favicon';
import { applyMeta, appState, exchangeLaunchToken, startHeartbeat } from './lib/appmode.svelte';
import { clearHashQuery } from './lib/router.svelte';
  import GlobalNav from './components/GlobalNav.svelte';
  import ContextMenu from './components/ContextMenu.svelte';
  import ToastHost from './components/ToastHost.svelte';
  import ShortcutsModal from './components/ShortcutsModal.svelte';
  import ThemePickerModal from './components/ThemePickerModal.svelte';
  import Login from './routes/Login.svelte';
  import Projects from './routes/Projects.svelte';
  import Workspace from './routes/Workspace.svelte';
  import type { Component } from 'svelte';

  let routeState = $state<RouteState>(parseHash(window.location.hash || '#/projects'));
  let meta = $state<ApiMeta | null>(null);
  // Null until the server theme arrives: with no data-theme the first paint
  // follows the OS instead of flashing a default that gets replaced.
  let theme = $state<string | null>(null);
  let showShortcuts = $state(false);
  let showThemePicker = $state(false);
  // Settings float as a modal so opening them never navigates away from the
  // workspace (and its in-progress selection).
  let showSettings = $state(false);
  // Guard so a re-render never re-exchanges a spent launch token.
  let tokenAttempted = $state(false);
  // True only while a launch token is being exchanged. Routes stay unmounted
  // until it settles, so nothing can 401 its way to the login screen while the
  // session is still being created.
  //
  // This MUST be true on the first render when booting with a token. Effects
  // (including the exchange below) run after first paint, and child effects
  // run before parent ones, so initializing to false would mount Projects and
  // fire an authenticated request before the session exists — deterministically
  // landing on the login screen despite a successful exchange.
  let launchPending = $state(routeState.launchToken !== null);

  // Lazy-load the settings modal to keep the initial bundle small. Resolved
  // inside an effect, never during render (render-phase $state writes throw).
  let settingsModal = $state<Component | null>(null);

  $effect(() => {
    if (showSettings && !settingsModal) {
      import('./components/SettingsModal.svelte').then((m) => {
        settingsModal = m.default;
      });
    }
  });

  // Last server settings snapshot: every theme change persists against it,
  // so picking a theme in the picker (not just Settings Save) survives reload.
  let lastSettings = $state<SettingsData | null>(null);

  function setTheme(id: string) {
    if (id === 'system') {
      theme = null;
      applySystemTheme();
    } else {
      theme = id;
    }
    void persistTheme(id);
  }

  async function persistTheme(id: string): Promise<void> {
    try {
      const base = lastSettings ?? (await api.getSettings());
      lastSettings = { ...base, theme: id };
      await api.saveSettings(lastSettings);
    } catch {
      // Logged out or unreachable: the theme still applies for the session.
    }
  }

  // Concrete OS-matched theme for system mode. The attribute is never
  // removed: an absent data-theme mixed light surfaces with dark page
  // colors and must stay unreachable.
  function applySystemTheme() {
    const dark =
      typeof window.matchMedia === 'function' &&
      window.matchMedia('(prefers-color-scheme: dark)').matches;
    document.documentElement.setAttribute('data-theme', dark ? 'classic-dark' : 'classic-light');
    applyFavicon(document.documentElement.getAttribute('data-theme') ?? 'classic-dark');
  }

  // Apply the theme via data-theme on <html> and retint the favicon. In system
// mode applySystemTheme() resolves the concrete theme and does both.
  $effect(() => {
    if (theme) {
      document.documentElement.setAttribute('data-theme', theme);
      applyFavicon(theme);
    } else {
      applySystemTheme();
    }
  });

  // Follow OS changes while in system mode.
  $effect(() => {
    if (theme !== null) return;
    if (typeof window.matchMedia !== 'function') return;
    const mq = window.matchMedia('(prefers-color-scheme: dark)');
    const onChange = () => applySystemTheme();
    mq.addEventListener('change', onChange);
    return () => mq.removeEventListener('change', onChange);
  });

  // Meta + settings refresh. Runs on mount and again after login success:
  // the mount-time fetch is unauthenticated on the login path, so without
  // the second call the redirect would keep the OS-guess theme (and stale
  // meta) instead of the saved one.
  // App-mode auto-login. The launch token lives in the URL fragment, so it is
  // exchanged over POST and then stripped from the URL. A failure (expired or
  // already used) is not an error: the user falls through to the login screen.
  $effect(() => {
    const token = routeState.launchToken;
    if (!token || tokenAttempted) return;
    tokenAttempted = true;
    void (async () => {
      const ok = await exchangeLaunchToken(token);
      // Resync: replaceState fires no hashchange, so the launch token would
      // otherwise linger in route state forever.
      clearHashQuery();
      routeState = parseHash(window.location.hash);
      await refreshAuth();
      // A spent or expired token leaves the user on the normal login screen
      // rather than a route that would only 401.
      if (!ok && routeState.route !== 'login') navigate('/login');
      launchPending = false;
    })();
  });

  async function refreshAuth(): Promise<void> {
    try {
      const data = await api.getMeta();
      meta = data;
      applyMeta(data);
      if (!data.authenticated) return;
      // The heartbeat is what keeps an --app server alive, and the launch
      // token is one-time: after a refresh there is no token to exchange, so
      // an authenticated app window must (re)start the stream here, or the
      // server's grace timer exits 15s after the reload.
      if (data.app) startHeartbeat();
      try {
        const s = await api.getSettings();
        lastSettings = s;
        if (s.theme && s.theme !== 'system') theme = s.theme;
      } catch {
        // Unreachable server keeps the OS-guess paint.
      }
    } catch (err) {
      console.error('Failed to load server metadata', err);
    }
  }

  $effect(() => {
    // Settings ride along with an authenticated meta only: fetching them
    // anonymously 401s, and the bounce must never run on the login route
    // where it would wipe a token fragment before auto-submit.
    //
    // With a launch token, refreshAuth runs from the handshake instead, after
    // the session exists; racing it here would cache a stale "not signed in".
    if (routeState.launchToken) return;
    void refreshAuth();
  });

  function handleHashChange() {
    routeState = parseHash(window.location.hash);
    closeCtxMenu();
  }

  function handleGlobalKey(e: KeyboardEvent) {
    const target = e.target as HTMLElement;
    if (
      target.tagName === 'INPUT' ||
      target.tagName === 'TEXTAREA' ||
      target.tagName === 'SELECT' ||
      target.isContentEditable
    ) {
      return;
    }

    if (e.key === '?' && !e.metaKey && !e.ctrlKey) {
      e.preventDefault();
      showShortcuts = !showShortcuts;
      return;
    }

    if ((e.key === 't' || e.key === 'T') && !e.metaKey && !e.ctrlKey) {
      e.preventDefault();
      showThemePicker = !showThemePicker;
    }

    if (e.key === ',' && !e.metaKey && !e.ctrlKey) {
      e.preventDefault();
      showSettings = !showSettings;
    }
  }

  // Global custom right-click menu. A surface with its own items
  // preventDefaults first (see the explorer), which we honor here.
  function handleContextMenu(e: MouseEvent) {
    if (e.defaultPrevented) return;
    const items = buildContextMenuItems(e.target);
    e.preventDefault();
    let x = e.clientX;
    let y = e.clientY;
    // Keyboard-opened menus arrive at 0,0: anchor on the focused element.
    if (x === 0 && y === 0 && e.target instanceof HTMLElement) {
      const r = e.target.getBoundingClientRect();
      x = r.left + r.width / 2;
      y = r.bottom;
    }
    openCtxMenu(x, y, items, e.target instanceof HTMLElement ? e.target : null);
  }

</script>

<svelte:window onhashchange={handleHashChange} onkeydown={handleGlobalKey} oncontextmenu={handleContextMenu} />

<div class="app-container">
  {#if appState.stopped}
    <!-- The server is gone. Never leave a live-looking UI pointed at a dead
         listener; window.close() is usually refused for --app windows, so
         this screen is the normal end state. -->
    <div class="stopped">
      <div class="stopped-card">
        <span class="font-mono stopped-title">sift has stopped.</span>
        <span class="stopped-sub">You can close this window.</span>
      </div>
    </div>
  {:else}
  {#if routeState.route !== 'login' && routeState.route !== 'workspace'}
    <GlobalNav
      {meta}
      currentTheme={theme ?? 'system'}
      onOpenThemePicker={() => (showThemePicker = true)}
      onOpenShortcuts={() => (showShortcuts = true)}
      onOpenSettings={() => (showSettings = true)}
      onNavigate={navigate}
    />
  {/if}

  {#if appState.reconnecting}
    <div role="status" class="reconnect-banner">Disconnected, retrying…</div>
  {/if}

  <main class="app-main">
    {#if launchPending}
      <!-- The launch token is still being exchanged. Mounting a route here
           would fire authenticated calls that 401 before the session exists,
           and the 401 handler would bounce the window to the login screen
           mid-handshake. -->
      <div class="settings-fallback">Starting sift…</div>
    {:else if routeState.route === 'login'}
      <Login
        loginToken={routeState.loginToken}
        authKind={meta?.authKind ?? 'password'}
        onLoginSuccess={() => {
          void refreshAuth();
          navigate('/projects');
        }}
      />
    {:else if routeState.route === 'projects'}
      <Projects {meta} onOpenProject={(root) => navigate(`/p/${encodeURIComponent(root)}`)} />
    {:else if routeState.route === 'workspace' && routeState.projectRoot}
      <Workspace
        projectRoot={routeState.projectRoot}
        {meta}
        onNavigate={navigate}
        onOpenShortcuts={() => (showShortcuts = true)}
        onOpenThemePicker={() => (showThemePicker = true)}
        onOpenSettings={() => (showSettings = true)}
        onProjectTitle={(name) => (document.title = name ? `sift: ${name}` : 'sift')}
      />
    {/if}
  </main>

  {#if showSettings}
    {#if settingsModal}
      {@const SettingsModalComp = settingsModal}
      <SettingsModalComp
        {meta}
        currentTheme={theme ?? 'system'}
        onThemeChange={setTheme}
        onClose={() => (showSettings = false)}
      />
    {:else}
      <div role="dialog" aria-label="Settings" class="modal-loading">
        <span class="font-mono">Loading settings...</span>
      </div>
    {/if}
  {/if}

  {#if ctxMenuState.current}
    {@const menu = ctxMenuState.current}
    <ContextMenu x={menu.x} y={menu.y} items={menu.items} opener={menu.opener} onClose={closeCtxMenu} />
  {/if}

  <ToastHost />

  <ShortcutsModal isOpen={showShortcuts} onClose={() => (showShortcuts = false)} />

  <ThemePickerModal
    isOpen={showThemePicker}
    currentTheme={theme ?? 'system'}
    onSelectTheme={setTheme}
    onClose={() => (showThemePicker = false)}
  />
  {/if}
</div>

<style>
  .settings-fallback {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--ink-faint);
  }
  .modal-loading {
    position: fixed;
    inset: 0;
    background-color: rgba(0, 0, 0, 0.45);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 9999;
    font-size: 12px;
    color: var(--ink-faint);
  }
  .reconnect-banner {
    padding: 4px 16px;
    font-size: 11.5px;
    text-align: center;
    color: var(--status-warn);
    background-color: rgba(128, 128, 128, 0.12);
    border-bottom: 1px solid var(--border);
  }
  .stopped {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    background-color: var(--bg);
  }
  .stopped-card {
    display: flex;
    flex-direction: column;
    align-items: center;
    gap: 6px;
    text-align: center;
    padding: 24px;
  }
  .stopped-title {
    font-size: 14px;
    color: var(--ink);
  }
  .stopped-sub {
    font-size: 12px;
    color: var(--ink-faint);
  }
</style>
