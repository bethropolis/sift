/**
 * sift serve API contracts.
 *
 * Server response shapes (final forms are recorded in docs/serve-protocol.md).
 * Pure types only — no runtime code, so this module is safe to import
 * anywhere without pulling in fetch or mock state.
 */

export interface ApiMeta {
  version: string;
  styles: string[];
  mode: 'local' | 'remote';
  tls: boolean;
  roots: string[];
  /** Preferred folder-browser start dir (~/Projects when jailed, else first root). */
  defaultBrowse?: string;
  authKind: 'token' | 'password';
  authenticated: boolean;
  /** Session came from `sift serve --app`; drives Quit + the liveness stream. */
  app?: boolean;
  /**
   * Server-side feature switches (authenticated meta only). A missing key
   * means off: the UI hides entry points for features that would 404.
   */
  features?: { clone?: boolean };
}

/** Body of POST /api/clone. */
export interface CloneRequest {
  url: string;
  branch?: string;
  /** History depth; the server defaults to 1 and caps at 1000. */
  depth?: number;
}

/** One git progress update streamed while cloning. */
export interface CloneProgress {
  phase: string;
  percent: number;
}

/** A finished temporary clone (lives until the server stops or it is removed). */
export interface TempClone {
  root: string;
  name: string;
  /** Repository URL with any credentials stripped. */
  url: string;
  branch?: string;
  /** Unix millis. */
  started: number;
}

/** Clone failure; `detail` is git's own last messages when it has them. */
export interface CloneError extends Error {
  detail?: string;
}

export interface RecentProject {
  root: string;
  name: string;
  lastOpened: string;
  branch: string;
}

export interface BrowseEntry {
  name: string;
  isDir: boolean;
  isGitRepo: boolean;
  /** Last-modified time, unix millis (0 when unknown). */
  modTime: number;
}

export interface BrowseResult {
  path: string;
  parent: string | null;
  entries: BrowseEntry[];
}

export interface TreeFile {
  path: string;
  size: number;
  tokens: number;
  language: string;
  score: number; // 0 to 1 (git relevance score)
}

export interface TreeResult {
  root: string;
  files: TreeFile[];
  /** Resolved token budget (flag > .sift.toml > persisted default > builtin). */
  budget: number;
  /** Where `budget` came from; `toml` means the project's .sift.toml won. */
  budgetSource: 'toml' | 'flag' | 'default';
}

export interface FileContentResult {
  content: string;
  tokens: number;
  language: string;
  truncated: boolean;
  /**
   * Preview highlight spans from the shared server engine
   * (`internal/highlight`, same as the TUI): [line, start, end, kind]
   * tuples with byte offsets into each line. Absent for unsupported
   * languages — render plain text.
   */
  spans?: Array<[number, number, number, string]>;
}

export type SelectionMode = 'full' | 'sigs' | 'skip';

export interface SmartSelectResult {
  selections: Record<string, SelectionMode>;
}

export interface PackPayload {
  root: string;
  selections: Record<string, SelectionMode>;
  budget: number;
  style: string;
  prompt: string;
  redact: boolean;
}

export interface PackResult {
  document: string;
  tokens: number;
  fileCount: number;
  redactions: number;
  skipped: string[];
  /**
   * Per-file byte ranges into `document` for outline navigation
   * (XML/Markdown/Plain; empty when no files are kept). `start` is the
   * first byte of the file's block, `end` one past its last byte.
   */
  sections: PackSection[];
}

export interface PackSection {
  path: string;
  tokens: number;
  start: number;
  end: number;
}

export interface SettingsData {
  defaultStyle: string;
  defaultBudget: number;
  theme: string;
  showHidden: boolean;
  fileSort: 'name' | 'updated';
}

export interface LoginError extends Error {
  retryAfter?: number;
}
