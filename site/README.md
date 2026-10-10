# Site

Astro 7 static site for sift: landing page, install page, and a `/docs/`
section generated from the repo's `docs/*.md`. Deployed to
`bethropolis.github.io/sift` via `.github/workflows/pages.yml`.

## Docs section: how it works

`docs/*.md` is the single source of truth — never hand-edit generated docs
pages. Before dev or build, `bun site/scripts/sync-docs.mjs` copies them into
`site/src/content/docs/` with front matter (title from the first `# H1`,
description from the first paragraph) and rewrites repo-relative links to
`/sift/` paths. Generated files are gitignored. Doc-local assets
(`docs/assets/*`) are copied to `site/public/assets/docs/`.

Sidebar order and the `/docs/` landing cards come from
`site/src/data/docs.ts`, which mirrors the grouping in `docs/README.md`.

## Local work

```sh
just site-serve   # sync docs, then `astro dev` on :3000
just site-build   # sync docs, `astro build`, stage install.sh like Pages
```

Directly: `cd site && bun install && bun scripts/sync-docs.mjs && bun run dev`.
