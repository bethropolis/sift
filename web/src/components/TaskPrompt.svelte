<script lang="ts">
  interface Props {
    prompt: string;
    onChange: (value: string) => void;
  }

  let { prompt, onChange }: Props = $props();
  let isExpanded = $state(false);
  let hasContent = $derived(prompt.trim().length > 0);
</script>

<div class="prompt-wrap">
  <div
    onclick={() => (isExpanded = !isExpanded)}
    onkeydown={(e) => {
      if (e.key === 'Enter' || e.key === ' ') {
        e.preventDefault();
        isExpanded = !isExpanded;
      }
    }}
    role="button"
    tabindex="0"
    aria-expanded={isExpanded}
    class="prompt-head"
  >
    <div class="prompt-label">
      <span class="prompt-caret">{isExpanded ? '▼' : '▶'}</span>
      <span class="prompt-name">prompt</span>
      {#if hasContent}
        <span class="prompt-count">[{prompt.length} chars]</span>
      {/if}
    </div>

    <span class="prompt-action">
      {isExpanded ? 'collapse' : hasContent ? 'edit' : '+ add task instructions'}
    </span>
  </div>

  {#if isExpanded}
    <div class="prompt-body">
      <textarea
        value={prompt}
        oninput={(e) => onChange(e.currentTarget.value)}
        placeholder="Type task prompt or instructions to prepend to context document..."
        rows={2}
        class="prompt-input"
      ></textarea>
      {#if hasContent}
        <div class="prompt-clear-row">
          <button
            onclick={(e) => {
              e.stopPropagation();
              onChange('');
            }}
            class="prompt-clear"
          >
            clear
          </button>
        </div>
      {/if}
    </div>
  {/if}
</div>

<style>
  .prompt-wrap {
    border-bottom: 1px solid var(--border);
    background-color: rgba(255, 255, 255, 0.015);
    user-select: none;
    font-family: var(--font-mono);
  }
  .prompt-head {
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 4px 12px;
    cursor: pointer;
    font-size: 10.5px;
  }
  .prompt-label {
    display: flex;
    align-items: center;
    gap: 6px;
  }
  .prompt-caret {
    color: var(--ink-faint);
    font-size: 9px;
  }
  .prompt-name {
    color: var(--ink-soft);
  }
  .prompt-count {
    font-size: 10px;
  }
  .prompt-action {
    font-size: 10px;
    color: var(--ink-faint);
  }
  .prompt-body {
    padding: 0 12px 6px 12px;
  }
  .prompt-input {
    width: 100%;
    background-color: var(--bg);
    color: var(--ink);
    font-family: var(--font-mono);
    font-size: 11px;
    line-height: 1.4;
    padding: 4px 8px;
    border: 1px solid var(--border);
    border-radius: var(--radius-sm);
    resize: vertical;
  }
  .prompt-clear-row {
    display: flex;
    justify-content: flex-end;
    margin-top: 2px;
  }
  .prompt-clear {
    background: none;
    border: none;
    color: var(--ink-faint);
    font-size: 10px;
    font-family: var(--font-mono);
    cursor: pointer;
    padding: 0;
  }
</style>
