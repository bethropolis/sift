#!/bin/sh
set -eu

# sift online installer
#   curl -fsSL https://bethropolis.github.io/sift/install.sh | sh
#
# Environment:
#   SIFT_REPOSITORY    GitHub repo (default: bethropolis/sift)
#   SIFT_INSTALL_DIR   destination directory (default: ~/.local/bin)
#   NO_COLOR           disable ANSI colors

usage() {
    cat <<'EOF'
Usage: install.sh [options]

Download the latest sift release and install the binary.

Options:
  -h, --help    show this help
  -q, --quiet   only print errors and the final result

Environment:
  SIFT_REPOSITORY   GitHub repository (default: bethropolis/sift)
  SIFT_INSTALL_DIR  install directory (default: ~/.local/bin)
  NO_COLOR          disable ANSI colors
EOF
}

quiet=0
while [ $# -gt 0 ]; do
    case $1 in
        -h|--help)
            usage
            exit 0
            ;;
        -q|--quiet)
            quiet=1
            shift
            ;;
        --)
            shift
            break
            ;;
        -*)
            printf 'unknown option: %s (try --help)\n' "$1" >&2
            exit 1
            ;;
        *)
            printf 'unexpected argument: %s (try --help)\n' "$1" >&2
            exit 1
            ;;
    esac
done

# Colors only when stdout is a TTY, TERM is usable, and NO_COLOR is unset.
# POSIX-portable: escape sequences come from printf, never from $'...'.
use_tty=0
use_color=0
if [ -t 1 ] && [ "${TERM:-dumb}" != "dumb" ]; then
    use_tty=1
    if [ -z "${NO_COLOR:-}" ]; then
        use_color=1
    fi
fi

if [ "$use_color" -eq 1 ]; then
    c_bold=$(printf '\033[1m')
    c_green=$(printf '\033[32m')
    c_yellow=$(printf '\033[33m')
    c_red=$(printf '\033[31m')
    c_cyan=$(printf '\033[36m')
    c_blue=$(printf '\033[94m')
    c_dim=$(printf '\033[2m')
    c_reset=$(printf '\033[0m')
    c_erase=$(printf '\r\033[2K')
else
    c_bold=""; c_green=""; c_yellow=""; c_red=""; c_cyan=""; c_blue=""
    c_dim=""; c_reset=""; c_erase=""
fi

info()  { printf '  %s→%s %s\n'  "$c_cyan" "$c_reset" "$*"; }
ok()    { printf '  %s✔%s %s\n'  "$c_green" "$c_reset" "$*"; }
warn()  { printf '  %s!%s %s\n'  "$c_yellow" "$c_reset" "$*" >&2; }
fail()  { printf '  %s✘%s %s\n'  "$c_red" "$c_reset" "$*" >&2; }
hint()  { printf '         %s%s%s\n' "$c_dim" "$*" "$c_reset" >&2; }

step_msg=""
step_begin() {
    step_msg=$1
    [ "$quiet" -eq 1 ] && return 0
    if [ "$use_tty" -eq 1 ]; then
        printf '  %s…%s %s' "$c_dim" "$c_reset" "$step_msg"
    else
        printf '  ... %s\n' "$step_msg"
    fi
}

step_ok() {
    detail=${1:-}
    [ "$quiet" -eq 1 ] && return 0
    if [ "$use_tty" -eq 1 ]; then
        printf '%s  %s✔%s %s' "$c_erase" "$c_green" "$c_reset" "$step_msg"
        if [ -n "$detail" ]; then
            printf '  %s%s%s' "$c_dim" "$detail" "$c_reset"
        fi
        printf '\n'
    else
        if [ -n "$detail" ]; then
            printf '  ok %s (%s)\n' "$step_msg" "$detail"
        else
            printf '  ok %s\n' "$step_msg"
        fi
    fi
}

step_fail() {
    detail=${1:-}
    if [ "$use_tty" -eq 1 ] && [ "$quiet" -eq 0 ]; then
        printf '%s' "$c_erase" >&2
    fi
    if [ -n "$detail" ]; then
        fail "$step_msg — $detail"
    else
        fail "$step_msg"
    fi
}

banner() {
    [ "$quiet" -eq 1 ] && return 0
    printf '\n'
    printf '  %s▍ sift%s  online installer\n' "$c_bold$c_blue" "$c_reset"
    printf '  %s  download the latest release and install the binary%s\n' "$c_dim" "$c_reset"
    printf '\n'
}

abort() {
    printf '\n  %s%sInstallation aborted.%s\n\n' "$c_red" "$c_bold" "$c_reset" >&2
    exit 1
}

die() {
    fail "$*"
    abort
}

have_cmd() { command -v "$1" >/dev/null 2>&1; }

banner

repo=${SIFT_REPOSITORY:-bethropolis/sift}

step_begin "checking prerequisites"
have_curl=1; command -v curl >/dev/null 2>&1 || have_curl=0
have_tar=1;  command -v tar  >/dev/null 2>&1 || have_tar=0
if [ "$have_curl" -eq 0 ] || [ "$have_tar" -eq 0 ]; then
    missing=""
    [ "$have_curl" -eq 0 ] && missing="curl"
    [ "$have_tar" -eq 0 ]  && missing="$missing tar"
    step_fail "missing: $missing"
    hint "install ${missing} with your package manager, then re-run"
    abort
fi
step_ok "curl, tar"

step_begin "detecting platform"
os=$(uname -s | tr '[:upper:]' '[:lower:]')
machine=$(uname -m)
case "$os" in
    linux|darwin|freebsd) ;;
    *)
        step_fail "unsupported OS: $os"
        hint "use Homebrew or Scoop on Windows"
        abort
        ;;
esac
case "$machine" in
    x86_64|amd64)   arch=amd64 ;;
    aarch64|arm64)  arch=arm64 ;;
    i386|i686)      arch=386 ;;
    armv7*|armv6*)  arch=armv7 ;;
    *)
        step_fail "unsupported architecture: $machine"
        hint "build from source instead: https://bethropolis.github.io/sift/install/#build-from-source"
        abort
        ;;
esac
step_ok "${os}/${arch}"

step_begin "resolving latest release"
tag=$(curl -fsSL -o /dev/null -w '%{url_effective}' \
    "https://github.com/$repo/releases/latest" 2>/dev/null || true)
tag=${tag##*/}
if [ -z "$tag" ]; then
    step_fail "could not determine the latest release"
    hint "check https://github.com/$repo/releases in a browser"
    hint "your network may block github.com, or no release may exist yet"
    abort
fi
version=${tag#v}
archive="sift_${version}_${os}_${arch}.tar.gz"
url="https://github.com/$repo/releases/download/$tag/$archive"
step_ok "$tag"

if [ -z "${SIFT_INSTALL_DIR+x}" ] || [ -z "${SIFT_INSTALL_DIR}" ]; then
    [ -n "${HOME:-}" ] || die "HOME is not set; export HOME or set SIFT_INSTALL_DIR"
    prefix=$HOME/.local/bin
else
    prefix=$SIFT_INSTALL_DIR
fi

step_begin "preparing install directory"
case $prefix in
    /*) ;;
    *) prefix=$(CDPATH= cd -- "$(pwd)" && printf '%s/%s' "$(pwd)" "$prefix") ;;
esac
if [ -e "$prefix" ] && [ ! -d "$prefix" ]; then
    step_fail "$prefix exists and is not a directory"
    hint "set SIFT_INSTALL_DIR to a directory you can write to"
    abort
fi
if ! mkdir -p "$prefix" 2>/dev/null; then
    step_fail "cannot create $prefix"
    hint "set SIFT_INSTALL_DIR or fix permissions on the parent directory"
    abort
fi
prefix=$(CDPATH= cd -- "$prefix" && pwd) || die "cannot resolve install directory"
if [ ! -w "$prefix" ]; then
    step_fail "$prefix is not writable"
    hint "set SIFT_INSTALL_DIR to a directory you can write to"
    abort
fi
if [ -e "$prefix/sift" ] && [ -d "$prefix/sift" ]; then
    step_fail "$prefix/sift is a directory"
    hint "move that directory aside, then re-run"
    abort
fi
step_ok "$prefix"

tmp_dir=$(mktemp -d 2>/dev/null || mktemp -d -t sift 2>/dev/null || true)
if [ -z "$tmp_dir" ] || [ ! -d "$tmp_dir" ]; then
    tmp_dir="${TMPDIR:-/tmp}/sift-install.$$"
    mkdir -p "$tmp_dir" || die "cannot create a temporary directory"
fi
trap 'rm -rf "$tmp_dir"' EXIT HUP INT TERM

old_ver=""
if [ -x "$prefix/sift" ]; then
    old_ver=$("$prefix/sift" version 2>/dev/null || true)
fi

step_begin "downloading release"
if ! curl -fL --retry 3 "$url" -o "$tmp_dir/$archive" 2>"$tmp_dir/curl.log"; then
    step_fail "download failed"
    if grep -q -e '404' "$tmp_dir/curl.log" 2>/dev/null; then
        hint "$tag has no asset named $archive"
        hint "check https://github.com/$repo/releases/tag/$tag for your platform"
    else
        hint "check your connection and proxy settings, then re-run"
    fi
    abort
fi
step_ok "$archive"

sums_url="https://github.com/$repo/releases/download/$tag/checksums.txt"
step_begin "verifying checksum"
skip_reason=""
if curl -fsSL "$sums_url" -o "$tmp_dir/checksums.txt" 2>/dev/null; then
    want=$(awk -v a="$archive" '$2 == a { print $1; exit }' "$tmp_dir/checksums.txt" 2>/dev/null || true)
    if [ -z "$want" ]; then
        skip_reason="no entry for $archive"
    else
        have=""
        if have_cmd sha256sum; then
            have=$(sha256sum "$tmp_dir/$archive" 2>/dev/null | awk '{ print $1 }' || true)
        elif have_cmd shasum; then
            have=$(shasum -a 256 "$tmp_dir/$archive" 2>/dev/null | awk '{ print $1 }' || true)
        elif have_cmd sha256; then
            # BSD sha256(1): -q prints the bare hash.
            have=$(sha256 -q "$tmp_dir/$archive" 2>/dev/null || true)
        fi
        if [ -z "$have" ]; then
            skip_reason="no sha256 tool on PATH"
        elif [ "$have" = "$want" ]; then
            step_ok "checksum verified"
        else
            step_fail "checksum mismatch for $archive"
            hint "expected $want"
            hint "got      $have"
            hint "the download may be corrupt or tampered with; aborting"
            abort
        fi
    fi
else
    skip_reason="checksums.txt unavailable"
fi
if [ -n "$skip_reason" ]; then
    warn "skipping checksum verification ($skip_reason)"
    step_ok "unverified"
fi

step_begin "extracting archive"
if ! tar -xzf "$tmp_dir/$archive" -C "$tmp_dir" 2>/dev/null; then
    step_fail "archive is corrupt or unreadable"
    hint "remove any cached download and re-run"
    abort
fi
if [ ! -f "$tmp_dir/sift" ]; then
    step_fail "archive has no sift binary"
    hint "the release asset layout may have changed; report it at https://github.com/$repo/issues"
    abort
fi
step_ok "sift binary"

step_begin "installing"
if ! install -m 755 "$tmp_dir/sift" "$prefix/sift" 2>/dev/null; then
    step_fail "could not write $prefix/sift"
    hint "the previous binary, if any, was left in place"
    abort
fi
if ! "$prefix/sift" version >/dev/null 2>&1; then
    step_fail "installed binary does not run"
    hint "your platform may need a different archive; see the note above"
    abort
fi
new_ver=$("$prefix/sift" version 2>/dev/null || true)
step_ok "$prefix/sift"

# Best-effort: generate completions from the installed binary into user-local
# directories. Never aborts the install; per-shell failures are warnings.
step_begin "installing shell completions"
comp_done=""
if [ -n "${HOME:-}" ]; then
    data_home=${XDG_DATA_HOME:-$HOME/.local/share}
    config_home=${XDG_CONFIG_HOME:-$HOME/.config}
    # Triplets of shell name and completion destination. PowerShell is
    # omitted: its profile paths vary and Windows installs go via Scoop.
    comp_specs="bash $data_home/bash-completion/completions/sift
zsh $data_home/zsh/site-functions/_sift
fish $config_home/fish/completions/sift.fish"
    # Iterate line-by-line without a subshell so comp_done survives.
    while IFS= read -r spec || [ -n "$spec" ]; do
        [ -n "$spec" ] || continue
        shell_name=${spec%% *}
        dest=${spec#* }
        have_cmd "$shell_name" || continue
        dest_dir=$(dirname -- "$dest")
        if mkdir -p "$dest_dir" 2>/dev/null \
            && "$prefix/sift" completion "$shell_name" >"$dest.tmp" 2>/dev/null \
            && mv -f "$dest.tmp" "$dest" 2>/dev/null; then
            if [ -z "$comp_done" ]; then comp_done=$shell_name; else comp_done="$comp_done, $shell_name"; fi
        else
            rm -f "$dest.tmp" 2>/dev/null
            warn "could not install $shell_name completions to $dest"
        fi
    done <<EOF
$comp_specs
EOF
    case ",$comp_done," in
        *,zsh*) hint "zsh: add $data_home/zsh/site-functions to fpath in your .zshrc" ;;
    esac
else
    warn "HOME is not set; skipping shell completions"
fi
if [ -n "$comp_done" ]; then
    step_ok "$comp_done"
else
    step_ok "skipped"
fi

[ "$quiet" -eq 1 ] || printf '\n'
if [ -n "$old_ver" ] && [ -n "$new_ver" ] && [ "$old_ver" != "$new_ver" ]; then
    ok "${c_bold}upgraded${c_reset}  ${c_dim}${old_ver}${c_reset} → ${c_bold}${new_ver}${c_reset}"
elif [ -n "$old_ver" ]; then
    ok "${c_bold}reinstalled${c_reset}  ${new_ver:-$tag}"
else
    ok "${c_bold}installed${c_reset}  ${new_ver:-$tag}"
fi
info "binary  $prefix/sift"

in_path=0
case ":${PATH-}:" in
    *:"$prefix":*) in_path=1 ;;
esac

if [ "$in_path" -eq 1 ]; then
    info "PATH    $prefix is already on PATH"
    [ "$quiet" -eq 1 ] || printf '  %snext%s   %ssift --help%s\n' "$c_dim" "$c_reset" "$c_green" "$c_reset"
else
    warn "add $prefix to your PATH"
    printf '          %sexport PATH="%s:$PATH"%s\n' "$c_dim" "$prefix" "$c_reset"
    printf '  %snext%s   %s%s/sift --help%s\n' "$c_dim" "$c_reset" "$c_green" "$prefix" "$c_reset"
fi
[ "$quiet" -eq 1 ] || printf '\n'
