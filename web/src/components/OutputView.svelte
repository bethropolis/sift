<script lang="ts">
  import type { PackResult } from '../lib/api';
  import { formatTokens } from '../lib/format';
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
  let showSkipped = $state(false);
  let copyTimer: ReturnType<typeof setTimeout> | null = null;
  // The wire contract is always an array, but never crash on a null again.
  let skipped = $derived(packResult?.skipped ?? []);

  function ext(): string {
    if (style === 'xml') return 'xml';
    if (style === 'markdown') return 'md';
    return 'txt';
  }

  async function handleCopy() {
    if (!packResult) return;
    try {
      await navigator.clipboard.writeText(packResult.document);
    } catch {
      const ta = document.createElement('textarea');
      ta.value = packResult.document;
      document.body.appendChild(ta);
      ta.select();
      document.execCommand('copy');
      document.body.removeChild(ta);
    }
    copied = true;
    if (copyTimer) clearTimeout(copyTimer);
    copyTimer = setTimeout(() => (copied = false), 2000);
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

    <div class="doc-scroll">
      <pre class="doc"><code>{packResult.document}</code></pre>
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
    background-color: rgba(255, 255, 255, 0.02);
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
  }
  .out-label {
    font-size: 11px;
    color: var(--ink-soft);
  }
  .out-name {
    font-size: 12px;
    font-weight: 600;
    color: var(--ink);
  }
  .head-actions {
    display: flex;
    align-items: center;
    gap: 8px;
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
  }
  .upper {
    text-transform: uppercase;
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
  }
  .skipped-panel {
    padding: 6px 14px;
    background-color: rgba(248, 113, 113, 0.05);
    border-bottom: 1px solid var(--border);
    font-size: 10.5px;
    font-family: var(--font-mono);
    max-height: 100px;
    overflow-y: auto;
  }
  .skipped-head {
    color: var(--ink-soft);
    margin-bottom: 3px;
  }
  .skipped-file {
    color: var(--ink-faint);
  }
  .doc-scroll {
    flex: 1;
    overflow: auto;
    padding: 8px 14px;
  }
  .doc {
    margin: 0;
    font-family: var(--font-mono);
    font-size: 11.5px;
    line-height: 1.6;
    color: var(--code-text);
    white-space: pre-wrap;
    word-break: break-all;
  }
</style>
