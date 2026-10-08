#!/bin/sh
set -eu

# Supply-chain and CSP hygiene gates for the serve web UI. Fails the caller
# on any violation; run in CI and before releases.
cd "$(dirname "$0")/../web"

[ -f bun.lock ] || { echo "web/bun.lock missing" >&2; exit 1; }

# Storage stays in src/lib/persist.ts (validated view-state only).
if grep -rn "localStorage\|sessionStorage" src --include="*.svelte"; then
    echo "storage outside lib/persist.ts" >&2
    exit 1
fi
found=$(grep -rln 'localStorage\|sessionStorage' src --include='*.ts' | grep -v '^src/lib/persist.ts$' | wc -l)
[ "$found" = "0" ] || { echo "storage outside lib/persist.ts" >&2; exit 1; }

# No raw HTML, scriptable sinks, timers, or inline styles (see
# docs/serve-protocol.md: content safety, CSP, idle design).
if grep -rn "{@html\|innerHTML\|eval(\|new Function\|setInterval" src --include="*.svelte" --include="*.ts"; then
    echo "forbidden pattern" >&2
    exit 1
fi
if grep -rn 'style="' src --include="*.svelte"; then
    echo 'inline style=" attribute' >&2
    exit 1
fi

echo "frontend hygiene OK"
