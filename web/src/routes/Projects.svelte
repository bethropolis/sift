<script lang="ts">
  import { type ApiMeta } from '../lib/api';
  import Icon from '../components/Icon.svelte';
  import { isPlainKey } from '../lib/keyboard';
  import {
    filterEntries,
    filterRecents,
    loadBrowse,
    loadBrowsePrefs,
    loadRecents,
    projects,
    switchToBrowse,
  } from './projects/store.svelte';
  import RecentsList from './projects/RecentsList.svelte';
  import FolderBrowser from './projects/FolderBrowser.svelte';

  interface Props {
    meta: ApiMeta | null;
    onOpenProject: (root: string) => void;
  }

  let { meta, onOpenProject }: Props = $props();

  let searchInput: HTMLInputElement | null = $state(null);

  let filteredRecents = $derived(filterRecents());
  let filteredEntries = $derived(filterEntries());

  // Keep selected index within bounds.
  $effect(() => {
    if (projects.selectedIndex >= filteredRecents.length) {
      projects.selectedIndex = Math.max(0, filteredRecents.length - 1);
    }
  });

  function handleSearchSubmit(e: SubmitEvent) {
    e.preventDefault();
    const query = projects.searchQuery.trim();
    if (!query) return;
    if (query.startsWith('/') || query.startsWith('~')) {
      onOpenProject(query);
      return;
    }
    if (projects.activeView === 'browse') {
      const target =
        filteredEntries[
          Math.min(projects.selectedIndex, Math.max(0, filteredEntries.length - 1))
        ] || filteredEntries[0];
      if (target) {
        void loadBrowse(`${projects.browsePath.replace(/\/+$/, '')}/${target.name}`);
      }
      return;
    }
    if (filteredRecents.length > 0) {
      const target = filteredRecents[projects.selectedIndex] || filteredRecents[0];
      onOpenProject(target.root);
    }
  }

  function handleWindowKey(e: KeyboardEvent) {
    const activeEl = document.activeElement;
    const isInput =
      activeEl?.tagName === 'INPUT' || activeEl?.tagName === 'TEXTAREA' || activeEl?.tagName === 'SELECT';

    if (isPlainKey(e, '/') && !isInput) {
      e.preventDefault();
      searchInput?.focus();
      return;
    }

    if (projects.activeView !== 'recents' || filteredRecents.length === 0) return;

    if (e.key === 'ArrowDown') {
      e.preventDefault();
      projects.selectedIndex = Math.min(filteredRecents.length - 1, projects.selectedIndex + 1);
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      projects.selectedIndex = Math.max(0, projects.selectedIndex - 1);
    } else if (e.key === 'Enter' && !isInput) {
      e.preventDefault();
      const target = filteredRecents[projects.selectedIndex];
      if (target) onOpenProject(target.root);
    }
  }

  $effect(() => {
    loadRecents();
    loadBrowsePrefs();
  });

  // Default the browser to ~/Projects (or the first root) once meta arrives,
  // so it never opens on a nonexistent directory.
  $effect(() => {
    if (!projects.browsePath && meta?.defaultBrowse) projects.browsePath = meta.defaultBrowse;
    else if (!projects.browsePath && meta?.roots?.[0]) projects.browsePath = meta.roots[0];
  });
</script>

<svelte:window onkeydown={handleWindowKey} />

<div class="page">
  <div class="page-inner">
    <div class="title-block">
      <h1 class="font-mono page-title">Projects</h1>
      <div class="subtitle-row">
        <p class="subtitle">Open a Git repository or directory to scan, rank, and pack context.</p>

        {#if meta?.roots && meta.roots.length > 0}
          <div class="roots-meta">
            <span>Allowed roots:</span>
            <span class="roots-list">{meta.roots.join(' · ')}</span>
          </div>
        {/if}
      </div>
    </div>

    <div class="panel">
      <div class="action-bar">
        <form onsubmit={handleSearchSubmit} class="search-form">
          <span class="search-icon"><Icon name="search" size={13} /></span>
          <input
            bind:this={searchInput}
            type="text"
            value={projects.searchQuery}
            oninput={(e) => {
              projects.searchQuery = e.currentTarget.value;
              projects.selectedIndex = 0;
            }}
            placeholder="Search projects or enter /path/to/folder... (Press /)"
            class="input input-mono search-input"
            class:has-query={!!projects.searchQuery}
          />
          {#if projects.searchQuery}
            <button
              type="button"
              onclick={() => (projects.searchQuery = '')}
              class="search-clear"
              title="Clear search"
              aria-label="Clear search"
            >
              ✕
            </button>
          {/if}
        </form>

        <div class="segmented-control">
          <button
            type="button"
            onclick={() => (projects.activeView = 'recents')}
            class="segmented-btn seg-with-count"
            class:active={projects.activeView === 'recents'}
          >
            <span>Recent Projects</span>
            <span class="font-mono tabular-nums seg-count">({projects.recents.length})</span>
          </button>

          <button
            type="button"
            onclick={() => switchToBrowse(meta)}
            class="segmented-btn"
            class:active={projects.activeView === 'browse'}
          >
            Folder Browser
          </button>
        </div>
      </div>


      {#if projects.activeView === 'recents'}
        <RecentsList {onOpenProject} onSwitchToBrowse={() => switchToBrowse(meta)} />
      {/if}

      {#if projects.activeView === 'browse'}
        <FolderBrowser {meta} {onOpenProject} />
      {/if}
    </div>
  </div>
</div>

<style>
  .page {
    flex: 1;
    overflow-y: auto;
    padding: 32px 20px;
    background-color: var(--bg);
    display: flex;
    flex-direction: column;
    align-items: center;
    width: 100%;
  }
  .page-inner {
    width: 100%;
    max-width: 860px;
    display: flex;
    flex-direction: column;
    gap: 18px;
  }
  .title-block {
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .page-title {
    font-size: 18px;
    font-weight: 600;
    color: var(--ink);
    letter-spacing: -0.02em;
  }
  .subtitle-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    flex-wrap: wrap;
    gap: 12px;
  }
  .subtitle {
    font-size: 12.5px;
    color: var(--ink-soft);
  }
  .roots-meta {
    font-size: 11px;
    font-family: var(--font-mono);
    color: var(--ink-faint);
    display: flex;
    align-items: center;
    gap: 6px;
    min-width: 0;
  }
  .roots-list {
    color: var(--ink-soft);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    min-width: 0;
  }
  .panel {
    background-color: var(--panel-bg);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    overflow: hidden;
    display: flex;
    flex-direction: column;
    box-shadow: 0 1px 3px rgba(0, 0, 0, 0.02);
  }
  .action-bar {
    padding: 10px 14px;
    border-bottom: 1px solid var(--border);
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    background-color: var(--surface-raised);
  }
  @media (max-width: 640px) {
    .page {
      padding: 16px 12px;
    }
    .page-inner {
      gap: 14px;
    }
    .action-bar {
      flex-direction: column;
      align-items: stretch;
      gap: 8px;
    }
    .search-form {
      max-width: none;
    }
    .action-bar .segmented-control {
      align-self: flex-start;
    }
  }
  .search-form {
    position: relative;
    flex: 1;
    max-width: 460px;
    display: flex;
    align-items: center;
  }
  .search-icon {
    position: absolute;
    left: 9px;
    color: var(--ink-faint);
    pointer-events: none;
    display: flex;
  }
  .search-input {
    width: 100%;
    padding-left: 28px;
    padding-right: 10px;
    height: 28px;
    font-size: 11.5px;
  }
  .search-input.has-query {
    padding-right: 24px;
  }
  .search-clear {
    position: absolute;
    right: 6px;
    background: none;
    border: none;
    color: var(--ink-faint);
    cursor: pointer;
    font-size: 11px;
    padding: 2px 4px;
  }
  .seg-with-count {
    display: inline-flex;
    align-items: center;
    gap: 5px;
  }
  .seg-count {
    font-size: 10px;
    opacity: 0.7;
  }
</style>
