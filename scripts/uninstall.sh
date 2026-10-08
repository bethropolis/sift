#!/bin/sh
set -eu

# Remove a sift installation made by scripts/install.sh (or the online
# installer): the binary plus the shell completions it generated.
# The desktop launcher, if any, goes first via the binary itself.
#
# Environment:
#   SIFT_INSTALL_DIR   install directory (default: ~/.local/bin)
#   NO_COLOR           disable ANSI colors

usage() {
    cat <<'EOF'
Usage: uninstall.sh [options]

Remove the sift binary and its generated shell completions.

Options:
  --purge     also remove the config/state directory (~/.config/sift)
  -y, --yes   remove without asking (otherwise confirms first)
  -h, --help  show this help
  -q, --quiet  only print errors and the final result

Environment:
  SIFT_INSTALL_DIR   install directory (default: ~/.local/bin)
  NO_COLOR           disable ANSI colors
EOF
}

purge=0
quiet=0
yes=0
while [ $# -gt 0 ]; do
    case $1 in
        --purge)
            purge=1
            shift
            ;;
        -y|--yes)
            yes=1
            shift
            ;;        -h|--help)
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

use_color=0
if [ -t 1 ] && [ "${TERM:-dumb}" != "dumb" ] && [ -z "${NO_COLOR:-}" ]; then
    use_color=1
fi

if [ "$use_color" -eq 1 ]; then
    c_green=$(printf '\033[32m')
    c_yellow=$(printf '\033[33m')
    c_bold=$(printf '\033[1m')
    c_dim=$(printf '\033[2m')
    c_reset=$(printf '\033[0m')
else
    c_green=""; c_yellow=""; c_bold=""; c_dim=""; c_reset=""
fi

say()  { [ "$quiet" -eq 1 ] || printf '  %s\n' "$*"; }
gone() { [ "$quiet" -eq 1 ] || printf '  %s✔%s removed %s\n' "$c_green" "$c_reset" "$*"; }
kept() { printf '  %s!%s %s\n' "$c_yellow" "$c_reset" "$*" >&2; }

if [ -z "${SIFT_INSTALL_DIR+x}" ] || [ -z "${SIFT_INSTALL_DIR}" ]; then
    [ -n "${HOME:-}" ] || { kept "HOME is not set; export HOME or set SIFT_INSTALL_DIR"; exit 1; }
    prefix=$HOME/.local/bin
else
    prefix=$SIFT_INSTALL_DIR
fi

removed=0
missing=0

# Confirm the destructive part up front. Non-interactive stdin without
# --yes aborts instead of guessing.
if [ "$yes" -eq 0 ]; then
    if [ ! -t 0 ]; then
        kept "confirmation needed; re-run with --yes"
        exit 1
    fi
    printf '  Remove sift from %s' "$prefix" >&2
    if [ "$purge" -eq 1 ]; then
        printf ' (including config/state)' >&2
    fi
    printf '? [y/N] ' >&2
    read -r answer || answer=""
    case $answer in
        [yY]*) ;;
        *)
            say "cancelled"
            exit 0
            ;;
    esac
fi

# The launcher first, while its binary still exists to do it properly.
if [ -x "$prefix/sift" ]; then
    if "$prefix/sift" desktop uninstall >/dev/null 2>&1; then
        :
    else
        kept "desktop launcher: leaving it ($prefix/sift desktop uninstall failed)"
    fi
fi

# Only ever the binary file itself: a directory at this path is not ours.
if [ -e "$prefix/sift" ] && [ ! -d "$prefix/sift" ]; then
    rm -f "$prefix/sift"
    removed=$((removed + 1))
    gone "$prefix/sift"
elif [ -d "$prefix/sift" ]; then
    kept "$prefix/sift is a directory; leaving it alone"
else
    missing=$((missing + 1))
    say "not installed: $prefix/sift"
fi

if [ -n "${HOME:-}" ]; then
    data_home=${XDG_DATA_HOME:-$HOME/.local/share}
    config_home=${XDG_CONFIG_HOME:-$HOME/.config}
    for comp in "$data_home/bash-completion/completions/sift" \
                "$data_home/zsh/site-functions/_sift" \
                "$config_home/fish/completions/sift.fish"; do
        if [ -e "$comp" ]; then
            rm -f "$comp"
            removed=$((removed + 1))
            gone "$comp"
        fi
    done
    if [ "$purge" -eq 1 ]; then
        if [ -e "$config_home/sift" ]; then
            rm -rf "$config_home/sift"
            removed=$((removed + 1))
            gone "$config_home/sift"
        else
            say "no config directory: $config_home/sift"
        fi
    fi
else
    kept "HOME is not set; skipping completions"
fi

[ "$quiet" -eq 1 ] || printf '\n'
if [ "$removed" -gt 0 ]; then
    say "${c_bold}uninstalled${c_reset}  ${c_dim}$removed file(s) removed${c_reset}"
else
    say "nothing to remove"
fi
