<script lang="ts">
  import type { TreeFile } from '../lib/api';
  import { formatTokens } from '../lib/format';
  import Icon from './Icon.svelte';

  export type FileSelectionMode = 'full' | 'sigs' | 'skip';

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
  }: Props = $props();

  interface TreeNode {
    name: string;
    path: string;
    isDir: boolean;
    file?: TreeFile;
    children: TreeNode[];
    depth: number;
  }

  interface FlatRow {
    key: string;
    node: TreeNode;
    isDir: boolean;
    depth: number;
    aggregateTokens: number;
    selectedState: 'all' | 'none' | 'partial';
    isExpanded: boolean;
  }

  const ROW_HEIGHT = 24;

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

  // Build tree hierarchy
  let rootTree = $derived.by(() => {
    const root: TreeNode = { name: '', path: '', isDir: true, children: [], depth: -1 };
    const dirMap = new Map<string, TreeNode>();
    dirMap.set('', root);
    const sortedFiles = [...files].sort((a, b) => a.path.localeCompare(b.path));

    for (const file of sortedFiles) {
      const parts = file.path.split('/');
      let currentPath = '';
      let parentNode = root;

      for (let i = 0; i < parts.length - 1; i++) {
        const dirName = parts[i];
        currentPath = currentPath ? `${currentPath}/${dirName}` : dirName;
        let dirNode = dirMap.get(currentPath);
        if (!dirNode) {
          dirNode = { name: dirName, path: currentPath, isDir: true, children: [], depth: i };
          dirMap.set(currentPath, dirNode);
          parentNode.children.push(dirNode);
        }
        parentNode = dirNode;
      }

      parentNode.children.push({
        name: parts[parts.length - 1],
        path: file.path,
        isDir: false,
        file,
        children: [],
        depth: parts.length - 1,
      });
    }
    return root;
  });

  // Collect descendants for directory selection calculations
  let dirDescendants = $derived.by(() => {
    const map = new Map<string, string[]>();
    const traverse = (node: TreeNode): string[] => {
      if (!node.isDir) return node.path ? [node.path] : [];
      const allFiles: string[] = [];
      for (const child of node.children) allFiles.push(...traverse(child));
      if (node.path) map.set(node.path, allFiles);
      return allFiles;
    };
    traverse(rootTree);
    return map;
  });

  let fileByPath = $derived.by(() => {
    const map = new Map<string, TreeFile>();
    for (const f of files) map.set(f.path, f);
    return map;
  });

  // Flatten visible rows based on collapsed state and filter
  let flatRows = $derived.by(() => {
    const query = filterQuery.trim().toLowerCase();
    const rows: FlatRow[] = [];

    const traverse = (node: TreeNode) => {
      if (node.depth === -1) {
        for (const child of node.children) traverse(child);
        return;
      }

      if (node.isDir) {
        const filePaths = dirDescendants.get(node.path) || [];
        let dirTokens = 0;
        let selectedCount = 0;
        let matchesQuery = false;

        for (const p of filePaths) {
          const f = fileByPath.get(p);
          const mode = selections[p] || 'full';
          if (f) {
            const tokenVal = mode === 'sigs' ? Math.floor(f.tokens * 0.22) : f.tokens;
            if (mode !== 'skip') {
              dirTokens += tokenVal;
              selectedCount++;
            }
          }
          if (query && p.toLowerCase().includes(query)) matchesQuery = true;
        }

        if (query && !matchesQuery && !node.path.toLowerCase().includes(query)) return;

        const totalFiles = filePaths.length;
        const selectedState: 'all' | 'none' | 'partial' =
          selectedCount === 0 ? 'none' : selectedCount === totalFiles ? 'all' : 'partial';
        const isExpanded = query ? true : !collapsedDirs[node.path];

        rows.push({
          key: node.path,
          node,
          isDir: true,
          depth: node.depth,
          aggregateTokens: dirTokens,
          selectedState,
          isExpanded,
        });

        if (isExpanded) for (const child of node.children) traverse(child);
      } else {
        if (query && !node.path.toLowerCase().includes(query)) return;
        const mode = selections[node.path] || 'full';
        const fileTokens =
          mode === 'sigs' ? Math.floor((node.file?.tokens || 0) * 0.22) : node.file?.tokens || 0;
        rows.push({
          key: node.path,
          node,
          isDir: false,
          depth: node.depth,
          aggregateTokens: fileTokens,
          selectedState: mode === 'skip' ? 'none' : 'all',
          isExpanded: false,
        });
      }
    };

    traverse(rootTree);
    return rows;
  });

  // Windowing calculations
  let totalRows = $derived(flatRows.length);
  let startIndex = $derived(Math.max(0, Math.floor(scrollTop / ROW_HEIGHT) - 10));
  let endIndex = $derived(Math.min(totalRows, Math.ceil((scrollTop + viewportHeight) / ROW_HEIGHT) + 10));
  let visibleRows = $derived(flatRows.slice(startIndex, endIndex));
  let offsetY = $derived(startIndex * ROW_HEIGHT);
  let totalHeight = $derived(totalRows * ROW_HEIGHT);

  function toggleDirectory(dirPath: string) {
    collapsedDirs = { ...collapsedDirs, [dirPath]: !collapsedDirs[dirPath] };
  }

  function handleDirCheckboxClick(e: MouseEvent, dirPath: string) {
    e.stopPropagation();
    const descendants = dirDescendants.get(dirPath) || [];
    if (descendants.length === 0) return;
    const anyActive = descendants.some((p) => selections[p] && selections[p] !== 'skip');
    onBatchModeChange(descendants, anyActive ? 'skip' : 'full');
  }

  function cycleMode(currentMode: FileSelectionMode): FileSelectionMode {
    if (currentMode === 'full') return 'sigs';
    if (currentMode === 'sigs') return 'skip';
    return 'full';
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
        placeholder="Filter files (/)..."
        class="input input-mono filter-input"
      />
      {#if filterQuery}
        <button onclick={() => onFilterChange('')} class="filter-clear">✕</button>
      {/if}
    </div>

    <button
      onclick={onSmartSelect}
      disabled={isSmartSelecting}
      class="btn btn-sm smart-btn"
      title="Smart select based on git relevance and budget (s)"
    >
      <Icon name="sparkles" size={11} />
      <span>Smart</span>
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
      <div class="no-match">No files match "{filterQuery}"</div>
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

              <span class="font-mono row-name" class:skipped={isSkipped} class:dir={row.isDir}>
                {node.name}
              </span>

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
                  title={`Folder selection: ${row.selectedState} (click to toggle all)`}
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
                  title="Click to cycle mode (f: full -> sigs -> skip)"
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
