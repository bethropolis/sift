<script lang="ts">
  import { formatTokens } from '../lib/format';
  import Icon from './Icon.svelte';
  import LoaderMark from './LoaderMark.svelte';

  interface Props {
    selectedCount: number;
    usedTokens: number;
    budget: number;
    style: string;
    stylesList: string[];
    onStyleChange: (style: string) => void;
    isGenerating: boolean;
    isSmartSelecting: boolean;
    onGenerate: () => void;
    onCopy: () => void;
    onDownload: () => void;
    onOpenShortcuts: () => void;
    onSmartSelect: () => void;
    onFocusFilter: () => void;
    hasOutput: boolean;
    isCopied: boolean;
  }

  let {
    selectedCount,
    usedTokens,
    budget,
    style,
    stylesList,
    onStyleChange,
    isGenerating,
    isSmartSelecting,
    onGenerate,
    onCopy,
    onDownload,
    onOpenShortcuts,
    onSmartSelect,
    onFocusFilter,
    hasOutput,
    isCopied,
  }: Props = $props();

  let percentage = $derived(Math.round((usedTokens / budget) * 100));
  let isOver = $derived(usedTokens > budget);
  let isNear = $derived(percentage >= 90 && !isOver);
  let fillWidth = $derived(Math.min(100, percentage));
  let meterColor = $derived(isOver ? 'var(--status-danger)' : isNear ? 'var(--accent)' : 'var(--ink-soft)');
</script>

<footer class="budget-bar">
  <div class="budget-left">
    <div class="stat">
      <span class="font-mono tabular-nums stat-strong">{selectedCount}</span>
      <span class="stat-label">selected</span>
    </div>

    <span class="dot">·</span>

    <div class="stat">
      <span class="font-mono tabular-nums stat-strong" class:over={isOver}>{formatTokens(usedTokens)}</span>
      <span class="stat-label">tokens</span>
    </div>

    <span class="dot">·</span>

    <div class="stat">
      <span class="format-label">format:</span>
      <select
        value={style}
        onchange={(e) => onStyleChange(e.currentTarget.value)}
        class="select select-mono format-select"
      >
        {#each stylesList as st (st)}
          <option value={st}>{st.toUpperCase()}</option>
        {/each}
      </select>
    </div>

    <div class="meter-group">
      <div
        role="meter"
        aria-label="Token budget usage"
        aria-valuenow={usedTokens}
        aria-valuemin={0}
        aria-valuemax={budget}
        class="meter-track"
      >
        <div class="meter-fill" style:width={`${fillWidth}%`} style:background-color={meterColor}></div>
      </div>

      <span class="font-mono tabular-nums meter-text" class:over={isOver}>
        {formatTokens(usedTokens)}/{formatTokens(budget)} ({percentage}%)
      </span>
    </div>
  </div>

  <div class="budget-right">
    <button onclick={onFocusFilter} class="btn btn-sm btn-ghost hint-btn hide-on-compact" title="Filter files (/)">
      <kbd class="font-mono hint-kbd">/</kbd> filter
    </button>

    <button onclick={onSmartSelect} disabled={isSmartSelecting} class="btn btn-sm btn-ghost hint-btn hide-on-compact" title="Smart select (s)">
      <LoaderMark size={12} animated={isSmartSelecting} />
      <span>{isSmartSelecting ? 'Selecting...' : 'smart'}</span>
    </button>

    <button
      onclick={onGenerate}
      disabled={isGenerating || selectedCount === 0}
      class="btn btn-sm action-btn"
      title="Generate context document (g)"
    >
      <LoaderMark size={12} animated={isGenerating} />
      <span>{isGenerating ? 'Packing...' : 'Generate'}</span>
      <kbd class="font-mono action-kbd">g</kbd>
    </button>

    <button
      onclick={onCopy}
      disabled={isGenerating || selectedCount === 0}
      class="btn btn-sm btn-primary action-btn"
      title="Generate and copy to clipboard (y)"
    >
      {#if isCopied}
        <Icon name="check" size={11} />
      {:else}
        <Icon name="copy" size={11} />
      {/if}
      <span>{isCopied ? 'Copied' : 'Copy'}</span>
      <kbd class="font-mono action-kbd-dim">y</kbd>
    </button>

    <button
      onclick={onDownload}
      disabled={!hasOutput && selectedCount === 0}
      class="btn btn-sm btn-ghost download-btn"
      title="Download context document"
    >
      <Icon name="download" size={11} />
    </button>

    <button
      onclick={onOpenShortcuts}
      class="btn btn-sm btn-ghost shortcuts-btn"
      title="Keyboard shortcuts (?)"
    >
      ?
    </button>
  </div>
</footer>

<style>
  .budget-bar {
    height: 34px;
    min-height: 34px;
    background-color: var(--surface-raised);
    border-top: 1px solid var(--border);
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0 12px;
    font-size: 11.5px;
    user-select: none;
    gap: 12px;
    overflow-x: auto;
  }
  .budget-left {
    display: flex;
    align-items: center;
    gap: 8px;
    min-width: 0;
    flex-shrink: 0;
  }
  .stat {
    display: flex;
    align-items: center;
    gap: 4px;
  }
  .stat-strong {
    font-weight: 600;
    color: var(--ink);
  }
  .stat-strong.over {
    color: var(--status-danger);
  }
  .stat-label {
    color: var(--ink-soft);
  }
  .dot {
    color: var(--border-strong);
  }
  .format-label {
    color: var(--ink-faint);
    font-size: 11px;
  }
  .format-select {
    padding: 1px 4px;
    font-size: 10.5px;
    height: 22px;
    text-transform: uppercase;
    background-color: var(--bg);
  }
  .meter-group {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-left: 4px;
  }
  .meter-track {
    width: 76px;
    height: 4px;
    background-color: var(--surface-alt);
    border-radius: 2px;
    overflow: hidden;
    border: 1px solid var(--border);
  }
  .meter-fill {
    height: 100%;
    border-radius: 2px;
    transition: width 150ms ease;
  }
  .meter-text {
    font-size: 10.5px;
    color: var(--ink-soft);
    white-space: nowrap;
  }
  .meter-text.over {
    color: var(--status-danger);
  }
  .budget-right {
    display: flex;
    align-items: center;
    gap: 6px;
    flex-shrink: 0;
  }
  .hint-btn {
    padding: 1px 5px;
    font-size: 10.5px;
    color: var(--ink-faint);
  }
  .hint-kbd {
    color: var(--ink-soft);
  }
  .action-btn {
    height: 24px;
    padding: 0 8px;
  }
  .action-kbd {
    font-size: 9px;
    opacity: 0.6;
  }
  .action-kbd-dim {
    font-size: 9px;
    opacity: 0.7;
  }
  .download-btn {
    height: 24px;
    padding: 0 5px;
  }
  .shortcuts-btn {
    height: 24px;
    padding: 0 6px;
    font-family: var(--font-mono);
    font-size: 11px;
  }
</style>
