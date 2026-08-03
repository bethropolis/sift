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
	return n.VisibleRowsWithOptions(filter, false, false)
}

// VisibleRowsWithOptions flattens the tree while applying fuzzy and
// presentation-visibility filters.
func (n *TreeNode) VisibleRowsWithOptions(filter string, showHidden, showGitIgnored bool) []*TreeNode {
	var rows []*TreeNode
	filter = strings.ToLower(strings.TrimSpace(filter))
	n.Filtered = filter != "" && !strings.Contains(strings.ToLower(n.Name), filter)
	if filter != "" {
		n.walkFiltered(&rows, filter, showHidden, showGitIgnored)
	} else {
		n.walk(&rows, showHidden, showGitIgnored)
	}
	return rows
}

func (n *TreeNode) walk(rows *[]*TreeNode, showHidden, showGitIgnored bool) {
	for _, c := range n.Children {
		if !nodeVisible(c, showHidden, showGitIgnored) {
			continue
		}
		*rows = append(*rows, c)
		if c.Kind == KindDir && c.Expanded {
			c.walk(rows, showHidden, showGitIgnored)
		}
	}
}

// ExpandAll expands every directory in the subtree. The root's own state is
// left untouched.
func (n *TreeNode) ExpandAll() {
	for _, c := range n.Children {
		if c.Kind == KindDir {
			c.Expanded = true
			c.ExpandAll()
		}
	}
}

// CollapseAll collapses every directory in the subtree. The root's own state
// is left untouched.
func (n *TreeNode) CollapseAll() {
	for _, c := range n.Children {
		if c.Kind == KindDir {
			c.Expanded = false
			c.CollapseAll()
		}
	}
}

// walkFiltered walks with a fuzzy filter: a node is kept when its name matches
// or any descendant matches. Ancestors of a match are expanded implicitly.
func (n *TreeNode) walkFiltered(rows *[]*TreeNode, filter string, showHidden, showGitIgnored bool) {
	for _, c := range n.Children {
		if !nodeVisible(c, showHidden, showGitIgnored) {
			continue
		}
		match := strings.Contains(strings.ToLower(c.Name), filter)
		c.Filtered = !match
		hasMatching := match || c.anyMatching(filter, showHidden, showGitIgnored)
		if hasMatching {
			*rows = append(*rows, c)
		}
		if c.Kind == KindDir && (c.Expanded || hasMatching) {
			c.walkFiltered(rows, filter, showHidden, showGitIgnored)
		}
	}
}

// anyMatching reports whether any descendant name matches the filter.
func (n *TreeNode) anyMatching(filter string, showHidden, showGitIgnored bool) bool {
	if !nodeVisible(n, showHidden, showGitIgnored) {
		return false
	}
	if strings.Contains(strings.ToLower(n.Name), filter) {
		return true
	}
	for _, c := range n.Children {
		if c.anyMatching(filter, showHidden, showGitIgnored) {
			return true
		}
	}
	return false
}

func nodeVisible(n *TreeNode, showHidden, showGitIgnored bool) bool {
	if n.Hidden && !showHidden {
		return false
	}
	if n.GitIgnored && !showGitIgnored {
		return false
	}
	return true
}
