#!/bin/sh
set -eu

# Install a checked-out copy of sift. For a network install, use:
#   curl -fsSL https://bethropolis.github.io/sift/install.sh | sh

repo_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
prefix=${SIFT_INSTALL_DIR:-"$HOME/.local/bin"}

command -v go >/dev/null 2>&1 || {
    echo "install.sh: Go is required to build sift locally" >&2
    exit 1
}

mkdir -p "$prefix"
tmp="$prefix/.sift.tmp.$$"
trap 'rm -f "$tmp"' EXIT HUP INT TERM

(
    cd "$repo_dir"
    go build -trimpath -ldflags='-s -w' -o "$tmp" ./cmd/sift
)
chmod 755 "$tmp"
mv -f "$tmp" "$prefix/sift"

echo "Installed sift to $prefix/sift"
case ":${PATH}:" in
    *:"$prefix":*) ;;
    *) echo "Add $prefix to PATH to run sift" >&2 ;;
esac
