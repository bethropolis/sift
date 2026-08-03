# Incremental deltas

A full context document can be large and expensive to re-send on every turn.
Delta mode lets an LLM see **only what changed** since a previous dump, instead
of the whole repository.

## How it works

After a successful `dump`, `pick`, `diff`, or `delta`, Sift records a baseline
for that project: the current Git `HEAD` hash, the number of files, and the
token count. `delta` later compares the current Git `HEAD` to that recorded
baseline and renders exactly the files that changed.

- The baseline is keyed by the project's absolute path and stored outside the
  repository (under the application configuration directory, e.g.
  `~/.config/sift/state.json`), so **no state file is added to your repo**.
- Outside a Git repository, delta mode is unavailable; the scan is still
  possible via plain `dump`.

## Commands

```sh
sift delta                 # full content of files changed since the last dump
sift delta --since main    # compare against a specific ref or branch
sift delta --patch         # emit a raw unified diff instead of full content
sift delta --patch --clipboard
```

### Full content (default)

The default strategy renders the full content of every changed file, wrapped in
the same `<context_update>` envelope as the patch strategy. This is useful when
the consumer benefits from complete surrounding context.

### Raw patch (`--patch`)

`--patch` emits the raw unified `git diff` between the baseline and current
`HEAD`, wrapped in a `<context_update>` block with the `from_commit` and
`to_commit` attributes:

```xml
<context_update type="delta_patch" from_commit="abc1234" to_commit="def5678">
<![CDATA[<the unified diff>]]>
</context_update>
```

A patch is dramatically more token-efficient for iterative coding and lets the
consumer apply or review the exact changes.

## From the interactive picker

Inside the picker, press `d` to open the delta modal, which lets you:

- **choose the comparison range** by checking/unchecking commits (`Space`), and
- **choose the strategy** — full changed files or a raw patch (`m` toggles),
- then **generate** (`Enter`) or **copy** (`c`) the result.

## Tips

- Run a fresh `sift dump` after a large review so the next delta starts from an
  up-to-date baseline.
- `--since <ref>` overrides the recorded baseline when you want to compare
  against a specific point (for example a release tag or branch).
- `delta` reports a clear error when no baseline exists yet
  ("no previous dump recorded …") — run `sift dump` first or pass `--since`.
