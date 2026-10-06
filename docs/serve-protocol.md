# serve protocol and threat model

This is the contract between the embedded Svelte app (`web/`) and the Go
server (`internal/serve/`). The frontend's `web/src/lib/api.ts` is the visual
reference; this document is normative where they differ.

## Endpoints

All `/api/*` responses are `Cache-Control: no-store`. POST/PUT bodies are
capped at 4 MiB and must be `application/json` (415 otherwise).

| Route | Auth | Notes |
| :-- | :-- | :-- |
| `GET /`, `/assets/*` | public | `index.html` is `no-cache`; hashed assets `immutable`. |
| `GET /api/meta` | public (minimal) | Always: `{version, mode, tls, authKind, authenticated}`. Authed adds `{roots, styles}`. |
| `POST /api/login`, `POST /api/logout` | public / session | `{password}` or `{token}`. 204, 401, or 429 with `retryAfter`. |
| `GET /api/recents`, `POST /api/recents`, `DELETE /api/recents?root=` | session | POST records an open (GETs never mutate). Cap 25. |
| `GET /api/browse?path=` | session | Directories only, 5000 entries, `isGitRepo` by `lstat(.git)`. |
| `GET /api/tree?root=` | session | `{path, size, tokens, language, score}` with TUI-identical token/score logic. |
| `GET /api/file?root=&path=&mode=full\|sigs` | session | Redacted. 1 MiB preview cap with `truncated`. |
| `POST /api/smart-select` | session | `{root, budget}` → `{selections: {path: full\|sigs\|skip}}`. |
| `POST /api/pack` | session | `{root, selections, budget, style, prompt, redact}` → `{document, tokens, fileCount, redactions, skipped}`. At most 2 concurrent packs (503 otherwise). |
| `GET/PUT /api/settings` | session | `{defaultStyle, defaultBudget, theme}`; theme validated `[a-z0-9-]{1,32}`. |

Failures use generic messages and status codes: 403 never reveals whether a
path exists; denied project opens log only the last two path segments.

## Auth

- **Local token:** 256-bit `crypto/rand`, base64url, printed as a URL
  fragment. Submitted once via `POST /api/login`, then cleared.
- **Password:** compared as `HMAC-SHA256(key, candidate)` vs
  `HMAC-SHA256(key, password)` with `subtle.ConstantTimeCompare`. Never
  persisted; restart-safe because there is nothing to persist.
- **Session cookie:** `v1.<expiry>.<nonce>.<HMAC>`, per-process key, 12h
  lifetime, re-issued under half-life. `HttpOnly; SameSite=Strict; Path=/`,
  `Secure` on TLS (or trusted-proxy https).
- **Throttling:** exponential per-IP backoff (first failure answers 401, the
  next waits) plus a global rate cap; 429 carries `retryAfter`. Pruned lazily.
- **Public set:** static assets, `GET /api/meta` (minimal), `POST /api/login`.
  Everything else 401s without a session (asserted by enumeration test).

## Network hygiene

- **Host allowlist** on every route (static included): bound host, loopback
  names, `--allowed-host`. Else 421 (DNS-rebinding defense).
- **Origin/fetch checks** on every non-GET: `Origin` host must equal `Host`;
  `Sec-Fetch-Site: cross-site` rejected. `OPTIONS` is 405.
- **No CORS, ever.** No `Access-Control-*` on any response (asserted).
- **Headers everywhere:** strict CSP (`script-src`/`style-src` `'self'`, no
  inline anything), `nosniff`, `no-referrer`, `DENY` framing, `same-origin`
  COOP/CORP, HSTS on TLS.
- **Limits:** 5s header / 30s read timeouts, 16 KiB headers, request-context
  cancellation into scan and render.

## Filesystem jail

One resolver (`internal/serve/jail`) serves every endpoint: absolute path →
`Clean` → `EvalSymlinks` → separator-boundary containment against
startup-resolved roots → denylist. Failures are a generic 403.

- **Denylist (any depth):** `.ssh`, `.gnupg`, `.aws`, `.kube`,
  `.config/gcloud`, `.password-store`, `.local/share/keyrings`,
  `.docker/config.json`, `.netrc`.
- Symlink-to-file inside the root reads normally; escaping links are
  skipped (walker) or 403 (API). Only regular files are ever opened.
- Project `.sift.toml` files are screened by `config.CheckProjectConfig`:
  `prompt_file` / `output` / `ui_theme_file` escapes refuse the project.
- The scan itself is read-only: hardened git (`GIT_OPTIONAL_LOCKS=0`, no
  fsmonitor/hooks/drivers, scrubbed env) plus the tree-hash proof test.

## Content safety

- Redaction defaults on for previews **and** output; token counts are
  post-redaction. Remote `redact=false` is 400 without `--allow-unredacted`.
- File content is text nodes only; downloads use fixed MIME types and
  sanitized filenames.
- Logs (`log/slog` to stderr) carry logins, refusals (trimmed paths), and
  startup facts — never tokens, passwords, cookies, or file contents.

## Idle design

No tickers, janitors, watchers, caches, or SSE. Grammars/redaction rules load
on first use. The only idle goroutines are the accept loop and per-connection
readers (asserted by the goroutine-baseline test). `--idle-timeout` exits the
process after the quiet period via a one-shot timer reset per request.
Frontend fetches on user action only (plus the login lockout countdown).
