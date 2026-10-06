<script lang="ts">
  import type { ApiMeta } from '../../lib/api';
  import Icon from '../../components/Icon.svelte';
  import Loader from '../../components/Loader.svelte';
  import {
    defaultStartDir,
    filterEntries,
    loadBrowse,
    parentName as parentNameOf,
    pathParts as pathPartsOf,
    projects,
  } from './store.svelte';

  interface Props {
    meta: ApiMeta | null;
    onOpenProject: (root: string) => void;
  }

  let { meta, onOpenProject }: Props = $props();

  let crumbs = $derived(pathPartsOf());
  let filteredEntries = $derived(filterEntries());
  let parentName = $derived(parentNameOf());
</script>

        <div class="browse">
          <div class="crumb-bar">
            <div class="crumbs">
              <button type="button" onclick={() => loadBrowse(meta?.roots?.[0] ?? defaultStartDir(meta))} class="crumb" title="Top of allowed roots">/</button>
              {#each crumbs as part, index (index)}
                {@const fullSubPath = '/' + crumbs.slice(0, index + 1).join('/')}
                {@const isLast = index === crumbs.length - 1}
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
              onclick={() => onOpenProject(projects.browsePath)}
              disabled={projects.browseLoading || !!projects.browseError}
              class="btn btn-sm btn-primary open-cta"
            >
              Open this folder
            </button>
          </div>

          <div class="browse-body">
            {#if projects.browseLoading}
              <Loader label="Scanning directory..." />
            {:else if projects.browseError}
              <div class="denied">
                <div class="denied-icon"><Icon name="warning" size={24} /></div>
                <div class="font-mono denied-title">Access Restricted (403)</div>
                <p class="denied-msg">{projects.browseError}</p>
                {#if meta?.roots && meta.roots[0]}
                  <button type="button" onclick={() => loadBrowse(defaultStartDir(meta))} class="btn btn-sm denied-cta">
                    Return to {defaultStartDir(meta)}
                  </button>
                {/if}
              </div>
            {:else}
              <div class="entries">
                {#if projects.browseData?.parent}
                  <div
                    onclick={() => projects.browseData && loadBrowse(projects.browseData.parent!)}
                    onkeydown={(e) => {
                      if ((e.key === 'Enter' || e.key === ' ') && projects.browseData?.parent) {
                        e.preventDefault();
                        loadBrowse(projects.browseData.parent);
                      }
                    }}
                    role="button"
                    tabindex="0"
                    class="entry parent-entry"
                    title={`Up to ${projects.browseData.parent}`}
                  >
                    <span class="up-arrow" aria-hidden="true">↑</span>
                    <span class="font-mono parent-label">{parentName}</span>
                  </div>
                {/if}

                {#if filteredEntries.length === 0 && projects.searchQuery}
                  <div class="no-filter-match">No folders match "{projects.searchQuery}"</div>
                {/if}

                {#each filteredEntries as entry, index (entry.name)}
                  {@const subPath = `${projects.browsePath.replace(/\/+$/, '')}/${entry.name}`}
                  {@const isLast = index === filteredEntries.length - 1}
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

<style>
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
  .parent-entry {
    gap: 8px;
  }
  .up-arrow {
    color: var(--ink-faint);
    font-size: 13px;
    line-height: 1;
  }
  .parent-label {
    font-size: 11.5px;
    color: var(--ink-soft);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .no-filter-match {
    padding: 16px;
    text-align: center;
    color: var(--ink-faint);
    font-size: 11.5px;
    font-family: var(--font-mono);
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
