<script lang="ts">
  import { tick } from 'svelte';
  import type { RecentProject } from '../lib/api';
  import { formatRelativeTime, truncateMiddle } from '../lib/format';
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
  let listEl: HTMLDivElement | null = $state(null);

  let projectName = $derived(currentRoot.split('/').filter(Boolean).pop() || currentRoot);

  function handleWindowMouse(e: MouseEvent) {
    if (isOpen && container && !container.contains(e.target as Node)) isOpen = false;
  }

  function handleWindowKey(e: KeyboardEvent) {
    if (e.key === 'Escape' && isOpen) {
      isOpen = false;
      container?.querySelector<HTMLButtonElement>('.switcher-btn')?.focus();
    }
  }

  function toggle() {
    isOpen = !isOpen;
    if (isOpen) {
      // Keyboard users land inside the list: the current project first, so
      // Enter re-confirms and Tab walks the rest.
      void tick().then(() => {
        const el =
          listEl?.querySelector<HTMLButtonElement>('.recent-row.selected') ??
          listEl?.querySelector<HTMLButtonElement>('.recent-row');
        el?.focus();
      });
    }
  }
</script>

<svelte:window onmousedown={handleWindowMouse} onkeydown={handleWindowKey} />

<div bind:this={container} class="switcher">
  <button
    onclick={toggle}
    class="switcher-btn"
    class:open={isOpen}
    title={`Switch project (${currentRoot})`}
    aria-haspopup="listbox"
    aria-expanded={isOpen}
  >
    <span class="switcher-name">{projectName}</span>
    <span class="switcher-path">({truncateMiddle(currentRoot, 24)})</span>
    <span class="chev" class:open={isOpen}><Icon name="chevron-down" size={10} /></span>
  </button>

  {#if isOpen}
    <div role="listbox" aria-label="Recent projects" class="dropdown">
      <div class="dropdown-head">Recent Projects <span class="head-count">{recents.length}</span></div>

      <div bind:this={listEl} class="dropdown-list">
        {#if recents.length === 0}
          <div class="empty">No recent projects yet</div>
        {/if}
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
            <span class="row-icon"><Icon name="git" size={12} /></span>
            <span class="recent-text">
              <span class="recent-top">
                <span class="font-mono recent-name">{item.name}</span>
                {#if item.branch}
                  <span class="font-mono recent-branch">{item.branch}</span>
                {/if}
                {#if item.lastOpened}
                  <span class="font-mono recent-time">{formatRelativeTime(item.lastOpened)}</span>
                {/if}
              </span>
              <span class="font-mono recent-root">{item.root}</span>
            </span>
            {#if isSelected}
              <span class="row-check"><Icon name="check" size={12} /></span>
            {/if}
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
    min-width: 0;
  }
  .switcher-btn {
    display: flex;
    align-items: center;
    gap: 5px;
    background: transparent;
    border: 1px solid transparent;
    color: var(--ink-soft);
    cursor: pointer;
    padding: 3px 6px;
    font-size: 11px;
    font-family: var(--font-mono);
    border-radius: var(--radius-sm);
    max-width: 100%;
    transition:
      background-color 70ms ease,
      border-color 70ms ease;
  }
  .switcher-btn:hover,
  .switcher-btn.open {
    background-color: var(--surface-alt);
    border-color: var(--border);
  }
  .switcher-name {
    color: var(--ink);
    font-weight: 600;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .switcher-path {
    color: var(--ink-faint);
    font-size: 10px;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  /* Narrow screens keep the project name; the full path lives in the
     button title and the dropdown. */
  @media (max-width: 720px) {
    .switcher-path {
      display: none;
    }
  }
  .chev {
    display: inline-flex;
    color: var(--ink-faint);
    transition: transform 120ms ease;
  }
  .chev.open {
    transform: rotate(180deg);
  }
  .dropdown {
    position: absolute;
    top: calc(100% + 4px);
    left: 0;
    width: 320px;
    max-width: min(320px, 80vw);
    background-color: var(--panel-bg);
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-md);
    box-shadow: 0 12px 32px rgba(0, 0, 0, 0.22);
    z-index: 50;
    overflow: hidden;
    animation: drop-in 110ms ease-out;
  }
  @keyframes drop-in {
    from {
      opacity: 0;
      transform: translateY(-4px);
    }
  }
  .dropdown-head {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 7px 12px;
    font-size: 10px;
    font-family: var(--font-mono);
    text-transform: uppercase;
    color: var(--ink-faint);
    letter-spacing: 0.06em;
    border-bottom: 1px solid var(--border);
  }
  .head-count {
    color: var(--ink-soft);
    background-color: var(--surface-alt);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 0 6px;
    font-size: 10px;
  }
  .dropdown-list {
    max-height: 220px;
    overflow-y: auto;
    padding: 4px;
    scrollbar-width: thin;
    scrollbar-color: var(--border-strong) transparent;
  }
  .empty {
    padding: 12px;
    font-size: 11px;
    font-family: var(--font-mono);
    color: var(--ink-faint);
    text-align: center;
  }
  .recent-row {
    width: 100%;
    display: flex;
    align-items: center;
    gap: 8px;
    padding: 7px 8px;
    border: none;
    border-radius: var(--radius-sm);
    background: transparent;
    cursor: pointer;
    text-align: left;
    transition: background-color 70ms ease;
  }
  .recent-row:hover {
    background-color: rgba(128, 128, 128, 0.12);
  }
  .recent-row.selected {
    background-color: var(--accent-soft);
    box-shadow: inset 2px 0 0 var(--accent);
  }
  .row-icon {
    color: var(--ink-faint);
    display: inline-flex;
    flex-shrink: 0;
  }
  .selected .row-icon {
    color: var(--accent);
  }
  .recent-text {
    display: flex;
    flex-direction: column;
    gap: 1px;
    min-width: 0;
    flex: 1;
  }
  .recent-top {
    display: flex;
    align-items: baseline;
    gap: 6px;
    min-width: 0;
  }
  .recent-name {
    font-size: 11.5px;
    font-weight: 600;
    color: var(--ink);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .recent-branch {
    font-size: 9.5px;
    color: var(--accent-ink);
    background-color: var(--surface-alt);
    border: 1px solid var(--border);
    border-radius: 8px;
    padding: 0 6px;
    white-space: nowrap;
    flex-shrink: 0;
  }
  .recent-time {
    margin-left: auto;
    font-size: 10px;
    color: var(--ink-faint);
    white-space: nowrap;
    flex-shrink: 0;
  }
  .recent-root {
    font-size: 10px;
    color: var(--ink-faint);
    width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .row-check {
    color: var(--accent);
    display: inline-flex;
    flex-shrink: 0;
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
    padding: 7px 8px;
    font-size: 11px;
    font-family: var(--font-mono);
    border: none;
    border-radius: var(--radius-sm);
    background: transparent;
    color: var(--accent);
    cursor: pointer;
    transition: background-color 70ms ease;
  }
  .browse-btn:hover {
    background-color: rgba(128, 128, 128, 0.12);
  }
</style>
