<script lang="ts">
  import { api, type SettingsData, type ApiMeta } from '../lib/api';
  import { formatTokens } from '../lib/format';
  import { THEMES } from '../lib/themes';
  import Icon from './Icon.svelte';

  interface Props {
    meta: ApiMeta | null;
    currentTheme: string;
    onThemeChange: (theme: string) => void;
    onClose: () => void;
  }

  let { meta, currentTheme, onThemeChange, onClose }: Props = $props();

  let settings = $state<SettingsData>({
    defaultStyle: 'xml',
    defaultBudget: 64000,
    theme: currentTheme,
    showHidden: false,
    fileSort: 'name',
  });
  let isSaving = $state(false);
  let saveSuccess = $state(false);
  let saveTimer: ReturnType<typeof setTimeout> | null = null;

  $effect(() => {
    api.getSettings().then((data) => {
      settings = data;
    });
  });

  function handleWindowKey(e: KeyboardEvent) {
    if (e.key === 'Escape') onClose();
  }

  async function handleSave(e: SubmitEvent) {
    e.preventDefault();
    isSaving = true;
    try {
      await api.saveSettings(settings);
      isSaving = false;
      saveSuccess = true;
      if (saveTimer) clearTimeout(saveTimer);
      saveTimer = setTimeout(() => (saveSuccess = false), 2000);
    } catch (err) {
      console.error(err);
      isSaving = false;
    }
  }

  let budgetPresets = [32000, 64000, 120000, 200000];
  let stylesList = $derived(meta?.styles || ['xml', 'markdown', 'plain']);
</script>

<svelte:window onkeydown={handleWindowKey} />

<div role="dialog" aria-modal="true" aria-label="Settings" onclick={onClose} class="overlay">
  <div onclick={(e) => e.stopPropagation()} class="panel">
    <div class="panel-head">
      <div>
        <h2 class="font-mono panel-title">Settings</h2>
        <p class="panel-sub">Defaults for fresh workspace sessions. Esc closes.</p>
      </div>
      <button onclick={onClose} class="btn btn-sm btn-ghost close-btn" aria-label="Close settings">
        <span aria-hidden="true" class="close-x">✕</span>
      </button>
    </div>

    <form onsubmit={handleSave} class="settings-form">
      <section class="card">
        <label class="card-label" for="modal-style-group">Default Output Document Style</label>
        <p class="card-desc">Rendered format when generating context documents for language models.</p>
        <div id="modal-style-group" class="btn-row">
          {#each stylesList as st (st)}
            <button
              type="button"
              onclick={() => (settings = { ...settings, defaultStyle: st })}
              class="btn btn-sm"
              class:btn-primary={settings.defaultStyle === st}
              class:style-btn={true}
            >
              {st}
            </button>
          {/each}
        </div>
      </section>

      <section class="card">
        <label class="card-label" for="modal-budget-input">Default Token Budget</label>
        <p class="card-desc">
          Initial target budget in tokens for fresh workspace sessions. Projects with
          <code class="font-mono">budget</code> in their
          <code class="font-mono">.sift.toml</code> use that instead.
        </p>

        <div class="budget-row">
          {#each budgetPresets as b (b)}
            <button
              type="button"
              onclick={() => (settings = { ...settings, defaultBudget: b })}
              class="btn btn-sm"
              class:btn-primary={settings.defaultBudget === b}
              class:budget-btn={true}
            >
              {formatTokens(b)}
            </button>
          {/each}
          <div class="budget-custom">
            <input
              id="modal-budget-input"
              type="number"
              value={settings.defaultBudget}
              oninput={(e) => (settings = { ...settings, defaultBudget: parseInt(e.currentTarget.value) || 32000 })}
              step={1000}
              min={4000}
              max={1000000}
              class="input input-mono budget-input"
            />
            <span class="tokens-suffix">tokens</span>
          </div>
        </div>
      </section>

      <section class="card">
        <span class="card-label" id="modal-browser-label">File Browser</span>
        <p class="card-desc">How the Projects folder browser lists directories.</p>

        <div class="browser-row">
          <button
            type="button"
            onclick={() => (settings = { ...settings, showHidden: !settings.showHidden })}
            class="btn btn-sm"
            class:btn-primary={settings.showHidden}
            aria-pressed={settings.showHidden}
            title="Show dot-directories like .config and .cache"
          >
            Show hidden files: {settings.showHidden ? 'ON' : 'OFF'}
          </button>

          <div class="segmented-control sort-seg" role="group" aria-labelledby="modal-browser-label">
            <button
              type="button"
              onclick={() => (settings = { ...settings, fileSort: 'name' })}
              class="segmented-btn"
              class:active={settings.fileSort !== 'updated'}
            >
              A–Z
            </button>
            <button
              type="button"
              onclick={() => (settings = { ...settings, fileSort: 'updated' })}
              class="segmented-btn"
              class:active={settings.fileSort === 'updated'}
            >
              Last updated
            </button>
          </div>
        </div>
      </section>

      <section class="card">
        <label class="card-label" for="modal-theme-select">Interface Theme</label>
        <p class="card-desc">
          Choose from popular developer palettes or press
          <kbd class="font-mono kbd-inline">t</kbd> anywhere for the interactive Theme Picker.
        </p>

        <div class="theme-row">
          <select
            id="modal-theme-select"
            value={currentTheme}
            onchange={(e) => {
              const newT = e.currentTarget.value;
              settings = { ...settings, theme: newT };
              onThemeChange(newT);
            }}
            class="select select-mono theme-select"
          >
            <option value="system">System (follows OS)</option>
            {#each THEMES as t (t.id)}
              <option value={t.id}>{t.name} {t.category === 'light' ? '(Light)' : '(Dark)'}</option>
            {/each}
          </select>
        </div>
      </section>

      <div class="save-row">
        <button type="submit" disabled={isSaving} class="btn btn-primary save-btn">
          {isSaving ? 'Saving...' : 'Save Settings'}
        </button>
        {#if saveSuccess}
          <div class="save-ok">
            <Icon name="check" size={14} />
            <span>Settings saved to server</span>
          </div>
        {/if}
      </div>
    </form>
  </div>
</div>

<style>
  .overlay {
    position: fixed;
    inset: 0;
    background-color: rgba(0, 0, 0, 0.45);
    backdrop-filter: blur(2px);
    display: flex;
    align-items: center;
    justify-content: center;
    z-index: 9999;
    padding: 16px;
    animation: fade-in 120ms ease-out;
  }
  .panel {
    width: 100%;
    max-width: 620px;
    max-height: calc(100vh - 32px);
    max-height: calc(100dvh - 32px);
    overflow-y: auto;
    scrollbar-width: thin;
    scrollbar-color: var(--border-strong) transparent;
    background-color: var(--bg);
    border-radius: var(--radius-lg);
    border: 1px solid var(--border-strong);
    box-shadow: 0 8px 30px rgba(0, 0, 0, 0.12);
    padding: 16px 20px;
    display: flex;
    flex-direction: column;
    gap: 12px;
    animation: pop-in 130ms ease-out;
  }
  .panel-head {
    display: flex;
    justify-content: space-between;
    align-items: flex-start;
    gap: 8px;
  }
  .panel-title {
    font-size: 15px;
    font-weight: 600;
    color: var(--ink);
  }
  .panel-sub {
    font-size: 11px;
    color: var(--ink-faint);
    font-family: var(--font-mono);
    margin-top: 2px;
  }
  .close-btn {
    padding: 4px 8px;
    color: var(--ink-soft);
    flex-shrink: 0;
  }
  .close-x {
    font-size: 12px;
    line-height: 1;
  }
  .settings-form {
    display: flex;
    flex-direction: column;
    gap: 10px;
  }
  .card {
    background-color: var(--surface-raised);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    padding: 12px 16px;
    display: flex;
    flex-direction: column;
    gap: 6px;
  }
  .card-label {
    font-size: 13px;
    font-weight: 600;
    color: var(--ink);
  }
  .card-desc {
    font-size: 12px;
    color: var(--ink-soft);
  }
  .btn-row {
    display: flex;
    gap: 8px;
    margin-top: 6px;
    flex-wrap: wrap;
  }
  .style-btn {
    text-transform: uppercase;
    font-family: var(--font-mono);
  }
  .budget-btn {
    font-family: var(--font-mono);
  }
  .budget-row {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-top: 6px;
    flex-wrap: wrap;
  }
  .budget-custom {
    display: flex;
    align-items: center;
    gap: 6px;
    margin-left: 8px;
  }
  .browser-row {
    display: flex;
    align-items: center;
    gap: 12px;
    margin-top: 6px;
    flex-wrap: wrap;
  }
  .sort-seg {
    font-family: var(--font-mono);
  }
  .budget-input {
    width: 90px;
    height: 28px;
    font-size: 12px;
  }
  .tokens-suffix {
    font-size: 11px;
    color: var(--ink-soft);
  }
  .kbd-inline {
    font-size: 11px;
    padding: 1px 4px;
    border: 1px solid var(--border);
    border-radius: 3px;
  }
  .theme-row {
    display: flex;
    align-items: center;
    gap: 8px;
    margin-top: 6px;
  }
  .theme-select {
    min-width: 240px;
    max-width: 100%;
    height: 30px;
    font-size: 12px;
  }
  .save-row {
    display: flex;
    align-items: center;
    gap: 12px;
    padding-bottom: 2px;
  }
  .save-btn {
    padding: 8px 18px;
  }
  .save-ok {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 12px;
    color: var(--status-ok);
  }
</style>
