#!/usr/bin/env bash
# Sync docs/*.md into a Jekyll site source as the `docs` collection.
#
#   bash scripts/sync-docs.sh [dest]
#
# `dest` is the site source directory (default: `site/`). In CI this is the
# `dist/` staging dir that gets copied from `site/` before the Jekyll build,
# so generated files never pollute the real source tree.
#
# For each markdown file in docs/ the script:
#   - extracts the first `# H1` as `title` and the first body paragraph as
#     `description` (jekyll-seo-tag picks both up),
#   - prepends YAML front matter (layout: docs, nav_order, permalink),
#   - rewrites repo-relative links so they resolve under /docs/:path/.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DEST="${1:-site}"
case "$DEST" in
  /*) SITE_DIR="$DEST" ;;
  *)  SITE_DIR="$REPO_ROOT/$DEST" ;;
esac
DOCS_SRC="$REPO_ROOT/docs"
DOCS_DEST="$SITE_DIR/_docs"

# slug -> nav order is 1-based position in SLUGS below.
REPO_URL="https://github.com/bethropolis/sift"

rm -rf "$DOCS_DEST"
mkdir -p "$DOCS_DEST"

# Copy any doc-local assets (e.g. docs/assets/*.png) so relative
# `](assets/...)` references keep working once rewritten.
if [ -d "$DOCS_SRC/assets" ]; then
  mkdir -p "$SITE_DIR/assets/docs"
  cp -R "$DOCS_SRC"/assets/. "$SITE_DIR/assets/docs/"
fi

# Canonical nav order (matches docs/README.md grouping).
SLUGS=(index installation usage picker configuration output delta dependencies follow mcp)

declare -A TITLES
for slug in "${SLUGS[@]}"; do
  if [ "$slug" = "index" ]; then src_file="$DOCS_SRC/README.md";
  else src_file="$DOCS_SRC/$slug.md"; fi
  TITLES[$slug]="$(grep -m1 '^# ' "$src_file" | sed 's/^# //')"
done

for idx in "${!SLUGS[@]}"; do
  slug="${SLUGS[$idx]}"
  if [ "$slug" = "index" ]; then src="$DOCS_SRC/README.md";
  else src="$DOCS_SRC/$slug.md"; fi
  if [ "$slug" = "index" ]; then
    permalink="/docs/"
  else
    permalink="/docs/$slug/"
  fi
  prev_i=$((idx - 1)); next_i=$((idx + 1))
  prev=""; next=""
  if [ "$prev_i" -ge 0 ]; then
    ps="${SLUGS[$prev_i]}"
    prev="$ps|${TITLES[$ps]}"
  fi
  if [ "$next_i" -lt "${#SLUGS[@]}" ]; then
    ns="${SLUGS[$next_i]}"
    next="$ns|${TITLES[$ns]}"
  fi

  python3 - "$src" "$DOCS_DEST/$slug.md" "$slug" "$permalink" "$((idx + 1))" "$REPO_URL" "$prev" "$next" <<'EOF'
import re, sys

src_path, dest_path, slug, permalink, order, repo_url, prev, nxt = sys.argv[1:9]
text = open(src_path, encoding="utf-8").read()
lines = text.split("\n")


def yq(s):
    return '"' + s.replace("\\", "\\\\").replace('"', '\\"') + '"'

# Title: first `# H1`.
title = slug
body_start = 0
for i, line in enumerate(lines):
    m = re.match(r"^#\s+(.*)", line)
    if m:
        title = m.group(1).strip()
        body_start = i + 1
        break

# Description: first non-empty body paragraph that is not a heading,
# blockquote, list item, table row, or fenced block.
description = ""
para: list[str] = []
in_fence = False
for line in lines[body_start:]:
    stripped = line.strip()
    if stripped.startswith("```"):
        in_fence = not in_fence
        continue
    if in_fence:
        continue
    if not stripped:
        if para:
            break
        continue
    if re.match(r"^(#{1,6}\s|>\s*|[-*+]\s|\d+\.\s|\||<)", stripped):
        if para:
            break
        continue
    para.append(stripped)
description = " ".join(para)
# Strip markdown links/images/code, collapse whitespace, cap length.
description = re.sub(r"!\[([^\]]*)\]\([^)]*\)", r"\1", description)
description = re.sub(r"\[([^\]]*)\]\([^)]*\)", r"\1", description)
description = description.replace("`", "")
description = re.sub(r"\s+", " ", description).strip()
if len(description) > 150:
    description = description[:147].rsplit(" ", 1)[0] + "..."

# The index page becomes link cards (rendered from _data/docs_nav.yml), so
# drop README's own link lists and keep the intro + cheat-sheet + closer.
if slug == "index":
    try:
        gs = next(i for i, l in enumerate(lines) if l.strip() == "## Getting started")
        cs = next(i for i, l in enumerate(lines) if l.strip() == "## Command cheat-sheet")
        lines = lines[:gs] + lines[cs:]
    except StopIteration:
        pass

body = "\n".join(lines[body_start:])

# Rewrite doc-to-doc links: ](name.md) / ](name.md#frag) -> site permalinks.
doc_names = ["installation", "usage", "picker", "configuration",
             "output", "delta", "dependencies", "follow", "mcp"]
for name in doc_names:
    body = re.sub(
        r"\]\(" + name + r"\.md(#[^)]*)?\)",
        r"]({{ '/docs/" + name + r"/' | relative_url }}\1)",
        body,
    )
# Repo-root links that only make sense on GitHub.
body = body.replace("](../justfile)",
                    f"]({repo_url}/blob/main/justfile)")
body = body.replace("](../README.md)", "]({{ '/' | relative_url }})")
# Doc-local assets, copied to /assets/docs/ above.
body = re.sub(r"\]\(assets/", r"]({{ '/assets/docs/", body)

front_matter = (
    "---\n"
    "layout: docs\n"
    f"title: {yq(title)}\n"
    f"description: {yq(description)}\n"
    f"nav_order: {order}\n"
    f"permalink: {permalink}\n"
)
if prev:
    pslug, ptitle = prev.split("|", 1)
    purl = "/docs/" if pslug == "index" else f"/docs/{pslug}/"
    front_matter += f"prev_title: {yq(ptitle)}\nprev_url: {purl}\n"
if nxt:
    nslug, ntitle = nxt.split("|", 1)
    nurl = "/docs/" if nslug == "index" else f"/docs/{nslug}/"
    front_matter += f"next_title: {yq(ntitle)}\nnext_url: {nurl}\n"
front_matter += "---\n"
open(dest_path, "w", encoding="utf-8").write(front_matter + body)
print(f"synced docs/{src_path.split('/')[-1]} -> _docs/{slug}.md "
      f"({title!r})")
EOF
done
