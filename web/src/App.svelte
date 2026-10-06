<script lang="ts">
  import { api, type ApiMeta } from './lib/api';
  import { parseHash, navigate, type RouteState } from './lib/router.svelte';
  import { DEFAULT_THEME_ID } from './lib/themes';
  import GlobalNav from './components/GlobalNav.svelte';
  import ShortcutsModal from './components/ShortcutsModal.svelte';
  import ThemePickerModal from './components/ThemePickerModal.svelte';
  import Login from './routes/Login.svelte';
  import Projects from './routes/Projects.svelte';
  import Workspace from './routes/Workspace.svelte';
  import type { Component } from 'svelte';

  let routeState = $state<RouteState>(parseHash(window.location.hash || '#/projects'));
  let meta = $state<ApiMeta | null>(null);
  let theme = $state<string>(DEFAULT_THEME_ID);
  let showShortcuts = $state(false);
  let showThemePicker = $state(false);

  // Lazy-load Settings to keep the initial bundle small. Resolved inside an
  // effect, never during render (render-phase $state writes throw).
  let settingsComponent = $state<Component | null>(null);

  $effect(() => {
    if (routeState.route === 'settings' && !settingsComponent) {
      import('./routes/Settings.svelte').then((m) => {
        settingsComponent = m.default;
      });
    }
  });

  // Apply the theme via data-theme on <html>.
  $effect(() => {
    document.documentElement.setAttribute('data-theme', theme);
  });

  $effect(() => {
    api
      .getMeta()
      .then((data) => {
        meta = data;
        // Adopt the server-stored theme once settings are known.
        if (data.authenticated) {
          api.getSettings().then((s) => {
            if (s.theme && s.theme !== 'system') theme = s.theme;
          }).catch(() => {});
        }
      })
      .catch((err) => console.error('Failed to load server metadata', err));
  });

  function handleHashChange() {
    routeState = parseHash(window.location.hash);
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
  }

</script>

<svelte:window onhashchange={handleHashChange} onkeydown={handleGlobalKey} />

<div class="app-container">
  {#if routeState.route !== 'login' && routeState.route !== 'workspace'}
    <GlobalNav
      {meta}
      currentRoute={routeState.route}
      currentTheme={theme}
      onOpenThemePicker={() => (showThemePicker = true)}
      onOpenShortcuts={() => (showShortcuts = true)}
      onNavigate={navigate}
    />
  {/if}

  <main class="app-main">
    {#if routeState.route === 'login'}
      <Login
        loginToken={routeState.loginToken}
        authKind={meta?.authKind ?? 'password'}
        onLoginSuccess={() => navigate('/projects')}
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
      />
    {:else if routeState.route === 'settings'}
      {#if settingsComponent}
        {@const SettingsComp = settingsComponent}
        <SettingsComp {meta} currentTheme={theme} onThemeChange={(t) => (theme = t)} />
      {:else}
        <div class="settings-fallback">Loading settings...</div>
      {/if}
    {/if}
  </main>

  <ShortcutsModal isOpen={showShortcuts} onClose={() => (showShortcuts = false)} />

  <ThemePickerModal
    isOpen={showThemePicker}
    currentTheme={theme}
    onSelectTheme={(t) => (theme = t)}
    onClose={() => (showThemePicker = false)}
  />
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
</style>
