// Package printer handles output formatting and display
package printer

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
)

// Printer handles output formatting and writing to the configured output destination
type Printer struct {
	output         io.Writer
	count          atomic.Int64
	mu             sync.Mutex
	useColors      bool
	jsonOutput     bool
	jsonStarted    bool
	markdownOutput bool
}

// New creates a new Printer with default settings
func New() *Printer {
	return &Printer{
		output:         os.Stdout,
		useColors:      true,
		jsonOutput:     false,
		markdownOutput: false,
	}
}

// WithOutput sets the output destination
func (p *Printer) WithOutput(w io.Writer) *Printer {
	p.output = w
	return p
}

// WithColors enables or disables colored output
func (p *Printer) WithColors(enabled bool) *Printer {
	p.useColors = enabled
	return p
}

// WithJSON enables JSON output mode
func (p *Printer) WithJSON(enabled bool) *Printer {
	p.jsonOutput = enabled
	return p
}

// WithMarkdown enables Markdown output mode
func (p *Printer) WithMarkdown(enabled bool) *Printer {
	p.markdownOutput = enabled
	return p
}

// JSONFileEntry represents a file entry in JSON output
type JSONFileEntry struct {
	Path    string `json:"path"`
	Content string `json:"content"` // Base64 encoded content
}

// PrintFile outputs the content of a file with its path
func (p *Printer) PrintFile(relativePath string, content []byte) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// Increment the file counter
	p.count.Add(1)

	if p.jsonOutput {
		// Handle JSON output mode
		if !p.jsonStarted {
			// Start the JSON array
			fmt.Fprint(p.output, "[\n")
			p.jsonStarted = true
		} else {
			// Add comma between entries
			fmt.Fprint(p.output, ",\n")
		}

		// Create and encode entry
		entry := JSONFileEntry{
			Path:    relativePath,
			Content: base64.StdEncoding.EncodeToString(content),
		}

		jsonData, err := json.MarshalIndent(entry, "  ", "  ")
		if err != nil {
			fmt.Fprintf(os.Stderr, "Error marshaling JSON: %v\n", err)
			return
		}

		// Write the JSON entry
		fmt.Fprintf(p.output, "  %s", jsonData)
	} else if p.markdownOutput {
		// Handle Markdown output mode
		lang := languageForPath(relativePath)
		fence := negotiateFence(content)
		fmt.Fprintf(p.output, "file: %s\n\n%s%s\n%s\n%s\n\n", relativePath, fence, lang, content, fence)
	} else {
		// Standard output mode
		if p.useColors {
			// Use colors for the filename
			fmt.Fprintf(p.output, "\033[1;36m%s\033[0m\n", relativePath)
		} else {
			fmt.Fprintf(p.output, "%s\n", relativePath)
		}

		// Write the content
		fmt.Fprintf(p.output, "%s\n\n", content)
	}
}

// Finalize completes any pending operations (like closing JSON array)
func (p *Printer) Finalize() {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.jsonOutput {
		if p.jsonStarted {
			fmt.Fprint(p.output, "\n]\n")
		} else {
			fmt.Fprint(p.output, "[]\n")
		}
	}
}

// negotiateFence returns a backtick fence that is longer than any run of
// backticks inside content, so nested code fences never break the block.
func negotiateFence(content []byte) string {
	maxRun := 0
	run := 0
	for _, b := range content {
		if b == '`' {
			run++
			if run > maxRun {
				maxRun = run
			}
		} else {
			run = 0
		}
	}

	n := maxRun + 1
	if n < 3 {
		n = 3
	}
	return strings.Repeat("`", n)
}

// languageForPath maps a file extension to a markdown code fence language tag.
func languageForPath(path string) string {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(path), "."))
	if lang, ok := fenceLanguages[ext]; ok {
		return lang
	}
	return ""
}

var fenceLanguages = map[string]string{
	"go":         "go",
	"rs":         "rust",
	"js":         "javascript",
	"jsx":        "jsx",
	"ts":         "typescript",
	"tsx":        "tsx",
	"py":         "python",
	"rb":         "ruby",
	"php":        "php",
	"java":       "java",
	"c":          "c",
	"h":          "c",
	"cpp":        "cpp",
	"cc":         "cpp",
	"hpp":        "cpp",
	"cs":         "csharp",
	"sh":         "bash",
	"bash":       "bash",
	"zsh":        "bash",
	"md":         "markdown",
	"markdown":   "markdown",
	"html":       "html",
	"htm":        "html",
	"css":        "css",
	"scss":       "scss",
	"json":       "json",
	"yaml":       "yaml",
	"yml":        "yaml",
	"toml":       "toml",
	"xml":        "xml",
	"sql":        "sql",
	"dockerfile": "dockerfile",
	"makefile":   "makefile",
	"lua":        "lua",
	"swift":      "swift",
	"kt":         "kotlin",
	"kts":        "kotlin",
}

// GetCount returns the number of files printed
func (p *Printer) GetCount() int64 {
	return p.count.Load()
}
