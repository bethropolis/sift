<script lang="ts">
  import type { ApiMeta } from '../lib/api';
  import { getThemeById } from '../lib/themes';
  import { quitApp } from '../lib/appmode.svelte';
  import Icon from './Icon.svelte';

  interface Props {
    meta: ApiMeta | null;
    currentRoute: string;
    currentTheme: string;
    onOpenThemePicker: () => void;
    onOpenShortcuts: () => void;
    onNavigate: (path: string) => void;
  }

  let { meta, currentRoute, currentTheme, onOpenThemePicker, onOpenShortcuts, onNavigate }: Props = $props();

  let isRemote = $derived(meta?.mode === 'remote');
  let isTlsUnencrypted = $derived(isRemote && meta?.tls === false);
  let themeInfo: { name: string } = $derived(
    currentTheme === 'system' ? { name: 'System' } : getThemeById(currentTheme),
  );
  // Quit stops the server, then the window goes with it.
  async function onQuit() {
    await quitApp();
  }
  // Only meaningful in a chromeless app window, where quitting should also
  // stop the server rather than leave it running behind a closed window.
  let isApp = $derived(meta?.app === true);
</script>

{#if isTlsUnencrypted}
  <div role="alert" class="tls-banner">
    <Icon name="warning" size={14} />
    <span>Connection is not encrypted. Data transferred over HTTP may be visible on your network.</span>
  </div>
{/if}

<header class="topbar">
  <div class="brand-group">
    <button onclick={() => onNavigate('/projects')} class="brand-btn" title="sift serve home">
      <Icon name="logo" size={20} />
      <span class="font-mono brand-word">sift</span>
    </button>

    <nav class="segmented-control">
      <button
        type="button"
        onclick={() => onNavigate('/projects')}
        class="segmented-btn"
        class:active={currentRoute === 'projects'}
      >
        Projects
      </button>
      <button
        type="button"
        onclick={() => onNavigate('/settings')}
        class="segmented-btn"
        class:active={currentRoute === 'settings'}
      >
        Settings
      </button>
    </nav>
  </div>

  <div class="topbar-right">
    <span class="status-line" title={isRemote ? 'Connected to a remote sift server' : 'Local sift server'}>
      <span class:remote={isRemote}>{isRemote ? 'Remote' : 'Local'}</span>
      {#if meta?.version}
        <span class="status-version">· {meta.version}</span>
      {/if}
    </span>

    <button
      onclick={onOpenThemePicker}
      class="btn btn-sm btn-ghost theme-btn"
      title="Switch color theme (t)"
      aria-label="Switch color theme"
    >
      <span class="theme-icon"><Icon name="palette" size={13} /></span>
      <span class="theme-name">{themeInfo.name}</span>
      <kbd class="font-mono hide-on-compact theme-kbd">t</kbd>
    </button>

    {#if isApp}
      <button onclick={onQuit} class="btn btn-sm btn-ghost quit-btn" title="Stop sift and close this window">
        <Icon name="power" size={12} />
        <span>Quit</span>
      </button>
    {/if}

    <button
      onclick={onOpenShortcuts}
      class="btn btn-sm btn-ghost shortcuts-btn"
      title="Keyboard shortcuts (?)"
      aria-label="View keyboard shortcuts"
    >
      ?
    </button>
  </div>
</header>

<style>
  .tls-banner {
    border-bottom: 1px solid var(--border);
    padding: 4px 16px;
    font-size: 12px;
    font-weight: 500;
    display: flex;
    align-items: center;
    justify-content: center;
    gap: 8px;
  }
  .topbar {
    height: 34px;
    min-height: 34px;
    border-bottom: 1px solid var(--border);
    background-color: var(--surface-raised);
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0 12px;
    user-select: none;
  }
  .brand-group {
    display: flex;
    align-items: center;
    gap: 16px;
  }
  .brand-btn {
    padding: 2px 6px;
    display: flex;
    align-items: center;
    gap: 8px;
    text-decoration: none;
    background: transparent;
    border: none;
    cursor: pointer;
  }
  .brand-word {
    font-size: 15px;
    font-weight: 600;
    letter-spacing: -0.02em;
    color: var(--ink);
  }
  .topbar-right {
    display: flex;
    align-items: center;
    gap: 10px;
  }
  .status-line {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    font-size: 11px;
    font-family: var(--font-mono);
    color: var(--ink-soft);
    padding: 2px 4px;
    white-space: nowrap;
  }
  .status-line .remote {
    color: var(--accent);
    font-weight: 600;
  }
  .status-version {
    color: var(--ink-faint);
  }
  .theme-btn {
    padding: 2px 7px;
    font-size: 11px;
    font-family: var(--font-mono);
    color: var(--ink-soft);
    display: inline-flex;
    align-items: center;
    gap: 6px;
    border: 1px solid var(--border);
  }
  .theme-icon {
    color: var(--accent);
    display: flex;
  }
  .theme-name {
    max-width: 120px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .theme-kbd {
    font-size: 9px;
    opacity: 0.6;
  }
  .quit-btn {
    display: inline-flex;
    align-items: center;
    gap: 4px;
    color: var(--status-danger);
    border-color: var(--border);
  }
  .shortcuts-btn {
    padding: 3px 8px;
    font-family: var(--font-mono);
    font-size: 12px;
    color: var(--ink-soft);
  }
  @media (max-width: 640px) {
    .topbar {
      padding: 0 8px;
    }
    .brand-group {
      gap: 8px;
      min-width: 0;
    }
    .status-version {
      display: none;
    }
  }
  @media (max-width: 560px) {
    .brand-word {
      display: none;
    }
    .theme-name {
      display: none;
    }
  }
</style>
