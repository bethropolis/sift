/**
 * Back-compat shim: `import ... from '../lib/api'` keeps working.
 * Real modules live in `./api-client/` (types, client, mockApi, live, index).
 */
export * from './api-client/index';
