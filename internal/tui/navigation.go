package tui

import "strings"

// TreePrefix computes the hierarchical guide lines prefix (e.g. "│   ├── ")
// for node rendering in the explorer view.
func (n *TreeNode) TreePrefix(g Glyphs) string {
	if n.Parent == nil || n.Parent.Parent == nil {
		if n.Parent == nil {
			return ""
		}
		if n.isLastSibling() {
			return g.TreeLast
		}
		return g.TreeMiddle
	}

	var ancestors []*TreeNode
	for p := n.Parent; p != nil && p.Parent != nil; p = p.Parent {
		ancestors = append(ancestors, p)
	}

	var sb strings.Builder
	for i := len(ancestors) - 1; i >= 0; i-- {
		anc := ancestors[i]
		if anc.isLastSibling() {
			sb.WriteString(g.TreeSpace)
		} else {
			sb.WriteString(g.TreePipe)
		}
	}

	if n.isLastSibling() {
		sb.WriteString(g.TreeLast)
	} else {
		sb.WriteString(g.TreeMiddle)
	}

	return sb.String()
}

func (n *TreeNode) depth() int {
	d := 0
	for p := n.Parent; p != nil; p = p.Parent {
		d++
	}
	return d
}

// VisibleRows returns the flattened, currently expanded nodes in display
// order. When filter is non-empty only matching nodes and their ancestors are
// included.
func (n *TreeNode) VisibleRows(filter string) []*TreeNode {
	var rows []*TreeNode
	filter = strings.ToLower(strings.TrimSpace(filter))
	n.Filtered = filter != "" && !strings.Contains(strings.ToLower(n.Name), filter)
	if filter != "" {
		n.walkFiltered(&rows, filter)
	} else {
		n.walk(&rows)
	}
	return rows
}

func (n *TreeNode) walk(rows *[]*TreeNode) {
	for _, c := range n.Children {
		*rows = append(*rows, c)
		if c.Kind == KindDir && c.Expanded {
			c.walk(rows)
		}
	}
}

// walkFiltered walks with a fuzzy filter: a node is kept when its name matches
// or any descendant matches. Ancestors of a match are expanded implicitly.
func (n *TreeNode) walkFiltered(rows *[]*TreeNode, filter string) {
	for _, c := range n.Children {
		match := strings.Contains(strings.ToLower(c.Name), filter)
		c.Filtered = !match
		hasMatching := match || c.anyMatching(filter)
		if hasMatching {
			*rows = append(*rows, c)
		}
		if c.Kind == KindDir && (c.Expanded || hasMatching) {
			c.walkFiltered(rows, filter)
		}
	}
}

// anyMatching reports whether any descendant name matches the filter.
func (n *TreeNode) anyMatching(filter string) bool {
	if strings.Contains(strings.ToLower(n.Name), filter) {
		return true
	}
	for _, c := range n.Children {
		if c.anyMatching(filter) {
			return true
		}
	}
	return false
}
