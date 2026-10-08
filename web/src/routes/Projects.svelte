<script lang="ts">
  import type { Component } from 'svelte';
  import { type ApiMeta, type TempClone } from '../lib/api';
  import Icon from '../components/Icon.svelte';
  import { clones, loadClones, looksLikeRepoUrl, repoNameFromUrl } from '../lib/clones.svelte';
  import { isPlainKey } from '../lib/keyboard';
  import { getUIPrefs } from '../lib/persist';
  import { toast } from '../lib/toast.svelte';
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
  import TempClones from './projects/TempClones.svelte';

  interface Props {
    meta: ApiMeta | null;
    onOpenProject: (root: string) => void;
  }

  let { meta, onOpenProject }: Props = $props();

  let searchInput: HTMLInputElement | null = $state(null);
  // Last opened project, so a reload can jump straight back into the
  // workspace (view state only — see lib/persist.ts).
  let lastProject = $state(getUIPrefs().lastProject);

  let filteredRecents = $derived(filterRecents());
  let filteredEntries = $derived(filterEntries());

  // Clone is offered only when the server says it can (git installed, and
  // loopback or --allow-clone); otherwise every entry point is hidden.
  let cloneEnabled = $derived(meta?.features?.clone === true);
  let queryIsRepoUrl = $derived(cloneEnabled && looksLikeRepoUrl(projects.searchQuery));

  // Lazy-load the clone dialog (same pattern as Settings): it is rarely
  // opened, so it stays out of the initial bundle.
  let showClone = $state(false);
  let cloneUrl = $state('');
  let cloneModal = $state<Component<any> | null>(null);

  $effect(() => {
    if (showClone && !cloneModal) {
      import('../components/CloneModal.svelte').then((m) => {
        cloneModal = m.default;
      });
    }
  });

  function openClone(url = '') {
    cloneUrl = url.trim();
    showClone = true;
  }

  function handleCloned(clone: TempClone) {
    showClone = false;
    projects.searchQuery = '';
    toast({ id: 'cloned', kind: 'success', message: `Cloned ${clone.name}`, duration: 2600 });
    onOpenProject(clone.root);
  }

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
    if (cloneEnabled && looksLikeRepoUrl(query)) {
      openClone(query);
      return;
    }
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
    lastProject = getUIPrefs().lastProject;
  });

  // The session's temporary clones come from the server, so they survive a
  // page reload (they only die with the server).
  $effect(() => {
    if (cloneEnabled) void loadClones();
  });

  // Default the browser to ~/Projects (or the first root) once meta arrives,
  // so it never opens on a nonexistent directory.
  $effect(() => {
    if (!projects.browsePath && meta?.defaultBrowse) projects.browsePath = meta.defaultBrowse;
    else if (!projects.browsePath && meta?.roots?.[0]) projects.browsePath = meta.roots[0];
  });
</script>

<svelte:window onkeydown={handleWindowKey} />

<div class="page anim-page">
  <div class="page-inner">
    <div class="title-block">
      <h1 class="font-mono page-title">Projects</h1>
      <div class="subtitle-row">
        <p class="subtitle">Open a Git repository or directory to scan, rank, and pack context.</p>

        {#if lastProject}
          <button type="button" onclick={() => onOpenProject(lastProject)} class="btn btn-sm btn-primary resume-btn">
            <Icon name="chevron-right" size={12} />
            <span>Resume {lastProject.split('/').filter(Boolean).pop()}</span>
          </button>
        {/if}
      </div>

      {#if meta?.roots && meta.roots.length > 0}
        <div class="roots-meta">
          <span>Allowed roots:</span>
          <span class="roots-list">{meta.roots.join(' · ')}</span>
        </div>
      {/if}
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
            placeholder={cloneEnabled
              ? 'Search, enter /path, or paste a git URL to clone… (Press /)'
              : 'Search projects or enter /path/to/folder... (Press /)'}
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

        {#if cloneEnabled}
          <button
            type="button"
            onclick={() => openClone()}
            class="btn btn-sm clone-btn"
            title="Clone a git repository"
          >
            <Icon name="clone" size={13} />
            <span>Clone</span>
          </button>
        {/if}
      </div>

      {#if queryIsRepoUrl}
        <button type="button" class="suggest" onclick={() => openClone(projects.searchQuery)}>
          <span class="suggest-icon"><Icon name="clone" size={14} /></span>
          <span class="suggest-text">
            Clone <span class="font-mono suggest-name">{repoNameFromUrl(projects.searchQuery)}</span>
            <span class="suggest-sub">as a temporary shallow checkout</span>
          </span>
          <kbd class="font-mono suggest-key">Enter</kbd>
        </button>
      {/if}

      {#if cloneEnabled && clones.items.length > 0}
        <TempClones {onOpenProject} />
      {/if}

      {#if projects.activeView === 'recents'}
        <RecentsList {onOpenProject} onSwitchToBrowse={() => switchToBrowse(meta)} />
      {/if}

      {#if projects.activeView === 'browse'}
        <FolderBrowser {meta} {onOpenProject} />
      {/if}
    </div>
  </div>
</div>

{#if showClone}
  {#if cloneModal}
    {@const CloneModalComp = cloneModal}
    <CloneModalComp initialUrl={cloneUrl} onClose={() => (showClone = false)} onCloned={handleCloned} />
  {:else}
    <div role="dialog" aria-label="Clone repository" class="modal-loading">
      <span class="font-mono">Loading…</span>
    </div>
  {/if}
{/if}

<style>
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
  .clone-btn {
    display: inline-flex;
    align-items: center;
    gap: 5px;
    flex-shrink: 0;
  }
  .suggest {
    display: flex;
    align-items: center;
    gap: 10px;
    width: 100%;
    padding: 9px 16px;
    border: none;
    border-bottom: 1px solid var(--border);
    background-color: var(--accent-soft);
    color: var(--ink);
    font-size: 12px;
    text-align: left;
    cursor: pointer;
    transition: background-color 60ms ease;
  }
  .suggest:hover,
  .suggest:focus-visible {
    background-color: color-mix(in srgb, var(--accent) 18%, var(--panel-bg));
  }
  .suggest-icon {
    display: flex;
    color: var(--accent);
    flex-shrink: 0;
  }
  .suggest-text {
    flex: 1;
    min-width: 0;
    display: flex;
    align-items: baseline;
    gap: 8px;
    overflow: hidden;
    white-space: nowrap;
  }
  .suggest-name {
    font-weight: 600;
  }
  .suggest-sub {
    color: var(--ink-faint);
    font-size: 11.5px;
    overflow: hidden;
    text-overflow: ellipsis;
  }
  .suggest-key {
    font-size: 10px;
    color: var(--ink-faint);
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-sm);
    padding: 1px 5px;
    flex-shrink: 0;
  }
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
  .resume-btn {
    flex-shrink: 0;
    display: inline-flex;
    align-items: center;
    gap: 4px;
    font-family: var(--font-mono);
    font-size: 11px;
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
