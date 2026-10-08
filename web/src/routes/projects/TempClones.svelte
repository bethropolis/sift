<script lang="ts">
  import { clones, removeClone } from '../../lib/clones.svelte';
  import { formatRelativeTime, truncateMiddle } from '../../lib/format';
  import Icon from '../../components/Icon.svelte';
  import type { TempClone } from '../../lib/api';

  interface Props {
    onOpenProject: (root: string) => void;
  }

  let { onOpenProject }: Props = $props();

  function handleRemove(e: MouseEvent, clone: TempClone) {
    e.stopPropagation();
    void removeClone(clone);
  }
</script>

{#if clones.items.length > 0}
  <div class="strip" aria-label="Temporary clones">
    <div class="strip-head">
      <span>Temporary clones</span>
      <span class="strip-note">removed when sift stops</span>
    </div>

    {#each clones.items as clone (clone.root)}
      <div
        role="button"
        tabindex="0"
        class="row"
        onclick={() => onOpenProject(clone.root)}
        onkeydown={(e) => {
          if (e.key === 'Enter') onOpenProject(clone.root);
        }}
      >
        <span class="icon"><Icon name="clone" size={14} /></span>
        <span class="font-mono name">{clone.name}</span>
        {#if clone.branch}
          <span class="font-mono branch">{clone.branch}</span>
        {/if}
        <span class="font-mono url" title={clone.url}>{truncateMiddle(clone.url, 52)}</span>
        <span class="tabular-nums when">{formatRelativeTime(new Date(clone.started).toISOString())}</span>
        <button
          type="button"
          onclick={(e) => handleRemove(e, clone)}
          class="del-btn"
          title="Remove this clone"
          aria-label={`Remove ${clone.name}`}
        >
          <Icon name="trash" size={13} />
        </button>
      </div>
    {/each}
  </div>
{/if}

<style>
  .strip {
    display: flex;
    flex-direction: column;
    border-bottom: 1px solid var(--border);
    background-color: color-mix(in srgb, var(--accent) 4%, var(--panel-bg));
  }
  .strip-head {
    display: flex;
    align-items: baseline;
    gap: 8px;
    padding: 6px 16px;
    font-size: 10.5px;
    font-family: var(--font-mono);
    color: var(--ink-faint);
    text-transform: uppercase;
    letter-spacing: 0.04em;
    user-select: none;
  }
  .strip-note {
    text-transform: none;
    letter-spacing: 0;
    opacity: 0.8;
  }
  .row {
    display: flex;
    align-items: center;
    gap: 10px;
    padding: 8px 16px;
    cursor: pointer;
    font-size: 12px;
    transition: background-color 60ms ease;
  }
  .row:hover,
  .row:focus-visible {
    background-color: var(--surface-alt);
  }
  .icon {
    display: flex;
    color: var(--accent);
    flex-shrink: 0;
  }
  .name {
    font-size: 12.5px;
    font-weight: 600;
    color: var(--ink);
    flex-shrink: 0;
  }
  .branch {
    font-size: 11px;
    color: var(--ink-soft);
    flex-shrink: 0;
  }
  .url {
    flex: 1;
    min-width: 0;
    font-size: 11px;
    color: var(--ink-faint);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .when {
    font-size: 11px;
    color: var(--ink-faint);
    white-space: nowrap;
    flex-shrink: 0;
  }
  .del-btn {
    background: none;
    border: none;
    color: var(--ink-faint);
    cursor: pointer;
    padding: 2px 3px;
    border-radius: 3px;
    display: flex;
    align-items: center;
    flex-shrink: 0;
    transition: color 70ms ease;
  }
  .del-btn:hover {
    color: var(--status-danger);
  }
  @media (max-width: 600px) {
    .url,
    .branch {
      display: none;
    }
    .name {
      flex: 1;
      min-width: 0;
      overflow: hidden;
      text-overflow: ellipsis;
      white-space: nowrap;
    }
    .row,
    .strip-head {
      padding-left: 12px;
      padding-right: 12px;
    }
  }
</style>
