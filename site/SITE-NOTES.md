# Site notes

Jekyll site for sift: landing page, install page, and a `/docs/` section
generated from the repo's `docs/*.md`. Plain-docs style (paper background,
mono display type, one amber accent). No theme gem. Deployed to
`bethropolis.github.io/sift` via `.github/workflows/pages.yml` (GitHub
Actions source).

## Docs section: how it works

`docs/*.md` is the single source of truth — never hand-edit generated docs
pages. Before the Jekyll build, `scripts/sync-docs.sh <dest>` copies them
into `<dest>/_docs/` with front matter (title from the first `# H1`,
description from the first paragraph, prev/next links) and rewrites
repo-relative links to `/docs/` permalinks:

- `](name.md)` → the `/docs/name/` page
- `](../justfile)` → the GitHub blob URL
- `](../README.md)` → the site root
- `](assets/...)` → `/assets/docs/...` (copied from `docs/assets/`)

Sidebar order and the `/docs/` landing cards come from
`site/_data/docs_nav.yml`, which mirrors the grouping in `docs/README.md`.
The `docs` collection (`output: true`, `permalink: /docs/:path/`) and the
`layout: docs` default live in `site/_config.yml`.

## Before you push this

- **`install.sh` must live at the site root** (`/install.sh`), because the
  curl command on both pages points at
  `https://bethropolis.github.io/sift/install.sh`. The deploy workflow
  copies `scripts/install-online.sh` there — don't break that step.
- Update `latest_release_url` in `_config.yml` if it ever changes.
- The hero eyebrow (`v1.2.0 released` in `index.html`) is manual — bump it
  on each release. Same for the version pill baked into
  `assets/img/og.png` (source SVG is not committed; regenerate with
  rsvg-convert at 1200×630 if you want it current).

## Local preview

```bash
just site-serve
```

This mirrors the Pages build (copies `site/` to `.site-preview/`, syncs
docs, stages the installer) and serves `http://localhost:3000/sift/`, bound
to `0.0.0.0` so it's also reachable from other devices on your network.
Or step through it manually:

```bash
bash scripts/sync-docs.sh site   # writes site/_docs/ (gitignored)
cd site && bundle exec jekyll serve
```

## Authoring notes

- Raw HTML in docs markdown is verbatim to kramdown (GFM): fenced code and
  other markdown inside elements like `<details>` stays unparsed. Add
  `markdown="1"` to the tag (e.g. the client-setup disclosures in
  `docs/mcp.md`) so kramdown processes the children; GitHub ignores the
  attribute and renders the same markdown fine.

## What's deliberately not here
```

## What's deliberately not here

- No search: seven pages don't need it yet.
- No JS framework. The install-method tabs are CSS-only (radio inputs);
  the docs sidebar and the mobile nav collapse via `<details>`; the only
  JS (`assets/js/site.js`) is the copy-button enhancement plus closing the
  mobile nav sheet when a link is chosen or Escape is pressed.
- Fonts are self-hosted (`assets/fonts/`, latin subsets of IBM Plex Mono
  + Sans with `font-display: swap`) — no third-party requests at all.
