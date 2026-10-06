/**
 * Flat-row derivation and virtual-list windowing for the file tree.
 *
 * Pure counterparts of the `flatRows`/`visibleRows` `$derived` blocks in
 * `FileTree.svelte`: same filter, token aggregation, and overscan behavior,
 * callable without a component instance.
 */

import { modeTokens } from '../../lib/selection';
import type { FileSelectionMode } from '../../lib/selection';
import type { FlatRow, TreeNode } from './tree';

export const ROW_HEIGHT = 24;
const OVERSCAN_ROWS = 10;

export interface FlattenOptions {
  root: TreeNode;
  descendants: Map<string, string[]>;
  fileByPath: Map<string, { tokens: number; path: string }>;
  selections: Record<string, FileSelectionMode>;
  collapsedDirs: Record<string, boolean>;
  filterQuery: string;
}

/** Flatten the visible hierarchy into rows honoring collapse + filter. */
export function flattenRows(opts: FlattenOptions): FlatRow[] {
  const { root, descendants, fileByPath, selections, collapsedDirs } = opts;
  const query = opts.filterQuery.trim().toLowerCase();
  const rows: FlatRow[] = [];

  const traverse = (node: TreeNode) => {
    if (node.depth === -1) {
      for (const child of node.children) traverse(child);
      return;
    }

    if (node.isDir) {
      const filePaths = descendants.get(node.path) || [];
      let dirTokens = 0;
      let selectedCount = 0;
      let matchesQuery = false;

      for (const p of filePaths) {
        const f = fileByPath.get(p);
        const mode = selections[p] || 'full';
        if (f) {
          const tokenVal = modeTokens(f.tokens, mode);
          if (mode !== 'skip') {
            dirTokens += tokenVal;
            selectedCount++;
          }
        }
        if (query && p.toLowerCase().includes(query)) matchesQuery = true;
      }

      if (query && !matchesQuery && !node.path.toLowerCase().includes(query)) return;

      const totalFiles = filePaths.length;
      const selectedState = selectedCount === 0 ? 'none' : selectedCount === totalFiles ? 'all' : 'partial';
      const isExpanded = query ? true : !collapsedDirs[node.path];

      rows.push({
        key: node.path,
        node,
        isDir: true,
        depth: node.depth,
        aggregateTokens: dirTokens,
        selectedState,
        isExpanded,
      });

      if (isExpanded) for (const child of node.children) traverse(child);
    } else {
      if (query && !node.path.toLowerCase().includes(query)) return;
      const mode = selections[node.path] || 'full';
      const fileTokens = modeTokens(node.file?.tokens || 0, mode);
      rows.push({
        key: node.path,
        node,
        isDir: false,
        depth: node.depth,
        aggregateTokens: fileTokens,
        selectedState: mode === 'skip' ? 'none' : 'all',
        isExpanded: false,
      });
    }
  };

  traverse(root);
  return rows;
}

export interface WindowSlice {
  startIndex: number;
  endIndex: number;
  visibleRows: FlatRow[];
  offsetY: number;
  totalHeight: number;
}

/** Slice the window of rows to render for the current scroll position. */
export function windowRows(rows: FlatRow[], scrollTop: number, viewportHeight: number): WindowSlice {
  const totalRows = rows.length;
  const startIndex = Math.max(0, Math.floor(scrollTop / ROW_HEIGHT) - OVERSCAN_ROWS);
  const endIndex = Math.min(totalRows, Math.ceil((scrollTop + viewportHeight) / ROW_HEIGHT) + OVERSCAN_ROWS);
  return {
    startIndex,
    endIndex,
    visibleRows: rows.slice(startIndex, endIndex),
    offsetY: startIndex * ROW_HEIGHT,
    totalHeight: totalRows * ROW_HEIGHT,
  };
}
