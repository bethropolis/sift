<script lang="ts">
  import { formatTokens } from '../lib/format';
  import Icon from './Icon.svelte';

  interface Props {
    filePath: string | null;
    content: string;
    tokens: number;
    language: string;
    mode: 'full' | 'sigs';
    isLoading: boolean;
    onModeToggle: (newMode: 'full' | 'sigs') => void;
  }

  let { filePath, content, tokens, language, mode, isLoading, onModeToggle }: Props = $props();

  const KEYWORDS = new Set([
    'var', 'const', 'type', 'func', 'struct', 'package', 'import', 'return',
    'if', 'else', 'switch', 'case', 'default', 'for', 'range', 'interface', 'select',
  ]);
  const TYPE_LITERALS = new Set(['error', 'string', 'int', 'bool', 'true', 'false', 'nil']);

  interface Token {
    text: string;
    kind: 'plain' | 'str' | 'kw' | 'type' | 'comment';
  }

  // Gentle, low-contrast syntax tones that reduce visual fatigue.
  function tokenize(line: string): Token[] {
    if (!line) return [{ text: '\n', kind: 'plain' }];
    const t = line.trim();
    if (t.startsWith('//') || t.startsWith('/*') || t.startsWith('*')) {
      return [{ text: line, kind: 'comment' }];
    }
    const parts = line.split(
      /("(?:[^"\\]|\\.)*"|`[^`]*`|\b(?:var|const|type|func|struct|package|import|return|if|else|switch|case|default|for|range|interface|select|error|string|int|bool|true|false|nil)\b)/g,
    );
    const out: Token[] = [];
    for (const part of parts) {
      if (!part) continue;
      if (part.startsWith('"') || part.startsWith('`')) out.push({ text: part, kind: 'str' });
      else if (KEYWORDS.has(part)) out.push({ text: part, kind: 'kw' });
      else if (TYPE_LITERALS.has(part)) out.push({ text: part, kind: 'type' });
      else out.push({ text: part, kind: 'plain' });
    }
    return out;
  }

  let lines = $derived(content.split('\n'));
</script>

{#if !filePath}
  <div class="empty">
    <span>No file selected</span>
    <span class="empty-hint">Select or navigate to a file (j/k or click) to inspect</span>
  </div>
{:else}
  <div class="preview">
    <div class="preview-head">
      <div class="file-id">
        <Icon name="file" size={13} />
        <span class="font-mono file-path">{filePath}</span>
        <span class="font-mono lang-tag">{language}</span>
        <span class="font-mono tabular-nums token-count">{formatTokens(tokens)} tokens</span>
      </div>

      <div class="mode-group">
        <div class="mode-seg">
          <button
            onclick={() => onModeToggle('full')}
            class="mode-btn"
            class:active={mode === 'full'}
          >
            full
          </button>
          <button onclick={() => onModeToggle('sigs')} class="mode-btn" class:active={mode === 'sigs'} class:sigs={mode === 'sigs'}>
            sigs
          </button>
        </div>
      </div>
    </div>

    <div class="code-scroll">
      {#if isLoading}
        <div class="loading"><span>Loading file content...</span></div>
      {:else}
        <div class="code-row">
          <div aria-hidden="true" class="gutter">
            {#each lines as _, i (i)}
              <div class="tabular-nums">{i + 1}</div>
            {/each}
          </div>

          <pre class="code"><code>{#each lines as line, i (i)}<div class="code-line">{#each tokenize(line) as tok (tok.text + tok.kind)}<span
                      class:syn-str={tok.kind === 'str'}
                      class:syn-kw={tok.kind === 'kw'}
                      class:syn-type={tok.kind === 'type'}
                      class:syn-comment={tok.kind === 'comment'}>{tok.text}</span
                    >{/each}</div>{/each}</code></pre>
        </div>
      {/if}
    </div>
  </div>
{/if}

<style>
  .empty {
    flex: 1;
    display: flex;
    flex-direction: column;
    align-items: center;
    justify-content: center;
    background-color: var(--code-bg);
    color: var(--code-dim);
    font-family: var(--font-mono);
    font-size: 12px;
    gap: 6px;
    padding: 24px;
  }
  .empty-hint {
    font-size: 11px;
    color: var(--code-line-number);
  }
  .preview {
    flex: 1;
    display: flex;
    flex-direction: column;
    background-color: var(--code-bg);
    overflow: hidden;
    color: var(--code-text);
  }
  .preview-head {
    height: 32px;
    min-height: 32px;
    background-color: var(--code-gutter-bg);
    border-bottom: 1px solid var(--code-border);
    display: flex;
    align-items: center;
    justify-content: space-between;
    padding: 0 10px;
    user-select: none;
  }
  .file-id {
    display: flex;
    align-items: center;
    gap: 8px;
    min-width: 0;
  }
  .file-path {
    font-size: 11.5px;
    font-weight: 500;
    color: var(--code-text);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
  .lang-tag {
    font-size: 10px;
    color: var(--code-dim);
    background-color: rgba(255, 255, 255, 0.05);
    padding: 1px 5px;
    border-radius: 3px;
    text-transform: uppercase;
  }
  .token-count {
    font-size: 10.5px;
    color: var(--code-dim);
  }
  .mode-group {
    display: flex;
    align-items: center;
    gap: 4px;
  }
  .mode-seg {
    display: inline-flex;
    background-color: rgba(255, 255, 255, 0.04);
    border-radius: 3px;
    padding: 1px;
    border: 1px solid var(--code-border);
  }
  .mode-btn {
    border: none;
    background-color: transparent;
    color: var(--code-dim);
    font-family: var(--font-mono);
    font-size: 10px;
    font-weight: 600;
    padding: 2px 7px;
    border-radius: 2px;
    cursor: pointer;
  }
  .mode-btn.active {
    background-color: rgba(255, 255, 255, 0.1);
    color: var(--code-text);
  }
  .mode-btn.active.sigs {
    color: var(--code-accent);
  }
  .code-scroll {
    flex: 1;
    overflow: auto;
    display: flex;
    font-family: var(--font-mono);
    font-size: 11.5px;
    line-height: 1.55;
  }
  .loading {
    padding: 16px;
    color: var(--code-dim);
    display: flex;
    align-items: center;
    gap: 6px;
  }
  .code-row {
    display: flex;
    min-width: 100%;
  }
  .gutter {
    padding: 8px 8px 8px 10px;
    background-color: var(--code-gutter-bg);
    border-right: 1px solid var(--code-border);
    text-align: right;
    user-select: none;
    color: var(--code-line-number);
    min-width: 38px;
  }
  .code {
    margin: 0;
    padding: 8px 14px;
    flex: 1;
    overflow-x: auto;
    white-space: pre;
    tab-size: 4;
    color: var(--code-text);
  }
  .code-line {
    min-height: 18px;
  }
  .syn-str {
    color: #98c379;
  }
  .syn-kw {
    color: #abb2bf;
    font-weight: 600;
  }
  .syn-type {
    color: #d19a66;
  }
  .syn-comment {
    color: var(--code-dim);
  }
</style>
