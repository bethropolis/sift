// Package format renders a scanned document in configurable output styles.
package format

import (
	"fmt"
	"io"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bethropolis/sift/internal/highlight"
)

// Style identifies a supported output format.
type Style string

const (
	StylePlain    Style = "plain"
	StyleMarkdown Style = "markdown"
	StyleJSON     Style = "json"
	StyleXML      Style = "xml"
)

// FileEntry holds one file's content and metadata for rendering.
type FileEntry struct {
	Path         string
	Content      []byte
	Tokens       int
	IsCompressed bool
	Language     string

	// Picker-only fields, populated by CollectPicker for the interactive TUI.
	// SigContent is the signature-only (tree-sitter) rendering of Content.
	SigContent  []byte
	TokensFull  int
	TokensSig   int
	SecretCount int
	RankScore   float64
	Hidden      bool
	GitIgnored  bool
}

// Document is the complete set of data handed to a Renderer.
type Document struct {
	DirectoryTree string
	Files         []FileEntry
	TotalTokens   int
	// Instructions is an optional task/directive section emitted before the
	// repository body when set (--prompt).
	Instructions string
}

// Renderer renders a Document to w.
type Renderer interface {
	Render(doc *Document, w io.Writer) error
}

// NewRenderer returns a Renderer for the given style.
func NewRenderer(style Style, useColors bool) (Renderer, error) {
	return NewRendererWithOptions(style, RenderOptions{UseColors: useColors})
}

// RenderOptions controls terminal-only presentation features.
type RenderOptions struct {
	UseColors bool
	Highlight highlight.Options
	// Sections, when non-nil, collects one byte range per rendered file
	// (XML/Markdown/Plain only; JSON marshals whole and records nothing).
	// Offsets index the exact bytes written to w.
	Sections *[]Section
}

// NewRendererWithOptions creates a renderer with explicit terminal display
// options. Non-terminal renderers ignore highlighting options.
func NewRendererWithOptions(style Style, options RenderOptions) (Renderer, error) {
	switch style {
	case StylePlain:
		return &plainRenderer{useColors: options.UseColors, highlight: options.Highlight, sections: options.Sections}, nil
	case StyleMarkdown:
		return &markdownRenderer{sections: options.Sections}, nil
	case StyleJSON:
		return &jsonRenderer{}, nil
	case StyleXML:
		return &xmlRenderer{sections: options.Sections}, nil
	default:
		return nil, fmt.Errorf("unknown output style %q", style)
	}
}

// ParseStyle converts a style string to a Style, defaulting to plain.
func ParseStyle(s string) Style {
	switch Style(strings.ToLower(strings.TrimSpace(s))) {
	case StyleMarkdown:
		return StyleMarkdown
	case StyleJSON:
		return StyleJSON
	case StyleXML:
		return StyleXML
	default:
		return StylePlain
	}
}

// fprint writes a formatted string to w, returning the first write error. It
// lets renderers propagate disk-full and broken-pipe failures that would
// otherwise be silently swallowed, so callers can surface a non-zero exit.
func fprint(w io.Writer, format string, args ...any) error {
	_, err := fmt.Fprintf(w, format, args...)
	return err
}

// BuildTree renders an indented directory tree for the given relative paths.
func BuildTree(paths []string) string {
	root := &treeNode{children: map[string]*treeNode{}}
	for _, p := range paths {
		node := root
		for _, part := range strings.Split(filepath.ToSlash(p), "/") {
			if part == "" {
				continue
			}
			if node.children[part] == nil {
				node.children[part] = &treeNode{name: part, children: map[string]*treeNode{}}
			}
			node = node.children[part]
		}
		node.file = true
	}

	var sb strings.Builder
	sb.WriteString(".\n")
	writeTree(&sb, root, "")
	return sb.String()
}

type treeNode struct {
	name     string
	file     bool
	children map[string]*treeNode
}

func writeTree(sb *strings.Builder, node *treeNode, prefix string) {
	names := make([]string, 0, len(node.children))
	for name := range node.children {
		names = append(names, name)
	}
	sort.Strings(names)

	for i, name := range names {
		child := node.children[name]
		last := i == len(names)-1

		connector := "├── "
		childPrefix := prefix + "│   "
		if last {
			connector = "└── "
			childPrefix = prefix + "    "
		}

		sb.WriteString(prefix)
		sb.WriteString(connector)
		sb.WriteString(name)
		if !child.file {
			sb.WriteString("/")
		}
		sb.WriteString("\n")
		writeTree(sb, child, childPrefix)
	}
}
