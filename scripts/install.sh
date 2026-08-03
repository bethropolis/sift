#!/bin/sh
set -eu

# Install a checked-out copy of sift. For a network install, use:
#   curl -fsSL https://bethropolis.github.io/sift/install.sh | sh

# --- output helpers ---------------------------------------------------------
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

banner

command -v go >/dev/null 2>&1 || die "Go is required to build sift locally"

repo_dir=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
prefix=${SIFT_INSTALL_DIR:-"$HOME/.local/bin"}

info "building sift from $c_dim$repo_dir$c_reset ..."
mkdir -p "$prefix"
tmp="$prefix/.sift.tmp.$$"
trap 'rm -f "$tmp"' EXIT HUP INT TERM

(
    cd "$repo_dir"
    go build -trimpath -ldflags='-s -w' -o "$tmp" ./cmd/sift
)
chmod 755 "$tmp"
mv -f "$tmp" "$prefix/sift"

printf '\n'
ok "$c_bold sift installed to $prefix/sift $c_reset"
case ":${PATH}:" in
    *:"$prefix":*) ;;
    *) warn "add $prefix to your PATH to run sift"
       printf '    %sexport PATH="%s:$PATH"%s\n' "$c_dim" "$prefix" "$c_reset" ;;
esac
printf '%s    run %ssift --help%s to get started.%s\n' "$c_dim" "$c_green" "$c_reset" "$c_reset"
