<script lang="ts">
  interface Props {
    isOpen: boolean;
    onClose: () => void;
  }

  let { isOpen, onClose }: Props = $props();

  const SHORTCUT_LIST = [
    { key: 'j / k · ↓ / ↑', desc: 'Move focus' },
    { key: 'Space', desc: 'Toggle select' },
    { key: 'm', desc: 'Cycle mode' },
    { key: 'E', desc: 'Expand all' },
    { key: 'C', desc: 'Collapse all' },
    { key: '/', desc: 'Filter files' },
    { key: 'Enter', desc: 'First filter match' },
    { key: 'Esc', desc: 'Close / blur' },
    { key: 'b', desc: 'Sidebar' },
    { key: 't', desc: 'Theme picker' },
    { key: 's', desc: 'Smart select' },
    { key: 'g', desc: 'Generate' },
    { key: 'y', desc: 'Generate + copy' },
    { key: '?', desc: 'This overlay' },
  ];

  function handleWindowKey(e: KeyboardEvent) {
    if (e.key === 'Escape' && isOpen) onClose();
  }
</script>

<svelte:window onkeydown={handleWindowKey} />

{#if isOpen}
  <div role="dialog" aria-modal="true" aria-label="Keyboard Shortcuts" onclick={onClose} class="overlay">
    <div onclick={(e) => e.stopPropagation()} class="panel">
      <div class="panel-head">
        <h2 class="font-mono panel-title">Keyboard Shortcuts</h2>
        <span class="panel-hint">Suspended while typing · Esc closes</span>
      </div>

      <div class="shortcut-list">
        {#each SHORTCUT_LIST as item (item.key)}
          <div class="shortcut-row">
            <span class="shortcut-desc">{item.desc}</span>
            <kbd class="font-mono shortcut-key">{item.key}</kbd>
          </div>
        {/each}
      </div>

      <div class="panel-foot">
        <button onclick={onClose} class="btn btn-sm">Close</button>
      </div>
    </div>
  </div>
{/if}

<style>
  .overlay {
    position: fixed;
    inset: 0;
    background-color: rgba(0, 0, 0, 0.45);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 9999;
    padding: 16px;
  }
  .panel {
    width: 100%;
    max-width: 580px;
    max-height: calc(100vh - 32px);
    overflow-y: auto;
    background-color: var(--bg);
    border-radius: var(--radius-lg);
    border: 1px solid var(--border-strong);
    box-shadow: 0 8px 30px rgba(0, 0, 0, 0.12);
    padding: 14px 20px;
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  .panel-head {
    display: flex;
    justify-content: space-between;
    align-items: center;
  }
  .panel-title {
    font-size: 15px;
    font-weight: 600;
    color: var(--ink);
  }
  .panel-hint {
    font-size: 11px;
    color: var(--ink-faint);
    font-family: var(--font-mono);
  }
  .shortcut-list {
    display: grid;
    grid-template-columns: 1fr 1fr;
    gap: 2px 24px;
  }
  .shortcut-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    gap: 8px;
    padding: 4px 0;
    border-bottom: 1px solid var(--border);
    font-size: 11.5px;
  }
  .shortcut-desc {
    color: var(--ink);
  }
  .shortcut-key {
    background-color: var(--surface-alt);
    border: 1px solid var(--border-strong);
    border-radius: 4px;
    padding: 1px 6px;
    font-size: 10.5px;
    color: var(--ink);
    white-space: nowrap;
    flex-shrink: 0;
  }
  @media (max-width: 560px) {
    .shortcut-list {
      grid-template-columns: 1fr;
    }
  }
  .panel-foot {
    display: flex;
    justify-content: flex-end;
    padding-top: 4px;
  }
</style>
