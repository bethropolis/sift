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

rm -rf ~/.config/sift                 # optional: config, state, and preferences
```
