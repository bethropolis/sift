<script lang="ts">
  import { tick } from 'svelte';
  import type { MenuItem } from '../lib/contextmenu';
  import { toast } from '../lib/toast.svelte';

  interface Props {
    x: number;
    y: number;
    items: MenuItem[];
    /** Element that spawned the menu; focus returns here on Esc/activate. */
    opener: HTMLElement | null;
    onClose: () => void;
  }

  let { x, y, items, opener, onClose }: Props = $props();
  let container: HTMLDivElement | null = $state(null);
  let pos = $state({ left: x, top: y });

  function focusables(): HTMLButtonElement[] {
    if (!container) return [];
    return [...container.querySelectorAll<HTMLButtonElement>('button:not(:disabled)')];
  }

  // Clamp into the viewport once measured, then park focus on the first
  // item so arrows/Enter work immediately (mouse users never see a ring:
  // programmatic focus from a mouse gesture skips :focus-visible).
  $effect(() => {
    void tick().then(() => {
      if (!container) return;
      pos = {
        left: Math.max(8, Math.min(x, window.innerWidth - container.offsetWidth - 8)),
        top: Math.max(8, Math.min(y, window.innerHeight - container.offsetHeight - 8)),
      };
      focusables()[0]?.focus();
    });
  });

  function dismiss(restoreFocus: boolean) {
    if (restoreFocus) opener?.focus();
    onClose();
  }

  // A rejected action (typically clipboard permission denied) surfaces as a
  // toast instead of a silent unhandled rejection.
  async function runItem(item: MenuItem) {
    try {
      await item.run();
    } catch {
      toast({
        id: 'menu-action',
        kind: 'error',
        message: `${item.label} didn’t work — the browser may have blocked clipboard access.`,
      });
    }
  }

  function handleWindowMouse(e: MouseEvent) {
    if (container && !container.contains(e.target as Node)) dismiss(false);
  }

  function handleWindowKey(e: KeyboardEvent) {
    if (e.key === 'Escape') dismiss(true);
  }

  function handleMenuKey(e: KeyboardEvent) {
    const btns = focusables();
    const i = btns.indexOf(document.activeElement as HTMLButtonElement);
    if (e.key === 'ArrowDown') {
      e.preventDefault();
      (btns[i + 1] ?? btns[0])?.focus();
    } else if (e.key === 'ArrowUp') {
      e.preventDefault();
      (btns[i - 1] ?? btns[btns.length - 1])?.focus();
    } else if (e.key === 'Home') {
      e.preventDefault();
      btns[0]?.focus();
    } else if (e.key === 'End') {
      e.preventDefault();
      btns[btns.length - 1]?.focus();
    } else if (e.key === 'Tab') {
      // Let focus travel on; the menu has served its purpose.
      dismiss(false);
    }
  }
</script>

<svelte:window
  onmousedown={handleWindowMouse}
  onkeydown={handleWindowKey}
  onwheel={() => dismiss(false)}
  ontouchmove={() => dismiss(false)}
  onresize={() => dismiss(false)}
/>

<div
  bind:this={container}
  role="menu"
  data-context-menu
  class="ctx-menu"
  style:left={`${pos.left}px`}
  style:top={`${pos.top}px`}
  onkeydown={handleMenuKey}
>
  {#each items as item (item.id)}
    <button
      role="menuitem"
      disabled={item.disabled}
      onclick={() => {
        dismiss(true);
        void runItem(item);
      }}
      class="ctx-item"
    >
      <span>{item.label}</span>
      {#if item.hint}
        <kbd class="font-mono ctx-hint">{item.hint}</kbd>
      {/if}
    </button>
  {/each}
</div>

<style>
  .ctx-menu {
    position: fixed;
    z-index: 10000;
    min-width: 210px;
    background-color: var(--panel-bg);
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-md);
    box-shadow: 0 12px 32px rgba(0, 0, 0, 0.25);
    padding: 4px;
    animation: pop-in 110ms ease-out;
  }
  .ctx-item {
    width: 100%;
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 24px;
    padding: 7px 10px;
    border: none;
    border-radius: var(--radius-sm);
    background: transparent;
    color: var(--ink);
    font-size: 12px;
    cursor: pointer;
    transition: background-color 70ms ease;
  }
  .ctx-item:hover:not(:disabled) {
    background-color: rgba(128, 128, 128, 0.12);
  }
  .ctx-item:disabled {
    opacity: 0.4;
    cursor: default;
  }
  .ctx-hint {
    font-size: 10.5px;
    color: var(--ink-faint);
  }
</style>
