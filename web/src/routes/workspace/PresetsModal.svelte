<script lang="ts">
  import { onDestroy } from 'svelte';
  import { api, type Preset } from '../../lib/api';
  import { formatTokens } from '../../lib/format';
  import { toast } from '../../lib/toast.svelte';
  import Dialog from '../../components/Dialog.svelte';
  import Icon from '../../components/Icon.svelte';

  interface Props {
    root: string;
    onApply: (preset: Preset) => void;
    onClose: () => void;
  }

  let { root, onApply, onClose }: Props = $props();

  let presets = $state<Preset[]>([]);
  let loading = $state(true);
  let error = $state<string | null>(null);
  let confirming = $state<string | null>(null);
  let deleting = $state<string | null>(null);
  let confirmTimer: ReturnType<typeof setTimeout> | null = null;

  function fileCount(preset: Preset): number {
    let count = 0;
    for (const mode of Object.values(preset.selections)) {
      if (mode !== 'skip') count++;
    }
    return count;
  }

  function summary(preset: Preset): string {
    const parts = [`${fileCount(preset)} files`];
    if (preset.style) parts.push(preset.style);
    if (preset.budget) parts.push(`${formatTokens(preset.budget)} budget`);
    if (preset.prompt?.trim()) parts.push('prompt');
    return parts.join(' · ');
  }

  function when(iso?: string): string {
    if (!iso) return '';
    const d = new Date(iso);
    if (Number.isNaN(d.getTime())) return '';
    return d.toLocaleDateString(undefined, { month: 'short', day: 'numeric' });
  }

  async function load() {
    loading = true;
    error = null;
    try {
      const result = await api.listPresets(root);
      presets = result.presets;
    } catch (err) {
      error = err instanceof Error ? err.message : 'Could not load presets';
    } finally {
      loading = false;
    }
  }

  $effect(() => {
    void root;
    void load();
  });

  function apply(preset: Preset) {
    onApply(preset);
    toast({ id: 'preset', kind: 'success', message: `Loaded “${preset.name}”`, duration: 2200 });
    onClose();
  }

  function armDelete(name: string) {
    confirming = confirming === name ? null : name;
    if (confirmTimer) clearTimeout(confirmTimer);
    if (confirming) {
      confirmTimer = setTimeout(() => (confirming = null), 3500);
    }
  }

  async function remove(preset: Preset) {
    if (confirming !== preset.name) {
      armDelete(preset.name);
      return;
    }
    deleting = preset.name;
    try {
      await api.deletePreset(root, preset.name);
      presets = presets.filter((p) => p.name !== preset.name);
      confirming = null;
      toast({ id: 'preset', kind: 'success', message: `Deleted “${preset.name}”`, duration: 2000 });
    } catch (err) {
      toast({
        id: 'preset',
        kind: 'error',
        message: err instanceof Error ? err.message : 'Could not delete the preset',
        duration: 2600,
      });
    } finally {
      deleting = null;
    }
  }

  onDestroy(() => {
    if (confirmTimer) clearTimeout(confirmTimer);
  });
</script>

<Dialog
  title="Presets"
  subtitle="Load a saved selection for this project."
  icon="folder"
  width="580px"
  {onClose}
>
  {#if loading}
    <div class="list-skeleton" aria-label="Loading presets">
      {#each [0, 1, 2] as i (i)}
        <span></span>
      {/each}
    </div>
  {:else if error}
    <div class="state error">
      <Icon name="warning" size={16} />
      <div>
        <p>{error}</p>
        <button class="btn btn-sm" onclick={() => void load()}>Retry</button>
      </div>
    </div>
  {:else if presets.length === 0}
    <div class="state empty">
      <span class="empty-icon"><Icon name="folder" size={26} /></span>
      <div>
        <p class="empty-title">No presets yet</p>
        <p class="empty-sub">Use <span class="font-mono">Save</span> in the toolbar to store the current selection.</p>
      </div>
    </div>
  {:else}
    <div class="preset-list">
      {#each presets as preset (preset.name)}
        <div class="preset-row">
          <button class="preset-main" onclick={() => apply(preset)} title={`Load ${preset.name}`}>
            <span class="preset-name font-mono">{preset.name}</span>
            <span class="preset-meta font-mono">{summary(preset)}</span>
            {#if preset.prompt?.trim()}
              <span class="preset-prompt">{preset.prompt.trim()}</span>
            {/if}
          </button>

          <div class="preset-side">
            {#if when(preset.updated_at)}<span class="preset-date font-mono">{when(preset.updated_at)}</span>{/if}
            <button
              class="btn btn-sm delete-btn"
              class:armed={confirming === preset.name}
              onclick={() => void remove(preset)}
              disabled={deleting === preset.name}
              aria-label={confirming === preset.name ? `Confirm delete ${preset.name}` : `Delete ${preset.name}`}
              title={confirming === preset.name ? 'Click again to delete' : 'Delete preset'}
            >
              <Icon name="trash" size={11} />
              {#if confirming === preset.name}
                <span>Delete?</span>
              {:else if deleting === preset.name}
                <span>…</span>
              {/if}
            </button>
            <button class="btn btn-sm load-btn" onclick={() => apply(preset)}>Load</button>
          </div>
        </div>
      {/each}
    </div>
    <p class="hint font-mono">{presets.length} preset{presets.length === 1 ? '' : 's'} · click a row to load</p>
  {/if}

  {#snippet footer()}
    <button class="btn" onclick={onClose}>Close</button>
  {/snippet}
</Dialog>

<style>
  .preset-list {
    display: flex;
    flex-direction: column;
    gap: 4px;
  }
  .preset-row {
    display: flex;
    align-items: center;
    gap: 8px;
    border: 1px solid transparent;
    border-radius: var(--radius-sm);
    padding: 2px 4px 2px 2px;
    transition:
      background-color 60ms ease,
      border-color 60ms ease;
  }
  .preset-row:hover {
    background-color: var(--surface-alt);
    border-color: var(--border);
  }
  .preset-main {
    flex: 1;
    min-width: 0;
    display: flex;
    flex-direction: column;
    align-items: flex-start;
    gap: 1px;
    padding: 7px 8px;
    border: none;
    border-radius: var(--radius-sm);
    background: transparent;
    color: inherit;
    text-align: left;
    cursor: pointer;
    font: inherit;
  }
  .preset-name {
    font-size: 12.5px;
    color: var(--ink);
    font-weight: 600;
    max-width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .preset-meta {
    font-size: 10px;
    color: var(--ink-faint);
  }
  .preset-prompt {
    margin-top: 2px;
    font-size: 10.5px;
    color: var(--ink-soft);
    max-width: 100%;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .preset-side {
    display: flex;
    align-items: center;
    gap: 4px;
    flex-shrink: 0;
  }
  .preset-date {
    font-size: 9.5px;
    color: var(--ink-faint);
    margin-right: 2px;
  }
  .load-btn {
    padding: 3px 9px;
    font-size: 11px;
  }
  .delete-btn {
    padding: 3px 5px;
    color: var(--ink-faint);
    font-size: 10px;
    gap: 3px;
  }
  .delete-btn:hover:not(:disabled) {
    color: var(--status-danger);
    border-color: color-mix(in srgb, var(--status-danger) 40%, var(--border));
  }
  .delete-btn.armed {
    color: var(--status-danger);
    border-color: var(--status-danger);
  }
  .hint {
    margin-top: 10px;
    font-size: 10px;
    color: var(--ink-faint);
  }
  .list-skeleton {
    display: flex;
    flex-direction: column;
    gap: 7px;
    padding: 4px 2px;
  }
  .list-skeleton span {
    height: 38px;
    border-radius: var(--radius-sm);
    background-color: var(--surface-alt);
    border: 1px solid var(--border);
    animation: pulse 1.2s ease-in-out infinite;
  }
  .list-skeleton span:nth-child(2) { opacity: 0.75; }
  .list-skeleton span:nth-child(3) { opacity: 0.5; }
  @keyframes pulse {
    50% { opacity: 0.4; }
  }
  .state {
    display: flex;
    align-items: flex-start;
    gap: 9px;
    padding: 12px;
    color: var(--ink-soft);
    font-size: 11.5px;
  }
  .state.error {
    color: var(--status-danger);
  }
  .state.empty {
    align-items: center;
    padding: 26px 12px;
    justify-content: center;
  }
  .empty-icon {
    color: var(--ink-faint);
    display: flex;
  }
  .empty-title {
    font-size: 12.5px;
    font-weight: 600;
    color: var(--ink);
    margin-bottom: 2px;
  }
  .empty-sub {
    color: var(--ink-faint);
    font-size: 11px;
  }
  .state p + button {
    margin-top: 7px;
  }
</style>
