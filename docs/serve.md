# `sift serve`: the browser file picker

`sift serve` starts a local web server for an embedded browser app: a visual
alternative to the TUI picker (`sift pick`). Open any project under the
allowed roots, pick files with per-file `FULL` / `SIGS` / `SKIP` modes, watch
the live token budget, run smart-select, then generate, copy, or download the
context document.

```bash
sift serve                        # http://127.0.0.1:7777, token printed on stderr
sift serve --open                 # also open the browser
sift serve --app                  # chromeless app window; server exits with it
sift serve --root ~/code          # restrict browsable projects
sift serve --idle-timeout 30m     # exit after 30 minutes without requests
```

## Local use

Local mode is the default: loopback-only bind, a generated 256-bit token,
default roots of `$HOME` minus credential directories. The token prints as a
`#/login?token=...` URL — the fragment never reaches the server, and the app
clears it after login. Sessions last 12 hours and are signed with a
per-process key, so restarting the server logs everyone out.

Recents, default style/budget, and the theme are shared with the TUI through
`~/.config/sift/` — set them in either surface.

## App mode

`sift serve --app` opens the UI in a chromeless browser window (Chromium's
`--app` mode) instead of a tab, logs it in automatically, and stops the server
when that window goes away — so a desktop app never lingers as a headless
daemon.

```bash
sift serve --app                    # app window; server exits with the window
sift serve --app --keep-alive       # keep serving after the window closes
SIFT_APP_BROWSER=/usr/bin/chromium sift serve --app   # explicit browser
```

- **Browsers:** Chromium family only (Chrome, Chromium, Edge, Brave). Firefox
  and Safari have no app mode and take the fallback path.
- **Auto-login:** the window receives a one-time token in the URL fragment
  (`#launch=…`) and exchanges it for a normal session. The token is single-use
  and expires after 60s; it is never written to logs.
- **Shutdown:** closing the window stops the server after ~15s.
- **`--keep-alive`** disables that shutdown, leaving a plain server behind.
- **Fallbacks, none fatal:** no Chromium found → your default browser (still
  auto-logged in, but no auto-shutdown); no display (SSH) →
  a warning and the printed URL.
- **Loopback only:** `--app` refuses a non-loopback `--listen`, since it would
  hand an auto-login token to a network-reachable listener.
- **Right-click menu:** a minimal custom menu everywhere in the UI
  (Cut/Copy/Paste/Select All in fields, Copy for selections,
  Back/Forward/Reload on plain content). Inspect stays on F12.
- **Re-attach:** if the port is already held by a live sift server (loopback
  only), `--open`/`--app` open a window on that server instead of failing —
  no new server, no new port. The window gets the plain URL (no auto-login;
  use the token from the original terminal), and closing it never stops a
  server it didn't start. Anything else on the port keeps the hard bind error.

The window keeps the server alive with a single long-lived connection; the
server sends nothing on a timer, so an idle app window costs no work. After a
laptop sleep the UI shows a brief "reconnecting" banner and retries by itself,
or the ended screen if the server is really gone.

## Clone a repository

The Projects page has a **Clone** button, and pasting a git URL into its
search box offers a "Clone …" row (Enter starts it). The dialog takes a URL,
an optional branch or tag, and a history depth (default 1, like
`sift clone`).

- **Temporary by design:** the checkout lives in a private temp directory and
  opens straight in the workspace, marked with a `temp` pill. It is removed
  when sift stops (or with its Remove button) and never lands in recents or
  the Resume target, so nothing points at a deleted directory.
- **Progress and cancel:** the dialog shows git's own progress with a Cancel
  button. Cancelling stops the clone and deletes the half-made checkout.
- **Your git, your credentials:** cloning runs the system `git`, so SSH keys,
  credential helpers, and `~/.gitconfig` work as in a terminal. A repo that
  needs a login you haven't set up fails with git's message (plus a hint for
  private repos). Only remote URLs are accepted — local paths and `file://`
  are refused.
- **Availability:** strictly opt-in with `sift serve --allow-clone` (and
  system git installed). Without the flag the button and the search-box offer
  are hidden entirely — on loopback binds too.

## Remote use

Remote mode is deliberately strict. Any non-loopback bind (including
`0.0.0.0`) **refuses to start** unless all of these hold:

- `--allow-remote`
- a password of at least 12 characters (`--password-file`, never a flag
  value, or `SIFT_SERVE_PASSWORD`; the file must be `chmod 600`)
- at least one explicit `--root`
- at least one `--allowed-host`
- exactly one of `--tls-cert`/`--tls-key`, `--tls-self-signed`,
  `--behind-proxy`, `--insecure-http`
- `--allow-clone` to enable cloning repositories (off by default on every
  bind; needs system git too)

The cheap, safe remote recipe is an SSH tunnel — no remote flags at all:

```bash
# on the remote machine
sift serve --root ~/code
# on your laptop
ssh -N -L 7777:127.0.0.1:7777 you@remote
# open http://127.0.0.1:7777 (token from the remote terminal)
```

Native remote example (self-signed TLS):

```bash
sift serve --listen 0.0.0.0:7777 --allow-remote \
  --password-file ~/.config/sift/serve-password \
  --root ~/code --allowed-host code.example.com \
  --tls-self-signed
```

Verify the printed SHA-256 fingerprint out of band on first connect.
`--insecure-http` works but prints a loud warning and the UI shows a
persistent "connection is not encrypted" banner; prefer the tunnel.

## Reference

- [serve protocol and threat model](serve-protocol.md) — endpoints, auth,
  jail rules, and the security tests.
- `sift serve --help` — every flag.
