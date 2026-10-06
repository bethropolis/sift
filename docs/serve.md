# `sift serve`: the browser file picker

`sift serve` starts a local web server for an embedded browser app: a visual
alternative to the TUI picker (`sift pick`). Open any project under the
allowed roots, pick files with per-file `FULL` / `SIGS` / `SKIP` modes, watch
the live token budget, run smart-select, then generate, copy, or download the
context document.

```bash
sift serve                        # http://127.0.0.1:7777, token printed on stderr
sift serve --open                 # also open the browser
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
