<script lang="ts">
  import { THEMES, getThemeById } from '../lib/themes';
  import Icon from './Icon.svelte';

  interface Props {
    isOpen: boolean;
    currentTheme: string;
    onSelectTheme: (themeId: string) => void;
    onClose: () => void;
  }

  let { isOpen, currentTheme, onSelectTheme, onClose }: Props = $props();

  let initialIdx = THEMES.findIndex((t) => t.id === currentTheme);
  let highlightedIndex = $state(initialIdx >= 0 ? initialIdx : 0);
  let listEl: HTMLDivElement | null = $state(null);
  // True only after keyboard navigation: hover and wheel scrolling must never
  // yank the list, or the two fight and the scroll feels broken.
  let followKeyboard = false;
  // Set by real mouse movement. Hover highlight requires it, so rows sliding
  // under a static cursor (wheel scroll, smooth-scroll animation) never steal
  // the highlight or fight the scroll.
  let mouseMoved = false;

  let highlightedTheme = $derived(THEMES[highlightedIndex] || THEMES[0]);
  let isLight = $derived(highlightedTheme.category === 'light');

  // Sync the highlight when the modal opens or the theme changes elsewhere
  // (settings modal, system switch). It must never react to browsing: reading
  // highlightedIndex here would resubscribe on every hover/arrow press and
  // yank the highlight back to the selected theme.
  let wasOpen = false;
  let syncedTheme: string | null = null;
  $effect(() => {
    const opened = isOpen && !wasOpen;
    const external = isOpen && syncedTheme !== null && syncedTheme !== currentTheme;
    wasOpen = isOpen;
    if (!isOpen || (!opened && !external)) return;
    syncedTheme = currentTheme;
    const idx = THEMES.findIndex((t) => t.id === currentTheme);
    if (idx >= 0) {
      highlightedIndex = idx;
      followKeyboard = true;
    }
  });

  // Keep the keyboard-highlighted item in view. Hover/wheel never scroll.
  $effect(() => {
    if (!isOpen || !listEl) return;
    const idx = highlightedIndex;
    if (!followKeyboard) return;
    followKeyboard = false;
    const items = listEl.querySelectorAll('[data-theme-item]');
    const active = items[idx] as HTMLElement | undefined;
    active?.scrollIntoView({ block: 'nearest', inline: 'nearest' });
  });

  function handleWindowKey(e: KeyboardEvent) {
    if (!isOpen) return;
    if (e.key === 'Escape' || e.key === 'q' || e.key === 't') {
      e.preventDefault();
      onClose();
    } else if (e.key === 'ArrowDown' || e.key === 'j') {
      e.preventDefault();
      followKeyboard = true;
      highlightedIndex = (highlightedIndex + 1) % THEMES.length;
    } else if (e.key === 'ArrowUp' || e.key === 'k') {
      e.preventDefault();
      followKeyboard = true;
      highlightedIndex = (highlightedIndex - 1 + THEMES.length) % THEMES.length;
    } else if (e.key === 'Enter' || e.key === ' ') {
      e.preventDefault();
      const selected = THEMES[highlightedIndex];
      if (selected) {
        onSelectTheme(selected.id);
        onClose();
      }
    }
  }

  // Keys the picker owns while open. Workspace and App also listen on window
  // (bubble phase, mounted first), so the picker intercepts these in the
  // capture phase and stops them there — preventDefault alone cannot shield
  // same-target listeners, and arrows would drive the explorer behind us.
  function ownsKey(e: KeyboardEvent): boolean {
    if (e.metaKey || e.ctrlKey || e.altKey) return false;
    return (
      e.key === 'Escape' ||
      e.key === 'ArrowDown' ||
      e.key === 'ArrowUp' ||
      e.key === 'Enter' ||
      e.key === ' ' ||
      e.key === 'q' ||
      e.key === 't' ||
      e.key === 'j' ||
      e.key === 'k'
    );
  }

  $effect(() => {
    if (!isOpen) return;
    const onKey = (e: KeyboardEvent) => {
      if (!ownsKey(e)) return;
      e.preventDefault();
      e.stopPropagation();
      handleWindowKey(e);
    };
    window.addEventListener('keydown', onKey, true);
    return () => window.removeEventListener('keydown', onKey, true);
  });

  // Focus the list while open so keys unambiguously belong to the picker;
  // restore focus on close.
  $effect(() => {
    if (!isOpen || !listEl) return;
    const prev = document.activeElement as HTMLElement | null;
    listEl.focus({ preventScroll: true });
    return () => {
      prev?.focus?.({ preventScroll: true });
    };
  });

  $effect(() => {
    if (!isOpen) return;
    const onMove = () => {
      mouseMoved = true;
    };
    window.addEventListener('mousemove', onMove, { passive: true });
    return () => window.removeEventListener('mousemove', onMove);
  });

  function handleHover(index: number) {
    if (!mouseMoved) return;
    mouseMoved = false;
    highlightedIndex = index;
  }

  function applySelected() {
    const selected = THEMES[highlightedIndex];
    if (selected) {
      onSelectTheme(selected.id);
      onClose();
    }
  }

  function applyHighlighted() {
    onSelectTheme(highlightedTheme.id);
    onClose();
  }
</script>

{#if isOpen}
  <div class="overlay" onclick={onClose}>
    <div class="panel" onclick={(e) => e.stopPropagation()}>
      <div class="panel-head">
        <div class="title-group">
          <span class="title-icon"><Icon name="palette" size={15} /></span>
          <span class="font-mono panel-title">Color Theme</span>
        </div>

        <div class="head-actions">
          <button
            onclick={() => {
              onSelectTheme('system');
              onClose();
            }}
            class="btn btn-sm btn-ghost system-btn"
            class:active={currentTheme === 'system'}
            title="Follow the OS appearance"
          >
            System
          </button>
          <button onclick={onClose} class="btn btn-sm btn-ghost close-btn" title="Close (Esc)">Esc</button>
        </div>
      </div>

      <div bind:this={listEl} class="theme-list" tabindex="-1">
        {#each THEMES as theme, index (theme.id)}
          {@const isHighlighted = index === highlightedIndex}
          {@const isSelected = theme.id === currentTheme}
          <div
            data-theme-item
            onclick={() => {
              highlightedIndex = index;
            }}
            ondblclick={applySelected}
            onmouseenter={() => handleHover(index)}
            role="option"
            aria-selected={isSelected}
            tabindex="-1"
            class="theme-row"
            class:highlighted={isHighlighted}
          >
            <div class="theme-id">
              <span class="font-mono cursor" class:visible={isHighlighted}>&gt;</span>

              <div class="swatch" style:background-color={theme.bg}>
                <span class="dot-round" style:background-color={theme.accent}></span>
                <span class="dot-sq" style:background-color={theme.surface}></span>
              </div>

              <span class="font-mono theme-name" class:highlighted={isHighlighted} class:selected={isSelected}>
                {theme.name}
              </span>

              {#if theme.category === 'light'}
                <span class="light-tag">Light</span>
              {/if}
            </div>

            {#if isSelected}
              <span class="check"><Icon name="check" size={14} /></span>
            {/if}
          </div>
        {/each}
      </div>

      <div
        class="live-preview"
        style:background-color={highlightedTheme.bg}
        style:color={isLight ? '#14171c' : '#e7e9ec'}
      >
        <div class="preview-head" style:border-color={isLight ? 'rgba(0,0,0,0.08)' : 'rgba(255,255,255,0.08)'}>
          <span class="font-mono preview-title" style:color={highlightedTheme.accent}>Theme Preview</span>
          <span class="font-mono preview-id">{highlightedTheme.name} · {highlightedTheme.id}</span>
        </div>

        <div class="font-mono preview-body">
          <div style:color={highlightedTheme.accent}>📁 internal/</div>
          <div class="tree-line">
            <span>▸ app.go <span class="tag-full">[FULL]</span></span>
            <span class="dim">1.8k</span>
          </div>
          <div class="tree-line">
            <span>config.go <span class="tag-sig">[SIG]</span></span>
            <span class="dim">0.7k</span>
          </div>

          <div
            class="code-sample"
            style:background-color={isLight ? 'rgba(0,0,0,0.04)' : 'rgba(255,255,255,0.04)'}
            style:border-color={isLight ? 'rgba(0,0,0,0.08)' : 'rgba(255,255,255,0.08)'}
          >
            <div>
              <span style:color={highlightedTheme.accent}>func </span>
              <span class="bold">greet</span>() string {'{'}
            </div>
            <div class="indent">
              <span style:color={highlightedTheme.accent}>return </span>
              <span class="str">"ready"</span>
            </div>
            <div>{'}'}</div>
          </div>

          <div class="status-line">
            <span class="ok">✓ Ready</span>
            <span class="warn">⚠ Warning</span>
          </div>
        </div>

        <div
          class="preview-foot"
          style:border-color={isLight ? 'rgba(0,0,0,0.08)' : 'rgba(255,255,255,0.08)'}
        >
          <span>↑/↓ move · Click previews · Enter applies · Esc closes</span>
          <button
            onclick={applyHighlighted}
            class="apply-btn"
            style:background-color={highlightedTheme.accent}
            style:color={isLight ? '#ffffff' : '#12141a'}
          >
            Apply Theme ↵
          </button>
        </div>
      </div>
    </div>
  </div>
{/if}

<style>
  .overlay {
    position: fixed;
    inset: 0;
    background-color: rgba(0, 0, 0, 0.65);
    backdrop-filter: blur(3px);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 100;
    padding: 16px;
    animation: fade-in 120ms ease-out;
  }
  .panel {
    width: 100%;
    max-width: 560px;
    background-color: var(--panel-bg);
    border: 1px solid var(--border-strong);
    border-radius: var(--radius-lg);
    box-shadow: 0 12px 36px rgba(0, 0, 0, 0.45);
    overflow: hidden;
    display: flex;
    flex-direction: column;
    max-height: 90vh;
    animation: pop-in 130ms ease-out;
  }
  .panel-head {
    padding: 10px 14px;
    border-bottom: 1px solid var(--border);
    background-color: var(--surface-raised);
    display: flex;
    align-items: center;
    justify-content: space-between;
  }
  .title-group {
    display: flex;
    align-items: center;
    gap: 8px;
  }
  .title-icon {
    color: var(--accent);
    display: flex;
  }
  .panel-title {
    font-size: 13px;
    font-weight: 600;
    color: var(--ink);
    letter-spacing: -0.01em;
  }
  .close-btn {
    padding: 2px 6px;
    font-size: 11px;
    color: var(--ink-faint);
  }
  .head-actions {
    display: flex;
    align-items: center;
    gap: 6px;
  }
  .system-btn {
    padding: 2px 6px;
    font-size: 11px;
    color: var(--ink-soft);
  }
  .system-btn.active {
    color: var(--accent);
    border-color: var(--accent);
  }
  .theme-list {
    max-height: 260px;
    overflow-y: auto;
    overscroll-behavior: contain;
    padding: 6px;
    display: flex;
    flex-direction: column;
    gap: 2px;
    outline: none;
  }
  /* scrollIntoView defaults to this; instant under reduced motion. */
  @media (prefers-reduced-motion: no-preference) {
    .theme-list {
      scroll-behavior: smooth;
    }
  }
  .theme-row {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 7px 10px;
    border-radius: var(--radius-sm);
    background-color: transparent;
    cursor: pointer;
    border: 1px solid transparent;
    transition: background-color 50ms ease;
  }
  .theme-row.highlighted {
    background-color: var(--surface-alt);
    border: 1px solid var(--border);
  }
  .theme-id {
    display: flex;
    align-items: center;
    gap: 8px;
    min-width: 0;
  }
  .cursor {
    width: 12px;
    color: transparent;
    font-weight: 700;
    font-size: 12px;
  }
  .cursor.visible {
    color: var(--accent);
  }
  .swatch {
    display: flex;
    align-items: center;
    gap: 2px;
    padding: 2px;
    border-radius: 3px;
    border: 1px solid var(--border);
  }
  .dot-round {
    width: 8px;
    height: 8px;
    border-radius: 50%;
    display: inline-block;
  }
  .dot-sq {
    width: 8px;
    height: 8px;
    border-radius: 2px;
    display: inline-block;
  }
  .theme-name {
    font-size: 12.5px;
    color: var(--ink-soft);
    font-weight: 400;
    white-space: nowrap;
  }
  .theme-name.highlighted {
    color: var(--ink);
  }
  .theme-name.selected {
    font-weight: 600;
  }
  .light-tag {
    font-size: 9.5px;
    font-family: var(--font-mono);
    color: var(--ink-faint);
    border: 1px solid var(--border);
    padding: 0 4px;
    border-radius: 2px;
    text-transform: uppercase;
  }
  .check {
    color: var(--accent);
    display: flex;
    align-items: center;
  }
  .live-preview {
    border-top: 1px solid var(--border);
    padding: 14px;
    display: flex;
    flex-direction: column;
    gap: 8px;
    user-select: none;
  }
  .preview-head {
    display: flex;
    align-items: baseline;
    justify-content: space-between;
    border-bottom: 1px solid;
    padding-bottom: 6px;
  }
  .preview-title {
    font-size: 11px;
    font-weight: 600;
    text-transform: uppercase;
    letter-spacing: 0.05em;
  }
  .preview-id {
    font-size: 10.5px;
    opacity: 0.6;
  }
  .preview-body {
    font-size: 11px;
    line-height: 1.5;
    display: flex;
    flex-direction: column;
    gap: 2px;
  }
  .tree-line {
    display: flex;
    justify-content: space-between;
    padding-left: 12px;
  }
  .dim {
    opacity: 0.6;
  }
  .tag-full {
    color: #86efac;
    font-weight: 600;
  }
  .tag-sig {
    color: #fbbf24;
    font-weight: 600;
  }
  .code-sample {
    margin-top: 4px;
    padding: 6px 8px;
    border-radius: 4px;
    border: 1px solid;
  }
  .bold {
    font-weight: 600;
  }
  .indent {
    padding-left: 14px;
  }
  .str {
    color: #a6e3a1;
  }
  .status-line {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-top: 4px;
    font-size: 10.5px;
  }
  .ok {
    color: #86efac;
  }
  .warn {
    color: #fbbf24;
  }
  .preview-foot {
    display: flex;
    align-items: center;
    justify-content: space-between;
    margin-top: 4px;
    padding-top: 6px;
    border-top: 1px solid;
    font-size: 10px;
    font-family: var(--font-mono);
    opacity: 0.7;
  }
  .apply-btn {
    border: none;
    border-radius: 3px;
    padding: 2px 8px;
    font-size: 10px;
    font-family: var(--font-mono);
    font-weight: 600;
    cursor: pointer;
  }
</style>
