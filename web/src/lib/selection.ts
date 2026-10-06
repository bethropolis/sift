/**
 * File selection modes shared by the workspace store and the file tree.
 *
 * `full` = include whole file, `sigs` = signatures only (~22% of tokens),
 * `skip` = exclude. The 0.22 factor mirrors the Go compressor's average
 * yield and must stay in sync with `FileTree` display math.
 */

export type FileSelectionMode = 'full' | 'sigs' | 'skip';

/** Persisted form used by the pack API (identical values, distinct name). */
export type ApiSelectionMode = FileSelectionMode;

/** Cycle Full → Sigs → Skip → Full (the `f` key and row mode button). */
export function cycleMode(current: FileSelectionMode): FileSelectionMode {
  if (current === 'full') return 'sigs';
  if (current === 'sigs') return 'skip';
  return 'full';
}

/** Toggle selection: any active mode → skip, skip → full (Space key). */
export function toggleMode(current: FileSelectionMode): FileSelectionMode {
  return current === 'skip' ? 'full' : 'skip';
}

/** Effective token cost of a file under a mode (sigs ≈ 22% of full). */
export function modeTokens(fullTokens: number, mode: FileSelectionMode): number {
  if (mode === 'skip') return 0;
  if (mode === 'sigs') return Math.floor(fullTokens * 0.22);
  return fullTokens;
}

/** True when the mode contributes to the packed document. */
export function isIncluded(mode: FileSelectionMode): boolean {
  return mode !== 'skip';
}
