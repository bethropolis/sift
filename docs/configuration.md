# Configuration and profiles

Command-line flags configure a single run. **Profiles** package a set of
settings for reuse across projects.

## Profiles

Create a `.sift.toml` in the scanned directory (project-local), or a global
`~/.config/sift/config.toml`:

```toml
[profiles.review]
style = "markdown"
budget = 60000
mode = "signatures"
secrets = true
prompt = "Review this codebase for correctness and security issues."
extensions = ["go", "md"]

[profiles.docs]
style = "xml"
highlight = true
```

Select a profile with:

```sh
sift dump . --profile review
```

### Precedence

1. **Explicit command-line flags** win.
2. Then the selected profile's values.
3. Then built-in defaults.

To apply a profile automatically when `--profile` is omitted, set
`default_profile` in the global configuration file.

### Tuning selection (`[scoring]`)

A profile may carry a `scoring` table that tunes relevance scoring and the
token-budget optimizer. Unset keys keep the built-in defaults, so only the
knobs you want to change need to be listed. `retention` overrides how strongly
a role is kept under budget; config takes priority over the language default.

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
retention        = { entrypoint = 0.30, docs = 0.22, config = 0.18, api = 0.12 }
```

## Settings reference

### Output

| Flag | Default | Purpose |
| --- | --- | --- |
| `--output` | `codebase.md` | Output file; `-` writes to stdout |
| `--style` | `markdown` | `plain`, `markdown`, `json`, or `xml` |
| `--mode` | `full` | `full` content or `signatures` outline |
| `--clipboard` | off | Copy the rendered output to the system clipboard |

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
  (`auto`, `none`, `dark`, `light`).
- The picker's **interactive color palette** (chosen with `t` in the TUI) is
  stored separately under the user's application configuration directory and
  reloaded on the next picker session.

## Where state lives

Per-project dump state (used by `delta`) and interactive preferences are kept
outside the scanned repository — by default under the application
configuration directory (e.g. `~/.config/sift/`). Nothing is written into your
project, so the context document and repo stay clean.
