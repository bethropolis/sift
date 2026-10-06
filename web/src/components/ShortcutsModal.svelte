<script lang="ts">
  interface Props {
    isOpen: boolean;
    onClose: () => void;
  }

  let { isOpen, onClose }: Props = $props();

  const SHORTCUT_LIST = [
    { key: 'j / k or ↓ / ↑', desc: 'Move tree item focus' },
    { key: 'Space', desc: 'Toggle selection (Full / Skip)' },
    { key: 'f', desc: 'Cycle mode (Full → Sigs → Skip)' },
    { key: 'b', desc: 'Toggle sidebar (collapse/expand)' },
    { key: 't', desc: 'Open Color Theme picker' },
    { key: '/', desc: 'Focus file filter' },
    { key: 's', desc: 'Run smart select' },
    { key: 'g', desc: 'Generate context document' },
    { key: 'y', desc: 'Generate and copy to clipboard' },
    { key: '?', desc: 'Open this shortcuts overlay' },
    { key: 'Esc', desc: 'Close overlay / blur inputs' },
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
        <span class="panel-hint">Press Esc to close</span>
      </div>

      <p class="panel-sub">
        Shortcuts mirror the sift terminal picker. They are suspended while typing in text fields.
      </p>

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
    max-width: 460px;
    background-color: var(--bg);
    border-radius: var(--radius-lg);
    border: 1px solid var(--border-strong);
    box-shadow: 0 8px 30px rgba(0, 0, 0, 0.12);
    padding: 20px 24px;
    display: flex;
    flex-direction: column;
    gap: 16px;
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
  .panel-sub {
    font-size: 12px;
    color: var(--ink-soft);
  }
  .shortcut-list {
    display: flex;
    flex-direction: column;
    gap: 8px;
  }
  .shortcut-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 6px 0;
    border-bottom: 1px solid var(--border);
    font-size: 12px;
  }
  .shortcut-desc {
    color: var(--ink);
  }
  .shortcut-key {
    background-color: var(--surface-alt);
    border: 1px solid var(--border-strong);
    border-radius: 4px;
    padding: 2px 6px;
    font-size: 11px;
    color: var(--ink);
  }
  .panel-foot {
    display: flex;
    justify-content: flex-end;
    padding-top: 4px;
  }
</style>
