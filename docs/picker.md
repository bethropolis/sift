# Interactive picker

The picker is a dual-pane terminal UI for curating context before generation: a
foldable file tree on the left, a live preview on the right, and a footer with
the selection total, a budget bar, and scan progress.

Launch it with bare `sift` or `sift pick [path]`:

```sh
sift
sift pick ./src
```

> The picker needs an interactive terminal. Bare `sift` shows help instead when
> stdin is not a terminal (for example, when piping). Use `sift pick` from a
> real terminal.

## Layout

- **Tree (left)** — navigate files and directories, expand/collapse folders,
  and toggle selection. Each row shows its mode (`FULL`/`SIGS`/`SKIP`) and
  token count.
- **Preview (right)** — the highlighted file's content with syntax
  highlighting and secret warnings where relevant.
- **Footer** — the active style, visibility state, selection count and token
  total, a live budget bar, and the scanning progress line.

Press `Tab` to move keyboard focus between the tree and preview panes. When the
preview has focus, `j`/`k` and the arrow keys scroll it instead of moving the
cursor.

## Keyboard reference

### Navigation and tree

| Key | Action |
| --- | --- |
| `j`/`k`, `Up`/`Down` | Move the cursor |
| `h`/`l`, `Left`/`Right` | Collapse / expand a directory |
| `Enter` | Expand a folder, or toggle a file's selection |
| `E` / `C` | Expand all / collapse all |
| `.` | Toggle hidden files in the tree |
| `H` | Toggle Git-ignored files in the tree |
| `Tab` | Switch focus between the tree and preview panes |

### Selection and context density

| Key | Action |
| --- | --- |
| `Space` | Select / deselect the current file or folder |
| `m` | Cycle a file's mode: `FULL` → `SIGS` → `SKIP` |
| `s` | Smart-select files by Git history and budget |
| `a` | Select all / deselect all |
| `d` | Open the incremental delta modal |
| `t` | Open the color-theme selector |
| `p` | Open the prompt / task-directive builder |

### Generation and actions

| Key | Action |
| --- | --- |
| `g` | Generate the output document (stay in the picker) |
| `y` | Copy the selection to the clipboard |
| `/` | Fuzzy path search (filter the tree) |
| `PgUp`/`PgDn`, `[`/`]` | Scroll the preview pane |
| `J` / `K` | Scroll the preview one line at a time |
| `?` | Toggle the in-app help overlay |
| `q` / `Esc` | Exit (`Esc` first clears an active filter) |
| `r` | Rescan the tree (refresh files and git history) |

## Selecting files and directories

- Selecting a directory applies the selection to all its children.
- A directory's checkbox reflects the aggregate state of its children.
- Select a useful subset by hand, press `a` for everything, or press `s` for a
  Git-informed smart selection.

## Modes (context density)

Each file balances detail against token cost:

- **FULL** — the file's full content is included.
- **SIGS** — a signature-only summary keeps declarations, imports, and useful
  comments while dropping implementation bodies.
- **SKIP** — the file is excluded from the output.

Press `m` on a selected file to cycle its mode. The picker can mix modes
freely; the footer's token tally and budget bar update live.

## Smart select

`s` chooses a selection automatically from Git history and commit size. Working
tree edits and small, focused commits favor `FULL`; large bulk commits favor
`SIGS`. The result is shaped by your current `--budget`.

## Filtering and visibility

- Press `/` to fuzzy-filter the tree by path. Type to narrow, `Enter` to
  commit, `Esc` (or `q`) to clear the filter.
- `.` reveals hidden (dotfile) entries and `H` reveals Git-ignored entries.
  These are display toggles inside the picker; what ends up in the output
  depends on your selection. Hidden and Git-ignored files are shown muted
  (dimmed) so they stand out from tracked files; hovering or selecting one
  restores full emphasis so it stays easy to work with.

## Task prompts

Press `p` to open the directive builder. It offers presets — such as code
review, refactoring, bug investigation, unit tests, and architecture
explanation — plus a custom mode (`Tab` or `c`) for free-form
instructions. The chosen prompt is prepended to generated or copied output.

## Color themes

Press `t` to open the theme selector. The built-in catalog includes Catppuccin
Mocha, Tokyo Night, Dracula, Gruvbox Dark and Light, Nord, Rosé Pine, Kanagawa
Wave/Dragon/Lotus, Everforest Dark and Light, One Dark, Solarized Dark and
Light, Monokai, GitHub Dark and Light, Night Owl, Poimandres, Classic/default,
and Terminal (Emulator). Terminal (Emulator) uses the configured ANSI palette
(including the terminal's foreground/background colors) instead of fixed RGB
colors. Your selection applies immediately and is saved for future sessions. The
theme also recolors preview syntax highlighting to match,
so keywords, strings, comments, diffs, and markup follow the same palette as
the rest of the picker.

Use `--ui-theme-file` to load custom TOML themes. See
[Configuration and profiles](configuration.md#custom-picker-themes) for the file
format and inheritance rules. If your terminal does not show the default Nerd
Font glyphs, run with `--no-nerd-fonts` to use plain ASCII icons.

## Incremental deltas from the picker

Press `d` to open the delta modal. It lets you choose the comparison range (by
checking/unchecking commits), the strategy (`FULL` content vs a raw `--patch`
stack), and whether to generate a document or copy it. See
[Incremental deltas](delta.md).

## Generating and copying

- `g` renders the current selection to the configured output destination
  (default `codebase.md`) without leaving the picker, so you can keep tweaking.
- `y` copies the rendered selection to the clipboard.

Both include the active task prompt and apply secret redaction.
