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
retention        = { entrypoint = 0.30, docs = 0.22, config = 0.18, api = 0.12 }
```

Top-level `[scoring]` is also honored.

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
  reloaded on the next picker session. Inside the picker, the active palette
  also drives preview syntax highlighting; `--theme` still governs
  non-interactive output.

## Where state lives

Per-project dump state (used by `delta`) and interactive preferences are kept
outside the scanned repository — by default under the application
configuration directory (e.g. `~/.config/sift/`). Nothing is written into your
project, so the context document and repo stay clean.
