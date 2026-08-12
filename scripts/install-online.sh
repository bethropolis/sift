#!/bin/sh
set -eu

# sift online installer
#   curl -fsSL https://bethropolis.github.io/sift/install.sh | sh

repo=${SIFT_REPOSITORY:-bethropolis/sift}
prefix=${SIFT_INSTALL_DIR:-"$HOME/.local/bin"}

# --- output helpers ---------------------------------------------------------
# Colors only when stdout is a TTY and NO_COLOR is unset. POSIX-portable:
# escape sequences come from printf, never from $'...' (dash does not support it).
if [ -t 1 ] && [ -z "${NO_COLOR:-}" ]; then
    c_bold=$(printf '\033[1m')
    c_green=$(printf '\033[32m')
    c_yellow=$(printf '\033[33m')
    c_red=$(printf '\033[31m')
    c_cyan=$(printf '\033[36m')
    c_dim=$(printf '\033[2m')
    c_reset=$(printf '\033[0m')
else
    c_bold=""; c_green=""; c_yellow=""; c_red=""; c_cyan=""; c_dim=""; c_reset=""
fi

info()  { printf '%s→%s %s\n'  "$c_cyan" "$c_reset" "$*"; }
ok()    { printf '%s✔%s %s\n'  "$c_green" "$c_reset" "$*"; }
warn()  { printf '%s!%s %s\n'  "$c_yellow" "$c_reset" "$*" >&2; }
fail()  { printf '%s✘%s %s\n'  "$c_red" "$c_reset" "$*" >&2; }

banner() {
    printf '%s\n' "${c_bold}==============================================${c_reset}"
    printf '%s%s sift installer %s\n' "$c_bold" "$c_cyan" "$c_reset"
    printf '%s\n' "${c_bold}==============================================${c_reset}"
    printf '\n'
}

die() {
    fail "$*"
    printf '\n%s%sInstallation aborted.%s\n' "$c_red" "$c_bold" "$c_reset" >&2
    exit 1
}

# --- prerequisites ----------------------------------------------------------
banner

command -v curl >/dev/null 2>&1 || die "curl is required"
command -v tar  >/dev/null 2>&1 || die "tar is required"

os=$(uname -s | tr '[:upper:]' '[:lower:]')
machine=$(uname -m)
case "$os" in
    linux|darwin|freebsd) ;;
    *) die "unsupported operating system: $os (use Homebrew or Scoop on Windows)" ;;
esac
case "$machine" in
    x86_64|amd64)   arch=amd64 ;;
    aarch64|arm64)  arch=arm64 ;;
    i386|i686)      arch=386 ;;
    armv7*|armv6*)  arch=armv7 ;;
    *) die "unsupported architecture: $machine" ;;
esac
info "platform: ${os}/${arch}"

info "resolving latest release ..."
tag=$(curl -fsSL -o /dev/null -w '%{url_effective}' \
    "https://github.com/$repo/releases/latest")
tag=${tag##*/}
[ -n "$tag" ] || die "could not determine the latest sift release"
version=${tag#v}
archive="sift_${version}_${os}_${arch}.tar.gz"
url="https://github.com/$repo/releases/download/$tag/$archive"
ok "found $tag (${arch})"

# --- download & install -----------------------------------------------------
tmp_dir=$(mktemp -d 2>/dev/null || mktemp -d -t sift)
trap 'rm -rf "$tmp_dir"' EXIT HUP INT TERM

info "downloading $c_dim$archive$c_reset ..."
curl -fL --retry 3 "$url" -o "$tmp_dir/$archive" 2>/dev/null \
    || die "download failed (is $tag published for ${os}/${arch}?)"

info "extracting archive ..."
tar -xzf "$tmp_dir/$archive" -C "$tmp_dir"

info "installing to $c_dim$prefix$c_reset ..."
mkdir -p "$prefix"
install -m 755 "$tmp_dir/sift" "$prefix/sift"

# --- summary ----------------------------------------------------------------
printf '\n'
ok "$c_bold sift $tag installed to $prefix/sift $c_reset"
case ":${PATH}:" in
    *:"$prefix":*) ;;
    *) warn "add $prefix to your PATH to run sift"
       printf '    %sexport PATH="%s:$PATH"%s\n' "$c_dim" "$prefix" "$c_reset" ;;
esac
printf '%s    run %ssift --help%s to get started.%s\n' "$c_dim" "$c_green" "$c_reset" "$c_reset"
