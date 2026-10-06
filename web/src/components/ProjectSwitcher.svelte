<script lang="ts">
  import type { RecentProject } from '../lib/api';
  import { truncateMiddle } from '../lib/format';
  import Icon from './Icon.svelte';

  interface Props {
    currentRoot: string;
    recents: RecentProject[];
    onSelectProject: (root: string) => void;
    onBrowse: () => void;
  }

  let { currentRoot, recents, onSelectProject, onBrowse }: Props = $props();
  let isOpen = $state(false);
  let container: HTMLDivElement | null = $state(null);

  let projectName = $derived(currentRoot.split('/').filter(Boolean).pop() || currentRoot);

  function handleWindowMouse(e: MouseEvent) {
    if (isOpen && container && !container.contains(e.target as Node)) isOpen = false;
  }
</script>

<svelte:window onmousedown={handleWindowMouse} />

<div bind:this={container} class="switcher">
  <button
    onclick={() => (isOpen = !isOpen)}
    class="switcher-btn"
    title={`Switch project (${currentRoot})`}
    aria-haspopup="listbox"
    aria-expanded={isOpen}
  >
    <span class="switcher-name">{projectName}</span>
    <span class="switcher-path">({truncateMiddle(currentRoot, 24)})</span>
    <Icon name="chevron-down" size={10} />
  </button>

  {#if isOpen}
    <div role="listbox" class="dropdown">
      <div class="dropdown-head">Recent Projects</div>

      <div class="dropdown-list">
        {#each recents as item (item.root)}
          {@const isSelected = item.root === currentRoot}
          <button
            role="option"
            aria-selected={isSelected}
            onclick={() => {
              isOpen = false;
              onSelectProject(item.root);
            }}
            class="recent-row"
            class:selected={isSelected}
          >
            <div class="recent-main">
              <Icon name="git" size={12} />
              <span class="font-mono recent-name" class:selected={isSelected}>{item.name}</span>
              <span class="font-mono recent-branch">{item.branch}</span>
            </div>
            <div class="font-mono recent-root">{item.root}</div>
          </button>
        {/each}
      </div>

      <div class="dropdown-foot">
        <button
          onclick={() => {
            isOpen = false;
            onBrowse();
          }}
          class="browse-btn"
        >
          <Icon name="folder" size={12} />
          <span>Browse folder...</span>
        </button>
      </div>
    </div>
  {/if}
</div>

<style>
  .switcher {
    position: relative;
    display: inline-flex;
    align-items: center;
  }
  .switcher-btn {
    display: flex;
    align-items: center;
    gap: 5px;
    background: transparent;
    border: none;
    color: var(--ink-soft);
    cursor: pointer;
    padding: 2px 4px;
    font-size: 11px;
    font-family: var(--font-mono);
    border-radius: 3px;
  }
  .switcher-name {
    color: var(--ink);
  }
  .switcher-path {
    color: var(--ink-faint);
    font-size: 10px;
  }
  /* Narrow screens keep the project name; the full path lives in the
     button title and the dropdown. */
  @media (max-width: 720px) {
    .switcher-path {
      display: none;
    }
  }
  .dropdown {
    position: absolute;
    top: calc(100% + 4px);
    left: 0;
    width: 300px;
    background-color: var(--panel-bg);
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-md);
    box-shadow: 0 6px 20px rgba(0, 0, 0, 0.3);
    z-index: 50;
    overflow: hidden;
  }
  .dropdown-head {
    padding: 6px 10px;
    font-size: 10px;
    font-family: var(--font-mono);
    text-transform: uppercase;
    color: var(--ink-faint);
    letter-spacing: 0.04em;
    border-bottom: 1px solid var(--border);
  }
  .dropdown-list {
    max-height: 180px;
    overflow-y: auto;
  }
  .recent-row {
    width: 100%;
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    padding: 6px 10px;
    border: none;
    background: transparent;
    cursor: pointer;
    text-align: left;
  }
  .recent-row:hover {
    background-color: rgba(255, 255, 255, 0.03);
  }
  .recent-row.selected {
    background: rgba(103, 203, 231, 0.08);
  }
  .recent-main {
    display: flex;
    align-items: center;
    gap: 6px;
    width: 100%;
  }
  .recent-name {
    font-size: 11px;
    font-weight: 600;
    color: var(--ink);
    flex: 1;
  }
  .recent-name.selected {
    color: var(--accent);
  }
  .recent-branch {
    font-size: 9px;
    color: var(--accent-ink);
  }
  .recent-root {
    font-size: 10px;
    color: var(--ink-faint);
    width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .dropdown-foot {
    border-top: 1px solid var(--border);
    padding: 4px;
  }
  .browse-btn {
    width: 100%;
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 6px 8px;
    font-size: 11px;
    font-family: var(--font-mono);
    border: none;
    border-radius: var(--radius-sm);
    background: transparent;
    color: var(--accent);
    cursor: pointer;
  }
  .browse-btn:hover {
    background-color: rgba(255, 255, 255, 0.03);
  }
</style>
