<script lang="ts">
  import { tick } from 'svelte';
  import type { MenuItem } from '../lib/contextmenu';

  interface Props {
    x: number;
    y: number;
    items: MenuItem[];
    onClose: () => void;
  }

  let { x, y, items, onClose }: Props = $props();
  let container: HTMLDivElement | null = $state(null);
  let pos = $state({ left: x, top: y });

  // Clamp into the viewport once measured; keyboard-opened menus arrive at
  // 0,0 and land top-left, which is also handled here.
  $effect(() => {
    void tick().then(() => {
      if (!container) return;
      pos = {
        left: Math.max(8, Math.min(x, window.innerWidth - container.offsetWidth - 8)),
        top: Math.max(8, Math.min(y, window.innerHeight - container.offsetHeight - 8)),
      };
    });
  });

  function handleWindowMouse(e: MouseEvent) {
    if (container && !container.contains(e.target as Node)) onClose();
  }

  function handleWindowKey(e: KeyboardEvent) {
    if (e.key === 'Escape') onClose();
  }
</script>

<svelte:window onmousedown={handleWindowMouse} onkeydown={handleWindowKey} />

<div
  bind:this={container}
  role="menu"
  data-context-menu
  class="ctx-menu"
  style:left={`${pos.left}px`}
  style:top={`${pos.top}px`}
>
  {#each items as item (item.id)}
    <button
      role="menuitem"
      disabled={item.disabled}
      onclick={() => {
        onClose();
        void item.run();
      }}
      class="ctx-item"
    >
      <span>{item.label}</span>
      <kbd class="font-mono ctx-hint">{item.hint}</kbd>
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
