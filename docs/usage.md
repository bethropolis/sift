# Usage

Sift turns a project directory into a single LLM-ready context document. The
examples assume you are inside the directory you want to scan; pass a path
argument to scan elsewhere.

## Quick start

Render the current directory to the default output file `codebase.md`:

```sh
sift dump .
```

Preview to standard output without writing a file:

```sh
sift dump . --output -
```

Open the interactive picker (bare `sift` when stdin is a terminal):

```sh
sift
```

## Commands

| Command | Description |
| --- | --- |
| `sift` | Launch the interactive picker |
| `sift dump [path]` | Render every eligible file in the tree |
| `sift pick [path]` | Select files interactively, then render |
| `sift select [path]` | Auto-select the most useful files and render them |
| `sift clone <repository>` | Temporarily shallow-clone a repository and dump it in the current directory |
| `sift copy [file]` | Copy `codebase.md` (or another document) to the clipboard |
| `sift diff [ref]` | Render only files changed relative to a Git ref (default `HEAD`) |
| `sift delta [path]` | Render only changes since the last recorded dump |
| `sift watch [path]` | Re-render the document whenever files change |
| `sift serve` | Serve the browser file picker (embedded web UI) |

Run `sift <command> --help` for each command's options, and `sift --help` for
the shared flags.

## Output destination

By default `dump`, `pick`, `select`, `diff`, and `watch` write to `codebase.md`
next to the scanned root. Control the destination with `--output`:

```sh
sift dump . --output -          # stdout (piping-safe)
sift dump . --output ../ctx.md  # a specific file
sift dump . --clipboard         # copy to the system clipboard instead
sift copy                       # copy the existing codebase.md for pasting
sift clone https://github.com/org/project --branch main --depth 1
sift clone https://github.com/org/project --here  # keep the checkout here (empty directory required)
```

`clone` accepts `--branch NAME` (or `-b NAME`) and `--depth N`; it uses the
installed Git client and the platform's temporary-directory support.
Depth 1 keeps the clone fast and is enough for dumping and asking about the
code, but history-based ranking (recency, churn) needs history: pass
`--depth 30` or more if you want `select` and smart-select to rank by it.

## Automatic selection

The `select` command combines Git-relevance ranking, language-aware rules, and
token cost to choose a useful context without a manual picker session. It uses
a 50,000-token budget by default. Test files, fixtures, mocks, generated code,
and vendored code are skipped by default; customize that policy in `[scoring]`:

```sh
sift select .                    # render the auto-selected files
sift select . --budget 50000     # fit the selection to a token budget
sift select . --budget 0         # explicitly disable the select budget
```

Inspect the decisions without writing a document:

```sh
sift select . --selection-only --print-selection
sift select . --selection-only --selection-format json   # also: ndjson
```

`--include-skipped` adds the filtered files and their skip reasons to the
report.

Selection policy can be configured in the repository's `.sift.toml`, the
global `~/.config/sift/config.toml`, or a named profile. For example:

```toml
[scoring]
skip_roles = ["test", "fixture", "mock", "generated", "vendor", "example"]
retention = { entrypoint = 0.4, api = 0.2, implementation = 0.08 }
test_task_boost = 0.2
```

Omit `skip_roles` to use the built-in defaults. Set it to a list to replace
them. An empty list restores the built-in defaults.
With the default role policy, a prompt about tests, bugs, regressions, or
coverage admits test files and boosts their priority. `test_task_boost` tunes
that boost; explicitly listing `test` in `skip_roles` keeps tests excluded.

## Token budgets

Keep output within a model's context window. Files are ordered by Git relevance
so the most important context survives the budget.

```sh
sift dump . --budget 50000      # 0 (default) means unlimited
sift select . --budget 100000
```

## Filtering

Control what the scan includes:

```sh
sift dump . --ext go,md             # only these extensions
sift dump . --ignore 'vendor,dist'  # extra gitignore-style patterns
sift dump . --max-size 2            # skip files larger than 2 MB
sift dump . --smart                 # skip generated/lock/minified/oversized
sift dump . --binary                # include binary files (skipped by default)
sift dump . --hidden=false          # include hidden files (dotfiles)
sift dump . --git=false             # include Git-ignored files
```

> The `--hidden` and `--git` flags default to *true*, meaning *ignore* hidden
> files and Git-ignored files. Pass `--hidden=false` or `--git=false` to
> include them. In the interactive picker you reveal them on demand instead
> with the `.` and `H` visibility toggles.

List everything that was excluded and why:

```sh
sift dump . --show-skipped
```

## Git-aware commands

`diff` and `delta` are built around Git:

```sh
sift diff                # files changed in the uncommitted working tree
sift diff main           # files changed since the main branch
sift delta               # changes since the last recorded dump
sift delta --patch       # raw unified diff in a <context_update> block
```

`diff` and `delta` require the scanned directory to be inside a Git repository.
See [Incremental deltas](delta.md) for details.

## More examples

```sh
# An XML outline of a filtered subset, copied to the clipboard
sift dump ./src --ext go,ts --style xml --clipboard

# Markdown with a task directive and a tight budget
sift dump . --budget 40000 --prompt "Review for race conditions."

# Re-render the context document continuously as you edit
sift watch . --style json
```

## Where to go next

- [Interactive picker](picker.md) for the full-featured TUI.
- [Configuration and profiles](configuration.md) for reusable flag sets.
- [Output formats and prompts](output.md) for styling and directives.
- [Incremental deltas](delta.md) for feeding an LLM only what changed.
