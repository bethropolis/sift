/**
 * File-tree hierarchy types and pure builders.
 *
 * `TreeNode`/`FlatRow` were inline in `FileTree.svelte`; `buildTree` and
 * `collectDescendants` are the hierarchy halves of its `$derived` blocks,
 * extracted verbatim so virtual-list math can be unit-checked without Svelte.
 */

import type { TreeFile } from '../../api-client/types';

export interface TreeNode {
  name: string;
  path: string;
  isDir: boolean;
  file?: TreeFile;
  children: TreeNode[];
  depth: number;
}

export type RowSelectionState = 'all' | 'none' | 'partial';

export interface FlatRow {
  key: string;
  node: TreeNode;
  isDir: boolean;
  depth: number;
  aggregateTokens: number;
  selectedState: RowSelectionState;
  isExpanded: boolean;
  /** Directory only: every descendant file is in sigs mode. */
  allSigs: boolean;
  /** Fuzzy-filter hit indices into node.name; null when unfiltered/unmatched. */
  match: number[] | null;
}

/** Build the directory hierarchy from a flat file list (sorted by path). */
export function buildTree(files: TreeFile[]): TreeNode {
  const root: TreeNode = { name: '', path: '', isDir: true, children: [], depth: -1 };
  const dirMap = new Map<string, TreeNode>();
  dirMap.set('', root);
  const sortedFiles = [...files].sort((a, b) => a.path.localeCompare(b.path));

  for (const file of sortedFiles) {
    const parts = file.path.split('/');
    let currentPath = '';
    let parentNode = root;

    for (let i = 0; i < parts.length - 1; i++) {
      const dirName = parts[i];
      currentPath = currentPath ? `${currentPath}/${dirName}` : dirName;
      let dirNode = dirMap.get(currentPath);
      if (!dirNode) {
        dirNode = { name: dirName, path: currentPath, isDir: true, children: [], depth: i };
        dirMap.set(currentPath, dirNode);
        parentNode.children.push(dirNode);
      }
      parentNode = dirNode;
    }

    parentNode.children.push({
      name: parts[parts.length - 1],
      path: file.path,
      isDir: false,
      file,
      children: [],
      depth: parts.length - 1,
    });
  }
  // Directories first, alphabetical within each kind (every level).
  const sortKids = (node: TreeNode): void => {
    node.children.sort(
      (a, b) => Number(b.isDir) - Number(a.isDir) || a.name.localeCompare(b.name),
    );
    for (const child of node.children) if (child.isDir) sortKids(child);
  };
  sortKids(root);
  return root;
}

/** Map every directory path to its descendant file paths. */
export function collectDescendants(root: TreeNode): Map<string, string[]> {
  const map = new Map<string, string[]>();
  const traverse = (node: TreeNode): string[] => {
    if (!node.isDir) return node.path ? [node.path] : [];
    const allFiles: string[] = [];
    for (const child of node.children) allFiles.push(...traverse(child));
    if (node.path) map.set(node.path, allFiles);
    return allFiles;
  };
  traverse(root);
  return map;
}
