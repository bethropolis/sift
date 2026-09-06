# Dependency expansion

`sift select`, the picker's generate/quit-and-render paths, and the MCP
`pack_context` tool automatically pull in the dependencies of whatever was
selected: after the budget optimizer picks the review set, sift walks the
import graph outward and re-runs the optimizer over the union — first-pass
picks keep their chosen modes as preferences, dependencies enter biased
toward `signatures`. One budget, one optimization, so dependencies compete
fairly instead of starving on post-upgrade crumbs.

## How it works

During ranking, sift already parses every collected file's imports for
fan-in centrality. That same pass now retains a `file → targets` adjacency
list, so dependency expansion costs no second walk. Language drivers resolve
their own import syntax to confirmed, collected files:

- **Go, JavaScript, TypeScript** commit to real files (relative imports,
  `@/`/`~/` aliases, extension and `index.*` fallbacks; Go package imports
  expand to the package's collected files).
- **All other languages** fall back to generic prefix expansion — correct
  enough, and nothing regresses.

Import cycles are broken with a visited set.

## Tuning

| Flag | Default | Meaning |
| --- | --- | --- |
| `--max-depth N` | `2` | How many import hops to follow from the selected set. `0` disables expansion, `-1` follows without limit (the token budget still caps spend). |

Dependencies enter the optimizer biased toward `signatures` mode — they're
context, not the thing under review — and the optimizer decides the rest
within the leftover budget.
