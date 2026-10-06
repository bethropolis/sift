// Package serve implements `sift serve`: a local web server for the embedded
// Svelte single-page app, a browser alternative to the TUI picker.
//
// Security is load-bearing here, not layered on. The rules, in brief:
//   - loopback by default; anything else demands --allow-remote plus a full
//     credential/TLS/host/root configuration, refused before binding;
//   - token (local) or password (remote) auth with stateless signed cookies;
//   - every client path is jailed to configured roots with symlink
//     containment and a credential-directory denylist;
//   - redaction defaults on and covers previews and output;
//   - no CORS, strict security headers, Host/Origin/fetch-metadata checks;
//   - idle means zero: no tickers, watchers, caches, or polling.
package serve
