# MCP server

`sift mcp [path]` starts a stateless [Model Context Protocol](https://modelcontextprotocol.io/)
server over stdio for coding agents (Claude Code, Cursor, and friends).
Agents on generic filesystem tools burn context reading whole files when they
need outlines or just what changed; sift's ranking, signature compression,
and budget optimizer are the missing primitive. The server is hand-rolled on
the standard library — no new dependencies — and speaks newline-delimited
JSON-RPC 2.0.

## Tools

| Tool | Params | What it does |
| --- | --- | --- |
| `pack_context` | `budget`, `prompt`, `mode` (`full`/`signatures`), `style`, `ext`, `ignore` | Pack the directory into one ranked, token-budgeted context document |
| `pack_diff` | `from` (git ref; empty = uncommitted working-tree changes), `patch` (bool) | Pack only what changed, as full files or a raw patch |
| `list_tree` | `ext`, `ignore` | List the repository structure (paths only, no content) |

`get_file_signatures` needs no fourth tool: call `pack_context` with
`mode: "signatures"` and narrow with `ext`.

## Client configuration

Point your MCP client at the binary:

```json
{
  "mcpServers": {
    "sift": {
      "command": "sift",
      "args": ["mcp", "/path/to/repo"]
    }
  }
}
```

Only `initialize`, `notifications/initialized`, `tools/list`, and
`tools/call` are implemented. `resources/*` and `prompts/*` are deferred.

## Statelessness

The server never reads or writes the shared delta baseline
(`~/.config/sift/state.json`): `pack_diff` takes explicit refs and never
falls back to a recorded baseline, and `pack_context` never records a dump.
An agent hammering these tools in a loop cannot perturb what a human's next
`sift pick` session considers "the last dump."
