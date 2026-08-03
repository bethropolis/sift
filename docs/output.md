# Output formats and prompts

Sift renders the same content in four styles. Markdown is the default and the
best all-purpose choice; JSON and XML add machine-readable structure for
downstream tooling.

## Formats

```sh
sift dump . --style markdown
sift dump . --style plain
sift dump . --style json
sift dump . --style xml
```

Every document begins with an ASCII directory tree of the selected files and
carries a per-file token count. An optional `--prompt` section can be
prepended (see below).

### Markdown

The default. Each file becomes a fenced code block titled by its path:

````
file: main.go

```go
package main

func main() { println("hello") }
```
````

### Plain

Minimal, wrapping-free text with no fenced blocks — easiest to diff or pipe
into other tools.

### JSON

An array of file objects with the path, token count, and **base64-encoded**
content (so arbitrary bytes survive the format):

```json
[
  {
    "path": "main.go",
    "content": "cGFja2FnZSBtYWluCg==",
    "tokens": 17
  }
]
```

### XML

A `<repository>` element with a `<structure>` directory tree and a `<files>`
list. Each `<file>` carries `path`, `language`, `tokens`, and `mode`
attributes; content is wrapped in CDATA so it needs no escaping:

```xml
<repository>
  <structure>
    .
    ├── main.go
    └── readme.md
  </structure>
  <files>
    <file path="main.go" language="go" tokens="17" mode="full">
<![CDATA[package main

func main() { println("hello") }
]]>
    </file>
  </files>
</repository>
```

The `mode` attribute is `full` or `signatures` depending on how the file was
rendered.

## Full and signature representations

- **Full mode** (default) keeps file contents intact.
- **Signature mode** (`--mode signatures`) replaces the implementation bodies
  of supported source files with their structure: doc comments, imports,
  package/clause headers, and declaration signatures — while dropping method
  and function bodies. This gives an LLM the shape of the code at a fraction
  of the token cost.

```sh
sift dump . --mode signatures
```

Signature compression uses tree-sitter and currently understands Go, Rust,
JavaScript/TypeScript (incl. TSX), Python, PHP, Java, Kotlin, C#, C/C++, Ruby,
and Swift. Files in other formats are emitted in full. The interactive picker
can mix full and signature modes per file.

## Task prompts

Attach instructions to the generated context with `--prompt` (short `-p`):

```sh
sift dump . --prompt "Find race conditions and explain how to fix them."
sift dump . -p "Summarize the architecture for a new teammate."
```

The prompt is emitted as an instruction section before the repository body. In
the picker, press `p` to pick a preset or write a custom directive before
generating or copying.

## Secret redaction

Secret scanning is on by default. Before rendering, Sift detects credential-
like strings (for example AWS, GitHub, Slack, Google, and Stripe keys) and
replaces each match with a placeholder such as
`[REDACTED_SECRET: aws-api-key]`. Redaction runs only on the files that will
actually be emitted, so scanning stays fast.

```sh
sift dump . --secrets            # explicit (already default)
sift dump . --force-secrets      # keep secrets instead of redacting
```

Redaction is best-effort; treat it as a guardrail, not a guarantee.

## Clipboard output

Copy a non-interactive result straight to the system clipboard instead of
writing a file:

```sh
sift dump . --clipboard
sift delta --patch --clipboard
```

## Token budgets

Keep output inside a model's context window with `--budget`. Files are ordered
by Git relevance, so the most important context survives the budget cut:

```sh
sift dump . --budget 50000
```

`0` (the default) means unlimited. See [Usage](usage.md) and
[Configuration](configuration.md) for related settings.
