# Sift

[![Go Report Card](https://goreportcard.com/badge/github.com/bethropolis/sift)](https://goreportcard.com/report/github.com/bethropolis/sift)
[![GitHub release (latest by date)](https://img.shields.io/github/v/release/bethropolis/sift?style=flat-square&labelColor=1e1e2e&color=89b4fa)](https://github.com/bethropolis/sift/releases/latest)
[![GitHub license](https://img.shields.io/github/license/bethropolis/sift?style=flat-square&labelColor=1e1e2e&color=cba6f7)](https://github.com/bethropolis/sift/blob/main/LICENSE)
[![Go Reference](https://pkg.go.dev/badge/github.com/bethropolis/sift.svg)](https://pkg.go.dev/github.com/bethropolis/sift/)
[![Go Version](https://img.shields.io/badge/Go-1.26+-a6e3a1?style=flat-square&logo=go&labelColor=1e1e2e)](https://golang.org/doc/go1.21)

`sift` is a command-line tool written in Go that turns a project directory
into a single LLM-ready context document. It walks the tree, respects
`.gitignore` rules, and renders the contents to a `codebase.md` file (or to
stdout with `--output -`) in plain text, Markdown, JSON, or XML — with optional
token budgeting, secret redaction, signature-only compression, and
git-relevance ranking so the most important context fits in a model's window.

## Features

*   **Recursive Traversal:** Scans directories and subdirectories, respecting `.gitignore`.
*   **Output Formats:** `plain`, `markdown`, `json`, and `xml` (with CDATA escaping and token counts).
*   **LLM Context Packing:**
    *   Token counting via tiktoken and a `--budget` to keep only the highest-priority files.
    *   Secret scanning that redacts AWS/GitHub/Slack/Google/Stripe keys before output.
    *   Signature-only compression (`--mode signatures`) via tree-sitter for Go, Rust, JS, TS, Python, and PHP.
    *   Git-relevance ranking: modified files score highest, then recent diffs, then recent commits.
    *   One-click `--clipboard` copy to paste into a chat.
*   **Profiles:** Reusable TOML profiles in `.sift.toml` or `$XDG_CONFIG_HOME/sift/config.toml`.
*   **Interactive Picker (`pick`):** A foldable, dual-pane tree TUI with:
    *   3-state checkboxes (`[x]`/`[-]`/`[ ]`) and directory toggling (`Space`).
    *   Per-file/folder compression modes — `FULL`/`SIGS`/`SKIP` — cycled with `m`.
    *   Live token tallies per folder and a budget bar in the footer.
    *   A right-hand preview pane with secret warning badges.
    *   `s` smart auto-select by git relevance, `/` fuzzy filter, `y` clipboard copy.
    *   Nerd Font glyphs by default, plain ASCII with `--no-nerd-fonts`.
    *   Tree guide lines (`│ ├── └──`), folder icons before each name, and
        directories starting collapsed for a tidy top-level view.
    *   `d` opens the **delta modal** — select the commit range and strategy,
        then dump or copy only the incremental changes.
*   **Incremental Delta Dumps (`delta`):** After a baseline dump, feed an LLM
    only what changed since the last dump instead of the whole repository.
    *   Full content of the changed files (default) or a raw unified `git diff`
        patch (`--patch`) wrapped in a `<context_update>` block.
    *   The comparison base is the commit recorded by the most recent
        `dump`/`pick`/`diff`/`delta` for that project (or `--since <ref>`).
    *   State lives outside the repo in `~/.config/sift/state.json`,
        keeping working trees and git history clean.
*   **Prompt Directives:** `--prompt` (or `-p`) prepends a task/instructions
    section to any output format, so the document arrives with its mission.
*   **Subcommands:** `dump`, `pick` (interactive TUI), `diff [ref]`, `delta`, and `watch`.
*   **Filtering:** extension filters, custom ignore patterns, hidden/git handling, binary skipping, size limits.
*   **Concurrency, progress, timeouts, and colored output.**

## Installation

```bash
go install github.com/bethropolis/sift/cmd/sift@latest
```

Or build from source:

```bash
git clone https://github.com/bethropolis/sift.git
cd sift
go build -o sift ./cmd/sift/
```

## Usage

```bash
sift                # bare invocation launches the interactive picker (TUI)
sift dump [path]  # scan a directory and render its contents
sift pick [path]  # interactively choose files, then render
sift diff [ref]   # dump files changed relative to a git ref (default HEAD)
sift delta [path] # dump only changes since the last recorded dump
sift watch [path] # re-render the document on file changes
```

Running `sift` with no subcommand (in a terminal) launches the interactive
file picker; otherwise the help text is shown. `sift dump` scans the current
directory and writes the result to `codebase.md` (Markdown by default) next to
the scanned root; use `--output -` to print to stdout instead. The output file
is excluded from the scan so it never contains itself.

### Incremental Deltas

`sift delta` compares the current tree against the commit recorded by the
last successful `dump`, `pick`, `diff`, or `delta` for this project and dumps
only what changed:

```bash
sift delta                 # full content of files changed since last dump
sift delta --patch         # raw unified diff in a <context_update> block
sift delta --since main    # compare against a specific ref or branch
sift delta --patch --clipboard
```

The record is kept in `~/.config/sift/state.json`, keyed by the hash of
the project's absolute path, so repositories stay free of state files. Outside
a git repository, or with no baseline yet, `sift delta` explains what to run.
Inside the picker, press `d` to choose the commit range and strategy
interactively before dumping or copying.

### Examples

```bash
# Scan the current directory in Markdown.
sift dump --style markdown

# Emit XML with token counts and copy to the clipboard.
sift dump -dir ./src --style xml --clipboard

# Keep output under 50k tokens, prioritizing changed files.
sift dump --budget 50000

# Strip function bodies down to signatures.
sift dump --mode signatures --style xml

# Only dump the files you have changed.
sift diff

# Dump just the changes since the last recorded dump.
sift delta

# Token-minimal incremental update as a raw diff patch.
sift delta --patch

# Pick files interactively (requires a TTY).
sift pick

# Attach a task prompt to the dump.
sift dump --prompt "Review this codebase for race conditions."

# Use the picker without Nerd Font glyphs.
sift pick --no-nerd-fonts
```

### Flags

```
-dir string                 Root directory to scan
-style string               Output style: plain, markdown, json, xml
-json                       Legacy: output JSON (same as --style json)
-markdown                   Legacy: output Markdown
-output string              Output file (default "codebase.md", use "-" for stdout)
-clipboard                  Copy the rendered output to the system clipboard
-budget int                 Maximum token budget (0 = no limit)
-tokenize-model string      Tokenizer model encoding (default: cl100k_base)
-mode string                Compression mode: full, signatures
-prompt, -p string          Task/instruction directives prepended to the output
-no-nerd-fonts              Use plain ASCII glyphs in the interactive picker
-secrets                    Scan output for secrets and redact them (default true)
-force-secrets              Include secrets instead of redacting
-binary                     Include binary files (default: skipped)
-ext string                 Only include files with these extensions
-ignore string              Custom ignore patterns (comma-separated, gitignore syntax)
-hidden                     Ignore hidden files/directories (default true)
-git                        Ignore .git directories (default true)
-max-size int               Max file size to process in MB (0 = no limit)
-concurrent                 Enable concurrent file processing
-workers int                Max concurrent workers
-progress                   Show progress information
-timeout duration           Maximum execution time (e.g., '30s', '5m')
-show-skipped               Show skipped files and reasons at the end
-profile string             Config profile to use (see below)
-verbose / -quiet / -no-color / -log-level string
-version                    Show version information
```

## Profiles

Create a `.sift.toml` in the scanned directory (or
`~/.config/sift/config.toml` globally) and reference it with
`--profile`:

```toml
[profiles.claude]
style = "xml"
budget = 60000
secrets = true

[profiles.rust-strict]
extensions = ["rs"]
mode = "signatures"
prompt = "Review this Rust codebase for unsafe usage."
```

```bash
sift dump --profile claude
```

Command-line flags always override profile values. A global `default_profile`
can be set in the global config file.

## Development

```bash
go build ./...
go vet ./...
go test -race ./...
```

## Contributing

Contributions are welcome! Please feel free to submit issues and pull requests.

## License

This project is licensed under the [MIT License](LICENSE).
