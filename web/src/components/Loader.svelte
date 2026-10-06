<script lang="ts">
  import LoaderMark from './LoaderMark.svelte';

  interface Props {
    label?: string;
    size?: number;
    /** Inline mark for buttons: no label, no fill height. */
    inline?: boolean;
    /** Forwarded to the mark: the embedding body decides animated or static. */
    animated?: boolean;
  }

  let { label = 'Loading...', size = 40, inline = false, animated = true }: Props = $props();
</script>

{#if inline}
  <LoaderMark size={size} {animated} />
{:else}
  <div class="loader" role="status" aria-label={label}>
    <LoaderMark size={size} {animated} />
    <span class="loader-label">{label}</span>
  </div>
{/if}

<style>
  .loader {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    gap: 12px;
    min-height: 200px;
    padding: 24px;
    color: var(--code-dim);
    font-family: var(--font-mono);
    font-size: 12px;
  }
  .loader-label {
    opacity: 0.85;
  }
</style>
