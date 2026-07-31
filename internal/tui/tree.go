package tui

import (
	"path/filepath"
	"sort"
	"strings"
)

// NodeKind identifies whether a tree node is a file or a directory.
type NodeKind int

const (
	// KindFile is a regular file leaf.
	KindFile NodeKind = iota
	// KindDir is a directory that may contain children.
	KindDir
)

// SelectState describes the checkbox state of a node.
type SelectState int

const (
	// Unselected means no descendant is selected.
	Unselected SelectState = iota
	// Partial means some descendants are selected.
	Partial
	// Selected means all descendants are selected.
	Selected
)

// CompressMode is the per-node output mode: full content, signatures only,
// or skip entirely.
type CompressMode string

const (
	// ModeFull includes the file's full content.
	ModeFull CompressMode = "full"
	// ModeSignatures includes only the tree-sitter signature summary.
	ModeSignatures CompressMode = "signatures"
	// ModeSkip excludes the node from the output.
	ModeSkip CompressMode = "skip"
)

// TreeNode is one node in the picker's foldable repository tree.
type TreeNode struct {
	Name     string
	Path     string // Relative slash-separated path; "" for the root.
	Kind     NodeKind
	Children []*TreeNode
	Parent   *TreeNode
	Expanded bool // Folder open/closed.

	SelectState SelectState
	Mode        CompressMode // Effective mode, default full.

	// TokensFull and TokensSig aggregate descendant token counts. For files
	// they come straight from the item; for directories they are the sum of
	// all descendants.
	TokensFull       int
	TokensSig        int
	ActiveTokens     int // Cached active tokens for O(1) lookup
	FileCountVal     int // Cached file count for O(1) lookup
	SelectedCountVal int // Cached selected file count for O(1) lookup
	SecretCount      int
	RankScore        float64

	// Content and SigContent are the file's two renderings (only set on file
	// nodes). They are nil on directories.
	Content    []byte
	SigContent []byte

	// Filtered marks whether the node is hidden by the active fuzzy filter.
	Filtered bool
}

// Item is one selectable file with all data the picker needs.
type Item struct {
	// Path is the file's path relative to the scanned root.
	Path string
	// Content is the full (redacted) content, used for the preview pane.
	Content []byte
	// SigContent is the signature-only summary when available.
	SigContent []byte
	// TokensFull and TokensSig are the token counts of the two variants.
	TokensFull int
	TokensSig  int
	// SecretCount is the number of secrets detected in the file.
	SecretCount int
	// RankScore is the git relevance score.
	RankScore float64
}

// BuildTree assembles a repository tree from flat file items. Directories are
// created implicitly and sorted before files at each level. The returned root
// has Name "." and Kind KindDir with Expanded true.
func BuildTree(items []Item) *TreeNode {
	root := &TreeNode{Name: ".", Kind: KindDir, Expanded: true}
	// Keep files sorted by path so ranks remain stable.
	sorted := append([]Item(nil), items...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Path < sorted[j].Path })

	for _, it := range sorted {
		parts := strings.Split(filepath.ToSlash(it.Path), "/")
		cur := root
		path := ""
		for i, part := range parts {
			if i > 0 {
				path += "/"
			}
			path += part
			isFile := i == len(parts)-1
			child := cur.findChild(part)
			if child == nil {
				child = &TreeNode{
					Name:   part,
					Path:   path,
					Parent: cur,
				}
				if isFile {
					child.Kind = KindFile
				} else {
					child.Kind = KindDir
					// Directories start collapsed so the initial view shows a
					// tidy top level; expand with Enter, l, or Right.
					child.Expanded = false
				}
				cur.Children = append(cur.Children, child)
			}
			if isFile {
				child.TokensFull = it.TokensFull
				child.TokensSig = it.TokensSig
				child.SecretCount = it.SecretCount
				child.RankScore = it.RankScore
				child.Mode = ModeFull
				child.Content = it.Content
				child.SigContent = it.SigContent
				child.Children = nil
			}
			cur = child
		}
	}

	sortTree(root)
	root.recompute()
	return root
}

// findChild returns the child with the given name, or nil.
func (n *TreeNode) findChild(name string) *TreeNode {
	for _, c := range n.Children {
		if c.Name == name {
			return c
		}
	}
	return nil
}

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

func (n *TreeNode) isLastSibling() bool {
	if n.Parent == nil || len(n.Parent.Children) == 0 {
		return true
	}
	return n.Parent.Children[len(n.Parent.Children)-1] == n
}

// sortTree orders children: directories first, then files, alphabetically.
func sortTree(n *TreeNode) {
	sort.Slice(n.Children, func(i, j int) bool {
		a, b := n.Children[i], n.Children[j]
		if a.Kind != b.Kind {
			return a.Kind == KindDir
		}
		return a.Name < b.Name
	})
	for _, c := range n.Children {
		sortTree(c)
	}
}

// recompute aggregates token counts, selection state, and effective mode up
// the tree. Call after any structural change.
func (n *TreeNode) recompute() {
	if n.Kind == KindFile {
		if n.SelectState == Selected && n.Mode != ModeSkip {
			if n.Mode == ModeSignatures {
				n.ActiveTokens = n.TokensSig
			} else {
				n.ActiveTokens = n.TokensFull
			}
			n.SelectedCountVal = 1
		} else {
			n.ActiveTokens = 0
			n.SelectedCountVal = 0
		}
		n.FileCountVal = 1
		return
	}

	n.TokensFull, n.TokensSig, n.SecretCount = 0, 0, 0
	n.FileCountVal = 0
	n.SelectedCountVal = 0
	n.ActiveTokens = 0

	anySelected, anyUnselected := false, false
	effectiveMode := n.Mode
	modeSet := false

	for _, c := range n.Children {
		c.recompute()
		n.TokensFull += c.TokensFull
		n.TokensSig += c.TokensSig
		n.SecretCount += c.SecretCount
		n.FileCountVal += c.FileCountVal
		n.SelectedCountVal += c.SelectedCountVal
		n.ActiveTokens += c.ActiveTokens

		switch c.SelectState {
		case Selected:
			anySelected = true
		case Unselected:
			anyUnselected = true
		case Partial:
			anySelected, anyUnselected = true, true
		}

		if modeSet {
			if effectiveMode != c.Mode {
				effectiveMode = ModeFull // Mixed modes under one folder.
			}
		} else {
			effectiveMode = c.Mode
			modeSet = true
		}
	}

	switch {
	case anySelected && anyUnselected:
		n.SelectState = Partial
	case anySelected:
		n.SelectState = Selected
	default:
		n.SelectState = Unselected
	}

	if modeSet {
		n.Mode = effectiveMode
	} else {
		n.Mode = ModeFull
	}

	// Fast O(1) override for uniform directory states
	if n.SelectState == Unselected || n.Mode == ModeSkip {
		n.ActiveTokens = 0
	} else if n.SelectState == Selected {
		if n.Mode == ModeFull {
			n.ActiveTokens = n.TokensFull
		} else if n.Mode == ModeSignatures {
			n.ActiveTokens = n.TokensSig
		}
	}
}

// Toggle flips the node's selection. Files flip between selected and not;
// directories recursively select or deselect all descendants. Selection
// bubbles up to parents as Partial/Selected.
func (n *TreeNode) Toggle() {
	switch n.SelectState {
	case Selected, Partial:
		n.setSelected(false)
	default:
		n.setSelected(true)
	}
}

// setSelected sets the subtree to the given selection state.
func (n *TreeNode) setSelected(on bool) {
	n.setSelectedSubtree(on)
	n.recompute()
	n.bubbleUp()
}

func (n *TreeNode) setSelectedSubtree(on bool) {
	if on {
		n.SelectState = Selected
	} else {
		n.SelectState = Unselected
	}
	for _, c := range n.Children {
		c.setSelectedSubtree(on)
	}
}

// bubbleUp recomputes the SelectState of every ancestor.
func (n *TreeNode) bubbleUp() {
	for p := n.Parent; p != nil; p = p.Parent {
		p.recompute()
	}
}

// CycleMode advances the effective mode FULL -> SIGS -> SKIP -> FULL and
// applies it to the whole subtree, so a directory set to a mode propagates to
// every file beneath it.
func (n *TreeNode) CycleMode() {
	var next CompressMode
	switch n.Mode {
	case ModeFull:
		next = ModeSignatures
	case ModeSignatures:
		next = ModeSkip
	default:
		next = ModeFull
	}
	n.applyMode(next)
	n.recompute()
	n.bubbleUp()
}

// applyMode sets the mode on n and every descendant.
func (n *TreeNode) applyMode(m CompressMode) {
	n.Mode = m
	for _, c := range n.Children {
		c.applyMode(m)
	}
}

// TotalActiveTokens returns the cached active tokens for n in O(1) time.
func (n *TreeNode) TotalActiveTokens() int {
	return n.ActiveTokens
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

// Selection pairs a selected path with its output mode.
type Selection struct {
	Path string
	Mode CompressMode
}

// Selections returns the chosen files in presentation order, each with its
// active mode. Files under skipped directories are omitted.
func (n *TreeNode) Selections() []Selection {
	var out []Selection
	n.collectSelections(&out)
	return out
}

func (n *TreeNode) collectSelections(out *[]Selection) {
	for _, c := range n.Children {
		if c.SelectState == Unselected || c.Mode == ModeSkip {
			continue
		}
		if c.Kind == KindFile {
			*out = append(*out, Selection{Path: c.Path, Mode: c.Mode})
			continue
		}
		c.collectSelections(out)
	}
}

// FileCount returns the cached file count under n in O(1) time.
func (n *TreeNode) FileCount() int {
	return n.FileCountVal
}

// SelectedCount returns the cached selected file count under n in O(1) time.
func (n *TreeNode) SelectedCount() int {
	return n.SelectedCountVal
}

// SelectByRank selects files in descending rank order until the cumulative
// active tokens would exceed budget (0 = unlimited). Returns the number of
// files selected.
func (n *TreeNode) SelectByRank(budget int) int {
	var files []*TreeNode
	n.collectFiles(&files)
	sort.SliceStable(files, func(i, j int) bool {
		if files[i].RankScore != files[j].RankScore {
			return files[i].RankScore > files[j].RankScore
		}
		return files[i].Path < files[j].Path
	})

	used := 0
	count := 0
	for _, f := range files {
		toks := f.TokensFull
		if f.Mode == ModeSignatures {
			toks = f.TokensSig
		}
		if budget > 0 && used+toks > budget {
			continue
		}
		f.setSelectedSubtree(true)
		used += toks
		count++
	}
	n.recompute()
	return count
}

// ClearSelection deselects every file.
func (n *TreeNode) ClearSelection() {
	for _, c := range n.Children {
		c.setSelectedSubtree(false)
	}
	n.recompute()
}

func (n *TreeNode) collectFiles(out *[]*TreeNode) {
	for _, c := range n.Children {
		if c.Kind == KindFile {
			*out = append(*out, c)
			continue
		}
		c.collectFiles(out)
	}
}
