#!/bin/sh
set -eu

# Build the sift serve web UI (web/dist/*.gz), which the Go binary embeds.
# Used by GoReleaser's before-hooks and by `just web`. Fails the release
# when the UI is over budget.
cd "$(dirname "$0")/../web"
bun install --frozen-lockfile --silent
VITE_MOCK=false bun run build
bun scripts/check-budget.ts
