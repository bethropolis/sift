/**
 * Packed-document navigation for the output view.
 *
 * The server reports per-file byte ranges (`PackSection`); the renderer
 * works in lines, so this module bridges the two: line splitting, UTF-8 byte
 * offsets per line start, and offset→line binary search. Pure and
 * component-free for direct testing.
 */

export const DOC_ROW_HEIGHT = 18;
const DOC_OVERSCAN_ROWS = 20;

const encoder = new TextEncoder();

/** UTF-8 byte offset of every line start in `lines` (line i starts at out[i]). */
export function lineByteStarts(lines: string[]): number[] {
  const starts = new Array<number>(lines.length);
  let off = 0;
  for (let i = 0; i < lines.length; i++) {
    starts[i] = off;
    off += encoder.encode(lines[i]).length + 1; // + '\n'
  }
  return starts;
}

/** Line index containing byte offset `off` (clamped to range). */
export function lineForOffset(starts: number[], off: number): number {
  if (starts.length === 0) return 0;
  if (off <= 0) return 0;
  let lo = 0;
  let hi = starts.length - 1;
  while (lo < hi) {
    const mid = (lo + hi + 1) >> 1;
    if (starts[mid] <= off) lo = mid;
    else hi = mid - 1;
  }
  return lo;
}

/** First/last line touched by the byte range [start, end). */
export function sectionLineRange(
  starts: number[],
  lineCount: number,
  start: number,
  end: number,
): { startLine: number; endLine: number } {
  const startLine = lineForOffset(starts, Math.max(0, start));
  const endLine = Math.min(lineCount - 1, Math.max(startLine, lineForOffset(starts, Math.max(0, end - 1))));
  return { startLine, endLine };
}

export interface LineWindow {
  startIndex: number;
  endIndex: number;
  offsetY: number;
  totalHeight: number;
}

/** Fixed-row window slice for the virtualized document renderer. */
export function windowLines(
  lineCount: number,
  scrollTop: number,
  viewportHeight: number,
  rowHeight: number = DOC_ROW_HEIGHT,
  overscan: number = DOC_OVERSCAN_ROWS,
): LineWindow {
  const startIndex = Math.max(0, Math.floor(scrollTop / rowHeight) - overscan);
  const endIndex = Math.min(lineCount, Math.ceil((scrollTop + viewportHeight) / rowHeight) + overscan);
  return {
    startIndex,
    endIndex,
    offsetY: startIndex * rowHeight,
    totalHeight: lineCount * rowHeight,
  };
}
