<script lang="ts">
  import {
    api,
    type TreeFile,
    type PackResult,
    type RecentProject,
    type ApiMeta,
  } from '../lib/api';
  import FileTree, { type TreeFoldActions } from '../components/FileTree.svelte';
  import CodePreview from '../components/CodePreview.svelte';
  import OutputView from '../components/OutputView.svelte';
  import BudgetMeter from '../components/BudgetMeter.svelte';
  import TaskPrompt from '../components/TaskPrompt.svelte';
  import RedactionModal from '../components/RedactionModal.svelte';
  import Icon from '../components/Icon.svelte';
  import TopBar from './workspace/TopBar.svelte';
  import { formatTokens } from '../lib/format';
  import { cycleMode, isIncluded, modeTokens, toggleMode, type FileSelectionMode } from '../lib/selection';
  import { isEditingTarget, isPlainKey } from '../lib/keyboard';

  interface Props {
    projectRoot: string;
    meta: ApiMeta | null;
    onNavigate: (path: string) => void;
    onOpenShortcuts: () => void;
    onOpenThemePicker?: () => void;
  }

  let { projectRoot, meta, onNavigate, onOpenShortcuts, onOpenThemePicker }: Props = $props();

  // Tree & file state
  let files = $state<TreeFile[]>([]);
  let selections = $state<Record<string, FileSelectionMode>>({});
  let focusedPath = $state<string | null>(null);
  let filterQuery = $state('');
  let loadingTree = $state(true);
  let treeError = $state<string | null>(null);

  // File preview state
  let previewContent = $state('');
  let previewTokens = $state(0);
  let previewLang = $state('go');
  let previewMode = $state<'full' | 'sigs'>('sigs');
  let previewLoading = $state(false);

  // Pack & Document state
  let taskPrompt = $state('');
  let activeTab = $state<'preview' | 'output'>('preview');
  let packResult = $state<PackResult | null>(null);
  let isGenerating = $state(false);
  let isSmartSelecting = $state(false);
  let copiedNotification = $state(false);

  // Top/footer configuration
  let selectedStyle = $state('xml');
  let budget = $state(64000);
  let redact = $state(true);
  let showRedactionModal = $state(false);

  // Recents for project switcher
  let recents = $state<RecentProject[]>([]);

  // Responsive sidebar state
  let sidebarOpen = $state(true);
  let windowWidth = $state(typeof window !== 'undefined' ? window.innerWidth : 1200);
  let isCompact = $derived(windowWidth < 900);

  let filterEl = $state<HTMLInputElement | null>(null);
  let treeActions = $state<TreeFoldActions | null>(null);
  let copyTimer: ReturnType<typeof setTimeout> | null = null;

  // Load project tree (re-runs when projectRoot changes; stale responses ignored).
  $effect(() => {
    const root = projectRoot;
    let alive = true;
    loadingTree = true;
    treeError = null;

    // Record the open so recents stay fresh; never blocks the tree.
    api.recordRecent(root).catch(() => {});

    Promise.all([api.getTree(root), api.getRecents()])
      .then(([treeRes, recentsRes]) => {
        if (!alive) return;
        files = treeRes.files;
        recents = recentsRes;

        const initial: Record<string, FileSelectionMode> = {};
        for (const file of treeRes.files) {
          if (file.path.includes('main.go') || file.path.includes('clone.go') || file.path.includes('copy.go')) {
            initial[file.path] = 'sigs';
          } else if (
            file.path.includes('diff.go') ||
            file.path.includes('dump.go') ||
            file.path.includes('watch.go') ||
            file.path.includes('completion.go')
          ) {
            initial[file.path] = 'full';
          } else if (
            file.path.includes('delta.go') ||
            file.path.includes('mcp.go') ||
            file.path.includes('record.go') ||
            file.path.includes('pick.go')
          ) {
            initial[file.path] = 'skip';
          } else if (file.score >= 0.85) {
            initial[file.path] = 'full';
          } else if (file.score >= 0.6) {
            initial[file.path] = 'sigs';
          } else {
            initial[file.path] = 'skip';
          }
        }
        selections = initial;

        const mainFile = treeRes.files.find((f) => f.path === 'cmd/sift/main.go') || treeRes.files[0];
        if (mainFile) focusedPath = mainFile.path;
        loadingTree = false;
      })
      .catch((err) => {
        if (!alive) return;
        treeError = err instanceof Error ? err.message : 'Failed to load project tree';
        loadingTree = false;
      });

    return () => {
      alive = false;
    };
  });

  // Load file preview when focused file or mode changes.
  $effect(() => {
    const root = projectRoot;
    const path = focusedPath;
    const mode = previewMode;
    if (!path) return;
    let alive = true;
    previewLoading = true;

    api
      .getFile(root, path, mode)
      .then((res) => {
        if (!alive) return;
        previewContent = res.content;
        previewTokens = res.tokens;
        previewLang = res.language;
        previewLoading = false;
      })
      .catch((err) => {
        if (!alive) return;
        previewContent = `// Failed to load file: ${err instanceof Error ? err.message : err}`;
        previewLoading = false;
      });

    return () => {
      alive = false;
    };
  });

  function handleFocusFile(path: string) {
    focusedPath = path;
  }

  function handleModeChange(path: string, mode: FileSelectionMode) {
    selections = { ...selections, [path]: mode };
    if (path === focusedPath) {
      if (mode === 'sigs') previewMode = 'sigs';
      else if (mode === 'full') previewMode = 'full';
    }
  }

  function handleBatchModeChange(paths: string[], mode: FileSelectionMode) {
    const updated = { ...selections };
    for (const p of paths) updated[p] = mode;
    selections = updated;
  }

  async function handleSmartSelect() {
    try {
      isSmartSelecting = true;
      const res = await api.smartSelect(projectRoot, budget);
      selections = res.selections;
    } catch (err) {
      console.error(err);
    } finally {
      isSmartSelecting = false;
    }
  }

  let fileByPath = $derived.by(() => {
    const map = new Map<string, TreeFile>();
    for (const f of files) map.set(f.path, f);
    return map;
  });

  // Live selected count & tokens calculation
  let selectedCount = $derived.by(() => {
    let count = 0;
    for (const [path, mode] of Object.entries(selections)) {
      if (mode === 'skip') continue;
      if (fileByPath.has(path)) count++;
    }
    return count;
  });
  let usedTokens = $derived.by(() => {
    let tokens = 0;
    for (const [path, mode] of Object.entries(selections)) {
      if (!isIncluded(mode)) continue;
      const file = fileByPath.get(path);
      if (!file) continue;
      tokens += modeTokens(file.tokens, mode);
    }
    return tokens;
  });

  async function handleGenerate() {
    if (isGenerating || selectedCount === 0) return;
    isGenerating = true;
    activeTab = 'output';

    try {
      packResult = await api.pack({
        root: projectRoot,
        selections,
        budget,
        style: selectedStyle,
        prompt: taskPrompt,
        redact,
      });
    } catch (err) {
      console.error(err);
    } finally {
      isGenerating = false;
    }
  }

  async function handleGenerateAndCopy() {
    if (isGenerating || selectedCount === 0) return;
    isGenerating = true;
    activeTab = 'output';

    try {
      const res = await api.pack({
        root: projectRoot,
        selections,
        budget,
        style: selectedStyle,
        prompt: taskPrompt,
        redact,
      });
      packResult = res;
      await navigator.clipboard.writeText(res.document);
      copiedNotification = true;
      if (copyTimer) clearTimeout(copyTimer);
      copyTimer = setTimeout(() => (copiedNotification = false), 2000);
    } catch (err) {
      console.error(err);
    } finally {
      isGenerating = false;
    }
  }

  function handleDownload() {
    if (!packResult || !packResult.document) {
      void handleGenerate();
      return;
    }
    const name = projectRoot.split('/').filter(Boolean).pop() || 'project';
    const safeName = name.replace(/[^a-zA-Z0-9._-]+/g, '_');
    let ext = 'txt';
    if (selectedStyle === 'xml') ext = 'xml';
    else if (selectedStyle === 'markdown') ext = 'md';

    const blob = new Blob([packResult.document], { type: 'text/plain;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = `${safeName}-context.${ext}`;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    URL.revokeObjectURL(url);
  }

  function handleRedactionToggle() {
    if (redact) showRedactionModal = true;
    else redact = true;
  }

  let filePathsList = $derived(files.map((f) => f.path));
  let stylesList = $derived(meta?.styles || ['xml', 'markdown', 'plain']);
  let projectName = $derived(projectRoot.split('/').filter(Boolean).pop() || 'project');

  function focusFilter() {
    if (!sidebarOpen) sidebarOpen = true;
    setTimeout(() => filterEl?.focus(), 50);
  }

  function handleWindowKey(e: KeyboardEvent) {
    if (isEditingTarget(e)) {
      if (e.key === 'Escape') (e.target as HTMLElement).blur();
      return;
    }

    if (isPlainKey(e, '?')) {
      e.preventDefault();
      onOpenShortcuts();
      return;
    }
    if (isPlainKey(e, 't') || isPlainKey(e, 'T')) {
      e.preventDefault();
      onOpenThemePicker?.();
      return;
    }
    if (isPlainKey(e, 'b') || isPlainKey(e, 'B')) {
      e.preventDefault();
      sidebarOpen = !sidebarOpen;
      return;
    }
    if (isPlainKey(e, '/')) {
      e.preventDefault();
      focusFilter();
      return;
    }
    if (isPlainKey(e, 's')) {
      e.preventDefault();
      void handleSmartSelect();
      return;
    }
    if (isPlainKey(e, 'g')) {
      e.preventDefault();
      void handleGenerate();
      return;
    }
    if (isPlainKey(e, 'y') || isPlainKey(e, 'Y')) {
      e.preventDefault();
      void handleGenerateAndCopy();
      return;
    }
    if ((e.key === 'j' || e.key === 'ArrowDown') && filePathsList.length > 0) {
      e.preventDefault();
      const currentIndex = focusedPath ? filePathsList.indexOf(focusedPath) : -1;
      handleFocusFile(filePathsList[Math.min(filePathsList.length - 1, currentIndex + 1)]);
      return;
    }
    if ((e.key === 'k' || e.key === 'ArrowUp') && filePathsList.length > 0) {
      e.preventDefault();
      const currentIndex = focusedPath ? filePathsList.indexOf(focusedPath) : 0;
      handleFocusFile(filePathsList[Math.max(0, currentIndex - 1)]);
      return;
    }
    if (e.key === ' ' && focusedPath) {
      e.preventDefault();
      handleModeChange(focusedPath, toggleMode(selections[focusedPath] || 'full'));
      return;
    }
    if ((e.key === 'm' || e.key === 'M') && focusedPath) {
      e.preventDefault();
      handleModeChange(focusedPath, cycleMode(selections[focusedPath] || 'full'));
      return;
    }
    if (e.key === 'E') {
      e.preventDefault();
      treeActions?.expandAll();
      return;
    }
    if (e.key === 'C') {
      e.preventDefault();
      treeActions?.collapseAll();
      return;
    }
  }
</script>

<svelte:window onkeydown={handleWindowKey} onresize={() => (windowWidth = window.innerWidth)} />

<div class="workspace">
  <TopBar
    {sidebarOpen}
    onToggleSidebar={() => (sidebarOpen = !sidebarOpen)}
    {projectRoot}
    {recents}
    {onNavigate}
    {onOpenThemePicker}
    {budget}
    onBudgetChange={(b) => (budget = b)}
    {redact}
    onRedactToggle={handleRedactionToggle}
    fileCount={files.length}
  />

  <div class="panes">
    {#if sidebarOpen}
      <div class="sidebar" class:compact={isCompact}>
        {#if loadingTree}
          <div class="pane-status">Scanning repository...</div>
        {:else if treeError}
          <div class="pane-error">{treeError}</div>
        {:else}
          <FileTree
            {files}
            {selections}
            {focusedPath}
            onFocusFile={(p) => {
              handleFocusFile(p);
              if (isCompact) sidebarOpen = false;
            }}
            onModeChange={handleModeChange}
            onBatchModeChange={handleBatchModeChange}
            onSmartSelect={() => void handleSmartSelect()}
            {isSmartSelecting}
            {filterQuery}
            onFilterChange={(q) => (filterQuery = q)}
            bind:filterInput={filterEl}
            bind:treeActions
          />
        {/if}
      </div>
    {/if}

    {#if !isCompact || !sidebarOpen}
      <div class="main">
        <div class="tabs">
          <div class="tabs-left">
            {#if !sidebarOpen || isCompact}
              <button
                onclick={() => (sidebarOpen = true)}
                class="btn btn-sm btn-ghost files-btn"
                title="Open files sidebar (b)"
              >
                <Icon name="sidebar" size={12} />
                <span>Files</span>
              </button>
            {/if}

            <div class="segmented-control">
              <button
                onclick={() => (activeTab = 'preview')}
                class="segmented-btn"
                class:active={activeTab === 'preview'}
              >
                Preview
              </button>
              <button onclick={() => (activeTab = 'output')} class="segmented-btn tab-with-count" class:active={activeTab === 'output'}>
                <span>Output</span>
                {#if packResult}
                  <span class="font-mono tabular-nums out-count">({formatTokens(packResult.tokens)})</span>
                {/if}
              </button>
            </div>
          </div>
        </div>

        {#if activeTab === 'preview'}
          <TaskPrompt prompt={taskPrompt} onChange={(v) => (taskPrompt = v)} />
        {/if}

        <div class="tab-body">
          {#if activeTab === 'preview'}
            <CodePreview
              filePath={focusedPath}
              content={previewContent}
              tokens={previewTokens}
              language={previewLang}
              mode={previewMode}
              isLoading={previewLoading}
              onModeToggle={(m) => {
                previewMode = m;
                if (focusedPath) handleModeChange(focusedPath, m);
              }}
            />
          {:else}
            <OutputView
              {packResult}
              {projectName}
              style={selectedStyle}
              {isGenerating}
              onRegenerate={() => void handleGenerate()}
            />
          {/if}
        </div>
      </div>
    {/if}
  </div>

  <BudgetMeter
    {selectedCount}
    {usedTokens}
    {budget}
    style={selectedStyle}
    {stylesList}
    onStyleChange={(s) => (selectedStyle = s)}
    {isGenerating}
    {isSmartSelecting}
    onGenerate={() => void handleGenerate()}
    onCopy={() => void handleGenerateAndCopy()}
    onDownload={handleDownload}
    onOpenShortcuts={onOpenShortcuts}
    onSmartSelect={() => void handleSmartSelect()}
    onFocusFilter={focusFilter}
    hasOutput={!!packResult}
    isCopied={copiedNotification}
  />

  <RedactionModal
    isOpen={showRedactionModal}
    onConfirm={() => {
      redact = false;
      showRedactionModal = false;
    }}
    onCancel={() => (showRedactionModal = false)}
  />
</div>

<style>
  .workspace {
    flex: 1;
    display: flex;
    flex-direction: column;
    height: 100%;
    background-color: var(--bg);
    overflow: hidden;
  }
  .panes {
    flex: 1;
    display: flex;
    overflow: hidden;
    background-color: var(--bg);
  }
  .sidebar {
    width: 320px;
    min-width: 260px;
    max-width: 440px;
    border-right: 1px solid var(--border);
    display: flex;
    flex-direction: column;
    background-color: var(--bg);
    flex-shrink: 0;
  }
  .sidebar.compact {
    width: 100%;
    min-width: 100%;
    max-width: 100%;
    border-right: none;
  }
  .pane-status {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    color: var(--ink-faint);
    font-size: 11px;
    font-family: var(--font-mono);
  }
  .pane-error {
    flex: 1;
    padding: 16px;
    text-align: center;
    color: var(--status-danger);
    font-size: 11px;
  }
  .main {
    flex: 1;
    display: flex;
    flex-direction: column;
    overflow: hidden;
    background-color: var(--code-bg);
  }
  .tabs {
    height: 30px;
    min-height: 30px;
    background-color: var(--surface-raised);
    border-bottom: 1px solid var(--border);
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0 8px;
  }
  .tabs-left {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .files-btn {
    padding: 2px 6px;
    font-size: 11px;
    font-family: var(--font-mono);
    color: var(--ink-soft);
    display: inline-flex;
    align-items: center;
    gap: 4px;
  }
  .tab-with-count {
    display: inline-flex;
    align-items: center;
    gap: 5px;
  }
  .out-count {
    font-size: 9.5px;
    opacity: 0.75;
  }
  .tab-body {
    flex: 1;
    display: flex;
    overflow: hidden;
  }
</style>
