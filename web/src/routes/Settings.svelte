<script lang="ts">
  import { api, type SettingsData, type ApiMeta } from '../lib/api';
  import { formatTokens } from '../lib/format';
  import { THEMES } from '../lib/themes';
  import Icon from '../components/Icon.svelte';

  interface Props {
    meta: ApiMeta | null;
    currentTheme: string;
    onThemeChange: (theme: string) => void;
  }

  let { meta, currentTheme, onThemeChange }: Props = $props();

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

  const SHORTCUT_LIST = [
    { key: 'j / k or ↓ / ↑', desc: 'Move tree item focus' },
    { key: 'Space', desc: 'Toggle selection (Full / Skip)' },
    { key: 'f', desc: 'Cycle mode (Full → Sigs → Skip)' },
    { key: 'b', desc: 'Toggle sidebar (collapse/expand)' },
    { key: 't', desc: 'Open Color Theme picker' },
    { key: '/', desc: 'Focus file filter' },
    { key: 's', desc: 'Run smart select based on git relevance and budget' },
    { key: 'g', desc: 'Generate context document' },
    { key: 'y', desc: 'Generate and copy to clipboard' },
    { key: '?', desc: 'Open shortcuts overlay modal' },
    { key: 'Esc', desc: 'Close overlay / blur inputs' },
  ];

  $effect(() => {
    api.getSettings().then((data) => {
      settings = data;
    });
  });

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

<div class="settings-page">
  <div>
    <h1 class="font-mono settings-title">Settings</h1>
    <p class="settings-sub">Configure default prompt formatting and server defaults.</p>
  </div>

  <form onsubmit={handleSave} class="settings-form">
    <section class="card">
      <label class="card-label" for="style-group">Default Output Document Style</label>
      <p class="card-desc">Rendered format when generating context documents for language models.</p>
      <div id="style-group" class="btn-row">
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
      <label class="card-label" for="budget-input">Default Token Budget</label>
      <p class="card-desc">Initial target budget in tokens for fresh workspace sessions.</p>

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
            id="budget-input"
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
      <span class="card-label" id="browser-label">File Browser</span>
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

        <div class="segmented-control sort-seg" role="group" aria-labelledby="browser-label">
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
      <label class="card-label" for="theme-select">Interface Theme</label>
      <p class="card-desc">
        Choose from popular developer palettes or press
        <kbd class="font-mono kbd-inline">t</kbd> anywhere for the interactive Theme Picker.
      </p>

      <div class="theme-row">
        <select
          id="theme-select"
          value={currentTheme}
          onchange={(e) => {
            const newT = e.currentTarget.value;
            settings = { ...settings, theme: newT };
            onThemeChange(newT);
          }}
          class="select select-mono theme-select"
        >
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

  <section class="card shortcuts-card">
    <div class="shortcuts-head">
      <h2 class="font-mono shortcuts-title">Keyboard Shortcuts Reference</h2>
      <p class="shortcuts-sub">All terminal keybindings are supported in the browser picker.</p>
    </div>

    <div class="shortcuts-body">
      {#each SHORTCUT_LIST as item (item.key)}
        <div class="shortcut-row">
          <span class="shortcut-desc">{item.desc}</span>
          <kbd class="font-mono shortcut-key">{item.key}</kbd>
        </div>
      {/each}
    </div>
  </section>
</div>

<style>
  .settings-page {
    flex: 1;
    overflow-y: auto;
    padding: 24px 32px;
    background-color: var(--bg);
    display: flex;
    flex-direction: column;
    gap: 24px;
    max-width: 720px;
    margin: 0 auto;
    width: 100%;
  }
  .settings-title {
    font-size: 18px;
    font-weight: 600;
    color: var(--ink);
  }
  .settings-sub {
    font-size: 13px;
    color: var(--ink-soft);
    margin-top: 2px;
  }
  .settings-form {
    display: flex;
    flex-direction: column;
    gap: 20px;
  }
  .card {
    background-color: var(--surface-raised);
    border: 1px solid var(--border);
    border-radius: var(--radius-lg);
    padding: 16px 20px;
    display: flex;
    flex-direction: column;
    gap: 8px;
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
    height: 30px;
    font-size: 12px;
  }
  .save-row {
    display: flex;
    align-items: center;
    gap: 12px;
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
  .shortcuts-card {
    margin-top: 12px;
    overflow: hidden;
    padding: 0;
    gap: 0;
  }
  .shortcuts-head {
    padding: 12px 20px;
    border-bottom: 1px solid var(--border);
  }
  .shortcuts-title {
    font-size: 13px;
    font-weight: 600;
    color: var(--ink);
  }
  .shortcuts-sub {
    font-size: 12px;
    color: var(--ink-soft);
    margin-top: 2px;
  }
  .shortcuts-body {
    padding: 8px 20px 16px;
  }
  .shortcut-row {
    display: flex;
    justify-content: space-between;
    align-items: center;
    padding: 8px 0;
    border-bottom: 1px solid var(--border);
    font-size: 12px;
  }
  .shortcut-desc {
    color: var(--ink);
  }
  .shortcut-key {
    background-color: var(--surface-alt);
    border: 1px solid var(--border-strong);
    border-radius: 4px;
    padding: 2px 8px;
    font-size: 11px;
    color: var(--ink);
  }
</style>
