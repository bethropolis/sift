/**
 * Flat-row derivation and virtual-list windowing for the file tree.
 *
 * Pure counterparts of the `flatRows`/`visibleRows` `$derived` blocks in
 * `FileTree.svelte`: same filter, token aggregation, and overscan behavior,
 * callable without a component instance.
 */

import { modeTokens } from '../../lib/selection';
import type { FileSelectionMode } from '../../lib/selection';
import { fuzzyMatch, nameMatchIndices } from '../../lib/filter';
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
  const query = opts.filterQuery.trim();
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
      let sigsCount = 0;
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
          if (mode === 'sigs') sigsCount++;
        }
        if (query && !matchesQuery && fuzzyMatch(p, query)) matchesQuery = true;
      }

      const selfHit = query ? fuzzyMatch(node.path, query) : null;
      if (query && !matchesQuery && !selfHit) return;

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
        allSigs: selectedState === 'all' && sigsCount === totalFiles,
        match: selfHit ? nameMatchIndices(node.path, node.name, selfHit.indices) : null,
      });

      if (isExpanded) for (const child of node.children) traverse(child);
    } else {
      const hit = query ? fuzzyMatch(node.path, query) : null;
      if (query && !hit) return;
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
        allSigs: false,
        match: hit ? nameMatchIndices(node.path, node.name, hit.indices) : null,
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
