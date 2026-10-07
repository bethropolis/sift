/**
 * Custom context menu for the sift UI: editing commands inside text fields,
 * Copy for a document selection, and window navigation (Back / Forward /
 * Reload) on plain content, so every right-click resolves to something.
 * Paste goes through the async clipboard API, so the browser shows its
 * one-time permission grant instead of failing silently.
 */

export interface MenuItem {
  id: string;
  label: string;
  /** Keyboard hint shown right-aligned, e.g. "Ctrl+C". */
  hint: string;
  disabled: boolean;
  run: () => void | Promise<void>;
}

function isTextField(el: HTMLElement | null): el is HTMLInputElement | HTMLTextAreaElement {
  if (!el) return false;
  if (el.tagName === 'TEXTAREA') return true;
  if (el.tagName !== 'INPUT') return false;
  switch ((el as HTMLInputElement).type) {
    case 'text':
    case 'password':
    case 'search':
    case 'number':
    case 'url':
    case 'tel':
      return true;
    default:
      return false;
  }
}

/** Selected text inside a field; empty when collapsed or unreadable. */
function fieldSelection(el: HTMLInputElement | HTMLTextAreaElement): string {
  try {
    const s = el.selectionStart;
    const e = el.selectionEnd;
    if (s === null || e === null || e <= s) return '';
    return el.value.slice(s, e);
  } catch {
    // Number inputs (and friends) throw on selection access in some engines.
    return '';
  }
}

/** Replace the field's selection (or insert at the caret) and notify Svelte. */
function replaceFieldSelection(el: HTMLInputElement | HTMLTextAreaElement, text: string): void {
  const s = el.selectionStart ?? el.value.length;
  const e = el.selectionEnd ?? el.value.length;
  el.value = el.value.slice(0, s) + text + el.value.slice(e);
  el.selectionStart = el.selectionEnd = s + text.length;
  el.dispatchEvent(new Event('input', { bubbles: true }));
}

async function writeText(text: string): Promise<void> {
  await navigator.clipboard.writeText(text);
}

/**
 * Build the menu for a right-click target. Always returns at least the
 * window-navigation items, so every right-click resolves to something.
 */
export function buildContextMenuItems(target: EventTarget | null): MenuItem[] {
  const el = target instanceof HTMLElement ? target : null;
  if (isTextField(el)) {
    const sel = fieldSelection(el);
    const readOnly = el.readOnly;
    const canPaste = !readOnly && !!navigator.clipboard?.readText;
    return [
      {
        id: 'cut',
        label: 'Cut',
        hint: 'Ctrl+X',
        disabled: sel.length === 0 || readOnly,
        run: async () => {
          await writeText(sel);
          replaceFieldSelection(el, '');
        },
      },
      {
        id: 'copy',
        label: 'Copy',
        hint: 'Ctrl+C',
        disabled: sel.length === 0,
        run: () => writeText(sel),
      },
      {
        id: 'paste',
        label: 'Paste',
        hint: 'Ctrl+V',
        disabled: !canPaste,
        run: async () => {
          const text = await navigator.clipboard.readText();
          if (text) {
            el.focus();
            replaceFieldSelection(el, text);
          }
        },
      },
      {
        id: 'select-all',
        label: 'Select All',
        hint: 'Ctrl+A',
        disabled: readOnly && el.value.length === 0,
        run: () => {
          el.focus();
          el.select();
        },
      },
    ];
  }
  const sel = window.getSelection()?.toString() ?? '';
  if (sel.length > 0) {
    return [
      {
        id: 'copy',
        label: 'Copy',
        hint: 'Ctrl+C',
        disabled: false,
        run: () => writeText(sel),
      },
    ];
  }
  // Plain content: window navigation, the useful remainder of the native
  // menu in a chromeless window (and harmless redundancy in a tab).
  return [
    {
      id: 'back',
      label: 'Back',
      hint: 'Alt+←',
      disabled: false,
      run: () => history.back(),
    },
    {
      id: 'forward',
      label: 'Forward',
      hint: 'Alt+→',
      disabled: false,
      run: () => history.forward(),
    },
    {
      id: 'reload',
      label: 'Reload',
      hint: 'Ctrl+R',
      disabled: false,
      run: () => location.reload(),
    },
  ];
}
