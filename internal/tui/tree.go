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
	// PreferredMode is the mode smart select should fall back to for this
	// file, derived from git commit history. Empty means "no preference".
	PreferredMode CompressMode

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
	ModeReason       string
	TestAffinity     float64
	// ApproxTokens is a byte-based token estimate shown with a "~" until the
	// exact count arrives via the scan stream. Zero once TokensFull is known.
	ApproxTokens int

	// Content and SigContent are the file's two renderings (only set on file
	// nodes). They are nil on directories.
	Content    []byte
	SigContent []byte

	// Filtered marks whether the node is hidden by the active fuzzy filter.
	Filtered   bool
	Hidden     bool
	GitIgnored bool
}

// Item is one selectable file with all data the picker needs.
type Item struct {
	// Path is the file's path relative to the scanned root.
	Path string
	// IsDir marks a directory for the structure skeleton. BuildTree forces a
	// KindDir leaf so empty directories render as folders, not files. Files
	// never set it.
	IsDir bool
	// Content is the full (redacted) content, used for the preview pane.
	Content []byte
	// SigContent is the signature-only summary when available.
	SigContent []byte
	// TokensFull and TokensSig are the token counts of the two variants.
	TokensFull int
	TokensSig  int
	// ApproxTokens is a byte-based estimate shown as "~n tok" until the exact
	// count lands. Skeleton items carry it; enriched items do not.
	ApproxTokens int
	// SecretCount is the number of secrets detected in the file.
	SecretCount int
	Hidden      bool
	GitIgnored  bool
	// RankScore is the git relevance score.
	RankScore float64
	// ModeReason explains the automatic mode recommendation when available.
	ModeReason string
	TestAffinity float64
	// PreferredMode is the git-history-backed mode preference for this file.
	// Empty means no preference; BuildTree falls back to the default full.
	PreferredMode CompressMode
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
			isFile := i == len(parts)-1 && !it.IsDir
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
			if strings.HasPrefix(part, ".") {
				child.Hidden = true
			}
			if it.IsDir && it.GitIgnored {
				child.GitIgnored = true
			}
			if isFile {
				child.TokensFull = it.TokensFull
				child.TokensSig = it.TokensSig
				child.SecretCount = it.SecretCount
				child.Hidden = it.Hidden || child.Hidden
				child.GitIgnored = it.GitIgnored
				child.RankScore = it.RankScore
				child.ModeReason = it.ModeReason
				child.TestAffinity = it.TestAffinity
				child.ApproxTokens = it.ApproxTokens
				if child.Hidden || child.GitIgnored {
					// Hidden and Git-ignored entries are opt-in only: they
					// start skipped and unselected, contributing no tokens.
					child.Mode = ModeSkip
					child.SelectState = Unselected
				} else {
					child.Mode = ModeFull
					if it.PreferredMode != "" {
						child.Mode = it.PreferredMode
					}
				}
				child.PreferredMode = it.PreferredMode
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

// indexTree records every node in the subtree into idx by path.
func indexTree(n *TreeNode, idx map[string]*TreeNode) {
	if n.Path != "" {
		idx[n.Path] = n
	}
	for _, c := range n.Children {
		indexTree(c, idx)
	}
}

// upsertItems merges a batch of items into the tree, inserting missing nodes
// and patching existing ones. It reorders the display and refreshes aggregates
// when anything changed, keeping the cursor's node stable.
func (m *model) upsertItems(items []Item) {
	changed := false
	for i := range items {
		it := &items[i]
		if m.applyItem(it) {
			changed = true
		}
	}
	if changed {
		sortTree(m.root)
		m.root.recompute()
	}
}

// applyItem inserts or patches a single item, returning true when the tree
// needs a recompute (a node was inserted or its scalar data changed). Patch-only
// batches still recompute so directory aggregates track incoming tokens.
func (m *model) applyItem(it *Item) bool {
	path := filepath.ToSlash(it.Path)
	if n, ok := m.nodeIndex[path]; ok {
		return m.patchNode(n, it)
	}

	parts := strings.Split(path, "/")
	cur := m.root
	p := ""
	changed := false
	for i, part := range parts {
		if i > 0 {
			p += "/"
		}
		p += part
		isFile := i == len(parts)-1 && !it.IsDir
		child := cur.findChild(part)
		if child == nil {
			child = &TreeNode{Name: part, Path: p, Parent: cur}
			if isFile {
				child.Kind = KindFile
			} else {
				child.Kind = KindDir
				child.Expanded = false
			}
			cur.Children = append(cur.Children, child)
			m.nodeIndex[p] = child
			changed = true
		}
		if isFile {
			if m.patchNode(child, it) {
				changed = true
			}
		}
		cur = child
	}
	return changed
}

// patchNode applies the non-zero fields of an item onto an existing node
// without disturbing content that has not arrived yet. It reports whether any
// field was applied.
func (m *model) patchNode(n *TreeNode, it *Item) bool {
	changed := false
	if it.Hidden {
		n.Hidden = true
		changed = true
	}
	if it.GitIgnored {
		n.GitIgnored = true
		changed = true
	}
	// Hidden and Git-ignored entries stay opt-in: they remain skipped and
	// unselected unless the user has already switched them to FULL or SIGS.
	if n.Hidden || n.GitIgnored {
		if n.Mode != ModeFull && n.Mode != ModeSignatures {
			n.Mode = ModeSkip
			n.SelectState = Unselected
		}
	}
	if len(it.Content) > 0 {
		n.Content = it.Content
		changed = true
	}
	if len(it.SigContent) > 0 {
		n.SigContent = it.SigContent
		changed = true
	}
	if it.TokensFull > 0 {
		n.TokensFull = it.TokensFull
		n.ApproxTokens = 0 // Exact count supersedes the estimate.
		changed = true
	}
	if it.TokensSig > 0 {
		n.TokensSig = it.TokensSig
		changed = true
	}
	if it.SecretCount > 0 {
		n.SecretCount = it.SecretCount
		changed = true
	}
	if it.RankScore != 0 {
		n.RankScore = it.RankScore
		changed = true
	}
	if it.ModeReason != "" {
		n.ModeReason = it.ModeReason
		changed = true
	}
	if it.TestAffinity != 0 {
		n.TestAffinity = it.TestAffinity
		changed = true
	}
	if it.PreferredMode != "" {
		n.PreferredMode = it.PreferredMode
		if n.Mode == "" || n.Mode == ModeFull {
			n.Mode = it.PreferredMode
		}
		changed = true
	}
	if it.ApproxTokens > 0 && n.TokensFull == 0 {
		n.ApproxTokens = it.ApproxTokens
		changed = true
	}
	return changed
}

// findRow returns the index of the first visible row whose path matches, or
// -1. It backs cursor stability when streaming inserts reorder the tree.
func (m model) findRow(path string) int {
	for i, r := range m.rows {
		if r.Path == path {
			return i
		}
	}
	return -1
}

// removeNodes deletes the given paths from the tree, reindexing and
// recomputing when anything was removed. It backs dropping skeleton files the
// full walk later rejected (e.g. generated headers), so the picker never
// offers files that can never be enriched.
func (m *model) removeNodes(paths []string) {
	removed := false
	for _, p := range paths {
		if m.removeNode(filepath.ToSlash(p)) {
			removed = true
		}
	}
	if removed {
		sortTree(m.root)
		m.root.recompute()
	}
}

// removeNode unlinks a single node (and its subtree) from the tree and drops
// it from the index. It returns false when the path is unknown.
func (m *model) removeNode(path string) bool {
	n, ok := m.nodeIndex[path]
	if !ok {
		return false
	}
	if n.Parent != nil {
		for i, c := range n.Parent.Children {
			if c == n {
				n.Parent.Children = append(n.Parent.Children[:i], n.Parent.Children[i+1:]...)
				break
			}
		}
	}
	var unindex func(x *TreeNode)
	unindex = func(x *TreeNode) {
		delete(m.nodeIndex, x.Path)
		for _, c := range x.Children {
			unindex(c)
		}
	}
	unindex(n)
	return true
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
}

func (n *TreeNode) isLastSibling() bool {
	if n.Parent == nil || len(n.Parent.Children) == 0 {
		return true
	}
	return n.Parent.Children[len(n.Parent.Children)-1] == n
}
