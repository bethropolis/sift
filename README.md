# Dir-Dumper

[![Go Report Card](https://goreportcard.com/badge/github.com/bethropolis/dir-dumper)](https://goreportcard.com/report/github.com/bethropolis/dir-dumper)
[![GitHub release (latest by date)](https://img.shields.io/github/v/release/bethropolis/dir-dumper?style=flat-square&labelColor=1e1e2e&color=89b4fa)](https://github.com/bethropolis/dir-dumper/releases/latest)
[![GitHub license](https://img.shields.io/github/license/bethropolis/dir-dumper?style=flat-square&labelColor=1e1e2e&color=cba6f7)](https://github.com/bethropolis/dir-dumper/blob/main/LICENSE)
[![Go Reference](https://pkg.go.dev/badge/github.com/bethropolis/dir-dumper.svg)](https://pkg.go.dev/github.com/bethropolis/dir-dumper/)
[![Go Version](https://img.shields.io/badge/Go-1.26+-a6e3a1?style=flat-square&logo=go&labelColor=1e1e2e)](https://golang.org/doc/go1.21)

`dumper` is a command-line tool written in Go that turns a project directory
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
*   **Profiles:** Reusable TOML profiles in `.dirdumper.toml` or `$XDG_CONFIG_HOME/dir-dumper/config.toml`.
*   **Subcommands:** `dump`, `pick` (interactive TUI), `diff [ref]`, and `watch`.
*   **Filtering:** extension filters, custom ignore patterns, hidden/git handling, binary skipping, size limits.
*   **Concurrency, progress, timeouts, and colored output.**

## Installation

```bash
go install github.com/bethropolis/dir-dumper/cmd/dumper@latest
```

Or build from source:

```bash
git clone https://github.com/bethropolis/dir-dumper.git
cd dir-dumper
go build -o dumper ./cmd/dumper/
```

## Usage

```bash
dumper              # bare invocation launches the interactive picker (TUI)
dumper dump [path]  # scan a directory and render its contents
dumper pick [path]  # interactively choose files, then render
dumper diff [ref]   # dump files changed relative to a git ref (default HEAD)
dumper watch [path] # re-render the document on file changes
```

Running `dumper` with no subcommand (in a terminal) launches the interactive
file picker; otherwise the help text is shown. `dumper dump` scans the current
directory and writes the result to `codebase.md` (Markdown by default) next to
the scanned root; use `--output -` to print to stdout instead. The output file
is excluded from the scan so it never contains itself.

### Examples

```bash
# Scan the current directory in Markdown.
dumper dump --style markdown

# Emit XML with token counts and copy to the clipboard.
dumper dump -dir ./src --style xml --clipboard

# Keep output under 50k tokens, prioritizing changed files.
dumper dump --budget 50000

# Strip function bodies down to signatures.
dumper dump --mode signatures --style xml

# Only dump the files you have changed.
dumper diff

# Pick files interactively (requires a TTY).
dumper pick
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

Create a `.dirdumper.toml` in the scanned directory (or
`~/.config/dir-dumper/config.toml` globally) and reference it with
`--profile`:

```toml
[profiles.claude]
style = "xml"
budget = 60000
secrets = true

[profiles.rust-strict]
extensions = ["rs"]
mode = "signatures"
```

```bash
dumper dump --profile claude
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
