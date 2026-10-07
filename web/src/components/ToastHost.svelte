<script lang="ts">
  import { toast, toasts } from '../lib/toast.svelte';
  import Icon from './Icon.svelte';
</script>

<!-- One host for the whole app. Individual errors use role="alert" so they are
     announced assertively; the container covers everything else politely. -->
<div class="toast-host" aria-live="polite">
  {#each toasts.items as item (item.id)}
    <div
      class="toast {item.kind}"
      role={item.kind === 'error' ? 'alert' : 'status'}
      onmouseenter={() => toast.pause(item.id)}
      onmouseleave={() => toast.resume(item.id)}
    >
      <span class="toast-icon">
        {#if item.kind === 'success'}
          <Icon name="check" size={13} />
        {:else if item.kind === 'error'}
          <Icon name="warning" size={13} />
        {:else}
          <span class="dot"></span>
        {/if}
      </span>

      <span class="toast-msg">{item.message}</span>

      {#if item.action}
        {@const action = item.action}
        <button
          class="toast-action"
          onclick={() => {
            toast.dismiss(item.id);
            action.run();
          }}
        >
          {action.label}
        </button>
      {/if}

      <button class="toast-close" onclick={() => toast.dismiss(item.id)} aria-label="Dismiss notification">
        ✕
      </button>
    </div>
  {/each}
</div>

<style>
  .toast-host {
    position: fixed;
    right: 16px;
    /* Clears the 34px footer bar so Copy/Download stay reachable. */
    bottom: 48px;
    z-index: 10001;
    display: flex;
    flex-direction: column;
    align-items: flex-end;
    gap: 8px;
    /* The host never blocks clicks; only the toasts themselves do. */
    pointer-events: none;
    max-width: calc(100vw - 32px);
  }
  .toast {
    pointer-events: auto;
    display: flex;
    align-items: center;
    gap: 9px;
    min-width: 220px;
    max-width: 420px;
    padding: 8px 8px 8px 12px;
    background-color: var(--panel-bg);
    color: var(--ink);
    border: 1px solid var(--border-strong);
    border-left: 3px solid var(--accent);
    border-radius: var(--radius-md);
    box-shadow: 0 10px 28px rgba(0, 0, 0, 0.22);
    font-size: 12px;
    animation: pop-in 130ms ease-out;
  }
  .toast.success {
    border-left-color: var(--status-ok);
  }
  .toast.error {
    border-left-color: var(--status-danger);
  }
  .toast-icon {
    display: inline-flex;
    flex-shrink: 0;
    color: var(--accent);
  }
  .toast.success .toast-icon {
    color: var(--status-ok);
  }
  .toast.error .toast-icon {
    color: var(--status-danger);
  }
  .dot {
    width: 7px;
    height: 7px;
    border-radius: 50%;
    background-color: currentColor;
  }
  .toast-msg {
    flex: 1;
    min-width: 0;
    overflow-wrap: anywhere;
    line-height: 1.35;
  }
  .toast-action {
    border: 1px solid var(--border-strong);
    background: transparent;
    color: var(--accent);
    font-size: 11px;
    font-family: var(--font-mono);
    padding: 2px 8px;
    border-radius: var(--radius-sm);
    cursor: pointer;
    flex-shrink: 0;
  }
  .toast-action:hover {
    background-color: var(--surface-alt);
  }
  .toast-close {
    border: none;
    background: transparent;
    color: var(--ink-faint);
    font-size: 11px;
    line-height: 1;
    padding: 4px 6px;
    border-radius: var(--radius-sm);
    cursor: pointer;
    flex-shrink: 0;
  }
  .toast-close:hover {
    color: var(--ink);
    background-color: var(--surface-alt);
  }
  @media (max-width: 560px) {
    .toast-host {
      left: 12px;
      right: 12px;
      align-items: stretch;
    }
    .toast {
      max-width: none;
    }
  }
</style>
