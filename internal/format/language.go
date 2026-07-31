package format

import (
	"path/filepath"
	"strings"
)

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

// languageForPath maps a file extension to a language tag.
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
