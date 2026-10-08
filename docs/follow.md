# Following imports

`sift follow <file>` renders one file plus the files connected to it through
imports, followed transitively. Point it at a file you are about to change to
see its dependents ("what could break?"), or at a file you are trying to
understand to see its dependencies ("what does this need?").

```bash
sift follow internal/api/router.go
sift follow internal/api/router.go --depth 3 --budget 60000
sift follow internal/api/router.go --direction both
```

## Directions

Given `A -> B -> C` (A imports B, B imports C), pointing at `B`:

| Direction | Result |
| :-- | :-- |
| `deps` | `C` — files `B` needs. |
| `dependents` (default) | `A` — files that need `B`. |
| `both` | `A` and `C` — the union of the two walks from every visited node. |

## Depth and modes

| Flag | Default | Meaning |
| :-- | :-- | :-- |
| `--direction deps\|dependents\|both` | `dependents` | Which way to walk the import graph. |
| `--depth N` | `2` | Max hops from the seed. `-1` is unlimited, `0` renders the seed only. |
| `--full-depth N` | `1` | Files within N hops render in full; farther files render as signatures. `-1` renders everything in full. |

The seed itself always renders in full. Everything else enters the usual
budget optimizer with its assigned mode as a preference, so a tight budget
downgrades far hops first and keeps the seed. If the seed alone does not fit
the budget, `follow` fails instead of emitting a downgraded seed.

A one-line summary goes to stderr even when the document goes to a file:

```
follow: internal/api/router.go, dependents, depth 2: 17 files (hop 1: 5, hop 2: 12), 3 not analyzed (no resolver)
```

## Supported languages

Go, TypeScript/TSX, and JavaScript resolve imports to confirmed files
(relative specifiers, extension and `index.*` probing; Go package imports
expand to the package's files). Python and other languages fall back to
generic prefix expansion. Files in any other language count toward the "not
analyzed" number in the summary.

## Limits

- Only languages with an import resolver are analyzed; the rest of the tree
  is invisible to the walk.
- Dynamic imports and reflection are invisible.
- Go resolves at package level: dependents of a Go file include importers of
  its whole package, and files in the same package are not dependents of
  each other.
- No `tsconfig` `paths`, Webpack aliases, or Python `sys.path` tricks.
  Relative `@/`/`~/` specifiers in JS/TS do resolve.
- Works identically with and without the cgo build. Without cgo, far files
  render in full instead of signatures.

See also [Dependency expansion](dependencies.md), which auto-includes imports
of whatever `select` (or the picker) chose.

## In the TUI, web UI and select

In the picker, `f` follows dependents from the cursor file and `F` follows
dependencies, replacing the selection with the walk.

Right-click a supported file in the explorer and pick **Show dependents** or
**Show dependencies**: the selection becomes exactly what the CLI would pick
(seed full, near hops full, far hops signatures), with a banner naming the
seed, direction, and hop counts. **Clear** restores the previous selection.

`sift select --follow <file>` (plus `--follow-direction`, `--follow-depth`,
`--follow-full-depth`) restricts a select run to one walk, so the selection
report shows what follow picks. Each decision's signals carry the hop
`distance` (visible in `json`/`ndjson` reports; the seed itself is distance
0), and the walk summary prints to stderr ahead of the report.
