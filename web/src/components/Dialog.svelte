<script lang="ts">
  import type { Snippet } from 'svelte';
  import Icon from './Icon.svelte';

  interface Props {
    title: string;
    subtitle?: string;
    icon?: 'copy' | 'download' | 'folder' | 'check' | 'trash' | 'gear';
    width?: string;
    onClose: () => void;
    children: Snippet;
    footer?: Snippet;
  }

  let { title, subtitle = '', icon = 'gear', width = '560px', onClose, children, footer }: Props = $props();

  let panelEl: HTMLDivElement | null = $state(null);

  function isEditable(target: EventTarget | null): boolean {
    const el = target as HTMLElement | null;
    return !!el && (el.tagName === 'INPUT' || el.tagName === 'TEXTAREA' || el.tagName === 'SELECT' || el.isContentEditable);
  }

  // The workspace listens for shortcuts on window. While a dialog is open,
  // only its own controls should react, so stop keys in the capture phase
  // (preventDefault alone would still let the workspace handler run).
  $effect(() => {
    if (!panelEl) return;
    // Keep focus inside the dialog from the first keypress. Child dialogs may
    // focus their own input first; only take focus when it is still outside.
    if (!panelEl.contains(document.activeElement)) panelEl.focus({ preventScroll: true });
  });

  $effect(() => {
    const onKey = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        e.preventDefault();
        e.stopPropagation();
        onClose();
        return;
      }
      // App-level shortcuts (theme, settings, help) also listen on window.
      // Keep them from stacking another modal on top of this one.
      if (!isEditable(e.target) && (e.key === '?' || e.key === 't' || e.key === 'T' || e.key === ',')) {
        e.stopPropagation();
        return;
      }
      if (e.key !== 'Tab' || !panelEl) return;
      const focusable = Array.from(
        panelEl.querySelectorAll<HTMLElement>(
          'button:not(:disabled), input:not(:disabled), select:not(:disabled), textarea:not(:disabled), a[href], [tabindex]:not([tabindex="-1"])',
        ),
      ).filter((el) => el.offsetParent !== null);
      if (focusable.length === 0) return;
      const first = focusable[0];
      const last = focusable[focusable.length - 1];
      if (document.activeElement === panelEl) {
        e.preventDefault();
        (e.shiftKey ? last : first).focus();
      } else if (!e.shiftKey && document.activeElement === last) {
        e.preventDefault();
        first.focus();
      } else if (e.shiftKey && document.activeElement === first) {
        e.preventDefault();
        last.focus();
      }
    };
    window.addEventListener('keydown', onKey, true);
    return () => window.removeEventListener('keydown', onKey, true);
  });
</script>

<div class="overlay" role="presentation">
  <button class="backdrop" type="button" tabindex="-1" aria-label="Close dialog" onclick={onClose}></button>
  <div
    bind:this={panelEl}
    class="panel"
    role="dialog"
    aria-modal="true"
    aria-label={title}
    tabindex="-1"
    style:max-width={width}
  >
    <header class="panel-head">
      <div class="head-copy">
        <span class="head-icon"><Icon name={icon} size={15} /></span>
        <div>
          <h2 class="panel-title font-mono">{title}</h2>
          {#if subtitle}<p class="panel-sub">{subtitle}</p>{/if}
        </div>
      </div>
      <button onclick={onClose} class="btn btn-sm btn-ghost close-btn" aria-label="Close dialog" title="Close (Esc)">
        Esc
      </button>
    </header>

    <div class="panel-body">{@render children()}</div>

    {#if footer}<footer class="panel-foot">{@render footer()}</footer>{/if}
  </div>
</div>

<style>
  .overlay {
    position: fixed;
    inset: 0;
    background-color: rgba(0, 0, 0, 0.6);
    backdrop-filter: blur(3px);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 100;
    padding: 16px;
    animation: fade-in 120ms ease-out;
  }
  .backdrop {
    position: absolute;
    inset: 0;
    border: none;
    background: transparent;
    cursor: default;
    padding: 0;
  }
  .panel {
    position: relative;
    width: 100%;
    max-height: calc(100vh - 32px);
    max-height: calc(100dvh - 32px);
    background-color: var(--panel-bg);
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-lg);
    box-shadow: 0 12px 36px rgba(0, 0, 0, 0.45);
    display: flex;
    flex-direction: column;
    overflow: hidden;
    outline: none;
    animation: pop-in 130ms ease-out;
  }
  .panel-head {
    padding: 10px 14px;
    border-bottom: 1px solid var(--border);
    background-color: var(--surface-raised);
    display: flex;
    align-items: flex-start;
    justify-content: space-between;
    gap: 10px;
  }
  .head-copy {
    display: flex;
    align-items: flex-start;
    gap: 9px;
    min-width: 0;
  }
  .head-icon {
    color: var(--accent);
    display: flex;
    padding-top: 1px;
  }
  .panel-title {
    font-size: 13px;
    font-weight: 600;
    color: var(--ink);
    letter-spacing: -0.01em;
  }
  .panel-sub {
    margin-top: 2px;
    font-size: 11px;
    color: var(--ink-faint);
    line-height: 1.35;
  }
  .close-btn {
    padding: 2px 6px;
    font-size: 11px;
    color: var(--ink-faint);
    flex-shrink: 0;
  }
  .panel-body {
    padding: 14px;
    overflow-y: auto;
    overscroll-behavior: contain;
    min-height: 0;
  }
  .panel-foot {
    display: flex;
    align-items: center;
    justify-content: flex-end;
    gap: 8px;
    padding: 10px 14px;
    border-top: 1px solid var(--border);
    background-color: var(--surface-raised);
    flex-wrap: wrap;
  }
</style>
