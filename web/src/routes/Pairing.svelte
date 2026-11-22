<script lang="ts">
  import LoaderMark from '../components/LoaderMark.svelte';

  interface Props {
    error: string | null;
  }

  let { error }: Props = $props();
</script>

<div class="pair-page">
  <div class="pair-card">
    <div class="pair-brand">
      <span class="pair-dot" class:error={!!error}></span>
      <span class="font-mono pair-title">sift pair</span>
    </div>
    {#if error}
      <p role="alert" class="pair-error">{error}</p>
      <p class="pair-hint">
        The QR token is single-use. Press Enter in the terminal running
        <code class="font-mono">sift serve --pair</code> and scan the new code.
      </p>
    {:else}
      <div class="pair-working">
        <LoaderMark size={16} />
        <span class="font-mono">Pairing this phone…</span>
      </div>
      <p class="pair-hint">The code is being exchanged for a read-only session.</p>
    {/if}
  </div>
</div>

<style>
  .pair-page {
    flex: 1;
    display: flex;
    align-items: center;
    justify-content: center;
    padding: 24px;
    background-color: var(--bg);
  }
  .pair-card {
    width: min(420px, 100%);
    padding: 24px;
    background-color: var(--surface-raised);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    display: flex;
    flex-direction: column;
    gap: 14px;
  }
  .pair-brand {
    display: flex;
    align-items: center;
    gap: 9px;
  }
  .pair-dot {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    background-color: var(--accent);
  }
  .pair-dot.error {
    background-color: var(--danger, #d9534f);
  }
  .pair-title {
    font-size: 14px;
    font-weight: 600;
    color: var(--ink);
  }
  .pair-working {
    display: flex;
    align-items: center;
    gap: 8px;
    color: var(--ink-soft);
    font-size: 12px;
  }
  .pair-error {
    margin: 0;
    color: var(--ink);
    font-size: 13px;
    line-height: 1.5;
  }
  .pair-hint {
    margin: 0;
    color: var(--ink-soft);
    font-size: 12px;
    line-height: 1.5;
  }
</style>
