<script lang="ts">
  import { api, type RecentProject, type BrowseResult, type ApiMeta } from '../lib/api';
  import { truncateMiddle } from '../lib/format';
  import Icon from '../components/Icon.svelte';

  interface Props {
    meta: ApiMeta | null;
    onOpenProject: (root: string) => void;
  }

  let { meta, onOpenProject }: Props = $props();

  let recents = $state<RecentProject[]>([]);
  let loadingRecents = $state(true);
  let searchQuery = $state('');
  let activeView = $state<'recents' | 'browse'>('recents');
  let selectedIndex = $state(0);

  let browsePath = $state('/Users/developer/code');
  let browseData = $state<BrowseResult | null>(null);
  let browseLoading = $state(false);
  let browseError = $state<string | null>(null);

  let hoveredRoot = $state<string | null>(null);
  let searchInput: HTMLInputElement | null = $state(null);

  async function loadRecents() {
    try {
      loadingRecents = true;
      recents = await api.getRecents();
    } catch (err) {
      console.error('Failed to load recent projects', err);
    } finally {
      loadingRecents = false;
    }
  }

  async function loadBrowse(path: string) {
    try {
      browseLoading = true;
      browseError = null;
      const res = await api.browse(path);
      browseData = res;
      browsePath = res.path;
    } catch (err) {
      browseError = err instanceof Error ? err.message : "Couldn't read that folder: outside allowed roots";
    } finally {
      browseLoading = false;
    }
  }

  async function handleRemoveRecent(e: MouseEvent, root: string) {
    e.stopPropagation();
    try {
      await api.deleteRecent(root);
      recents = recents.filter((r) => r.root !== root);
    } catch (err) {
      console.error('Failed to delete recent project', err);
    }
  }

  let filteredRecents = $derived.by(() => {
    const q = searchQuery.trim().toLowerCase();
    if (!q) return recents;
    return recents.filter(
      (r) =>
        r.name.toLowerCase().includes(q) ||
        r.root.toLowerCase().includes(q) ||
        r.branch.toLowerCase().includes(q),
    );
  });

  // Keep selected index within bounds.
  $effect(() => {
    if (selectedIndex >= filteredRecents.length) {
      selectedIndex = Math.max(0, filteredRecents.length - 1);
    }
  });

  function handleSearchSubmit(e: SubmitEvent) {
    e.preventDefault();
    const query = searchQuery.trim();
    if (!query) return;
    if (query.startsWith('/') || query.startsWith('~')) {
      onOpenProject(query);
      return;
    }
    if (filteredRecents.length > 0) {
      const target = filteredRecents[selectedIndex] || filteredRecents[0];
      onOpenProject(target.root);
    }
  }

  function handleWindowKey(e: KeyboardEvent) {
    const activeEl = document.activeElement;
    const isInput =
      activeEl?.tagName === 'INPUT' || activeEl?.tagName === 'TEXTAREA' || activeEl?.tagName === 'SELECT';

    if (e.key === '/' && !isInput && !e.metaKey && !e.ctrlKey) {
      e.preventDefault();
      searchInput?.focus();
      return;
    }

    if (activeView !== 'recents' || filteredRecents.length === 0) return;

    if (e.key === 'ArrowDown') {
      e.preventDefault();
      selectedIndex = Math.min(filteredRecents.length - 1, selectedIndex + 1);
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      selectedIndex = Math.max(0, selectedIndex - 1);
    } else if (e.key === 'Enter' && !isInput) {
      e.preventDefault();
      const target = filteredRecents[selectedIndex];
      if (target) onOpenProject(target.root);
    }
  }

  let pathParts = $derived(browsePath.split('/').filter(Boolean));
  let dirEntries = $derived((browseData?.entries ?? []).filter((entry) => entry.isDir));

  $effect(() => {
    loadRecents();
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
            value={searchQuery}
            oninput={(e) => {
              searchQuery = e.currentTarget.value;
              selectedIndex = 0;
            }}
            placeholder="Search projects or enter /path/to/folder... (Press /)"
            class="input input-mono search-input"
            class:has-query={!!searchQuery}
          />
          {#if searchQuery}
            <button
              type="button"
              onclick={() => (searchQuery = '')}
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
            onclick={() => (activeView = 'recents')}
            class="segmented-btn seg-with-count"
            class:active={activeView === 'recents'}
          >
            <span>Recent Projects</span>
            <span class="font-mono tabular-nums seg-count">({recents.length})</span>
          </button>

          <button
            type="button"
            onclick={() => {
              activeView = 'browse';
              loadBrowse(browsePath);
            }}
            class="segmented-btn"
            class:active={activeView === 'browse'}
          >
            Folder Browser
          </button>
        </div>
      </div>

      {#if activeView === 'recents'}
        <div class="recents">
          {#if loadingRecents}
            <div class="loading-pad">Loading recent projects...</div>
          {:else if filteredRecents.length === 0}
            <div class="empty-state">
              <Icon name="sieve-empty" size={48} />
              <span class="font-mono empty-title">
                {searchQuery ? `No projects match "${searchQuery}"` : 'No recent projects yet'}
              </span>
              <span class="empty-sub">
                {searchQuery
                  ? 'Press Enter to open this path directly, or browse local directories.'
                  : 'Switch to the Folder Browser above to open your first local repository.'}
              </span>
              {#if !searchQuery}
                <button type="button" onclick={() => (activeView = 'browse')} class="btn btn-sm btn-primary empty-cta">
                  Browse local folder
                </button>
              {/if}
            </div>
          {:else}
            <div class="table">
              <div class="col-head">
                <div class="c-icon"></div>
                <div class="c-project">Project</div>
                <div class="c-branch">Branch</div>
                <div class="c-loc">Location</div>
                <div class="c-time">Last Opened</div>
                <div class="c-act"></div>
              </div>

              {#each filteredRecents as item, index (item.root)}
                {@const isHovered = hoveredRoot === item.root}
                {@const isKeyboardSelected = index === selectedIndex}
                {@const isLast = index === filteredRecents.length - 1}
                <div
                  onclick={() => onOpenProject(item.root)}
                  onmouseenter={() => {
                    hoveredRoot = item.root;
                    selectedIndex = index;
                  }}
                  onmouseleave={() => (hoveredRoot = null)}
                  role="button"
                  tabindex="0"
                  onkeydown={(e) => {
                    if (e.key === 'Enter') onOpenProject(item.root);
                  }}
                  class="row"
                  class:last={isLast}
                  class:active={isHovered || isKeyboardSelected}
                >
                  <div class="c-icon cell-icon"><Icon name="git" size={14} /></div>

                  <div class="c-project"><span class="font-mono proj-name">{item.name}</span></div>

                  <div class="c-branch"><span class="font-mono branch-name">{item.branch}</span></div>

                  <div class="c-loc">
                    <span class="font-mono loc-path" title={item.root}>{truncateMiddle(item.root, 40)}</span>
                  </div>

                  <div class="c-time"><span class="tabular-nums opened-at">{item.lastOpened}</span></div>

                  <div class="c-act">
                    <button
                      type="button"
                      onclick={(e) => handleRemoveRecent(e, item.root)}
                      class="del-btn"
                      class:visible={isHovered || isKeyboardSelected}
                      title="Remove from recent projects"
                      aria-label={`Remove ${item.name} from recent projects`}
                    >
                      <Icon name="trash" size={13} />
                    </button>
                  </div>
                </div>
              {/each}
            </div>
          {/if}
        </div>
      {/if}

      {#if activeView === 'browse'}
        <div class="browse">
          <div class="crumb-bar">
            <div class="crumbs">
              <button type="button" onclick={() => loadBrowse('/')} class="crumb">/</button>
              {#each pathParts as part, index (part)}
                {@const fullSubPath = '/' + pathParts.slice(0, index + 1).join('/')}
                {@const isLast = index === pathParts.length - 1}
                <span class="crumb-sep"><Icon name="chevron-right" size={10} /></span>
                <button
                  type="button"
                  onclick={() => loadBrowse(fullSubPath)}
                  class="crumb"
                  class:last={isLast}
                >
                  {part}
                </button>
              {/each}
            </div>

            <button
              type="button"
              onclick={() => onOpenProject(browsePath)}
              disabled={browseLoading || !!browseError}
              class="btn btn-sm btn-primary open-cta"
            >
              Open this folder
            </button>
          </div>

          <div class="browse-body">
            {#if browseLoading}
              <div class="loading-pad">Scanning directory...</div>
            {:else if browseError}
              <div class="denied">
                <div class="denied-icon"><Icon name="warning" size={24} /></div>
                <div class="font-mono denied-title">Access Restricted (403)</div>
                <p class="denied-msg">{browseError}</p>
                {#if meta?.roots && meta.roots[0]}
                  <button type="button" onclick={() => loadBrowse(meta.roots[0])} class="btn btn-sm denied-cta">
                    Return to {meta.roots[0]}
                  </button>
                {/if}
              </div>
            {:else}
              <div class="entries">
                {#if browseData?.parent}
                  <div
                    onclick={() => browseData && loadBrowse(browseData.parent!)}
                    onkeydown={(e) => {
                      if ((e.key === 'Enter' || e.key === ' ') && browseData?.parent) {
                        e.preventDefault();
                        loadBrowse(browseData.parent);
                      }
                    }}
                    role="button"
                    tabindex="0"
                    class="entry"
                  >
                    <span class="entry-icon dim"><Icon name="folder" size={14} /></span>
                    <span class="font-mono">.. (Parent directory)</span>
                  </div>
                {/if}

                {#each dirEntries as entry, index (entry.name)}
                  {@const subPath = `${browsePath.replace(/\/+$/, '')}/${entry.name}`}
                  {@const isLast = index === dirEntries.length - 1}
                  <div
                    onclick={() => loadBrowse(subPath)}
                    onkeydown={(e) => {
                      if (e.key === 'Enter' || e.key === ' ') {
                        e.preventDefault();
                        loadBrowse(subPath);
                      }
                    }}
                    role="button"
                    tabindex="0"
                    class="dir-entry"
                    class:last={isLast}
                  >
                    <div class="dir-id">
                      <span class="entry-icon" class:dim={!entry.isGitRepo}>
                        <Icon name={entry.isGitRepo ? 'git' : 'folder'} size={14} />
                      </span>

                      <span class="font-mono dir-name" class:repo={entry.isGitRepo}>{entry.name}/</span>

                      {#if entry.isGitRepo}
                        <span class="font-mono repo-tag">git repo</span>
                      {/if}
                    </div>

                    {#if entry.isGitRepo}
                      <button
                        type="button"
                        onclick={(e) => {
                          e.stopPropagation();
                          onOpenProject(subPath);
                        }}
                        class="btn btn-sm btn-ghost open-mini"
                      >
                        Open →
                      </button>
                    {/if}
                  </div>
                {/each}
              </div>
            {/if}
          </div>
        </div>
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
  }
  .roots-list {
    color: var(--ink-soft);
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
  .recents {
    display: flex;
    flex-direction: column;
  }
  .loading-pad {
    padding: 42px 16px;
    text-align: center;
    color: var(--ink-faint);
    font-size: 12px;
    font-family: var(--font-mono);
  }
  .empty-state {
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    padding: 48px 16px;
    gap: 12px;
    text-align: center;
  }
  .empty-title {
    font-size: 13px;
    color: var(--ink-soft);
  }
  .empty-sub {
    font-size: 11.5px;
    color: var(--ink-faint);
    max-width: 320px;
  }
  .empty-cta {
    margin-top: 4px;
  }
  .table {
    display: flex;
    flex-direction: column;
  }
  .col-head {
    display: flex;
    align-items: center;
    padding: 6px 16px;
    border-bottom: 1px solid var(--border);
    background-color: var(--surface-alt);
    font-size: 10.5px;
    font-family: var(--font-mono);
    color: var(--ink-faint);
    text-transform: uppercase;
    letter-spacing: 0.04em;
    user-select: none;
  }
  .c-icon {
    width: 22px;
    flex-shrink: 0;
  }
  .c-project {
    width: 160px;
    flex-shrink: 0;
  }
  .c-branch {
    width: 110px;
    flex-shrink: 0;
  }
  .c-loc {
    flex: 1;
    min-width: 0;
    padding-right: 12px;
  }
  .c-time {
    width: 90px;
    text-align: right;
    flex-shrink: 0;
  }
  .c-act {
    width: 26px;
    flex-shrink: 0;
  }
  .row {
    display: flex;
    align-items: center;
    padding: 9px 16px;
    border-bottom: 1px solid var(--border);
    cursor: pointer;
    transition: background-color 60ms ease;
    font-size: 12px;
  }
  .row.last {
    border-bottom: none;
  }
  .row.active {
    background-color: var(--surface-alt);
  }
  .cell-icon {
    display: flex;
    align-items: center;
    color: var(--ink-soft);
  }
  .proj-name {
    font-size: 12.5px;
    font-weight: 600;
    color: var(--ink);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    display: block;
    padding-right: 10px;
  }
  .branch-name {
    font-size: 11px;
    color: var(--ink-soft);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    display: block;
    padding-right: 10px;
  }
  .loc-path {
    font-size: 11px;
    color: var(--ink-faint);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    display: block;
    padding-right: 12px;
  }
  .opened-at {
    font-size: 11px;
    color: var(--ink-faint);
    white-space: nowrap;
  }
  .del-btn {
    background: none;
    border: none;
    color: var(--ink-faint);
    cursor: pointer;
    padding: 2px 3px;
    border-radius: 3px;
    display: flex;
    align-items: center;
    opacity: 0;
    transition:
      opacity 70ms ease,
      color 70ms ease;
  }
  .del-btn.visible {
    opacity: 1;
  }
  .del-btn:hover {
    color: var(--status-danger);
  }
  .browse {
    display: flex;
    flex-direction: column;
  }
  .crumb-bar {
    padding: 8px 14px;
    border-bottom: 1px solid var(--border);
    background-color: var(--surface-alt);
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 12px;
    font-size: 11.5px;
    font-family: var(--font-mono);
    user-select: none;
  }
  .crumbs {
    display: flex;
    align-items: center;
    gap: 4px;
    overflow-x: auto;
    white-space: nowrap;
    min-width: 0;
  }
  .crumb {
    background: none;
    border: none;
    cursor: pointer;
    color: var(--ink-soft);
    padding: 2px 4px;
    font-size: 12px;
    font-family: var(--font-mono);
  }
  .crumb.last {
    color: var(--ink);
    font-weight: 600;
  }
  .crumb-sep {
    color: var(--border-strong);
    font-size: 10px;
    display: flex;
  }
  .open-cta {
    flex-shrink: 0;
  }
  .browse-body {
    min-height: 260px;
    display: flex;
    flex-direction: column;
  }
  .denied {
    padding: 40px 16px;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 10px;
    text-align: center;
  }
  .denied-icon {
    color: var(--status-danger);
  }
  .denied-title {
    font-size: 13px;
    font-weight: 600;
    color: var(--ink);
  }
  .denied-msg {
    font-size: 12px;
    color: var(--ink-soft);
    max-width: 340px;
  }
  .denied-cta {
    margin-top: 4px;
  }
  .entries {
    display: flex;
    flex-direction: column;
  }
  .entry {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 9px 16px;
    border-bottom: 1px solid var(--border);
    cursor: pointer;
    font-size: 12px;
    color: var(--ink-soft);
  }
  .entry:hover {
    background-color: var(--surface-alt);
  }
  .entry-icon {
    display: flex;
  }
  .entry-icon.dim {
    color: var(--ink-faint);
  }
  .dir-entry {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 9px 16px;
    border-bottom: 1px solid var(--border);
    cursor: pointer;
    font-size: 12px;
    transition: background-color 60ms ease;
  }
  .dir-entry.last {
    border-bottom: none;
  }
  .dir-entry:hover {
    background-color: var(--surface-alt);
  }
  .dir-id {
    display: flex;
    align-items: center;
    gap: 10px;
  }
  .dir-name {
    color: var(--ink);
    font-weight: 400;
  }
  .dir-name.repo {
    font-weight: 600;
  }
  .repo-tag {
    font-size: 10px;
    color: var(--ink-faint);
    margin-left: 4px;
  }
  .open-mini {
    font-size: 11px;
    padding: 2px 6px;
    color: var(--ink-soft);
  }
</style>
