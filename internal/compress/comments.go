//go:build cgo

package compress

import (
	"strings"

	sitter "github.com/smacker/go-tree-sitter"
)

// docComment returns the consecutive comment nodes immediately above the
// declaration, or "" when none are attached. A blank line detaches a comment.
// For wrappers like export_statement the comment attaches to the wrapper, so
// the search climbs parents until a comment is found or a non-wrapper is hit.
func docComment(src []byte, n *sitter.Node) string {
	cur := n
	for cur != nil {
		if parts := attachedComments(src, cur); len(parts) > 0 {
			for i := range parts {
				parts[i] = strings.TrimRight(parts[i], " \t\r")
			}
			return strings.Join(parts, "\n") + "\n"
		}
		parent := cur.Parent()
		if parent == nil || !isWrapper(parent) {
			break
		}
		cur = parent
	}
	return ""
}

var wrapperTypes = stringSet(
	"export_statement",
	"export_clause",
	"export_default",
	"module_item",
	"declaration",
)

func isWrapper(n *sitter.Node) bool {
	return n != nil && wrapperTypes[n.Type()]
}

func attachedComments(src []byte, n *sitter.Node) []string {
	var parts []string
	cur := n
	for {
		prev := cur.PrevNamedSibling()
		if prev == nil || !isComment(prev.Type()) {
			break
		}
		if !attached(src, prev, cur) {
			break
		}
		parts = append(parts, prev.Content(src))
		cur = prev
	}
	return parts
}

// attached reports whether prev and next are separated by at most one newline.
func attached(src []byte, prev, next *sitter.Node) bool {
	gap := src[prev.EndByte():next.StartByte()]
	return strings.Count(string(gap), "\n") <= 1
}
