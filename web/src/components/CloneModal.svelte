<script lang="ts">
  import { api, type CloneError, type CloneProgress, type TempClone } from '../lib/api';
  import { addClone, looksLikeRepoUrl, repoNameFromUrl } from '../lib/clones.svelte';
  import LoaderMark from './LoaderMark.svelte';

  interface Props {
    /** Prefill (e.g. a URL pasted into the Projects search box). */
    initialUrl?: string;
    onClose: () => void;
    onCloned: (clone: TempClone) => void;
  }

  let { initialUrl = '', onClose, onCloned }: Props = $props();

  let url = $state(initialUrl);
  let branch = $state('');
  let depth = $state(1);

  let running = $state(false);
  let progress = $state<CloneProgress | null>(null);
  let errorMsg = $state<string | null>(null);
  let errorDetail = $state<string | null>(null);

  let urlInput: HTMLInputElement | null = $state(null);
  let controller: AbortController | null = null;

  let canSubmit = $derived(!running && looksLikeRepoUrl(url));
  let target = $derived(repoNameFromUrl(url));
  // git's own wording varies by transport; these cover the usual "needs a
  // login" failures so the hint shows when it is actually useful.
  let looksLikeAuth = $derived(
    !!errorDetail && /authentication|could not read (username|password)|permission denied|publickey|terminal prompts disabled|host key verification/i.test(errorDetail),
  );

  // Focus the URL box once it mounts, cursor at the end of any prefill.
  $effect(() => {
    if (!urlInput) return;
    urlInput.focus();
    const end = urlInput.value.length;
    urlInput.setSelectionRange(end, end);
  });

  // Leaving while a clone runs aborts it: the server stops git and removes
  // the half-made checkout when the request goes away.
  $effect(() => () => controller?.abort());

  async function start(e?: SubmitEvent) {
    e?.preventDefault();
    if (!canSubmit) return;
    running = true;
    progress = null;
    errorMsg = null;
    errorDetail = null;
    const ctl = new AbortController();
    controller = ctl;
    try {
      const clone = await api.cloneRepo(
        {
          url: url.trim(),
          branch: branch.trim() || undefined,
          depth: Math.min(1000, Math.max(1, Math.floor(depth) || 1)),
        },
        (p) => (progress = p),
        ctl.signal,
      );
      addClone(clone);
      onCloned(clone);
    } catch (err) {
      if (ctl.signal.aborted) return; // the user cancelled; nothing to report
      const ce = err as CloneError;
      errorMsg = ce.message || 'Clone failed';
      errorDetail = ce.detail ?? null;
    } finally {
      running = false;
      if (controller === ctl) controller = null;
    }
  }

  function cancel() {
    controller?.abort();
  }

  function closeModal() {
    cancel();
    onClose();
  }

  function handleWindowKey(e: KeyboardEvent) {
    if (e.key !== 'Escape') return;
    // Esc stops a running clone first; a second Esc closes the dialog.
    if (running) cancel();
    else onClose();
  }
</script>

<svelte:window onkeydown={handleWindowKey} />

<div role="dialog" aria-modal="true" aria-label="Clone repository" onclick={closeModal} class="overlay">
  <div onclick={(e) => e.stopPropagation()} class="panel">
    <div class="panel-head">
      <div>
        <h2 class="font-mono panel-title">Clone repository</h2>
        <p class="panel-sub">Shallow, temporary checkout. Removed when sift stops.</p>
      </div>
      <button onclick={closeModal} class="btn btn-sm btn-ghost close-btn" aria-label="Close">
        <span aria-hidden="true" class="close-x">✕</span>
      </button>
    </div>

    <form onsubmit={start} class="form">
      <div class="field">
        <label class="label" for="clone-url">Repository URL</label>
        <input
          id="clone-url"
          bind:this={urlInput}
          bind:value={url}
          type="text"
          inputmode="url"
          autocomplete="off"
          autocapitalize="off"
          spellcheck="false"
          disabled={running}
          placeholder="https://github.com/owner/repo"
          class="input input-mono url-input"
        />
        <p class="hint">
          https, ssh, or <span class="font-mono">git@host:owner/repo</span>. Uses your system git and
          its credentials.
        </p>
      </div>

      <div class="opts">
        <div class="field grow">
          <label class="label" for="clone-branch">Branch or tag <span class="opt">optional</span></label>
          <input
            id="clone-branch"
            bind:value={branch}
            type="text"
            autocomplete="off"
            autocapitalize="off"
            spellcheck="false"
            disabled={running}
            placeholder="default branch"
            class="input input-mono"
          />
        </div>
        <div class="field depth">
          <label class="label" for="clone-depth">Depth</label>
          <input
            id="clone-depth"
            bind:value={depth}
            type="number"
            min="1"
            max="1000"
            step="1"
            disabled={running}
            class="input input-mono"
          />
        </div>
      </div>

      {#if running}
        <div class="progress" role="status" aria-live="polite">
          <div class="p-head">
            <LoaderMark size={14} />
            <span class="p-phase">{progress ? progress.phase : `Connecting to ${target}…`}</span>
            {#if progress}
              <span class="font-mono tabular-nums p-pct">{progress.percent}%</span>
            {/if}
          </div>
          <div class="track" class:skeleton={!progress}>
            {#if progress}
              <div class="fill" style:width={`${progress.percent}%`}></div>
            {/if}
          </div>
        </div>
      {:else if errorMsg}
        <div class="error" role="alert">
          <span class="error-msg">{errorMsg}</span>
          {#if errorDetail}
            <pre class="error-detail font-mono">{errorDetail}</pre>
          {/if}
          {#if looksLikeAuth}
            <span class="error-hint">
              Private repository? sift uses your system git, so set up an SSH key or a git credential
              helper, then try again.
            </span>
          {/if}
        </div>
      {/if}

      <div class="actions">
        {#if running}
          <button type="button" onclick={cancel} class="btn btn-sm">Cancel clone</button>
        {:else}
          <button type="button" onclick={closeModal} class="btn btn-sm btn-ghost">Close</button>
          <button type="submit" disabled={!canSubmit} class="btn btn-primary submit-btn">
            Clone and open
          </button>
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
    max-width: 520px;
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
  .form {
    display: flex;
    flex-direction: column;
    gap: 12px;
  }
  .field {
    display: flex;
    flex-direction: column;
    gap: 5px;
    min-width: 0;
  }
  .label {
    font-size: 12px;
    font-weight: 600;
    color: var(--ink);
  }
  .opt {
    font-weight: 400;
    color: var(--ink-faint);
    margin-left: 4px;
  }
  .url-input {
    height: 32px;
    font-size: 12px;
  }
  .hint {
    font-size: 11.5px;
    color: var(--ink-faint);
    line-height: 1.4;
  }
  .opts {
    display: flex;
    gap: 10px;
    align-items: flex-start;
  }
  .grow {
    flex: 1;
  }
  .depth {
    width: 84px;
    flex-shrink: 0;
  }
  .progress {
    display: flex;
    flex-direction: column;
    gap: 7px;
    padding: 10px 12px;
    background-color: var(--surface-raised);
    border: 1px solid var(--border);
    border-radius: var(--radius-md);
  }
  .p-head {
    display: flex;
    align-items: center;
    gap: 8px;
    font-size: 12px;
    color: var(--ink);
  }
  .p-phase {
    flex: 1;
    min-width: 0;
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .p-pct {
    font-size: 11px;
    color: var(--ink-soft);
  }
  .track {
    height: 4px;
    border-radius: 2px;
    background-color: var(--surface-alt);
    overflow: hidden;
  }
  .fill {
    height: 100%;
    background-color: var(--accent);
    border-radius: 2px;
    transition: width 120ms ease-out;
  }
  .error {
    display: flex;
    flex-direction: column;
    gap: 6px;
    padding: 10px 12px;
    border: 1px solid var(--status-danger);
    border-radius: var(--radius-md);
    background-color: color-mix(in srgb, var(--status-danger) 8%, var(--bg));
    font-size: 12px;
  }
  .error-msg {
    color: var(--status-danger);
    font-weight: 600;
  }
  .error-detail {
    margin: 0;
    font-size: 11px;
    line-height: 1.45;
    color: var(--ink-soft);
    white-space: pre-wrap;
    overflow-wrap: anywhere;
    max-height: 120px;
    overflow-y: auto;
  }
  .error-hint {
    font-size: 11.5px;
    color: var(--ink-soft);
    line-height: 1.4;
  }
  .actions {
    display: flex;
    justify-content: flex-end;
    align-items: center;
    gap: 8px;
  }
  .submit-btn {
    padding: 7px 16px;
  }
  @media (max-width: 480px) {
    .opts {
      flex-direction: column;
    }
    .depth {
      width: 100%;
    }
  }
</style>
