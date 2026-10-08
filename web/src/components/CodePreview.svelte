<script lang="ts">
  import { formatTokens } from '../lib/format';
  import Icon from './Icon.svelte';
  import Loader from './Loader.svelte';

  interface Props {
    filePath: string | null;
    content: string;
    tokens: number;
    language: string;
    /** Server highlight spans ([line, start, end, kind], byte offsets). */
    spans?: Array<[number, number, number, string]>;
    mode: 'full' | 'sigs';
    isLoading: boolean;
    onModeToggle: (newMode: 'full' | 'sigs') => void;
  }

  let { filePath, content, tokens, language, spans, mode, isLoading, onModeToggle }: Props = $props();

  // Server TokenKind -> visual group. Unlisted kinds render plain, and files
  // without spans (unsupported language) render exactly as before.
  const KIND_GROUP: Record<string, string> = {
    keyword: 'kw',
    string: 'str',
    stringescape: 'str',
    regex: 'str',
    comment: 'com',
    doccomment: 'com',
    shebang: 'com',
    number: 'num',
    bool: 'num',
    null: 'num',
    type: 'typ',
    builtin: 'typ',
    constant: 'typ',
    tag: 'typ',
    function: 'fn',
    decorator: 'fn',
    markupheading: 'fn',
    variable: 'var',
    property: 'var',
    attribute: 'var',
    tagattribute: 'var',
    markuplink: 'var',
  };

  interface Seg {
    text: string;
    group: string;
  }

  const encoder = new TextEncoder();

  // Go byte offset -> JS string index within one line.
  function byteToChar(line: string, target: number): number {
    if (target <= 0) return 0;
    let bytes = 0;
    let i = 0;
    while (i < line.length && bytes < target) {
      const ch = String.fromCodePoint(line.codePointAt(i) ?? 0);
      bytes += encoder.encode(ch).length;
      i += ch.length;
    }
    return i;
  }

  // Clip spans to the line (mirroring the server renderer) and split into
  // plain/highlighted segments. Stale or out-of-range spans are skipped.
  function renderLine(line: string, tuples: Array<[number, number, number, string]>): Seg[] {
    if (line === '') return [{ text: '\n', group: '' }];
    if (tuples.length === 0) return [{ text: line, group: '' }];
    const lineBytes = encoder.encode(line).length;
    const sorted = [...tuples].sort((a, b) => a[1] - b[1]);
    const segs: Seg[] = [];
    let last = 0;
    for (const [, start, end, kind] of sorted) {
      if (end <= start || start < last || start >= lineBytes) continue;
      const s = byteToChar(line, start);
      const e = byteToChar(line, Math.min(end, lineBytes));
      if (s < last || e <= s) continue;
      if (s > last) segs.push({ text: line.slice(last, s), group: '' });
      segs.push({ text: line.slice(s, e), group: KIND_GROUP[kind] ?? '' });
      last = e;
    }
    if (last < line.length) segs.push({ text: line.slice(last), group: '' });
    if (segs.length === 0) segs.push({ text: line, group: '' });
    return segs;
  }

  let lines = $derived(content.split('\n'));
  let spansByLine = $derived.by(() => {
    const map = new Map<number, Array<[number, number, number, string]>>();
    for (const t of spans ?? []) {
      const arr = map.get(t[0]);
      if (arr) arr.push(t);
      else map.set(t[0], [t]);
    }
    return map;
  });
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
        <Loader label="Loading file content..." />
      {:else}
        <div class="code-row">
          <div aria-hidden="true" class="gutter">
            {#each lines as _, i (i)}
              <div class="tabular-nums">{i + 1}</div>
            {/each}
          </div>

          <pre class="code"><code>{#each lines as line, i (i)}<div class="code-line">{#each renderLine(line, spansByLine.get(i) ?? []) as seg, si (si)}<span
                      class={seg.group ? `tok-${seg.group}` : undefined}>{seg.text}</span
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
    min-width: 0;
    min-height: 0;
    display: flex;
    flex-direction: column;
    background-color: var(--code-bg);
    overflow: hidden;
    color: var(--code-text);
  }
  .preview-head {
    height: 34px;
    min-height: 34px;
    flex-shrink: 0;
    background-color: var(--code-gutter-bg);
    border-bottom: 1px solid var(--code-border);
    display: flex;
    align-items: center;
    justify-content: space-between;
    gap: 8px;
    padding: 0 10px;
    user-select: none;
    min-width: 0;
  }
  .file-id {
    display: flex;
    align-items: center;
    gap: 8px;
    min-width: 0;
    flex: 1;
    overflow: hidden;
  }
  .file-path {
    font-size: 11.5px;
    font-weight: 500;
    color: var(--code-text);
    overflow: hidden;
    text-overflow: ellipsis;
    white-space: nowrap;
    min-width: 0;
    flex: 1;
  }
  .lang-tag {
    font-size: 10px;
    color: var(--code-dim);
    background-color: rgba(128, 128, 128, 0.14);
    padding: 1px 5px;
    border-radius: 3px;
    text-transform: uppercase;
    flex-shrink: 0;
  }
  .token-count {
    font-size: 10.5px;
    color: var(--code-dim);
  }
  .mode-group {
    display: flex;
    align-items: center;
    gap: 4px;
    flex-shrink: 0;
  }
  .mode-seg {
    display: inline-flex;
    background-color: rgba(128, 128, 128, 0.12);
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
    background-color: rgba(128, 128, 128, 0.2);
    color: var(--code-text);
  }
  .mode-btn.active.sigs {
    color: var(--code-accent);
  }
  /* Single scroll container on both axes: the gutter sticks left while
     long lines scroll underneath. No nested scrollers, no drift. */
  .code-scroll {
    flex: 1;
    min-height: 0;
    overflow: auto;
    overscroll-behavior: contain;
    scrollbar-gutter: stable;
    scrollbar-width: thin;
    scrollbar-color: transparent transparent;
    font-family: var(--font-mono);
    font-size: 12px;
    line-height: 1.6;
  }
  .code-scroll:hover {
    scrollbar-color: var(--border) transparent;
  }
  .code-row {
    display: flex;
    align-items: stretch;
    width: max-content;
    min-width: 100%;
    min-height: 100%;
  }
  .gutter {
    position: sticky;
    left: 0;
    z-index: 2;
    align-self: stretch;
    padding: 10px 10px 24px 12px;
    background-color: var(--code-gutter-bg);
    border-right: 1px solid var(--code-border);
    text-align: right;
    user-select: none;
    color: var(--code-line-number);
    min-width: 44px;
    font-variant-numeric: tabular-nums;
  }
  .gutter > div {
    white-space: pre;
  }
  .gutter > div:hover {
    color: var(--code-dim);
  }
  .code {
    margin: 0;
    padding: 10px 20px 24px 14px;
    flex: 1;
    min-width: 0;
    overflow: visible;
    white-space: pre;
    tab-size: 4;
    font-variant-ligatures: none;
    color: var(--code-text);
  }
  .code-line {
    min-height: 1.6em;
    white-space: pre;
    padding-right: 8px;
    border-radius: 3px;
  }
  .code-line:hover {
    background-color: rgba(128, 128, 128, 0.09);
  }
  @media (max-width: 640px) {
    .token-count {
      display: none;
    }
  }
  @media (max-width: 480px) {
    .lang-tag {
      display: none;
    }
    .code {
      padding-right: 12px;
    }
  }
  /* Server-span token colors (--syn-* per theme in app.css). */
  .tok-kw {
    color: var(--syn-kw);
  }
  .tok-str {
    color: var(--syn-str);
  }
  .tok-com {
    color: var(--syn-com);
  }
  .tok-num {
    color: var(--syn-num);
  }
  .tok-typ {
    color: var(--syn-typ);
  }
  .tok-fn {
    color: var(--syn-fn);
  }
  .tok-var {
    color: var(--syn-var);
  }
</style>
