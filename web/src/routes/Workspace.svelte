<script lang="ts">
  import {
    api,
    type TreeFile,
    type PackResult,
    type RecentProject,
    type ApiMeta,
    type FollowHit,
  } from '../lib/api';
  import FileTree, { type TreeFoldActions } from '../components/FileTree.svelte';
  import CodePreview from '../components/CodePreview.svelte';
  import OutputView from '../components/OutputView.svelte';
  import BudgetMeter from '../components/BudgetMeter.svelte';
  import TaskPrompt from '../components/TaskPrompt.svelte';
  import Icon from '../components/Icon.svelte';
  import TopBar from './workspace/TopBar.svelte';
  import { formatBytes, formatTokens } from '../lib/format';
  import { cycleMode, isIncluded, modeTokens, toggleMode, type FileSelectionMode } from '../lib/selection';
  import type { MenuItem } from '../lib/contextmenu';
  import { openCtxMenu } from '../lib/ctxmenu.svelte';
  import { toast } from '../lib/toast.svelte';
  import { isTempCloneRoot } from '../lib/clones.svelte';
  import { isEditingTarget, isPlainKey } from '../lib/keyboard';
  import { getUIPrefs, setUIPrefs } from '../lib/persist';

  interface Props {
    projectRoot: string;
    meta: ApiMeta | null;
    onNavigate: (path: string) => void;
    onOpenShortcuts: () => void;
    onOpenThemePicker?: () => void;
    onOpenSettings: () => void;
    /** Reports the open project name for the window title (app mode). */
    onProjectTitle?: (name: string) => void;
  }

  let { projectRoot, meta, onNavigate, onOpenShortcuts, onOpenThemePicker, onOpenSettings, onProjectTitle }: Props = $props();

  // Tree & file state
  let files = $state<TreeFile[]>([]);
  let selections = $state<Record<string, FileSelectionMode>>({});
  let focusedPath = $state<string | null>(null);
  let filterQuery = $state('');
  let loadingTree = $state(true);
  let treeError = $state<string | null>(null);

  // Follow walk state: the applied hit set plus the selections it replaced,
  // so clearing restores exactly what was there. Session-only by design.
  let followState = $state<{
    seed: string;
    direction: string;
    hits: FollowHit[];
    unanalyzed: number;
    prev: Record<string, FileSelectionMode>;
  } | null>(null);
  let isFollowing = $state(false);

  // File preview state
  let previewContent = $state('');
  let previewTokens = $state(0);
  let previewLang = $state('go');
  let previewMode = $state<'full' | 'sigs'>('sigs');
  let previewSpans = $state<Array<[number, number, number, string]> | undefined>(undefined);
  let previewLoading = $state(false);

  // Pack & Document state
  let taskPrompt = $state('');
  // Layout prefs (sidebar, tab) survive reloads; see lib/persist.ts.
  let uiPrefs = getUIPrefs();
  let activeTab = $state<'preview' | 'output'>(uiPrefs.tab);
  let packResult = $state<PackResult | null>(null);
  let isGenerating = $state(false);
  let isSmartSelecting = $state(false);
  let copiedNotification = $state(false);

  // Top/footer configuration. The format choice persists: the server default
  // seeds it, the user's own pick is kept (see lib/persist.ts).
  let selectedStyle = $state(uiPrefs.style);
  let budget = $state(64000);
  // Where the displayed budget came from. A preset pick sets a session-only
  // override (still sent with pack/smart-select); otherwise the server's
  // resolved value wins: flag > .sift.toml > persisted default > builtin.
  let treeBudgetSource = $state<'toml' | 'flag' | 'default'>('default');
  let budgetOverride = $state<number | null>(null);
  // Secret redaction is always on in the web UI: there is no toggle.

  // Recents for project switcher
  let recents = $state<RecentProject[]>([]);

  // Responsive sidebar state
  let sidebarOpen = $state(uiPrefs.sidebarOpen);
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
    followState = null;
    // A new project starts from the server's resolved budget, not the last
    // session's override.
    budgetOverride = null;

    // Record the open so recents stay fresh; never blocks the tree.
    api.recordRecent(root).catch(() => {});

    Promise.all([api.getTree(root), api.getRecents()])
      .then(([treeRes, recentsRes]) => {
        if (!alive) return;
        files = treeRes.files;
        recents = recentsRes;
        // Adopt the resolved budget unless the user already picked a preset
        // while the tree was loading (their explicit pick wins).
        if (budgetOverride === null) {
          budget = treeRes.budget;
          treeBudgetSource = treeRes.budgetSource;
        }

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
        previewSpans = res.spans;
        previewLoading = false;
      })
      .catch((err) => {
        if (!alive) return;
        previewContent = `// Failed to load file: ${err instanceof Error ? err.message : err}`;
        previewSpans = undefined;
        previewLoading = false;
      });

    return () => {
      alive = false;
    };
  });

  function handleFocusFile(path: string) {
    focusedPath = path;
    // A picked file belongs in the preview: following it there beats staring
    // at a stale output document.
    if (activeTab !== 'preview') activeTab = 'preview';
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

  // Right-click on an explorer row: file-aware items through the shared menu.
  // preventDefault also keeps the App-level fallback from firing its own.
  function handleRowContextMenu(e: MouseEvent, path: string, isDir: boolean) {
    e.preventDefault();
    const opener = e.target instanceof HTMLElement ? e.target : null;
    const items: MenuItem[] = [];
    if (!isDir) {
      items.push({
        id: 'open',
        label: 'Open preview',
        hint: 'Enter',
        disabled: false,
        run: () => {
          handleFocusFile(path);
        },
      });
    }
    const fullPath = `${projectRoot.replace(/\/+$/, '')}/${path}`;
    if (isDir) {
      const under = files.filter((f) => f.path.startsWith(path + '/')).map((f) => f.path);
      const anyIn = under.some((p) => (selections[p] || 'full') !== 'skip');
      items.push(
        anyIn
          ? {
              id: 'exclude-dir',
              label: 'Exclude folder',
              hint: '',
              disabled: under.length === 0,
              run: () => {
                handleBatchModeChange(under, 'skip');
              },
            }
          : {
              id: 'include-dir',
              label: 'Include folder',
              hint: '',
              disabled: under.length === 0,
              run: () => {
                handleBatchModeChange(under, 'full');
              },
            },
      );
    } else {
      const mode = selections[path] || 'full';
      // Explicit results instead of "skip → full": name what the file
      // becomes and leave out the mode it already has.
      const setTo = (id: string, label: string, to: FileSelectionMode, hint = ''): MenuItem => ({
        id,
        label,
        hint,
        disabled: false,
        run: () => {
          handleModeChange(path, to);
        },
      });
      if (mode === 'full') {
        items.push(setTo('to-sigs', 'Signatures only', 'sigs', 'M'));
        items.push(setTo('to-skip', 'Exclude from output', 'skip', 'Space'));
      } else if (mode === 'sigs') {
        items.push(setTo('to-full', 'Include full file', 'full'));
        items.push(setTo('to-skip', 'Exclude from output', 'skip', 'Space'));
      } else {
        items.push(setTo('to-full', 'Include as full file', 'full', 'Space'));
        items.push(setTo('to-sigs', 'Include as signatures only', 'sigs'));
      }
    }
    const copyPath = (text: string): (() => Promise<void>) => async () => {
      try {
        await navigator.clipboard.writeText(text);
        toast({ id: 'copy-path', kind: 'success', message: 'Path copied', duration: 1800 });
      } catch {
        toast({ id: 'copy-path', kind: 'error', message: 'Couldn’t copy the path' });
      }
    };
    if (!isDir && files.some((f) => f.path === path && f.followable)) {
      const follow = (
        direction: 'deps' | 'dependents',
        label: string,
        first: boolean,
      ): MenuItem => ({
        id: `follow-${direction}`,
        label,
        hint: '',
        disabled: isFollowing,
        separatorBefore: first,
        run: () => {
          void handleFollowFile(path, direction);
        },
      });
      items.push(follow('dependents', 'Show dependents', true));
      items.push(follow('deps', 'Show dependencies', false));
    }
    items.push(
      {
        id: 'expand-all',
        label: 'Expand all',
        hint: 'E',
        disabled: treeActions === null,
        separatorBefore: true,
        run: () => {
          treeActions?.expandAll();
        },
      },
      {
        id: 'collapse-all',
        label: 'Collapse all',
        hint: 'C',
        disabled: treeActions === null,
        run: () => {
          treeActions?.collapseAll();
        },
      },
    );
    items.push({
      id: 'copy-full',
      label: 'Copy full path',
      hint: '',
      disabled: false,
      separatorBefore: true,
      run: copyPath(fullPath),
    });
    items.push({
      id: 'copy-rel',
      label: 'Copy relative path',
      hint: '',
      disabled: false,
      run: copyPath(path),
    });
    openCtxMenu(e.clientX, e.clientY, items, opener);
  }

  // Follow walk from the explorer: replaces the selection with exactly what
  // the CLI follow picks (seed full, near hops full, far hops signatures)
  // and remembers the previous selection for Clear. Failures toast the
  // server's reason; the selection is untouched.
  async function handleFollowFile(path: string, direction: 'deps' | 'dependents') {
    if (isFollowing) return;
    isFollowing = true;
    focusedPath = path;
    try {
      const res = await api.follow({ root: projectRoot, path, direction });
      const prev = { ...selections };
      const next: Record<string, FileSelectionMode> = {};
      for (const f of files) next[f.path] = 'skip';
      for (const h of res.hits) next[h.path] = h.mode;
      selections = next;
      followState = {
        seed: res.seed,
        direction: res.direction,
        hits: res.hits,
        unanalyzed: res.unanalyzed,
        prev,
      };
    } catch (err) {
      toast({
        id: 'follow-failed',
        kind: 'error',
        message: err instanceof Error ? err.message : 'Follow walk failed',
      });
    } finally {
      isFollowing = false;
    }
  }

  function clearFollow() {
    if (!followState) return;
    selections = followState.prev;
    followState = null;
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

  // A preset pick is a session-only override (still sent with requests).
  // It also becomes the persisted server default so it sticks for projects
  // without their own budget — unless a .sift.toml (or serve flag) sets one,
  // which stays authoritative and keeps the pick session-local.
  function handleBudgetChange(b: number) {
    budgetOverride = b;
    budget = b;
    if (treeBudgetSource === 'default') {
      void api
        .getSettings()
        .then((s) => api.saveSettings({ ...s, defaultBudget: b }))
        .catch(() => {});
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
        redact: true,
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
        redact: true,
      });
      packResult = res;
      try {
        await navigator.clipboard.writeText(res.document);
        copiedNotification = true;
        if (copyTimer) clearTimeout(copyTimer);
        copyTimer = setTimeout(() => (copiedNotification = false), 2000);
      } catch (err) {
        // The pack itself succeeded; only the clipboard write failed. Say so,
        // and point at the output they can still copy or save.
        console.error(err);
        toast({
          id: 'copy-failed',
          kind: 'error',
          message: 'Couldn’t copy to the clipboard — the output is ready in the Output tab.',
        });
      }
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
    const filename = `${safeName}-context.${ext}`;
    const link = document.createElement('a');
    link.href = url;
    link.download = filename;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    URL.revokeObjectURL(url);
    // An anchor click cannot report whether the browser actually saved the
    // file (and --app windows show no download shelf), so say what we did:
    // started the download, with the size — never "saved".
    toast({
      id: 'download',
      kind: 'success',
      message: `Downloading ${filename} · ${formatBytes(blob.size)}`,
    });
  }

  // Persist layout prefs (pure view state: sidebar, tab, format, last
  // project — see lib/persist.ts). No server data is cached here.
  $effect(() => {
    // A temporary clone dies with the server, so it must never become the
    // "resume" target: keep whatever real project was remembered before.
    setUIPrefs({
      sidebarOpen,
      tab: activeTab,
      style: selectedStyle,
      lastProject: isTempCloneRoot(projectRoot) ? getUIPrefs().lastProject : projectRoot,
    });
  });

  let filePathsList = $derived(files.map((f) => f.path));
  // Keyboard nav follows the visible tree order (filter- and
  // collapse-aware) whenever the sidebar is mounted; otherwise all files.
  let visiblePaths = $state<string[]>([]);
  let navList = $derived(sidebarOpen && visiblePaths.length > 0 ? visiblePaths : filePathsList);
  let stylesList = $derived(meta?.styles || ['xml', 'markdown', 'plain']);
  let projectName = $derived(projectRoot.split('/').filter(Boolean).pop() || 'project');

  // App windows show this in their titlebar; the browser tab keeps its own.
  $effect(() => {
    onProjectTitle?.(projectName);
  });

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
    if ((e.key === 'j' || e.key === 'ArrowDown') && navList.length > 0) {
      e.preventDefault();
      const currentIndex = focusedPath ? navList.indexOf(focusedPath) : -1;
      handleFocusFile(navList[Math.min(navList.length - 1, currentIndex + 1)]);
      return;
    }
    if ((e.key === 'k' || e.key === 'ArrowUp') && navList.length > 0) {
      e.preventDefault();
      const currentIndex = focusedPath ? navList.indexOf(focusedPath) : 0;
      handleFocusFile(navList[Math.max(0, currentIndex - 1)]);
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

<div class="workspace anim-page">
  <TopBar
    {sidebarOpen}
    onToggleSidebar={() => (sidebarOpen = !sidebarOpen)}
    {projectRoot}
    {recents}
    {onNavigate}
    {onOpenThemePicker}
    {onOpenSettings}
    {budget}
    onBudgetChange={handleBudgetChange}
    budgetSource={budgetOverride !== null ? 'custom' : treeBudgetSource}
    fileCount={files.length}
  />

  <div class="panes">
    {#if sidebarOpen}
      <div class="sidebar" class:compact={isCompact}>
        {#if loadingTree}
          <div class="pane-scan" role="status" aria-label="Scanning repository">
            {#each [96, 82, 88, 70, 92, 78] as w, i (i)}
              <div class="skeleton skel-line" style:width={`${w}%`} style:margin-left={`${(i % 3) * 12}px`}></div>
            {/each}
          </div>
        {:else if treeError}
          <div class="pane-error">{treeError}</div>
        {:else}
          {#if followState}
            {@const hopCounts = (() => {
              const byHop = new Map<number, number>();
              for (const h of followState.hits) {
                if (h.distance === 0) continue;
                byHop.set(h.distance, (byHop.get(h.distance) ?? 0) + 1);
              }
              return [...byHop.entries()]
                .sort((a, b) => a[0] - b[0])
                .map(([d, n]) => `hop ${d}: ${n}`)
                .join(', ');
            })()}
            <div class="follow-banner" role="status">
              <span
                class="follow-text"
                title={`${followState.seed} · ${followState.direction} · ${hopCounts}${followState.unanalyzed > 0 ? ` · ${followState.unanalyzed} files need a resolver` : ''}`}
              >
                Following {followState.seed} · {followState.direction} · {followState.hits.length} files
              </span>
              <button class="btn btn-sm btn-ghost" onclick={clearFollow} title="Restore previous selection">
                Clear
              </button>
            </div>
          {/if}
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
            bind:visibleFilePaths={visiblePaths}
            bind:treeActions
            onRowContextMenu={handleRowContextMenu}
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
              spans={previewSpans}
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
</div>

<style>
  .workspace {
    flex: 1;
    min-height: 0;
    display: flex;
    flex-direction: column;
    height: 100%;
    background-color: var(--bg);
    overflow: hidden;
  }
  .panes {
    flex: 1;
    min-height: 0;
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
  .pane-scan {
    flex: 1;
    display: flex;
    flex-direction: column;
    gap: 10px;
    padding: 14px 12px;
  }
  .skel-line {
    height: 12px;
  }
  .pane-error {
    flex: 1;
    padding: 16px;
    text-align: center;
    color: var(--status-danger);
    font-size: 11px;
  }
  .follow-banner {
    display: flex;
    align-items: center;
    gap: 6px;
    padding: 6px 8px;
    border-bottom: 1px solid var(--border);
    font-size: 11px;
  }
  .follow-text {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    color: var(--ink-faint);
    font-family: var(--font-mono);
  }
  .main {
    flex: 1;
    min-width: 0;
    min-height: 0;
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
    min-height: 0;
    min-width: 0;
    display: flex;
    overflow: hidden;
  }
</style>
