---
name: sift-project-dotfile
description: Inspect a software repository and create or revise a project-specific .sift.toml that produces useful, safe, budget-aware context dumps for coding agents. Use when asked to configure Sift for a codebase, create a Sift project dotfile, or improve an existing .sift.toml.
---

# Sift Project Dotfile Authoring

Create a repository-specific `.sift.toml` that makes `sift dump` useful immediately, without requiring users or agents to remember a long list of flags.

The goal is **high-signal project context, not the largest possible file dump**. Preserve the source, tests, interfaces, build configuration, schemas, protocols, and deployment details that explain how the project works. Exclude confirmed noise, protect secrets, and make every filtering decision explainable.

## 1. Operating defaults and user preferences

Apply explicit user preferences first. For the user this skill was created for, use these defaults unless a request says otherwise:

| Setting | Preferred default | Reason |
|---|---|---|
| Full-dump budget | `150000` tokens | Large enough for substantial project context without being unlimited |
| Output style | `xml` | Established preference in prior project configurations |
| Compression | `full` | Preserve implementation details for coding, debugging, and refactoring |
| Markdown files | Only repository-root `README.md` and `AGENTS.md` | Avoid pulling every document under `docs/` into normal source context |
| Secret scanning | Enabled | Redact detected credentials before rendering |
| Hidden files | Ignore by default, but inspect the tree first | Hidden directories may contain important CI and project configuration |
| Git-ignored files | Ignore | Avoid local build output and untracked machine-specific state |
| Smart filter | Usually disabled for the full dump | Keep tests and less-common implementation files available |
| Tests | Include unless there is a strong, documented reason not to | Tests explain invariants, regressions, and expected behavior |

These are preferences, not universal truths. If another user or repository has different explicit needs, follow those instead.

If no output-style preference exists, Sift's current documentation describes Markdown as its default all-purpose format. XML is useful when a stable structured wrapper is preferred. Use JSON only when a downstream program needs to parse the result; use plain output when minimal wrapping or diffability is the priority.

## 2. Gather evidence before writing TOML

Do not generate a generic config from the project name alone. First identify the repository root and inspect the evidence available.

### 2.1 Inspect the tree

Read the supplied tree dump or list the repository. Identify:

- Primary languages and frameworks.
- Entry points and executable targets.
- Architectural boundaries: packages, modules, layers, services, screens, repositories, adapters, etc.
- Tests, fixtures, mocks, snapshots, and test data.
- Build manifests, lockfiles, toolchain files, CI workflows, and task runners.
- Data contracts: schemas, migrations, API specifications, protocol definitions, generated clients, serialization models.
- Deployment and runtime files: Dockerfiles, Compose manifests, service units, package scripts, installers, launchers, and environment examples.
- Platform-specific implementations and build constraints.
- Generated, vendored, cached, or machine-local directories.
- Hidden files and directories such as `.github/`, `.build/`, `.config/`, `.idea/`, and `.vscode/`.
- Existing output files from Sift or other context-dump tools.

A tree reveals names and relationships, not implementation behavior. Treat it as an orientation map, not proof of runtime semantics.

### 2.2 Read the highest-value files

When file contents are available, inspect a small, purposeful set before writing the prompt and filters:

1. Root `README.md` and `AGENTS.md`, if present.
2. Main build manifest and toolchain declaration (for example `go.mod`, `Cargo.toml`, `package.json`, `settings.gradle.kts`, `pyproject.toml`).
3. Application or CLI entry point.
4. Architecture overview and key design documents, while respecting any user-specified Markdown restriction.
5. Main dependency-injection, routing, navigation, or composition root.
6. Representative core implementation and its tests.
7. Database schema/migrations, API contracts, protocol files, or serialization definitions.
8. CI/build/deployment configuration if it affects development or runtime behavior.

If only a tree is supplied, say so in the prompt or final note. Do not imply that architecture documents or source implementation were read when only their filenames were visible.

### 2.3 Build a compact project map

Before writing the prompt, make a short internal map:

- **What the project is:** one sentence supported by the available evidence.
- **Entry points:** where execution begins.
- **Core flow:** how requests or user actions move through the system.
- **State and persistence:** databases, files, caches, local state, or remote state.
- **External boundaries:** APIs, network services, IPC, devices, operating-system APIs.
- **Cross-cutting concerns:** security, concurrency, retries, error handling, logging, configuration, platform differences.
- **Verification:** where relevant tests and fixtures live.
- **Operational surface:** how it is built, packaged, installed, and run.

Use this map to write the project-specific prompt. Do not copy a package map from another project.

## 3. Understand the Sift configuration model

The project config file is `.sift.toml`, located at the root of the directory being scanned. When running `sift dump /path/to/repo`, Sift resolves the config relative to that scanned repository, not simply the caller's current working directory. You can also pass `--config /path/to/file.toml` to select an alternate config explicitly.

Sift configuration has several layers:

1. Built-in defaults.
2. Global `~/.config/sift/config.toml` settings.
3. Local `.sift.toml` `[sift]` defaults.
4. `default_profile`, when configured.
5. Selected profile, including its `extends` chain.
6. Selected `[[targets]]` overrides.
7. Explicit CLI flags, which win over config values.

### 3.1 Put top-level keys first

Bare top-level keys such as `default_profile` must appear before the first TOML table header. TOML assigns later bare keys to the currently open table; a misplaced `default_profile` can silently become `profiles.some_profile.default_profile` and fail to activate the default profile.

Correct:

```toml
default_profile = "full"

[sift]
style = "xml"
```

Incorrect:

```toml
[profiles.full]
style = "xml"

default_profile = "full" # This is now inside [profiles.full].
```

### 3.2 Profile inheritance, `ignore`, and `override_ignore`

Profiles automatically inherit ignore patterns from `[sift].ignore`. A profile can add more patterns to that inherited set. It does not need a special base profile merely to retain repository-wide exclusions.

```toml
[sift]
ignore = [
  "**/build/**",
  "**/.gradle/**",
  "**/*.md",
  "!/README.md",
  "!/AGENTS.md",
]

[profiles.tests]
# Inherits the root ignore rules and adds a test-specific exclusion.
ignore = ["**/generated-test-fixtures/**"]

[profiles.raw]
# Explicitly discards [sift].ignore and starts a clean ignore list.
override_ignore = true
ignore = ["node_modules/**"]
```

Use `override_ignore = true` only when a profile genuinely needs an isolated filtering policy. Otherwise, let it inherit `[sift].ignore`. This makes root-level filtering rules easier to maintain and avoids duplicating them across profiles.

`extends` remains useful for composing named profiles: ancestor profiles are overlaid in order, then the profile's own fields win. Use it to share budgets, prompts, compression preferences, or output conventions.

### 3.3 Path-based compression mapping with `[modes]`

Use `[modes]` to assign deterministic dump behavior by path glob without manually opening the picker:

```toml
[modes]
full = [
  "cmd/**",
  "internal/core/**",
  "pkg/domain/**",
  "**/*_test.go",
]
signatures = [
  "internal/ui/**",
  "web/**",
  "generated/**",
]
skip = [
  "**/*.min.js",
  "**/coverage/**",
]
```

- `full`: keep matching files at full fidelity.
- `signatures`: compress matching files to tree-sitter structural signatures.
- `skip`: omit matching files entirely.

Use `[modes]` to preserve high-value implementation and test code in full while reducing bulky peripheral areas to signatures. Use `skip` sparingly: skipping tests, fixtures, migrations, protocol definitions, schemas, or generated contracts can remove important behavioral evidence. Never blanket-skip tests just to make the dump smaller unless the user explicitly asks for that trade-off.

Keep the path lists narrow, repository-specific, and non-overlapping where possible. Do not assume an overlapping glob has a particular precedence unless the installed Sift version's docs or implementation explicitly guarantees it.

`[modes]` is a dump-rendering policy, not a replacement for `ignore` or `extensions`: use `ignore` to exclude unwanted paths from the scan, `extensions` for a deliberately narrow file-type view, and `[modes]` to choose full/signature/skip treatment for paths that are otherwise in scope.

### 3.4 Targets are distinct artifacts

Use `[[targets]]` when one repository needs more than one repeatable view, such as:

- A full implementation context.
- A Markdown-only guidance context.
- A signatures-only architecture overview.
- A focused API, backend, mobile, or deployment context.

A target can choose a profile and override `style`, `budget`, `compress_mode`, `output`, `prompt`/`prompt_file`, `extensions`, `ignore`, and supported scoring fields. Give every target a distinct output filename and exclude those generated outputs from later scans.

In a monorepo, a target can set `config = "services/api/.sift.toml"` to reuse a module's standalone config while writing the artifact at the project root. The module config supplies `[sift]`, profiles, prompts, modes, scoring, and history; the target's own fields still win.

Do not create targets just to demonstrate the feature. Add only targets with a clear recurring use.

## 4. Choose files deliberately

### 4.1 Prefer a broad full-project scan by default

For the primary full-context dump, usually omit `extensions`. Extension allowlists are easy to get wrong because important project files may have no conventional extension.

Examples of files that can be lost by an over-aggressive extension allowlist:

- `Makefile`, `Dockerfile`, `Containerfile`, `Justfile`, `gradlew`.
- `AndroidManifest.xml`, Gradle Kotlin DSL files, and resource XML.
- `.github/workflows/*`, `.gitmodules`, toolchain files, and dotfiles.
- Systemd units, shell scripts, config files, schema snapshots, and protocol definitions.
- Tests, fixtures, and test resources.

Only set `extensions` on a target whose purpose is intentionally narrow, such as a Markdown-only documentation target.

### 4.2 Exclude confirmed noise, not unfamiliar files

Typical candidates for `ignore` (include only what applies to this repository):

```toml
ignore = [
  "**/node_modules/**",
  "**/vendor/**",
  "**/.gradle/**",
  "**/build/**",
  "**/dist/**",
  "**/out/**",
  "**/target/**",
  "**/coverage/**",
  "**/.cache/**",
  "**/tmp/**",
  "**/*.min.js",
  "codebase.xml",
  "guidance-context.xml",
]
```

Before adding a pattern, check that the directory or file is actually generated, vendored, cached, or irrelevant. A directory named `build`, `target`, or `dist` can sometimes contain real project source. A `generated` directory can contain checked-in code that is necessary to understand the project.

Preserve important artifacts even if they look generated:

- Database schema snapshots and migration history.
- API/protocol schemas and generated code that defines a public contract.
- Lockfiles or dependency manifests when reproducibility and version resolution matter.
- Baseline profiles, assets, fixtures, snapshots, and platform-specific variants when they affect behavior.

Do not blanket-exclude all tests for a full dump. Test files are often essential evidence for expected behavior and regression coverage.

### 4.3 Hidden files require an explicit decision

`ignore_hidden = true` filters hidden files and directories. That may remove important repository context such as `.github/workflows`, `.gitlab`, `.build`, `.config`, or tool-specific project settings.

Inspect the tree before choosing:

- Keep `ignore_hidden = true` when hidden paths are only local editor/cache state or are not needed.
- Set `ignore_hidden = false` when hidden project configuration is important, then explicitly ignore confirmed noise such as `.idea/`, `.vscode/`, or local caches.
- Remember that `.git` is always skipped.

Do not claim that CI configuration was included if `.github/` was filtered by `ignore_hidden`.

### 4.4 Git-ignored files

`ignore_git = true` honors repository `.gitignore` rules and is normally appropriate. Inspect `.gitignore` if an apparently important source, fixture, or generated contract is missing. Do not set `ignore_git = false` just to capture local build output; add a narrow exception only when the task requires those files.

### 4.5 Markdown policy: root README and AGENTS only

For this user's established preference, include only the **repository-root** `README.md` and `AGENTS.md` in Markdown-aware dumps. Exclude nested Markdown documents such as `docs/ARCHITECTURE.md`, `docs/README.md`, generated reports, and scratch notes unless the user explicitly requests them.

Put these ordered Git-ignore-style rules directly in `[sift].ignore`. Sift automatically carries them into profiles unless a profile sets `override_ignore = true`:

```toml
ignore = [
  "**/*.md",
  "!/README.md",
  "!/AGENTS.md",
]
```

The `**/*.md` rule excludes Markdown at every depth. The negated, root-anchored patterns restore only the two repository-root files. Sift's ignore matcher is based on Git-ignore semantics and supports `!` negation.

Important caveats:

- Do not exclude a parent directory and then attempt to re-include a file inside it. Git-ignore semantics cannot re-include a file when its parent directory has already been pruned.
- Custom ignore patterns are evaluated before repository `.gitignore` rules, so a custom `!` negation can re-include a file that `.gitignore` excludes (unless a parent directory was pruned before the file was reached). Use this deliberately and verify the result with `sift dump --show-skipped`.
- If the user wants `README.md` and `AGENTS.md` at any depth, remove the leading `/` from the two negations. The default above is root-only.
- A Markdown-only target still needs both `extensions = ["md"]` and the root `[sift].ignore` allowlist. Profiles and targets retain the inherited ignore rules unless explicitly overridden; the extension filter alone includes every Markdown file.
- If neither permitted file exists, do not pretend the guidance target contains documentation. Omit the target or keep it with an explicit empty-source warning, depending on the user's intended workflow.

## 5. Set budgets and compression modes

### 5.1 Budget

For this user, set the primary full-project budget to:

```toml
budget = 150000
```

A Sift budget of `0` means unlimited; do not use `0` when the user requested a bounded context size. Budgets are token counts, not file counts or bytes.

Use smaller budgets for narrow targets, for example 8,000–20,000 tokens for project guidance. Choose the number based on how much content the target is intended to include, not just because a smaller number looks tidy.

### 5.2 Progressive fallback compression

With `compress_mode = "full"`, a dump that reaches its budget normally drops remaining files. Configuring `fallback_mode = "signatures"` allows remaining or overflowing files to be represented by structural signatures instead of disappearing entirely once the configured threshold is reached. This preserves broad architectural coverage while keeping already-packed high-priority code in full fidelity.

```toml
[sift]
budget = 150000
compress_mode = "full"
fallback_mode = "signatures"
fallback_threshold = 0.9
```

`fallback_threshold` is a fraction of the budget; `0.9` starts fallback around 90% usage, while `1.0` waits until the budget is reached. Choose the threshold based on the trade-off: a lower value reserves more room for signatures, while `1.0` maximizes full-fidelity content before fallback begins. Do not claim every file is guaranteed to fit: files without a supported tree-sitter signature parser may not compress as expected, and the total output still remains constrained by the token budget. Verify real output on the target repository.

Fallback is often a good complement to a 150k full-mode dump when the user values seeing more of the repository. It is not mandatory: use it when broad coverage is more valuable than having the overflow files omitted, and explain the fidelity trade-off.

### 5.3 Human-readable token units

Token budgets accept numeric values with separators (`150_000`) or human-readable suffixes (`150k`, `1.5M`) in both the CLI and configuration. Both `budget = 150000` and `budget = "150k"` represent the same intended scale. Prefer the representation the project maintainers find clearest; never confuse token count with bytes or file count.

### 5.4 Compression

Sift supports `full`, `signatures`, and `skip` modes in dump rendering:

- `full` preserves implementation bodies. Prefer it for debugging, security review, refactoring, behavioral analysis, or when the agent must reason about exact control flow.
- `signatures` retains supported declarations, imports, and useful comments while dropping implementation bodies. Prefer it for architecture orientation or a lower-cost map of a large codebase.
- `skip` is also available as a path treatment in `[modes].skip`.

For this user, use `compress_mode = "full"` for the main dump. Use `[modes]` when the repository has clear high-value full-fidelity paths or bulky peripheral paths worth reducing. Add a signatures profile/target only when it serves a distinct purpose.

Important command differences:

- `compress_mode` supplies the overall dump mode; `[modes]` provides deterministic per-path modes for `sift dump`.
- `sift select` derives a mode per file from its relevance ranker and does not use `compress_mode` to flatten that selection. Tune `[scoring]` for `select`.
- The interactive picker supports mixed `FULL`, `SIGS`, and `SKIP` modes per file.
- Do not confuse `[modes]` path mapping with `[scoring]`: they solve different problems. `[modes]` maps explicit paths to modes for dump; `[scoring]` tunes automatic selection/picker smart-select.

Signature-mode language support is implementation-dependent and can change. Check the current Sift output-format documentation before assuming a particular language receives syntax-aware signatures; files without a supported parser are emitted in full.

## 6. Smart filtering, tests, and automatic selection

`smart_filter` can skip generated, lock, minified, and oversized files during `dump`. For this user's main full dump, default to `smart_filter = false` so important but unusual files are not silently filtered. Use explicit `ignore` patterns for known build noise.

Sift's `select` and picker smart-select workflows force smart filtering on. They also use role-aware selection: tests, fixtures, mocks, generated files, and vendored files are normally deprioritized or skipped, with test-related prompts able to boost tests unless `skip_roles` explicitly excludes them.

`[scoring]` and `skip_roles` tune `select` and picker smart-select, not `dump`. Do not add scoring weights to a configuration intended only for `sift dump` and imply they affect it. Add scoring only when the user expects to use `sift select` or smart-select.

Use the commands according to the job:

| Command | Best use |
|---|---|
| `sift dump` | Broad, deterministic project context using configured filters and budget |
| `sift select` | Automatically choose a task-relevant subset using Git relevance and role-aware ranking |
| `sift pick` | Manually curate FULL/SIGS/SKIP modes in the interactive TUI |
| `sift follow <file>` | Trace import dependencies or dependents around a specific file |
| `sift diff` / `sift delta` | Focus on changes relative to a Git ref or previous dump |

Do not configure the project as if one command's behavior applies to all the others.

## 7. Write a useful project-specific prompt

The prompt is often more valuable than adding dozens of file globs. It should teach an agent how to interpret the included repository, not replace the source code with assumptions.

A good prompt should:

1. State the project identity and its major verified technologies.
2. Name important architectural layers and entry points found in the tree/source.
3. Explain what to trace end-to-end for feature work.
4. Point out cross-cutting concerns such as persistence, networking, concurrency, lifecycle, permissions, security, and platform-specific behavior where applicable.
5. Tell the agent to use tests, fixtures, schemas, migrations, and protocol definitions as evidence.
6. Tell the agent to distinguish verified behavior from inference.
7. Warn against assuming a file is compiled or active merely because it exists; respect build tags, source sets, feature flags, and platform conditions.
8. Call out gaps in the supplied context rather than inventing missing documentation or behavior.

### Prompt patterns by project type

Use only the relevant patterns; do not paste every category into every prompt.

- **Go services/CLIs:** trace `main` and command registration into packages; follow handlers → services → repositories/adapters; inspect goroutines, contexts, cancellation, error wrapping, interfaces, build tags, `*_linux.go`/`*_windows.go` variants, IPC, and network boundaries.
- **Android/Kotlin:** trace `Application`/`Activity` and navigation into Compose screens, ViewModels, repositories, data sources, Room/DAO/entities, DataStore/preferences, API services, and background work; inspect manifest permissions, Gradle modules, resource qualifiers, lifecycle/state ownership, and migration tests.
- **Web/frontend:** trace bootstrap/routing, page or component state, data fetching, server/client boundaries, API contracts, build tooling, generated assets, and browser/runtime compatibility.
- **Rust/native systems:** trace binaries and library crates, feature flags, traits and ownership boundaries, FFI, unsafe blocks, platform modules, build scripts, and integration tests.
- **Python/data projects:** trace entry points, package boundaries, configuration, data/model pipelines, schemas, serialization, notebooks only when relevant, and reproducible environment definitions.
- **Cross-platform applications:** explicitly compare platform variants and their shared abstractions; do not assume one platform's implementation represents all platforms.

Keep prompts concise enough to remain useful. A prompt should not repeat the entire README or narrate every file in the tree.

## 8. Recommended configuration shape

Use this as a starting structure, then customize it from repository evidence. Remove unused examples and add only real directories, files, and targets.

```toml
# Top-level keys must come before any table.
default_profile = "full"

[sift]
style = "xml"
output = "codebase.xml"
budget = 150000
compress_mode = "full"
smart_filter = false
ignore_hidden = true
ignore_git = true
secret_scan = true
fallback_mode = "signatures"
fallback_threshold = 0.9
ignore = [
  "**/vendor/**",
  "**/node_modules/**",
  "**/.gradle/**",
  "**/build/**",
  "**/dist/**",
  "**/out/**",
  "**/coverage/**",
  "**/.cache/**",
  "**/tmp/**",
  "**/*.min.js",
  "codebase.xml",
  "guidance-context.xml",
  "**/*.md",
  "!/README.md",
  "!/AGENTS.md",
]

[profiles.full]
style = "xml"
output = "codebase.xml"
budget = 150000
compress_mode = "full"
smart_filter = false
prompt = """Describe the project from the included source. Trace behavior through entry points, core layers, persistence, external boundaries, and tests. Treat the file tree as orientation rather than proof. Distinguish verified behavior from inference, and call out missing evidence instead of inventing details."""

# Optional: path-specific modes. Populate only from repository evidence.
[modes]
full = [
  "cmd/**",
  "internal/core/**",
  "**/*_test.go",
]
signatures = [
  "internal/ui/**",
  "web/**",
]

# Add only when repository-root README.md and/or AGENTS.md exist.
[[targets]]
name = "guidance"
profile = "full"
output = "guidance-context.xml"
style = "xml"
extensions = ["md"]
budget = 12000
compress_mode = "full"
prompt = """Summarize only the included repository-root README.md and AGENTS.md. Extract project purpose, documented architecture, development and test commands, conventions, and contribution guidance. Preserve the documents' terminology. Separate explicit statements from inference, and identify what the documents do not explain. Do not imply that excluded nested Markdown documents were read."""
```

This template is not a universal drop-in. In particular, choose `ignore_hidden` after checking hidden project configuration, add only confirmed generated/vendor paths, and replace the generic prompt with the repository's actual architecture.

## 9. Validate the generated configuration

Never stop after writing syntactically plausible TOML. Validate both the file and the behavior it produces.

### 9.1 Validate with `sift check`

Run Sift's built-in validation command to verify TOML syntax, table ordering, profile inheritance, mode mappings, token budgets, and safety constraints:

```bash
sift check
```

You can also validate a specific file or check silently in CI:

```bash
sift check path/to/.sift.toml
sift check --quiet
```

If the Sift binary is not installed in the environment yet, fall back to Python 3.11+ `tomllib`:

```bash
python - <<'PY'
import tomllib
from pathlib import Path

path = Path(".sift.toml")
with path.open("rb") as f:
    config = tomllib.load(f)
print("Valid TOML:", path)
print("Default profile:", config.get("default_profile"))
print("Targets:", [target["name"] for target in config.get("targets", [])])
PY
```

### 9.2 Sift config and output smoke test

Check the installed Sift version and current command help before depending on a command or flag:

```bash
sift --version
sift dump --help
```

Run the configured default dump to a temporary destination so validation does not overwrite a user's existing artifact:

```bash
sift dump . --output /tmp/sift-smoke-test.xml --show-skipped
```

If the project defines a guidance target, verify that separately:

```bash
sift dump . --target guidance --output /tmp/sift-guidance-test.xml --show-skipped
```

Use the output tree and skipped-file report to check the actual result. Confirm that:

- The intended source, entry points, tests, build manifests, schemas, and platform variants are present.
- Important extensionless files were not lost to an extension filter.
- Generated/vendor/cache exclusions are narrow and correct.
- The output file itself is not re-ingested on the next run.
- The guidance target contains only the permitted Markdown files.
- The configured budget is applied and is not accidentally `0` (unlimited).
- Secret scanning remains enabled.

If the Sift binary is unavailable, still parse the TOML and inspect the patterns carefully, but be honest that runtime behavior has not been smoke-tested. Use `sift check` whenever the binary is present.

### 9.3 Verify Markdown allowlisting explicitly

For the guidance target, inspect the emitted directory tree and confirm the only Markdown paths are:

- `README.md`
- `AGENTS.md`

Only list files that actually exist. Nested `docs/*.md` files, nested READMEs, generated context files, and scratch notes must not appear unless the user changes the policy.

## 10. Common mistakes to avoid

- Putting `default_profile` after a table header.
- Adding a base profile solely to carry `ignore` rules when profiles already inherit `[sift].ignore` automatically.
- Setting `override_ignore = true` accidentally and silently discarding repository-wide ignores.
- Forgetting that `[modes].skip` completely omits matched paths, or using it on tests/contracts without considering the loss of evidence.
- Assuming `fallback_mode = "signatures"` preserves full implementation bodies; it deliberately degrades overflow files to structural signatures.
- Using `[modes]` with overlapping globs and relying on undocumented overlap precedence.
- Forgetting that `extensions = ["md"]` includes **all** Markdown files, not only README and AGENTS.
- Using extension allowlists on a full dump and accidentally excluding extensionless build/deployment files.
- Excluding all tests or fixtures from a general context dump.
- Excluding schema snapshots, protocol definitions, lockfiles, or checked-in generated contracts without understanding their purpose.
- Setting `ignore_hidden = true` without checking whether `.github/` or other hidden project files are important.
- Setting `budget = 0` when a bounded budget was requested.
- Assuming `compress_mode` controls `sift select` or that `skip_roles` affects `sift dump`.
- Assuming `[modes]` affects `select` or picker workflows the same way it affects `dump`; verify command-specific behavior rather than transferring semantics between commands.
- Excluding a parent directory and then attempting to re-include a child file with `!`.
- Forgetting to ignore the generated context output, causing it to be included in future dumps.
- Treating Sift's secret redaction as a guarantee. It is best-effort; never deliberately include credentials.
- Writing a project prompt based only on filenames and presenting assumptions as confirmed architecture.
- Claiming a config is validated when only the TOML parser was run and Sift itself was not.

## 11. Delivery requirements

When asked to create a project dotfile:

1. Create `.sift.toml` at the repository root if the repository is accessible; otherwise create it as a downloadable artifact and state where it belongs.
2. Preserve an existing configuration's useful settings unless the user requests a replacement. Inspect before overwriting.
3. Explain the meaningful choices briefly: budget, style, compression, Markdown policy, hidden-file behavior, exclusions, and targets.
4. Include exact commands for the primary dump and any secondary target.
5. State what was actually validated and what could not be tested.
6. Do not claim the project documentation or source contents were reviewed unless they were actually available and read.

When asked to write or improve this skill, keep the current Sift documentation links below up to date and revisit the relevant sections before changing configuration guidance.

## 12. Sift configuration capability checklist

When authoring or revising a dotfile, consider the following capabilities. Use only the features that solve a real repository problem:

| Feature | Configuration / command | When it helps |
|---|---|---|
| Progressive fallback compression | `fallback_mode`, `fallback_threshold` | Preserve broad file coverage after full-fidelity content reaches the token budget. |
| Path-based compression mapping | `[modes]` with `full`, `signatures`, `skip` | Keep core areas/tests full while compressing bulky peripheral code. |
| Human-readable token units | `budget = "150k"`; `--budget 150k` | Make large budgets easier to read. |
| Profile ignore inheritance | `[sift].ignore`, `override_ignore` | Share repository exclusions automatically; opt out only for isolated profiles. |
| Alternate config path | `--config path.toml` | Test or use a separate config without replacing the repository dotfile. |
| Configuration check & validation | `sift check [path]`, `sift config check` | Verify `.sift.toml` syntax, profile inheritance, mode mappings, and safety constraints before dumping. |
| Skill distribution and MCP support | `sift skill ...`, `get_skill`, `skill://sift-project-dotfile` | Discover/install Sift's own usage skills in supported agent environments. This is separate from the project-specific `.sift.toml` being authored here. |

Sift's repository includes its own self-dumping `.sift.toml`. Study it as a practical reference for profile and mode composition, but do not copy its Go-specific globs, extension list, or exclusions into unrelated repositories.

For Sift usage guidance in supported coding-agent environments, Sift provides `sift skill status`, `sift skill install`, `sift skill show`, and `sift skill uninstall`. These manage Sift's bundled skills; `sift skill install` installs the bundled `sift-project-dotfile` skill (this one) by default, and `sift skill show` displays it.

## 13. Canonical references

Use Sift's own documentation and repository configuration as the authority. Re-check these sources because flags, supported signature parsers, and config behavior can evolve.

- [Sift documentation home](https://bethropolis.github.io/sift/)
- [Configuration and profiles](https://bethropolis.github.io/sift/docs/configuration/)
- [Usage and filtering](https://bethropolis.github.io/sift/docs/usage/)
- [Output formats, signatures, prompts, and secret redaction](https://bethropolis.github.io/sift/docs/output/)
- [Dependency expansion](https://bethropolis.github.io/sift/docs/dependencies/)
- [Following imports](https://bethropolis.github.io/sift/docs/follow/)
- [Incremental deltas](https://bethropolis.github.io/sift/docs/delta/)
- [Sift repository README](https://github.com/bethropolis/sift)
- [Sift releases](https://github.com/bethropolis/sift/releases) — release history and feature updates.
- [Sift's own `.sift.toml`](https://github.com/bethropolis/sift/blob/main/.sift.toml) — practical reference for root ignore patterns, `[modes]`, profiles, and project-specific prompts.
- [Sift's Git-ignore matcher dependency](https://github.com/denormal/go-gitignore) — useful background for ordered `!` negation semantics and parent-directory re-inclusion limitations.

Last reviewed against live configuration documentation: **2026-11-16**.
