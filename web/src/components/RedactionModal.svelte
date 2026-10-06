<script lang="ts">
  import Icon from './Icon.svelte';

  interface Props {
    isOpen: boolean;
    onConfirm: () => void;
    onCancel: () => void;
  }

  let { isOpen, onConfirm, onCancel }: Props = $props();
</script>

{#if isOpen}
  <div role="dialog" aria-modal="true" aria-label="Disable Secret Redaction" class="overlay">
    <div class="panel">
      <div class="panel-head">
        <Icon name="warning" size={20} />
        <h2 class="font-mono panel-title">Expose Unredacted Secrets?</h2>
      </div>

      <p class="body-text">
        Turning off secret redaction means API tokens, private keys, passwords, and connection strings
        found in matched code will be included raw in the generated context document.
      </p>

      <p class="fine-text">Only disable this if you are feeding the document to a local, air-gapped model.</p>

      <div class="btn-row">
        <button onclick={onCancel} class="btn btn-sm">Keep Redaction On</button>
        <button onclick={onConfirm} class="btn btn-sm danger-btn">Disable Redaction</button>
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
    max-width: 440px;
    background-color: var(--bg);
    border-radius: var(--radius-lg);
    border: 1px solid var(--border-strong);
    box-shadow: 0 8px 30px rgba(0, 0, 0, 0.12);
    padding: 20px 24px;
    display: flex;
    flex-direction: column;
    gap: 14px;
  }
  .panel-head {
    display: flex;
    align-items: center;
    gap: 10px;
    color: var(--status-danger);
  }
  .panel-title {
    font-size: 15px;
    font-weight: 600;
    color: var(--ink);
  }
  .body-text {
    font-size: 13px;
    color: var(--ink-soft);
    line-height: 1.5;
  }
  .fine-text {
    font-size: 12px;
    color: var(--ink-faint);
  }
  .btn-row {
    display: flex;
    justify-content: flex-end;
    gap: 8px;
    padding-top: 6px;
  }
  .danger-btn {
    background-color: var(--status-danger);
    color: #fff;
    border-color: var(--status-danger);
    font-weight: 600;
  }
</style>
