# MCP server

`sift mcp [path]` starts a stateless [Model Context Protocol](https://modelcontextprotocol.io/)
server over stdio. It gives coding agents direct access to sift's ranking,
signature compression, and token-budget optimizer, instead of making them
read whole files through generic filesystem tools and burn context on
outlines or unrelated diffs.

The server is hand-rolled on the standard library (`encoding/json` +
`bufio.Scanner`) — no `mcp-go`, no extra dependencies — and speaks
newline-delimited JSON-RPC 2.0 over stdin/stdout.

## Setup: one global entry, or one per project?

**Short answer: for terminal/editor-based agents, set it up once, globally,
with no path — it just works per-project. For chat-first desktop apps, you
need one entry per repo.**

The `[path]` argument to `sift mcp` is optional. Omitted, sift serves the
process's current working directory at launch. Whether that's "dynamic"
comes down to how the client starts the subprocess:

- **Workspace-aware clients** (Claude Code, Cursor, Windsurf, Cline, Zed,
  VS Code, OpenCode) spawn the local MCP server as a child process of the
  session you have open, with **cwd already set to the project you're
  working in**. Point one **global/user-scoped** config entry at
  `sift mcp` (no path), and every project you open gets its own correctly-
  scoped server automatically — nothing to edit per-repo.
- **Chat-first apps with no open project** (Claude Desktop) don't have a
  "current project" to inherit — the subprocess launches from wherever the
  app starts subprocesses (typically your home directory), not your repo.
  For these you must pass an **absolute path** explicitly, which in
  practice means one config block per repo you want to serve.
- **The spec-correct mechanism** for this is the MCP `roots` capability,
  where the *client* tells the server its workspace root over the
  protocol (`roots/list`) instead of the server inferring it from process
  cwd. Some harnesses (Claude Code, OpenCode) already lean on this model.
  sift's server doesn't implement `roots/list` yet — cwd-at-launch is the
  current mechanism, and `roots` support is on the roadmap so a single
  long-lived server instance could serve multiple open folders correctly.
  Until then, cwd-per-launch is what makes the "global config" pattern
  work, and it covers every harness below except Claude Desktop.

## Requirements

- `sift` installed and resolvable as a binary (either on `PATH`, or an
  absolute path in the config).
- Nothing else — no network port, no config file, no touching
  `~/.config/sift/state.json` (see [Statelessness](#statelessness)).

> **stdio hygiene:** the protocol is framed over stdout. Anything sift or
> a shell wrapper prints to stdout that isn't a JSON-RPC message will
> break the client's parser. If you invoke sift through a wrapper script,
> make sure it doesn't echo anything.

## Client setup

Global config (no path — recommended) unless noted otherwise.

<details>
<summary><strong>Claude Code</strong></summary>

CLI, one-time, user-scoped (applies to every project you open):

```bash
claude mcp add --scope user sift -- sift mcp
```

Or edit `~/.claude.json` / `~/.claude/settings.json` directly:

```json
{
  "mcpServers": {
    "sift": {
      "command": "sift",
      "args": ["mcp"]
    }
  }
}
```

Use `--scope local` (or a project-level `.mcp.json`) instead if you want
it scoped to one repo, or need a different `sift` build per project.

</details>

<details>
<summary><strong>Claude Desktop</strong></summary>

Not project-scoped, so cwd can't move with you — pass the path explicitly.
Settings → Developer → Edit Config → `claude_desktop_config.json`:

```json
{
  "mcpServers": {
    "sift-myrepo": {
      "command": "sift",
      "args": ["mcp", "/absolute/path/to/myrepo"]
    }
  }
}
```

Add one entry per repo you want available (`sift-myrepo`, `sift-otherrepo`, …).

</details>

<details>
<summary><strong>OpenAI Codex (CLI &amp; IDE extension — shared config)</strong></summary>

Codex uses TOML, not JSON. Either:

```bash
codex mcp add sift -- sift mcp
```

or edit `~/.codex/config.toml` directly:

```toml
[mcp_servers.sift]
command = "sift"
args = ["mcp"]
```

This applies globally across the Codex CLI and the VS Code/IDE extension.
Drop a project-scoped `.codex/config.toml` in a repo instead if you want
per-project overrides (only loaded for trusted directories).

</details>

<details>
<summary><strong>OpenCode</strong></summary>

Global: `~/.config/opencode/opencode.json`. Project (overrides global):
`opencode.json` in the repo root.

```json
{
  "$schema": "https://opencode.ai/config.json",
  "mcp": {
    "sift": {
      "type": "local",
      "command": ["sift", "mcp"],
      "enabled": true
    }
  }
}
```

OpenCode also exposes an explicit `cwd` field on local servers (relative
paths resolve from the workspace root, which is already the default), so
you rarely need it — it's there if you want to pin a server to a specific
subfolder.

</details>

<details>
<summary><strong>Cursor</strong></summary>

Global: `~/.cursor/mcp.json`. Project: `.cursor/mcp.json` in the repo.

```json
{
  "mcpServers": {
    "sift": {
      "command": "sift",
      "args": ["mcp"]
    }
  }
}
```

</details>

<details>
<summary><strong>Windsurf</strong></summary>

`~/.codeium/windsurf/mcp_config.json`:

```json
{
  "mcpServers": {
    "sift": {
      "command": "sift",
      "args": ["mcp"]
    }
  }
}
```

</details>

<details>
<summary><strong>Cline (VS Code extension)</strong></summary>

Via Cline's MCP settings UI, or `cline_mcp_settings.json` directly:

```json
{
  "mcpServers": {
    "sift": {
      "command": "sift",
      "args": ["mcp"],
      "disabled": false
    }
  }
}
```

</details>

<details>
<summary><strong>Zed</strong></summary>

`settings.json`, under `context_servers`:

```json
{
  "context_servers": {
    "sift": {
      "command": {
        "path": "sift",
        "args": ["mcp"]
      }
    }
  }
}
```

</details>

<details>
<summary><strong>VS Code (GitHub Copilot MCP)</strong></summary>

`.vscode/mcp.json` (or the user-level MCP settings for a global entry):

```json
{
  "servers": {
    "sift": {
      "command": "sift",
      "args": ["mcp"]
    }
  }
}
```

</details>

> If a client can't find `sift` on `PATH` (common with GUI apps that don't
> inherit your shell's environment on macOS), replace `"sift"` with the
> absolute path from `which sift`.

## Tools

| Tool | Params | What it does |
| --- | --- | --- |
| `pack_context` | `budget` (int, tokens), `prompt` (string, optional relevance hint), `mode` (`"full"` \| `"signatures"`, default `"full"`), `style` (output style, e.g. `markdown`/`xml`), `ext` (comma-separated string, e.g. `"go,md"`), `ignore` (comma-separated string, gitignore syntax) | Packs the directory into one ranked, token-budgeted context document. This is the primary tool — most sessions only need this one. |
| `pack_diff` | `from` (git ref; omit/empty for uncommitted working-tree changes), `patch` (bool, default `false`) | Packs only what changed since `from`, either as full file contents or a raw unified patch (`patch: true`). |
| `list_tree` | `ext` (comma-separated string), `ignore` (comma-separated string) | Lists the repository structure — paths only, no content. Cheap way to orient before calling `pack_context`. |

There is no separate `get_file_signatures` tool — call `pack_context` with
`mode: "signatures"` and narrow with `ext` if you only want, say, Go or
TypeScript outlines.

### Example: `tools/call` request/response

Request:

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "method": "tools/call",
  "params": {
    "name": "pack_context",
    "arguments": { "budget": 8000, "mode": "signatures", "ext": "go" }
  }
}
```

Response (truncated — a tree, then one fenced block per file):

```json
{
  "jsonrpc": "2.0",
  "id": 1,
  "result": {
    "content": [
      { "type": "text", "text": "```\n.\n└── main.go\n```\n\nfile: main.go\n\n```go\n...\n```\n" }
    ]
  }
}
```

## Protocol surface

Only these methods are implemented:

- `initialize`
- `notifications/initialized`
- `tools/list`
- `tools/call`

`resources/*` and `prompts/*` are explicitly deferred — the server returns
a "method not found" error rather than an empty result, so a well-behaved
client should recognize sift as tools-only rather than retrying those
calls in a loop.

## Statelessness

The server never reads or writes the shared delta baseline
(`~/.config/sift/state.json`):

- `pack_diff` requires an explicit `from` ref (or defaults to uncommitted
  changes) — it never falls back to a recorded baseline.
- `pack_context` never records a dump.

An agent hammering these tools in a tight loop can't perturb what a
human's next interactive `sift pick` session considers "the last dump."
The MCP surface and the interactive CLI are isolated from each other by
design.

## Troubleshooting

| Symptom | Likely cause |
| --- | --- |
| Client shows "server disconnected" immediately | `sift` isn't on `PATH` for that process — use an absolute path. |
| `pack_context` returns the wrong repo's content | The client launched the subprocess with a stale/wrong cwd (rare, but happens with some plugin/user-scope combinations) — pass an explicit path as a workaround, or check the client's scope settings. |
| Works in one client, not another, with identical config | The other client may not set cwd to the open project (see [Setup](#setup-one-global-entry-or-one-per-project) — Claude Desktop is the common case) — pass an explicit path. |
| `pack_context` returns far more than `budget` | `budget` omitted or zero means *unlimited*, not "a small default" — always pass an explicit token budget. |
| Client repeatedly retries `resources/list` | Client assumes resources support; harmless, confirms this server is tools-only. |
| JSON parse errors in the client log | Something else (a wrapper script, a shell rc file) is writing to stdout before the JSON-RPC handshake. |
