# Sift

[![Go Report Card](https://goreportcard.com/badge/github.com/bethropolis/sift)](https://goreportcard.com/report/github.com/bethropolis/sift)
[![GitHub release (latest by date)](https://img.shields.io/github/v/release/bethropolis/sift?style=flat-square&labelColor=1e1e2e&color=89b4fa)](https://github.com/bethropolis/sift/releases/latest)
[![GitHub license](https://img.shields.io/github/license/bethropolis/sift?style=flat-square&labelColor=1e1e2e&color=cba6f7)](https://github.com/bethropolis/sift/blob/main/LICENSE)
[![Go Reference](https://pkg.go.dev/badge/github.com/bethropolis/sift.svg)](https://pkg.go.dev/github.com/bethropolis/sift/)
[![Go Version](https://img.shields.io/badge/Go-1.26+-a6e3a1?style=flat-square&logo=go&labelColor=1e1e2e)](https://golang.org/doc/go1.26)

`sift` is a command-line tool written in Go that turns a project directory
into a single LLM-ready context document. It walks the tree, respects
`.gitignore` rules, and renders the contents to a `codebase.md` file (or to
stdout with `--output -`) in plain text, Markdown, JSON, or XML — with optional
token budgeting, secret redaction, signature-only compression, and
git-relevance ranking so the most important context fits in a model's window.

## Features

* **Recursive Traversal:** Scans directories and subdirectories, respecting `.gitignore`.
* **Output Formats:** `plain`, `markdown`, `json`, and `xml`, with CDATA escaping and token counts.
* **LLM Context Packing:** Token-counts files against a `--budget` to keep only the highest-priority context.
  * Redacts secrets (AWS/GitHub/Slack/Google/Stripe keys) before output.
  * Signature-only compression (`--mode signatures`) via tree-sitter for Go, Rust, JS, TS, Python, and PHP.
  * Git-relevance ranking — modified files first, then recent diffs, then recent commits.
  * One-click `--clipboard` copy to paste into a chat.
* **Profiles:** Reusable TOML profiles in `.sift.toml` or `$XDG_CONFIG_HOME/sift/config.toml`.
* **Interactive Picker (`pick`):** A foldable, dual-pane tree TUI with 3-state checkboxes, per-file `FULL`/`SIGS`/`SKIP` modes, live token tallies, a budget bar, and a preview pane with secret warnings.
  * `s` smart-selects by git relevance, `/` fuzzy-filters, `y` copies to clipboard, `d` opens the delta modal.
  * Nerd Font glyphs by default (plain ASCII with `--no-nerd-fonts`), hidden/gitignored toggles, persisted color themes, and task directives.
* **Incremental Delta Dumps (`delta`):** After a baseline dump, feed an LLM only what changed since the last dump.
  * Full changed files (default) or a raw unified `git diff` patch (`--patch`) wrapped in a `<context_update>` block.
* **Prompt Directives:** `--prompt` (or `-p`) prepends a task/instructions section to any output format; the picker adds task presets and a custom directive builder.
* **Subcommands:** `dump`, `pick`, `select`, `diff`, `delta`, and `watch`.
* **Filtering:** Extension filters, custom ignore patterns, hidden/Git handling, binary skipping, and size limits.
* **Concurrency, progress, timeouts, and colored output.**

## Installation

### macOS and Linux

```bash
curl -fsSL https://bethropolis.github.io/sift/install.sh | sh
```

The installer places `sift` in `~/.local/bin`. Set `SIFT_INSTALL_DIR` to use a
different directory.

### Homebrew

```bash
brew install bethropolis/tap/sift
```

### Windows

```powershell
scoop bucket add bethropolis https://github.com/bethropolis/scoop-bucket
scoop install sift
```

### go install
```bash
go install github.com/bethropolis/sift/cmd/sift@latest
```

Release archives are published for Linux, Windows, macOS, and FreeBSD on
amd64, arm64, 386, and armv7 where supported by Go.

### Build from source

```bash
git clone https://github.com/bethropolis/sift.git
cd sift
./scripts/install.sh
```

The local installer builds the current checkout and installs it to
`~/.local/bin` by default. Use `SIFT_INSTALL_DIR` to change the destination.

## Usage

```bash
sift                # bare invocation launches the interactive picker (TUI)
sift dump [path]    # scan a directory and render its contents
sift pick [path]    # interactively choose files, then render
sift select [path]  # automatically select useful files
sift diff [ref]     # dump files changed relative to a git ref
sift delta [path]   # dump only changes since the last recorded dump
sift watch [path]   # re-render the document on file changes
```

Running `sift` with no subcommand in a terminal launches the interactive file
picker. `sift dump` writes `codebase.md` by default; use `--output -` for
stdout instead.

### Examples

```bash
sift dump --style markdown
sift dump -dir ./src --style xml --clipboard
sift dump --budget 50000
sift dump --mode signatures --style xml
sift dump --prompt "Review this codebase for race conditions."
sift select . --selection-only --print-selection
sift select . --selection-only --selection-format json
sift pick --no-nerd-fonts
```

### Interactive picker quick reference

The picker is a dual-pane TUI: a foldable file tree on the left and a live
preview on the right, with a budget bar and footer.

See [docs/picker.md](docs/picker.md) for the complete keybinding reference.

### Incremental Deltas

```bash
sift delta                 # full content of files changed since last dump
sift delta --patch         # raw unified diff in a <context_update> block
sift delta --since main    # compare against a specific ref or branch
sift delta --patch --clipboard
```

The record is kept in `~/.config/sift/state.json`, keyed by the hash of the
project's absolute path. Inside the picker, press `d` to choose the commit
range and strategy interactively.

Run `sift --help` or `sift <command> --help` for all flags, including profiles,
budgets, ignore rules, secret scanning, and window-title configuration.

## Profiles

Create a `.sift.toml` in the scanned directory (or
`~/.config/sift/config.toml` globally) and reference it with `--profile`:

```toml
[profiles.claude]
style = "xml"
budget = 60000

[profiles.rust-strict]
extensions = ["rs"]
mode = "signatures"
prompt = "Review this Rust codebase for unsafe usage."
```

```bash
sift dump --profile claude
```

Command-line flags always override profile values.

## Documentation and releases

In-depth guides are kept in [`docs/`](docs/README.md):

- [Installation](docs/installation.md)
- [Usage](docs/usage.md)
- [Interactive picker](docs/picker.md)
- [Configuration and profiles](docs/configuration.md)
- [Output formats and prompts](docs/output.md)
- [Incremental deltas](docs/delta.md)

Prebuilt releases are available on the
[GitHub Releases page](https://github.com/bethropolis/sift/releases). Online
installation instructions are available at
[bethropolis.github.io/sift](https://bethropolis.github.io/sift/).

## Development

Requirements: Go 1.26 or newer and a POSIX shell for the helper scripts.

```bash
go test ./...
go vet ./...
go test -race ./...
```

Common `just` recipes include `just test`, `just test-race`, `just build`,
`just dump`, and `just pick`.

## Contributing

Contributions are welcome. Please feel free to submit issues and pull requests.

## License

This project is licensed under the [MIT License](LICENSE).
