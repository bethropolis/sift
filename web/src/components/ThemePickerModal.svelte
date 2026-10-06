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

  let highlightedIndex = $state(() => {
    const idx = THEMES.findIndex((t) => t.id === currentTheme);
    return idx >= 0 ? idx : 0;
  });
  let listEl: HTMLDivElement | null = $state(null);

  let highlightedTheme = $derived(THEMES[highlightedIndex] || THEMES[0]);
  let isLight = $derived(highlightedTheme.category === 'light');

  // Sync index when the modal opens or the theme changes elsewhere.
  $effect(() => {
    if (isOpen) {
      const idx = THEMES.findIndex((t) => t.id === currentTheme);
      if (idx >= 0) highlightedIndex = idx;
    }
  });

  // Keep the highlighted item in view.
  $effect(() => {
    if (!isOpen || !listEl) return;
    const items = listEl.querySelectorAll('[data-theme-item]');
    const active = items[highlightedIndex] as HTMLElement | undefined;
    active?.scrollIntoView({ block: 'nearest' });
  });

  function handleWindowKey(e: KeyboardEvent) {
    if (!isOpen) return;
    if (e.key === 'Escape' || e.key === 'q') {
      e.preventDefault();
      onClose();
    } else if (e.key === 'ArrowDown' || e.key === 'j') {
      e.preventDefault();
      highlightedIndex = (highlightedIndex + 1) % THEMES.length;
    } else if (e.key === 'ArrowUp' || e.key === 'k') {
      e.preventDefault();
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

  function applyHighlighted() {
    onSelectTheme(highlightedTheme.id);
    onClose();
  }
</script>

<svelte:window onkeydown={handleWindowKey} />

{#if isOpen}
  <div class="overlay" onclick={onClose}>
    <div class="panel" onclick={(e) => e.stopPropagation()}>
      <div class="panel-head">
        <div class="title-group">
          <span class="title-icon"><Icon name="palette" size={15} /></span>
          <span class="font-mono panel-title">Color Theme</span>
        </div>

        <button onclick={onClose} class="btn btn-sm btn-ghost close-btn" title="Close (Esc)">Esc</button>
      </div>

      <div bind:this={listEl} class="theme-list">
        {#each THEMES as theme, index (theme.id)}
          {@const isHighlighted = index === highlightedIndex}
          {@const isSelected = theme.id === currentTheme}
          <div
            data-theme-item
            onclick={() => {
              highlightedIndex = index;
              onSelectTheme(theme.id);
              onClose();
            }}
            onmouseenter={() => (highlightedIndex = index)}
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
          <span>↑/↓ move · Enter/Space apply · Esc/q close</span>
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
  .theme-list {
    max-height: 260px;
    overflow-y: auto;
    padding: 6px;
    display: flex;
    flex-direction: column;
    gap: 2px;
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
