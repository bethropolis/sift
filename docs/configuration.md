# Configuration

Command-line flags configure a single run. Profiles and targets make those
settings reusable and committable so `sift dump` does the right thing without
flags — suitable for CI and automated dumping.

## Quick start: `[sift]` defaults

Put repository-wide defaults in `.sift.toml` (project-local, committed) so
any clone can run `sift dump` with no flags:

```toml
[sift]
style = "xml"
budget = 40000
mode = "full"
output = "codebase.md"
smart = true
extensions = ["go", "md", "ts"]
ignore = ["vendor/**", "dist/**"]
prompt = "Review for correctness."
# prompt_file = "prompts/review.md"   # file path instead of inline prompt
```

Flags still win: `sift dump --budget 80000` overrides `budget = 40000`.

## Profiles

Profiles are named presets you select with `--profile` (or via a target):

```toml
[profiles.review]
style = "markdown"
budget = 60000
mode = "signatures"
prompt = "Review this codebase for correctness and security issues."
extensions = ["go", "md"]

[profiles.claude]
style = "xml"
budget = 60000

[profiles.rust-strict]
extends = ["claude"]           # inherit, then override
extensions = ["rs"]
mode = "signatures"
prompt_file = "prompts/rust-review.md"
output = "rust-context.md"
```

```sh
sift dump . --profile review
sift dump . --profile rust-strict
```

`extends` composes profiles: ancestors are overlaid in order, then the
profile's own fields win. Cycles are an error.

## Targets: multiple artifacts from one file

Each `[[targets]]` is one dump artifact. Select one with `--target`:

```toml
[[targets]]
name = "full"
profile = "claude"             # optional; inherits that profile first
output = "codebase.md"

[[targets]]
name = "sig"
profile = "rust-strict"
output = "codebase.sig.xml"
style = "xml"

[[targets]]
name = "docs"
extensions = ["md"]
prompt = "Summarize the architecture for a new teammate."
output = "docs-context.md"
budget = 12000
```

```sh
sift dump --target full        # codebase.md
sift dump --target sig         # codebase.sig.xml
sift dump --target docs        # docs-context.md (md-only, short budget)
```

Target fields (`style`, `budget`, `mode`, `output`, `prompt`/`prompt_file`,
`extensions`, `ignore`, `scoring`) override the resolved profile and `[sift]`
defaults. Flags override everything.

## Prompts

Inline `prompt` or file-backed `prompt_file` on `[sift]`, any profile, or any
target:

```toml
[profiles.review]
prompt_file = "prompts/review.md"

[[targets]]
name = "api"
prompt_file = "prompts/api.md"
```

A reusable prompt library is also available:

```toml
[prompts.review]
text = "Review for correctness and security."

[prompts.docs]
file = "prompts/docs.md"
```

Selected with the resolver's `WithPromptRef("review")` (programmatic) or via
future `--prompt-ref` CLI flag.

## Automation hints

Declarative hints for `watch` / hooks (opt-in; nothing runs from TOML alone):

```toml
[automation]
watch_interval = "500ms"
git_hook = "post-commit"       # `sift hook install` reads this
hook_targets = ["full", "sig"]
```

## Precedence

```
Built-in defaults (Config.New())
  → global ~/.config/sift/config.toml
  → local .sift.toml [sift] defaults
  → selected profile (+ extends expansion)
  → selected target overrides
  → explicit --flags (pflag.Changed guard)
```

Local `.sift.toml` overlays global; `sift dump /path` resolves
`.sift.toml` relative to the scanned root, not just `cwd`.

## Legacy

`default_profile = "NAME"` in either config file, and bare
`[profiles.NAME]` without `[sift]`/`[[targets]]`, continue to work:

```toml
[profiles.claude]
style = "xml"
budget = 60000
```

`extensions` and `ignore` as TOML arrays (e.g. `extensions = ["go", "md"]`)
are canonical; comma strings are a CLI concern (`--ext go,md`), not TOML.

## Tuning selection (`[scoring]`)

Any `[sift]`, profile, or target may carry a `scoring` table. Unset keys keep
built-in defaults; `retention` overrides per-role retention under budget
(config priority over language default):

```toml
[profiles.min]
scoring = { retention = { docs = 0.0 } }        # don't guarantee docs under budget

[profiles.lean]
[profiles.lean.scoring]
recency_weight = 0.40
churn_weight   = 0.20
centrality_weight = 0.20
role_weight    = 1.0
full_band      = 0.55
skip_band      = 0.20
base_min       = 0.01
base_max       = 1.0
relevance_weight = 0.20
preference_bonus = 0.05
signature_bonus  = 0.05
skip_multiplier  = 0.35
sig_quality_min  = 0.35
sig_quality_max  = 0.85
test_task_boost  = 0.20
area_diminishing = 0.65
area_budget_share = 0.35
area_roots       = { "internal/ui" = "frontend", "pkg/api" = "public-api" }
skip_roles       = ["test", "fixture", "mock", "generated", "vendor"]
retention        = { entrypoint = 0.30, docs = 0.22, config = 0.18, api = 0.12 }
```

Top-level `[scoring]` is also honored. The default `skip_roles` are tests,
fixtures, mocks, generated files, and vendored code. With the default policy,
test files become eligible and receive `test_task_boost` when the task prompt
mentions tests, bugs, regressions, coverage, or fixing an issue. An explicit
`skip_roles` list replaces the defaults and always takes precedence.
Automatic selection first reserves strong prompt matches and an affordable
representative from each architectural area, then enforces a soft per-area
budget ceiling and applies diminishing utility to later files in the same
area. Role-based retention is limited to two reserved files per area, while
recent implementation files get at most one reservation per area.

Areas default to the first two path components (`internal/app`, `cmd/sift`);
root files and files directly under a top-level directory share their
respective areas. `area_diminishing` controls the multiplier for the second
file and repeated later files (default `0.65`, with a floor of `0.20`).
`area_budget_share` sets the per-area budget ceiling (default `0.35`; the
effective share is raised when there are few areas). `area_roots` maps
directory prefixes to custom area names; the longest matching prefix wins.

## Settings reference

### Output

| Flag | Default | Purpose |
| --- | --- | --- |
| `--output` | `codebase.md` | Output file; `-` writes to stdout |
| `--style` | `markdown` | `plain`, `markdown`, `json`, or `xml` |
| `--mode` | `full` | `full` content or `signatures` outline |
| `--clipboard` | off | Copy the rendered output to the system clipboard |
| `--target` | — | Target from `.sift.toml` (`--target NAME`) |
| `--profile` | — | Profile from config (`--profile NAME`) |

### Context and tokens

| Flag | Default | Purpose |
| --- | --- | --- |
| `--budget` | `0` | Maximum output tokens; `0` is unlimited |
| `--tokenize-model` | `cl100k_base` | BPE encoding used for counting |
| `--prompt`, `-p` | — | Task directive prepended to the output |

### Filtering

| Flag | Default | Purpose |
| --- | --- | --- |
| `--ext` | — | Only include these extensions (comma-separated) |
| `--ignore` | — | Extra gitignore-style patterns (comma-separated) |
| `--max-size` | `0` | Max file size in MB; `0` is unlimited |
| `--hidden` | `true` | Ignore hidden files; set `false` to include them |
| `--git` | `true` | Ignore Git-ignored files; set `false` to include them |
| `--binary` | off | Include binary files (skipped by default) |
| `--smart` | off | Skip generated/lock/minified/oversized files |
| `--smart-max-tokens` | `15000` | Per-file token ceiling for the smart filter |

### Safety

| Flag | Default | Purpose |
| --- | --- | --- |
| `--secrets` | `true` | Scan and redact detected credentials |
| `--force-secrets` | off | Include secrets instead of redacting |

### Terminal display

| Flag | Default | Purpose |
| --- | --- | --- |
| `--highlight` | `true` | Terminal syntax highlighting |
| `--theme` | `auto` | Highlight palette: `auto`, `none`, `dark`, `light` |
| `--ui-theme-file` | — | TOML file containing custom interactive picker themes |
| `--no-color` | off | Disable color output |
| `--window-title` | — | Set the picker's terminal title |
| `--no-window-title` | off | Disable terminal title updates |
| `--no-nerd-fonts` | off | Plain ASCII glyphs in the picker |

### Scan behavior

| Flag | Default | Purpose |
| --- | --- | --- |
| `--dir` | `.` | The root directory to scan |
| `--concurrent` | `true` | Concurrent file processing |
| `--workers` | CPU count | Number of concurrent workers |
| `--timeout` | — | Maximum execution time (e.g. `30s`, `5m`) |
| `--show-skipped` | off | List excluded files and reasons |
| `--progress` | off | Show progress information |

### Diagnostics

| Flag | Default | Purpose |
| --- | --- | --- |
| `--verbose` | off | Debug-level logging |
| `--quiet` | off | Only warn/error logging |
| `--log-level` | — | Set the logging level explicitly |

## Interactive theme vs. syntax theme

Two related but separate settings:

- `--theme` controls **terminal syntax highlighting** of rendered output
  (`auto`, `none`, `dark`, `light`). Color output automatically degrades for
  truecolor, ANSI256, ANSI16, and no-color terminals.
- The picker's **interactive color palette** (chosen with `t` in the TUI) is
  stored separately under the user's application configuration directory and
  reloaded on the next picker session. Inside the picker, the active palette
  also drives preview syntax highlighting; `--theme` still governs
  non-interactive output. The built-in `Terminal (Emulator)` palette uses the
  terminal's configured ANSI colors rather than fixed RGB values.

### Custom picker themes

Pass a TOML file with `--ui-theme-file` (or set `ui_theme_file` in `.sift.toml`):

```toml
[[theme]]
id = "midnight"
name = "Midnight"
extends = "catppuccin-mocha"
border = "#112233"
title = "#abcdef"

[[theme]]
id = "midnight-soft"
name = "Midnight Soft"
extends = "midnight"
selected = "#89b4fa"
```

Each theme requires a stable `id` and display `name`. Themes may inherit from a
built-in theme or an earlier definition in the same file. The file is loaded only
by the interactive picker; it does not change the `--theme` setting used by
non-interactive output. Theme IDs are persisted across sessions, while older
saved display names remain supported.

## Where state lives

Per-project dump state (used by `delta`) and interactive preferences are kept
outside the scanned repository — by default under the application
configuration directory (e.g. `~/.config/sift/`). Nothing is written into your
project, so the context document and repo stay clean.
