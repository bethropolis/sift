/**
 * sift serve API client. UI work can use the in-file mock with
 * VITE_MOCK=true; every real build (`just web`, release, `just web-dev`)
 * sets VITE_MOCK=false to talk to the Go server at /api.
 *
 * Barrel: keeps `import { api } from '../lib/api'` working. Types live in
 * `./types`, the live `fetch` client in `./client`, the mock in
 * `./mockApi`, shared HTTP helpers in `./live`, and mock state in `./mock`.
 */

import {
  browse as liveBrowse,
  cloneRepo as liveCloneRepo,
  deleteClone as liveDeleteClone,
  deleteRecent as liveDeleteRecent,
  getClones as liveGetClones,
  getFile as liveGetFile,
  getMeta as liveGetMeta,
  getRecents as liveGetRecents,
  getSettings as liveGetSettings,
  getTree as liveGetTree,
  launchSession as liveLaunchSession,
  login as liveLogin,
  loginWithToken as liveLoginWithToken,
  logout as liveLogout,
  pack as livePack,
  recordRecent as liveRecordRecent,
  saveSettings as liveSaveSettings,
  smartSelect as liveSmartSelect,
} from './client';
import {
  browse as mockBrowse,
  cloneRepo as mockCloneRepo,
  deleteClone as mockDeleteClone,
  deleteRecent as mockDeleteRecent,
  getClones as mockGetClones,
  getFile as mockGetFile,
  getMeta as mockGetMeta,
  getRecents as mockGetRecents,
  getSettings as mockGetSettings,
  getTree as mockGetTree,
  launchSession as mockLaunchSession,
  login as mockLogin,
  loginWithToken as mockLoginWithToken,
  logout as mockLogout,
  pack as mockPack,
  recordRecent as mockRecordRecent,
  saveSettings as mockSaveSettings,
  smartSelect as mockSmartSelect,
} from './mockApi';
import type {
  BrowseResult,
  FileContentResult,
  PackPayload,
  PackResult,
  PackSection,
  RecentProject,
  SettingsData,
  SmartSelectResult,
  TreeResult,
} from './types';

// Live by default in real builds; only an explicit VITE_MOCK=true (plain
// `bun run dev` / `bunx vite build`) keeps the mock for isolated UI work.
export const USE_MOCK = import.meta.env.VITE_MOCK === 'true';

const pick = <L, M>(live: L, mock: M): L | M => (USE_MOCK ? mock : live);

export const api = {
  getMeta: pick(liveGetMeta, mockGetMeta),
  login: pick(liveLogin, mockLogin),
  /** Token login (local mode): the token travels in the POST body, never in a query string. */
  loginWithToken: pick(liveLoginWithToken, mockLoginWithToken),
  logout: pick(liveLogout, mockLogout),
  getRecents: pick(liveGetRecents, mockGetRecents),
  recordRecent: pick(liveRecordRecent, mockRecordRecent),
  deleteRecent: pick(liveDeleteRecent, mockDeleteRecent),
  browse: pick(liveBrowse, mockBrowse),
  getTree: pick(liveGetTree, mockGetTree),
  getFile: pick(liveGetFile, mockGetFile),
  smartSelect: pick(liveSmartSelect, mockSmartSelect),
  pack: pick(livePack, mockPack),
  getSettings: pick(liveGetSettings, mockGetSettings),
  saveSettings: pick(liveSaveSettings, mockSaveSettings),
  /** App-mode auto-login (sift serve --app). */
  launchSession: pick(liveLaunchSession, mockLaunchSession),
  /** Clone a repo into a server-side temp dir (streams progress; abortable). */
  cloneRepo: pick(liveCloneRepo, mockCloneRepo),
  getClones: pick(liveGetClones, mockGetClones),
  deleteClone: pick(liveDeleteClone, mockDeleteClone),
};

// Re-export contracts so callers keep importing from '../lib/api'.
export type {
  ApiMeta,
  BrowseEntry,
  BrowseResult,
  CloneError,
  CloneProgress,
  CloneRequest,
  FileContentResult,
  PackPayload,
  PackResult,
  PackSection,
  RecentProject,
  SelectionMode,
  SettingsData,
  SmartSelectResult,
  TempClone,
  TreeFile,
  TreeResult,
  LoginError,
} from './types';
