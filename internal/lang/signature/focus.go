//go:build cgo

package signature

import (
	"fmt"
	"strings"

	sitter "github.com/smacker/go-tree-sitter"

	"github.com/bethropolis/sift/internal/lang/types"
)

// Focus renders a file with one declaration expanded in full and its siblings
// compressed to signatures. A line range returns that slice with omission
// markers. The boolean result reports whether any declaration was emitted.
func (e *Engine) Focus(src []byte, id types.ID, symbol string, start, end int) (string, bool, error) {
	spec, ok := Lookup(id)
	if !ok {
		if symbol != "" {
			return "", false, fmt.Errorf("focus: unsupported language for symbol %q", symbol)
		}
		return "", false, nil
	}
	pool, ok := e.parsers[id]
	if !ok {
		if symbol != "" {
			return "", false, fmt.Errorf("focus: no parser for symbol %q", symbol)
		}
		return "", false, nil
	}
	parser := pool.Get().(*sitter.Parser)
	defer pool.Put(parser)
	tree := parser.Parse(nil, src)
	if tree == nil {
		return "", false, fmt.Errorf("focus: parse failed")
	}
	root := tree.RootNode()

	if start > 0 {
		return "", false, nil // line ranges use the raw slice in compress.Focus
	}

	target := findSymbolNode(root, src, symbol)
	if target == nil {
		return "", false, fmt.Errorf("focus: symbol %q not found", symbol)
	}

	var b strings.Builder
	for i := 0; i < int(root.NamedChildCount()); i++ {
		n := root.NamedChild(i)
		if n == nil {
			continue
		}
		if n == target || isAncestor(n, target) {
			if doc := docComment(src, n); doc != "" {
				b.WriteString(doc)
			}
			b.WriteString(strings.TrimSpace(n.Content(src)))
			b.WriteString("\n\n")
			continue
		}
		if spec.Declarations[n.Type()] {
			e.emitNode(n, src, spec, &b, true)
			continue
		}
		// Wrappers such as export_statement: walk so nested declarations are
		// still emitted as signatures.
		e.walk(n, src, spec, &b, true)
	}
	if b.Len() == 0 {
		return "", false, fmt.Errorf("focus: symbol %q is not in a top-level declaration", symbol)
	}
	return strings.TrimRight(b.String(), "\n"), true, nil
}

// findSymbolNode finds a declaration named symbol. "Type.Method" also matches
// a member inside the declaration named Type.
func findSymbolNode(root *sitter.Node, src []byte, symbol string) *sitter.Node {
	wantType, wantName := "", symbol
	if idx := strings.LastIndex(symbol, "."); idx > 0 {
		wantType, wantName = symbol[:idx], symbol[idx+1:]
	}
	var found *sitter.Node
	var walk func(n *sitter.Node)
	walk = func(n *sitter.Node) {
		if found != nil || n == nil {
			return
		}
		if n.IsNamed() && declarationName(n, src) == wantName {
			if wantType == "" || ancestorNamed(n, src, wantType) || receiverHasType(n, src, wantType) {
				found = n
				return
			}
		}
		for i := 0; i < int(n.NamedChildCount()); i++ {
			walk(n.NamedChild(i))
		}
	}
	walk(root)
	return found
}

// declarationName returns the node's declared name, trying the grammar's name
// field first and then the first identifier-ish child.
func declarationName(n *sitter.Node, src []byte) string {
	if name := n.ChildByFieldName("name"); name != nil {
		return name.Content(src)
	}
	for i := 0; i < int(n.NamedChildCount()); i++ {
		child := n.NamedChild(i)
		switch child.Type() {
		case "identifier", "type_identifier", "field_identifier", "property_identifier":
			return child.Content(src)
		}
	}
	return ""
}

// receiverHasType reports whether a method receiver names typ (Go's
// "func (t T) M").
func receiverHasType(n *sitter.Node, src []byte, typ string) bool {
	recv := n.ChildByFieldName("receiver")
	return recv != nil && strings.Contains(recv.Content(src), typ)
}

// ancestorNamed reports whether any ancestor of n (within the tree) is a
// declaration named name.
func ancestorNamed(n *sitter.Node, src []byte, name string) bool {
	for p := n.Parent(); p != nil; p = p.Parent() {
		if declarationName(p, src) == name {
			return true
		}
	}
	return false
}

// isAncestor reports whether a contains b.
func isAncestor(a, b *sitter.Node) bool {
	for p := b.Parent(); p != nil; p = p.Parent() {
		if p == a {
			return true
		}
	}
	return false
}
