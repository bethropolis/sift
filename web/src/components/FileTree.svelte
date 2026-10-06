<script module lang="ts">
  /** Fold actions exposed to the workspace keybinds via bind:treeActions. */
  export interface TreeFoldActions {
    expandAll: () => void;
    collapseAll: () => void;
  }
</script>

<script lang="ts">
  import type { TreeFile } from '../lib/api';
  import { formatTokens } from '../lib/format';
  import { cycleMode, type FileSelectionMode } from '../lib/selection';
  import { splitHighlight } from '../lib/filter';
  import { buildTree, collectDescendants } from './tree/tree';
  import { flattenRows, ROW_HEIGHT, windowRows } from './tree/rows';
  import Icon from './Icon.svelte';

  // Back-compat: existing importers use `import ... from './FileTree.svelte'`.
  export type { FileSelectionMode };

  /** Fold actions for the workspace keybinds, assigned after mount. */
  interface Props {
    files: TreeFile[];
    selections: Record<string, FileSelectionMode>;
    focusedPath: string | null;
    onFocusFile: (path: string) => void;
    onModeChange: (path: string, mode: FileSelectionMode) => void;
    onBatchModeChange: (paths: string[], mode: FileSelectionMode) => void;
    onSmartSelect: () => void;
    isSmartSelecting: boolean;
    filterQuery: string;
    onFilterChange: (query: string) => void;
    filterInput?: HTMLInputElement | null;
    treeActions?: TreeFoldActions | null;
    /** Visible file order (filter- and collapse-aware) for keyboard nav. */
    visibleFilePaths?: string[];
  }

  // Re-exported via bind:filterInput so parents can focus the filter box.

  let {
    files,
    selections,
    focusedPath,
    onFocusFile,
    onModeChange,
    onBatchModeChange,
    onSmartSelect,
    isSmartSelecting,
    filterQuery,
    onFilterChange,
    filterInput = $bindable(null),
    treeActions = $bindable(null),
    visibleFilePaths = $bindable([]),
  }: Props = $props();

  let collapsedDirs = $state<Record<string, boolean>>({});
  let showBatchMenu = $state(false);
  let containerEl: HTMLDivElement | null = $state(null);
  let scrollTop = $state(0);
  let viewportHeight = $state(500);

  function updateViewport() {
    viewportHeight = containerEl?.clientHeight || 500;
  }

  $effect(() => {
    updateViewport();
    window.addEventListener('resize', updateViewport);
    return () => window.removeEventListener('resize', updateViewport);
  });

  // Filter input element, bindable so parents can focus it (keyboard `/`).

  // Build tree hierarchy (pure helper in ./tree/tree.ts)
  let rootTree = $derived(buildTree(files));

  // Collect descendants for directory selection calculations
  let dirDescendants = $derived(collectDescendants(rootTree));

  let fileByPath = $derived.by(() => {
    const map = new Map<string, TreeFile>();
    for (const f of files) map.set(f.path, f);
    return map;
  });

  // Flatten visible rows based on collapsed state and filter (./tree/rows.ts)
  let flatRows = $derived(
    flattenRows({
      root: rootTree,
      descendants: dirDescendants,
      fileByPath,
      selections,
      collapsedDirs,
      filterQuery,
    }),
  );

  // Windowing calculations (overscan ±10 rows, see ./tree/rows.ts)
  let totalRows = $derived(flatRows.length);
  let win = $derived(windowRows(flatRows, scrollTop, viewportHeight));
  let visibleRows = $derived(win.visibleRows);
  let offsetY = $derived(win.offsetY);
  let totalHeight = $derived(win.totalHeight);

  function toggleDirectory(dirPath: string) {
    collapsedDirs = { ...collapsedDirs, [dirPath]: !collapsedDirs[dirPath] };
  }

  function handleDirCheckboxClick(e: MouseEvent, dirPath: string) {
    e.stopPropagation();
    const descendants = dirDescendants.get(dirPath) || [];
    if (descendants.length === 0) return;
    // TUI parity: the directory aggregate cycles Full → Sigs → Skip → Full.
    // All-sigs advances to skip, all-skip wraps to full, and all-full and
    // mixed sets both advance to sigs.
    const modes = descendants.map((p) => selections[p] || 'full');
    let next: FileSelectionMode;
    if (modes.every((m) => m === 'sigs')) next = 'skip';
    else if (modes.every((m) => m === 'skip')) next = 'full';
    else next = 'sigs';
    onBatchModeChange(descendants, next);
  }

  function expandAll() {
    collapsedDirs = {};
  }

  function collapseAll() {
    const all: Record<string, boolean> = {};
    for (const dirPath of dirDescendants.keys()) all[dirPath] = true;
    collapsedDirs = all;
  }

  // TUI parity: directories start (and reload) fully collapsed. Pre-effect
  // so the collapse lands in the same flush as the new file list, with no
  // expanded first frame. Filtering still force-expands matches.
  $effect.pre(() => {
    const all: Record<string, boolean> = {};
    for (const dirPath of dirDescendants.keys()) all[dirPath] = true;
    collapsedDirs = all;
  });

  // Expose fold actions to the workspace keybinds (E/C, TUI parity).
  $effect(() => {
    treeActions = { expandAll, collapseAll };
  });

  // Visible file order for filter-aware keyboard nav (workspace j/k).
  $effect(() => {
    visibleFilePaths = flatRows.filter((r) => !r.isDir).map((r) => r.node.path);
  });

  // Keep keyboard-driven focus visible (virtual list: manual scroll math,
  // adjusted only when the focused row is outside the window).
  $effect(() => {
    const path = focusedPath;
    const el = containerEl;
    if (!path || !el) return;
    const idx = flatRows.findIndex((r) => !r.isDir && r.node.path === path);
    if (idx === -1) return;
    const top = idx * ROW_HEIGHT;
    if (top < el.scrollTop) el.scrollTop = top;
    else if (top + ROW_HEIGHT > el.scrollTop + el.clientHeight) {
      el.scrollTop = top + ROW_HEIGHT - el.clientHeight;
    }
  });

  // Matched (non-dir) rows while filtering, for the `n/m` counter.
  let matchCount = $derived(flatRows.filter((r) => !r.isDir).length);

  function clearFilter() {
    onFilterChange('');
    filterInput?.focus();
  }

  function handleFilterKey(e: KeyboardEvent) {
    if (e.key === 'Enter') {
      e.preventDefault();
      const first = flatRows.find((r) => !r.isDir);
      if (first) {
        onFocusFile(first.node.path);
        filterInput?.blur();
      }
    } else if (e.key === 'Escape' && filterQuery) {
      // Non-empty: clear but keep focus. Empty: fall through to the global
      // handler, which blurs.
      e.preventDefault();
      e.stopPropagation();
      onFilterChange('');
    }
  }
</script>

<div class="tree">
  <div class="toolbar">
    <div class="filter-wrap">
      <span class="filter-icon"><Icon name="search" size={12} /></span>
      <input
        bind:this={filterInput}
        type="text"
        value={filterQuery}
        oninput={(e) => onFilterChange(e.currentTarget.value)}
        onkeydown={handleFilterKey}
        placeholder="Filter files (/)..."
        class="input input-mono filter-input"
        class:has-query={!!filterQuery}
      />
      {#if filterQuery}
        <span class="font-mono tabular-nums match-count">{matchCount}/{files.length}</span>
        <button onclick={clearFilter} title="Clear filter (Esc)" class="filter-clear">✕</button>
      {/if}
    </div>

    <button
      onclick={onSmartSelect}
      disabled={isSmartSelecting}
      class="btn btn-sm smart-btn"
      title="Smart select based on git relevance and budget (s)"
    >
      <Icon name="sparkles" size={11} />
      <span>{isSmartSelecting ? 'Selecting...' : 'Smart'}</span>
    </button>

    <div class="batch-wrap">
      <button
        onclick={() => (showBatchMenu = !showBatchMenu)}
        class="btn btn-sm btn-ghost batch-btn"
        title="Batch actions"
      >
        ···
      </button>

      {#if showBatchMenu}
        <div class="batch-menu">
          <button
            onclick={() => {
              showBatchMenu = false;
              onBatchModeChange(files.map((f) => f.path), 'full');
            }}
            class="btn btn-sm btn-ghost batch-item"
          >
            Select all (Full)
          </button>
          <button
            onclick={() => {
              showBatchMenu = false;
              onBatchModeChange(files.map((f) => f.path), 'sigs');
            }}
            class="btn btn-sm btn-ghost batch-item"
          >
            Select all (Sigs)
          </button>
          <button
            onclick={() => {
              showBatchMenu = false;
              onBatchModeChange(files.map((f) => f.path), 'skip');
            }}
            class="btn btn-sm btn-ghost batch-item"
          >
            Clear all (Skip)
          </button>
        </div>
      {/if}
    </div>
  </div>

  <div
    bind:this={containerEl}
    role="tree"
    aria-label="Files tree"
    onscroll={(e) => (scrollTop = e.currentTarget.scrollTop)}
    class="tree-list"
  >
    {#if totalRows === 0}
      <div class="no-match">No files match "{filterQuery}" (Esc clears)</div>
    {:else}
      <div class="spacer" style:height={`${totalHeight}px`}>
        <div class="window" style:transform={`translateY(${offsetY}px)`}>
          {#each visibleRows as row (row.key)}
            {@const node = row.node}
            {@const isFocused = !row.isDir && focusedPath === node.path}
            {@const mode = selections[node.path] || 'full'}
            {@const isSkipped = mode === 'skip' && !row.isDir}
            <div
              role="treeitem"
              aria-selected={row.isDir ? undefined : !isSkipped}
              aria-expanded={row.isDir ? row.isExpanded : undefined}
              onclick={() => {
                if (row.isDir) toggleDirectory(node.path);
                else onFocusFile(node.path);
              }}
              onkeydown={(e) => {
                if (e.key === 'Enter' || e.key === ' ') {
                  e.preventDefault();
                  if (row.isDir) toggleDirectory(node.path);
                  else onFocusFile(node.path);
                }
              }}
              tabindex={isFocused ? 0 : -1}
              class="row"
              class:focused={isFocused}
              style:padding-left={`${row.depth * 14 + 6}px`}
            >
              {#if row.isDir}
                <span class="chevron">
                  <Icon name={row.isExpanded ? 'chevron-down' : 'chevron-right'} size={11} />
                </span>
              {:else}
                <span class="chevron-spacer"></span>
              {/if}

              <span class="row-icon" class:dim={isSkipped}>
                <Icon name={row.isDir ? 'folder' : 'file'} size={13} />
              </span>

              <span class="font-mono row-name" class:skipped={isSkipped} class:dir={row.isDir}
                >{#each splitHighlight(node.name, row.match) as seg, si (si)}<span
                    class:hl={seg.hit}>{seg.text}</span
                  >{/each}</span
              >

              {#if !row.isDir && node.file && node.file.score >= 0.75}
                <span class="rel-dot" title={`Git relevance: ${Math.round(node.file.score * 100)}%`}></span>
              {/if}

              {#if row.isDir}
                <div
                  onclick={(e) => handleDirCheckboxClick(e, node.path)}
                  onkeydown={(e) => {
                    if (e.key === 'Enter' || e.key === ' ') {
                      e.preventDefault();
                      handleDirCheckboxClick(e as unknown as MouseEvent, node.path);
                    }
                  }}
                  role="checkbox"
                  aria-checked={row.selectedState === 'all'}
                  tabindex="0"
                  class="dir-check"
                  class:all={row.selectedState === 'all'}
                  class:partial={row.selectedState === 'partial'}
                  class:sigs={row.allSigs}
                  title={`Folder selection (click to cycle full → sigs → skip)`}
                >
                  {#if row.selectedState === 'all'}
                    <span class="check-full"></span>
                  {/if}
                  {#if row.selectedState === 'partial'}
                    <span class="check-partial"></span>
                  {/if}
                </div>
              {:else}
                <button
                  onclick={(e) => {
                    e.stopPropagation();
                    onModeChange(node.path, cycleMode(mode));
                  }}
                  class={`mode-badge ${mode} row-mode`}
                  title="Click to cycle mode (m: full -> sigs -> skip)"
                >
                  {mode}
                </button>
              {/if}

              <span class="font-mono tabular-nums row-tokens" class:skipped={isSkipped}>
                {formatTokens(row.aggregateTokens)}
              </span>
            </div>
          {/each}
        </div>
      </div>
    {/if}
  </div>
</div>

<style>
  .tree {
    flex: 1;
    display: flex;
    flex-direction: column;
    height: 100%;
    background-color: var(--bg);
    overflow: hidden;
    user-select: none;
  }
  .toolbar {
    padding: 6px 8px;
    border-bottom: 1px solid var(--border);
    display: flex;
    align-items: center;
    gap: 6px;
    background-color: var(--surface-raised);
  }
  .filter-wrap {
    position: relative;
    flex: 1;
    display: flex;
    align-items: center;
  }
  .filter-icon {
    position: absolute;
    left: 7px;
    color: var(--ink-faint);
    pointer-events: none;
    display: flex;
  }
  .filter-input {
    width: 100%;
    padding-left: 24px;
    padding-right: 18px;
    height: 24px;
    font-size: 11px;
  }
  .filter-input.has-query {
    padding-right: 64px;
  }
  .match-count {
    position: absolute;
    right: 22px;
    font-size: 9.5px;
    color: var(--ink-faint);
    pointer-events: none;
    white-space: nowrap;
  }
  .row-name .hl {
    background-color: rgba(128, 128, 128, 0.3);
    border-radius: 2px;
  }
  .filter-clear {
    position: absolute;
    right: 5px;
    background: transparent;
    border: none;
    color: var(--ink-faint);
    cursor: pointer;
    font-size: 10px;
  }
  .smart-btn {
    border-color: var(--border);
    color: var(--ink-soft);
    height: 24px;
    padding: 0 7px;
  }
  .batch-wrap {
    position: relative;
  }
  .batch-btn {
    padding: 2px 5px;
    height: 24px;
    color: var(--ink-faint);
  }
  .batch-menu {
    position: absolute;
    top: calc(100% + 4px);
    right: 0;
    width: 150px;
    background-color: var(--bg);
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-sm);
    box-shadow: 0 4px 16px rgba(0, 0, 0, 0.08);
    z-index: 40;
    padding: 3px;
  }
  .batch-item {
    width: 100%;
    justify-content: flex-start;
    font-size: 11px;
  }
  .tree-list {
    flex: 1;
    overflow-y: auto;
    overflow-x: hidden;
    position: relative;
    scrollbar-width: thin;
    scrollbar-color: var(--border-strong) transparent;
  }
  .tree-list::-webkit-scrollbar {
    width: 10px;
  }
  .tree-list::-webkit-scrollbar-track {
    background: transparent;
  }
  .tree-list::-webkit-scrollbar-thumb {
    background-color: var(--border-strong);
    border-radius: 6px;
    border: 3px solid var(--bg);
  }
  .tree-list::-webkit-scrollbar-thumb:hover {
    background-color: var(--ink-faint);
  }
  .no-match {
    padding: 24px;
    text-align: center;
    color: var(--ink-faint);
    font-size: 11px;
    font-family: var(--font-mono);
  }
  .spacer {
    width: 100%;
    position: relative;
  }
  .window {
    position: absolute;
    top: 0;
    left: 0;
    width: 100%;
  }
  .row {
    height: 24px;
    display: flex;
    align-items: center;
    padding-right: 8px;
    background-color: transparent;
    cursor: pointer;
    border-left: 2px solid transparent;
    font-size: 11.5px;
    transition: background-color 50ms ease;
  }
  .row:hover {
    background-color: var(--surface-alt);
  }
  .row.focused {
    background-color: var(--surface-alt);
    border-left: 2px solid var(--accent);
  }
  .chevron {
    width: 14px;
    display: flex;
    align-items: center;
    color: var(--ink-faint);
    margin-right: 2px;
  }
  .chevron-spacer {
    width: 14px;
    margin-right: 2px;
  }
  .row-icon {
    margin-right: 6px;
    display: flex;
    align-items: center;
    color: var(--ink-soft);
    flex-shrink: 0;
  }
  .row-icon.dim {
    color: var(--ink-faint);
  }
  .row-name {
    flex: 1;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--ink);
    font-weight: 400;
    font-size: 11.5px;
    margin-right: 8px;
  }
  .row-name.dir {
    font-weight: 600;
  }
  .row-name.skipped {
    color: var(--ink-faint);
    opacity: 0.75;
  }
  .rel-dot {
    width: 5px;
    height: 5px;
    border-radius: 50%;
    background-color: var(--accent);
    opacity: 0.85;
    margin-right: 6px;
    flex-shrink: 0;
  }
  .dir-check {
    width: 12px;
    height: 12px;
    border: 1px solid var(--border-strong);
    border-radius: 2px;
    margin-right: 8px;
    display: flex;
    align-items: center;
    justify-content: center;
    cursor: pointer;
    flex-shrink: 0;
    background-color: transparent;
  }
  .dir-check.all {
    background-color: var(--accent);
  }
  .dir-check.partial {
    background-color: var(--surface-alt);
  }
  /* All-sigs directory: hollow sigs-tinted box instead of the solid accent. */
  .dir-check.sigs {
    background-color: transparent;
    border-color: var(--mode-sigs);
  }
  .dir-check.sigs .check-full {
    background-color: var(--mode-sigs);
  }
  .check-full {
    width: 6px;
    height: 6px;
    background-color: #12141a;
    border-radius: 1px;
  }
  .check-partial {
    width: 4px;
    height: 2px;
    background-color: var(--ink);
  }
  .row-mode {
    margin-right: 8px;
    flex-shrink: 0;
  }
  .row-tokens {
    font-size: 10.5px;
    color: var(--ink-soft);
    text-align: right;
    min-width: 34px;
    flex-shrink: 0;
    opacity: 0.9;
  }
  .row-tokens.skipped {
    color: var(--ink-faint);
    opacity: 0.6;
  }
</style>
