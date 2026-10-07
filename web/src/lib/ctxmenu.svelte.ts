import type { MenuItem } from './contextmenu';

export interface CtxMenuState {
  x: number;
  y: number;
  items: MenuItem[];
  /** Element that spawned the menu; focus returns here on Esc/activate. */
  opener: HTMLElement | null;
}

/**
 * The single open context menu (or null). Module-level so any surface —
 * the global fallback in App, the file explorer, future callers — opens
 * the same menu instead of prop-drilling callbacks. View state only, never
 * persisted (see lib/persist.ts for the storage rule).
 */
export const ctxMenuState = $state<{ current: CtxMenuState | null }>({ current: null });

export function openCtxMenu(x: number, y: number, items: MenuItem[], opener: HTMLElement | null): void {
  ctxMenuState.current = { x, y, items, opener };
}

export function closeCtxMenu(): void {
  ctxMenuState.current = null;
}
