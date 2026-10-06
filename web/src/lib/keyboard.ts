/**
 * Shared keyboard helpers for routes and modals.
 *
 * All handlers bail when focus is in an input/textarea/select or a
 * contentEditable node, so typing never triggers shortcuts.
 */

/** True when the event target is an editable element (shortcuts must yield). */
export function isEditingTarget(e: KeyboardEvent): boolean {
  const target = e.target as HTMLElement | null;
  if (!target || typeof target.tagName !== 'string') return false;
  return (
    target.tagName === 'INPUT' ||
    target.tagName === 'TEXTAREA' ||
    target.tagName === 'SELECT' ||
    target.isContentEditable
  );
}

/** True for a plain keypress (no Cmd/Ctrl modifier). */
export function isPlainKey(e: KeyboardEvent, key: string): boolean {
  return e.key === key && !e.metaKey && !e.ctrlKey;
}
