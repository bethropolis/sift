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
| `GET /api/meta` | public (minimal) | Always: `{version, mode, tls, authKind, authenticated}`. Authed adds `{roots, styles, defaultBrowse}` (`~/Projects` when jailed, else first root) and `{features: {clone}}` (true only with `--allow-clone` and git installed); the UI hides clone entry points when false. |
| `POST /api/login`, `POST /api/logout` | public / session | `{password}` or `{token}`. 204, 401, or 429 with `retryAfter`. |
| `GET /api/recents`, `POST /api/recents`, `DELETE /api/recents?root=` | session | POST records an open (GETs never mutate). Cap 25. |
| `GET /api/browse?path=` | session | Directories only, 5000 entries, `isGitRepo` by `lstat(.git)`. Dot-directories hidden unless `&hidden=1`. Entries carry `modTime` (unix millis) for sort-by-updated. |
| `GET /api/tree?root=` | session | `{path, size, tokens, language, score}` with TUI-identical token/score logic. Plus `budget` + `budgetSource` (`toml`/`flag`/`default`): the resolved token budget the client displays and echoes back (see Budget below). |
| `GET /api/file?root=&path=&mode=full\|sigs` | session | Redacted. 1 MiB preview cap with `truncated`. Plus `spans`: `[line, start, end, kind]` tuples (byte offsets per line) from the shared `internal/highlight` engine — the same spans the TUI preview renders. Kinds: `keyword string stringescape regex comment doccomment shebang number bool null type builtin constant tag function decorator markupheading variable property attribute tagattribute markuplink`; anything else renders plain. Spans are parsed on the exact redacted bytes sent, so offsets always line up; absent for unsupported languages. |
| `POST /api/smart-select` | session | `{root, budget}` → `{selections: {path: full\|sigs\|skip}}`. |
| `POST /api/pack` | session | `{root, selections, budget, style, prompt, redact}` → `{document, tokens, fileCount, redactions, skipped, sections}`. At most 2 concurrent packs (503 otherwise. `sections` is always an array of `{path, tokens, start, end}` byte ranges into `document` (XML/Markdown/Plain) for outline navigation; empty when no files are kept. |
| `GET/PUT /api/settings` | session | `{defaultStyle, defaultBudget, theme, showHidden, fileSort}`; theme validated `[a-z0-9-]{1,32}` (`system` follows the OS), `fileSort` is `name` or `updated`. The theme id is shared with the TUI (mirrored to the legacy field), so changing it in either frontend changes it in both. `defaultBudget` seeds fresh workspaces (see Budget). |
| `POST /api/session/launch` | public (token is the credential) | `{token}` → `{app}`. `--app` auto-login: one-time, 60s TTL, loopback-only. Shares the login lockout. Any failure is the same generic `401`. |
| `GET /api/app/heartbeat` | session | SSE liveness stream, app mode only (`404` otherwise). No server-side ticker; the handler blocks until the client leaves or the server starts shutting down. |
| `POST /api/clone` | session | `{url, branch?, depth?}` → NDJSON progress lines (`{"type":"progress","phase","percent"}`), then one `done` (`{root, name, url}`) or `error` (`{message, detail?}`). 404 without `--allow-clone` (or without git). Rejects local paths, `file://`, `ext::`, leading `-`, URLs with embedded passwords, and depths outside 1–1000. One clone at a time, 5-minute timeout; aborting the request cancels git and removes the checkout. |
| `GET /api/clones` | session | This session's temporary clones, newest first (`{root, name, url, branch?, started}`). Same 404 gate. |
| `DELETE /api/clone?root=` | session | Removes one server-created clone (directory deleted, jail root withdrawn). Anything else is 404 and untouched. Same 404 gate. |

## Budget

One chain, server and client agree: explicit request `budget`
(pack/smart-select body) > serve `--budget` flag > project `.sift.toml`
(`[sift] budget`) > global config file > persisted `defaultBudget`
(preferences) > builtin 64000. The `.sift.toml` always beats the persisted
default so per-project files stay authoritative; the web client still
overrides per session via the TopBar presets (badged `.sift.toml` when the
file won) and persists preset picks back to `defaultBudget` only when no
`.sift.toml`/flag budget is set. A literal `0` request budget keeps endpoint
semantics (pack: no trim; smart-select: config budget).

Failures use generic messages and status codes: 403 never reveals whether a
path exists; denied project opens log only the last two path segments.

## Auth

- **Local token:** 256-bit `crypto/rand`, base64url, printed as a URL
  fragment. Submitted once via `POST /api/login`, then cleared.
- **Password:** compared as `HMAC-SHA256(key, candidate)` vs
  `HMAC-SHA256(key, password)` with `subtle.ConstantTimeCompare`. Never
  persisted; restart-safe because there is nothing to persist.
- **Session cookie:** `v1.<expiry>.<app>.<nonce>.<HMAC>`, per-process key, 12h
  lifetime, re-issued under half-life. `app` is `1` for a session created by a
  `--app` launch token, so app mode survives reloads without server state.
  `HttpOnly; SameSite=Strict; Path=/`, `Secure` on TLS (or trusted-proxy https).
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
- **Headers everywhere:** strict CSP (`script-src 'self'`; `style-src 'self'`
  plus one `sha256` hash for the inline first-paint `<style>` in
  `web/index.html`, pinned by `TestFirstPaintStyleHash`; `style-src-elem`
  repeats both, and `style-src-attr 'unsafe-inline'` allows Svelte's runtime
  style bindings, which cannot execute script), `nosniff`, `no-referrer`,
  `DENY` framing, `same-origin` COOP/CORP, HSTS on TLS.
- **Limits:** 5s header / 30s read timeouts, 16 KiB headers, request-context
  cancellation into scan and render.
- **Re-attach (loopback only):** on a bind conflict with `--open`/`--app`, the
  new process probes `GET /api/meta` (public version facts, no credentials)
  and, only for a matching sift shape, opens the plain base URL. It never
  learns the existing token, mints no launch token, and arms no shutdown.

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
- Every request resolves its engine config like the CLI inside the target
  directory (defaults → global file → that project's `.sift.toml`); the
  server's startup directory never leaks into other projects. Explicit serve
  flags overlay the project file, exactly like CLI flag precedence. Output
  sinks stay disabled.
- The scan itself is read-only: hardened git (`GIT_OPTIONAL_LOCKS=0`, no
  fsmonitor/hooks/drivers, scrubbed env) plus the tree-hash proof test.

## Clone

Cloning is the one deliberate exception to read-only: the server runs
`git clone` on behalf of the session (shared `internal/gitclone` package with
the `sift clone` CLI, same validation).

- **URLs:** remote only (`https`, `http`, `ssh`, `git`, scp-like
  `user@host:path`). Local paths and `file://` are refused in both the CLI
  and the server; `ext::<helper>` remotes, leading-`-` arguments, and URLs
  with embedded `user:password@` are refused too. The URL always follows `--`.
- **Git environment:** `GIT_ALLOW_PROTOCOL=http:https:ssh:git`,
  `GIT_TERMINAL_PROMPT=0`, a detached session (no terminal prompts), plus the
  scanner's hardened git flags. Auth is whatever the system git already does
  (credential helpers, `~/.gitconfig`, ssh agent); nothing credential-shaped
  crosses the browser. Private repos that need a login fail fast with git's
  own message.
- **Checkouts:** private `sift-clone-*` temp dirs holding one repo dir each,
  deleted on every shutdown path (signal, idle timeout, window closed) and by
  the OS temp cleaner after a crash. At most 5 per session, one clone at a
  time, 5-minute timeout, no submodules, `--depth` 1–1000 (default 1).
- **Jail:** each finished checkout is registered as an extra jail root scoped
  to exactly that directory (siblings and the temp parent stay 403); removal
  withdraws it. The startup roots shown in the UI never change, and the
  denylist applies beneath clone roots too. A cloned repo's own `.sift.toml`
  is screened at clone time and refused with its checkout removed.
- **Not recents:** clones are listed per session (`GET /api/clones`) and open
  like projects, but never enter the on-disk recents or the resume target —
  both would dangle after the server stops.
- **Availability:** strictly opt-in via `--allow-clone` on every bind
  (plus system git). Without the flag the routes are 404 and
  `meta.features.clone` is false. Clone URLs are logged credential-stripped.

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
process after the quiet period via a one-shot timer reset per request; a clone
in flight postpones the exit until it finishes (its request began long ago),
and a long clone resets the timer when it ends. Frontend fetches on user action only (plus the login lockout countdown).
