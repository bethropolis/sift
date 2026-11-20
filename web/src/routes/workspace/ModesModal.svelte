<script lang="ts">
  import { onDestroy } from 'svelte';
  import { api, type SelectionMode } from '../../lib/api';
  import { formatTokens } from '../../lib/format';
  import { toast } from '../../lib/toast.svelte';
  import Dialog from '../../components/Dialog.svelte';
  import Icon from '../../components/Icon.svelte';

  interface Props {
    root: string;
    selections: Record<string, SelectionMode>;
    selectedCount: number;
    usedTokens: number;
    onClose: () => void;
  }

  let { root, selections, selectedCount, usedTokens, onClose }: Props = $props();

  let toml = $state('');
  let loading = $state(true);
  let error = $state<string | null>(null);
  let copied = $state(false);
  let copyError = $state<string | null>(null);
  let copyTimer: ReturnType<typeof setTimeout> | null = null;

  let counts = $derived.by(() => {
    let full = 0;
    let sigs = 0;
    let skip = 0;
    for (const mode of Object.values(selections)) {
      if (mode === 'full') full++;
      else if (mode === 'sigs') sigs++;
      else if (mode === 'skip') skip++;
    }
    return { full, sigs, skip };
  });

  // Full is the default and deliberately omitted from [modes]; the exported
  // TOML only records the deviations.
  let patternCount = $derived((toml.match(/^\s+"/gm) ?? []).length);

  async function load() {
    loading = true;
    error = null;
    try {
      const result = await api.exportModes({ root, selections });
      toml = result.toml;
    } catch (err) {
      error = err instanceof Error ? err.message : 'Could not export modes';
    } finally {
      loading = false;
    }
  }

  $effect(() => {
    // Track the selection snapshot; the dialog is only mounted while open.
    void root;
    void selections;
    void load();
  });

  async function handleCopy() {
    if (!toml) return;
    copyError = null;
    try {
      await navigator.clipboard.writeText(toml);
      copied = true;
      if (copyTimer) clearTimeout(copyTimer);
      copyTimer = setTimeout(() => (copied = false), 2000);
      toast({ id: 'export-modes', kind: 'success', message: 'Copied [modes] TOML', duration: 1800 });
    } catch {
      copyError = 'Clipboard access was denied — select the TOML and copy it manually.';
    }
  }

  onDestroy(() => {
    if (copyTimer) clearTimeout(copyTimer);
  });
</script>

<Dialog
  title="Export Modes"
  subtitle="Copy the current file modes as a [modes] block for .sift.toml."
  icon="copy"
  width="610px"
  {onClose}
>
  <div class="summary">
    <div class="stat">
      <span class="stat-value font-mono">{selectedCount}</span>
      <span class="stat-label">files included</span>
    </div>
    <div class="stat">
      <span class="stat-value font-mono">{formatTokens(usedTokens)}</span>
      <span class="stat-label">tokens selected</span>
    </div>
    <div class="stat">
      <span class="stat-value font-mono">{counts.sigs}</span>
      <span class="stat-label">signatures</span>
    </div>
    <div class="stat">
      <span class="stat-value font-mono">{counts.skip}</span>
      <span class="stat-label">excluded</span>
    </div>
  </div>

  <p class="note">
    <span class="note-key font-mono">full</span> is the default and is not written. Uniform folders collapse
    to <span class="font-mono">dir/**</span>; mixed folders keep exact paths.
  </p>

  <div class="toml-card">
    <div class="toml-head">
      <span class="font-mono toml-label">[modes]</span>
      <span class="font-mono toml-meta">
        {#if loading}reading selection…{:else if toml}{patternCount} pattern{patternCount === 1 ? '' : 's'}{:else}empty{/if}
      </span>
    </div>

    {#if loading}
      <div class="toml-skeleton" aria-label="Building TOML">
        <span></span><span></span><span></span><span></span>
      </div>
    {:else if error}
      <div class="state error">
        <Icon name="warning" size={15} />
        <div>
          <p>{error}</p>
          <button class="btn btn-sm" onclick={() => void load()}>Retry</button>
        </div>
      </div>
    {:else if toml}
      <pre class="toml-code font-mono">{toml}</pre>
    {:else}
      <div class="state">
        <Icon name="check" size={15} />
        <div>
          <p>Nothing to export — every file is already the default full mode.</p>
          <p class="state-sub">Change a file to signatures or excluded, then reopen Modes.</p>
        </div>
      </div>
    {/if}
  </div>

  {#if copyError}<p class="copy-error">{copyError}</p>{/if}

  {#snippet footer()}
    <span class="foot-hint font-mono">Paste under the [sift] section of .sift.toml</span>
    <button class="btn" onclick={onClose}>Close</button>
    <button class="btn btn-primary copy-btn" onclick={() => void handleCopy()} disabled={loading || !toml}>
      <Icon name={copied ? 'check' : 'copy'} size={12} />
      {copied ? 'Copied' : 'Copy TOML'}
    </button>
  {/snippet}
</Dialog>

<style>
  .summary {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 8px;
  }
  .stat {
    border: 1px solid var(--border);
    background-color: var(--surface-raised);
    border-radius: var(--radius-sm);
    padding: 8px 10px;
    display: flex;
    flex-direction: column;
    gap: 1px;
  }
  .stat-value {
    font-size: 15px;
    color: var(--ink);
    font-weight: 600;
    line-height: 1.2;
  }
  .stat-label {
    font-size: 10px;
    color: var(--ink-faint);
    white-space: nowrap;
  }
  .note {
    margin: 10px 0;
    font-size: 11px;
    color: var(--ink-soft);
    line-height: 1.5;
  }
  .note-key {
    color: var(--accent);
    font-weight: 600;
  }
  .toml-card {
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    overflow: hidden;
    background-color: var(--code-bg);
  }
  .toml-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 6px 10px;
    background-color: var(--surface-alt);
    border-bottom: 1px solid var(--border);
  }
  .toml-label {
    font-size: 10.5px;
    color: var(--ink);
    font-weight: 600;
  }
  .toml-meta {
    font-size: 10px;
    color: var(--ink-faint);
  }
  .toml-code {
    margin: 0;
    padding: 10px 12px;
    font-size: 11.5px;
    line-height: 1.6;
    color: var(--code-text);
    background-color: var(--code-bg);
    max-height: 260px;
    overflow: auto;
    white-space: pre;
    tab-size: 2;
  }
  .toml-skeleton {
    display: flex;
    flex-direction: column;
    gap: 7px;
    padding: 12px;
  }
  .toml-skeleton span {
    height: 9px;
    border-radius: 3px;
    background-color: var(--border);
    animation: pulse 1.2s ease-in-out infinite;
  }
  .toml-skeleton span:nth-child(1) { width: 28%; }
  .toml-skeleton span:nth-child(2) { width: 62%; }
  .toml-skeleton span:nth-child(3) { width: 48%; }
  .toml-skeleton span:nth-child(4) { width: 56%; }
  @keyframes pulse {
    50% { opacity: 0.45; }
  }
  .state {
    display: flex;
    align-items: flex-start;
    gap: 8px;
    padding: 14px;
    color: var(--ink-soft);
    font-size: 11.5px;
  }
  .state.error {
    color: var(--status-danger);
  }
  .state p {
    margin-bottom: 6px;
  }
  .state-sub {
    color: var(--ink-faint);
    font-size: 10.5px;
  }
  .copy-error {
    margin-top: 8px;
    font-size: 11px;
    color: var(--status-danger);
  }
  .foot-hint {
    margin-right: auto;
    font-size: 10px;
    color: var(--ink-faint);
  }
  .copy-btn {
    min-width: 108px;
  }
  @media (max-width: 520px) {
    .summary {
      grid-template-columns: repeat(2, 1fr);
    }
    .foot-hint {
      width: 100%;
      margin-bottom: 2px;
    }
  }
</style>
