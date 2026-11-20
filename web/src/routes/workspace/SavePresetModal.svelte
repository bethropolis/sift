<script lang="ts">
  import { api, type SelectionMode } from '../../lib/api';
  import { formatTokens } from '../../lib/format';
  import { toast } from '../../lib/toast.svelte';
  import Dialog from '../../components/Dialog.svelte';
  import Icon from '../../components/Icon.svelte';

  interface Props {
    root: string;
    selections: Record<string, SelectionMode>;
    prompt: string;
    style: string;
    budget: number;
    selectedCount: number;
    usedTokens: number;
    onClose: () => void;
  }

  let { root, selections, prompt, style, budget, selectedCount, usedTokens, onClose }: Props = $props();

  let name = $state('');
  let existingNames = $state<string[]>([]);
  let saving = $state(false);
  let error = $state<string | null>(null);
  let nameEl: HTMLInputElement | null = $state(null);

  let trimmed = $derived(name.trim());
  let duplicate = $derived(existingNames.includes(trimmed));
  let promptLine = $derived(prompt.trim().replace(/\s+/g, ' '));

  $effect(() => {
    nameEl?.focus();
  });

  $effect(() => {
    let alive = true;
    api
      .listPresets(root)
      .then((res) => {
        if (alive) existingNames = res.presets.map((p) => p.name);
      })
      .catch(() => {
        // Duplicate detection is only a hint; saving still works.
      });
    return () => {
      alive = false;
    };
  });

  async function handleSave(e: SubmitEvent) {
    e.preventDefault();
    if (!trimmed || saving) return;
    saving = true;
    error = null;
    try {
      await api.savePreset({
        root,
        name: trimmed,
        selections,
        prompt,
        style,
        budget,
      });
      toast({
        id: 'preset',
        kind: 'success',
        message: `${duplicate ? 'Updated' : 'Saved'} preset “${trimmed}”`,
        duration: 2200,
      });
      onClose();
    } catch (err) {
      error = err instanceof Error ? err.message : 'Could not save the preset';
    } finally {
      saving = false;
    }
  }
</script>

<Dialog
  title="Save Preset"
  subtitle="Store this selection, prompt, style and budget for the project."
  icon="download"
  width="510px"
  {onClose}
>
  <form id="save-preset-form" onsubmit={handleSave} class="form">
    <label class="field">
      <span class="field-label">Preset name</span>
      <input
        bind:this={nameEl}
        bind:value={name}
        class="input input-mono name-input"
        placeholder="e.g. auth refactor"
        maxlength={80}
        autocomplete="off"
        spellcheck="false"
      />
    </label>

    <div class="summary">
      <div class="stat">
        <span class="stat-value font-mono">{selectedCount}</span>
        <span class="stat-label">files</span>
      </div>
      <div class="stat">
        <span class="stat-value font-mono">{formatTokens(usedTokens)}</span>
        <span class="stat-label">tokens</span>
      </div>
      <div class="stat">
        <span class="stat-value font-mono">{style}</span>
        <span class="stat-label">style</span>
      </div>
      <div class="stat">
        <span class="stat-value font-mono">{formatTokens(budget)}</span>
        <span class="stat-label">budget</span>
      </div>
    </div>

    <div class="prompt-row">
      <span class="field-label">Task prompt</span>
      {#if promptLine}
        <span class="prompt-text" title={prompt}>{promptLine}</span>
      {:else}
        <span class="prompt-empty">None — the preset stores the selection only.</span>
      {/if}
    </div>

    {#if duplicate}
      <p class="duplicate">
        <Icon name="warning" size={13} />
        A preset named “{trimmed}” exists; saving replaces it.
      </p>
    {/if}
    {#if error}<p class="error">{error}</p>{/if}

    <p class="note">Stored per project in presets.json. Source files are not touched.</p>

    <button type="submit" class="hidden-submit" tabindex="-1" aria-hidden="true"></button>
  </form>

  {#snippet footer()}
    <button class="btn" onclick={onClose}>Cancel</button>
    <button type="submit" form="save-preset-form" class="btn btn-primary save-btn" disabled={!trimmed || saving}>
      {saving ? 'Saving…' : duplicate ? 'Replace preset' : 'Save preset'}
    </button>
  {/snippet}
</Dialog>

<style>
  .form {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: 5px;
  }
  .field-label {
    font-size: 10.5px;
    font-weight: 600;
    color: var(--ink-soft);
    text-transform: uppercase;
    letter-spacing: 0.04em;
  }
  .name-input {
    height: 32px;
    font-size: 12.5px;
  }
  .summary {
    display: grid;
    grid-template-columns: repeat(4, 1fr);
    gap: 8px;
  }
  .stat {
    border: 1px solid var(--border);
    background-color: var(--surface-raised);
    border-radius: var(--radius-sm);
    padding: 7px 9px;
    display: flex;
    flex-direction: column;
    gap: 1px;
    min-width: 0;
  }
  .stat-value {
    font-size: 13px;
    color: var(--ink);
    font-weight: 600;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .stat-label {
    font-size: 9.5px;
    color: var(--ink-faint);
  }
  .prompt-row {
    display: flex;
    flex-direction: column;
    gap: 4px;
    border-top: 1px solid var(--border);
    padding-top: 10px;
  }
  .prompt-text {
    font-size: 11.5px;
    color: var(--ink);
    line-height: 1.45;
    display: -webkit-box;
    -webkit-line-clamp: 3;
    -webkit-box-orient: vertical;
    overflow: hidden;
  }
  .prompt-empty {
    font-size: 11px;
    color: var(--ink-faint);
  }
  .duplicate {
    display: flex;
    align-items: center;
    gap: 6px;
    font-size: 11px;
    color: var(--status-warn);
  }
  .error {
    font-size: 11.5px;
    color: var(--status-danger);
  }
  .note {
    font-size: 10.5px;
    color: var(--ink-faint);
  }
  .hidden-submit {
    display: none;
  }
  .save-btn {
    min-width: 108px;
  }
  @media (max-width: 480px) {
    .summary {
      grid-template-columns: repeat(2, 1fr);
    }
  }
</style>
