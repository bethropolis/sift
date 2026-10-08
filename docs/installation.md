# Installation

Sift ships as a single self-contained binary with no runtime dependencies.
Choose the path that fits your platform.

## Requirements

- **Runtime:** any supported operating system below. The prebuilt binaries need
  nothing else installed.
- **To build from source:** [Go](https://golang.org/dl/) 1.26 or newer.
- Signature compression (`--mode signatures`) is included in the prebuilt
  release binaries. Building from source enables it when cgo is available.

## macOS and Linux (curl)

```sh
curl -fsSL https://bethropolis.github.io/sift/install.sh | sh
```

The installer places `sift` in `~/.local/bin`. Set `SIFT_INSTALL_DIR` to choose
a different destination:

```sh
SIFT_INSTALL_DIR="$HOME/bin" curl -fsSL https://bethropolis.github.io/sift/install.sh | sh
```

If `~/.local/bin` is not already on your `PATH`, add it to your shell profile:

```sh
export PATH="$HOME/.local/bin:$PATH"
```

## Homebrew

```sh
brew install bethropolis/tap/sift
```

## Windows (Scoop)

```powershell
scoop bucket add bethropolis https://github.com/bethropolis/scoop-bucket
scoop install sift
```

## FreeBSD and other release archives

Download a prebuilt archive for your operating system and architecture from
the [latest GitHub release](https://github.com/bethropolis/sift/releases/latest).
Archives are produced for Linux, Windows, macOS, and FreeBSD on `amd64`,
`arm64`, `386`, and `armv7` where supported by Go.

## Build from source

Clone the repository and run the local installer, which builds the checkout you
are in and installs to `~/.local/bin` (override with `SIFT_INSTALL_DIR`):

```sh
git clone https://github.com/bethropolis/sift.git
cd sift
./scripts/install.sh
```

Or build manually:

```sh
go build -o "$HOME/.local/bin/sift" ./cmd/sift
```

During development the [`justfile`](../justfile) provides `just build` (writes
`bin/sift`), `just test`, `just vet`, and `just fmt`.

## Verify the installation

```sh
sift version
# e.g.  sift version 1.0.4
```

Confirm it can render by scanning a small project to standard output:

```sh
sift dump . --output -
```

## Shell completions

`sift` can generate completion scripts for Bash, Zsh, Fish, and PowerShell:

```sh
sift completion bash   # or: zsh, fish, powershell
```

Both installers write Bash, Zsh, and Fish completions into your user-local
directories automatically when those shells are present (failures are
non-fatal warnings). Zsh additionally needs the site-functions directory on
your `fpath`; the installer prints the exact line to add. To load completions
without reinstalling, source the output directly:

```sh
source <(sift completion bash)
```

## Desktop launcher (Linux and macOS)

`sift desktop install` adds Sift to your app launcher, opening
`sift serve --app` (run it with the installed binary, not a `go run`
cache path — the entry embeds the binary that ran the install):

```sh
sift desktop install    # install the launcher
sift desktop status     # show what is installed
sift desktop uninstall  # remove it again
```

Linux writes `~/.local/share/applications/sift.desktop` plus an icon and
refreshes the desktop database. macOS builds `~/Applications/Sift.app`
(the icon needs `sips`, which ships with macOS; without it the bundle
installs iconless, and first launch may show a Gatekeeper prompt for the
unsigned bundle). Everything is user-local and marked, so uninstall only
removes files sift created — a foreign file at one of these paths is left
alone with an explanation.

Distro packages may ship the same entry system-wide: `packaging/sift.desktop`
launches `sift serve --app` and travels in the release tarballs (including
`sift-context-bin` from v1.4.0 on).

## Upgrading

Re-run the same install command you used originally — the installer overwrites
the existing binary — or download the newest release archive. State and
preferences written during use live under your application configuration
directory and are preserved across upgrades.

## Uninstalling

Remove the installed binary, and optionally the config/state directory:

```sh
rm -f ~/.local/bin/sift               # curl / local installer path
brew uninstall bethropolis/tap/sift   # Homebrew
scoop uninstall sift                  # Scoop

sift desktop uninstall                # remove the app-launcher entry first, if any

rm -rf ~/.config/sift                 # optional: config, state, and preferences
```
