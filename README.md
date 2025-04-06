# Dir-Dumper

[![Go Report Card](https://goreportcard.com/badge/github.com/bethropolis/dir-dumper)](https://goreportcard.com/report/github.com/bethropolis/dir-dumper)
[![GitHub release (latest by date)](https://img.shields.io/github/v/release/bethropolis/dir-dumper?style=flat-square&labelColor=1e1e2e&color=89b4fa)](https://github.com/bethropolis/dir-dumper/releases/latest)
[![GitHub license](https://img.shields.io/github/license/bethropolis/dir-dumper?style=flat-square&labelColor=1e1e2e&color=cba6f7)](https://github.com/bethropolis/dir-dumper/blob/main/LICENSE)
[![Go Reference](https://pkg.go.dev/badge/github.com/bethropolis/dir-dumper.svg)](https://pkg.go.dev/github.com/bethropolis/dir-dumper/) 
[![GitHub stars](https://img.shields.io/github/stars/bethropolis/dir-dumper?style=flat-square&labelColor=1e1e2e&color=f9e2af)](https://github.com/bethropolis/dir-dumper/stargazers)
[![GitHub issues](https://img.shields.io/github/issues/bethropolis/dir-dumper?style=flat-square&labelColor=1e1e2e&color=f38ba8)](https://github.com/bethropolis/dir-dumper/issues)
[![Go Version](https://img.shields.io/badge/Go-1.21+-a6e3a1?style=flat-square&logo=go&labelColor=1e1e2e)](https://golang.org/doc/go1.21)


`dir-dumper` is a command-line tool written in Go that recursively traverses a directory, reads the content of non-ignored files, and prints them to standard output or a specified file. It respects `.gitignore` rules, hidden file conventions, and provides various filtering and formatting options.

The primary goal is to easily aggregate the content of a project's codebase or configuration files into a single block of text, useful for sharing context, documentation, or feeding into other tools (like Large Language Models).

## Features

*   **Recursive Traversal:** Scans directories and subdirectories.
*   **.gitignore Aware:** Respects rules defined in `.gitignore` files found within the scanned directory tree.
*   **Hidden File Handling:** Option to ignore or include hidden files and directories (those starting with `.`).
*   **Filtering:**
    *   Filter included files by extension (`-ext`).
    *   Define custom ignore patterns (`-ignore`).
    *   Set maximum file size limits (`-max-size`).
*   **Output Formats:**
    *   Standard plain text (default).
    *   JSON output (`-json`).
    *   Markdown output (`-markdown`).
*   **Concurrency:** Optional parallel processing for faster scans (`-concurrent`).
*   **Customizable:** Numerous flags to control behavior (see Usage).
*   **Tracking:** Option to display a summary of skipped files and reasons (`-show-skipped`).
*   **Progress:** Optional progress display for long scans (`-progress`).
*   **Timeout:** Set a maximum execution time (`-timeout`).
*   **Cross-Platform:** Built with Go, runs on Linux, macOS, and Windows.

## Installation

### Using `go install` (Recommended)

If you have Go (1.21+) installed and configured:

```bash
go install github.com/bethropolis/dir-dumper/cmd/dir-dumper@latest
```

> [!NOTE]
> This will download the source code, compile it, and place the `dir-dumper` binary in your `$GOPATH/bin` directory (usually `$HOME/go/bin`). Ensure this directory is in your system's `PATH`.

### From Source

1.  **Clone the repository:**
    ```bash
    git clone https://github.com/bethropolis/dir-dumper.git
    cd dir-dumper
    ```
2.  **Build the binary:**
    ```bash
    go build -o dir-dumper ./cmd/dir-dumper/
    ```
3.  **(Optional) Move the binary to a directory in your PATH:**
    ```bash
    # Example: move to ~/.local/bin 
    mv dir-dumper ~/.local/bin/
    ```


## Usage

```bash
dir-dumper [flags]
```
> [!NOTE]
> By default, `dir-dumper` scans the current directory (`.`) and prints the content of non-ignored files to standard output.

<details>
<summary>Examples</summary>

*   **Scan the current directory:**
      ```bash
      dir-dumper
      ```
*   **Scan a specific directory:**
      ```bash
      dir-dumper -dir /path/to/your/project
      ```
*   **Only include Go and Markdown files:**
      ```bash
      dir-dumper -ext go,md
      ```
*   **Ignore all `.log` files and the `dist/` directory, in addition to `.gitignore` rules:**
      ```bash
      dir-dumper -ignore "*.log,dist/"
      ```
*   **Include hidden files (usually ignored):**
      ```bash
      dir-dumper -hidden=false
      ```
*   **Output to a file:**
      ```bash
      dir-dumper -output project_dump.txt
      ```
*   **Output in JSON format:**
      ```bash
      dir-dumper -json -output dump.json
      ```
*   **Output in Markdown format:**
      ```bash
      dir-dumper -markdown -output dump.md
      ```
*   **Use concurrent processing and show progress:**
      ```bash
      dir-dumper -concurrent -progress
      ```
*   **Show skipped files at the end:**
      ```bash
      dir-dumper -show-skipped
      ```
*   **Set a 5-minute timeout:**
      ```bash
      dir-dumper -timeout 5m
      ```
*   **Combine multiple options:**
      ```bash
      dir-dumper -dir ../other-project -ext go,mod -ignore "vendor/,*_test.go" -concurrent -output ../dump.txt
      ```
</details>

<details>
<summary>Flags</summary>

```
Flags:
      -concurrent
                        Enable concurrent file processing
      -dir string
                        The root directory to scan (default ".")
      -ext string
                        Only include files with these extensions (comma-separated, e.g., 'go,md,txt')
      -git
                        Ignore .git directories (default true)
      -hidden
                        Ignore hidden files/directories (starting with '.') (default true)
      -ignore string
                        Custom ignore patterns (comma-separated, gitignore syntax)
      -json
                        Output results in JSON format
      -log-level string
                        Set the logging level (DEBUG, INFO, WARN, ERROR) (default "INFO")
      -markdown
                        Output results in Markdown format
      -max-size int
                        Max file size to process in MB (0 = no limit)
      -no-color
                        Disable color output
      -output string
                        Output to file instead of stdout
      -progress
                        Show progress information
      -quiet
                        Suppress INFO messages (only show WARN, ERROR)
      -show-skipped
                        Show a list of skipped files/directories and reasons at the end
      -timeout duration
                        Maximum execution time (e.g., '30s', '5m')
      -verbose
                        Enable verbose logging (DEBUG, WARN, ERROR)
      -version
                        Show version information
      -workers int
                        Max number of concurrent workers (defaults to number of CPU cores)
```

</details>

## Development

### Prerequisites

*   Go 1.21 or later

### Building

```bash
go build -o dir-dumper ./cmd/dir-dumper/
```

### Pre-built Binaries (Optional)

Pre-built binaries for Linux, macOS, and Windows are available on the [Releases](https://github.com/bethropolis/dir-dumper/releases) page.

## Contributing

Contributions are welcome! Please feel free to submit issues and pull requests.

## License

This project is licensed under the [MIT License](LICENSE).