#!/bin/sh
set -eu

repo=${SIFT_REPOSITORY:-bethropolis/sift}
prefix=${SIFT_INSTALL_DIR:-"$HOME/.local/bin"}

command -v curl >/dev/null 2>&1 || {
    echo "install-online.sh: curl is required" >&2
    exit 1
}
command -v tar >/dev/null 2>&1 || {
    echo "install-online.sh: tar is required" >&2
    exit 1
}

os=$(uname -s | tr '[:upper:]' '[:lower:]')
machine=$(uname -m)
case "$os" in
    linux|darwin|freebsd) ;;
    *) echo "Unsupported operating system: $os (use Homebrew or Scoop on Windows)" >&2; exit 1 ;;
esac
case "$machine" in
    x86_64|amd64) arch=amd64;;
    aarch64|arm64) arch=arm64;;
    i386|i686) arch=386;;
    armv7*|armv6*) arch=armv7;;
    *) echo "Unsupported architecture: $machine" >&2; exit 1 ;;
esac

tag=$(curl -fsSL "https://api.github.com/repos/$repo/releases/latest" \
    | sed -n 's/^[[:space:]]*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' \
    | head -n 1)
[ -n "$tag" ] || { echo "Could not determine the latest sift release" >&2; exit 1; }
version=${tag#v}
archive="sift_${version}_${os}_${arch}.tar.gz"
url="https://github.com/$repo/releases/download/$tag/$archive"

tmp_dir=$(mktemp -d 2>/dev/null || mktemp -d -t sift)
trap 'rm -rf "$tmp_dir"' EXIT HUP INT TERM
curl -fL --retry 3 "$url" -o "$tmp_dir/$archive"
tar -xzf "$tmp_dir/$archive" -C "$tmp_dir"
mkdir -p "$prefix"
install -m 755 "$tmp_dir/sift" "$prefix/sift"

echo "Installed sift $tag to $prefix/sift"
case ":${PATH}:" in
    *:"$prefix":*) ;;
    *) echo "Add $prefix to PATH to run sift" >&2 ;;
esac
