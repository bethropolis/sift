# Site notes

This is a from-scratch Jekyll rebuild of the sift site: a landing page and
an install page, in a plain-docs style (paper background, mono display type,
one amber accent). No theme gem, so it doesn't inherit the generic Cayman /
Minima look.

## Before you push this

- **`install.sh` must live at the site root** (`/install.sh`, next to
  `index.html`), because the curl command on both pages points at
  `https://bethropolis.github.io/sift/install.sh`. Copy your existing
  script into this repo's root, unchanged. Jekyll passes files it doesn't
  process straight through to `_site/`, so it'll keep working as-is.
- Update `latest_release_url` and `repository` in `_config.yml` if either
  ever changes.
- The nav/footer link to `docs/README.md` in the main repo rather than
  duplicating the full docs here, since this build only covers the landing
  page and install instructions. If you want a real docs section later,
  it's a straightforward add: new pages under `docs/`, plus a sidebar
  include.

## Local preview

```bash
bundle install
bundle exec jekyll serve
```

Visit `http://localhost:4000/sift/` (the `baseurl` in `_config.yml` means it
won't serve from `/`).

## Deploying

Two options, pick one:

**A. GitHub Actions (recommended)** — add
`.github/workflows/pages.yml` (included in this bundle), then in the repo's
Settings → Pages, set Source to "GitHub Actions." Every push to `main`
rebuilds and deploys.

**B. Classic branch build** — if Pages is set to build from a branch
(e.g. `gh-pages`), just commit this content to that branch. GitHub runs
Jekyll itself; no workflow file needed. `Gemfile` already pins
`github-pages` so the local gem versions match what GitHub builds with.

## What's deliberately not here

- No search, no sidebar nav, no multi-level docs tree, since the brief was
  landing page + install only.
- No JS framework. The install-method tabs are CSS-only (radio inputs), so
  they still work with JavaScript disabled; the only JS is the copy-button
  enhancement.
