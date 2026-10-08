#!/bin/sh
set -eu

# Install a checked-out copy of sift. For a network install, use:
#   curl -fsSL https://bethropolis.github.io/sift/install.sh | sh
#
# Environment:
#   SIFT_INSTALL_DIR   destination directory (default: ~/.local/bin)
#   CGO_ENABLED        forwarded to go build; auto-disabled if no C compiler
#   NO_COLOR           disable ANSI colors

usage() {
    cat <<'EOF'
Usage: install.sh [options]

Build this sift checkout and install the binary.

Options:
  -h, --help    show this help
  -q, --quiet   only print errors and the final result

Environment:
  SIFT_INSTALL_DIR   install directory (default: ~/.local/bin)
  CGO_ENABLED        passed through to go build
  NO_COLOR           disable ANSI colors
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
    printf '  %s▍ sift%s  local installer\n' "$c_bold$c_blue" "$c_reset"
    printf '  %s  build this checkout and install the binary%s\n' "$c_dim" "$c_reset"
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

tmp_dir=""
tmp_bin=""
cleanup() {
    [ -n "$tmp_bin" ] && rm -f "$tmp_bin"
    [ -n "$tmp_dir" ] && rm -rf "$tmp_dir"
}
trap 'cleanup; exit 1' HUP INT TERM
trap cleanup EXIT

have_cmd() { command -v "$1" >/dev/null 2>&1; }

# Compare dotted major.minor versions. Returns 0 when $1 >= $2.
version_ge() {
    a=$1
    b=$2
    a_maj=${a%%.*}
    a_min=${a#*.}
    a_min=${a_min%%.*}
    b_maj=${b%%.*}
    b_min=${b#*.}
    b_min=${b_min%%.*}
    a_maj=$(printf '%s' "$a_maj" | tr -cd '0-9')
    a_min=$(printf '%s' "$a_min" | tr -cd '0-9')
    b_maj=$(printf '%s' "$b_maj" | tr -cd '0-9')
    b_min=$(printf '%s' "$b_min" | tr -cd '0-9')
    [ -n "$a_maj" ] && [ -n "$a_min" ] && [ -n "$b_maj" ] && [ -n "$b_min" ] || return 1
    [ "$a_maj" -gt "$b_maj" ] && return 0
    [ "$a_maj" -eq "$b_maj" ] && [ "$a_min" -ge "$b_min" ] && return 0
    return 1
}

# Strip a go env version (go1.26.0, go1.26-devel) down to major.minor.
go_major_minor() {
    raw=$1
    raw=${raw#go}
    maj=${raw%%.*}
    rest=${raw#*.}
    min=${rest%%[!0-9]*}
    maj=$(printf '%s' "$maj" | tr -cd '0-9')
    min=$(printf '%s' "$min" | tr -cd '0-9')
    [ -n "$maj" ] && [ -n "$min" ] || return 1
    printf '%s.%s' "$maj" "$min"
}

profile_path() {
    if [ -z "${HOME:-}" ]; then
        printf '%s' "your shell profile"
        return
    fi
    shell_name=${SHELL:-}
    shell_name=${shell_name##*/}
    case $shell_name in
        zsh)  printf '%s' "${ZDOTDIR:-$HOME}/.zshrc" ;;
        bash)
            if [ -f "$HOME/.bash_profile" ]; then
                printf '%s' "$HOME/.bash_profile"
            else
                printf '%s' "$HOME/.bashrc"
            fi
            ;;
        fish) printf '%s' "${XDG_CONFIG_HOME:-$HOME/.config}/fish/config.fish" ;;
        *)    printf '%s' "$HOME/.profile" ;;
    esac
}

path_export() {
    shell_name=${SHELL:-}
    shell_name=${shell_name##*/}
    dest=$1
    case $shell_name in
        fish) printf 'fish_add_path %s' "$dest" ;;
        *)    printf 'export PATH="%s:$PATH"' "$dest" ;;
    esac
}

banner

# Resolve the checkout root from this script's location, even when invoked as
# `sh scripts/install.sh` or via a relative path.
self=$0
case $self in
    /*) ;;
    *)
        self_dir=$(CDPATH= cd -- "$(dirname -- "$self")" && pwd) || die "cannot resolve script directory"
        self=$self_dir/$(basename -- "$self")
        ;;
esac
repo_dir=$(CDPATH= cd -- "$(dirname -- "$self")/.." && pwd) || die "cannot resolve repository root"

step_begin "checking source tree"
if [ ! -f "$repo_dir/go.mod" ]; then
    step_fail "no go.mod in $repo_dir"
    hint "run this script from a sift checkout (./scripts/install.sh)"
    abort
fi
if [ ! -d "$repo_dir/cmd/sift" ]; then
    step_fail "cmd/sift is missing"
    hint "this does not look like a sift source tree: $repo_dir"
    abort
fi
mod_name=$(awk '/^module / { print $2; exit }' "$repo_dir/go.mod")
case $mod_name in
    github.com/bethropolis/sift) ;;
    *)
        step_fail "unexpected module $mod_name"
        hint "expected module github.com/bethropolis/sift"
        abort
        ;;
esac
mod_go=$(awk '/^go / { print $2; exit }' "$repo_dir/go.mod")
[ -n "$mod_go" ] || mod_go=1.26
step_ok "$repo_dir"

step_begin "checking Go toolchain"
if ! have_cmd go; then
    step_fail "go is not on PATH"
    hint "Go ${mod_go}+ is required — install it from https://go.dev/dl/"
    abort
fi
if ! go_version=$(go env GOVERSION 2>/dev/null); then
    step_fail "go env failed"
    hint "Go is installed but not usable; check GOROOT and your PATH"
    abort
fi
go_os=$(go env GOOS 2>/dev/null || printf '%s' "unknown")
go_arch=$(go env GOARCH 2>/dev/null || printf '%s' "unknown")
if parsed=$(go_major_minor "$go_version"); then
    if ! version_ge "$parsed" "$mod_go"; then
        step_fail "$go_version is older than go.mod requires ($mod_go)"
        hint "upgrade Go to ${mod_go}+ from https://go.dev/dl/"
        abort
    fi
else
    warn "could not parse Go version ($go_version); continuing anyway"
fi
step_ok "$go_version  ${go_os}/${go_arch}"

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
if [ -e "$prefix/sift" ]; then
    if [ -d "$prefix/sift" ]; then
        step_fail "$prefix/sift is a directory"
        hint "move that directory aside, then re-run"
        abort
    fi
    if [ ! -w "$prefix/sift" ]; then
        step_fail "cannot overwrite $prefix/sift"
        hint "fix permissions or choose a different SIFT_INSTALL_DIR"
        abort
    fi
fi
step_ok "$prefix"

if have_cmd id && [ "$(id -u 2>/dev/null || printf 1)" = "0" ]; then
    warn "running as root; the binary will be owned by root"
fi

# Respect an explicit CGO_ENABLED; otherwise skip cgo when no compiler is around
# so the build still succeeds (signature compression stays off).
cgo_note=""
if [ -z "${CGO_ENABLED+x}" ]; then
    if have_cmd gcc || have_cmd clang || have_cmd cc; then
        cgo_note="cgo on"
    else
        CGO_ENABLED=0
        export CGO_ENABLED
        cgo_note="cgo off (no C compiler)"
        warn "no C compiler on PATH; building without cgo"
        warn "signature compression (--mode signatures) will be unavailable"
    fi
else
    cgo_note="CGO_ENABLED=$CGO_ENABLED"
fi

tmp_dir=$(mktemp -d 2>/dev/null || mktemp -d -t siftinstall 2>/dev/null || true)
if [ -z "$tmp_dir" ] || [ ! -d "$tmp_dir" ]; then
    tmp_dir="${TMPDIR:-/tmp}/sift-install.$$"
    mkdir -p "$tmp_dir" || die "cannot create a temporary directory"
fi
tmp_bin=$prefix/.sift.$$.tmp
build_log=$tmp_dir/build.log

# `go` looks up caches under $HOME; keep the build working when HOME is unset
# but the caller gave an explicit install directory.
if [ -z "${HOME:-}" ]; then
    [ -n "${GOCACHE:-}" ]    || { GOCACHE=$tmp_dir/gocache; export GOCACHE; }
    [ -n "${GOMODCACHE:-}" ] || { GOMODCACHE=$tmp_dir/pkg/mod; export GOMODCACHE; }
    [ -n "${GOPATH:-}" ]     || { GOPATH=$tmp_dir/gopath; export GOPATH; }
fi

old_ver=""
if [ -x "$prefix/sift" ]; then
    old_ver=$("$prefix/sift" version 2>/dev/null || true)
fi

step_begin "building web UI"
# `sift serve` embeds web/dist/*.gz. Build it when missing so a local
# install ships a working browser picker; keep going without bun, but say so.
has_dist=0
for f in "$repo_dir"/web/dist/*.gz; do
    [ -e "$f" ] && has_dist=1
    break
done
if [ "$has_dist" -eq 1 ]; then
    step_ok "cached"
elif ! have_cmd bun; then
    step_ok "skipped"
    warn "bun is not on PATH; skipping the serve web UI"
    hint "install bun 1.4.2+ and re-run, or run: just web"
    hint "without it, 'sift serve' exits with the missing-UI message"
elif [ ! -f "$repo_dir/web/bun.lock" ]; then
    step_fail "web/bun.lock is missing"
    hint "this does not look like a complete checkout; re-clone sift"
    abort
else
    web_log=$tmp_dir/webbuild.log
    set +e
    (
        cd "$repo_dir/web" || exit 1
        bun install --frozen-lockfile && VITE_MOCK=false bun run build
    ) >"$web_log" 2>&1
    web_status=$?
    set -e
    if [ "$web_status" -ne 0 ]; then
        step_fail "web build exited $web_status"
        if [ -s "$web_log" ]; then
            printf '\n' >&2
            tail -n 40 "$web_log" >&2
            printf '\n' >&2
        fi
        hint "fix the errors above and re-run (or install bun 1.4.2+)"
        abort
    fi
    step_ok "embedded UI"
fi

step_begin "building sift"
# Stamp the checkout's version into the binary so `sift version` reports the
# tag instead of "dev". Falls back to "dev" when git metadata is missing.
ver=$(git -C "$repo_dir" describe --tags --always --dirty 2>/dev/null || printf 'dev')
set +e
(
    cd "$repo_dir" || exit 1
    # Keep GOPATH mode from hijacking a checkout that has go.mod.
    GO111MODULE=on
    export GO111MODULE
    go build -trimpath -ldflags="-s -w -X github.com/bethropolis/sift/internal/config.Version=$ver" -o "$tmp_bin" ./cmd/sift
) >"$build_log" 2>&1
build_status=$?
set -e
if [ "$build_status" -ne 0 ]; then
    step_fail "go build exited $build_status"
    if [ -s "$build_log" ]; then
        printf '\n' >&2
        tail -n 40 "$build_log" >&2
        printf '\n' >&2
    fi
    if grep -q -e 'proxy.golang.org' -e 'dial tcp' -e 'TLS handshake' "$build_log" 2>/dev/null; then
        hint "module download looks like a network failure; check GOPROXY and your connection"
    fi
    hint "fix the errors above and re-run"
    abort
fi
if [ ! -f "$tmp_bin" ]; then
    step_fail "build produced no file"
    hint "go build reported success but wrote nothing to $tmp_bin"
    abort
fi
chmod 755 "$tmp_bin" || die "cannot chmod the built binary"
step_ok "$cgo_note"

step_begin "verifying binary"
if [ ! -x "$tmp_bin" ]; then
    step_fail "binary is not executable"
    hint "chmod 755 failed to make $tmp_bin runnable"
    abort
fi
new_ver=$("$tmp_bin" version 2>/dev/null || true)
if [ -z "$new_ver" ]; then
    step_fail "sift version did not run"
    hint "the built binary is not usable"
    abort
fi
step_ok "$new_ver"

step_begin "installing"
if ! mv -f "$tmp_bin" "$prefix/sift"; then
    step_fail "could not write $prefix/sift"
    hint "the previous binary, if any, was left in place"
    abort
fi
tmp_bin=""
if [ ! -x "$prefix/sift" ]; then
    step_fail "installed file is missing or not executable"
    hint "install did not complete"
    abort
fi
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
if [ -n "$old_ver" ] && [ "$old_ver" != "$new_ver" ]; then
    ok "${c_bold}upgraded${c_reset}  ${c_dim}${old_ver}${c_reset} → ${c_bold}${new_ver}${c_reset}"
elif [ -n "$old_ver" ]; then
    ok "${c_bold}reinstalled${c_reset}  ${new_ver}"
else
    ok "${c_bold}installed${c_reset}  ${new_ver}"
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
    rc=$(profile_path)
    printf '          %s\n' "$(path_export "$prefix")"
    printf '          %s(add that line to %s)%s\n' "$c_dim" "$rc" "$c_reset"
    printf '  %snext%s   %s%s/sift --help%s\n' "$c_dim" "$c_reset" "$c_green" "$prefix" "$c_reset"
fi
[ "$quiet" -eq 1 ] || printf '\n'
