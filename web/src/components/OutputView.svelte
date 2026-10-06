<script lang="ts">
  import type { PackResult } from '../lib/api';
  import { formatTokens } from '../lib/format';
  import {
    DOC_ROW_HEIGHT,
    lineByteStarts,
    sectionLineRange,
    windowLines,
  } from '../lib/output';
  import Icon from './Icon.svelte';
  import Loader from './Loader.svelte';

  interface Props {
    packResult: PackResult | null;
    projectName: string;
    style: string;
    isGenerating: boolean;
    onRegenerate: () => void;
  }

  let { packResult, projectName, style, isGenerating, onRegenerate }: Props = $props();
  let copied = $state(false);
  let copiedSection = $state<string | null>(null);
  let copyError = $state<string | null>(null);
  let showSkipped = $state(false);
  let outlineOpen = $state(true);
  let copyTimer: ReturnType<typeof setTimeout> | null = null;
  let containerEl: HTMLDivElement | null = $state(null);
  let scrollTop = $state(0);
  let viewportHeight = $state(500);
  // The wire contract is always an array, but never crash on a null again.
  let skipped = $derived(packResult?.skipped ?? []);
  let sections = $derived(packResult?.sections ?? []);

  // Past ~1M chars the legacy textarea copy path hangs the tab: clipboard
  // API or Save only.
  const COPY_FALLBACK_LIMIT = 1_000_000;

  function ext(): string {
    if (style === 'xml') return 'xml';
    if (style === 'markdown') return 'md';
    return 'txt';
  }

  function updateViewport() {
    viewportHeight = containerEl?.clientHeight || 500;
  }

  $effect(() => {
    updateViewport();
    window.addEventListener('resize', updateViewport);
    return () => window.removeEventListener('resize', updateViewport);
  });

  // Fresh document resets transient copy state.
  $effect(() => {
    void (packResult?.document ?? null);
    copied = false;
    copiedSection = null;
    copyError = null;
  });

  // Document lines + UTF-8 byte starts (server sections are byte offsets).
  let doc = $derived(packResult?.document ?? '');
  let lines = $derived(doc.split('\n'));
  let starts = $derived(lineByteStarts(lines));
  let win = $derived(windowLines(lines.length, scrollTop, viewportHeight));
  let visibleLines = $derived(lines.slice(win.startIndex, win.endIndex));

  // Sections resolved to line ranges once per document.
  let sectionLines = $derived(
    sections.map((s) => ({
      ...s,
      ...sectionLineRange(starts, lines.length, s.start, s.end),
    })),
  );

  function jumpToSection(startLine: number) {
    if (!containerEl) return;
    containerEl.scrollTop = Math.max(0, startLine * DOC_ROW_HEIGHT - 40);
  }

  function sectionText(startLine: number, endLine: number): string {
    return lines.slice(startLine, endLine + 1).join('\n');
  }

  async function copyText(text: string): Promise<boolean> {
    try {
      await navigator.clipboard.writeText(text);
      return true;
    } catch {
      return false;
    }
  }

  function legacyCopy(text: string): boolean {
    try {
      const ta = document.createElement('textarea');
      ta.value = text;
      document.body.appendChild(ta);
      ta.select();
      const ok = document.execCommand('copy');
      document.body.removeChild(ta);
      return ok;
    } catch {
      return false;
    }
  }

  function flashCopied(section: string | null) {
    if (section === null) copied = true;
    else copiedSection = section;
    copyError = null;
    if (copyTimer) clearTimeout(copyTimer);
    copyTimer = setTimeout(() => {
      copied = false;
      copiedSection = null;
    }, 2000);
  }

  async function handleCopy() {
    if (!packResult) return;
    if (await copyText(packResult.document)) {
      flashCopied(null);
      return;
    }
    if (packResult.document.length > COPY_FALLBACK_LIMIT) {
      copyError = 'Too large for the clipboard — use Save';
      return;
    }
    if (legacyCopy(packResult.document)) flashCopied(null);
    else copyError = 'Copy failed — use Save';
  }

  async function handleSectionCopy(path: string, startLine: number, endLine: number) {
    if (await copyText(sectionText(startLine, endLine))) {
      flashCopied(path);
      return;
    }
    copyError = `Copy failed for ${path}`;
  }

  function handleDownload() {
    if (!packResult) return;
    // Sanitize the project name so the filename cannot carry path separators.
    const safeName = (projectName || 'project').replace(/[^a-zA-Z0-9._-]+/g, '_');
    const blob = new Blob([packResult.document], { type: 'text/plain;charset=utf-8' });
    const url = URL.createObjectURL(blob);
    const link = document.createElement('a');
    link.href = url;
    link.download = `${safeName}-context.${ext()}`;
    document.body.appendChild(link);
    link.click();
    document.body.removeChild(link);
    URL.revokeObjectURL(url);
  }
</script>

{#if isGenerating}
  <Loader label="Packing context document & masking secrets..." />
{:else if !packResult || !packResult.document}
  <div class="center mono-dim padded">
    <span>No context document generated yet</span>
    <button onclick={onRegenerate} class="generate-cta">Generate Context (g)</button>
  </div>
{:else}
  <div class="output">
    <div class="output-head">
      <div class="head-row">
        <div class="file-id">
          <Icon name="file" size={13} />
          <span class="out-label">Output:</span>
          <span class="out-name">{projectName}-context.{ext()}</span>
        </div>

        <div class="head-actions">
          {#if sections.length > 0}
            <button
              onclick={() => (outlineOpen = !outlineOpen)}
              class="mini-btn dim"
              title={outlineOpen ? 'Hide file outline' : 'Show file outline'}
            >
              <Icon name="sidebar" size={12} />
              <span>Files</span>
            </button>
          {/if}
          <button onclick={handleCopy} class="mini-btn" class:ok={copied}>
            {#if copied}
              <Icon name="check" size={12} />
            {:else}
              <Icon name="copy" size={12} />
            {/if}
            <span>{copied ? 'Copied' : 'Copy'}</span>
          </button>

          <button onclick={handleDownload} class="mini-btn dim">
            <Icon name="download" size={12} />
            <span>Save</span>
          </button>
        </div>
      </div>

      <div class="stats-line">
        <span class="upper">{style}</span>
        <span>·</span>
        <span class="tabular-nums">{packResult.fileCount} files</span>
        <span>·</span>
        <span class="tabular-nums">{formatTokens(packResult.tokens)} tokens</span>
        <span>·</span>
        <span class="tabular-nums">{lines.length.toLocaleString()} lines</span>
        <span>·</span>
        <span class="tabular-nums">{packResult.redactions} redacted</span>

        {#if skipped.length > 0}
          <span>·</span>
          <button onclick={() => (showSkipped = !showSkipped)} class="skipped-toggle">
            {#if showSkipped}
              <Icon name="chevron-down" size={11} />
            {:else}
              <Icon name="chevron-right" size={11} />
            {/if}
            <span>{skipped.length} skipped</span>
          </button>
        {/if}
        {#if copyError}
          <span>·</span>
          <span class="copy-error">{copyError}</span>
        {/if}
      </div>
    </div>

    <div class="divider"></div>

    {#if showSkipped && skipped.length > 0}
      <div class="skipped-panel">
        <div class="skipped-head">Skipped due to budget limit:</div>
        {#each skipped as f (f)}
          <div class="skipped-file">- {f}</div>
        {/each}
      </div>
    {/if}

    <div class="output-body">
      {#if sections.length > 0 && outlineOpen}
        <div class="outline">
          {#each sectionLines as s (s.path)}
            <div class="outline-row">
              <button
                onclick={() => jumpToSection(s.startLine)}
                class="outline-jump"
                title={`Jump to ${s.path}`}
              >
                <span class="font-mono outline-path">{s.path}</span>
                <span class="font-mono tabular-nums outline-tokens">{formatTokens(s.tokens)}</span>
              </button>
              <button
                onclick={() => void handleSectionCopy(s.path, s.startLine, s.endLine)}
                class="outline-copy"
                title={`Copy ${s.path} only`}
              >
                {#if copiedSection === s.path}
                  <Icon name="check" size={11} />
                {:else}
                  <Icon name="copy" size={11} />
                {/if}
              </button>
            </div>
          {/each}
        </div>
      {/if}

      <div
        bind:this={containerEl}
        onscroll={(e) => (scrollTop = e.currentTarget.scrollTop)}
        class="doc-scroll"
      >
        <div class="doc-spacer" style:height={`${win.totalHeight}px`}>
          <div class="doc-window" style:transform={`translateY(${win.offsetY}px)`}>
            {#each visibleLines as line, i (win.startIndex + i)}<div class="doc-line">{line}</div
              >{/each}
          </div>
        </div>
      </div>
    </div>
  </div>
{/if}

<style>
  .center {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    background-color: var(--panel-bg);
    gap: 8px;
  }
  .mono-dim {
    color: var(--ink-faint);
    font-family: var(--font-mono);
    font-size: 11px;
  }
  .padded {
    padding: 24px;
    text-align: center;
  }
  .generate-cta {
    background: var(--surface-raised);
    border: 1px solid var(--border-strong);
    color: var(--accent);
    font-family: var(--font-mono);
    font-size: 11px;
    padding: 4px 10px;
    border-radius: 4px;
    cursor: pointer;
  }
  .output {
    flex: 1;
    min-height: 0;
    min-width: 0;
    display: flex;
    flex-direction: column;
    background-color: var(--panel-bg);
    overflow: hidden;
    color: var(--code-text);
    border-radius: var(--radius-md);
  }
  .output-head {
    padding: 8px 14px 6px;
    font-family: var(--font-mono);
    user-select: none;
    background-color: rgba(128, 128, 128, 0.04);
    flex-shrink: 0;
  }
  .head-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
  }
  .file-id {
    display: flex;
    align-items: center;
    gap: 6px;
    min-width: 0;
    overflow: hidden;
  }
  .out-label {
    font-size: 11px;
    color: var(--ink-soft);
    flex-shrink: 0;
  }
  .out-name {
    font-size: 12px;
    font-weight: 600;
    color: var(--ink);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .head-actions {
    display: flex;
    align-items: center;
    gap: 8px;
    flex-shrink: 0;
  }
  .mini-btn {
    background: none;
    border: none;
    color: var(--ink);
    font-family: var(--font-mono);
    font-size: 11px;
    cursor: pointer;
    display: flex;
    align-items: center;
    gap: 4px;
    padding: 2px 4px;
  }
  .mini-btn.ok {
    color: var(--status-ok);
  }
  .mini-btn.dim {
    color: var(--ink-soft);
  }
  .stats-line {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 11px;
    color: var(--ink-faint);
    margin-top: 2px;
    flex-wrap: wrap;
  }
  .upper {
    text-transform: uppercase;
  }
  .copy-error {
    color: var(--status-warn);
  }
  .skipped-toggle {
    background: none;
    border: none;
    cursor: pointer;
    display: flex;
    align-items: center;
    gap: 3px;
    font-size: 11px;
    font-family: var(--font-mono);
    padding: 0;
  }
  .divider {
    height: 1px;
    background-color: var(--border);
    margin: 0 12px 4px;
    flex-shrink: 0;
  }
  .skipped-panel {
    padding: 6px 14px;
    background-color: rgba(248, 113, 113, 0.05);
    border-bottom: 1px solid var(--border);
    font-size: 10.5px;
    font-family: var(--font-mono);
    max-height: 100px;
    overflow-y: auto;
    flex-shrink: 0;
  }
  .skipped-head {
    color: var(--ink-soft);
    margin-bottom: 3px;
  }
  .skipped-file {
    color: var(--ink-faint);
  }
  .output-body {
    flex: 1;
    min-height: 0;
    display: flex;
    overflow: hidden;
  }
  .outline {
    width: 230px;
    min-width: 180px;
    max-width: 320px;
    flex-shrink: 0;
    min-height: 0;
    overflow-y: auto;
    overflow-x: hidden;
    scrollbar-width: thin;
    scrollbar-color: var(--border-strong) transparent;
    border-right: 1px solid var(--border);
    padding: 4px 0;
    overscroll-behavior: contain;
  }
  .outline-row {
    display: flex;
    align-items: center;
    gap: 2px;
    padding: 0 4px 0 8px;
  }
  .outline-row:hover {
    background-color: rgba(128, 128, 128, 0.09);
  }
  .outline-jump {
    flex: 1;
    min-width: 0;
    background: none;
    border: none;
    cursor: pointer;
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    gap: 6px;
    padding: 3px 2px;
    text-align: left;
  }
  .outline-path {
    font-size: 10.5px;
    color: var(--ink-soft);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .outline-tokens {
    font-size: 9.5px;
    color: var(--ink-faint);
    flex-shrink: 0;
  }
  .outline-copy {
    background: none;
    border: none;
    cursor: pointer;
    color: var(--ink-faint);
    padding: 3px 4px;
    display: flex;
    flex-shrink: 0;
  }
  .outline-copy:hover {
    color: var(--ink);
  }
  /* Virtualized document: fixed rows, single scroll container on both axes
     (same pattern as the tree and the code preview). Long lines scroll
     horizontally instead of wrapping, keeping row math exact at any size. */
  .doc-scroll {
    flex: 1;
    min-width: 0;
    overflow: auto;
    overscroll-behavior: contain;
    scrollbar-gutter: stable;
    scrollbar-width: thin;
    padding: 10px 14px 24px;
  }
  .doc-spacer {
    width: max-content;
    min-width: 100%;
  }
  .doc-window {
    width: max-content;
    min-width: 100%;
    font-family: var(--font-mono);
    font-size: 12px;
    line-height: 18px;
    color: var(--code-text);
  }
  .doc-line {
    height: 18px;
    white-space: pre;
    padding-right: 16px;
  }
</style>
