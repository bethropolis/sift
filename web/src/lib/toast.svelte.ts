/**
 * App-wide toast store. Any module can call `toast(...)`; one <ToastHost>
 * (mounted in App.svelte) renders them. View state only — never persisted
 * (see lib/persist.ts for the storage rule).
 *
 *   const id = toast({ message: 'Generating…', id: 'gen' });
 *   toast.update(id, { message: 'Done', kind: 'success' });
 *   toast.dismiss(id);
 *
 * Passing an `id` that is already showing replaces that toast in place
 * instead of stacking a duplicate. Timers are chained one-shot timeouts (no
 * repeating timers, per the CI hygiene gate), and idle costs nothing.
 */

export type ToastKind = 'info' | 'success' | 'error';

export interface ToastAction {
  label: string;
  run: () => void;
}

export interface ToastInput {
  message: string;
  kind?: ToastKind;
  /** Milliseconds on screen; 0 keeps it until dismissed. */
  duration?: number;
  /** Stable key: re-using one updates the existing toast. */
  id?: string;
  action?: ToastAction;
}

export interface ToastItem {
  id: string;
  message: string;
  kind: ToastKind;
  duration: number;
  action?: ToastAction;
}

/** Newest toasts beyond this drop the oldest, so a burst never walls the UI. */
export const MAX_TOASTS = 3;

const DEFAULT_DURATION: Record<ToastKind, number> = {
  info: 3500,
  success: 3500,
  // Errors linger: they usually need to be read, not glanced at.
  error: 6000,
};

export const toasts = $state<{ items: ToastItem[] }>({ items: [] });

const timers = new Map<string, ReturnType<typeof setTimeout>>();
let seq = 0;

function clearTimer(id: string): void {
  const t = timers.get(id);
  if (t !== undefined) {
    clearTimeout(t);
    timers.delete(id);
  }
}

function arm(item: ToastItem): void {
  clearTimer(item.id);
  if (item.duration <= 0) return;
  timers.set(
    item.id,
    setTimeout(() => dismiss(item.id), item.duration),
  );
}

function dismiss(id: string): void {
  clearTimer(id);
  toasts.items = toasts.items.filter((t) => t.id !== id);
}

function update(id: string, patch: Partial<Omit<ToastInput, 'id'>>): void {
  const current = toasts.items.find((t) => t.id === id);
  if (!current) return;
  const kind = patch.kind ?? current.kind;
  const next: ToastItem = {
    ...current,
    ...patch,
    kind,
    // A kind change without an explicit duration picks up that kind's default
    // ("Generating…" sticky → "Done" auto-dismiss).
    duration: patch.duration ?? (patch.kind ? DEFAULT_DURATION[kind] : current.duration),
  };
  toasts.items = toasts.items.map((t) => (t.id === id ? next : t));
  arm(next);
}

/** Show a toast; returns its id for update/dismiss. */
function show(input: ToastInput): string {
  const kind = input.kind ?? 'info';
  const id = input.id ?? `toast-${++seq}`;
  const item: ToastItem = {
    id,
    message: input.message,
    kind,
    duration: input.duration ?? DEFAULT_DURATION[kind],
    action: input.action,
  };
  const exists = toasts.items.some((t) => t.id === id);
  if (exists) {
    toasts.items = toasts.items.map((t) => (t.id === id ? item : t));
  } else {
    let next = [...toasts.items, item];
    while (next.length > MAX_TOASTS) {
      const dropped = next.shift();
      if (dropped) clearTimer(dropped.id);
    }
    toasts.items = next;
  }
  arm(item);
  return id;
}

/** Hold a toast open while the pointer is over it. */
function pause(id: string): void {
  clearTimer(id);
}

/** Restart the full duration after the pointer leaves. */
function resume(id: string): void {
  const item = toasts.items.find((t) => t.id === id);
  if (item) arm(item);
}

export const toast = Object.assign(show, { update, dismiss, pause, resume });
