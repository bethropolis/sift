# Sift documentation

[Sift](../README.md) turns a project directory into a single, LLM-friendly
context document. It walks the tree, honors `.gitignore`, and renders the
selected files in plain, Markdown, JSON, or XML — with token budgeting,
secret redaction, signature-only compression, and Git-relevance ranking so the
most important context fits in a model's window.

## Getting started

- [Installation](installation.md) — install on macOS, Linux, Windows, or
  FreeBSD, or build from source
- [Usage](usage.md) — commands, output destinations, filtering, and examples

## Features in depth

- [Interactive picker](picker.md) — the dual-pane TUI: navigation, per-file
  modes, smart selection, themes, and task prompts
- [Configuration and profiles](configuration.md) — flags, `.sift.toml`
  profiles, and how precedence works
- [Output formats and prompts](output.md) — the four styles, signature mode,
  task directives, and clipboard output
- [Incremental deltas](delta.md) — feed an LLM only what changed since the
  last dump
- [Dependency expansion](dependencies.md) — auto-include imports of the
  selected files as signature-compressed context
- [MCP server](mcp.md) — expose sift to coding agents over stdio

## Command cheat-sheet

| Command | What it does |
| --- | --- |
| `sift` | Open the interactive picker (bare invocation) |
| `sift dump [path]` | Render the whole eligible tree to `codebase.md` |
| `sift pick [path]` | Select files interactively, then render |
| `sift select [path]` | Auto-select useful files and render them |
| `sift diff [ref]` | Render only files changed relative to a Git ref |
| `sift delta [path]` | Render only changes since the last recorded dump |
| `sift watch [path]` | Re-render the document whenever files change |
| `sift mcp [path]` | Start an MCP server over stdio for coding agents |

Every command accepts `--help`; `sift --help` lists the shared flags.

> New to Sift? Start with [Usage](usage.md) and run `sift dump . --output -`
> to see a context document for the current directory.
